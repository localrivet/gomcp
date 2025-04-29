// Package client provides the MCP client implementation.
package client

import (
	"bytes" // Added for TrimSpace
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io" // For EOF check in receive loop
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/hooks" // Added hooks import
	"github.com/localrivet/gomcp/logx"  // Import the new logger package
	"github.com/localrivet/gomcp/protocol"

	// sse "github.com/localrivet/gomcp/transport/sse" // REMOVED - No longer needed for type assertion
	"github.com/localrivet/gomcp/types"
)

// Client represents a generic MCP client instance.
type Client struct {
	transport  types.Transport // Abstract transport interface
	clientName string
	logger     types.Logger

	// Server info and capabilities
	serverInfo         protocol.Implementation
	serverCapabilities protocol.ServerCapabilities
	clientCapabilities protocol.ClientCapabilities
	negotiatedVersion  string // Store the negotiated protocol version
	capabilitiesMu     sync.RWMutex

	// Request handling
	pendingRequests map[string]chan *protocol.JSONRPCResponse
	pendingMu       sync.Mutex

	// Request handlers (for server-to-client requests)
	requestHandlers map[string]RequestHandlerFunc
	handlerMu       sync.RWMutex
	// Specific handler for Sampling requests
	samplingHandler SamplingRequestHandlerFunc // Added for Sampling

	// Notification handlers (for server-to-client notifications)
	notificationHandlers map[string]NotificationHandlerFunc
	notificationMu       sync.RWMutex

	// Client-side state
	roots   map[string]protocol.Root // Changed to map for easier updates
	rootsMu sync.RWMutex             // Changed to RWMutex

	preferredVersion string // Store the preferred version for initialization

	// Client state
	initialized bool
	// 'connected' is implicitly managed by transport state and initialized flag
	closed  bool
	stateMu sync.RWMutex

	// Message processing
	processingCtx    context.Context
	processingCancel context.CancelFunc
	processingWg     sync.WaitGroup

	// Hooks
	hooksMu                             sync.RWMutex // Mutex for hook registration
	beforeSendRequestHooks              []hooks.ClientBeforeSendRequestHook
	beforeSendNotificationHooks         []hooks.ClientBeforeSendNotificationHook
	onReceiveRawMessageHooks            []hooks.OnReceiveRawMessageHook
	beforeHandleResponseHooks           []hooks.ClientBeforeHandleResponseHook
	clientBeforeHandleNotificationHooks []hooks.ClientBeforeHandleNotificationHook
	clientBeforeHandleRequestHooks      []hooks.ClientBeforeHandleRequestHook
}

// RequestHandlerFunc defines the signature for functions that handle generic server-to-client requests.
type RequestHandlerFunc func(ctx context.Context, id interface{}, params interface{}) error

// SamplingRequestHandlerFunc defines the signature for the specific sampling request handler.
// It must return a result or an error to be sent back to the server.
type SamplingRequestHandlerFunc func(ctx context.Context, id interface{}, params protocol.SamplingRequestParams) (*protocol.SamplingResult, error)

// NotificationHandlerFunc defines the signature for functions that handle server-to-client notifications.
type NotificationHandlerFunc func(ctx context.Context, params interface{}) error

// ClientOptions contains configuration options for creating a Client.
type ClientOptions struct {
	Logger                   types.Logger
	ClientCapabilities       protocol.ClientCapabilities // User can pre-configure capabilities including Roots.ListChanged
	PreferredProtocolVersion *string                     // Optional: Defaults to OldProtocolVersion
	Transport                types.Transport             // Required: Must be provided by specific constructors
	SamplingHandler          SamplingRequestHandlerFunc  // Added for Sampling
	InitialRoots             []protocol.Root             // Optional initial roots
	// Removed HTTP/SSE specific options

	// Hooks
	BeforeSendRequestHooks              []hooks.ClientBeforeSendRequestHook
	BeforeSendNotificationHooks         []hooks.ClientBeforeSendNotificationHook
	OnReceiveRawMessageHooks            []hooks.OnReceiveRawMessageHook
	BeforeHandleResponseHooks           []hooks.ClientBeforeHandleResponseHook
	ClientBeforeHandleNotificationHooks []hooks.ClientBeforeHandleNotificationHook
	ClientBeforeHandleRequestHooks      []hooks.ClientBeforeHandleRequestHook
}

