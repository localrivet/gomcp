// client.go (Refactored Connect method)
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Client represents an MCP client instance. It manages the connection to an
// MCP server, handles the handshake/initialization, and provides methods for interacting
// with the server (e.g., requesting tool definitions, using tools - to be added).
type Client struct {
	conn       *Connection // The underlying MCP connection handler
	clientName string      // Name announced during initialization
	serverName string      // Name of the server, discovered during initialization
	// Store capabilities after handshake
	clientCapabilities ClientCapabilities // Capabilities announced by this client
	serverCapabilities ServerCapabilities // Capabilities received from the server

	// For handling concurrent requests/responses
	pendingRequests map[string]chan *JSONRPCResponse // Map request ID to a channel for the response
	pendingMu       sync.Mutex                       // Mutex to protect pendingRequests map

	// For handling server-to-client requests
	requestHandlers map[string]func(id interface{}, params interface{}) error // Map method name to handler
	handlerMu       sync.Mutex                                                // Mutex to protect requestHandlers map

	// For handling server-to-client notifications
	notificationHandlers map[string]func(params interface{}) // Map method name to handler
	notificationMu       sync.Mutex                          // Mutex to protect notificationHandlers map

	// Client-side state
	roots   map[string]Root // Stores client-known roots (URI -> Root)
	rootsMu sync.Mutex      // Mutex to protect roots map
}

// NewClient creates and initializes a new MCP Client instance.
// It configures logging to stderr and sets up the underlying stdio connection.
// The provided clientName is used during the MCP initialization.
func NewClient(clientName string) *Client {
	// Configure logging to stderr for client messages
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile) // Add timestamp and file/line

	return &Client{
		conn:                 NewStdioConnection(), // Assumes stdio for now
		clientName:           clientName,
		pendingRequests:      make(map[string]chan *JSONRPCResponse),                          // Initialize map
		requestHandlers:      make(map[string]func(id interface{}, params interface{}) error), // Initialize map
		notificationHandlers: make(map[string]func(params interface{})),                       // Initialize map
		roots:                make(map[string]Root),                                           // Initialize map
	}
}

// RegisterNotificationHandler registers a handler function for a specific server-to-client notification method.
func (c *Client) RegisterNotificationHandler(method string, handler func(params interface{})) error {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	if _, exists := c.notificationHandlers[method]; exists {
		return fmt.Errorf("notification handler already registered for method: %s", method)
	}
	c.notificationHandlers[method] = handler
	log.Printf("Registered notification handler for method: %s", method)
	return nil
}

// RegisterRequestHandler registers a handler function for a specific server-to-client request method.
func (c *Client) RegisterRequestHandler(method string, handler func(id interface{}, params interface{}) error) error {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	if _, exists := c.requestHandlers[method]; exists {
		return fmt.Errorf("handler already registered for method: %s", method)
	}
	c.requestHandlers[method] = handler
	log.Printf("Registered request handler for method: %s", method)
	return nil
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
	// Note: requestID is implicitly handled by sendRequestAndWait and processIncomingMessages

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

	// Store server info and capabilities
	c.serverName = initResult.ServerInfo.Name
	c.serverCapabilities = initResult.Capabilities
	c.clientCapabilities = clientCapabilities // Store the capabilities we sent

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

	// Start the background message processing loop
	go c.processIncomingMessages()

	return nil // Success
}

// ServerName returns the name of the server as reported during the handshake.
// Returns an empty string if the handshake has not completed successfully or
// if the server did not provide a name.
func (c *Client) ServerName() string {
	return c.serverName
}

// AddRoot adds or updates a root in the client's list and sends a notification if supported.
func (c *Client) AddRoot(root Root) error {
	if root.URI == "" {
		return fmt.Errorf("root URI cannot be empty")
	}
	c.rootsMu.Lock()
	c.roots[root.URI] = root
	c.rootsMu.Unlock()
	log.Printf("Added/Updated root: %s", root.URI)

	// Send notification if client supports it
	// Note: Assumes Connect() has been called and capabilities are populated.
	if c.clientCapabilities.Roots != nil && c.clientCapabilities.Roots.ListChanged {
		log.Printf("Sending roots/list_changed notification to server")
		err := c.SendRootsListChanged()
		if err != nil {
			log.Printf("Warning: failed to send roots/list_changed notification: %v", err)
		}
	}
	return nil
}

