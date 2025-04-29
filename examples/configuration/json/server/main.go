// Command json-config-demo demonstrates loading MCP server configuration from JSON files.
//
// Features
// --------
//
//   - JSON configuration loading
//   - Multiple transport support (stdio, WebSocket, SSE)
//   - Configurable logging levels
//   - Dynamic tool registration
//
// Build & run:
//
//	go run .
//
// Example request via stdio:
//
//	{"method":"echo","params":{"message":"Hello from JSON config!"}}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/transport/websocket"
	"github.com/localrivet/gomcp/types"
	"github.com/localrivet/gomcp/util/schema"

	"github.com/gobwas/ws"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Configuration Types
// ---------------------------------------------------------------------------

type ToolConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TransportConfig struct {
	Type        string `json:"type"`
	Address     string `json:"address,omitempty"`
	Path        string `json:"path,omitempty"`
	SSEPath     string `json:"sse_path,omitempty"`
	MessagePath string `json:"message_path,omitempty"`
}

type AppConfig struct {
	ServerName  string          `json:"server_name"`
	LoggerLevel string          `json:"logger_level"`
	Transport   TransportConfig `json:"transport"`
	Tools       []ToolConfig    `json:"tools"`
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
// Session Types
// ---------------------------------------------------------------------------

type WebSocketClientSession struct {
	sessionID          string
	transport          *websocket.WebSocketTransport
	initialized        bool
	initMutex          sync.RWMutex
	logger             types.Logger
	negotiatedVersion  string
	clientCapabilities protocol.ClientCapabilities
}

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

func (s *WebSocketClientSession) Close() error                        { return s.transport.Close() }
func (s *WebSocketClientSession) Initialized() bool                   { return s.initialized }
func (s *WebSocketClientSession) Initialize()                         { s.initialized = true }
func (s *WebSocketClientSession) SetNegotiatedVersion(version string) { s.negotiatedVersion = version }
func (s *WebSocketClientSession) GetNegotiatedVersion() string        { return s.negotiatedVersion }
func (s *WebSocketClientSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	s.clientCapabilities = caps
}
func (s *WebSocketClientSession) GetClientCapabilities() protocol.ClientCapabilities {
	return s.clientCapabilities
}

type StdioClientSession struct {
	sessionID          string
	transport          *stdio.StdioTransport
	initialized        bool
	initMutex          sync.RWMutex
	logger             types.Logger
	negotiatedVersion  string
	clientCapabilities protocol.ClientCapabilities
}

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

func (s *StdioClientSession) Close() error                        { return s.transport.Close() }
func (s *StdioClientSession) Initialized() bool                   { return s.initialized }
func (s *StdioClientSession) Initialize()                         { s.initialized = true }
func (s *StdioClientSession) SetNegotiatedVersion(version string) { s.negotiatedVersion = version }
func (s *StdioClientSession) GetNegotiatedVersion() string        { return s.negotiatedVersion }
func (s *StdioClientSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	s.clientCapabilities = caps
}
func (s *StdioClientSession) GetClientCapabilities() protocol.ClientCapabilities {
	return s.clientCapabilities
}

// ---------------------------------------------------------------------------
// Helper Functions
// ---------------------------------------------------------------------------

func loadConfigFromJson(filePath string) (*AppConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Validate required fields
	if config.ServerName == "" {
		return nil, fmt.Errorf("server_name is required")
	}
	if config.Transport.Type == "" {
		return nil, fmt.Errorf("transport.type is required")
	}

	return &config, nil
}

func registerTools(svr *server.Server, tools []ToolConfig, logger types.Logger) {
	for _, tool := range tools {
		var handler svr.ToolHandlerFunc
		switch tool.Name {
		case "echo":
			handler = createEchoHandler(logger)
		case "ping":
			handler = createPingHandler(logger)
		default:
			logger.Warn("unknown tool: %s", tool.Name)
			continue
		}

		if err := server.AddTool(svr, tool.Name, tool.Description, handler); err != nil {
			logger.Error("failed to register tool %s: %v", tool.Name, err)
		}
	}
}

func createEchoHandler(logger types.Logger) server.ToolHandlerFunc {
	return func(args EchoArgs) (protocol.Content, error) {
		// Validate arguments using schema
		if err := schema.Validate(args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		logger.Debug("echo handler: %s", args.Message)
		return server.Text(args.Message), nil
	}
}

func createPingHandler(logger types.Logger) server.ToolHandlerFunc {
	return func(_ PingArgs) (protocol.Content, error) {
		return server.Text("pong"), nil
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
		startSSETransport(svr, cfg, logger)
	case "websocket":
		startWebSocketTransport(svr, cfg, logger)
	default:
		logger.Error("unsupported transport type: %s", cfg.Type)
		os.Exit(1)
	}
}

func startStdioTransport(svr *server.Server, logger types.Logger) {
	transport := stdio.NewStdioTransport()
	session := NewStdioClientSession(transport, logger)

	go func() {
		for {
			data, err := transport.Receive()
			if err != nil {
				if err == io.EOF {
					logger.Info("stdio connection closed")
					return
				}
				logger.Error("stdio receive error: %v", err)
				continue
			}

			var request protocol.JSONRPCRequest
			if err := json.Unmarshal(data, &request); err != nil {
				logger.Error("stdio parse error: %v", err)
				continue
			}

			if err := svr.HandleRequest(session, &request); err != nil {
				logger.Error("stdio handle error: %v", err)
			}
		}
	}()

	logger.Info("stdio transport ready")
	select {} // Block forever
}

func startSSETransport(svr *server.Server, cfg TransportConfig, logger types.Logger) {
	if cfg.Address == "" {
		logger.Error("SSE transport requires address")
		os.Exit(1)
	}

	transport := sse.NewSSETransport()
	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc(cfg.SSEPath, func(w http.ResponseWriter, r *http.Request) {
		logger.Info("new SSE connection from %s", r.RemoteAddr)
		transport.ServeHTTP(w, r)
	})

	// Message endpoint
	mux.HandleFunc(cfg.MessagePath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var request protocol.JSONRPCRequest
		if err := json.Unmarshal(data, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := svr.HandleRequest(nil, &request); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: mux,
	}

	go func() {
		logger.Info("starting SSE server on %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("SSE server error: %v", err)
			os.Exit(1)
		}
	}()

	waitForShutdownSignal(server, logger, "SSE")
}

func startWebSocketTransport(svr *server.Server, cfg TransportConfig, logger types.Logger) {
	if cfg.Address == "" {
		logger.Error("WebSocket transport requires address")
		os.Exit(1)
	}

	sessions := make(map[string]*WebSocketClientSession)
	var sessionsMutex sync.RWMutex

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Path, func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			logger.Error("ws upgrade error: %v", err)
			return
		}

		transport := websocket.NewWebSocketTransport(conn)
		session := NewWebSocketClientSession(transport, logger)

		sessionsMutex.Lock()
		sessions[session.SessionID()] = session
		sessionsMutex.Unlock()

		logger.Info("new ws connection from %s (session: %s)", r.RemoteAddr, session.SessionID())

		go func() {
			defer func() {
				sessionsMutex.Lock()
				delete(sessions, session.SessionID())
				sessionsMutex.Unlock()
				session.Close()
				logger.Info("ws connection closed (session: %s)", session.SessionID())
			}()

			for {
				data, err := transport.Receive()
				if err != nil {
					if err == io.EOF {
						return
					}
					logger.Error("ws receive error: %v", err)
					continue
				}

				var request protocol.JSONRPCRequest
				if err := json.Unmarshal(data, &request); err != nil {
					logger.Error("ws parse error: %v", err)
					continue
				}

				if err := svr.HandleRequest(session, &request); err != nil {
					logger.Error("ws handle error: %v", err)
				}
			}
		}()
	})

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: mux,
	}

	go func() {
		logger.Info("starting WebSocket server on %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("WebSocket server error: %v", err)
			os.Exit(1)
		}
	}()

	waitForShutdownSignal(server, logger, "WebSocket")
}

func waitForShutdownSignal(httpServer *http.Server, logger types.Logger, serverType string) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down %s server...", serverType)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// Load configuration
	config, err := loadConfigFromJson("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := logx.NewLogger(config.LoggerLevel)

	// Create MCP server instance
	srv := server.NewServer(config.ServerName)

	// ---------------------------------------------------------------------
	// Register configured tools
	// ---------------------------------------------------------------------
	registerTools(srv, config.Tools, logger)

	// ---------------------------------------------------------------------
	// Start configured transport
	// ---------------------------------------------------------------------
	startTransport(srv, config.Transport, logger)
}
