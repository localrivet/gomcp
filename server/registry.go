package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/schema"
	"github.com/localrivet/wilduri"
	"github.com/mitchellh/mapstructure"
)

// registeredResource holds the processed configuration and state for a static resource.
// Content might be loaded lazily.
type registeredResource struct {
	config resourceConfig // Store the original configuration

	// Potentially pre-loaded content or other derived data
	// For now, we rely mostly on config, but this allows for future optimization.
}

// ToProtocolResource converts the internal registeredResource representation
// to the public protocol.Resource struct, primarily using the stored config.
func (rr *registeredResource) ToProtocolResource(uri string) protocol.Resource {
	// Determine the resource kind based on config
	kind := "static" // Default
	if rr.config.FilePath != "" {
		// For files, try to determine kind from file extension
		if strings.HasSuffix(rr.config.FilePath, ".mp3") ||
			strings.HasSuffix(rr.config.FilePath, ".wav") ||
			strings.HasSuffix(rr.config.FilePath, ".ogg") ||
			strings.HasSuffix(rr.config.FilePath, ".audio") || // Special test extension
			(rr.config.MimeType != "" && strings.HasPrefix(rr.config.MimeType, "audio/")) {
			kind = "audio"
		} else {
			kind = "file"
		}
	} else if rr.config.DirPath != "" {
		kind = "directory"
	} else if rr.config.URL != "" {
		kind = "url"
	} else if rr.config.MimeType != "" && strings.HasPrefix(rr.config.MimeType, "audio/") {
		kind = "audio"
	} else if rr.config.Content != nil {
		// Try to infer kind from content type if available
		switch rr.config.Content.(type) {
		case []byte:
			kind = "binary"
		case string:
			kind = "text"
		}
	}

	// Populate metadata from config
	meta := make(map[string]interface{}) // Use map[string]interface{} as defined in protocol.Resource
	if rr.config.MimeType != "" {
		meta["mimeType"] = rr.config.MimeType
	}
	if len(rr.config.Tags) > 0 {
		meta["tags"] = rr.config.Tags // Store tags directly in metadata
	}
	// Merge custom annotations from config into metadata
	for key, value := range rr.config.Annotations {
		if _, exists := meta[key]; !exists { // Avoid overwriting mimeType/tags if keys clash
			meta[key] = value
		} else {
			// Handle potential key clashes if necessary (e.g., log a warning)
			log.Printf("Warning: Annotation key '%s' clashes with standard metadata key during resource conversion for %s", key, uri)
		}
	}

	// protocol.Annotations struct seems specific (Audience, Priority), not for general annotations.
	// We'll use the Metadata field for mimeType, tags, and custom annotations.
	protoAnnotations := protocol.Annotations{}

	return protocol.Resource{
		URI:         uri,
		Name:        rr.config.Name,
		Description: rr.config.Description,
		Kind:        kind, // Now uses dynamic kind based on content type
		Metadata:    meta,
		Annotations: protoAnnotations, // Use empty specific Annotations struct
		// Version:     "", // TODO: Implement content hashing for version
		// Size:        nil, // TODO: Implement size calculation
	}
}

// toolHandlerInfo stores information about a registered tool handler.
type toolHandlerInfo struct {
	Fn       any
	ArgsType reflect.Type // Type of the arguments struct
	RetType  reflect.Type // Type of the return value
}

// resourceTemplateParamInfo stores details about a parameter extracted from a URI template.
type resourceTemplateParamInfo struct {
	Name         string       // Name from the URI template, e.g., "city"
	HandlerIndex int          // Index of the corresponding argument in the handler function
	HandlerType  reflect.Type // Expected type in the handler function
}

// resourceTemplateInfo stores information about a registered resource template handler.
type resourceTemplateInfo struct {
	config          resourceConfig              // Store the configuration used to create this template
	Pattern         string                      // The original URI pattern string, e.g., "weather://{city}/current"
	HandlerFn       any                         // The handler function itself (redundant with config.HandlerFn? Keep for now)
	Params          []resourceTemplateParamInfo // Information about extracted parameters
	Matcher         *wilduri.Template           // Use the compiled wilduri template object
	ContextArgIndex int                         // Index of the *server.Context argument (-1 if not present)
}