// NewClient creates a new generic Client instance.
// Transport must be provided via ClientOptions by specific constructors (NewSSEClient, NewWebSocketClient, etc.).
func NewClient(clientName string, opts ClientOptions) (*Client, error) {
	logger := opts.Logger
	if logger == nil {
		logger = logx.NewDefaultLogger() // Use the new logger package
	}

	if opts.Transport == nil {
		return nil, fmt.Errorf("transport is required in ClientOptions")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Determine preferred version
	clientPreferredVersion := protocol.OldProtocolVersion // Default to old version for compatibility
	if opts.PreferredProtocolVersion != nil {
		clientPreferredVersion = *opts.PreferredProtocolVersion
	}

	// Ensure sampling capability is set if handler is provided
	clientCaps := opts.ClientCapabilities // Start with user-provided caps
	if opts.SamplingHandler != nil {
		if clientCaps.Sampling == nil {
			clientCaps.Sampling = &protocol.SamplingCapability{}
		}
		clientCaps.Sampling.Enabled = true // Advertise support if handler is present
	}
	// Ensure Roots capability struct exists if ListChanged is set or initial roots provided
	hasInitialRoots := len(opts.InitialRoots) > 0
	rootsListChangedSet := clientCaps.Roots != nil && clientCaps.Roots.ListChanged
	if rootsListChangedSet || hasInitialRoots {
		if clientCaps.Roots == nil {
			clientCaps.Roots = &struct {
				ListChanged bool `json:"listChanged,omitempty"`
			}{}
			// If initial roots are provided but ListChanged wasn't explicitly set by user,
			// we still need the struct, but don't force ListChanged=true.
			// The user must explicitly set clientCaps.Roots.ListChanged = true if they want notifications.
		}
	}

	// Initialize roots map
	initialRootsMap := make(map[string]protocol.Root)
	if opts.InitialRoots != nil {
		for _, root := range opts.InitialRoots {
			initialRootsMap[root.URI] = root
		}
	}

	c := &Client{
		clientName:           clientName,
		transport:            opts.Transport, // Store the provided transport
		preferredVersion:     clientPreferredVersion,
		logger:               logger,
		clientCapabilities:   clientCaps, // Use potentially modified caps
		pendingRequests:      make(map[string]chan *protocol.JSONRPCResponse),
		requestHandlers:      make(map[string]RequestHandlerFunc),
		samplingHandler:      opts.SamplingHandler, // Store sampling handler
		notificationHandlers: make(map[string]NotificationHandlerFunc),
		roots:                initialRootsMap, // Initialize with provided roots
		initialized:          false,
		closed:               false,
		processingCtx:        ctx,
		processingCancel:     cancel,

		// Initialize hook slices
		beforeSendRequestHooks:              make([]hooks.ClientBeforeSendRequestHook, 0),
		beforeSendNotificationHooks:         make([]hooks.ClientBeforeSendNotificationHook, 0),
		onReceiveRawMessageHooks:            make([]hooks.OnReceiveRawMessageHook, 0),
		beforeHandleResponseHooks:           make([]hooks.ClientBeforeHandleResponseHook, 0),
		clientBeforeHandleNotificationHooks: make([]hooks.ClientBeforeHandleNotificationHook, 0),
		clientBeforeHandleRequestHooks:      make([]hooks.ClientBeforeHandleRequestHook, 0),
	}

	// Copy hooks from options, ensuring nil slices are handled gracefully
	if opts.BeforeSendRequestHooks != nil {
		c.beforeSendRequestHooks = append(c.beforeSendRequestHooks, opts.BeforeSendRequestHooks...)
	}
	if opts.BeforeSendNotificationHooks != nil {
		c.beforeSendNotificationHooks = append(c.beforeSendNotificationHooks, opts.BeforeSendNotificationHooks...)
	}
	if opts.OnReceiveRawMessageHooks != nil {
		c.onReceiveRawMessageHooks = append(c.onReceiveRawMessageHooks, opts.OnReceiveRawMessageHooks...)
	}
	if opts.BeforeHandleResponseHooks != nil {
		c.beforeHandleResponseHooks = append(c.beforeHandleResponseHooks, opts.BeforeHandleResponseHooks...)
	}
	if opts.ClientBeforeHandleNotificationHooks != nil {
		c.clientBeforeHandleNotificationHooks = append(c.clientBeforeHandleNotificationHooks, opts.ClientBeforeHandleNotificationHooks...)
	}
	if opts.ClientBeforeHandleRequestHooks != nil {
		c.clientBeforeHandleRequestHooks = append(c.clientBeforeHandleRequestHooks, opts.ClientBeforeHandleRequestHooks...)
	}

	logger.Info("MCP Client '%s' created", clientName)
	return c, nil
}

// Connect performs the MCP handshake using the configured transport.
func (c *Client) Connect(ctx context.Context) error {
	c.stateMu.Lock()
	// Check the initialized and closed state directly instead of calling IsConnected()
	// while holding the lock, which would cause a deadlock
	if c.initialized && !c.closed {
		c.stateMu.Unlock()
		return fmt.Errorf("client is already connected and initialized")
	}
	if c.closed {
		c.stateMu.Unlock()
		return fmt.Errorf("client is closed")
	}
	c.stateMu.Unlock()

	c.logger.Info("Client '%s' starting connection & initialization...", c.clientName)

	// Start the message processing loop that uses c.transport.Receive
	c.startMessageProcessing()

	// --- Step 1: Establish Receiver (e.g., SSE GET stream) ---
	c.logger.Info("Establishing transport receiver...")
	if err := c.transport.EstablishReceiver(ctx); err != nil {
		// Attempt to close transport if receiver setup fails
		_ = c.Close()
		return fmt.Errorf("failed to establish transport receiver: %w", err)
	}
	c.logger.Info("Transport receiver established.")

	// --- Step 2: Send Initialize Request & Process Response ---
	c.logger.Info("Sending InitializeRequest via transport...")
	clientInfo := protocol.Implementation{Name: c.clientName, Version: "0.1.0"} // Using fixed version for now
	initParams := protocol.InitializeRequestParams{
		ProtocolVersion: c.preferredVersion, // Use stored preferred version
		Capabilities:    c.clientCapabilities,
		ClientInfo:      clientInfo,
	}
	initReqID := uuid.NewString() // Generate ID here
	// initReqPayload := protocol.JSONRPCRequest{JSONRPC: "2.0", ID: initReqID, Method: protocol.MethodInitialize, Params: initParams} // No longer needed
	// initReqBytes, err := json.Marshal(initReqPayload) // No longer needed, sendRequestAndWait handles marshaling
	// if err != nil {
	// 	return fmt.Errorf("failed to marshal initialize request: %w", err)
	// }

	// Use a specific timeout for the initialize request itself
	initTimeout := 60 * time.Second
	initCtx, initCancel := context.WithTimeout(ctx, initTimeout)
	defer initCancel()

	// --- Removed SSE Specific Handling ---
	// All transports now use sendRequestAndWait for initialize,
	// after EstablishReceiver has been called.

	// --- Standard Transport Handling (Applies to all transports now) ---
	c.logger.Debug("Using standard transport initialization flow (sendRequestAndWait).")
	// Use sendRequestAndWait for transports where response comes via Receive loop
	response, err := c.sendRequestAndWait(initCtx, protocol.MethodInitialize, initReqID, initParams, initTimeout)
	if err != nil {
		_ = c.Close() // Attempt to close transport on handshake failure
		return fmt.Errorf("initialization handshake failed: %w", err)
	}

	// Process the response received via the Receive loop
	if response.Error != nil {
		_ = c.Close()
		return fmt.Errorf("received error response during initialization: [%d] %s", response.Error.Code, response.Error.Message)
	}
	var initResult protocol.InitializeResult
	if err := protocol.UnmarshalPayload(response.Result, &initResult); err != nil {
		_ = c.Close()
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Check if server responded with a version we support
	serverSelectedVersion := initResult.ProtocolVersion
	if serverSelectedVersion != protocol.CurrentProtocolVersion && serverSelectedVersion != protocol.OldProtocolVersion {
		_ = c.Close()
		return fmt.Errorf("server selected unsupported protocol version: %s (client supports %s, %s)",
			serverSelectedVersion, protocol.CurrentProtocolVersion, protocol.OldProtocolVersion)
	}

	c.capabilitiesMu.Lock()
	c.serverInfo = initResult.ServerInfo
	c.serverCapabilities = initResult.Capabilities
	c.negotiatedVersion = serverSelectedVersion // Store the negotiated version
	c.capabilitiesMu.Unlock()
	c.logger.Info("Negotiated protocol version: %s", serverSelectedVersion)

	// --- Step 3: Send Initialized Notification ---
	c.logger.Info("Sending InitializedNotification via transport...")
	initNotifParams := protocol.InitializedNotificationParams{}
	initNotif := protocol.JSONRPCNotification{JSONRPC: "2.0", Method: protocol.MethodInitialized, Params: initNotifParams}
	// Use a short timeout context for sending the notification
	notifCtx, notifCancel := context.WithTimeout(ctx, 5*time.Second)
	defer notifCancel()
	if err := c.sendNotification(notifCtx, initNotif); err != nil {
		// This is not necessarily fatal, but log it.
		c.logger.Warn("Failed to send initialized notification: %v", err)
		// Allow proceeding but log the warning.
	}

	c.stateMu.Lock()
	c.initialized = true
	// 'connected' state is now implicitly true if initialized is true and not closed
	c.stateMu.Unlock()

	// Log success - server info might not be available for SSE in this flow
	c.logger.Info("Initialization successful.")
	return nil
}

// --- Public Methods ---

// ServerInfo returns the information about the connected server.
func (c *Client) ServerInfo() protocol.Implementation {
	c.capabilitiesMu.RLock()
	defer c.capabilitiesMu.RUnlock()
	return c.serverInfo
}

// ServerCapabilities returns the capabilities reported by the server.
func (c *Client) ServerCapabilities() protocol.ServerCapabilities {
	c.capabilitiesMu.RLock()
	defer c.capabilitiesMu.RUnlock()
	return c.serverCapabilities
}

// IsConnected returns true if the client has successfully completed the handshake
// and the underlying transport is not known to be closed.
func (c *Client) IsConnected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	transportClosed := true // Assume closed if transport is nil
	if c.transport != nil {
		transportClosed = c.transport.IsClosed()
	}
	// Considered connected if initialized and neither client nor transport is closed.
	return c.initialized && !c.closed && !transportClosed
}

// IsInitialized returns true if the client has successfully completed the initialization handshake.
func (c *Client) IsInitialized() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.initialized && !c.closed
}

