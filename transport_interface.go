package gomcp

import (
	"context"
)

// Transport defines the interface for different communication mechanisms
// (stdio, HTTP, WebSocket, etc.) that the core MCP Server can use.
type Transport interface {
	// SendMessage sends a raw JSON message (response or notification) to a specific client session.
	// The sessionID identifies the target client connection managed by the transport.
	SendMessage(sessionID string, message []byte) error

	// Disconnect notifies the core server that a specific client session has disconnected.
	// The transport implementation is responsible for detecting disconnections.
	// The reason error provides context about the disconnection (e.g., io.EOF, network error).
	Disconnect(sessionID string, reason error)

	// RegisterServer allows the transport layer to hold a reference to the core Server
	// instance, enabling it to pass incoming messages for processing.
	RegisterServer(server *Server)

	// GetSessionContext retrieves the context associated with a specific session.
	// This can be used for managing session lifecycle and cancellation.
	// Returns context.Background() if the session is not found or doesn't have a specific context.
	GetSessionContext(sessionID string) context.Context

	// Start initiates the transport layer, making it ready to accept connections
	// or begin communication (e.g., start listening on stdio or HTTP port).
	// This method might block depending on the transport implementation.
	Start() error

	// Stop gracefully shuts down the transport layer, closing connections.
	Stop() error
}

// Session represents a single client connection managed by a transport.
// This might be extended by specific transport implementations.
type Session interface {
	ID() string
	Context() context.Context
	// Send(message []byte) error // Maybe sending is handled by Transport.SendMessage?
}
