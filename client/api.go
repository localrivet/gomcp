package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// Implementation of the Client interface methods for clientImpl

// Connection state constants
const (
	connectionStateDisconnected = iota
	connectionStateConnecting
	connectionStateConnected
)

// Connection Management
func (c *clientImpl) Connect(ctx context.Context) error {
	// --- Check if already connected (Read Lock first, then Write Lock if needed) ---
	c.connectMu.RLock()
	alreadyConnected := c.connectionState == connectionStateConnected
	c.connectMu.RUnlock()
	if alreadyConnected {
		return NewConnectionError("client", "already connected", ErrAlreadyConnected)
	}

	// Set state to connecting first to prevent inconsistency detection
	c.connectMu.Lock()
	c.connectionState = connectionStateConnecting
	c.connectMu.Unlock()
	c.config.Logger.Debug("Connect: Setting client state to connecting")
	// -----------------------------------------------------------------------------

	c.config.Logger.Debug("Connect: Starting connection process")

	// Connect the transport (outside lock)
	if err := c.transport.Connect(ctx); err != nil {
		c.config.Logger.Error("Connect: Transport connection failed: %v", err)
		c.handleConnectionError(err)
		return err // Return early if transport fails to connect
	}
	c.config.Logger.Debug("Connect: Transport connected")

	// --- Send initialize request (outside lock) ---
	c.config.Logger.Debug("Connect: Sending initialize request")
	initializeParams := protocol.InitializeRequestParams{
		ProtocolVersion: c.config.PreferredProtocolVersion,
		ClientInfo: protocol.Implementation{
			Name:    "gomcp",
			Version: "0.1.0", // TODO: Get this from a version constant
		},
		Capabilities: protocol.ClientCapabilities{},
	}
	req, err := c.protocolHandler.FormatRequest("initialize", initializeParams)
	if err != nil {
		c.config.Logger.Error("Connect: Error formatting initialize request: %v", err)
		c.handleConnectionError(err)
		return err
	}

	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		c.config.Logger.Error("Connect: Error sending initialize request: %v", err)
		c.handleConnectionError(err)
		return err
	}
	c.config.Logger.Debug("Connect: Received initialize response")
	// -------------------------------------------

	// --- Parse initialize response (outside lock) ---
	c.config.Logger.Debug("Connect: Parsing initialize response")
	// (Raw response logging omitted for brevity)
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		c.config.Logger.Error("Connect: Error parsing initialize response: %v", err)
		c.handleConnectionError(err)
		return err
	}

	initResult, ok := result.(*protocol.InitializeResult)
	if !ok {
		msg := fmt.Sprintf("invalid initialize response type: %T", result)
		c.config.Logger.Error("Connect: %s", msg)
		err := NewClientError(msg, 0, nil)
		c.handleConnectionError(err)
		return err
	}

	// Store server info (outside lock - assuming reads elsewhere are safe or protected)
	c.serverInfo = initResult.ServerInfo
	c.serverCapabilities = initResult.Capabilities
	c.negotiatedVersion = initResult.ProtocolVersion
	c.config.Logger.Debug("Connect: Parsed initialize response, server info stored")
	// ------------------------------------------------

	// --- Send initialized notification (outside lock) ---
	c.config.Logger.Debug("Connect: Sending initialized notification")
	initNotiParams := protocol.InitializedNotificationParams{}
	initNotiReq, err := c.protocolHandler.FormatRequest("initialized", initNotiParams)
	if err != nil {
		c.config.Logger.Error("Connect: Error formatting initialized notification: %v", err)
		c.handleConnectionError(err)
		return err
	}
	initNotiReq.ID = nil // Convert to notification

	// Don't wait for the response to initialized notification - it's not expected
	// Just fire and forget since it's a notification
	go func() {
		if _, err := c.transport.SendRequest(ctx, initNotiReq); err != nil {
			c.config.Logger.Warn("Connect: Error sending initialized notification: %v", err)
			// Don't disconnect on notification send error
		}
		c.config.Logger.Debug("Connect: Sent initialized notification")
	}()

	// --- Set connected flag (Write Lock - minimal scope) ---
	c.connectMu.Lock()
	c.connectionState = connectionStateConnected
	c.connectMu.Unlock()
	c.config.Logger.Debug("Connect: Setting client state to connected")
	// -----------------------------------------------------

	// Notify handlers (outside lock)
	c.notifyConnectionStatusHandlers(true)

	c.config.Logger.Info("Client connected to %s (protocol version: %s)", c.serverInfo.Name, c.negotiatedVersion)
	c.config.Logger.Debug("Connect: Connection process completed successfully")
	return nil // Success
}

