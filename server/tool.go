package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/localrivet/gomcp/util/schema"
)

// ToolHandler is a function that handles tool calls.
// It receives a context with request information and arguments,
// and returns a result and any error that occurred.
type ToolHandler func(ctx *Context, args interface{}) (interface{}, error)

// Tool represents a tool registered with the server.
// Tools are functions that can be called by clients connected to the server.
type Tool struct {
	// Name is the unique identifier for the tool
	Name string

	// Description explains what the tool does
	Description string

	// Handler is the function that executes when the tool is called
	Handler ToolHandler

	// Schema defines the expected input format for the tool
	Schema interface{}

	// Annotations contains additional metadata about the tool
	Annotations map[string]interface{}
}

// Tool registers a tool with the server.
// The function returns the server instance to allow for method chaining.
// The name parameter is used as the identifier for the tool.
// The description parameter explains what the tool does.
// The handler parameter is a function that is called when the tool is invoked.
func (s *serverImpl) Tool(name string, description string, handler interface{}) Server {
	toolHandler, ok := convertToToolHandler(handler)
	if !ok {
		s.logger.Error("invalid tool handler type", "name", name)
		return s
	}

	// Extract schema from the handler
	schema, err := extractSchema(handler)
	if err != nil {
		s.logger.Error("failed to extract schema from handler", "name", name, "error", err)
		// Use a generic schema as fallback
		schema = map[string]interface{}{
			"type": "object",
		}
	}

	// Use the internal registerTool method to store the tool
	s.registerTool(name, description, toolHandler, schema)
	return s
}

// registerTool registers a tool with the server.
// It's an internal method used by the Tool method.
// This method handles validation, duplicate detection, and notifications.
func (s *serverImpl) registerTool(name, description string, handler ToolHandler, schema map[string]interface{}) *serverImpl {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate tool name is not empty
	if name == "" {
		s.logger.Error("tool name cannot be empty")
		return s
	}

	// Check for duplicate tool names
	existingTool, exists := s.tools[name]
	isUpdate := false
	if exists {
		// For functions and maps, we can't do direct comparison
		// We'll consider it an update if the description changed, which is a simple way to trigger a notification
		isUpdate = existingTool.Description != description
		s.logger.Warn("tool already registered, overwriting", "name", name)
	}

	// Store the tool in the server's tools map
	s.tools[name] = &Tool{
		Name:        name,
		Description: description,
		Handler:     handler,
		Schema:      schema,
		Annotations: make(map[string]interface{}),
	}

	s.logger.Debug("registered tool", "name", name)

	// If this is a new tool or an update to an existing one, send a notification
	if !exists || isUpdate {
		// Send notification asynchronously to avoid blocking
		go func() {
			if err := s.SendToolsListChangedNotification(); err != nil {
				s.logger.Error("failed to send tools list changed notification", "error", err)
			}
		}()
	}

	return s
}

// ProcessToolList processes a tool list request and returns the list of available tools.
// It supports pagination through an optional cursor parameter.
// The response includes the tools' name, description, and input schema.
func (s *serverImpl) ProcessToolList(ctx *Context) (interface{}, error) {
	// Get pagination cursor if provided
	var cursor string
	if ctx.Request.Params != nil {
		var params struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		cursor = params.Cursor
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// For now, we'll use a simple pagination that returns all tools
	// In a real implementation, you'd parse the cursor and limit results
	const maxPageSize = 50
	var tools = make([]map[string]interface{}, 0, len(s.tools))
	var nextCursor string

	// Convert tools to the expected format
	i := 0
	for name, tool := range s.tools {
		// If we have a cursor, skip until we find it
		// This is a simplistic approach; real cursor would be more sophisticated
		if cursor != "" && name <= cursor {
			continue
		}

		// Add the tool to the result
		toolInfo := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.Schema,
		}

		// Only include annotations if they exist
		if len(tool.Annotations) > 0 {
			toolInfo["annotations"] = tool.Annotations
		}

		tools = append(tools, toolInfo)

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = name
			break
		}
	}

	// Return the list of tools
	result := map[string]interface{}{
		"tools": tools,
	}

	// Only add nextCursor if there are more results
	if nextCursor != "" {
		result["nextCursor"] = nextCursor
	}

	return result, nil
}

