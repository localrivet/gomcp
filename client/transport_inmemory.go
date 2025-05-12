package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// ServerRequestHandler defines a function that handles requests sent to the in-memory server
type ServerRequestHandler func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse

// inMemoryTransport implements ClientTransport using an in-memory server
type inMemoryTransport struct {
	options       *TransportOptions
	logger        logx.Logger
	notifyHandler NotificationHandler

	connected    bool
	connMutex    sync.RWMutex
	responseMap  *sync.Map // map[string]chan *protocol.JSONRPCResponse
	notifyBuffer chan *protocol.JSONRPCNotification

	// Server-side handling
	serverHandler        ServerRequestHandler
	pendingNotifications []protocol.JSONRPCNotification

	// Context for managing the event stream goroutine
	ctx    context.Context
	cancel context.CancelFunc
}

// NewInMemoryTransport creates a new in-memory transport
func NewInMemoryTransport(logger logx.Logger, serverHandler ServerRequestHandler, options ...TransportOption) (ClientTransport, error) {
	// Apply transport options
	opts := DefaultTransportOptions()
	for _, option := range options {
		option(opts)
	}

	t := &inMemoryTransport{
		options:              opts,
		logger:               logger,
		connected:            false,
		responseMap:          &sync.Map{},
		notifyBuffer:         make(chan *protocol.JSONRPCNotification, 100),
		serverHandler:        serverHandler,
		pendingNotifications: []protocol.JSONRPCNotification{},
	}

	// Create root context that will be used to manage the event stream
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t, nil
}

// Connect establishes the in-memory connection
func (t *inMemoryTransport) Connect(ctx context.Context) error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.connected {
		return NewConnectionError("inmemory", "already connected", ErrAlreadyConnected)
	}

	t.connected = true
	t.logger.Info("InMemory transport connected")
	return nil
}

// Close closes the in-memory connection
func (t *inMemoryTransport) Close() error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if !t.connected {
		return nil
	}

	// Cancel context to stop all goroutines
	t.cancel()

	// Close any pending response channels
	t.responseMap.Range(func(key, value interface{}) bool {
		if ch, ok := value.(chan *protocol.JSONRPCResponse); ok {
			close(ch)
		}
		t.responseMap.Delete(key)
		return true
	})

	t.connected = false
	t.logger.Info("InMemory transport disconnected")

	return nil
}

// IsConnected returns true if the transport is connected
func (t *inMemoryTransport) IsConnected() bool {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()
	return t.connected
}

// SendRequest sends a request to the in-memory server and waits for a response
func (t *inMemoryTransport) SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.IsConnected() {
		return nil, NewConnectionError("inmemory", "not connected", ErrNotConnected)
	}

	// Create a response channel for this request
	responseCh := make(chan *protocol.JSONRPCResponse, 1)
	defer close(responseCh)

	// If no ID is provided (notification), create a response anyway for testing purposes
	if req.ID == nil {
		// For notifications, call the server handler but don't wait for response
		if t.serverHandler != nil {
			go t.serverHandler(req)
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  nil,
		}, nil
	}

	// For regular requests with ID
	go func() {
		var resp *protocol.JSONRPCResponse

		// Call the server handler if provided
		if t.serverHandler != nil {
			resp = t.serverHandler(req)
		} else {
			// Default response if no handler provided
			resp = &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  nil,
			}
		}

		// Send response to channel
		select {
		case responseCh <- resp:
			// Response sent
		case <-ctx.Done():
			// Context cancelled
		}
	}()

	// Wait for response or timeout
	select {
	case resp := <-responseCh:
		return resp, nil
	case <-ctx.Done():
		return nil, NewTimeoutError("SendRequest", t.options.RequestTimeout, ctx.Err())
	}
}

// SendRequestAsync sends a request to the in-memory server without waiting for a response
func (t *inMemoryTransport) SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error {
	if !t.IsConnected() {
		return NewConnectionError("inmemory", "not connected", ErrNotConnected)
	}

	// If no response channel or it's a notification, just call the handler and return
	if responseCh == nil || req.ID == nil {
		if t.serverHandler != nil {
			go t.serverHandler(req)
		}
		return nil
	}

	// For regular requests with response channel
	go func() {
		var resp *protocol.JSONRPCResponse

		// Call the server handler if provided
		if t.serverHandler != nil {
			resp = t.serverHandler(req)
		} else {
			// Default response if no handler provided
			resp = &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  nil,
			}
		}

		// Send response to channel
		select {
		case responseCh <- resp:
			// Response sent
		default:
			// Channel full or closed
			t.logger.Warn("Response channel full or closed for request ID: %v", req.ID)
		}
	}()

	return nil
}

// SetNotificationHandler sets the handler for incoming server notifications
func (t *inMemoryTransport) SetNotificationHandler(handler NotificationHandler) {
	t.notifyHandler = handler

	// If we have pending notifications, dispatch them now
	if handler != nil && len(t.pendingNotifications) > 0 {
		go func() {
			for _, notification := range t.pendingNotifications {
				notificationCopy := notification
				if err := handler(&notificationCopy); err != nil {
					t.logger.Error("Error handling notification: %v", err)
				}
			}
			t.pendingNotifications = []protocol.JSONRPCNotification{} // Clear after processing
		}()
	}
}

