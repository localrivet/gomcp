package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// sseTransport implements ClientTransport using Server-Sent Events
type sseTransport struct {
	baseURL       string
	basePath      string
	options       *TransportOptions
	logger        logx.Logger
	client        *http.Client
	notifyHandler NotificationHandler

	connected   bool
	connMutex   sync.RWMutex
	eventSource *eventSource
	lastEventID string
	sessionID   string     // Session ID received from the server
	sessionMu   sync.Mutex // Mutex for sessionID access

	responseChannels sync.Map // map[string]chan *protocol.JSONRPCResponse

	// Context for managing the event stream goroutine
	ctx    context.Context
	cancel context.CancelFunc

	endpointURL string
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(baseURL, basePath string, logger logx.Logger, options ...TransportOption) (ClientTransport, error) {
	// Parse base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, NewTransportError("sse", "invalid base URL", err)
	}

	// Ensure basePath has a leading slash but no trailing slash
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimSuffix(basePath, "/")

	// Apply transport options
	opts := DefaultTransportOptions()
	for _, option := range options {
		option(opts)
	}

	// Create the transport
	t := &sseTransport{
		baseURL:     parsedURL.String(),
		basePath:    basePath,
		options:     opts,
		logger:      logger,
		client:      opts.HTTPClient,
		connected:   false,
		endpointURL: "", // Initialize to empty string
	}

	// Create root context that will be used to manage the event stream
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t, nil
}

// Connect establishes the SSE connection
func (t *sseTransport) Connect(ctx context.Context) error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.connected {
		return NewConnectionError(t.baseURL, "already connected", ErrAlreadyConnected)
	}

	eventsURL := fmt.Sprintf("%s%s/sse", t.baseURL, t.basePath)

	// Create a request for the SSE endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return NewTransportError("sse", "failed to create SSE request", err)
	}

	// Add headers
	req.Header.Add("Accept", "text/event-stream")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Connection", "keep-alive")

	// Add last event ID if we have one from a previous connection
	if t.lastEventID != "" {
		req.Header.Add("Last-Event-ID", t.lastEventID)
	}

	// Add custom headers from the options if provided
	if t.options != nil && t.options.Headers != nil {
		for k, values := range t.options.Headers {
			for _, v := range values {
				req.Header.Add(k, v)
			}
		}
	}

	// Add the protocol version header
	version := LatestProtocolVersion
	req.Header.Add("X-MCP-Protocol-Version", version)

	// Create a context with cancel for the event stream
	t.ctx, t.cancel = context.WithCancel(context.Background())

	// Send the request
	resp, err := t.client.Do(req)
	if err != nil {
		return NewTransportError("sse", "failed to connect to SSE endpoint", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return NewTransportError("sse", fmt.Sprintf("unexpected status code: %d", resp.StatusCode), nil)
	}

	// Extract and store the session ID from response headers
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		t.sessionMu.Lock()
		t.sessionID = sessionID
		t.sessionMu.Unlock()
		t.logger.Info("Received session ID: %s", sessionID)
	} else {
		t.logger.Warn("No session ID received from server")
	}

	// Create and start the event source
	t.eventSource = newEventSource(resp.Body)
	go t.eventSource.start()

	// Start the event handler
	go t.handleEvents()

	// Mark as connected
	t.connected = true
	t.logger.Info("SSE transport connected to %s", eventsURL)

	return nil
}

// IsConnected returns true if the transport is connected
func (t *sseTransport) IsConnected() bool {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()

	// First check our connected flag
	if !t.connected {
		return false
	}

	// Then verify the event source is valid
	if t.eventSource == nil {
		t.logger.Debug("IsConnected: eventSource is nil despite connected=true")
		return false
	}

	// Check if context is still valid
	if t.ctx.Err() != nil {
		t.logger.Debug("IsConnected: context is cancelled despite connected=true: %v", t.ctx.Err())
		return false
	}

	// Perform additional checks if needed in the future

	// All checks passed
	return true
}

// Close closes the SSE connection
func (t *sseTransport) Close() error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if !t.connected {
		return nil
	}

	t.logger.Debug("SSE transport Close: Calling t.cancel() to stop event handling goroutines")
	// Cancel the context to stop all goroutines
	t.cancel()

	// Close the event source if it exists
	if t.eventSource != nil {
		// Use a separate goroutine to close the event source to avoid deadlocks
		// This is because eventSource.close() might be waiting on the Events channel
		// which could be blocked if handleEvents isn't running due to a panic
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.logger.Error("Panic while closing event source: %v", r)
				}
			}()
			t.logger.Debug("SSE transport Close: Closing event source")
			t.eventSource.close()
		}()
	}

	t.logger.Debug("SSE transport Close: Setting connected=false")
	t.connected = false
	t.logger.Info("SSE transport disconnected from %s", t.baseURL)

	return nil
}

