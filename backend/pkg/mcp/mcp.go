package mcp

import (
    "context"
    "encoding/json"
    "fmt"
    "uf/mcp/pkg/common"
    "uf/mcp/pkg/llm"

    "github.com/ThinkInAIXYZ/go-mcp/client"
    "github.com/ThinkInAIXYZ/go-mcp/protocol"
)

type ToolInfo struct {
    ToolName     *protocol.Tool
    SourceClient *client.Client
}

var (
    toolList   string
    toolsTable = make(map[string]ToolInfo)
)

func GetToolListSchema() string {
    return toolList
}

// Default CallTool: uses toolsTable lookup
func CallTool(ctx context.Context, selectedTool *llm.SelectedToolInfo) (string, error) {
    toolInfo, ok := toolsTable[selectedTool.ToolName]
    if !ok {
        return "", fmt.Errorf("unknown tool: %s", selectedTool.ToolName)
    }

    targetClient := toolInfo.SourceClient
    request := protocol.NewCallToolRequest(selectedTool.ToolName, selectedTool.ToolArgs)

    result, err := targetClient.CallTool(ctx, request)
    if err != nil {
        return "", err
    }

    b, err := json.Marshal(result)
    if err != nil {
        return "", err
    }

    if result.IsError {
        return string(b), fmt.Errorf("error calling tool")
    }
    return string(b), nil
}

// CallToolWithClient: send tool calls directly to a cached MCP client
func CallToolWithClient(ctx context.Context, c *client.Client, selectedTool *llm.SelectedToolInfo) (string, error) {
    request := protocol.NewCallToolRequest(selectedTool.ToolName, selectedTool.ToolArgs)
    result, err := c.CallTool(ctx, request)
    if err != nil {
        return "", err
    }

    b, err := json.Marshal(result)
    if err != nil {
        return "", err
    }

    if result.IsError {
        return string(b), fmt.Errorf("error calling tool")
    }
    return string(b), nil
}

// Initialize all tools from all MCP clients
func InitalizeTools() {
    allToolsForSchema := []*protocol.Tool{}
    toolsTable = make(map[string]ToolInfo)
    mcpClients := common.GetMcpClients()

    if len(mcpClients) == 0 {
        toolList = `{tools: []}`
        return
    }

    for _, client := range mcpClients {
        result, err := client.ListTools(context.Background())
        if err != nil {
            continue
        }
        for _, tool := range result.Tools {
            if _, exists := toolsTable[tool.Name]; exists {
                continue
            }
            toolsTable[tool.Name] = ToolInfo{
                ToolName:     tool,
                SourceClient: client,
            }
            allToolsForSchema = append(allToolsForSchema, tool)
        }
    }

    jsonDoc, _ := json.Marshal(struct {
        Tools []*protocol.Tool `json:"tools"`
    }{Tools: allToolsForSchema})
    toolList = string(jsonDoc)
}
