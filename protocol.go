// Package mcp provides the core implementation for the Model Context Protocol (MCP)
// in Go. It defines message structures, transport mechanisms (currently stdio),
// and basic client/server logic for establishing connections via the MCP handshake.
package mcp

// Import needed for json.RawMessage used in comments

// Message represents the base structure for all MCP messages.
// Specific message types typically embed this struct and define their own
// payload structure. The Payload field here is often handled as
// json.RawMessage during transport to allow for type-specific unmarshalling
// after the MessageType is identified.
type Message struct {
	// ProtocolVersion indicates the version of the MCP protocol being used (e.g., "1.0").
	ProtocolVersion string `json:"protocol_version"`
	// MessageID is a unique identifier (e.g., UUID) for this specific message instance.
	MessageID string `json:"message_id"`
	// MessageType indicates the kind of message (e.g., "HandshakeRequest", "UseToolRequest").
	MessageType string `json:"message_type"`
	// Payload contains the message-specific data. Its actual type depends on MessageType.
	// During transport and initial unmarshalling, this is often treated as json.RawMessage.
	Payload interface{} `json:"payload"`
}

// ErrorPayload defines the structure for the payload within an Error message.
type ErrorPayload struct {
	// Code is a machine-readable error identifier (e.g., "UnsupportedProtocolVersion", "ToolNotFound").
	Code string `json:"code"`
	// Message is a human-readable description of the error.
	Message string `json:"message"`
}

// ErrorMessage represents an MCP Error message, sent when a request cannot be
// processed or a protocol violation occurs.
type ErrorMessage struct {
	Message              // Embeds base Message fields (ProtocolVersion, MessageID, MessageType="Error")
	Payload ErrorPayload `json:"payload"`
}

// HandshakeRequestPayload defines the structure for the payload within a HandshakeRequest message.
type HandshakeRequestPayload struct {
	// SupportedProtocolVersions lists the MCP protocol versions the sender supports (e.g., ["1.0"]).
	SupportedProtocolVersions []string `json:"supported_protocol_versions"`
	// ServerName is optionally sent by a server during handshake (e.g., if initiating). Usually empty for clients.
	ServerName string `json:"server_name,omitempty"`
	// ClientName is optionally sent by a client during handshake. Usually empty for servers.
	ClientName string `json:"client_name,omitempty"`
}

// HandshakeRequest represents the initial message sent by a client (or sometimes server)
// to initiate an MCP connection and negotiate the protocol version.
type HandshakeRequest struct {
	Message                         // Embeds base Message fields (ProtocolVersion, MessageID, MessageType="HandshakeRequest")
	Payload HandshakeRequestPayload `json:"payload"`
}

// HandshakeResponsePayload defines the structure for the payload within a HandshakeResponse message.
type HandshakeResponsePayload struct {
	// SelectedProtocolVersion is the protocol version chosen by the responder (usually server)
	// from the sender's supported list.
	SelectedProtocolVersion string `json:"selected_protocol_version"`
	// ServerName is optionally sent by the server in its response.
	ServerName string `json:"server_name,omitempty"`
	// ClientName is optionally sent by the client in its response (e.g., if server initiated).
	ClientName string `json:"client_name,omitempty"`
}

// HandshakeResponse represents the message sent by a server (or client, if server initiated)
// in response to a HandshakeRequest, confirming the connection and selected protocol version.
type HandshakeResponse struct {
	Message                          // Embeds base Message fields (ProtocolVersion, MessageID, MessageType="HandshakeResponse")
	Payload HandshakeResponsePayload `json:"payload"`
}

// TODO: Add structs for other message types (ToolDefinitionRequest, ToolDefinitionResponse, etc.)
// as we implement them.

const (
	// CurrentProtocolVersion defines the MCP version this library implementation supports.
	CurrentProtocolVersion = "1.0"

	// --- Message Type Constants ---

	// MessageTypeError identifies an Error message.
	MessageTypeError = "Error"
	// MessageTypeHandshakeRequest identifies a HandshakeRequest message.
	MessageTypeHandshakeRequest = "HandshakeRequest"
	// MessageTypeHandshakeResponse identifies a HandshakeResponse message.
	MessageTypeHandshakeResponse = "HandshakeResponse"
	// TODO: Add other message type constants (e.g., MessageTypeToolDefinitionRequest)
)
