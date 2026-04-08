package transit_agent

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

var transitToolPrompt = `
You are a tool selector for the Transit Agent.
Available tools:
- TransitArrivalPlanner

Arguments:
- TransitArrivalPlanner: {"origin_stop":"<stop>","destination_stop":"<stop>","intent":"NEXT_BUS|LIST_BUSES|ALERTS"}

Examples:
User: "Next bus from Reitz Union to Library West"
→ {"tool_name":"TransitArrivalPlanner","tool_args":{"origin_stop":"Reitz Union","destination_stop":"Library West","intent":"NEXT_BUS"}}

User: "List buses at Reitz Union"
→ {"tool_name":"TransitArrivalPlanner","tool_args":{"origin_stop":"Reitz Union","intent":"LIST_BUSES"}}

Return ONLY valid JSON: {"tool_name":"TransitArrivalPlanner","tool_args":{...}}
Do not include backticks, code fences, or any extra text.
`

func SelectTransitTool(ctx context.Context, _ *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
	messages := []map[string]string{
		{"role": "system", "content": transitToolPrompt},
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
		return nil, fmt.Errorf("transit: request failed: %w", err)
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
