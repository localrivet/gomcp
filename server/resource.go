package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/localrivet/gomcp/util/schema"
	"github.com/localrivet/wilduri"
)

// ResourceHandler is a function that handles resource requests.
type ResourceHandler func(ctx *Context, args interface{}) (interface{}, error)

// Resource represents a resource registered with the server.
type Resource struct {
	Path        string
	Description string
	Handler     ResourceHandler
	Schema      interface{}
	Template    *wilduri.Template
	IsTemplate  bool // Whether this resource is a template with parameters
}

// Resource registers a resource with the server.
func (s *serverImpl) Resource(path string, description string, handler interface{}) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	resourceHandler, ok := ConvertToResourceHandler(handler)
	if !ok {
		s.logger.Error("invalid resource handler type", "path", path)
		return s
	}

	// Extract schema from the handler
	handlerSchema, err := extractSchemaFromHandler(handler)
	if err != nil {
		s.logger.Error("failed to extract schema from handler", "path", path, "error", err)
		// Use a generic schema as fallback
		handlerSchema = map[string]interface{}{
			"type": "object",
		}
	}

	// Parse the path template using wilduri
	template, err := wilduri.New(path)
	if err != nil {
		s.logger.Error("failed to parse path template", "path", path, "error", err)
		return s
	}

	// Determine if this is a template resource (has parameters)
	// A path containing '{' and '}' is considered a template
	isTemplate := strings.Contains(path, "{") && strings.Contains(path, "}")

	// Create a new resource
	resource := &Resource{
		Path:        path,
		Description: description,
		Handler:     resourceHandler,
		Schema:      handlerSchema,
		Template:    template,
		IsTemplate:  isTemplate,
	}

	// Store the resource
	s.resources[path] = resource

	// Send notification asynchronously to avoid blocking
	go func() {
		// TODO: Implement SendResourcesListChangedNotification
	}()

	return s
}

// ProcessResourceSubscribe processes a resource subscription request.
func (s *serverImpl) ProcessResourceSubscribe(ctx *Context) (interface{}, error) {
	// TODO: Implement resource subscription
	return map[string]interface{}{"subscribed": true}, nil
}

// ProcessResourceUnsubscribe processes a resource unsubscription request.
func (s *serverImpl) ProcessResourceUnsubscribe(ctx *Context) (interface{}, error) {
	// TODO: Implement resource unsubscription
	return map[string]interface{}{"unsubscribed": true}, nil
}

// ProcessResourceTemplatesList processes a resource templates list request.
func (s *serverImpl) ProcessResourceTemplatesList(ctx *Context) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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

	// For now, we'll use a simple pagination that returns all template resources
	const maxPageSize = 50
	templates := make([]map[string]interface{}, 0)
	var nextCursor string

	// Convert resources to the expected format
	i := 0
	for path, resource := range s.resources {
		// Skip if not a template or if we haven't reached the cursor yet
		if !resource.IsTemplate || (cursor != "" && path <= cursor) {
			continue
		}

		// Use the full path as the name if no other name is available
		name := resource.Path
		if path != "" {
			name = path
		}

		// Extract MIME type if available from schema or set a default
		mimeType := "application/octet-stream" // Default MIME type
		if schemaMap, ok := resource.Schema.(map[string]interface{}); ok {
			if mt, ok := schemaMap["mimeType"].(string); ok && mt != "" {
				mimeType = mt
			}
		}

		// Add the template to the result
		templates = append(templates, map[string]interface{}{
			"uriTemplate": resource.Path,
			"name":        name,
			"description": resource.Description,
			"mimeType":    mimeType,
		})

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = path
			break
		}
	}

	// Return the list of resource templates
	result := map[string]interface{}{
		"resourceTemplates": templates,
	}

	// Only add nextCursor if there are more results
	if nextCursor != "" {
		result["nextCursor"] = nextCursor
	}

	return result, nil
}