// IsClosed returns true if the client has been explicitly closed.
func (c *Client) IsClosed() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.closed
}

// CurrentState returns the current connection, initialization, and closed states.
func (c *Client) CurrentState() (connected bool, initialized bool, closed bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	transportClosed := true // Assume closed if transport is nil
	if c.transport != nil {
		transportClosed = c.transport.IsClosed()
	}
	connected = c.initialized && !c.closed && !transportClosed
	initialized = c.initialized && !c.closed
	closed = c.closed
	return
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandlerFunc) error {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	if _, exists := c.notificationHandlers[method]; exists {
		return fmt.Errorf("notification handler already registered: %s", method)
	}
	c.notificationHandlers[method] = handler
	c.logger.Info("Registered notification handler: %s", method)
	return nil
}

// RegisterRequestHandler registers a handler for generic server-to-client requests.
// Note: Sampling requests (sampling/request) should be handled via RegisterSamplingHandler.
func (c *Client) RegisterRequestHandler(method string, handler RequestHandlerFunc) error {
	if method == protocol.MethodSamplingRequest {
		return fmt.Errorf("use RegisterSamplingHandler for method '%s'", protocol.MethodSamplingRequest)
	}
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	if _, exists := c.requestHandlers[method]; exists {
		return fmt.Errorf("request handler already registered: %s", method)
	}
	c.requestHandlers[method] = handler
	c.logger.Info("Registered request handler: %s", method)
	return nil
}

// RegisterSamplingHandler registers the specific handler for 'sampling/request'.
// Providing a handler implies the client supports sampling and will advertise it.
func (c *Client) RegisterSamplingHandler(handler SamplingRequestHandlerFunc) error {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	if c.samplingHandler != nil {
		return fmt.Errorf("sampling handler already registered")
	}
	c.samplingHandler = handler
	// Ensure capability is advertised
	c.capabilitiesMu.Lock()
	if c.clientCapabilities.Sampling == nil {
		c.clientCapabilities.Sampling = &protocol.SamplingCapability{}
	}
	c.clientCapabilities.Sampling.Enabled = true
	c.capabilitiesMu.Unlock()
	c.logger.Info("Registered sampling handler and enabled sampling capability.")
	return nil
}

// GetRoots returns the current list of roots known to the client.
func (c *Client) GetRoots() []protocol.Root {
	c.rootsMu.RLock()
	defer c.rootsMu.RUnlock()
	rootsList := make([]protocol.Root, 0, len(c.roots))
	for _, root := range c.roots {
		rootsList = append(rootsList, root)
	}
	return rootsList
}

// SetRoots updates the client's list of roots.
// If the client advertised the 'roots.listChanged' capability, it sends a
// 'notifications/roots/list_changed' notification to the server.
func (c *Client) SetRoots(ctx context.Context, roots []protocol.Root) error {
	c.rootsMu.Lock()
	newRootsMap := make(map[string]protocol.Root, len(roots))
	changed := false
	if len(roots) != len(c.roots) {
		changed = true
	}
	for _, root := range roots {
		newRootsMap[root.URI] = root
		if !changed {
			if _, exists := c.roots[root.URI]; !exists { // Check if root is new
				changed = true
			}
			// Could add more detailed comparison if needed (e.g., check Title, Metadata)
		}
	}
	// Check if any roots were removed
	if !changed {
		for uri := range c.roots {
			if _, exists := newRootsMap[uri]; !exists {
				changed = true
				break
			}
		}
	}

	c.roots = newRootsMap
	c.rootsMu.Unlock() // Unlock before potentially sending notification

	if changed {
		c.logger.Info("Client roots updated.")
		// Check if we need to notify the server
		c.capabilitiesMu.RLock()
		shouldNotify := c.clientCapabilities.Roots != nil && c.clientCapabilities.Roots.ListChanged
		c.capabilitiesMu.RUnlock()

		if shouldNotify {
			c.logger.Debug("Sending roots list changed notification...")
			notif := protocol.JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  protocol.MethodNotifyRootsListChanged,
				Params:  protocol.RootsListChangedParams{}, // Empty params
			}
			// Use a short timeout for the notification
			notifCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := c.sendNotification(notifCtx, notif); err != nil {
				// Log error but don't fail the SetRoots operation itself
				c.logger.Error("Failed to send roots list changed notification: %v", err)
			} else {
				c.logger.Debug("Roots list changed notification sent.")
			}
		}
	} else {
		c.logger.Debug("SetRoots called but no changes detected.")
	}
	return nil
}

