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
// handles the handshake/initialization, tool registration, and processes incoming messages.
type Server struct {
	conn         *Connection // The underlying MCP connection handler
	serverName   string
	toolRegistry map[string]ToolDefinition  // Stores tool definitions
	toolHandlers map[string]ToolHandlerFunc // Stores handlers for each tool
	// TODO: Store client/server capabilities after handshake
	// clientCapabilities ClientCapabilities
	// serverCapabilities ServerCapabilities
}

// NewServer creates and initializes a new MCP Server instance using stdio.
func NewServer(serverName string) *Server {
	return NewServerWithConnection(serverName, NewStdioConnection())
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

// handleInitialize performs the server side of the MCP initialization protocol.
// It waits for an InitializeRequest, validates the protocol version,
// determines capabilities, and sends back either an InitializeResponse or an Error message.
// It also waits for the client's InitializedNotification.
// Returns the client's info/capabilities on success.
func (s *Server) handleInitialize() (clientInfo Implementation, clientCapabilities ClientCapabilities, err error) {
	log.Println("Waiting for InitializeRequest...")
	msg, err := s.conn.ReceiveMessage()
	if err != nil {
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to receive initial message: %w", err)
	}

	// Check message type (method)
	// TODO: This check might be redundant if transport layer handles JSON-RPC method/id directly
	if msg.MessageType != MethodInitialize { // Use MethodInitialize constant
		errMsg := fmt.Sprintf("Expected '%s' request, got '%s'", MethodInitialize, msg.MessageType)
		// Use standard JSON-RPC code if method is wrong
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeMethodNotFound, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg) // Use %s
	}

	// Unmarshal params
	var reqParams InitializeRequestParams
	// TODO: ReceiveMessage should ideally return params directly for requests
	err = UnmarshalPayload(msg.Payload, &reqParams)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to unmarshal InitializeRequest params: %v", err)
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeInvalidParams, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to unmarshal InitializeRequest params: %w", err)
	}

	// Basic validation of received params
	if reqParams.ProtocolVersion == "" {
		errMsg := "malformed InitializeRequest params: missing protocolVersion"
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeInvalidParams, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg) // Use %s
	}
	if reqParams.ClientInfo.Name == "" {
		log.Println("Warning: Received InitializeRequest with missing clientInfo.name")
	}

	log.Printf("Received InitializeRequest from client: %s (Version: %s)", reqParams.ClientInfo.Name, reqParams.ClientInfo.Version)
	log.Printf("Client Capabilities: %+v", reqParams.Capabilities) // Log received capabilities

	// --- Protocol Version Negotiation ---
	// For now, we only support CurrentProtocolVersion exactly.
	if reqParams.ProtocolVersion != CurrentProtocolVersion {
		errMsg := fmt.Sprintf("Unsupported protocol version '%s' requested by client. Server requires '%s'.", reqParams.ProtocolVersion, CurrentProtocolVersion)
		// Send specific MCP error code
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeMCPUnsupportedProtocolVersion, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg) // Use %s
	}
	selectedVersion := CurrentProtocolVersion
	// --- End Version Negotiation ---

	// --- Define Server Capabilities ---
	// Advertise the capabilities this server supports.
	serverCapabilities := ServerCapabilities{
		// Indicate tool support if tools are registered
		Tools: &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{ListChanged: false},
		// Add other capabilities like Logging, Resources etc. here if implemented
		// Experimental: map[string]interface{}{"featureX": true},
	}
	if len(s.toolRegistry) == 0 {
		serverCapabilities.Tools = nil // Don't advertise tools if none registered
	}

	// Define server info
	serverInfo := Implementation{
		Name:    s.serverName,
		Version: "0.1.0", // Example server version
	}

	// --- Send InitializeResponse ---
	// Construct the result part of the JSON-RPC response
	initResult := InitializeResult{
		ProtocolVersion: selectedVersion,
		Capabilities:    serverCapabilities,
		ServerInfo:      serverInfo,
		// Instructions: "Optional instructions for the client/LLM...",
	}

	// Send the response.
	// NOTE: SendMessage currently wraps this in a basic Message struct.
	// A strict JSON-RPC layer would construct a JSONRPCResponse with id, jsonrpc, result.
	// We also need the original request ID (msg.MessageID) to put in the response.
	// Let's modify SendMessage slightly or add SendResponse later.
	// For now, sending the payload directly with a conceptual type.
	log.Printf("Sending InitializeResponse: %+v", initResult)
	// We need a way to associate response with request ID. Let's assume SendMessage handles it magically for now.
	// A conceptual "InitializeResponse" type is used here. This needs fixing.
	// TODO: Refactor SendMessage/ReceiveMessage for proper JSON-RPC request/response/notification handling.
	err = s.conn.SendMessage("InitializeResponse", initResult) // Conceptual type - NEEDS FIXING
	if err != nil {
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to send InitializeResponse: %w", err)
	}

	// --- Wait for Initialized Notification ---
	// The client MUST send this after receiving the InitializeResponse.
	log.Println("Waiting for InitializedNotification...")
	initMsg, err := s.conn.ReceiveMessage()
	if err != nil {
		// If client disconnects here, maybe it's okay? Or maybe handshake failed implicitly.
		log.Printf("Failed to receive InitializedNotification: %v", err)
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to receive InitializedNotification: %w", err)
	}
	// TODO: This check might be redundant if transport layer handles JSON-RPC method/id directly
	if initMsg.MessageType != MethodInitialized { // Use MethodInitialized constant
		errMsg := fmt.Sprintf("Expected '%s' notification after initialize response, got '%s'", MethodInitialized, initMsg.MessageType)
		// This is a protocol violation by the client. Send an error.
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{Code: ErrorCodeInvalidRequest, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg) // Use %s
	}
	log.Println("Received InitializedNotification from client.")
	// --- End Initialized Notification ---

	log.Printf("Initialization successful with client: %s", reqParams.ClientInfo.Name)
	// Return the client's info and capabilities, and nil error
	// TODO: Store these capabilities on the server struct
	return reqParams.ClientInfo, reqParams.Capabilities, nil
}

