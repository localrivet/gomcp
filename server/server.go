// Package server provides the MCP server implementation.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/localrivet/gomcp/auth"
	"github.com/localrivet/gomcp/hooks" // Added hooks import
	"github.com/localrivet/gomcp/logx"  // Added logx import
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// NotificationHandlerFunc defines the signature for functions that handle client-to-server notifications.
// Note: ToolHandlerFunc is now defined in the hooks package as hooks.FinalToolHandler
type NotificationHandlerFunc func(ctx context.Context, params interface{}) error

// Local ToolHandlerFunc definition removed. Use hooks.FinalToolHandler instead.

// Server represents the core MCP server logic, independent of transport.
type Server struct {
	serverName         string
	logger             types.Logger
	serverInstructions string // Added for InitializeResult

	// Registries
	toolRegistry     map[string]protocol.Tool
	toolHandlers     map[string]hooks.FinalToolHandler // Use type from hooks package
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

	// Hooks
	hooksMu                             sync.RWMutex // Separate mutex for hook registration
	beforeHandleMessageHooks            []hooks.BeforeHandleMessageHook
	beforeUnmarshalHooks                []hooks.BeforeUnmarshalHook
	serverBeforeHandleRequestHooks      []hooks.ServerBeforeHandleRequestHook
	serverBeforeHandleNotificationHooks []hooks.ServerBeforeHandleNotificationHook
	beforeToolCallHooks                 []hooks.BeforeToolCallHook
	afterToolCallHooks                  []hooks.AfterToolCallHook
	beforeSendResponseHooks             []hooks.BeforeSendResponseHook
	serverBeforeSendNotificationHooks   []hooks.ServerBeforeSendNotificationHook
	onSessionCreateHooks                []hooks.OnSessionCreateHook
	beforeSessionDestroyHooks           []hooks.BeforeSessionDestroyHook

	// Auth
	permissionChecker auth.PermissionChecker // Added for authorization checks
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

// WithResourceSubscription sets the resource subscription capability flag.
// It ensures the Resources capability struct is initialized.
func WithResourceSubscription(subscribe bool) ServerOption {
	return func(s *Server) {
		if s.serverCapabilities.Resources == nil {
			s.serverCapabilities.Resources = &struct {
				Subscribe   bool `json:"subscribe,omitempty"`
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
		}
		s.serverCapabilities.Resources.Subscribe = subscribe
	}
}

// WithResourceListChanged sets the resource listChanged capability flag.
// It ensures the Resources capability struct is initialized.
func WithResourceListChanged(listChanged bool) ServerOption {
	return func(s *Server) {
		if s.serverCapabilities.Resources == nil {
			s.serverCapabilities.Resources = &struct {
				Subscribe   bool `json:"subscribe,omitempty"`
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
		}
		s.serverCapabilities.Resources.ListChanged = listChanged
	}
}

// WithPromptListChanged sets the prompt listChanged capability flag.
// It ensures the Prompts capability struct is initialized.
func WithPromptListChanged(listChanged bool) ServerOption {
	return func(s *Server) {
		if s.serverCapabilities.Prompts == nil {
			s.serverCapabilities.Prompts = &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
		}
		s.serverCapabilities.Prompts.ListChanged = listChanged
	}
}

// WithToolListChanged sets the tool listChanged capability flag.
// It ensures the Tools capability struct is initialized.
func WithToolListChanged(listChanged bool) ServerOption {
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

// --- Hook Registration Options ---

// WithBeforeHandleMessageHook adds one or more BeforeHandleMessageHook functions.
func WithBeforeHandleMessageHook(hooks ...hooks.BeforeHandleMessageHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.beforeHandleMessageHooks = append(s.beforeHandleMessageHooks, hooks...)
	}
}

// WithBeforeUnmarshalHook adds one or more BeforeUnmarshalHook functions.
func WithBeforeUnmarshalHook(hooks ...hooks.BeforeUnmarshalHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.beforeUnmarshalHooks = append(s.beforeUnmarshalHooks, hooks...)
	}
}

// WithServerBeforeHandleRequestHook adds one or more ServerBeforeHandleRequestHook functions.
func WithServerBeforeHandleRequestHook(hooks ...hooks.ServerBeforeHandleRequestHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.serverBeforeHandleRequestHooks = append(s.serverBeforeHandleRequestHooks, hooks...)
	}
}

// WithServerBeforeHandleNotificationHook adds one or more ServerBeforeHandleNotificationHook functions.
func WithServerBeforeHandleNotificationHook(hooks ...hooks.ServerBeforeHandleNotificationHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.serverBeforeHandleNotificationHooks = append(s.serverBeforeHandleNotificationHooks, hooks...)
	}
}