// SendRequest sends a request to the server and waits for a response
func (t *sseTransport) SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.IsConnected() {
		return nil, NewConnectionError(t.baseURL, "not connected", ErrNotConnected)
	}

	// Create a response channel
	responseCh := make(chan *protocol.JSONRPCResponse, 1)
	defer close(responseCh)

	// Register the response channel for this request ID
	reqID := fmt.Sprintf("%v", req.ID)
	t.responseChannels.Store(reqID, responseCh)
	defer t.responseChannels.Delete(reqID)

	// Send the request via HTTP POST
	if err := t.sendHTTPRequest(ctx, req); err != nil {
		return nil, err
	}

	// Wait for response or timeout
	select {
	case resp := <-responseCh:
		return resp, nil
	case <-ctx.Done():
		return nil, NewTimeoutError("SendRequest", t.options.RequestTimeout, ctx.Err())
	}
}

// SendRequestAsync sends a request to the server without waiting for a response
func (t *sseTransport) SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error {
	if !t.IsConnected() {
		return NewConnectionError(t.baseURL, "not connected", ErrNotConnected)
	}

	// Register the response channel for this request ID
	reqID := fmt.Sprintf("%v", req.ID)
	t.responseChannels.Store(reqID, responseCh)

	// Send the request via HTTP POST
	return t.sendHTTPRequest(ctx, req)
}

// sendHTTPRequest sends a JSON-RPC request via HTTP POST
func (t *sseTransport) sendHTTPRequest(ctx context.Context, req *protocol.JSONRPCRequest) error {
	// Ensure we're connected
	if !t.IsConnected() {
		return ErrNotConnected
	}

	// Marshal the request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return NewTransportError("sse", "failed to marshal request", err)
	}

	// Determine the URL to use
	var url string
	if t.endpointURL != "" {
		// Use the endpoint URL received in the "endpoint" event (2024-11-05 protocol)
		url = t.endpointURL

		// If it's a relative URL, join it with the baseURL
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = fmt.Sprintf("%s%s", t.baseURL, url)
		}
	} else {
		// Default is to use the configured baseURL and basePath
		url = fmt.Sprintf("%s%s", t.baseURL, t.basePath)
	}

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return NewTransportError("sse", "failed to create HTTP request", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Set the last event ID if we have one
	if t.lastEventID != "" {
		httpReq.Header.Set("Last-Event-ID", t.lastEventID)
	}

	// Set the session ID if we have one
	t.sessionMu.Lock()
	sessionID := t.sessionID
	t.sessionMu.Unlock()

	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
		t.logger.Debug("Including session ID in request: %s", sessionID)
	} else {
		t.logger.Warn("No session ID available for request")
	}

	// Apply authentication if provided
	if t.options.AuthProvider != nil {
		for key, value := range t.options.AuthProvider.GetAuthHeaders() {
			httpReq.Header.Add(key, value)
		}
	}

	// Send the request
	t.logger.Debug("Sending request to %s: %s", url, string(reqBody))
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return NewTransportError("sse", "failed to send HTTP request", err)
	}
	defer resp.Body.Close()

	// For synchronous/immediate responses, we read the body and process it.
	// For async responses, we'll get them via SSE events.
	// Here we only check if the request was accepted.
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return NewTransportError("sse", fmt.Sprintf("HTTP request failed: %s - %s", resp.Status, string(bodyBytes)), nil)
	}

	// If this is an HTTP 202 (Accepted), the server will process the request asynchronously
	// and send the response via SSE later. No need to parse the response body.
	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	// For immediate responses (HTTP 200), we need to read and process the response body
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return NewTransportError("sse", "failed to read HTTP response body", err)
		}

		// Parse the response
		var jsonRpcResp protocol.JSONRPCResponse
		if err := json.Unmarshal(bodyBytes, &jsonRpcResp); err != nil {
			return NewTransportError("sse", "failed to unmarshal response", err)
		}

		// Handle the response by sending it to the corresponding channel
		if ch, ok := t.getResponseChannel(fmt.Sprintf("%v", jsonRpcResp.ID)); ok {
			select {
			case ch <- &jsonRpcResp:
				// Successfully sent
			default:
				// Channel buffer full or closed
				t.logger.Warn("Failed to send response to channel: %v", jsonRpcResp.ID)
			}
		}
	}

	return nil
}