// Run starts the server's main loop. It performs the initial handshake/initialization
// and then enters a loop to continuously receive and dispatch messages
// to registered handlers or default handlers.
// This method blocks until a fatal error occurs or the connection is closed.
func (s *Server) Run() error {
	log.Printf("Server '%s' starting...", s.serverName)

	// 1. Perform Initialization (replaces handshake)
	clientInfo, clientCaps, err := s.handleInitialize() // Use new initialize handler
	if err != nil {
		log.Printf("Initialization failed: %v", err)
		// Initialization errors are returned directly
		return fmt.Errorf("initialization failed: %w", err)
	}
	log.Printf("Initialization successful with client: %s %+v", clientInfo.Name, clientCaps)
	// TODO: Store clientCaps on s.clientCapabilities

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
		// TODO: Update these case statements to use Method constants (e.g., MethodListTools)
		switch msg.MessageType {
		case MessageTypeToolDefinitionRequest: // To become MethodListTools
			handlerErr = s.handleToolDefinitionRequest(msg) // Call internal handler
		case MessageTypeUseToolRequest: // To become MethodCallTool
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
		}
	}
}

// handleToolDefinitionRequest needs renaming to handleListToolsRequest
// ... (implementation remains similar for now) ...
func (s *Server) handleToolDefinitionRequest(requestMsg *Message) error {
	log.Println("Handling ToolDefinitionRequest (soon ListToolsRequest)")
	tools := make([]ToolDefinition, 0, len(s.toolRegistry))
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	responsePayload := ToolDefinitionResponsePayload{Tools: tools}
	log.Printf("Sending ToolDefinitionResponse with %d tools", len(tools))
	// TODO: Update MessageType when renaming is done
	return s.conn.SendMessage(MessageTypeToolDefinitionResponse, responsePayload)
}

// handleUseToolRequest needs renaming to handleCallToolRequest
// ... (implementation needs changes for CallToolResult structure) ...
func (s *Server) handleUseToolRequest(requestMsg *Message) error {
	log.Println("Handling UseToolRequest (soon CallToolRequest)")
	var requestPayload UseToolRequestPayload
	err := UnmarshalPayload(requestMsg.Payload, &requestPayload)
	if err != nil {
		log.Printf("Error unmarshalling UseToolRequest payload: %v", err)
		return s.conn.SendMessage(MessageTypeError, ErrorPayload{
			Code:    ErrorCodeMCPInvalidPayload,
			Message: fmt.Sprintf("Failed to unmarshal UseToolRequest payload: %v", err),
		})
	}

	log.Printf("Requested tool: %s with args: %v", requestPayload.ToolName, requestPayload.Arguments)

	handler, exists := s.toolHandlers[requestPayload.ToolName]
	if !exists {
		log.Printf("Tool not found or no handler registered: %s", requestPayload.ToolName)
		return s.conn.SendMessage(MessageTypeError, ErrorPayload{
			Code:    ErrorCodeMCPToolNotFound,
			Message: fmt.Sprintf("Tool '%s' not found or not implemented", requestPayload.ToolName),
		})
	}

	result, execErr := handler(requestPayload.Arguments)

	if execErr != nil {
		if execErr.Code == 0 {
			execErr.Code = ErrorCodeMCPToolExecutionError
		}
		log.Printf("Tool '%s' execution failed: [%d] %s", requestPayload.ToolName, execErr.Code, execErr.Message)
		return s.conn.SendMessage(MessageTypeError, *execErr)
	}

	log.Printf("Tool '%s' execution successful.", requestPayload.ToolName)
	// TODO: Refactor response to use CallToolResult structure (content array, isError bool)
	responsePayload := UseToolResponsePayload{Result: result}
	// TODO: Update MessageType when renaming is done
	return s.conn.SendMessage(MessageTypeUseToolResponse, responsePayload)
}
