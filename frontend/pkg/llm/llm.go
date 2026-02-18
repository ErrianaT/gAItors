package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/llms/openai"
)

var (
    llmUrl   = os.Getenv("LLM_URL")
    llmToken = os.Getenv("LLM_TOKEN")
)

const maxTokens = 2048

// Shared struct: all agents return this type
type SelectedToolInfo struct {
    ToolName    string         `json:"tool_name"`
    ToolArgs    map[string]any `json:"tool_args"`
    MissingArgs []string       `json:"missing_args,omitempty"`
}

// Agent type for registry
type Agent struct {
    Name       string
    SelectTool func(ctx context.Context, model *openai.LLM, query string) (*SelectedToolInfo, error)
}

// AgentSelectionResult is the JSON returned by the LLM when choosing an agent
type AgentSelectionResult struct {
    AgentName string `json:"agent_name"`
}

var agentSelectionPrompt = `
You are an agent selector.
Decide which agent should handle the query.

Agents and their tools:
- Safety Agent: tools include Blue Phone, SNAP, Parking, Directions
- Schedule Agent: tools include UFClassSchedule, ProfessorRating
- Weather Agent: tools include UFWeather, GymCam
- Restaurants Agent: tools include UFRestaurants
- Transit Agent: tools include TransitArrivalPlanner
- RAG Agent: tools include file_search_store_create, file_search_store_list, file_search_store_delete, file_search_store_get, file_search_store_upload_media, file_search_store_import_file, operation_get, document_delete, document_get, document_list, generate_content

============================================================
DEFAULT RULE
============================================================
If the query does not clearly match any specialized agent,
select the Chat Agent.

============================================================
OUTPUT FORMAT
============================================================
Return ONLY valid JSON:
{"agent_name": "<agent>"}

Do not include backticks, code fences, or any extra text.
The first character MUST be '{' and the last character MUST be '}'.
`

// SelectAgent chooses which agent should handle the query
func SelectAgent(ctx context.Context, llmClient *openai.LLM, query string) (string, error) {
    messages := []map[string]string{
        {"role": "system", "content": agentSelectionPrompt},
        {"role": "user", "content": query},
    }

    payload := map[string]interface{}{
        "model":      "gpt-4o",
        "messages":   messages,
        "max_tokens": maxTokens,
        "stream":     false,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return "", fmt.Errorf("failed to marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", llmUrl, strings.NewReader(string(body)))
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llmToken))

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response: %w", err)
    }

    var raw map[string]interface{}
    if err := json.Unmarshal(respBody, &raw); err != nil {
        return "", fmt.Errorf("invalid JSON response: %w", err)
    }

    choices, ok := raw["choices"].([]interface{})
    if !ok || len(choices) == 0 {
        return "", fmt.Errorf("no choices returned")
    }

    message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
    content := message["content"].(string)

    fmt.Printf("Agent selection raw content: %q\n", content)

    clean := sanitizeLLMOutput(content)

    fmt.Printf("Agent selection sanitized content: %q\n", clean)

    var result AgentSelectionResult
    if err := json.Unmarshal([]byte(clean), &result); err != nil {
        return "", fmt.Errorf("failed to parse agent selection: %w", err)
    }

    return result.AgentName, nil
}

// BuildToolSelectionResponse parses tool selection JSON
func BuildToolSelectionResponse(llmResp string) (*SelectedToolInfo, error) {
    jsonDoc, found := extractJson(llmResp)
    if !found {
        return nil, fmt.Errorf("response is not JSON document.\nOutput from llm:\n%s", llmResp)
    }

    toolInfo := SelectedToolInfo{}
    if err := json.Unmarshal([]byte(jsonDoc), &toolInfo); err != nil {
        return nil, fmt.Errorf("unable to parse tool selection.\nOutput from llm:\n%s", llmResp)
    }

    return &toolInfo, nil
}

// GenericResponse: used when Chat Agent is selected or tool_name == "none"
func GenericResponse(ctx context.Context, llm *openai.LLM, query string) (string, error) {
    messages := []map[string]string{
        {
            "role":    "user",
            "content": fmt.Sprintf("Query: %s", query),
        },
    }

    payload := map[string]interface{}{
        "model":      "gpt-4o",
        "messages":   messages,
        "max_tokens": maxTokens,
        "stream":     false,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return "", fmt.Errorf("failed to marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", llmUrl, strings.NewReader(string(body)))
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llmToken))

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response: %w", err)
    }

    var raw map[string]interface{}
    if err := json.Unmarshal(respBody, &raw); err != nil {
        return "", fmt.Errorf("invalid JSON response: %w", err)
    }

    choices, ok := raw["choices"].([]interface{})
    if !ok || len(choices) == 0 {
        return "", fmt.Errorf("no choices returned")
    }

    message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
    content := message["content"].(string)

    return content, nil
}

// FormatOutput: formats raw tool output into a clean, user-friendly response
func FormatOutput(ctx context.Context, llm *openai.LLM, toolName, output, input string) (string, error) {

    messages := []map[string]string{
        {
            "role":    "system",
            "content": "You are a skilled text formatter. Rewrite the provided tool output into a clear, concise, user-friendly response.",
        },
        {
            "role":    "user",
            "content": fmt.Sprintf("Tool: %s\nInput: %s\nText: %s", toolName, input, output),
        },
    }

    payload := map[string]interface{}{
        "model":      "gpt-4o",
        "messages":   messages,
        "max_tokens": maxTokens,
        "stream":     false,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return "", fmt.Errorf("failed to marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", llmUrl, strings.NewReader(string(body)))
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llmToken))

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response: %w", err)
    }

    var raw map[string]interface{}
    if err := json.Unmarshal(respBody, &raw); err != nil {
        return "", fmt.Errorf("invalid JSON response: %w", err)
    }

    choices, ok := raw["choices"].([]interface{})
    if !ok || len(choices) == 0 {
        return "", fmt.Errorf("no choices returned")
    }

    message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
    content := message["content"].(string)

    return content, nil
}

// Utility to extract JSON substring from LLM response
func extractJson(s string) (string, bool) {
    start := strings.Index(s, "{")
    if start < 0 {
        return "", false
    }
    end := strings.LastIndex(s, "}")
    if end < 0 {
        return "", false
    }
    return s[start : end+1], true
}

// Sanitizer for LLM output
func sanitizeLLMOutput(content string) string {
    clean := strings.TrimSpace(content)

    reFence := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
    matches := reFence.FindStringSubmatch(clean)
    if len(matches) > 1 {
        clean = matches[1]
    }

    if idx := strings.Index(clean, "{"); idx >= 0 {
        clean = clean[idx:]
    }

    clean = strings.TrimRight(clean, "`")
    clean = strings.TrimSpace(clean)

    return clean
}
