package schedule_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"uf/mcp/pkg/llm"

	"github.com/tmc/langchaingo/llms/openai"
)

const maxTokens = 2048

var (
	llmURL   = os.Getenv("LLM_URL")
	llmToken = os.Getenv("LLM_TOKEN")
)

var scheduleToolPrompt = `
You are a tool selector for the Schedule Agent.
Available tools:
- UFClassSchedule
- ProfessorRating

Arguments:
- UFClassSchedule: {"term":"<term code>","category":"<RES|UGRD|GRAD>","instructor_last":"<last name>","course_code":"<course prefix+number>","max_sections_output":<int>,"page":<int>}
- ProfessorRating: {"name":"<professor name>"}

Examples:
User: "When is COP3502 offered in Spring 2026?"
→ {"tool_name":"UFClassSchedule","tool_args":{"term":"2261","category":"UGRD","course_code":"COP3502"}}

User: "Rate William Anderson"
→ {"tool_name":"ProfessorRating","tool_args":{"name":"William Anderson"}}

User: "Show me graduate courses taught by Smith in Fall 2025"
→ {"tool_name":"UFClassSchedule","tool_args":{"term":"2261","category":"GRAD","instructor_last":"Smith"}}

User: "List all RES category classes for term 2261"
→ {"tool_name":"UFClassSchedule","tool_args":{"term":"2261","category":"RES","last-updated":"1"}}

Return ONLY valid JSON: {"tool_name":"<tool>","tool_args":{...}}
Do not include backticks, code fences, or any extra text.
The first character MUST be '{' and the last character MUST be '}'.
`

func SelectScheduleTool(ctx context.Context, _ *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
	messages := []map[string]string{
		{"role": "system", "content": scheduleToolPrompt},
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
		return nil, fmt.Errorf("schedule: request failed: %w", err)
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

	// Log the raw LLM output
	log.Printf("LLM raw output: %q", content)

	// Clean up any stray markdown fences or backticks
	clean := SanitizeLLMOutput(content)

	// Log the sanitized output
	log.Printf("LLM sanitized output: %q", clean)

	return llm.BuildToolSelectionResponse(clean)
}

// SanitizeLLMOutput removes markdown fences/backticks so JSON can be parsed cleanly.
func SanitizeLLMOutput(content string) string {
	clean := strings.TrimSpace(content)

	// Regex to capture JSON inside ```...``` fences (with or without "json")
	reFence := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
	matches := reFence.FindStringSubmatch(clean)
	if len(matches) > 1 {
		clean = matches[1]
	}

	// Remove stray leading/trailing backticks or quotes
	clean = strings.Trim(clean, "`")
	clean = strings.Trim(clean, "\"")
	clean = strings.TrimSpace(clean)

	return clean
}
