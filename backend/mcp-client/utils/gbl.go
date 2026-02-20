package utils

import (
	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/tmc/langchaingo/llms/openai"
)

// MCP server configuration

type MCPServerConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// MCPServerResponse holds metadata for routing

type MCPServerResponse struct {
	ServerID string
	ToolName string
	Response any
}

type AppConfig struct {
	AppRoot   string `json:"app_root"`
	LLMUrl    string `json:"llm_url"`
	KUBEUrl   string `json:"kube_url"`
	RTSUrl    string `json:"rts_url"`
	SEARCHUrl string `json:"search_url"`
	UFUrl     string `json:"uf_url"`

	//MCPConfig []MCPConfig `json:"mcp_config"`
}

var (
	AppRoot    string
	mcpClients map[string]*client.Client
	model      *openai.LLM
)

func GetMCPClients() map[string]*client.Client {
	return mcpClients
}

func GetModel() *openai.LLM {
	return model
}

func Stop() {
	for _, c := range mcpClients {
		c.Close()
	}

}
