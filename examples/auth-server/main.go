// examples/auth-server/main.go (Refactored)
package main

import (
	"log"
	"os"

	// "strings" // No longer needed

	mcp "github.com/localrivet/gomcp"
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the secure echo tool
var secureEchoTool = mcp.ToolDefinition{
	Name:        "secure-echo",
	Description: "Echoes back the provided message (Requires API Key Auth).",
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
	OutputSchema: mcp.ToolOutputSchema{
		Type:        "string",
		Description: "The original message.",
	},
}

// secureEchoHandler implements the logic for the secure-echo tool.
func secureEchoHandler(arguments map[string]interface{}) (result interface{}, errorPayload *mcp.ErrorPayload) {
	log.Printf("Executing secure-echo tool with args: %v", arguments)
	messageArg, ok := arguments["message"]
	if !ok {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeInvalidArgument, Message: "Missing required argument 'message' for tool 'secure-echo'"}
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeInvalidArgument, Message: "Argument 'message' for tool 'secure-echo' must be a string"}
	}
	log.Printf("Securely Echoing message: %s", messageStr)
	return messageStr, nil // Return result and nil error
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// --- API Key Check ---
	// This check happens *before* starting the MCP server loop.
	// If the key is invalid, the server exits immediately.
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: MCP_API_KEY environment variable not set.")
	}
	if apiKey != expectedApiKey {
		log.Fatalf("FATAL: Invalid MCP_API_KEY provided. Expected '%s'", expectedApiKey)
	}
	log.Println("API Key validated successfully.")
	// --- End API Key Check ---

	log.Println("Starting Auth Example MCP Server (Refactored)...")

	// Create a new server instance using the library
	serverName := "GoAuthServer-Refactored"
	server := mcp.NewServer(serverName) // Uses stdio connection

	// Register the secure echo tool and its handler
	err := server.RegisterTool(secureEchoTool, secureEchoHandler)
	if err != nil {
		log.Fatalf("Failed to register secure-echo tool: %v", err)
	}

	// Run the server's main loop (handles handshake and message dispatch)
	// Note: The API key check already happened. If more granular auth per request
	// is needed, it would have to be implemented within the ToolHandlerFunc,
	// potentially by passing context or modifying the handler signature.
	err = server.Run()
	if err != nil {
		log.Printf("Server exited with error: %v", err)
		os.Exit(1) // Exit with non-zero status on error
	} else {
		log.Println("Server finished.")
	}
}