// Registry holds the registered tools, resources, and prompts.
type Registry struct {
	toolRegistry     map[string]protocol.Tool         // Use protocol.Tool
	toolHandlers     map[string]toolHandlerInfo       // Store tool handler info
	resourceRegistry map[string]*registeredResource   // Map URI to internal static resource representation
	templateRegistry map[string]*resourceTemplateInfo // Map pattern string to template info (Pointer now)
	promptRegistry   map[string]protocol.Prompt

	promptChangedCallback   func()           // Callback to notify when prompts change
	resourceChangedCallback func(uri string) // Callback to notify when resources change
	toolChangedCallback     func()           // Callback to notify when tools change

	registryMu sync.RWMutex
}

// NewRegistry creates a new Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		toolRegistry:     make(map[string]protocol.Tool), // Use protocol.Tool
		toolHandlers:     make(map[string]toolHandlerInfo),
		resourceRegistry: make(map[string]*registeredResource),   // Initialize new map type
		templateRegistry: make(map[string]*resourceTemplateInfo), // Initialize new map type
		promptRegistry:   make(map[string]protocol.Prompt),
		// Callbacks are not initialized here, set them using Set...ChangedCallback
	}
}

// SetResourceChangedCallback sets the callback function to be called when resources change.
func (r *Registry) SetResourceChangedCallback(callback func(uri string)) {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	r.resourceChangedCallback = callback
}

// SetToolChangedCallback sets the callback function to be called when tools change.
func (r *Registry) SetToolChangedCallback(callback func()) {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	r.toolChangedCallback = callback
}

// SetPromptChangedCallback sets the callback function to be called when prompts change.
func (r *Registry) SetPromptChangedCallback(callback func()) {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	r.promptChangedCallback = callback
}

// AddRoot registers a new root with the registry.
func (r *Registry) AddRoot(root protocol.Root) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	// Assuming roots are stored in a separate map or slice in Registry
	// For now, just logging
	log.Printf("Registered root: %s", root.URI)
	return r
}

// RemoveRoot removes a root from the registry.
func (r *Registry) RemoveRoot(root protocol.Root) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	log.Printf("Removed root: %s", root.URI)
	return r
}

// ResourceRegistry returns the map of registered resources (static ones).
// Note: This currently only returns static resources registered via the new API.
// Templates are handled separately.
func (r *Registry) ResourceRegistry() map[string]protocol.Resource {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()

	// Create a new map to hold the protocol.Resource versions
	publicRegistry := make(map[string]protocol.Resource)
	for uri, regRes := range r.resourceRegistry {
		if regRes != nil { // Basic nil check
			publicRegistry[uri] = regRes.ToProtocolResource(uri) // Pass URI
		}
	}
	return publicRegistry
}

// GetResource retrieves a resource by its URI.
// It currently only checks static resources registered via the new API.
func (r *Registry) GetResource(uri string) (protocol.Resource, bool) {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	regRes, ok := r.resourceRegistry[uri]
	if !ok || regRes == nil {
		return protocol.Resource{}, false // Return empty struct if not found
	}
	// Convert the internal representation to the protocol type
	return regRes.ToProtocolResource(uri), true // Pass URI
}

// GetRegisteredResource retrieves the internal registered resource by its URI.
// This is primarily for internal use to access the resource config.
func (r *Registry) GetRegisteredResource(uri string) (*registeredResource, bool) {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	regRes, ok := r.resourceRegistry[uri]
	return regRes, ok
}

