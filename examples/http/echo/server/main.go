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

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

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
	listenAddr := "127.0.0.1:8083" // Use a different port for the Echo example

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"EchoExampleMCPServer",       // Updated server name
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

	// 3. Create an Echo instance
	e := echo.New()
	e.HideBanner = true // Optional: hide Echo banner
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 4. Register the SSE transport handlers with the Echo router
	// Adapt http.HandlerFunc to echo.HandlerFunc
	sseHandler := func(c echo.Context) error {
		sseTransport.HandleSSE(c.Response().Writer, c.Request())
		return nil // Indicate success to Echo
	}
	messageHandler := func(c echo.Context) error {
		sseTransport.HandleMessage(c.Response().Writer, c.Request())
		// HandleMessage writes the response itself, check if it wrote status
		if c.Response().Committed {
			return nil
		}
		// If HandleMessage didn't write a response (e.g., error handled internally),
		// we might need to return an error or specific status code here.
		// For simplicity, assume HandleMessage handles its response fully.
		return nil
	}

	e.GET("/sse", sseHandler)
	e.POST("/message", messageHandler)

	// Add a simple root handler for testing the Echo server itself
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Echo server running. MCP endpoints at /sse and /message")
	})

	// 5. Start the server in a goroutine
	go func() {
		logger.Info("Starting MCP Server with Echo router on %s...", listenAddr)
		if err := e.Start(listenAddr); err != nil && err != http.ErrServerClosed {
			logger.Error("Could not start Echo server on %s: %v", listenAddr, err)
			e.Close() // Attempt to close Echo instance on error
			os.Exit(1)
		}
	}()

	// 6. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	logger.Info("Shutdown signal received, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the Echo server
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("Echo server shutdown error: %v", err)
	} else {
		logger.Info("Echo server gracefully stopped.")
	}

	logger.Info("Server shutdown complete.")
}
