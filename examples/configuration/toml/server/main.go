package main

import (
	"context"
	"encoding/json" // Still needed for session Send methods

	// "flag" // Removed
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"

	// "path/filepath" // Removed
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml" // Keep TOML
	"github.com/gobwas/ws"
	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	mcpServer "github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/transport/websocket"
	"github.com/localrivet/gomcp/types"
	// "gopkg.in/yaml.v3" // Removed
)

// --- Configuration Structs (TOML tags needed) ---

type ToolConfig struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

type TransportConfig struct {
	Type        string `toml:"type"` // "stdio", "sse", "websocket"
	Address     string `toml:"address,omitempty"`
	Path        string `toml:"path,omitempty"`         // For WebSocket
	SSEPath     string `toml:"sse_path,omitempty"`     // For SSE
	MessagePath string `toml:"message_path,omitempty"` // For SSE
}

type AppConfig struct {
	ServerName  string          `toml:"server_name"`
	LoggerLevel string          `toml:"logger_level"` // "debug", "info", "warn", "error"
	Transport   TransportConfig `toml:"transport"`
	Tools       []ToolConfig    `toml:"tools"`
}

// --- Logger Implementation (Identical) ---
type configLogger struct{ level int }

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

func NewConfigLogger(levelStr string) *configLogger {
	level := levelInfo
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

// --- WebSocket Session (Identical) ---
type WebSocketClientSession struct {
	sessionID          string
	transport          *websocket.WebSocketTransport
	initialized        bool
	initMutex          sync.RWMutex
	logger             types.Logger
	negotiatedVersion  string                      // Added to satisfy interface
	clientCapabilities protocol.ClientCapabilities // Added to satisfy interface
}

var _ types.ClientSession = (*WebSocketClientSession)(nil) // Use types.ClientSession

func NewWebSocketClientSession(transport *websocket.WebSocketTransport, logger types.Logger) *WebSocketClientSession {
	return &WebSocketClientSession{sessionID: uuid.NewString(), transport: transport, logger: logger}
}
func (s *WebSocketClientSession) SessionID() string { return s.sessionID }
func (s *WebSocketClientSession) SendResponse(response protocol.JSONRPCResponse) error {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("ws session marshal response: %w", err)
	}
	jsonData = append(jsonData, '\n')
	return s.transport.Send(jsonData)
}
func (s *WebSocketClientSession) SendNotification(notification protocol.JSONRPCNotification) error {
	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("ws session marshal notification: %w", err)
	}
	jsonData = append(jsonData, '\n')
	return s.transport.Send(jsonData)
}
func (s *WebSocketClientSession) Close() error { return s.transport.Close() }
func (s *WebSocketClientSession) Initialized() bool {
	s.initMutex.RLock()
	defer s.initMutex.RUnlock()
	return s.initialized
}
func (s *WebSocketClientSession) Initialize() {
	s.initMutex.Lock()
	defer s.initMutex.Unlock()
	s.initialized = true
}
func (s *WebSocketClientSession) SetNegotiatedVersion(version string) {
	// Add locking if concurrent access is possible
	s.negotiatedVersion = version
}
func (s *WebSocketClientSession) GetNegotiatedVersion() string {
	// Add locking if concurrent access is possible
	return s.negotiatedVersion
}
func (s *WebSocketClientSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	// Add locking if concurrent access is possible
	s.clientCapabilities = caps
}
func (s *WebSocketClientSession) GetClientCapabilities() protocol.ClientCapabilities {
	// Add locking if concurrent access is possible
	return s.clientCapabilities
}

// --- End WebSocket Session ---

// --- Stdio Session (Identical) ---
type StdioClientSession struct {
	sessionID          string
	transport          *stdio.StdioTransport
	initialized        bool
	initMutex          sync.RWMutex
	logger             types.Logger
	negotiatedVersion  string                      // Added to satisfy interface
	clientCapabilities protocol.ClientCapabilities // Added to satisfy interface
}

var _ types.ClientSession = (*StdioClientSession)(nil) // Use types.ClientSession

func NewStdioClientSession(transport *stdio.StdioTransport, logger types.Logger) *StdioClientSession {
	return &StdioClientSession{sessionID: uuid.NewString(), transport: transport, logger: logger}
}
func (s *StdioClientSession) SessionID() string { return s.sessionID }
func (s *StdioClientSession) SendResponse(response protocol.JSONRPCResponse) error {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("stdio session marshal response: %w", err)
	}
	jsonData = append(jsonData, '\n')
	return s.transport.Send(jsonData)
}
func (s *StdioClientSession) SendNotification(notification protocol.JSONRPCNotification) error {
	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("stdio session marshal notification: %w", err)
	}
	jsonData = append(jsonData, '\n')
	return s.transport.Send(jsonData)
}
func (s *StdioClientSession) Close() error { return s.transport.Close() }
func (s *StdioClientSession) Initialized() bool {
	s.initMutex.RLock()
	defer s.initMutex.RUnlock()
	return s.initialized
}
func (s *StdioClientSession) Initialize() {
	s.initMutex.Lock()
	defer s.initMutex.Unlock()
	s.initialized = true
}
func (s *StdioClientSession) SetNegotiatedVersion(version string) {
	// Add locking if concurrent access is possible
	s.negotiatedVersion = version
}
func (s *StdioClientSession) GetNegotiatedVersion() string {
	// Add locking if concurrent access is possible
	return s.negotiatedVersion
}
func (s *StdioClientSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	// Add locking if concurrent access is possible
	s.clientCapabilities = caps
}
func (s *StdioClientSession) GetClientCapabilities() protocol.ClientCapabilities {
	// Add locking if concurrent access is possible
	return s.clientCapabilities
}

