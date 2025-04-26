package main

import (
	"context" // Needed for handler
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// Needed for handler signature
	// Removed unused imports: errors, io, strings, stdio
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
	promptStore = make(map[string]protocol.Prompt)
	promptMu    sync.RWMutex

	resourceContentStore = make(map[string]string)
	resourceVersionStore = make(map[string]string)
	resourceMu           sync.RWMutex
)

// --- Tool Handlers ---

func handleEcho(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolEcho)
	if ctx.Err() != nil {
		log.Println("Echo tool cancelled")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'echo' (expected object)"}}, true
	}

	message, ok := argsMap["message"].(string)
	if !ok {
		errContent := protocol.TextContent{Type: "text", Text: "Error: Invalid or missing 'message' argument (string expected)"}
		return []protocol.Content{errContent}, true
	}
	respContent := protocol.TextContent{Type: "text", Text: fmt.Sprintf("Echo: %s", message)}
	return []protocol.Content{respContent}, false
}

func handleAdd(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolAdd)
	if ctx.Err() != nil {
		log.Println("Add tool cancelled")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'add' (expected object)"}}, true
	}

	a, ok1 := argsMap["a"].(float64)
	b, ok2 := argsMap["b"].(float64)
	if !ok1 || !ok2 {
		errContent := protocol.TextContent{Type: "text", Text: "Error: Invalid or missing 'a' or 'b' arguments (number expected)"}
		return []protocol.Content{errContent}, true
	}
	sum := a + b
	respContent := protocol.TextContent{Type: "text", Text: fmt.Sprintf("The sum of %f and %f is %f.", a, b, sum)}
	return []protocol.Content{respContent}, false
}

// handleLongRunning - Note: Progress reporting requires server/session access, removed for simplicity.
func handleLongRunning(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolLongRunning)

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'longRunningOperation' (expected object)"}}, true
	}

	durationVal, ok := argsMap["duration"].(float64)
	if !ok {
		durationVal = 5.0
	}
	stepsVal, ok := argsMap["steps"].(float64)
	if !ok || stepsVal <= 0 {
		stepsVal = 5.0
	}

	duration := time.Duration(durationVal * float64(time.Second))
	steps := int(stepsVal)
	stepDuration := duration / time.Duration(steps)

	log.Printf("Starting long operation: %v total, %d steps, %v per step.", duration, steps, stepDuration)

	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			log.Printf("Long operation cancelled at step %d", i)
			errContent := protocol.TextContent{Type: "text", Text: fmt.Sprintf("Operation cancelled by client at step %d", i)}
			return []protocol.Content{errContent}, true
		case <-time.After(stepDuration):
			log.Printf("Long operation step %d/%d complete", i, steps)
			// Progress reporting removed as server/session access is not available here.
			// if progressToken != nil {
			// 	log.Printf("Progress update (token %s): step %d/%d (SendProgress call commented out)", *progressToken, i, steps)
			// }
		}
	}

	resultText := fmt.Sprintf("Long running operation finished. Duration: %v, Steps: %d.", duration, steps)
	respContent := protocol.TextContent{Type: "text", Text: resultText}
	return []protocol.Content{respContent}, false
}

func handleGetTinyImage(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
	log.Printf("[Handler] Received call for %s", ToolGetTinyImage)
	if ctx.Err() != nil {
		log.Println("GetTinyImage tool cancelled")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}

	imgContent := protocol.ImageContent{
		Type:      "image",
		Data:      MCP_TINY_IMAGE_BASE64,
		MediaType: "image/png",
		Annotations: &protocol.ContentAnnotations{
			Title: StringPtr("MCP Logo Tiny"),
		},
	}
	textBefore := protocol.TextContent{Type: "text", Text: "This is a tiny image:"}
	textAfter := protocol.TextContent{Type: "text", Text: "The image above is the MCP tiny image."}

	return []protocol.Content{textBefore, imgContent, textAfter}, false
}

