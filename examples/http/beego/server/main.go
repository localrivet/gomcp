package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	// "time" // Beego handles shutdown timing

	"github.com/beego/beego/v2/server/web" // Import Beego web module

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

// Beego Controller to handle root requests (optional, for testing Beego itself)
type MainController struct {
	web.Controller
}

func (c *MainController) Get() {
	// Access context via embedded Controller field
	c.Controller.Ctx.WriteString("Beego server running. MCP endpoints at /sse and /message")
}

func main() {
	logger := &simpleLogger{}
	listenAddr := "127.0.0.1" // Beego uses configuration for port
	listenPort := 8090        // Use a different port for the Beego example

	// Configure Beego (minimal programmatic config)
	web.BConfig.AppName = "BeegoExampleMCPServer"
	web.BConfig.RunMode = "prod" // Keep logs minimal
	web.BConfig.CopyRequestBody = true
	web.BConfig.Listen.HTTPAddr = listenAddr
	web.BConfig.Listen.HTTPPort = listenPort
	web.BConfig.Log.AccessLogs = false
	web.BConfig.Log.EnableStaticLogs = false
	// Disable Beego's default logger or configure it if needed
	// web.BeeLogger.DelLogger("console")

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"BeegoExampleMCPServer",      // Server name
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

	// 3. Register the SSE transport handlers with Beego
	// Beego uses http.HandlerFunc directly
	web.Handler("/sse", http.HandlerFunc(sseTransport.HandleSSE), web.WithRouterMethods(&MainController{}, http.MethodGet))
	web.Handler("/message", http.HandlerFunc(sseTransport.HandleMessage), web.WithRouterMethods(&MainController{}, http.MethodPost))

	// Register the root controller (optional)
	web.Router("/", &MainController{})

	// 4. Start Beego server (blocking) and handle graceful shutdown
	logger.Info("Starting MCP Server with Beego on %s:%d...", listenAddr, listenPort)

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		web.Run() // This blocks until server stops
	}()

	<-stop // Wait for shutdown signal

	logger.Info("Shutdown signal received, shutting down server...")
	// Beego handles graceful shutdown internally when the process receives SIGINT/SIGTERM
	// No explicit shutdown call needed here like in net/http based servers.

	logger.Info("Server shutdown complete.")
}
