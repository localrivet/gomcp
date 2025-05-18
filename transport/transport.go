// Package transport provides the transport layer implementations for the MCP protocol.
//
// This package contains the Transport interface and implementations for different communication methods.
package transport

import (
	"errors"
)

// MessageHandler represents a function that handles incoming messages
type MessageHandler func(message []byte) ([]byte, error)

// Transport represents a communication transport for MCP messages.
type Transport interface {
	// Initialize initializes the transport
	Initialize() error

	// Start starts the transport
	Start() error

	// Stop stops the transport
	Stop() error

	// Send sends a message over the transport.
	Send(message []byte) error

	// Receive receives a message from the transport.
	Receive() ([]byte, error)

	// SetMessageHandler sets the message handler
	SetMessageHandler(handler MessageHandler)
}

// BaseTransport provides common transport functionality
type BaseTransport struct {
	handler MessageHandler
	// Additional fields can be added as needed
}

// SetMessageHandler sets the message handler
func (t *BaseTransport) SetMessageHandler(handler MessageHandler) {
	t.handler = handler
}

// HandleMessage handles an incoming message
func (t *BaseTransport) HandleMessage(message []byte) ([]byte, error) {
	if t.handler == nil {
		return nil, errors.New("no message handler set")
	}
	return t.handler(message)
}