// extractSchema extracts a JSON Schema from a handler function.
// It analyzes the function's parameter structure and generates a schema
// that describes the expected input format. This is used to inform clients
// about the structure of arguments the tool expects.
func extractSchema(handler interface{}) (map[string]interface{}, error) {
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		return nil, errors.New("handler must be a function")
	}

	// Functions must have at least two parameters (context and args)
	if handlerType.NumIn() < 2 {
		return nil, errors.New("handler must have at least two parameters (context and args)")
	}

	// Get the second parameter (args)
	argType := handlerType.In(1)

	// If it's a pointer, get the element type
	if argType.Kind() == reflect.Ptr {
		argType = argType.Elem()
	}

	// Try to infer the schema from the parameter type
	if argType.Kind() == reflect.Struct {
		// Create an instance of the struct for schema generation
		structVal := reflect.New(argType).Elem().Interface()

		// Use the schema generator to create a schema from the struct
		generator := schema.NewGenerator()
		schemaMap, err := generator.GenerateSchema(structVal)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema: %w", err)
		}

		// If the schema is empty, add some defaults
		if props, ok := schemaMap["properties"].(map[string]interface{}); ok && len(props) == 0 {
			// Default to a generic object schema
			schemaMap = map[string]interface{}{
				"type": "object",
			}
		}

		return schemaMap, nil
	}

	// For non-struct types, return a generic schema
	return map[string]interface{}{
		"type": "object",
	}, nil
}

// executeTool executes a registered tool with the given arguments.
// It handles argument validation, conversion, and execution of the tool handler.
// Returns the result from the tool handler or an error if execution fails.
func (s *serverImpl) executeTool(ctx *Context, name string, args map[string]interface{}) (interface{}, error) {
	s.mu.RLock()
	tool, exists := s.tools[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	// Get the handler's parameter type
	handlerType := reflect.TypeOf(tool.Handler)
	paramType := handlerType.In(1)

	// Validate and convert the arguments using schema package
	convertedArgs, err := schema.ValidateAndConvertArgs(tool.Schema.(map[string]interface{}), args, paramType)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Execute the tool handler
	result, err := tool.Handler(ctx, convertedArgs)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// ProcessToolCall processes a tool call message and returns the result.
// It executes the requested tool with the provided arguments and formats the response
// according to the MCP protocol specification.
func (s *serverImpl) ProcessToolCall(ctx *Context) (interface{}, error) {
	if ctx.Request == nil || ctx.Request.ToolName == "" {
		return nil, errors.New("invalid tool call request")
	}

	// Execute the requested tool
	result, err := s.executeTool(ctx, ctx.Request.ToolName, ctx.Request.ToolArgs)
	if err != nil {
		// For tool-specific errors, we still return a valid result but with isError=true
		if strings.Contains(err.Error(), "tool execution failed:") {
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": err.Error(),
					},
				},
				"isError": true,
			}, nil
		}
		// For other errors (like tool not found), return a protocol error
		return nil, err
	}

	// Format the result according to the specification
	formattedResult := map[string]interface{}{
		"content": []map[string]interface{}{},
		"isError": false,
	}

	// Add appropriate content based on result type
	switch v := result.(type) {
	case string:
		// Simple text result
		formattedResult["content"] = []map[string]interface{}{
			{
				"type": "text",
				"text": v,
			},
		}
	case map[string]interface{}:
		// If result is already in the expected format with content field, use it directly
		if content, ok := v["content"]; ok {
			formattedResult["content"] = content
			if isError, ok := v["isError"].(bool); ok {
				formattedResult["isError"] = isError
			}
		} else if imageUrl, ok := v["imageUrl"].(string); ok {
			// Handle image result
			formattedResult["content"] = []map[string]interface{}{
				{
					"type":     "image",
					"imageUrl": imageUrl,
					"altText":  v["altText"], // Include alt text if provided
				},
			}
		} else if url, ok := v["url"].(string); ok {
			// Handle link result
			formattedResult["content"] = []map[string]interface{}{
				{
					"type":  "link",
					"url":   url,
					"title": v["title"], // Include title if provided
				},
			}
		} else if mimeType, ok := v["mimeType"].(string); ok && v["data"] != nil {
			// Handle binary/file data
			formattedResult["content"] = []map[string]interface{}{
				{
					"type":     "file",
					"mimeType": mimeType,
					"data":     v["data"],
					"filename": v["filename"], // Include filename if provided
				},
			}
		} else {
			// Otherwise convert the map to JSON and use as text
			jsonData, _ := json.MarshalIndent(v, "", "  ")
			formattedResult["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonData),
				},
			}
		}
	case []interface{}:
		// If it's an array of content items, try to use it directly
		contentItems := make([]map[string]interface{}, 0, len(v))

		// Process each item and add to content
		for _, item := range v {
			if contentMap, ok := item.(map[string]interface{}); ok {
				// Verify it has a type field
				if contentType, hasType := contentMap["type"].(string); hasType {
					// Validate based on content type
					switch contentType {
					case "text":
						if _, hasText := contentMap["text"]; !hasText {
							contentMap["text"] = "Missing text content"
						}
					case "image":
						if _, hasUrl := contentMap["imageUrl"]; !hasUrl {
							continue // Skip invalid image items
						}
					case "link":
						if _, hasUrl := contentMap["url"]; !hasUrl {
							continue // Skip invalid link items
						}
					case "file":
						if _, hasMime := contentMap["mimeType"]; !hasMime || contentMap["data"] == nil {
							continue // Skip invalid file items
						}
					default:
						// Unknown content type, skip
						continue
					}

					contentItems = append(contentItems, contentMap)
				}
			}
		}

		// If we found valid content items, use them
		if len(contentItems) > 0 {
			formattedResult["content"] = contentItems
		} else {
			// Fallback: Convert the array to JSON
			jsonData, _ := json.MarshalIndent(v, "", "  ")
			formattedResult["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonData),
				},
			}
		}
	default:
		// For other types, convert to JSON
		jsonData, _ := json.MarshalIndent(v, "", "  ")
		formattedResult["content"] = []map[string]interface{}{
			{
				"type": "text",
				"text": string(jsonData),
			},
		}
	}

	return formattedResult, nil
}

