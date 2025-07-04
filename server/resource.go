package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/util/schema"
	"github.com/localrivet/wilduri"
)

// Resource represents a resource registered with the server.
// Resources are endpoints that clients can access to retrieve structured data.
type Resource struct {
	// Path is the URL path pattern for the resource
	Path string

	// Description explains what the resource provides
	Description string

	// Handler is the function that executes when the resource is accessed
	Handler interface{}

	// Schema defines the expected input format for the resource
	Schema interface{}

	// IsTemplate indicates if this is a template resource (path contains {})
	IsTemplate bool

	// Annotations contains additional metadata about the resource
	Annotations map[string]interface{}

	// Template is the parsed path template used for matching URLs
	Template *wilduri.Template
}

// Resource registers a resource with the server.
// The function returns the server instance to allow for method chaining.
// The path parameter defines the resource URL pattern, which can include parameters in {braces}.
// The description parameter provides human-readable documentation.
// The handler parameter must be a function with signature: func(ctx *Context, args *StructType) (interface{}, error)
// where StructType is a pointer to a struct (nillable).
// The handler parameter must be a function with signature:
//
//	func(ctx *Context, args *StructType) (interface{}, error)
//
// Where StructType is a pointer to a struct and can be nil. The schema is automatically
// extracted from the struct type using reflection and JSON tags.
//
// Path parameters are extracted from URI templates (e.g., /users/{id}) and
// JSON parameters come from request body. Use struct tags to map them:
//   - `path:"name"` for URI template parameters
//   - `json:"name"` for JSON body parameters
//
// Example:
//
//	func(ctx *Context, args *StructType) (interface{}, error)
func (s *serverImpl) Resource(path, description string, handler interface{}) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate handler is not nil
	if handler == nil {
		s.logger.Error("resource handler cannot be nil", "path", path)
		return s
	}

	// handler must be a function with the correct signature
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		s.logger.Error("resource handler must be a function with signature: func(ctx *Context, args interface{}) (interface{}, error)", "path", path)
		return s
	}

	// Validate handler signature and extract schema
	handlerFunc, schema, err := s.validateAndExtractResourceHandler(handler)
	if err != nil {
		s.logger.Error("invalid resource handler", "path", path, "error", err)
		return s
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
		Handler:     handlerFunc,
		Schema:      schema,
		Template:    template,
		IsTemplate:  isTemplate,
	}

	// Store the resource
	s.resources[path] = resource

	// Emit resource registration event
	go func() {
		events.Publish[events.ResourceRegisteredEvent](s.events, events.TopicResourceRegistered, events.ResourceRegisteredEvent{
			URI:          path,
			Name:         path,
			Description:  description,
			MimeType:     "application/octet-stream",
			RegisteredAt: time.Now(),
		})
	}()

	// Mark resources as changed for potential notifications
	s.capabilityCache.MarkResourcesChanged()

	// Release the lock before sending notification to avoid deadlock
	s.mu.Unlock()

	// Send simple notification if client is already initialized
	s.sendCapabilityNotification("resources")

	// Re-acquire lock to satisfy defer unlock
	s.mu.Lock()

	return s
}

// validateAndExtractResourceHandler validates a handler function and extracts its schema.
// The handler must have signature: func(ctx *Context, args interface{}) (interface{}, error)
func (s *serverImpl) validateAndExtractResourceHandler(handler interface{}) (interface{}, map[string]interface{}, error) {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	// Must be a function
	if handlerType.Kind() != reflect.Func {
		return nil, nil, errors.New("handler must be a function")
	}

	// Must have exactly 2 parameters and 2 return values
	if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
		return nil, nil, errors.New("handler must have signature: func(ctx *Context, args interface{}) (interface{}, error)")
	}

	// First parameter must be *Context
	if handlerType.In(0) != reflect.TypeOf((*Context)(nil)) {
		return nil, nil, errors.New("first parameter must be *Context")
	}

	// Second parameter must be interface{} for Handler type
	if handlerType.In(1) != reflect.TypeOf((*interface{})(nil)).Elem() {
		return nil, nil, errors.New("second parameter must be interface{}")
	}

	// First return value must be assignable to interface{}
	if !handlerType.Out(0).AssignableTo(reflect.TypeOf((*interface{})(nil)).Elem()) {
		return nil, nil, errors.New("first return value must be assignable to interface{}")
	}

	// Second return value must be error
	if !handlerType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, nil, errors.New("second return value must be error")
	}

	// Schema extraction from a concrete struct type is not possible if handler is already of type Handler.
	// Path parameter extraction from struct tags is also not applicable here.
	// Returning a generic schema.
	schemaMap := map[string]interface{}{
		"type": "object", // Generic schema
	}

	// Return the original handler as it's already the correct type
	return handler, schemaMap, nil
}

