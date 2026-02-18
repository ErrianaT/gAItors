package tools

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

// Define the data structure for the tool
type Calculator struct {
	Operation string  `json:"operation" description:"Operation to be performed on the two numbers" required:"true"`
	A         float64 `json:"a" description:"First operand of the operation" required:"true"`
	B         float64 `json:"b" description:"Second operand of the operation" required:"true"`
}

// Name of the tool
func (t *Calculator) Name() string {
	return "Calculator"
}

// Descrition of the tool
func (t *Calculator) Description() string {
	var desc = []string{
		"Tool to perform arithmetic operation on two numbers.",
		"Valid operations are add, subtract, multiply, divide, and exponentiate.",
		"The input to this tool should be two floating point numbers",
	}
	return strings.Join(desc, "\n")
}

// A function to do the tool's work
func handleCalculator(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var request Calculator
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &request); err != nil {
		return nil, err
	}

	var result float64

	switch request.Operation {
	case "add":
		result = request.A + request.B
	case "subtract":
		result = request.A - request.B
	case "multiply":
		result = request.A * request.B
	case "divide":
		result = request.A / request.B
	case "exponentiate":
		result = math.Pow(request.A, request.B)
	}

	return &protocol.CallToolResult{

		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%.4f", result),
			},
		},
		IsError: false,
	}, nil
}

func GetCalculatorTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := Calculator{}
	tool, err := protocol.NewTool(toolStruct.Name(), // Name of the tool
		toolStruct.Description(), // Description of the tool
		toolStruct,               // Data structure for the tool
	)

	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	return tool, handleCalculator
}
