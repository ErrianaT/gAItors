package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

type CampusSafetyLocator struct {
	Location   string `json:"location" description:"Campus landmark to search near (e.g. Library West)" required:"true"`
	Type       string `json:"type" description:"Layer to query: Blue Phone, SNAP, or Parking" required:"true"`
	Diagnostic bool   `json:"diagnostic" description:"If true, list all nearby points within 500 feet instead of only the nearest" required:"false"`
	TravelMode string `json:"travel_mode" description:"Travel mode for directions: walking, driving, bicycling, transit" required:"false"`
}

func (t *CampusSafetyLocator) Name() string { return "CampusSafetyLocator" }
func (t *CampusSafetyLocator) Description() string {
	return "Find the nearest Blue Phone, SNAP stop, or Parking zone to a campus landmark, with ArcGIS routing directions."
}

type SafetyPoint struct {
	Name        string
	Type        string
	Description string
	Latitude    float64
	Longitude   float64
}

// Fetch points from UF JSON feeds, ArcGIS feeds, or fallback to Places API
func fetchSafetyPoints(layer string) ([]SafetyPoint, error) {
	var url string
	switch strings.ToLower(layer) {
	case "blue phone":
		url = "https://campusmap.ufl.edu/assets/blue_phones.json"
	case "snap":
		url = "https://campusmap.ufl.edu/assets/snap_stops.json"
	case "parking":
		// Attempt UF ArcGIS Parking zones first (may fail if only VectorTileServer is available)
		url = "https://services.arcgis.com/IiuFUnlkob76Az9k/arcgis/rest/services/TAPS_Enforcement_Zones_Tile_Update/FeatureServer/0/query?where=1=1&outFields=*&returnGeometry=true&outSR=4326&f=json"
	case "building":
		url = "https://campusmap.ufl.edu/assets/buildings.json"
	default:
		return nil, fmt.Errorf("Unknown layer type: %s", layer)
	}

	// Try UF feed first
	resp, err := http.Get(url)
	if err == nil {
		defer resp.Body.Close()

		var geo struct {
			Features []struct {
				Properties map[string]interface{} `json:"properties"`
				Geometry   struct {
					Type        string      `json:"type"`
					Coordinates interface{} `json:"coordinates"`
				} `json:"geometry"`
			} `json:"features"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&geo); err == nil && len(geo.Features) > 0 {
			points := []SafetyPoint{}
			for _, f := range geo.Features {
				switch f.Geometry.Type {
				case "Point":
					if arr, ok := f.Geometry.Coordinates.([]interface{}); ok && len(arr) == 2 {
						lon, _ := arr[0].(float64)
						lat, _ := arr[1].(float64)
						points = append(points, SafetyPoint{
							Name:        fmt.Sprintf("%v", f.Properties["NAME"]),
							Type:        layer,
							Description: fmt.Sprintf("%v", f.Properties["DESCRIPTION"]),
							Latitude:    lat,
							Longitude:   lon,
						})
					}
				case "Polygon":
					if rings, ok := f.Geometry.Coordinates.([][][]float64); ok {
						var sumX, sumY float64
						var count int
						for _, ring := range rings {
							for _, pt := range ring {
								sumX += pt[0]
								sumY += pt[1]
								count++
							}
						}
						if count > 0 {
							cx := sumX / float64(count)
							cy := sumY / float64(count)
							points = append(points, SafetyPoint{
								Name:        fmt.Sprintf("%v", f.Properties["NAME"]),
								Type:        layer,
								Description: fmt.Sprintf("%v", f.Properties["DESCRIPTION"]),
								Latitude:    cy,
								Longitude:   cx,
							})
						}
					}
				}
			}
			if len(points) > 0 {
				log.Printf("[CampusSafetyLocator] Parking results from UF ArcGIS feed: %d features", len(points))
				return points, nil
			}
			log.Printf("[CampusSafetyLocator] UF ArcGIS feed returned 0 features for %s", layer)
		} else {
			log.Printf("[CampusSafetyLocator] Failed to decode UF feed for %s: %v", layer, err)
		}
	} else {
		log.Printf("[CampusSafetyLocator] Error fetching UF feed for %s: %v", layer, err)
	}

	// 🚨 Fallback for Parking: use ArcGIS Places API
	if strings.ToLower(layer) == "parking" {
		ufLat, ufLon := 29.6516, -82.3410 // UF campus coordinates (Reitz Union area)
		placesURL := fmt.Sprintf(
			"https://places-api.arcgis.com/v1/places/nearby?categories=parking&location=%f,%f&radius=2000&f=json&token=%s",
			ufLat, ufLon, os.Getenv("ARCGIS_API_KEY"),
		)

		resp, err := http.Get(placesURL)
		if err != nil {
			return nil, fmt.Errorf("Places API request failed: %v", err)
		}
		defer resp.Body.Close()

		var places struct {
			Results []struct {
				Name     string `json:"name"`
				Category string `json:"category"`
				Location struct {
					Lat float64 `json:"lat"`
					Lon float64 `json:"lon"`
				} `json:"location"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&places); err != nil {
			return nil, fmt.Errorf("Places API decode failed: %v", err)
		}

		points := []SafetyPoint{}
		for _, r := range places.Results {
			points = append(points, SafetyPoint{
				Name:        r.Name,
				Type:        "parking",
				Description: r.Category,
				Latitude:    r.Location.Lat,
				Longitude:   r.Location.Lon,
			})
		}
		if len(points) > 0 {
			log.Printf("[CampusSafetyLocator] Parking results from ArcGIS Places API: %d features", len(points))
			return points, nil
		}
		log.Printf("[CampusSafetyLocator] Places API returned 0 features for Parking near Reitz Union")
		return nil, fmt.Errorf("No valid parking locations found near Reitz Union")
	}

	return nil, fmt.Errorf("No valid features decoded for layer %s", layer)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371e3
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

var (
	safetyTokenCache     string
	safetyTokenExpiresAt time.Time
	safetyTokenLock      sync.Mutex
)

func fetchSafetyArcGISToken() (string, error) {
	safetyTokenLock.Lock()
	defer safetyTokenLock.Unlock()

	if safetyTokenCache != "" && time.Now().Before(safetyTokenExpiresAt) {
		return safetyTokenCache, nil
	}

	clientID := os.Getenv("ARCGIS_CLIENT_ID")
	clientSecret := os.Getenv("ARCGIS_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing ARCGIS_CLIENT_ID or ARCGIS_CLIENT_SECRET")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "client_credentials")
	form.Set("f", "pjson")

	resp, err := http.PostForm("https://www.arcgis.com/sharing/rest/oauth2/token", form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Token     string `json:"access_token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	safetyTokenCache = result.Token
	safetyTokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return safetyTokenCache, nil
}

func resolveSafetyCoords(place, token string) (float64, float64, error) {
	params := url.Values{}
	params.Set("SingleLine", place)
	params.Set("f", "json")
	params.Set("outSR", "4326")
	params.Set("token", token)

	u := "https://geocode.arcgis.com/arcgis/rest/services/World/GeocodeServer/findAddressCandidates?" + params.Encode()
	resp, err := http.Get(u)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var r struct {
		Candidates []struct {
			Location struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"location"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, 0, err
	}
	if len(r.Candidates) == 0 {
		return 0, 0, fmt.Errorf("no geocode result for %s", place)
	}
	return r.Candidates[0].Location.X, r.Candidates[0].Location.Y, nil
}

// Resolve ArcGIS travel mode ID from user-friendly string
func resolveSafetyTravelMode(userMode, token string) (string, error) {
	url := "https://route.arcgis.com/arcgis/rest/services/World/Route/NAServer/Route_World/travelModes?f=pjson&token=" + token
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		SupportedTravelModes []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"supportedTravelModes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	userMode = strings.ToLower(userMode)
	for _, m := range result.SupportedTravelModes {
		if strings.Contains(strings.ToLower(m.Name), userMode) {
			return m.ID, nil
		}
	}
	// fallback to first mode
	return result.SupportedTravelModes[0].ID, nil
}

// Utility to map a generic JSON object into a struct
func mapToSafetyStruct(m map[string]interface{}, out interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// Enrich direction text with nearest building name
func enrichDirection(text string, lat, lon float64) string {
	buildings, err := fetchSafetyPoints("building")
	if err != nil {
		return text
	}
	nearest := ""
	minDist := 100.0
	for _, b := range buildings {
		if b.Latitude == 0 || b.Longitude == 0 {
			continue
		}
		d := haversine(lat, lon, b.Latitude, b.Longitude)
		if d < minDist {
			minDist = d
			nearest = b.Name
		}
	}
	if nearest != "" {
		return fmt.Sprintf("%s near %s", text, nearest)
	}
	return text
}

// ---------------- ArcGIS Route Call ----------------
func callSafetyArcGISRoute(orig string, destLat, destLon float64, mode string, destType string, destName string) (string, error) {
	token, err := fetchSafetyArcGISToken()
	if err != nil {
		return "", err
	}

	ox, oy, err := resolveSafetyCoords(orig, token)
	if err != nil {
		return "", fmt.Errorf("origin geocode failed: %v", err)
	}

	// IMPORTANT: ArcGIS expects x=longitude, y=latitude
	stops := fmt.Sprintf(
		`{"features":[{"geometry":{"x":%.6f,"y":%.6f}},{"geometry":{"x":%.6f,"y":%.6f}}]}`,
		ox, oy, destLon, destLat,
	)

	modeID, err := resolveSafetyTravelMode(mode, token)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("f", "json")
	params.Set("token", token)
	params.Set("stops", stops)
	params.Set("outSR", "4326")
	params.Set("returnDirections", "true")
	params.Set("directionsLanguage", "en")
	params.Set("travelMode", modeID)

	routeURL := "https://route.arcgis.com/arcgis/rest/services/World/Route/NAServer/Route_World/solve?" + params.Encode()
	resp, err := http.Get(routeURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", err
	}

	var r struct {
		Routes struct {
			Features []struct {
				Attributes struct {
					Total_Miles   float64 `json:"Total_Miles"`
					Total_Minutes float64 `json:"Total_Minutes"`
				} `json:"attributes"`
			} `json:"features"`
		} `json:"routes"`
		Directions []struct {
			Features []struct {
				Attributes struct {
					Text   string  `json:"text"`
					Length float64 `json:"length"`
					Time   float64 `json:"time"`
				} `json:"attributes"`
			} `json:"features"`
		} `json:"directions"`
		Error *struct {
			Message string   `json:"message"`
			Details []string `json:"details"`
		} `json:"error"`
	}
	if err := mapToSafetyStruct(raw, &r); err != nil {
		return "", err
	}

	if r.Error != nil {
		return "", fmt.Errorf("arcgis error: %s (%v)", r.Error.Message, r.Error.Details)
	}
	if len(r.Routes.Features) == 0 {
		return "", fmt.Errorf("no route found")
	}

	attrs := r.Routes.Features[0].Attributes
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "From %s to destination:\n", orig)
	fmt.Fprintf(sb, "Distance: %.2f km\n", attrs.Total_Miles*1.60934)
	fmt.Fprintf(sb, "Estimated time: %.0f minutes\n", attrs.Total_Minutes)
	fmt.Fprintf(sb, "\nSteps:\n")

	if len(r.Directions) > 0 {
		for i, f := range r.Directions[0].Features {
			meters := f.Attributes.Length * 1609.34
			enriched := enrichDirection(f.Attributes.Text, oy, ox)

			if i == len(r.Directions[0].Features)-1 {
				enriched = fmt.Sprintf("Arrive at %s: %s", destType, destName)
			}

			fmt.Fprintf(sb, "%d. %s (%.0f m, %.0f min)\n",
				i+1, enriched, meters, f.Attributes.Time)
		}
	}

	return sb.String(), nil
}

// ---------------- Tool Handler ----------------

func handleCampusSafetyLocator(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var request CampusSafetyLocator
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &request); err != nil {
		return nil, err
	}

	points, err := fetchSafetyPoints(request.Type)
	if err != nil {
		return nil, err
	}

	token, err := fetchSafetyArcGISToken()
	if err != nil {
		return nil, err
	}
	ox, oy, err := resolveSafetyCoords(request.Location, token)
	if err != nil {
		return nil, fmt.Errorf("origin geocode failed: %v", err)
	}

	originLat := oy
	originLon := ox
	log.Printf("[CampusSafetyLocator] Origin resolved: lat=%.6f lon=%.6f", originLat, originLon)

	if request.Diagnostic {
		var results []string
		for _, p := range points {
			if p.Latitude == 0 || p.Longitude == 0 {
				continue
			}
			dist := haversine(originLat, originLon, p.Latitude, p.Longitude)
			if dist <= 152.4 {
				results = append(results, fmt.Sprintf("%s (%.0f feet)", p.Name, dist*3.28084))
			}
		}
		if len(results) == 0 {
			return &protocol.CallToolResult{
				Content: []protocol.Content{
					&protocol.TextContent{Type: "text", Text: fmt.Sprintf(
						"Unable to provide direction as UF JSON feeds (%s) don’t include usable coordinates for nearby points.",
						request.Type)},
				},
				IsError: false,
			}, nil
		}
		resultText := fmt.Sprintf("Nearby %s locations around %s:\n%s",
			request.Type, request.Location, strings.Join(results, "\n"))
		return &protocol.CallToolResult{
			Content: []protocol.Content{&protocol.TextContent{Type: "text", Text: resultText}},
			IsError: false,
		}, nil
	}

	var closest *SafetyPoint
	minDist := math.MaxFloat64
	for _, p := range points {
		if p.Latitude == 0 || p.Longitude == 0 {
			continue
		}
		dist := haversine(originLat, originLon, p.Latitude, p.Longitude)
		if dist < minDist {
			minDist = dist
			closest = &p
		}
	}
	if closest == nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf(
					"No valid %s locations found near %s", request.Type, request.Location)},
			},
			IsError: false,
		}, nil
	}

	directions, err := callSafetyArcGISRoute(
		request.Location,
		closest.Latitude,
		closest.Longitude,
		request.TravelMode,
		request.Type,
		closest.Name,
	)
	if err != nil {
		return nil, err
	}

	resultText := fmt.Sprintf("Nearest %s to %s:\n%s\nDistance: %.0f feet\nDirections:\n%s",
		request.Type, request.Location, closest.Name, minDist*3.28084, directions)

	return &protocol.CallToolResult{
		Content: []protocol.Content{&protocol.TextContent{Type: "text", Text: resultText}},
		IsError: false,
	}, nil
}

func GetCampusSafetyLocatorTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := CampusSafetyLocator{}
	tool, err := protocol.NewTool(
		toolStruct.Name(),
		toolStruct.Description(),
		toolStruct,
	)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}
	return tool, handleCampusSafetyLocator
}
