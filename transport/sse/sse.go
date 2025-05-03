// Package sse provides MCP server implementation over Server-Sent Events (SSE)
// using a hybrid approach (SSE for server->client, HTTP POST for client->server).
package sse

import (
	"context" // Needed for MCPServerLogic interface
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	// "log" // Remove unused import
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/auth" // Added for ContextWithToken
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types" // For Logger and ClientSession interface
)

// MCPServerLogic defines the interface SSEServer needs from the core server logic,
// using the ClientSession interface defined in the types package.
type MCPServerLogic interface {
	HandleMessage(ctx context.Context, session types.ClientSession, rawMessage json.RawMessage) []*protocol.JSONRPCResponse
	RegisterSession(session types.ClientSession) error // Use types.ClientSession
	UnregisterSession(sessionID string)
}

// sseSession represents an active SSE connection and implements the types.ClientSession interface.
type sseSession struct {
	writer             http.ResponseWriter
	flusher            http.Flusher
	done               chan struct{} // Closed when the connection is done
	closeOnce          sync.Once     // Ensures done channel is closed only once
	closed             atomic.Bool   // Tracks if Close has been called
	eventQueue         chan string   // Channel for queuing formatted SSE event strings
	sessionID          string        // Our internal unique session ID
	initialized        atomic.Bool
	logger             types.Logger
	negotiatedVersion  string                      // Stores the protocol version agreed upon
	clientCapabilities protocol.ClientCapabilities // Added
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
		logger:     logger,
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
	eventString := fmt.Sprintf("event: message\ndata: %s\n\n", string(eventData))
	select {
	case s.eventQueue <- eventString:
		s.logger.Debug("Session %s: Queued notification %s", s.sessionID, notification.Method)
		return nil
	case <-s.done:
		s.logger.Warn("Session %s: Attempted to send notification %s but session is done.", s.sessionID, notification.Method)
		return fmt.Errorf("session closed")
	default:
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
	eventString := fmt.Sprintf("event: message\ndata: %s\n\n", string(eventData))
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
	if s.closed.Load() {
		return nil
	}
	s.closeOnce.Do(func() {
		s.logger.Info("Session %s: Closing done channel.", s.sessionID)
		s.closed.Store(true)
		close(s.done)
	})
	return nil
}

func (s *sseSession) Initialize() {
	s.initialized.Store(true)
}

func (s *sseSession) Initialized() bool {
	return s.initialized.Load()
}

func (s *sseSession) SetNegotiatedVersion(version string) {
	s.negotiatedVersion = version
	s.logger.Debug("Session %s: Negotiated protocol version set to %s", s.sessionID, version)
}

func (s *sseSession) GetNegotiatedVersion() string {
	return s.negotiatedVersion
}

func (s *sseSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	s.clientCapabilities = caps
	s.logger.Debug("Session %s stored client capabilities", s.sessionID)
}

func (s *sseSession) GetClientCapabilities() protocol.ClientCapabilities {
	return s.clientCapabilities
}

// SendRequest returns an error because sending requests from server-to-client
// is not supported by the SSE transport implementation.
func (s *sseSession) SendRequest(request protocol.JSONRPCRequest) error {
	s.logger.Error("SendRequest called on sseSession, which is not supported.")
	return fmt.Errorf("sending requests from server to client is not supported over SSE transport")
}

// GetWriter returns nil for SSE sessions as direct writing to the underlying
// connection bypasses SSE formatting and is not the intended use case.
// The SendNotification/SendResponse methods should be used.
func (s *sseSession) GetWriter() io.Writer {
	s.logger.Warn("Session %s: GetWriter() called on sseSession, returning nil (direct write not supported).", s.sessionID)
	return nil
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

// SSEContextFunc is defined locally to avoid server import.
type SSEContextFunc func(ctx context.Context, r *http.Request) context.Context

// SSEServerOptions configure the SSEServer.
type SSEServerOptions struct {
	Logger          types.Logger
	ContextFunc     SSEContextFunc // Use local definition
	BasePath        string
	MessageEndpoint string
	SSEEndpoint     string
}

// NewSSEServer creates a new HTTP server providing MCP over SSE+HTTP.
func NewSSEServer(mcpServer MCPServerLogic, opts SSEServerOptions) *SSEServer {
	// Logger is expected to be provided or handled by the caller now
	logger := opts.Logger
	// if logger == nil {
	// 	logger = &defaultLogger{} // Removed local default logger instantiation
	// }

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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	session := newSSESession(w, flusher, s.logger)
	s.sessions.Store(session.SessionID(), session)
	defer func() {
		s.sessions.Delete(session.SessionID())
		s.logger.Info("Session %s: Deleted from SSE transport map.", session.SessionID())
	}()

	if err := s.mcpServer.RegisterSession(session); err != nil {
		s.logger.Error("Failed to register session %s with MCP server: %v", session.SessionID(), err)
		http.Error(w, "Session registration failed", http.StatusInternalServerError)
		return
	}
	defer func() {
		s.mcpServer.UnregisterSession(session.SessionID())
		s.logger.Info("Session %s: Unregistered from MCP core server.", session.SessionID())
	}()

	s.logger.Info("SSE connection established for session %s from %s", session.SessionID(), r.RemoteAddr)

	messageEndpointURL := s.getMessageEndpointURL(session.SessionID())
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageEndpointURL)
	flusher.Flush()
	s.logger.Info("Sent endpoint event to session %s", session.SessionID())

	ctx := r.Context()
	for {
		select {
		case eventString := <-session.eventQueue:
			s.logger.Debug("Session %s: Dequeued event, attempting write...", session.SessionID())
			_, err := fmt.Fprint(w, eventString)
			if err != nil {
				s.logger.Error("Session %s: Error during fmt.Fprint to client: %v. Closing SSE stream.", session.SessionID(), err)
				return
			}
			s.logger.Debug("Session %s: Write successful, attempting flush...", session.SessionID())
			flusher.Flush()
			s.logger.Debug("Session %s: Flush completed.", session.SessionID())
		case <-session.done:
			s.logger.Info("Session %s: Done channel closed, closing SSE connection.", session.SessionID())
			return
		case <-ctx.Done():
			s.logger.Info("Session %s: Request context done (%v), closing SSE connection.", session.SessionID(), ctx.Err())
			return
		}
	}
}

// HandleMessage processes incoming JSON-RPC messages via HTTP POST.
func (s *SSEServer) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSONRPCError(w, nil, protocol.CodeInvalidRequest, "Method not allowed, use POST")
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-Id")
		if sessionID == "" {
			s.writeJSONRPCError(w, nil, protocol.CodeInvalidParams, "Missing sessionId query parameter or X-Session-Id header")
			return
		}
	}
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		s.writeJSONRPCError(w, nil, protocol.CodeInvalidParams, fmt.Sprintf("Invalid or expired session ID: %s", sessionID))
		return
	}
	session, ok := sessionValue.(types.ClientSession) // Assert against interface
	if !ok {
		s.logger.Error("Session %s stored value is not a types.ClientSession (%T).", sessionID, sessionValue)
		s.writeJSONRPCError(w, nil, protocol.CodeInternalError, "Internal server error: invalid session type")
		return
	}

	ctx := r.Context()
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx, r) // Apply original context func first
	}

	// --- Token Extraction from Header ---
	tokenHeader := r.Header.Get("Authorization") // Standard header
	if tokenHeader != "" {
		// Add token to context for the auth hook
		ctx = auth.ContextWithToken(ctx, tokenHeader)
		s.logger.Debug("Session %s: Found Authorization header, added token to context.", sessionID)
	} else {
		s.logger.Debug("Session %s: No Authorization header found.", sessionID)
	}
	// --- End Token Extraction ---

	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		s.writeJSONRPCError(w, nil, protocol.CodeParseError, fmt.Sprintf("Parse error: %v", err))
		return
	}

	responses := s.mcpServer.HandleMessage(ctx, session, rawMessage)

	if responses != nil && len(responses) > 0 {
		for _, response := range responses {
			if response == nil {
				continue
			}
			resp := *response // Create copy
			if err := session.SendResponse(resp); err != nil {
				s.logger.Error("Session %s: Failed to queue response ID %v for SSE: %v", sessionID, resp.ID, err)
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSONRPCError writes a JSON-RPC error response.
func (s *SSEServer) writeJSONRPCError(w http.ResponseWriter, id interface{}, code protocol.ErrorCode, message string) {
	response := protocol.JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &protocol.ErrorPayload{Code: code, Message: message}}
	w.Header().Set("Content-Type", "application/json")
	httpStatus := http.StatusBadRequest // Default
	if code == protocol.CodeInternalError {
		httpStatus = http.StatusInternalServerError
	}
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to write JSON-RPC error response: %v", err)
	}
}

// getMessageEndpointURL constructs the relative path.
func (s *SSEServer) getMessageEndpointURL(sessionID string) string {
	return fmt.Sprintf("%s%s?sessionId=%s", s.basePath, s.messageEndpoint, sessionID)
}

// Default logger definition removed, should be handled by caller or logx