// Tool registers a new tool with the registry (non-generic method).
// The fn parameter should be a function with signature func(Context, Args) (Ret, error).
// This method stores the tool information and handler.
func (r *Registry) Tool(
	name, desc string,
	fn any, // Accept function as any
) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()

	// Use reflection to get the function's type
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		log.Printf("Error registering tool %s: handler is not a function", name)
		return r // Or return an error
	}
	// Expecting signature func(Context, Args) (Ret, error)
	if fnType.NumIn() != 2 || fnType.NumOut() != 2 {
		log.Printf("Error registering tool %s: handler function must have 2 inputs (Context, Args) and 2 outputs (result, error)", name)
		return r // Or return an error
	}
	// Refer to Context directly as it's in the same package
	if fnType.In(0) != reflect.TypeOf((*Context)(nil)) {
		log.Printf("Error registering tool %s: handler function first input must be Context", name)
		return r // Or return an error
	}
	if fnType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		log.Printf("Error registering tool %s: handler function second output must be error", name)
		return r // Or return an error
	}

	argsType := fnType.In(1) // Args is the second input
	retType := fnType.Out(0) // Ret is the first output

	var inputSchema protocol.ToolInputSchema // Use the correct type
	// Check if the second argument is json.RawMessage
	isRawMessageHandler := argsType == reflect.TypeOf((*json.RawMessage)(nil)).Elem()

	if !isRawMessageHandler {
		// Generate input schema from the Args struct type if it's not json.RawMessage
		// Ensure ArgsType is a struct or pointer to struct for schema generation
		schemaArgsType := argsType
		if schemaArgsType.Kind() == reflect.Ptr {
			schemaArgsType = schemaArgsType.Elem()
		}
		if schemaArgsType.Kind() != reflect.Struct {
			log.Printf("Error registering tool %s: handler function second input must be Context and either json.RawMessage or a struct/pointer-to-struct", name)
			return r // Or return an error
		}
		// Create a new instance of the struct type to pass to FromStruct
		structInstance := reflect.New(schemaArgsType).Elem().Interface()
		inputSchema = schema.FromStruct(structInstance) // Pass struct instance, not type
	} else {
		log.Printf("Registering tool %s with json.RawMessage input type. Input schema will be null.", name)
		// For raw message handlers, input schema is effectively null/any
		// Assign an empty or default ToolInputSchema struct
		inputSchema = protocol.ToolInputSchema{} // Assign empty struct
	}

	// Store tool info and handler info
	r.toolRegistry[name] = protocol.Tool{
		Name:        name,
		Description: desc,
		InputSchema: inputSchema,
		// TODO: Add Annotations if needed
	}
	r.toolHandlers[name] = toolHandlerInfo{
		Fn:       fn,
		ArgsType: argsType,
		RetType:  retType,
	}

	log.Printf("Registered tool: %s - %s %+v", name, desc, inputSchema) // Placeholder

	// Call the callback if set
	if r.toolChangedCallback != nil {
		// Unlock first? No, keep lock during callback for consistency?
		// Let's keep it simple for now, call under lock like others.
		r.toolChangedCallback()
	}
	return r
}

// RemoveTool removes a tool from the registry.
func (r *Registry) RemoveTool(name string) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	delete(r.toolRegistry, name)
	delete(r.toolHandlers, name) // Also remove handler info
	log.Printf("Removed tool: %s", name)

	// Call the callback if set
	if r.toolChangedCallback != nil {
		r.toolChangedCallback()
	}
	return r
}

// AddPrompt registers a new prompt with the registry.
func (r *Registry) AddPrompt(prompt protocol.Prompt) *Registry {
	r.registryMu.Lock()
	// Use prompt.URI as key if not empty, otherwise use Name
	promptKey := prompt.URI
	if promptKey == "" {
		promptKey = prompt.Name
	}
	r.promptRegistry[promptKey] = prompt // Using Name as the key
	log.Printf("Registered prompt: %s", prompt.Name)

	// Call the callback if set, AFTER releasing the lock
	callback := r.promptChangedCallback
	r.registryMu.Unlock() // Explicitly unlock before callback
	if callback != nil {
		log.Printf("DEBUG: Calling promptChangedCallback for prompt %s", prompt.Name)
		callback()
	}

	return r
}

// RemovePrompt removes a prompt from the registry by name.
func (r *Registry) RemovePrompt(name string) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock() // Will unlock after function returns

	delete(r.promptRegistry, name)
	log.Printf("Removed prompt: %s", name)

	// Call the callback if set, but after releasing the lock
	if r.promptChangedCallback != nil {
		// Need to release the lock before calling the callback to avoid deadlocks
		r.registryMu.Unlock()
		r.promptChangedCallback()
		r.registryMu.Lock() // Re-lock to maintain the deferred unlock pattern
	}

	return r
}

// GetPrompts returns a slice of all registered prompts.
func (r *Registry) GetPrompts() []protocol.Prompt {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()

	prompts := make([]protocol.Prompt, 0, len(r.promptRegistry))
	for _, prompt := range r.promptRegistry {
		prompts = append(prompts, prompt)
	}
	return prompts
}

