package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

type RestaurantsTool struct {
	Location   string  `json:"location"`
	Mode       string  `json:"mode"`
	OpenNow    *bool   `json:"openNow,omitempty"` // ✅ optional now
	PriceLevel *string `json:"priceLevel,omitempty"`
	Query      string  `json:"query"`
	Radius     int     `json:"radius"`
}

// Tool metadata
func (RestaurantsTool) Name() string {
	return "UFRestaurants"
}

func (RestaurantsTool) Description() string {
	return "Search for nearby restaurants around UF campus landmarks, enriched with travel distance and open/closed status."
}

func handleRestaurants(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var args RestaurantsTool
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &args); err != nil {
		return nil, err
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: "Error: GEMINI_API_KEY not set"},
			},
			IsError: true,
		}, nil
	}

	// ✅ Apply defaults if missing
	if args.OpenNow == nil {
		def := false
		args.OpenNow = &def
	}

	lat, lng := 29.6516, -82.3426 // default UF campus
	geoQuery := args.Location + ", University of Florida, Gainesville, FL"
	geoURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?address=%s&key=%s",
		url.QueryEscape(geoQuery), apiKey)

	if geoResp, err := http.Get(geoURL); err == nil {
		defer geoResp.Body.Close()
		var geoResult struct {
			Results []struct {
				Geometry struct {
					Location struct {
						Lat float64 `json:"lat"`
						Lng float64 `json:"lng"`
					} `json:"location"`
				} `json:"geometry"`
			} `json:"results"`
			Status string `json:"status"`
		}
		_ = json.NewDecoder(geoResp.Body).Decode(&geoResult)
		if geoResult.Status == "OK" && len(geoResult.Results) > 0 {
			lat = geoResult.Results[0].Geometry.Location.Lat
			lng = geoResult.Results[0].Geometry.Location.Lng
		}
	}

	// Step 2: Normalize query and decide endpoint
	var endpoint string
	var body map[string]interface{}

	coreQuery := strings.ToLower(args.Query)
	var includedType string
	if strings.Contains(coreQuery, "pizza") {
		includedType = "pizza_restaurant"
	} else if strings.Contains(coreQuery, "indian") {
		includedType = "indian_restaurant"
	} else if strings.Contains(coreQuery, "chinese") {
		includedType = "chinese_restaurant"
	} else if strings.Contains(coreQuery, "cafe") {
		includedType = "cafe"
	}

	if includedType != "" {
		endpoint = "https://places.googleapis.com/v1/places:searchNearby"
		body = map[string]interface{}{
			"locationRestriction": map[string]interface{}{
				"circle": map[string]interface{}{
					"center": map[string]float64{"latitude": lat, "longitude": lng},
					"radius": float64(args.Radius),
				},
			},
			"includedTypes": []string{includedType},
			"openNow":       *args.OpenNow, // ✅ include openNow if set
		}
	} else {
		endpoint = "https://places.googleapis.com/v1/places:searchText"
		body = map[string]interface{}{
			"textQuery": fmt.Sprintf("%s near University of Florida %s", args.Query, args.Location),
		}
	}

	bodyBytes, _ := json.Marshal(body)
	reqPlaces, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(bodyBytes))
	reqPlaces.Header.Set("Content-Type", "application/json")
	reqPlaces.Header.Set("X-Goog-Api-Key", apiKey)
	reqPlaces.Header.Set("X-Goog-FieldMask", "*")

	respPlaces, err := http.DefaultClient.Do(reqPlaces)
	if err != nil {
		return nil, err
	}
	defer respPlaces.Body.Close()

	rawBody, _ := io.ReadAll(respPlaces.Body)
	respPlaces.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	var placesResult struct {
		Places []struct {
			DisplayName struct {
				Text string `json:"text"`
			} `json:"displayName"`
			FormattedAddress string  `json:"formattedAddress"`
			PriceLevel       string  `json:"priceLevel"`
			Rating           float64 `json:"rating"`
			Location         struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"location"`
			BusinessStatus string `json:"businessStatus"`
		} `json:"places"`
	}
	_ = json.NewDecoder(respPlaces.Body).Decode(&placesResult)

	restaurants := []map[string]interface{}{}
	for _, p := range placesResult.Places {
		if args.PriceLevel != nil && *args.PriceLevel != "" {
			if !strings.EqualFold(p.PriceLevel, strings.ToUpper(*args.PriceLevel)) {
				continue
			}
		}

		restaurants = append(restaurants, map[string]interface{}{
			"name":       p.DisplayName.Text,
			"vicinity":   p.FormattedAddress,
			"priceLevel": p.PriceLevel,
			"rating":     p.Rating,
			"status":     p.BusinessStatus,
		})
	}

	outBytes, err := json.Marshal(restaurants)
	if err != nil {
		return nil, err
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{Type: "text", Text: string(outBytes)},
		},
		IsError: false,
	}, nil
}

// Tool registration
func GetRestaurantsTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := RestaurantsTool{}
	tool, err := protocol.NewTool(
		toolStruct.Name(),
		toolStruct.Description(),
		toolStruct,
	)
	if err != nil {
		log.Fatalf("Failed to create Restaurants tool: %v", err)
	}
	return tool, handleRestaurants
}
