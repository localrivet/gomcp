// Package sse provides MCP client transport implementation over Server-Sent Events (SSE)
// using a hybrid approach (SSE for server->client, HTTP POST for client->server).
package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http" // ADDED for tracing
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
	// Removed: "github.com/r3labs/sse/v2"
	// Removed: "gopkg.in/cenkalti/backoff.v1"
)

var (
	headerID    = []byte("id:")
	headerData  = []byte("data:")
	headerEvent = []byte("event:")
	headerRetry = []byte("retry:")
)

// SSEvent represents a Server-Sent Event
type SSEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// establishResult represents the result of attempting to establish an SSE connection
type establishResult struct {
	success bool
	err     error
}

// SSETransport implements the types.Transport interface for the client-side
// hybrid SSE+HTTP MCP transport using standard net/http.
type SSETransport struct {
	httpClient         *http.Client // Used for POST requests
	sseGetClient       *http.Client // Dedicated client for the GET SSE stream
	serverBaseURL      string       // e.g., "http://localhost:8080"
	mcpEndpoint        string       // e.g., "/mcp" (relative path for the single endpoint)
	messageEndpointURL string       // For 2024-11-05 protocol: URL received in the endpoint event
	sessionID          string       // Received from server via Mcp-Session-Id header
	logger             types.Logger
	closed             bool
	closeMutex         sync.Mutex
	sessionMu          sync.Mutex // Mutex for sessionID access

	// Channel for received messages
	receiveChan chan messageOrError
	receiveOnce sync.Once          // Ensures receiver goroutine starts only once
	ctx         context.Context    // Overall context for the transport
	cancel      context.CancelFunc // Function to cancel the context

	// Fields for managing the GET SSE stream
	getRespBody io.ReadCloser  // Stores the response body of the GET request
	getRespMu   sync.Mutex     // Mutex for getRespBody access
	receiverWg  sync.WaitGroup // Waits for the receiver goroutine to finish

	protocolVersion string // Optional: Protocol version (2024-11-05 or 2025-03-26) - defaults to 2025-03-26
}

// messageOrError holds either a received message or an error from the receiver goroutine.
type messageOrError struct {
	data []byte
	err  error
}

// SSETransportOptions holds configuration for the SSE transport.
type SSETransportOptions struct {
	BaseURL      string       // Base URL of the server (e.g., "http://localhost:8080")
	BasePath     string       // Path for the MCP endpoint (e.g., "/mcp")
	HTTPClient   *http.Client // Optional: Custom client for POST requests
	SSEGetClient *http.Client // Optional: Custom client for the GET SSE stream request
	Logger       types.Logger
	// MaxReconnectTries is removed as we don't handle auto-reconnect in this basic version
	ProtocolVersion string // Optional: Protocol version (2024-11-05 or 2025-03-26) - defaults to 2025-03-26
}

// NewSSETransport creates a new SSETransport instance using net/http.
func NewSSETransport(opts SSETransportOptions) (*SSETransport, error) {
	logger := opts.Logger
	if logger == nil {
		logger = logx.NewDefaultLogger() // Use logx
	}

	// Create separate transports for POST and GET clients to ensure complete isolation
	// Disable keep-alives to rule out connection pooling issues with httptest.Server
	postTransport := &http.Transport{DisableKeepAlives: true}
	getTransport := &http.Transport{DisableKeepAlives: true}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		// Default client with timeout for POSTs, using its own transport
		httpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: postTransport, // Assign dedicated transport
		}
	} else if httpClient.Transport == nil {
		// If user provided client without transport, assign ours
		httpClient.Transport = postTransport
	}

	// Use a separate client for the GET SSE stream to avoid potential interactions
	sseGetClient := opts.SSEGetClient
	if sseGetClient == nil {
		// Default client for GET, using its own transport
		sseGetClient = &http.Client{
			Timeout:   30 * time.Second, // Keep timeout for connection phase
			Transport: getTransport,     // Assign dedicated transport
		}
	} else if sseGetClient.Transport == nil {
		// If user provided client without transport, assign ours
		sseGetClient.Transport = getTransport
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Ensure base path starts with a slash if not empty
	basePath := opts.BasePath
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	// Determine protocol version - default to 2024-11-05 if not specified
	protocolVersion := opts.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = protocol.OldProtocolVersion // Default to old version for compatibility
	}

	// Validate protocol version
	if protocolVersion != protocol.CurrentProtocolVersion && protocolVersion != protocol.OldProtocolVersion {
		logger.Warn("Invalid protocol version '%s' specified, defaulting to '%s'",
			protocolVersion, protocol.OldProtocolVersion)
		protocolVersion = protocol.OldProtocolVersion
	}

	t := &SSETransport{
		httpClient:      httpClient,   // For POSTs
		sseGetClient:    sseGetClient, // For GET
		serverBaseURL:   strings.TrimSuffix(opts.BaseURL, "/"),
		mcpEndpoint:     basePath,
		logger:          logger,
		closed:          false,
		receiveChan:     make(chan messageOrError, 100), // Buffered channel
		ctx:             ctx,
		cancel:          cancel,
		protocolVersion: protocolVersion, // Store protocol version
	}

	logger.Info("SSE Transport (net/http) created for server %s using protocol version %s", t.serverBaseURL, t.protocolVersion)
	return t, nil
}

