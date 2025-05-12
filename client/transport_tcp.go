package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/tcp"
	"github.com/localrivet/gomcp/types"
)

// tcpTransport implements ClientTransport using TCP connections
type tcpTransport struct {
	addr          string
	options       *TransportOptions
	logger        logx.Logger
	notifyHandler NotificationHandler

	transport    types.Transport
	conn         net.Conn
	connected    bool
	connMutex    sync.RWMutex
	responseMap  *sync.Map // map[string]chan *protocol.JSONRPCResponse
	notifyBuffer chan *protocol.JSONRPCNotification

	// Context for managing the connection lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTCPTransport creates a new TCP transport
func NewTCPTransport(addr string, logger logx.Logger, options ...TransportOption) (ClientTransport, error) {
	// Apply transport options
	opts := DefaultTransportOptions()
	for _, option := range options {
		option(opts)
	}

	t := &tcpTransport{
		addr:         addr,
		options:      opts,
		logger:       logger,
		connected:    false,
		responseMap:  &sync.Map{},
		notifyBuffer: make(chan *protocol.JSONRPCNotification, 100),
	}

	// Create root context that will be used to manage the connection
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t, nil
}

// Connect establishes a TCP connection to the specified address
func (t *tcpTransport) Connect(ctx context.Context) error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.connected {
		return NewConnectionError("tcp", "already connected", ErrAlreadyConnected)
	}

	// Set up a dialer with connection timeout
	dialer := &net.Dialer{
		Timeout: t.options.ConnectTimeout,
	}

	// Connect to the server
	t.logger.Info("Connecting to TCP server at %s", t.addr)
	conn, err := dialer.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return NewConnectionError("tcp", fmt.Sprintf("failed to connect to %s", t.addr), err)
	}

	// Create transport options
	tcpOpts := types.TransportOptions{
		Logger: t.logger,
	}

	// Create the TCP transport
	t.conn = conn
	t.transport = tcp.NewTCPTransport(conn, tcpOpts)
	t.connected = true

	// Start the receive loop
	go t.receiveLoop()

	t.logger.Info("TCP transport connected to %s", t.addr)
	return nil
}

// Close terminates the TCP connection and cleans up resources
func (t *tcpTransport) Close() error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if !t.connected {
		return nil
	}

	// Cancel context to stop all goroutines
	t.cancel()

	// Close the transport
	if err := t.transport.Close(); err != nil {
		t.logger.Error("Failed to close TCP transport: %v", err)
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
	t.logger.Info("TCP transport disconnected from %s", t.addr)

	return nil
}

// IsConnected returns true if the transport is connected
func (t *tcpTransport) IsConnected() bool {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()
	return t.connected && !t.transport.IsClosed()
}

