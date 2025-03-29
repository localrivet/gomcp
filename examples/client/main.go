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

// requestToolDefinitions sends a ListToolsRequest and processes the response.
// It returns the list of tools defined by the server or an error.
func requestToolDefinitions(conn *mcp.Connection) ([]mcp.Tool, error) { // Return []mcp.Tool
	log.Println("Sending ListToolsRequest...")
	reqPayload := mcp.ListToolsRequestParams{}               // Use new params struct (empty for now)
	err := conn.SendMessage(mcp.MethodListTools, reqPayload) // Use new method name
	if err != nil {
		return nil, fmt.Errorf("failed to send ListToolsRequest: %w", err)
	}

	log.Println("Waiting for ListToolsResponse...")
	// Use a timeout mechanism for receiving the response to prevent hangs.
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() {
		defer close(done) // Ensure channel is closed even on panic
		responseMsg, receiveErr = conn.ReceiveMessage()
	}()

	select {
	case <-done:
		// Received message or error within the timeout
	case <-time.After(5 * time.Second): // 5-second timeout
		return nil, fmt.Errorf("timeout waiting for ListToolsResponse") // Update error message
	}

	// Check for errors during receive
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive ListToolsResponse: %w", receiveErr) // Update error message
	}

	// Check if the server sent back an MCP Error message
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("received MCP Error: [%d] %s", errPayload.Code, errPayload.Message)
		}
		return nil, fmt.Errorf("received MCP Error with unparsable payload")
	}

	// Ensure the received message is the expected type (conceptual for now)
	// TODO: Update this check when transport handles JSON-RPC responses properly
	if responseMsg.MessageType != "ListToolsResponse" {
		return nil, fmt.Errorf("expected ListToolsResponse, got %s", responseMsg.MessageType)
	}

	// Unmarshal the actual payload (which should be ListToolsResult)
	var responsePayload mcp.ListToolsResult // Use new result struct
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListToolsResult payload: %w", err) // Update error message
	}

	// Return the list of tools
	// TODO: Handle pagination (responsePayload.NextCursor)
	return responsePayload.Tools, nil
}

// useTool sends a CallToolRequest for the specified tool and arguments,
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

