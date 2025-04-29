package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io" // Added for fatalf
	"net"
	"net/http"
	"sync"

	"github.com/gobwas/ws"
	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/websocket" // Import our transport
	"github.com/localrivet/gomcp/types"
)

// --- WebSocket Session ---
// wsSession implements types.ClientSession for a WebSocket connection.
// Defined locally within the server package for use by ServeWebSocket.
type wsSession struct {
	sessionID          string
	transport          *websocket.WebSocketTransport
	initialized        bool
	initMutex          sync.RWMutex
	logger             types.Logger
	negotiatedVersion  string
	clientCapabilities protocol.ClientCapabilities
}

// Ensure wsSession implements types.ClientSession
var _ types.ClientSession = (*wsSession)(nil)

func newWebSocketSession(transport *websocket.WebSocketTransport, logger types.Logger) *wsSession {
	return &wsSession{
		sessionID: uuid.NewString(),
		transport: transport,
		logger:    logger,
	}
}

func (s *wsSession) SessionID() string { return s.sessionID }
func (s *wsSession) SendResponse(response protocol.JSONRPCResponse) error {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("ws session marshal response: %w", err)
	}
	jsonData = append(jsonData, '\n') // MCP Framing
	// Use context.Background() as responses are server-initiated
	return s.transport.Send(context.Background(), jsonData)
}
func (s *wsSession) SendNotification(notification protocol.JSONRPCNotification) error {
	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("ws session marshal notification: %w", err)
	}
	jsonData = append(jsonData, '\n') // MCP Framing
	// Use context.Background() as notifications are server-initiated
	return s.transport.Send(context.Background(), jsonData)
}
func (s *wsSession) Close() error { return s.transport.Close() }
func (s *wsSession) Initialized() bool {
	s.initMutex.RLock()
	defer s.initMutex.RUnlock()
	return s.initialized
}
func (s *wsSession) Initialize() {
	s.initMutex.Lock()
	defer s.initMutex.Unlock()
	s.initialized = true
}
func (s *wsSession) SetNegotiatedVersion(version string) {
	// Add locking if concurrent access is possible
	s.negotiatedVersion = version
}
func (s *wsSession) GetNegotiatedVersion() string {
	// Add locking if concurrent access is possible
	return s.negotiatedVersion
}
func (s *wsSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	// Add locking if concurrent access is possible
	s.clientCapabilities = caps
}
func (s *wsSession) GetClientCapabilities() protocol.ClientCapabilities {
	// Add locking if concurrent access is possible
	return s.clientCapabilities
}

// --- End WebSocket Session ---

// ServeWebSocket runs the MCP server, handling connections via WebSockets.
// It listens on the specified network address (e.g., ":8080") and handles
// upgrade requests at the given path (e.g., "/mcp").
func ServeWebSocket(srv *Server, addr string, path string) error {
	logger := srv.logger // Use the server's configured logger

	if path == "" {
		path = "/" // Default path if none provided
	}
	if path[0] != '/' {
		path = "/" + path
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Received WebSocket upgrade request from %s for path %s", r.RemoteAddr, r.URL.Path)

		// Upgrade connection using gobwas/ws
		// The Upgrade function handles writing the 101 response on success or error response on failure.
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			// Error is logged by ws.UpgradeHTTP or handled by writing response
			logger.Error("WebSocket upgrade failed: %v", err)
			// No need to write http.Error here as UpgradeHTTP does it.
			return
		}
		logger.Info("WebSocket connection established with %s", conn.RemoteAddr())

		// Create the MCP WebSocket transport wrapper using the upgraded net.Conn
		transportOpts := types.TransportOptions{Logger: logger}
		mcpTransport := websocket.NewWebSocketTransport(conn, ws.StateServerSide, transportOpts)

		// Create and register the session
		session := newWebSocketSession(mcpTransport, logger)
		if err := srv.RegisterSession(session); err != nil {
			logger.Error("Failed to register WebSocket session %s: %v", session.SessionID(), err)
			mcpTransport.Close() // Close the transport/connection
			return
		}
		logger.Info("Registered WebSocket session %s for %s", session.SessionID(), conn.RemoteAddr())

		// Start the message handling loop for this session in a new goroutine
		go func() {
			defer func() {
				logger.Info("Unregistering WebSocket session %s for %s", session.SessionID(), conn.RemoteAddr())
				srv.UnregisterSession(session.SessionID())
				// Transport Close should be called implicitly by Receive error or explicitly via session.Close() elsewhere
			}()

			ctx := context.Background() // Base context for message handling loop

			for {
				// Receive raw message using the transport
				// Use background context for read loop; cancellation happens via Close()
				// Use the updated Receive method
				rawMessage, err := mcpTransport.Receive(ctx)
				if err != nil {
					// Check for expected closure errors
					if errors.Is(err, io.EOF) || err.Error() == "transport is closed" || errors.Is(err, net.ErrClosed) {
						logger.Info("WebSocket session %s closed.", session.SessionID())
					} else {
						// Log unexpected errors
						logger.Error("WebSocket session %s receive error: %v", session.SessionID(), err)
					}
					break // Exit loop on any error or closure
				}

				if len(rawMessage) == 0 {
					logger.Debug("WebSocket session %s received empty message, skipping.", session.SessionID())
					continue
				}

				// Handle the message using the core server logic
				// Use request's context if available and meaningful, otherwise background
				handlerCtx := r.Context() // Or context.Background() if request context isn't suitable long-term
				responses := srv.HandleMessage(handlerCtx, session.SessionID(), rawMessage)

				// Send response(s) back through the session
				for _, resp := range responses {
					if resp == nil {
						continue
					}
					if err := session.SendResponse(*resp); err != nil {
						logger.Error("WebSocket session %s send response error: %v", session.SessionID(), err)
						// Decide if we should break the loop on send error
						// break // Example: break if sending fails
					}
				}
			}
			logger.Info("WebSocket message handling loop finished for session %s", session.SessionID())
		}() // End of goroutine
	}) // End of HandleFunc

	// Add a simple root handler for testing the HTTP server itself
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("HTTP server running. MCP WebSocket endpoint at %s", path)))
	})

	// Start the HTTP server
	printBanner()
	logger.Info("Starting MCP server with WebSocket transport...")
	logger.Info("Listening on: %s", addr)
	logger.Info("WebSocket Endpoint Path: %s", path)

	err := http.ListenAndServe(addr, mux)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server (WebSocket) error: %v", err)
	} else {
		logger.Info("HTTP server (WebSocket) stopped.")
	}
	return err // Return the error from ListenAndServe
}