// Ensure SSETransport implements types.Transport
var _ types.Transport = (*SSETransport)(nil)

// EstablishReceiver establishes the server-to-client communication channel for this transport.
func (t *SSETransport) EstablishReceiver(ctx context.Context) error {
	t.closeMutex.Lock()
	isClosed := t.closed
	t.closeMutex.Unlock()
	if isClosed {
		return fmt.Errorf("transport is closed")
	}

	// Use receiveOnce to ensure the receiver is only established and started once.
	var establishErr error
	connectResultChan := make(chan error, 1) // Channel to signal connection result

	t.receiveOnce.Do(func() {
		t.logger.Info("SSETransport: Establishing receiver...")
		// We pass the transport's context (t.ctx) to the goroutine,
		// but use the provided ctx for the initial GET request.
		t.receiverWg.Add(1) // Increment wait group counter before starting goroutine
		// Pass the result channel to the goroutine
		go t.connectAndListenSSE(ctx, connectResultChan)
		t.logger.Info("SSETransport: Receiver establishment initiated (waiting for GET confirmation)...")
	})

	// Wait for either the connection to be established or a timeout
	select {
	case err := <-connectResultChan:
		establishErr = err
	case <-time.After(30 * time.Second): // Using a 30 second timeout
		establishErr = fmt.Errorf("timeout waiting for SSE connection to be established")
	}

	if establishErr != nil {
		t.logger.Error("SSETransport: Failed to establish SSE connection: %v", establishErr)
		return establishErr
	}

	t.logger.Info("SSETransport: Receiver established successfully")
	return nil
}