// runClientLogic performs the handshake and executes the example tool calls sequence.
// Returns an error if any fatal step fails (handshake, getting tool defs).
// Tool usage errors are logged but do not cause this function to return an error.
func runClientLogic(conn *mcp.Connection, clientName string) error {
	// --- Perform Initialization ---
	log.Println("Sending InitializeRequest...")
	clientCapabilities := mcp.ClientCapabilities{}
	clientInfo := mcp.Implementation{Name: clientName, Version: "0.1.0"}
	initReqParams := mcp.InitializeRequestParams{
		ProtocolVersion: mcp.CurrentProtocolVersion,
		Capabilities:    clientCapabilities,
		ClientInfo:      clientInfo,
	}
	err := conn.SendMessage(mcp.MethodInitialize, initReqParams)
	if err != nil {
		return fmt.Errorf("failed to send InitializeRequest: %w", err)
	}

	log.Println("Waiting for InitializeResponse...")
	msg, err := conn.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}
	if msg.MessageType == mcp.MessageTypeError { // Assuming errors still use MessageTypeError for now
		var errPayload mcp.ErrorPayload
		_ = mcp.UnmarshalPayload(msg.Payload, &errPayload) // Error handling simplified for brevity
		return fmt.Errorf("initialize failed with MCP Error: [%d] %s", errPayload.Code, errPayload.Message)
	}
	// TODO: Improve response type checking based on JSON-RPC structure
	log.Printf("Received potential InitializeResponse message (Payload Type: %T)", msg.Payload)

	var initResult mcp.InitializeResult
	err = mcp.UnmarshalPayload(msg.Payload, &initResult) // Assumes payload is InitializeResult
	if err != nil {
		return fmt.Errorf("failed to unmarshal InitializeResult payload: %w", err)
	}
	if initResult.ProtocolVersion != mcp.CurrentProtocolVersion {
		return fmt.Errorf("server selected unsupported protocol version: %s", initResult.ProtocolVersion)
	}
	serverName := initResult.ServerInfo.Name // Store server name locally if needed
	log.Printf("Initialization successful with server: %s", serverName)

	// Send Initialized Notification
	log.Println("Sending InitializedNotification...")
	initParams := mcp.InitializedNotificationParams{}
	err = conn.SendMessage(mcp.MethodInitialized, initParams)
	if err != nil {
		log.Printf("Warning: failed to send InitializedNotification: %v", err)
	}
	// --- End Initialization ---

	// --- Request Tool Definitions ---
	tools, err := requestToolDefinitions(conn)
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
		result, err := useTool(conn, "echo", args)
		if err != nil {
			log.Printf("ERROR: Failed to use 'echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result) // Log the content slice
			// Extract text from the first TextContent element
			if len(result) > 0 {
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
		result1, err1 := useTool(conn, "calculator", calcArgs1)
		if err1 != nil {
			log.Printf("ERROR: Failed to use 'calculator' tool (add): %v", err1)
		} else {
			log.Printf("Calculator(add) Content: %+v", result1)
			// Extract text, parse as float
			if len(result1) > 0 {
				if textContent, ok := result1[0].(mcp.TextContent); ok { // Use type assertion on result1[0]
					var resNum float64
					if _, err := fmt.Sscan(textContent.Text, &resNum); err != nil || resNum != 12.0 {
						log.Printf("WARNING: Calculator(add) result unexpected or failed parse: %s", textContent.Text)
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
		_, err2 := useTool(conn, "calculator", calcArgs2)
		if err2 == nil {
			log.Printf("WARNING: Calculator(divide by zero) should have failed, but succeeded.")
		} else {
			log.Printf("Calculator(divide by zero) failed as expected: %v", err2)
			if !strings.Contains(err2.Error(), "Division by zero") && !strings.Contains(err2.Error(), "CalculationError") {
				log.Printf("WARNING: Calculator(divide by zero) error message unexpected: %v", err2)
			}
		}
		// Example 3: Missing argument
		calcArgs3 := map[string]interface{}{"operand1": 10.0, "operation": "multiply"}
		_, err3 := useTool(conn, "calculator", calcArgs3)
		if err3 == nil {
			log.Printf("WARNING: Calculator(missing arg) should have failed, but succeeded.")
		} else {
			log.Printf("Calculator(missing arg) failed as expected: %v", err3)
			if !strings.Contains(err3.Error(), "InvalidArgument") && !strings.Contains(err3.Error(), "Missing required arguments") {
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
		fsResult1, fsErr1 := useTool(conn, fsToolName, fsArgs1)
		if fsErr1 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (list_files .): %v", fsToolName, fsErr1)
		} else {
			log.Printf("Filesystem(list .) Result: %v", fsResult1)
		}
		// Example 2: Write file
		fsArgs2 := map[string]interface{}{"operation": "write_file", "path": testFilePath, "content": testFileContent}
		fsResult2, fsErr2 := useTool(conn, fsToolName, fsArgs2)
		if fsErr2 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (write_file): %v", fsToolName, fsErr2)
		} else {
			log.Printf("Filesystem(write) Result: %v", fsResult2)
		}
		// Example 3: Read file back
		fsArgs3 := map[string]interface{}{"operation": "read_file", "path": testFilePath}
		fsResult3, fsErr3 := useTool(conn, fsToolName, fsArgs3)
		if fsErr3 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (read_file): %v", fsToolName, fsErr3)
		} else {
			log.Printf("Filesystem(read) Content: %+v", fsResult3)
			// Extract text, unmarshal JSON, check content field
			if len(fsResult3) > 0 {
				if textContent, ok := fsResult3[0].(mcp.TextContent); ok { // Use type assertion on fsResult3[0]
					var resultMap map[string]interface{}
					if err := json.Unmarshal([]byte(textContent.Text), &resultMap); err != nil {
						log.Printf("WARNING: Filesystem read result failed to unmarshal JSON: %v", err)
					} else if content, ok := resultMap["content"].(string); ok {
						log.Printf("  Extracted Content Length: %d", len(content))
						if content != testFileContent {
							log.Printf("WARNING: Filesystem read content mismatch!")
						} else {
							log.Println("  Read content matches written content.")
						}
					} else {
						log.Printf("WARNING: Filesystem read result JSON missing 'content' string field.")
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
		fsResult4, fsErr4 := useTool(conn, fsToolName, fsArgs4)
		if fsErr4 != nil {
			log.Printf("ERROR: Failed to use '%s' tool (list_files test_dir): %v", fsToolName, fsErr4)
		} else {
			log.Printf("Filesystem(list test_dir) Result: %v", fsResult4)
		}
		// Example 5: Read non-existent
		fsArgs5 := map[string]interface{}{"operation": "read_file", "path": "non_existent_file.txt"}
		_, fsErr5 := useTool(conn, fsToolName, fsArgs5)
		if fsErr5 == nil {
			log.Printf("WARNING: Filesystem(read non-existent) should have failed.")
		} else {
			log.Printf("Filesystem(read non-existent) failed as expected: %v", fsErr5)
			if !strings.Contains(fsErr5.Error(), "NotFound") && !strings.Contains(fsErr5.Error(), "not found") {
				log.Printf("WARNING: Filesystem(read non-existent) error message unexpected: %v", fsErr5)
			}
		}
		// Example 6: Write outside sandbox
		fsArgs6 := map[string]interface{}{"operation": "write_file", "path": "../outside_sandbox.txt", "content": "attempt escape"}
		_, fsErr6 := useTool(conn, fsToolName, fsArgs6)
		if fsErr6 == nil {
			log.Printf("WARNING: Filesystem(write outside) should have failed.")
		} else {
			log.Printf("Filesystem(write outside) failed as expected: %v", fsErr6)
			if !strings.Contains(fsErr6.Error(), "SecurityViolation") && !strings.Contains(fsErr6.Error(), "escape the sandbox") {
				log.Printf("WARNING: Filesystem(write outside) error message unexpected: %v", fsErr6)
			}
		}
	} else {
		log.Println("Could not find 'filesystem' tool definition from server.")
	}
	// --- End Use Filesystem Tool ---

	log.Println("Client finished.")
	return nil // Indicate success
}

// --- Main Function ---
// Sets up logging and stdio connection, then runs the client logic.
func main() {
	// Log informational messages to stderr so stdout can be used purely for MCP messages.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Client...")

	clientName := "GoExampleClient"
	// Create a connection using standard input/output
	conn := mcp.NewStdioConnection()

	// Run the main client logic
	err := runClientLogic(conn, clientName)
	if err != nil {
		// Log fatal error from the client logic run
		log.Fatalf("Client exited with error: %v", err)
	}
	// No explicit close needed for stdio connection typically
}