// ensureContentsArray ensures a response has a properly formatted contents array
func ensureContentsArray(response map[string]interface{}, uri string) map[string]interface{} {
	// If it already has a properly formatted contents array, we're good
	if contentsArr, hasContents := response["contents"].([]interface{}); hasContents && len(contentsArr) > 0 {
		// Convert interface{} array to properly formatted contents array with maps
		contents := make([]map[string]interface{}, 0, len(contentsArr))

		for _, item := range contentsArr {
			if contentMap, ok := item.(map[string]interface{}); ok {
				// Ensure URI is set
				if contentMap["uri"] == nil || contentMap["uri"] == "" {
					contentMap["uri"] = uri
				}

				// Ensure content field is properly formatted
				if contentItems, ok := contentMap["content"].([]interface{}); ok && len(contentItems) > 0 {
					// Validate content items
					contentMap["content"] = ensureValidContentItems(contentItems)
				} else if contentItems, ok := contentMap["content"].([]interface{}); ok && len(contentItems) == 0 {
					// This is an explicitly empty content array, preserve it as is
					contentMap["content"] = []interface{}{}
					// But ensure there's a text field at the top level for rendering
					if contentMap["text"] == nil {
						contentMap["text"] = "Empty content"
					}
				} else {
					// If content doesn't exist or is nil, create a default one
					contentMap["content"] = []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Empty content",
						},
					}
				}
				contents = append(contents, contentMap)
			}
		}

		if len(contents) > 0 {
			response["contents"] = contents
			return response
		}
	}

	// If it already has a contents array but it needs conversion
	if contentsArr, hasContents := response["contents"].([]map[string]interface{}); hasContents && len(contentsArr) > 0 {
		for i, contentMap := range contentsArr {
			// Ensure URI is set
			if contentMap["uri"] == nil || contentMap["uri"] == "" {
				contentMap["uri"] = uri
			}

			// Ensure content field is properly formatted
			if contentItems, ok := contentMap["content"].([]interface{}); ok && len(contentItems) > 0 {
				// Validate content items
				contentMap["content"] = ensureValidContentItems(contentItems)
			} else if contentItems, ok := contentMap["content"].([]interface{}); ok && len(contentItems) == 0 {
				// This is an explicitly empty content array, preserve it as is
				contentMap["content"] = []interface{}{}
				// But ensure there's a text field at the top level for rendering
				if contentMap["text"] == nil {
					contentMap["text"] = "Empty content"
				}
			} else {
				// If content doesn't exist or is nil, create a default one
				contentMap["content"] = []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Empty content",
					},
				}
			}
			contentsArr[i] = contentMap
		}

		response["contents"] = contentsArr
		return response
	}

	// If it has content, move it to contents array
	if content, hasContent := response["content"]; hasContent {
		// Check if this is an explicitly empty content array
		if contentArr, ok := content.([]interface{}); ok && len(contentArr) == 0 {
			// This is an explicitly empty content array, preserve it
			contents := []map[string]interface{}{
				{
					"uri":     uri,
					"text":    "Empty content", // Required field at contents level
					"content": []interface{}{}, // Keep the empty array
				},
			}
			response["contents"] = contents
			delete(response, "content") // Remove the original content
			return response
		}

		// Regular content array handling
		var contentArray []interface{}

		if contentArr, ok := content.([]interface{}); ok {
			contentArray = ensureValidContentItems(contentArr)
		} else {
			// Not an array, convert to a single item array
			contentArray = []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": fmt.Sprintf("%v", content), // Convert to string
				},
			}
		}

		// Create contents item
		contentsItem := map[string]interface{}{
			"uri":     uri,
			"content": contentArray,
		}

		// Find a suitable value for the required 'text' field at the contents level
		// First look for a text item in the content array
		textFound := false
		for _, item := range contentArray {
			if contentItem, ok := item.(map[string]interface{}); ok {
				if contentItem["type"] == "text" && contentItem["text"] != nil {
					contentsItem["text"] = contentItem["text"]
					textFound = true
					break
				}
			}
		}

		// If no text item found, add a default text
		if !textFound {
			contentsItem["text"] = "Content" // Required field
		}

		// Create a single item in contents array
		contents := []map[string]interface{}{contentsItem}
		response["contents"] = contents
		delete(response, "content") // Remove the original content
		return response
	}

	// Create an empty contents array if nothing else
	if response["contents"] == nil {
		response["contents"] = []map[string]interface{}{
			{
				"uri":  uri,
				"text": "Empty content", // Required field at contents level
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Empty content", // Non-empty string to ensure text field is present
					},
				},
			},
		}
	}

	return response
}