// --- Main Function ---
func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Kitchen Sink MCP Server...")

	// Create server with default options
	srv := server.NewServer("GoMCPKitchenSinkServer",
		// Use functional option to set capabilities
		server.WithServerCapabilities(protocol.ServerCapabilities{
			Experimental: map[string]interface{}{"progress": true}, // Example capability
			// Ensure other default capabilities are implicitly set by NewServer
			// or explicitly set here if needed.
		}),
	)

	// --- Register Tools ---
	log.Println("Registering tools...")
	echoTool := protocol.Tool{
		Name:        ToolEcho,
		Description: "Echoes back the input message.",
		InputSchema: protocol.ToolInputSchema{
			Type: "object",
			Properties: map[string]protocol.PropertyDetail{
				"message": {Type: "string", Description: "The message to echo back."},
			},
			Required: []string{"message"},
		},
	}
	if err := srv.RegisterTool(echoTool, handleEcho); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolEcho, err)
	}

	addTool := protocol.Tool{
		Name:        ToolAdd,
		Description: "Adds two numbers.",
		InputSchema: protocol.ToolInputSchema{
			Type: "object",
			Properties: map[string]protocol.PropertyDetail{
				"a": {Type: "number", Description: "First number."},
				"b": {Type: "number", Description: "Second number."},
			},
			Required: []string{"a", "b"},
		},
	}
	if err := srv.RegisterTool(addTool, handleAdd); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolAdd, err)
	}

	longRunningTool := protocol.Tool{
		Name:        ToolLongRunning,
		Description: "A long running operation that reports progress.",
		InputSchema: protocol.ToolInputSchema{
			Type: "object",
			Properties: map[string]protocol.PropertyDetail{
				"duration": {Type: "number", Description: "Duration in seconds (default 5)."},
				"steps":    {Type: "number", Description: "Number of steps (default 5)."},
			},
		},
		Annotations: protocol.ToolAnnotations{
			Title: "Long Runner",
		},
	}
	// Pass the simplified handler directly
	if err := srv.RegisterTool(longRunningTool, handleLongRunning); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolLongRunning, err)
	}

	getTinyImageTool := protocol.Tool{
		Name:        ToolGetTinyImage,
		Description: "Returns a tiny base64 encoded PNG image.",
		InputSchema: protocol.ToolInputSchema{Type: "object"}, // No input arguments
		Annotations: protocol.ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
	}
	if err := srv.RegisterTool(getTinyImageTool, handleGetTinyImage); err != nil {
		log.Fatalf("Failed to register tool %s: %v", ToolGetTinyImage, err)
	}

	// --- Register Resources ---
	log.Println("Registering resources...")
	staticResource := protocol.Resource{
		URI:         ResourceStaticURI,
		Kind:        "file",
		Title:       "Static Test Resource",
		Description: "A simple text resource provided by the server.",
		Version:     "v1.0",
		Metadata:    map[string]interface{}{"encoding": "utf-8", "contentType": "text/plain"},
	}
	resourceMu.Lock()
	resourceContentStore[ResourceStaticURI] = "Initial content of the static resource."
	resourceVersionStore[ResourceStaticURI] = staticResource.Version
	resourceMu.Unlock()
	if err := srv.RegisterResource(staticResource); err != nil {
		log.Fatalf("Failed to register resource %s: %v", ResourceStaticURI, err)
	}

	// --- Register Prompts ---
	log.Println("Registering prompts...")
	simplePrompt := protocol.Prompt{
		URI:         PromptSimple,
		Title:       "Simple Prompt",
		Description: "A basic prompt with no arguments.",
		Messages: []protocol.PromptMessage{
			{Role: "user", Content: []protocol.Content{protocol.TextContent{Type: "text", Text: "Tell me a short joke."}}},
		},
	}
	promptMu.Lock()
	promptStore[PromptSimple] = simplePrompt
	promptMu.Unlock()
	if err := srv.RegisterPrompt(simplePrompt); err != nil {
		log.Fatalf("Failed to register prompt %s: %v", PromptSimple, err)
	}

	complexPrompt := protocol.Prompt{
		URI:         PromptComplex,
		Title:       "Complex Prompt",
		Description: "A prompt demonstrating arguments and image content.",
		Arguments: []protocol.PromptArgument{
			{Name: "topic", Type: "string", Description: "The topic to discuss.", Required: true},
			{Name: "style", Type: "string", Description: "The desired writing style (e.g., formal, casual)."},
		},
		Messages: []protocol.PromptMessage{
			{Role: "system", Content: []protocol.Content{protocol.TextContent{Type: "text", Text: "You are a helpful assistant."}}},
			{Role: "user", Content: []protocol.Content{
				protocol.TextContent{Type: "text", Text: "Please explain ${topic} in a ${style:-casual} style."},
				protocol.ImageContent{Type: "image", Data: MCP_TINY_IMAGE_BASE64, MediaType: "image/png"},
			}},
		},
	}
	promptMu.Lock()
	promptStore[PromptComplex] = complexPrompt
	promptMu.Unlock()
	if err := srv.RegisterPrompt(complexPrompt); err != nil {
		log.Fatalf("Failed to register prompt %s: %v", PromptComplex, err)
	}

	// --- Simulate Resource Updates ---
	go simulateResourceUpdates(srv)

	// --- Run Server ---
	log.Println("Server setup complete. Starting Run loop via ServeStdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}

// --- Helper Functions ---

func BoolPtr(b bool) *bool       { return &b }
func StringPtr(s string) *string { return &s }

func simulateResourceUpdates(s *server.Server) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	versionCounter := 1
	for range ticker.C {
		resourceMu.Lock()
		_, contentExists := resourceContentStore[ResourceStaticURI]
		if contentExists {
			newVersion := fmt.Sprintf("v1.%d", versionCounter)
			newContent := fmt.Sprintf("Updated content at %s. Version: %s", time.Now().Format(time.RFC3339), newVersion)
			resourceContentStore[ResourceStaticURI] = newContent
			resourceVersionStore[ResourceStaticURI] = newVersion

			registry := s.ResourceRegistry()
			updatedResourceMeta, metaExists := registry[ResourceStaticURI]
			if metaExists {
				updatedResourceMeta.Version = newVersion
				resourceMu.Unlock()

				log.Printf("Simulating update for resource %s to version %s", ResourceStaticURI, newVersion)
				s.NotifyResourceUpdated(updatedResourceMeta)
			} else {
				log.Printf("Warning: Could not find resource %s in registry for update notification.", ResourceStaticURI)
				resourceMu.Unlock()
			}
		} else {
			log.Printf("Resource %s no longer in content store, stopping updates.", ResourceStaticURI)
			resourceMu.Unlock()
			return
		}
		versionCounter++
	}
}
