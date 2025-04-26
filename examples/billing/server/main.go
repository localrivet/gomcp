package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// Keep for handler signature if needed, otherwise remove
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the chargeable echo tool
var chargeableEchoTool = protocol.Tool{
	Name:        "chargeable-echo",
	Description: "Echoes back the provided message (Simulates Billing/Tracking).",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
}

// chargeableEchoHandlerFactory creates a handler closure that captures the apiKey.
func chargeableEchoHandlerFactory(apiKey string) server.ToolHandlerFunc {
	return func(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
		args, ok := arguments.(map[string]interface{})
		if !ok {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'chargeable-echo' (expected object)"}}, true
		}
		// Use standard log for handler logging in this simplified example
		log.Printf("Executing chargeable-echo tool with args: %v", args)

		newErrorContent := func(msg string) []protocol.Content {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}}
		}

		// --- Simulate Billing/Tracking Event ---
		billingEvent := map[string]interface{}{
			"event_type": "tool_usage",
			"api_key":    apiKey,
			"tool_name":  chargeableEchoTool.Name,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		}
		eventJson, _ := json.Marshal(billingEvent)
		log.Printf("BILLING_EVENT: %s", string(eventJson)) // Use standard log
		// --- End Billing/Tracking ---

		// --- Execute the "chargeable-echo" tool ---
		messageArg, ok := args["message"]
		if !ok {
			return newErrorContent("Missing required argument 'message' for tool 'chargeable-echo'"), true
		}
		messageStr, ok := messageArg.(string)
		if !ok {
			return newErrorContent("Argument 'message' for tool 'chargeable-echo' must be a string"), true
		}

		log.Printf("Chargeable Echoing message: %s", messageStr) // Use standard log
		successContent := protocol.TextContent{Type: "text", Text: messageStr}
		return []protocol.Content{successContent}, false
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

	log.Println("Starting Billing/Tracking Example MCP Server...")

	// Create the server with default options (uses internal default logger)
	srv := server.NewServer("GoBillingServer") // Use default options

	// Create the handler closure, capturing the validated apiKey
	handler := chargeableEchoHandlerFactory(apiKey)

	// Register the tool and its handler
	if err := srv.RegisterTool(chargeableEchoTool, handler); err != nil {
		log.Fatalf("Failed to register chargeable-echo tool: %v", err)
	}

	// Run the server using the ServeStdio helper
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}
