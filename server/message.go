package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// handleMessage processes incoming JSON-RPC messages from clients.
// It determines if the message is a request or response and routes it appropriately.
// For requests, it calls HandleMessage to process them; for responses, it calls
// HandleJSONRPCResponse to match them with pending requests.
func (s *serverImpl) handleMessage(message []byte) ([]byte, error) {
	// Check if this is a response (has no "method" field but has "id")
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err == nil {
		if _, hasMethod := msg["method"]; !hasMethod {
			if _, hasID := msg["id"]; hasID {
				// This is a response, process it differently
				if err := s.HandleJSONRPCResponse(message); err != nil {
					s.logger.Error("failed to handle JSON-RPC response", "error", err)
				}
				return nil, nil
			}
		}
	}

	// This is a request, process normally
	return HandleMessage(s, message)
}

// HandleMessage handles an incoming message from the transport.
// It parses the message, routes it to the appropriate handler, and returns the response.
func HandleMessage(s *serverImpl, message []byte) ([]byte, error) {
	// Create a new context with the incoming message
	ctx, err := NewContext(context.Background(), message, s)
	if err != nil {
		s.logger.Error("failed to create context", "error", err)
		return createErrorResponse(nil, -32700, "Parse error", err.Error()), nil
	}

	var result interface{}

	// Process the message based on its method
	switch ctx.Request.Method {
	// Lifecycle methods
	case "initialize":
		result, err = s.ProcessInitialize(ctx)
	case "shutdown":
		result, err = s.ProcessShutdown(ctx)
	case "ping":
		// Simple working ping implementation
		result = map[string]interface{}{} // Return empty object as specified in the protocol

	// Tool methods
	case "tools/list":
		result, err = s.ProcessToolList(ctx)
	case "tools/call":
		result, err = s.ProcessToolCall(ctx)

	// Resource methods
	case "resources/list":
		result, err = s.ProcessResourceList(ctx)
	case "resources/read":
		result, err = s.ProcessResourceRequest(ctx)
	case "resources/subscribe":
		result, err = s.ProcessResourceSubscribe(ctx)
	case "resources/unsubscribe":
		result, err = s.ProcessResourceUnsubscribe(ctx)
	case "resources/templates/list":
		result, err = s.ProcessResourceTemplatesList(ctx)

	// Prompt methods
	case "prompts/list":
		result, err = s.ProcessPromptList(ctx)
	case "prompts/get":
		result, err = s.ProcessPromptRequest(ctx)

	// Utility methods
	case "logging/setLevel":
		result, err = s.ProcessLoggingSetLevel(ctx)
	case "completion/complete":
		result, err = s.ProcessCompletionComplete(ctx)

	// Client methods (server -> client)
	case "sampling/createMessage":
		result, err = s.ProcessSamplingCreateMessage(ctx)
	case "roots/list":
		// This is typically a client method that the server calls
		err = fmt.Errorf("method not implemented: %s", ctx.Request.Method)

	// Notifications
	case "notifications/initialized":
	case "notifications/cancelled":
	case "notifications/progress":
	case "notifications/message":
	case "notifications/resources/list_changed":
	case "notifications/resources/updated":
	case "notifications/tools/list_changed":
	case "notifications/prompts/list_changed":
	case "notifications/roots/list_changed":
		// Notifications don't need responses
		return nil, nil

	default:
		err = fmt.Errorf("method not found: %s", ctx.Request.Method)
		return createErrorResponse(ctx.Request.ID, -32601, "Method not found", err.Error()), nil
	}

	if err != nil {
		s.logger.Error("failed to process message", "method", ctx.Request.Method, "error", err)
		// Use the right error code:
		// -32601 for "Method not implemented" messages
		// -32602 for "Invalid parameters" errors
		// -32603 for other internal errors
		if err.Error() == fmt.Sprintf("method not implemented: %s", ctx.Request.Method) {
			return createErrorResponse(ctx.Request.ID, -32601, "Method not implemented", err.Error()), nil
		}

		// Check if it's an invalid parameters error
		if _, ok := err.(*InvalidParametersError); ok {
			return createErrorResponse(ctx.Request.ID, -32602, "Invalid params", err.Error()), nil
		}

		return createErrorResponse(ctx.Request.ID, -32603, "Internal error", err.Error()), nil
	}

	// Set the result in the response
	ctx.Response.Result = result

	// Encode the response as JSON
	responseBytes, err := json.Marshal(ctx.Response)
	if err != nil {
		s.logger.Error("failed to marshal response", "error", err)
		return createErrorResponse(ctx.Request.ID, -32603, "Internal error", "Failed to marshal response"), nil
	}

	return responseBytes, nil
}

// HandleMessageWithVersion handles a JSON-RPC message with a forced MCP version.
// This is primarily used for testing and allows processing messages with a
// specific protocol version regardless of what was negotiated during initialization.
// It provides a simplified subset of method handlers compared to the main HandleMessage function.
func HandleMessageWithVersion(srv Server, message []byte, version string) ([]byte, error) {
	serverImpl := srv.GetServer()
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	// Parse the message to get the method
	var request Request
	if err := json.Unmarshal(message, &request); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Create a context for the request
	ctx := &Context{
		Request: &request,
		Version: version, // Use the forced version
		server:  serverImpl,
	}

	// Extract resource path if this is a resource request
	if request.Method == "resources/read" && request.Params != nil {
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(request.Params, &params); err == nil && params.URI != "" {
			ctx.Request.ResourcePath = params.URI
		}
	}

	// Process the method
	var result interface{}
	var err error

	// Use the appropriate method handler based on the request method
	switch request.Method {
	case "ping":
		result = map[string]interface{}{} // Return empty object as specified in the protocol
	case "roots/list":
		// This is a client-side method, server should reject it
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	case "resources/list":
		result, err = serverImpl.ProcessResourceList(ctx)
	case "resources/read":
		result, err = serverImpl.ProcessResourceRequest(ctx)
	case "resources/templates/list":
		result, err = serverImpl.ProcessResourceTemplatesList(ctx)
	case "resources/subscribe":
		result, err = serverImpl.ProcessResourceSubscribe(ctx)
	case "resources/unsubscribe":
		result, err = serverImpl.ProcessResourceUnsubscribe(ctx)
	default:
		return nil, fmt.Errorf("method not found: %s", request.Method)
	}

	if err != nil {
		// Return the error
		errorResponse := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"error": map[string]interface{}{
				"code":    -32603, // Internal error
				"message": err.Error(),
			},
		}
		jsonResponse, _ := json.Marshal(errorResponse)
		return jsonResponse, nil
	}

	// Return the result
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result":  result,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return jsonResponse, nil
}
