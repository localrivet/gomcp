// server.go (Modified)
package mcp

import (
	// Required for UnmarshalPayload usage within server logic
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings" // Needed for error check in loop
)

// ToolHandlerFunc defines the signature for functions that handle tool execution.
// It receives the arguments provided by the client and should return the result
// content and a boolean indicating if an error occurred.
// TODO: Refine this signature further. How to pass structured content? How to detail errors?
type ToolHandlerFunc func(arguments map[string]interface{}) (content []Content, isError bool)

// Server represents an MCP server instance. It manages the connection,
// handles the handshake/initialization, tool registration, and processes incoming messages.
type Server struct {
	conn         *Connection // The underlying MCP connection handler
	serverName   string
	toolRegistry map[string]Tool            // Stores tool definitions (now using Tool struct)
	toolHandlers map[string]ToolHandlerFunc // Stores handlers for each tool
	// Store client/server capabilities after handshake
	clientCapabilities ClientCapabilities // Capabilities supported by the connected client
	serverCapabilities ServerCapabilities // Capabilities supported by this server
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
		toolRegistry: make(map[string]Tool), // Use Tool struct
		toolHandlers: make(map[string]ToolHandlerFunc),
	}
}

// RegisterTool adds a tool definition and its corresponding handler function to the server.
// It returns an error if a tool with the same name is already registered or handler is nil.
func (s *Server) RegisterTool(tool Tool, handler ToolHandlerFunc) error { // Accept Tool struct
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := s.toolRegistry[tool.Name]; exists {
		return fmt.Errorf("tool '%s' already registered", tool.Name)
	}
	if handler == nil {
		return fmt.Errorf("handler for tool '%s' cannot be nil", tool.Name)
	}
	s.toolRegistry[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
	log.Printf("Registered tool: %s", tool.Name)
	return nil
}

// handleInitialize performs the server side of the MCP initialization protocol.
// It waits for an InitializeRequest, validates the protocol version,
// determines capabilities, and sends back either an InitializeResponse or an Error message.
// It also waits for the client's InitializedNotification.
// Returns the client's info/capabilities on success.
func (s *Server) handleInitialize() (clientInfo Implementation, clientCapabilities ClientCapabilities, err error) {
	log.Println("Waiting for InitializeRequest...")
	// Receive raw JSON
	rawJSON, err := s.conn.ReceiveRawMessage()
	if err != nil {
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to receive initial message: %w", err)
	}

	// Attempt to unmarshal into JSONRPCRequest
	var jsonrpcReq JSONRPCRequest
	err = json.Unmarshal(rawJSON, &jsonrpcReq)
	if err != nil {
		// If basic unmarshal fails, send ParseError with null ID
		_ = s.conn.SendErrorResponse(nil, ErrorPayload{Code: ErrorCodeParseError, Message: fmt.Sprintf("Failed to parse incoming JSON: %v", err)})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to parse incoming JSON: %w", err)
	}

	// Check if it's a valid request with the correct method
	if jsonrpcReq.Method != MethodInitialize {
		errMsg := fmt.Sprintf("Expected method '%s', got '%s'", MethodInitialize, jsonrpcReq.Method)
		_ = s.conn.SendErrorResponse(jsonrpcReq.ID, ErrorPayload{Code: ErrorCodeMethodNotFound, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg)
	}

	// Unmarshal params from the request
	var reqParams InitializeRequestParams
	if jsonrpcReq.Params == nil {
		errMsg := "Missing params for InitializeRequest"
		_ = s.conn.SendErrorResponse(jsonrpcReq.ID, ErrorPayload{Code: ErrorCodeInvalidParams, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg)
	}
	paramsBytes, err := json.Marshal(jsonrpcReq.Params)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to re-marshal InitializeRequest params: %v", err)
		_ = s.conn.SendErrorResponse(jsonrpcReq.ID, ErrorPayload{Code: ErrorCodeInvalidParams, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to re-marshal InitializeRequest params: %w", err)
	}
	err = json.Unmarshal(paramsBytes, &reqParams)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to unmarshal InitializeRequest params: %v", err)
		_ = s.conn.SendErrorResponse(jsonrpcReq.ID, ErrorPayload{Code: ErrorCodeInvalidParams, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to unmarshal InitializeRequest params: %w", err)
	}

	// Basic validation of received params (ProtocolVersion checked later)
	if reqParams.ProtocolVersion == "" {
		errMsg := "malformed InitializeRequest params: missing protocolVersion"
		_ = s.conn.SendErrorResponse(jsonrpcReq.ID, ErrorPayload{Code: ErrorCodeInvalidParams, Message: errMsg}) // Use jsonrpcReq.ID
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg)                                  // Use %s
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
		_ = s.conn.SendErrorResponse(jsonrpcReq.ID, ErrorPayload{Code: ErrorCodeMCPUnsupportedProtocolVersion, Message: errMsg}) // Use jsonrpcReq.ID
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg)                                                  // Use %s
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
	// We need a way to associate response with request ID.
	// Use SendResponse with the original request ID.
	err = s.conn.SendResponse(jsonrpcReq.ID, initResult) // Use jsonrpcReq.ID
	if err != nil {
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to send InitializeResponse: %w", err)
	}

	// --- Wait for Initialized Notification ---
	// The client MUST send this after receiving the InitializeResponse.
	log.Println("Waiting for InitializedNotification...")
	rawJSONInit, err := s.conn.ReceiveRawMessage() // Use ReceiveRawMessage
	if err != nil {
		// If client disconnects here, maybe it's okay? Or maybe handshake failed implicitly.
		log.Printf("Failed to receive InitializedNotification: %v", err)
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("failed to receive InitializedNotification: %w", err)
	}
	// Attempt to unmarshal into JSONRPCNotification
	var jsonrpcNotif JSONRPCNotification
	err = json.Unmarshal(rawJSONInit, &jsonrpcNotif)
	if err != nil {
		// Invalid JSON received where notification was expected
		errMsg := fmt.Sprintf("Failed to parse InitializedNotification JSON: %v", err)
		_ = s.conn.SendErrorResponse(nil, ErrorPayload{Code: ErrorCodeParseError, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg)
	}
	// Check if it's the correct method
	if jsonrpcNotif.Method != MethodInitialized {
		errMsg := fmt.Sprintf("Expected '%s' notification after initialize response, got method '%s'", MethodInitialized, jsonrpcNotif.Method)
		// This is a protocol violation by the client. Send an error.
		_ = s.conn.SendErrorResponse(nil, ErrorPayload{Code: ErrorCodeInvalidRequest, Message: errMsg})
		return Implementation{}, ClientCapabilities{}, fmt.Errorf("%s", errMsg)
	}
	log.Println("Received InitializedNotification from client.")
	// --- End Initialized Notification ---

	log.Printf("Initialization successful with client: %s", reqParams.ClientInfo.Name)
	// Store capabilities on the server struct
	s.clientCapabilities = reqParams.Capabilities
	s.serverCapabilities = serverCapabilities // Stored from earlier definition
	// Return the client's info and capabilities, and nil error
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
	// Capabilities are now stored in s.clientCapabilities and s.serverCapabilities

	// 2. Main Message Loop with Dispatch
	log.Println("Entering main message loop...")
	for {
		// Receive raw JSON
		rawJSON, err := s.conn.ReceiveRawMessage()
		if err != nil {
			if err.Error() == "failed to read message line: EOF" || strings.Contains(err.Error(), "EOF") {
				log.Println("Client disconnected (EOF received). Server shutting down.")
				return nil // Clean exit
			}
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			return err
		}

		// Attempt to unmarshal into a generic structure to determine type (Req/Notif)
		var baseMessage struct {
			JSONRPC string      `json:"jsonrpc"`
			ID      interface{} `json:"id"`     // Present in requests, missing in notifications
			Method  string      `json:"method"` // Present in requests and notifications
			Params  interface{} `json:"params"` // Present in requests and notifications
			Result  interface{} `json:"result"` // Present in responses
			Error   interface{} `json:"error"`  // Present in error responses
		}
		err = json.Unmarshal(rawJSON, &baseMessage)
		if err != nil {
			// This should ideally not happen if ReceiveRawMessage validated JSON
			log.Printf("Failed to unmarshal base JSON-RPC structure: %v. Raw: %s", err, string(rawJSON))
			_ = s.conn.SendErrorResponse(nil, ErrorPayload{Code: ErrorCodeParseError, Message: fmt.Sprintf("Failed to parse JSON: %v", err)})
			continue // Skip this message
		}

		// Basic validation
		if baseMessage.JSONRPC != "2.0" {
			log.Printf("Received message with invalid jsonrpc version: %s", baseMessage.JSONRPC)
			_ = s.conn.SendErrorResponse(baseMessage.ID, ErrorPayload{Code: ErrorCodeInvalidRequest, Message: "Invalid jsonrpc version"})
			continue
		}

		// Determine if it's a Request or Notification based on ID presence
		isRequest := baseMessage.ID != nil
		isNotification := baseMessage.ID == nil && baseMessage.Method != ""

		if !isRequest && !isNotification {
			log.Printf("Received message that is neither a valid Request nor Notification. Raw: %s", string(rawJSON))
			_ = s.conn.SendErrorResponse(baseMessage.ID, ErrorPayload{Code: ErrorCodeInvalidRequest, Message: "Invalid message structure"})
			continue
		}

		// We only handle Requests in this loop for now (dispatching based on Method)
		// TODO: Handle notifications separately if needed
		if !isRequest {
			log.Printf("Received notification (method: %s), ignoring.", baseMessage.Method)
			continue
		}
		log.Printf("Received request: Method=%s, ID=%v", baseMessage.Method, baseMessage.ID)
		var handlerErr error

		// Dispatch based on message method
		switch baseMessage.Method {
		case MethodListTools:
			handlerErr = s.handleListToolsRequest(baseMessage.ID, baseMessage.Params)
		case MethodCallTool:
			handlerErr = s.handleCallToolRequest(baseMessage.ID, baseMessage.Params)
		case MethodListResources: // Add case for listing resources
			handlerErr = s.handleListResources(baseMessage.ID, baseMessage.Params)
		case MethodReadResource: // Add case for reading resource
			handlerErr = s.handleReadResource(baseMessage.ID, baseMessage.Params)
		// TODO: Add cases for Resource subscription etc.
		default:
			// Handle unknown methods
			log.Printf("Method not implemented: %s", baseMessage.Method)
			handlerErr = s.conn.SendErrorResponse(baseMessage.ID, ErrorPayload{
				Code:    ErrorCodeMethodNotFound,
				Message: fmt.Sprintf("Method '%s' not implemented by server", baseMessage.Method),
			})
		}

		// Check for errors during handling (especially sending response/error)
		if handlerErr != nil {
			log.Printf("Error handling method %s (ID: %v): %v", baseMessage.Method, baseMessage.ID, handlerErr)
			// If sending the response/error failed, the connection is likely broken, so exit.
			if strings.Contains(handlerErr.Error(), "write") || strings.Contains(handlerErr.Error(), "pipe") {
				log.Println("Detected write error, assuming client disconnected. Shutting down.")
				return handlerErr // Return the underlying write error
			}
		}
	}
}

// handleListToolsRequest handles the 'tools/list' request.
// Accepts request ID and params directly.
func (s *Server) handleListToolsRequest(requestID interface{}, params interface{}) error {
	log.Println("Handling ListToolsRequest")
	// TODO: Unmarshal params into ListToolsRequestParams if pagination is added
	// var listParams ListToolsRequestParams
	// if params != nil { ... unmarshal ... }

	tools := make([]Tool, 0, len(s.toolRegistry)) // Use Tool struct
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	responsePayload := ListToolsResult{Tools: tools} // Use ListToolsResult
	log.Printf("Sending ListToolsResponse with %d tools", len(tools))
	// Use SendResponse with the passed request ID
	return s.conn.SendResponse(requestID, responsePayload)
}

// handleCallToolRequest handles the 'tools/call' request.
// Accepts request ID and params directly.
func (s *Server) handleCallToolRequest(requestID interface{}, params interface{}) error {
	log.Println("Handling CallToolRequest")
	var requestParams CallToolParams // Use CallToolParams

	// Unmarshal params
	if params == nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: "Missing params for tools/call",
		})
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal CallTool params: %v", err),
		})
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		log.Printf("Error unmarshalling CallTool params: %v", err)
		return s.conn.SendErrorResponse(requestID, ErrorPayload{ // Use requestID
			Code:    ErrorCodeInvalidParams, // Use standard JSON-RPC code
			Message: fmt.Sprintf("Failed to unmarshal CallTool params: %v", err),
		})
	}

	log.Printf("Requested tool: %s with args: %v", requestParams.Name, requestParams.Arguments) // Use Name field

	// Find the handler
	handler, exists := s.toolHandlers[requestParams.Name]
	if !exists {
		log.Printf("Tool not found or no handler registered: %s", requestParams.Name)
		return s.conn.SendErrorResponse(requestID, ErrorPayload{ // Use requestID
			Code:    ErrorCodeMCPToolNotFound, // Use MCP specific code
			Message: fmt.Sprintf("Tool '%s' not found or not implemented", requestParams.Name),
		})
	}

	// Execute the handler
	content, isError := handler(requestParams.Arguments)

	// Construct the CallToolResult payload
	responsePayload := CallToolResult{
		Content: content,
	}
	if isError {
		responsePayload.IsError = &isError // Assign pointer to true only if error occurred
		log.Printf("Tool '%s' execution finished with error.", requestParams.Name)
		// Note: Tool errors are reported in the result, not as protocol errors.
		// The handler function itself should format the error message within the returned Content.
	} else {
		log.Printf("Tool '%s' execution successful.", requestParams.Name)
	}

	// Send the response
	// Use SendResponse with the original request ID
	return s.conn.SendResponse(requestID, responsePayload) // Use requestID
}

