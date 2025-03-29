package main

import (
	"context" // Needed for simulating JSON in filesystem tool response
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	mcp "github.com/localrivet/gomcp" // Import our library
	// "github.com/google/uuid" // Keep commented for now
)

// Define constants for clarity
const (
	ToolEcho          = "echo"
	ToolAdd           = "add"
	ToolLongRunning   = "longRunningOperation"
	ToolGetTinyImage  = "getTinyImage"
	PromptSimple      = "mcp://example.com/prompts/simple"
	PromptComplex     = "mcp://example.com/prompts/complex"
	ResourceStaticURI = "file:///tmp/static_resource.txt" // Example static file URI
)

// Base64 encoded 1x1 transparent PNG
const MCP_TINY_IMAGE_BASE64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

// --- In-Memory Storage (for resources/prompts that need state) ---
var (
	// Store prompts by URI for the custom GetPrompt handler logic
	promptStore = make(map[string]mcp.Prompt)
	promptMu    sync.RWMutex

	// Store resource content and version by URI for custom ReadResource and update simulation
	resourceContentStore = make(map[string]string)
	resourceVersionStore = make(map[string]string)
	resourceMu           sync.RWMutex
)

// --- Tool Handlers ---

func handleEcho(ctx context.Context, progressToken *mcp.ProgressToken, arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolEcho)
	// Example: Check for cancellation
	if ctx.Err() != nil {
		log.Println("Echo tool cancelled")
		return []mcp.Content{mcp.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}

	message, ok := arguments["message"].(string)
	if !ok {
		errContent := mcp.TextContent{Type: "text", Text: "Error: Invalid or missing 'message' argument (string expected)"}
		return []mcp.Content{errContent}, true // Indicate error
	}
	respContent := mcp.TextContent{Type: "text", Text: fmt.Sprintf("Echo: %s", message)}
	return []mcp.Content{respContent}, false
}

func handleAdd(ctx context.Context, progressToken *mcp.ProgressToken, arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolAdd)
	// Example: Check for cancellation
	if ctx.Err() != nil {
		log.Println("Add tool cancelled")
		return []mcp.Content{mcp.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}

	a, ok1 := arguments["a"].(float64) // JSON numbers decode to float64
	b, ok2 := arguments["b"].(float64)
	if !ok1 || !ok2 {
		errContent := mcp.TextContent{Type: "text", Text: "Error: Invalid or missing 'a' or 'b' arguments (number expected)"}
		return []mcp.Content{errContent}, true // Indicate error
	}
	sum := a + b
	respContent := mcp.TextContent{Type: "text", Text: fmt.Sprintf("The sum of %f and %f is %f.", a, b, sum)}
	return []mcp.Content{respContent}, false
}

// handleLongRunning needs access to the server instance to send progress
func handleLongRunning(server *mcp.Server) mcp.ToolHandlerFunc {
	// Use a closure to capture the server instance needed for SendProgress
	return func(ctx context.Context, progressToken *mcp.ProgressToken, arguments map[string]interface{}) (content []mcp.Content, isError bool) {
		log.Printf("[Handler] Received call for %s", ToolLongRunning)

		durationVal, ok := arguments["duration"].(float64)
		if !ok {
			durationVal = 5.0 // Default duration reduced for quicker testing
		}
		stepsVal, ok := arguments["steps"].(float64)
		if !ok || stepsVal <= 0 {
			stepsVal = 5.0 // Default steps
		}

		duration := time.Duration(durationVal * float64(time.Second))
		steps := int(stepsVal)
		stepDuration := duration / time.Duration(steps)

		log.Printf("Starting long operation: %v total, %d steps, %v per step. Progress Token: %v", duration, steps, stepDuration, progressToken)

		for i := 1; i <= steps; i++ {
			select {
			case <-ctx.Done(): // Check for cancellation
				log.Printf("Long operation cancelled at step %d", i)
				errContent := mcp.TextContent{Type: "text", Text: fmt.Sprintf("Operation cancelled by client at step %d", i)}
				return []mcp.Content{errContent}, true // Indicate error
			case <-time.After(stepDuration):
				log.Printf("Long operation step %d/%d complete", i, steps)
				if progressToken != nil {
					// Send progress notification
					progressPayload := map[string]interface{}{
						"message":  fmt.Sprintf("Completed step %d of %d", i, steps),
						"progress": i,
						"total":    steps,
					}
					progParams := mcp.ProgressParams{Token: string(*progressToken), Value: progressPayload}
					err := server.SendProgress(progParams)
					if err != nil {
						log.Printf("Warning: failed to send progress update for token %s: %v", *progressToken, err)
					} else {
						log.Printf("Sent progress update for token %s: step %d/%d", *progressToken, i, steps)
					}
				}
			}
		}

		resultText := fmt.Sprintf("Long running operation finished. Duration: %v, Steps: %d.", duration, steps)
		respContent := mcp.TextContent{Type: "text", Text: resultText}
		return []mcp.Content{respContent}, false
	}
}

