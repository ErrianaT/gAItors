package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

// Gazetteer fallback for UF landmarks (lon, lat)
var ufGazetteer = map[string][2]float64{
	"reitz union":          {-82.34804, 29.64679},
	"reitz welcome center": {-82.34804, 29.64679},
	"library west":         {-82.34262, 29.65115},
	"main library":         {-82.34262, 29.65115},
	"uf auditorium":        {-82.34380, 29.64952},
	"auditorium":           {-82.34380, 29.64952},
}

// Tool schema
type DirectionsTool struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
}

func (t *DirectionsTool) Name() string { return "Directions" }
func (t *DirectionsTool) Description() string {
	return "Get campus directions using ArcGIS Route with gazetteer fallback and diagnostics."
}

// OAuth token manager
var (
	tokenCache     string
	tokenExpiresAt time.Time
	tokenLock      sync.Mutex
)

func fetchArcGISToken() (string, error) {
	tokenLock.Lock()
	defer tokenLock.Unlock()

	if tokenCache != "" && time.Now().Before(tokenExpiresAt) {
		return tokenCache, nil
	}

	clientID := os.Getenv("ARCGIS_CLIENT_ID")
	clientSecret := os.Getenv("ARCGIS_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing ARCGIS_CLIENT_ID or ARCGIS_CLIENT_SECRET env vars")
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
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("ArcGIS token error: %s", result.Error.Message)
	}

	tokenCache = result.Token
	tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return tokenCache, nil
}

// Resolve coordinates: gazetteer first, then ArcGIS geocoder
func resolveCoords(place, token string) (float64, float64, string, error) {
	key := strings.ToLower(place)
	if coords, ok := ufGazetteer[key]; ok {
		return coords[0], coords[1], "gazetteer", nil
	}

	params := url.Values{}
	params.Set("SingleLine", place)
	params.Set("f", "json")
	params.Set("outSR", "4326")
	params.Set("token", token)

	u := "https://geocode.arcgis.com/arcgis/rest/services/World/GeocodeServer/findAddressCandidates?" + params.Encode()
	resp, err := http.Get(u)
	if err != nil {
		return 0, 0, "geocoder", err
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
		return 0, 0, "geocoder", err
	}
	if len(r.Candidates) == 0 {
		return 0, 0, "geocoder", fmt.Errorf("no geocode result for %s", place)
	}
	return r.Candidates[0].Location.X, r.Candidates[0].Location.Y, "geocoder", nil
}

// Travel mode resolver
var (
	travelModesCache map[string]string
	travelModesLock  sync.Mutex
)

func fetchTravelModes(token string) (map[string]string, error) {
	travelModesLock.Lock()
	defer travelModesLock.Unlock()

	if travelModesCache != nil {
		return travelModesCache, nil
	}

	url := "https://route.arcgis.com/arcgis/rest/services/World/Route/NAServer/Route_World/travelModes?f=pjson&token=" + token
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		SupportedTravelModes []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"supportedTravelModes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	travelModesCache = make(map[string]string)
	fmt.Println("Available travel modes:")
	for _, m := range result.SupportedTravelModes {
		travelModesCache[strings.ToLower(m.Name)] = m.ID
		//fmt.Printf("- %s (ID: %s)\n", m.Name, m.ID)
	}
	return travelModesCache, nil
}

func resolveTravelMode(userMode, token string) (string, error) {
	modes, err := fetchTravelModes(token)
	if err != nil {
		return "", err
	}

	userMode = strings.ToLower(userMode)
	if userMode == "" {
		userMode = "walking"
	}

	// Try walking first
	for name, id := range modes {
		if strings.Contains(name, "walk") || strings.Contains(name, "pedestrian") {
			if userMode == "walking" || userMode == "walk" || userMode == "foot" {
				fmt.Printf("Selected travel mode: %s (ID: %s)\n", name, id)
				return id, nil
			}
		}
	}

	// Try driving
	for name, id := range modes {
		if strings.Contains(name, "driving") {
			fmt.Printf("Selected travel mode: %s (ID: %s)\n", name, id)
			return id, nil
		}
	}

	// Fallback: first available mode
	for name, id := range modes {
		fmt.Printf("Fallback travel mode: %s (ID: %s)\n", name, id)
		return id, nil
	}
	return "", fmt.Errorf("no travel modes available")
}

// Helper to convert raw map to struct
func mapToStruct(m map[string]interface{}, out interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// Route call with diagnostics
func callArcGISRoute(orig, dest, mode string) (string, error) {
	token, err := fetchArcGISToken()
	if err != nil {
		return "", err
	}

	ox, oy, oSrc, err := resolveCoords(orig, token)
	if err != nil {
		return "", fmt.Errorf("origin geocode failed: %v", err)
	}
	dx, dy, dSrc, err := resolveCoords(dest, token)
	if err != nil {
		return "", fmt.Errorf("destination geocode failed: %v", err)
	}

	fmt.Printf("Origin: %s → (%.6f, %.6f) [%s]\n", orig, ox, oy, oSrc)
	fmt.Printf("Destination: %s → (%.6f, %.6f) [%s]\n", dest, dx, dy, dSrc)

	stops := fmt.Sprintf(`{"features":[{"geometry":{"x":%.6f,"y":%.6f}},{"geometry":{"x":%.6f,"y":%.6f}}]}`, ox, oy, dx, dy)

	modeID, err := resolveTravelMode(mode, token)
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
	fmt.Println("Routing URL:", routeURL)

	resp, err := http.Get(routeURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", err
	}
	//rawJSON, _ := json.MarshalIndent(raw, "", "  ")
	//fmt.Println("Raw ArcGIS response:\n", string(rawJSON))

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
	if err := mapToStruct(raw, &r); err != nil {
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
	fmt.Fprintf(sb, "From %s to %s:\n", orig, dest)
	fmt.Fprintf(sb, "Distance: %.2f km\n", attrs.Total_Miles*1.60934)
	fmt.Fprintf(sb, "Estimated time: %.0f minutes\n", attrs.Total_Minutes)
	fmt.Fprintf(sb, "\nSteps:\n")
	if len(r.Directions) > 0 {
		for i, f := range r.Directions[0].Features {
			meters := f.Attributes.Length * 1609.34
			fmt.Fprintf(sb, "%d. %s (%.0f m, %.0f min)\n",
				i+1, f.Attributes.Text, meters, f.Attributes.Time)
		}
	}
	return sb.String(), nil
}

// MCP handler
func handleDirections(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var args DirectionsTool
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &args); err != nil {
		return nil, err
	}

	result, err := callArcGISRoute(args.Origin, args.Destination, args.Mode)
	if err != nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{Type: "text", Text: result},
		},
		IsError: false,
	}, nil
}

// Tool registration
func GetDirectionsTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := DirectionsTool{}
	tool, err := protocol.NewTool(
		toolStruct.Name(),
		toolStruct.Description(),
		toolStruct,
	)
	if err != nil {
		log.Fatalf("Failed to create Directions tool: %v", err)
	}
	return tool, handleDirections
}
