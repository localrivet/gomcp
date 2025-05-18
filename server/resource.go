package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/localrivet/gomcp/util/schema"
	"github.com/localrivet/wilduri"
)

// ResourceHandler is a function that handles resource requests.
// It receives a context with the request information and arguments,
// and returns a result and any error that occurred.
type ResourceHandler func(ctx *Context, args interface{}) (interface{}, error)

// Resource represents a resource registered with the server.
// Resources are endpoints that clients can access to retrieve structured data.
type Resource struct {
	// Path is the URL pattern for accessing this resource
	Path string

	// Description explains what the resource provides
	Description string

	// Handler is the function that executes when the resource is accessed
	Handler ResourceHandler

	// Schema defines the expected format of the resource data
	Schema interface{}

	// Template is the parsed path template used for matching URLs
	Template *wilduri.Template

	// IsTemplate indicates whether this resource path contains parameters
	IsTemplate bool // Whether this resource is a template with parameters
}

// Resource registers a resource with the server.
// The function returns the server instance to allow for method chaining.
// The path parameter defines the resource URL pattern, which can include parameters in {braces}.
// The description parameter provides human-readable documentation.
// The handler parameter is a function that implements the resource's logic.
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
// Resource subscriptions allow clients to receive notifications when resource data changes.
// Returns a response indicating whether the subscription was successful.
func (s *serverImpl) ProcessResourceSubscribe(ctx *Context) (interface{}, error) {
	// TODO: Implement resource subscription
	return map[string]interface{}{"subscribed": true}, nil
}

// ProcessResourceUnsubscribe processes a resource unsubscription request.
// This allows clients to stop receiving notifications for a previously subscribed resource.
// Returns a response indicating whether the unsubscription was successful.
func (s *serverImpl) ProcessResourceUnsubscribe(ctx *Context) (interface{}, error) {
	// TODO: Implement resource unsubscription
	return map[string]interface{}{"unsubscribed": true}, nil
}

// ProcessResourceTemplatesList processes a resource templates list request.
// This returns a list of all resource templates (resources with path parameters)
// registered with the server. Supports pagination through an optional cursor parameter.
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

// ensureContentsArray ensures a response has a properly formatted contents array.
// This function standardizes resource response format by ensuring the contents field
// follows the expected structure, with proper URI and content fields.
// Returns the properly formatted response.
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

// ensureValidContentItems validates and normalizes content items in a resource response.
// It ensures that each content item has the required fields based on its type
// and that all fields are properly formatted.
// Returns a normalized array of content items that conform to the specification.
func ensureValidContentItems(items []interface{}) []interface{} {
	validItems := make([]interface{}, 0, len(items))

	for _, item := range items {
		if contentMap, ok := item.(map[string]interface{}); ok {
			// Must have a type field
			contentType, hasType := contentMap["type"].(string)
			if !hasType {
				// Skip items without type
				continue
			}

			// Validate based on content type
			switch contentType {
			case "text":
				// Text must have a text field
				if _, hasText := contentMap["text"].(string); !hasText {
					contentMap["text"] = "Missing text"
				}
				validItems = append(validItems, contentMap)

			case "image":
				// Image must have an imageUrl field
				if _, hasURL := contentMap["imageUrl"].(string); !hasURL {
					// Skip invalid image items
					continue
				}
				validItems = append(validItems, contentMap)

			case "link":
				// Link must have a url field
				if _, hasURL := contentMap["url"].(string); !hasURL {
					// Skip invalid link items
					continue
				}
				validItems = append(validItems, contentMap)

			case "file":
				// File must have a mimeType and data field
				if _, hasMime := contentMap["mimeType"].(string); !hasMime {
					// Skip invalid file items
					continue
				}
				if contentMap["data"] == nil {
					// Skip invalid file items
					continue
				}
				validItems = append(validItems, contentMap)

			default:
				// Skip unknown content types
				continue
			}
		}
	}

	// If no valid items, create a default text item
	if len(validItems) == 0 {
		validItems = append(validItems, map[string]interface{}{
			"type": "text",
			"text": "No valid content",
		})
	}

	return validItems
}

