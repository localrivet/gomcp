// Package sse provides MCP server implementation over Server-Sent Events (SSE)
// using a hybrid approach (SSE for server->client, HTTP POST for client->server).
// This implementation uses standard net/http without external SSE libraries.
package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	// Only needed for potential timeouts, not currently used
	"context" // Needed for MCPServerLogic interface

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types" // For Logger and ClientSession interface
)

// ClientSession interface is now defined in the types package.
// We will use types.ClientSession directly.

// MCPServerLogic defines the interface SSEServer needs from the core server logic,
// using the ClientSession interface defined in the types package.
type MCPServerLogic interface {
	HandleMessage(ctx context.Context, sessionID string, rawMessage json.RawMessage) []*protocol.JSONRPCResponse
	RegisterSession(session types.ClientSession) error // Use types.ClientSession
	UnregisterSession(sessionID string)
}

// sseSession represents an active SSE connection and implements the types.ClientSession interface.
type sseSession struct {
	writer              http.ResponseWriter
	flusher             http.Flusher
	done                chan struct{}                     // Closed when the connection is done
	eventQueue          chan string                       // Channel for queuing formatted SSE event strings
	sessionID           string                            // Our internal unique session ID
	notificationChannel chan protocol.JSONRPCNotification // Channel for receiving notifications from MCPServer (alternative to direct Send) - DEPRECATED?
	initialized         atomic.Bool
	logger              types.Logger
	negotiatedVersion   string                      // Stores the protocol version agreed upon
	clientCapabilities  protocol.ClientCapabilities // Added
}

// Ensure sseSession implements the types.ClientSession interface
var _ types.ClientSession = (*sseSession)(nil)

// NewSSESession creates a new session.
func newSSESession(w http.ResponseWriter, flusher http.Flusher, logger types.Logger) *sseSession {
	return &sseSession{
		writer:     w,
		flusher:    flusher,
		done:       make(chan struct{}),
		eventQueue: make(chan string, 100), // Buffered queue
		sessionID:  uuid.NewString(),
		// notificationChannel: make(chan protocol.JSONRPCNotification, 100), // Maybe not needed if server calls SendNotification directly
		logger: logger,
	}
}

func (s *sseSession) SessionID() string {
	return s.sessionID
}

