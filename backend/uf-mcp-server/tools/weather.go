package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

// Tool arguments: optional fields
type WeatherTool struct {
	Location string `json:"location,omitempty"`
	Forecast bool   `json:"forecast,omitempty"`
}

// Tool metadata
func (WeatherTool) Name() string {
	return "UFWeather"
}

func (WeatherTool) Description() string {
	return "Get current weather or forecast for a location (defaults to Gainesville, FL, US) using OpenWeatherMap."
}

// Resolve coordinates from a location string using OpenWeatherMap Geocoding API
func resolveCoordinates(location string, apiKey string) (float64, float64, error) {
	endpoint := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s",
		url.QueryEscape(location), apiKey)

	resp, err := http.Get(endpoint)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var results []struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, err
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no coordinates found for %s", location)
	}
	return results[0].Lat, results[0].Lon, nil
}

// Handler
func handleWeather(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var args WeatherTool
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &args); err != nil {
		return nil, err
	}

	// ✅ Apply defaults if missing
	if args.Location == "" {
		args.Location = "Gainesville,FL,US" // default city for UF campus
	}
	// Forecast defaults to false (current weather)

	apiKey := os.Getenv("OPENWEATHERMAP_KEY")
	if apiKey == "" {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: "Error: OPENWEATHERMAP_KEY not set"},
			},
			IsError: true,
		}, nil
	}

	// Resolve coordinates dynamically
	lat, lon, err := resolveCoordinates(args.Location, apiKey)
	if err != nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error resolving location: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Build endpoint
	var endpoint string
	if args.Forecast {
		endpoint = fmt.Sprintf("https://api.openweathermap.org/data/2.5/forecast?lat=%f&lon=%f&units=imperial&appid=%s", lat, lon, apiKey)
	} else {
		endpoint = fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&units=imperial&appid=%s", lat, lon, apiKey)
	}

	// Fetch data
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Weather API error: %s", string(rawBody))},
			},
			IsError: true,
		}, nil
	}

	// Parse response
	if args.Forecast {
		// Forecast parsing (limit to 5 entries)
		var forecast struct {
			List []struct {
				Dt_txt string `json:"dt_txt"`
				Main   struct {
					Temp     float64 `json:"temp"`
					Humidity int     `json:"humidity"`
				} `json:"main"`
				Weather []struct {
					Description string `json:"description"`
				} `json:"weather"`
			} `json:"list"`
		}
		if err := json.Unmarshal(rawBody, &forecast); err != nil {
			return nil, err
		}

		results := []map[string]interface{}{}
		for i, f := range forecast.List {
			if i >= 5 {
				break
			}
			results = append(results, map[string]interface{}{
				"time":        f.Dt_txt,
				"temperature": fmt.Sprintf("%.1f °F", f.Main.Temp),
				"humidity":    fmt.Sprintf("%d%%", f.Main.Humidity),
				"condition":   f.Weather[0].Description,
			})
		}

		outBytes, _ := json.Marshal(results)
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: string(outBytes)},
			},
			IsError: false,
		}, nil
	} else {
		// Current weather parsing
		var current struct {
			Main struct {
				Temp     float64 `json:"temp"`
				Humidity int     `json:"humidity"`
			} `json:"main"`
			Weather []struct {
				Description string `json:"description"`
			} `json:"weather"`
		}
		if err := json.Unmarshal(rawBody, &current); err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"temperature": fmt.Sprintf("%.1f °F", current.Main.Temp),
			"humidity":    fmt.Sprintf("%d%%", current.Main.Humidity),
			"condition":   current.Weather[0].Description,
		}

		outBytes, _ := json.Marshal(result)
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: string(outBytes)},
			},
			IsError: false,
		}, nil
	}
}

// Tool registration
func GetWeatherTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := WeatherTool{}
	tool, err := protocol.NewTool(
		toolStruct.Name(),
		toolStruct.Description(),
		toolStruct,
	)
	if err != nil {
		log.Fatalf("Failed to create Weather tool: %v", err)
	}
	return tool, handleWeather
}