// ListTools sends a 'tools/list' request via the transport.
func (c *Client) ListTools(ctx context.Context, params protocol.ListToolsRequestParams) (*protocol.ListToolsResult, error) {
	timeout := 15 * time.Second // Default timeout for this request type
	requestID := uuid.NewString()
	response, err := c.sendRequestAndWait(ctx, protocol.MethodListTools, requestID, params, timeout)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("ListTools failed: [%d] %s", response.Error.Code, response.Error.Message)
	}
	var listResult protocol.ListToolsResult
	if err := protocol.UnmarshalPayload(response.Result, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListTools result: %w", err)
	}
	return &listResult, nil
}

// CallTool sends a 'tools/call' request via the transport.
func (c *Client) CallTool(ctx context.Context, params protocol.CallToolParams, progressToken interface{}) (*protocol.CallToolResult, error) {
	timeout := 60 * time.Second // Default timeout, adjust as needed
	if progressToken != nil {
		if params.Meta == nil {
			params.Meta = &protocol.RequestMeta{}
		}
		params.Meta.ProgressToken = progressToken
	}
	requestID := uuid.NewString()
	response, err := c.sendRequestAndWait(ctx, protocol.MethodCallTool, requestID, params, timeout)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("CallTool '%s' failed: [%d] %s", params.Name, response.Error.Code, response.Error.Message)
	}

	var callResult protocol.CallToolResult
	if err := protocol.UnmarshalPayload(response.Result, &callResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CallTool result: %w", err)
	}

	if callResult.IsError != nil && *callResult.IsError {
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", params.Name)
		if len(callResult.Content) > 0 {
			if textContent, ok := callResult.Content[0].(protocol.TextContent); ok {
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", params.Name, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", params.Name, callResult.Content[0])
			}
		}
		// Return the result along with the error for inspection
		return &callResult, fmt.Errorf("%s", errMsg)
	}
	return &callResult, nil
}

// CancelRequest sends a '$/cancelRequest' notification to the server.
func (c *Client) CancelRequest(ctx context.Context, id interface{}, reason *string) error {
	c.logger.Info("Sending cancel notification for request ID: %v", id)
	params := protocol.CancelledParams{
		ID:     id,
		Reason: reason,
	}
	notif := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  protocol.MethodCancelled,
		Params:  params,
	}
	// Use a short timeout context for sending the notification
	notifCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.sendNotification(notifCtx, notif)
}

// Ping sends a 'ping' request to the server and waits for the response.
func (c *Client) Ping(ctx context.Context, timeout time.Duration) error {
	c.logger.Debug("Sending ping request...")
	requestID := uuid.NewString()
	// Ping has no parameters
	response, err := c.sendRequestAndWait(ctx, protocol.MethodPing, requestID, nil, timeout)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("ping failed: [%d] %s", response.Error.Code, response.Error.Message)
	}
	// Success is indicated by a non-error response (result is usually null or empty object)
	c.logger.Debug("Ping successful.")
	return nil
}

// Close gracefully shuts down the client connection and its transport.
func (c *Client) Close() error {
	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		return nil // Already closed
	}
	c.closed = true
	// Mark as uninitialized immediately
	c.initialized = false
	c.stateMu.Unlock() // Unlock after setting closed flag

	c.logger.Info("Closing client '%s'...", c.clientName)

	// 1. Signal background processing loop to stop
	c.processingCancel()

	// 2. Close the underlying transport
	var transportErr error
	if c.transport != nil {
		c.logger.Debug("Closing transport...")
		transportErr = c.transport.Close()
		if transportErr != nil {
			c.logger.Error("Error closing transport: %v", transportErr)
		} else {
			c.logger.Debug("Transport closed.")
		}
	}

	// 3. Wait for the processing loop to finish
	c.logger.Debug("Waiting for processing loop to stop...")
	c.processingWg.Wait()
	c.logger.Debug("Processing loop stopped.")

	// 4. Close all pending request channels to unblock callers
	c.closePendingRequests(fmt.Errorf("client closed"))

	c.logger.Info("Client '%s' closed.", c.clientName)
	return transportErr // Return the error from closing the transport, if any
}

// --- Internal Methods ---

