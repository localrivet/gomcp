// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport/stdio"
)

// StdioTransport adapts the stdio transport to the client Transport interface.
type StdioTransport struct {
	transport           *stdio.Transport
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationHandler func(method string, params []byte)
	mu                  sync.Mutex
	respChan            chan []byte // channel for receiving responses
	respErr             chan error  // channel for receiving errors
}

// NewStdioTransport creates a new stdio transport adapter.
func NewStdioTransport() *StdioTransport {
	t := &StdioTransport{
		transport:         stdio.NewTransport(),
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		respChan:          make(chan []byte, 1),
		respErr:           make(chan error, 1),
	}

	// Set message handler to capture responses
	t.transport.SetMessageHandler(t.handleMessage)

	return t
}

// NewStdioTransportWithIO creates a new stdio transport adapter with custom IO.
func NewStdioTransportWithIO(in io.Reader, out io.Writer) *StdioTransport {
	t := &StdioTransport{
		transport:         stdio.NewTransportWithIO(in, out),
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		respChan:          make(chan []byte, 1),
		respErr:           make(chan error, 1),
	}

	// Set message handler to capture responses
	t.transport.SetMessageHandler(t.handleMessage)

	return t
}

// handleMessage processes incoming messages and routes them accordingly
func (t *StdioTransport) handleMessage(message []byte) ([]byte, error) {
	// In a real implementation, we'd parse the message to determine if it's a notification
	// or a response to a previous request

	// For now, we'll assume it's a response to the most recent request
	select {
	case t.respChan <- message:
		// Message successfully sent to response channel
	default:
		// Channel is full or closed, possibly no request is waiting for a response
		// This could be a notification
		if t.notificationHandler != nil {
			// Parse notification (method and params) in a real implementation
			go t.notificationHandler("", message)
		}
	}

	// Return nil to prevent the StdIO transport from automatically responding
	return nil, nil
}

// Connect establishes a connection to the server.
func (t *StdioTransport) Connect() error {
	// Initialize and start the stdio transport
	if err := t.transport.Initialize(); err != nil {
		return err
	}

	return t.transport.Start()
}

// ConnectWithContext establishes a connection to the server with context for timeout/cancellation.
func (t *StdioTransport) ConnectWithContext(ctx context.Context) error {
	// Check for immediate cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return t.Connect()
	}
}

// Disconnect closes the connection to the server.
func (t *StdioTransport) Disconnect() error {
	return t.transport.Stop()
}

// Send sends a message to the server and waits for a response.
func (t *StdioTransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

// SendWithContext sends a message with context for timeout/cancellation.
func (t *StdioTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Send the message
	if err := t.transport.Send(message); err != nil {
		return nil, err
	}

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-t.respErr:
		return nil, err
	case response := <-t.respChan:
		return response, nil
	case <-time.After(t.requestTimeout):
		return nil, context.DeadlineExceeded
	}
}

// SetRequestTimeout sets the default timeout for request operations.
func (t *StdioTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetConnectionTimeout sets the default timeout for connection operations.
func (t *StdioTransport) SetConnectionTimeout(timeout time.Duration) {
	t.connectionTimeout = timeout
}

// RegisterNotificationHandler registers a handler for server-initiated messages.
func (t *StdioTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notificationHandler = handler
}
