// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/transport/sse"
)

// SSETransport adapts the sse.Transport to implement the client.Transport interface
type SSETransport struct {
	transport           *sse.Transport
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationHandler func(method string, params []byte)
	mu                  sync.Mutex
	respChan            chan []byte // channel for receiving responses
	respErr             chan error  // channel for receiving errors
	connected           bool
	postEndpoint        string // endpoint for sending messages (received from server)
	debugEnabled        bool
}

// NewSSETransport creates a new SSE transport adapter.
func NewSSETransport(url string) *SSETransport {
	// Ensure the URL uses a valid scheme (http:// or https://)
	// First check if it's an SSE scheme, which we'll convert to HTTP
	if strings.HasPrefix(url, "sse://") {
		url = "http://" + url[6:]
		fmt.Printf("DEBUG: Converting 'sse://' to 'http://': %s\n", url)
	} else if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// If no scheme or unsupported scheme, default to http://
		if !strings.Contains(url, "://") {
			url = "http://" + url
			fmt.Printf("DEBUG: No scheme provided, defaulting to 'http://': %s\n", url)
		} else {
			// Extract host and path from URL with unknown scheme
			parts := strings.SplitN(url, "://", 2)
			if len(parts) == 2 {
				url = "http://" + parts[1]
				fmt.Printf("DEBUG: Converting unknown scheme to 'http://': %s\n", url)
			}
		}
	}

	t := &SSETransport{
		transport:         sse.NewTransport(url),
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		respChan:          make(chan []byte, 10),
		respErr:           make(chan error, 5),
		connected:         false,
		debugEnabled:      true,
	}

	// Set message handler to capture responses
	t.transport.SetMessageHandler(t.handleMessage)

	// Set debug handler
	t.transport.SetDebugHandler(func(msg string) {
		if t.debugEnabled {
			fmt.Printf("SSE DEBUG: %s\n", msg)
		}
	})

	return t
}

// handleMessage processes incoming messages and routes them accordingly
func (t *SSETransport) handleMessage(message []byte) ([]byte, error) {
	fmt.Printf("SSE ADAPTER DEBUG: Received message [%d bytes]: %s\n", len(message), string(message))

	// Check if this looks like the endpoint message
	// The endpoint message could be either:
	// 1. A plain URL string starting with http(s)://
	// 2. A JSON object containing an endpoint field

	// Case 1: Plain URL string
	if len(message) > 0 && (bytes.HasPrefix(message, []byte("http://")) ||
		bytes.HasPrefix(message, []byte("https://"))) {
		fmt.Printf("SSE ADAPTER DEBUG: Detected endpoint URL (direct): %s\n", string(message))
		t.handleEndpointMessage(message)
		return nil, nil
	}

	// Case 2: JSON object with endpoint
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(message, &jsonMsg); err == nil {
		// Check if it's an endpoint notification
		if endpoint, ok := jsonMsg["endpoint"].(string); ok && strings.HasPrefix(endpoint, "http") {
			fmt.Printf("SSE ADAPTER DEBUG: Detected endpoint URL (in JSON): %s\n", endpoint)
			t.handleEndpointMessage([]byte(endpoint))
			return nil, nil
		}

		// Check if it's a connected notification
		if connected, ok := jsonMsg["connected"].(bool); ok && connected {
			fmt.Printf("SSE ADAPTER DEBUG: Received connected notification\n")
			// If we don't have an endpoint yet but received confirmation, use the base URL
			t.mu.Lock()
			if t.postEndpoint == "" {
				baseURL := t.transport.GetAddr()
				fmt.Printf("SSE ADAPTER DEBUG: Base URL from transport: %s\n", baseURL)
				if !strings.HasSuffix(baseURL, "/message") {
					if !strings.HasSuffix(baseURL, "/") {
						baseURL += "/"
					}
					baseURL += "message"
				}
				t.postEndpoint = baseURL
				t.connected = true
				fmt.Printf("SSE ADAPTER DEBUG: Derived endpoint URL: %s\n", baseURL)

				// Notify about the endpoint
				if t.notificationHandler != nil {
					go t.notificationHandler("endpoint", []byte(t.postEndpoint))
				}
			}
			t.mu.Unlock()
			return nil, nil
		}
	}

	// Forward to notification handler if it's a notification
	if t.notificationHandler != nil {
		// Try to determine if this is a JSON-RPC notification vs a response
		var msg struct {
			ID interface{} `json:"id"`
		}
		if err := json.Unmarshal(message, &msg); err == nil && msg.ID == nil {
			// No ID means it's a notification
			fmt.Printf("SSE ADAPTER DEBUG: Detected notification (no ID), forwarding to handler\n")
			go t.notificationHandler("", message)
			return nil, nil
		}
	}

	// Put on response channel for any waiting requests
	fmt.Printf("SSE ADAPTER DEBUG: Putting message on response channel\n")
	select {
	case t.respChan <- message:
		fmt.Printf("SSE ADAPTER DEBUG: Successfully put message on response channel\n")
	default:
		fmt.Printf("SSE ADAPTER DEBUG: Response channel full or no one waiting\n")
	}

	// Return nil to prevent the SSE transport from automatically responding
	return nil, nil
}

