package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/websocket"
	"github.com/localrivet/gomcp/types"
)

// baseJSONRPCMessage is used for initial parsing to determine message type
type baseJSONRPCMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
}

// websocketTransport implements ClientTransport using WebSockets
type websocketTransport struct {
	baseURL       string
	basePath      string
	options       *TransportOptions
	logger        logx.Logger
	notifyHandler NotificationHandler

	transport    types.Transport
	connected    bool
	connMutex    sync.RWMutex
	responseMap  *sync.Map // map[string]chan *protocol.JSONRPCResponse
	notifyBuffer chan *protocol.JSONRPCNotification

	// Context for managing the event stream goroutine
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(baseURL, basePath string, logger logx.Logger, options ...TransportOption) (ClientTransport, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, NewTransportError("websocket", "invalid base URL", err)
	}

	// Ensure basePath has a leading slash but no trailing slash
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimSuffix(basePath, "/")

	// Convert http(s) URL to ws(s) URL if necessary
	scheme := parsedURL.Scheme
	if scheme == "http" {
		scheme = "ws"
	} else if scheme == "https" {
		scheme = "wss"
	}

	// Rebuild the URL with the correct scheme and path
	wsURL := fmt.Sprintf("%s://%s%s", scheme, parsedURL.Host, basePath)

	// Apply transport options
	opts := DefaultTransportOptions()
	for _, option := range options {
		option(opts)
	}

	t := &websocketTransport{
		baseURL:      wsURL,
		basePath:     basePath,
		options:      opts,
		logger:       logger,
		connected:    false,
		responseMap:  &sync.Map{},
		notifyBuffer: make(chan *protocol.JSONRPCNotification, 100),
	}

	// Create root context that will be used to manage the WebSocket stream
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t, nil
}

// Connect establishes the WebSocket connection
func (t *websocketTransport) Connect(ctx context.Context) error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.connected {
		return NewConnectionError(t.baseURL, "already connected", ErrAlreadyConnected)
	}

	// Create transport options for the WebSocket
	wsOpts := types.TransportOptions{
		Logger: t.logger,
	}

	// Dial the WebSocket URL
	t.logger.Info("Connecting to WebSocket at %s", t.baseURL)
	transport, err := websocket.Dial(ctx, t.baseURL, wsOpts)
	if err != nil {
		return NewTransportError("websocket", fmt.Sprintf("failed to connect to WebSocket at %s", t.baseURL), err)
	}

	t.transport = transport
	t.connected = true

	// Start a goroutine to handle incoming messages
	go t.receiveLoop()

	t.logger.Info("WebSocket transport connected to %s", t.baseURL)
	return nil
}

// Close closes the WebSocket connection
func (t *websocketTransport) Close() error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if !t.connected {
		return nil
	}

	// Cancel context to stop all goroutines
	t.cancel()

	// Close the transport
	if err := t.transport.Close(); err != nil {
		t.logger.Error("Failed to close WebSocket connection: %v", err)
	}

	// Close any pending response channels
	t.responseMap.Range(func(key, value interface{}) bool {
		if ch, ok := value.(chan *protocol.JSONRPCResponse); ok {
			close(ch)
		}
		t.responseMap.Delete(key)
		return true
	})

	t.connected = false
	t.logger.Info("WebSocket transport disconnected from %s", t.baseURL)

	return nil
}

// IsConnected returns true if the transport is connected
func (t *websocketTransport) IsConnected() bool {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()

	// Check internal state first
	if !t.connected || t.transport == nil {
		return false
	}

	// Check if the transport is closed
	if t.transport.IsClosed() {
		// If the transport reports closed but our state says connected,
		// update our internal state to be consistent
		t.connMutex.RUnlock()
		t.connMutex.Lock()
		if t.connected {
			t.connected = false
			t.logger.Warn("WebSocket connection was closed externally")
		}
		t.connMutex.Unlock()
		t.connMutex.RLock()
		return false
	}

	return true
}

// SendRequest sends a request to the server and waits for a response
func (t *websocketTransport) SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.IsConnected() {
		return nil, NewConnectionError(t.baseURL, "not connected", ErrNotConnected)
	}

	// Marshal the request to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, NewTransportError("websocket", "failed to marshal request", err)
	}

	// Create a response channel for this request
	responseCh := make(chan *protocol.JSONRPCResponse, 1)
	defer close(responseCh)

	// Register the response channel
	t.responseMap.Store(fmt.Sprintf("%v", req.ID), responseCh)
	defer t.responseMap.Delete(fmt.Sprintf("%v", req.ID))

	// Send the request
	// Add newline for compatibility with line-based protocols
	reqData = append(reqData, '\n')
	if err := t.transport.Send(ctx, reqData); err != nil {
		return nil, NewTransportError("websocket", "failed to send request", err)
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
func (t *websocketTransport) SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error {
	if !t.IsConnected() {
		return NewConnectionError(t.baseURL, "not connected", ErrNotConnected)
	}

	// Marshal the request to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return NewTransportError("websocket", "failed to marshal request", err)
	}

	// Store the request ID for logging
	reqID := fmt.Sprintf("%v", req.ID)

	// Register the response channel if not nil
	if responseCh != nil {
		t.logger.Debug("Registering response channel for async request ID: %s", reqID)
		// We need to store the channel exactly as provided to preserve its type
		t.responseMap.Store(reqID, responseCh)
	}

	// Send the request
	// Add newline for compatibility with line-based protocols
	reqData = append(reqData, '\n')
	t.logger.Debug("WebSocketTransport Send: %s", string(reqData[:len(reqData)-1])) // Log without newline

	if err := t.transport.Send(ctx, reqData); err != nil {
		// Clean up the response channel registration on error
		if responseCh != nil {
			t.responseMap.Delete(reqID)
		}
		return NewTransportError("websocket", "failed to send request", err)
	}

	t.logger.Debug("WebSocketTransport Send: Write completed.")
	t.logger.Debug("Async request sent with ID: %s", reqID)

	return nil
}

