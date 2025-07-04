// Package http provides a Streamable HTTP implementation of the MCP transport.
//
// This package implements the Transport interface using Streamable HTTP,
// following the MCP 2025-03-26 specification. It supports streaming HTTP responses,
// session management, OAuth authentication, and proper content negotiation.
package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// Option is a function that configures a Transport
type Option func(*Transport)

// WithPathPrefix returns an option that sets the path prefix for all endpoints
func WithPathPrefix(prefix string) Option {
	return func(t *Transport) {
		t.SetPathPrefix(prefix)
	}
}

// WithMCPEndpoint returns an option that sets the MCP endpoint path
func WithMCPEndpoint(path string) Option {
	return func(t *Transport) {
		t.SetMCPEndpoint(path)
	}
}

// WithHTTPClient returns an option that sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(t *Transport) {
		t.client = client
	}
}

// WithHeaders returns an option that sets additional HTTP headers
func WithHeaders(headers map[string]string) Option {
	return func(t *Transport) {
		t.headers = headers
	}
}

// WithTimeout returns an option that sets the HTTP timeout
func WithTimeout(timeout time.Duration) Option {
	return func(t *Transport) {
		if t.client != nil {
			t.client.Timeout = timeout
		}
	}
}

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 10 * time.Second

// DefaultMCPEndpoint is the default MCP endpoint path
const DefaultMCPEndpoint = "/mcp"

// Transport implements the transport.Transport interface for Streamable HTTP
type Transport struct {
	transport.BaseTransport
	addr     string
	server   *http.Server
	isClient bool

	// For server mode
	pathPrefix  string // Optional prefix for endpoint paths (e.g., "/api")
	mcpEndpoint string // MCP endpoint path

	// Session management (2025-03-26/2025-06-18)
	sessions       map[string]*SessionInfo // Map session ID to session info
	sessionsMu     sync.Mutex
	enableSessions bool // Whether to use session management

	// For client mode
	url       string
	client    *http.Client
	headers   map[string]string
	sessionID atomic.Pointer[string] // Current session ID
}

// SessionInfo holds information about an active session
type SessionInfo struct {
	ID        string
	CreatedAt time.Time
	LastSeen  time.Time
	ClientID  string
}

// HTTPHeaderFunc is a function type for generating dynamic headers
type HTTPHeaderFunc func() map[string]string

// NewTransport creates a new Streamable HTTP transport
func NewTransport(addr string, options ...Option) *Transport {
	t := &Transport{
		addr:           addr,
		client:         &http.Client{Timeout: 30 * time.Second},
		headers:        make(map[string]string),
		sessions:       make(map[string]*SessionInfo),
		pathPrefix:     "", // Empty by default
		mcpEndpoint:    DefaultMCPEndpoint,
		enableSessions: true, // Enable sessions by default for 2025-03-26/2025-06-18
	}

	// Apply options
	for _, opt := range options {
		opt(t)
	}

	return t
}

// SetPathPrefix sets a prefix for all endpoint paths
func (t *Transport) SetPathPrefix(prefix string) *Transport {
	if prefix != "" {
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		// Remove trailing slash to avoid double slashes
		prefix = strings.TrimSuffix(prefix, "/")
	}
	t.pathPrefix = prefix
	return t
}

// SetMCPEndpoint sets the path for the MCP endpoint
func (t *Transport) SetMCPEndpoint(path string) *Transport {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	t.mcpEndpoint = path
	return t
}

// GetFullMCPEndpoint returns the complete path for the MCP endpoint
func (t *Transport) GetFullMCPEndpoint() string {
	if t.pathPrefix == "" {
		return t.mcpEndpoint
	}
	return t.pathPrefix + t.mcpEndpoint
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if t.isClient {
		return t.startClient()
	}
	return t.startServer()
}