// getResponseChannel returns the response channel for a request ID
func (t *sseTransport) getResponseChannel(reqID string) (chan *protocol.JSONRPCResponse, bool) {
	if value, ok := t.responseChannels.Load(reqID); ok {
		if ch, ok := value.(chan *protocol.JSONRPCResponse); ok {
			return ch, true
		}
	}
	return nil, false
}

// SetNotificationHandler sets the handler for incoming server notifications
func (t *sseTransport) SetNotificationHandler(handler NotificationHandler) {
	t.notifyHandler = handler
}

// GetTransportType returns the transport type
func (t *sseTransport) GetTransportType() TransportType {
	return TransportTypeSSE
}

// GetTransportInfo returns transport-specific information
func (t *sseTransport) GetTransportInfo() map[string]interface{} {
	return map[string]interface{}{
		"baseURL":     t.baseURL,
		"basePath":    t.basePath,
		"connected":   t.IsConnected(),
		"lastEventID": t.lastEventID,
	}
}

// handleEvents processes SSE events from the event source
func (t *sseTransport) handleEvents() {
	for {
		select {
		case <-t.ctx.Done():
			// Context cancelled, stop handling events
			t.logger.Debug("SSE handleEvents stopping: context cancelled")
			return
		case event, ok := <-t.eventSource.Events:
			if !ok {
				// Channel closed, likely due to server disconnect
				t.logger.Warn("SSE event channel closed unexpectedly, but not automatically marking as disconnected")

				// We'll no longer automatically disconnect here as this could be a temporary condition
				// or part of normal reconnection behavior. Instead, we'll just exit the handler.
				// The client will detect disconnection through other means (via periodic checks or explicit errors)

				// Instead, log more information to help troubleshoot
				if t.ctx.Err() != nil {
					t.logger.Debug("Context was already cancelled: %v", t.ctx.Err())
				}

				return
			}
			if event != nil {
				t.handleEvent(event)
			}
		}
	}
}

// handleEvent processes a single SSE event
func (t *sseTransport) handleEvent(event *sseEvent) {
	// Safety check to avoid nil pointer dereference
	if event == nil {
		t.logger.Error("Received nil event in handleEvent")
		return
	}

	// Update last event ID
	if event.ID != "" {
		t.lastEventID = event.ID
	}

	// Handle different event types
	switch event.Event {
	case "message":
		t.handleMessageEvent(event.Data)
	case "notification":
		t.handleNotificationEvent(event.Data)
	case "error":
		t.handleErrorEvent(event.Data)
		// We'll be more cautious about EOF errors, log more information but don't disconnect immediately
		if strings.Contains(event.Data, "EOF") {
			t.logger.Warn("Received EOF error event, but not automatically marking as disconnected: %s", event.Data)

			// We won't automatically disconnect on EOF anymore since this could be a temporary condition
			// The client-side periodic health checks will detect persistent disconnection
		}
	case "ping":
		// Just a keepalive, nothing to do
	case "endpoint":
		// Handle the endpoint event for 2024-11-05 protocol
		// This event sends the URL for client->server communication
		t.logger.Debug("Received endpoint URL from server: %s", event.Data)

		// Store the endpoint URL from the server
		// This URL is used for sending requests in the 2024-11-05 protocol
		t.endpointURL = event.Data
		t.logger.Info("Using endpoint URL from server: %s", t.endpointURL)

		// Extract session ID from the endpoint URL if present as a query parameter
		if endpointURL, err := url.Parse(event.Data); err == nil {
			if sessionID := endpointURL.Query().Get("sessionId"); sessionID != "" {
				t.sessionMu.Lock()
				t.sessionID = sessionID
				t.sessionMu.Unlock()
				t.logger.Info("Extracted session ID from endpoint URL: %s", sessionID)
			}
		}
	default:
		t.logger.Warn("Unknown SSE event type: %s", event.Event)
	}
}

// handleMessageEvent processes a message event (response to a request)
func (t *sseTransport) handleMessageEvent(data string) {
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.logger.Error("Failed to parse message event: %v", err)
		return
	}

	// Find the response channel for this request ID
	reqID := fmt.Sprintf("%v", resp.ID)
	if ch, ok := t.getResponseChannel(reqID); ok {
		select {
		case ch <- &resp:
			// Successfully sent response to channel
		default:
			// Channel buffer full or closed
			t.logger.Warn("Failed to send response to channel: %v", resp.ID)
		}

		// For channels we don't own (sent in by the client), don't remove them
		// Only remove channels we created internally
		if _, ok := t.responseChannels.Load(reqID); ok {
			t.responseChannels.Delete(reqID)
		}
	} else {
		t.logger.Warn("Received response for unknown request ID: %v", resp.ID)
	}
}

