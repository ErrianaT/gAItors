package utils

import (
	"log"
	"os"
	"uf/mcp/pkg/common"
)

func InitializeConfiguration() {

	// Get AppRoot from environment variable
	if v, found := os.LookupEnv("APP_ROOT"); found {
		AppRoot = v
	} else {
		log.Fatalf("env variable APP_ROOT is required")
	}

	// Get MCP Client ...
	mcpClients = common.GetMcpClients()

	// Get LLM ...
	model = common.GetModel()

}
