// examples/server/main.go (Refactored)
// This is the main file for the example MCP server.
package main

import (
	"context"
	// "encoding/json" // No longer needed here
	// "errors"        // No longer needed here
	// "fmt"           // No longer needed here
	// "io"            // No longer needed here
	"log"
	"os"

	// "os/signal" // No longer needed here
	// "strings"   // No longer needed here
	// "sync"      // No longer needed here
	// "time" // No longer needed here

	// Import new packages
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"

	// "github.com/localrivet/gomcp/transport/stdio" // No longer needed here
	"github.com/localrivet/gomcp/types" // Needed for logger interface
)

// --- Tool Definitions ---
// See calculator.go and filesystem.go for the other definitions.

// Define the echo tool
var echoTool = protocol.Tool{
	Name:        "echo",
	Description: "Echoes back the provided message.",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
}

// calculatorToolDefinition is defined in calculator.go
// fileSystemToolDefinition is defined in filesystem.go

// --- Tool Handler Functions ---

// echoHandler implements the logic for the echo tool.
func echoHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, ok := arguments.(map[string]interface{})
	if !ok {
		errorContent := protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'echo' (expected object)"}
		return []protocol.Content{errorContent}, true
	}
	log.Printf("Executing echo tool with args: %v", args)
	if ctx.Err() != nil {
		log.Println("Echo tool cancelled")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}
	messageArg, ok := args["message"]
	if !ok {
		errorContent := protocol.TextContent{Type: "text", Text: "Missing required argument 'message' for tool 'echo'"}
		return []protocol.Content{errorContent}, true
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		errorContent := protocol.TextContent{Type: "text", Text: "Argument 'message' for tool 'echo' must be a string"}
		return []protocol.Content{errorContent}, true
	}
	log.Printf("Echoing message: %s", messageStr)
	successContent := protocol.TextContent{Type: "text", Text: messageStr}
	return []protocol.Content{successContent}, false
}

// calculatorHandler implements the logic for the calculator tool.
func calculatorHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, ok := arguments.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for calculator tool (expected object)"}}, true
	}
	log.Printf("Executing calculator tool with args: %v", args)
	return executeCalculator(ctx, progressToken, args)
}

// filesystemHandler implements the logic for the filesystem tool.
func filesystemHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, ok := arguments.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for filesystem tool (expected object)"}}, true
	}
	log.Printf("Executing filesystem tool with args: %v", args)
	return executeFileSystem(ctx, progressToken, args)
}

// --- runServerLoop (Removed) ---

// --- mockSession (Removed) ---

// --- Main Function ---
func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Server (Refactored)...")

	// NOTE: This main function now only sets up the server but doesn't run it.
	//       To run this server standalone, use `go run ./examples/server/` which
	//       will build main.go, calculator.go, and filesystem.go together,
	//       and then pipe input/output to it.

	// transport := stdio.NewStdioTransport() // Example: Create transport (unused)
	logger := NewDefaultLogger() // Use local helper

	serverName := "GoMultiToolServer-Refactored"
	srv := server.NewServer(serverName, server.WithLogger(logger)) // Use functional option

	// Register tools and their corresponding handler functions
	err := srv.RegisterTool(echoTool, echoHandler)
	if err != nil {
		log.Fatalf("Failed to register echo tool: %v", err)
	}
	err = srv.RegisterTool(calculatorToolDefinition, calculatorHandler)
	if err != nil {
		log.Fatalf("Failed to register calculator tool: %v", err)
	}
	err = srv.RegisterTool(fileSystemToolDefinition, filesystemHandler)
	if err != nil {
		log.Fatalf("Failed to register filesystem tool: %v", err)
	}

	log.Println("Server setup complete. Exiting (no run loop implemented in this example main).")
	// The actual server loop needs to be implemented, similar to cmd/gomcp-server/main.go
	// using a chosen transport (like stdio).
}

// Added Default Logger definition
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

func NewDefaultLogger() *defaultLogger { return &defaultLogger{} }

var _ types.Logger = (*defaultLogger)(nil)

// Ensure handlers match the expected type (defined in server package)
var _ server.ToolHandlerFunc = echoHandler
var _ server.ToolHandlerFunc = calculatorHandler
var _ server.ToolHandlerFunc = filesystemHandler
