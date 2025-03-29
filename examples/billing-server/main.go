// examples/billing-server/main.go
package main

import (
	"encoding/json" // For logging structured billing event
	"fmt"
	"log"
	"os"
	"strings"
	"time" // For timestamp

	mcp "github.com/localrivet/gomcp"
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the chargeable echo tool
var chargeableEchoTool = mcp.ToolDefinition{
	Name:        "chargeable-echo",
	Description: "Echoes back the provided message (Simulates Billing/Tracking).",
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
	OutputSchema: mcp.ToolOutputSchema{
		Type:        "string",
		Description: "The original message.",
	},
}

// Tool registry for this server
var toolRegistry = map[string]mcp.ToolDefinition{
	chargeableEchoTool.Name: chargeableEchoTool,
}

// handleToolDefinitionRequest sends the list of defined tools.
func handleToolDefinitionRequest(conn *mcp.Connection) error {
	log.Println("Handling ToolDefinitionRequest")
	tools := make([]mcp.ToolDefinition, 0, len(toolRegistry))
	for _, tool := range toolRegistry {
		tools = append(tools, tool)
	}
	responsePayload := mcp.ToolDefinitionResponsePayload{Tools: tools}
	log.Printf("Sending ToolDefinitionResponse with %d tools", len(tools))
	return conn.SendMessage(mcp.MessageTypeToolDefinitionResponse, responsePayload)
}

// handleUseToolRequest handles the execution, logging a billing event first.
// Takes the validated apiKey used for the session.
func handleUseToolRequest(conn *mcp.Connection, requestPayload *mcp.UseToolRequestPayload, apiKey string) error {
	log.Println("Handling UseToolRequest")
	log.Printf("Requested tool: %s with args: %v", requestPayload.ToolName, requestPayload.Arguments)

	// --- Simulate Billing/Tracking Event ---
	// In a real system, this would record to a database or billing service.
	// Here, we just log a structured message to stderr.
	billingEvent := map[string]interface{}{
		"event_type": "tool_usage",
		"api_key":    apiKey, // Include the validated API key
		"tool_name":  requestPayload.ToolName,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		// Could also include argument summaries if needed, be careful with sensitive data
	}
	eventJson, _ := json.Marshal(billingEvent) // Ignore error for logging
	log.Printf("BILLING_EVENT: %s", string(eventJson))
	// --- End Billing/Tracking ---

	// Validate tool exists
	if requestPayload.ToolName != chargeableEchoTool.Name {
		log.Printf("Tool not found: %s", requestPayload.ToolName)
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "ToolNotFound", Message: fmt.Sprintf("Tool '%s' not found", requestPayload.ToolName)})
	}

	// --- Execute the "chargeable-echo" tool ---
	messageArg, ok := requestPayload.Arguments["message"]
	if !ok {
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "InvalidArgument", Message: "Missing required argument 'message' for tool 'chargeable-echo'"})
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "InvalidArgument", Message: "Argument 'message' for tool 'chargeable-echo' must be a string"})
	}

	log.Printf("Chargeable Echoing message: %s", messageStr)
	responsePayload := mcp.UseToolResponsePayload{Result: messageStr}
	return conn.SendMessage(mcp.MessageTypeUseToolResponse, responsePayload)
	// --- End chargeable-echo tool execution ---
}

