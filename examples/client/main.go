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

// Helper function to request tool definitions
func requestToolDefinitions(conn *mcp.Connection) ([]mcp.ToolDefinition, error) {
	log.Println("Sending ToolDefinitionRequest...")
	reqPayload := mcp.ToolDefinitionRequestPayload{} // Empty payload
	err := conn.SendMessage(mcp.MessageTypeToolDefinitionRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send ToolDefinitionRequest: %w", err)
	}

	log.Println("Waiting for ToolDefinitionResponse...")
	// Add a timeout for receiving the response
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() {
		responseMsg, receiveErr = conn.ReceiveMessage()
		close(done)
	}()

	select {
	case <-done:
		// Received message or error
	case <-time.After(5 * time.Second): // 5-second timeout
		return nil, fmt.Errorf("timeout waiting for ToolDefinitionResponse")
	}

	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive ToolDefinitionResponse: %w", receiveErr)
	}

	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("received MCP Error: [%s] %s", errPayload.Code, errPayload.Message)
		}
		return nil, fmt.Errorf("received MCP Error with unparsable payload")
	}

	if responseMsg.MessageType != mcp.MessageTypeToolDefinitionResponse {
		return nil, fmt.Errorf("expected ToolDefinitionResponse, got %s", responseMsg.MessageType)
	}

	var responsePayload mcp.ToolDefinitionResponsePayload
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ToolDefinitionResponse payload: %w", err)
	}

	return responsePayload.Tools, nil
}

// Helper function to use a tool
func useTool(conn *mcp.Connection, toolName string, args map[string]interface{}) (interface{}, error) {
	log.Printf("Sending UseToolRequest for tool '%s'...", toolName)
	reqPayload := mcp.UseToolRequestPayload{
		ToolName:  toolName,
		Arguments: args,
	}
	err := conn.SendMessage(mcp.MessageTypeUseToolRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send UseToolRequest: %w", err)
	}

	log.Println("Waiting for UseToolResponse...")
	// Add a timeout
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() {
		responseMsg, receiveErr = conn.ReceiveMessage()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for UseToolResponse")
	}

	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive UseToolResponse: %w", receiveErr)
	}

	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("received MCP Error: [%s] %s", errPayload.Code, errPayload.Message)
		}
		return nil, fmt.Errorf("received MCP Error with unparsable payload")
	}

	if responseMsg.MessageType != mcp.MessageTypeUseToolResponse {
		return nil, fmt.Errorf("expected UseToolResponse, got %s", responseMsg.MessageType)
	}

	var responsePayload mcp.UseToolResponsePayload
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal UseToolResponse payload: %w", err)
	}

	return responsePayload.Result, nil
}

