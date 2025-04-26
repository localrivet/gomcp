package main

import (
	"context"
	"fmt"
	"log"
	"net/http" // Import net/http for status codes
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor" // To adapt http.HandlerFunc
	"github.com/gofiber/fiber/v2/middleware/logger"  // Fiber's logger middleware
	"github.com/gofiber/fiber/v2/middleware/recover" // Fiber's recover middleware

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
	mcpLogger := &simpleLogger{}   // Use a different name to avoid conflict
	listenAddr := "127.0.0.1:8085" // Use a different port for the Fiber example

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"FiberExampleMCPServer",         // Updated server name
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

	// 3. Create a Fiber app
	app := fiber.New(fiber.Config{
		// Optional: Fiber configuration
		DisableStartupMessage: true,
	})
	app.Use(logger.New())  // Use Fiber's logger middleware
	app.Use(recover.New()) // Use Fiber's recover middleware

	// 4. Register the SSE transport handlers with the Fiber router
	// Use adaptor.HTTPHandlerFunc to convert http.HandlerFunc to fiber.Handler
	app.Get("/sse", adaptor.HTTPHandlerFunc(sseTransport.HandleSSE))
	app.Post("/message", adaptor.HTTPHandlerFunc(sseTransport.HandleMessage))

	// Add a simple root handler for testing the Fiber server itself
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("Fiber server running. MCP endpoints at /sse and /message")
	})

	// 5. Start the server in a goroutine
	go func() {
		mcpLogger.Info("Starting MCP Server with Fiber router on %s...", listenAddr)
		if err := app.Listen(listenAddr); err != nil {
			// Fiber's Listen doesn't return http.ErrServerClosed on graceful shutdown
			mcpLogger.Error("Could not start Fiber server on %s: %v", listenAddr, err)
			// Attempt graceful shutdown even on startup error? Maybe just exit.
			_ = app.Shutdown() // Attempt shutdown
			os.Exit(1)
		}
	}()

	// 6. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	mcpLogger.Info("Shutdown signal received, shutting down server...")

	// Shutdown the Fiber app
	// Fiber's Shutdown doesn't require a context
	if err := app.Shutdown(); err != nil {
		mcpLogger.Error("Fiber server shutdown error: %v", err)
	} else {
		mcpLogger.Info("Fiber server gracefully stopped.")
	}

	mcpLogger.Info("Server shutdown complete.")
}
