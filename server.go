// server.go (Refactored for Multi-Transport)
package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os" // Needed for log.SetOutput
	"sync"
)

// ToolHandlerFunc defines the signature for functions that handle tool execution.
// It receives the arguments provided by the client and should return the result
// content and a boolean indicating if an error occurred. It also receives a context.Context
// (for cancellation) and an optional ProgressToken (which may be nil).
type ToolHandlerFunc func(ctx context.Context, progressToken *ProgressToken, arguments map[string]interface{}) (content []Content, isError bool)

// Server represents the core MCP server logic, independent of the transport layer.
// It handles initialization, tool/resource/prompt registration, and processes incoming messages.
type Server struct {
	transport        Transport // Interface to the communication layer
	serverName       string
	toolRegistry     map[string]Tool            // Stores tool definitions (now using Tool struct)
	toolHandlers     map[string]ToolHandlerFunc // Stores handlers for each tool
	resourceRegistry map[string]Resource        // Stores available resources (URI -> Resource)
	promptRegistry   map[string]Prompt          // Stores available prompts (URI -> Prompt)
	// Store client/server capabilities after handshake (TODO: How to manage this per-session?)
	clientCapabilities ClientCapabilities // Capabilities supported by the connected client
	serverCapabilities ServerCapabilities // Capabilities supported by this server

	// For handling client-to-server notifications
	notificationHandlers map[string]func(params interface{}) // Map method name to handler
	notificationMu       sync.Mutex                          // Mutex to protect notificationHandlers map

	// For managing cancellation of active requests
	activeRequests map[string]context.CancelFunc // Map request ID (as string) to its cancel function
	requestMu      sync.Mutex                    // Mutex to protect activeRequests map

	// For managing resource subscriptions (URI -> subscribed?)
	// TODO: This needs to be per-session for multi-transport.
	resourceSubscriptions map[string]bool // Map resource URI to subscription status
	subscriptionMu        sync.Mutex      // Mutex to protect resourceSubscriptions map
}

// NewServer creates and initializes a new core MCP Server instance.
// The transport layer must be registered separately using RegisterTransport.
func NewServer(serverName string) *Server {
	log.SetOutput(os.Stderr) // Configure logging
	log.SetFlags(log.Ltime | log.Lshortfile)

	srv := &Server{
		transport:             nil, // Transport must be registered later
		serverName:            serverName,
		toolRegistry:          make(map[string]Tool),
		toolHandlers:          make(map[string]ToolHandlerFunc),
		resourceRegistry:      make(map[string]Resource),
		promptRegistry:        make(map[string]Prompt),
		notificationHandlers:  make(map[string]func(params interface{})),
		activeRequests:        make(map[string]context.CancelFunc),
		resourceSubscriptions: make(map[string]bool), // TODO: Needs per-session handling
	}
	// Register the built-in handler for cancellation notifications
	// Pass a function literal that calls the method on the created server instance (srv)
	srv.RegisterNotificationHandler(MethodCancelled, func(params interface{}) {
		srv.handleCancellationNotification(params)
	})
	return srv
}

// RegisterTransport associates a transport layer implementation with the server.
func (s *Server) RegisterTransport(t Transport) error {
	if s.transport != nil {
		return fmt.Errorf("transport already registered for this server")
	}
	s.transport = t
	// The transport should register the server back
	t.RegisterServer(s)
	log.Println("Transport registered successfully.")
	return nil
}

// RegisterNotificationHandler registers a handler function for a specific client-to-server notification method.
func (s *Server) RegisterNotificationHandler(method string, handler func(params interface{})) error {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()
	if _, exists := s.notificationHandlers[method]; exists {
		return fmt.Errorf("notification handler already registered for method: %s", method)
	}
	s.notificationHandlers[method] = handler
	log.Printf("Registered notification handler for method: %s", method)
	return nil
}

