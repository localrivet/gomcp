package types

import (
	"io"

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

	// SendRequest sends a JSON-RPC request to the client session.
	SendRequest(request protocol.JSONRPCRequest) error

	// Close terminates the client session and cleans up resources.
	Close() error

	// Initialize marks the session as having completed the MCP handshake.
	Initialize()

	// Initialized returns true if the session has completed the MCP handshake.
	Initialized() bool

	// SetNegotiatedVersion stores the protocol version agreed upon during initialization.
	SetNegotiatedVersion(version string)

	// GetNegotiatedVersion returns the protocol version agreed upon during initialization.
	GetNegotiatedVersion() string

	// StoreClientCapabilities stores the capabilities received from the client during initialization.
	StoreClientCapabilities(caps protocol.ClientCapabilities)

	// GetClientCapabilities returns the stored client capabilities.
	GetClientCapabilities() protocol.ClientCapabilities
	// GetWriter returns the underlying io.Writer for the session.
	GetWriter() io.Writer
}
