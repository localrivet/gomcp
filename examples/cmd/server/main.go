package main

import (
	"context"
	"fmt" // Added for echo handler
	"log"
	"net/http" // Added for HTTP server
	"os"
	"os/signal" // Added for graceful shutdown
	"syscall"   // Added for graceful shutdown
	"time"      // Added for shutdown timeout

	"github.com/localrivet/gomcp/protocol" // Added for tool defs
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse" // Import SSE transport server
	// "github.com/localrivet/gomcp/transport/tcp" // No longer needed
	// "github.com/localrivet/gomcp/types"         // No longer needed directly
)

const defaultListenAddr = "127.0.0.1:8081" // Use explicit IP and port

// --- Echo Tool Definition ---
var echoTool = protocol.Tool{
	Name:        "echo",
	Description: "Echoes back the input message.",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo back."},
		},
		Required: []string{"message"},
	},
}

func handleEcho(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	log.Printf("[Handler] Received call for echo")
	if ctx.Err() != nil {
		log.Println("Echo tool cancelled")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled"}}, true
	}
	args, ok := arguments.(map[string]interface{})
	if !ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'echo' (expected object)"}}, true
	}
	message, ok := args["message"].(string)
	if !ok {
		errContent := protocol.TextContent{Type: "text", Text: "Error: Invalid or missing 'message' argument (string expected)"}
		return []protocol.Content{errContent}, true
	}
	respContent := protocol.TextContent{Type: "text", Text: fmt.Sprintf("Echo: %s", message)}
	return []protocol.Content{respContent}, false
}

// --- End Echo Tool ---

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	listenAddr := defaultListenAddr
	log.Printf("Starting MCP Server (SSE+HTTP) on %s...", listenAddr)

	// 1. Create the core MCP Server logic
	mcpServer := server.NewServer("GoExampleSSEServer") // Use default options

	// 2. Register tools
	if err := mcpServer.RegisterTool(echoTool, handleEcho); err != nil {
		log.Fatalf("Failed to register tool %s: %v", echoTool.Name, err)
	}

	// 3. Create the SSE+HTTP transport server, passing the core server logic
	sseServer := sse.NewSSEServer(mcpServer, sse.SSEServerOptions{})

	// 4. Create and run the HTTP server
	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: sseServer,
	}

	// Channel to listen for OS signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the HTTP server
	go func() {
		log.Printf("HTTP server listening on %s", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe error: %v", err)
		}
		log.Println("HTTP server stopped.")
	}()

	// Wait for shutdown signal
	<-stopChan
	log.Println("Shutdown signal received, shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server gracefully stopped.")
	}

	log.Println("Server shutdown complete.")
}
