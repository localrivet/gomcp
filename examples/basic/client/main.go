package main

import (
	"context" // Added context
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time" // Added for timeout context

	// Import new packages
	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	// "github.com/localrivet/gomcp/transport/stdio" // No longer using stdio directly here
	// "github.com/localrivet/gomcp/types" // Not needed directly here
	// "github.com/google/uuid" // For progress token generation if needed
)

// --- Helper Functions ---

// Helper function to get a pointer to a boolean value.
func BoolPtr(b bool) *bool {
	return &b
}

// Helper function to get a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}

// --- Main Client Logic ---

// runClientLogic connects the provided client and executes the example tool calls sequence.
// Returns an error if any fatal step fails (connection, getting tool defs).
// Tool usage errors are logged but do not cause this function to return an error.
func runClientLogic(ctx context.Context, clt *client.Client) error { // Accept *client.Client, add ctx
	// Connect and perform initialization
	log.Println("Connecting to server...")
	// Pass context to Connect
	err := clt.Connect(ctx) // Use the provided client and ctx
	if err != nil {
		return fmt.Errorf("client failed to connect: %w", err)
	}
	defer clt.Close() // Ensure connection is closed eventually

	// Access server info and capabilities using the new getters
	serverInfo := clt.ServerInfo()
	serverCaps := clt.ServerCapabilities()
	log.Printf("Client connected successfully to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)
	log.Printf("Server Capabilities: %+v", serverCaps)

	// --- Request Tool Definitions ---
	log.Println("\n--- Requesting Tool Definitions ---")
	listParams := protocol.ListToolsRequestParams{}
	toolsResult, err := clt.ListTools(ctx, listParams) // Add ctx
	if err != nil {
		// Treat failure to get definitions as fatal for this example client
		return fmt.Errorf("failed to get tool definitions: %w", err)
	}
	log.Printf("Received %d tool definitions:", len(toolsResult.Tools))
	tools := toolsResult.Tools // Store for later checks
	for _, tool := range tools {
		toolJson, _ := json.MarshalIndent(tool, "", "  ")
		fmt.Fprintf(os.Stderr, "%s\n", string(toolJson))
	}
	// --- End Request Tool Definitions ---

	// --- Use the Echo Tool ---
	echoToolFound := false
	for _, tool := range tools {
		if tool.Name == "echo" {
			echoToolFound = true
			break
		}
	}
	if echoToolFound {
		log.Println("\n--- Testing Echo Tool ---")
		echoMessage := "Hello from Go MCP Client!"
		args := map[string]interface{}{"message": echoMessage}
		callParams := protocol.CallToolParams{Name: "echo", Arguments: args}
		// Call tool without requesting progress
		result, err := clt.CallTool(ctx, callParams, nil) // Add ctx
		if err != nil {
			log.Printf("ERROR: Failed to use 'echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result.Content) // Log the content slice
			// Extract text from the first TextContent element
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(protocol.TextContent); ok {
					log.Printf("  Extracted Text: %s", textContent.Text)
					// Note: The echo tool in kitchen-sink prepends "Echo: "
					expectedEcho := "Echo: " + echoMessage
					if textContent.Text != expectedEcho {
						log.Printf("WARNING: Echo result '%s' did not match expected '%s'", textContent.Text, expectedEcho)
					}
				} else {
					log.Printf("WARNING: Echo result content[0] was not TextContent: %T", result.Content[0])
				}
			} else {
				log.Printf("WARNING: Echo result content was empty!")
			}
		}
	} else {
		log.Println("Could not find 'echo' tool definition from server.")
	}
	// --- End Use Echo Tool ---

	// --- Use the Calculator Tool (Add) ---
	calculatorToolFound := false
	for _, tool := range tools {
		if tool.Name == "add" { // Assuming kitchen-sink uses 'add' now
			calculatorToolFound = true
			break
		}
	}
	if calculatorToolFound {
		log.Println("\n--- Testing Add Tool ---")
		// Example 1: Add
		calcArgs1 := map[string]interface{}{"a": 5.0, "b": 7.0} // Use 'a' and 'b' as per kitchen-sink
		calcParams1 := protocol.CallToolParams{Name: "add", Arguments: calcArgs1}
		result1, err1 := clt.CallTool(ctx, calcParams1, nil) // Add ctx
		if err1 != nil {
			log.Printf("ERROR: Failed to use 'add' tool: %v", err1)
		} else {
			log.Printf("Add Tool Content: %+v", result1.Content)
			// Extract text, check result
			if len(result1.Content) > 0 {
				if textContent, ok := result1.Content[0].(protocol.TextContent); ok {
					expectedResultStr := "The sum of 5.000000 and 7.000000 is 12.000000."
					if textContent.Text != expectedResultStr {
						log.Printf("WARNING: Add tool result unexpected: %s", textContent.Text)
					} else {
						log.Printf("  Parsed Result Correct: %s", textContent.Text)
					}
				} else {
					log.Printf("WARNING: Add tool result content[0] was not TextContent: %T", result1.Content[0])
				}
			} else {
				log.Printf("WARNING: Add tool result content was empty!")
			}
		}
		// Example 2: Missing argument (expecting error in result)
		calcArgs3 := map[string]interface{}{"a": 10.0} // Missing 'b'
		calcParams3 := protocol.CallToolParams{Name: "add", Arguments: calcArgs3}
		result3, err3 := clt.CallTool(ctx, calcParams3, nil) // Add ctx
		if err3 == nil {
			if result3.IsError != nil && *result3.IsError {
				log.Printf("Add(missing arg) failed as expected (IsError=true): Content=%+v", result3.Content)
				if len(result3.Content) > 0 {
					if textContent, ok := result3.Content[0].(protocol.TextContent); ok {
						if !strings.Contains(textContent.Text, "Invalid or missing") { // Check error message from handler
							log.Printf("WARNING: Add(missing arg) error message unexpected: %s", textContent.Text)
						}
					}
				}
			} else {
				log.Printf("WARNING: Add(missing arg) should have failed (IsError=true), but succeeded with result: %+v", result3)
			}
		} else {
			log.Printf("ERROR: Add(missing arg) failed with protocol error: %v", err3)
		}
	} else {
		log.Println("Could not find 'add' tool definition from server.")
	}
	// --- End Use Calculator Tool ---

	// --- Use the GetTinyImage Tool ---
	getTinyImageToolFound := false
	for _, tool := range tools {
		if tool.Name == "getTinyImage" {
			getTinyImageToolFound = true
			break
		}
	}
	if getTinyImageToolFound {
		log.Println("\n--- Testing GetTinyImage Tool ---")
		callParams := protocol.CallToolParams{Name: "getTinyImage"}
		result, err := clt.CallTool(ctx, callParams, nil) // Add ctx
		if err != nil {
			log.Printf("ERROR: Failed to use 'getTinyImage' tool: %v", err)
		} else {
			log.Printf("Successfully used 'getTinyImage' tool.")
			log.Printf("  Received Content Count: %d", len(result.Content))
			foundImage := false
			for i, contentItem := range result.Content {
				log.Printf("  Content[%d]: Type=%T, Value=%+v", i, contentItem, contentItem)
				if _, ok := contentItem.(protocol.ImageContent); ok {
					foundImage = true
				}
			}
			if !foundImage {
				log.Println("WARNING: Did not find ImageContent in getTinyImage result.")
			}
		}
	} else {
		log.Println("Could not find 'getTinyImage' tool definition from server.")
	}
	// --- End Use GetTinyImage Tool ---

	// --- Use the LongRunning Tool ---
	longRunningToolFound := false
	for _, tool := range tools {
		if tool.Name == "longRunningOperation" {
			longRunningToolFound = true
			break
		}
	}
	if longRunningToolFound {
		log.Println("\n--- Testing LongRunning Tool (No Progress Requested) ---")
		lrArgs := map[string]interface{}{"duration": 2.0, "steps": 4.0} // Shorter duration for test
		lrParams := protocol.CallToolParams{Name: "longRunningOperation", Arguments: lrArgs}
		lrResult, lrErr := clt.CallTool(ctx, lrParams, nil) // Add ctx, No progress token
		if lrErr != nil {
			log.Printf("ERROR: Failed to use 'longRunningOperation' tool: %v", lrErr)
		} else {
			log.Printf("LongRunning tool finished successfully.")
			log.Printf("  Result Content: %+v", lrResult.Content)
		}

		// TODO: Add example requesting progress (requires client handling progress notifications)
		// log.Println("\n--- Testing LongRunning Tool (With Progress Requested) ---")
		// progressToken := protocol.ProgressToken(uuid.NewString())
		// lrParamsWithProgress := protocol.CallToolParams{
		// 	Name:      "longRunningOperation",
		// 	Arguments: lrArgs,
		// 	Meta:      &protocol.RequestMeta{ProgressToken: &progressToken},
		// }
		// // Need to register a progress handler before calling
		// // clt.RegisterNotificationHandler(protocol.MethodProgress, func(ctx context.Context, params interface{}) error { ... })
		// lrResultProg, lrErrProg := clt.CallTool(ctx, lrParamsWithProgress, &progressToken) // Pass token pointer? Check signature
		// ... handle result and progress notifications ...

	} else {
		log.Println("Could not find 'longRunningOperation' tool definition from server.")
	}
	// --- End Use LongRunning Tool ---

	log.Println("Client operations finished.")
	return nil // Indicate success
}

// --- Main Function ---
// Sets up logging and runs the client logic.
func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Client...")

	clientName := "GoExampleClient-Refactored"
	// Create client assuming kitchen-sink server runs on 8080
	clt, err := client.NewClient(clientName, client.ClientOptions{
		ServerBaseURL: "http://127.0.0.1:8080",
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second) // Longer timeout for tests
	defer cancel()

	// Run the main client logic
	err = runClientLogic(ctx, clt) // Pass client instance and ctx
	if err != nil {
		// Log fatal error from the client logic run
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}