// handleListResources handles the 'resources/list' request.
// TODO: Implement actual resource listing and pagination.
func (s *Server) handleListResources(requestID interface{}, params interface{}) error {
	log.Println("Handling ListResourcesRequest (stub)")
	// TODO: Unmarshal params ListResourcesRequestParams
	responsePayload := ListResourcesResult{
		Resources: []Resource{}, // Return empty list for now
	}
	return s.conn.SendResponse(requestID, responsePayload)
}

// handleReadResource handles the 'resources/read' request.
// TODO: Implement actual resource reading.
func (s *Server) handleReadResource(requestID interface{}, params interface{}) error {
	log.Println("Handling ReadResourceRequest (stub)")
	var requestParams ReadResourceRequestParams
	// Unmarshal params
	if params == nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: "Missing params for resources/read",
		})
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal ReadResource params: %v", err),
		})
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal ReadResource params: %v", err),
		})
	}

	// For now, always return not found
	log.Printf("ReadResource requested for URI: %s (returning Not Found)", requestParams.URI)
	return s.conn.SendErrorResponse(requestID, ErrorPayload{
		Code:    ErrorCodeMCPResourceNotFound, // Use placeholder code
		Message: fmt.Sprintf("Resource not found (stub implementation): %s", requestParams.URI),
	})
}