// SetNotificationHandler sets the handler for incoming server notifications
func (t *websocketTransport) SetNotificationHandler(handler NotificationHandler) {
	t.notifyHandler = handler
}

// GetTransportType returns the transport type
func (t *websocketTransport) GetTransportType() TransportType {
	return TransportTypeWebSocket
}

// GetTransportInfo returns transport-specific information
func (t *websocketTransport) GetTransportInfo() map[string]interface{} {
	return map[string]interface{}{
		"baseURL":   t.baseURL,
		"basePath":  t.basePath,
		"connected": t.IsConnected(),
	}
}

// receiveLoop handles incoming messages from the WebSocket
func (t *websocketTransport) receiveLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Create a context with timeout for the receive operation
			ctx, cancel := context.WithTimeout(t.ctx, t.options.ReadTimeout)
			data, err := t.transport.Receive(ctx)
			cancel()

			if err != nil {
				// Check if the context was cancelled or closed intentionally
				if t.ctx.Err() != nil || t.transport.IsClosed() {
					return
				}

				t.logger.Error("Error receiving from WebSocket: %v", err)

				// Attempt to reconnect or close connection on fatal errors
				if strings.Contains(err.Error(), "connection reset") ||
					strings.Contains(err.Error(), "EOF") ||
					strings.Contains(err.Error(), "use of closed") {
					t.logger.Warn("Fatal WebSocket error, closing connection: %v", err)
					t.Close()
					return
				}

				// For non-fatal errors, continue the loop
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Skip empty messages
			if len(data) == 0 {
				continue
			}

			// Remove any trailing newline
			data = bytes.TrimSuffix(data, []byte("\n"))

			// Log the received data for debugging
			t.logger.Debug("WebSocketTransport Received: %s", string(data))

			// Try to parse the message as a JSON-RPC response or notification
			var message baseJSONRPCMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.logger.Error("Failed to parse WebSocket message: %v", err)
				continue
			}

			// Check if it's a response (has ID)
			if message.ID != nil {
				var response protocol.JSONRPCResponse
				if err := json.Unmarshal(data, &response); err != nil {
					t.logger.Error("Failed to parse WebSocket response: %v", err)
					continue
				}

				// Find the response channel
				id := fmt.Sprintf("%v", response.ID)
				t.logger.Debug("Looking for response handler for ID: %s", id)

				if ch, ok := t.responseMap.Load(id); ok {
					t.logger.Debug("Found response handler for ID: %s", id)

					// Try to deliver the response based on channel type
					switch respCh := ch.(type) {
					case chan *protocol.JSONRPCResponse:
						t.logger.Debug("Delivering response to internal channel for ID: %s", id)
						select {
						case respCh <- &response:
							t.logger.Debug("Response delivered successfully to internal channel for ID %s", id)
						case <-time.After(100 * time.Millisecond):
							t.logger.Warn("Timeout delivering response to internal channel for ID %s", id)
						}
					case chan<- *protocol.JSONRPCResponse:
						t.logger.Debug("Delivering response to external channel for ID: %s", id)
						select {
						case respCh <- &response:
							t.logger.Debug("Response delivered successfully to external channel for ID %s", id)
						case <-time.After(100 * time.Millisecond):
							t.logger.Warn("Timeout delivering response to external channel for ID %s", id)
						}
					default:
						t.logger.Error("Response channel for ID %s is not a valid channel type: %T", id, ch)
					}
				} else {
					t.logger.Debug("No handler found for response ID: %s", id)
				}
			} else if message.Method != "" {
				// It's a notification
				var notification protocol.JSONRPCNotification
				if err := json.Unmarshal(data, &notification); err != nil {
					t.logger.Error("Failed to parse WebSocket notification: %v", err)
					continue
				}

				// Process the notification
				if t.notifyHandler != nil {
					if err := t.notifyHandler(&notification); err != nil {
						t.logger.Error("Notification handler error: %v", err)
					}
				} else {
					// Buffer the notification for later processing
					select {
					case t.notifyBuffer <- &notification:
						// Notification buffered successfully
					default:
						t.logger.Warn("Notification buffer is full, dropping notification")
					}
				}
			} else {
				t.logger.Warn("Received message is neither a response nor a notification: %s", string(data))
			}
		}
	}
}