// --- End Stdio Session ---

// --- Main Application ---
func main() {
	// Hardcode config file path for TOML example
	configFile := "../config.toml"

	// Load configuration
	cfg, err := loadConfigFromToml(configFile) // Use TOML specific loader
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", configFile, err)
	}

	// Create logger based on config
	logger := NewConfigLogger(cfg.LoggerLevel)
	logger.Info("Configuration loaded from %s", configFile)
	logger.Info("Server Name: %s, Log Level: %s", cfg.ServerName, cfg.LoggerLevel)
	logger.Info("Transport Type: %s", cfg.Transport.Type)

	// Create the core MCP Server
	coreServer := mcpServer.NewServer(
		cfg.ServerName,
		mcpServer.WithLogger(logger), // Use functional option
	)

	// Register tools from config
	registerTools(coreServer, cfg.Tools, logger)

	// Start the configured transport
	startTransport(coreServer, cfg.Transport, logger)

	logger.Info("Server shutdown complete.")
}

// --- Helper Functions ---

func loadConfigFromToml(filePath string) (*AppConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg AppConfig
	err = toml.Unmarshal(data, &cfg) // Use toml.Unmarshal (returns only error)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
	}
	// Basic validation
	if cfg.ServerName == "" {
		return nil, fmt.Errorf("server_name is required")
	}
	if cfg.Transport.Type == "" {
		return nil, fmt.Errorf("transport.type is required")
	}
	return &cfg, nil
}

// registerTools (Identical)
func registerTools(server *mcpServer.Server, tools []ToolConfig, logger types.Logger) {
	for _, toolCfg := range tools {
		tool := protocol.Tool{Name: toolCfg.Name, Description: toolCfg.Description, InputSchema: protocol.ToolInputSchema{Type: "string"}}
		var handler mcpServer.ToolHandlerFunc
		switch toolCfg.Name {
		case "echo":
			handler = createEchoHandler(logger)
		case "ping":
			handler = createPingHandler(logger)
		default:
			logger.Warn("No handler for tool: %s", toolCfg.Name)
			continue
		}
		if err := server.RegisterTool(tool, handler); err != nil {
			logger.Error("Failed to register tool '%s': %v", toolCfg.Name, err)
		} else {
			logger.Info("Registered tool from config: %s", toolCfg.Name)
		}
	}
}