// ensureValidContentItems ensures each content item has required fields to satisfy MCP Inspector validation
func ensureValidContentItems(items []interface{}) []interface{} {
	result := make([]interface{}, len(items))

	for i, item := range items {
		if contentMap, ok := item.(map[string]interface{}); ok {
			contentType, hasType := contentMap["type"].(string)

			// Ensure we have a valid content type
			if !hasType || contentType == "" {
				contentMap["type"] = "text" // Default to text type
				contentType = "text"
			}

			// Ensure we have required field based on type
			switch contentType {
			case "text":
				// Text type MUST have a text field (even if empty) to satisfy the MCP Inspector
				if _, hasText := contentMap["text"].(string); !hasText {
					contentMap["text"] = "Empty text content" // Non-empty string to satisfy validator
				}
			case "blob":
				// Blob type MUST have a blob field (even if empty) to satisfy the MCP Inspector
				if _, hasBlob := contentMap["blob"].(string); !hasBlob {
					contentMap["blob"] = "Empty blob content" // Non-empty string to satisfy validator
				}
				// Blob should also have a mimeType
				if _, hasMimeType := contentMap["mimeType"].(string); !hasMimeType {
					contentMap["mimeType"] = "application/octet-stream" // Default MIME type
				}
			case "image":
				if url, hasURL := contentMap["imageUrl"].(string); !hasURL || url == "" {
					// Convert to text type if missing required fields
					contentMap["type"] = "text"
					contentMap["text"] = "Image URL missing"
				}
			case "link":
				if url, hasURL := contentMap["url"].(string); !hasURL || url == "" {
					// Convert to text type if missing required fields
					contentMap["type"] = "text"
					contentMap["text"] = "Link URL missing"
				}
			default:
				// For unknown types, ensure they have a text field
				contentMap["type"] = "text"
				if _, hasText := contentMap["text"].(string); !hasText {
					contentMap["text"] = "Unknown content type converted to text"
				}
			}

			result[i] = contentMap
		} else if item == nil {
			// Handle nil items by creating a text item
			result[i] = map[string]interface{}{
				"type": "text",
				"text": "Empty content", // Non-empty string
			}
		} else {
			// Convert non-map items to text items
			result[i] = map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("%v", item), // Convert to string
			}
		}
	}

	// If there are no items, add a default text item
	if len(result) == 0 {
		result = append(result, map[string]interface{}{
			"type": "text",
			"text": "Empty content", // Non-empty string to ensure text field is present
		})
	}

	// Double-check each content item has required fields
	for i, item := range result {
		if contentMap, ok := item.(map[string]interface{}); ok {
			contentType, _ := contentMap["type"].(string)

			// Final validation to ensure each item has either text or blob
			if contentType == "text" {
				if _, hasText := contentMap["text"].(string); !hasText {
					contentMap["text"] = "Empty text content"
				}
			} else if contentType == "blob" {
				if _, hasBlob := contentMap["blob"].(string); !hasBlob {
					contentMap["blob"] = "Empty blob content"
				}
				if _, hasMimeType := contentMap["mimeType"].(string); !hasMimeType {
					contentMap["mimeType"] = "application/octet-stream"
				}
			} else {
				// For any other type, ensure it has text as a fallback
				if _, hasText := contentMap["text"].(string); !hasText {
					contentMap["text"] = "Content of type: " + contentType
				}
			}

			result[i] = contentMap
		}
	}

	return result
}