// handleConnectionError handles errors during connection process
func (c *clientImpl) handleConnectionError(err error) {
	c.config.Logger.Debug("Connection error (%v), closing transport", err)
	c.transport.Close()
	// Ensure consistent connection state
	c.connectMu.Lock()
	c.connectionState = connectionStateDisconnected
	c.connectMu.Unlock()
}

// Cleanup performs graceful cleanup of client resources
// Applications should call this before exiting to ensure proper resource cleanup
func (c *clientImpl) Cleanup() {
	c.config.Logger.Debug("Cleanup: Starting graceful cleanup")

	// Close the connection if connected, which will also clean up resources
	if c.IsConnected() {
		c.Close()
	} else {
		// Clean up resources even if we're not connected
		c.cleanupInternalResources()
	}

	c.config.Logger.Debug("Cleanup: Graceful cleanup completed")
}

func (c *clientImpl) Close() error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.connectionState == connectionStateDisconnected {
		return nil
	}

	c.config.Logger.Debug("Close: Closing client connection")

	// Close the transport first
	if err := c.transport.Close(); err != nil {
		c.config.Logger.Error("Close: Error closing transport: %v", err)
		return err
	}

	// Set connected to false AFTER closing the transport
	c.connectionState = connectionStateDisconnected
	c.config.Logger.Debug("Close: Client connection state set to disconnected")

	// Cleanup resources in a controlled manner
	c.cleanupInternalResources()

	// Notify any connection status handlers
	c.notifyConnectionStatusHandlers(false)

	c.config.Logger.Info("Client disconnected")
	return nil
}

// cleanupInternalResources performs cleanup of internal resources
// This is called by Close and Cleanup
func (c *clientImpl) cleanupInternalResources() {
	// Cancel any pending operations
	select {
	case <-c.done:
		// Already closed
		c.config.Logger.Debug("cleanupInternalResources: done channel already closed")
	default:
		c.config.Logger.Debug("cleanupInternalResources: closing done channel")
		close(c.done)
	}

	// Add any other resource cleanup here
}

func (c *clientImpl) IsConnected() bool {
	c.connectMu.RLock()
	clientConnected := c.connectionState == connectionStateConnected
	isConnecting := c.connectionState == connectionStateConnecting
	c.connectMu.RUnlock()

	// If we're in the process of connecting, don't check transport state yet
	if isConnecting {
		c.config.Logger.Debug("IsConnected: Client is in connecting state, returning false")
		return false
	}

	// Check transport connection state
	transportConnected := c.transport.IsConnected()

	// Log any inconsistency between client and transport states
	if clientConnected != transportConnected {
		c.config.Logger.Warn("Connection state inconsistency detected: client=%v, transport=%v",
			clientConnected, transportConnected)

		// If there's an inconsistency, update client state to match transport
		// since transport has the more accurate view of the actual connection
		if clientConnected && !transportConnected {
			c.connectMu.Lock()
			c.connectionState = connectionStateDisconnected
			c.connectMu.Unlock()

			// Also notify connection status handlers
			c.notifyConnectionStatusHandlers(false)

			c.config.Logger.Info("Client connection state updated to disconnected to match transport state")
			return false
		}
	}

	// Both must be true for us to be considered connected
	return clientConnected && transportConnected
}