// GetPrompt retrieves a specific prompt by its URI.
func (r *Registry) GetPrompt(uri string) (protocol.Prompt, bool) {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()

	// First, try to look up directly in the map using the URI as the key
	if prompt, exists := r.promptRegistry[uri]; exists {
		return prompt, true
	}

	// If not found by direct lookup, try linear search checking both URI and Name fields
	for _, p := range r.promptRegistry {
		if p.URI == uri || p.Name == uri {
			return p, true
		}
	}

	return protocol.Prompt{}, false
}

// GetToolHandler retrieves the wrapper handler for a registered tool.
// This wrapper handles argument parsing/validation and calls the original function.
// It now accepts a pre-created Context instance.
func (r *Registry) GetToolHandler(name string) (func(ctx *Context, rawArgs json.RawMessage) (interface{}, error), bool) { // Return interface{}
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()

	handlerInfo, ok := r.toolHandlers[name] // Get handler info
	if !ok {
		return nil, false
	}

	originalFn := reflect.ValueOf(handlerInfo.Fn)
	argsType := handlerInfo.ArgsType

	// Create the wrapper function
	wrapper := func(ctx *Context, rawArgs json.RawMessage) (interface{}, error) { // Return interface{} and error
		log.Printf("Executing tool wrapper for: %s with raw args: %s", name, string(rawArgs))

		var inputs []reflect.Value
		var callErr error

		// Check if the handler expects json.RawMessage or a specific struct
		if argsType == reflect.TypeOf((*json.RawMessage)(nil)).Elem() {
			// Handler expects raw message, pass it directly
			inputs = []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(rawArgs)}
		} else {
			// Handler expects a struct, perform unmarshalling
			argsPtrValue := reflect.New(argsType)
			argsInterface := argsPtrValue.Interface()

			if len(rawArgs) > 0 {
				// Parse rawArgs into a map first
				var argsMap map[string]interface{}
				if mapErr := json.Unmarshal(rawArgs, &argsMap); mapErr != nil {
					log.Printf("Error unmarshalling raw arguments for tool %s: %v", name, mapErr)
					callErr = fmt.Errorf("invalid arguments format: %w", mapErr)
				} else {
					// Configure mapstructure for case-insensitive field matching
					decoderConfig := &mapstructure.DecoderConfig{
						Result:           argsInterface,
						TagName:          "json",
						WeaklyTypedInput: true, // Allow type conversions (string to int, etc)
						DecodeHook: mapstructure.ComposeDecodeHookFunc(
							mapstructure.StringToTimeHookFunc(time.RFC3339),
							mapstructure.StringToSliceHookFunc(","), // Handle comma-separated strings as slices
						),
						// Enable case-insensitive matching
						MatchName: func(mapKey, fieldName string) bool {
							// Check exact match first
							if mapKey == fieldName {
								return true
							}
							// Fall back to case-insensitive matching
							return strings.EqualFold(mapKey, fieldName)
						},
						ZeroFields:  true,  // Set fields to zero value before decoding
						ErrorUnused: false, // Don't error on unused fields (more forgiving)
						Squash:      true,  // Squash embedded structs
					}

					// Try to decode with case-insensitive behavior
					decoder, err := mapstructure.NewDecoder(decoderConfig)
					if err != nil {
						log.Printf("Internal error creating argument decoder for tool %s: %v", name, err)
						callErr = fmt.Errorf("internal error creating decoder: %w", err)
					} else if decodeErr := decoder.Decode(argsMap); decodeErr != nil {
						log.Printf("Error decoding arguments for tool %s: %v", name, decodeErr)

						// Fall back to direct JSON unmarshaling as a last resort
						if jsonErr := json.Unmarshal(rawArgs, argsInterface); jsonErr != nil {
							callErr = fmt.Errorf("invalid arguments: %w", decodeErr)
						}
					}
				}
			} else {
				// Handle case where rawArgs is empty or null for struct type
				log.Printf("Tool %s called with empty/null input for struct type %s", name, argsType.String())
			}
			inputs = []reflect.Value{reflect.ValueOf(ctx), argsPtrValue.Elem()} // Pass context and decoded/empty struct
		}

		// Return decoding error immediately if it occurred
		if callErr != nil {
			return nil, callErr // Return nil result and the decoding error
		}

		// 2. Call the original tool handler function using reflection
		if len(inputs) != originalFn.Type().NumIn() {
			return nil, fmt.Errorf("internal error: mismatch in expected (%d) and provided (%d) arguments for tool '%s'", originalFn.Type().NumIn(), len(inputs), name)
		}

		results := originalFn.Call(inputs)

		// 3. Handle the function's return values (result and error)
		resultValue := results[0]
		errValue := results[1]

		var returnedErr error
		if errValue.Interface() != nil {
			err, ok := errValue.Interface().(error)
			if !ok {
				return nil, fmt.Errorf("tool '%s' returned a non-error value in the error position", name)
			}
			log.Printf("Tool %s returned an error: %v", name, err)
			returnedErr = err
		}

		// Return the actual result interface{} and the error
		return resultValue.Interface(), returnedErr
	}

	return wrapper, true
}

