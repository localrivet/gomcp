// This is the main file for the example MCP client.
// It demonstrates how to:
// 1. Connect to an MCP server using the gomcp library.
// 2. Perform the MCP handshake.
// 3. Request tool definitions from the server.
// 4. Use the available tools (echo, calculator, filesystem) with various arguments.
// 5. Handle successful responses and expected error conditions.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mcp "github.com/localrivet/gomcp" // Import root package
)

// --- Helper Functions ---

// requestToolDefinitions uses the client to request tool definitions.
// It returns the list of tools defined by the server or an error.
func requestToolDefinitions(client *mcp.Client) ([]mcp.Tool, error) {
	log.Println("Sending ListToolsRequest...")
	// Params struct is empty for now, no filtering/pagination implemented in this example
	params := mcp.ListToolsRequestParams{}
	result, err := client.ListTools(params)
	if err != nil {
		// Error could be a transport error or an MCP error response
		return nil, fmt.Errorf("ListTools failed: %w", err)
	}

	// TODO: Handle pagination if result.NextCursor is not empty

	log.Printf("Received %d tool definitions", len(result.Tools))
	return result.Tools, nil
}

// useTool sends a CallToolRequest using the client and processes the response.
// then processes the response.
// It returns the result Content slice or an error.
// TODO: Refine error handling and return value based on CallToolResult structure.
func useTool(conn *mcp.Connection, toolName string, args map[string]interface{}) ([]mcp.Content, error) { // Return []Content
	log.Printf("Sending CallToolRequest for tool '%s'...", toolName)
	reqPayload := mcp.CallToolParams{ // Use new params struct
		Name:      toolName, // Use 'Name' field
		Arguments: args,
	}
	err := conn.SendMessage(mcp.MethodCallTool, reqPayload) // Use new method name
	if err != nil {
		return nil, fmt.Errorf("failed to send CallToolRequest for '%s': %w", toolName, err)
	}

	log.Println("Waiting for CallToolResponse...")
	// Use a timeout mechanism for receiving the response.
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() {
		defer close(done) // Ensure channel is closed
		responseMsg, receiveErr = conn.ReceiveMessage()
	}()

	select {
	case <-done:
		// Received message or error within the timeout
	case <-time.After(5 * time.Second): // 5-second timeout
		return nil, fmt.Errorf("timeout waiting for CallToolResponse for '%s'", toolName) // Update error message
	}

	// Check for errors during receive
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive CallToolResponse for '%s': %w", toolName, receiveErr) // Update error message
	}

	// Check if the server sent back an MCP Error message
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("tool '%s' failed with MCP Error: [%d] %s", toolName, errPayload.Code, errPayload.Message)
		}
		return nil, fmt.Errorf("tool '%s' failed with an unparsable MCP Error payload", toolName)
	}

	// Ensure the received message is the expected type (conceptual for now)
	// TODO: Update this check when transport handles JSON-RPC responses properly
	if responseMsg.MessageType != "CallToolResponse" {
		return nil, fmt.Errorf("expected CallToolResponse for '%s', got %s", toolName, responseMsg.MessageType)
	}

	// Unmarshal the actual payload (which should be CallToolResult)
	var responsePayload mcp.CallToolResult // Use new result struct
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CallToolResult payload for '%s': %w", toolName, err) // Update error message
	}

	// Check if the result itself indicates an error
	if responsePayload.IsError != nil && *responsePayload.IsError {
		// Extract error message from content (assuming first text content)
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", toolName)
		if len(responsePayload.Content) > 0 {
			if textContent, ok := responsePayload.Content[0].(mcp.TextContent); ok {
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", toolName, textContent.Text)
			} else {
				// Handle cases where error content isn't simple text if necessary
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", toolName, responsePayload.Content[0])
			}
		}
		return responsePayload.Content, fmt.Errorf("%s", errMsg) // Return content and an error, use %s
	}

	// Return the successful result content
	return responsePayload.Content, nil
}