// RemoveRoot removes a root from the client's list and sends a notification if supported.
func (c *Client) RemoveRoot(uri string) error {
	if uri == "" {
		return fmt.Errorf("root URI cannot be empty")
	}
	c.rootsMu.Lock()
	_, exists := c.roots[uri]
	if !exists {
		c.rootsMu.Unlock()
		return fmt.Errorf("root '%s' not found", uri)
	}
	delete(c.roots, uri)
	c.rootsMu.Unlock()
	log.Printf("Removed root: %s", uri)

	// Send notification if client supports it
	if c.clientCapabilities.Roots != nil && c.clientCapabilities.Roots.ListChanged {
		log.Printf("Sending roots/list_changed notification to server")
		err := c.SendRootsListChanged()
		if err != nil {
			log.Printf("Warning: failed to send roots/list_changed notification: %v", err)
		}
	}
	return nil
}

// GenerateProgressToken creates a new unique progress token.
// Currently uses UUIDs.
func (c *Client) GenerateProgressToken() ProgressToken {
	// We need to import "github.com/google/uuid"
	// For simplicity, let's assume it's imported (it should be from transport.go usage)
	return ProgressToken(uuid.NewString())
}

// sendRequestAndWait sends a request, registers it, and waits for a response.
// Returns the received JSONRPCResponse or an error (including timeout).
func (c *Client) sendRequestAndWait(method string, params interface{}, timeout time.Duration) (*JSONRPCResponse, error) {
	// 1. Create response channel
	respChan := make(chan *JSONRPCResponse, 1) // Buffered channel

	// 2. Send request and get ID
	requestID, err := c.conn.SendRequest(method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to send request for method %s: %w", method, err)
	}

	// 3. Register pending request
	c.pendingMu.Lock()
	c.pendingRequests[requestID] = respChan
	c.pendingMu.Unlock()

	// 4. Wait for response or timeout
	select {
	case response := <-respChan:
		if response == nil {
			// Channel closed by receiver loop due to error
			return nil, fmt.Errorf("connection closed or error occurred while waiting for response to %s (ID: %s)", method, requestID)
		}
		// Received response
		return response, nil
	case <-time.After(timeout):
		// Timeout occurred
		// Clean up the pending request map
		c.pendingMu.Lock()
		delete(c.pendingRequests, requestID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response to %s (ID: %s)", method, requestID)
	}
}

// ListTools sends a 'tools/list' request and waits for the response.
func (c *Client) ListTools(params ListToolsRequestParams) (*ListToolsResult, error) {
	// Use a default timeout, e.g., 10 seconds
	timeout := 10 * time.Second

	response, err := c.sendRequestAndWait(MethodListTools, params, timeout)
	if err != nil {
		return nil, err // Error includes timeout message
	}

	// Check for JSON-RPC level error
	if response.Error != nil {
		return nil, fmt.Errorf("received MCP Error for ListTools: [%d] %s", response.Error.Code, response.Error.Message)
	}

	// Unmarshal the result
	var listResult ListToolsResult
	if response.Result == nil {
		return nil, fmt.Errorf("received successful ListTools response but 'result' field was null")
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal ListTools 'result' field: %w", err)
	}
	err = json.Unmarshal(resultBytes, &listResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListToolsResult from response: %w", err)
	}

	return &listResult, nil
}

// CallTool sends a 'tools/call' request and waits for the response.
// It optionally includes a progress token in the request's _meta field.
func (c *Client) CallTool(params CallToolParams, progressToken *ProgressToken) (*CallToolResult, error) {
	// Use a default timeout, e.g., 30 seconds (tool calls might take longer)
	timeout := 30 * time.Second

	// Add progress token to meta if provided
	if progressToken != nil {
		if params.Meta == nil {
			params.Meta = &RequestMeta{}
		}
		params.Meta.ProgressToken = progressToken
	}

	response, err := c.sendRequestAndWait(MethodCallTool, params, timeout)
	if err != nil {
		return nil, err // Error includes timeout message
	}

	// Check for JSON-RPC level error
	if response.Error != nil {
		return nil, fmt.Errorf("received MCP Error for CallTool (tool: %s): [%d] %s", params.Name, response.Error.Code, response.Error.Message)
	}

	// Unmarshal the result
	var callResult CallToolResult
	if response.Result == nil {
		// This might be valid if the tool has no output, but spec implies content should be at least []
		// Let's treat null result as an issue for now.
		return nil, fmt.Errorf("received successful CallTool response but 'result' field was null")
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal CallTool 'result' field: %w", err)
	}
	err = json.Unmarshal(resultBytes, &callResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CallToolResult from response: %w", err)
	}

	// Check the business logic error flag within the result
	if callResult.IsError != nil && *callResult.IsError {
		// Extract error message from content (assuming first text content)
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", params.Name)
		if len(callResult.Content) > 0 {
			if textContent, ok := callResult.Content[0].(TextContent); ok { // Use concrete type
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", params.Name, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", params.Name, callResult.Content[0])
			}
		}
		// Return the result along with an error to indicate failure
		return &callResult, fmt.Errorf("%s", errMsg)
	}
	return &callResult, nil // Success
}