// ProcessResourceRequest processes a resource request message and returns the result.
func (s *serverImpl) ProcessResourceRequest(ctx *Context) (interface{}, error) {
	// Get the resource URI from params
	if ctx.Request == nil {
		return nil, errors.New("invalid resource request")
	}

	var uri string
	if ctx.Request.ResourcePath != "" {
		// Use the parsed ResourcePath if available
		uri = ctx.Request.ResourcePath
	} else if ctx.Request.Params != nil {
		// Try to extract the URI from params
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		uri = params.URI
	}

	if uri == "" {
		return nil, errors.New("missing resource URI")
	}

	// Use a mutex read lock to protect the resource map
	s.mu.RLock()
	// Find the resource handler that matches the URI
	resource, routeParams, exists := s.findResourceAndExtractParams(uri)
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	// Prepare the arguments
	var args interface{}
	if len(routeParams) > 0 {
		args = routeParams
	} else if ctx.Request.Params != nil {
		// Parse params separately
		var params map[string]interface{}
		if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		args = params
	}

	// Execute the resource handler
	result, err := resource.Handler(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("resource execution failed: %w", err)
	}

	// Format the response according to the protocol version
	formattedResult := FormatResourceResponse(uri, result, ctx.Version)
	return formattedResult, nil
}

// ProcessResourceList processes a resource list request and returns the list of available resources
func (s *serverImpl) ProcessResourceList(ctx *Context) (interface{}, error) {
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

	// For now, we'll use a simple pagination that returns all resources
	const maxPageSize = 50
	resources := make([]map[string]interface{}, 0) // Initialize as empty array, not nil
	var nextCursor string

	// Convert resources to the expected format
	i := 0
	for path, resource := range s.resources {
		// If we have a cursor, skip until we find it
		if cursor != "" && path <= cursor {
			continue
		}

		// Skip template resources (they should only appear in templates list)
		if resource.IsTemplate {
			continue
		}

		// Use the full path as the name if no other name is available
		name := resource.Path
		// Extract the last part of the path as the name if no other name is available
		// This provides better identification, especially for parameterized paths
		if path != "" {
			name = path
		}

		// Add the resource to the result
		resources = append(resources, map[string]interface{}{
			"uri":         resource.Path,
			"description": resource.Description,
			"kind":        "file", // Default kind, should be determined based on actual resource
			"name":        name,   // Name is required by the spec
		})

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = path
			break
		}
	}

	// Return the list of resources
	result := map[string]interface{}{
		"resources": resources, // Changed back to "resources" to match the spec for resource list
	}

	// Only add nextCursor if there are more results
	if nextCursor != "" {
		result["nextCursor"] = nextCursor
	}

	return result, nil
}

// findResourceAndExtractParams finds a resource that matches the URI and extracts path parameters
func (s *serverImpl) findResourceAndExtractParams(uri string) (*Resource, map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First, try direct match
	if resource, exists := s.resources[uri]; exists {
		return resource, nil, true
	}

	// Try pattern matching for resources with path parameters
	for _, resource := range s.resources {
		if resource.Template == nil {
			continue
		}

		params, matched := resource.Template.Match(uri)
		if matched {
			// Convert to map[string]interface{} if not already
			if len(params) > 0 {
				return resource, params, true
			}
			// Matched but no parameters
			return resource, nil, true
		}
	}

	return nil, nil, false
}

