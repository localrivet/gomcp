package server

import (
	"context"       // Added for context.Context
	"encoding/json" // Added for encoding/json
	"net/http"
	"strings"
	"time"

	"github.com/localrivet/gomcp/protocol" // Added for protocol types
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/types" // Added for types.ClientSession
)

// serverLogicAdapter adapts the Server struct to implement sse.MCPServerLogic.
type serverLogicAdapter struct {
	server *Server
}

// HandleMessage implements the sse.MCPServerLogic interface.
func (a *serverLogicAdapter) HandleMessage(ctx context.Context, session types.ClientSession, rawMessage json.RawMessage) []*protocol.JSONRPCResponse {
	// Delegate to the Server's MessageHandler.HandleMessage
	// Note: MessageHandler now returns error and sends responses internally.
	// The SSE transport might not use the returned []*JSONRPCResponse.
	// Log errors and return nil.
	if err := a.server.MessageHandler.HandleMessage(session, []byte(rawMessage)); err != nil {
		// Use the server's logger
		a.server.logger.Error("SSE Adapter: Error handling message for session %s: %v", session.SessionID(), err)
	}
	return nil // Return nil as responses are sent via session
}

// RegisterSession implements the sse.MCPServerLogic interface.
func (a *serverLogicAdapter) RegisterSession(session types.ClientSession) error {
	// Directly add the session to the TransportManager
	sessionID := session.SessionID()
	tm := a.server.TransportManager
	tm.sessionsMu.Lock()
	tm.Sessions[sessionID] = session
	tm.sessionsMu.Unlock()
	a.server.logger.Info("SSE Adapter: Registered session %s", sessionID)
	return nil
}

// UnregisterSession implements the sse.MCPServerLogic interface.
func (a *serverLogicAdapter) UnregisterSession(sessionID string) {
	// Call TransportManager.RemoveSession and SubscriptionManager.UnsubscribeAll
	tm := a.server.TransportManager
	tm.RemoveSession(sessionID) // This already logs removal
	a.server.SubscriptionManager.UnsubscribeAll(sessionID)
	a.server.logger.Info("SSE Adapter: Unregistered session %s and unsubscribed resources", sessionID)
}

// runSseTransport runs the MCP server, handling connections via Server-Sent Events (SSE)
// over HTTP using the implementation from the transport/sse package.
// It listens on the configured network address and base path.
func (tm *TransportManager) runSseTransport(s *Server) error {
	logger := s.logger // Use the main server logger

	// Normalize base path
	basePath := tm.sseBasePath
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	// Ensure trailing slash for Handle registration
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	// Create the SSE handler options
	// Note: transport/sse NewSSEServer expects MCPServerLogic, which *Server implements.
	sseOpts := sse.SSEServerOptions{
		Logger:   logger,
		BasePath: basePath, // Pass the normalized base path
		// ContextFunc: // Optional: Add if context modification per request is needed
	}
	// The sse.NewSSEServer likely needs the main server instance to access MessageHandler, etc.
	// Assuming sse.NewSSEServer accepts an interface that *Server implements.
	sseHandler := sse.NewSSEServer(&serverLogicAdapter{server: s}, sseOpts) // Pass the adapter

	// Create a new ServeMux and register the handler
	mux := http.NewServeMux()
	mux.Handle(basePath, sseHandler) // Handle base path and all sub-paths

	logger.Info("Starting MCP server with SSE transport...")
	logger.Info("Listening on: %s", tm.sseAddr)
	logger.Info("SSE Endpoint Base Path: %s (SSE at %ssse, Messages at %smessage)",
		strings.TrimSuffix(basePath, "/"),
		basePath, // Use the basePath which includes trailing slash here
		basePath) // Use the basePath which includes trailing slash here
	logger.Info("GET requests to base path (%s or %s) will be redirected to the SSE endpoint",
		strings.TrimSuffix(basePath, "/"),
		strings.TrimSuffix(basePath, "/")+"/")

	// Configure and start the HTTP server explicitly to set timeouts
	// SSE connections need longer timeouts, especially IdleTimeout.
	httpServer := &http.Server{
		Addr:    tm.sseAddr,
		Handler: mux,
		// Set timeouts appropriate for long-lived SSE connections
		// IdleTimeout prevents the server from keeping idle connections open indefinitely.
		// Set a much longer timeout when potentially used with proxies like mcp-remote.
		// ReadHeaderTimeout is also good practice. WriteTimeout might be less critical for SSE.
		IdleTimeout:       10 * time.Minute, // Increased to 10 minutes
		ReadHeaderTimeout: 30 * time.Second, // Increased slightly
		// WriteTimeout: 60 * time.Second, // Optional: Might interrupt long SSE pushes if set too low
	}

	logger.Info("Starting HTTP server with timeouts (Idle: %v, ReadHeader: %v)...", httpServer.IdleTimeout, httpServer.ReadHeaderTimeout)
	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed { // Ignore ErrServerClosed on graceful shutdown
		logger.Error("HTTP server ListenAndServe error: %v", err)
	}
	return err
}
