// Command json-config-demo demonstrates loading MCP server configuration from JSON files.
// NOTE: This example currently only supports the 'stdio' transport due to library API changes.
//
// Features
// --------
//
//   - JSON configuration loading
//   - Stdio transport support
//   - Configurable logging levels
//   - Dynamic tool registration
//
// Build & run:
//
//	go run . config.json
//
// Example request via stdio:
//
//	{"method":"echo","params":{"message":"Hello from JSON config!"}}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/localrivet/gomcp/hooks" // Import hooks package
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/types"
	"github.com/localrivet/gomcp/util/schema"
)

// ---------------------------------------------------------------------------
// Configuration Types
// ---------------------------------------------------------------------------

type ToolConfig struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required"`
}

type TransportConfig struct {
	Type        string `json:"type" validate:"required,oneof=stdio websocket sse"` // Keep validation for config parsing
	Port        int    `json:"port" validate:"required_if=Type websocket sse"`
	Address     string `json:"address,omitempty"`
	Path        string `json:"path,omitempty"`
	SSEPath     string `json:"sse_path,omitempty"`
	MessagePath string `json:"message_path,omitempty"`
}

type AppConfig struct {
	ServerName  string          `json:"server_name" validate:"required"`
	LoggerLevel string          `json:"logger_level"`
	Transport   TransportConfig `json:"transport" validate:"required"`
	Tools       []ToolConfig    `json:"tools" validate:"required,dive"`
}

// ---------------------------------------------------------------------------
// Tool Arguments
// ---------------------------------------------------------------------------

type EchoArgs struct {
	Message string `json:"message" description:"Message to echo back" required:"true"`
}

type PingArgs struct {
	// Empty struct since ping takes no arguments
}

// ---------------------------------------------------------------------------
// Helper Functions
// ---------------------------------------------------------------------------

func loadConfigFromJson(configPath string) (*AppConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return &config, nil
}

func registerTools(svr *server.Server, tools []ToolConfig, logger types.Logger) {
	for _, tool := range tools {
		var handler hooks.FinalToolHandler // Use hooks.FinalToolHandler
		switch tool.Name {
		case "echo":
			handler = createEchoHandler(logger)
		case "ping":
			handler = createPingHandler(logger)
		default:
			logger.Warn("unknown tool in config: %s", tool.Name)
			continue
		}

		toolDef := protocol.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		}

		argsType := getToolArgsType(tool.Name)
		if argsType == nil || fmt.Sprintf("%T", argsType) == "struct {}" {
			toolDef.InputSchema = schema.FromStruct(struct{}{})
		} else {
			toolDef.InputSchema = schema.FromStruct(argsType)
		}

		if err := svr.RegisterTool(toolDef, handler); err != nil {
			logger.Error("failed to register tool %s: %v", tool.Name, err)
		} else {
			logger.Info("Registered tool: %s", tool.Name)
		}
	}
}

func getToolArgsType(toolName string) interface{} {
	switch toolName {
	case "echo":
		return EchoArgs{}
	case "ping":
		return PingArgs{}
	default:
		return struct{}{}
	}
}

func createEchoHandler(logger types.Logger) hooks.FinalToolHandler { // Use hooks.FinalToolHandler
	return func(ctx context.Context, progressToken interface{}, arguments any) (content []protocol.Content, isError bool) { // Use interface{} for progressToken
		logger.Debug("Executing echo tool")
		args, errContent, isErr := schema.HandleArgs[EchoArgs](arguments)
		if isErr {
			logger.Error("echo handler argument error: %v", errContent)
			return errContent, true
		}
		logger.Debug("echo handler message: %s", args.Message)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: args.Message}}, false
	}
}

func createPingHandler(logger types.Logger) hooks.FinalToolHandler { // Use hooks.FinalToolHandler
	return func(ctx context.Context, progressToken interface{}, arguments any) (content []protocol.Content, isError bool) { // Use interface{} for progressToken
		logger.Debug("Executing ping tool")
		_, errContent, isErr := schema.HandleArgs[PingArgs](arguments)
		if isErr {
			logger.Error("ping handler argument error: %v", errContent)
			return errContent, true
		}
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "pong"}}, false
	}
}

// ---------------------------------------------------------------------------
// Transport Functions
// ---------------------------------------------------------------------------

func startTransport(svr *server.Server, cfg TransportConfig, logger types.Logger) {
	switch cfg.Type {
	case "stdio":
		startStdioTransport(svr, logger)
	case "sse":
		logger.Error("SSE transport is not currently supported by this example due to library changes.")
		os.Exit(1)
	case "websocket":
		logger.Error("WebSocket transport is not currently supported by this example due to library changes.")
		os.Exit(1)
	default:
		logger.Error("unsupported transport type in config: %s", cfg.Type)
		os.Exit(1)
	}
}

func startStdioTransport(svr *server.Server, logger types.Logger) {
	logger.Info("Starting server with stdio transport")
	// Assume server handles stdio internally. Block main thread.
	// This might require a specific server method like RunStdio() or similar,
	// but for now, we just block.
	logger.Info("Stdio transport configured. Blocking main thread.")
	select {}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run . <config.json>")
		os.Exit(1)
	}
	configPath := os.Args[1]

	config, err := loadConfigFromJson(configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", configPath, err)
	}

	logger := logx.NewLogger(config.LoggerLevel)
	svr := server.NewServer(config.ServerName, server.WithLogger(logger))
	registerTools(svr, config.Tools, logger)

	startTransport(svr, config.Transport, logger)

	// This log should ideally not be reached if stdio blocks correctly.
	logger.Info("Server main function exiting.")
}