// GetTransportType returns the transport type
func (t *inMemoryTransport) GetTransportType() TransportType {
	return TransportTypeInMemory
}

// GetTransportInfo returns transport-specific information
func (t *inMemoryTransport) GetTransportInfo() map[string]interface{} {
	return map[string]interface{}{
		"connected":  t.IsConnected(),
		"hasHandler": t.serverHandler != nil,
		"serverType": "in-memory",
	}
}

// SendNotification sends a notification from the server to the client
func (t *inMemoryTransport) SendNotification(method string, params interface{}) error {
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	// If not connected or no handler, buffer the notification
	if !t.IsConnected() || t.notifyHandler == nil {
		t.pendingNotifications = append(t.pendingNotifications, notification)
		return nil
	}

	// Always create a goroutine to deliver the notification to prevent blocking
	go func(n protocol.JSONRPCNotification) {
		if err := t.notifyHandler(&n); err != nil {
			t.logger.Error("Error handling notification: %v", err)
		}
	}(notification)

	return nil
}

// DefaultServerHandler returns a default server handler that processes common MCP requests
func DefaultServerHandler(tools []protocol.Tool, resources []protocol.Resource, prompts []protocol.Prompt) ServerRequestHandler {
	return func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		switch req.Method {
		case protocol.MethodInitialize:
			var params protocol.InitializeRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return protocol.NewErrorResponse(req.ID, protocol.CodeInvalidParams, "Invalid initialize params", nil)
			}

			result := protocol.InitializeResult{
				ProtocolVersion: params.ProtocolVersion,
				ServerInfo: protocol.Implementation{
					Name:    "InMemoryMCPServer",
					Version: "0.1.0",
				},
				Capabilities: protocol.ServerCapabilities{
					Tools: &struct {
						ListChanged bool `json:"listChanged,omitempty"`
					}{
						ListChanged: true,
					},
					Resources: &struct {
						Subscribe   bool `json:"subscribe,omitempty"`
						ListChanged bool `json:"listChanged,omitempty"`
					}{
						Subscribe:   true,
						ListChanged: true,
					},
					Prompts: &struct {
						ListChanged bool `json:"listChanged,omitempty"`
					}{
						ListChanged: true,
					},
				},
			}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				return protocol.NewErrorResponse(req.ID, protocol.CodeInternalError, "Failed to marshal initialize result", nil)
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultBytes),
			}

		case protocol.MethodListTools:
			result := &protocol.ListToolsResult{
				Tools: tools,
			}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				return protocol.NewErrorResponse(req.ID, protocol.CodeInternalError, "Failed to marshal tools result", nil)
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultBytes),
			}

		case protocol.MethodCallTool:
			// For testing, return a simple text result
			result := &protocol.CallToolResult{
				ToolCallID: "test-tool-call-id",
				Output:     json.RawMessage(`{"text": "This is a mock tool response"}`),
			}
			return protocol.NewSuccessResponse(req.ID, result)

		case protocol.MethodListResources:
			result := &protocol.ListResourcesResult{
				Resources: resources,
			}
			return protocol.NewSuccessResponse(req.ID, result)

		case protocol.MethodReadResource:
			var params protocol.ReadResourceRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return protocol.NewErrorResponse(req.ID, protocol.CodeInvalidParams, "Invalid read resource params", nil)
			}

			// Find the resource
			var resource protocol.Resource
			found := false
			for _, r := range resources {
				if r.URI == params.URI {
					resource = r
					found = true
					break
				}
			}

			if !found {
				return protocol.NewErrorResponse(req.ID, protocol.CodeMCPResourceNotFound, "Resource not found", nil)
			}

			// Mock resource content
			content := protocol.TextResourceContents{
				ContentType: "text/plain",
				Text:        "This is a mock resource content for " + params.URI,
			}

			result := &protocol.ReadResourceResult{
				Resource: resource,
				Contents: []protocol.ResourceContents{content},
			}
			return protocol.NewSuccessResponse(req.ID, result)

		case protocol.MethodListPrompts:
			result := &protocol.ListPromptsResult{
				Prompts: prompts,
			}
			return protocol.NewSuccessResponse(req.ID, result)

		case protocol.MethodGetPrompt:
			var params protocol.GetPromptRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return protocol.NewErrorResponse(req.ID, protocol.CodeInvalidParams, "Invalid get prompt params", nil)
			}

			// Find the prompt
			var prompt protocol.Prompt
			found := false
			for _, p := range prompts {
				if p.URI == params.URI {
					prompt = p
					found = true
					break
				}
			}

			if !found {
				return protocol.NewErrorResponse(req.ID, protocol.CodeMCPResourceNotFound, "Prompt not found", nil)
			}

			result := &protocol.GetPromptResult{
				Messages:    prompt.Messages,
				Description: prompt.Description,
			}
			return protocol.NewSuccessResponse(req.ID, result)

		default:
			return protocol.NewErrorResponse(req.ID, protocol.CodeMethodNotFound, fmt.Sprintf("Method not supported: %s", req.Method), nil)
		}
	}
}
