// server.go (Modified)
package gomcp

import (
	// Needed for cancellation
	// Required for UnmarshalPayload usage within server logic
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings" // Needed for error check in loop
	"sync"
)

// ToolHandlerFunc defines the signature for functions that handle tool execution.
// It receives the arguments provided by the client and should return the result
// content and a boolean indicating if an error occurred. It also receives a context.Context
// (for cancellation) and an optional ProgressToken (which may be nil).
// TODO: Refine this signature further. How to pass structured content? How to detail errors?
type ToolHandlerFunc func(ctx context.Context, progressToken *ProgressToken, arguments map[string]interface{}) (content []Content, isError bool)

// Server represents an MCP server instance. It manages the connection,
// handles the handshake/initialization, tool registration, and processes incoming messages.
type Server struct {
	conn             *Connection // The underlying MCP connection handler
	serverName       string
	toolRegistry     map[string]Tool            // Stores tool definitions (now using Tool struct)
	toolHandlers     map[string]ToolHandlerFunc // Stores handlers for each tool
	resourceRegistry map[string]Resource        // Stores available resources (URI -> Resource)
	promptRegistry   map[string]Prompt          // Stores available prompts (URI -> Prompt)
	// Store client/server capabilities after handshake
	clientCapabilities ClientCapabilities // Capabilities supported by the connected client
	serverCapabilities ServerCapabilities // Capabilities supported by this server

	// For handling client-to-server notifications
	notificationHandlers map[string]func(params interface{}) // Map method name to handler
	notificationMu       sync.Mutex                          // Mutex to protect notificationHandlers map

	// For managing cancellation of active requests
	activeRequests map[string]context.CancelFunc // Map request ID (as string) to its cancel function
	requestMu      sync.Mutex                    // Mutex to protect activeRequests map

	// For managing resource subscriptions (URI -> subscribed?)
	// Note: Assumes one client per server instance for now.
	resourceSubscriptions map[string]bool // Map resource URI to subscription status
	subscriptionMu        sync.Mutex      // Mutex to protect resourceSubscriptions map
}

// NewServer creates and initializes a new MCP Server instance using stdio.
func NewServer(serverName string) *Server {
	return NewServerWithConnection(serverName, NewStdioConnection())
}

// NewServerWithConnection creates and initializes a new MCP Server instance
// using the provided gomcp.Connection. This is useful for testing or integrating
// with different transport mechanisms.
func NewServerWithConnection(serverName string, conn *Connection) *Server {
	log.SetOutput(os.Stderr) // Still configure logging
	log.SetFlags(log.Ltime | log.Lshortfile)

	if conn == nil {
		log.Println("Warning: NewServerWithConnection received nil connection, falling back to stdio.")
		conn = NewStdioConnection()
	}
	// Assign to variable first
	srv := &Server{
		conn:                  conn, // Use provided connection
		serverName:            serverName,
		toolRegistry:          make(map[string]Tool), // Use Tool struct
		toolHandlers:          make(map[string]ToolHandlerFunc),
		resourceRegistry:      make(map[string]Resource),                 // Initialize map
		promptRegistry:        make(map[string]Prompt),                   // Initialize map
		notificationHandlers:  make(map[string]func(params interface{})), // Initialize map
		activeRequests:        make(map[string]context.CancelFunc),       // Initialize map
		resourceSubscriptions: make(map[string]bool),                     // Initialize map
	}
	// Register the built-in handler for cancellation notifications
	srv.RegisterNotificationHandler(MethodCancelled, srv.handleCancellationNotification)
	return srv
}

// Conn returns the underlying Connection object for the server.
// Use with caution, primarily for sending custom responses/errors if needed.
func (s *Server) Conn() *Connection {
	return s.conn
}