func (c *clientImpl) Run(ctx context.Context) error {
	// First, connect if not connected
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return err
		}
	}

	// Create a context that we can cancel when done
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor connection status
	statusCh := make(chan bool, 1)
	reconnectCh := make(chan struct{}, 1)

	// Set up a goroutine to monitor the connection status
	go func() {
		defer close(statusCh)
		defer close(reconnectCh)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		wasConnected := c.IsConnected()

		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				// Check if connection status changed
				isConnected := c.IsConnected()
				if wasConnected && !isConnected {
					// Connection was lost
					statusCh <- false

					// Trigger reconnect
					if c.config.RetryStrategy != nil {
						reconnectCh <- struct{}{}
					}
				} else if !wasConnected && isConnected {
					// Connection was established
					statusCh <- true
				}
				wasConnected = isConnected
			}
		}
	}()

	// Set up reconnection logic if a retry strategy is configured
	if c.config.RetryStrategy != nil {
		go func() {
			var attemptCount int

			for {
				select {
				case <-runCtx.Done():
					return
				case <-reconnectCh:
					// Connection was lost, try to reconnect
					for attemptCount = 0; attemptCount < c.config.RetryStrategy.MaxAttempts(); attemptCount++ {
						// Calculate delay for this attempt
						delay := c.config.RetryStrategy.NextDelay(attemptCount + 1)

						// Wait for the delay or until context is cancelled
						select {
						case <-runCtx.Done():
							return
						case <-time.After(delay):
							// Try to reconnect
							c.config.Logger.Info("Attempting to reconnect (attempt %d/%d)", attemptCount+1, c.config.RetryStrategy.MaxAttempts())
							if err := c.Connect(runCtx); err != nil {
								c.config.Logger.Error("Reconnection attempt %d failed: %v", attemptCount+1, err)
							} else {
								// Successfully reconnected
								c.config.Logger.Info("Successfully reconnected to server")
								break
							}
						}
					}
				}
			}
		}()
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Close the connection
	return c.Close()
}

// notifyConnectionStatusHandlers notifies all registered handlers of a connection status change
func (c *clientImpl) notifyConnectionStatusHandlers(connected bool) {
	for _, handler := range c.connectionHandlers {
		if err := handler(connected); err != nil {
			c.config.Logger.Error("Connection status handler error: %v", err)
		}
	}
}

// handleNotification processes incoming notifications from the server
func (c *clientImpl) handleNotification(notification *protocol.JSONRPCNotification) error {
	method := notification.Method

	// Call method-specific handlers
	if handlers, ok := c.notificationHandlers[method]; ok {
		for _, handler := range handlers {
			if err := handler(notification); err != nil {
				c.config.Logger.Error("Notification handler error for method %s: %v", method, err)
			}
		}
	}

	// Special handling for progress notifications
	if method == "$/progress" {
		var params protocol.ProgressParams
		if err := protocol.UnmarshalPayload(notification.Params, &params); err != nil {
			return err
		}

		// Call the dedicated progress handler
		return c.handleProgress(params)
	}

	// Special handling for log notifications
	if method == "$/logMessage" {
		var params protocol.LoggingMessageParams
		if err := protocol.UnmarshalPayload(notification.Params, &params); err != nil {
			return err
		}

		// Call the dedicated log handler
		return c.handleLog(params.Level, params.Message)
	}

	// Special handling for resource update notifications
	if method == "$/resourceUpdated" {
		var params protocol.ResourceUpdatedParams
		if err := protocol.UnmarshalPayload(notification.Params, &params); err != nil {
			return err
		}

		// Call the dedicated resource update handler
		return c.handleResourceUpdate(params.Resource.URI)
	}

	return nil
}

// MCP Methods - High-level API
func (c *clientImpl) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	c.config.Logger.Debug("ListTools: Preparing to send request")

	// Create and format the request
	req, err := c.protocolHandler.FormatRequest("tools/list", nil)
	if err != nil {
		c.config.Logger.Error("ListTools: Error formatting request: %v", err)
		return nil, err
	}

	c.config.Logger.Debug("ListTools: Sending request with ID %v", req.ID)

	// Send the request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		c.config.Logger.Error("ListTools: Error sending request: %v", err)
		return nil, err
	}

	c.config.Logger.Debug("ListTools: Received response, parsing")

	// Parse the response
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		c.config.Logger.Error("ListTools: Error parsing response: %v", err)
		return nil, err
	}

	// Type assert and return the tools
	toolsResult, ok := result.(*protocol.ListToolsResult)
	if !ok {
		c.config.Logger.Error("ListTools: Invalid response type: %T", result)
		return nil, NewClientError("invalid tools/list response", 0, nil)
	}

	c.config.Logger.Debug("ListTools: Successfully retrieved %d tools", len(toolsResult.Tools))
	return toolsResult.Tools, nil
}