// RegisterTool adds a tool definition and its corresponding handler function to the server.
func (s *Server) RegisterTool(tool Tool, handler ToolHandlerFunc) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := s.toolRegistry[tool.Name]; exists {
		return fmt.Errorf("tool '%s' already registered", tool.Name)
	}
	if handler == nil {
		return fmt.Errorf("handler for tool '%s' cannot be nil", tool.Name)
	}
	s.toolRegistry[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
	log.Printf("Registered tool: %s", tool.Name)

	// TODO: Refactor notification sending for multi-transport
	// if s.serverCapabilities.Tools != nil && s.serverCapabilities.Tools.ListChanged {
	// 	log.Printf("Sending tools/list_changed notification to client")
	// 	err := s.SendToolsListChanged()
	// 	if err != nil {
	// 		log.Printf("Warning: failed to send tools/list_changed notification: %v", err)
	// 	}
	// }
	return nil
}

// RegisterResource adds or updates a resource in the server's registry.
func (s *Server) RegisterResource(resource Resource) error {
	if resource.URI == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	s.subscriptionMu.Lock() // Assuming resource registry and subscriptions might be related
	s.resourceRegistry[resource.URI] = resource
	s.subscriptionMu.Unlock()
	log.Printf("Registered/Updated resource: %s", resource.URI)

	// TODO: Refactor notification sending for multi-transport
	// if s.serverCapabilities.Resources != nil && s.serverCapabilities.Resources.ListChanged {
	// 	log.Printf("Sending resources/list_changed notification to client")
	// 	err := s.SendResourcesListChanged()
	// 	if err != nil {
	// 		log.Printf("Warning: failed to send resources/list_changed notification: %v", err)
	// 	}
	// }
	return nil
}

// UnregisterResource removes a resource from the server's registry.
func (s *Server) UnregisterResource(uri string) error {
	if uri == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}
	s.subscriptionMu.Lock()
	_, exists := s.resourceRegistry[uri]
	if !exists {
		s.subscriptionMu.Unlock()
		return fmt.Errorf("resource '%s' not found", uri)
	}
	delete(s.resourceRegistry, uri)
	delete(s.resourceSubscriptions, uri) // Also remove subscriptions
	s.subscriptionMu.Unlock()
	log.Printf("Unregistered resource: %s", uri)

	// TODO: Refactor notification sending for multi-transport
	// if s.serverCapabilities.Resources != nil && s.serverCapabilities.Resources.ListChanged {
	// 	log.Printf("Sending resources/list_changed notification to client")
	// 	err := s.SendResourcesListChanged()
	// 	if err != nil {
	// 		log.Printf("Warning: failed to send resources/list_changed notification: %v", err)
	// 	}
	// }
	return nil
}

// ResourceRegistry returns a read-only copy of the current resource registry.
func (s *Server) ResourceRegistry() map[string]Resource {
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()
	registryCopy := make(map[string]Resource, len(s.resourceRegistry))
	for k, v := range s.resourceRegistry {
		registryCopy[k] = v
	}
	return registryCopy
}

// RegisterPrompt adds or updates a prompt in the server's registry.
func (s *Server) RegisterPrompt(prompt Prompt) error {
	if prompt.URI == "" {
		return fmt.Errorf("prompt URI cannot be empty")
	}
	// TODO: Add locking if needed
	s.promptRegistry[prompt.URI] = prompt
	log.Printf("Registered/Updated prompt: %s", prompt.URI)

	// TODO: Refactor notification sending for multi-transport
	// if s.serverCapabilities.Prompts != nil && s.serverCapabilities.Prompts.ListChanged {
	// 	log.Printf("Sending prompts/list_changed notification to client")
	// 	err := s.SendPromptsListChanged()
	// 	if err != nil {
	// 		log.Printf("Warning: failed to send prompts/list_changed notification: %v", err)
	// 	}
	// }
	return nil
}

// UnregisterPrompt removes a prompt from the server's registry.
func (s *Server) UnregisterPrompt(uri string) error {
	if uri == "" {
		return fmt.Errorf("prompt URI cannot be empty")
	}
	// TODO: Add locking if needed
	_, exists := s.promptRegistry[uri]
	if !exists {
		return fmt.Errorf("prompt '%s' not found", uri)
	}
	delete(s.promptRegistry, uri)
	log.Printf("Unregistered prompt: %s", uri)

	// TODO: Refactor notification sending for multi-transport
	// if s.serverCapabilities.Prompts != nil && s.serverCapabilities.Prompts.ListChanged {
	// 	log.Printf("Sending prompts/list_changed notification to client")
	// 	err := s.SendPromptsListChanged()
	// 	if err != nil {
	// 		log.Printf("Warning: failed to send prompts/list_changed notification: %v", err)
	// 	}
	// }
	return nil
}