func handleGetTinyImage(ctx context.Context, progressToken *mcp.ProgressToken, arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolGetTinyImage)
	// Example: Check for cancellation
	if ctx.Err() != nil {
		log.Println("GetTinyImage tool cancelled")
		return []mcp.Content{mcp.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}

	imgContent := mcp.ImageContent{
		Type:      "image",
		Data:      MCP_TINY_IMAGE_BASE64, // Use the base64 constant
		MediaType: "image/png",
		Annotations: &mcp.ContentAnnotations{
			Title: StringPtr("MCP Logo Tiny"), // Use helper for pointer
		},
	}
	textBefore := mcp.TextContent{Type: "text", Text: "This is a tiny image:"}
	textAfter := mcp.TextContent{Type: "text", Text: "The image above is the MCP tiny image."}

	return []mcp.Content{textBefore, imgContent, textAfter}, false
}

// --- Custom Handlers (Illustrative - Not directly registered in current server design) ---
// These demonstrate how custom handlers *could* work if the library allowed overriding built-ins.
// For now, the server uses its internal logic based on the registries for list/get/read/subscribe.

// handleGetPromptCustom demonstrates logic for retrieving and templating a prompt.
func handleGetPromptCustom(s *mcp.Server, requestID interface{}, params interface{}) error {
	log.Println("[Handler] Custom GetPrompt logic executing")
	var requestParams mcp.GetPromptRequestParams
	if err := mcp.UnmarshalPayload(params, &requestParams); err != nil {
		return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
			Code: mcp.ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal GetPrompt params: %v", err),
		})
	}

	promptMu.RLock()
	promptTmpl, exists := promptStore[requestParams.URI]
	promptMu.RUnlock()

	if !exists {
		log.Printf("Prompt template not found: %s", requestParams.URI)
		return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
			Code:    mcp.ErrorCodeMCPResourceNotFound,
			Message: fmt.Sprintf("Prompt template not found: %s", requestParams.URI),
		})
	}

	// --- Basic Templating Logic ---
	renderedMessages := make([]mcp.PromptMessage, len(promptTmpl.Messages))

	// Create a map of provided arguments, checking against definition
	providedArgs := make(map[string]string)
	if requestParams.Arguments != nil {
		for _, argDef := range promptTmpl.Arguments {
			if val, ok := requestParams.Arguments[argDef.Name]; ok {
				if strVal, okStr := val.(string); okStr {
					providedArgs[argDef.Name] = strVal
				} else {
					log.Printf("Warning: Argument '%s' for prompt '%s' expected string, got %T", argDef.Name, requestParams.URI, val)
					// Handle type mismatch - for simplicity, convert to string
					providedArgs[argDef.Name] = fmt.Sprintf("%v", val)
				}
			} else if argDef.Required {
				return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
					Code:    mcp.ErrorCodeInvalidParams,
					Message: fmt.Sprintf("Missing required argument '%s' for prompt '%s'", argDef.Name, requestParams.URI),
				})
			}
		}
	} else {
		// Check if any arguments were required but none provided
		for _, argDef := range promptTmpl.Arguments {
			if argDef.Required {
				return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
					Code:    mcp.ErrorCodeInvalidParams,
					Message: fmt.Sprintf("Missing required argument '%s' for prompt '%s'", argDef.Name, requestParams.URI),
				})
			}
		}
	}

	// Iterate through messages and content to render templates
	for i := range renderedMessages { // Iterate using index to modify the slice
		msg := promptTmpl.Messages[i]                            // Get the original message template
		renderedContent := make([]mcp.Content, len(msg.Content)) // Create the slice for the new message
		for j, contentItem := range msg.Content {
			if textContent, ok := contentItem.(mcp.TextContent); ok {
				renderedText := textContent.Text // Start with original text
				// Replace ${var}
				for name, val := range providedArgs {
					placeholder := fmt.Sprintf("${%s}", name)
					renderedText = strings.ReplaceAll(renderedText, placeholder, val)
				}
				// Replace ${var:-default} for missing args
				for _, argDef := range promptTmpl.Arguments {
					if _, provided := providedArgs[argDef.Name]; !provided {
						defaultPlaceholderPrefix := fmt.Sprintf("${%s:-", argDef.Name)
						startIdx := strings.Index(renderedText, defaultPlaceholderPrefix)
						if startIdx != -1 {
							endIdx := strings.Index(renderedText[startIdx:], "}")
							if endIdx != -1 {
								defaultValue := ""
								// Check if there is a default value between :- and }
								if endIdx > len(defaultPlaceholderPrefix) {
									defaultValue = renderedText[startIdx+len(defaultPlaceholderPrefix) : startIdx+endIdx]
								}
								fullPlaceholder := renderedText[startIdx : startIdx+endIdx+1]
								renderedText = strings.ReplaceAll(renderedText, fullPlaceholder, defaultValue)
							}
						} else {
							// Also remove simple ${var} if not provided and no default
							placeholder := fmt.Sprintf("${%s}", argDef.Name)
							renderedText = strings.ReplaceAll(renderedText, placeholder, "")
						}
					}
				}
				// Assign the *new* TextContent to the correct index in renderedContent
				renderedContent[j] = mcp.TextContent{
					Type:        textContent.Type,
					Text:        renderedText,
					Annotations: textContent.Annotations, // Preserve annotations
				}
			} else {
				renderedContent[j] = contentItem // Keep non-text content as is
			}
		}
		// Assign the fully rendered content slice to the message in the renderedMessages slice
		renderedMessages[i] = mcp.PromptMessage{Role: msg.Role, Content: renderedContent}
	}
	// --- End Templating Logic ---

	// Create the final prompt result with rendered messages
	responsePrompt := promptTmpl               // Copy base prompt info
	responsePrompt.Messages = renderedMessages // Use the rendered messages

	responsePayload := mcp.GetPromptResult{Prompt: responsePrompt}
	log.Printf("Sending GetPromptResponse for URI: %s", requestParams.URI)
	return s.Conn().SendResponse(requestID, responsePayload)
}