// ProcessResourceRequest processes a resource request.
// This method handles client requests to access resources, finding the appropriate
// resource handler based on the URI, executing it, and formatting the response
// according to the MCP protocol specification.
func (s *serverImpl) ProcessResourceRequest(ctx *Context) (interface{}, error) {
	// Get the resource URI from the request params
	if ctx.Request.Params == nil {
		return nil, errors.New("missing params in resource request")
	}

	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	uri := params.URI
	if uri == "" {
		return nil, errors.New("missing or empty uri in resource request")
	}

	// Find the resource and extract params
	resource, pathParams, found := s.findResourceAndExtractParams(uri)
	if !found {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	// Execute the resource handler
	result, err := resource.Handler(ctx, pathParams)
	if err != nil {
		return nil, fmt.Errorf("resource handler error: %w", err)
	}

	// Format the response based on the protocol version
	// Get the protocol version from the context
	version := ctx.Version
	if version == "" {
		// Default to latest protocol version
		version = "2025-03-26"
	}

	return formatResourceResponse(result, version), nil
}

// ProcessResourceList processes a resource list request.
// This method returns a list of all resources registered with the server,
// supporting pagination through an optional cursor parameter.
// The response includes resource metadata such as URI, description, and MIME type.
func (s *serverImpl) ProcessResourceList(ctx *Context) (interface{}, error) {
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

	// For now, we'll use a simple pagination that returns all resources
	const maxPageSize = 50
	resources := make([]map[string]interface{}, 0)
	var nextCursor string

	// Convert resources to the expected format
	i := 0
	for path, resource := range s.resources {
		// Skip if we haven't reached the cursor yet
		if cursor != "" && path <= cursor {
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

		// Add the resource to the result
		resourceInfo := map[string]interface{}{
			"uri":         resource.Path,
			"name":        name,
			"description": resource.Description,
			"mimeType":    mimeType,
		}

		// Add isTemplate if this is a template resource
		if resource.IsTemplate {
			resourceInfo["isTemplate"] = true
		}

		resources = append(resources, resourceInfo)

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = path
			break
		}
	}

	// Return the list of resources
	result := map[string]interface{}{
		"resources": resources,
	}

	// Only add nextCursor if there are more results
	if nextCursor != "" {
		result["nextCursor"] = nextCursor
	}

	return result, nil
}

// findResourceAndExtractParams finds a resource matching the given URI
// and extracts any path parameters from the URI.
// Returns the matched resource, extracted parameters, and a boolean indicating success.
func (s *serverImpl) findResourceAndExtractParams(uri string) (*Resource, map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check for exact match first (for non-template resources)
	if resource, ok := s.resources[uri]; ok {
		return resource, make(map[string]interface{}), true
	}

	// For template resources, try to match against the pattern
	for _, resource := range s.resources {
		if !resource.IsTemplate {
			continue
		}

		// Use the template to match the URI
		matches, matched := resource.Template.Match(uri)
		if matched && matches != nil {
			// Convert matches to a map for the handler
			params := make(map[string]interface{})
			for key, value := range matches {
				params[key] = value
			}
			return resource, params, true
		}
	}

	return nil, nil, false
}

// formatResourceResponse formats the result of a resource handler execution
// according to the MCP protocol specification for the given version.
// Handles different response formats based on the type of the result.
func formatResourceResponse(result interface{}, version string) interface{} {
	// If the result is already a properly formatted resource response, handle it appropriately
	if _, ok := result.(*ResourceResponse); ok {
		// Since ResourceResponse.Format() is undefined, just return the result
		return result
	}

	// For map results, check if it's a properly formatted response
	if mapResult, ok := result.(map[string]interface{}); ok {
		// If it has a contents field, it might be a properly formatted response
		if _, hasContents := mapResult["contents"]; hasContents {
			// This seems to be a formatted response already, ensure its format
			formatted := ensureContentsArray(mapResult, "")
			return formatted
		}
	}

	// For other result types, convert to a generic response based on protocol version
	switch version {
	case "2024-11-05":
		// 2024-11-05 uses contents directly
		return formatResourceContentArray(result, version)
	case "2025-03-26", "draft":
		// 2025-03-26 and draft use structured content
		return formatContentResponse(result, true)
	default:
		// Default to the latest version
		return formatContentResponse(result, true)
	}
}

// formatResourceContentArray formats a resource response as a content array.
// This is used for older protocol versions that expect a different response format.
// Returns a properly formatted response for the appropriate protocol version.
func formatResourceContentArray(result interface{}, version string) interface{} {
	// Create a default content array
	var contents []interface{}

	// Convert the result to a content array based on its type
	switch v := result.(type) {
	case []interface{}:
		// If it's already an array, validate each item is a content object
		if isContentArray(v) {
			contents = v
		} else {
			// Convert non-content array to JSON text
			jsonStr, _ := json.MarshalIndent(v, "", "  ")
			contents = []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": string(jsonStr),
				},
			}
		}
	case string:
		// Convert string to text content
		contents = []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": v,
			},
		}
	default:
		// Convert other types to JSON text
		jsonStr, _ := json.MarshalIndent(v, "", "  ")
		contents = []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(jsonStr),
			},
		}
	}

	// Format based on version
	return map[string]interface{}{
		"contents": contents,
	}
}

