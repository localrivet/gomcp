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

	"github.com/localrivet/gomcp/protocol" // Import protocol package
	mcpServer "github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/types" // Import types for logger interface
)

// Simple logger implementation (implements types.Logger)
type simpleLogger struct{}

func (l *simpleLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *simpleLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *simpleLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *simpleLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

// Ensure simpleLogger implements types.Logger
var _ types.Logger = (*simpleLogger)(nil)

func main() {
	logger := &simpleLogger{}
	listenAddr := "127.0.0.1:8087" // Use a different port for the net/http example

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"NetHttpExampleMCPServer",    // Updated server name
		mcpServer.WithLogger(logger), // Use functional option
	)

	// Register example tool (optional)
	coreServer.RegisterTool(
		protocol.Tool{
			Name:        "echo",
			Description: "Simple echo tool",
			InputSchema: protocol.ToolInputSchema{Type: "string"},
		},
		func(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
			argsMap, ok := args.(map[string]interface{})
			if !ok {
				return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'echo' (expected object)"}}, true
			}

			inputText := "nil"
			if argsMap != nil {
				if strArg, ok := argsMap["input"].(string); ok {
					inputText = strArg
				} else if strArg, ok := argsMap[""].(string); ok {
					inputText = strArg
				}
			}
			resultText := fmt.Sprintf("You said: %s", inputText)
			content = []protocol.Content{
				protocol.TextContent{Type: "text", Text: resultText},
			}
			isError = false
			return content, isError
		},
	)

	// 2. Create the SSE Transport
	sseTransport := sse.NewSSEServer(
		coreServer,
		sse.SSEServerOptions{
			Logger: logger,
			// BasePath, MessageEndpoint, SSEEndpoint default to "/" "/message", "/sse"
		},
	)

	// 3. Create a ServeMux (standard library router)
	mux := http.NewServeMux()

	// 4. Register the SSE transport handlers with the ServeMux
	mux.HandleFunc("/sse", sseTransport.HandleSSE)
	mux.HandleFunc("/message", sseTransport.HandleMessage)

	// Add a simple root handler for testing the server itself
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Prevent matching other paths
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("net/http server running. MCP endpoints at /sse and /message"))
	})

	// 5. Create and configure the HTTP server using the ServeMux
	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: mux, // Use the ServeMux
	}

	// 6. Start the server in a goroutine
	go func() {
		logger.Info("Starting MCP Server with net/http router on %s...", listenAddr)
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

	// Shutdown the HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	} else {
		logger.Info("HTTP server gracefully stopped.")
	}

	logger.Info("Server shutdown complete.")
}