// handleReadResourceCustom demonstrates logic for reading a resource.
func handleReadResourceCustom(s *mcp.Server, requestID interface{}, params interface{}) error {
	log.Println("[Handler] Custom ReadResource logic executing")
	var requestParams mcp.ReadResourceRequestParams
	if err := mcp.UnmarshalPayload(params, &requestParams); err != nil {
		return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
			Code: mcp.ErrorCodeInvalidParams, Message: fmt.Sprintf("Failed to unmarshal ReadResource params: %v", err),
		})
	}

	resourceMu.RLock()
	content, contentExists := resourceContentStore[requestParams.URI]
	currentVersion, versionExists := resourceVersionStore[requestParams.URI]
	resourceMu.RUnlock()

	// Get metadata from registry
	registry := s.ResourceRegistry() // Use getter
	resourceMeta, metaExists := registry[requestParams.URI]

	if !contentExists || !metaExists || !versionExists {
		log.Printf("Resource not found or missing data: %s", requestParams.URI)
		return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
			Code:    mcp.ErrorCodeMCPResourceNotFound,
			Message: fmt.Sprintf("Resource not found: %s", requestParams.URI),
		})
	}

	// Check version if provided by client
	if requestParams.Version != "" && requestParams.Version != currentVersion {
		log.Printf("Version mismatch for resource %s: requested %s, current %s", requestParams.URI, requestParams.Version, currentVersion)
		return s.Conn().SendErrorResponse(requestID, mcp.ErrorPayload{
			Code:    mcp.ErrorCodeInvalidParams, // Or a more specific version mismatch code
			Message: fmt.Sprintf("Version mismatch for resource %s. Requested: %s, Current: %s", requestParams.URI, requestParams.Version, currentVersion),
		})
	}

	// Update the version in the metadata copy before sending
	resourceMeta.Version = currentVersion

	// Determine content type (simplified)
	var resourceContents mcp.ResourceContents
	contentType := "application/octet-stream" // Default
	if ctMeta, ok := resourceMeta.Metadata["contentType"].(string); ok {
		contentType = ctMeta
	} else if resourceMeta.Kind == "file" {
		// Basic guess based on extension (very rudimentary)
		if strings.HasSuffix(resourceMeta.URI, ".txt") {
			contentType = "text/plain"
		} else if strings.HasSuffix(resourceMeta.URI, ".json") {
			contentType = "application/json"
		}
	}

	if strings.HasPrefix(contentType, "text/") || contentType == "application/json" || contentType == "application/xml" {
		resourceContents = mcp.TextResourceContents{
			ContentType: contentType,
			Content:     content,
		}
	} else {
		// Assume blob for others in this example
		// In a real scenario, you'd likely base64 encode binary file content here
		resourceContents = mcp.BlobResourceContents{
			ContentType: contentType,
			Blob:        content, // Assuming content is already base64 encoded if not text
		}
	}

	responsePayload := mcp.ReadResourceResult{
		Resource: resourceMeta,
		Contents: resourceContents,
	}
	log.Printf("Sending ReadResourceResponse for URI: %s", requestParams.URI)
	return s.Conn().SendResponse(requestID, responsePayload)
}