// sendRequestAndWait sends a request via the transport and waits for the response.
func (c *Client) sendRequestAndWait(ctx context.Context, method string, id interface{}, params interface{}, timeout time.Duration) (*protocol.JSONRPCResponse, error) {
	c.stateMu.RLock()
	isClosed := c.closed
	isInitialized := c.initialized
	c.stateMu.RUnlock()

	if isClosed {
		return nil, fmt.Errorf("client is closed")
	}
	// Allow initialize request even if not initialized
	if !isInitialized && method != protocol.MethodInitialize {
		return nil, fmt.Errorf("client is not initialized")
	}

	requestIDStr, ok := id.(string)
	if !ok {
		requestIDStr = fmt.Sprintf("%v", id)
	}

	// Create channel for response
	respChan := make(chan *protocol.JSONRPCResponse, 1)
	c.pendingMu.Lock()
	// Check again if closed after acquiring lock
	if c.closed {
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("client closed before request could be registered")
	}
	c.pendingRequests[requestIDStr] = respChan
	c.pendingMu.Unlock()

	// Prepare request payload
	request := &protocol.JSONRPCRequest{ // Use pointer for modification
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// --- Execute ClientBeforeSendRequest hooks ---
	c.hooksMu.RLock()
	bsReqHooks := make([]hooks.ClientBeforeSendRequestHook, len(c.beforeSendRequestHooks))
	copy(bsReqHooks, c.beforeSendRequestHooks)
	c.hooksMu.RUnlock()

	if len(bsReqHooks) > 0 {
		// Create context for hooks
		c.capabilitiesMu.RLock()
		hookCtx := hooks.ClientHookContext{
			Ctx:               ctx,
			ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"}, // Use actual client info if available
			NegotiatedVersion: c.negotiatedVersion,
			ServerInfo:        c.serverInfo,
			ServerCaps:        c.serverCapabilities,
			MessageID:         id,
			Method:            method,
		}
		c.capabilitiesMu.RUnlock()

		var hookErr error
		for _, hook := range bsReqHooks {
			request, hookErr = hook(hookCtx, request) // Pass pointer, update request
			if hookErr != nil {
				c.logger.Error("Error executing ClientBeforeSendRequestHook for method %s: %v. Aborting send.", method, hookErr)
				// Clean up pending request entry
				c.pendingMu.Lock()
				delete(c.pendingRequests, requestIDStr)
				c.pendingMu.Unlock()
				close(respChan) // Close channel to signal error
				return nil, fmt.Errorf("client before send request hook failed: %w", hookErr)
			}
			if request == nil { // Hook suppressed the request
				c.logger.Info("ClientBeforeSendRequestHook suppressed request for method %s.", method)
				// Clean up pending request entry
				c.pendingMu.Lock()
				delete(c.pendingRequests, requestIDStr)
				c.pendingMu.Unlock()
				close(respChan)                                             // Close channel to signal suppression
				return nil, fmt.Errorf("request suppressed by client hook") // Or return a specific error/nil?
			}
		}
	}
	// --- End Hook Execution ---

	// Marshal the potentially modified request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		// Clean up pending request entry
		c.pendingMu.Lock()
		delete(c.pendingRequests, requestIDStr)
		c.pendingMu.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to marshal potentially modified request: %w", err)
	}

	// Send request via the transport
	c.logger.Debug("Sending request via transport: %s %s", method, requestIDStr)
	// Use the context passed to sendRequestAndWait for the Send operation
	if err := c.transport.Send(ctx, requestBytes); err != nil {
		// Clean up pending request entry
		c.pendingMu.Lock()
		delete(c.pendingRequests, requestIDStr)
		c.pendingMu.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to send request via transport: %w", err)
	}

	// Wait for response via the transport's receive loop (handled in processMessage)
	// Use a timeout context derived from the original context
	waitCtx, waitCancel := context.WithTimeout(ctx, timeout)
	defer waitCancel()

	select {
	case resp := <-respChan:
		if resp == nil { // Channel closed by Close() or handleResponse error
			return nil, fmt.Errorf("connection closed or error occurred while waiting for response to %s (ID: %s)", method, requestIDStr)
		}
		return resp, nil
	case <-waitCtx.Done():
		err := waitCtx.Err()
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("request timed out after %v waiting for response to %s (ID: %s)", timeout, method, requestIDStr)
		}
		return nil, fmt.Errorf("context cancelled while waiting for response to %s (ID: %s): %w", method, requestIDStr, err)
	}
}

// sendNotification sends a notification via the transport.
func (c *Client) sendNotification(ctx context.Context, notification protocol.JSONRPCNotification) error {
	c.stateMu.RLock()
	isClosed := c.closed
	isInitialized := c.initialized
	c.stateMu.RUnlock()

	if isClosed {
		return fmt.Errorf("client is closed")
	}
	// Allow initialized notification even if not initialized
	if !isInitialized && notification.Method != protocol.MethodInitialized {
		return fmt.Errorf("client is not initialized")
	}

	notifPtr := &notification // Use pointer for modification

	// --- Execute ClientBeforeSendNotification hooks ---
	c.hooksMu.RLock()
	bsNotifHooks := make([]hooks.ClientBeforeSendNotificationHook, len(c.beforeSendNotificationHooks))
	copy(bsNotifHooks, c.beforeSendNotificationHooks)
	c.hooksMu.RUnlock()

	if len(bsNotifHooks) > 0 {
		// Create context for hooks
		c.capabilitiesMu.RLock()
		hookCtx := hooks.ClientHookContext{
			Ctx:               ctx,
			ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"}, // Use actual client info
			NegotiatedVersion: c.negotiatedVersion,
			ServerInfo:        c.serverInfo,
			ServerCaps:        c.serverCapabilities,
			MessageID:         nil, // Notifications have no ID
			Method:            notification.Method,
		}
		c.capabilitiesMu.RUnlock()

		var hookErr error
		for _, hook := range bsNotifHooks {
			notifPtr, hookErr = hook(hookCtx, notifPtr) // Pass pointer, update notification
			if hookErr != nil {
				c.logger.Error("Error executing ClientBeforeSendNotificationHook for method %s: %v. Aborting send.", notification.Method, hookErr)
				return fmt.Errorf("client before send notification hook failed: %w", hookErr)
			}
			if notifPtr == nil { // Hook suppressed the notification
				c.logger.Info("ClientBeforeSendNotificationHook suppressed notification for method %s.", notification.Method)
				return nil // Suppressed, not an error
			}
		}
	}
	// --- End Hook Execution ---

	// Marshal the potentially modified notification
	notifBytes, err := json.Marshal(notifPtr)
	if err != nil {
		return fmt.Errorf("failed to marshal potentially modified notification: %w", err)
	}

	c.logger.Debug("Sending notification via transport: %s", notification.Method)
	if err := c.transport.Send(ctx, notifBytes); err != nil {
		return fmt.Errorf("failed to send notification via transport: %w", err)
	}
	return nil
}

