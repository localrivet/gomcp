// Package server provides the MCP server implementation.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// ToolHandlerFunc defines the signature for functions that handle tool execution.
type ToolHandlerFunc func(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool)

// NotificationHandlerFunc defines the signature for functions that handle client-to-server notifications.
type NotificationHandlerFunc func(ctx context.Context, params interface{}) error

// Server represents the core MCP server logic, independent of transport.
type Server struct {
	serverName         string
	logger             types.Logger
	serverInstructions string // Added for InitializeResult

	// Registries
	toolRegistry     map[string]protocol.Tool
	toolHandlers     map[string]ToolHandlerFunc
	resourceRegistry map[string]protocol.Resource
	promptRegistry   map[string]protocol.Prompt
	registryMu       sync.RWMutex

	// Server capabilities
	serverCapabilities protocol.ServerCapabilities

	// Request/Notification Handling
	activeRequests       map[string]context.CancelFunc
	requestMu            sync.Mutex
	notificationHandlers map[string]NotificationHandlerFunc
	notificationMu       sync.RWMutex

	// Session Management
	sessions sync.Map

	// Resource Subscriptions
	resourceSubscriptions map[string]map[string]bool
	subscriptionMu        sync.Mutex
}

// ServerOption defines a function signature for configuring a Server.
type ServerOption func(*Server)

