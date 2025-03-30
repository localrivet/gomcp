// Package types defines core interfaces and common types used across the GoMCP library.
package types

import (
	"context"
)

// Transport defines the interface for communication between MCP clients and servers.
// It abstracts the underlying transport mechanism (stdio, websocket, etc.) and
// provides a consistent API for sending and receiving messages.
type Transport interface {
	// Send transmits a message over the transport.
	// It returns an error if the message could not be sent.
	Send(data []byte) error

	// Receive blocks until a message is received or an error occurs.
	// It returns the received message as a byte slice and any error that occurred.
	Receive() ([]byte, error)

	// ReceiveWithContext is like Receive but respects the provided context.
	// It allows for cancellation and timeouts when waiting for messages.
	ReceiveWithContext(ctx context.Context) ([]byte, error)

	// Close terminates the transport connection.
	// After Close is called, the transport should not be used.
	Close() error
}

// TransportFactory creates new Transport instances.
// Different implementations can create different types of transports.
type TransportFactory interface {
	// NewTransport creates a new Transport instance.
	NewTransport() (Transport, error)

	// NewTransportWithOptions creates a new Transport instance with the specified options.
	NewTransportWithOptions(opts TransportOptions) (Transport, error)
}

// TransportOptions contains configuration options for creating a Transport.
// Different transport implementations may use different fields.
type TransportOptions struct {
	// BufferSize specifies the size of the read/write buffers.
	BufferSize int

	// Logger is used for logging transport-related events.
	Logger Logger

	// Custom options can be provided as key-value pairs.
	Custom map[string]interface{}
}

// MessageHandler is a function that processes received messages.
// It is used by higher-level components to handle incoming messages.
type MessageHandler func(data []byte) error
