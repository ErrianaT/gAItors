package safety_agent

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

var safetyToolPrompt = `
You are a tool selector for the Safety Agent.
Available tools:
- CampusSafetyLocator
- Directions

Arguments:
- CampusSafetyLocator: {"location":"<UF landmark>","type":"<Blue Phone|SNAP|Parking>","diagnostic":false,"travel_mode":"walking|driving|bicycling|transit"}
- Directions: {"origin":"<place>","destination":"<place>","mode":"walking|driving|bicycling|transit"}

Examples:
User: "Find the nearest Blue Phone to Reitz Union"
→ {"tool_name":"CampusSafetyLocator","tool_args":{"location":"Reitz Union","type":"Blue Phone","diagnostic":false,"travel_mode":"walking"}}

User: "Walking directions from Reitz Union to Library West"
→ {"tool_name":"Directions","tool_args":{"origin":"Reitz Union","destination":"Library West","mode":"walking"}}

Return ONLY valid JSON: {"tool_name":"<tool>","tool_args":{...}}
Do not include backticks, code fences, or any extra text.
`

func SelectSafetyTool(ctx context.Context, _ *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
	messages := []map[string]string{
		{"role": "system", "content": safetyToolPrompt},
		{"role": "user", "content": query},
	}
	payload := map[string]interface{}{"model": "gpt-4o", "messages": messages, "max_tokens": maxTokens, "stream": false}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", llmURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llmToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("safety: request failed: %w", err)
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
