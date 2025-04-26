package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gobwas/ws"
	"github.com/localrivet/gomcp/protocol"
	mcpServer "github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/transport/websocket"
	"github.com/localrivet/gomcp/types"
	"gopkg.in/yaml.v3"
)

// --- Configuration Structs ---

type ToolConfig struct {
	Name        string `json:"name" yaml:"name" toml:"name"`
	Description string `json:"description" yaml:"description" toml:"description"`
	// InputSchema could be added here if needed
}

type TransportConfig struct {
	Type        string `json:"type" yaml:"type" toml:"type"` // "stdio", "sse", "websocket"
	Address     string `json:"address,omitempty" yaml:"address,omitempty" toml:"address,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty" toml:"path,omitempty"`                         // For WebSocket
	SSEPath     string `json:"sse_path,omitempty" yaml:"sse_path,omitempty" toml:"sse_path,omitempty"`             // For SSE
	MessagePath string `json:"message_path,omitempty" yaml:"message_path,omitempty" toml:"message_path,omitempty"` // For SSE
}

type AppConfig struct {
	ServerName  string          `json:"server_name" yaml:"server_name" toml:"server_name"`
	LoggerLevel string          `json:"logger_level" yaml:"logger_level" toml:"logger_level"` // "debug", "info", "warn", "error"
	Transport   TransportConfig `json:"transport" yaml:"transport" toml:"transport"`
	Tools       []ToolConfig    `json:"tools" yaml:"tools" toml:"tools"`
}

// --- Logger Implementation ---

type configLogger struct {
	level int // 0:debug, 1:info, 2:warn, 3:error
}

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

func NewConfigLogger(levelStr string) *configLogger {
	level := levelInfo // Default
	switch strings.ToLower(levelStr) {
	case "debug":
		level = levelDebug
	case "info":
		level = levelInfo
	case "warn":
		level = levelWarn
	case "error":
		level = levelError
	}
	return &configLogger{level: level}
}

func (l *configLogger) Debug(msg string, args ...interface{}) {
	if l.level <= levelDebug {
		log.Printf("DEBUG: "+msg, args...)
	}
}
func (l *configLogger) Info(msg string, args ...interface{}) {
	if l.level <= levelInfo {
		log.Printf("INFO: "+msg, args...)
	}
}
func (l *configLogger) Warn(msg string, args ...interface{}) {
	if l.level <= levelWarn {
		log.Printf("WARN: "+msg, args...)
	}
}
func (l *configLogger) Error(msg string, args ...interface{}) {
	if l.level <= levelError {
		log.Printf("ERROR: "+msg, args...)
	}
}

var _ types.Logger = (*configLogger)(nil)

// --- WebSocket Session (Copied from websocket example) ---
type WebSocketClientSession struct {
	sessionID   string
	transport   *websocket.WebSocketTransport
	initialized bool
	initMutex   sync.RWMutex
	logger      types.Logger
}

var _ mcpServer.ClientSession = (*WebSocketClientSession)(nil)

func NewWebSocketClientSession(transport *websocket.WebSocketTransport, logger types.Logger) *WebSocketClientSession { /* ... implementation ... */
}
func (s *WebSocketClientSession) SessionID() string                                    { /* ... */ }
func (s *WebSocketClientSession) SendResponse(response protocol.JSONRPCResponse) error { /* ... */ }
func (s *WebSocketClientSession) SendNotification(notification protocol.JSONRPCNotification) error { /* ... */
}
func (s *WebSocketClientSession) Close() error      { /* ... */ }
func (s *WebSocketClientSession) Initialized() bool { /* ... */ }
func (s *WebSocketClientSession) Initialize()       { /* ... */ }

// --- End WebSocket Session ---

// --- Main Application ---

func main() {
	// Command line flag for config file path
	// Default changed to point to the new json config location
	configFile := flag.String("config", "../json/config.json", "Path to configuration file (json, yaml, or toml)")
	flag.Parse()

	// Load configuration
	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", *configFile, err)
	}

	// Create logger based on config
	logger := NewConfigLogger(cfg.LoggerLevel)
	logger.Info("Configuration loaded from %s", *configFile)
	logger.Info("Server Name: %s, Log Level: %s", cfg.ServerName, cfg.LoggerLevel)
	logger.Info("Transport Type: %s", cfg.Transport.Type)

	// Create the core MCP Server
	coreServer := mcpServer.NewServer(
		cfg.ServerName,
		mcpServer.ServerOptions{Logger: logger},
	)

	// Register tools from config
	registerTools(coreServer, cfg.Tools, logger)

	// Start the configured transport
	startTransport(coreServer, cfg.Transport, logger)

	logger.Info("Server shutdown complete.")
}

// --- Helper Functions ---