// GetTools returns a slice of all registered tools (protocol.Tool definition).
func (r *Registry) GetTools() []protocol.Tool {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	tools := make([]protocol.Tool, 0, len(r.toolRegistry))
	for _, tool := range r.toolRegistry {
		tools = append(tools, tool)
	}
	return tools
}

// RegisterResourceTemplate registers a new resource template handler based on the provided config.
// It parses the URI pattern, validates the handler signature using reflection,
// and stores the information for later matching and invocation.
// Handler signature must be func(ctx types.Context, [param1 Type1, ...]) (ResultType, error).
func (r *Registry) RegisterResourceTemplate(uriPattern string, config resourceConfig) error {
	r.registryMu.Lock()
	// Defer unlock until after callback might be called

	// 1. Validate URI Pattern
	tmpl, err := wilduri.New(uriPattern)
	if err != nil {
		r.registryMu.Unlock()
		return fmt.Errorf("invalid URI template pattern '%s': %w", uriPattern, err)
	}
	if _, exists := r.templateRegistry[uriPattern]; exists {
		// Handle duplicate based on config
		switch config.duplicateHandling {
		case DuplicateError:
			r.registryMu.Unlock()
			return fmt.Errorf("URI template pattern '%s' is already registered", uriPattern)
		case DuplicateIgnore:
			log.Printf("Ignoring duplicate template registration for URI pattern: %s (keeping existing)", uriPattern)
			r.registryMu.Unlock()
			return nil
		case DuplicateWarn:
			log.Printf("Warning: Overwriting existing resource template registration for URI pattern: %s", uriPattern)
			// Continue with replacement
		case DuplicateReplace:
			// Just replace silently
		default:
			// Default behavior: Warn and replace
			log.Printf("Warning: Overwriting existing resource template registration for URI pattern: %s", uriPattern)
		}
	}

	// 2. Validate Handler Function Signature
	handlerFn := config.HandlerFn
	if handlerFn == nil {
		r.registryMu.Unlock()
		return fmt.Errorf("handler function is nil for template pattern '%s'", uriPattern)
	}
	handlerVal := reflect.ValueOf(handlerFn)
	handlerType := handlerVal.Type()

	if handlerType.Kind() != reflect.Func {
		r.registryMu.Unlock()
		return fmt.Errorf("handler for pattern '%s' is not a function", uriPattern)
	}

	// Check inputs: Must have at least Context
	numIn := handlerType.NumIn()
	if numIn == 0 {
		r.registryMu.Unlock()
		return fmt.Errorf("handler for pattern '%s' must accept at least context.Context or *server.Context as the first argument", uriPattern)
	}
	// Check first input type: Allow context.Context or *server.Context or interface{}
	firstArgType := handlerType.In(0)
	contextType := reflect.TypeOf((*Context)(nil))                   // *server.Context
	stdContextType := reflect.TypeOf((*context.Context)(nil)).Elem() // context.Context
	if firstArgType != contextType && firstArgType != stdContextType && firstArgType.Kind() != reflect.Interface {
		r.registryMu.Unlock()
		return fmt.Errorf("handler for pattern '%s' must accept context.Context, *server.Context, or interface{} as the first argument, got %s", uriPattern, firstArgType.String())
	}

	// Check outputs: Must be (result, error)
	if handlerType.NumOut() != 2 {
		r.registryMu.Unlock()
		return fmt.Errorf("handler for pattern '%s' must return exactly two values (result, error)", uriPattern)
	}
	if handlerType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		r.registryMu.Unlock()
		return fmt.Errorf("handler for pattern '%s' must return error as the second value", uriPattern)
	}

	// 3. Extract Parameter Info
	paramInfos := make([]resourceTemplateParamInfo, 0, numIn-1)
	templateVars := tmpl.Varnames() // Get expected var names from template

	// Check if wildcards are defined
	wildcardVars := make(map[string]bool)
	for varName, isWildcard := range config.wildcardParams {
		if isWildcard {
			wildcardVars[varName] = true
		}
	}

	// In Phase 3, we support wildcards and default values, so param count might not match exactly
	// Count non-wildcard variables to check if handler args match
	nonWildcardVars := 0
	for _, varName := range templateVars {
		if !wildcardVars[varName] {
			nonWildcardVars++
		}
	}

	// Handler must have at least enough args for all non-wildcard params
	if numIn-1 < nonWildcardVars {
		r.registryMu.Unlock()
		return fmt.Errorf("handler for pattern '%s' has %d parameters (excluding Context), but template expects at least %d non-wildcard variables", uriPattern, numIn-1, nonWildcardVars)
	}

	// Map template variables to handler arguments
	// We'll create a parameter info entry for each argument
	for i := 1; i < numIn; i++ {
		// Map function args to template vars by position
		// This is simplified and could be improved with explicit mapping
		varIndex := i - 1
		var paramName string
		if varIndex < len(templateVars) {
			paramName = templateVars[varIndex]
		} else {
			// If more args than vars, use position-based naming
			paramName = fmt.Sprintf("param%d", varIndex)
		}

		paramInfos = append(paramInfos, resourceTemplateParamInfo{
			Name:         paramName,
			HandlerIndex: i,
			HandlerType:  handlerType.In(i),
		})
	}

	// 4. Store the template information
	config.template = tmpl // Store the compiled template in the config
	info := resourceTemplateInfo{
		config:          config, // Store the full config
		Pattern:         uriPattern,
		HandlerFn:       handlerFn,
		Params:          paramInfos,
		Matcher:         tmpl, // Store the compiled template for efficient matching
		ContextArgIndex: 0,    // We enforce context is the first arg
	}

	// Use custom key if provided, otherwise use pattern
	registryKey := uriPattern
	if config.customKey != "" {
		registryKey = config.customKey
	}

	r.templateRegistry[registryKey] = &info

	// Register additional URIs if specified
	for _, additionalPattern := range config.additionalURIs {
		// Validate additional pattern
		additionalTmpl, err := wilduri.New(additionalPattern)
		if err != nil {
			log.Printf("Skipping invalid additional URI template pattern '%s': %v", additionalPattern, err)
			continue
		}

		// Create a copy of the info with the new pattern and matcher
		additionalInfo := resourceTemplateInfo{
			config:          config,
			Pattern:         additionalPattern,
			HandlerFn:       handlerFn,
			Params:          paramInfos,
			Matcher:         additionalTmpl,
			ContextArgIndex: 0,
		}
		r.templateRegistry[additionalPattern] = &additionalInfo
	}

	log.Printf("Registered resource template: %s", uriPattern)

	// Call the callback AFTER releasing the lock
	callback := r.resourceChangedCallback
	r.registryMu.Unlock() // Unlock before calling callback
	if callback != nil {
		// For templates, maybe we don't notify with a specific URI?
		// Or should we notify with the pattern URI?
		// Let's notify with the pattern URI for now.
		log.Printf("Calling resourceChangedCallback for template pattern %s", uriPattern)
		callback(uriPattern)

		// Also call callback for additional patterns
		for _, additionalPattern := range config.additionalURIs {
			callback(additionalPattern)
		}
	}

	return nil
}