// RegisterNotificationHandler registers a handler function for a specific client-to-server notification method.
func (s *Server) RegisterNotificationHandler(method string, handler func(params interface{})) error {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()
	if _, exists := s.notificationHandlers[method]; exists {
		return fmt.Errorf("notification handler already registered for method: %s", method)
	}
	s.notificationHandlers[method] = handler
	log.Printf("Registered notification handler for method: %s", method)
	return nil
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

	// Send notification if server supports it (and presumably connection is active)
	// Note: This assumes RegisterTool is called *after* initialization is complete
	// and s.serverCapabilities is populated. A check for active connection might also be needed.
	if s.serverCapabilities.Tools != nil && s.serverCapabilities.Tools.ListChanged {
		log.Printf("Sending tools/list_changed notification to client")
		err := s.SendToolsListChanged()
		if err != nil {
			// Log error but don't fail registration
			log.Printf("Warning: failed to send tools/list_changed notification: %v", err)
		}
	}

	return nil
}

// RegisterResource adds or updates a resource in the server's registry.
// It sends a resources/list_changed notification if supported.
func (s *Server) RegisterResource(resource Resource) error {
	if resource.URI == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	// TODO: Add locking if registry access needs to be thread-safe
	s.resourceRegistry[resource.URI] = resource
	// Use mutex for thread-safe access to the registry
	s.subscriptionMu.Lock() // Assuming resource registry and subscriptions might be related, use same mutex for now
	s.resourceRegistry[resource.URI] = resource
	s.subscriptionMu.Unlock()
	log.Printf("Registered/Updated resource: %s", resource.URI)

	// Send notification if server supports it and connection is established (capabilities are set)
	// Note: This assumes RegisterResource is called *after* initialization.
	if s.serverCapabilities.Resources != nil && s.serverCapabilities.Resources.ListChanged {
		log.Printf("Sending resources/list_changed notification to client")
		err := s.SendResourcesListChanged() // Use the specific sender method
		if err != nil {
			log.Printf("Warning: failed to send resources/list_changed notification: %v", err)
		}
	}
	return nil
}

// UnregisterResource removes a resource from the server's registry.
// It sends a resources/list_changed notification if supported.
func (s *Server) UnregisterResource(uri string) error {
	if uri == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	// TODO: Add locking if registry access needs to be thread-safe
	_, exists := s.resourceRegistry[uri]
	if !exists {
		return fmt.Errorf("resource '%s' not found", uri)
	}
	delete(s.resourceRegistry, uri)
	// Also remove any subscriptions for this resource
	delete(s.resourceSubscriptions, uri)
	s.subscriptionMu.Unlock() // Unlock after modifications
	log.Printf("Unregistered resource: %s", uri)

	// Send notification if server supports it and connection is established
	if s.serverCapabilities.Resources != nil && s.serverCapabilities.Resources.ListChanged {
		log.Printf("Sending resources/list_changed notification to client")
		err := s.SendResourcesListChanged() // Use the specific sender method
		if err != nil {
			log.Printf("Warning: failed to send resources/list_changed notification: %v", err)
		}
	}
	return nil
}

// ResourceRegistry returns a read-only copy of the current resource registry.
// Note: Modifying the returned map will not affect the server's internal state.
// Use RegisterResource/UnregisterResource for modifications.
func (s *Server) ResourceRegistry() map[string]Resource {
	// Using the subscription mutex for simplicity as both registries might be accessed together.
	// Consider separate mutexes if contention becomes an issue.
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()
	registryCopy := make(map[string]Resource, len(s.resourceRegistry))
	for k, v := range s.resourceRegistry {
		registryCopy[k] = v
	}
	return registryCopy
}

// RegisterPrompt adds or updates a prompt in the server's registry.
// It sends a prompts/list_changed notification if supported.
func (s *Server) RegisterPrompt(prompt Prompt) error {
	if prompt.URI == "" {
		return fmt.Errorf("prompt URI cannot be empty")
	}
	// TODO: Add locking if registry access needs to be thread-safe
	s.promptRegistry[prompt.URI] = prompt
	log.Printf("Registered/Updated prompt: %s", prompt.URI)

	// Send notification if server supports it
	if s.serverCapabilities.Prompts != nil && s.serverCapabilities.Prompts.ListChanged {
		log.Printf("Sending prompts/list_changed notification to client")
		err := s.SendPromptsListChanged()
		if err != nil {
			log.Printf("Warning: failed to send prompts/list_changed notification: %v", err)
		}
	}
	return nil
}

