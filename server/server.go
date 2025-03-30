// Package server provides the MCP server implementation.
package server

import (
	"context"
	"encoding/json"

	// "errors" // Not used directly
	"fmt"
	// "io" // Not used directly
	"log" // For default logger
	// "net" // Not used directly
	// "os"  // Not used directly
	// "strings" // Not used directly
	"sync"
	// "time" // Not used directly

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types" // Keep for Logger interface
)

// ToolHandlerFunc defines the signature for functions that handle tool execution.
type ToolHandlerFunc func(ctx context.Context, progressToken *protocol.ProgressToken, arguments map[string]interface{}) (content []protocol.Content, isError bool)

// NotificationHandlerFunc defines the signature for functions that handle client-to-server notifications.
type NotificationHandlerFunc func(ctx context.Context, params interface{}) error

// Server represents the core MCP server logic, independent of transport.
type Server struct {
	serverName string
	logger     types.Logger

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
	sessions   sync.Map
	sessionsMu sync.Mutex

	// Resource Subscriptions
	resourceSubscriptions map[string]map[string]bool
	subscriptionMu        sync.Mutex
}

// ServerOptions contains configuration options for creating a Server.
type ServerOptions struct {
	Logger             types.Logger
	ServerCapabilities protocol.ServerCapabilities
}

// NewServer creates a new core MCP Server logic instance.
func NewServer(serverName string, opts ServerOptions) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	serverCaps := opts.ServerCapabilities
	if serverCaps.Tools == nil {
		serverCaps.Tools = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}
	if serverCaps.Resources == nil {
		serverCaps.Resources = &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}
	if serverCaps.Prompts == nil {
		serverCaps.Prompts = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}

	srv := &Server{
		serverName:            serverName,
		logger:                logger,
		serverCapabilities:    serverCaps,
		toolRegistry:          make(map[string]protocol.Tool),
		toolHandlers:          make(map[string]ToolHandlerFunc),
		resourceRegistry:      make(map[string]protocol.Resource),
		promptRegistry:        make(map[string]protocol.Prompt),
		activeRequests:        make(map[string]context.CancelFunc),
		notificationHandlers:  make(map[string]NotificationHandlerFunc),
		resourceSubscriptions: make(map[string]map[string]bool),
	}

	srv.RegisterNotificationHandler(protocol.MethodCancelled, srv.handleCancellationNotification)
	logger.Info("MCP Core Server '%s' created.", serverName)
	return srv
}

// --- Session Management ---

func (s *Server) RegisterSession(session ClientSession) error {
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

func (s *Server) HandleMessage(ctx context.Context, sessionID string, rawMessage json.RawMessage) *protocol.JSONRPCResponse {
	s.logger.Debug("HandleMessage for session %s: %s", sessionID, string(rawMessage))
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		s.logger.Error("Received message for unknown session ID: %s", sessionID)
		return createErrorResponse(nil, protocol.ErrorCodeInternalError, "Unknown session ID")
	}
	session := sessionI.(ClientSession)

	var baseMessage struct {
		ID     interface{}
		Method string `json:"method"`
	}
	if err := json.Unmarshal(rawMessage, &baseMessage); err != nil {
		s.logger.Error("Failed to parse base message structure for session %s: %v", sessionID, err)
		return createErrorResponse(nil, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to parse JSON: %v", err))
	}

	// Handle Initialization Phase
	if !session.Initialized() {
		return s.handleInitializationMessage(ctx, session, baseMessage.ID, baseMessage.Method, rawMessage)
	}

	// Handle Regular Messages
	if baseMessage.ID != nil { // Request
		return s.handleRequest(ctx, session, baseMessage.ID, baseMessage.Method, rawMessage)
	} else if baseMessage.Method != "" { // Notification
		err := s.handleNotification(ctx, session, baseMessage.Method, rawMessage)
		if err != nil {
			s.logger.Error("Error handling notification '%s' for session %s: %v", baseMessage.Method, sessionID, err)
		}
		return nil
	} else {
		s.logger.Warn("Received message with no ID or Method for session %s: %s", sessionID, string(rawMessage))
		return createErrorResponse(nil, protocol.ErrorCodeInvalidRequest, "Invalid message: missing id and method")
	}
}

// --- Initialization Handling ---

func (s *Server) handleInitializationMessage(ctx context.Context, session ClientSession, id interface{}, method string, rawMessage json.RawMessage) *protocol.JSONRPCResponse {
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

func (s *Server) handleInitializeRequest(ctx context.Context, session ClientSession, requestID interface{}, rawMessage json.RawMessage) (*protocol.JSONRPCResponse, error) {
	var req protocol.JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		return createErrorResponse(requestID, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to re-parse initialize request: %v", err)), nil
	}
	var initParams protocol.InitializeRequestParams
	if err := protocol.UnmarshalPayload(req.Params, &initParams); err != nil {
		return createErrorResponse(requestID, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to parse initialize params: %v", err)), nil
	}
	if initParams.ProtocolVersion != protocol.CurrentProtocolVersion {
		errMsg := fmt.Sprintf("Unsupported protocol version '%s'. Server requires '%s'.", initParams.ProtocolVersion, protocol.CurrentProtocolVersion)
		return createErrorResponse(requestID, protocol.ErrorCodeMCPUnsupportedProtocolVersion, errMsg), nil
	}
	s.logger.Info("Session %s: Received InitializeRequest from client: %s (Version: %s)", session.SessionID(), initParams.ClientInfo.Name, initParams.ClientInfo.Version)
	s.logger.Debug("Session %s: Client Capabilities: %+v", session.SessionID(), initParams.Capabilities)
	// TODO: Store client capabilities per session

	serverCaps := s.serverCapabilities
	responsePayload := protocol.InitializeResult{
		ProtocolVersion: protocol.CurrentProtocolVersion,
		Capabilities:    serverCaps,
		ServerInfo:      protocol.Implementation{Name: s.serverName, Version: "0.1.0"},
	}
	resp := createSuccessResponse(requestID, responsePayload)
	return resp, nil
}

func (s *Server) handleInitializedNotification(ctx context.Context, session ClientSession, rawMessage json.RawMessage) error {
	var notif protocol.JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &notif); err != nil {
		return fmt.Errorf("failed to parse initialized notification: %w", err)
	}
	s.logger.Info("Session %s: Received InitializedNotification.", session.SessionID())
	return nil
}

// --- Request/Notification Routing (Post-Initialization) ---

func (s *Server) handleRequest(ctx context.Context, session ClientSession, id interface{}, method string, rawMessage json.RawMessage) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling request for session %s: Method=%s, ID=%v", session.SessionID(), method, id)
	var params interface{}
	var baseReq protocol.JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &baseReq); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to parse request: %v", err))
	}
	params = baseReq.Params
	handlerCtx := ctx

	switch method {
	case protocol.MethodListTools:
		return s.handleListToolsRequest(handlerCtx, id, params)
	case protocol.MethodCallTool:
		return s.handleCallToolRequest(handlerCtx, session, id, params)
	case protocol.MethodListResources:
		return s.handleListResources(handlerCtx, id, params)
	case protocol.MethodReadResource:
		return s.handleReadResource(handlerCtx, id, params)
	case protocol.MethodListPrompts:
		return s.handleListPrompts(handlerCtx, id, params)
	case protocol.MethodGetPrompt:
		return s.handleGetPrompt(handlerCtx, id, params)
	case protocol.MethodSubscribeResource:
		return s.handleSubscribeResource(handlerCtx, session, id, params)
	case protocol.MethodUnsubscribeResource:
		return s.handleUnsubscribeResource(handlerCtx, session, id, params)
	case protocol.MethodPing:
		return s.handlePing(handlerCtx, id, params)
	default:
		s.logger.Warn("Method not found for session %s: %s", session.SessionID(), method)
		return createErrorResponse(id, protocol.ErrorCodeMethodNotFound, fmt.Sprintf("Method '%s' not implemented", method))
	}
}

