// Package client provides the MCP client implementation.
package client

import (
	"bytes" // For HTTP POST body
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"       // Added import
	"log"      // For default logger
	"net/http" // For HTTP client
	"net/url"  // For joining URLs
	"strings"  // Added import
	"sync"     // Added import
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
	"github.com/r3labs/sse/v2" // SSE Client library
)

// Client represents an MCP client instance using the SSE+HTTP hybrid transport.
type Client struct {
	// Removed transport field
	clientName string
	logger     types.Logger

	// Server info and capabilities
	serverInfo         protocol.Implementation
	serverCapabilities protocol.ServerCapabilities
	clientCapabilities protocol.ClientCapabilities
	negotiatedVersion  string // Added: Store the negotiated protocol version
	capabilitiesMu     sync.RWMutex

	// Request handling
	pendingRequests map[string]chan *protocol.JSONRPCResponse
	pendingMu       sync.Mutex

	// Request handlers (for server-to-client requests - less common in hybrid model)
	requestHandlers map[string]RequestHandlerFunc
	handlerMu       sync.RWMutex

	// Notification handlers (for server-to-client notifications via SSE)
	notificationHandlers map[string]NotificationHandlerFunc
	notificationMu       sync.RWMutex

	// Client-side state
	roots   map[string]protocol.Root
	rootsMu sync.Mutex

	// HTTP + SSE specific fields
	httpClient       *http.Client
	serverBaseURL    string      // e.g., "http://localhost:8080"
	messageEndpoint  string      // e.g., "/message" (relative path)
	sseEndpoint      string      // e.g., "/sse" (relative path)
	sessionID        string      // Received from server on SSE connection
	sseClient        *sse.Client // SSE client connection handler
	sseMu            sync.Mutex  // Mutex for SSE client state/reconnection?
	preferredVersion string      // Added: Store the preferred version for initialization

	// Client state
	initialized bool
	connected   bool // Represents SSE connection established
	closed      bool
	stateMu     sync.RWMutex

	// Message processing (for SSE messages)
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
	HTTPClient               *http.Client // Allow providing a custom HTTP client
	ServerBaseURL            string       // Required: Base URL of the MCP server
	MessageEndpoint          string       // Optional: Defaults to "/message"
	SSEEndpoint              string       // Optional: Defaults to "/sse"
	PreferredProtocolVersion string       // Optional: Defaults to CurrentProtocolVersion
}