// startMessageProcessing starts the background goroutine for receiving messages.
func (c *Client) startMessageProcessing() {
	c.processingWg.Add(1)
	go func() {
		defer c.processingWg.Done()
		c.logger.Info("Starting transport message receiver loop...")
		for {
			select {
			case <-c.processingCtx.Done():
				c.logger.Info("Transport message receiver loop stopping due to context cancellation.")
				return
			default:
				// Blocking receive call using the processing context
				data, err := c.transport.Receive(c.processingCtx)
				if err != nil {
					// Check if the error is due to context cancellation or genuine error
					if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
						c.logger.Info("Transport Receive loop ending: %v", err)
					} else if c.transport.IsClosed() {
						// Check IsClosed explicitly after an error
						c.logger.Info("Transport Receive loop ending because transport is closed.")
					} else {
						c.logger.Error("Error receiving message from transport: %v", err)
						// Add a small delay to prevent tight loop on persistent errors?
						// time.Sleep(100 * time.Millisecond)
					}

					// If context is done or transport is closed, exit the loop.
					if c.processingCtx.Err() != nil || c.transport.IsClosed() {
						// Ensure client state reflects closure if transport closed unexpectedly
						if !c.IsClosed() {
							c.logger.Warn("Transport closed unexpectedly, closing client.")
							_ = c.Close() // Trigger client close logic
						}
						return
					}
					continue // Continue loop on recoverable errors if context is not done and transport not closed
				}

				// --- Execute OnReceiveRawMessage hooks ---
				currentData := data
				var hookErr error
				c.hooksMu.RLock()
				orMsgHooks := make([]hooks.OnReceiveRawMessageHook, len(c.onReceiveRawMessageHooks))
				copy(orMsgHooks, c.onReceiveRawMessageHooks)
				c.hooksMu.RUnlock()

				if len(orMsgHooks) > 0 {
					// Create context (minimal context available here)
					c.capabilitiesMu.RLock()
					hookCtx := hooks.ClientHookContext{
						Ctx:               c.processingCtx,                                               // Use the processing context
						ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"}, // Use actual client info
						NegotiatedVersion: c.negotiatedVersion,
						ServerInfo:        c.serverInfo,
						ServerCaps:        c.serverCapabilities,
						// MessageID and Method are not known yet
					}
					c.capabilitiesMu.RUnlock()

					for _, hook := range orMsgHooks {
						currentData, hookErr = hook(hookCtx, currentData)
						if hookErr != nil {
							c.logger.Error("Error executing OnReceiveRawMessageHook: %v. Skipping message.", hookErr)
							currentData = nil // Prevent processing
							break
						}
						if currentData == nil { // Hook suppressed the message
							c.logger.Info("OnReceiveRawMessageHook suppressed message.")
							break
						}
					}
				}
				// --- End Hook Execution ---

				if currentData != nil { // Only process if not suppressed or errored
					// Process the potentially modified message in a separate goroutine
					c.processingWg.Add(1)
					go func(msgData []byte) {
						defer c.processingWg.Done()
						if err := c.processMessage(msgData); err != nil {
							c.logger.Error("Error processing received message: %v", err)
						}
					}(currentData) // Pass potentially modified data
				}
			}
		}
	}()
}

// processMessage handles a single raw message received from the transport, AFTER OnReceiveRawMessageHook.
func (c *Client) processMessage(data []byte) error {
	c.logger.Debug("Processing received message: %s", string(data))

	// Trim whitespace and check if it's a batch (starts with '[')
	trimmedData := bytes.TrimSpace(data)
	if len(trimmedData) == 0 {
		c.logger.Warn("Received empty message, skipping.")
		return nil // Not an error, just nothing to process
	}
	isBatch := trimmedData[0] == '['

	if isBatch {
		// --- Handle Batch ---
		c.logger.Debug("Detected batch message.")
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmedData, &batch); err != nil {
			c.logger.Error("Failed to unmarshal batch message: %v", err)
			// According to JSON-RPC spec, should not return error response for batch parse failure.
			return fmt.Errorf("failed to parse batch JSON: %w", err)
		}

		if len(batch) == 0 {
			c.logger.Warn("Received empty batch array.")
			return nil // Not an error, just nothing to process
		}

		// Process each message in the batch
		var batchErrors []error
		for i, singleRawMsg := range batch {
			err := c.processSingleMessage(singleRawMsg)
			if err != nil {
				c.logger.Error("Error processing message %d in batch: %v", i+1, err)
				batchErrors = append(batchErrors, err)
				// Continue processing other messages in the batch
			}
		}

		if len(batchErrors) > 0 {
			// Return a combined error? Or just log? Logging seems sufficient for client-side.
			return fmt.Errorf("encountered %d error(s) while processing batch", len(batchErrors))
		}
		return nil // Batch processed (potentially with individual errors logged)

	} else {
		// --- Handle Single Message ---
		return c.processSingleMessage(trimmedData)
	}
}