// SendNotification formats and queues a notification to be sent over the SSE stream.
func (s *sseSession) SendNotification(notification protocol.JSONRPCNotification) error {
	eventData, err := json.Marshal(notification)
	if err != nil {
		s.logger.Error("Session %s: Failed to marshal notification %s: %v", s.sessionID, notification.Method, err)
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Format as SSE event string
	eventString := fmt.Sprintf("event: message\ndata: %s\n\n", string(eventData))

	// Queue the event string
	select {
	case s.eventQueue <- eventString:
		s.logger.Debug("Session %s: Queued notification %s", s.sessionID, notification.Method)
		return nil
	case <-s.done:
		s.logger.Warn("Session %s: Attempted to send notification %s but session is done.", s.sessionID, notification.Method)
		return fmt.Errorf("session closed")
	default:
		// Should not happen with a buffered channel unless extremely backed up
		s.logger.Error("Session %s: Event queue full when sending notification %s", s.sessionID, notification.Method)
		return fmt.Errorf("event queue full")
	}
}

// SendResponse formats and queues a response to be sent over the SSE stream.
func (s *sseSession) SendResponse(response protocol.JSONRPCResponse) error {
	eventData, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("Session %s: Failed to marshal response for ID %v: %v", s.sessionID, response.ID, err)
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Format as SSE event string
	eventString := fmt.Sprintf("event: message\ndata: %s\n\n", string(eventData))

	// Queue the event string
	select {
	case s.eventQueue <- eventString:
		s.logger.Debug("Session %s: Queued response for ID %v", s.sessionID, response.ID)
		return nil
	case <-s.done:
		s.logger.Warn("Session %s: Attempted to send response for ID %v but session is done.", s.sessionID, response.ID)
		return fmt.Errorf("session closed")
	default:
		s.logger.Error("Session %s: Event queue full when sending response for ID %v", s.sessionID, response.ID)
		return fmt.Errorf("event queue full")
	}
}

// Close signals the SSE sending loop to terminate by closing the done channel.
func (s *sseSession) Close() error {
	s.logger.Info("Session %s: Close called", s.sessionID)
	// Signal the SSE writing loop to stop
	close(s.done)
	// Closing the underlying HTTP connection is handled by the http server/handler lifecycle
	return nil
}

func (s *sseSession) Initialize() {
	s.initialized.Store(true)
}

func (s *sseSession) Initialized() bool {
	return s.initialized.Load()
}

// SetNegotiatedVersion stores the protocol version agreed upon during initialization.
func (s *sseSession) SetNegotiatedVersion(version string) {
	// Assuming version is set only once during initialization, skipping mutex for now.
	s.negotiatedVersion = version
	s.logger.Debug("Session %s: Negotiated protocol version set to %s", s.sessionID, version)
}

// GetNegotiatedVersion returns the protocol version agreed upon during initialization.
func (s *sseSession) GetNegotiatedVersion() string {
	// Assuming version is set only once during initialization, skipping mutex for now.
	return s.negotiatedVersion
}

// StoreClientCapabilities implements server.ClientSession
func (s *sseSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	// Add locking if needed, though likely set only once during init
	s.clientCapabilities = caps
	s.logger.Debug("Session %s stored client capabilities", s.sessionID)
}

// GetClientCapabilities implements server.ClientSession
func (s *sseSession) GetClientCapabilities() protocol.ClientCapabilities {
	// Add locking if needed
	return s.clientCapabilities
}

// --- SSEServer ---

// SSEServer implements the HTTP handlers for the hybrid SSE/HTTP POST transport.
type SSEServer struct {
	mcpServer       MCPServerLogic // Use the interface for testability
	sessions        sync.Map       // Map[string]*sseSession (sessionID -> session)
	logger          types.Logger
	contextFunc     SSEContextFunc // Use local definition
	basePath        string
	messageEndpoint string
	sseEndpoint     string
}

// SSEServerOptions configure the SSEServer.
type SSEServerOptions struct {
	Logger          types.Logger
	ContextFunc     SSEContextFunc // Use local definition
	BasePath        string
	MessageEndpoint string
	SSEEndpoint     string
}

// NewSSEServer creates a new HTTP server providing MCP over SSE+HTTP.
// It takes an MCPServerLogic interface instead of a concrete *server.Server.
func NewSSEServer(mcpServer MCPServerLogic, opts SSEServerOptions) *SSEServer {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	basePath := opts.BasePath
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimSuffix(basePath, "/")

	sseEndpoint := opts.SSEEndpoint
	if sseEndpoint == "" {
		sseEndpoint = "/sse"
	}
	if !strings.HasPrefix(sseEndpoint, "/") {
		sseEndpoint = "/" + sseEndpoint
	}

	messageEndpoint := opts.MessageEndpoint
	if messageEndpoint == "" {
		messageEndpoint = "/message"
	}
	if !strings.HasPrefix(messageEndpoint, "/") {
		messageEndpoint = "/" + messageEndpoint
	}

	s := &SSEServer{
		mcpServer:       mcpServer,
		logger:          logger,
		contextFunc:     opts.ContextFunc,
		basePath:        basePath,
		sseEndpoint:     sseEndpoint,
		messageEndpoint: messageEndpoint,
	}

	logger.Info("SSE Server created. SSE Endpoint: %s%s, Message Endpoint: %s%s", basePath, s.sseEndpoint, basePath, messageEndpoint)
	return s
}

// ServeHTTP implements http.Handler, routing to SSE or Message handlers.
func (s *SSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	ssePath := s.basePath + s.sseEndpoint
	messagePath := s.basePath + s.messageEndpoint

	s.logger.Debug("ServeHTTP: Received request for %s", path)

	// Add CORS headers globally for simplicity here
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-Id")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if path == ssePath {
		s.HandleSSE(w, r)
	} else if path == messagePath {
		s.HandleMessage(w, r)
	} else {
		s.logger.Warn("ServeHTTP: Path not found: %s", path)
		http.NotFound(w, r)
	}
}

