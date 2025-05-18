// Package transport provides the transport layer implementations for the MCP protocol.
//
// This package contains the Transport interface and implementations for different communication methods.
package transport

import (
	"errors"
)

// MessageHandler represents a function that handles incoming messages
type MessageHandler func(message []byte) ([]byte, error)

// DebugHandler represents a function that receives debug messages from the transport
type DebugHandler func(message string)

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

	// SetDebugHandler sets a handler for debug messages
	SetDebugHandler(handler DebugHandler)
}

// BaseTransport provides common transport functionality
type BaseTransport struct {
	handler      MessageHandler
	debugHandler DebugHandler
	// Additional fields can be added as needed
}

// SetMessageHandler sets the message handler
func (t *BaseTransport) SetMessageHandler(handler MessageHandler) {
	t.handler = handler
}

// SetDebugHandler sets the debug handler
func (t *BaseTransport) SetDebugHandler(handler DebugHandler) {
	t.debugHandler = handler
}

// GetDebugHandler returns the current debug handler
func (t *BaseTransport) GetDebugHandler() DebugHandler {
	return t.debugHandler
}

// HandleMessage handles an incoming message
func (t *BaseTransport) HandleMessage(message []byte) ([]byte, error) {
	if t.handler == nil {
		return nil, errors.New("no message handler set")
	}
	return t.handler(message)
}
