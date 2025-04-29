// Package types defines core interfaces and common types used across the GoMCP library.
package types

import (
	"context"
)

// Transport defines the interface for communication between MCP clients and servers.
// It abstracts the underlying transport mechanism (stdio, websocket, etc.) and
// provides a consistent API for sending and receiving messages.
type Transport interface {
	// Send transmits a message over the transport, respecting the provided context.
	// It returns an error if the message could not be sent or if the context is cancelled.
	Send(ctx context.Context, data []byte) error

	// Receive blocks until a message is received, respecting the provided context, or an error occurs.
	// It returns the received message as a byte slice and any error that occurred (including context errors).
	Receive(ctx context.Context) ([]byte, error)

	// EstablishReceiver sets up the channel for receiving messages from the server.
	// For transports like SSE, this might involve making a GET request.
	// This should be called before sending the 'initialize' request in some protocols.
	EstablishReceiver(ctx context.Context) error // ADDED

	// Close terminates the transport connection.
	// After Close is called, the transport should not be used.
	Close() error

	// IsClosed returns true if the transport connection is closed.
	IsClosed() bool
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
