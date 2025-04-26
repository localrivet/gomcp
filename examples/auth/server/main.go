package main

import (
	"context" // Needed for handler
	// Needed for handler
	"log"
	"os"

	// Needed for handler
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// "github.com/localrivet/gomcp/types" // No longer needed directly
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the secure echo tool
var secureEchoTool = protocol.Tool{
	Name:        "secure-echo",
	Description: "Echoes back the provided message (Requires API Key Auth).",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
}

// secureEchoHandler implements the logic for the secure-echo tool.
// Note: In this simplified example, the API key check happens at startup.
// A real implementation might involve passing auth info via context.
func secureEchoHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, ok := arguments.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'secure-echo' (expected object)"}}, true
	}
	log.Printf("Executing secure-echo tool with args: %v", args) // Use standard log

	newErrorContent := func(msg string) []protocol.Content {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}}
	}

	messageArg, ok := args["message"]
	if !ok {
		return newErrorContent("Missing required argument 'message' for tool 'secure-echo'"), true
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		return newErrorContent("Argument 'message' for tool 'secure-echo' must be a string"), true
	}
	log.Printf("Securely Echoing message: %s", messageStr) // Use standard log
	successContent := protocol.TextContent{Type: "text", Text: messageStr}
	return []protocol.Content{successContent}, false
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// --- API Key Check ---
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: MCP_API_KEY environment variable not set.")
	}
	if apiKey != expectedApiKey {
		log.Fatalf("FATAL: Invalid MCP_API_KEY provided. Expected '%s'", expectedApiKey)
	}
	log.Println("API Key validated successfully.")
	// --- End API Key Check ---

	log.Println("Starting Auth Example MCP Server...")

	// Create the server with default options (uses internal default logger)
	srv := server.NewServer("GoAuthServer") // Use default options

	// Register the secure echo tool and its handler
	if err := srv.RegisterTool(secureEchoTool, secureEchoHandler); err != nil {
		log.Fatalf("Failed to register secure-echo tool: %v", err)
	}

	// Run the server using the ServeStdio helper
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}