// HandleSSE handles the persistent SSE connection for server-to-client messages.
func (s *SSEServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// CORS already set in ServeHTTP

	// Create and register session
	session := newSSESession(w, flusher, s.logger)
	s.sessions.Store(session.SessionID(), session)
	defer s.sessions.Delete(session.SessionID()) // Ensure cleanup on exit

	if err := s.mcpServer.RegisterSession(session); err != nil {
		s.logger.Error("Failed to register session %s with MCP server: %v", session.SessionID(), err)
		http.Error(w, "Session registration failed", http.StatusInternalServerError)
		return
	}
	defer s.mcpServer.UnregisterSession(session.SessionID()) // Ensure unregistration

	s.logger.Info("SSE connection established for session %s from %s", session.SessionID(), r.RemoteAddr)

	// Send the initial endpoint event with session ID
	messageEndpointURL := s.getMessageEndpointURL(session.SessionID())
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageEndpointURL)
	flusher.Flush()
	s.logger.Info("Sent endpoint event to session %s", session.SessionID())

	// Main event loop: read from queue and write to client
	ctx := r.Context() // Get request context for cancellation
	for {
		select {
		case eventString := <-session.eventQueue:
			_, err := fmt.Fprint(w, eventString)
			if err != nil {
				s.logger.Error("Session %s: Error writing event to client: %v", session.SessionID(), err)
				// Assume connection is broken, trigger cleanup by returning
				return
			}
			flusher.Flush() // Flush after each event
			s.logger.Debug("Session %s: Wrote event to client", session.SessionID())
		case <-session.done: // Closed by session.Close()
			s.logger.Info("Session %s: Done channel closed, closing SSE connection.", session.SessionID())
			return
		case <-ctx.Done(): // Closed by client disconnecting or server shutdown
			s.logger.Info("Session %s: Request context done (%v), closing SSE connection.", session.SessionID(), ctx.Err())
			return
		}
	}
}

// HandleMessage processes incoming JSON-RPC messages via HTTP POST.
func (s *SSEServer) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSONRPCError(w, nil, protocol.ErrorCodeInvalidRequest, "Method not allowed, use POST")
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-Id")
		if sessionID == "" {
			s.writeJSONRPCError(w, nil, protocol.ErrorCodeInvalidParams, "Missing sessionId query parameter or X-Session-Id header")
			return
		}
	}
	// Check if session exists, but we don't need the session object directly here
	_, ok := s.sessions.Load(sessionID)
	if !ok {
		s.writeJSONRPCError(w, nil, protocol.ErrorCodeInvalidParams, fmt.Sprintf("Invalid or expired session ID: %s", sessionID))
		return
	}
	// We don't need the session object itself here, just the ID for HandleMessage

	ctx := r.Context()
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx, r)
	}

	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		s.writeJSONRPCError(w, nil, protocol.ErrorCodeParseError, fmt.Sprintf("Parse error: %v", err))
		return
	}

	// Process message through MCPServer's HandleMessage
	responses := s.mcpServer.HandleMessage(ctx, sessionID, rawMessage) // Returns []*protocol.JSONRPCResponse

	// Send response(s) back via HTTP POST response body
	if responses != nil && len(responses) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Encode the entire slice (will be a single object if only one response)
		if err := json.NewEncoder(w).Encode(responses); err != nil {
			s.logger.Error("Session %s: Failed to write HTTP response(s): %v", sessionID, err)
		} else {
			// Explicitly flush the response to ensure it's sent immediately over the network.
			// This is crucial because the client is waiting for this HTTP response.
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
				s.logger.Debug("Session %s: Flushed HTTP response", sessionID)
			}
		}
	} else {
		// For notifications or empty batches resulting in no response, send 204 No Content
		w.WriteHeader(http.StatusNoContent)
	}
}

// writeJSONRPCError writes a JSON-RPC error response to the http.ResponseWriter.
func (s *SSEServer) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := protocol.JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &protocol.ErrorPayload{Code: code, Message: message}}
	w.Header().Set("Content-Type", "application/json")
	httpStatus := http.StatusBadRequest
	if code == protocol.ErrorCodeParseError || code == protocol.ErrorCodeInvalidRequest {
		httpStatus = http.StatusBadRequest
	} else if code == protocol.ErrorCodeMethodNotFound {
		httpStatus = http.StatusNotFound
	} else if code == protocol.ErrorCodeInternalError {
		httpStatus = http.StatusInternalServerError
	}
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to write JSON-RPC error response: %v", err)
	}
}

// getMessageEndpointURL constructs the relative path for the message endpoint.
func (s *SSEServer) getMessageEndpointURL(sessionID string) string {
	// Return relative path suitable for client use with base URL
	return fmt.Sprintf("%s%s?sessionId=%s", s.basePath, s.messageEndpoint, sessionID)
}

// --- Default Logger ---
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

// SSEContextFunc is a function type used by the SSEServer to allow
// customization of the context passed to the core MCPServer's HandleMessage method,
// based on the incoming HTTP request for client->server messages.
// This allows injecting values from HTTP headers (like auth tokens) into the context.
// Defined locally to avoid server import.
type SSEContextFunc func(ctx context.Context, r *http.Request) context.Context