// Example tool handlers (Identical)
func createEchoHandler(logger types.Logger) mcpServer.ToolHandlerFunc {
	return func(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) {
		inputText := "nil"
		if args != nil {
			if strArg, ok := args.(map[string]interface{})["input"].(string); ok {
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
		content = []protocol.Content{protocol.TextContent{Type: "text", Text: "pong"}}
		return content, false
	}
}

// startTransport (Identical)
func startTransport(coreServer *mcpServer.Server, cfg TransportConfig, logger types.Logger) {
	switch strings.ToLower(cfg.Type) {
	case "stdio":
		startStdioTransport(coreServer, logger)
	case "sse":
		startSSETransport(coreServer, cfg, logger)
	case "websocket":
		startWebSocketTransport(coreServer, cfg, logger)
	default:
		logger.Error("Unsupported transport type: %s", cfg.Type)
		os.Exit(1)
	}
}

// startStdioTransport (Identical)
func startStdioTransport(coreServer *mcpServer.Server, logger types.Logger) {
	logger.Info("Starting Stdio transport")
	stdioTransport := stdio.NewStdioTransportWithOptions(types.TransportOptions{Logger: logger})
	session := NewStdioClientSession(stdioTransport, logger)
	if err := coreServer.RegisterSession(session); err != nil {
		logger.Error("Failed to register stdio session: %v", err)
		os.Exit(1)
	}
	logger.Info("Registered stdio session: %s", session.SessionID())
	go func() {
		defer func() {
			logger.Info("Unregistering stdio session %s", session.SessionID())
			coreServer.UnregisterSession(session.SessionID())
			stdioTransport.Close()
		}()
		for {
			rawMessage, err := stdioTransport.ReceiveWithContext(context.Background())
			if err != nil {
				if err == io.EOF {
					logger.Info("Stdio transport closed (EOF).")
				} else {
					logger.Error("Stdio receive error: %v", err)
				}
				break
			}
			if len(rawMessage) == 0 {
				continue
			}
			responses := coreServer.HandleMessage(context.Background(), session.SessionID(), rawMessage)
			// HandleMessage returns a slice, iterate over it
			for _, resp := range responses {
				if resp == nil {
					continue
				}
				if err := session.SendResponse(*resp); err != nil { // Send each response individually
					logger.Error("Stdio failed to send response: %v", err)
				}
			}
			// Removed extra closing brace here
		}
		logger.Info("Stdio message handling loop finished.")
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutdown signal received, closing stdio transport...")
	session.Close()
}

// startSSETransport (Identical)
func startSSETransport(coreServer *mcpServer.Server, cfg TransportConfig, logger types.Logger) {
	if cfg.Address == "" {
		logger.Error("SSE transport requires 'address'")
		os.Exit(1)
	}
	ssePath := cfg.SSEPath
	if ssePath == "" {
		ssePath = "/sse"
	}
	messagePath := cfg.MessagePath
	if messagePath == "" {
		messagePath = "/message"
	}
	logger.Info("Starting SSE transport on %s (SSE Path: %s, Message Path: %s)", cfg.Address, ssePath, messagePath)
	sseTransportServer := sse.NewSSEServer(coreServer, sse.SSEServerOptions{Logger: logger, SSEEndpoint: ssePath, MessageEndpoint: messagePath})
	mux := http.NewServeMux()
	// Use the path variables directly
	mux.HandleFunc(ssePath, sseTransportServer.HandleSSE)
	mux.HandleFunc(messagePath, sseTransportServer.HandleMessage)
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
			logger.Error("HTTP server (SSE) error: %v", err)
			os.Exit(1)
		}
	}()
	waitForShutdownSignal(httpServer, logger, "SSE HTTP server")
}

// startWebSocketTransport (Identical)
func startWebSocketTransport(coreServer *mcpServer.Server, cfg TransportConfig, logger types.Logger) {
	if cfg.Address == "" {
		logger.Error("WebSocket transport requires 'address'")
		os.Exit(1)
	}
	wsPath := cfg.Path
	if wsPath == "" {
		wsPath = "/mcp"
	}
	logger.Info("Starting WebSocket transport on %s (Path: %s)", cfg.Address, wsPath)
	upgrader := ws.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	mux := http.NewServeMux()
	mux.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) {
		logger.Info("WS upgrade request from %s", r.RemoteAddr)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			logger.Error("WS hijacking not supported")
			http.Error(w, "Hijacking not supported", 500)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			logger.Error("WS hijack failed: %v", err)
			http.Error(w, "Hijack failed", 500)
			return
		}
		defer func() {
			if err != nil {
				logger.Warn("Closing hijacked conn due to error: %v", err)
				conn.Close()
			}
		}()
		hs, err := upgrader.Upgrade(rw)
		if err != nil {
			logger.Error("WS handshake failed: %v", err)
			return
		}
		logger.Debug("WS Handshake: Proto: %s", hs.Protocol)
		logger.Info("WS connection established with %s", conn.RemoteAddr())
		if err = rw.Flush(); err != nil {
			logger.Error("WS flush failed: %v", err)
			return
		}
		transportOpts := types.TransportOptions{Logger: logger}
		mcpTransport := websocket.NewWebSocketTransport(conn, ws.StateServerSide, transportOpts)
		session := NewWebSocketClientSession(mcpTransport, logger)
		if err := coreServer.RegisterSession(session); err != nil {
			logger.Error("WS register session %s failed: %v", session.SessionID(), err)
			mcpTransport.Close()
			return
		}
		logger.Info("WS registered session %s for %s", session.SessionID(), conn.RemoteAddr())
		go func() {
			defer func() {
				logger.Info("WS unregistering session %s for %s", session.SessionID(), conn.RemoteAddr())
				coreServer.UnregisterSession(session.SessionID())
				mcpTransport.Close()
			}()
			for {
				rawMessage, err := mcpTransport.ReceiveWithContext(context.Background())
				if err != nil {
					if err == io.EOF || err.Error() == "transport is closed" {
						logger.Info("WS session %s closed.", session.SessionID())
					} else {
						logger.Error("WS session %s receive error: %v", session.SessionID(), err)
					}
					break
				}
				if len(rawMessage) == 0 {
					continue
				}
				responses := coreServer.HandleMessage(r.Context(), session.SessionID(), rawMessage)
				// HandleMessage returns a slice, iterate over it
				for _, resp := range responses {
					if resp == nil {
						continue
					}
					if err := session.SendResponse(*resp); err != nil { // Send each response individually
						logger.Error("WS session %s send response error: %v", session.SessionID(), err)
					}
				}
				// Removed extra closing brace here
			}
			logger.Info("WS message loop finished for session %s", session.SessionID())
		}()
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
			logger.Error("HTTP server (WS) error: %v", err)
			os.Exit(1)
		}
	}()
	waitForShutdownSignal(httpServer, logger, "WebSocket HTTP server")
}

// waitForShutdownSignal (Identical)
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