func (c *clientImpl) CallTool(ctx context.Context, name string, args map[string]interface{}, progressCh chan<- protocol.ProgressParams) ([]protocol.Content, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	// Create the params with proper tool call structure
	toolCall := &protocol.ToolCall{
		ID:       generateID(),
		ToolName: name,
	}

	// Marshal args to RawMessage if provided
	if args != nil {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, NewClientError("failed to marshal tool arguments", 0, err)
		}
		toolCall.Input = argsJSON
	}

	params := protocol.CallToolRequestParams{
		ToolCall: toolCall,
	}

	// Create progress token if needed
	progressID := ""
	if progressCh != nil {
		progressID = generateProgressID()
		params.Meta = &protocol.RequestMeta{
			ProgressToken: progressID,
		}

		// Register progress handler
		registeredHandler := c.registerProgressHandler(progressID, progressCh)
		defer registeredHandler()
	}

	// Create and format the request
	req, err := c.protocolHandler.FormatRequest(protocol.MethodCallTool, params)
	if err != nil {
		return nil, err
	}

	// Send the request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the response
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Handle different protocol versions - check for both 2024 and 2025 result types
	switch r := result.(type) {
	case *protocol.CallToolResult:
		// 2025-03-26 schema
		if r.Error != nil {
			return nil, NewServerError(protocol.MethodCallTool, name, int(r.Error.Code), r.Error.Message, nil)
		}

		// Convert raw output to content if needed
		var contentList []protocol.Content
		if err := json.Unmarshal(r.Output, &contentList); err != nil {
			// If not an array, try to unmarshal as a text content
			contentList = []protocol.Content{
				protocol.TextContent{
					Type: "text",
					Text: string(r.Output),
				},
			}
		}
		return contentList, nil

	case *protocol.CallToolResultV2024:
		// 2024-11-05 schema
		if r.IsError {
			return nil, NewServerError(protocol.MethodCallTool, name, 0, "Tool call failed", nil)
		}
		return r.Content, nil

	default:
		return nil, NewClientError("invalid "+protocol.MethodCallTool+" response type", 0, nil)
	}
}

// generateID creates a unique ID for requests
func generateID() string {
	return fmt.Sprintf("id-%d", time.Now().UnixNano())
}

// registerProgressHandler registers a temporary handler that forwards progress notifications to the provided channel
func (c *clientImpl) registerProgressHandler(progressID string, progressCh chan<- protocol.ProgressParams) func() {
	handler := func(progress *protocol.ProgressParams) error {
		// Only forward progress for the specific ID
		if progress.Token == progressID {
			select {
			case progressCh <- *progress:
				// Successfully sent
			default:
				// Channel buffer full or closed
				c.config.Logger.Warn("Failed to send progress update to channel")
			}
		}
		return nil
	}

	// Add the handler
	c.progressHandlers = append(c.progressHandlers, handler)

	// Return a function to remove the handler
	return func() {
		// Find and remove the handler
		for i, h := range c.progressHandlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.progressHandlers = append(c.progressHandlers[:i], c.progressHandlers[i+1:]...)
				break
			}
		}
	}
}

// generateProgressID creates a unique progress ID for tracking tool call progress
func generateProgressID() string {
	return fmt.Sprintf("progress-%d", time.Now().UnixNano())
}

func (c *clientImpl) ListResources(ctx context.Context) ([]protocol.Resource, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	// Create and format the request
	req, err := c.protocolHandler.FormatRequest(protocol.MethodListResources, nil)
	if err != nil {
		return nil, err
	}

	// Send the request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the response
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Type assert and return the resources
	resourcesResult, ok := result.(*protocol.ListResourcesResult)
	if !ok {
		return nil, NewClientError("invalid listResources response", 0, nil)
	}

	return resourcesResult.Resources, nil
}

func (c *clientImpl) ReadResource(ctx context.Context, uri string) ([]protocol.ResourceContents, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	// Create the params
	params := protocol.ReadResourceRequestParams{
		URI: uri,
	}

	// Create and format the request
	req, err := c.protocolHandler.FormatRequest(protocol.MethodReadResource, params)
	if err != nil {
		return nil, err
	}

	// Send the request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the response
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Type assert and return the contents
	readResult, ok := result.(*protocol.ReadResourceResult)
	if !ok {
		return nil, NewClientError("invalid "+protocol.MethodReadResource+" response", 0, nil)
	}

	return readResult.Contents, nil
}

