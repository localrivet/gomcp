// examples/auth-client/main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mcp "github.com/localrivet/gomcp"
)

// requestToolDefinitions sends a ListToolsRequest and processes the response.
func requestToolDefinitions(conn *mcp.Connection) ([]mcp.Tool, error) { // Return []mcp.Tool
	log.Println("Sending ListToolsRequest...")
	reqPayload := mcp.ListToolsRequestParams{}               // Use new params struct (empty for now)
	err := conn.SendMessage(mcp.MethodListTools, reqPayload) // Use new method name
	if err != nil {
		return nil, fmt.Errorf("failed to send ListToolsRequest: %w", err)
	}

	log.Println("Waiting for ListToolsResponse...")
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() { defer close(done); responseMsg, receiveErr = conn.ReceiveMessage() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for ListToolsResponse") // Update error message
	}
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive ListToolsResponse: %w", receiveErr) // Update error message
	}
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("received MCP Error: [%d] %s", errPayload.Code, errPayload.Message)
		}
		return nil, fmt.Errorf("received MCP Error with unparsable payload")
	}
	// TODO: Update this check when transport handles JSON-RPC responses properly
	if responseMsg.MessageType != "ListToolsResponse" {
		return nil, fmt.Errorf("expected ListToolsResponse, got %s", responseMsg.MessageType)
	}
	var responsePayload mcp.ListToolsResult // Use new result struct
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListToolsResult payload: %w", err) // Update error message
	}
	// TODO: Handle pagination (responsePayload.NextCursor)
	return responsePayload.Tools, nil
}

// useTool sends a CallToolRequest and processes the response.
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
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() { defer close(done); responseMsg, receiveErr = conn.ReceiveMessage() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for CallToolResponse for '%s'", toolName) // Update error message
	}
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive CallToolResponse for '%s': %w", toolName, receiveErr) // Update error message
	}
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("tool '%s' failed with MCP Error: [%d] %s", toolName, errPayload.Code, errPayload.Message)
		}
		return nil, fmt.Errorf("tool '%s' failed with an unparsable MCP Error payload", toolName)
	}
	// TODO: Update this check when transport handles JSON-RPC responses properly
	if responseMsg.MessageType != "CallToolResponse" {
		return nil, fmt.Errorf("expected CallToolResponse for '%s', got %s", toolName, responseMsg.MessageType)
	}
	var responsePayload mcp.CallToolResult // Use new result struct
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CallToolResult payload for '%s': %w", toolName, err) // Update error message
	}
	// Check if the result itself indicates an error
	if responsePayload.IsError != nil && *responsePayload.IsError {
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", toolName)
		if len(responsePayload.Content) > 0 {
			if textContent, ok := responsePayload.Content[0].(mcp.TextContent); ok {
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", toolName, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", toolName, responsePayload.Content[0])
			}
		}
		return responsePayload.Content, fmt.Errorf("%s", errMsg) // Return content and an error, use %s
	}
	return responsePayload.Content, nil
}

// runClientLogic performs the initialization and executes the example tool calls sequence.
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
		return fmt.Errorf("failed to get tool definitions: %w", err)
	}
	log.Printf("Received %d tool definitions:", len(tools))
	for _, tool := range tools {
		toolJson, _ := json.MarshalIndent(tool, "", "  ")
		fmt.Fprintf(os.Stderr, "%s\n", string(toolJson))
	}
	// --- End Request Tool Definitions ---

	// --- Use the Secure Echo Tool ---
	secureEchoToolFound := false
	for _, tool := range tools {
		if tool.Name == "secure-echo" {
			secureEchoToolFound = true
			break
		}
	}
	if secureEchoToolFound {
		log.Println("\n--- Testing Secure Echo Tool ---")
		echoMessage := "Secret message!"
		args := map[string]interface{}{"message": echoMessage}
		result, err := useTool(conn, "secure-echo", args)
		if err != nil {
			log.Printf("ERROR: Failed to use 'secure-echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'secure-echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result) // Log the content slice
			// Extract text from the first TextContent element
			if len(result) > 0 {
				if textContent, ok := result[0].(mcp.TextContent); ok {
					log.Printf("  Extracted Text: %s", textContent.Text)
					if textContent.Text != echoMessage {
						log.Printf("WARNING: Secure Echo result '%s' did not match sent message '%s'", textContent.Text, echoMessage)
					}
				} else {
					log.Printf("WARNING: Secure Echo result content[0] was not TextContent: %T", result[0])
				}
			} else {
				log.Printf("WARNING: Secure Echo result content was empty!")
			}
		}
	} else {
		log.Println("Could not find 'secure-echo' tool definition from server.")
	}
	// --- End Use Secure Echo Tool ---

	log.Println("Client finished.")
	return nil // Indicate success
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Auth Example MCP Client...")

	clientName := "GoAuthClient"
	conn := mcp.NewStdioConnection()

	// Run the core client logic
	err := runClientLogic(conn, clientName)
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}
}