// ListResources sends a 'resources/list' request and waits for the response.
func (c *Client) ListResources(params ListResourcesRequestParams) (*ListResourcesResult, error) {
	timeout := 10 * time.Second // Default timeout
	response, err := c.sendRequestAndWait(MethodListResources, params, timeout)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("received MCP Error for ListResources: [%d] %s", response.Error.Code, response.Error.Message)
	}
	var listResult ListResourcesResult
	if response.Result == nil {
		return nil, fmt.Errorf("received successful ListResources response but 'result' field was null")
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal ListResources 'result' field: %w", err)
	}
	err = json.Unmarshal(resultBytes, &listResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListResourcesResult from response: %w", err)
	}
	return &listResult, nil
}

// SubscribeResources sends a 'resources/subscribe' request and waits for the response.
func (c *Client) SubscribeResources(params SubscribeResourceParams) error {
	timeout := 10 * time.Second // Default timeout
	response, err := c.sendRequestAndWait(MethodSubscribeResource, params, timeout)
	if err != nil {
		return err
	}
	if response.Error != nil {
		return fmt.Errorf("received MCP Error for SubscribeResources: [%d] %s", response.Error.Code, response.Error.Message)
	}
	// Success result is empty
	return nil
}

// UnsubscribeResources sends a 'resources/unsubscribe' request and waits for the response.
func (c *Client) UnsubscribeResources(params UnsubscribeResourceParams) error {
	timeout := 10 * time.Second // Default timeout
	response, err := c.sendRequestAndWait(MethodUnsubscribeResource, params, timeout)
	if err != nil {
		return err
	}
	if response.Error != nil {
		return fmt.Errorf("received MCP Error for UnsubscribeResources: [%d] %s", response.Error.Code, response.Error.Message)
	}
	// Success result is empty
	return nil
}

// ReadResource sends a 'resources/read' request and waits for the response.
func (c *Client) ReadResource(params ReadResourceRequestParams) (*ReadResourceResult, error) {
	timeout := 15 * time.Second // Potentially longer timeout for reading
	response, err := c.sendRequestAndWait(MethodReadResource, params, timeout)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("received MCP Error for ReadResource (URI: %s): [%d] %s", params.URI, response.Error.Code, response.Error.Message)
	}
	var readResult ReadResourceResult
	if response.Result == nil {
		return nil, fmt.Errorf("received successful ReadResource response but 'result' field was null")
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal ReadResource 'result' field: %w", err)
	}
	// Need custom unmarshalling for ResourceContents interface
	// First, unmarshal into a temporary struct with RawMessage for contents
	var tempResult struct {
		Resource Resource        `json:"resource"`
		Contents json.RawMessage `json:"contents"`
	}
	err = json.Unmarshal(resultBytes, &tempResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ReadResource temporary result: %w", err)
	}
	readResult.Resource = tempResult.Resource

	// Now determine the type of contents based on a field (e.g., contentType)
	var contentType struct {
		ContentType string `json:"contentType"`
	}
	err = json.Unmarshal(tempResult.Contents, &contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal contentType from ReadResource contents: %w", err)
	}

	// Unmarshal into the correct concrete type based on contentType
	// This assumes Text or Blob based on common prefixes. A more robust solution
	// might inspect the 'type' field if MCP adds one to ResourceContents.
	if strings.HasPrefix(contentType.ContentType, "text/") || strings.Contains(contentType.ContentType, "json") || strings.Contains(contentType.ContentType, "xml") {
		var textContents TextResourceContents
		err = json.Unmarshal(tempResult.Contents, &textContents)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal TextResourceContents: %w", err)
		}
		readResult.Contents = textContents
	} else { // Assume blob otherwise
		var blobContents BlobResourceContents
		err = json.Unmarshal(tempResult.Contents, &blobContents)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal BlobResourceContents: %w", err)
		}
		readResult.Contents = blobContents
	}
	return &readResult, nil
}