// handleNotificationEvent processes a notification event
func (t *sseTransport) handleNotificationEvent(data string) {
	var notification protocol.JSONRPCNotification
	if err := json.Unmarshal([]byte(data), &notification); err != nil {
		t.logger.Error("Failed to parse notification event: %v", err)
		return
	}

	// Call the notification handler if set
	if t.notifyHandler != nil {
		if err := t.notifyHandler(&notification); err != nil {
			t.logger.Error("Notification handler returned error: %v", err)
		}
	}
}

// handleErrorEvent processes an error event
func (t *sseTransport) handleErrorEvent(data string) {
	t.logger.Error("Server sent error event: %s", data)

	// We don't automatically disconnect on error, that's up to the client
}

// --- SSE Event Source Implementation ---

// sseEvent represents a single Server-Sent Event
type sseEvent struct {
	ID    string
	Event string
	Data  string
}

// eventSource reads and parses Server-Sent Events from a reader
type eventSource struct {
	reader      io.ReadCloser
	Events      chan *sseEvent
	done        chan struct{}
	lastEventID string
}

// newEventSource creates a new event source
func newEventSource(reader io.ReadCloser) *eventSource {
	return &eventSource{
		reader: reader,
		Events: make(chan *sseEvent, 100), // Buffer up to 100 events
		done:   make(chan struct{}),
	}
}

// start reads events from the reader
func (es *eventSource) start() {
	defer func() {
		// Safely close the events channel when done
		close(es.Events)

		// Handle any panics that might occur during reading
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Panic in eventSource.start: %v\n", r)
		}
	}()

	// Create a buffer for accumulating event data
	var eventID, eventType, eventData strings.Builder

	// Read the stream line by line
	buf := make([]byte, 4096)
	for {
		select {
		case <-es.done:
			return
		default:
			// Continue reading
		}

		n, err := es.reader.Read(buf)
		if err != nil {
			// Handle EOF or other errors
			errMsg := "Read error: unexpected EOF"
			if err != io.EOF {
				errMsg = fmt.Sprintf("Read error: %v", err)
			}

			// Send an error event and return
			select {
			case es.Events <- &sseEvent{
				Event: "error",
				Data:  errMsg,
			}:
				// Successfully sent the error event
			case <-es.done:
				// We're shutting down, just return
			default:
				// Channel might be full or closed, just log and return
				fmt.Fprintf(os.Stderr, "Failed to send error event: %s\n", errMsg)
			}
			return
		}

		// Process the data we read
		if n == 0 {
			// No data read, possibly due to connection issues
			continue
		}

		data := string(buf[:n])
		lines := strings.Split(data, "\n")

		for _, line := range lines {
			// Empty line signifies end of event
			if line == "" {
				// Send the event if we have data
				if eventData.Len() > 0 {
					es.Events <- &sseEvent{
						ID:    strings.TrimSpace(eventID.String()),
						Event: strings.TrimSpace(eventType.String()),
						Data:  strings.TrimSpace(eventData.String()),
					}

					// Save the last event ID
					if eventID.Len() > 0 {
						es.lastEventID = strings.TrimSpace(eventID.String())
					}

					// Reset the builders for the next event
					eventID.Reset()
					eventType.Reset()
					eventData.Reset()
				}
				continue
			}

			// Parse field based on prefix and handle both formats: "field:value" and "field: value"
			if strings.HasPrefix(line, "id:") {
				value := strings.TrimSpace(strings.TrimPrefix(line, "id:"))
				eventID.WriteString(value)
			} else if strings.HasPrefix(line, "event:") {
				value := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				eventType.WriteString(value)
			} else if strings.HasPrefix(line, "data:") {
				// Append data fields with newlines
				if eventData.Len() > 0 {
					eventData.WriteString("\n")
				}
				value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				eventData.WriteString(value)
			} else if line == ":" {
				// Comment, ignore
			}
		}
	}
}

// close closes the event source
func (es *eventSource) close() {
	// Close the done channel to signal goroutines to stop
	select {
	case <-es.done:
		// Already closed
	default:
		close(es.done)
	}

	// Close the reader if it exists
	if es.reader != nil {
		es.reader.Close()
		es.reader = nil
	}
}