// handleEndpointMessage processes and stores the endpoint URL
func (t *SSETransport) handleEndpointMessage(message []byte) {
	endpointURL := string(message)

	fmt.Printf("SSE ADAPTER DEBUG: Processing endpoint URL: %s\n", endpointURL)

	t.mu.Lock()
	t.postEndpoint = endpointURL
	wasConnected := t.connected
	t.connected = true
	t.mu.Unlock()

	fmt.Printf("SSE ADAPTER DEBUG: Stored endpoint URL: %s, was connected: %v\n", endpointURL, wasConnected)

	// Notify that the endpoint has been received
	if t.notificationHandler != nil {
		fmt.Printf("SSE ADAPTER DEBUG: Calling notification handler with endpoint\n")
		t.notificationHandler("endpoint", message)
	} else {
		fmt.Printf("SSE ADAPTER DEBUG: No notification handler registered\n")
	}

	// Signal connection success if this is the first time
	if !wasConnected {
		fmt.Printf("SSE ADAPTER DEBUG: Connection established with endpoint\n")
		select {
		case t.respChan <- []byte(`{"connected":true}`):
			fmt.Printf("SSE ADAPTER DEBUG: Sent connected notification\n")
		default:
			fmt.Printf("SSE ADAPTER DEBUG: Connected notification channel full, skipping\n")
		}
	}
}

