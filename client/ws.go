package client

import (
	"context"
	"fmt"
	"time"

	"github.com/localrivet/gomcp/transport/ws"
)

// WSTransport wraps a ws.Transport to implement the client.Transport interface
type WSTransport struct {
	transport     *ws.Transport
	notifyHandler func(method string, params []byte)
	reqTimeout    time.Duration
	connTimeout   time.Duration
}

// Connect establishes a connection to the server
func (t *WSTransport) Connect() error {
	if err := t.transport.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize WebSocket transport: %w", err)
	}
	return t.transport.Start()
}

// ConnectWithContext establishes a connection to the server with context
func (t *WSTransport) ConnectWithContext(ctx context.Context) error {
	// Using the context for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return t.Connect()
	}
}

// Disconnect closes the connection to the server
func (t *WSTransport) Disconnect() error {
	return t.transport.Stop()
}

// Send sends a message to the server and waits for a response
func (t *WSTransport) Send(message []byte) ([]byte, error) {
	if err := t.transport.Send(message); err != nil {
		return nil, err
	}

	// Set up a timeout context for receiving the response
	ctx := context.Background()
	if t.reqTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.reqTimeout)
		defer cancel()
	}

	// Create a separate goroutine to handle the response
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		resp, err := t.transport.Receive()
		if err != nil {
			errorCh <- err
			return
		}
		responseCh <- resp
	}()

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errorCh:
		return nil, err
	case resp := <-responseCh:
		return resp, nil
	}
}

// SendWithContext sends a message with context for timeout/cancellation
func (t *WSTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	if err := t.transport.Send(message); err != nil {
		return nil, err
	}

	// Create a separate goroutine to handle the response
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		resp, err := t.transport.Receive()
		if err != nil {
			errorCh <- err
			return
		}
		responseCh <- resp
	}()

	// Wait for response or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errorCh:
		return nil, err
	case resp := <-responseCh:
		return resp, nil
	}
}

// SetRequestTimeout sets the default timeout for request operations
func (t *WSTransport) SetRequestTimeout(timeout time.Duration) {
	t.reqTimeout = timeout
}

// SetConnectionTimeout sets the default timeout for connection operations
func (t *WSTransport) SetConnectionTimeout(timeout time.Duration) {
	t.connTimeout = timeout
}

// RegisterNotificationHandler registers a handler for server-initiated messages
func (t *WSTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notifyHandler = handler
	// The WebSocket transport doesn't have a direct SetNotificationHandler method
	// We would need to implement the notification handling via message parsing
}

// WithWebsocket returns a client configuration option that uses WebSocket transport.
// The WebSocket transport provides a persistent connection for communication with a server.
//
// Parameters:
//   - url: The WebSocket server URL to connect to (e.g., "ws://localhost:8080/ws")
//
// Returns:
//   - A client configuration option
func WithWebsocket(url string) Option {
	return func(c *clientImpl) {
		// Create the WebSocket transport
		wsTransport := ws.NewTransport(url)

		// Wrap it with our adapter
		transport := &WSTransport{
			transport:   wsTransport,
			reqTimeout:  c.requestTimeout,
			connTimeout: c.connectionTimeout,
		}

		c.transport = transport
	}
}

// WithWSPath configures the WebSocket path for the WebSocket connection.
func WithWSPath(path string) Option {
	return func(c *clientImpl) {
		if transport, ok := c.transport.(*WSTransport); ok {
			transport.transport.SetWSPath(path)
		}
	}
}

// WithWSPathPrefix configures the path prefix for the WebSocket connection.
func WithWSPathPrefix(prefix string) Option {
	return func(c *clientImpl) {
		if transport, ok := c.transport.(*WSTransport); ok {
			transport.transport.SetPathPrefix(prefix)
		}
	}
}