// UnregisterPrompt removes a prompt from the server's registry.
// It sends a prompts/list_changed notification if supported.
func (s *Server) UnregisterPrompt(uri string) error {
	if uri == "" {
		return fmt.Errorf("prompt URI cannot be empty")
	}
	// TODO: Add locking if registry access needs to be thread-safe
	_, exists := s.promptRegistry[uri]
	if !exists {
		return fmt.Errorf("prompt '%s' not found", uri)
	}
	delete(s.promptRegistry, uri)
	log.Printf("Unregistered prompt: %s", uri)

	// Send notification if server supports it
	if s.serverCapabilities.Prompts != nil && s.serverCapabilities.Prompts.ListChanged {
		log.Printf("Sending prompts/list_changed notification to client")
		err := s.SendPromptsListChanged()
		if err != nil {
			log.Printf("Warning: failed to send prompts/list_changed notification: %v", err)
		}
	}
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
			// Use errors.Is for robust EOF check, also check for common pipe errors
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || strings.Contains(err.Error(), "pipe closed") {
				log.Println("Client disconnected (EOF or pipe error received). Server shutting down.")
				return nil // Clean exit on expected closure
			}
			// Log and return unexpected errors
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			return err // Return the actual error
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

		if isNotification {
			// --- Dispatch client-to-server notifications ---
			log.Printf("Server received notification: Method=%s", baseMessage.Method)
			s.notificationMu.Lock()
			handler, ok := s.notificationHandlers[baseMessage.Method]
			s.notificationMu.Unlock()

			if ok {
				// Run handler in a new goroutine to avoid blocking the main loop
				go func(params interface{}) {
					// TODO: Consider adding error handling/logging for notification handlers
					handler(params)
				}(baseMessage.Params)
			} else {
				log.Printf("No handler registered for notification method: %s", baseMessage.Method)
			}
			continue // Don't process notifications further in the request dispatch below
		}

		// --- Dispatch client-to-server requests ---
		log.Printf("Server received request: Method=%s, ID=%v", baseMessage.Method, baseMessage.ID)
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
		case MethodListPrompts: // Add case for listing prompts
			handlerErr = s.handleListPrompts(baseMessage.ID, baseMessage.Params)
		case MethodGetPrompt: // Add case for getting prompt
			handlerErr = s.handleGetPrompt(baseMessage.ID, baseMessage.Params)
		case MethodLoggingSetLevel: // Add case for setting log level
			handlerErr = s.handleLoggingSetLevel(baseMessage.ID, baseMessage.Params)
		case MethodPing: // Add case for ping
			handlerErr = s.handlePing(baseMessage.ID, baseMessage.Params)
		case MethodSubscribeResource: // Add case for resource subscription
			handlerErr = s.handleSubscribeResource(baseMessage.ID, baseMessage.Params)
		case MethodUnsubscribeResource: // Add case for resource unsubscription
			handlerErr = s.handleUnsubscribeResource(baseMessage.ID, baseMessage.Params)
		// TODO: Add cases for other Resource/Prompt notifications etc.
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

	// Check for progress token
	var progressToken *ProgressToken
	if requestParams.Meta != nil && requestParams.Meta.ProgressToken != nil {
		progressToken = requestParams.Meta.ProgressToken
		log.Printf("Client requested progress reporting with token: %s", *progressToken)
		// TODO: Pass this token to the handler or use it to wrap the handler
		//       to automatically send $/progress notifications.
	}

	// Find the handler
	handler, exists := s.toolHandlers[requestParams.Name]
	if !exists {
		log.Printf("Tool not found or no handler registered: %s", requestParams.Name)
		return s.conn.SendErrorResponse(requestID, ErrorPayload{ // Use requestID
			Code:    ErrorCodeMCPToolNotFound, // Use MCP specific code
			Message: fmt.Sprintf("Tool '%s' not found or not implemented", requestParams.Name),
		})
	}

	// --- Context Management for Cancellation ---
	// Create a new context for this request that can be cancelled.
	// Use context.Background() as the parent.
	ctx, cancel := context.WithCancel(context.Background())
	// Store the cancel function, associated with the request ID.
	// Convert requestID to string for map key.
	requestIDStr := fmt.Sprintf("%v", requestID)
	s.requestMu.Lock()
	s.activeRequests[requestIDStr] = cancel
	s.requestMu.Unlock()
	// Ensure the cancel function is removed from the map when the handler finishes.
	defer func() {
		s.requestMu.Lock()
		delete(s.activeRequests, requestIDStr)
		s.requestMu.Unlock()
		// Note: We don't call cancel() here automatically. Cancellation is triggered
		// only by the '$/cancelled' notification. If the handler finishes normally,
		// the context associated with it effectively becomes irrelevant.
	}()
	// --- End Context Management ---

	// Execute the handler, passing the cancellable context and optional progress token
	content, isError := handler(ctx, progressToken, requestParams.Arguments) // Pass ctx and progressToken

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

// handleListPrompts handles the 'prompts/list' request.
// TODO: Implement pagination/filtering based on params.
func (s *Server) handleListPrompts(requestID interface{}, params interface{}) error {
	log.Println("Handling ListPromptsRequest")
	// TODO: Unmarshal params ListPromptsRequestParams for pagination/filtering
	// TODO: Add locking if registry access needs to be thread-safe
	prompts := make([]Prompt, 0, len(s.promptRegistry))
	for _, prompt := range s.promptRegistry {
		prompts = append(prompts, prompt)
	}
	responsePayload := ListPromptsResult{
		Prompts: prompts,
		// TODO: Add NextCursor for pagination
	}
	log.Printf("Sending ListPromptsResponse with %d prompts", len(prompts))
	return s.conn.SendResponse(requestID, responsePayload)
}

// handleGetPrompt handles the 'prompts/get' request.
// TODO: Implement actual prompt retrieval.
func (s *Server) handleGetPrompt(requestID interface{}, params interface{}) error {
	log.Println("Handling GetPromptRequest (stub)")
	var requestParams GetPromptRequestParams
	// Unmarshal params
	if params == nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: "Missing params for prompts/get",
		})
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal GetPrompt params: %v", err),
		})
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal GetPrompt params: %v", err),
		})
	}

	// For now, always return not found
	log.Printf("GetPrompt requested for URI: %s (returning Not Found)", requestParams.URI)
	// TODO: Define a more specific error code for prompt not found? Using ResourceNotFound for now.
	return s.conn.SendErrorResponse(requestID, ErrorPayload{
		Code:    ErrorCodeMCPResourceNotFound,
		Message: fmt.Sprintf("Prompt not found (stub implementation): %s", requestParams.URI),
	})
}

