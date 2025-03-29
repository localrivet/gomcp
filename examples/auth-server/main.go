// examples/auth-server/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	mcp "github.com/localrivet/gomcp"
)

// For this simple example, the expected API key is hardcoded.
// In a real application, this should come from secure configuration.
const expectedApiKey = "test-key-123"

// Define the secure echo tool
var secureEchoTool = mcp.ToolDefinition{
	Name:        "secure-echo",
	Description: "Echoes back the provided message (Requires API Key Auth).",
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

// Tool registry for this server, only containing the secure tool.
var toolRegistry = map[string]mcp.ToolDefinition{
	secureEchoTool.Name: secureEchoTool,
}

// handleToolDefinitionRequest sends the list of defined tools.
// Note: Tool definitions are typically public and don't require auth.
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

// handleUseToolRequest handles the execution of the secure-echo tool.
// Assumes authentication happened at server startup in this simple example.
func handleUseToolRequest(conn *mcp.Connection, requestPayload *mcp.UseToolRequestPayload) error {
	log.Println("Handling UseToolRequest")
	log.Printf("Requested tool: %s with args: %v", requestPayload.ToolName, requestPayload.Arguments)

	// Check if the requested tool is the one we handle
	if requestPayload.ToolName != secureEchoTool.Name {
		log.Printf("Tool not found: %s", requestPayload.ToolName)
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "ToolNotFound",
			Message: fmt.Sprintf("Tool '%s' not found", requestPayload.ToolName),
		})
	}

	// --- Execute the "secure-echo" tool ---
	messageArg, ok := requestPayload.Arguments["message"]
	if !ok {
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "InvalidArgument", Message: "Missing required argument 'message' for tool 'secure-echo'"})
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "InvalidArgument", Message: "Argument 'message' for tool 'secure-echo' must be a string"})
	}

	log.Printf("Securely Echoing message: %s", messageStr)
	responsePayload := mcp.UseToolResponsePayload{Result: messageStr}
	return conn.SendMessage(mcp.MessageTypeUseToolResponse, responsePayload)
	// --- End secure-echo tool execution ---
}

// runServerLogic performs the handshake and runs the main message loop.
// Returns error on failure.
func runServerLogic(conn *mcp.Connection, serverName string) error {
	// --- Perform Handshake ---
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
	// Basic version check
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
	// Send HandshakeResponse
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
				return nil // Clean exit
			}
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			return err
		}

		log.Printf("Received message type: %s", msg.MessageType)
		var handlerErr error

		switch msg.MessageType {
		case mcp.MessageTypeToolDefinitionRequest:
			handlerErr = handleToolDefinitionRequest(conn) // Pass only conn
		case mcp.MessageTypeUseToolRequest:
			// **Auth Check could happen here if key was passed in message**
			// For this example, auth happened at startup via env var check.
			var utReqPayload mcp.UseToolRequestPayload
			err := mcp.UnmarshalPayload(msg.Payload, &utReqPayload)
			if err != nil {
				log.Printf("Error unmarshalling UseToolRequest payload: %v", err)
				handlerErr = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{Code: "InvalidPayload", Message: fmt.Sprintf("Failed to unmarshal UseToolRequest payload: %v", err)})
			} else {
				handlerErr = handleUseToolRequest(conn, &utReqPayload) // Pass parsed payload
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
	// Read the required API key from the environment.
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: MCP_API_KEY environment variable not set.")
	}
	// Validate the key against the expected value.
	if apiKey != expectedApiKey {
		log.Fatalf("FATAL: Invalid MCP_API_KEY provided. Expected '%s'", expectedApiKey)
	}
	log.Println("API Key validated successfully.")
	// --- End API Key Check ---

	log.Println("Starting Auth Example MCP Server...")
	serverName := "GoAuthServer"
	conn := mcp.NewStdioConnection()

	// Run the core server logic (handshake + message loop)
	err := runServerLogic(conn, serverName)
	if err != nil {
		log.Printf("Server exited with error: %v", err)
		os.Exit(1) // Exit with non-zero status on error
	} else {
		log.Println("Server finished.")
	}
}