// WithBeforeToolCallHook adds one or more BeforeToolCallHook functions.
func WithBeforeToolCallHook(hooks ...hooks.BeforeToolCallHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.beforeToolCallHooks = append(s.beforeToolCallHooks, hooks...)
	}
}

// WithAfterToolCallHook adds one or more AfterToolCallHook functions.
func WithAfterToolCallHook(hooks ...hooks.AfterToolCallHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.afterToolCallHooks = append(s.afterToolCallHooks, hooks...)
	}
}

// WithBeforeSendResponseHook adds one or more BeforeSendResponseHook functions.
func WithBeforeSendResponseHook(hooks ...hooks.BeforeSendResponseHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.beforeSendResponseHooks = append(s.beforeSendResponseHooks, hooks...)
	}
}

// WithServerBeforeSendNotificationHook adds one or more ServerBeforeSendNotificationHook functions.
func WithServerBeforeSendNotificationHook(hooks ...hooks.ServerBeforeSendNotificationHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.serverBeforeSendNotificationHooks = append(s.serverBeforeSendNotificationHooks, hooks...)
	}
}

// WithOnSessionCreateHook adds one or more OnSessionCreateHook functions.
func WithOnSessionCreateHook(hooks ...hooks.OnSessionCreateHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.onSessionCreateHooks = append(s.onSessionCreateHooks, hooks...)
	}
}

// WithBeforeSessionDestroyHook adds one or more BeforeSessionDestroyHook functions.
func WithBeforeSessionDestroyHook(hooks ...hooks.BeforeSessionDestroyHook) ServerOption {
	return func(s *Server) {
		s.hooksMu.Lock()
		defer s.hooksMu.Unlock()
		s.beforeSessionDestroyHooks = append(s.beforeSessionDestroyHooks, hooks...)
	}
}