// --- Notification Handlers (Client -> Server) ---
func handleClientNotification(params interface{}) {
	log.Printf("Received notification from client with params: %+v", params)
}

// --- Main Function ---
func main() {
	// Configure logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Kitchen Sink MCP Server...")

	// Create a new server instance
	server := mcp.NewServer("GoMCPKitchenSinkServer")

	// --- Register Tools ---
	log.Println("Registering tools...")
	echoTool := mcp.Tool{
		Name:        ToolEcho,
		Description: "Echoes back the input message.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]mcp.PropertyDetail{
				"message": {Type: "string", Description: "The message to echo back."},
			},
			Required: []string{"message"},
		},
	}
	if err := server.RegisterTool(echoTool, handleEcho); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolEcho, err)
	}

	addTool := mcp.Tool{
		Name:        ToolAdd,
		Description: "Adds two numbers.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]mcp.PropertyDetail{
				"a": {Type: "number", Description: "First number."},
				"b": {Type: "number", Description: "Second number."},
			},
			Required: []string{"a", "b"},
		},
	}
	if err := server.RegisterTool(addTool, handleAdd); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolAdd, err)
	}

	longRunningTool := mcp.Tool{
		Name:        ToolLongRunning,
		Description: "A long running operation that reports progress.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]mcp.PropertyDetail{
				"duration": {Type: "number", Description: "Duration in seconds (default 10)."},
				"steps":    {Type: "number", Description: "Number of steps (default 5)."},
			},
		},
		Annotations: mcp.ToolAnnotations{ // Example annotation
			Title: "Long Runner", // Title is string
		},
	}
	// Pass the server instance to the handler factory using a closure
	if err := server.RegisterTool(longRunningTool, handleLongRunning(server)); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolLongRunning, err)
	}

	getTinyImageTool := mcp.Tool{
		Name:        ToolGetTinyImage,
		Description: "Returns a tiny base64 encoded PNG image.",
		InputSchema: mcp.ToolInputSchema{Type: "object"}, // No input arguments
		Annotations: mcp.ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
	}
	if err := server.RegisterTool(getTinyImageTool, handleGetTinyImage); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolGetTinyImage, err)
	}

	// --- Register Resources ---
	log.Println("Registering resources...")
	staticResource := mcp.Resource{
		URI:         ResourceStaticURI,
		Kind:        "file", // Could be "text/plain" or more specific
		Title:       "Static Test Resource",
		Description: "A simple text resource provided by the server.",
		Version:     "v1.0", // Initial version
		Metadata:    map[string]interface{}{"encoding": "utf-8", "contentType": "text/plain"},
	}
	// Store initial content and version for the static resource
	resourceMu.Lock()
	resourceContentStore[ResourceStaticURI] = "Initial content of the static resource."
	resourceVersionStore[ResourceStaticURI] = staticResource.Version
	resourceMu.Unlock()

	if err := server.RegisterResource(staticResource); err != nil {
		log.Fatalf("Failed to register resource %s: %v", ResourceStaticURI, err)
	}

	// --- Register Prompts ---
	log.Println("Registering prompts...")
	simplePrompt := mcp.Prompt{
		URI:         PromptSimple,
		Title:       "Simple Prompt",
		Description: "A basic prompt with no arguments.",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Tell me a short joke."}}},
		},
	}
	promptMu.Lock()
	promptStore[PromptSimple] = simplePrompt // Store for potential custom handler use
	promptMu.Unlock()
	if err := server.RegisterPrompt(simplePrompt); err != nil { // Register metadata with server
		log.Fatalf("Failed to register prompt %s: %v", PromptSimple, err)
	}

	complexPrompt := mcp.Prompt{
		URI:         PromptComplex,
		Title:       "Complex Prompt",
		Description: "A prompt demonstrating arguments and image content.",
		Arguments: []mcp.PromptArgument{
			{Name: "topic", Type: "string", Description: "The topic to discuss.", Required: true},
			{Name: "style", Type: "string", Description: "The desired writing style (e.g., formal, casual)."},
		},
		Messages: []mcp.PromptMessage{
			{Role: "system", Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "You are a helpful assistant."}}},
			{Role: "user", Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "Please explain ${topic} in a ${style:-casual} style."},
				mcp.ImageContent{Type: "image", Data: MCP_TINY_IMAGE_BASE64, MediaType: "image/png"},
			}},
		},
	}
	promptMu.Lock()
	promptStore[PromptComplex] = complexPrompt // Store for potential custom handler use
	promptMu.Unlock()
	if err := server.RegisterPrompt(complexPrompt); err != nil { // Register metadata with server
		log.Fatalf("Failed to register prompt %s: %v", PromptComplex, err)
	}

	// --- Register Notification Handlers (Client -> Server) ---
	// Example: server.RegisterNotificationHandler(mcp.MethodProgress, handleClientNotification)

	// --- Register Request Handlers (Server -> Client) ---
	// Note: Overriding built-in handlers like get/read requires library changes or a different approach.
	// The provided handleGetPromptCustom and handleReadResourceCustom are illustrative
	// but won't be called by the current server.Run() dispatch logic for those methods.
	log.Println("Note: Using built-in handlers for list/subscribe/unsubscribe based on registries.")
	log.Println("Note: Built-in handlers for get/read are currently stubs and will return 'Not Found'.") // Add clarification

	// --- Simulate Resource Updates ---
	go simulateResourceUpdates(server) // Start the simulation in a goroutine

	log.Println("Server setup complete. Starting Run loop...")
	// Run the server's main loop
	err := server.Run() // This blocks until connection closes or error
	if err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server finished.")
}

