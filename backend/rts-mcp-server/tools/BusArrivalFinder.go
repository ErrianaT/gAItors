package tools

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

//
// -----------------------------------------------------------------------------
// Central debug logger (busfinder.log)
// -----------------------------------------------------------------------------

// busLog writes all debug output to busfinder.log in the current working dir.
var busLog *log.Logger

// initBusFinderLogger initializes the logger once and writes to busfinder.log.
// If the file cannot be opened, it falls back to the default logger.
func initBusFinderLogger() {
    if busLog != nil {
        return
    }

    f, err := os.OpenFile("busfinder.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
    if err != nil {
        log.Printf("busfinder: failed to open busfinder.log: %v", err)
        busLog = log.Default()
        return
    }

    busLog = log.New(f, "", log.LstdFlags)
}

//
// -----------------------------------------------------------------------------
// Tool definition
// -----------------------------------------------------------------------------

// TransitArrivalPlanner is the MCP tool schema.
type TransitArrivalPlanner struct {
    OriginStop      string `json:"origin_stop,omitempty"`
    DestinationStop string `json:"destination_stop,omitempty"`
    Intent          string `json:"intent,omitempty"`
}

func (t *TransitArrivalPlanner) Name() string { return "TransitArrivalPlanner" }

func (t *TransitArrivalPlanner) Description() string {
    return "Realtime RTSFL bus arrivals with static GTFS fallback and 1–2 transfer options via major hubs."
}

// GetTransitArrivalPlannerTool wires the tool into the MCP server and eagerly
// loads static GTFS data for fallback. It also initializes the debug logger.
func GetTransitArrivalPlannerTool() (*protocol.Tool, server.ToolHandlerFunc) {
    initBusFinderLogger()

    toolStruct := TransitArrivalPlanner{}
    tool, err := protocol.NewTool(toolStruct.Name(), toolStruct.Description(), toolStruct)
    if err != nil {
        log.Fatalf("Failed to create tool: %v", err)
    }

    if err := loadGTFSStops("../data/stops.txt"); err != nil {
        busLog.Printf("Static fallback: failed to load stops.txt: %v", err)
    }
    if err := indexStopTimes("../data/stop_times.txt"); err != nil {
        busLog.Printf("Static fallback: failed to load stop_times.txt: %v", err)
    }

    return tool, handleTransitArrivalPlanner
}

//
// -----------------------------------------------------------------------------
// GTFS static data
// -----------------------------------------------------------------------------

// StaticStop represents a GTFS stop with coordinates.
type StaticStop struct {
    StopID   string
    StopName string
    Lat      float64
    Lon      float64
}

// stopsCache holds all GTFS stops in memory for fuzzy resolution.
var stopsCache []StaticStop

// StaticStopTime is a single row from stop_times.txt.
type StaticStopTime struct {
    TripID string
    StopID string
    Arr    string
    Dep    string
    Seq    int
}

// tripStopIndex maps trip_id → ordered list of stop times for static fallback.
var tripStopIndex map[string][]StaticStopTime

// loadGTFSStops loads stops.txt into memory and logs how many stops were loaded.
func loadGTFSStops(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    reader := csv.NewReader(bufio.NewReader(f))
    headers, err := reader.Read()
    if err != nil {
        return err
    }
    col := indexCols(headers)

    count := 0
    for {
        rec, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        lat, _ := strconv.ParseFloat(rec[col["stop_lat"]], 64)
        lon, _ := strconv.ParseFloat(rec[col["stop_lon"]], 64)

        stopsCache = append(stopsCache, StaticStop{
            StopID:   rec[col["stop_id"]],
            StopName: rec[col["stop_name"]],
            Lat:      lat,
            Lon:      lon,
        })
        count++
    }

    busLog.Printf("Loaded %d GTFS stops from %s", count, path)
    return nil
}

// indexCols builds a header → index map for CSV files.
func indexCols(headers []string) map[string]int {
    m := map[string]int{}
    for i, h := range headers {
        m[h] = i
    }
    return m
}

// indexStopTimes loads stop_times.txt into an in-memory index keyed by trip_id.
func indexStopTimes(path string) error {
    tripStopIndex = make(map[string][]StaticStopTime)

    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    reader := csv.NewReader(bufio.NewReader(f))
    headers, err := reader.Read()
    if err != nil {
        return err
    }
    col := indexCols(headers)

    rowCount := 0
    for {
        rec, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        seq, _ := strconv.Atoi(rec[col["stop_sequence"]])

        tripID := rec[col["trip_id"]]
        tripStopIndex[tripID] = append(tripStopIndex[tripID], StaticStopTime{
            TripID: tripID,
            StopID: rec[col["stop_id"]],
            Arr:    rec[col["arrival_time"]],
            Dep:    rec[col["departure_time"]],
            Seq:    seq,
        })
        rowCount++
    }

    busLog.Printf("Indexed %d stop_times from %s into %d trips", rowCount, path, len(tripStopIndex))
    return nil
}

//
// -----------------------------------------------------------------------------
// Fuzzy resolver (Jaro-Winkler + Levenshtein hybrid)
// -----------------------------------------------------------------------------

func jaroWinkler(s1, s2 string) float64 {
    s1 = strings.ToLower(s1)
    s2 = strings.ToLower(s2)

    if s1 == s2 {
        return 1.0
    }

    matchDistance := max(len(s1), len(s2))/2 - 1
    if matchDistance < 0 {
        matchDistance = 0
    }

    s1Matches := make([]bool, len(s1))
    s2Matches := make([]bool, len(s2))

    matches := 0
    transpositions := 0

    for i := range s1 {
        start := max(0, i-matchDistance)
        end := min(i+matchDistance+1, len(s2))

        for j := start; j < end; j++ {
            if s2Matches[j] {
                continue
            }
            if s1[i] != s2[j] {
                continue
            }
            s1Matches[i] = true
            s2Matches[j] = true
            matches++
            break
        }
    }

    if matches == 0 {
        return 0
    }

    k := 0
    for i := range s1 {
        if !s1Matches[i] {
            continue
        }
        for !s2Matches[k] {
            k++
        }
        if s1[i] != s2[k] {
            transpositions++
        }
        k++
    }

    m := float64(matches)
    jaro := (m/float64(len(s1)) +
        m/float64(len(s2)) +
        (m-float64(transpositions)/2)/m) / 3

    prefix := 0
    for i := 0; i < min(4, min(len(s1), len(s2))); i++ {
        if s1[i] == s2[i] {
            prefix++
        } else {
            break
        }
    }

    return jaro + float64(prefix)*0.1*(1.0-jaro)
}

func levenshtein(a, b string) float64 {
    a = strings.ToLower(a)
    b = strings.ToLower(b)

    da := make([][]int, len(a)+1)
    for i := range da {
        da[i] = make([]int, len(b)+1)
    }

    for i := 0; i <= len(a); i++ {
        da[i][0] = i
    }
    for j := 0; j <= len(b); j++ {
        da[0][j] = j
    }

    for i := 1; i <= len(a); i++ {
        for j := 1; j <= len(b); j++ {
            cost := 0
            if a[i-1] != b[j-1] {
                cost = 1
            }
            da[i][j] = min3(
                da[i-1][j]+1,
                da[i][j-1]+1,
                da[i-1][j-1]+cost,
            )
        }
    }

    dist := da[len(a)][len(b)]
    maxLen := max(len(a), len(b))
    if maxLen == 0 {
        return 1
    }
    return 1 - float64(dist)/float64(maxLen)
}

// resolveStaticStopFuzzy finds the best GTFS stop by fuzzy name match.
func resolveStaticStopFuzzy(name string) (*StaticStop, error) {
    raw := strings.TrimSpace(name)
    name = strings.ToLower(raw)

    bestScore := 0.0
    var best *StaticStop

    for i := range stopsCache {
        s := stopsCache[i].StopName
        jw := jaroWinkler(name, s)
        lv := levenshtein(name, s)
        score := 0.7*jw + 0.3*lv

        if score > bestScore {
            bestScore = score
            best = &stopsCache[i]
        }
    }

    if best == nil || bestScore < 0.75 {
        return nil, fmt.Errorf("no fuzzy match for %q (score %.3f)", raw, bestScore)
    }

    busLog.Printf("GTFS fuzzy resolve OK: input=%q → %q (score=%.3f)", raw, best.StopName, bestScore)
    return best, nil
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
func min3(a, b, c int) int {
    return min(a, min(b, c))
}

// scoreStops is a shared fuzzy scorer for comparing stop names.
func scoreStops(a, b string) float64 {
    jw := jaroWinkler(a, b)
    lv := levenshtein(a, b)
    return 0.7*jw + 0.3*lv
}

//
// -----------------------------------------------------------------------------
// Static planner: direct + 1-transfer + 2-transfer via major hubs
// -----------------------------------------------------------------------------

// StaticResult is a human-readable representation of a static trip.
type StaticResult struct {
    TripID     string
    DepStr     string
    ArrStr     string
    OriginID   string
    DestID     string
    OriginName string
    DestName   string
}

// majorHubs are the only transfer points we consider for connections.
var majorHubs = []string{
    "Rosa Parks RTS Downtown Station",
    "Reitz Union",
    "Butler Plaza Transfer Station",
    "Oaks Mall",
    "UF Health",
    "Santa Fe College",
    "N Walmart Supercenter",
}

// resolveStaticStopsByName returns all GTFS stops whose names contain the query.
func resolveStaticStopsByName(name string) []StaticStop {
    q := strings.ToLower(strings.TrimSpace(name))
    var out []StaticStop
    for _, s := range stopsCache {
        if strings.Contains(strings.ToLower(s.StopName), q) {
            out = append(out, s)
        }
    }
    return out
}

// staticDirectTrips returns all future direct trips between two GTFS stops.
func staticDirectTrips(origin, dest StaticStop, now time.Time, maxResults int) []StaticResult {
    var cands []StaticResult

    for tripID, seq := range tripStopIndex {
        var o *StaticStopTime
        var d *StaticStopTime

        for i := range seq {
            if seq[i].StopID == origin.StopID {
                o = &seq[i]
            }
            if seq[i].StopID == dest.StopID {
                d = &seq[i]
            }
        }

        if o != nil && d != nil && o.Seq < d.Seq {
            depTime, ok := parseTodayTime(o.Dep, now)
            if !ok {
                continue
            }
            if depTime.After(now) {
                cands = append(cands, StaticResult{
                    TripID:     tripID,
                    DepStr:     o.Dep,
                    ArrStr:     d.Arr,
                    OriginID:   origin.StopID,
                    DestID:     dest.StopID,
                    OriginName: origin.StopName,
                    DestName:   dest.StopName,
                })
            }
        }
    }

    if len(cands) == 0 {
        return nil
    }

    sort.Slice(cands, func(i, j int) bool {
        ti, _ := parseTodayTime(cands[i].DepStr, now)
        tj, _ := parseTodayTime(cands[j].DepStr, now)
        return ti.Before(tj)
    })

    if maxResults > 0 && len(cands) > maxResults {
        cands = cands[:maxResults]
    }
    return cands
}

// staticNextTripsString is the original string-returning wrapper for direct trips.
func staticNextTripsString(originName, destName string, maxResults int) (string, error) {
    busLog.Printf("Static fallback search: origin=%q dest=%q maxResults=%d", originName, destName, maxResults)

    orig := resolveStaticStopsByName(originName)
    dest := resolveStaticStopsByName(destName)

    if len(orig) == 0 || len(dest) == 0 {
        busLog.Printf("Static fallback: cannot resolve origin/destination: originMatches=%d destMatches=%d", len(orig), len(dest))
        return "", fmt.Errorf("static fallback: cannot resolve origin/destination")
    }

    origin := orig[0]
    destination := dest[0]
    busLog.Printf("Static fallback using originID=%s(%q) destID=%s(%q)", origin.StopID, origin.StopName, destination.StopID, destination.StopName)

    now := time.Now()
    cands := staticDirectTrips(origin, destination, now, maxResults)
    if len(cands) == 0 {
        busLog.Printf("Static fallback found 0 candidate trips")
        return "No upcoming scheduled trips found today.", nil
    }

    var b strings.Builder
    b.WriteString("Scheduled trips (static GTFS):\n")
    for _, c := range cands {
        b.WriteString(fmt.Sprintf("- Trip %s: depart %s → arrive %s\n",
            c.TripID, c.DepStr, c.ArrStr))
    }

    busLog.Printf("Static fallback returning %d candidate trips", len(cands))
    return b.String(), nil
}

// parseTodayTime converts an HH:MM:SS string (possibly >24h) into a time.Time.
func parseTodayTime(hms string, base time.Time) (time.Time, bool) {
    parts := strings.Split(hms, ":")
    if len(parts) != 3 {
        return time.Time{}, false
    }
    h, _ := strconv.Atoi(parts[0])
    m, _ := strconv.Atoi(parts[1])
    s, _ := strconv.Atoi(parts[2])

    dayOffset := h / 24
    h = h % 24

    return time.Date(base.Year(), base.Month(), base.Day()+dayOffset, h, m, s, 0, base.Location()), true
}

// staticConnections1Transfer finds 1-transfer options via major hubs.
func staticConnections1Transfer(origin, dest StaticStop, now time.Time, maxPerHub int) []string {
    var out []string

    for _, hubName := range majorHubs {
        // Skip if hub is effectively origin or destination.
        if strings.EqualFold(hubName, origin.StopName) || strings.EqualFold(hubName, dest.StopName) {
            continue
        }
        hubs := resolveStaticStopsByName(hubName)
        if len(hubs) == 0 {
            continue
        }
        hub := hubs[0]

        leg1 := staticDirectTrips(origin, hub, now, maxPerHub)
        if len(leg1) == 0 {
            continue
        }

        // Use arrival of first leg as earliest departure for second leg.
        arr1Time, ok := parseTodayTime(leg1[0].ArrStr, now)
        if !ok {
            continue
        }
        // Add a small transfer buffer (e.g., 5 minutes).
        arr1Time = arr1Time.Add(5 * time.Minute)

        leg2 := staticDirectTrips(hub, dest, arr1Time, maxPerHub)
        if len(leg2) == 0 {
            continue
        }

        // Build a human-readable description for the best combo.
        leg2ArrTime, _ := parseTodayTime(leg2[0].ArrStr, now)
        desc := fmt.Sprintf(
            "- %s → %s → %s\n  Leg 1: depart %s → arrive %s\n  Leg 2: depart %s → arrive %s\n  Final arrival: %s\n",
            origin.StopName, hub.StopName, dest.StopName,
            leg1[0].DepStr, leg1[0].ArrStr,
            leg2[0].DepStr, leg2[0].ArrStr,
            leg2ArrTime.Format("15:04:05"),
        )
        out = append(out, desc)
    }

    return out
}

// staticConnections2Transfer finds 2-transfer options via pairs of major hubs.
func staticConnections2Transfer(origin, dest StaticStop, now time.Time, maxPerLeg int) []string {
    var out []string

    for i, hub1Name := range majorHubs {
        for j, hub2Name := range majorHubs {
            if i == j {
                continue
            }
            // Avoid trivial loops.
            if strings.EqualFold(hub1Name, origin.StopName) ||
                strings.EqualFold(hub2Name, origin.StopName) ||
                strings.EqualFold(hub1Name, dest.StopName) ||
                strings.EqualFold(hub2Name, dest.StopName) {
                continue
            }

            h1s := resolveStaticStopsByName(hub1Name)
            h2s := resolveStaticStopsByName(hub2Name)
            if len(h1s) == 0 || len(h2s) == 0 {
                continue
            }
            h1 := h1s[0]
            h2 := h2s[0]

            // Leg 1: origin → hub1
            leg1 := staticDirectTrips(origin, h1, now, maxPerLeg)
            if len(leg1) == 0 {
                continue
            }
            arr1, ok := parseTodayTime(leg1[0].ArrStr, now)
            if !ok {
                continue
            }
            arr1 = arr1.Add(5 * time.Minute)

            // Leg 2: hub1 → hub2
            leg2 := staticDirectTrips(h1, h2, arr1, maxPerLeg)
            if len(leg2) == 0 {
                continue
            }
            arr2, ok := parseTodayTime(leg2[0].ArrStr, now)
            if !ok {
                continue
            }
            arr2 = arr2.Add(5 * time.Minute)

            // Leg 3: hub2 → dest
            leg3 := staticDirectTrips(h2, dest, arr2, maxPerLeg)
            if len(leg3) == 0 {
                continue
            }
            finalArr, _ := parseTodayTime(leg3[0].ArrStr, now)

            desc := fmt.Sprintf(
                "- %s → %s → %s → %s\n  Leg 1: %s → %s\n  Leg 2: %s → %s\n  Leg 3: %s → %s\n  Final arrival: %s\n",
                origin.StopName, h1.StopName, h2.StopName, dest.StopName,
                leg1[0].DepStr, leg1[0].ArrStr,
                leg2[0].DepStr, leg2[0].ArrStr,
                leg3[0].DepStr, leg3[0].ArrStr,
                finalArr.Format("15:04:05"),
            )
            out = append(out, desc)
        }
    }

    return out
}

//
// -----------------------------------------------------------------------------
// Transit API client
// -----------------------------------------------------------------------------

const TransitBaseURL = "https://external.transitapp.com"

// TransitClient wraps HTTP access to the Transit API.
type TransitClient struct {
    HTTPClient *http.Client
    APIKey     string
}

// NewTransitClient constructs a client with timeout and API key from env.
func NewTransitClient() *TransitClient {
    apiKey := os.Getenv("TRANSIT_API_KEY")
    if apiKey == "" {
        apiKey = "123456789"
    }
    busLog.Printf("TransitClient created with API key (len=%d)", len(apiKey))

    return &TransitClient{
        HTTPClient: &http.Client{Timeout: 12 * time.Second},
        APIKey:     apiKey,
    }
}

// doGET performs a GET request, logs the path and query, and unmarshals JSON.
func (c *TransitClient) doGET(path string, query map[string]string, out interface{}) error {
    req, err := http.NewRequest("GET", TransitBaseURL+path, nil)
    if err != nil {
        return err
    }

    q := req.URL.Query()
    for k, v := range query {
        q.Set(k, v)
    }
    req.URL.RawQuery = q.Encode()
    req.Header.Set("apiKey", c.APIKey)

    busLog.Printf("HTTP GET %s?%s", path, q.Encode())

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        busLog.Printf("HTTP GET %s returned status %d", path, resp.StatusCode)
        return fmt.Errorf("API returned %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    if out != nil {
        return json.Unmarshal(body, out)
    }
    return nil
}

//
// -----------------------------------------------------------------------------
// Nearby stops resolver (RTSFL-only)
// -----------------------------------------------------------------------------

type NearbyStopsResponse struct {
    Stops []struct {
        GlobalStopID string  `json:"global_stop_id"`
        StopName     string  `json:"stop_name"`
        StopLat      float64 `json:"stop_lat"`
        StopLon      float64 `json:"stop_lon"`
        Distance     float64 `json:"distance"`
    } `json:"stops"`
}

// resolveStopByNearbyRTSFL calls nearby_stops and returns the closest RTSFL:*
// stop within a small radius, expanding once if needed.
func resolveStopByNearbyRTSFL(c *TransitClient, lat, lon float64) (string, string, error) {
    radius := 100
    for attempt := 0; attempt < 2; attempt++ {
        busLog.Printf("Calling nearby_stops for lat=%.6f lon=%.6f radius=%dm (RTSFL only)", lat, lon, radius)

        var resp NearbyStopsResponse
        err := c.doGET("/v3/public/nearby_stops", map[string]string{
            "lat":    fmt.Sprintf("%f", lat),
            "lon":    fmt.Sprintf("%f", lon),
            "radius": fmt.Sprintf("%d", radius),
        }, &resp)
        if err != nil {
            return "", "", err
        }
        if len(resp.Stops) == 0 {
            busLog.Printf("nearby_stops returned 0 stops at radius=%dm", radius)
        }

        var bestGSID, bestName string
        bestDist := 0.0
        for _, s := range resp.Stops {
            if !strings.HasPrefix(s.GlobalStopID, "RTSFL:") {
                continue
            }
            if bestGSID == "" || s.Distance < bestDist {
                bestGSID = s.GlobalStopID
                bestName = s.StopName
                bestDist = s.Distance
            }
        }

        if bestGSID != "" {
            busLog.Printf("nearby_stops RTSFL best match at radius=%dm: GSID=%s name=%q dist=%.2f",
                radius, bestGSID, bestName, bestDist)
            return bestGSID, bestName, nil
        }

        if attempt == 0 {
            radius = 250
            continue
        }
    }

    return "", "", fmt.Errorf("no RTSFL nearby stops found")
}

//
// -----------------------------------------------------------------------------
// Realtime models and direct realtime finder (no trip_details)
// -----------------------------------------------------------------------------

type StopDeparturesResponse struct {
    RouteDepartures []TransitRoute `json:"route_departures"`
}

type TransitRoute struct {
    GlobalRouteID  string      `json:"global_route_id"`
    RouteShortName string      `json:"route_short_name"`
    RouteLongName  string      `json:"route_long_name"`
    Itineraries    []Itinerary `json:"itineraries"`
}

type Itinerary struct {
    Headsign      string         `json:"headsign"`
    ScheduleItems []ScheduleItem `json:"schedule_items"`
}

type ScheduleItem struct {
    DepartureTime int64  `json:"departure_time"`
    IsCancelled   bool   `json:"is_cancelled"`
    IsRealTime    bool   `json:"is_real_time"`
    TripSearchKey string `json:"trip_search_key"`
}

// PlannedDeparture is the internal representation of a candidate realtime trip.
type PlannedDeparture struct {
    RouteShortName string
    RouteLongName  string
    DepartureUnix  int64
    Headsign       string
    IsRealTime     bool
    OriginName     string
    DestName       string
}

// findDirectRealtimeTripsFast finds direct realtime trips by matching the
// destination name against the route headsign or route long name. It does not
// call trip_details, so it is safe from 429 rate limits.
func findDirectRealtimeTripsFast(c *TransitClient, originGSID, originName, destName string) ([]PlannedDeparture, error) {
    busLog.Printf("Realtime fast-path search: originGSID=%s originName=%q destName=%q", originGSID, originName, destName)

    var dep StopDeparturesResponse
    err := c.doGET("/v3/public/stop_departures", map[string]string{
        "global_stop_id":         originGSID,
        "max_num_departures":     "20",
        "should_update_realtime": "true",
        "remove_cancelled":       "true",
    }, &dep)
    if err != nil {
        return nil, err
    }

    busLog.Printf("stop_departures returned %d routes", len(dep.RouteDepartures))

    var results []PlannedDeparture
    destLower := strings.ToLower(destName)

    for _, route := range dep.RouteDepartures {
        busLog.Printf("Route %s (%s) has %d itineraries", route.RouteShortName, route.RouteLongName, len(route.Itineraries))
        for _, itin := range route.Itineraries {
            busLog.Printf("  Itinerary headsign=%q has %d schedule items", itin.Headsign, len(itin.ScheduleItems))

            headLower := strings.ToLower(itin.Headsign)
            routeLongLower := strings.ToLower(route.RouteLongName)

            if !strings.Contains(headLower, destLower) && !strings.Contains(routeLongLower, destLower) {
                continue
            }

            for _, sched := range itin.ScheduleItems {
                if sched.IsCancelled {
                    continue
                }
                results = append(results, PlannedDeparture{
                    RouteShortName: route.RouteShortName,
                    RouteLongName:  route.RouteLongName,
                    DepartureUnix:  sched.DepartureTime,
                    Headsign:       itin.Headsign,
                    IsRealTime:     sched.IsRealTime,
                    OriginName:     originName,
                    DestName:       destName,
                })
                busLog.Printf("    Fast-path match: route=%s headsign=%q departure=%d isRealtime=%v",
                    route.RouteShortName, itin.Headsign, sched.DepartureTime, sched.IsRealTime)
                break
            }
        }
    }

    if len(results) == 0 {
        busLog.Printf("Realtime fast-path produced 0 matching direct trips")
        return nil, errors.New("no direct realtime trips found")
    }

    sort.Slice(results, func(i, j int) bool {
        return results[i].DepartureUnix < results[j].DepartureUnix
    })
    busLog.Printf("Realtime fast-path produced %d matching direct trips", len(results))
    return results, nil
}

//
// -----------------------------------------------------------------------------
// Formatting helpers
// -----------------------------------------------------------------------------

func formatDirectRealtimeTrips(trips []PlannedDeparture) string {
    if len(trips) == 0 {
        return "Direct realtime trips:\n- None found.\n"
    }

    var b strings.Builder
    b.WriteString("Direct realtime trips:\n")
    for _, d := range trips {
        dep := time.Unix(d.DepartureUnix, 0)
        rtTag := "scheduled"
        if d.IsRealTime {
            rtTag = "real-time"
        }
        b.WriteString(fmt.Sprintf(
            "- Route %s (%s)\n  Headsign: %s\n  Departure at %s (%s)\n  Origin: %s\n  Destination: %s\n",
            d.RouteShortName, d.RouteLongName, nonEmpty(d.Headsign),
            dep.Format(time.RFC1123), rtTag,
            d.OriginName, d.DestName,
        ))
    }
    return b.String()
}

func formatStaticDirectTrips(trips []StaticResult, now time.Time) string {
    if len(trips) == 0 {
        return "Direct scheduled trips (static GTFS):\n- None found.\n"
    }

    var b strings.Builder
    b.WriteString("Direct scheduled trips (static GTFS):\n")
    for _, c := range trips {
        dep, _ := parseTodayTime(c.DepStr, now)
        arr, _ := parseTodayTime(c.ArrStr, now)
        b.WriteString(fmt.Sprintf(
            "- Trip %s: depart %s (%s) → arrive %s (%s)\n",
            c.TripID,
            c.DepStr, dep.Format("15:04"),
            c.ArrStr, arr.Format("15:04"),
        ))
    }
    return b.String()
}

func formatConnectionsSection(title string, conns []string) string {
    if len(conns) == 0 {
        return title + ":\n- None found.\n"
    }
    var b strings.Builder
    b.WriteString(title + ":\n")
    for _, c := range conns {
        b.WriteString(c)
    }
    return b.String()
}

// nonEmpty provides a placeholder when a string is blank.
func nonEmpty(s string) string {
    if strings.TrimSpace(s) == "" {
        return "(no headsign)"
    }
    return s
}

//
// -----------------------------------------------------------------------------
// MCP helpers
// -----------------------------------------------------------------------------

func okText(s string) *protocol.CallToolResult {
    return &protocol.CallToolResult{
        Content: []protocol.Content{
            &protocol.TextContent{
                Type: "text",
                Text: s,
            },
        },
    }
}

func errorText(msg string) *protocol.CallToolResult {
    return &protocol.CallToolResult{
        Content: []protocol.Content{
            &protocol.TextContent{
                Type: "text",
                Text: msg,
            },
        },
        IsError: true,
    }
}

//
// -----------------------------------------------------------------------------
// Handler
// -----------------------------------------------------------------------------

// handleTransitArrivalPlanner:
//   1) Fuzzy-resolves origin/destination in GTFS.
//   2) Resolves RTSFL global_stop_id via nearby_stops.
//   3) Finds direct realtime trips (fast-path, no trip_details).
//   4) Computes static direct trips.
//   5) Computes 1- and 2-transfer static options via major hubs.
//   6) Returns grouped output.
func handleTransitArrivalPlanner(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
    var r TransitArrivalPlanner
    if err := protocol.VerifyAndUnmarshal(req.RawArguments, &r); err != nil {
        return errorText("Invalid request"), nil
    }

    initBusFinderLogger()
    busLog.Printf("CallToolRequest: origin=%q dest=%q intent=%q", r.OriginStop, r.DestinationStop, r.Intent)

    if strings.TrimSpace(r.OriginStop) == "" || strings.TrimSpace(r.DestinationStop) == "" {
        return errorText("Origin and destination stop names are required."), nil
    }

    client := NewTransitClient()

    // 1) Fuzzy GTFS resolution.
    originGTFS, err := resolveStaticStopFuzzy(r.OriginStop)
    if err != nil {
        return errorText("Could not resolve origin in GTFS: " + err.Error()), nil
    }
    destGTFS, err := resolveStaticStopFuzzy(r.DestinationStop)
    if err != nil {
        return errorText("Could not resolve destination in GTFS: " + err.Error()), nil
    }

    busLog.Printf("GTFS origin resolved: input=%q → %q (lat=%.6f lon=%.6f)",
        r.OriginStop, originGTFS.StopName, originGTFS.Lat, originGTFS.Lon)
    busLog.Printf("GTFS destination resolved: input=%q → %q (lat=%.6f lon=%.6f)",
        r.DestinationStop, destGTFS.StopName, destGTFS.Lat, destGTFS.Lon)

    // 2) Use GTFS lat/lon → nearby_stops (RTSFL only) → GlobalStopID.
    originGSID, originAPIName, err := resolveStopByNearbyRTSFL(client, originGTFS.Lat, originGTFS.Lon)
    if err != nil {
        return errorText("Could not resolve origin via nearby_stops: " + err.Error()), nil
    }
    _, destAPIName, err := resolveStopByNearbyRTSFL(client, destGTFS.Lat, destGTFS.Lon)
    if err != nil {
        return errorText("Could not resolve destination via nearby_stops: " + err.Error()), nil
    }

    busLog.Printf("Nearby origin: GTFS=%q → API=%q (GSID=%s)", originGTFS.StopName, originAPIName, originGSID)
    busLog.Printf("Nearby destination: GTFS=%q → API=%q", destGTFS.StopName, destAPIName)

    // 3) Direct realtime trips (fast-path).
    var realtimeTrips []PlannedDeparture
    realtimeTrips, err = findDirectRealtimeTripsFast(client, originGSID, originAPIName, destAPIName)
    if err != nil {
        busLog.Printf("Realtime fast-path failed or no trips: %v", err)
    }

    // 4) Static direct + connections.
    now := time.Now()
    staticDirect := staticDirectTrips(*originGTFS, *destGTFS, now, 3)
    conns1 := staticConnections1Transfer(*originGTFS, *destGTFS, now, 2)
    conns2 := staticConnections2Transfer(*originGTFS, *destGTFS, now, 1)

    // 5) Build grouped output.
    var b strings.Builder

    // Direct realtime section.
    b.WriteString(formatDirectRealtimeTrips(realtimeTrips))
    b.WriteString("\n")

    // Direct static section.
    b.WriteString(formatStaticDirectTrips(staticDirect, now))
    b.WriteString("\n")

    // 1-transfer section.
    b.WriteString(formatConnectionsSection("1-transfer scheduled options via major hubs", conns1))
    b.WriteString("\n")

    // 2-transfer section.
    b.WriteString(formatConnectionsSection("2-transfer scheduled options via major hubs", conns2))

    return okText(b.String()), nil
}