// WithLogger provides an option to set a custom logger.
func WithLogger(logger types.Logger) ServerOption {
	return func(s *Server) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithServerCapabilities provides an option to set the server's capabilities.
// Note: This replaces all existing capabilities. Consider more granular options
// like WithToolCapabilities, WithResourceCapabilities if needed.
func WithServerCapabilities(caps protocol.ServerCapabilities) ServerOption {
	return func(s *Server) {
		// We might want to merge or provide more granular control later,
		// but for now, this replaces the default capabilities.
		s.serverCapabilities = caps
	}
}

// WithResourceCapabilities sets specific resource-related capabilities.
// It ensures the Resources capability struct is initialized.
func WithResourceCapabilities(subscribe, listChanged bool) ServerOption {
	return func(s *Server) {
		if s.serverCapabilities.Resources == nil {
			s.serverCapabilities.Resources = &struct {
				Subscribe   bool `json:"subscribe,omitempty"`
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
		}
		s.serverCapabilities.Resources.Subscribe = subscribe
		s.serverCapabilities.Resources.ListChanged = listChanged
	}
}

// WithPromptCapabilities sets specific prompt-related capabilities.
// It ensures the Prompts capability struct is initialized.
func WithPromptCapabilities(listChanged bool) ServerOption {
	return func(s *Server) {
		if s.serverCapabilities.Prompts == nil {
			s.serverCapabilities.Prompts = &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
		}
		s.serverCapabilities.Prompts.ListChanged = listChanged
	}
}

// WithToolCapabilities sets specific tool-related capabilities.
// It ensures the Tools capability struct is initialized.
func WithToolCapabilities(listChanged bool) ServerOption {
	return func(s *Server) {
		if s.serverCapabilities.Tools == nil {
			s.serverCapabilities.Tools = &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
		}
		s.serverCapabilities.Tools.ListChanged = listChanged
	}
}

// WithInstructions sets the server instructions string returned during initialization.
func WithInstructions(instructions string) ServerOption {
	return func(s *Server) {
		s.serverInstructions = instructions
	}
}

// NewServer creates a new core MCP Server logic instance with the provided options.
func NewServer(serverName string, opts ...ServerOption) *Server {
	// Initialize with default values
	srv := &Server{
		serverName: serverName,
		logger:     &defaultLogger{}, // Default logger
		serverCapabilities: protocol.ServerCapabilities{ // Default capabilities
			Tools: &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{},
			Resources: &struct {
				Subscribe   bool `json:"subscribe,omitempty"`
				ListChanged bool `json:"listChanged,omitempty"`
			}{},
			Prompts: &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{},
			// Initialize other capability fields to nil or default as needed
		},
		toolRegistry:          make(map[string]protocol.Tool),
		toolHandlers:          make(map[string]ToolHandlerFunc),
		resourceRegistry:      make(map[string]protocol.Resource),
		promptRegistry:        make(map[string]protocol.Prompt),
		activeRequests:        make(map[string]context.CancelFunc),
		notificationHandlers:  make(map[string]NotificationHandlerFunc),
		resourceSubscriptions: make(map[string]map[string]bool),
	}

	// Apply provided options
	for _, opt := range opts {
		opt(srv)
	}

	// Ensure essential capabilities structs are non-nil after options are applied
	// (This might overwrite parts of user-provided caps if they set the top-level to nil)
	if srv.serverCapabilities.Tools == nil {
		srv.serverCapabilities.Tools = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}
	if srv.serverCapabilities.Resources == nil {
		srv.serverCapabilities.Resources = &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}
	if srv.serverCapabilities.Prompts == nil {
		srv.serverCapabilities.Prompts = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}

	// Register internal handlers
	srv.RegisterNotificationHandler(protocol.MethodCancelled, srv.handleCancellationNotification)

	srv.logger.Info("MCP Core Server '%s' created.", serverName)
	return srv
}

// --- Session Management ---

func (s *Server) RegisterSession(session types.ClientSession) error { // Use types.ClientSession
	if session == nil {
		return fmt.Errorf("cannot register nil session")
	}
	sessionID := session.SessionID()
	if _, loaded := s.sessions.LoadOrStore(sessionID, session); loaded {
		return fmt.Errorf("session with ID '%s' already registered", sessionID)
	}
	s.subscriptionMu.Lock()
	s.resourceSubscriptions[sessionID] = make(map[string]bool)
	s.subscriptionMu.Unlock()
	s.logger.Info("Registered session: %s", sessionID)
	return nil
}

func (s *Server) UnregisterSession(sessionID string) {
	_, loaded := s.sessions.LoadAndDelete(sessionID)
	s.subscriptionMu.Lock()
	delete(s.resourceSubscriptions, sessionID)
	s.subscriptionMu.Unlock()
	if loaded {
		s.logger.Info("Unregistered session: %s", sessionID)
	}
}

// --- Message Handling (Called by Transport Layer) ---

// HandleMessage processes an incoming raw JSON message, which can be a single JSON-RPC object
// or a JSON array representing a batch of requests/notifications.
// It returns a slice of responses. For single requests, the slice will contain 0 or 1 response.
// For batches, it will contain responses for all processed requests in the batch (notifications produce no response).
// Returns nil or an empty slice if no responses should be sent.
func (s *Server) HandleMessage(ctx context.Context, sessionID string, rawMessage json.RawMessage) []*protocol.JSONRPCResponse {
	s.logger.Debug("HandleMessage for session %s: %s", sessionID, string(rawMessage))
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		s.logger.Error("Received message for unknown session ID: %s", sessionID)
		// Cannot determine request ID here, so return nil response slice. Error logged.
		return nil
	}
	session := sessionI.(types.ClientSession) // Use types.ClientSession

	// Trim whitespace and check if it's a batch request (starts with '[')
	trimmedMsg := bytes.TrimSpace(rawMessage)
	isBatch := len(trimmedMsg) > 0 && trimmedMsg[0] == '['

	if isBatch {
		// Check protocol version for batch support
		if session.GetNegotiatedVersion() != protocol.CurrentProtocolVersion {
			s.logger.Error("Session %s: Received batch request, but negotiated protocol version (%s) does not support it.", sessionID, session.GetNegotiatedVersion())
			// JSON-RPC spec says for batch errors like this, return a single error response.
			// However, we don't have a single ID. Returning nil and logging seems safest.
			// Alternatively, could return a generic parse error response without ID.
			return []*protocol.JSONRPCResponse{createErrorResponse(nil, protocol.ErrorCodeInvalidRequest, "Batch requests not supported for negotiated protocol version")}
		}

		s.logger.Debug("Session %s: Handling batch request.", sessionID)
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmedMsg, &batch); err != nil {
			s.logger.Error("Session %s: Failed to unmarshal batch request: %v", sessionID, err)
			return []*protocol.JSONRPCResponse{createErrorResponse(nil, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to parse batch JSON: %v", err))}
		}

		if len(batch) == 0 {
			s.logger.Warn("Session %s: Received empty batch request.", sessionID)
			return []*protocol.JSONRPCResponse{createErrorResponse(nil, protocol.ErrorCodeInvalidRequest, "Received empty batch request")}
		}

		responses := make([]*protocol.JSONRPCResponse, 0, len(batch))
		for _, singleRawMsg := range batch {
			// Process each message in the batch individually
			response := s.handleSingleMessage(ctx, session, singleRawMsg)
			if response != nil {
				responses = append(responses, response)
			}
		}

		// Only return responses if there are any (JSON-RPC spec: don't return empty array)
		if len(responses) > 0 {
			return responses
		}
		return nil // No responses to send (e.g., batch of notifications)

	} else {
		// Handle single message
		s.logger.Debug("Session %s: Handling single request/notification.", sessionID)
		response := s.handleSingleMessage(ctx, session, rawMessage)
		if response != nil {
			return []*protocol.JSONRPCResponse{response}
		}
		return nil // Single notification processed
	}
}

// handleSingleMessage processes a single JSON-RPC request or notification object.
func (s *Server) handleSingleMessage(ctx context.Context, session types.ClientSession, rawMessage json.RawMessage) *protocol.JSONRPCResponse { // Use types.ClientSession
	sessionID := session.SessionID() // Get session ID from session object

	// Attempt to parse basic structure first to get ID/Method
	var baseMessage struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"` // Keep params raw initially
	}
	if err := json.Unmarshal(rawMessage, &baseMessage); err != nil {
		s.logger.Error("Session %s: Failed to parse base message structure: %v. Raw: %s", sessionID, err, string(rawMessage))
		return createErrorResponse(nil, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to parse JSON: %v", err))
	}

	// Basic validation
	if baseMessage.JSONRPC != "2.0" {
		s.logger.Warn("Session %s: Received message with invalid jsonrpc version: %s", sessionID, baseMessage.JSONRPC)
		return createErrorResponse(baseMessage.ID, protocol.ErrorCodeInvalidRequest, "Invalid jsonrpc version")
	}

	// Handle Initialization Phase (should not happen in batch, but check anyway)
	if !session.Initialized() {
		// Initialization messages MUST NOT be batched according to some interpretations,
		// handle them specifically before the main request/notification logic.
		if baseMessage.Method == protocol.MethodInitialize && baseMessage.ID != nil {
			return s.handleInitializationMessage(ctx, session, baseMessage.ID, baseMessage.Method, rawMessage)
		} else if baseMessage.Method == protocol.MethodInitialized && baseMessage.ID == nil {
			// This is a notification, handleInitializedNotification doesn't return a response
			err := s.handleInitializedNotification(ctx, session, rawMessage)
			if err != nil {
				s.logger.Error("Error handling initialized notification for session %s: %v", sessionID, err)
				// Cannot send error response for notification
				// Should we still mark as initialized? Probably not if parsing failed.
			} else {
				// Mark session as initialized *after* successfully processing the notification
				session.Initialize()
				s.logger.Info("Session %s marked as initialized.", sessionID)
			}
			return nil // No response for notification
		} else {
			// Invalid message during initialization phase
			s.logger.Error("Session %s: Received invalid message (method: %s, id: %v) during initialization", sessionID, baseMessage.Method, baseMessage.ID)
			return createErrorResponse(baseMessage.ID, protocol.ErrorCodeInvalidRequest, "Expected 'initialize' request or 'initialized' notification during handshake")
		}
	}

	// Handle Regular Messages (Post-Initialization)
	isRequest := baseMessage.ID != nil
	isNotification := baseMessage.ID == nil && baseMessage.Method != ""

	if isRequest {
		// Pass raw params to handleRequest
		return s.handleRequest(ctx, session, baseMessage.ID, baseMessage.Method, baseMessage.Params) // Pass raw params
	} else if isNotification {
		// Pass raw params to handleNotification
		err := s.handleNotification(ctx, session, baseMessage.Method, baseMessage.Params) // Pass raw params
		if err != nil {
			s.logger.Error("Error handling notification '%s' for session %s: %v", baseMessage.Method, sessionID, err)
		}
		return nil // No response for notifications
	} else {
		// Invalid message (e.g., missing method for notification)
		s.logger.Warn("Received message with no ID or Method for session %s: %s", sessionID, string(rawMessage))
		return createErrorResponse(baseMessage.ID, protocol.ErrorCodeInvalidRequest, "Invalid message: must be request (with id) or notification (with method)")
	}
}

// --- Initialization Handling ---

func (s *Server) handleInitializationMessage(ctx context.Context, session types.ClientSession, id interface{}, method string, rawMessage json.RawMessage) *protocol.JSONRPCResponse { // Use types.ClientSession
	sessionID := session.SessionID()
	if method == protocol.MethodInitialize && id != nil {
		resp, err := s.handleInitializeRequest(ctx, session, id, rawMessage)
		if err != nil {
			s.logger.Error("Initialization failed for session %s: %v", sessionID, err)
			if resp != nil {
				_ = session.SendResponse(*resp)
			} else {
				errResp := createErrorResponse(id, protocol.ErrorCodeInternalError, fmt.Sprintf("Initialization error: %v", err))
				_ = session.SendResponse(*errResp)
			}
			_ = session.Close()
			s.UnregisterSession(sessionID)
			return nil
		}
		_ = session.SendResponse(*resp)
		return nil
	} else if method == protocol.MethodInitialized && id == nil {
		err := s.handleInitializedNotification(ctx, session, rawMessage)
		if err != nil {
			s.logger.Error("Error processing initialized notification for session %s: %v", sessionID, err)
			_ = session.Close()
			s.UnregisterSession(sessionID)
		} else {
			session.Initialize()
			s.logger.Info("Session %s initialized successfully.", sessionID)
		}
		return nil
	} else {
		s.logger.Error("Received invalid message (method: %s, id: %v) during initialization for session %s", method, id, sessionID)
		errResp := createErrorResponse(id, protocol.ErrorCodeInvalidRequest, "Expected 'initialize' request or 'initialized' notification")
		// Don't close session here, let caller handle potential SendResponse error
		return errResp
	}
}

func (s *Server) handleInitializeRequest(ctx context.Context, session types.ClientSession, requestID interface{}, rawMessage json.RawMessage) (*protocol.JSONRPCResponse, error) { // Use types.ClientSession
	var req protocol.JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		return createErrorResponse(requestID, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to re-parse initialize request: %v", err)), nil
	}
	var initParams protocol.InitializeRequestParams
	if err := protocol.UnmarshalPayload(req.Params, &initParams); err != nil {
		return createErrorResponse(requestID, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to parse initialize params: %v", err)), nil
	}
	// Check if the client's requested version is supported
	negotiatedVersion := ""
	switch initParams.ProtocolVersion {
	case protocol.CurrentProtocolVersion:
		negotiatedVersion = protocol.CurrentProtocolVersion
		s.logger.Info("Session %s: Client requested protocol version %s (Current).", session.SessionID(), initParams.ProtocolVersion)
	case protocol.OldProtocolVersion:
		negotiatedVersion = protocol.OldProtocolVersion
		s.logger.Info("Session %s: Client requested protocol version %s (Old).", session.SessionID(), initParams.ProtocolVersion)
	default:
		errMsg := fmt.Sprintf("Unsupported protocol version '%s'. Server supports '%s' and '%s'.",
			initParams.ProtocolVersion, protocol.CurrentProtocolVersion, protocol.OldProtocolVersion)
		return createErrorResponse(requestID, protocol.ErrorCodeMCPUnsupportedProtocolVersion, errMsg), nil
	}

	// Store the negotiated version in the session
	session.SetNegotiatedVersion(negotiatedVersion)

	s.logger.Info("Session %s: Received InitializeRequest from client: %s (Version: %s)", session.SessionID(), initParams.ClientInfo.Name, initParams.ClientInfo.Version)
	s.logger.Debug("Session %s: Client Capabilities: %+v", session.SessionID(), initParams.Capabilities)
	session.StoreClientCapabilities(initParams.Capabilities) // Store received client capabilities

	// Start with the base server capabilities
	advertisedCaps := s.serverCapabilities

	// If the negotiated version is the older one, remove capabilities not present in that version.
	if negotiatedVersion == protocol.OldProtocolVersion {
		s.logger.Debug("Session %s: Adjusting capabilities for older protocol version %s", session.SessionID(), negotiatedVersion)
		// Make a shallow copy to avoid modifying the server's base capabilities
		adjustedCaps := advertisedCaps
		adjustedCaps.Authorization = nil // Authorization added in 2025-03-26
		adjustedCaps.Completions = nil   // Completions added in 2025-03-26
		// Add any other capabilities specific to 2025-03-26 here to nil them out
		advertisedCaps = adjustedCaps
	}

	responsePayload := protocol.InitializeResult{
		ProtocolVersion: negotiatedVersion,                                             // Respond with the *negotiated* version
		Capabilities:    advertisedCaps,                                                // Use the potentially adjusted capabilities
		ServerInfo:      protocol.Implementation{Name: s.serverName, Version: "0.1.0"}, // Using fixed version for now
		Instructions:    s.serverInstructions,                                          // Add instructions
	}
	resp := createSuccessResponse(requestID, responsePayload)
	return resp, nil
}

func (s *Server) handleInitializedNotification(ctx context.Context, session types.ClientSession, rawMessage json.RawMessage) error { // Use types.ClientSession
	var notif protocol.JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notif); err != nil {
		return fmt.Errorf("failed to parse initialized notification: %w", err)
	}
	s.logger.Info("Session %s: Received InitializedNotification.", session.SessionID())
	return nil
}

// --- Request/Notification Routing (Post-Initialization) ---

// handleRequest processes a single JSON-RPC request after initial parsing.
// Takes rawParams json.RawMessage instead of the full rawMessage.
func (s *Server) handleRequest(ctx context.Context, session types.ClientSession, id interface{}, method string, rawParams json.RawMessage) *protocol.JSONRPCResponse { // Use types.ClientSession
	s.logger.Debug("Handling request for session %s: Method=%s, ID=%v", session.SessionID(), method, id)
	handlerCtx := ctx
	// Note: params are now passed as json.RawMessage, unmarshalling happens in specific handlers

	switch method {
	case protocol.MethodListTools:
		return s.handleListToolsRequest(handlerCtx, id, rawParams)
	case protocol.MethodCallTool:
		return s.handleCallToolRequest(handlerCtx, session, id, rawParams)
	case protocol.MethodListResources:
		return s.handleListResources(handlerCtx, id, rawParams)
	case protocol.MethodReadResource:
		return s.handleReadResource(handlerCtx, id, rawParams)
	case protocol.MethodListPrompts:
		return s.handleListPrompts(handlerCtx, id, rawParams)
	case protocol.MethodGetPrompt:
		return s.handleGetPrompt(handlerCtx, id, rawParams)
	case protocol.MethodSubscribeResource:
		return s.handleSubscribeResource(handlerCtx, session, id, rawParams)
	case protocol.MethodUnsubscribeResource:
		return s.handleUnsubscribeResource(handlerCtx, session, id, rawParams)
	case protocol.MethodPing:
		return s.handlePing(handlerCtx, id, rawParams)
	default:
		s.logger.Warn("Method not found for session %s: %s", session.SessionID(), method)
		return createErrorResponse(id, protocol.ErrorCodeMethodNotFound, fmt.Sprintf("Method '%s' not implemented", method))
	}
}

// handleNotification processes a single JSON-RPC notification after initial parsing.
// Takes rawParams json.RawMessage instead of the full rawMessage.
func (s *Server) handleNotification(ctx context.Context, session types.ClientSession, method string, rawParams json.RawMessage) error { // Use types.ClientSession
	s.notificationMu.RLock()
	handler, ok := s.notificationHandlers[method]
	s.notificationMu.RUnlock()
	if !ok {
		s.logger.Info("No handler registered for notification method '%s' from session %s", method, session.SessionID())
		return nil // Not an error if handler doesn't exist
	}

	// Unmarshal params only if handler exists
	var params interface{}
	if len(rawParams) > 0 && string(rawParams) != "null" {
		// Attempt to unmarshal into a generic interface{} first.
		// Specific handlers might need to unmarshal again into concrete types.
		if err := json.Unmarshal(rawParams, &params); err != nil {
			s.logger.Error("Failed to parse params for notification %s from session %s: %v. Raw: %s", method, session.SessionID(), err, string(rawParams))
			// Cannot send error response for notification parse error
			return fmt.Errorf("failed to parse notification params: %w", err)
		}
	} // else: params are null or absent, pass nil to handler

	handlerCtx := ctx
	err := handler(handlerCtx, params) // Pass potentially nil params
	if err != nil {
		s.logger.Error("Error executing notification handler for method %s from session %s: %v", method, session.SessionID(), err)
		// Return the error, but can't send JSON-RPC error response
	}
	return err
}

// --- Public Registration Methods ---

func (s *Server) RegisterNotificationHandler(method string, handler NotificationHandlerFunc) error {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()
	if _, exists := s.notificationHandlers[method]; exists {
		return fmt.Errorf("notification handler already registered for method: %s", method)
	}
	s.notificationHandlers[method] = handler
	s.logger.Info("Registered notification handler for method: %s", method)
	return nil
}

func (s *Server) RegisterTool(tool protocol.Tool, handler ToolHandlerFunc) error {
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
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
	s.logger.Info("Registered tool: %s", tool.Name)
	if s.serverCapabilities.Tools != nil && s.serverCapabilities.Tools.ListChanged {
		go func() {
			if err := s.SendToolsListChanged(); err != nil {
				s.logger.Warn("Failed to send tools/list_changed notification: %v", err)
			}
		}()
	}
	return nil
}

func (s *Server) RegisterResource(resource protocol.Resource) error {
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
	if resource.URI == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	s.resourceRegistry[resource.URI] = resource
	s.logger.Info("Registered/Updated resource: %s", resource.URI)
	if s.serverCapabilities.Resources != nil && s.serverCapabilities.Resources.ListChanged {
		go func() {
			if err := s.SendResourcesListChanged(); err != nil {
				s.logger.Warn("Failed to send resources/list_changed notification: %v", err)
			}
		}()
	}
	return nil
}

func (s *Server) UnregisterResource(uri string) error {
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()
	if uri == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	_, exists := s.resourceRegistry[uri]
	if !exists {
		return fmt.Errorf("resource '%s' not found", uri)
	}
	delete(s.resourceRegistry, uri)
	for sessionID := range s.resourceSubscriptions {
		delete(s.resourceSubscriptions[sessionID], uri)
	}
	s.logger.Info("Unregistered resource: %s", uri)
	if s.serverCapabilities.Resources != nil && s.serverCapabilities.Resources.ListChanged {
		go func() {
			if err := s.SendResourcesListChanged(); err != nil {
				s.logger.Warn("Failed to send resources/list_changed notification: %v", err)
			}
		}()
	}
	return nil
}

func (s *Server) ResourceRegistry() map[string]protocol.Resource {
	s.registryMu.RLock()
	defer s.registryMu.RUnlock()
	registryCopy := make(map[string]protocol.Resource, len(s.resourceRegistry))
	for k, v := range s.resourceRegistry {
		registryCopy[k] = v
	}
	return registryCopy
}

func (s *Server) RegisterPrompt(prompt protocol.Prompt) error {
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
	if prompt.URI == "" {
		return fmt.Errorf("prompt URI cannot be empty")
	}
	s.promptRegistry[prompt.URI] = prompt
	s.logger.Info("Registered/Updated prompt: %s", prompt.URI)
	if s.serverCapabilities.Prompts != nil && s.serverCapabilities.Prompts.ListChanged {
		go func() {
			if err := s.SendPromptsListChanged(); err != nil {
				s.logger.Warn("Failed to send prompts/list_changed notification: %v", err)
			}
		}()
	}
	return nil
}

func (s *Server) UnregisterPrompt(uri string) error {
	s.registryMu.Lock()
	defer s.registryMu.Unlock()
	if uri == "" {
		return fmt.Errorf("prompt URI cannot be empty")
	}
	_, exists := s.promptRegistry[uri]
	if !exists {
		return fmt.Errorf("prompt '%s' not found", uri)
	}
	delete(s.promptRegistry, uri)
	s.logger.Info("Unregistered prompt: %s", uri)
	if s.serverCapabilities.Prompts != nil && s.serverCapabilities.Prompts.ListChanged {
		go func() {
			if err := s.SendPromptsListChanged(); err != nil {
				s.logger.Warn("Failed to send prompts/list_changed notification: %v", err)
			}
		}()
	}
	return nil
}

// --- Built-in Request Handlers ---

// handleListToolsRequest handles the 'tools/list' request. Params are expected to be json.RawMessage.
func (s *Server) handleListToolsRequest(ctx context.Context, id interface{}, rawParams json.RawMessage) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListToolsRequest")
	// TODO: Potentially parse ListToolsRequestParams from rawParams if needed (e.g., for pagination cursor)
	// var requestParams protocol.ListToolsRequestParams
	// if err := protocol.UnmarshalPayload(rawParams, &requestParams); err != nil {
	// 	 return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal ListTools params: %v", err))
	// }

	s.registryMu.RLock()
	tools := make([]protocol.Tool, 0, len(s.toolRegistry))
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	s.registryMu.RUnlock()
	return createSuccessResponse(id, protocol.ListToolsResult{Tools: tools})
}

// handleCallToolRequest handles the 'tools/call' request. Params are expected to be json.RawMessage.
func (s *Server) handleCallToolRequest(ctx context.Context, session types.ClientSession, id interface{}, rawParams json.RawMessage) *protocol.JSONRPCResponse { // Use types.ClientSession
	s.logger.Debug("Handling CallToolRequest for session %s", session.SessionID())
	var requestParams protocol.CallToolParams
	if err := protocol.UnmarshalPayload(rawParams, &requestParams); err != nil { // Unmarshal from rawParams
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal CallTool params: %v", err))
	}
	s.registryMu.RLock()
	handler, exists := s.toolHandlers[requestParams.Name]
	s.registryMu.RUnlock()
	if !exists {
		return createErrorResponse(id, protocol.ErrorCodeMCPToolNotFound, fmt.Sprintf("Tool '%s' not found", requestParams.Name))
	}

	reqCtx, cancel := context.WithCancel(ctx)
	requestIDStr := fmt.Sprintf("%v", id)
	s.requestMu.Lock()
	s.activeRequests[requestIDStr] = cancel
	s.requestMu.Unlock()
	defer func() { s.requestMu.Lock(); delete(s.activeRequests, requestIDStr); s.requestMu.Unlock(); cancel() }()

	// Extract progress token safely, checking if Meta is nil
	var progressToken *protocol.ProgressToken
	if requestParams.Meta != nil {
		progressToken = requestParams.Meta.ProgressToken
	}

	content, isError := handler(reqCtx, progressToken, requestParams.Arguments) // Pass potentially nil progressToken
	responsePayload := protocol.CallToolResult{Content: content}
	if isError {
		responsePayload.IsError = &isError
	}
	return createSuccessResponse(id, responsePayload)
}

// handleListResources handles the 'resources/list' request. Params are expected to be json.RawMessage.
func (s *Server) handleListResources(ctx context.Context, id interface{}, rawParams json.RawMessage) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListResourcesRequest")
	// TODO: Parse ListResourcesRequestParams if needed
	s.registryMu.RLock()
	resources := make([]protocol.Resource, 0, len(s.resourceRegistry))
	for _, res := range s.resourceRegistry {
		resources = append(resources, res)
	}
	s.registryMu.RUnlock()
	return createSuccessResponse(id, protocol.ListResourcesResult{Resources: resources})
}