// NewServer creates a new core MCP Server logic instance with the provided options.
func NewServer(serverName string, opts ...ServerOption) *Server {
	// Initialize with default values
	srv := &Server{
		serverName: serverName,
		logger:     logx.NewDefaultLogger(), // Use logx default logger
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
		toolHandlers:          make(map[string]hooks.FinalToolHandler), // Use type from hooks package
		resourceRegistry:      make(map[string]protocol.Resource),
		promptRegistry:        make(map[string]protocol.Prompt),
		activeRequests:        make(map[string]context.CancelFunc),
		notificationHandlers:  make(map[string]NotificationHandlerFunc),
		resourceSubscriptions: make(map[string]map[string]bool),

		// Initialize hook slices
		beforeHandleMessageHooks:            make([]hooks.BeforeHandleMessageHook, 0),
		beforeUnmarshalHooks:                make([]hooks.BeforeUnmarshalHook, 0),
		serverBeforeHandleRequestHooks:      make([]hooks.ServerBeforeHandleRequestHook, 0),
		serverBeforeHandleNotificationHooks: make([]hooks.ServerBeforeHandleNotificationHook, 0),
		beforeToolCallHooks:                 make([]hooks.BeforeToolCallHook, 0),
		afterToolCallHooks:                  make([]hooks.AfterToolCallHook, 0),
		beforeSendResponseHooks:             make([]hooks.BeforeSendResponseHook, 0),
		serverBeforeSendNotificationHooks:   make([]hooks.ServerBeforeSendNotificationHook, 0),
		onSessionCreateHooks:                make([]hooks.OnSessionCreateHook, 0),
		beforeSessionDestroyHooks:           make([]hooks.BeforeSessionDestroyHook, 0),
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

	// --- Execute OnSessionCreate hooks ---
	s.hooksMu.RLock()
	hooksToRun := make([]hooks.OnSessionCreateHook, len(s.onSessionCreateHooks))
	copy(hooksToRun, s.onSessionCreateHooks)
	s.hooksMu.RUnlock()

	if len(hooksToRun) > 0 {
		hookCtx := hooks.ServerHookContext{
			// Ctx might need to be derived or passed in if RegisterSession gets context
			Ctx:       context.Background(), // Placeholder context
			Session:   session,
			MessageID: nil, // Not applicable for session creation
			Method:    "",  // Not applicable for session creation
		}
		for _, hook := range hooksToRun {
			if err := hook(hookCtx); err != nil {
				// Log error but don't prevent session registration? Or should it?
				// For now, log and continue. Could make this configurable later.
				s.logger.Error("Error executing OnSessionCreateHook for session %s: %v", sessionID, err)
			}
		}
	}
	// --- End Hook Execution ---

	return nil
}

func (s *Server) UnregisterSession(sessionID string) {
	s.logger.Info("UnregisterSession called for session: %s", sessionID)

	// --- Execute BeforeSessionDestroy hooks ---
	sessionI, sessionExists := s.sessions.Load(sessionID)
	if sessionExists {
		session := sessionI.(types.ClientSession) // Cast to the interface
		s.hooksMu.RLock()
		hooksToRun := make([]hooks.BeforeSessionDestroyHook, len(s.beforeSessionDestroyHooks))
		copy(hooksToRun, s.beforeSessionDestroyHooks)
		s.hooksMu.RUnlock()

		if len(hooksToRun) > 0 {
			hookCtx := hooks.ServerHookContext{
				// Ctx might need to be derived or passed in if UnregisterSession gets context
				Ctx:       context.Background(), // Placeholder context
				Session:   session,
				MessageID: nil, // Not applicable
				Method:    "",  // Not applicable
			}
			for _, hook := range hooksToRun {
				if err := hook(hookCtx); err != nil {
					// Log error but proceed with unregistration
					s.logger.Error("Error executing BeforeSessionDestroyHook for session %s: %v", sessionID, err)
				}
			}
		}
	} else {
		s.logger.Warn("Attempted to run BeforeSessionDestroyHook for non-existent session: %s", sessionID)
	}
	// --- End Hook Execution ---

	// Now proceed with actual unregistration
	_, loaded := s.sessions.LoadAndDelete(sessionID)
	s.logger.Info("Session %s: LoadAndDelete executed. Loaded: %t", sessionID, loaded)

	s.subscriptionMu.Lock()
	delete(s.resourceSubscriptions, sessionID)
	s.subscriptionMu.Unlock()
	s.logger.Info("Session %s: Deleted from resourceSubscriptions", sessionID)

	if loaded {
		s.logger.Info("Unregistered session: %s", sessionID)
	} else {
		s.logger.Warn("Attempted to unregister non-existent session: %s", sessionID)
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

	// --- Get Session ---
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		s.logger.Error("Received message for unknown session ID: %s", sessionID)
		// Cannot determine request ID here, so return nil response slice. Error logged.
		return nil
	}
	session := sessionI.(types.ClientSession)

	// --- Execute BeforeHandleMessage hooks ---
	var err error
	currentRawMessage := rawMessage // Start with the original message
	s.hooksMu.RLock()
	bhMsgHooks := make([]hooks.BeforeHandleMessageHook, len(s.beforeHandleMessageHooks))
	copy(bhMsgHooks, s.beforeHandleMessageHooks)
	s.hooksMu.RUnlock()

	for _, hook := range bhMsgHooks {
		currentRawMessage, err = hook(ctx, session, currentRawMessage)
		if err != nil {
			s.logger.Error("Error executing BeforeHandleMessageHook for session %s: %v. Stopping processing.", sessionID, err)
			// Cannot determine request ID here, return nil response slice.
			return nil
		}
	}
	// Use the potentially modified message going forward
	rawMessage = currentRawMessage
	// --- End Hook Execution ---

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
	// Use the potentially modified rawMessage from hooks
	if err := json.Unmarshal(rawMessage, &baseMessage); err != nil {
		s.logger.Error("Session %s: Failed to parse base message structure: %v. Raw: %s", sessionID, err, string(rawMessage))
		// Run hook for parse error response
		errResp := createErrorResponse(nil, protocol.ErrorCodeParseError, fmt.Sprintf("Failed to parse JSON: %v", err))
		modifiedResp, hookErr := s.runBeforeSendResponseHooks(ctx, session, errResp) // Session is known here
		if hookErr != nil {
			s.logger.Error("Error in BeforeSendResponseHook for ParseError: %v. Sending original error.", hookErr)
		}
		// Return modified (or original if hook error) response. If hook suppressed, modifiedResp is nil.
		return modifiedResp
	}

	// --- Execute BeforeUnmarshal hooks ---
	s.hooksMu.RLock()
	bUnmarshalHooks := make([]hooks.BeforeUnmarshalHook, len(s.beforeUnmarshalHooks))
	copy(bUnmarshalHooks, s.beforeUnmarshalHooks)
	s.hooksMu.RUnlock()

	if len(bUnmarshalHooks) > 0 {
		hookCtx := hooks.ServerHookContext{
			Ctx:       ctx,
			Session:   session,
			MessageID: baseMessage.ID,
			Method:    baseMessage.Method,
		}
		for _, hook := range bUnmarshalHooks {
			if err := hook(hookCtx, baseMessage.Params); err != nil {
				s.logger.Error("Error executing BeforeUnmarshalHook for session %s, method %s: %v. Stopping processing.", sessionID, baseMessage.Method, err)
				// Attempt to return error response if ID is available
				return createErrorResponse(baseMessage.ID, protocol.ErrorCodeInternalError, fmt.Sprintf("Hook error before unmarshal: %v", err))
			}
		}
	}
	// --- End Hook Execution ---

	// Basic validation
	if baseMessage.JSONRPC != "2.0" {
		s.logger.Warn("Session %s: Received message with invalid jsonrpc version: %s", sessionID, baseMessage.JSONRPC)
		errResp := createErrorResponse(baseMessage.ID, protocol.ErrorCodeInvalidRequest, "Invalid jsonrpc version")
		modifiedResp, hookErr := s.runBeforeSendResponseHooks(ctx, session, errResp)
		if hookErr != nil {
			s.logger.Error("Error in BeforeSendResponseHook for InvalidVersion: %v. Sending original error.", hookErr)
		}
		return modifiedResp
	}

	// Handle Initialization Phase (should not happen in batch, but check anyway)
	if !session.Initialized() {
		// Initialization messages MUST NOT be batched according to some interpretations,
		// handle them specifically before the main request/notification logic.
		if baseMessage.Method == protocol.MethodInitialize && baseMessage.ID != nil {
			return s.handleInitializationMessage(ctx, session, baseMessage.ID, baseMessage.Method, rawMessage)
			// Accept both "initialized" and "notifications/initialized" for compatibility
		} else if (baseMessage.Method == protocol.MethodInitialized || baseMessage.Method == "notifications/initialized") && baseMessage.ID == nil {
			// Mark session as initialized immediately upon receiving the notification signal.
			// The handshake is complete at this point per the spec.
			session.Initialize()
			s.logger.Info("Session %s marked as initialized upon receiving 'initialized' notification.", sessionID)

			// Now, attempt to process/log the notification details (optional params etc.)
			// Failure here should not prevent the session from being initialized.
			err := s.handleInitializedNotification(ctx, session, rawMessage)
			if err != nil {
				s.logger.Warn("Error processing details of initialized notification for session %s: %v", sessionID, err)
				// Logged the warning, but session is already initialized.
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
		// --- Execute ServerBeforeHandleRequest hooks ---
		s.hooksMu.RLock()
		bHandleReqHooks := make([]hooks.ServerBeforeHandleRequestHook, len(s.serverBeforeHandleRequestHooks))
		copy(bHandleReqHooks, s.serverBeforeHandleRequestHooks)
		s.hooksMu.RUnlock()

		if len(bHandleReqHooks) > 0 {
			// Note: Params are still raw here. Hooks needing parsed params must do it themselves or wait for later hooks.
			// For now, pass rawParams by attempting a generic unmarshal.
			var genericParams any
			_ = json.Unmarshal(baseMessage.Params, &genericParams) // Attempt best-effort unmarshal

			hookCtx := hooks.ServerHookContext{
				Ctx:       ctx,
				Session:   session,
				MessageID: baseMessage.ID,
				Method:    baseMessage.Method,
			}
			for _, hook := range bHandleReqHooks {
				if err := hook(hookCtx, genericParams); err != nil {
					s.logger.Error("Error executing ServerBeforeHandleRequestHook for session %s, method %s: %v. Rejecting request.", sessionID, baseMessage.Method, err)
					// Return error response from hook
					return createErrorResponse(baseMessage.ID, protocol.ErrorCodeInternalError, fmt.Sprintf("Request rejected by hook: %v", err))
				}
			}
		}
		// --- End Hook Execution ---

		// Pass raw params to handleRequest
		return s.handleRequest(ctx, session, baseMessage.ID, baseMessage.Method, baseMessage.Params) // Pass raw params

	} else if isNotification {
		// --- Execute ServerBeforeHandleNotification hooks ---
		s.hooksMu.RLock()
		bHandleNotifHooks := make([]hooks.ServerBeforeHandleNotificationHook, len(s.serverBeforeHandleNotificationHooks))
		copy(bHandleNotifHooks, s.serverBeforeHandleNotificationHooks)
		s.hooksMu.RUnlock()

		if len(bHandleNotifHooks) > 0 {
			var genericParams any
			_ = json.Unmarshal(baseMessage.Params, &genericParams) // Attempt best-effort unmarshal

			hookCtx := hooks.ServerHookContext{
				Ctx:       ctx,
				Session:   session,
				MessageID: nil, // No ID for notifications
				Method:    baseMessage.Method,
			}
			for _, hook := range bHandleNotifHooks {
				if err := hook(hookCtx, genericParams); err != nil {
					// Log error, but typically don't stop notification processing unless critical
					s.logger.Error("Error executing ServerBeforeHandleNotificationHook for session %s, method %s: %v.", sessionID, baseMessage.Method, err)
					// Decide whether to stop processing based on error? For now, just log.
				}
			}
		}
		// --- End Hook Execution ---

		// Pass raw params to handleNotification
		err := s.handleNotification(ctx, session, baseMessage.Method, baseMessage.Params) // Pass raw params
		if err != nil {
			s.logger.Error("Error handling notification '%s' for session %s: %v", baseMessage.Method, sessionID, err)
		}
		return nil // No response for notifications

	} else {
		// Invalid message (e.g., missing method for notification)
		s.logger.Warn("Received message with no ID or Method for session %s: %s", sessionID, string(rawMessage))
		errResp := createErrorResponse(baseMessage.ID, protocol.ErrorCodeInvalidRequest, "Invalid message: must be request (with id) or notification (with method)")
		modifiedResp, hookErr := s.runBeforeSendResponseHooks(ctx, session, errResp)
		if hookErr != nil {
			s.logger.Error("Error in BeforeSendResponseHook for InvalidMessage: %v. Sending original error.", hookErr)
		}
		return modifiedResp
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

	// --- Advertise Auth Capability if Configured ---
	// Check the original server capabilities *before* potential adjustment for old protocol versions
	if s.permissionChecker != nil {
		// Ensure the Authorization capability struct exists in the *response* capabilities
		// (even if it was nilled out for old protocol version compatibility, signal support if configured)
		if responsePayload.Capabilities.Authorization == nil {
			responsePayload.Capabilities.Authorization = &struct{}{} // Add an empty struct to signal support
		}
		s.logger.Info("Session %s: Advertising Authorization capability because permission checker is configured.", session.SessionID())
	}
	// --- End Auth Capability Advertising ---

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
		// Pass session to handleListToolsRequest
		return s.handleListToolsRequest(handlerCtx, session, id, rawParams)
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

func (s *Server) RegisterTool(tool protocol.Tool, handler hooks.FinalToolHandler) error { // Use type from hooks package
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
// Added session parameter to support hooks needing session context.
func (s *Server) handleListToolsRequest(ctx context.Context, session types.ClientSession, id interface{}, rawParams json.RawMessage) *protocol.JSONRPCResponse {
	s.logger.Debug("Handling ListToolsRequest for session %s", session.SessionID())
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
	resp := createSuccessResponse(id, protocol.ListToolsResult{Tools: tools})
	// Run hook before returning
	modifiedResp, err := s.runBeforeSendResponseHooks(ctx, session, resp) // Assuming session is available or passed down
	if err != nil {
		s.logger.Error("Error in BeforeSendResponseHook for ListTools: %v. Sending original response.", err)
		// Decide if hook error should prevent sending? For now, send original.
		return resp
	}
	return modifiedResp
}

// handleCallToolRequest handles the 'tools/call' request. Params are expected to be json.RawMessage.
func (s *Server) handleCallToolRequest(ctx context.Context, session types.ClientSession, id interface{}, rawParams json.RawMessage) *protocol.JSONRPCResponse { // Use types.ClientSession
	s.logger.Debug("Handling CallToolRequest for session %s", session.SessionID())
	var requestParams protocol.CallToolParams
	if err := protocol.UnmarshalPayload(rawParams, &requestParams); err != nil { // Unmarshal from rawParams
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal CallTool params: %v", err))
	}
	s.registryMu.RLock()
	originalHandler, exists := s.toolHandlers[requestParams.Name]
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
	var progressToken interface{} // Changed type
	if requestParams.Meta != nil {
		progressToken = requestParams.Meta.ProgressToken // Assign interface{} directly
	}

	// --- Wrap handler with BeforeToolCall hooks ---
	finalHandler := originalHandler // Start with the actual handler
	s.hooksMu.RLock()
	// Iterate hooks in reverse to build the chain correctly: Hook1(Hook2(Hook3(originalHandler)))
	for i := len(s.beforeToolCallHooks) - 1; i >= 0; i-- {
		hook := s.beforeToolCallHooks[i]
		finalHandler = hook(finalHandler) // Wrap the current handler with the hook
	}
	s.hooksMu.RUnlock()
	// --- End Hook Wrapping ---

	// Call the final (potentially wrapped) handler
	// The arguments are passed directly; any modification happens within the hook wrappers.
	content, isError := finalHandler(reqCtx, progressToken, requestParams.Arguments)
	var toolErr error // Initialize toolErr, hooks might populate it

	// --- Get Tool Definition for Hooks ---
	s.registryMu.RLock()
	toolDef, toolDefExists := s.toolRegistry[requestParams.Name]
	s.registryMu.RUnlock()
	if !toolDefExists {
		// This should technically not happen if the handler was found, but safety check.
		s.logger.Error("Session %s: Tool definition not found for '%s' during hook processing.", session.SessionID(), requestParams.Name)
		// Return error, as the tool definition is needed for context.
		return createErrorResponse(id, protocol.ErrorCodeInternalError, fmt.Sprintf("Tool definition '%s' missing during hook processing", requestParams.Name))
	}

	// --- Execute AfterToolCall hooks ---
	s.hooksMu.RLock()
	aToolHooks := make([]hooks.AfterToolCallHook, len(s.afterToolCallHooks))
	copy(aToolHooks, s.afterToolCallHooks)
	s.hooksMu.RUnlock()

	if len(aToolHooks) > 0 {
		// Create context WITH the tool definition
		hookCtx := hooks.ServerHookContext{
			Ctx:            reqCtx,
			Session:        session,
			MessageID:      id,
			Method:         protocol.MethodCallTool, // Corrected method constant if needed
			ToolDefinition: &toolDef,                // Add the tool definition here
		}
		// Start with the results from the handler
		currentContent := content
		currentIsError := isError
		currentToolErr := toolErr // This is currently nil if Before hook didn't error

		for _, hook := range aToolHooks {
			// Pass the *current* state and context to the After hook (signature updated)
			currentContent, currentIsError, currentToolErr = hook(
				hookCtx, // Pass context with ToolDefinition
				requestParams.Arguments,
				currentContent,
				currentIsError,
				currentToolErr, // Pass the error state *before* this hook runs
			)
			// Note: currentToolErr could be set/modified by the hook here
		}
		// Update results based on the final state after all hooks ran
		content = currentContent
		isError = currentIsError
		// If hook introduced an error, reflect it
		if currentToolErr != nil && !isError {
			// If the handler didn't report an error but a hook did, log it and set isError
			s.logger.Error("Error introduced by AfterToolCallHook for tool %s: %v.", toolDef.Name, currentToolErr)
			isError = true
			// Optionally modify content or return a specific error response based on hook error
			// For now, just setting isError=true to indicate failure.
		} else if currentToolErr != nil && isError {
			// Log if hook modified an existing error (optional logging)
			s.logger.Warn("AfterToolCallHook for tool %s modified error state: %v", toolDef.Name, currentToolErr)
		} else if currentToolErr == nil && toolErr != nil {
			// Log if hook cleared an error (optional logging)
			s.logger.Warn("AfterToolCallHook for tool %s cleared a previous error.", toolDef.Name)
		}
	}
	// --- End Hook Execution ---

	responsePayload := protocol.CallToolResult{Content: content}
	if isError {
		responsePayload.IsError = &isError
	}
	resp := createSuccessResponse(id, responsePayload)
	// Run hook before returning
	modifiedResp, err := s.runBeforeSendResponseHooks(reqCtx, session, resp) // Use reqCtx and session
	if err != nil {
		s.logger.Error("Error in BeforeSendResponseHook for CallTool %s: %v. Sending original response.", requestParams.Name, err)
		return resp // Send original on hook error
	}
	// Return potentially modified response (or nil if suppressed)
	return modifiedResp
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
	sessionID := session.SessionID()
	method := protocol.MethodSubscribeResource // Use correct constant
	s.logger.Debug("Session %s: Handling %s request ID %v", sessionID, method, id)

	// --- Authorization Check ---
	// Checking before unmarshalling params. Checker needs to handle interface{} or we unmarshal first.
	if s.permissionChecker != nil {
		principal, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			s.logger.Warn("Session %s: No principal found in context for %s request ID %v.", sessionID, method, id)
			return createErrorResponse(id, protocol.ErrorCodeMCPAuthenticationFailed, "Authentication required but no principal found")
		}
		if err := s.permissionChecker.CheckPermission(ctx, principal, method, params); err != nil {
			s.logger.Warn("Session %s: Permission denied for %s request ID %v by principal '%s': %v", sessionID, method, id, principal.GetSubject(), err)
			if mcpErr, ok := err.(*protocol.MCPError); ok {
				return createErrorResponse(id, mcpErr.Code, mcpErr.Message)
			}
			return createErrorResponse(id, protocol.ErrorCodeMCPAccessDenied, fmt.Sprintf("Permission denied for %s: %v", method, err))
		}
		s.logger.Debug("Session %s: Permission granted for %s request ID %v by principal '%s'", sessionID, method, id, principal.GetSubject())
	} else {
		s.logger.Debug("Session %s: No permission checker configured, skipping auth check for %s request ID %v.", sessionID, method, id)
	}
	// --- End Authorization Check ---

	var requestParams protocol.SubscribeResourceParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal %s params: %v", method, err))
	}
	// sessionID already defined above
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
	sessionID := session.SessionID()
	method := protocol.MethodUnsubscribeResource // Use correct constant
	s.logger.Debug("Session %s: Handling %s request ID %v", sessionID, method, id)

	// --- Authorization Check ---
	// Checking *after* unmarshalling params.
	var requestParams protocol.UnsubscribeResourceParams
	if err := protocol.UnmarshalPayload(params, &requestParams); err != nil {
		return createErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to unmarshal %s params: %v", method, err))
	}

	if s.permissionChecker != nil {
		principal, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			s.logger.Warn("Session %s: No principal found in context for %s request ID %v.", sessionID, method, id)
			return createErrorResponse(id, protocol.ErrorCodeMCPAuthenticationFailed, "Authentication required but no principal found")
		}
		// Pass parsed params to checker
		if err := s.permissionChecker.CheckPermission(ctx, principal, method, requestParams); err != nil {
			s.logger.Warn("Session %s: Permission denied for %s request ID %v by principal '%s': %v", sessionID, method, id, principal.GetSubject(), err)
			if mcpErr, ok := err.(*protocol.MCPError); ok {
				return createErrorResponse(id, mcpErr.Code, mcpErr.Message)
			}
			return createErrorResponse(id, protocol.ErrorCodeMCPAccessDenied, fmt.Sprintf("Permission denied for %s: %v", method, err))
		}
		s.logger.Debug("Session %s: Permission granted for %s request ID %v by principal '%s'", sessionID, method, id, principal.GetSubject())
	} else {
		s.logger.Debug("Session %s: No permission checker configured, skipping auth check for %s request ID %v.", sessionID, method, id)
	}
	// --- End Authorization Check ---

	// sessionID already defined above
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
	// Spec requires the result to be an empty JSON object `{}`
	return createSuccessResponse(id, map[string]interface{}{})
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

// --- Hook Execution Helper ---

// runBeforeSendResponseHooks executes the registered hooks before sending a response.
// It returns the potentially modified response or an error if a hook failed.
// If a hook returns (nil, nil), it means the response should be suppressed.
func (s *Server) runBeforeSendResponseHooks(ctx context.Context, session types.ClientSession, response *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	s.hooksMu.RLock()
	hooksToRun := make([]hooks.BeforeSendResponseHook, len(s.beforeSendResponseHooks))
	copy(hooksToRun, s.beforeSendResponseHooks)
	s.hooksMu.RUnlock()

	if len(hooksToRun) == 0 || response == nil { // Also check if response is nil
		return response, nil // No hooks or nothing to hook
	}

	currentResponse := response
	var err error

	// Create hook context - session might be nil if called outside session context (e.g., generic error)
	hookCtx := hooks.ServerHookContext{
		Ctx:       ctx,
		Session:   session,     // Can be nil
		MessageID: response.ID, // Get ID from response
		Method:    "",          // Method isn't directly known here, maybe add if needed?
	}

	for _, hook := range hooksToRun {
		currentResponse, err = hook(hookCtx, currentResponse)
		if err != nil {
			// Return the error and the *original* response
			return response, fmt.Errorf("BeforeSendResponseHook failed: %w", err)
		}
		if currentResponse == nil {
			// Hook decided to suppress the response entirely
			s.logger.Warn("BeforeSendResponseHook suppressed response for ID %v", response.ID)
			return nil, nil // Indicate response should not be sent
		}
	}
	return currentResponse, nil
}

// --- Hook Execution Helper ---

// Removed duplicate runBeforeSendResponseHooks function and defaultLogger

// ServeStdio helper moved to stdio_server.go