// Connect establishes a connection to the server.
func (t *SSETransport) Connect() error {
	t.mu.Lock()
	if t.connected && t.postEndpoint != "" {
		t.mu.Unlock()
		fmt.Printf("SSE ADAPTER DEBUG: Already connected with endpoint %s\n", t.postEndpoint)
		return nil
	}
	t.mu.Unlock()

	// Initialize the transport
	if err := t.transport.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize SSE transport: %w", err)
	}

	fmt.Printf("SSE ADAPTER DEBUG: Starting transport\n")

	// Start the transport
	if err := t.transport.Start(); err != nil {
		return fmt.Errorf("failed to start SSE transport: %w", err)
	}

	fmt.Printf("SSE ADAPTER DEBUG: Transport started, waiting for endpoint\n")

	// We need to wait for the endpoint URL to be received
	endpointReceived := make(chan struct{})

	// Create a temporary notification handler that will signal when endpoint is received
	previousHandler := t.notificationHandler
	t.mu.Lock()
	t.notificationHandler = func(method string, params []byte) {
		fmt.Printf("SSE ADAPTER DEBUG: Notification handler called with method: %s, params: %s\n", method, string(params))

		// Call the previous handler if it exists
		if previousHandler != nil {
			previousHandler(method, params)
		}

		// If this is the endpoint notification, signal that we received it
		if method == "endpoint" {
			fmt.Printf("SSE ADAPTER DEBUG: Endpoint notification received: %s\n", string(params))
			select {
			case <-endpointReceived: // Already closed
				fmt.Printf("SSE ADAPTER DEBUG: Endpoint already received, ignoring duplicate\n")
			default:
				fmt.Printf("SSE ADAPTER DEBUG: Signaling endpoint received\n")
				close(endpointReceived)
			}
		} else if t.postEndpoint != "" {
			// If we already have the endpoint URL but haven't signaled it yet
			fmt.Printf("SSE ADAPTER DEBUG: We have endpoint URL but notification came through different channel\n")
			select {
			case <-endpointReceived: // Already closed
				fmt.Printf("SSE ADAPTER DEBUG: Channel already closed, ignoring\n")
			default:
				fmt.Printf("SSE ADAPTER DEBUG: Signaling endpoint received\n")
				close(endpointReceived)
			}
		}
	}
	t.mu.Unlock()

	// Check if we got the endpoint while setting up the handler
	t.mu.Lock()
	if t.connected && t.postEndpoint != "" {
		t.mu.Unlock()
		fmt.Printf("SSE ADAPTER DEBUG: Endpoint was already set: %s\n", t.postEndpoint)
		select {
		case <-endpointReceived: // Already closed
			fmt.Printf("SSE ADAPTER DEBUG: Channel already closed\n")
		default:
			fmt.Printf("SSE ADAPTER DEBUG: Closing endpoint channel\n")
			close(endpointReceived)
		}
	} else {
		t.mu.Unlock()
		fmt.Printf("SSE ADAPTER DEBUG: Endpoint not set yet, waiting for it\n")
	}

	// Wait for the endpoint with a timeout
	fmt.Printf("SSE ADAPTER DEBUG: Waiting for endpoint signal with timeout %v\n", t.connectionTimeout)
	select {
	case <-endpointReceived:
		// Endpoint received, connection established
		fmt.Printf("SSE ADAPTER DEBUG: Connection successfully established\n")
		t.mu.Lock()
		fmt.Printf("SSE ADAPTER DEBUG: Final endpoint URL: %s\n", t.postEndpoint)
		t.connected = true
		t.mu.Unlock()
		return nil
	case <-time.After(t.connectionTimeout / 2):
		// Timeout waiting for endpoint - use a derived endpoint
		fmt.Printf("SSE ADAPTER DEBUG: Partial timeout - generating default endpoint URL\n")
		t.mu.Lock()
		baseURL := t.transport.GetAddr()
		// If we don't already have a post endpoint, derive one
		if t.postEndpoint == "" {
			if !strings.HasSuffix(baseURL, "/message") {
				if !strings.HasSuffix(baseURL, "/") {
					baseURL += "/"
				}
				baseURL += "message"
			}
			t.postEndpoint = baseURL
			fmt.Printf("SSE ADAPTER DEBUG: Using derived endpoint URL: %s\n", baseURL)
		}
		t.connected = true
		t.mu.Unlock()

		// Even if we didn't receive the notification, let's try to proceed
		return nil
	}
}

// ConnectWithContext establishes a connection to the server with context for timeout/cancellation.
func (t *SSETransport) ConnectWithContext(ctx context.Context) error {
	// Create a channel to signal completion
	done := make(chan error, 1)

	// Start connection in a goroutine
	go func() {
		done <- t.Connect()
	}()

	// Wait for connection or context cancellation
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Disconnect closes the connection to the server.
func (t *SSETransport) Disconnect() error {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil
	}

	// Mark as disconnected before stopping to prevent reconnection attempts
	t.connected = false
	postEndpoint := t.postEndpoint
	t.postEndpoint = ""
	t.mu.Unlock()

	if t.debugEnabled {
		fmt.Printf("SSE ADAPTER DEBUG: Disconnecting from endpoint %s\n", postEndpoint)
	}

	// Stop the transport
	err := t.transport.Stop()
	if err != nil && t.debugEnabled {
		fmt.Printf("SSE ADAPTER DEBUG: Error stopping transport: %v\n", err)
	}

	return err
}