// startServer starts the HTTP server
func (t *Transport) startServer() error {
	mux := http.NewServeMux()

	// Register the MCP endpoint
	mux.HandleFunc(t.GetFullMCPEndpoint(), t.handleMCPRequest)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	// Start the server in a goroutine
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.GetLogger().Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// startClient starts the HTTP client
func (t *Transport) startClient() error {
	// Parse the server URL
	parsedURL, err := url.Parse(t.addr)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Construct the MCP endpoint URL
	t.url = parsedURL.String()
	if !strings.HasSuffix(t.url, "/") {
		t.url += "/"
	}
	t.url = strings.TrimSuffix(t.url, "/") + t.GetFullMCPEndpoint()

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	if t.isClient {
		// Terminate session if active
		if sessionID := t.sessionID.Load(); sessionID != nil {
			t.terminateSession(*sessionID)
		}
		return nil
	}

	if t.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer cancel()
		return t.server.Shutdown(ctx)
	}
	return nil
}

// Send sends a JSON-RPC message
func (t *Transport) Send(message []byte) error {
	if t.isClient {
		// Client mode: Send POST request to server
		return t.sendClientRequest(message)
	} else {
		// Server mode: Send message to clients via SSE streams
		return t.sendServerMessage(message)
	}
}

// sendClientRequest sends a POST request from client to server
func (t *Transport) sendClientRequest(message []byte) error {
	// Create POST request
	req, err := http.NewRequest("POST", t.url, bytes.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add session ID if available
	if sessionID := t.sessionID.Load(); sessionID != nil {
		req.Header.Set("MCP-Session-ID", *sessionID)
	}

	// Add custom headers
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// Make the request
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Check for session ID in response
	if sessionID := resp.Header.Get("MCP-Session-ID"); sessionID != "" {
		t.sessionID.Store(&sessionID)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST request returned status code %d", resp.StatusCode)
	}

	return nil
}

// sendServerMessage sends a message from server to clients via SSE streams
func (t *Transport) sendServerMessage(message []byte) error {
	// For server mode, we need to send the message via SSE streams
	// This would typically involve maintaining active SSE connections
	// and sending the message as SSE events to connected clients

	// For now, we'll store the message to be sent when clients connect
	// In a full implementation, this would maintain active SSE connections
	t.sessionsMu.Lock()
	defer t.sessionsMu.Unlock()

	// Send to all active sessions (simplified implementation)
	// In production, this would send via established SSE streams
	for sessionID, session := range t.sessions {
		// Check if session is still active (within last 5 minutes)
		if time.Since(session.LastSeen) > 5*time.Minute {
			delete(t.sessions, sessionID)
			continue
		}

		// In a full implementation, this would write to the SSE stream
		// associated with this session. For now, we'll log it.
		t.GetLogger().Info("Would send SSE message to session",
			"sessionID", sessionID,
			"messageSize", len(message))
	}

	// TODO: Implement actual SSE message sending to active streams
	// This requires maintaining a map of active SSE connections

	return nil
}

// Receive receives a message (not applicable for HTTP client)
func (t *Transport) Receive() ([]byte, error) {
	return nil, errors.New("receive not supported for HTTP transport - use request/response pattern")
}

// handleMCPRequest handles incoming MCP requests
func (t *Transport) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		t.handleClientMessage(w, r)
	case http.MethodGet:
		t.handleSSEStream(w, r)
	case http.MethodDelete:
		t.handleSessionTermination(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClientMessage handles POST requests from clients
func (t *Transport) handleClientMessage(w http.ResponseWriter, r *http.Request) {
	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Handle session management
	sessionID := r.Header.Get("MCP-Session-ID")
	if t.enableSessions {
		if sessionID == "" {
			// Create new session
			sessionID = t.generateSessionID()
			w.Header().Set("MCP-Session-ID", sessionID)

			t.sessionsMu.Lock()
			t.sessions[sessionID] = &SessionInfo{
				ID:        sessionID,
				CreatedAt: time.Now(),
				LastSeen:  time.Now(),
				ClientID:  r.RemoteAddr,
			}
			t.sessionsMu.Unlock()
		} else {
			// Update existing session
			t.sessionsMu.Lock()
			if session, exists := t.sessions[sessionID]; exists {
				session.LastSeen = time.Now()
				w.Header().Set("MCP-Session-ID", sessionID)
			} else {
				// Session not found, create new one
				sessionID = t.generateSessionID()
				w.Header().Set("MCP-Session-ID", sessionID)
				t.sessions[sessionID] = &SessionInfo{
					ID:        sessionID,
					CreatedAt: time.Now(),
					LastSeen:  time.Now(),
					ClientID:  r.RemoteAddr,
				}
			}
			t.sessionsMu.Unlock()
		}
	}

	// Handle the message
	response, err := t.HandleMessage(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Message handling failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Send JSON response
	if _, err := w.Write(response); err != nil {
		t.GetLogger().Error("Failed to write response", "error", err)
	}
}

// handleSSEStream handles GET requests for SSE streams
func (t *Transport) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Check Accept header for text/event-stream
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "text/event-stream") {
		http.Error(w, "text/event-stream not accepted", http.StatusNotAcceptable)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle session management
	sessionID := r.Header.Get("MCP-Session-ID")
	if t.enableSessions && sessionID != "" {
		t.sessionsMu.Lock()
		if session, exists := t.sessions[sessionID]; exists {
			session.LastSeen = time.Now()
			w.Header().Set("MCP-Session-ID", sessionID)
		} else {
			// Session not found
			http.Error(w, "Session not found", http.StatusNotFound)
			t.sessionsMu.Unlock()
			return
		}
		t.sessionsMu.Unlock()
	}

	// Start SSE stream
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Keep the connection alive and handle server-initiated messages
	// For now, this is a placeholder - in a real implementation,
	// this would listen for messages to send to the client
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// This is where server-initiated messages would be sent
			// For the basic implementation, we just keep the connection open
			time.Sleep(1 * time.Second)

			// Send a heartbeat event to keep connection alive
			fmt.Fprintf(w, "data: {\"type\":\"heartbeat\",\"timestamp\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}

// handleSessionTermination handles DELETE requests for session termination
func (t *Transport) handleSessionTermination(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("MCP-Session-ID")
	if sessionID == "" {
		http.Error(w, "Missing MCP-Session-ID header", http.StatusBadRequest)
		return
	}

	t.sessionsMu.Lock()
	_, exists := t.sessions[sessionID]
	if exists {
		delete(t.sessions, sessionID)
	}
	t.sessionsMu.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"session_terminated"}`))
}

// terminateSession terminates a client session
func (t *Transport) terminateSession(sessionID string) error {
	req, err := http.NewRequest("DELETE", t.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	req.Header.Set("MCP-Session-ID", sessionID)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DELETE request returned status code %d", resp.StatusCode)
	}

	t.sessionID.Store(nil)
	return nil
}

// generateSessionID generates a random session ID
func (t *Transport) generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GetAddr returns the transport's address
func (t *Transport) GetAddr() string {
	return t.addr
}

// SetClientMode sets the transport to client mode
func (t *Transport) SetClientMode(isClient bool) *Transport {
	t.isClient = isClient
	return t
}

// SetMessageHandler sets the message handler function (already available from BaseTransport)
// This method is inherited from BaseTransport, so no need to redefine

// GetFullAPIPath returns the complete path for the MCP endpoint (compatibility alias)
func (t *Transport) GetFullAPIPath() string {
	return t.GetFullMCPEndpoint()
}

// SetAPIPath sets the path for the MCP endpoint (compatibility alias)
func (t *Transport) SetAPIPath(path string) *Transport {
	return t.SetMCPEndpoint(path)
}

// NewServerTransport creates a new Streamable HTTP transport configured for server mode
func NewServerTransport(addr string, options ...Option) *Transport {
	t := NewTransport(addr, options...)
	t.isClient = false
	return t
}

// NewClientTransport creates a new Streamable HTTP transport configured for client mode
func NewClientTransport(url string, options ...Option) *Transport {
	t := NewTransport(url, options...)
	t.isClient = true
	t.url = url
	return t
}
