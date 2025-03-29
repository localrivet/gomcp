// server.go (Modified)
package mcp

import (
	// Required for UnmarshalPayload usage within server logic
	"fmt"
	"log"
	"os"
	"strings" // Needed for error check in loop
)

// ToolHandlerFunc defines the signature for functions that handle tool execution.
// It receives the arguments provided by the client and should return the result
// or an ErrorPayload if execution fails.
type ToolHandlerFunc func(arguments map[string]interface{}) (result interface{}, errorPayload *ErrorPayload)

// Server represents an MCP server instance. It manages the connection,
// handles the handshake, tool registration, and processes incoming messages.
type Server struct {
	conn         *Connection // The underlying MCP connection handler
	serverName   string
	toolRegistry map[string]ToolDefinition  // Stores tool definitions
	toolHandlers map[string]ToolHandlerFunc // Stores handlers for each tool
}

// NewServer creates and initializes a new MCP Server instance.
func NewServer(serverName string) *Server {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	return &Server{
		conn:         NewStdioConnection(), // Assumes stdio for now
		serverName:   serverName,
		toolRegistry: make(map[string]ToolDefinition),
		toolHandlers: make(map[string]ToolHandlerFunc),
	}
}

// NewServerWithConnection creates and initializes a new MCP Server instance
// using the provided mcp.Connection. This is useful for testing or integrating
// with different transport mechanisms.
func NewServerWithConnection(serverName string, conn *Connection) *Server {
	log.SetOutput(os.Stderr) // Still configure logging
	log.SetFlags(log.Ltime | log.Lshortfile)

	if conn == nil {
		log.Println("Warning: NewServerWithConnection received nil connection, falling back to stdio.")
		conn = NewStdioConnection()
	}

	return &Server{
		conn:         conn, // Use provided connection
		serverName:   serverName,
		toolRegistry: make(map[string]ToolDefinition),
		toolHandlers: make(map[string]ToolHandlerFunc),
	}
}

// RegisterTool adds a tool definition and its corresponding handler function to the server.
// It returns an error if a tool with the same name is already registered or handler is nil.
func (s *Server) RegisterTool(definition ToolDefinition, handler ToolHandlerFunc) error {
	if definition.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := s.toolRegistry[definition.Name]; exists {
		return fmt.Errorf("tool '%s' already registered", definition.Name)
	}
	if handler == nil {
		return fmt.Errorf("handler for tool '%s' cannot be nil", definition.Name)
	}
	s.toolRegistry[definition.Name] = definition
	s.toolHandlers[definition.Name] = handler
	log.Printf("Registered tool: %s", definition.Name)
	return nil
}

// handleHandshake performs the server side of the MCP handshake protocol.
// (This function remains largely the same as before, using new Error Codes)
func (s *Server) handleHandshake() (clientName string, err error) {
	log.Println("Waiting for HandshakeRequest...")
	msg, err := s.conn.ReceiveMessage()
	if err != nil {
		return "", fmt.Errorf("failed to receive initial message: %w", err)
	}

	if msg.MessageType != MessageTypeHandshakeRequest {
		errMsg := fmt.Sprintf("Expected HandshakeRequest, got %s", msg.MessageType)
		// Attempt to send specific error code
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeMCPHandshakeFailed, Message: errMsg}) // Use MCP code
		return "", fmt.Errorf("%s", errMsg)
	}

	var reqPayload HandshakeRequestPayload
	err = UnmarshalPayload(msg.Payload, &reqPayload)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to unmarshal HandshakeRequest payload: %v", err)
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeMCPInvalidPayload, Message: errMsg}) // Correct
		return "", fmt.Errorf("failed to unmarshal HandshakeRequest payload: %w", err)
	}
	// Validate required payload field
	if reqPayload.SupportedProtocolVersions == nil {
		errMsg := "malformed HandshakeRequest payload: missing supported_protocol_versions"
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeMCPInvalidPayload, Message: errMsg}) // Correct
		return "", fmt.Errorf(errMsg)
	}

	log.Printf("Received HandshakeRequest from client: %s", reqPayload.ClientName)
	clientSupportsCurrent := false
	for _, v := range reqPayload.SupportedProtocolVersions {
		if v == CurrentProtocolVersion {
			clientSupportsCurrent = true
			break
		}
	}
	if !clientSupportsCurrent {
		errMsg := fmt.Sprintf("Client does not support protocol version %s", CurrentProtocolVersion)
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeMCPUnsupportedProtocolVersion, Message: fmt.Sprintf("Server requires protocol version %s", CurrentProtocolVersion)}) // Use MCP code
		return "", fmt.Errorf("%s", errMsg)
	}

	respPayload := HandshakeResponsePayload{SelectedProtocolVersion: CurrentProtocolVersion, ServerName: s.serverName}
	err = s.conn.SendMessage(MessageTypeHandshakeResponse, respPayload)
	if err != nil {
		return "", fmt.Errorf("failed to send HandshakeResponse: %w", err)
	}

	log.Printf("Handshake successful with client: %s", reqPayload.ClientName)
	return reqPayload.ClientName, nil
}

