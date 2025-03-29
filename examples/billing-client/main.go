// examples/billing-client/main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mcp "github.com/localrivet/gomcp"
)

// Helper function to request tool definitions (same as other client)
func requestToolDefinitions(conn *mcp.Connection) ([]mcp.ToolDefinition, error) {
	log.Println("Sending ToolDefinitionRequest...")
	reqPayload := mcp.ToolDefinitionRequestPayload{}
	err := conn.SendMessage(mcp.MessageTypeToolDefinitionRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send ToolDefinitionRequest: %w", err)
	}

	log.Println("Waiting for ToolDefinitionResponse...")
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() { defer close(done); responseMsg, receiveErr = conn.ReceiveMessage() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for ToolDefinitionResponse")
	}
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive ToolDefinitionResponse: %w", receiveErr)
	}
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("received MCP Error: [%d] %s", errPayload.Code, errPayload.Message) // Use %d for int code
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

// Helper function to use a tool (same as other client)
func useTool(conn *mcp.Connection, toolName string, args map[string]interface{}) (interface{}, error) {
	log.Printf("Sending UseToolRequest for tool '%s'...", toolName)
	reqPayload := mcp.UseToolRequestPayload{ToolName: toolName, Arguments: args}
	err := conn.SendMessage(mcp.MessageTypeUseToolRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send UseToolRequest for '%s': %w", toolName, err)
	}

	log.Println("Waiting for UseToolResponse...")
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() { defer close(done); responseMsg, receiveErr = conn.ReceiveMessage() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for UseToolResponse for '%s'", toolName)
	}
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive UseToolResponse for '%s': %w", toolName, receiveErr)
	}
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("tool '%s' failed with MCP Error: [%d] %s", toolName, errPayload.Code, errPayload.Message) // Use %d for int code
		}
		return nil, fmt.Errorf("tool '%s' failed with an unparsable MCP Error payload", toolName)
	}
	if responseMsg.MessageType != mcp.MessageTypeUseToolResponse {
		return nil, fmt.Errorf("expected UseToolResponse for '%s', got %s", toolName, responseMsg.MessageType)
	}
	var responsePayload mcp.UseToolResponsePayload
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal UseToolResponse payload for '%s': %w", toolName, err)
	}
	return responsePayload.Result, nil
}

// runClientLogic performs the handshake and executes the example tool calls sequence.
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
		result, err := useTool(conn, "chargeable-echo", args)
		if err != nil {
			log.Printf("ERROR: Failed to use 'chargeable-echo' tool: %v", err)
		} else {
			log.Printf("Successfully used 'chargeable-echo' tool.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received: %v (Type: %T)", result, result)
			resultStr, ok := result.(string)
			if !ok {
				log.Printf("WARNING: Chargeable Echo result was not a string!")
			} else if resultStr != echoMessage {
				log.Printf("WARNING: Chargeable Echo result '%s' did not match sent message '%s'", resultStr, echoMessage)
			}
		}
	} else {
		log.Println("Could not find 'chargeable-echo' tool definition from server.")
	}
	// --- End Use Chargeable Echo Tool ---

	log.Println("Client finished.")
	return nil // Indicate success
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Billing Example MCP Client...")

	clientName := "GoBillingClient"
	conn := mcp.NewStdioConnection()

	// Run the core client logic
	err := runClientLogic(conn, clientName)
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}
}
