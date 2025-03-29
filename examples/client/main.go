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

// requestToolDefinitions sends a ToolDefinitionRequest and processes the response.
// It returns the list of tools defined by the server or an error.
func requestToolDefinitions(conn *mcp.Connection) ([]mcp.ToolDefinition, error) {
	log.Println("Sending ToolDefinitionRequest...")
	reqPayload := mcp.ToolDefinitionRequestPayload{} // Payload is empty for this request type
	err := conn.SendMessage(mcp.MessageTypeToolDefinitionRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send ToolDefinitionRequest: %w", err)
	}

	log.Println("Waiting for ToolDefinitionResponse...")
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
		return nil, fmt.Errorf("timeout waiting for ToolDefinitionResponse")
	}

	// Check for errors during receive
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive ToolDefinitionResponse: %w", receiveErr)
	}

	// Check if the server sent back an MCP Error message
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("received MCP Error: [%d] %s", errPayload.Code, errPayload.Message) // Corrected
		}
		// If unmarshalling the error payload itself fails
		return nil, fmt.Errorf("received MCP Error with unparsable payload")
	}

	// Ensure the received message is the expected type
	if responseMsg.MessageType != mcp.MessageTypeToolDefinitionResponse {
		return nil, fmt.Errorf("expected ToolDefinitionResponse, got %s", responseMsg.MessageType)
	}

	// Unmarshal the actual payload
	var responsePayload mcp.ToolDefinitionResponsePayload
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ToolDefinitionResponse payload: %w", err)
	}

	// Return the list of tools
	return responsePayload.Tools, nil
}

// useTool sends a UseToolRequest for the specified tool and arguments,
// then processes the response.
// It returns the tool's result (as interface{}) or an error.
func useTool(conn *mcp.Connection, toolName string, args map[string]interface{}) (interface{}, error) {
	log.Printf("Sending UseToolRequest for tool '%s'...", toolName)
	reqPayload := mcp.UseToolRequestPayload{
		ToolName:  toolName,
		Arguments: args,
	}
	err := conn.SendMessage(mcp.MessageTypeUseToolRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send UseToolRequest for '%s': %w", toolName, err)
	}

	log.Println("Waiting for UseToolResponse...")
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
		return nil, fmt.Errorf("timeout waiting for UseToolResponse for '%s'", toolName)
	}

	// Check for errors during receive
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive UseToolResponse for '%s': %w", toolName, receiveErr)
	}

	// Check if the server sent back an MCP Error message
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			// Return a specific error indicating the tool use failed with an MCP error
			return nil, fmt.Errorf("tool '%s' failed with MCP Error: [%d] %s", toolName, errPayload.Code, errPayload.Message) // Corrected
		}
		return nil, fmt.Errorf("tool '%s' failed with an unparsable MCP Error payload", toolName)
	}

	// Ensure the received message is the expected type
	if responseMsg.MessageType != mcp.MessageTypeUseToolResponse {
		return nil, fmt.Errorf("expected UseToolResponse for '%s', got %s", toolName, responseMsg.MessageType)
	}

	// Unmarshal the actual payload
	var responsePayload mcp.UseToolResponsePayload
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal UseToolResponse payload for '%s': %w", toolName, err)
	}

	// Return the result from the tool execution
	return responsePayload.Result, nil
}

// runClientLogic performs the handshake and executes the example tool calls sequence.
// Returns an error if any fatal step fails (handshake, getting tool defs).
// Tool usage errors are logged but do not cause this function to return an error.
func runClientLogic(conn *mcp.Connection, clientName string) error {
	// --- Perform Handshake ---
	log.Println("Sending HandshakeRequest...")
	handshakeReqPayload := mcp.HandshakeRequestPayload{
		SupportedProtocolVersions: []string{mcp.CurrentProtocolVersion},
		ClientName:                clientName,
	}
	err := conn.SendMessage(mcp.MessageTypeHandshakeRequest, handshakeReqPayload)
	if err != nil {
		return fmt.Errorf("failed to send HandshakeRequest: %w", err)
	}

	log.Println("Waiting for HandshakeResponse...")
	msg, err := conn.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive HandshakeResponse: %w", err)
	}
	if msg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		_ = mcp.UnmarshalPayload(msg.Payload, &errPayload)
		return fmt.Errorf("handshake failed with MCP Error: [%d] %s", errPayload.Code, errPayload.Message) // Use %d for int code
	}
	if msg.MessageType != mcp.MessageTypeHandshakeResponse {
		return fmt.Errorf("expected HandshakeResponse, got %s", msg.MessageType)
	}
	var handshakeRespPayload mcp.HandshakeResponsePayload
	err = mcp.UnmarshalPayload(msg.Payload, &handshakeRespPayload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal HandshakeResponse payload: %w", err)
	}
	if handshakeRespPayload.SelectedProtocolVersion != mcp.CurrentProtocolVersion {
		return fmt.Errorf("server selected unsupported protocol version: %s", handshakeRespPayload.SelectedProtocolVersion)
	}
	log.Printf("Handshake successful with server: %s", handshakeRespPayload.ServerName)
	// --- End Handshake ---

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
			log.Printf("  Received: %v (Type: %T)", result, result)
			resultStr, ok := result.(string)
			if !ok {
				log.Printf("WARNING: Echo result was not a string!")
			} else if resultStr != echoMessage {
				log.Printf("WARNING: Echo result '%s' did not match sent message '%s'", resultStr, echoMessage)
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
			log.Printf("Calculator(add) Result: %v (Type: %T)", result1, result1)
			if resNum, ok := result1.(float64); !ok || resNum != 12.0 {
				log.Printf("WARNING: Calculator(add) result unexpected: %v", result1)
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
			log.Printf("Filesystem(read) Result: [content length=%d]", len(fmt.Sprintf("%v", fsResult3)))
			if resultMap, ok := fsResult3.(map[string]interface{}); ok {
				if content, ok := resultMap["content"].(string); ok {
					if content != testFileContent {
						log.Printf("WARNING: Filesystem read content mismatch!")
					} else {
						log.Println("  Read content matches written content.")
					}
				} else {
					log.Printf("WARNING: Filesystem read result content is not a string: %T", resultMap["content"])
				}
			} else {
				log.Printf("WARNING: Filesystem read result is not a map: %T", fsResult3)
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