func loadConfig(filePath string) (*AppConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AppConfig
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		err = json.Unmarshal(data, &cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
		}
	case ".toml":
		_, err = toml.Unmarshal(data, &cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported configuration file extension: %s", ext)
	}

	// Basic validation
	if cfg.ServerName == "" {
		return nil, fmt.Errorf("server_name is required in configuration")
	}
	if cfg.Transport.Type == "" {
		return nil, fmt.Errorf("transport.type is required in configuration")
	}

	return &cfg, nil
}

func registerTools(server *mcpServer.Server, tools []ToolConfig, logger types.Logger) {
	for _, toolCfg := range tools {
		tool := protocol.Tool{
			Name:        toolCfg.Name,
			Description: toolCfg.Description,
			// Define InputSchema based on tool name or add to config?
			// For simplicity, assume string input for echo/ping
			InputSchema: protocol.ToolInputSchema{Type: "string"},
		}

		var handler mcpServer.ToolHandlerFunc
		switch toolCfg.Name {
		case "echo":
			handler = createEchoHandler(logger)
		case "ping":
			handler = createPingHandler(logger)
		default:
			logger.Warn("No handler implemented for configured tool: %s", toolCfg.Name)
			continue // Skip registering tools without handlers
		}

		if err := server.RegisterTool(tool, handler); err != nil {
			logger.Error("Failed to register tool '%s': %v", toolCfg.Name, err)
		} else {
			logger.Info("Registered tool from config: %s", toolCfg.Name)
		}
	}
}

// Example tool handlers
func createEchoHandler(logger types.Logger) mcpServer.ToolHandlerFunc {
	return func(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
		logger.Debug("Executing echo tool")
		argsMap, ok := args.(map[string]interface{})
		if !ok {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'echo' (expected object)"}}, true
		}
		inputText := "nil"
		if argsMap != nil {
			if strArg, ok := argsMap["input"].(string); ok { // Assuming input is passed as {"input": "value"}
				inputText = strArg
			} else if strArg, ok := argsMap[""].(string); ok { // Or just the string value directly
				inputText = strArg
			}
		}
		resultText := fmt.Sprintf("Echo: %s", inputText)
		content = []protocol.Content{protocol.TextContent{Type: "text", Text: resultText}}
		return content, false
	}
}

func createPingHandler(logger types.Logger) mcpServer.ToolHandlerFunc {
	return func(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
		logger.Debug("Executing ping tool")
		argsMap, ok := args.(map[string]interface{})
		if !ok {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments for tool 'ping' (expected object)"}}, true
		}
		content = []protocol.Content{protocol.TextContent{Type: "text", Text: "pong"}}
		return content, false
	}
}

func startTransport(coreServer *mcpServer.Server, cfg TransportConfig, logger types.Logger) {
	switch strings.ToLower(cfg.Type) {
	case "stdio":
		startStdioTransport(coreServer, logger)
	case "sse":
		startSSETransport(coreServer, cfg, logger)
	case "websocket":
		startWebSocketTransport(coreServer, cfg, logger)
	default:
		logger.Error("Unsupported transport type in configuration: %s", cfg.Type)
		os.Exit(1)
	}
}

func startStdioTransport(coreServer *mcpServer.Server, logger types.Logger) {
	logger.Info("Starting Stdio transport")
	stdioTransport := stdio.NewStdioTransport(types.TransportOptions{Logger: logger})
	session := stdio.NewStdioClientSession(stdioTransport, logger) // Use Stdio session

	if err := coreServer.RegisterSession(session); err != nil {
		logger.Error("Failed to register stdio session: %v", err)
		os.Exit(1)
	}
	logger.Info("Registered stdio session: %s", session.SessionID())

	// Run the message handling loop (blocking)
	stdio.RunStdioSession(context.Background(), coreServer, session, logger) // Use RunStdioSession helper

	logger.Info("Stdio transport finished.")
	coreServer.UnregisterSession(session.SessionID())
}

func startSSETransport(coreServer *mcpServer.Server, cfg TransportConfig, logger types.Logger) {
	if cfg.Address == "" {
		logger.Error("SSE transport requires 'address' in configuration")
		os.Exit(1)
	}
	ssePath := cfg.SSEPath
	if ssePath == "" {
		ssePath = "/sse" // Default
	}
	messagePath := cfg.MessagePath
	if messagePath == "" {
		messagePath = "/message" // Default
	}

	logger.Info("Starting SSE transport on %s (SSE Path: %s, Message Path: %s)", cfg.Address, ssePath, messagePath)

	sseTransportServer := sse.NewSSEServer(
		coreServer,
		sse.SSEServerOptions{
			Logger:          logger,
			BasePath:        "/", // Base path not directly used if specifying endpoints
			SSEEndpoint:     ssePath,
			MessageEndpoint: messagePath,
		},
	)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(sseTransportServer.SSEEndpoint(), sseTransportServer.HandleSSE)
	mux.HandleFunc(sseTransportServer.MessageEndpoint(), sseTransportServer.HandleMessage)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(fmt.Sprintf("HTTP server running. MCP SSE endpoint at %s, Message endpoint at %s", ssePath, messagePath)))
	})

	httpServer := &http.Server{Addr: cfg.Address, Handler: mux}

	go func() {
		logger.Info("Starting HTTP Server for SSE on %s", cfg.Address)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server (SSE) ListenAndServe error: %v", err)
			os.Exit(1)
		}
	}()

	waitForShutdownSignal(httpServer, logger, "SSE HTTP server")
}