func (s *Server) handleReadResource(ctx context.Context, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ReadResourceRequest (stub)")
	var requestParams protocol.ReadResourceRequestParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal ReadResource params: %v", err))
	}
	return createErrorResponse(id, protocol.ErrorCodeMCPResourceNotFound, fmt.Sprintf("Resource not found (stub): %s", requestParams.URI))
}

// handleListPrompts handles the 'prompts/list' request. Params are expected to be json.RawMessage.
func (s *Server) handleListPrompts(ctx context.Context, id interface{}, rawParams json.RawMessage) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListPromptsRequest")
	// TODO: Parse ListPromptsRequestParams if needed
	s.registryMu.RLock()
	prompts := make([]protocol.Prompt, 0, len(s.promptRegistry))
	for _, p := range s.promptRegistry {
		prompts = append(prompts, p)
	}
	s.registryMu.RUnlock()
	return createSuccessResponse(id, protocol.ListPromptsResult{Prompts: prompts})
}

func (s *Server) handleGetPrompt(ctx context.Context, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling GetPromptRequest (stub)")
	var requestParams protocol.GetPromptRequestParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal GetPrompt params: %v", err))
	}
	return createErrorResponse(id, protocol.ErrorCodeMCPResourceNotFound, fmt.Sprintf("Prompt not found (stub): %s", requestParams.URI))
}