// handleListResources handles the 'resources/list' request.
// TODO: Implement pagination/filtering based on params.
func (s *Server) handleListResources(requestID interface{}, params interface{}) error {
	log.Println("Handling ListResourcesRequest")
	// TODO: Unmarshal params ListResourcesRequestParams for pagination/filtering
	// TODO: Add locking if registry access needs to be thread-safe
	resources := make([]Resource, 0, len(s.resourceRegistry))
	for _, resource := range s.resourceRegistry {
		resources = append(resources, resource)
	}
	responsePayload := ListResourcesResult{
		Resources: resources,
		// TODO: Add NextCursor for pagination
	}
	log.Printf("Sending ListResourcesResponse with %d resources", len(resources))
	return s.conn.SendResponse(requestID, responsePayload)
}

// handleLoggingSetLevel handles the 'logging/set_level' request.
// TODO: Implement actual level setting and potentially use it to filter server-sent logs.
func (s *Server) handleLoggingSetLevel(requestID interface{}, params interface{}) error {
	log.Println("Handling LoggingSetLevelRequest (stub)")
	var requestParams SetLevelRequestParams
	// Unmarshal params
	if params == nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: "Missing params for logging/set_level",
		})
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal SetLevel params: %v", err),
		})
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal SetLevel params: %v", err),
		})
	}

	// TODO: Store requestParams.Level on the server or connection state
	log.Printf("Client requested logging level: %s (not yet implemented)", requestParams.Level)

	// Send empty successful response
	return s.conn.SendResponse(requestID, nil)
}