// ProcessRequest handles an incoming raw message from a specific session.
// It unmarshals the message, determines if it's a request or notification,
// dispatches it to the appropriate handler, and sends back a response if required.
// This method is intended to be called by the transport layer.
func (s *Server) ProcessRequest(sessionCtx context.Context, sessionID string, rawMessage []byte) error {
	// Attempt to unmarshal into a generic structure to determine type (Req/Notif)
	var baseMessage struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`     // Present in requests, missing in notifications
		Method  string      `json:"method"` // Present in requests and notifications
		Params  interface{} `json:"params"` // Present in requests and notifications
		Result  interface{} `json:"result"` // Present in responses (not expected from client)
		Error   interface{} `json:"error"`  // Present in error responses (not expected from client)
	}
	err := json.Unmarshal(rawMessage, &baseMessage)
	if err != nil {
		log.Printf("Session %s: Failed to unmarshal base JSON-RPC structure: %v. Raw: %s", sessionID, err, string(rawMessage))
		// Send error response via transport
		errPayload := ErrorPayload{Code: ErrorCodeParseError, Message: fmt.Sprintf("Failed to parse JSON: %v", err)}
		responseBytes, _ := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: nil, Error: &errPayload}) // ID is null for parse error before ID is known
		return s.transport.SendMessage(sessionID, responseBytes)
	}

	// Basic validation
	if baseMessage.JSONRPC != "2.0" {
		log.Printf("Session %s: Received message with invalid jsonrpc version: %s", sessionID, baseMessage.JSONRPC)
		errPayload := ErrorPayload{Code: ErrorCodeInvalidRequest, Message: "Invalid jsonrpc version"}
		responseBytes, _ := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: baseMessage.ID, Error: &errPayload})
		return s.transport.SendMessage(sessionID, responseBytes)
	}

	// Determine if it's a Request or Notification based on ID presence
	isRequest := baseMessage.ID != nil
	isNotification := baseMessage.ID == nil && baseMessage.Method != ""

	if !isRequest && !isNotification {
		log.Printf("Session %s: Received message that is neither a valid Request nor Notification. Raw: %s", sessionID, string(rawMessage))
		errPayload := ErrorPayload{Code: ErrorCodeInvalidRequest, Message: "Invalid message structure"}
		responseBytes, _ := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: baseMessage.ID, Error: &errPayload})
		return s.transport.SendMessage(sessionID, responseBytes)
	}

	if isNotification {
		// --- Dispatch client-to-server notifications ---
		log.Printf("Session %s: Received notification: Method=%s", sessionID, baseMessage.Method)
		s.notificationMu.Lock()
		handler, ok := s.notificationHandlers[baseMessage.Method]
		s.notificationMu.Unlock()

		if ok {
			// Run handler in a new goroutine to avoid blocking the message processing
			go func(p interface{}) {
				// TODO: Consider adding error handling/logging for notification handlers
				// Use sessionCtx for the handler's context?
				handler(p)
			}(baseMessage.Params)
		} else {
			log.Printf("Session %s: No handler registered for notification method: %s", sessionID, baseMessage.Method)
		}
		return nil // No response needed for notifications
	}

	// --- Dispatch client-to-server requests ---
	log.Printf("Session %s: Received request: Method=%s, ID=%v", sessionID, baseMessage.Method, baseMessage.ID)
	var handlerErr error           // Error returned *by* the handler (e.g., internal error), distinct from response sending errors
	var resultPayload interface{}  // Payload returned by successful handler execution
	var errorPayload *ErrorPayload // Error payload returned by handler or generated here

	// Dispatch based on message method
	switch baseMessage.Method {
	case MethodInitialize:
		// Initialization needs special handling as it sets up capabilities
		// It should likely be handled by the transport layer before general message processing.
		log.Printf("Session %s: Received Initialize request after connection established. This should be handled by the transport.", sessionID)
		errorPayload = &ErrorPayload{Code: ErrorCodeInvalidRequest, Message: "Initialize method cannot be called after connection setup"}

	case MethodListTools:
		resultPayload, errorPayload = s.handleListToolsRequest(baseMessage.ID, baseMessage.Params)

	case MethodCallTool:
		resultPayload, errorPayload = s.handleCallToolRequest(sessionCtx, baseMessage.ID, baseMessage.Params)

	case MethodListResources:
		resultPayload, errorPayload = s.handleListResources(baseMessage.ID, baseMessage.Params)

	case MethodReadResource:
		resultPayload, errorPayload = s.handleReadResource(baseMessage.ID, baseMessage.Params)

	case MethodListPrompts:
		resultPayload, errorPayload = s.handleListPrompts(baseMessage.ID, baseMessage.Params)

	case MethodGetPrompt:
		resultPayload, errorPayload = s.handleGetPrompt(baseMessage.ID, baseMessage.Params)

	case MethodLoggingSetLevel:
		resultPayload, errorPayload = s.handleLoggingSetLevel(baseMessage.ID, baseMessage.Params)

	case MethodPing:
		resultPayload, errorPayload = s.handlePing(baseMessage.ID, baseMessage.Params)

	case MethodSubscribeResource:
		resultPayload, errorPayload = s.handleSubscribeResource(baseMessage.ID, baseMessage.Params)

	case MethodUnsubscribeResource:
		resultPayload, errorPayload = s.handleUnsubscribeResource(baseMessage.ID, baseMessage.Params)

	default:
		// Handle unknown methods
		log.Printf("Session %s: Method not implemented: %s", sessionID, baseMessage.Method)
		errorPayload = &ErrorPayload{
			Code:    ErrorCodeMethodNotFound,
			Message: fmt.Sprintf("Method '%s' not implemented by server", baseMessage.Method),
		}
	}

	// If the handler itself returned an error (e.g., internal logic error), it should be in errorPayload
	if handlerErr != nil {
		// This case should ideally not happen if handlers only return payloads
		log.Printf("Session %s: Unexpected internal error during handler execution for method %s (ID: %v): %v", sessionID, baseMessage.Method, baseMessage.ID, handlerErr)
		// Send a generic internal error
		errorPayload = &ErrorPayload{Code: ErrorCodeInternalError, Message: "Internal server error during request handling"}
	}

	// --- Send Response/Error via Transport ---
	if errorPayload != nil {
		// Send error response
		responseBytes, err := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: baseMessage.ID, Error: errorPayload})
		if err != nil {
			log.Printf("Session %s: Failed to marshal error response: %v", sessionID, err)
			return fmt.Errorf("failed to marshal error response: %w", err) // Internal server error
		}
		return s.transport.SendMessage(sessionID, responseBytes)
	} else if isRequest { // Only send success responses for requests
		// Send success response
		responseBytes, err := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: baseMessage.ID, Result: resultPayload})
		if err != nil {
			log.Printf("Session %s: Failed to marshal success response: %v", sessionID, err)
			// Attempt to send an internal error response back to the client
			errPayload := ErrorPayload{Code: ErrorCodeInternalError, Message: "Failed to marshal success response"}
			errorBytes, _ := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: baseMessage.ID, Error: &errPayload})
			_ = s.transport.SendMessage(sessionID, errorBytes)               // Best effort
			return fmt.Errorf("failed to marshal success response: %w", err) // Return internal error
		}
		return s.transport.SendMessage(sessionID, responseBytes)
	}

	// If it was a notification, return nil
	return nil
}

// handleListToolsRequest prepares the result payload for a 'tools/list' request.
// It returns the result payload (ListToolsResult) or an error payload.
func (s *Server) handleListToolsRequest(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling ListToolsRequest")
	// TODO: Unmarshal params into ListToolsRequestParams if pagination/filtering is needed

	tools := make([]Tool, 0, len(s.toolRegistry))
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	resultPayload := ListToolsResult{Tools: tools}
	log.Printf("Prepared ListToolsResponse with %d tools", len(tools))
	return resultPayload, nil // Return result payload, no error
}

// handleListPrompts prepares the result payload for a 'prompts/list' request.
// TODO: Implement pagination/filtering based on params.
func (s *Server) handleListPrompts(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling ListPromptsRequest")
	// TODO: Unmarshal params ListPromptsRequestParams for pagination/filtering
	// TODO: Add locking if registry access needs to be thread-safe
	prompts := make([]Prompt, 0, len(s.promptRegistry))
	for _, prompt := range s.promptRegistry {
		prompts = append(prompts, prompt)
	}
	resultPayload := ListPromptsResult{
		Prompts: prompts,
		// TODO: Add NextCursor for pagination
	}
	log.Printf("Prepared ListPromptsResponse with %d prompts", len(prompts))
	return resultPayload, nil // Return result payload, no error
}

// handleGetPrompt prepares the result or error payload for a 'prompts/get' request.
// TODO: Implement actual prompt retrieval from s.promptRegistry.
func (s *Server) handleGetPrompt(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling GetPromptRequest (stub)")
	var requestParams GetPromptRequestParams

	// Unmarshal and validate params
	if params == nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: "Missing params for prompts/get"}
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal GetPrompt params: %v", err)}
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal GetPrompt params: %v", err)}
	}

	// TODO: Implement actual lookup in s.promptRegistry
	// prompt, exists := s.promptRegistry[requestParams.URI]
	// if !exists { ... return error payload ... }
	// return prompt, nil

	// For now, always return not found error payload
	log.Printf("GetPrompt requested for URI: %s (returning Not Found)", requestParams.URI)
	return nil, &ErrorPayload{
		Code:    ErrorCodeMCPResourceNotFound, // Using ResourceNotFound for now
		Message: fmt.Sprintf("Prompt not found (stub implementation): %s", requestParams.URI),
	}
}

// handleListResources prepares the result payload for a 'resources/list' request.
// TODO: Implement pagination/filtering based on params.
func (s *Server) handleListResources(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling ListResourcesRequest")
	// TODO: Unmarshal params ListResourcesRequestParams for pagination/filtering
	// TODO: Add locking if registry access needs to be thread-safe (using existing subscriptionMu for now)
	s.subscriptionMu.Lock()
	resources := make([]Resource, 0, len(s.resourceRegistry))
	for _, resource := range s.resourceRegistry {
		resources = append(resources, resource)
	}
	s.subscriptionMu.Unlock()

	resultPayload := ListResourcesResult{
		Resources: resources,
		// TODO: Add NextCursor for pagination
	}
	log.Printf("Prepared ListResourcesResponse with %d resources", len(resources))
	return resultPayload, nil // Return result payload, no error
}

// handleLoggingSetLevel prepares the result or error payload for a 'logging/set_level' request.
// TODO: Implement actual level setting and potentially use it to filter server-sent logs.
func (s *Server) handleLoggingSetLevel(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling LoggingSetLevelRequest (stub)")
	var requestParams SetLevelRequestParams

	// Unmarshal and validate params
	if params == nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: "Missing params for logging/set_level"}
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal SetLevel params: %v", err)}
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal SetLevel params: %v", err)}
	}

	// TODO: Store requestParams.Level on the server or connection state
	log.Printf("Client requested logging level: %s (not yet implemented)", requestParams.Level)

	// Return empty successful result payload
	return nil, nil
}

// handlePing prepares the result payload for a 'ping' request.
func (s *Server) handlePing(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling Ping request")
	// Ping request has no params and expects an empty result on success
	return nil, nil // Return nil result, no error
}

// handleSubscribeResource prepares the result or error payload for a 'resources/subscribe' request.
// TODO: Implement actual subscription logic (store subscriptions).
func (s *Server) handleSubscribeResource(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling SubscribeResource request (stub)")
	var requestParams SubscribeResourceParams
	err := UnmarshalPayload(params, &requestParams)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal SubscribeResource params: %v", err)}
	}
	log.Printf("Client requested subscription to URIs: %v", requestParams.URIs)

	s.subscriptionMu.Lock()
	if len(requestParams.URIs) == 0 {
		// Empty list means unsubscribe all (clear the map)
		log.Println("Unsubscribing from all resources.")
		s.resourceSubscriptions = make(map[string]bool)
	} else {
		for _, uri := range requestParams.URIs {
			// Check if resource actually exists in registry? Spec doesn't explicitly require it.
			// _, exists := s.resourceRegistry[uri]
			// if !exists { log.Printf("Warning: Client subscribed to non-existent resource URI: %s", uri) }
			s.resourceSubscriptions[uri] = true
			log.Printf("Subscribed to resource: %s", uri)
		}
	}
	s.subscriptionMu.Unlock()

	return SubscribeResourceResult{}, nil // Return empty result, no error
}

// handleUnsubscribeResource prepares the result or error payload for a 'resources/unsubscribe' request.
// TODO: Implement actual unsubscription logic (remove subscriptions).
func (s *Server) handleUnsubscribeResource(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling UnsubscribeResource request (stub)")
	var requestParams UnsubscribeResourceParams
	err := UnmarshalPayload(params, &requestParams)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal UnsubscribeResource params: %v", err)}
	}
	log.Printf("Client requested unsubscription from URIs: %v", requestParams.URIs)

	s.subscriptionMu.Lock()
	if len(requestParams.URIs) == 0 {
		// Spec doesn't define unsubscribe with empty list, but arguably should do nothing.
		log.Println("Received unsubscribe request with empty URI list, doing nothing.")
	} else {
		for _, uri := range requestParams.URIs {
			delete(s.resourceSubscriptions, uri)
			log.Printf("Unsubscribed from resource: %s", uri)
		}
	}
	s.subscriptionMu.Unlock()

	return UnsubscribeResourceResult{}, nil // Return empty result, no error
}

// handleCancellationNotification handles the '$/cancelled' notification from the client.
func (s *Server) handleCancellationNotification(params interface{}) {
	var cancelParams CancelledParams
	err := UnmarshalPayload(params, &cancelParams)
	if err != nil {
		log.Printf("Error unmarshalling $/cancelled params: %v", err)
		return
	}

	if cancelParams.ID == nil {
		log.Printf("Received $/cancelled notification with nil ID.")
		return
	}

	requestIDStr := fmt.Sprintf("%v", cancelParams.ID)
	log.Printf("Received cancellation request for ID: %s", requestIDStr)

	s.requestMu.Lock()
	cancelFunc, ok := s.activeRequests[requestIDStr]
	// It's okay if the request is not found, it might have already finished.
	// Remove it from the map regardless, to prevent potential leaks if cancellation
	// arrives after the defer in handleCallToolRequest but before it finishes execution.
	delete(s.activeRequests, requestIDStr)
	s.requestMu.Unlock()

	if ok {
		log.Printf("Found active request %s, calling cancel function.", requestIDStr)
		cancelFunc() // Call the context cancel function
	} else {
		log.Printf("No active request found for cancellation ID: %s (might have already completed)", requestIDStr)
	}
}

// NotifyResourceUpdated informs the library that a resource has changed.
// The library will then send 'notifications/resources/changed' to subscribed clients.
// The provided 'resource' struct MUST contain the updated version string.
func (s *Server) NotifyResourceUpdated(resource Resource) {
	if resource.URI == "" {
		log.Printf("Warning: NotifyResourceUpdated called with empty URI.")
		return
	}

	// Check if the client is actually subscribed to this resource
	s.subscriptionMu.Lock()
	subscribed, ok := s.resourceSubscriptions[resource.URI]
	s.subscriptionMu.Unlock()

	if !ok || !subscribed {
		// Client is not subscribed, do nothing
		return
	}

	// Send the notification
	log.Printf("Resource %s updated, sending notification.", resource.URI)
	params := ResourceUpdatedParams{ // Use renamed struct
		Resource: resource,
	}
	err := s.SendResourceUpdated(params) // Use renamed method
	if err != nil {
		log.Printf("Warning: failed to send resources/updated notification for URI %s: %v", resource.URI, err)
	}
}

// SendCancellation sends a '$/cancelled' notification to the client.
// TODO: Needs sessionID for targeted sending.
func (s *Server) SendCancellation(params CancelledParams) error {
	return s.sendNotification(MethodCancelled, params)
}

// SendProgress sends a '$/progress' notification to the client.
// TODO: Needs sessionID for targeted sending based on progress token/request ID.
func (s *Server) SendProgress(params ProgressParams) error {
	return s.sendNotification(MethodProgress, params)
}

// SendResourceUpdated sends a 'notifications/resources/updated' notification.
// TODO: Needs sessionID for targeted sending based on subscription.
func (s *Server) SendResourceUpdated(params ResourceUpdatedParams) error {
	return s.sendNotification(MethodNotifyResourceUpdated, params)
}

// SendToolsListChanged sends a 'notifications/tools/list_changed' notification.
// TODO: This should likely broadcast via transport.
func (s *Server) SendToolsListChanged() error {
	return s.sendNotification(MethodNotifyToolsListChanged, ToolsListChangedParams{})
}

// SendResourcesListChanged sends a 'notifications/resources/list_changed' notification.
// TODO: This should likely broadcast via transport.
func (s *Server) SendResourcesListChanged() error {
	return s.sendNotification(MethodNotifyResourcesListChanged, ResourcesListChangedParams{})
}

// SendPromptsListChanged sends a 'notifications/prompts/list_changed' notification.
// TODO: This should likely broadcast via transport.
func (s *Server) SendPromptsListChanged() error {
	return s.sendNotification(MethodNotifyPromptsListChanged, PromptsListChangedParams{})
}

// handleReadResource prepares the result or error payload for a 'resources/read' request.
// TODO: Implement actual resource reading from s.resourceRegistry or elsewhere.
func (s *Server) handleReadResource(requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling ReadResourceRequest (stub)")
	var requestParams ReadResourceRequestParams

	// Unmarshal and validate params
	if params == nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: "Missing params for resources/read"}
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal ReadResource params: %v", err)}
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal ReadResource params: %v", err)}
	}

	// TODO: Implement actual resource lookup and reading logic.
	// resource, exists := s.resourceRegistry[requestParams.URI]
	// if !exists { ... return error payload ... }
	// content, err := readResourceContent(resource) // Hypothetical function
	// if err != nil { ... return error payload ... }
	// return ReadResourceResult{Content: content, Resource: resource}, nil

	// For now, always return not found error payload
	log.Printf("ReadResource requested for URI: %s (returning Not Found)", requestParams.URI)
	return nil, &ErrorPayload{
		Code:    ErrorCodeMCPResourceNotFound, // Use placeholder code
		Message: fmt.Sprintf("Resource not found (stub implementation): %s", requestParams.URI),
	}
}

// handleCallToolRequest prepares the result or error payload for a 'tools/call' request.
// It executes the registered tool handler with appropriate context and arguments.
func (s *Server) handleCallToolRequest(ctx context.Context, requestID interface{}, params interface{}) (interface{}, *ErrorPayload) {
	log.Println("Handling CallToolRequest")
	var requestParams CallToolParams

	// Unmarshal and validate params
	if params == nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: "Missing params for tools/call"}
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to re-marshal CallTool params: %v", err)}
	}
	err = json.Unmarshal(paramsBytes, &requestParams)
	if err != nil {
		log.Printf("Error unmarshalling CallTool params: %v", err)
		return nil, &ErrorPayload{Code: ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal CallTool params: %v", err)}
	}

	log.Printf("Requested tool: %s with args: %v", requestParams.Name, requestParams.Arguments)

	// Find handler
	handler, exists := s.toolHandlers[requestParams.Name]
	if !exists {
		log.Printf("Tool not found or no handler registered: %s", requestParams.Name)
		return nil, &ErrorPayload{Code: ErrorCodeMCPToolNotFound, Message: fmt.Sprintf("Tool '%s' not found or not implemented", requestParams.Name)}
	}

	// Check for progress token
	var progressToken *ProgressToken
	if requestParams.Meta != nil && requestParams.Meta.ProgressToken != nil {
		progressToken = requestParams.Meta.ProgressToken
		log.Printf("Client requested progress reporting with token: %s", *progressToken)
	}

	// --- Context Management for Cancellation ---
	// Use the passed sessionCtx (ctx) as the parent for the tool's context.
	toolCtx, cancel := context.WithCancel(ctx)
	requestIDStr := fmt.Sprintf("%v", requestID)
	s.requestMu.Lock()
	s.activeRequests[requestIDStr] = cancel
	s.requestMu.Unlock()
	defer func() {
		s.requestMu.Lock()
		delete(s.activeRequests, requestIDStr)
		s.requestMu.Unlock()
	}()
	// --- End Context Management ---

	// Execute the handler
	content, isError := handler(toolCtx, progressToken, requestParams.Arguments)

	// Construct the result payload
	resultPayload := CallToolResult{
		Content: content,
	}
	if isError {
		resultPayload.IsError = &isError
		log.Printf("Tool '%s' execution finished with error.", requestParams.Name)
	} else {
		log.Printf("Tool '%s' execution successful.", requestParams.Name)
	}

	return resultPayload, nil // Return result payload, no protocol error
}

// --- Helper methods for sending responses via transport ---
// These will replace direct s.conn calls in handlers eventually.

// sendResponse marshals and sends a successful JSON-RPC response via the transport.
// TODO: This needs the sessionID. For now, it's unusable.
func (s *Server) sendResponse(requestID interface{}, result interface{}) error {
	if s.transport == nil {
		return fmt.Errorf("cannot send response: no transport registered")
	}
	sessionID := "unknown" // FIXME: Need to get sessionID associated with requestID
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      requestID,
		Result:  result,
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		// Log internal error, but try to send a generic error response to client
		log.Printf("Internal error: Failed to marshal success response for ID %v: %v", requestID, err)
		errPayload := ErrorPayload{Code: ErrorCodeInternalError, Message: "Failed to marshal response"}
		errorBytes, _ := json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: requestID, Error: &errPayload})
		_ = s.transport.SendMessage(sessionID, errorBytes) // Attempt to send error notification
		return fmt.Errorf("failed to marshal success response: %w", err)
	}
	return s.transport.SendMessage(sessionID, responseBytes)
}

// sendErrorResponse marshals and sends a JSON-RPC error response via the transport.
// TODO: This needs the sessionID. For now, it's unusable.
func (s *Server) sendErrorResponse(requestID interface{}, errPayload ErrorPayload) error {
	if s.transport == nil {
		// Log the error locally if transport isn't available
		log.Printf("Error for request ID %v (no transport to send): Code=%d, Message=%s", requestID, errPayload.Code, errPayload.Message)
		return fmt.Errorf("cannot send error response: no transport registered")
	}
	sessionID := "unknown" // FIXME: Need to get sessionID associated with requestID
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      requestID,
		Error:   &errPayload,
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		// This is a critical internal error
		log.Printf("Internal error: Failed to marshal error response for ID %v: %v", requestID, err)
		// Cannot reliably send an error back to the client here
		return fmt.Errorf("failed to marshal error response: %w", err)
	}
	return s.transport.SendMessage(sessionID, responseBytes)
}

// sendNotification marshals and sends a JSON-RPC notification via the transport
// to a specific session, or broadcasts if sessionID is empty/special value.
// TODO: Implement actual broadcasting or session lookup based on sessionID.
func (s *Server) sendNotification(sessionID string, method string, params interface{}) error {
	if s.transport == nil {
		return fmt.Errorf("cannot send notification: no transport registered")
	}

	// FIXME: Implement proper session targeting/broadcasting logic here.
	// For now, just log a warning and send to the placeholder sessionID if provided,
	// or use a hardcoded "broadcast" placeholder if sessionID is empty.
	targetSessionID := sessionID
	if targetSessionID == "" {
		targetSessionID = "broadcast" // Placeholder for broadcast
		log.Printf("Warning: Broadcasting notification %s (actual broadcast not implemented)", method)
	} else {
		log.Printf("Warning: Sending notification %s to specific session %s (actual targeting not fully implemented)", method, targetSessionID)
	}

	notification := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	messageBytes, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Internal error: Failed to marshal notification %s: %v", method, err)
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	// FIXME: This needs to send to the correct session(s)
	log.Printf("Warning: Sending notification %s to placeholder session '%s'", method, sessionID)
	return s.transport.SendMessage(sessionID, messageBytes)
}