func (s *Server) handleSubscribeResource(ctx context.Context, session types.ClientSession, id interface{}, params interface{}) *protocol.JSONRPCResponse { // Use types.ClientSession
	s.logger.Debug("Handling SubscribeResource request for session %s", session.SessionID())
	var requestParams protocol.SubscribeResourceParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal SubscribeResource params: %v", err))
	}
	sessionID := session.SessionID()
	s.subscriptionMu.Lock()
	if _, ok := s.resourceSubscriptions[sessionID]; !ok {
		s.resourceSubscriptions[sessionID] = make(map[string]bool)
	}
	if len(requestParams.URIs) == 0 {
		s.logger.Info("Session %s unsubscribing from all resources.", sessionID)
		s.resourceSubscriptions[sessionID] = make(map[string]bool)
	} else {
		for _, uri := range requestParams.URIs {
			s.resourceSubscriptions[sessionID][uri] = true
			s.logger.Info("Session %s subscribed to resource: %s", sessionID, uri)
		}
	}
	s.subscriptionMu.Unlock()
	return createSuccessResponse(id, protocol.SubscribeResourceResult{})
}

func (s *Server) handleUnsubscribeResource(ctx context.Context, session types.ClientSession, id interface{}, params interface{}) *protocol.JSONRPCResponse { // Use types.ClientSession
	s.logger.Debug("Handling UnsubscribeResource request for session %s", session.SessionID())
	var requestParams protocol.UnsubscribeResourceParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal UnsubscribeResource params: %v", err))
	}
	sessionID := session.SessionID()
	s.subscriptionMu.Lock()
	if subs, ok := s.resourceSubscriptions[sessionID]; ok {
		if len(requestParams.URIs) == 0 {
			s.logger.Warn("Session %s sent unsubscribe request with empty URI list, doing nothing.", sessionID)
		} else {
			for _, uri := range requestParams.URIs {
				delete(subs, uri)
				s.logger.Info("Session %s unsubscribed from resource: %s", sessionID, uri)
			}
		}
	} else {
		s.logger.Warn("Session %s sent unsubscribe request but had no active subscriptions.", sessionID)
	}
	s.subscriptionMu.Unlock()
	return createSuccessResponse(id, protocol.UnsubscribeResourceResult{})
}