// formatContentResponse formats a resource response for the latest protocol versions.
// It creates a structured response with contents and metadata based on the result type.
// The includeMetadata parameter controls whether to include additional metadata fields.
func formatContentResponse(result interface{}, includeMetadata bool) map[string]interface{} {
	var content []map[string]interface{}

	// Convert the result to a content array based on its type
	switch v := result.(type) {
	case string:
		// Simple text content
		content = []map[string]interface{}{
			{
				"type": "text",
				"text": v,
			},
		}
	case map[string]interface{}:
		// Check for special content types
		if imageUrl, ok := v["imageUrl"].(string); ok && imageUrl != "" {
			// Handle image
			contentItem := map[string]interface{}{
				"type":     "image",
				"imageUrl": imageUrl,
			}
			if altText, ok := v["altText"].(string); ok {
				contentItem["altText"] = altText
			}
			content = []map[string]interface{}{contentItem}
		} else if url, ok := v["url"].(string); ok && url != "" {
			// Handle link
			contentItem := map[string]interface{}{
				"type": "link",
				"url":  url,
			}
			if title, ok := v["title"].(string); ok {
				contentItem["title"] = title
			}
			content = []map[string]interface{}{contentItem}
		} else if mimeType, ok := v["mimeType"].(string); ok && mimeType != "" && v["data"] != nil {
			// Handle file
			contentItem := map[string]interface{}{
				"type":     "file",
				"mimeType": mimeType,
				"data":     v["data"],
			}
			if filename, ok := v["filename"].(string); ok {
				contentItem["filename"] = filename
			}
			content = []map[string]interface{}{contentItem}
		} else if resourceType, ok := v["resourceType"].(string); ok && resourceType != "" {
			// Handle structured resource data
			contentItem := make(map[string]interface{})
			for key, value := range v {
				contentItem[key] = value
			}
			// Ensure it has a type field for content
			if contentItem["type"] == nil {
				contentItem["type"] = "resource"
			}
			content = []map[string]interface{}{contentItem}
		} else {
			// Convert generic map to JSON text
			jsonStr, _ := json.MarshalIndent(v, "", "  ")
			content = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonStr),
				},
			}
		}
	case []interface{}:
		// If it's an array, check if it's already a valid content array
		if contentArray, isValid := validateContentArray(v); isValid {
			content = contentArray
		} else {
			// Convert generic array to JSON text
			jsonStr, _ := json.MarshalIndent(v, "", "  ")
			content = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonStr),
				},
			}
		}
	default:
		// Convert other types to JSON text
		jsonStr, _ := json.MarshalIndent(v, "", "  ")
		content = []map[string]interface{}{
			{
				"type": "text",
				"text": string(jsonStr),
			},
		}
	}

	// Create the response
	response := map[string]interface{}{
		"content": content,
	}

	// Add metadata if requested
	if includeMetadata {
		response["metadata"] = map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		}
	}

	return response
}