// processSingleMessage handles a single JSON-RPC object (request, response, or notification).
func (c *Client) processSingleMessage(data []byte) error {
	var baseMessage struct {
		ID     interface{} `json:"id"`
		Method string      `json:"method"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		c.logger.Error("Failed to parse base structure of single message: %v", err)
		return fmt.Errorf("failed to parse single message: %w", err)
	}

	if baseMessage.ID != nil { // Response
		return c.handleResponse(data, baseMessage.ID)
	} else if baseMessage.Method != "" { // Notification or Request (server->client)
		// Check if it's a server-to-client request (specifically sampling or generic)
		if baseMessage.Method == protocol.MethodSamplingRequest {
			// Need to parse ID for requests, even if baseMessage.ID was nil initially
			var reqMessage struct {
				ID interface{} `json:"id"`
			}
			if err := json.Unmarshal(data, &reqMessage); err != nil || reqMessage.ID == nil {
				c.logger.Error("Failed to parse ID or ID is null for server request %s: %v", baseMessage.Method, err)
				return fmt.Errorf("invalid request format: missing or null id for method %s", baseMessage.Method)
			}
			return c.handleSamplingRequest(data, baseMessage.Method, reqMessage.ID)
		} else {
			c.handlerMu.RLock()
			_, isRegisteredGenericRequest := c.requestHandlers[baseMessage.Method]
			c.handlerMu.RUnlock()

			if isRegisteredGenericRequest {
				// Need to re-parse to get ID correctly for server->client requests
				var reqMessage struct {
					ID interface{} `json:"id"`
				}
				if err := json.Unmarshal(data, &reqMessage); err != nil || reqMessage.ID == nil {
					c.logger.Error("Failed to parse ID or ID is null for server request %s: %v", baseMessage.Method, err)
					return fmt.Errorf("invalid request format: missing or null id for method %s", baseMessage.Method)
				}
				return c.handleGenericRequest(data, baseMessage.Method, reqMessage.ID)
			} else {
				// Assume it's a notification
				return c.handleNotification(data, baseMessage.Method)
			}
		}
	} else {
		c.logger.Warn("Received single message with no ID or Method: %s", string(data))
		return fmt.Errorf("invalid single message format: missing id and method")
	}
}

// handleResponse handles an incoming response message.
func (c *Client) handleResponse(data []byte, id interface{}) error { // Added id parameter
	var response protocol.JSONRPCResponse
	if err := json.Unmarshal(data, &response); err != nil {
		c.logger.Error("Failed to unmarshal response: %v", err)
		// Cannot determine request ID if unmarshal fails here, maybe log raw data?
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Ensure the parsed response ID matches the ID from the base message
	// (This is a sanity check, they should always match if parsing is correct)
	if fmt.Sprintf("%v", response.ID) != fmt.Sprintf("%v", id) {
		c.logger.Error("Mismatched ID in response: base message ID '%v', parsed response ID '%v'", id, response.ID)
		return fmt.Errorf("mismatched response ID")
	}

	responsePtr := &response // Use pointer for modification

	// --- Execute ClientBeforeHandleResponse hooks ---
	c.hooksMu.RLock()
	bhResHooks := make([]hooks.ClientBeforeHandleResponseHook, len(c.beforeHandleResponseHooks))
	copy(bhResHooks, c.beforeHandleResponseHooks)
	c.hooksMu.RUnlock()

	if len(bhResHooks) > 0 {
		// Create context for hooks
		c.capabilitiesMu.RLock()
		hookCtx := hooks.ClientHookContext{
			Ctx:               c.processingCtx,                                               // Use processing context
			ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"}, // Use actual client info
			NegotiatedVersion: c.negotiatedVersion,
			ServerInfo:        c.serverInfo,
			ServerCaps:        c.serverCapabilities,
			MessageID:         id,
			// Method is not applicable/known for a response
		}
		c.capabilitiesMu.RUnlock()

		var hookErr error
		for _, hook := range bhResHooks {
			responsePtr, hookErr = hook(hookCtx, responsePtr)
			if hookErr != nil {
				c.logger.Error("Error executing ClientBeforeHandleResponseHook for response ID %v: %v. Dropping response.", id, hookErr)
				// Don't send to pending channel if hook fails
				return fmt.Errorf("client before handle response hook failed: %w", hookErr)
			}
			if responsePtr == nil { // Hook suppressed the response
				c.logger.Info("ClientBeforeHandleResponseHook suppressed response for ID %v.", id)
				// Don't send to pending channel if suppressed
				return nil // Suppressed, not an error
			}
		}
	}
	// --- End Hook Execution ---

	requestIDStr, ok := id.(string)
	if !ok {
		requestIDStr = fmt.Sprintf("%v", id)
	}

	c.pendingMu.Lock()
	respChan, exists := c.pendingRequests[requestIDStr]
	if exists {
		delete(c.pendingRequests, requestIDStr) // Remove from map once processed
	}
	c.pendingMu.Unlock()

	if exists {
		select {
		case respChan <- responsePtr: // Send potentially modified response
			c.logger.Debug("Sent response ID %s to pending channel.", requestIDStr)
		default:
			// This case should ideally not happen if channel buffer is 1 and not closed
			c.logger.Warn("Could not send response ID %s to pending channel (channel full or closed?).", requestIDStr)
		}
		// Close the channel after sending? No, let the receiver handle it.
	} else {
		c.logger.Warn("Received response for unknown or timed-out request ID: %s", requestIDStr)
	}
	return nil
}

// handleSamplingRequest handles the specific 'sampling/request' from the server.
func (c *Client) handleSamplingRequest(data []byte, method string, id interface{}) error {
	c.handlerMu.RLock()
	handler := c.samplingHandler
	c.handlerMu.RUnlock()

	if handler == nil {
		c.logger.Warn("Received sampling/request but no handler is registered.")
		// Send MethodNotFound error response
		errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeMethodNotFound, "Sampling handler not registered", nil)
		return c.sendResponse(c.processingCtx, errResp) // Use processingCtx
	}

	var request struct { // Define locally
		Params protocol.SamplingRequestParams `json:"params"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to parse sampling/request params: %v", err), nil)
		return c.sendResponse(c.processingCtx, errResp)
	}

	// --- Execute ClientBeforeHandleRequest hooks ---
	var hookErr error
	c.hooksMu.RLock()
	bhReqHooks := make([]hooks.ClientBeforeHandleRequestHook, len(c.clientBeforeHandleRequestHooks))
	copy(bhReqHooks, c.clientBeforeHandleRequestHooks)
	c.hooksMu.RUnlock()

	// Pass parsed params to hooks for sampling request
	parsedParams := request.Params // Use the specifically parsed struct

	if len(bhReqHooks) > 0 {
		c.capabilitiesMu.RLock()
		hookCtx := hooks.ClientHookContext{
			Ctx:               c.processingCtx,
			ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"},
			NegotiatedVersion: c.negotiatedVersion,
			ServerInfo:        c.serverInfo,
			ServerCaps:        c.serverCapabilities,
			MessageID:         id,
			Method:            method,
		}
		c.capabilitiesMu.RUnlock()

		for _, hook := range bhReqHooks {
			// Hook expects 'any', pass the specific parsed struct
			hookErr = hook(hookCtx, id, method, parsedParams)
			if hookErr != nil {
				c.logger.Error("Error executing ClientBeforeHandleRequestHook for method %s: %v. Aborting handler.", method, hookErr)
				errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeInternalError, fmt.Sprintf("Hook failed before handling sampling request: %v", hookErr), nil)
				return c.sendResponse(c.processingCtx, errResp)
			}
			// Note: Hooks cannot easily modify params here.
		}
	}
	// --- End Hook Execution ---

	// Execute handler in a goroutine to avoid blocking message processing loop
	// and allow handler to perform potentially long-running operations.
	go func() {
		result, err := handler(c.processingCtx, id, parsedParams) // Pass parsed params
		var resp *protocol.JSONRPCResponse
		if err != nil {
			c.logger.Error("Error executing sampling request handler: %v", err)
			// Determine appropriate JSON-RPC error code based on err type?
			resp = protocol.NewErrorResponse(id, protocol.ErrorCodeInternalError, fmt.Sprintf("Sampling handler failed: %v", err), nil)
		} else {
			resp = protocol.NewSuccessResponse(id, result)
		}
		// Send response back to server
		if sendErr := c.sendResponse(c.processingCtx, resp); sendErr != nil {
			c.logger.Error("Failed to send response for sampling request ID %v: %v", id, sendErr)
		}
	}()
	return nil
}