func (s *Server) handlePing(ctx context.Context, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling Ping request")
	return createSuccessResponse(id, nil)
}

// --- Built-in Notification Handlers ---

func (s *Server) handleCancellationNotification(ctx context.Context, params interface{}) error {
	var cancelParams protocol.CancelledParams
	if err := protocol.UnmarshalPayload(params, &cancelParams); err != nil {
		s.logger.Error("Error unmarshalling $/cancelled params: %v", err)
		return err
	}
	if cancelParams.ID == nil {
		s.logger.Warn("Received $/cancelled notification with nil ID.")
		return nil
	}
	requestIDStr := fmt.Sprintf("%v", cancelParams.ID)
	s.logger.Info("Received cancellation request for ID: %s", requestIDStr)
	s.requestMu.Lock()
	cancelFunc, ok := s.activeRequests[requestIDStr]
	delete(s.activeRequests, requestIDStr)
	s.requestMu.Unlock()
	if ok {
		s.logger.Info("Found active request %s, calling cancel function.", requestIDStr)
		cancelFunc()
	} else {
		s.logger.Info("No active request found for cancellation ID: %s", requestIDStr)
	}
	return nil
}

// --- Methods for Sending Notifications/Requests TO Client (via Session) ---

func (s *Server) SendProgress(sessionID string, params protocol.ProgressParams) error {
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return s.sendNotificationToSession(sessionI.(types.ClientSession), protocol.MethodProgress, params) // Use types.ClientSession
}