// runServerLogic performs the handshake and runs the main message loop.
// Takes the validated API key as an argument.
func runServerLogic(conn *mcp.Connection, serverName string, apiKey string) error {
	// --- Handshake (same as auth-server) ---
	log.Println("Waiting for HandshakeRequest...")
	msg, err := conn.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive initial message: %w", err)
	}
	if msg.MessageType != mcp.MessageTypeHandshakeRequest {
		errMsg := fmt.Sprintf("Expected HandshakeRequest, got %s", msg.MessageType)
		_ = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "HandshakeFailed", Message: errMsg})
		return fmt.Errorf("%s", errMsg)
	}
	var hsReqPayload mcp.HandshakeRequestPayload
	err = mcp.UnmarshalPayload(msg.Payload, &hsReqPayload)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to unmarshal HandshakeRequest payload: %v", err)
		_ = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "HandshakeFailed", Message: errMsg})
		return fmt.Errorf("failed to unmarshal HandshakeRequest payload: %w", err)
	}
	log.Printf("Received HandshakeRequest from client: %s", hsReqPayload.ClientName)
	clientSupportsCurrent := false
	for _, v := range hsReqPayload.SupportedProtocolVersions {
		if v == mcp.CurrentProtocolVersion {
			clientSupportsCurrent = true
			break
		}
	}
	if !clientSupportsCurrent {
		errMsg := fmt.Sprintf("Client does not support protocol version %s", mcp.CurrentProtocolVersion)
		_ = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "UnsupportedProtocolVersion", Message: fmt.Sprintf("Server requires protocol version %s", mcp.CurrentProtocolVersion)})
		return fmt.Errorf("%s", errMsg)
	}
	hsRespPayload := mcp.HandshakeResponsePayload{SelectedProtocolVersion: mcp.CurrentProtocolVersion, ServerName: serverName}
	err = conn.SendMessage(mcp.MessageTypeHandshakeResponse, hsRespPayload)
	if err != nil {
		return fmt.Errorf("failed to send HandshakeResponse: %w", err)
	}
	log.Printf("Handshake successful with client: %s", hsReqPayload.ClientName)
	// --- End Handshake ---

	// --- Main Message Loop ---
	log.Println("Entering main message loop...")
	for {
		msg, err := conn.ReceiveMessage()
		if err != nil {
			if err.Error() == "failed to read message line: EOF" || strings.Contains(err.Error(), "EOF") {
				log.Println("Client disconnected (EOF received). Server shutting down.")
				return nil
			}
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			return err
		}

		log.Printf("Received message type: %s", msg.MessageType)
		var handlerErr error

		switch msg.MessageType {
		case mcp.MessageTypeToolDefinitionRequest:
			handlerErr = handleToolDefinitionRequest(conn)
		case mcp.MessageTypeUseToolRequest:
			var utReqPayload mcp.UseToolRequestPayload
			err := mcp.UnmarshalPayload(msg.Payload, &utReqPayload)
			if err != nil {
				log.Printf("Error unmarshalling UseToolRequest payload: %v", err)
				handlerErr = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "InvalidPayload", Message: fmt.Sprintf("Failed to unmarshal UseToolRequest payload: %v", err)})
			} else {
				// Pass the validated API key to the handler for potential use (logging)
				handlerErr = handleUseToolRequest(conn, &utReqPayload, apiKey)
			}
		default:
			log.Printf("Handler not implemented for message type: %s", msg.MessageType)
			handlerErr = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "NotImplemented", Message: fmt.Sprintf("Message type '%s' not implemented by server", msg.MessageType)})
		}

		if handlerErr != nil {
			log.Printf("Error handling message type %s: %v", msg.MessageType, handlerErr)
			if strings.Contains(handlerErr.Error(), "write") || strings.Contains(handlerErr.Error(), "pipe") {
				log.Println("Detected write error, assuming client disconnected. Shutting down.")
				return handlerErr
			}
		}
	}
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// --- API Key Check ---
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: MCP_API_KEY environment variable not set.")
	}
	if apiKey != expectedApiKey {
		log.Fatalf("FATAL: Invalid MCP_API_KEY provided. Expected '%s'", expectedApiKey)
	}
	log.Println("API Key validated successfully.")
	// --- End API Key Check ---

	log.Println("Starting Billing/Tracking Example MCP Server...")
	serverName := "GoBillingServer"
	conn := mcp.NewStdioConnection()

	// Pass the validated API key to the server logic
	err := runServerLogic(conn, serverName, apiKey)
	if err != nil {
		log.Printf("Server exited with error: %v", err)
		os.Exit(1)
	} else {
		log.Println("Server finished.")
	}
}