// ListPrompts sends a 'prompts/list' request and waits for the response.
func (c *Client) ListPrompts(params ListPromptsRequestParams) (*ListPromptsResult, error) {
	timeout := 10 * time.Second // Default timeout
	response, err := c.sendRequestAndWait(MethodListPrompts, params, timeout)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("received MCP Error for ListPrompts: [%d] %s", response.Error.Code, response.Error.Message)
	}
	var listResult ListPromptsResult
	if response.Result == nil {
		return nil, fmt.Errorf("received successful ListPrompts response but 'result' field was null")
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal ListPrompts 'result' field: %w", err)
	}
	err = json.Unmarshal(resultBytes, &listResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListPromptsResult from response: %w", err)
	}
	return &listResult, nil
}

// GetPrompt sends a 'prompts/get' request and waits for the response.
func (c *Client) GetPrompt(params GetPromptRequestParams) (*GetPromptResult, error) {
	timeout := 10 * time.Second // Default timeout
	response, err := c.sendRequestAndWait(MethodGetPrompt, params, timeout)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("received MCP Error for GetPrompt (URI: %s): [%d] %s", params.URI, response.Error.Code, response.Error.Message)
	}
	var getResult GetPromptResult
	if response.Result == nil {
		return nil, fmt.Errorf("received successful GetPrompt response but 'result' field was null")
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal GetPrompt 'result' field: %w", err)
	}
	err = json.Unmarshal(resultBytes, &getResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal GetPromptResult from response: %w", err)
	}
	return &getResult, nil
}

// Ping sends a 'ping' request and waits for the (empty) response.
// Returns an error if the ping fails or times out.
func (c *Client) Ping(timeout time.Duration) error {
	response, err := c.sendRequestAndWait(MethodPing, nil, timeout) // Ping has nil params
	if err != nil {
		return err // Error includes timeout message
	}
	// Check for JSON-RPC level error
	if response.Error != nil {
		return fmt.Errorf("received MCP Error for Ping: [%d] %s", response.Error.Code, response.Error.Message)
	}
	// Success (result should be null or absent, which we don't need to check explicitly here)
	return nil
}

// SendCancellation sends a '$/cancelled' notification to the server.
func (c *Client) SendCancellation(params CancelledParams) error {
	return c.conn.SendNotification(MethodCancelled, params)
}

// SendProgress sends a '$/progress' notification to the server.
func (c *Client) SendProgress(params ProgressParams) error {
	return c.conn.SendNotification(MethodProgress, params)
}

// SendRootsListChanged sends a 'notifications/roots/list_changed' notification to the server.
func (c *Client) SendRootsListChanged() error {
	return c.conn.SendNotification(MethodNotifyRootsListChanged, RootsListChangedParams{})
}

