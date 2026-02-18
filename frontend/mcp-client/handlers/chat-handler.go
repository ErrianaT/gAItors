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
            // Chat agent has no tools — return a no-op
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

    if err := json.NewDecoder(r.Body).Decode(&userMsg); err != nil {
        exception = fmt.Sprintf("Invalid JSON payload: %v", err)
    } else if userMsg.Role != "user" || userMsg.Content == "" {
        exception = fmt.Sprintf("Invalid message format: %v", userMsg)
    }

    if exception == "" {
        ctx := r.Context()

        agentName, err := llm.SelectAgent(ctx, utils.GetModel(), userMsg.Content)
        if err != nil {
            exception = fmt.Sprintf("Agent selection error: %v", err)
        } else {
            log.Printf("[ChatHandler] Agent selected: %s", agentName)

            agent, ok := AgentRegistry[agentName]
            if !ok {
                exception = fmt.Sprintf("Unknown agent selected: %s", agentName)
            } else {

                // If Chat Agent → return GenericResponse
                if agentName == "Chat Agent" {
                    resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
                    output = resp
                    goto RESPONSE
                }

                toolInfo, err := agent.SelectTool(ctx, utils.GetModel(), userMsg.Content)
                if err != nil {
                    exception = fmt.Sprintf("Tool selection error: %v", err)
                } else {
                    log.Printf("[ChatHandler] Tool selected: %s with args %+v", toolInfo.ToolName, toolInfo.ToolArgs)

                    // If tool is "none" → return GenericResponse
                    if toolInfo.ToolName == "none" {
                        resp, _ := llm.GenericResponse(ctx, utils.GetModel(), userMsg.Content)
                        output = resp
                        goto RESPONSE
                    }

                    // agent logic
                    mcpClients := common.GetMcpClients()
                    clientKey, ok := common.AgentClientMap[agentName]
                    if !ok {
                        log.Printf("[ChatHandler] Unknown agent: %s, defaulting to UF MCP client", agentName)
                        clientKey = "uf"
                    }
                    log.Printf("[ChatHandler] Routing agent=%s tool=%s to client=%s", agentName, toolInfo.ToolName, clientKey)

                    toolOutputStr, err := mcp.CallToolWithClient(ctx, mcpClients[clientKey], toolInfo)
                    log.Printf("toolOutputStr: %v", toolOutputStr)

                    if err != nil {
                        exception = fmt.Sprintf("CallTool error: %v", err)
                    } else {
                        var result protocol.CallToolResult
                        if err := json.Unmarshal([]byte(toolOutputStr), &result); err != nil {
                            output = toolOutputStr
                        } else {
                            var textParts []string
                            for _, c := range result.Content {
                                if tc, ok := c.(*protocol.TextContent); ok {
                                    if tc.Type == "image" {
                                        parts := strings.SplitN(tc.Text, "|", 2)
                                        if len(parts) == 2 {
                                            images = append(images, map[string]string{
                                                "mimeType": parts[0],
                                                "data":     parts[1],
                                            })
                                        }
                                    } else {
                                        textParts = append(textParts, tc.Text)
                                    }
                                }
                            }
                            if len(textParts) > 0 {
                                formattedOutput, _ := llm.FormatOutput(ctx, utils.GetModel(), toolInfo.ToolName, strings.Join(textParts, "\n"), userMsg.Content)
                                output = formattedOutput
                            }
                            if output == "" && len(images) > 0 {
                                output = fmt.Sprintf("Here is the live feed for %s camera.", toolInfo.ToolArgs["camera"])
                            }
                        }
                    }
                }
            }
        }
    }

RESPONSE:

    content := output
    if exception != "" {
        log.Printf("[ChatHandler] Exception occurred: %s", exception)
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
    if err := json.NewEncoder(w).Encode(assistantMsg); err != nil {
        http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
    }
}
