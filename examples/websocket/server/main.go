package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io" // For io.EOF
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gobwas/ws" // Import gobwas/ws for upgrade
	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	mcpServer "github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/websocket" // Import our transport implementation
	"github.com/localrivet/gomcp/types"
)

// Simple logger implementation
type simpleLogger struct{}

func (l *simpleLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *simpleLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *simpleLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *simpleLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

var _ types.Logger = (*simpleLogger)(nil)

// WebSocketClientSession implements server.ClientSession for WebSocket transport
type WebSocketClientSession struct {
	sessionID          string
	transport          *websocket.WebSocketTransport
	initialized        bool
	initMutex          sync.RWMutex
	logger             types.Logger
	negotiatedVersion  string                      // Added to satisfy interface
	clientCapabilities protocol.ClientCapabilities // Added to satisfy interface
}

// Ensure WebSocketClientSession implements types.ClientSession
var _ types.ClientSession = (*WebSocketClientSession)(nil) // Use types.ClientSession

func NewWebSocketClientSession(transport *websocket.WebSocketTransport, logger types.Logger) *WebSocketClientSession {
	return &WebSocketClientSession{
		sessionID: uuid.NewString(),
		transport: transport,
		logger:    logger,
	}
}

func (s *WebSocketClientSession) SessionID() string {
	return s.sessionID
}

func (s *WebSocketClientSession) SendResponse(response protocol.JSONRPCResponse) error {
	s.logger.Debug("Session %s: Sending response: %+v", s.sessionID, response)
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	// Append newline for MCP framing
	jsonData = append(jsonData, '\n')
	return s.transport.Send(jsonData)
}

func (s *WebSocketClientSession) SendNotification(notification protocol.JSONRPCNotification) error {
	s.logger.Debug("Session %s: Sending notification: %+v", s.sessionID, notification)
	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	// Append newline for MCP framing
	jsonData = append(jsonData, '\n')
	return s.transport.Send(jsonData)
}

func (s *WebSocketClientSession) Close() error {
	return s.transport.Close()
}

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

func main() {
	logger := &simpleLogger{}
	listenAddr := "127.0.0.1:8092"
	wsPath := "/mcp"

	// 1. Create the core MCP Server
	coreServer := mcpServer.NewServer(
		"WebSocketExampleMCPServer",
		mcpServer.WithLogger(logger), // Use functional option
	)

	// Register example tool
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
			content = []protocol.Content{protocol.TextContent{Type: "text", Text: resultText}}
			return content, false
		},
	)

	// 2. Setup HTTP Server and WebSocket Handler
	upgrader := ws.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Received WebSocket upgrade request from %s", r.RemoteAddr)

		// Hijack the connection to get the underlying net.Conn
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
		// Important: Close the connection if anything goes wrong after hijacking
		defer func() {
			if err != nil {
				logger.Warn("Closing hijacked connection due to error: %v", err)
				conn.Close()
			}
		}()

		// Perform the WebSocket handshake using the hijacked connection (io.ReadWriter)
		// The Upgrade function handles writing the 101 response on success.
		hs, err := upgrader.Upgrade(rw) // Pass the buffered ReadWriter
		if err != nil {
			// Upgrade writes the error response, just log and return
			logger.Error("WebSocket handshake failed: %v", err)
			// conn is closed by the defer above
			return
		}

		// Log handshake details if needed
		logger.Debug("Handshake details: Proto: %s", hs.Protocol) // Removed Method and RequestURI as they are not fields of ws.Handshake
		logger.Info("WebSocket connection established with %s", conn.RemoteAddr())

		// Flush the buffered writer to ensure the 101 response is sent
		if err = rw.Flush(); err != nil {
			logger.Error("Failed to flush hijacked connection writer after upgrade: %v", err)
			// conn is closed by the defer above
			return
		}

		// Create the MCP WebSocket transport wrapper using the raw net.Conn
		transportOpts := types.TransportOptions{Logger: logger}
		// Pass the original net.Conn, not the buffered rw
		mcpTransport := websocket.NewWebSocketTransport(conn, ws.StateServerSide, transportOpts)

		// Create and register the session
		session := NewWebSocketClientSession(mcpTransport, logger)
		if err := coreServer.RegisterSession(session); err != nil {
			logger.Error("Failed to register session %s: %v", session.SessionID(), err)
			mcpTransport.Close()
			return
		}
		logger.Info("Registered session %s for %s", session.SessionID(), conn.RemoteAddr())

		// Start reading messages from the transport in a loop
		go func() {
			defer func() {
				logger.Info("Unregistering session %s for %s", session.SessionID(), conn.RemoteAddr())
				coreServer.UnregisterSession(session.SessionID())
				mcpTransport.Close() // Ensure underlying connection is closed
			}()

			for {
				// Use background context for reads, cancellation handled by transport Close()
				rawMessage, err := mcpTransport.ReceiveWithContext(context.Background())
				if err != nil {
					if err == io.EOF || err.Error() == "transport is closed" {
						logger.Info("Session %s transport closed.", session.SessionID())
					} else {
						logger.Error("Session %s receive error: %v", session.SessionID(), err)
					}
					break // Exit loop on error or close
				}

				if len(rawMessage) == 0 {
					logger.Debug("Session %s received empty message, skipping.", session.SessionID())
					continue
				}

				// Handle the message using the core server
				// Use request context for message handling logic
				responses := coreServer.HandleMessage(r.Context(), session.SessionID(), rawMessage)

				// Send response(s) if any were generated
				for _, resp := range responses {
					if resp == nil {
						continue
					}
					if err := session.SendResponse(*resp); err != nil { // Send each response individually
						logger.Error("Session %s failed to send response: %v", session.SessionID(), err)
						// Optionally break loop on send error? Depends on desired behavior.
					}
				}
			}
			logger.Info("Message handling loop finished for session %s", session.SessionID())
		}()
	})

	// Add a simple root handler for testing the HTTP server itself
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("HTTP server running. MCP WebSocket endpoint at %s", wsPath)))
	})

	// 3. Create and start the HTTP server
	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		logger.Info("Starting HTTP Server for WebSocket on %s", listenAddr)
		logger.Info("MCP WebSocket endpoint available at ws://%s%s", listenAddr, wsPath)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server ListenAndServe error: %v", err)
			os.Exit(1)
		}
	}()

	// 4. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	logger.Info("Shutdown signal received, shutting down HTTP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	} else {
		logger.Info("HTTP server gracefully stopped.")
	}

	logger.Info("Server shutdown complete.")
}