// setFieldValue sets a struct field value from an interface{}, handling type conversion
func setFieldValue(fieldValue reflect.Value, value interface{}) error {
	if value == nil {
		return nil
	}

	valueReflect := reflect.ValueOf(value)
	fieldType := fieldValue.Type()

	// Handle type conversion
	if valueReflect.Type().ConvertibleTo(fieldType) {
		fieldValue.Set(valueReflect.Convert(fieldType))
		return nil
	}

	// Handle string conversion for common types
	if valueReflect.Kind() == reflect.String && fieldType.Kind() == reflect.String {
		fieldValue.SetString(valueReflect.String())
		return nil
	}

	return fmt.Errorf("cannot convert %T to %s", value, fieldType)
}

// ProcessResourceSubscribe processes a resource subscription request.
// Resource subscriptions allow clients to receive notifications when resource data changes.
// Returns a response indicating whether the subscription was successful.
func (s *serverImpl) ProcessResourceSubscribe(ctx *Context) (interface{}, error) {
	// Parse the request parameters
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.URI == "" {
		return nil, fmt.Errorf("uri parameter is required")
	}

	// Get session ID from context
	sessionID, ok := ctx.Metadata["sessionID"].(string)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	// Add subscription to the session
	success := s.sessionManager.UpdateSession(SessionID(sessionID), func(session *ClientSession) {
		// Check if already subscribed
		for _, uri := range session.ResourceSubscriptions {
			if uri == params.URI {
				return // Already subscribed
			}
		}
		// Add new subscription
		session.ResourceSubscriptions = append(session.ResourceSubscriptions, params.URI)
	})

	if !success {
		return nil, fmt.Errorf("session not found")
	}

	s.logger.Debug("client subscribed to resource", "uri", params.URI, "sessionID", sessionID)

	return map[string]interface{}{}, nil
}

// ProcessResourceUnsubscribe processes a resource unsubscription request.
// This allows clients to stop receiving notifications for a previously subscribed resource.
// Returns a response indicating whether the unsubscription was successful.
func (s *serverImpl) ProcessResourceUnsubscribe(ctx *Context) (interface{}, error) {
	// Parse the request parameters
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.URI == "" {
		return nil, fmt.Errorf("uri parameter is required")
	}

	// Get session ID from context
	sessionID, ok := ctx.Metadata["sessionID"].(string)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	// Remove subscription from the session
	success := s.sessionManager.UpdateSession(SessionID(sessionID), func(session *ClientSession) {
		// Find and remove the subscription
		for i, uri := range session.ResourceSubscriptions {
			if uri == params.URI {
				// Remove by swapping with last element and truncating
				session.ResourceSubscriptions[i] = session.ResourceSubscriptions[len(session.ResourceSubscriptions)-1]
				session.ResourceSubscriptions = session.ResourceSubscriptions[:len(session.ResourceSubscriptions)-1]
				return
			}
		}
	})

	if !success {
		return nil, fmt.Errorf("session not found")
	}

	s.logger.Debug("client unsubscribed from resource", "uri", params.URI, "sessionID", sessionID)

	return map[string]interface{}{}, nil
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
	templates := make([]ResourceTemplateInfo, 0)
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
		templates = append(templates, ResourceTemplateInfo{
			URITemplate: resource.Path,
			Name:        name,
			Description: resource.Description,
			MimeType:    mimeType,
		})

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = path
			break
		}
	}

	// Return the list of resource templates using structured response
	return NewResourceTemplatesListResponse(templates, nextCursor), nil
}

