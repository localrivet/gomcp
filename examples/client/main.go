package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mcp "github.com/localrivet/gomcp"
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
func runClientLogic(client *mcp.Client) error { // Accept *mcp.Client
	// Connect and perform initialization
	log.Println("Connecting to server...")
	err := client.Connect() // Use the provided client
	if err != nil {
		return fmt.Errorf("client failed to connect: %w", err)
	}
	defer client.Close() // Ensure connection is closed eventually

	// Access server info and capabilities using the new getters
	serverInfo := client.ServerInfo()
	serverCaps := client.ServerCapabilities()
	log.Printf("Client connected successfully to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)
	log.Printf("Server Capabilities: %+v", serverCaps)

	// --- Request Tool Definitions ---
	log.Println("\n--- Requesting Tool Definitions ---")
	listParams := mcp.ListToolsRequestParams{} // No pagination/filtering in this example
	toolsResult, err := client.ListTools(listParams)
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
		callParams := mcp.CallToolParams{Name: "echo", Arguments: args}
		// Call tool without requesting progress
		result, err := client.CallTool(callParams, nil)
		if err != nil {
			log.Printf("ERROR: Failed to use 'echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result.Content) // Log the content slice
			// Extract text from the first TextContent element
			if len(result.Content) > 0 {
				// Use type assertion on the Content interface
				if textContent, ok := result.Content[0].(mcp.TextContent); ok {
					log.Printf("  Extracted Text: %s", textContent.Text)
					if textContent.Text != echoMessage {
						log.Printf("WARNING: Echo result '%s' did not match sent message '%s'", textContent.Text, echoMessage)
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

	// --- Use the Calculator Tool ---
	calculatorToolFound := false
	for _, tool := range tools {
		if tool.Name == "calculator" {
			calculatorToolFound = true
			break
		}
	}
	if calculatorToolFound {
		log.Println("\n--- Testing Calculator Tool ---")
		// Example 1: Add
		calcArgs1 := map[string]interface{}{"operand1": 5.0, "operand2": 7.0, "operation": "add"}
		calcParams1 := mcp.CallToolParams{Name: "calculator", Arguments: calcArgs1}
		result1, err1 := client.CallTool(calcParams1, nil)
		if err1 != nil {
			log.Printf("ERROR: Failed to use 'calculator' tool (add): %v", err1)
		} else {
			log.Printf("Calculator(add) Content: %+v", result1.Content)
			// Extract text, parse as float
			if len(result1.Content) > 0 {
				if textContent, ok := result1.Content[0].(mcp.TextContent); ok {
					var resNum float64
					// Use fmt.Sscanf for safer parsing
					if _, err := fmt.Sscanf(textContent.Text, "%f", &resNum); err != nil || resNum != 12.0 {
						log.Printf("WARNING: Calculator(add) result unexpected or failed parse: %s (error: %v)", textContent.Text, err)
					} else {
						log.Printf("  Parsed Result: %f", resNum)
					}
				} else {
					log.Printf("WARNING: Calculator(add) result content[0] was not TextContent: %T", result1.Content[0])
				}
			} else {
				log.Printf("WARNING: Calculator(add) result content was empty!")
			}
		}
		// Example 2: Divide by zero (expecting error in result)
		calcArgs2 := map[string]interface{}{"operand1": 10.0, "operand2": 0.0, "operation": "divide"}
		calcParams2 := mcp.CallToolParams{Name: "calculator", Arguments: calcArgs2}
		result2, err2 := client.CallTool(calcParams2, nil)
		if err2 == nil {
			// Check IsError flag in the result
			if result2.IsError != nil && *result2.IsError {
				log.Printf("Calculator(divide by zero) failed as expected (IsError=true): Content=%+v", result2.Content)
				// Optionally check the error message in content
				if len(result2.Content) > 0 {
					if textContent, ok := result2.Content[0].(mcp.TextContent); ok {
						if !strings.Contains(textContent.Text, "Division by zero") {
							log.Printf("WARNING: Calculator(divide by zero) error message unexpected: %s", textContent.Text)
						}
					}
				}
			} else {
				log.Printf("WARNING: Calculator(divide by zero) should have failed (IsError=true), but succeeded with result: %+v", result2)
			}
		} else {
			// Protocol level error (e.g., timeout, connection issue, server error response)
			log.Printf("ERROR: Calculator(divide by zero) failed with protocol error: %v", err2)
		}

		// Example 3: Missing argument (expecting error in result)
		calcArgs3 := map[string]interface{}{"operand1": 10.0, "operation": "multiply"}
		calcParams3 := mcp.CallToolParams{Name: "calculator", Arguments: calcArgs3}
		result3, err3 := client.CallTool(calcParams3, nil)
		if err3 == nil {
			if result3.IsError != nil && *result3.IsError {
				log.Printf("Calculator(missing arg) failed as expected (IsError=true): Content=%+v", result3.Content)
				if len(result3.Content) > 0 {
					if textContent, ok := result3.Content[0].(mcp.TextContent); ok {
						if !strings.Contains(textContent.Text, "Missing required arguments") {
							log.Printf("WARNING: Calculator(missing arg) error message unexpected: %s", textContent.Text)
						}
					}
				}
			} else {
				log.Printf("WARNING: Calculator(missing arg) should have failed (IsError=true), but succeeded with result: %+v", result3)
			}
		} else {
			log.Printf("ERROR: Calculator(missing arg) failed with protocol error: %v", err3)
		}
	} else {
		log.Println("Could not find 'calculator' tool definition from server.")
	}
	// --- End Use Calculator Tool ---

	// --- Use the Filesystem Tool ---
	filesystemToolFound := false
	for _, tool := range tools {
		if tool.Name == "filesystem" {
			filesystemToolFound = true
			break
		}
	}
	if filesystemToolFound {
		log.Println("\n--- Testing Filesystem Tool ---")
		fsToolName := "filesystem"
		testFilePath := "test_dir/my_file.txt" // Relative path for the server's sandbox
		testFileContent := "This is the content of the test file.\nIt has multiple lines."

		// Example 1: List root
		fsArgs1 := map[string]interface{}{"operation": "list_files", "path": "."}
		fsParams1 := mcp.CallToolParams{Name: fsToolName, Arguments: fsArgs1}
		fsResult1, fsErr1 := client.CallTool(fsParams1, nil)
		if fsErr1 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (list_files .): %v", fsToolName, fsErr1)
		} else {
			log.Printf("Filesystem(list .) Result Content: %+v", fsResult1.Content)
			// Assuming the result is JSON text, try to unmarshal and print nicely
			if len(fsResult1.Content) > 0 {
				if textContent, ok := fsResult1.Content[0].(mcp.TextContent); ok {
					var listData interface{}
					if err := json.Unmarshal([]byte(textContent.Text), &listData); err == nil {
						prettyJSON, _ := json.MarshalIndent(listData, "  ", "  ")
						log.Printf("  Formatted List: %s", string(prettyJSON))
					} else {
						log.Printf("  Raw Text: %s", textContent.Text)
					}
				}
			}
		}

		// Example 2: Write file
		fsArgs2 := map[string]interface{}{"operation": "write_file", "path": testFilePath, "content": testFileContent}
		fsParams2 := mcp.CallToolParams{Name: fsToolName, Arguments: fsArgs2}
		fsResult2, fsErr2 := client.CallTool(fsParams2, nil)
		if fsErr2 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (write_file): %v", fsToolName, fsErr2)
		} else {
			log.Printf("Filesystem(write) Result Content: %+v", fsResult2.Content)
		}

		// Example 3: Read file back
		fsArgs3 := map[string]interface{}{"operation": "read_file", "path": testFilePath}
		fsParams3 := mcp.CallToolParams{Name: fsToolName, Arguments: fsArgs3}
		fsResult3, fsErr3 := client.CallTool(fsParams3, nil)
		if fsErr3 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (read_file): %v", fsToolName, fsErr3)
		} else {
			log.Printf("Filesystem(read) Content: %+v", fsResult3.Content)
			// Extract text content
			if len(fsResult3.Content) > 0 {
				if textContent, ok := fsResult3.Content[0].(mcp.TextContent); ok {
					log.Printf("  Extracted Content: %q", textContent.Text) // Quote to see newlines
					if textContent.Text != testFileContent {
						log.Printf("WARNING: Filesystem read content mismatch!")
					} else {
						log.Println("  Read content matches written content.")
					}
				} else {
					log.Printf("WARNING: Filesystem read result content[0] was not TextContent: %T", fsResult3.Content[0])
				}
			} else {
				log.Printf("WARNING: Filesystem read result content was empty!")
			}
		}

		// Example 4: List dir
		fsArgs4 := map[string]interface{}{"operation": "list_files", "path": "test_dir"}
		fsParams4 := mcp.CallToolParams{Name: fsToolName, Arguments: fsArgs4}
		fsResult4, fsErr4 := client.CallTool(fsParams4, nil)
		if fsErr4 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (list_files test_dir): %v", fsToolName, fsErr4)
		} else {
			log.Printf("Filesystem(list test_dir) Result Content: %+v", fsResult4.Content)
			// Assuming the result is JSON text, try to unmarshal and print nicely
			if len(fsResult4.Content) > 0 {
				if textContent, ok := fsResult4.Content[0].(mcp.TextContent); ok {
					var listData interface{}
					if err := json.Unmarshal([]byte(textContent.Text), &listData); err == nil {
						prettyJSON, _ := json.MarshalIndent(listData, "  ", "  ")
						log.Printf("  Formatted List: %s", string(prettyJSON))
					} else {
						log.Printf("  Raw Text: %s", textContent.Text)
					}
				}
			}
		}

		// Example 5: Read non-existent (expecting tool error)
		fsArgs5 := map[string]interface{}{"operation": "read_file", "path": "non_existent_file.txt"}
		fsParams5 := mcp.CallToolParams{Name: fsToolName, Arguments: fsArgs5}
		result5, fsErr5 := client.CallTool(fsParams5, nil)
		if fsErr5 == nil {
			if result5.IsError != nil && *result5.IsError {
				log.Printf("Filesystem(read non-existent) failed as expected (IsError=true): Content=%+v", result5.Content)
				if len(result5.Content) > 0 {
					if textContent, ok := result5.Content[0].(mcp.TextContent); ok {
						if !strings.Contains(textContent.Text, "not found") {
							log.Printf("WARNING: Filesystem(read non-existent) error message unexpected: %s", textContent.Text)
						}
					}
				}
			} else {
				log.Printf("WARNING: Filesystem(read non-existent) should have failed (IsError=true), but succeeded with result: %+v", result5)
			}
		} else {
			log.Printf("ERROR: Filesystem(read non-existent) failed with protocol error: %v", fsErr5)
		}

		// Example 6: Write outside sandbox (expecting tool error)
		fsArgs6 := map[string]interface{}{"operation": "write_file", "path": "../outside_sandbox.txt", "content": "attempt escape"}
		fsParams6 := mcp.CallToolParams{Name: fsToolName, Arguments: fsArgs6}
		result6, fsErr6 := client.CallTool(fsParams6, nil)
		if fsErr6 == nil {
			if result6.IsError != nil && *result6.IsError {
				log.Printf("Filesystem(write outside) failed as expected (IsError=true): Content=%+v", result6.Content)
				if len(result6.Content) > 0 {
					if textContent, ok := result6.Content[0].(mcp.TextContent); ok {
						if !strings.Contains(textContent.Text, "escape the sandbox") {
							log.Printf("WARNING: Filesystem(write outside) error message unexpected: %s", textContent.Text)
						}
					}
				}
			} else {
				log.Printf("WARNING: Filesystem(write outside) should have failed (IsError=true), but succeeded with result: %+v", result6)
			}
		} else {
			log.Printf("ERROR: Filesystem(write outside) failed with protocol error: %v", fsErr6)
		}
	} else {
		log.Println("Could not find 'filesystem' tool definition from server.")
	}
	// --- End Use Filesystem Tool ---

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

// --- Main Function ---
// Sets up logging and runs the client logic.
func main() {
	// Log informational messages to stderr so stdout can be used purely for MCP messages.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Client...")

	clientName := "GoExampleClient-Refactored"
	client := mcp.NewClient(clientName) // Create client in main

	// Run the main client logic
	err := runClientLogic(client) // Pass client instance
	if err != nil {
		// Log fatal error from the client logic run
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}
