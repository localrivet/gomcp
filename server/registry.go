package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sync"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/schema"
	"github.com/mitchellh/mapstructure" // Import mapstructure

	"github.com/yosida95/uritemplate/v3" // Correct library import
	// Removed incorrect context import
	// "github.com/localrivet/gomcp/server/context"
)

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
	Pattern         string                      // The original URI pattern string, e.g., "weather://{city}/current"
	HandlerFn       any                         // The handler function itself
	Params          []resourceTemplateParamInfo // Information about extracted parameters
	Matcher         any                         // Placeholder for compiled pattern matcher (e.g., regex, custom)
	ContextArgIndex int                         // Index of the *server.Context argument (-1 if not present)
	// TODO: Add metadata like name, description if needed separate from handler
}

// Registry holds the registered tools, resources, and prompts.
type Registry struct {
	toolRegistry     map[string]protocol.Tool        // Use protocol.Tool
	toolHandlers     map[string]toolHandlerInfo      // Store tool handler info
	resourceRegistry map[string]protocol.Resource    // Use protocol.Resource
	templateRegistry map[string]resourceTemplateInfo // Map pattern string to template info
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
		resourceRegistry: make(map[string]protocol.Resource), // Use protocol.Resource
		templateRegistry: make(map[string]resourceTemplateInfo),
		promptRegistry:   make(map[string]protocol.Prompt),
		// Callbacks are not initialized here, set them using Set...ChangedCallback
	}
}

