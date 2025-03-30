// Package server provides the MCP server implementation.
package server

import (
	"context"  // Added for SSEContextFunc
	"net/http" // Added for SSEContextFunc

	"github.com/localrivet/gomcp/protocol"
)

// ClientSession represents an active connection from a single client.
// The core MCPServer uses this interface to interact with connected clients,
// primarily for sending asynchronous messages like notifications or responses
// that aren't part of a direct request-response flow handled by HandleMessage.
type ClientSession interface {
	// SessionID returns a unique identifier for this session.
	SessionID() string

	// SendNotification sends a JSON-RPC notification to the client session.
	SendNotification(notification protocol.JSONRPCNotification) error

	// SendResponse sends a JSON-RPC response to the client session.
	SendResponse(response protocol.JSONRPCResponse) error

	// Close terminates the client session and cleans up resources.
	Close() error

	// Initialize marks the session as having completed the MCP handshake.
	Initialize()

	// Initialized returns true if the session has completed the MCP handshake.
	Initialized() bool
}

// SSEContextFunc is a function type used by the SSEServer to allow
// customization of the context passed to the core MCPServer's HandleMessage method,
// based on the incoming HTTP request for client->server messages.
// This allows injecting values from HTTP headers (like auth tokens) into the context.
type SSEContextFunc func(ctx context.Context, r *http.Request) context.Context

// Note: The ClientSession interface focuses on server-to-client communication needs
// for asynchronous messages. Client-to-server requests are handled separately
// (e.g., via an HTTP endpoint calling MCPServer.HandleMessage).