// Note: Duplicate ListRoots removed below

// CreateMessage sends a 'sampling/create_message' request to the client.
// Note: This sends the request but doesn't wait for the response here.
// The response would need to be handled asynchronously, likely by the client's
// main application logic after being received by the client's receive loop.
func (s *Server) CreateMessage(params CreateMessageRequestParams) (string, error) {
	log.Printf("Server sending CreateMessage request...")
	// SendRequest generates and returns the ID
	requestID, err := s.conn.SendRequest(MethodSamplingCreateMessage, params)
	if err != nil {
		log.Printf("Error sending CreateMessage request: %v", err)
		return "", err
	}
	log.Printf("CreateMessage request sent with ID: %s", requestID)
	// The caller would need to store this ID if they want to match a potential future response,
	// but JSON-RPC doesn't guarantee a response to server-sent requests in the same way.
	return requestID, nil
}

// ListRoots sends a 'roots/list' request to the client.
// The response needs to be handled asynchronously by the client's application logic
// after being received by the client's receive loop and potentially passed back.
func (s *Server) ListRoots(params ListRootsRequestParams) (string, error) {
	log.Printf("Server sending ListRoots request...")
	requestID, err := s.conn.SendRequest(MethodRootsList, params)
	if err != nil {
		log.Printf("Error sending ListRoots request: %v", err)
		return "", err
	}
	log.Printf("ListRoots request sent with ID: %s", requestID)
	return requestID, nil
}

// handlePing handles the 'ping' request by sending back an empty success response.
func (s *Server) handlePing(requestID interface{}, params interface{}) error {
	log.Println("Handling Ping request")
	// Ping request has no params and expects an empty result on success
	return s.conn.SendResponse(requestID, nil) // Send nil result
}

// handleCancellationNotification handles the '$/cancelled' notification from the client.
func (s *Server) handleCancellationNotification(params interface{}) {
	var cancelParams CancelledParams
	err := UnmarshalPayload(params, &cancelParams)
	if err != nil {
		log.Printf("Error unmarshalling $/cancelled params: %v", err)
		return
	}

	if cancelParams.ID == nil {
		log.Printf("Received $/cancelled notification with nil ID.")
		return
	}

	requestIDStr := fmt.Sprintf("%v", cancelParams.ID)
	log.Printf("Received cancellation request for ID: %s", requestIDStr)

	s.requestMu.Lock()
	cancelFunc, ok := s.activeRequests[requestIDStr]
	// It's okay if the request is not found, it might have already finished.
	// Remove it from the map regardless, to prevent potential leaks if cancellation
	// arrives after the defer in handleCallToolRequest but before it finishes execution.
	delete(s.activeRequests, requestIDStr)
	s.requestMu.Unlock()

	if ok {
		log.Printf("Found active request %s, calling cancel function.", requestIDStr)
		cancelFunc() // Call the context cancel function
	} else {
		log.Printf("No active request found for cancellation ID: %s (might have already completed)", requestIDStr)
	}
}

// handleSubscribeResource handles the 'resources/subscribe' request.
// TODO: Implement actual subscription logic (store subscriptions).
func (s *Server) handleSubscribeResource(requestID interface{}, params interface{}) error {
	log.Println("Handling SubscribeResource request (stub)")
	var requestParams SubscribeResourceParams
	err := UnmarshalPayload(params, &requestParams)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal SubscribeResource params: %v", err),
		})
	}
	log.Printf("Client requested subscription to URIs: %v", requestParams.URIs)

	s.subscriptionMu.Lock()
	if len(requestParams.URIs) == 0 {
		// Empty list means unsubscribe all (clear the map)
		log.Println("Unsubscribing from all resources.")
		s.resourceSubscriptions = make(map[string]bool)
	} else {
		for _, uri := range requestParams.URIs {
			// Check if resource actually exists in registry? Spec doesn't explicitly require it.
			// _, exists := s.resourceRegistry[uri]
			// if !exists { log.Printf("Warning: Client subscribed to non-existent resource URI: %s", uri) }
			s.resourceSubscriptions[uri] = true
			log.Printf("Subscribed to resource: %s", uri)
		}
	}
	s.subscriptionMu.Unlock()

	return s.conn.SendResponse(requestID, SubscribeResourceResult{}) // Empty result
}