// AddResourceTemplate is a backward compatibility wrapper for RegisterResourceTemplate.
// It maintains the old signature for compatibility with existing code.
// DEPRECATED: Use Resource(uri, WithHandler(handlerFn)) instead.
func (r *Registry) AddResourceTemplate(uriPattern string, handlerFn any) error {
	// Create a minimal resourceConfig with just the handler function
	config := resourceConfig{
		HandlerFn: handlerFn,
	}
	// Delegate to the new method
	return r.RegisterResourceTemplate(uriPattern, config)
}

// parseSimpleTemplateParams extracts parameter names from within {} in a URI template.
// This is a basic implementation and doesn't handle complex RFC 6570 features.
func parseSimpleTemplateParams(pattern string) []string {
	var params []string
	start := -1
	for i, r := range pattern {
		if r == '{' {
			start = i
		} else if r == '}' && start != -1 {
			params = append(params, pattern[start+1:i])
			start = -1 // Reset start after finding a closing brace
		}
	}
	return params
}

// TemplateRegistry returns the map of registered resource templates.
func (r *Registry) TemplateRegistry() map[string]*resourceTemplateInfo {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	// Return a copy? For now, return direct map for simplicity in handler.
	// Consider implications if handlers modify the returned map.
	return r.templateRegistry
}