// ensureContentsArray ensures a response has a properly formatted contents array.
// This function standardizes resource response format by ensuring the contents field
// follows the expected structure, with proper URI and content fields.
// Returns the properly formatted response.
// TODO: This function is currently unused but may be needed for future resource formatting
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
					// Ensure text field at contents level
					if contentMap["text"] == nil {
						// Find a suitable text value from the content items
						textFound := false
						for _, item := range contentItems {
							if contentItem, ok := item.(map[string]interface{}); ok {
								if contentItem["type"] == "text" && contentItem["text"] != nil {
									contentMap["text"] = contentItem["text"]
									textFound = true
									break
								}
							}
						}
						if !textFound {
							contentMap["text"] = "Content"
						}
					}
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
					// Ensure text field at contents level
					if contentMap["text"] == nil {
						contentMap["text"] = "Empty content"
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
				// Ensure text field at contents level
				if contentMap["text"] == nil {
					// Find a suitable text value from the content items
					textFound := false
					for _, item := range contentItems {
						if contentItem, ok := item.(map[string]interface{}); ok {
							if contentItem["type"] == "text" && contentItem["text"] != nil {
								contentMap["text"] = contentItem["text"]
								textFound = true
								break
							}
						}
					}
					if !textFound {
						contentMap["text"] = "Content"
					}
				}
			} else if contentItems, ok := contentMap["content"].([]map[string]interface{}); ok && len(contentItems) > 0 {
				// Convert []map[string]interface{} to []interface{} and validate
				interfaceItems := make([]interface{}, len(contentItems))
				for j, item := range contentItems {
					interfaceItems[j] = item
				}
				contentMap["content"] = ensureValidContentItems(interfaceItems)
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
				// Ensure text field at contents level
				if contentMap["text"] == nil {
					contentMap["text"] = "Empty content"
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
		} else if contentMapArr, ok := content.([]map[string]interface{}); ok {
			// Convert []map[string]interface{} to []interface{}
			interfaceArr := make([]interface{}, len(contentMapArr))
			for i, item := range contentMapArr {
				interfaceArr[i] = item
			}
			contentArray = ensureValidContentItems(interfaceArr)
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
// TODO: This function is currently unused but may be needed for future content validation
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
				// File must have a mimeType and either data or url field
				if _, hasMime := contentMap["mimeType"].(string); !hasMime {
					// Skip invalid file items
					continue
				}
				// File must have either data (embedded) or url (reference)
				hasData := contentMap["data"] != nil
				hasURL := contentMap["url"] != nil
				if !hasData && !hasURL {
					// Skip invalid file items
					continue
				}
				validItems = append(validItems, contentMap)

			case "blob":
				// Blob must have a blob field and mimeType
				if _, hasBlob := contentMap["blob"].(string); !hasBlob {
					// Skip invalid blob items
					continue
				}
				if _, hasMime := contentMap["mimeType"].(string); !hasMime {
					// Add default mimeType if missing
					contentMap["mimeType"] = "application/octet-stream"
				}
				validItems = append(validItems, contentMap)

			case "audio":
				// Audio must have either audioUrl (draft) or data (v20250326) field, plus mimeType
				hasAudioUrl := contentMap["audioUrl"] != nil
				hasData := contentMap["data"] != nil

				if !hasAudioUrl && !hasData {
					// Skip invalid audio items - need either audioUrl or data
					continue
				}

				if _, hasMime := contentMap["mimeType"].(string); !hasMime {
					// Add default mimeType if missing
					contentMap["mimeType"] = "audio/mpeg"
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

// ProcessResourceRequest handles resource requests from clients
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

	// Find the resource and extract parameters
	resource, pathParams, found := s.findResourceAndExtractParams(uri)
	if !found {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	// Publish resource access event
	startTime := time.Now()

	// Execute the resource handler
	// Convert pathParams to interface{} using schema validation
	var resourceArgs interface{}
	if len(pathParams) > 0 {
		// Get the handler's parameter type
		handlerType := reflect.TypeOf(resource.Handler)
		paramType := handlerType.In(1)

		// Validate and convert the arguments using schema package
		convertedArgs, err := schema.ValidateAndConvertArgs(resource.Schema.(map[string]interface{}), pathParams, paramType)
		if err != nil {
			// Publish failure event
			evt := events.ResourceAccessedEvent{
				URI:          uri,
				Method:       "resources/read",
				AccessedAt:   startTime,
				Success:      false,
				ErrorMessage: fmt.Sprintf("invalid resource arguments: %v", err),
			}

			if err := events.Publish[events.ResourceAccessedEvent](s.events, events.TopicResourceAccessed, evt); err != nil {
				s.logger.Debug("failed to publish resource accessed event", "error", err)
			}

			return nil, fmt.Errorf("invalid resource arguments: %w", err)
		}

		// Convert to interface{}
		if convertedArgs != nil {
			resourceArgs = convertedArgs
		}
	}

	// Call the handler using reflection since it's stored as interface{}
	handlerValue := reflect.ValueOf(resource.Handler)

	// Prepare arguments for the call
	args := []reflect.Value{
		reflect.ValueOf(ctx),
	}

	// Handle the resourceArgs parameter - if nil, pass a proper zero value for interface{}
	if resourceArgs != nil {
		args = append(args, reflect.ValueOf(resourceArgs))
	} else {
		// Create a zero value for interface{} type
		args = append(args, reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem()))
	}

	// Call the handler
	results := handlerValue.Call(args)

	// Extract the results
	var result interface{}
	var err error

	// Handle first return value (result) - check if it can be nil before calling IsNil()
	if results[0].IsValid() {
		if results[0].CanInterface() {
			// Only check IsNil for nillable types (pointers, interfaces, maps, slices, channels, functions)
			kind := results[0].Kind()
			if (kind == reflect.Ptr || kind == reflect.Interface || kind == reflect.Map ||
				kind == reflect.Slice || kind == reflect.Chan || kind == reflect.Func) && results[0].IsNil() {
				// Value is nil, leave result as nil
			} else {
				result = results[0].Interface()
			}
		}
	}

	// Handle second return value (error) - check if it can be nil before calling IsNil()
	if results[1].IsValid() {
		if results[1].CanInterface() {
			// Only check IsNil for nillable types
			kind := results[1].Kind()
			if (kind == reflect.Ptr || kind == reflect.Interface || kind == reflect.Map ||
				kind == reflect.Slice || kind == reflect.Chan || kind == reflect.Func) && results[1].IsNil() {
				// Error is nil, leave err as nil
			} else {
				err = results[1].Interface().(error)
			}
		}
	}

	// Publish resource access completion event
	evt := events.ResourceAccessedEvent{
		URI:        uri,
		Method:     "resources/read",
		AccessedAt: startTime,
		Success:    err == nil,
	}

	if err != nil {
		evt.ErrorMessage = err.Error()
		// Publish failure event
		if publishErr := events.Publish[events.ResourceAccessedEvent](s.events, events.TopicResourceAccessed, evt); publishErr != nil {
			s.logger.Debug("failed to publish resource accessed event", "error", publishErr)
		}
		return nil, fmt.Errorf("resource handler error: %w", err)
	}

	// Publish success event (ignore errors to avoid affecting resource access)
	if publishErr := events.Publish[events.ResourceAccessedEvent](s.events, events.TopicResourceAccessed, evt); publishErr != nil {
		s.logger.Debug("failed to publish resource accessed event", "error", publishErr)
	}

	// Use the public FormatResourceResponse function for consistent formatting
	version := ctx.Version
	if version == "" {
		// Default to latest protocol version
		version = "2025-03-26"
	}

	return FormatResourceResponse(uri, result, version), nil
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
	resources := make([]ResourceInfo, 0)
	var nextCursor string

	// Convert resources to the expected format
	i := 0
	for path, resource := range s.resources {
		// Skip template resources - they should only appear in resources/templates/list
		if resource.IsTemplate {
			continue
		}

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
		resourceInfo := ResourceInfo{
			URI:         resource.Path,
			Name:        name,
			Description: resource.Description,
			MimeType:    mimeType,
		}

		resources = append(resources, resourceInfo)

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = path
			break
		}
	}

	// Return the list of resources using structured response
	return NewResourceListResponse(resources, nextCursor), nil
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

// SendResourcesListChangedNotification sends a notification to inform clients that the resource list has changed.
// This is called when resources are added, removed, or updated, allowing clients to refresh their available resources.
func (s *serverImpl) SendResourcesListChangedNotification() error {
	// Create the notification using structured type
	notification := mcp.NewNotification("notifications/resources/list_changed", nil)

	// Marshal the notification to JSON
	notificationBytes, err := notification.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Check if the server is initialized (minimize lock scope)
	s.mu.RLock()
	initialized := s.initialized
	transport := s.transport
	s.mu.RUnlock()

	// If the server is not initialized, queue the notification for later
	if !initialized {
		s.capabilityCache.QueueNotification(notificationBytes)
		s.logger.Debug("queued resources/list_changed notification for after initialization")
		return nil
	}

	// Send the notification through the configured transport (no mutex needed for this)
	if transport != nil {
		if err := transport.Send(notificationBytes); err != nil {
			s.logger.Error("failed to send notification", "error", err)
			return fmt.Errorf("failed to send notification: %w", err)
		}
	} else {
		s.logger.Warn("no transport configured, skipping notification")
	}

	s.logger.Debug("sent resources/list_changed notification")
	return nil
}
