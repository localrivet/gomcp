package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	mcp "github.com/localrivet/gomcp" // Import root package
)

// Define the echo tool
var echoTool = mcp.ToolDefinition{
	Name:        "echo",
	Description: "Echoes back the provided message.",
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

// Simple map to hold our tools
var toolRegistry = map[string]mcp.ToolDefinition{
	echoTool.Name:                 echoTool,
	calculatorToolDefinition.Name: calculatorToolDefinition,
	fileSystemToolDefinition.Name: fileSystemToolDefinition, // Add filesystem tool
}

// handleToolDefinitionRequest sends the list of defined tools.
func handleToolDefinitionRequest(conn *mcp.Connection, requestMsg *mcp.Message) error {
	log.Println("Handling ToolDefinitionRequest")
	tools := make([]mcp.ToolDefinition, 0, len(toolRegistry))
	for _, tool := range toolRegistry {
		tools = append(tools, tool)
	}

	responsePayload := mcp.ToolDefinitionResponsePayload{
		Tools: tools,
	}
	// Note: The spec implies the payload should be sent directly, not wrapped in another struct.
	// Assuming SendMessage handles wrapping in the base Message struct correctly.
	return conn.SendMessage(mcp.MessageTypeToolDefinitionResponse, responsePayload)
}

// handleUseToolRequest executes the requested tool.
func handleUseToolRequest(conn *mcp.Connection, requestMsg *mcp.Message) error {
	log.Println("Handling UseToolRequest")
	var requestPayload mcp.UseToolRequestPayload
	err := mcp.UnmarshalPayload(requestMsg.Payload, &requestPayload)
	if err != nil {
		log.Printf("Error unmarshalling UseToolRequest payload: %v", err)
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "InvalidPayload",
			Message: fmt.Sprintf("Failed to unmarshal UseToolRequest payload: %v", err),
		})
	}

	log.Printf("Requested tool: %s with args: %v", requestPayload.ToolName, requestPayload.Arguments)

	// Find the tool
	tool, exists := toolRegistry[requestPayload.ToolName]
	if !exists {
		log.Printf("Tool not found: %s", requestPayload.ToolName)
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "ToolNotFound",
			Message: fmt.Sprintf("Tool '%s' not found", requestPayload.ToolName),
		})
	}

	// --- Execute the "echo" tool ---
	if tool.Name == "echo" {
		messageArg, ok := requestPayload.Arguments["message"]
		if !ok {
			log.Println("Missing 'message' argument for echo tool")
			return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
				Code:    "InvalidArgument",
				Message: "Missing required argument 'message' for tool 'echo'",
			})
		}
		messageStr, ok := messageArg.(string)
		if !ok {
			log.Printf("'message' argument is not a string: %T", messageArg)
			return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
				Code:    "InvalidArgument",
				Message: "Argument 'message' for tool 'echo' must be a string",
			})
		}

		log.Printf("Echoing message: %s", messageStr)
		responsePayload := mcp.UseToolResponsePayload{
			Result: messageStr, // Echo the message back
		}
		return conn.SendMessage(mcp.MessageTypeUseToolResponse, responsePayload)
	}
	// --- End echo tool execution ---

	// --- Execute the "calculator" tool ---
	if tool.Name == "calculator" {
		result, calcErr := executeCalculator(requestPayload.Arguments)
		if calcErr != nil {
			log.Printf("Calculator execution error: %v", calcErr.Message)
			return conn.SendMessage(mcp.MessageTypeError, *calcErr)
		}

		log.Printf("Calculator result: %v", result)
		responsePayload := mcp.UseToolResponsePayload{
			Result: result,
		}
		return conn.SendMessage(mcp.MessageTypeUseToolResponse, responsePayload)
	}
	// --- End calculator tool execution ---

	// --- Execute the "filesystem" tool ---
	if tool.Name == "filesystem" {
		result, fsErr := executeFileSystem(requestPayload.Arguments)
		if fsErr != nil {
			log.Printf("Filesystem execution error: %v", fsErr.Message)
			return conn.SendMessage(mcp.MessageTypeError, *fsErr)
		}

		log.Printf("Filesystem result: %v", result) // Result might be large, consider summarizing
		responsePayload := mcp.UseToolResponsePayload{
			Result: result,
		}
		return conn.SendMessage(mcp.MessageTypeUseToolResponse, responsePayload)
	}
	// --- End filesystem tool execution ---

	// Default error for unhandled but defined tools
	log.Printf("Tool '%s' execution not implemented", requestPayload.ToolName)
	return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
		Code:    "NotImplemented",
		Message: fmt.Sprintf("Execution for tool '%s' is not implemented", requestPayload.ToolName),
	})
}