// runClientLogic creates a client, connects, and executes the example tool calls sequence.
// Returns an error if any fatal step fails (connection, getting tool defs).
// Tool usage errors are logged but do not cause this function to return an error.
func runClientLogic(clientName string) error {
	// Create a new client instance
	client := mcp.NewClient(clientName)

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
		// Treat failure to get definitions as fatal for this example client
		return fmt.Errorf("failed to get tool definitions: %w", err)
	}
	log.Printf("Received %d tool definitions:", len(tools))
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
		result, err := useTool(client, "echo", args) // Pass client instance
		if err != nil {
			log.Printf("ERROR: Failed to use 'echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result) // Log the content slice
			// Extract text from the first TextContent element
			if len(result) > 0 {
				// Use type assertion on the Content interface
				if textContent, ok := result[0].(mcp.TextContent); ok {
					log.Printf("  Extracted Text: %s", textContent.Text)
					if textContent.Text != echoMessage {
						log.Printf("WARNING: Echo result '%s' did not match sent message '%s'", textContent.Text, echoMessage)
					}
				} else {
					log.Printf("WARNING: Echo result content[0] was not TextContent: %T", result[0])
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
		result1, err1 := useTool(client, "calculator", calcArgs1) // Pass client instance
		if err1 != nil {
			log.Printf("ERROR: Failed to use 'calculator' tool (add): %v", err1)
		} else {
			log.Printf("Calculator(add) Content: %+v", result1)
			// Extract text, parse as float
			if len(result1) > 0 {
				if textContent, ok := result1[0].(mcp.TextContent); ok { // Use type assertion on result1[0]
					var resNum float64
					// Use fmt.Sscanf for safer parsing
					if _, err := fmt.Sscanf(textContent.Text, "%f", &resNum); err != nil || resNum != 12.0 {
						log.Printf("WARNING: Calculator(add) result unexpected or failed parse: %s (error: %v)", textContent.Text, err)
					} else {
						log.Printf("  Parsed Result: %f", resNum)
					}
				} else {
					log.Printf("WARNING: Calculator(add) result content[0] was not TextContent: %T", result1[0])
				}
			} else {
				log.Printf("WARNING: Calculator(add) result content was empty!")
			}
		}
		// Example 2: Divide by zero
		calcArgs2 := map[string]interface{}{"operand1": 10.0, "operand2": 0.0, "operation": "divide"}
		_, err2 := useTool(client, "calculator", calcArgs2) // Pass client instance
		if err2 == nil {
			log.Printf("WARNING: Calculator(divide by zero) should have failed, but succeeded.")
		} else {
			log.Printf("Calculator(divide by zero) failed as expected: %v", err2)
			// Check if the error message indicates a tool execution error
			if !strings.Contains(err2.Error(), "Tool 'calculator' failed") {
				log.Printf("WARNING: Calculator(divide by zero) error message unexpected: %v", err2)
			}
		}
		// Example 3: Missing argument
		calcArgs3 := map[string]interface{}{"operand1": 10.0, "operation": "multiply"}
		_, err3 := useTool(client, "calculator", calcArgs3) // Pass client instance
		if err3 == nil {
			log.Printf("WARNING: Calculator(missing arg) should have failed, but succeeded.")
		} else {
			log.Printf("Calculator(missing arg) failed as expected: %v", err3)
			// Check if the error message indicates a tool execution error
			if !strings.Contains(err3.Error(), "Tool 'calculator' failed") {
				log.Printf("WARNING: Calculator(missing arg) error message unexpected: %v", err3)
			}
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
		testFilePath := "test_dir/my_file.txt"
		testFileContent := "This is the content of the test file.\nIt has multiple lines."
		// Example 1: List root
		fsArgs1 := map[string]interface{}{"operation": "list_files", "path": "."}
		fsResult1, fsErr1 := useTool(client, fsToolName, fsArgs1) // Pass client instance
		if fsErr1 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (list_files .): %v", fsToolName, fsErr1)
		} else {
			log.Printf("Filesystem(list .) Result Content: %+v", fsResult1)
			// Assuming the result is JSON text, try to unmarshal and print nicely
			if len(fsResult1) > 0 {
				if textContent, ok := fsResult1[0].(mcp.TextContent); ok {
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
		fsResult2, fsErr2 := useTool(client, fsToolName, fsArgs2) // Pass client instance
		if fsErr2 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (write_file): %v", fsToolName, fsErr2)
		} else {
			log.Printf("Filesystem(write) Result Content: %+v", fsResult2)
		}
		// Example 3: Read file back
		fsArgs3 := map[string]interface{}{"operation": "read_file", "path": testFilePath}
		fsResult3, fsErr3 := useTool(client, fsToolName, fsArgs3) // Pass client instance
		if fsErr3 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (read_file): %v", fsToolName, fsErr3)
		} else {
			log.Printf("Filesystem(read) Content: %+v", fsResult3)
			// Extract text content
			if len(fsResult3) > 0 {
				if textContent, ok := fsResult3[0].(mcp.TextContent); ok {
					log.Printf("  Extracted Content: %q", textContent.Text) // Quote to see newlines
					if textContent.Text != testFileContent {
						log.Printf("WARNING: Filesystem read content mismatch!")
					} else {
						log.Println("  Read content matches written content.")
					}
				} else {
					log.Printf("WARNING: Filesystem read result content[0] was not TextContent: %T", fsResult3[0])
				}
			} else {
				log.Printf("WARNING: Filesystem read result content was empty!")
			}
		}
		// Example 4: List dir
		fsArgs4 := map[string]interface{}{"operation": "list_files", "path": "test_dir"}
		fsResult4, fsErr4 := useTool(client, fsToolName, fsArgs4) // Pass client instance
		if fsErr4 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (list_files test_dir): %v", fsToolName, fsErr4)
		} else {
			log.Printf("Filesystem(list test_dir) Result Content: %+v", fsResult4)
			// Assuming the result is JSON text, try to unmarshal and print nicely
			if len(fsResult4) > 0 {
				if textContent, ok := fsResult4[0].(mcp.TextContent); ok {
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
		// Example 5: Read non-existent
		fsArgs5 := map[string]interface{}{"operation": "read_file", "path": "non_existent_file.txt"}
		_, fsErr5 := useTool(client, fsToolName, fsArgs5) // Pass client instance
		if fsErr5 == nil {
			log.Printf("WARNING: Filesystem(read non-existent) should have failed.")
		} else {
			log.Printf("Filesystem(read non-existent) failed as expected: %v", fsErr5)
			if !strings.Contains(fsErr5.Error(), "not found") { // Check for specific error text
				log.Printf("WARNING: Filesystem(read non-existent) error message unexpected: %v", fsErr5)
			}
		}
		// Example 6: Write outside sandbox
		fsArgs6 := map[string]interface{}{"operation": "write_file", "path": "../outside_sandbox.txt", "content": "attempt escape"}
		_, fsErr6 := useTool(client, fsToolName, fsArgs6) // Pass client instance
		if fsErr6 == nil {
			log.Printf("WARNING: Filesystem(write outside) should have failed.")
		} else {
			log.Printf("Filesystem(write outside) failed as expected: %v", fsErr6)
			if !strings.Contains(fsErr6.Error(), "escape the sandbox") { // Check for specific error text
				log.Printf("WARNING: Filesystem(write outside) error message unexpected: %v", fsErr6)
			}
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

	// Run the main client logic
	err := runClientLogic(clientName) // No longer pass connection
	if err != nil {
		// Log fatal error from the client logic run
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}