// NewClient creates a new Client for SSE+HTTP transport.
func NewClient(clientName string, opts ClientOptions) (*Client, error) {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	if opts.ServerBaseURL == "" {
		return nil, fmt.Errorf("ServerBaseURL is required in ClientOptions")
	}
	baseURL, err := url.Parse(opts.ServerBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid ServerBaseURL: %w", err)
	}

	messageEndpoint := opts.MessageEndpoint
	if messageEndpoint == "" {
		messageEndpoint = "/message"
	}
	if !strings.HasPrefix(messageEndpoint, "/") {
		messageEndpoint = "/" + messageEndpoint
	}

	sseEndpoint := opts.SSEEndpoint
	if sseEndpoint == "" {
		sseEndpoint = "/sse"
	}
	if !strings.HasPrefix(sseEndpoint, "/") {
		sseEndpoint = "/" + sseEndpoint
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Determine preferred version
	clientPreferredVersion := opts.PreferredProtocolVersion
	if clientPreferredVersion == "" {
		clientPreferredVersion = protocol.CurrentProtocolVersion
	}

	c := &Client{
		clientName:           clientName,
		preferredVersion:     clientPreferredVersion, // Store preferred version
		logger:               logger,
		clientCapabilities:   opts.ClientCapabilities,
		httpClient:           httpClient,
		serverBaseURL:        strings.TrimSuffix(baseURL.String(), "/"),
		messageEndpoint:      messageEndpoint,
		sseEndpoint:          sseEndpoint,
		pendingRequests:      make(map[string]chan *protocol.JSONRPCResponse),
		requestHandlers:      make(map[string]RequestHandlerFunc),
		notificationHandlers: make(map[string]NotificationHandlerFunc),
		roots:                make(map[string]protocol.Root),
		initialized:          false,
		connected:            false,
		closed:               false,
		processingCtx:        ctx,
		processingCancel:     cancel,
	}

	logger.Info("MCP Client '%s' created for server %s", clientName, c.serverBaseURL)
	return c, nil
}

// Connect establishes the SSE connection and performs the MCP handshake.
func (c *Client) Connect(ctx context.Context) error {
	c.stateMu.Lock()
	if c.connected {
		c.stateMu.Unlock()
		return fmt.Errorf("client is already connected")
	}
	if c.closed {
		c.stateMu.Unlock()
		return fmt.Errorf("client is closed")
	}
	c.stateMu.Unlock()

	c.logger.Info("Client '%s' starting connection & initialization...", c.clientName)

	// --- Step 1: Establish SSE Connection ---
	sseURL := c.serverBaseURL + c.sseEndpoint
	c.logger.Info("Connecting to SSE endpoint: %s", sseURL)
	c.sseClient = sse.NewClient(sseURL) // Create SSE client instance

	endpointCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Start listening in a goroutine
	c.processingWg.Add(1) // Add to wait group for graceful shutdown
	go func() {
		defer c.processingWg.Done() // Signal when done
		c.logger.Debug("Starting SSE event listener goroutine...")
		err := c.sseClient.SubscribeRawWithContext(c.processingCtx, func(msg *sse.Event) { // Use processingCtx
			eventName := string(msg.Event)
			eventData := string(msg.Data)
			c.logger.Debug("SSE Received: Event=%s, Data=%s", eventName, eventData)

			c.sseMu.Lock()
			if eventName == "endpoint" && c.sessionID == "" {
				fullMessageURL := eventData
				u, err := url.Parse(fullMessageURL)
				if err != nil {
					c.logger.Error("Failed to parse message endpoint URL from endpoint event: %v", err)
					select {
					case errCh <- fmt.Errorf("invalid endpoint event data: %w", err):
					default:
					}
				} else {
					c.sessionID = u.Query().Get("sessionId")
					if c.sessionID == "" {
						c.logger.Error("No sessionId found in endpoint event URL: %s", fullMessageURL)
						select {
						case errCh <- fmt.Errorf("missing sessionId in endpoint event"):
						default:
						}
					} else {
						c.logger.Info("Received session ID: %s", c.sessionID)
						select {
						case endpointCh <- c.sessionID:
						default:
						} // Signal success
					}
				}
			} else if eventName == "message" && c.sessionID != "" {
				go func(data []byte) { // Process concurrently
					if err := c.processMessage(data); err != nil {
						c.logger.Error("Error processing SSE message: %v", err)
					}
				}(append([]byte{}, msg.Data...)) // Pass data copy
			} else {
				c.logger.Warn("Received unexpected SSE event '%s' or message before session ID.", eventName)
			}
			c.sseMu.Unlock()
		})
		// SubscribeRaw returned
		c.logger.Warn("SSE SubscribeRaw finished: %v", err)
		c.sseMu.Lock()
		if c.sessionID == "" && err != nil {
			select {
			case errCh <- fmt.Errorf("sse connection failed: %w", err):
			default:
			}
		}
		c.sseMu.Unlock()
		c.Close() // Ensure cleanup on connection error
	}()

	// Wait for session ID or error/timeout
	select {
	case <-ctx.Done():
		c.logger.Error("Context canceled while waiting for SSE endpoint event.")
		c.Close()
		return ctx.Err()
	case err := <-errCh:
		c.logger.Error("Failed to establish SSE connection or get session ID: %v", err)
		c.Close()
		return err
	case sessionID := <-endpointCh:
		if sessionID == "" {
			c.Close()
			return fmt.Errorf("received empty session ID")
		}
	}

	// --- Step 2: Send Initialize Request via HTTP POST ---
	c.logger.Info("SSE connected (Session: %s). Sending InitializeRequest via HTTP POST...", c.sessionID)
	clientInfo := protocol.Implementation{Name: c.clientName, Version: "0.1.0"} // Using fixed version for now
	initParams := protocol.InitializeRequestParams{
		ProtocolVersion: c.preferredVersion, // Use stored preferred version
		Capabilities:    c.clientCapabilities,
		ClientInfo:      clientInfo,
	}
	initReq := protocol.JSONRPCRequest{JSONRPC: "2.0", ID: uuid.NewString(), Method: protocol.MethodInitialize, Params: initParams}

	initTimeout := 60 * time.Second
	response, err := c.sendRequestAndWait(ctx, initReq.Method, initReq.ID, initParams, initTimeout)
	if err != nil {
		c.Close()
		return fmt.Errorf("initialization handshake failed: %w", err)
	}

	// --- Step 3: Process Initialize Response (received via SSE) ---
	if response.Error != nil {
		c.Close()
		return fmt.Errorf("received error response during initialization: [%d] %s", response.Error.Code, response.Error.Message)
	}
	var initResult protocol.InitializeResult
	if err := protocol.UnmarshalPayload(response.Result, &initResult); err != nil {
		c.Close()
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Check if server responded with a version we support
	serverSelectedVersion := initResult.ProtocolVersion
	if serverSelectedVersion != protocol.CurrentProtocolVersion && serverSelectedVersion != protocol.OldProtocolVersion {
		c.Close()
		return fmt.Errorf("server selected unsupported protocol version: %s (client supports %s, %s)",
			serverSelectedVersion, protocol.CurrentProtocolVersion, protocol.OldProtocolVersion)
	}

	c.capabilitiesMu.Lock()
	c.serverInfo = initResult.ServerInfo
	c.serverCapabilities = initResult.Capabilities
	c.negotiatedVersion = serverSelectedVersion // Store the negotiated version
	c.capabilitiesMu.Unlock()
	c.logger.Info("Negotiated protocol version: %s", serverSelectedVersion)

	// --- Step 4: Send Initialized Notification via HTTP POST ---
	c.logger.Info("Sending InitializedNotification via HTTP POST...")
	initNotifParams := protocol.InitializedNotificationParams{}
	initNotif := protocol.JSONRPCNotification{JSONRPC: "2.0", Method: protocol.MethodInitialized, Params: initNotifParams}
	if err := c.sendNotificationViaPost(ctx, initNotif); err != nil {
		c.logger.Warn("Failed to send initialized notification: %v", err)
	}

	c.stateMu.Lock()
	c.initialized = true
	c.connected = true
	c.stateMu.Unlock()

	c.logger.Info("Initialization successful with server: %s", initResult.ServerInfo.Name)
	return nil
}

// --- Public Methods ---

func (c *Client) ServerInfo() protocol.Implementation {
	c.capabilitiesMu.RLock()
	defer c.capabilitiesMu.RUnlock()
	return c.serverInfo
}
func (c *Client) ServerCapabilities() protocol.ServerCapabilities {
	c.capabilitiesMu.RLock()
	defer c.capabilitiesMu.RUnlock()
	return c.serverCapabilities
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

// ListTools sends a 'tools/list' request via HTTP POST and waits for the response via SSE.
func (c *Client) ListTools(ctx context.Context, params protocol.ListToolsRequestParams) (*protocol.ListToolsResult, error) {
	timeout := 15 * time.Second
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

// CallTool sends a 'tools/call' request via HTTP POST and waits for the response via SSE.
func (c *Client) CallTool(ctx context.Context, params protocol.CallToolParams, progressToken *protocol.ProgressToken) (*protocol.CallToolResult, error) {
	timeout := 60 * time.Second
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
	// Standard unmarshalling should invoke CallToolResult's custom UnmarshalJSON if present
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
		return &callResult, fmt.Errorf("%s", errMsg) // Use format specifier
	}
	return &callResult, nil
}

// Close gracefully shuts down the client connection.
func (c *Client) Close() error {
	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		return nil
	}
	c.closed = true
	c.connected = false
	c.initialized = false
	c.stateMu.Unlock()

	c.logger.Info("Closing client...")
	c.processingCancel() // Signal background loop to stop

	// Close SSE connection if active
	c.sseMu.Lock()
	if c.sseClient != nil {
		// r3labs client doesn't have explicit Close, rely on context cancellation
		// Maybe we need to store the underlying http client/transport if used?
		c.logger.Debug("SSE client Close() called (relying on context cancel)")
		c.sseClient = nil
	}
	c.sseMu.Unlock()

	c.processingWg.Wait() // Wait for loop to finish

	c.closePendingRequests(fmt.Errorf("client closed"))

	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	c.logger.Info("Client closed.")
	return nil
}

// --- Internal Methods ---

// sendRequestAndWait sends request via HTTP POST and waits for response via SSE channel.
func (c *Client) sendRequestAndWait(ctx context.Context, method string, id interface{}, params interface{}, timeout time.Duration) (*protocol.JSONRPCResponse, error) {
	c.stateMu.RLock()
	closed := c.closed
	sessionID := c.sessionID
	c.stateMu.RUnlock()
	if closed {
		return nil, fmt.Errorf("client is closed")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("client is not connected (no session ID)")
	}

	requestIDStr := fmt.Sprintf("%v", id)
	responseCh := make(chan *protocol.JSONRPCResponse, 1)

	c.pendingMu.Lock()
	if c.closed {
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("client closed before request could be registered")
	}
	c.pendingRequests[requestIDStr] = responseCh
	c.pendingMu.Unlock()

	defer func() { c.pendingMu.Lock(); delete(c.pendingRequests, requestIDStr); c.pendingMu.Unlock() }()

	// Construct POST request
	messageURL := fmt.Sprintf("%s%s?sessionId=%s", c.serverBaseURL, c.messageEndpoint, sessionID)
	request := protocol.JSONRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	reqBodyBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	postReq, err := http.NewRequestWithContext(ctx, "POST", messageURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Sending POST request to %s: %s", messageURL, string(reqBodyBytes))
	httpResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("http request failed: status %d, body: %s", httpResp.StatusCode, string(bodyBytes))
	}

	// Wait for the response via SSE with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case response := <-responseCh:
		if response == nil {
			return nil, fmt.Errorf("connection closed or error occurred while waiting for response to %s (ID: %s)", method, requestIDStr)
		}
		return response, nil
	case <-timeoutCtx.Done():
		err := timeoutCtx.Err()
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("request timed out after %v waiting for response to %s (ID: %s)", timeout, method, requestIDStr)
		}
		return nil, fmt.Errorf("request canceled while waiting for response to %s (ID: %s): %w", method, requestIDStr, err)
	}
}