func main() {
	// Log to stderr so stdout can be used purely for MCP messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Echo MCP Server...")

	serverName := "GoEchoServer"
	conn := mcp.NewStdioConnection() // Use stdio

	// --- Perform Handshake (Manual for more control over loop) ---
	log.Println("Waiting for HandshakeRequest...")
	msg, err := conn.ReceiveMessage()
	if err != nil {
		log.Fatalf("Failed to receive initial message: %v", err)
	}
	if msg.MessageType != mcp.MessageTypeHandshakeRequest {
		// Send error back before failing
		_ = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "HandshakeFailed",
			Message: fmt.Sprintf("Expected HandshakeRequest, got %s", msg.MessageType),
		})
		log.Fatalf("Expected HandshakeRequest, got %s", msg.MessageType)
	}
	var reqPayload mcp.HandshakeRequestPayload
	err = mcp.UnmarshalPayload(msg.Payload, &reqPayload)
	if err != nil {
		_ = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "HandshakeFailed",
			Message: fmt.Sprintf("Failed to unmarshal HandshakeRequest payload: %v", err),
		})
		log.Fatalf("Failed to unmarshal HandshakeRequest payload: %v", err)
	}
	log.Printf("Received HandshakeRequest from client: %s", reqPayload.ClientName)

	// Validate version
	clientSupportsCurrent := false
	for _, v := range reqPayload.SupportedProtocolVersions {
		if v == mcp.CurrentProtocolVersion {
			clientSupportsCurrent = true
			break
		}
	}
	if !clientSupportsCurrent {
		_ = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "UnsupportedProtocolVersion",
			Message: fmt.Sprintf("Server requires protocol version %s", mcp.CurrentProtocolVersion),
		})
		log.Fatalf("Client does not support protocol version %s", mcp.CurrentProtocolVersion)
	}

	// Send HandshakeResponse
	respPayload := mcp.HandshakeResponsePayload{
		SelectedProtocolVersion: mcp.CurrentProtocolVersion,
		ServerName:              serverName,
	}
	err = conn.SendMessage(mcp.MessageTypeHandshakeResponse, respPayload)
	if err != nil {
		// Don't try to send another error if sending response failed
		log.Fatalf("Failed to send HandshakeResponse: %v", err)
	}
	log.Printf("Handshake successful with client: %s", reqPayload.ClientName)
	// --- End Handshake ---

	// --- Main Message Loop ---
	log.Println("Entering main message loop...")
	for {
		msg, err := conn.ReceiveMessage()
		if err != nil {
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			break // Exit loop on error (e.g., EOF)
		}

		log.Printf("Received message type: %s", msg.MessageType)
		var handlerErr error

		switch msg.MessageType {
		case mcp.MessageTypeToolDefinitionRequest:
			handlerErr = handleToolDefinitionRequest(conn, msg)
		case mcp.MessageTypeUseToolRequest:
			handlerErr = handleUseToolRequest(conn, msg)
		// TODO: Add cases for ResourceAccessRequest etc.
		default:
			log.Printf("Handler not implemented for message type: %s", msg.MessageType)
			handlerErr = conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
				Code:    "NotImplemented",
				Message: fmt.Sprintf("Message type '%s' not implemented by server", msg.MessageType),
			})
		}

		if handlerErr != nil {
			log.Printf("Error handling message type %s: %v", msg.MessageType, handlerErr)
			// If sending the response/error failed, the connection is likely broken, so exit.
			if strings.Contains(handlerErr.Error(), "write") || strings.Contains(handlerErr.Error(), "pipe") {
				log.Println("Detected write error, shutting down.")
				break
			}
		}
	}

	log.Println("Server finished.")
}