// --- Helper Functions ---

// BoolPtr returns a pointer to a boolean value.
func BoolPtr(b bool) *bool {
	return &b
}

// StringPtr returns a pointer to a string value.
// Keep this as ContentAnnotations.Title is *string
func StringPtr(s string) *string {
	return &s
}

// simulateResourceUpdates periodically updates the static resource and notifies subscribed clients.
func simulateResourceUpdates(s *mcp.Server) {
	ticker := time.NewTicker(30 * time.Second) // Update every 30 seconds
	defer ticker.Stop()
	versionCounter := 1
	for range ticker.C {
		resourceMu.Lock()
		// Check if the resource still exists in the content store
		_, contentExists := resourceContentStore[ResourceStaticURI]
		if contentExists {
			newVersion := fmt.Sprintf("v1.%d", versionCounter)
			newContent := fmt.Sprintf("Updated content at %s. Version: %s", time.Now().Format(time.RFC3339), newVersion)
			resourceContentStore[ResourceStaticURI] = newContent
			resourceVersionStore[ResourceStaticURI] = newVersion

			// Get the latest resource metadata from the registry using the getter
			// Note: This assumes the resource metadata itself doesn't change frequently.
			// A more robust implementation might fetch/update the full Resource struct
			// or have a dedicated way to update just the version in the registry.
			registry := s.ResourceRegistry() // Use getter
			updatedResourceMeta, metaExists := registry[ResourceStaticURI]
			if metaExists {
				updatedResourceMeta.Version = newVersion // Update the version in the copy
				resourceMu.Unlock()                      // Unlock before notifying

				log.Printf("Simulating update for resource %s to version %s", ResourceStaticURI, newVersion)
				s.NotifyResourceUpdated(updatedResourceMeta) // Notify subscribed clients
			} else {
				log.Printf("Warning: Could not find resource %s in registry for update notification.", ResourceStaticURI)
				resourceMu.Unlock()
			}
		} else {
			// Resource might have been unregistered
			log.Printf("Resource %s no longer in content store, stopping updates.", ResourceStaticURI)
			resourceMu.Unlock()
			return // Stop simulating updates for this resource
		}
		versionCounter++
	}
}