func (s *Server) NotifyResourceUpdated(resource protocol.Resource) {
	if resource.URI == "" {
		s.logger.Warn("NotifyResourceUpdated called with empty URI.")
		return
	}
	s.logger.Info("Resource %s updated, sending notifications.", resource.URI)
	params := protocol.ResourceUpdatedParams{Resource: resource}
	s.subscriptionMu.Lock()
	subscribedSessions := []types.ClientSession{} // Use types.ClientSession
	for sessionID, subs := range s.resourceSubscriptions {
		if subs[resource.URI] {
			if sessionI, ok := s.sessions.Load(sessionID); ok {
				subscribedSessions = append(subscribedSessions, sessionI.(types.ClientSession)) // Use types.ClientSession
			}
		}
	}
	s.subscriptionMu.Unlock()
	for _, session := range subscribedSessions {
		go func(sess types.ClientSession) { // Use types.ClientSession
			if err := s.sendNotificationToSession(sess, protocol.MethodNotifyResourceUpdated, params); err != nil {
				s.logger.Warn("Failed to send resources/updated notification to session %s for URI %s: %v", sess.SessionID(), resource.URI, err)
			}
		}(session)
	}
}

func (s *Server) SendToolsListChanged() error {
	s.logger.Info("Sending tools/list_changed notification to all sessions.")
	return s.broadcastNotification(protocol.MethodNotifyToolsListChanged, protocol.ToolsListChangedParams{})
}

