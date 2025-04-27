package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

// runClientLogic handles connecting to the server and executing operations
func runClientLogic(ctx context.Context, clt *client.Client) error {
	// Connect to the server
	log.Println("Connecting to server...")
	err := clt.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer clt.Close()

	// Get server info after successful connection
	serverInfo := clt.ServerInfo()
	log.Printf("Connected to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)

	// Get list of available tools
	log.Println("Requesting tool definitions...")
	toolsResult, err := clt.ListTools(ctx, protocol.ListToolsRequestParams{})
	if err != nil {
		return fmt.Errorf("failed to get tool definitions: %w", err)
	}

	log.Printf("Received %d tool definitions", len(toolsResult.Tools))

	// Log all available tools
	for _, tool := range toolsResult.Tools {
		log.Printf("Tool: %s - %s", tool.Name, tool.Description)
	}

	// Look for get_all_users tool
	getAllUsersFound := false
	for _, tool := range toolsResult.Tools {
		if tool.Name == "get_all_users" {
			getAllUsersFound = true
			break
		}
	}

	if getAllUsersFound {
		log.Println("Found get_all_users tool, calling it...")

		// Call the get_all_users tool
		callParams := protocol.CallToolParams{
			Name: "get_all_users",
		}

		result, err := clt.CallTool(ctx, callParams, nil)
		if err != nil {
			log.Printf("ERROR: Failed to call get_all_users tool: %v", err)
		} else {
			log.Println("Successfully called get_all_users tool")

			// Process and display the result
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(protocol.TextContent); ok {
					// Try to pretty-print the JSON
					var users map[string]interface{}
					if err := json.Unmarshal([]byte(textContent.Text), &users); err != nil {
						log.Printf("Raw result: %s", textContent.Text)
					} else {
						usersJSON, _ := json.MarshalIndent(users, "", "  ")
						log.Printf("Users: %s", string(usersJSON))
					}
				} else {
					log.Printf("Result content is not text: %T", result.Content[0])
				}
			} else {
				log.Println("Result content was empty")
			}
		}
	} else {
		log.Println("get_all_users tool not found")
	}

	log.Println("Client operations completed")
	return nil
}

func main() {
	// Set up logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[BasicClient] ")

	// Get server URL from environment or use default
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8681"
	}

	log.Printf("Starting basic MCP client, connecting to %s", serverURL)

	// Create the client with options
	clt, err := client.NewClient("BasicMCPClient", client.ClientOptions{
		ServerBaseURL: serverURL,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run the client logic
	if err := runClientLogic(ctx, clt); err != nil {
		log.Fatalf("Client error: %v", err)
	}

	log.Println("Client finished successfully")
}
