// examples/billing-server/main.go (Refactored)
package main

import (
	"encoding/json" // For logging structured billing event
	// Needed for handler error messages
	"log"
	"os"
	"time" // For timestamp

	mcp "github.com/localrivet/gomcp"
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the chargeable echo tool
var chargeableEchoTool = mcp.Tool{ // Use new Tool struct
	Name:        "chargeable-echo",
	Description: "Echoes back the provided message (Simulates Billing/Tracking).",
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
	// OutputSchema removed
	// Annotations: mcp.ToolAnnotations{}, // Optional
}

// chargeableEchoHandlerFactory creates a handler closure that captures the apiKey.
// This allows the handler to access the validated API key without needing it passed
// explicitly on every call within the server's Run loop.
func chargeableEchoHandlerFactory(apiKey string) mcp.ToolHandlerFunc {
	// Return a function matching the new signature: ([]Content, bool)
	return func(arguments map[string]interface{}) ([]mcp.Content, bool) {
		log.Printf("Executing chargeable-echo tool with args: %v", arguments)

		// Helper to create error response content
		newErrorContent := func(msg string) []mcp.Content {
			return []mcp.Content{mcp.TextContent{Type: "text", Text: msg}}
		}

		// --- Simulate Billing/Tracking Event ---
		// In a real system, this would record to a database or billing service.
		// Here, we just log a structured message to stderr.
		billingEvent := map[string]interface{}{
			"event_type": "tool_usage",
			"api_key":    apiKey,                  // Use the captured apiKey
			"tool_name":  chargeableEchoTool.Name, // Use tool name from definition
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		}
		eventJson, _ := json.Marshal(billingEvent) // Ignore error for logging
		log.Printf("BILLING_EVENT: %s", string(eventJson))
		// --- End Billing/Tracking ---

		// --- Execute the "chargeable-echo" tool ---
		messageArg, ok := arguments["message"]
		if !ok {
			return newErrorContent("Missing required argument 'message' for tool 'chargeable-echo'"), true // isError = true
		}
		messageStr, ok := messageArg.(string)
		if !ok {
			return newErrorContent("Argument 'message' for tool 'chargeable-echo' must be a string"), true // isError = true
		}

		log.Printf("Chargeable Echoing message: %s", messageStr)
		successContent := mcp.TextContent{Type: "text", Text: messageStr}
		return []mcp.Content{successContent}, false // isError = false
		// --- End chargeable-echo tool execution ---
	}
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

	log.Println("Starting Billing/Tracking Example MCP Server (Refactored)...")

	// Create a new server instance using the library
	serverName := "GoBillingServer-Refactored"
	server := mcp.NewServer(serverName) // Uses stdio connection

	// Create the handler closure, capturing the validated apiKey
	handler := chargeableEchoHandlerFactory(apiKey)

	// Register the tool and its handler
	err := server.RegisterTool(chargeableEchoTool, handler) // Pass updated tool struct
	if err != nil {
		log.Fatalf("Failed to register chargeable-echo tool: %v", err)
	}

	// Run the server's main loop (handles handshake and message dispatch)
	err = server.Run()
	if err != nil {
		log.Printf("Server exited with error: %v", err)
		os.Exit(1) // Exit with non-zero status on error
	} else {
		log.Println("Server finished.")
	}
}