// processIncomingMessages runs in a separate goroutine to handle responses and notifications.
func (c *Client) processIncomingMessages() { // Add missing function signature
	log.Println("Client message processing loop started.")
	defer log.Println("Client message processing loop stopped.")

	for {
		rawJSON, err := c.conn.ReceiveRawMessage()
		if err != nil {
			log.Printf("Error receiving message in client loop: %v. Exiting loop.", err)
			// TODO: How to signal this error back to the main client logic or pending requests?
			// Maybe close all pending channels with an error?
			c.closePendingRequests(err)
			return
		}

		// Attempt to unmarshal into a generic structure to determine type
		var baseMessage struct {
			JSONRPC string        `json:"jsonrpc"`
			ID      interface{}   `json:"id"`     // Present in responses
			Method  string        `json:"method"` // Present in notifications/requests
			Result  interface{}   `json:"result"`
			Error   *ErrorPayload `json:"error"`
			Params  interface{}   `json:"params"`
		}
		err = json.Unmarshal(rawJSON, &baseMessage)
		if err != nil {
			log.Printf("Client failed to unmarshal base JSON-RPC structure: %v. Raw: %s", err, string(rawJSON))
			// Cannot send error back as we don't know the context/ID
			continue // Skip malformed message
		}

		if baseMessage.ID != nil { // It's a Response
			// Convert ID to string for map lookup
			idStr := fmt.Sprintf("%v", baseMessage.ID)

			c.pendingMu.Lock()
			respChan, ok := c.pendingRequests[idStr]
			if ok {
				// Found pending request, remove it from map
				delete(c.pendingRequests, idStr)
				c.pendingMu.Unlock()

				// Send the full response back to the waiting caller
				// Re-marshal into JSONRPCResponse to ensure correct structure
				var jsonrpcResp JSONRPCResponse
				if err := json.Unmarshal(rawJSON, &jsonrpcResp); err == nil {
					select {
					case respChan <- &jsonrpcResp:
						// Response sent successfully
					default:
						// Channel likely closed by timeout on caller side
						log.Printf("Warning: Response channel for request ID %s closed before response could be sent.", idStr)
					}
				} else {
					log.Printf("Error re-marshalling JSONRPCResponse for ID %s: %v", idStr, err)
					// How to signal this error back? Close channel?
					close(respChan) // Signal error by closing channel
				}

			} else {
				c.pendingMu.Unlock()
				log.Printf("Warning: Received response for unknown or timed-out request ID: %v", baseMessage.ID)
			}

		} else if baseMessage.Method != "" { // It's a Notification or a Request from server
			if baseMessage.ID == nil { // It's a Notification
				log.Printf("Client received notification: Method=%s", baseMessage.Method)
				// --- Dispatch server-to-client notifications ---
				c.notificationMu.Lock()
				handler, ok := c.notificationHandlers[baseMessage.Method]
				c.notificationMu.Unlock()

				if ok {
					// Run handler in a new goroutine to avoid blocking the receive loop
					go func(params interface{}) {
						// TODO: Consider adding error handling/logging for notification handlers
						handler(params)
					}(baseMessage.Params)
				} else {
					log.Printf("No handler registered for notification method: %s", baseMessage.Method)
				}
			} else { // It's a Request from the server
				log.Printf("Client received request from server: Method=%s, ID=%v", baseMessage.Method, baseMessage.ID)

				// --- Dispatch server-to-client requests ---
				c.handlerMu.Lock()
				handler, handlerRegistered := c.requestHandlers[baseMessage.Method]
				c.handlerMu.Unlock()

				if handlerRegistered {
					// Run registered handler in a new goroutine
					go func(id interface{}, params interface{}) {
						err := handler(id, params)
						if err != nil {
							// If handler returns error, send JSON-RPC error response
							log.Printf("Error executing handler for method %s (ID: %v): %v", baseMessage.Method, id, err)
							// Check if the returned error is an *MCPError
							var mcpErr *MCPError
							if errors.As(err, &mcpErr) {
								// If it is, send its embedded ErrorPayload
								_ = c.conn.SendErrorResponse(id, mcpErr.ErrorPayload)
							} else {
								// Otherwise, wrap the generic error message
								_ = c.conn.SendErrorResponse(id, ErrorPayload{
									Code:    ErrorCodeInternalError, // Generic internal error
									Message: fmt.Sprintf("Handler error: %v", err),
								})
							}
						}
						// If handler returns nil, assume it sent the response itself (e.g., for sampling/roots)
					}(baseMessage.ID, baseMessage.Params)
				} else {
					// No registered handler, check for built-in handlers or send error
					switch baseMessage.Method {
					case MethodRootsList:
						// Handle roots/list internally by returning the client's known roots.
						// TODO: Allow overriding this with a registered handler?
						log.Printf("Received roots/list request from server.")
						c.rootsMu.Lock()
						roots := make([]Root, 0, len(c.roots))
						for _, root := range c.roots {
							roots = append(roots, root)
						}
						c.rootsMu.Unlock()
						log.Printf("Sending roots/list response with %d roots.", len(roots))
						_ = c.conn.SendResponse(baseMessage.ID, ListRootsResult{Roots: roots})
						// Note: We don't set handlerErr here as SendResponse handles its own errors internally
					default:
						// No handler registered and not a built-in method, send MethodNotFound error
						log.Printf("No handler registered for server request method: %s", baseMessage.Method)
						err := c.conn.SendErrorResponse(baseMessage.ID, ErrorPayload{
							Code:    ErrorCodeMethodNotFound,
							Message: fmt.Sprintf("Client does not handle request method: %s", baseMessage.Method),
						})
						if err != nil {
							log.Printf("Error sending MethodNotFound error response to server: %v", err)
						}
					}
				}
			}
		} else {
			log.Printf("Warning: Received message with no ID or Method. Raw: %s", string(rawJSON))
		}
	}
}

// closePendingRequests closes all pending request channels, typically with an error.
// This should be called when the connection is lost.
func (c *Client) closePendingRequests(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	log.Printf("Closing %d pending request channels due to error: %v", len(c.pendingRequests), err)
	for id, ch := range c.pendingRequests {
		// Optionally send an error indication through the channel if possible,
		// but closing is the primary signal.
		close(ch)
		delete(c.pendingRequests, id) // Clean up map
	}
}

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

// Duplicates removed below
// Duplicates removed below
