package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"uf/mcp/mcp-client/utils"
	"uf/mcp/pkg/common"
	"uf/mcp/pkg/llm"
	"uf/mcp/pkg/mcp"

	// Import agent adapters
	"uf/mcp/agents/rag_agent"
	"uf/mcp/agents/restaurants_agent"
	"uf/mcp/agents/safety_agent"
	"uf/mcp/agents/schedule_agent"
	"uf/mcp/agents/transit_agent"
	"uf/mcp/agents/weather_agent"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/tmc/langchaingo/llms/openai"
)

type Message struct {
    Role    string      `json:"role"`
    Content string      `json:"content"`
    Images  interface{} `json:"images,omitempty"`
}

var AgentRegistry = map[string]llm.Agent{
    "Safety Agent": {
        Name: "Safety Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return safety_agent.SelectSafetyTool(ctx, model, query)
        },
    },
    "Schedule Agent": {
        Name: "Schedule Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return schedule_agent.SelectScheduleTool(ctx, model, query)
        },
    },
    "Weather Agent": {
        Name: "Weather Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return weather_agent.SelectWeatherTool(ctx, model, query)
        },
    },
    "Restaurants Agent": {
        Name: "Restaurants Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return restaurants_agent.SelectRestaurantsTool(ctx, model, query)
        },
    },
    "Transit Agent": {
        Name: "Transit Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return transit_agent.SelectTransitTool(ctx, model, query)
        },
    },
    "RAG Agent": {
        Name: "RAG Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return rag_agent.SelectRAGTool(ctx, model, query)
        },
    },
    "Chat Agent": {
        Name: "Chat Agent",
        SelectTool: func(ctx context.Context, model *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
            return &llm.SelectedToolInfo{
                ToolName: "none",
                ToolArgs: map[string]any{},
            }, nil
        },
    },
}

func ChatHandler(w http.ResponseWriter, r *http.Request) {
    log.Printf("[ChatHandler] Processing new user message")

    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "POST")
    w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")

    var userMsg Message
    var output string
    var exception string
    var images []map[string]string
    var done bool

    // -------------------------------
    // USER INPUT VALIDATION
    // -------------------------------
    if err := json.NewDecoder(r.Body).Decode(&userMsg); err != nil {
        exception = fmt.Sprintf("Invalid JSON payload: %v", err)
        done = true
    }
    if !done && (userMsg.Role != "user" || userMsg.Content == "") {
        exception = fmt.Sprintf("Invalid message format: %v", userMsg)
        done = true
    }

    ctx := r.Context()

    // -------------------------------
    // AGENT SELECTION
    // -------------------------------
    var agentName string
    var agent llm.Agent
    if !done {
        var err error
        agentName, err = llm.SelectAgent(ctx, utils.GetModel(), userMsg.Content)
        if err != nil {
            exception = fmt.Sprintf("Agent selection error: %v", err)
            done = true
        }
    }

    if !done {
        var ok bool
        agent, ok = AgentRegistry[agentName]
        if !ok {
            exception = fmt.Sprintf("Unknown agent selected: %s", agentName)
            done = true
        }
    }

    // Chat Agent → fallback to GenericResponse
    if !done && agentName == "Chat Agent" {
        resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
        output = resp
        done = true
    }

    // -------------------------------
    // TOOL SELECTION
    // -------------------------------
    var toolInfo *llm.SelectedToolInfo
    if !done {
        var err error
        toolInfo, err = agent.SelectTool(ctx, utils.GetModel(), userMsg.Content)
        if err != nil {
            // FALLBACK (not exception)
            resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
            output = resp
            done = true
        }
    }

    // Tool = none → fallback
    if !done && toolInfo.ToolName == "none" {
        resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
        output = resp
        done = true
    }

    // -------------------------------
    // MCP CLIENT ROUTING + TOOL CALL
    // -------------------------------
    if !done {
        mcpClients := common.GetMcpClients()
        clientKey, ok := common.AgentClientMap[agentName]
        if !ok {
            clientKey = "uf"
        }

        toolOutputStr, err := mcp.CallToolWithClient(ctx, mcpClients[clientKey], toolInfo)

        // Transport error → fallback
        if err != nil {
            resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
            output = resp
            done = true
        }

        // Parse JSON
        var result protocol.CallToolResult
        if !done {
            jsonErr := json.Unmarshal([]byte(toolOutputStr), &result)
            if jsonErr != nil {
                resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
                output = resp
                done = true
            }

            // MCP error → fallback
            if !done && result.IsError {
                resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
                output = resp
                done = true
            }

            // -------------------------------
            // EXTRACT CONTENT
            // -------------------------------
            var textParts []string
            if !done {
                for _, c := range result.Content {
                    switch content := c.(type) {

                    case *protocol.TextContent:
                        if content.Type == "image" {
                            parts := strings.SplitN(content.Text, "|", 2)
                            if len(parts) == 2 {
                                images = append(images, map[string]string{
                                    "mimeType": parts[0],
                                    "data":     parts[1],
                                })
                            }
                            continue
                        }
                        textParts = append(textParts, content.Text)

                    default:
                        raw, _ := json.Marshal(content)
                        var maybe struct {
                            Text string `json:"text"`
                        }
                        if json.Unmarshal(raw, &maybe) == nil && maybe.Text != "" {
                            textParts = append(textParts, maybe.Text)
                        }
                    }
                }

                // Semantic error detection
                joined := strings.ToLower(strings.Join(textParts, "\n"))
                if strings.Contains(joined, "error:") ||
                    strings.Contains(joined, "unable to") ||
                    strings.Contains(joined, "failed") ||
                    strings.Contains(joined, "exception") {

                    resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
                    output = resp
                    done = true
                }

                // Successful tool output
                if !done && len(textParts) > 0 {
                    formattedOutput, _ := llm.FormatOutput(
                        ctx,
                        utils.GetModel(),
                        toolInfo.ToolName,
                        strings.Join(textParts, "\n"),
                        userMsg.Content,
                    )
                    output = formattedOutput
                    done = true
                }

                if !done && output == "" && len(images) > 0 {
                    output = fmt.Sprintf("Here is the live feed for %s camera.", toolInfo.ToolArgs["camera"])
                    done = true
                }
            }
        }
    }

    // -------------------------------
    // FINAL RESPONSE BLOCK
    // -------------------------------
    content := output
    if exception != "" {
        content = exception
    }

    assistantMsg := Message{
        Role:    "assistant",
        Content: content,
    }
    if len(images) > 0 {
        assistantMsg.Images = images
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(assistantMsg)
}