// SendRequest sends a request to the server and waits for a response
func (t *tcpTransport) SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.IsConnected() {
		return nil, NewConnectionError("tcp", "not connected", ErrNotConnected)
	}

	// Marshal the request to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, NewTransportError("tcp", "failed to marshal request", err)
	}

	// Create a response channel for this request
	responseCh := make(chan *protocol.JSONRPCResponse, 1)
	defer close(responseCh)

	// Register the response channel
	reqID := fmt.Sprintf("%v", req.ID)
	t.responseMap.Store(reqID, responseCh)
	defer t.responseMap.Delete(reqID)

	// Send the request
	if err := t.transport.Send(ctx, reqData); err != nil {
		return nil, NewTransportError("tcp", "failed to send request", err)
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
func (t *tcpTransport) SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error {
	if !t.IsConnected() {
		return NewConnectionError("tcp", "not connected", ErrNotConnected)
	}

	// Marshal the request to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return NewTransportError("tcp", "failed to marshal request", err)
	}

	// Register the response channel if not nil
	if responseCh != nil {
		reqID := fmt.Sprintf("%v", req.ID)
		t.logger.Debug("Registering response channel for async request ID: %s", reqID)

		// Note: we need to use an unbuffered channel internally to avoid type issues
		// when retrieving from the sync.Map
		internalCh := make(chan *protocol.JSONRPCResponse, 1)
		t.responseMap.Store(reqID, internalCh)

		// Start a goroutine to forward responses to the client's channel
		go func(id string, internal <-chan *protocol.JSONRPCResponse, external chan<- *protocol.JSONRPCResponse) {
			defer t.responseMap.Delete(id)
			select {
			case resp, ok := <-internal:
				if !ok {
					t.logger.Debug("Internal channel closed for request ID: %s", id)
					return
				}
				t.logger.Debug("Forwarding response from internal to external channel for ID: %s", id)
				external <- resp
			case <-t.ctx.Done():
				t.logger.Debug("Context done before response received for request ID: %s", id)
				return
			case <-ctx.Done():
				t.logger.Debug("Request context done before response received for ID: %s", id)
				return
			}
		}(reqID, internalCh, responseCh)
	}

	// Send the request
	// Add newline for consistency with other transports
	reqData = append(reqData, '\n')
	if err := t.transport.Send(ctx, reqData); err != nil {
		// Clean up the response channel registration on error
		if responseCh != nil {
			reqID := fmt.Sprintf("%v", req.ID)
			t.responseMap.Delete(reqID)
		}
		return NewTransportError("tcp", "failed to send request", err)
	}

	t.logger.Debug("Async request sent with ID: %v", req.ID)
	return nil
}

// SetNotificationHandler sets the handler for incoming server notifications
func (t *tcpTransport) SetNotificationHandler(handler NotificationHandler) {
	t.notifyHandler = handler
}

// GetTransportType returns the transport type
func (t *tcpTransport) GetTransportType() TransportType {
	return TransportTypeTCP
}

// GetTransportInfo returns transport-specific information
func (t *tcpTransport) GetTransportInfo() map[string]interface{} {
	info := map[string]interface{}{
		"type":      "tcp",
		"addr":      t.addr,
		"connected": t.IsConnected(),
	}

	// Add remote and local addresses if connected
	if t.IsConnected() && t.conn != nil {
		info["localAddr"] = t.conn.LocalAddr().String()
		info["remoteAddr"] = t.conn.RemoteAddr().String()
	}

	return info
}

// receiveLoop handles incoming messages from the TCP transport
func (t *tcpTransport) receiveLoop() {
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

				t.logger.Error("Error receiving from TCP transport: %v", err)

				// Check for network errors
				if _, ok := err.(net.Error); ok {
					t.logger.Warn("Network error, closing connection")
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

			t.logger.Debug("Received TCP data: %s", string(data))

			// Try to parse the message as a JSON-RPC response or notification
			var message baseJSONRPCMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.logger.Error("Failed to parse TCP message: %v", err)
				continue
			}

			// Check if it's a response (has ID)
			if message.ID != nil {
				var response protocol.JSONRPCResponse
				if err := json.Unmarshal(data, &response); err != nil {
					t.logger.Error("Failed to parse TCP response: %v", err)
					continue
				}

				// Find the response channel
				id := fmt.Sprintf("%v", message.ID)
				t.logger.Debug("Looking for response handler for ID: %s", id)

				if ch, ok := t.responseMap.Load(id); ok {
					if respCh, validCh := ch.(chan *protocol.JSONRPCResponse); validCh {
						t.logger.Debug("Found response channel for ID %s, delivering response", id)
						select {
						case respCh <- &response:
							t.logger.Debug("Response delivered successfully for ID %s", id)
						default:
							t.logger.Warn("Response channel for ID %s is full or closed", id)
						}
					} else {
						t.logger.Error("Retrieved channel for ID %s is not a valid response channel (type: %T)", id, ch)
					}
				} else {
					t.logger.Debug("No handler found for response ID: %s", id)
				}
			} else if message.Method != "" {
				// It's a notification
				var notification protocol.JSONRPCNotification
				if err := json.Unmarshal(data, &notification); err != nil {
					t.logger.Error("Failed to parse TCP notification: %v", err)
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