// formatResourceResponse formats the resource response according to the specification version
func formatResourceResponse(result interface{}, version string) interface{} {
	// First check if result implements ResourceConverter
	if converter, ok := result.(ResourceConverter); ok {
		result = converter.ToResourceResponse()
	}

	// Handle specialized resource types
	switch v := result.(type) {
	case TextResource:
		result = v.ToResourceResponse()
	case ImageResource:
		result = v.ToResourceResponse()
	case LinkResource:
		result = v.ToResourceResponse()
	case FileResource:
		result = v.ToResourceResponse()
	case JSONResource:
		result = v.ToResourceResponse()
	case string:
		// Convert string to text content
		result = SimpleTextResponse(v)
	case []byte:
		// Convert bytes to text content
		result = SimpleTextResponse(string(v))
	case map[string]interface{}:
		// Already a map, might need structure validation/conversion
		result = v
	}

	// Now convert to the appropriate format for the MCP version
	if resultMap, ok := result.(map[string]interface{}); ok {
		// Handle different versions
		if version == "2024-11-05" {
			// For 2024-11-05, we have a flat content array structure

			// If the resource already has a properly formatted content field, just ensure it meets validation requirements
			if content, hasContent := resultMap["content"]; hasContent {
				// Make sure content is an array according to 2024-11-05 spec
				contentArray, ok := content.([]interface{})
				if !ok {
					// Not an array, make it one
					resultMap["content"] = []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": fmt.Sprintf("%v", content),
						},
					}
				} else if len(contentArray) > 0 {
					// Ensure content items are valid (have text field for text type, etc.)
					resultMap["content"] = ensureValidContentItems(contentArray)
				}
				return resultMap
			}

			// If we have a contents array (from newer specs), extract the content from the first item
			if contents, hasContents := resultMap["contents"].([]interface{}); hasContents && len(contents) > 0 {
				firstContent, ok := contents[0].(map[string]interface{})
				if ok {
					if content, hasContent := firstContent["content"]; hasContent {
						// Ensure we have a proper array of content items
						contentArray, ok := content.([]interface{})
						if ok && len(contentArray) > 0 {
							// Ensure content items are valid
							resultMap["content"] = ensureValidContentItems(contentArray)
						} else {
							// Not an array or empty array, create a valid content array
							resultMap["content"] = []interface{}{
								map[string]interface{}{
									"type": "text",
									"text": fmt.Sprintf("%v", content),
								},
							}
						}
					}
					// Copy metadata if present in firstContent
					if metadata, hasMetadata := firstContent["metadata"].(map[string]interface{}); hasMetadata {
						if resultMap["metadata"] == nil {
							resultMap["metadata"] = metadata
						}
					}
					delete(resultMap, "contents") // Remove contents array
				}
			}

			// If we still don't have a content field, create a default one
			if _, hasContent := resultMap["content"]; !hasContent {
				// Create a default content array with a text item
				resultMap["content"] = []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": fmt.Sprintf("%v", resultMap),
					},
				}
			}

			return resultMap
		} else if version == "2025-03-26" || version == "draft" {
			// These versions expect a contents array with nested content
			return ensureContentsArray(resultMap, "")
		}

		// For any version, ensure we have content
		return resultMap
	}

	// Default - pass it through
	return result
}

// formatContentResponse formats a result as a content response
func formatContentResponse(result interface{}, includeMetadata bool) map[string]interface{} {
	response := map[string]interface{}{
		"isError": false, // Default to false
	}

	// Handle different result types
	switch v := result.(type) {
	case string:
		// Simple text content
		response["content"] = []map[string]interface{}{
			{
				"type": "text",
				"text": v,
			},
		}
	case map[string]interface{}:
		// If it already has a content field, use it
		if contentField, ok := v["content"]; ok {
			// Ensure content is an array
			if contentArr, isArray := contentField.([]interface{}); isArray {
				response["content"] = contentArr
			} else {
				response["content"] = []interface{}{contentField}
			}

			// Copy other fields like metadata and isError if present
			for key, value := range v {
				if key != "content" {
					response[key] = value
				}
			}
		} else if imgURL, ok := v["imageUrl"].(string); ok {
			// Handle image content
			response["content"] = []map[string]interface{}{
				{
					"type":     "image",
					"imageUrl": imgURL,
					"altText":  v["altText"],
				},
			}
		} else {
			// Convert the map to JSON and use as text
			jsonData, _ := json.MarshalIndent(v, "", "  ")
			response["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonData),
				},
			}
		}
	case []interface{}:
		// If it's an array, assume it's content items or ensure they're valid content items
		if isContentArray(v) {
			response["content"] = v
		} else {
			// Convert to JSON
			jsonData, _ := json.MarshalIndent(v, "", "  ")
			response["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonData),
				},
			}
		}
	default:
		// For other types, convert to JSON
		jsonData, _ := json.MarshalIndent(v, "", "  ")
		response["content"] = []map[string]interface{}{
			{
				"type": "text",
				"text": string(jsonData),
			},
		}
	}

	// Add metadata if needed
	if includeMetadata && response["metadata"] == nil {
		response["metadata"] = map[string]interface{}{
			"generated": true,
		}
	}

	return response
}

