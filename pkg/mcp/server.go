package mcp

import (
	"fmt"
	"log"
	"os"
)

// Server represents an MCP server instance. It manages the connection,
// handles the handshake, and processes incoming messages from an MCP client.
// It is responsible for defining and executing tools or providing resources
// based on client requests (functionality to be added).
type Server struct {
	conn       *Connection // The underlying MCP connection handler
	serverName string      // Name announced during handshake
	// TODO: Add fields for tool definitions, resource handlers, etc.
	// Example: toolRegistry map[string]ToolDefinition
}

// NewServer creates and initializes a new MCP Server instance.
// It configures logging to stderr and sets up the underlying stdio connection.
// The provided serverName is used during the MCP handshake.
func NewServer(serverName string) *Server {
	// Configure logging to stderr for server messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile) // Add timestamp and file/line

	return &Server{
		conn:       NewStdioConnection(), // Assumes stdio for now
		serverName: serverName,
	}
}

// Run starts the server's main loop. It performs the initial handshake
// and then enters a loop to continuously receive and process messages
// from the client.
// This method blocks until a fatal error occurs during message processing
// or the underlying connection is closed (e.g., receiving io.EOF).
// Currently, it only handles the handshake and logs/errors on other message types.
// Future implementations should add dispatch logic here to handle various
// MCP requests like ToolDefinitionRequest, UseToolRequest, etc.
func (s *Server) Run() error {
	log.Printf("Server '%s' starting...", s.serverName)

	// 1. Perform Handshake
	clientName, err := s.handleHandshake()
	if err != nil {
		log.Printf("Handshake failed: %v", err)
		// Attempt to send error before exiting, ignore error during send
		_ = s.conn.SendMessage(MessageTypeError, ErrorPayload{
			Code:    "HandshakeFailed",
			Message: fmt.Sprintf("Handshake failed: %v", err),
		})
		return fmt.Errorf("handshake failed: %w", err)
	}
	log.Printf("Handshake successful with client: %s", clientName)

	// 2. Main message loop (TODO: Implement full dispatch logic)
	log.Println("Entering main message loop...")
	for {
		msg, err := s.conn.ReceiveMessage()
		if err != nil {
			log.Printf("Error receiving message: %v. Server shutting down.", err)
			return err // Exit loop on receive error (e.g., EOF)
		}

		log.Printf("Received message type: %s", msg.MessageType)

		// --- TODO: Implement message dispatch logic here ---
		switch msg.MessageType {
		// case MessageTypeToolDefinitionRequest:
		//     s.handleToolDefinitionRequest(msg)
		// case MessageTypeUseToolRequest:
		//     s.handleUseToolRequest(msg)
		// ... other message types
		default:
			log.Printf("Handler not implemented for message type: %s", msg.MessageType)
			// Example: Echo back an error for unhandled types
			err = s.conn.SendMessage(MessageTypeError, ErrorPayload{
				Code:    "NotImplemented",
				Message: fmt.Sprintf("Message type '%s' not implemented by server", msg.MessageType),
			})
			if err != nil {
				log.Printf("Error sending NotImplemented error: %v", err)
				// Consider if this error should be fatal for the connection
			}
		}
		// --- End TODO ---
	}
	// Unreachable under normal operation due to the infinite loop
}

// handleHandshake performs the server side of the MCP handshake protocol.
// It waits for a HandshakeRequest, validates the protocol version,
// and sends back either a HandshakeResponse or an Error message.
// Returns the client's name (if provided) and nil on success, or an error.
func (s *Server) handleHandshake() (clientName string, err error) { // Named return values for clarity
	log.Println("Waiting for HandshakeRequest...")
	msg, err := s.conn.ReceiveMessage()
	if err != nil {
		return "", fmt.Errorf("failed to receive initial message: %w", err)
	}

	if msg.MessageType != MessageTypeHandshakeRequest {
		return "", fmt.Errorf("expected HandshakeRequest, got %s", msg.MessageType)
	}

	var reqPayload HandshakeRequestPayload
	// The payload in msg should be json.RawMessage as returned by ReceiveMessage.
	// Use the helper to unmarshal it into the specific payload struct.
	err = UnmarshalPayload(msg.Payload, &reqPayload)
	if err != nil {
		// If unmarshalling the payload fails (e.g., wrong structure, missing fields), return the error.
		log.Printf("Error unmarshalling HandshakeRequest payload: %v", err)
		// Optionally send a specific error message back to the client here?
		// For now, just return the error, Run() will send HandshakeFailed.
		return "", fmt.Errorf("failed to unmarshal HandshakeRequest payload: %w", err)
	}

	// Validate required fields in the payload *after* successful unmarshal
	if reqPayload.SupportedProtocolVersions == nil || len(reqPayload.SupportedProtocolVersions) == 0 {
		// Even if JSON was valid, the required field is missing/empty
		return "", fmt.Errorf("malformed HandshakeRequest payload: missing or empty supported_protocol_versions")
	}

	log.Printf("Received HandshakeRequest: %+v", reqPayload) // Log after successful unmarshal and validation

	// Validate protocol version
	clientSupportsCurrent := false
	for _, v := range reqPayload.SupportedProtocolVersions {
		if v == CurrentProtocolVersion {
			clientSupportsCurrent = true
			break
		}
	}

	if !clientSupportsCurrent {
		// Send specific error according to spec
		sendErr := s.conn.SendMessage(MessageTypeError, ErrorPayload{
			Code:    "UnsupportedProtocolVersion",
			Message: fmt.Sprintf("Server requires protocol version %s, client supports %v", CurrentProtocolVersion, reqPayload.SupportedProtocolVersions),
		})
		if sendErr != nil {
			log.Printf("Failed to send UnsupportedProtocolVersion error: %v", sendErr)
		}
		return "", fmt.Errorf("client does not support protocol version %s", CurrentProtocolVersion)
	}

	// Send HandshakeResponse
	respPayload := HandshakeResponsePayload{
		SelectedProtocolVersion: CurrentProtocolVersion,
		ServerName:              s.serverName,
	}

	log.Printf("Sending HandshakeResponse: %+v", respPayload)
	err = s.conn.SendMessage(MessageTypeHandshakeResponse, respPayload)
	if err != nil {
		return "", fmt.Errorf("failed to send HandshakeResponse: %w", err)
	}

	// Return the client name if provided
	return reqPayload.ClientName, nil
}
