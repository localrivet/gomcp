// examples/server/main.go (Refactored)
// This is the main file for the example MCP server.
// It demonstrates how to use the refactored mcp.Server from the library:
// 1. Create an mcp.Server.
// 2. Define tool logic as functions matching mcp.ToolHandlerFunc.
// 3. Register tools using server.RegisterTool.
// 4. Run the server using server.Run(), which handles handshake and message dispatch.
package main

import (
	// Keep fmt for error messages if needed by tool handlers
	"log"
	"os"

	// "strings" // No longer needed here

	mcp "github.com/localrivet/gomcp" // Import root package
)

// --- Tool Definitions ---
// Definitions remain the same.
// See calculator.go and filesystem.go for the other definitions.

// Define the echo tool (simple tool defined directly in main)
var echoTool = mcp.Tool{ // Use new Tool struct
	Name:        "echo",
	Description: "Echoes back the provided message.",
	InputSchema: mcp.ToolInputSchema{ // InputSchema remains similar
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
	// OutputSchema is removed in the new spec for Tool definition
	// Annotations: mcp.ToolAnnotations{}, // Optional annotations
}

// calculatorToolDefinition is defined in calculator.go
// fileSystemToolDefinition is defined in filesystem.go

// --- Tool Handler Functions ---
// These functions now match the mcp.ToolHandlerFunc signature.
// They take arguments and return ([]Content, isError bool).

// echoHandler implements the logic for the echo tool.
func echoHandler(arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("Executing echo tool with args: %v", arguments)
	messageArg, ok := arguments["message"]
	if !ok {
		// Return error content and isError=true
		errorContent := mcp.TextContent{Type: "text", Text: "Missing required argument 'message' for tool 'echo'"}
		return []mcp.Content{errorContent}, true
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		errorContent := mcp.TextContent{Type: "text", Text: "Argument 'message' for tool 'echo' must be a string"}
		return []mcp.Content{errorContent}, true
	}
	log.Printf("Echoing message: %s", messageStr)
	// Return success content and isError=false
	successContent := mcp.TextContent{Type: "text", Text: messageStr}
	return []mcp.Content{successContent}, false
}

// calculatorHandler implements the logic for the calculator tool.
// It calls the executeCalculator function which now returns ([]Content, bool).
func calculatorHandler(arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("Executing calculator tool with args: %v", arguments)
	// Assumes executeCalculator is defined in calculator.go and matches the new signature
	return executeCalculator(arguments)
}

// filesystemHandler implements the logic for the filesystem tool.
// It calls the executeFileSystem function which now returns ([]Content, bool).
func filesystemHandler(arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("Executing filesystem tool with args: %v", arguments)
	// Assumes executeFileSystem is defined in filesystem.go and matches the new signature
	return executeFileSystem(arguments)
}

// --- Main Function ---
func main() {
	// Setup logging
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Server (Refactored)...")

	// Create a new server instance using the library's NewServer
	serverName := "GoMultiToolServer-Refactored"
	server := mcp.NewServer(serverName) // Uses stdio connection by default

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