// Run starts the server's main loop. It performs the initial handshake
// and then enters a loop to continuously receive and dispatch messages
// to registered handlers or default handlers.
// This method blocks until a fatal error occurs or the connection is closed.
func (s *Server) Run() error {
	log.Printf("Server '%s' starting...", s.serverName)

	// 1. Perform Handshake
	clientName, err := s.handleHandshake()
	if err != nil {
		log.Printf("Handshake failed: %v", err)
		// Handshake errors are returned directly, no extra HandshakeFailed needed here
		return fmt.Errorf("handshake failed: %w", err)
	}
	log.Printf("Handshake successful with client: %s", clientName)

	// 2. Main Message Loop with Dispatch
	log.Println("Entering main message loop...")
	for {
		msg, err := s.conn.ReceiveMessage()
		if err != nil {
			if err.Error() == "failed to read message line: EOF" || strings.Contains(err.Error(), "EOF") {
				log.Println("Client disconnected (EOF received). Server shutting down.")
				return nil // Clean exit
			}
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			return err // Return other receive errors
		}

		log.Printf("Received message type: %s", msg.MessageType)
		var handlerErr error

		// Dispatch based on message type
		switch msg.MessageType {
		case MessageTypeToolDefinitionRequest:
			handlerErr = s.handleToolDefinitionRequest(msg) // Call internal handler
		case MessageTypeUseToolRequest:
			handlerErr = s.handleUseToolRequest(msg) // Call internal handler
		// TODO: Add cases for ResourceAccessRequest etc.
		default:
			// Handle unknown message types
			log.Printf("Handler not implemented for message type: %s", msg.MessageType)
			handlerErr = s.conn.SendMessage(MessageTypeError, ErrorPayload{
				Code:    ErrorCodeMCPNotImplemented, // Use correct MCP code
				Message: fmt.Sprintf("Message type '%s' not implemented by server", msg.MessageType),
			})
		}

		// Check for errors during handling (especially sending response/error)
		if handlerErr != nil {
			log.Printf("Error handling message type %s: %v", msg.MessageType, handlerErr)
			// If sending the response/error failed, the connection is likely broken, so exit.
			if strings.Contains(handlerErr.Error(), "write") || strings.Contains(handlerErr.Error(), "pipe") {
				log.Println("Detected write error, assuming client disconnected. Shutting down.")
				return handlerErr // Return the underlying write error
			}
			// Otherwise, log the error but continue the loop. The handler itself
			// should have sent an MCP Error message if it was a recoverable error.
		}
	}
}

// handleToolDefinitionRequest is the internal handler called by Run.
// It sends back the list of tools registered with the server.
func (s *Server) handleToolDefinitionRequest(requestMsg *Message) error { // Use Message directly
	log.Println("Handling ToolDefinitionRequest")
	tools := make([]ToolDefinition, 0, len(s.toolRegistry))
	// Consider sorting tools by name for consistent output if needed
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	responsePayload := ToolDefinitionResponsePayload{Tools: tools} // Use ToolDefinitionResponsePayload directly
	log.Printf("Sending ToolDefinitionResponse with %d tools", len(tools))
	return s.conn.SendMessage(MessageTypeToolDefinitionResponse, responsePayload)
}

// handleUseToolRequest is the internal handler called by Run.
// It unmarshals the request, finds the appropriate tool handler, executes it,
// and sends back the result or an error.
func (s *Server) handleUseToolRequest(requestMsg *Message) error { // Use Message directly
	log.Println("Handling UseToolRequest")
	var requestPayload UseToolRequestPayload // Use UseToolRequestPayload directly
	err := UnmarshalPayload(requestMsg.Payload, &requestPayload)
	if err != nil {
		log.Printf("Error unmarshalling UseToolRequest payload: %v", err)
		return s.conn.SendMessage(MessageTypeError, ErrorPayload{ // Use ErrorPayload directly
			Code:    ErrorCodeMCPInvalidPayload, // Use correct MCP code
			Message: fmt.Sprintf("Failed to unmarshal UseToolRequest payload: %v", err),
		})
	}

	log.Printf("Requested tool: %s with args: %v", requestPayload.ToolName, requestPayload.Arguments)

	// Find the handler for the requested tool
	handler, exists := s.toolHandlers[requestPayload.ToolName]
	if !exists {
		log.Printf("Tool not found or no handler registered: %s", requestPayload.ToolName)
		return s.conn.SendMessage(MessageTypeError, ErrorPayload{ // Use ErrorPayload directly
			Code:    ErrorCodeMCPToolNotFound, // Use correct MCP code
			Message: fmt.Sprintf("Tool '%s' not found or not implemented", requestPayload.ToolName),
		})
	}

	// --- Execute the registered tool handler ---
	// The handler function is responsible for its own argument validation and execution logic.
	result, execErr := handler(requestPayload.Arguments)
	// --- End tool execution ---

	// Send response (either result or error returned by the handler)
	if execErr != nil {
		// Ensure the error code is set, default if necessary
		if execErr.Code == 0 { // Check against zero value for int
			execErr.Code = ErrorCodeMCPToolExecutionError // Use correct MCP code
		}
		// Log the numeric code
		log.Printf("Tool '%s' execution failed: [%d] %s", requestPayload.ToolName, execErr.Code, execErr.Message)
		return s.conn.SendMessage(MessageTypeError, *execErr) // Use ErrorPayload directly
	}

	// Send successful UseToolResponse
	log.Printf("Tool '%s' execution successful.", requestPayload.ToolName)
	responsePayload := UseToolResponsePayload{Result: result} // Use UseToolResponsePayload directly
	return s.conn.SendMessage(MessageTypeUseToolResponse, responsePayload)
}
