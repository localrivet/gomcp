package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp"
)

// requestToolDefinitions uses the client to request tool definitions.
func requestToolDefinitions(client *gomcp.Client) ([]gomcp.Tool, error) {
	log.Println("Sending ListToolsRequest...")
	params := gomcp.ListToolsRequestParams{} // No pagination/filtering in this example
	result, err := client.ListTools(params)
	if err != nil {
		return nil, fmt.Errorf("ListTools failed: %w", err)
	}
	// TODO: Handle pagination if result.NextCursor is not empty
	log.Printf("Received %d tool definitions", len(result.Tools))
	return result.Tools, nil
}

// useTool sends a CallToolRequest using the client and processes the response.
func useTool(client *gomcp.Client, toolName string, args map[string]interface{}) ([]gomcp.Content, error) {
	log.Printf("Sending CallToolRequest for tool '%s'...", toolName)
	reqParams := gomcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	// Call the tool using the client method
	result, err := client.CallTool(reqParams, nil) // Use default timeout
	if err != nil {
		// This error could be a transport error, timeout, or an MCP error response
		return nil, fmt.Errorf("CallTool '%s' failed: %w", toolName, err)
	}

	// Check if the tool execution itself resulted in an error (IsError flag)
	if result.IsError != nil && *result.IsError {
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", toolName)
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(gomcp.TextContent); ok {
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", toolName, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", toolName, result.Content[0])
			}
		}
		return result.Content, fmt.Errorf(errMsg)
	}

	log.Printf("Tool '%s' executed successfully.", toolName)
	return result.Content, nil
}

// runClientLogic creates a client, connects, and executes the example tool calls sequence.
func runClientLogic(clientName string) error {
	// Create a new client instance
	client := gomcp.NewClient(clientName)

	// Connect and perform initialization
	log.Println("Connecting to server...")
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("client failed to connect: %w", err)
	}
	defer client.Close() // Ensure connection is closed eventually
	log.Printf("Client connected successfully to server: %s (Version: %s)", client.ServerInfo().Name, client.ServerInfo().Version)
	log.Printf("Server Capabilities: %+v", client.ServerCapabilities())

	// --- Request Tool Definitions ---
	tools, err := requestToolDefinitions(client) // Pass client instance
	if err != nil {
		return fmt.Errorf("failed to get tool definitions: %w", err)
	}
	log.Printf("Received %d tool definitions:", len(tools))
	for _, tool := range tools {
		toolJson, _ := json.MarshalIndent(tool, "", "  ")
		fmt.Fprintf(os.Stderr, "%s\n", string(toolJson))
	}
	// --- End Request Tool Definitions ---

	// --- Use the Chargeable Echo Tool ---
	chargeableEchoToolFound := false
	for _, tool := range tools {
		if tool.Name == "chargeable-echo" {
			chargeableEchoToolFound = true
			break
		}
	}
	if chargeableEchoToolFound {
		log.Println("\n--- Testing Chargeable Echo Tool ---")
		echoMessage := "This message should be billed!"
		args := map[string]interface{}{"message": echoMessage}
		result, err := useTool(client, "chargeable-echo", args) // Pass client instance
		if err != nil {
			// This might fail if the server requires auth and none was provided (e.g., via env var)
			log.Printf("ERROR: Failed to use 'chargeable-echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'chargeable-echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result)
			if len(result) > 0 {
				if textContent, ok := result[0].(gomcp.TextContent); ok {
					log.Printf("  Extracted Text: %s", textContent.Text)
					if textContent.Text != echoMessage {
						log.Printf("WARNING: Chargeable Echo result '%s' did not match sent message '%s'", textContent.Text, echoMessage)
					}
				} else {
					log.Printf("WARNING: Chargeable Echo result content[0] was not TextContent: %T", result[0])
				}
			} else {
				log.Printf("WARNING: Chargeable Echo result content was empty!")
			}
		}
	} else {
		log.Println("Could not find 'chargeable-echo' tool definition from server.")
	}
	// --- End Use Chargeable Echo Tool ---

	// --- Ping Server ---
	log.Println("\n--- Testing Ping ---")
	err = client.Ping(5 * time.Second)
	if err != nil {
		log.Printf("ERROR: Ping failed: %v", err)
	} else {
		log.Println("Ping successful!")
	}
	// --- End Ping Server ---

	log.Println("Client operations finished.")
	return nil // Indicate success
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Billing Example MCP Client...")

	clientName := "GoBillingClient-Refactored"

	// Run the core client logic
	err := runClientLogic(clientName)
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}
