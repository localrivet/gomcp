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

	"github.com/kataras/iris/v12" // Import Iris

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
	mcpLogger := &simpleLogger{}   // Use specific name for MCP logger
	listenAddr := "127.0.0.1:8091" // Use a different port for the Iris example

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"IrisExampleMCPServer",          // Updated server name
		mcpServer.WithLogger(mcpLogger), // Use functional option
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
			Logger: mcpLogger,
			// BasePath, MessageEndpoint, SSEEndpoint default to "/" "/message", "/sse"
		},
	)

	// 3. Create an Iris application
	app := iris.New()
	// Optional: Configure Iris logger if needed, or disable it
	app.Logger().SetLevel("disable") // Disable Iris default logger for this example

	// 4. Register the SSE transport handlers with Iris
	// Iris can handle standard http.HandlerFunc
	app.Get("/sse", iris.FromStd(sseTransport.HandleSSE))
	app.Post("/message", iris.FromStd(sseTransport.HandleMessage))

	// Add a simple root handler for testing the Iris server itself
	app.Get("/", func(ctx iris.Context) {
		ctx.WriteString("Iris server running. MCP endpoints at /sse and /message")
	})

	// 5. Start the Iris server (blocking) and handle graceful shutdown
	mcpLogger.Info("Starting MCP Server with Iris on %s...", listenAddr)

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Use app.Listen for graceful shutdown handling integrated with signals
		err := app.Listen(listenAddr, iris.WithoutInterruptHandler) // Let our handler manage interrupt
		if err != nil && err != http.ErrServerClosed {
			mcpLogger.Error("Could not start Iris server: %v", err)
			os.Exit(1)
		}
	}()

	<-stop // Wait for shutdown signal
	mcpLogger.Info("Shutdown signal received, shutting down server...")

	// Shutdown the Iris server
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		mcpLogger.Error("Iris server shutdown error: %v", err)
	} else {
		mcpLogger.Info("Iris server gracefully stopped.")
	}

	mcpLogger.Info("Server shutdown complete.")
}
