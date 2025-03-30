// examples/server/main.go (Refactored)
// This is the main file for the example MCP server.
// It demonstrates how to use the refactored gomcp.Server from the library:
// 1. Create an gomcp.Server.
// 2. Define tool logic as functions matching gomcp.ToolHandlerFunc.
// 3. Register tools using server.RegisterTool.
// 4. Run the server using server.Run(), which handles handshake and message dispatch.
package main

import (
	// Keep fmt for error messages if needed by tool handlers
	"context"
	"log"
	"os"

	"github.com/localrivet/gomcp"
	// "strings" // No longer needed here
	// Import root package
)

// --- Tool Definitions ---
// Definitions remain the same.
// See calculator.go and filesystem.go for the other definitions.

// Define the echo tool (simple tool defined directly in main)
var echoTool = gomcp.Tool{ // Use new Tool struct
	Name:        "echo",
	Description: "Echoes back the provided message.",
	InputSchema: gomcp.ToolInputSchema{ // InputSchema remains similar
		Type: "object",
		Properties: map[string]gomcp.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
	// OutputSchema is removed in the new spec for Tool definition
	// Annotations: gomcp.ToolAnnotations{}, // Optional annotations
}

// calculatorToolDefinition is defined in calculator.go
// fileSystemToolDefinition is defined in filesystem.go

// --- Tool Handler Functions ---
// These functions now match the gomcp.ToolHandlerFunc signature.
// They take arguments and return ([]Content, isError bool).

// echoHandler implements the logic for the echo tool.
func echoHandler(ctx context.Context, progressToken *gomcp.ProgressToken, arguments map[string]interface{}) (content []gomcp.Content, isError bool) {
	log.Printf("Executing echo tool with args: %v", arguments)
	// Example: Check for cancellation
	if ctx.Err() != nil {
		log.Println("Echo tool cancelled")
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}
	messageArg, ok := arguments["message"]
	if !ok {
		// Return error content and isError=true
		errorContent := gomcp.TextContent{Type: "text", Text: "Missing required argument 'message' for tool 'echo'"}
		return []gomcp.Content{errorContent}, true
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		errorContent := gomcp.TextContent{Type: "text", Text: "Argument 'message' for tool 'echo' must be a string"}
		return []gomcp.Content{errorContent}, true
	}
	log.Printf("Echoing message: %s", messageStr)
	// Return success content and isError=false
	successContent := gomcp.TextContent{Type: "text", Text: messageStr}
	return []gomcp.Content{successContent}, false
}

// calculatorHandler implements the logic for the calculator tool.
// It calls the executeCalculator function which now needs to match the ToolHandlerFunc signature.
func calculatorHandler(ctx context.Context, progressToken *gomcp.ProgressToken, arguments map[string]interface{}) (content []gomcp.Content, isError bool) {
	log.Printf("Executing calculator tool with args: %v", arguments)
	// Assumes executeCalculator is defined in calculator.go and matches the new signature
	// We pass the context and progress token along.
	return executeCalculator(ctx, progressToken, arguments)
}

// filesystemHandler implements the logic for the filesystem tool.
// It calls the executeFileSystem function which now needs to match the ToolHandlerFunc signature.
func filesystemHandler(ctx context.Context, progressToken *gomcp.ProgressToken, arguments map[string]interface{}) (content []gomcp.Content, isError bool) {
	log.Printf("Executing filesystem tool with args: %v", arguments)
	// Assumes executeFileSystem is defined in filesystem.go and matches the new signature
	// We pass the context and progress token along.
	return executeFileSystem(ctx, progressToken, arguments)
}

// --- Main Function ---
func main() {
	// Setup logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Server (Refactored)...")

	// Create a new server instance using the library's NewServer
	serverName := "GoMultiToolServer-Refactored"
	server := gomcp.NewServer(serverName) // Uses stdio connection by default

	// Register tools and their corresponding handler functions
	err := server.RegisterTool(echoTool, echoHandler)
	if err != nil {
		log.Fatalf("Failed to register echo tool: %v", err)
	}

	// Assumes calculatorToolDefinition is available from calculator.go
	err = server.RegisterTool(calculatorToolDefinition, calculatorHandler)
	if err != nil {
		log.Fatalf("Failed to register calculator tool: %v", err)
	}

	// Assumes fileSystemToolDefinition is available from filesystem.go
	err = server.RegisterTool(fileSystemToolDefinition, filesystemHandler)
	if err != nil {
		log.Fatalf("Failed to register filesystem tool: %v", err)
	}

	// Run the server's main loop.
	// server.Run() now handles the handshake and message dispatch internally.
	err = server.Run()
	if err != nil {
		// Log the final error before exiting
		log.Printf("Server exited with error: %v", err)
		os.Exit(1) // Exit with non-zero status on error
	} else {
		log.Println("Server finished.")
	}
}