// RegisterResource registers a new resource with the registry.
func (r *Registry) RegisterResource(resource protocol.Resource) *Registry {
	r.registryMu.Lock()
	r.resourceRegistry[resource.URI] = resource
	r.registryMu.Unlock() // Unlock BEFORE calling the callback

	log.Printf("Registered resource: %s", resource.URI)

	// Call the callback if set, AFTER releasing the lock
	if r.resourceChangedCallback != nil {
		log.Printf("Calling resourceChangedCallback for %s", resource.URI)
		r.resourceChangedCallback(resource.URI)
	}
	return r
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

// UnregisterResource removes a resource from the registry by its URI.
func (r *Registry) UnregisterResource(uri string) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	delete(r.resourceRegistry, uri)
	log.Printf("Unregistered resource: %s", uri)
	// Call the callback if set
	if r.resourceChangedCallback != nil {
		r.resourceChangedCallback(uri)
	}
	return r
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

// ResourceRegistry returns the map of registered resources.
func (r *Registry) ResourceRegistry() map[string]protocol.Resource {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	return r.resourceRegistry
}

// GetResource retrieves a resource by its URI.
func (r *Registry) GetResource(uri string) (protocol.Resource, bool) {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	resource, ok := r.resourceRegistry[uri]
	return resource, ok
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
		inputSchema = schema.FromStruct(schemaArgsType) // Assign directly
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

	log.Printf("Registered tool: %s - %s", name, desc) // Placeholder

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
	defer r.registryMu.Unlock()
	r.promptRegistry[prompt.Title] = prompt // Assuming Title is the key for prompts
	log.Printf("Registered prompt: %s", prompt.Title)
	// Call the callback if set
	if r.promptChangedCallback != nil {
		log.Printf("DEBUG: Calling promptChangedCallback for prompt %s", prompt.Title)
		r.promptChangedCallback()
	}
	return r
}

// RemovePrompt removes a prompt from the registry.
func (r *Registry) RemovePrompt(title string) *Registry {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()
	delete(r.promptRegistry, title)
	log.Printf("Removed prompt: %s", title)
	// Call the callback if set
	if r.promptChangedCallback != nil {
		r.promptChangedCallback()
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
	// Simple linear scan for now. Consider map if performance becomes an issue.
	for _, p := range r.promptRegistry {
		if p.URI == uri {
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
	// retType := handlerInfo.RetType // Keep this line for potential future use if needed elsewhere - REMOVED

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
			var argsMap map[string]interface{}
			if len(rawArgs) > 0 {
				if err := json.Unmarshal(rawArgs, &argsMap); err != nil {
					log.Printf("Error unmarshalling raw arguments for tool %s: %v", name, err)
					callErr = fmt.Errorf("invalid arguments format: %w", err)
				} else {
					// Use mapstructure to decode the map into the struct
					decoderConfig := &mapstructure.DecoderConfig{
						Result:  argsInterface,
						TagName: "json",
					}
					decoder, err := mapstructure.NewDecoder(decoderConfig)
					if err != nil {
						log.Printf("Internal error creating argument decoder for tool %s: %v", name, err)
						callErr = fmt.Errorf("internal error creating decoder: %w", err)
					} else {
						if decodeErr := decoder.Decode(argsMap); decodeErr != nil {
							log.Printf("Error decoding arguments for tool %s: %v", name, decodeErr)
							callErr = fmt.Errorf("invalid arguments: %w", decodeErr)
						}
					}
				}
			} else { // Handle case where rawArgs is empty or null for struct type
				// Depending on the tool, this might be an error or might be okay if struct fields are optional/have defaults.
				// For now, assume empty struct is okay if input is empty.
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

// AddResourceTemplate registers a new resource template handler.
// It parses the URI pattern, validates the handler signature using reflection,
// and stores the information for later matching and invocation.
// Handler signature must be func(ctx types.Context, [param1 Type1, ...]) (ResultType, error).
func (r *Registry) AddResourceTemplate(uriPattern string, handlerFn any) error {
	r.registryMu.Lock()
	defer r.registryMu.Unlock()

	// 1. Validate URI Pattern
	tmpl, err := uritemplate.New(uriPattern)
	if err != nil {
		return fmt.Errorf("invalid URI template pattern '%s': %w", uriPattern, err)
	}
	if _, exists := r.templateRegistry[uriPattern]; exists {
		return fmt.Errorf("URI template pattern '%s' is already registered", uriPattern)
	}

	// 2. Validate Handler Function Signature
	handlerVal := reflect.ValueOf(handlerFn)
	handlerType := handlerVal.Type()

	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("handler for pattern '%s' is not a function", uriPattern)
	}

	// Check inputs: Must have at least Context
	numIn := handlerType.NumIn()
	if numIn == 0 {
		return fmt.Errorf("handler for pattern '%s' must accept at least context.Context or *server.Context as the first argument", uriPattern)
	}
	// Check first input type: Allow context.Context or *server.Context or interface{}
	firstArgType := handlerType.In(0)
	contextType := reflect.TypeOf((*Context)(nil))                   // *server.Context
	stdContextType := reflect.TypeOf((*context.Context)(nil)).Elem() // context.Context
	if firstArgType != contextType && firstArgType != stdContextType && firstArgType.Kind() != reflect.Interface {
		return fmt.Errorf("handler for pattern '%s' must accept context.Context, *server.Context, or interface{} as the first argument, got %s", uriPattern, firstArgType.String())
	}

	// Check outputs: Must be (result, error)
	if handlerType.NumOut() != 2 {
		return fmt.Errorf("handler for pattern '%s' must return exactly two values (result, error)", uriPattern)
	}
	if handlerType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("handler for pattern '%s' must return error as the second value", uriPattern)
	}

	// 3. Extract Parameter Info (Basic - relies on order for now due to reflection limits)
	paramInfos := make([]resourceTemplateParamInfo, numIn-1)
	templateVars := tmpl.Varnames() // Get expected var names from template

	if len(templateVars) != numIn-1 {
		// Mismatch between template vars and function args (excluding context)
		// This check might be too simple if optional params or different structures are used.
		return fmt.Errorf("handler for pattern '%s' has %d parameters (excluding Context), but template expects %d variables", uriPattern, numIn-1, len(templateVars))
	}

	for i := 1; i < numIn; i++ {
		// WARNING: Relies on order matching between template vars and func args!
		paramName := templateVars[i-1] // Map template var order to func arg order
		paramInfos[i-1] = resourceTemplateParamInfo{
			Name:         paramName,
			HandlerIndex: i,
			HandlerType:  handlerType.In(i),
		}
	}

	// 4. Store the template information
	info := resourceTemplateInfo{
		Pattern:         uriPattern,
		HandlerFn:       handlerFn,
		Params:          paramInfos,
		Matcher:         tmpl, // Store the compiled template for efficient matching
		ContextArgIndex: 0,    // We enforce context is the first arg
	}
	r.templateRegistry[uriPattern] = info

	log.Printf("Registered resource template: %s", uriPattern)

	// TODO: Add a templateChangedCallback if needed?

	return nil
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
func (r *Registry) TemplateRegistry() map[string]resourceTemplateInfo {
	r.registryMu.RLock()
	defer r.registryMu.RUnlock()
	// Return a copy? For now, return direct map for simplicity in handler.
	// Consider implications if handlers modify the returned map.
	return r.templateRegistry
}