func main() {
	// Log to stderr so stdout can be used purely for MCP messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Client...")

	clientName := "GoExampleClient"
	conn := mcp.NewStdioConnection() // Use stdio

	// --- Perform Handshake ---
	log.Println("Sending HandshakeRequest...")
	handshakeReqPayload := mcp.HandshakeRequestPayload{
		SupportedProtocolVersions: []string{mcp.CurrentProtocolVersion},
		ClientName:                clientName,
	}
	err := conn.SendMessage(mcp.MessageTypeHandshakeRequest, handshakeReqPayload)
	if err != nil {
		log.Fatalf("Failed to send HandshakeRequest: %v", err)
	}

	log.Println("Waiting for HandshakeResponse...")
	msg, err := conn.ReceiveMessage() // Assume blocking read is okay for simple example
	if err != nil {
		log.Fatalf("Failed to receive HandshakeResponse: %v", err)
	}
	if msg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		_ = mcp.UnmarshalPayload(msg.Payload, &errPayload) // Ignore error here
		log.Fatalf("Handshake failed with MCP Error: [%s] %s", errPayload.Code, errPayload.Message)
	}
	if msg.MessageType != mcp.MessageTypeHandshakeResponse {
		log.Fatalf("Expected HandshakeResponse, got %s", msg.MessageType)
	}
	var handshakeRespPayload mcp.HandshakeResponsePayload
	err = mcp.UnmarshalPayload(msg.Payload, &handshakeRespPayload)
	if err != nil {
		log.Fatalf("Failed to unmarshal HandshakeResponse payload: %v", err)
	}
	if handshakeRespPayload.SelectedProtocolVersion != mcp.CurrentProtocolVersion {
		log.Fatalf("Server selected unsupported protocol version: %s", handshakeRespPayload.SelectedProtocolVersion)
	}
	log.Printf("Handshake successful with server: %s", handshakeRespPayload.ServerName)
	// --- End Handshake ---

	// --- Request Tool Definitions ---
	tools, err := requestToolDefinitions(conn)
	if err != nil {
		log.Fatalf("Failed to get tool definitions: %v", err)
	}
	log.Printf("Received %d tool definitions:", len(tools))
	for _, tool := range tools {
		// Pretty print the tool definition using JSON marshal indent
		toolJson, _ := json.MarshalIndent(tool, "", "  ")
		fmt.Fprintf(os.Stderr, "%s\n", string(toolJson)) // Print to stderr
	}
	// --- End Request Tool Definitions ---

	// --- Use the Echo Tool ---
	if len(tools) > 0 && tools[0].Name == "echo" { // Assuming echo is the first/only tool
		echoMessage := "Hello from Go MCP Client!"
		args := map[string]interface{}{
			"message": echoMessage,
		}
		result, err := useTool(conn, "echo", args)
		if err != nil {
			log.Fatalf("Failed to use 'echo' tool: %v", err)
		}
		log.Printf("Successfully used 'echo' tool.")
		log.Printf("  Sent: %s", echoMessage)
		log.Printf("  Received: %v (Type: %T)", result, result)

		// Verify result type
		resultStr, ok := result.(string)
		if !ok {
			log.Printf("WARNING: Echo result was not a string!")
		} else if resultStr != echoMessage {
			log.Printf("WARNING: Echo result '%s' did not match sent message '%s'", resultStr, echoMessage)
		}

	} else {
		log.Println("Could not find 'echo' tool to test.")
	}
	// --- End Use Echo Tool ---

	// --- Use the Calculator Tool ---
	log.Println("\n--- Testing Calculator Tool ---")

	// Example 1: Add 5 and 7
	calcArgs1 := map[string]interface{}{
		"operand1":  5.0, // Use float64 for numbers
		"operand2":  7.0,
		"operation": "add",
	}
	result1, err1 := useTool(conn, "calculator", calcArgs1)
	if err1 != nil {
		log.Printf("Failed to use 'calculator' tool (add): %v", err1)
	} else {
		log.Printf("Calculator(add) Result: %v (Type: %T)", result1, result1)
		// Basic check
		if resNum, ok := result1.(float64); !ok || resNum != 12.0 {
			log.Printf("WARNING: Calculator(add) result unexpected: %v", result1)
		}
	}

	// Example 2: Divide by zero (expecting error)
	calcArgs2 := map[string]interface{}{
		"operand1":  10.0,
		"operand2":  0.0,
		"operation": "divide",
	}
	_, err2 := useTool(conn, "calculator", calcArgs2)
	if err2 == nil {
		log.Printf("WARNING: Calculator(divide by zero) should have failed, but succeeded.")
	} else {
		log.Printf("Calculator(divide by zero) failed as expected: %v", err2)
		// Check if it's the specific error code
		if !strings.Contains(err2.Error(), "Division by zero") && !strings.Contains(err2.Error(), "CalculationError") {
			log.Printf("WARNING: Calculator(divide by zero) error message unexpected: %v", err2)
		}
	}

	// Example 3: Missing argument (expecting error)
	calcArgs3 := map[string]interface{}{
		"operand1":  10.0,
		"operation": "multiply", // Missing operand2
	}
	_, err3 := useTool(conn, "calculator", calcArgs3)
	if err3 == nil {
		log.Printf("WARNING: Calculator(missing arg) should have failed, but succeeded.")
	} else {
		log.Printf("Calculator(missing arg) failed as expected: %v", err3)
		if !strings.Contains(err3.Error(), "InvalidArgument") && !strings.Contains(err3.Error(), "Missing required arguments") {
			log.Printf("WARNING: Calculator(missing arg) error message unexpected: %v", err3)
		}
	}
	// --- End Use Calculator Tool ---

	// --- Use the Filesystem Tool ---
	log.Println("\n--- Testing Filesystem Tool ---")
	fsToolName := "filesystem"
	testFilePath := "test_dir/my_file.txt"
	testFileContent := "This is the content of the test file.\nIt has multiple lines."

	// Example 1: List root (expect empty or just .DS_Store initially)
	fsArgs1 := map[string]interface{}{"operation": "list_files", "path": "."}
	fsResult1, fsErr1 := useTool(conn, fsToolName, fsArgs1)
	if fsErr1 != nil {
		log.Printf("Failed to use '%s' tool (list_files .): %v", fsToolName, fsErr1)
	} else {
		log.Printf("Filesystem(list .) Result: %v", fsResult1)
	}

	// Example 2: Write a file
	fsArgs2 := map[string]interface{}{"operation": "write_file", "path": testFilePath, "content": testFileContent}
	fsResult2, fsErr2 := useTool(conn, fsToolName, fsArgs2)
	if fsErr2 != nil {
		log.Printf("Failed to use '%s' tool (write_file): %v", fsToolName, fsErr2)
	} else {
		log.Printf("Filesystem(write) Result: %v", fsResult2)
	}

	// Example 3: Read the file back
	fsArgs3 := map[string]interface{}{"operation": "read_file", "path": testFilePath}
	fsResult3, fsErr3 := useTool(conn, fsToolName, fsArgs3)
	if fsErr3 != nil {
		log.Printf("Failed to use '%s' tool (read_file): %v", fsToolName, fsErr3)
	} else {
		log.Printf("Filesystem(read) Result: %v", fsResult3)
		// Verify content
		if resultMap, ok := fsResult3.(map[string]interface{}); ok {
			if content, ok := resultMap["content"].(string); ok {
				if content != testFileContent {
					log.Printf("WARNING: Filesystem read content mismatch!")
					log.Printf("  Expected: %q", testFileContent)
					log.Printf("  Received: %q", content)
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

	// Example 4: List dir containing the file
	fsArgs4 := map[string]interface{}{"operation": "list_files", "path": "test_dir"} // List the directory
	fsResult4, fsErr4 := useTool(conn, fsToolName, fsArgs4)
	if fsErr4 != nil {
		log.Printf("Failed to use '%s' tool (list_files test_dir): %v", fsToolName, fsErr4)
	} else {
		log.Printf("Filesystem(list test_dir) Result: %v", fsResult4)
	}

	// Example 5: Read non-existent file (expect error)
	fsArgs5 := map[string]interface{}{"operation": "read_file", "path": "non_existent_file.txt"}
	_, fsErr5 := useTool(conn, fsToolName, fsArgs5)
	if fsErr5 == nil {
		log.Printf("WARNING: Filesystem(read non-existent) should have failed, but succeeded.")
	} else {
		log.Printf("Filesystem(read non-existent) failed as expected: %v", fsErr5)
	}

	// Example 6: Write outside sandbox (expect error)
	fsArgs6 := map[string]interface{}{"operation": "write_file", "path": "../outside_sandbox.txt", "content": "attempt escape"}
	_, fsErr6 := useTool(conn, fsToolName, fsArgs6)
	if fsErr6 == nil {
		log.Printf("WARNING: Filesystem(write outside) should have failed, but succeeded.")
	} else {
		log.Printf("Filesystem(write outside) failed as expected: %v", fsErr6)
		if !strings.Contains(fsErr6.Error(), "SecurityViolation") && !strings.Contains(fsErr6.Error(), "escape the sandbox") {
			log.Printf("WARNING: Filesystem(write outside) error message unexpected: %v", fsErr6)
		}
	}
	// --- End Use Filesystem Tool ---

	log.Println("Client finished.")
	// No explicit close needed for stdio connection typically
}
