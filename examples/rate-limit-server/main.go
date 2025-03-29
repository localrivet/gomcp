// examples/rate-limit-server/main.go (Refactored)
package main

import (
	"fmt"
	"log"
	"os"

	// "strings" // No longer needed
	// "sync" // No longer needed
	// "time" // No longer needed

	mcp "github.com/localrivet/gomcp"
	"golang.org/x/time/rate"
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Rate limiting parameters: Allow 2 requests per second, with bursts up to 4.
const requestsPerSecond = 2
const burstLimit = 4

// Define the limited echo tool
var limitedEchoTool = mcp.ToolDefinition{
	Name:        "limited-echo",
	Description: "Echoes back the provided message, but is rate limited.",
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

// Global rate limiter (for simplicity in this example)
// In a real app, you might have per-API-key limiters stored in a map.
var globalLimiter = rate.NewLimiter(rate.Limit(requestsPerSecond), burstLimit)

// limitedEchoHandler implements the logic for the rate-limited echo tool.
// It checks the rate limit before processing.
func limitedEchoHandler(arguments map[string]interface{}) (result interface{}, errorPayload *mcp.ErrorPayload) {
	log.Printf("Executing limited-echo tool with args: %v", arguments)

	// --- Rate Limit Check ---
	// Check if the request is allowed according to the limiter.
	if !globalLimiter.Allow() {
		log.Println("Rate limit exceeded!")
		// Send a specific MCP error if rate limited.
		return nil, &mcp.ErrorPayload{
			Code:    mcp.ErrorCodeMCPRateLimitExceeded, // Use MCP code
			Message: fmt.Sprintf("Too many requests. Limit is %d per second (burst %d).", requestsPerSecond, burstLimit),
		}
	}
	log.Println("Rate limit check passed.")
	// --- End Rate Limit Check ---

	// --- Execute the "limited-echo" tool ---
	messageArg, ok := arguments["message"]
	if !ok {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Missing required argument 'message' for tool 'limited-echo'"} // Use MCP code
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Argument 'message' for tool 'limited-echo' must be a string"} // Use MCP code
	}

	log.Printf("Rate-Limited Echoing message: %s", messageStr)
	return messageStr, nil // Return result and nil error
	// --- End limited-echo tool execution ---
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// --- API Key Check (same as auth-server) ---
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: MCP_API_KEY environment variable not set.")
	}
	if apiKey != expectedApiKey {
		log.Fatalf("FATAL: Invalid MCP_API_KEY provided. Expected '%s'", expectedApiKey)
	}
	log.Println("API Key validated successfully.")
	// --- End API Key Check ---

	log.Println("Starting Rate Limit Example MCP Server (Refactored)...")

	// Create a new server instance using the library
	serverName := "GoRateLimitServer-Refactored"
	server := mcp.NewServer(serverName) // Uses stdio connection

	// Register the tool and its handler
	err := server.RegisterTool(limitedEchoTool, limitedEchoHandler)
	if err != nil {
		log.Fatalf("Failed to register limited-echo tool: %v", err)
	}

	// Run the server's main loop
	err = server.Run()
	if err != nil {
		log.Printf("Server exited with error: %v", err)
		os.Exit(1)
	} else {
		log.Println("Server finished.")
	}
}