func (c *clientImpl) ListPrompts(ctx context.Context) ([]protocol.Prompt, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	// Create and format the request
	req, err := c.protocolHandler.FormatRequest(protocol.MethodListPrompts, nil)
	if err != nil {
		return nil, err
	}

	// Send the request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the response
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Type assert and return the prompts
	promptsResult, ok := result.(*protocol.ListPromptsResult)
	if !ok {
		return nil, NewClientError("invalid "+protocol.MethodListPrompts+" response", 0, nil)
	}

	return promptsResult.Prompts, nil
}

func (c *clientImpl) GetPrompt(ctx context.Context, name string, args map[string]interface{}) ([]protocol.PromptMessage, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	// Create the params
	params := protocol.GetPromptRequestParams{
		URI:       name,
		Arguments: args,
	}

	// Create and format the request
	req, err := c.protocolHandler.FormatRequest(protocol.MethodGetPrompt, params)
	if err != nil {
		return nil, err
	}

	// Send the request
	resp, err := c.transport.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the response
	result, err := c.protocolHandler.ParseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Type assert and return the messages
	promptResult, ok := result.(*protocol.GetPromptResult)
	if !ok {
		return nil, NewClientError("invalid "+protocol.MethodGetPrompt+" response", 0, nil)
	}

	return promptResult.Messages, nil
}

// Server Information
func (c *clientImpl) ServerInfo() protocol.Implementation {
	return c.serverInfo
}

func (c *clientImpl) ServerCapabilities() protocol.ServerCapabilities {
	return c.serverCapabilities
}

// Raw Protocol Access
func (c *clientImpl) SendRequest(ctx context.Context, method string, params interface{}) (*protocol.JSONRPCResponse, error) {
	if !c.IsConnected() {
		return nil, NewConnectionError("client", "not connected", ErrNotConnected)
	}

	req, err := c.protocolHandler.FormatRequest(method, params)
	if err != nil {
		return nil, err
	}

	return c.transport.SendRequest(ctx, req)
}

// Configuration methods (fluent interface)
func (c *clientImpl) WithTimeout(timeout time.Duration) Client {
	c.config.DefaultTimeout = timeout
	return c
}

func (c *clientImpl) WithRetry(maxAttempts int, backoff BackoffStrategy) Client {
	c.config.RetryStrategy = backoff
	return c
}

func (c *clientImpl) WithMiddleware(middleware ClientMiddleware) Client {
	c.config.Middleware = append(c.config.Middleware, middleware)
	return c
}

func (c *clientImpl) WithAuth(auth AuthProvider) Client {
	c.config.AuthProvider = auth
	return c
}

func (c *clientImpl) WithLogger(logger logx.Logger) Client {
	c.config.Logger = logger
	return c
}

// newTestClient creates a client for testing that skips initialization
func newTestClient(config *ClientConfig, transport ClientTransport) (Client, error) {
	client := &clientImpl{
		config:                 *config,
		transport:              transport,
		serverInfo:             protocol.Implementation{Name: "Test Server", Version: "1.0.0"},
		serverCapabilities:     protocol.ServerCapabilities{},
		negotiatedVersion:      protocol.CurrentProtocolVersion,
		connectionState:        connectionStateDisconnected,
		notificationHandlers:   make(map[string][]NotificationHandler),
		progressHandlers:       []ProgressHandler{},
		resourceUpdateHandlers: make(map[string][]ResourceUpdateHandler),
		logHandlers:            []LogHandler{},
		connectionHandlers:     []ConnectionStatusHandler{},
		pendingRequests:        make(map[string]chan *protocol.JSONRPCResponse),
		done:                   make(chan struct{}),
	}

	// Set up notification handler on the transport
	transport.SetNotificationHandler(client.handleNotification)

	// Create protocol handler based on preferred version
	client.protocolHandler = newProtocolHandler(config.PreferredProtocolVersion, config.Logger)

	return client, nil
}
