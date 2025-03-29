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
	// SendRequest generates and returns the ID. We need to store it to match the response.
	requestID, err := c.conn.SendRequest(MethodInitialize, reqParams) // Use SendRequest
	if err != nil {
		return fmt.Errorf("failed to send InitializeRequest: %w", err)
	}
	// TODO: Store requestID to match with response ID later in ReceiveMessage/ReceiveResponse
	_ = requestID // Avoid unused variable error for now

	// --- Wait for InitializeResponse or Error ---
	log.Println("Waiting for InitializeResponse...")
	// Receive the raw JSON bytes
	rawJSON, err := c.conn.ReceiveRawMessage()
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	// Attempt to unmarshal into a JSONRPCResponse
	var jsonrpcResp JSONRPCResponse
	err = json.Unmarshal(rawJSON, &jsonrpcResp)
	if err != nil {
		// This indicates a fundamental issue with the received message structure
		log.Printf("Failed to unmarshal received message into JSONRPCResponse: %v. Raw: %s", err, string(rawJSON))
		return fmt.Errorf("failed to parse response from server: %w", err)
	}

	// Check if the response ID matches the request ID
	if jsonrpcResp.ID != requestID {
		// This is a protocol violation or a mismatched response
		log.Printf("Received response with mismatched ID. Expected: %v, Got: %v", requestID, jsonrpcResp.ID)
		return fmt.Errorf("received response with mismatched ID (Expected: %v, Got: %v)", requestID, jsonrpcResp.ID)
	}

	// Check if it's an error response
	if jsonrpcResp.Error != nil {
		errPayload := *jsonrpcResp.Error
		log.Printf("Received JSON-RPC Error during initialize: Code=%d, Message=%s", errPayload.Code, errPayload.Message)
		return fmt.Errorf("received MCP Error during initialize: [%d] %s", errPayload.Code, errPayload.Message)
	}

	// --- Process Successful InitializeResponse ---
	// Unmarshal the 'result' field into InitializeResult
	var initResult InitializeResult
	if jsonrpcResp.Result == nil {
		return fmt.Errorf("received successful JSON-RPC response but 'result' field was null")
	}

	// The 'result' field is interface{}, needs marshalling back to bytes then unmarshalling
	resultBytes, err := json.Marshal(jsonrpcResp.Result)
	if err != nil {
		return fmt.Errorf("failed to re-marshal 'result' field: %w", err)
	}
	err = json.Unmarshal(resultBytes, &initResult)
	if err != nil {
		log.Printf("Failed to unmarshal 'result' field into InitializeResult. Raw Result: %s", string(resultBytes))
		return fmt.Errorf("failed to unmarshal InitializeResult from response: %w", err)
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
	err = c.conn.SendNotification(MethodInitialized, initParams) // Use SendNotification
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
