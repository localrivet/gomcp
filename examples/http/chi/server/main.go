package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/localrivet/gomcp/protocol" // Import protocol package
	mcpServer "github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
)

// Simple logger implementation (implements types.Logger)
type simpleLogger struct{}

func (l *simpleLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *simpleLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *simpleLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *simpleLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

func main() {
	logger := &simpleLogger{}
	listenAddr := "127.0.0.1:8082" // Use a different port for the example

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"ChiExampleMCPServer",        // Pass server name directly
		mcpServer.WithLogger(logger), // Use functional option
		// Add other options like WithServerCapabilities if needed
	)

	// Register example tool (optional)
	coreServer.RegisterTool(
		protocol.Tool{ // Use protocol.Tool
			Name:        "echo",
			Description: "Simple echo tool",
			InputSchema: protocol.ToolInputSchema{Type: "string"}, // Adjust schema if needed
		},
		// Match server.ToolHandlerFunc signature
		func(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
			argsMap, ok := args.(map[string]interface{})
			if !ok {
				return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'echo' (expected object)"}}, true
			}

			// Assume args is the string itself for this simple echo
			inputText := "nil"
			if argsMap != nil {
				if strArg, ok := argsMap["input"].(string); ok { // Assuming input is passed as {"input": "value"}
					inputText = strArg
				} else if strArg, ok := argsMap[""].(string); ok { // Or just the string value directly
					inputText = strArg
				}
			}
			resultText := fmt.Sprintf("You said: %s", inputText)
			content = []protocol.Content{
				protocol.TextContent{Type: "text", Text: resultText},
			}
			isError = false // Indicate success
			return content, isError
		},
	)

	// 2. Create the SSE Transport
	sseTransport := sse.NewSSEServer(
		coreServer,
		sse.SSEServerOptions{ // Use SSEServerOptions struct
			Logger: logger,
			// BasePath, MessageEndpoint, SSEEndpoint default to "/" "/message", "/sse"
		},
	)

	// 3. Create a Chi router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// 4. Register the SSE transport handlers with the Chi router
	r.Get("/sse", sseTransport.HandleSSE)          // Use exported HandleSSE
	r.Post("/message", sseTransport.HandleMessage) // Use exported HandleMessage

	// Add a simple root handler for testing the Chi server itself
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Chi server running. MCP endpoints at /sse and /message"))
	})

	// 5. Create and configure the HTTP server using the Chi router
	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: r,
	}

	// 6. Start the server in a goroutine
	go func() {
		logger.Info("Starting MCP Server with Chi router on %s...", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Could not listen on %s: %v", listenAddr, err)
			os.Exit(1)
		}
	}()

	// 7. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	logger.Info("Shutdown signal received, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the HTTP server (this will close SSE connections via context cancellation)
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	} else {
		logger.Info("HTTP server gracefully stopped.")
	}

	logger.Info("Server shutdown complete.")
}