// handleUnsubscribeResource handles the 'resources/unsubscribe' request.
// TODO: Implement actual unsubscription logic (remove subscriptions).
func (s *Server) handleUnsubscribeResource(requestID interface{}, params interface{}) error {
	log.Println("Handling UnsubscribeResource request (stub)")
	var requestParams UnsubscribeResourceParams
	err := UnmarshalPayload(params, &requestParams)
	if err != nil {
		return s.conn.SendErrorResponse(requestID, ErrorPayload{
			Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal UnsubscribeResource params: %v", err),
		})
	}
	log.Printf("Client requested unsubscription from URIs: %v", requestParams.URIs)

	s.subscriptionMu.Lock()
	if len(requestParams.URIs) == 0 {
		// Spec doesn't define unsubscribe with empty list, but arguably should do nothing.
		log.Println("Received unsubscribe request with empty URI list, doing nothing.")
	} else {
		for _, uri := range requestParams.URIs {
			delete(s.resourceSubscriptions, uri)
			log.Printf("Unsubscribed from resource: %s", uri)
		}
	}
	s.subscriptionMu.Unlock()

	return s.conn.SendResponse(requestID, UnsubscribeResourceResult{}) // Empty result
}

// NotifyResourceUpdated informs the library that a resource has changed.
// The library will then send 'notifications/resources/changed' to subscribed clients.
// The provided 'resource' struct MUST contain the updated version string.
func (s *Server) NotifyResourceUpdated(resource Resource) {
	if resource.URI == "" {
		log.Printf("Warning: NotifyResourceUpdated called with empty URI.")
		return
	}

	// Check if the client is actually subscribed to this resource
	s.subscriptionMu.Lock()
	subscribed, ok := s.resourceSubscriptions[resource.URI]
	s.subscriptionMu.Unlock()

	if !ok || !subscribed {
		// Client is not subscribed, do nothing
		return
	}

	// Send the notification
	log.Printf("Resource %s updated, sending notification.", resource.URI)
	params := ResourceUpdatedParams{ // Use renamed struct
		Resource: resource,
	}
	err := s.SendResourceUpdated(params) // Use renamed method
	if err != nil {
		log.Printf("Warning: failed to send resources/updated notification for URI %s: %v", resource.URI, err)
	}
}

// SendCancellation sends a '$/cancelled' notification to the client.
func (s *Server) SendCancellation(params CancelledParams) error {
	return s.conn.SendNotification(MethodCancelled, params)
}

// SendProgress sends a '$/progress' notification to the client.
func (s *Server) SendProgress(params ProgressParams) error {
	return s.conn.SendNotification(MethodProgress, params)
}

// SendResourceUpdated sends a 'notifications/resources/updated' notification. (Renamed from SendResourceChanged)
func (s *Server) SendResourceUpdated(params ResourceUpdatedParams) error {
	return s.conn.SendNotification(MethodNotifyResourceUpdated, params) // Use renamed constant and param type
}

// SendToolsListChanged sends a 'notifications/tools/list_changed' notification.
func (s *Server) SendToolsListChanged() error {
	return s.conn.SendNotification(MethodNotifyToolsListChanged, ToolsListChangedParams{})
}

// SendResourcesListChanged sends a 'notifications/resources/list_changed' notification.
func (s *Server) SendResourcesListChanged() error {
	return s.conn.SendNotification(MethodNotifyResourcesListChanged, ResourcesListChangedParams{})
}

// SendPromptsListChanged sends a 'notifications/prompts/list_changed' notification.
func (s *Server) SendPromptsListChanged() error {
	return s.conn.SendNotification(MethodNotifyPromptsListChanged, PromptsListChangedParams{})
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