// handleGenericRequest handles incoming generic requests from the server (non-sampling).
func (c *Client) handleGenericRequest(data []byte, method string, id interface{}) error {
	c.handlerMu.RLock()
	handler, ok := c.requestHandlers[method]
	c.handlerMu.RUnlock()
	if !ok {
		// This case should technically not be reached due to the check in processSingleMessage
		c.logger.Error("Internal error: No generic handler found for method %s in handleGenericRequest", method)
		errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeMethodNotFound, fmt.Sprintf("No handler registered for method: %s", method), nil)
		return c.sendResponse(c.processingCtx, errResp)
	}

	var baseMessage struct { // Define locally
		Params json.RawMessage `json:"params"` // Keep params raw for hooks
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to parse params for server request %s: %v", method, err), nil)
		return c.sendResponse(c.processingCtx, errResp)
	}

	// --- Execute ClientBeforeHandleRequest hooks ---
	var hookErr error
	c.hooksMu.RLock()
	bhReqHooks := make([]hooks.ClientBeforeHandleRequestHook, len(c.clientBeforeHandleRequestHooks))
	copy(bhReqHooks, c.clientBeforeHandleRequestHooks)
	c.hooksMu.RUnlock()

	rawParams := baseMessage.Params

	if len(bhReqHooks) > 0 {
		c.capabilitiesMu.RLock()
		hookCtx := hooks.ClientHookContext{
			Ctx:               c.processingCtx,
			ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"},
			NegotiatedVersion: c.negotiatedVersion,
			ServerInfo:        c.serverInfo,
			ServerCaps:        c.serverCapabilities,
			MessageID:         id,
			Method:            method,
		}
		c.capabilitiesMu.RUnlock()

		for _, hook := range bhReqHooks {
			var parsedParams interface{}
			if err := json.Unmarshal(rawParams, &parsedParams); err != nil {
				c.logger.Error("Error parsing params before ClientBeforeHandleRequestHook for method %s: %v. Skipping hook.", method, err)
			} else {
				hookErr = hook(hookCtx, id, method, parsedParams) // Pass parsed params
				if hookErr != nil {
					c.logger.Error("Error executing ClientBeforeHandleRequestHook for method %s: %v. Aborting handler.", method, hookErr)
					errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeInternalError, fmt.Sprintf("Hook failed before handling request: %v", hookErr), nil)
					return c.sendResponse(c.processingCtx, errResp)
				}
				// Note: Hooks cannot easily modify params here.
			}
		}
	}
	// --- End Hook Execution ---

	// Parse params again for the actual handler
	var finalParsedParams interface{}
	if err := json.Unmarshal(rawParams, &finalParsedParams); err != nil {
		errResp := protocol.NewErrorResponse(id, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Failed to parse final params for server request %s: %v", method, err), nil)
		return c.sendResponse(c.processingCtx, errResp)
	}

	// Execute handler in a goroutine
	go func() {
		err := handler(c.processingCtx, id, finalParsedParams) // Pass final parsed params
		if err != nil {
			c.logger.Error("Error executing server request handler for method %s: %v", method, err)
			// TODO: Send error response back to server? Requires request ID.
			// For generic requests, the handler only returns error, not results.
			// We might need a different handler signature or convention if responses are needed.
			// For now, just logging the error.
		}
		// TODO: Send success response back to server? Protocol doesn't mandate this for generic requests.
	}()
	return nil
}

// handleNotification handles incoming notifications from the server.
func (c *Client) handleNotification(data []byte, method string) error {
	c.notificationMu.RLock()
	handler, ok := c.notificationHandlers[method]
	c.notificationMu.RUnlock()
	if !ok {
		c.logger.Info("No notification handler registered for method: %s", method)
		return nil // Ignore notifications without handlers
	}

	var baseMessage struct { // Define locally
		Params json.RawMessage `json:"params"` // Keep params raw for hooks
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse params for notification %s: %w", method, err)
	}

	// --- Execute ClientBeforeHandleNotification hooks ---
	var hookErr error
	c.hooksMu.RLock()
	bhNotifHooks := make([]hooks.ClientBeforeHandleNotificationHook, len(c.clientBeforeHandleNotificationHooks))
	copy(bhNotifHooks, c.clientBeforeHandleNotificationHooks)
	c.hooksMu.RUnlock()

	rawParams := baseMessage.Params

	if len(bhNotifHooks) > 0 {
		// Create context for hooks
		c.capabilitiesMu.RLock()
		hookCtx := hooks.ClientHookContext{
			Ctx:               c.processingCtx,                                               // Use processing context
			ClientInfo:        protocol.Implementation{Name: c.clientName, Version: "0.1.0"}, // Use actual client info
			NegotiatedVersion: c.negotiatedVersion,
			ServerInfo:        c.serverInfo,
			ServerCaps:        c.serverCapabilities,
			MessageID:         nil, // No ID for notifications
			Method:            method,
		}
		c.capabilitiesMu.RUnlock()

		for _, hook := range bhNotifHooks {
			// Similar issue as handleRequest: hook expects parsed params. Parse first.
			var parsedParams interface{}
			if err := json.Unmarshal(rawParams, &parsedParams); err != nil {
				c.logger.Error("Error parsing params before ClientBeforeHandleNotificationHook for method %s: %v. Skipping hook.", method, err)
			} else {
				hookErr = hook(hookCtx, method, parsedParams) // Pass parsed params
				if hookErr != nil {
					c.logger.Error("Error executing ClientBeforeHandleNotificationHook for method %s: %v. Aborting handler.", method, hookErr)
					return fmt.Errorf("client before handle notification hook failed: %w", hookErr)
				}
				// Note: Hooks cannot easily modify params here.
			}
		}
	}
	// --- End Hook Execution ---

	// Parse params again for the actual handler
	var finalParsedParams interface{}
	if err := json.Unmarshal(rawParams, &finalParsedParams); err != nil {
		return fmt.Errorf("failed to parse final params for notification %s: %w", method, err)
	}

	// Execute handler in a goroutine
	go func() {
		err := handler(c.processingCtx, finalParsedParams) // Pass final parsed params
		if err != nil {
			c.logger.Error("Error executing notification handler for method %s: %v", method, err)
		}
	}()
	return nil
}

// sendResponse sends a JSON-RPC response back to the server. Used for server-to-client requests.
func (c *Client) sendResponse(ctx context.Context, response *protocol.JSONRPCResponse) error {
	c.stateMu.RLock()
	isClosed := c.closed
	c.stateMu.RUnlock()
	if isClosed {
		return fmt.Errorf("client is closed, cannot send response")
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	c.logger.Debug("Sending response via transport: ID %v", response.ID)
	if err := c.transport.Send(ctx, respBytes); err != nil {
		return fmt.Errorf("failed to send response via transport: %w", err)
	}
	return nil
}

// closePendingRequests closes all pending request channels with a given error.
func (c *Client) closePendingRequests(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if len(c.pendingRequests) > 0 {
		c.logger.Info("Closing %d pending request channels due to: %v", len(c.pendingRequests), err)
		for id, ch := range c.pendingRequests {
			// Send an error response? Or just close? Closing is simpler.
			// If sending error: ch <- createErrorResponse(id, ...)
			close(ch) // Signal error/closure by closing the channel
			delete(c.pendingRequests, id)
		}
	}
}