// Send uses HTTP POST to send messages to the server's message endpoint.
func (t *SSETransport) Send(ctx context.Context, data []byte) error {
	t.closeMutex.Lock()
	isClosed := t.closed
	t.closeMutex.Unlock()
	if isClosed {
		return fmt.Errorf("transport is closed")
	}

	// Check if it's the initialize request
	var req struct {
		Method string `json:"method"`
	}
	// Use Unmarshal directly on data, no need for intermediate struct if only method is needed
	_ = json.Unmarshal(data, &req) // Ignore error, just check method
	isInitialize := req.Method == protocol.MethodInitialize

	t.sessionMu.Lock()
	sessionID := t.sessionID
	t.sessionMu.Unlock()

	if !isInitialize && sessionID == "" {
		return fmt.Errorf("cannot send non-initialize message, no active session ID yet")
	}

	// Construct the full URL for the POST request
	var postURL string
	var postErr error

	// For 2024-11-05 protocol, use the messageEndpointURL if set
	if t.protocolVersion == protocol.OldProtocolVersion && t.messageEndpointURL != "" {
		rawEndpointURL := t.messageEndpointURL

		// Try parsing endpoint as a URL itself
		endpointParsed, err := url.Parse(rawEndpointURL)
		if err != nil {
			postErr = fmt.Errorf("failed to parse messageEndpointURL '%s': %w", rawEndpointURL, err)
		} else if endpointParsed.IsAbs() {
			// If endpoint is already absolute, use it directly
			postURL = rawEndpointURL
			t.logger.Debug("SSETransport: Using absolute endpoint URL: %s", postURL)
		} else {
			// If endpoint is relative, resolve it against the base server URL
			baseParsed, err := url.Parse(t.serverBaseURL)
			if err != nil {
				postErr = fmt.Errorf("failed to parse serverBaseURL '%s': %w", t.serverBaseURL, err)
			} else {
				log.Printf("[DEBUG TEMP] SSETransport: Resolving Ref: Base='%s', Rel='%s'", baseParsed.String(), endpointParsed.String())
				resolvedURL := baseParsed.ResolveReference(endpointParsed)
				postURL = resolvedURL.String()
				t.logger.Debug("SSETransport: Resolved relative endpoint URL '%s' against base '%s' to: %s", rawEndpointURL, t.serverBaseURL, postURL)
			}
		}
	} else {
		// Default behavior (e.g., 2025 protocol or if endpoint URL not received)
		// Resolve mcpEndpoint against serverBaseURL
		baseParsed, err := url.Parse(t.serverBaseURL)
		if err != nil {
			postErr = fmt.Errorf("failed to parse serverBaseURL '%s': %w", t.serverBaseURL, err)
		} else {
			mcpEndpointParsed, err := url.Parse(t.mcpEndpoint) // Assuming mcpEndpoint is a relative path
			if err != nil {
				postErr = fmt.Errorf("failed to parse mcpEndpoint '%s': %w", t.mcpEndpoint, err)
			} else {
				log.Printf("[DEBUG TEMP] SSETransport: Resolving Ref (Default): Base='%s', Rel='%s'", baseParsed.String(), mcpEndpointParsed.String())
				resolvedURL := baseParsed.ResolveReference(mcpEndpointParsed)
				postURL = resolvedURL.String()
				t.logger.Debug("SSETransport: Using default resolved endpoint: %s", postURL)
			}
		}
	}

	// Check for errors during URL construction
	if postErr != nil {
		t.logger.Error("SSETransport: Failed to construct POST URL: %v", postErr)
		return postErr
	}

	t.logger.Debug("SSETransport Sending POST to %s: %s", postURL, string(data))

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json") // Expect JSON response
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	// Execute the request
	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	t.logger.Debug("SSETransport httpClient.Do completed for POST. Error: %v, Response Status: %s", err, resp.Status)

	// Process response in a nested func to ensure body is closed
	processResponse := func() error {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body) // Read body for error details
			t.logger.Error("SSETransport: POST request failed with status %s: %s", resp.Status, string(bodyBytes))
			return fmt.Errorf("POST request failed with status %s", resp.Status)
		}

		// If this was the Initialize request, extract the session ID from the header
		if isInitialize {
			receivedSessionID := resp.Header.Get("Mcp-Session-Id")
			if receivedSessionID == "" {
				t.logger.Warn("SSETransport: Initialize response missing Mcp-Session-Id header")
				// Continue processing, but session ID will remain unset
			} else {
				t.sessionMu.Lock()
				t.sessionID = receivedSessionID
				t.sessionMu.Unlock()
				t.logger.Info("SSETransport: Stored session ID from InitializeResponse header: %s", receivedSessionID)
			}
			// Forward the response body (which contains the InitializeResult) to the receive channel
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read initialize response body: %w", err)
			}
			t.logger.Debug("SSETransport: Forwarding initialize response to receive channel")
			t.sendToReceiveChan(bodyBytes, nil)
			t.logger.Debug("SSETransport Initialize POST successful.")
			return nil
		} else {
			// For non-initialize POSTs, the response body is usually empty (e.g., 202 Accepted)
			// We don't need to forward it.
			t.logger.Debug("SSETransport non-Initialize POST successful (Status: %d)", resp.StatusCode)
			return nil
		}
	}

	return processResponse()
}

