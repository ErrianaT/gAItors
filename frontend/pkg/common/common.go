package common

import (
	"log"
	"net/http"
	"os"

	custom "uf/mcp/pkg/transport"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/tmc/langchaingo/llms/openai"
)

// GetModel initializes the LLM client
func GetModel() *openai.LLM {
	var llmUrl, llmToken string

	if v, found := os.LookupEnv("LLM_URL"); found {
		llmUrl = v
	} else {
		log.Fatalf("env variable LLM_URL is required")
	}

	if v, found := os.LookupEnv("LLM_TOKEN"); found {
		llmToken = v
	} else {
		log.Fatalf("env variable LLM_TOKEN is required")
	}

	ct := custom.NewCustomTransport()
	ct.Token = llmToken
	ct.Path = "v1/chat/completions"

	clientHTTP := &http.Client{Transport: ct}
	ct.AlterBody = true

	llm, err := openai.New(
		openai.WithBaseURL(llmUrl),
		openai.WithHTTPClient(clientHTTP),
		openai.WithModel("gpt-4o"),
		openai.WithToken(llmToken),
	)
	if err != nil {
		log.Fatalf("Failed to initialize LLM: %v", err)
	}
	return llm
}

// GetMcpClients initializes MCP clients for all configured servers
func GetMcpClients() map[string]*client.Client {
	mcpClients := make(map[string]*client.Client)

	// Map environment variables to client names
	envUrls := map[string]string{
		"UF_MCP_URL":          "uf",     // Go MCP server (campus tools)
		"FILE_SEARCH_MCP_URL": "search", // Node rag-mcp-server (RAG tools)
		"KUBE_MCP_URL":        "kube",
		"RTS_MCP_URL":         "rts",
	}

	for envName, clientName := range envUrls {
		url, found := os.LookupEnv(envName)
		if !found {
			log.Printf("env variable %s not found, skipping client %s", envName, clientName)
			continue
		}

		ct := custom.NewCustomTransport()
		httpClient := &http.Client{Transport: ct}

		transportClient, err := transport.NewStreamableHTTPClientTransport(
			url,
			transport.WithStreamableHTTPClientOptionHTTPClient(httpClient),
		)
		if err != nil {
			log.Printf("Failed to create transport for %s: %v", clientName, err)
			continue
		}

		mcpClient, err := client.NewClient(transportClient)
		if err != nil {
			log.Printf("Failed to create MCP client for %s: %v", clientName, err)
			continue
		}

		log.Printf("Initialized MCP client: %s (%s)", clientName, url)
		mcpClients[clientName] = mcpClient
	}

	return mcpClients
}

// Agent-to-client mapping
var AgentClientMap = map[string]string{
	"RAG Agent":         "search", // Node rag-mcp-server
	"Transit Agent":     "rts",    // RTS MCP server
	"Safety Agent":      "uf",     // Go MCP server
	"Weather Agent":     "uf",     // Go MCP server
	"Restaurants Agent": "uf",     // Go MCP server
	"Schedule Agent":    "uf",     // Go MCP server
	"Kube Agent":        "kube",   // Kube MCP server
}
