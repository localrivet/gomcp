// client.go (Refactored Connect method)
package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Client represents an MCP client instance. It manages the connection to an
// MCP server, handles the handshake/initialization, and provides methods for interacting
// with the server (e.g., requesting tool definitions, using tools - to be added).
type Client struct {
	conn       *Connection // The underlying MCP connection handler
	clientName string      // Name announced during initialization
	serverName string      // Name of the server, discovered during initialization
	// TODO: Store client/server capabilities
	// clientCapabilities ClientCapabilities
	// serverCapabilities ServerCapabilities
}

// NewClient creates and initializes a new MCP Client instance.
// It configures logging to stderr and sets up the underlying stdio connection.
// The provided clientName is used during the MCP initialization.
func NewClient(clientName string) *Client {
	// Configure logging to stderr for client messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile) // Add timestamp and file/line

	return &Client{
		conn:       NewStdioConnection(), // Assumes stdio for now
		clientName: clientName,
	}
}

// Connect performs the MCP Initialization sequence with the server.
// It sends an InitializeRequest with client capabilities and info.
// It then waits for an InitializeResponse or an Error message from the server.
// If successful, it validates the protocol version, stores server info,
// and sends an InitializedNotification.
// Returns nil on successful initialization, otherwise returns an error.
func (c *Client) Connect() error {
	log.Printf("Client '%s' starting initialization...", c.clientName)

	// --- Send InitializeRequest ---
	// Define client capabilities (can be expanded later)
	clientCapabilities := ClientCapabilities{
		// Experimental: map[string]interface{}{"myFeature": true},
	}
	// Define client info
	clientInfo := Implementation{
		Name:    c.clientName,
		Version: "0.1.0", // Example client version
	}
	// Create request parameters
	reqParams := InitializeRequestParams{
		ProtocolVersion: CurrentProtocolVersion, // Send the version we support
		Capabilities:    clientCapabilities,
		ClientInfo:      clientInfo,
	}

	log.Printf("Sending InitializeRequest: %+v", reqParams)
	// Send the request using the correct method name "initialize"
	// Note: SendMessage currently wraps this in a basic Message struct.
	// A stricter JSON-RPC implementation might require changes here or in SendMessage.
	// We also need to handle the JSON-RPC 'id' field properly for request/response matching.
	// For now, we assume SendMessage handles basic wrapping and we match responses sequentially.
	err := c.conn.SendMessage(MethodInitialize, reqParams) // Use MethodInitialize constant
	if err != nil {
		return fmt.Errorf("failed to send InitializeRequest: %w", err)
	}

	// --- Wait for InitializeResponse or Error ---
	log.Println("Waiting for InitializeResponse...")
	msg, err := c.conn.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	// Check for Error message first
	// TODO: Refine error handling to properly parse JSONRPCError structure
	if msg.MessageType == MessageTypeError { // Assuming errors are still sent this way for now
		var errPayload ErrorPayload
		// Attempt to unmarshal the 'error' field if Payload is map/RawMessage
		rawPayload, ok := msg.Payload.(json.RawMessage)
		if !ok {
			// Fallback: try unmarshalling the whole payload if not RawMessage
			payloadBytes, _ := json.Marshal(msg.Payload) // Marshal then unmarshal
			if err := json.Unmarshal(payloadBytes, &errPayload); err != nil {
				return fmt.Errorf("received Error message with non-RawMessage/non-ErrorPayload payload type: %T", msg.Payload)
			}
		} else {
			// It is RawMessage, unmarshal it
			unmarshalErr := UnmarshalPayload(rawPayload, &errPayload)
			if unmarshalErr != nil {
				log.Printf("Failed to unmarshal error payload. Raw: %s", string(rawPayload))
				return fmt.Errorf("received Error message, but failed to unmarshal its payload: %w", unmarshalErr)
			}
		}
		// Use the numeric error code in the error message
		return fmt.Errorf("received MCP Error during initialize: [%d] %s", errPayload.Code, errPayload.Message)
	}

	// --- Process InitializeResponse ---
	// TODO: This check needs refinement. JSON-RPC responses don't have MessageType.
	// We should check if msg.Payload corresponds to an InitializeResult structure.
	// A better approach involves matching the response 'id' to the request 'id'.
	// For now, assume the first non-error response is the InitializeResponse.
	log.Printf("Received potential InitializeResponse message (Payload Type: %T)", msg.Payload)

	var initResult InitializeResult
	// Assume the payload IS the InitializeResult for now
	rawPayload, ok := msg.Payload.(json.RawMessage)
	if !ok {
		// Fallback: try marshalling and unmarshalling if not RawMessage
		payloadBytes, errJson := json.Marshal(msg.Payload)
		if errJson != nil {
			return fmt.Errorf("failed to re-marshal InitializeResponse payload: %w", errJson)
		}
		err = json.Unmarshal(payloadBytes, &initResult)
		if err != nil {
			log.Printf("Failed to unmarshal InitializeResult payload after re-marshal. Raw: %s", string(payloadBytes))
			return fmt.Errorf("failed to unmarshal InitializeResult payload after re-marshal: %w", err)
		}
	} else {
		// It is RawMessage, unmarshal it
		err = UnmarshalPayload(rawPayload, &initResult)
		if err != nil {
			log.Printf("Failed to unmarshal InitializeResult payload. Raw: %s", string(rawPayload))
			return fmt.Errorf("failed to unmarshal InitializeResult payload: %w", err)
		}
	}

	log.Printf("Received InitializeResult: %+v", initResult)

	// Validate selected protocol version
	if initResult.ProtocolVersion != CurrentProtocolVersion {
		// TODO: Handle version negotiation properly if client supports multiple versions
		return fmt.Errorf("server selected unsupported protocol version: %s", initResult.ProtocolVersion)
	}

	// Store server info
	c.serverName = initResult.ServerInfo.Name
	// TODO: Store server capabilities (initResult.Capabilities) for later use

	log.Printf("Initialization successful with server: %s (Version: %s)", c.serverName, initResult.ServerInfo.Version)

	// --- Send Initialized Notification ---
	log.Println("Sending InitializedNotification...")
	initParams := InitializedNotificationParams{}
	// Send notification using the correct method name
	// Note: SendMessage wraps this in a basic Message struct. JSON-RPC notifications don't have 'id'.
	err = c.conn.SendMessage(MethodInitialized, initParams) // Use MethodInitialized constant
	if err != nil {
		// Log warning but don't fail initialization if notification send fails
		log.Printf("Warning: failed to send InitializedNotification: %v", err)
	}

	return nil // Success
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
