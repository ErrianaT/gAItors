package weather_agent

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

var weatherToolPrompt = `
You are a tool selector for the Weather Agent.
Available tools:
- UFWeather
- GymCam
- TransitArrivalPlanner

Arguments:
- UFWeather: {"location":"<city,state,country>","forecast":<true|false>}
- GymCam: {"camera":"<gym camera id>"}
- TransitArrivalPlanner: {"stop":"<stop name>","route":"<route number>"}

Examples:
User: "What's the weather in Gainesville today?"
→ {"tool_name":"UFWeather","tool_args":{"location":"Gainesville,FL,US","forecast":false}}

User: "Show me the forecast for Gainesville tomorrow"
→ {"tool_name":"UFWeather","tool_args":{"location":"Gainesville,FL,US","forecast":true}}

User: "Show me the Southwest Rec Center gym cam"
→ {"tool_name":"GymCam","tool_args":{"camera":"Southwest Rec"}}

User: "When does Route 5 arrive at Hub stop?"
→ {"tool_name":"TransitArrivalPlanner","tool_args":{"stop":"Hub","route":"5"}}

Return ONLY valid JSON: {"tool_name":"<tool>","tool_args":{...}}
Do not include backticks, code fences, or any extra text.
The first character MUST be '{' and the last character MUST be '}'.
`

func SelectWeatherTool(ctx context.Context, _ *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
    messages := []map[string]string{
        {"role": "system", "content": weatherToolPrompt},
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
        return nil, fmt.Errorf("weather: request failed: %w", err)
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

    // 🔎 Log raw output
    log.Printf("Weather Agent raw output: %q", content)

    // Sanitize
    clean := sanitizeLLMOutput(content)

    // 🔎 Log sanitized output
    log.Printf("Weather Agent sanitized output: %q", clean)

    return llm.BuildToolSelectionResponse(clean)
}

// SanitizeLLMOutput removes markdown fences/backticks so JSON can be parsed cleanly.
func sanitizeLLMOutput(content string) string {
    clean := strings.TrimSpace(content)

    // Regex to capture JSON inside ```...``` fences
    reFence := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
    matches := reFence.FindStringSubmatch(clean)
    if len(matches) > 1 {
        clean = matches[1]
    }

    // Ensure we start at the first '{'
    if idx := strings.Index(clean, "{"); idx >= 0 {
        clean = clean[idx:]
    }

    // Trim trailing junk
    clean = strings.TrimRight(clean, "`")
    clean = strings.TrimSpace(clean)

    return clean
}
