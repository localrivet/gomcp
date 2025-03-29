package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Client represents an MCP client instance. It manages the connection to an
// MCP server, handles the handshake, and provides methods for interacting
// with the server (e.g., requesting tool definitions, using tools - to be added).
type Client struct {
	conn       *Connection // The underlying MCP connection handler
	clientName string      // Name announced during handshake
	serverName string      // Name of the server, discovered during handshake
}

// NewClient creates and initializes a new MCP Client instance.
// It configures logging to stderr and sets up the underlying stdio connection.
// The provided clientName is used during the MCP handshake.
func NewClient(clientName string) *Client {
	// Configure logging to stderr for client messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile) // Add timestamp and file/line

	return &Client{
		conn:       NewStdioConnection(), // Assumes stdio for now
		clientName: clientName,
	}
}

// Connect performs the MCP handshake sequence with the server.
// It sends a HandshakeRequest with supported protocol versions and the client name.
// It then waits for a HandshakeResponse or an Error message from the server.
// If the handshake is successful, it validates the selected protocol version
// and stores the server's name.
// Returns nil on successful handshake, otherwise returns an error detailing the failure.
func (c *Client) Connect() error {
	log.Printf("Client '%s' starting handshake...", c.clientName)

	// Send HandshakeRequest
	reqPayload := HandshakeRequestPayload{
		SupportedProtocolVersions: []string{CurrentProtocolVersion},
		ClientName:                c.clientName,
	}
	log.Printf("Sending HandshakeRequest: %+v", reqPayload)
	err := c.conn.SendMessage(MessageTypeHandshakeRequest, reqPayload)
	if err != nil {
		return fmt.Errorf("failed to send HandshakeRequest: %w", err)
	}

	// Wait for HandshakeResponse or Error
	log.Println("Waiting for HandshakeResponse...")
	msg, err := c.conn.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive handshake response: %w", err)
	}

	// Check for Error message first
	if msg.MessageType == MessageTypeError {
		var errPayload ErrorPayload
		// Payload should be json.RawMessage from ReceiveMessage
		rawPayload, ok := msg.Payload.(json.RawMessage)
		if !ok {
			return fmt.Errorf("received Error message with non-RawMessage payload type: %T", msg.Payload)
		}
		unmarshalErr := UnmarshalPayload(rawPayload, &errPayload)
		if unmarshalErr != nil {
			// Log the raw payload if unmarshalling fails
			log.Printf("Failed to unmarshal error payload. Raw: %s", string(rawPayload))
			return fmt.Errorf("received Error message, but failed to unmarshal its payload: %w", unmarshalErr)
		}
		return fmt.Errorf("received MCP Error during handshake: [%s] %s", errPayload.Code, errPayload.Message)

	}

	// Expect HandshakeResponse
	if msg.MessageType != MessageTypeHandshakeResponse {
		return fmt.Errorf("expected HandshakeResponse or Error, got %s", msg.MessageType)
	}

	var respPayload HandshakeResponsePayload
	rawPayload, ok := msg.Payload.(json.RawMessage)
	if !ok {
		return fmt.Errorf("invalid payload format for HandshakeResponse: expected json.RawMessage, got %T", msg.Payload)
	}
	err = UnmarshalPayload(rawPayload, &respPayload)
	if err != nil {
		log.Printf("Failed to unmarshal HandshakeResponse payload. Raw: %s", string(rawPayload))
		return fmt.Errorf("failed to unmarshal HandshakeResponse payload: %w", err)
	}

	log.Printf("Received HandshakeResponse: %+v", respPayload)

	// Validate selected protocol version
	if respPayload.SelectedProtocolVersion != CurrentProtocolVersion {
		return fmt.Errorf("server selected unsupported protocol version: %s", respPayload.SelectedProtocolVersion)
	}

	c.serverName = respPayload.ServerName // Store server name
	log.Printf("Handshake successful with server: %s", c.serverName)

	// Connection is now established and ready for other messages
	return nil
}

// ServerName returns the name of the server as reported during the handshake.
// Returns an empty string if the handshake has not completed successfully or
// if the server did not provide a name.
func (c *Client) ServerName() string {
	return c.serverName
}

// TODO: Add methods for sending other client messages (UseTool, RequestToolDefinitions, etc.)
// TODO: Add a method to run the client's receive loop if needed (e.g., for notifications)

// Close handles the closing of the client connection.
// For the current stdio implementation, this is mostly a placeholder,
// as closing stdin/stdout is not typically done by the application itself.
// If other transports (like net.Conn) were used, this would close the underlying connection.
func (c *Client) Close() error {
	log.Println("Closing client connection (stdio)...")
	// For stdio, closing stdin/stdout isn't usually done by the application.
	// If using net.Conn, would call c.conn.Close() here.
	return nil
}