// Send sends a message to the server and waits for a response.
func (t *SSETransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

// SendWithContext sends a message with context for timeout/cancellation.
func (t *SSETransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Add more detailed debug logging about the message
	var msgType string
	// Try to detect if this is an initialization message with capabilities
	if bytes.Contains(message, []byte(`"method":"initialize"`)) {
		msgType = "initialize"
	} else if bytes.Contains(message, []byte(`"method":"ping"`)) {
		msgType = "ping"
	} else {
		msgType = "regular"
	}

	fmt.Printf("SSE TRANSPORT DEBUG: Sending %s message: %s\n", msgType, string(message))

	// Check if we're connected
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		fmt.Printf("SSE TRANSPORT DEBUG: Error - not connected to SSE server\n")
		return nil, fmt.Errorf("not connected to SSE server")
	}

	// Get the endpoint URL
	postEndpoint := t.postEndpoint
	t.mu.Unlock()

	if postEndpoint == "" {
		fmt.Printf("SSE TRANSPORT DEBUG: Error - missing POST endpoint URL\n")
		return nil, fmt.Errorf("missing POST endpoint URL")
	}

	// Create the HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", postEndpoint, bytes.NewReader(message))
	if err != nil {
		fmt.Printf("SSE TRANSPORT DEBUG: Error creating request: %v\n", err)
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set appropriate headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Create a client with appropriate timeout
	client := &http.Client{
		Timeout: t.requestTimeout,
	}

	fmt.Printf("SSE TRANSPORT DEBUG: Sending HTTP POST to %s\n", postEndpoint)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("SSE TRANSPORT DEBUG: Error sending HTTP POST: %v\n", err)
		// Check if error was due to context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("HTTP request failed with status: %d, body: %s", resp.StatusCode, string(body))
		fmt.Printf("SSE TRANSPORT DEBUG: %s\n", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("SSE TRANSPORT DEBUG: Error reading response: %v\n", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("SSE TRANSPORT DEBUG: Received response [%d bytes]: %s\n", len(body), string(body))

	// Check for empty response
	if len(body) == 0 {
		fmt.Printf("SSE TRANSPORT DEBUG: Empty response body\n")
		return nil, nil
	}

	return body, nil
}

// SetRequestTimeout sets the default timeout for request operations.
func (t *SSETransport) SetRequestTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.requestTimeout = timeout
	if t.debugEnabled {
		fmt.Printf("SSE ADAPTER DEBUG: Request timeout set to %v\n", timeout)
	}
}

// SetConnectionTimeout sets the default timeout for connection operations.
func (t *SSETransport) SetConnectionTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connectionTimeout = timeout
	if t.debugEnabled {
		fmt.Printf("SSE ADAPTER DEBUG: Connection timeout set to %v\n", timeout)
	}
}

// RegisterNotificationHandler registers a handler for server-initiated messages.
func (t *SSETransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.notificationHandler = handler
	if t.debugEnabled {
		fmt.Printf("SSE ADAPTER DEBUG: Notification handler registered\n")
	}
}

// SetDebugEnabled enables or disables debug logging
func (t *SSETransport) SetDebugEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.debugEnabled = enabled
}

// WithSSE returns a client configuration option that uses SSE transport.
// The SSE transport provides server-sent events for real-time updates from server to client.
// By default, it uses the oldest protocol version for maximum compatibility unless
// the user has explicitly set a different protocol version.
//
// Parameters:
//   - url: The SSE server URL to connect to (e.g., "sse://localhost:8080", "http://localhost:8080")
//
// Returns:
//   - A client configuration option
func WithSSE(url string) Option {
	return func(c *clientImpl) {
		// Log the configuration
		fmt.Printf("Configuring SSE transport with URL: %s\n", url)

		// Create and configure the SSE transport adapter
		transport := NewSSETransport(url)

		// Always enable debug logging for now
		transport.SetDebugEnabled(true)

		// Set timeouts if specified
		transport.SetRequestTimeout(c.requestTimeout)
		transport.SetConnectionTimeout(c.connectionTimeout)

		// Set the transport on the client
		c.transport = transport

		// If user hasn't explicitly set a protocol version, use the oldest one
		// for maximum compatibility with SSE connections
		if c.negotiatedVersion == "" {
			// Get the last element in the supported versions slice, which is the oldest
			if len(mcp.SupportedVersions) > 0 {
				c.negotiatedVersion = mcp.SupportedVersions[len(mcp.SupportedVersions)-1]
				fmt.Printf("Using oldest protocol version for maximum compatibility: %s\n", c.negotiatedVersion)
			}
		}
	}
}