// RegisterStaticResource registers a new static resource based on the provided config.
func (r *Registry) RegisterStaticResource(uri string, config resourceConfig) error {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()

	// Basic validation: Check for duplicate URI
	if _, exists := r.resourceRegistry[uri]; exists {
		// Handle duplicate registrations based on config
		switch config.duplicateHandling {
		case DuplicateError:
			r.registryMu.Unlock()
			return fmt.Errorf("resource URI already registered: %s", uri)
		case DuplicateIgnore:
			log.Printf("Ignoring duplicate resource registration for URI: %s (keeping existing)", uri)
			r.registryMu.Unlock()
			return nil
		case DuplicateWarn:
			log.Printf("Warning: Overwriting existing resource registration for URI: %s", uri)
			// Continue with replacement
		case DuplicateReplace:
			// Just replace silently
		default:
			// Default behavior: Warn and replace
			log.Printf("Warning: Overwriting existing resource registration for URI: %s", uri)
		}
	}

	// TODO: Add more config validation (e.g., ensure only one content source type is provided)

	regRes := &registeredResource{
		config: config, // Store the passed config
	}

	// Use custom key if provided, otherwise use URI
	registryKey := uri
	if config.customKey != "" {
		registryKey = config.customKey
	}

	r.resourceRegistry[registryKey] = regRes

	// Register additional URIs if specified
	for _, additionalURI := range config.additionalURIs {
		r.resourceRegistry[additionalURI] = regRes
	}

	log.Printf("Registered static resource: %s", uri)

	// Call the callback if set, AFTER releasing the lock (deferred unlock handles this)
	// We need to call it outside the lock to prevent deadlocks if the callback tries to access the registry.
	// But the lock is held until the function returns due to defer.
	// Let's capture the callback and call it after unlock.
	callback := r.resourceChangedCallback
	r.registryMu.Unlock() // Explicitly unlock before callback
	if callback != nil {
		log.Printf("Calling resourceChangedCallback for %s", uri)
		callback(uri)

		// Also call callback for additional URIs
		for _, additionalURI := range config.additionalURIs {
			callback(additionalURI)
		}
	}
	r.registryMu.Lock() // Re-lock just before defer would unlock again (bit awkward, consider alternatives)

	return nil
}

// RegisterResource registers a new resource with the registry.
// DEPRECATED: Use Resource(uri, WithTextContent(...)) or Resource(uri, WithFileContent(...)) instead.
func (r *Registry) RegisterResource(resource protocol.Resource) *Registry {
	r.registryMu.Lock()

	// Convert protocol.Resource to the new resourceConfig + registeredResource format
	config := resourceConfig{
		Name:        resource.Name, // Now using Name directly
		Description: resource.Description,
		// Extract MimeType from metadata if present
		// Extract any annotations/tags if needed
	}

	// Get MimeType from metadata if present
	if resource.Metadata != nil {
		if mimeType, ok := resource.Metadata["mimeType"].(string); ok {
			config.MimeType = mimeType
		}
	}

	// Create and store the registered resource
	r.resourceRegistry[resource.URI] = &registeredResource{
		config: config,
	}

	r.registryMu.Unlock() // Unlock BEFORE calling the callback

	log.Printf("Registered resource (legacy method): %s", resource.URI)

	// Call the callback if set, AFTER releasing the lock
	if r.resourceChangedCallback != nil {
		log.Printf("Calling resourceChangedCallback for %s", resource.URI)
		r.resourceChangedCallback(resource.URI)
	}
	return r
}