// Receive waits for the next message from the SSE stream.
func (t *SSETransport) Receive(ctx context.Context) ([]byte, error) {
	// Receiver should have been started by EstablishReceiver.
	// If receiveChan is nil or closed unexpectedly, it indicates a problem.
	// t.receiveOnce.Do(t.startReceiver) // REMOVED

	t.closeMutex.Lock()
	isClosed := t.closed
	t.closeMutex.Unlock()
	if isClosed {
		return nil, fmt.Errorf("transport is closed")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.ctx.Done(): // Check overall transport context
		return nil, fmt.Errorf("transport closed")
	case msg, ok := <-t.receiveChan:
		if !ok {
			// Check if closed flag is set, otherwise it might be a premature close
			t.closeMutex.Lock()
			isClosedAfterRead := t.closed
			t.closeMutex.Unlock()
			if isClosedAfterRead {
				return nil, fmt.Errorf("transport closed") // Channel closed normally
			}
			// If not marked as closed, the channel closed unexpectedly
			return nil, fmt.Errorf("receive channel closed unexpectedly")
		}
		return msg.data, msg.err // Return data or error from channel
	}
}

// startReceiver is removed, logic moved into EstablishReceiver/connectAndListenSSE

// connectAndListenSSE handles the connection and reading loop for the GET SSE stream.
// It now takes the initial context for the GET request and a channel to signal connection result.
func (t *SSETransport) connectAndListenSSE(ctx context.Context, result chan<- error) {
	t.logger.Debug("SSETransport: Connecting to SSE stream at %s", t.serverBaseURL+t.mcpEndpoint)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.serverBaseURL+t.mcpEndpoint, nil)
	if err != nil {
		result <- fmt.Errorf("failed to create request: %w", err)
		return
	}

	// Add headers
	t.sessionMu.Lock()
	sessionID := t.sessionID
	t.sessionMu.Unlock()
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Make the connection - this performs the GET request that opens the SSE stream
	t.logger.Debug("SSETransport: Executing SSE GET request with headers: %v", req.Header)
	resp, err := t.sseGetClient.Do(req)
	if err != nil {
		result <- fmt.Errorf("failed to connect to SSE stream: %w", err)
		return
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		result <- fmt.Errorf("non-200 response from SSE stream: %d, body: %s", resp.StatusCode, string(body))
		return
	}

	t.logger.Debug("SSETransport: SSE connection established with status %d", resp.StatusCode)

	// Create event source
	reader := bufio.NewReader(resp.Body)
	t.getRespMu.Lock() // Add lock before unlocking
	t.getRespBody = resp.Body
	t.getRespMu.Unlock()

	// Signal success immediately for 2025-03-26 protocol or wait for endpoint for older protocols
	if t.protocolVersion == protocol.CurrentProtocolVersion {
		t.logger.Debug("SSETransport: Using 2025-03-26 protocol, signaling connection success immediately")
		result <- nil
	}

	// Start the event reading loop
	go func() {
		defer func() {
			t.logger.Debug("SSETransport: SSE read loop ending, closing response body")
			t.getRespMu.Lock()
			t.getRespBody = nil
			t.getRespMu.Unlock()
		}()

		// For old protocol (2024-11-05), we need to wait for the endpoint event before signaling success
		endpointReceived := t.protocolVersion == protocol.CurrentProtocolVersion // Already signaled for new protocol

		for {
			// Check if we should stop
			select {
			case <-t.ctx.Done(): // Use the transport context instead of closeMutex
				t.logger.Debug("SSETransport: Closing SSE connection due to transport close")
				return
			case <-ctx.Done():
				t.logger.Debug("SSETransport: Closing SSE connection due to context cancellation")
				return
			default:
				// Continue reading
			}

			// Read the next event
			event, err := t.readEvent(reader)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || t.closed {
					t.logger.Debug("SSETransport: SSE stream closed by server or context canceled")
				} else {
					t.logger.Error("SSETransport: Error reading SSE event: %v", err)
				}
				return
			}

			// Process the event
			t.logger.Debug("SSETransport: Received SSE event: %s", event.Event)

			// Handle event types
			switch event.Event {
			case "endpoint":
				// For 2024-11-05 protocol, we get the endpoint URL in the data field
				if t.protocolVersion == protocol.OldProtocolVersion {
					// Store the endpoint URL exactly as received from the server
					// The Send method will handle resolving relative URLs when needed
					t.messageEndpointURL = event.Data
					t.logger.Debug("SSETransport: Got endpoint URL (proto 2024-11-05): %s", t.messageEndpointURL)

					// Signal connection success for 2024-11-05 only once we have the endpoint
					if !endpointReceived {
						t.logger.Debug("SSETransport: Signaling connection success after receiving endpoint")
						result <- nil
						endpointReceived = true
					}
				}
			case "message":
				// Parse and dispatch the message
				if event.Data == "" {
					t.logger.Warn("SSETransport: Received empty message data")
					continue
				}

				// Send the message to the receive channel
				t.sendToReceiveChan([]byte(event.Data), nil)
			}
		}
	}()
}

