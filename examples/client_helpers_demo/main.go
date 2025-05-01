package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

// --- Placeholder Structs for Tool Call Helper Demo ---

// GetAllUsersInput defines the (empty) input structure for the get_all_users tool.
type GetAllUsersInput struct{}

// UserSummary represents basic user info returned by get_all_users.
type UserSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetAllUsersOutput defines the expected output structure for the get_all_users tool.
type GetAllUsersOutput struct {
	Users []UserSummary `json:"users"`
}

// --- Helper Functions (Experimental) ---
/* ... commented out NewStdioClientAndConnect ... */

// --- Original runClientLogic (now uses CallToolByName helper) ---

// runClientLogic handles executing operations after connection
func runClientLogic(ctx context.Context, clt *client.Client) error {
	// NOTE: Connection is handled outside this function now

	// Get server info after successful connection
	if clt == nil {
		return fmt.Errorf("runClientLogic called with nil client")
	}

	// --- DEBUGGING ---
	connected, initialized, closed := clt.CurrentState()
	log.Printf("DEBUG: runClientLogic entry: Connected=%v, Initialized=%v, Closed=%v", connected, initialized, closed)
	// --- END DEBUGGING ---

	serverInfo := clt.ServerInfo() // Returns protocol.Implementation, not a pointer
	// Check if server info is valid (e.g., has a name)
	if serverInfo.Name == "" {
		// Additional logging if name is empty
		log.Printf("DEBUG: ServerInfo check failed. ServerInfo Name: '%s', Version: '%s'", serverInfo.Name, serverInfo.Version)
		return fmt.Errorf("failed to get valid server info (client not connected properly?)")
	}
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
		log.Println("Found get_all_users tool, calling it using CallToolByName helper...")

		// Use the CallToolByName helper
		input := GetAllUsersInput{} // Empty input struct
		usersOutput, err := CallToolByName[GetAllUsersInput, GetAllUsersOutput](ctx, clt, "get_all_users", input)

		if err != nil {
			log.Printf("ERROR using CallToolByName for get_all_users: %v", err)
		} else if usersOutput == nil {
			log.Println("CallToolByName for get_all_users returned nil output unexpectedly.")
		} else {
			log.Println("Successfully called get_all_users via CallToolByName.")

			// Process the structured output
			if len(usersOutput.Users) > 0 {
				log.Printf("Received %d users:", len(usersOutput.Users))
				// Pretty-print the structured result
				usersJSON, _ := json.MarshalIndent(usersOutput.Users, "", "  ")
				log.Printf("Users: %s", string(usersJSON))
				// Alternatively, iterate and print:
				// for _, user := range usersOutput.Users {
				// 	 log.Printf(" - ID: %s, Name: %s", user.ID, user.Name)
				// }
			} else {
				log.Println("get_all_users tool returned an empty list of users.")
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
	log.SetPrefix("[HelperDemoClient] ")

	log.Println("Starting MCP client helper demo...")

	// --- Configuration Loading Demo ---
	sampleConfig := client.MCPConfig{
		MCPServers: map[string]client.ServerConfig{
			"travel_agent_sse": { // Focus on this one for now
				Command: "@sse:http://localhost:8089/",
			},
		},
	}

	// Create a context for the application lifetime (used for loading & termination)
	// This context should also be passed down to long-running operations if any.
	appCtx, appCancel := context.WithCancel(context.Background())
	// Ensure appCancel is called eventually, even if signal handling has issues.
	// The defer for TerminateManagedClients is also important for cleanup.
	defer appCancel()

	// Attempt to load servers from the config
	serverClients, err := LoadServersFromConfig(appCtx, sampleConfig)
	if err != nil {
		log.Printf("Warning: Errors encountered during server loading/connection: %v", err)
	}
	// Ensure server client connections (and their processes) are terminated on exit
	defer TerminateManagedClients(serverClients)

	log.Printf("Loaded connections for %d MCP servers.", len(serverClients))

	// --- Using Dynamically Loaded Server Clients ---

	// Run initial logic for connected clients here, BEFORE waiting for signal
	if travelClientMC, ok := serverClients["travel_agent_sse"]; ok && travelClientMC.Client != nil {
		log.Println("--- Running initial logic using 'travel_agent_sse' client --- ")
		// Use a shorter timeout for this initial check
		initCtx, initCancel := context.WithTimeout(appCtx, 15*time.Second)
		if err := runClientLogic(initCtx, travelClientMC.Client); err != nil {
			log.Printf("Initial logic error using travel_agent_sse client: %v", err)
		}
		initCancel()
	} else {
		log.Println("Travel_agent_sse client not loaded or failed to connect.")
	}

	// Add blocks for other clients (basic_server, customer_service_docker) here if configs are re-enabled
	// ...

	log.Println("Initialization complete. Waiting for interrupt signal (Ctrl+C)...")

	// --- Wait for Termination Signal ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received.
	receivedSignal := <-sigChan
	log.Printf("Received signal: %v. Initiating shutdown...", receivedSignal)

	// --- Shutdown Triggered ---
	// Call appCancel() explicitly here to signal cancellation to any ongoing operations
	// although the defer will also call it on exit.
	appCancel()

	// The deferred TerminateManagedClients and appCancel calls will handle cleanup.
	log.Println("Client shutdown sequence initiated.")
}