// sendNotificationViaPost sends a notification via HTTP POST.
func (c *Client) sendNotificationViaPost(ctx context.Context, notification protocol.JSONRPCNotification) error {
	c.stateMu.RLock()
	closed := c.closed
	sessionID := c.sessionID
	c.stateMu.RUnlock()
	if closed {
		return fmt.Errorf("client is closed")
	}
	if sessionID == "" {
		return fmt.Errorf("client is not connected (no session ID)")
	}

	messageURL := fmt.Sprintf("%s%s?sessionId=%s", c.serverBaseURL, c.messageEndpoint, sessionID)
	reqBodyBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	postReq, err := http.NewRequestWithContext(ctx, "POST", messageURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Sending POST notification to %s: %s", messageURL, string(reqBodyBytes))
	httpResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return fmt.Errorf("failed to send POST notification: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusAccepted && httpResp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("http notification request failed: status %d, body: %s", httpResp.StatusCode, string(bodyBytes))
	}
	return nil
}

// startMessageProcessing starts the background goroutine for SSE.
func (c *Client) startMessageProcessing() {
	// This is now handled within the Connect method's goroutine listening to c.sseClient.SubscribeRawWithContext
	c.logger.Info("SSE message processing will be handled by the SubscribeRaw callback.")
}

// processIncomingMessages is called by the SSE SubscribeRaw callback.
func (c *Client) processMessage(data []byte) error {
	c.logger.Debug("Processing received SSE data: %s", string(data))
	var baseMessage struct {
		ID     interface{}
		Method string `json:"method"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		c.logger.Error("Failed to parse base message structure from SSE: %v", err)
		return fmt.Errorf("failed to parse base message from SSE: %w", err)
	}

	if baseMessage.ID != nil { // Response
		return c.handleResponse(data)
	} else if baseMessage.Method != "" { // Notification or Request (server->client)
		c.handlerMu.RLock()
		_, isRegisteredRequest := c.requestHandlers[baseMessage.Method]
		c.handlerMu.RUnlock()

		if isRegisteredRequest {
			return c.handleRequest(data, baseMessage.Method, baseMessage.ID) // ID will be nil here
		} else {
			return c.handleNotification(data, baseMessage.Method)
		}
	} else {
		c.logger.Warn("Received SSE message with no ID or Method: %s", string(data))
		return fmt.Errorf("invalid message format from SSE: missing id and method")
	}
}

// handleResponse routes incoming responses (via SSE) to waiting callers.
func (c *Client) handleResponse(data []byte) error {
	var response protocol.JSONRPCResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse response from SSE: %w", err)
	}
	idStr := fmt.Sprintf("%v", response.ID)
	c.pendingMu.Lock()
	ch, ok := c.pendingRequests[idStr]
	if ok {
		delete(c.pendingRequests, idStr)
	}
	c.pendingMu.Unlock()
	if ok {
		select {
		case ch <- &response:
			c.logger.Debug("Routed response for ID %s", idStr)
		default:
			c.logger.Warn("Response channel for request ID %s closed or full (likely timeout).", idStr)
		}
	} else {
		c.logger.Warn("Received response via SSE for unknown or timed-out request ID: %v", response.ID)
	}
	return nil
}

// handleRequest handles incoming requests from the server (via SSE - less common).
func (c *Client) handleRequest(data []byte, method string, id interface{}) error {
	c.handlerMu.RLock()
	handler, ok := c.requestHandlers[method]
	c.handlerMu.RUnlock()
	if !ok {
		c.logger.Warn("No request handler registered for server-sent method: %s", method)
		return fmt.Errorf("no handler for server request method: %s", method)
	}
	var baseMessage struct {
		Params interface{} `json:"params"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse params for server request %s: %w", method, err)
	}
	go func() {
		err := handler(c.processingCtx, id, baseMessage.Params) // ID will be nil if server sent request as notification
		if err != nil {
			c.logger.Error("Error executing server request handler for method %s: %v", method, err)
		}
	}()
	return nil
}

// handleNotification handles incoming notifications from the server (via SSE).
func (c *Client) handleNotification(data []byte, method string) error {
	c.notificationMu.RLock()
	handler, ok := c.notificationHandlers[method]
	c.notificationMu.RUnlock()
	if !ok {
		c.logger.Info("No notification handler registered for method: %s", method)
		return nil
	}
	var baseMessage struct {
		Params interface{} `json:"params"`
	}
	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse notification params %s: %w", method, err)
	}
	go func() {
		err := handler(c.processingCtx, baseMessage.Params)
		if err != nil {
			c.logger.Error("Error executing notification handler for method %s: %v", method, err)
		}
	}()
	return nil
}

// closePendingRequests closes all pending request channels.
func (c *Client) closePendingRequests(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if len(c.pendingRequests) > 0 {
		c.logger.Warn("Closing %d pending request channels due to error: %v", len(c.pendingRequests), err)
		for id, ch := range c.pendingRequests {
			close(ch)
			delete(c.pendingRequests, id)
		}
	}
}

// --- Default Logger ---
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

// --- TODO ---
// - AddRoot, RemoveRoot (need POST endpoints)
// - SendCancellation (needs POST endpoint)
// - Ping (needs POST endpoint)
// - Robust error handling for SSE connection/disconnection
// - Reconnection logic for SSE client?
// - Custom headers for Dial/POST (e.g., Auth)
// - Refine Content unmarshalling in CallTool if needed