func (s *Server) SendResourcesListChanged() error {
	s.logger.Info("Sending resources/list_changed notification to all sessions.")
	return s.broadcastNotification(protocol.MethodNotifyResourcesListChanged, protocol.ResourcesListChangedParams{})
}

func (s *Server) SendPromptsListChanged() error {
	s.logger.Info("Sending prompts/list_changed notification to all sessions.")
	return s.broadcastNotification(protocol.MethodNotifyPromptsListChanged, protocol.PromptsListChangedParams{})
}

// --- Internal Send Helpers ---

func (s *Server) broadcastNotification(method string, params interface{}) error {
	var firstErr error
	s.sessions.Range(func(key, value interface{}) bool {
		session := value.(types.ClientSession) // Use types.ClientSession
		err := s.sendNotificationToSession(session, method, params)
		if err != nil {
			s.logger.Warn("Failed to send broadcast notification %s to session %s: %v", method, session.SessionID(), err)
			if firstErr == nil {
				firstErr = err
			}
		}
		return true
	})
	return firstErr
}

func (s *Server) sendNotificationToSession(session types.ClientSession, method string, params interface{}) error { // Use types.ClientSession
	notif := protocol.JSONRPCNotification{JSONRPC: "2.0", Method: method, Params: params}
	return session.SendNotification(notif)
}

func createSuccessResponse(id interface{}, result interface{}) *protocol.JSONRPCResponse {
	return &protocol.JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func createErrorResponse(id interface{}, code int, message string) *protocol.JSONRPCResponse {
	return &protocol.JSONRPCResponse{
		JSONRPC: "2.0", ID: id,
		Error: &protocol.ErrorPayload{Code: code, Message: message},
	}
}

// --- Default Logger ---
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

// ServeStdio helper moved to stdio_server.go
