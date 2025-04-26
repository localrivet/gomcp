package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	// "time" // go-zero handles shutdown timing internally

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"

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

// Define go-zero server configuration
type Config struct {
	rest.RestConf
}

func main() {
	logger := &simpleLogger{}
	listenAddr := "127.0.0.1:8086" // Use a different port for the go-zero example

	// Configure go-zero logging (optional, redirects go-zero logs)
	logx.SetWriter(logx.NewWriter(os.Stdout)) // Or use your preferred writer

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"GoZeroExampleMCPServer",     // Updated server name
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

	// 3. Configure and create the go-zero server
	var c Config
	// Basic configuration programmatically - Set fields within RestConf
	c.RestConf.Host = "127.0.0.1"
	c.RestConf.Port = 8086
	c.RestConf.Name = "go-zero-mcp-example"
	c.RestConf.Log.Mode = "console" // Keep go-zero logs minimal for example
	c.RestConf.Verbose = false      // Disable verbose startup messages

	// You can also load from YAML using conf.MustLoad(...)
	// conf.MustLoad("config.yaml", &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop() // Ensure server stops on exit

	// 4. Register the SSE transport handlers with the go-zero server
	// go-zero uses its own handler signature, so we wrap the http.HandlerFunc
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		sseTransport.HandleSSE(w, r)
	}
	messageHandler := func(w http.ResponseWriter, r *http.Request) {
		sseTransport.HandleMessage(w, r)
	}

	// Use AddRoute with raw http.HandlerFunc
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/sse",
			Handler: sseHandler, // Assign http.HandlerFunc directly
		},
		{
			Method:  http.MethodPost,
			Path:    "/message",
			Handler: messageHandler, // Assign http.HandlerFunc directly
		},
		{ // Add a simple root handler for testing
			Method: http.MethodGet,
			Path:   "/",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("go-zero server running. MCP endpoints at /sse and /message"))
			},
		},
	})

	// 5. Start the server (blocking) and handle graceful shutdown
	logger.Info("Starting MCP Server with go-zero router on %s...", listenAddr)

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		server.Start() // This blocks until server stops
	}()

	<-stop // Wait for shutdown signal

	logger.Info("Shutdown signal received, shutting down server...")
	// go-zero's Stop() handles graceful shutdown
	// No explicit context timeout needed here as Stop manages it.
	// defer server.Stop() already registered

	logger.Info("Server shutdown complete.")
}