// SendToolsListChangedNotification sends a notification to inform clients that the tool list has changed.
// This is called when tools are added, removed, or updated, allowing clients to refresh their available tools.
func (s *serverImpl) SendToolsListChangedNotification() error {
	// Create the notification message
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/tools/list_changed",
	}

	// Marshal the notification to JSON
	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Send the notification through the configured transport
	if s.transport != nil {
		if err := s.transport.Send(notificationBytes); err != nil {
			s.logger.Error("failed to send notification", "error", err)
			return fmt.Errorf("failed to send notification: %w", err)
		}
	} else {
		s.logger.Warn("no transport configured, skipping notification")
	}

	s.logger.Debug("sent tools/list_changed notification")
	return nil
}

// WithAnnotations adds annotations to a tool.
// Annotations provide additional metadata that can be used by clients.
// The function returns the server instance to allow for method chaining.
func (s *serverImpl) WithAnnotations(toolName string, annotations map[string]interface{}) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool, exists := s.tools[toolName]
	if !exists {
		s.logger.Error("tool not found for annotations", "name", toolName)
		return s
	}

	// Update the tool's annotations
	for k, v := range annotations {
		tool.Annotations[k] = v
	}

	// Send notification asynchronously to avoid blocking
	go func() {
		if err := s.SendToolsListChangedNotification(); err != nil {
			s.logger.Error("failed to send tools list changed notification", "error", err)
		}
	}()

	return s
}

// convertToToolHandler converts a function to a ToolHandler if possible.
// It uses reflection to validate the function signature and creates a wrapper
// that adapts the function to the ToolHandler interface. Returns the converted
// handler and a boolean indicating success.
func convertToToolHandler(handler interface{}) (ToolHandler, bool) {
	if handler == nil {
		return nil, false
	}

	// Check if it's already a ToolHandler
	if h, ok := handler.(ToolHandler); ok {
		return h, true
	}

	// Check if it's a function with the right signature using reflection
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		return nil, false
	}

	if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
		return nil, false
	}

	// Check if first param is *Context
	if handlerType.In(0) != reflect.TypeOf((*Context)(nil)) {
		return nil, false
	}

	// Check if second return is error
	if !handlerType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, false
	}

	// Get the second parameter type (args type)
	paramType := handlerType.In(1)

	// Create a tool handler that calls the original function
	toolHandler := func(ctx *Context, args interface{}) (interface{}, error) {
		var argValue reflect.Value

		// If args is already the correct type, use it directly
		if reflect.TypeOf(args) == paramType {
			argValue = reflect.ValueOf(args)
		} else if mapArgs, ok := args.(map[string]interface{}); ok {
			// For map arguments going to a struct parameter, use a more robust conversion

			// Create a placeholder schema if one wasn't explicitly provided
			schemaMap := map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}

			// Use the schema validation and conversion utility
			convertedArg, err := schema.ValidateAndConvertArgs(schemaMap, mapArgs, paramType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert arguments: %w", err)
			}
			argValue = reflect.ValueOf(convertedArg)
		} else {
			// If types don't match and we can't convert, return an error
			return nil, fmt.Errorf("incompatible argument type: expected %s, got %T", paramType, args)
		}

		// Call the handler function with the context and args
		results := handlerValue.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			argValue,
		})

		// Get the result values
		result := results[0].Interface()
		var err error
		if !results[1].IsNil() {
			err = results[1].Interface().(error)
		}

		return result, err
	}

	return toolHandler, true
}