func startWebSocketTransport(coreServer *mcpServer.Server, cfg TransportConfig, logger types.Logger) {
	if cfg.Address == "" {
		logger.Error("WebSocket transport requires 'address' in configuration")
		os.Exit(1)
	}
	wsPath := cfg.Path
	if wsPath == "" {
		wsPath = "/mcp" // Default
	}

	logger.Info("Starting WebSocket transport on %s (Path: %s)", cfg.Address, wsPath)

	upgrader := ws.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	mux := http.NewServeMux()

	mux.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) {
		// ... WebSocket upgrade and session handling logic (copied from websocket example) ...
		// Hijack, Upgrade, Create Transport, Create Session, Register Session, Run message loop
		// --- Start Copy ---
		logger.Info("Received WebSocket upgrade request from %s", r.RemoteAddr)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			logger.Error("HTTP server does not support hijacking")
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			logger.Error("Failed to hijack connection: %v", err)
			http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
			return
		}
		defer func() {
			if err != nil {
				logger.Warn("Closing hijacked connection due to error: %v", err)
				conn.Close()
			}
		}()
		hs, err := upgrader.Upgrade(rw)
		if err != nil {
			logger.Error("WebSocket handshake failed: %v", err)
			return
		}
		logger.Debug("Handshake details: Proto: %s", hs.Protocol)
		logger.Info("WebSocket connection established with %s", conn.RemoteAddr())
		if err = rw.Flush(); err != nil {
			logger.Error("Failed to flush hijacked connection writer after upgrade: %v", err)
			return
		}
		transportOpts := types.TransportOptions{Logger: logger}
		mcpTransport := websocket.NewWebSocketTransport(conn, ws.StateServerSide, transportOpts)
		session := NewWebSocketClientSession(mcpTransport, logger) // Use the session defined above
		if err := coreServer.RegisterSession(session); err != nil {
			logger.Error("Failed to register session %s: %v", session.SessionID(), err)
			mcpTransport.Close()
			return
		}
		logger.Info("Registered session %s for %s", session.SessionID(), conn.RemoteAddr())
		go func() {
			defer func() {
				logger.Info("Unregistering session %s for %s", session.SessionID(), conn.RemoteAddr())
				coreServer.UnregisterSession(session.SessionID())
				mcpTransport.Close()
			}()
			for {
				rawMessage, err := mcpTransport.ReceiveWithContext(context.Background())
				if err != nil {
					if err == io.EOF || err.Error() == "transport is closed" {
						logger.Info("Session %s transport closed.", session.SessionID())
					} else {
						logger.Error("Session %s receive error: %v", session.SessionID(), err)
					}
					break
				}
				if len(rawMessage) == 0 {
					logger.Debug("Session %s received empty message, skipping.", session.SessionID())
					continue
				}
				response := coreServer.HandleMessage(r.Context(), session.SessionID(), rawMessage)
				if response != nil {
					if err := session.SendResponse(*response); err != nil {
						logger.Error("Session %s failed to send response: %v", session.SessionID(), err)
					}
				}
			}
			logger.Info("Message handling loop finished for session %s", session.SessionID())
		}()
		// --- End Copy ---
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(fmt.Sprintf("HTTP server running. MCP WebSocket endpoint at %s", wsPath)))
	})

	httpServer := &http.Server{Addr: cfg.Address, Handler: mux}

	go func() {
		logger.Info("Starting HTTP Server for WebSocket on %s", cfg.Address)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server (WebSocket) ListenAndServe error: %v", err)
			os.Exit(1)
		}
	}()

	waitForShutdownSignal(httpServer, logger, "WebSocket HTTP server")
}

func waitForShutdownSignal(httpServer *http.Server, logger types.Logger, serverType string) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutdown signal received, shutting down %s...", serverType)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("%s shutdown error: %v", serverType, err)
	} else {
		logger.Info("%s gracefully stopped.", serverType)
	}
}

// --- End Helper Functions ---
// Removed duplicated WebSocket Session implementation
