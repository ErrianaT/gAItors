package restaurants_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"uf/mcp/pkg/llm"

	"github.com/tmc/langchaingo/llms/openai"
)

const maxTokens = 2048

var (
	llmURL   = os.Getenv("LLM_URL")
	llmToken = os.Getenv("LLM_TOKEN")
)

var restaurantsToolPrompt = `
You are a tool selector for the Restaurants Agent.
Available tools:
- UFRestaurants

Arguments:
- UFRestaurants: {"location":"<UF landmark>","mode":"<WALKING|DRIVING|BICYCLING|TRANSIT>","openNow":true|false,"priceLevel":"<LOW|MEDIUM|HIGH>","query":"<food type>","radius":<int>}

Examples:
User: "Find pizza near Reitz Union"
→ {"tool_name":"UFRestaurants","tool_args":{"location":"Reitz Union","mode":"WALKING","query":"pizza","radius":1000}}

User: "Coffee shops open now near Library West"
→ {"tool_name":"UFRestaurants","tool_args":{"location":"Library West","mode":"WALKING","openNow":true,"query":"coffee","radius":800}}

Return ONLY valid JSON: {"tool_name":"UFRestaurants","tool_args":{...}}
Do not include backticks, code fences, or any extra text.
`

func SelectRestaurantsTool(ctx context.Context, _ *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
	messages := []map[string]string{
		{"role": "system", "content": restaurantsToolPrompt},
		{"role": "user", "content": query},
	}

	payload := map[string]interface{}{
		"model":      "gpt-4o",
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     false,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", llmURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llmToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("restaurants: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, err
	}

	choices := raw["choices"].([]interface{})
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	content := message["content"].(string)
	return llm.BuildToolSelectionResponse(content)
}
