// Package client provides the MCP client implementation.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io" // For EOF check in receive loop
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/logx" // Import the new logger package
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

	// Notification handlers (for server-to-client notifications)
	notificationHandlers map[string]NotificationHandlerFunc
	notificationMu       sync.RWMutex

	// Client-side state
	roots   map[string]protocol.Root
	rootsMu sync.Mutex

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
}

// RequestHandlerFunc defines the signature for functions that handle server-to-client requests.
type RequestHandlerFunc func(ctx context.Context, id interface{}, params interface{}) error

// NotificationHandlerFunc defines the signature for functions that handle server-to-client notifications.
type NotificationHandlerFunc func(ctx context.Context, params interface{}) error

// ClientOptions contains configuration options for creating a Client.
type ClientOptions struct {
	Logger                   types.Logger
	ClientCapabilities       protocol.ClientCapabilities
	PreferredProtocolVersion *string         // Optional: Defaults to OldProtocolVersion
	Transport                types.Transport // Required: Must be provided by specific constructors
	// Removed HTTP/SSE specific options
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

	c := &Client{
		clientName:           clientName,
		transport:            opts.Transport, // Store the provided transport
		preferredVersion:     clientPreferredVersion,
		logger:               logger,
		clientCapabilities:   opts.ClientCapabilities,
		pendingRequests:      make(map[string]chan *protocol.JSONRPCResponse),
		requestHandlers:      make(map[string]RequestHandlerFunc),
		notificationHandlers: make(map[string]NotificationHandlerFunc),
		roots:                make(map[string]protocol.Root),
		initialized:          false,
		closed:               false,
		processingCtx:        ctx,
		processingCancel:     cancel,
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
func (c *Client) RegisterRequestHandler(method string, handler RequestHandlerFunc) error {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	if _, exists := c.requestHandlers[method]; exists {
		return fmt.Errorf("request handler already registered: %s", method)
	}
	c.requestHandlers[method] = handler
	c.logger.Info("Registered request handler: %s", method)
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
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request via the transport
	c.logger.Debug("Sending request via transport: %s %s", method, requestIDStr)
	// Use the context passed to sendRequestAndWait for the Send operation
	if err := c.transport.Send(ctx, requestBytes); err != nil {
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
	if !isInitialized {
		// Allow initialized notification even if state isn't fully set yet
		if notification.Method != protocol.MethodInitialized {
			return fmt.Errorf("client is not initialized")
		}
	}

	// Prepare notification payload
	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Send notification via the transport
	c.logger.Debug("Sending notification via transport: %s", notification.Method)
	// Use the context passed to sendNotification for the Send operation
	if err := c.transport.Send(ctx, notificationBytes); err != nil {
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

				// Process the received message in a separate goroutine
				// to avoid blocking the receive loop.
				// Add to the WaitGroup before starting the goroutine.
				c.processingWg.Add(1)
				go func(msgData []byte) {
					// Ensure Done is called when the goroutine finishes.
					defer c.processingWg.Done()
					if err := c.processMessage(msgData); err != nil {
						c.logger.Error("Error processing received message: %v", err)
					}
				}(data)
			}
		}
	}()
}

// processMessage handles a single raw message received from the transport.
func (c *Client) processMessage(data []byte) error {
	c.logger.Debug("Processing received message: %s", string(data))
	var baseMessage struct {
		ID     interface{} `json:"id"` // Ensure ID is captured
		Method string      `json:"method"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		c.logger.Error("Failed to parse base message structure: %v", err)
		return fmt.Errorf("failed to parse base message: %w", err)
	}

	if baseMessage.ID != nil { // Response
		return c.handleResponse(data)
	} else if baseMessage.Method != "" { // Notification or Request (server->client)
		// Check if it's a server-to-client request
		c.handlerMu.RLock()
		_, isRegisteredRequest := c.requestHandlers[baseMessage.Method]
		c.handlerMu.RUnlock()

		if isRegisteredRequest {
			// Need to parse the ID properly for server->client requests
			var reqMessage struct {
				ID interface{} `json:"id"` // Re-parse to get ID correctly
			}
			if err := json.Unmarshal(data, &reqMessage); err != nil {
				c.logger.Error("Failed to parse ID for server request %s: %v", baseMessage.Method, err)
				return fmt.Errorf("failed to parse ID for server request %s: %w", baseMessage.Method, err)
			}
			return c.handleRequest(data, baseMessage.Method, reqMessage.ID)
		} else {
			// Assume it's a notification
			return c.handleNotification(data, baseMessage.Method)
		}
	} else {
		c.logger.Warn("Received message with no ID or Method: %s", string(data))
		return fmt.Errorf("invalid message format: missing id and method")
	}
}

// handleResponse routes incoming responses to waiting callers.
func (c *Client) handleResponse(data []byte) error {
	var response protocol.JSONRPCResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	idStr := fmt.Sprintf("%v", response.ID)
	c.pendingMu.Lock()
	ch, ok := c.pendingRequests[idStr]
	if ok {
		// Found the channel, remove it from the map *before* unlocking
		delete(c.pendingRequests, idStr)
	}
	c.pendingMu.Unlock() // Unlock *after* potential deletion

	if ok {
		// Channel was found and removed from map
		select {
		case ch <- &response:
			c.logger.Debug("Routed response for ID %s", idStr)
		default:
			// It might indicate a logic error or unexpected server behavior.
			// Or the channel might have been closed by closePendingRequests concurrently.
			c.logger.Warn("Response channel for request ID %s closed or full (unexpected).", idStr)
		}
	} else {
		c.logger.Warn("Received response for unknown or timed-out request ID: %v", response.ID)
	}
	return nil
}

// handleRequest handles incoming requests from the server.
func (c *Client) handleRequest(data []byte, method string, id interface{}) error {
	c.handlerMu.RLock()
	handler, ok := c.requestHandlers[method]
	c.handlerMu.RUnlock()
	if !ok {
		c.logger.Warn("No request handler registered for server-sent method: %s", method)
		// TODO: Should we send an error response back to the server?
		// MethodNotFound error? Requires sending a response.
		return fmt.Errorf("no handler for server request method: %s", method)
	}
	var baseMessage struct {
		Params interface{} `json:"params"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse params for server request %s: %w", method, err)
	}
	// Execute handler in a goroutine to avoid blocking message processing loop
	go func() {
		err := handler(c.processingCtx, id, baseMessage.Params)
		if err != nil {
			c.logger.Error("Error executing server request handler for method %s: %v", method, err)
			// TODO: Send error response back to server? Requires request ID.
		}
		// TODO: Send success response back to server? Requires request ID.
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
	var baseMessage struct {
		Params interface{} `json:"params"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse params for notification %s: %w", method, err)
	}
	// Execute handler in a goroutine
	go func() {
		err := handler(c.processingCtx, baseMessage.Params)
		if err != nil {
			c.logger.Error("Error executing notification handler for method %s: %v", method, err)
		}
	}()
	return nil
}

// closePendingRequests closes all pending request channels with an error.
func (c *Client) closePendingRequests(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if len(c.pendingRequests) > 0 {
		c.logger.Warn("Closing %d pending requests due to client closure.", len(c.pendingRequests))
		// Create a copy of the map keys to iterate over, as we modify the map
		idsToClose := make([]string, 0, len(c.pendingRequests))
		for id := range c.pendingRequests {
			idsToClose = append(idsToClose, id)
		}

		for _, id := range idsToClose {
			if ch, exists := c.pendingRequests[id]; exists {
				// Send error response non-blockingly in case channel is full/closed
				select {
				case ch <- &protocol.JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &protocol.ErrorPayload{Code: protocol.ErrorCodeInternalError, Message: fmt.Sprintf("Client closed: %v", err)}}:
				default:
				}
				close(ch)                     // Close channel *after* attempting send
				delete(c.pendingRequests, id) // Delete entry
			}
		}
	}
}

// Default logger definition removed, now in logx package