// isContentArray checks if an array is a valid content array.
// A valid content array contains maps with a "type" field indicating content type.
// Returns true if the array is a valid content array, false otherwise.
func isContentArray(arr []interface{}) bool {
	if len(arr) == 0 {
		return false
	}

	for _, item := range arr {
		contentMap, ok := item.(map[string]interface{})
		if !ok {
			return false
		}

		contentType, hasType := contentMap["type"].(string)
		if !hasType {
			return false
		}

		// Validate based on content type
		switch contentType {
		case "text":
			if _, hasText := contentMap["text"].(string); !hasText {
				return false
			}
		case "image":
			if _, hasURL := contentMap["imageUrl"].(string); !hasURL {
				return false
			}
		case "link":
			if _, hasURL := contentMap["url"].(string); !hasURL {
				return false
			}
		case "file":
			if _, hasMime := contentMap["mimeType"].(string); !hasMime {
				return false
			}
			if contentMap["data"] == nil {
				return false
			}
		default:
			// Unknown content type
			return false
		}
	}

	return true
}

// validateContentArray checks and converts an array to a valid content array if possible.
// If the array contains valid content items or can be converted to valid content items,
// it returns the content array and true. Otherwise, it returns nil and false.
func validateContentArray(arr []interface{}) ([]map[string]interface{}, bool) {
	if len(arr) == 0 {
		return nil, false
	}

	contentItems := make([]map[string]interface{}, 0, len(arr))

	for _, item := range arr {
		if contentMap, ok := item.(map[string]interface{}); ok {
			// Verify it has a type field
			if contentType, hasType := contentMap["type"].(string); hasType {
				// Validate based on content type
				isValid := false

				switch contentType {
				case "text":
					if _, hasText := contentMap["text"].(string); hasText {
						isValid = true
					}
				case "image":
					if _, hasURL := contentMap["imageUrl"].(string); hasURL {
						isValid = true
					}
				case "link":
					if _, hasURL := contentMap["url"].(string); hasURL {
						isValid = true
					}
				case "file":
					if _, hasMime := contentMap["mimeType"].(string); hasMime && contentMap["data"] != nil {
						isValid = true
					}
				}

				if isValid {
					contentItems = append(contentItems, contentMap)
				}
			}
		}
	}

	if len(contentItems) > 0 {
		return contentItems, true
	}

	return nil, false
}

// extractSchemaFromHandler extracts a JSON Schema from a resource handler function.
// It analyzes the function's parameter structure and generates a schema
// that describes the expected input format. This is used to inform clients
// about the structure of arguments the resource expects.
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

	// For non-struct types, return a generic schema
	return map[string]interface{}{
		"type": "object",
	}, nil
}

// ConvertToResourceHandler converts a function to a ResourceHandler if possible.
// It uses reflection to validate the function signature and creates a wrapper
// that adapts the function to the ResourceHandler interface. Returns the converted
// handler and a boolean indicating success.
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

	// Get the second parameter type (args type)
	paramType := handlerType.In(1)

	// Create a resource handler that calls the original function
	resourceHandler := func(ctx *Context, args interface{}) (interface{}, error) {
		var argValue reflect.Value

		// If args is already the correct type, use it directly
		if reflect.TypeOf(args) == paramType {
			argValue = reflect.ValueOf(args)
		} else if mapArgs, ok := args.(map[string]interface{}); ok {
			// For map arguments going to a struct parameter, use a more robust conversion
			// Use the schema validation and conversion utility
			convertedArg, err := schema.ValidateAndConvertArgs(nil, mapArgs, paramType)
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

	return resourceHandler, true
}