func (s *Server) handleNotification(ctx context.Context, session ClientSession, method string, rawMessage json.RawMessage) error {
	s.notificationMu.RLock()
	handler, ok := s.notificationHandlers[method]
	s.notificationMu.RUnlock()
	if !ok {
		s.logger.Info("No handler registered for notification method '%s' from session %s", method, session.SessionID())
		return nil
	}

	var baseNotif protocol.JSONRPCNotification
	if err := json.Unmarshal(rawMessage, &baseNotif); err != nil {
		s.logger.Error("Failed to parse params for notification %s from session %s: %v", method, session.SessionID(), err)
		return fmt.Errorf("failed to parse notification params: %w", err)
	}
	handlerCtx := ctx
	err := handler(handlerCtx, baseNotif.Params)
	if err != nil {
		s.logger.Error("Error executing notification handler for method %s from session %s: %v", method, session.SessionID(), err)
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

func (s *Server) handleListToolsRequest(ctx context.Context, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListToolsRequest")
	s.registryMu.RLock()
	tools := make([]protocol.Tool, 0, len(s.toolRegistry))
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	s.registryMu.RUnlock()
	return createSuccessResponse(id, protocol.ListToolsResult{Tools: tools})
}

func (s *Server) handleCallToolRequest(ctx context.Context, session ClientSession, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling CallToolRequest for session %s", session.SessionID())
	var requestParams protocol.CallToolParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
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

func (s *Server) handleListResources(ctx context.Context, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListResourcesRequest")
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

func (s *Server) handleListPrompts(ctx context.Context, id interface{}, params interface{}) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListPromptsRequest")
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

func (s *Server) handleSubscribeResource(ctx context.Context, session ClientSession, id interface{}, params interface{}) *protocol.JSONRPCResponse {
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

func (s *Server) handleUnsubscribeResource(ctx context.Context, session ClientSession, id interface{}, params interface{}) *protocol.JSONRPCResponse {
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
	return s.sendNotificationToSession(sessionI.(ClientSession), protocol.MethodProgress, params)
}

func (s *Server) NotifyResourceUpdated(resource protocol.Resource) {
	if resource.URI == "" {
		s.logger.Warn("NotifyResourceUpdated called with empty URI.")
		return
	}
	s.logger.Info("Resource %s updated, sending notifications.", resource.URI)
	params := protocol.ResourceUpdatedParams{Resource: resource}
	s.subscriptionMu.Lock()
	subscribedSessions := []ClientSession{}
	for sessionID, subs := range s.resourceSubscriptions {
		if subs[resource.URI] {
			if sessionI, ok := s.sessions.Load(sessionID); ok {
				subscribedSessions = append(subscribedSessions, sessionI.(ClientSession))
			}
		}
	}
	s.subscriptionMu.Unlock()
	for _, session := range subscribedSessions {
		go func(sess ClientSession) {
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
		session := value.(ClientSession)
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

func (s *Server) sendNotificationToSession(session ClientSession, method string, params interface{}) error {
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
