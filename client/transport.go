// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"time"
)

// Transport represents a transport layer for client communication.
// It handles the communication between the client and the server.
type Transport interface {
	// Connect establishes a connection to the server.
	Connect() error

	// ConnectWithContext establishes a connection to the server with context for timeout/cancellation.
	ConnectWithContext(ctx context.Context) error

	// Disconnect closes the connection to the server.
	Disconnect() error

	// Send sends a message to the server and waits for a response.
	Send(message []byte) ([]byte, error)

	// SendWithContext sends a message with context for timeout/cancellation.
	SendWithContext(ctx context.Context, message []byte) ([]byte, error)

	// SetRequestTimeout sets the default timeout for request operations.
	SetRequestTimeout(timeout time.Duration)

	// SetConnectionTimeout sets the default timeout for connection operations.
	SetConnectionTimeout(timeout time.Duration)

	// RegisterNotificationHandler registers a handler for server-initiated messages.
	RegisterNotificationHandler(handler func(method string, params []byte))
}
