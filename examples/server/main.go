// This is the main file for the example MCP server.
// It demonstrates how to:
// 1. Define multiple tools (echo, calculator, filesystem).
// 2. Register these tools.
// 3. Handle the MCP handshake manually.
// 4. Run a message loop to handle ToolDefinitionRequest and UseToolRequest messages.
//
// Tool logic for calculator and filesystem are in separate files (calculator.go, filesystem.go)
// but belong to the same 'main' package.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	mcp "github.com/localrivet/gomcp" // Import root package
)

// --- Tool Definitions ---
// Tools are defined as variables conforming to mcp.ToolDefinition.
// See calculator.go and filesystem.go for the other definitions.

// Define the echo tool (simple tool defined directly in main)
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

// --- Tool Registry ---
// A simple map to store the available tools by name.
// In a real application, this might be more dynamic.
var toolRegistry = map[string]mcp.ToolDefinition{
	echoTool.Name:                 echoTool,
	calculatorToolDefinition.Name: calculatorToolDefinition, // Defined in calculator.go
	fileSystemToolDefinition.Name: fileSystemToolDefinition, // Defined in filesystem.go
}

// --- Request Handlers ---

// handleToolDefinitionRequest processes a ToolDefinitionRequest message.
// It collects all tools from the registry and sends them back in a ToolDefinitionResponse.
func handleToolDefinitionRequest(conn *mcp.Connection, requestMsg *mcp.Message) error {
	log.Println("Handling ToolDefinitionRequest")
	tools := make([]mcp.ToolDefinition, 0, len(toolRegistry))
	// Ensure consistent order for easier testing/viewing if needed
	toolNames := []string{"echo", "calculator", "filesystem"} // Define order
	for _, name := range toolNames {
		if tool, ok := toolRegistry[name]; ok {
			tools = append(tools, tool)
		}
	}
	// Add any other tools not in the explicit order (if registry grows)
	for name, tool := range toolRegistry {
		found := false
		for _, orderedName := range toolNames {
			if name == orderedName {
				found = true
				break
			}
		}
		if !found {
			tools = append(tools, tool)
		}
	}

	responsePayload := mcp.ToolDefinitionResponsePayload{
		Tools: tools,
	}
	log.Printf("Sending ToolDefinitionResponse with %d tools", len(tools))
	return conn.SendMessage(mcp.MessageTypeToolDefinitionResponse, responsePayload)
}

// handleUseToolRequest processes a UseToolRequest message.
// It finds the requested tool in the registry and calls its specific execution function.
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

	// Find the tool in the registry
	_, exists := toolRegistry[requestPayload.ToolName]
	if !exists {
		log.Printf("Tool not found: %s", requestPayload.ToolName)
		return conn.SendMessage(mcp.MessageTypeError, mcp.ErrorPayload{
			Code:    "ToolNotFound",
			Message: fmt.Sprintf("Tool '%s' not found", requestPayload.ToolName),
		})
	}

	// --- Dispatch to specific tool execution logic ---
	var result interface{}
	var execErr *mcp.ErrorPayload

	switch requestPayload.ToolName { // Dispatch based on requested name
	case "echo":
		// Simple echo logic directly here
		messageArg, ok := requestPayload.Arguments["message"]
		if !ok {
			execErr = &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Missing required argument 'message' for tool 'echo'"}
		} else if messageStr, ok := messageArg.(string); !ok {
			execErr = &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Argument 'message' for tool 'echo' must be a string"}
		} else {
			log.Printf("Echoing message: %s", messageStr)
			result = messageStr // Echo the message back
		}

	case "calculator":
		// Call function defined in calculator.go
		result, execErr = executeCalculator(requestPayload.Arguments)
		if execErr == nil {
			log.Printf("Calculator result: %v", result)
		} else {
			log.Printf("Calculator execution error: %v", execErr.Message)
		}

	case "filesystem":
		// Call function defined in filesystem.go
		result, execErr = executeFileSystem(requestPayload.Arguments)
		if execErr == nil {
			log.Printf("Filesystem result: %v", result) // Result might be large, consider summarizing
		} else {
			log.Printf("Filesystem execution error: %v", execErr.Message)
		}

	default:
		// Should not happen if tool is in registry, but good practice
		log.Printf("Tool '%s' execution logic missing in handler", requestPayload.ToolName)
		execErr = &mcp.ErrorPayload{Code: "NotImplemented", Message: fmt.Sprintf("Execution logic for tool '%s' is not implemented", requestPayload.ToolName)}
	}
	// --- End dispatch ---

	// Send response (either result or error)
	if execErr != nil {
		// Send MCP Error message
		return conn.SendMessage(mcp.MessageTypeError, *execErr)
	}

	// Send successful UseToolResponse
	responsePayload := mcp.UseToolResponsePayload{
		Result: result,
	}
	return conn.SendMessage(mcp.MessageTypeUseToolResponse, responsePayload)
}

// --- Main Function ---
func main() {
	// Log to stderr so stdout can be used purely for MCP messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Server...") // Updated name

	serverName := "GoMultiToolServer" // Updated name
	conn := mcp.NewStdioConnection()  // Use stdio connection

	// --- Perform Handshake (Manual Implementation) ---
	// This section manually performs the handshake steps.
	// Alternatively, one could use the mcp.Server struct from the library,
	// but that requires integrating the tool handling logic differently (e.g., via callbacks or methods).
	log.Println("Waiting for HandshakeRequest...")
	msg, err := conn.ReceiveMessage()
	if err != nil {
		log.Fatalf("Failed to receive initial message: %v", err) // Exit on handshake failure
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

	// Validate protocol version
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
	// Continuously receive messages and dispatch them to handlers after successful handshake.
	log.Println("Entering main message loop...")
	for {
		msg, err := conn.ReceiveMessage()
		if err != nil {
			// io.EOF is expected when the client disconnects cleanly
			if err.Error() == "failed to read message line: EOF" || strings.Contains(err.Error(), "EOF") {
				log.Println("Client disconnected (EOF received). Server shutting down.")
			} else {
				log.Printf("Error receiving message: %v. Server shutting down.", err)
			}
			break // Exit loop on any receive error
		}

		log.Printf("Received message type: %s", msg.MessageType)
		var handlerErr error

		// Dispatch message to appropriate handler
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

		// Check for errors during handling (especially sending response/error)
		if handlerErr != nil {
			log.Printf("Error handling message type %s: %v", msg.MessageType, handlerErr)
			// If sending the response/error failed, the connection is likely broken, so exit.
			// Checking for "write" or "pipe" covers common scenarios.
			if strings.Contains(handlerErr.Error(), "write") || strings.Contains(handlerErr.Error(), "pipe") {
				log.Println("Detected write error, assuming client disconnected. Shutting down.")
				break
			}
			// Otherwise, log the error but continue the loop for potentially recoverable errors.
		}
	}

	log.Println("Server finished.")
}