// sendToReceiveChan safely sends data or an error to the receive channel.
func (t *SSETransport) sendToReceiveChan(data []byte, err error) {
	// Check context before sending to avoid blocking if already cancelled
	select {
	case <-t.ctx.Done():
		t.logger.Warn("SSETransport: Context cancelled, not sending message/error to receive channel.")
		return
	default:
	}

	// Use a timer to prevent blocking indefinitely if receiveChan is full and context isn't cancelled yet
	// This shouldn't happen with a buffered channel unless the client stops calling Receive.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case t.receiveChan <- messageOrError{data: data, err: err}:
	case <-t.ctx.Done():
		t.logger.Warn("SSETransport: Context cancelled while trying to send to receive channel.")
	case <-timer.C:
		t.logger.Error("SSETransport: Timeout sending message/error to receive channel. Client might not be calling Receive().")
	}
}

// Close terminates the transport connection.
func (t *SSETransport) Close() error {
	t.closeMutex.Lock()
	if t.closed {
		t.closeMutex.Unlock()
		return nil // Already closed
	}
	// Mark as closed early to prevent new operations
	t.closed = true
	t.logger.Info("SSETransport: Closing...")
	t.closeMutex.Unlock()

	// Cancel the context to signal the receiver goroutine to stop
	t.logger.Debug("SSETransport: Calling context cancel().") // ADDED
	t.cancel()

	// Close the underlying GET response body, if it exists
	t.logger.Debug("SSETransport: Closing GET response body.") // ADDED
	t.getRespMu.Lock()
	if t.getRespBody != nil {
		_ = t.getRespBody.Close()
		t.getRespBody = nil
	}
	t.getRespMu.Unlock()

	// Attempt to close idle connections in the HTTP client
	// This might help ensure the GET request's underlying connection is terminated
	t.logger.Debug("SSETransport: Closing idle HTTP client connections.") // ADDED
	t.sseGetClient.CloseIdleConnections()                                 // ADDED

	// Wait for the receiver goroutine to finish cleaning up
	// Add a timeout to prevent hanging indefinitely if receiverWg.Done() is never called
	t.logger.Debug("SSETransport: Waiting for receiver goroutine to finish...") // ADDED
	waitChan := make(chan struct{})
	go func() {
		t.receiverWg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		t.logger.Debug("SSETransport: Receiver goroutine finished.")
	case <-time.After(10 * time.Second): // INCREASED TIMEOUT
		t.logger.Error("SSETransport: Timeout waiting for receiver goroutine to stop during close.")
		// The receiver might be stuck, but we proceed with closing.
	}

	// The receiveChan is closed by the receiver goroutine itself in its defer func

	t.logger.Info("SSETransport: Closed.")
	return nil
}

// IsClosed returns true if the transport connection is closed.
func (t *SSETransport) IsClosed() bool {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()
	return t.closed
}

// readEvent reads a complete Server-Sent Event from the reader
func (t *SSETransport) readEvent(reader *bufio.Reader) (SSEvent, error) {
	event := SSEvent{}
	var data bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return event, err
		}

		// Trim trailing newlines
		line = bytes.TrimSuffix(line, []byte("\n"))
		line = bytes.TrimSuffix(line, []byte("\r"))

		// Empty line signals the end of an event
		if len(line) == 0 {
			event.Data = data.String()
			return event, nil
		}

		// Parse the line based on field type
		if bytes.HasPrefix(line, []byte("event:")) {
			event.Event = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			// For data fields, append with newlines between multiple data lines
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.Write(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
		} else if bytes.HasPrefix(line, []byte("id:")) {
			event.ID = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("id:"))))
		} else if bytes.HasPrefix(line, []byte("retry:")) {
			// Parse retry as int (ignore error)
			retryStr := string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("retry:"))))
			fmt.Sscanf(retryStr, "%d", &event.Retry)
		} else if bytes.HasPrefix(line, []byte(":")) {
			// Comment line, ignore
			continue
		}
	}
}