// isContentArray checks if an array is a valid content array
func isContentArray(arr []interface{}) bool {
	if len(arr) == 0 {
		return false
	}

	for _, item := range arr {
		if contentMap, ok := item.(map[string]interface{}); ok {
			if contentType, hasType := contentMap["type"].(string); hasType {
				switch contentType {
				case "text", "image", "link", "file", "audio":
					// Valid content type
					continue
				default:
					return false
				}
			} else {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

// extractSchemaFromHandler extracts a JSON Schema from a handler function.
func extractSchemaFromHandler(handler interface{}) (map[string]interface{}, error) {
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

	// For map[string]interface{} path parameters
	if argType == reflect.TypeOf(map[string]interface{}{}) {
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
		}, nil
	}

	// For non-struct types, return a generic schema
	return map[string]interface{}{
		"type": "object",
	}, nil
}

// ConvertToResourceHandler converts a function to a ResourceHandler if possible.
func ConvertToResourceHandler(handler interface{}) (ResourceHandler, bool) {
	if handler == nil {
		return nil, false
	}

	// Check if it's already a ResourceHandler
	if h, ok := handler.(ResourceHandler); ok {
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

	// Create a resource handler that calls the original function
	resourceHandler := func(ctx *Context, args interface{}) (interface{}, error) {
		// Create values for the function call
		values := make([]reflect.Value, 2)

		// First parameter is the context
		values[0] = reflect.ValueOf(ctx)

		// Handle the second parameter (args)
		// If args is nil, we need to create a zero value of the expected type
		if args == nil {
			// Create a zero value of the expected type
			values[1] = reflect.Zero(handlerType.In(1))
		} else {
			// For path parameters or other args, try to convert to the expected type
			argType := handlerType.In(1)
			if argType.Kind() == reflect.Map {
				// Map type is fine as is
				values[1] = reflect.ValueOf(args)
			} else if argType.Kind() == reflect.Struct || (argType.Kind() == reflect.Ptr && argType.Elem().Kind() == reflect.Struct) {
				// Need to convert map to struct
				mapArgs, ok := args.(map[string]interface{})
				if ok {
					// Create a placeholder schema if one wasn't explicitly provided
					schemaMap := map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					}

					// Use the schema validation and conversion utility
					convertedArg, err := schema.ValidateAndConvertArgs(schemaMap, mapArgs, argType)
					if err != nil {
						return nil, fmt.Errorf("failed to convert arguments: %w", err)
					}
					values[1] = reflect.ValueOf(convertedArg)
				} else {
					// Use the args as-is
					values[1] = reflect.ValueOf(args)
				}
			} else {
				// Use the args as-is
				values[1] = reflect.ValueOf(args)
			}
		}

		// Call the handler function
		results := handlerValue.Call(values)

		// Get the result values
		result := results[0].Interface()
		var err error
		if !results[1].IsNil() {
			err = results[1].Interface().(error)
		}

		// Check if there was an error
		if err != nil {
			return nil, err
		}

		// Return the result directly - formatResourceResponse will handle the conversion
		return result, nil
	}

	return resourceHandler, true
}
