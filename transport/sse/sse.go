// Package sse provides a Server-Sent Events implementation of the MCP transport.
//
// This package implements the Transport interface using Server-Sent Events (SSE),
// suitable for applications requiring server-to-client real-time updates.
//
// The implementation follows the MCP 2025-03-26 "Streamable HTTP" specification,
// using a single endpoint that handles both GET requests (for SSE streams) and
// POST requests (for client messages).
package sse

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// Option is a function that configures a Transport
type Option func(*Transport)

// Options provides a fluent API for configuring SSE transport options
type Options struct{}

// SSE provides access to SSE transport configuration options
var SSE = Options{}

// WithPathPrefix returns an option that sets the path prefix for all endpoints
func (Options) WithPathPrefix(prefix string) Option {
	return func(t *Transport) {
		t.SetPathPrefix(prefix)
	}
}

// WithMCPEndpoint returns an option that sets the unified MCP endpoint path
func (Options) WithMCPEndpoint(path string) Option {
	return func(t *Transport) {
		t.SetMCPEndpoint(path)
	}
}

// Deprecated: WithEventsPath is deprecated. Use WithMCPEndpoint instead.
// This method is kept for backward compatibility.
func (Options) WithEventsPath(path string) Option {
	return func(t *Transport) {
		t.SetMCPEndpoint(path)
	}
}

// Deprecated: WithMessagePath is deprecated. Use WithMCPEndpoint instead.
// This method is kept for backward compatibility.
func (Options) WithMessagePath(path string) Option {
	return func(t *Transport) {
		// This is now ignored since we use a single endpoint
	}
}

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 10 * time.Second

// DefaultMCPEndpoint is the default unified MCP endpoint path
const DefaultMCPEndpoint = "/mcp"

// Deprecated: Use DefaultMCPEndpoint instead
const DefaultEventsPath = "/sse"

// Deprecated: Use DefaultMCPEndpoint instead
const DefaultMessagePath = "/message"

// Transport implements the transport.Transport interface for SSE
type Transport struct {
	transport.BaseTransport
	addr     string
	server   *http.Server
	isClient bool

	// For server mode
	clients     map[string]chan []byte // Map client ID to message channel
	clientsMu   sync.Mutex
	pathPrefix  string // Optional prefix for endpoint paths (e.g., "/api")
	mcpEndpoint string // Unified MCP endpoint path
	eventsPath  string // Legacy events path for 2024-11-05 compatibility

	// Session management (2025-03-26/2025-06-18/draft)
	sessions       map[string]*SessionInfo // Map session ID to session info
	sessionsMu     sync.Mutex
	nextEventID    int64 // For SSE event IDs
	enableSessions bool  // Whether to use session management

	// For client mode
	url       string
	client    *http.Client
	readCh    chan []byte
	errCh     chan error
	doneCh    chan struct{}
	connected atomic.Bool
	mcpURL    atomic.Pointer[string] // Complete URL for the MCP endpoint
}

// SessionInfo holds information about an active session
type SessionInfo struct {
	ID        string
	CreatedAt time.Time
	LastSeen  time.Time
	ClientID  string
}

// NewTransport creates a new SSE transport
func NewTransport(addr string) *Transport {
	isClient := strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")

	t := &Transport{
		addr:       addr,
		isClient:   isClient,
		pathPrefix: "", // Empty by default
	}

	if isClient {
		t.url = addr
		t.client = &http.Client{}
		t.readCh = make(chan []byte, 100)
		t.errCh = make(chan error, 1)
		t.doneCh = make(chan struct{})
	} else {
		t.clients = make(map[string]chan []byte)
		t.sessions = make(map[string]*SessionInfo)
		t.enableSessions = true // Enable session management by default for 2025-03-26/2025-06-18/draft
		// Set default unified endpoint
		t.mcpEndpoint = DefaultMCPEndpoint
		// Set default legacy events path for 2024-11-05 compatibility
		t.eventsPath = DefaultEventsPath
	}

	return t
}

// SetPathPrefix sets a prefix for all endpoint paths
// For example, SetPathPrefix("/api") will result in endpoints like "/api/mcp"
func (t *Transport) SetPathPrefix(prefix string) *Transport {
	if !t.isClient {
		// Ensure the prefix starts with a slash if not empty
		if prefix != "" && !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		t.pathPrefix = prefix
	}
	return t
}

// SetMCPEndpoint sets the unified MCP endpoint path
func (t *Transport) SetMCPEndpoint(path string) *Transport {
	if !t.isClient {
		t.mcpEndpoint = path
	}
	return t
}

// Deprecated: SetEventPath is deprecated. Use SetMCPEndpoint instead.
func (t *Transport) SetEventPath(path string) *Transport {
	if !t.isClient {
		t.eventsPath = path
	}
	return t
}

// Deprecated: SetMessagePath is deprecated. Use SetMCPEndpoint instead.
func (t *Transport) SetMessagePath(path string) *Transport {
	// This is now ignored since we use a single endpoint
	return t
}

// GetFullMCPPath returns the complete path for the unified MCP endpoint
func (t *Transport) GetFullMCPPath() string {
	if t.pathPrefix == "" {
		return t.mcpEndpoint
	}
	return t.pathPrefix + t.mcpEndpoint
}

// Deprecated: GetFullEventsPath is deprecated. Use GetFullMCPPath instead.
func (t *Transport) GetFullEventsPath() string {
	if t.pathPrefix == "" {
		return t.eventsPath
	}
	return t.pathPrefix + t.eventsPath
}

// Deprecated: GetFullMessagePath is deprecated. Use GetFullMCPPath instead.
func (t *Transport) GetFullMessagePath() string {
	return t.GetFullMCPPath()
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	if t.isClient {
		// Client mode - nothing to initialize yet
		// We'll connect when Start is called
		return nil
	}

	// Server mode - nothing to initialize yet
	// We'll start the HTTP server when Start is called
	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if t.isClient {
		// Start the client connection
		go t.startClientConnection()
		return nil
	}

	// Start the server with unified MCP endpoint
	mux := http.NewServeMux()

	// Single unified MCP endpoint that handles both GET (SSE) and POST (messages)
	mux.HandleFunc(t.GetFullMCPPath(), t.handleMCPRequest)

	// For backward compatibility with 2024-11-05, also register the legacy SSE endpoint
	// This endpoint only handles GET requests for SSE connection with endpoint discovery
	mux.HandleFunc(t.GetFullEventsPath(), func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			t.handleLegacySSEConnection(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
			slog.Default().Error("SSE server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	if t.isClient {
		close(t.doneCh)
		t.connected.Store(false)
		return nil
	}

	// Server mode
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	// Notify all clients that we're shutting down
	t.clientsMu.Lock()
	for _, clientCh := range t.clients {
		close(clientCh)
	}
	t.clients = make(map[string]chan []byte)
	t.clientsMu.Unlock()

	// Shutdown the server
	return t.server.Shutdown(ctx)
}

// Send sends a message to the connected client(s)
func (t *Transport) Send(message []byte) error {
	if t.isClient {
		// In client mode, use the POST endpoint received from the server
		postEndpoint := t.mcpURL.Load()
		connected := t.connected.Load()

		if !connected || postEndpoint == nil || *postEndpoint == "" {
			return errors.New("not connected to server or missing POST endpoint")
		}

		// Send message to server via HTTP POST
		req, err := http.NewRequest("POST", *postEndpoint, bytes.NewReader(message))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := t.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		return nil
	}

	// Server mode - send to all connected SSE clients
	t.clientsMu.Lock()
	// Create a copy of the clients map to avoid holding the lock during channel operations
	clientChannels := make([]chan []byte, 0, len(t.clients))
	for _, clientCh := range t.clients {
		clientChannels = append(clientChannels, clientCh)
	}
	t.clientsMu.Unlock()

	// Send to all clients without holding the mutex
	for _, clientCh := range clientChannels {
		select {
		case clientCh <- message:
			// Message sent successfully
		default:
			// Client channel full, message dropped (could be a slow client)
			// In production, you might want to implement a more sophisticated strategy
		}
	}

	return nil
}

// Receive receives a message (client mode only)
func (t *Transport) Receive() ([]byte, error) {
	if !t.isClient {
		return nil, errors.New("receive is only supported in client mode")
	}

	connected := t.connected.Load()

	if !connected {
		return nil, errors.New("not connected to server")
	}

	select {
	case msg := <-t.readCh:
		return msg, nil
	case err := <-t.errCh:
		return nil, err
	case <-t.doneCh:
		return nil, errors.New("transport closed")
	}
}

// generateClientID creates a unique client ID
func (t *Transport) generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}

// generateSessionID creates a cryptographically secure session ID
func (t *Transport) generateSessionID() string {
	// Generate 16 random bytes (128 bits) and encode as hex
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// getNextEventID returns the next SSE event ID
func (t *Transport) getNextEventID() string {
	eventID := atomic.AddInt64(&t.nextEventID, 1)
	return fmt.Sprintf("%d", eventID)
}

// validateAcceptHeader validates the Accept header according to MCP spec
func (t *Transport) validateAcceptHeader(r *http.Request, expectedType string) bool {
	acceptHeader := r.Header.Get("Accept")
	if acceptHeader == "" {
		return false
	}

	// Split accept header by comma and check each type
	acceptTypes := strings.Split(acceptHeader, ",")
	for _, acceptType := range acceptTypes {
		acceptType = strings.TrimSpace(acceptType)
		// Handle media type with parameters (e.g., "text/event-stream; charset=utf-8")
		if strings.Contains(acceptType, ";") {
			acceptType = strings.TrimSpace(strings.Split(acceptType, ";")[0])
		}
		if acceptType == expectedType || acceptType == "*/*" {
			return true
		}
	}
	return false
}

// isNotificationRequest checks if a JSON-RPC request is a notification (no id field)
func (t *Transport) isNotificationRequest(body []byte) bool {
	var request map[string]interface{}
	if err := json.Unmarshal(body, &request); err != nil {
		return false
	}

	// If there's no "id" field, it's a notification
	_, hasID := request["id"]
	return !hasID
}

// handleMCPRequest handles incoming MCP requests using the unified endpoint pattern
// GET requests establish SSE streams, POST requests handle client messages
func (t *Transport) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Validate Accept header for GET requests (SSE) - required for SSE
		if !t.validateAcceptHeader(r, "text/event-stream") {
			http.Error(w, "Accept header must include text/event-stream", http.StatusBadRequest)
			return
		}
		// Handle SSE stream establishment
		t.handleSSEConnection(w, r)

	case http.MethodPost:
		// For POST requests, Accept header is recommended but not strictly required
		// if Content-Type is properly set (for backward compatibility)
		hasValidAccept := t.validateAcceptHeader(r, "application/json") || t.validateAcceptHeader(r, "text/event-stream")
		hasValidContentType := strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json")

		if !hasValidAccept && !hasValidContentType {
			http.Error(w, "Either Accept header must include application/json/text/event-stream or Content-Type must be application/json", http.StatusBadRequest)
			return
		}
		// Handle client message submission
		t.handleClientMessage(w, r)

	case http.MethodDelete:
		// Handle session termination (2025-03-26 spec)
		t.handleSessionTermination(w, r)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionTermination handles DELETE requests for explicit session termination
func (t *Transport) handleSessionTermination(w http.ResponseWriter, r *http.Request) {
	if !t.enableSessions {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id header required", http.StatusBadRequest)
		return
	}

	// Remove session
	t.sessionsMu.Lock()
	_, exists := t.sessions[sessionID]
	if exists {
		delete(t.sessions, sessionID)
	}
	t.sessionsMu.Unlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	t.GetLogger().Debug("Session terminated", "session_id", sessionID)
}

// handleSSEConnection handles GET requests for establishing SSE streams
func (t *Transport) handleSSEConnection(w http.ResponseWriter, r *http.Request) {
	t.GetLogger().Debug("New SSE connection", "remote_addr", r.RemoteAddr)

	// Handle session management for 2025-03-26/2025-06-18/draft
	var sessionID string
	if t.enableSessions && (t.GetProtocolVersion() == "2025-03-26" || t.GetProtocolVersion() == "2025-06-18" || t.GetProtocolVersion() == "draft") {
		sessionID = r.Header.Get("Mcp-Session-Id")
		if sessionID != "" {
			// Validate existing session
			t.sessionsMu.Lock()
			session, exists := t.sessions[sessionID]
			if !exists {
				t.sessionsMu.Unlock()
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			session.LastSeen = time.Now()
			t.sessionsMu.Unlock()
			t.GetLogger().Debug("Resuming session", "session_id", sessionID)
		}
	}

	// Validate Origin header for security
	origin := r.Header.Get("Origin")
	if origin != "" {
		// In a production environment, implement proper origin validation
		// For now, we'll accept any origin for development purposes
		w.Header().Set("Access-Control-Allow-Origin", origin)
		t.GetLogger().Debug("Origin header received", "origin", origin)
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Add session header if session management is enabled
	if sessionID != "" {
		w.Header().Set("Mcp-Session-Id", sessionID)
	}

	t.GetLogger().Debug("Set SSE headers")

	// Flush headers immediately to complete the HTTP response
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	t.GetLogger().Debug("Flushed SSE headers")

	// Generate a unique client ID
	clientID := t.generateClientID()
	t.GetLogger().Debug("Generated client ID", "client_id", clientID)

	// Create a channel for this client
	clientCh := make(chan []byte, 10)

	// Register the client
	t.clientsMu.Lock()
	t.clients[clientID] = clientCh
	t.clientsMu.Unlock()
	t.GetLogger().Debug("Registered client", "client_id", clientID)

	// Create the full MCP endpoint URL for this client (same endpoint for POST)
	mcpURL := fmt.Sprintf("http://%s%s", r.Host, t.GetFullMCPPath())
	t.GetLogger().Debug("MCP endpoint URL", "url", mcpURL)

	// Clean up when the client disconnects
	defer func() {
		t.GetLogger().Debug("Client disconnected", "client_id", clientID)
		t.clientsMu.Lock()
		// Only close the channel if the client is still in our map (not already cleaned up)
		if ch, exists := t.clients[clientID]; exists {
			delete(t.clients, clientID)
			close(ch)
		}
		t.clientsMu.Unlock()
	}()

	// For unified MCP endpoint (2025-03-26/2025-06-18/draft), we don't send endpoint events
	// The client already knows the endpoint from the URL they connected to
	// This is different from the legacy 2024-11-05 behavior

	// Check protocol version to determine behavior
	protocolVersion := t.GetProtocolVersion()
	if protocolVersion == "2024-11-05" {
		// For 2024-11-05, send endpoint discovery event
		eventID := t.getNextEventID()
		endpointEvent := fmt.Sprintf("id: %s\nevent: endpoint\ndata: %s\n\n", eventID, mcpURL)
		if _, err := fmt.Fprint(w, endpointEvent); err != nil {
			t.GetLogger().Debug("Failed to send endpoint event", "error", err)
			return
		}

		// Flush to ensure the event is sent immediately
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		t.GetLogger().Debug("Sent endpoint discovery event", "endpoint", mcpURL, "event_id", eventID)
	}
	// For draft, 2025-03-26, and 2025-06-18, we don't send endpoint events - unified endpoint pattern

	// Listen for messages and send them to the client
	for {
		select {
		case msg, ok := <-clientCh:
			if !ok {
				// Channel closed, client disconnected
				return
			}

			// Send message as SSE event with ID for resumability
			eventID := t.getNextEventID()
			event := fmt.Sprintf("id: %s\nevent: message\ndata: %s\n\n", eventID, string(msg))
			if _, err := fmt.Fprint(w, event); err != nil {
				t.GetLogger().Debug("Failed to send SSE message", "error", err, "event_id", eventID)
				return
			}

			// Flush to ensure the message is sent immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			t.GetLogger().Debug("Sent SSE message", "event_id", eventID)

		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// handleLegacySSEConnection handles GET requests for the legacy /sse endpoint (2024-11-05 compatibility)
// This endpoint always sends endpoint discovery events regardless of protocol version
func (t *Transport) handleLegacySSEConnection(w http.ResponseWriter, r *http.Request) {
	t.GetLogger().Debug("New legacy SSE connection", "remote_addr", r.RemoteAddr)

	// Validate Origin header for security
	origin := r.Header.Get("Origin")
	if origin != "" {
		// In a production environment, implement proper origin validation
		// For now, we'll accept any origin for development purposes
		w.Header().Set("Access-Control-Allow-Origin", origin)
		t.GetLogger().Debug("Origin header received", "origin", origin)
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	t.GetLogger().Debug("Set SSE headers")

	// Generate a unique client ID
	clientID := t.generateClientID()
	t.GetLogger().Debug("Generated client ID", "client_id", clientID)

	// Create a channel for this client
	clientCh := make(chan []byte, 10)

	// Register the client
	t.clientsMu.Lock()
	t.clients[clientID] = clientCh
	t.clientsMu.Unlock()
	t.GetLogger().Debug("Registered client", "client_id", clientID)

	// For legacy SSE endpoint, always send endpoint discovery event
	// The POST endpoint is the unified MCP endpoint
	mcpURL := fmt.Sprintf("http://%s%s", r.Host, t.GetFullMCPPath())
	t.GetLogger().Debug("MCP endpoint URL", "url", mcpURL)

	// Clean up when the client disconnects
	defer func() {
		t.GetLogger().Debug("Client disconnected", "client_id", clientID)
		t.clientsMu.Lock()
		// Only close the channel if the client is still in our map (not already cleaned up)
		if ch, exists := t.clients[clientID]; exists {
			delete(t.clients, clientID)
			close(ch)
		}
		t.clientsMu.Unlock()
	}()

	// Always send endpoint discovery event for legacy SSE endpoint
	eventID := t.getNextEventID()
	endpointEvent := fmt.Sprintf("id: %s\nevent: endpoint\ndata: %s\n\n", eventID, mcpURL)
	if _, err := fmt.Fprint(w, endpointEvent); err != nil {
		t.GetLogger().Debug("Failed to send endpoint event", "error", err)
		return
	}

	// Flush to ensure the event is sent immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	t.GetLogger().Debug("Sent endpoint discovery event", "endpoint", mcpURL, "event_id", eventID)

	// Listen for messages and send them to the client
	for {
		select {
		case msg, ok := <-clientCh:
			if !ok {
				// Channel closed, client disconnected
				return
			}

			// Send message as SSE event with ID
			eventID := t.getNextEventID()
			event := fmt.Sprintf("id: %s\nevent: message\ndata: %s\n\n", eventID, string(msg))
			if _, err := fmt.Fprint(w, event); err != nil {
				t.GetLogger().Debug("Failed to send SSE message", "error", err, "event_id", eventID)
				return
			}

			// Flush to ensure the message is sent immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// handleClientMessage handles POST requests for client message submission
func (t *Transport) handleClientMessage(w http.ResponseWriter, r *http.Request) {
	// Validate content type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Handle session management for 2025-03-26/2025-06-18/draft
	var sessionID string
	if t.enableSessions && (t.GetProtocolVersion() == "2025-03-26" || t.GetProtocolVersion() == "2025-06-18" || t.GetProtocolVersion() == "draft") {
		sessionID = r.Header.Get("Mcp-Session-Id")

		// For non-initialize requests, session ID might be required
		if sessionID != "" {
			// Validate existing session
			t.sessionsMu.Lock()
			session, exists := t.sessions[sessionID]
			if !exists {
				t.sessionsMu.Unlock()
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			session.LastSeen = time.Now()
			t.sessionsMu.Unlock()
		}
	}

	// Read message
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate JSON format before processing
	var jsonCheck interface{}
	if err := json.Unmarshal(body, &jsonCheck); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Check if this is a notification (no "id" field) - should return 202 Accepted
	if t.isNotificationRequest(body) {
		// For notifications, process and return appropriate status based on protocol version
		_, err := t.HandleMessage(body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error processing notification: %v", err), http.StatusInternalServerError)
			return
		}

		// Return version-specific status for notifications
		protocolVersion := t.GetProtocolVersion()
		if protocolVersion == "2024-11-05" {
			// 2024-11-05 spec: notifications return 200 OK
			w.WriteHeader(http.StatusOK)
		} else {
			// 2025-03-26/2025-06-18/draft spec: notifications return 202 Accepted
			w.WriteHeader(http.StatusAccepted)
		}
		return
	}

	// For unified MCP endpoint, POST requests should get direct responses
	// not go through the SSE broadcasting system which is for server-initiated messages

	// Process message directly and synchronously for POST requests
	response, err := t.HandleMessage(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error processing message: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle session creation for initialize responses
	if t.enableSessions && (t.GetProtocolVersion() == "2025-03-26" || t.GetProtocolVersion() == "2025-06-18" || t.GetProtocolVersion() == "draft") {
		// Check if this is an initialize response by looking at the request
		var request map[string]interface{}
		if json.Unmarshal(body, &request) == nil {
			if method, ok := request["method"].(string); ok && method == "initialize" {
				// Create new session for initialize request
				if sessionID == "" {
					sessionID = t.generateSessionID()
					clientID := t.generateClientID()

					session := &SessionInfo{
						ID:        sessionID,
						CreatedAt: time.Now(),
						LastSeen:  time.Now(),
						ClientID:  clientID,
					}

					t.sessionsMu.Lock()
					t.sessions[sessionID] = session
					t.sessionsMu.Unlock()

					// Add session header to response
					w.Header().Set("Mcp-Session-Id", sessionID)
					t.GetLogger().Debug("Created new session", "session_id", sessionID)
				}
			}
		}
	}

	// Send direct response to the HTTP client
	if response != nil {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(response); err != nil {
			// Log error but request is already processed
			t.GetLogger().Debug("Failed to write response", "error", err)
		}
	} else {
		// No response needed (e.g., notification), return 200 OK
		w.WriteHeader(http.StatusOK)
	}
}

// startClientConnection establishes and maintains the SSE connection
func (t *Transport) startClientConnection() {
	for {
		select {
		case <-t.doneCh:
			return
		default:
			// Attempt to connect or reconnect
			if err := t.connectToSSE(); err != nil {
				select {
				case t.errCh <- err:
					// Error sent
				default:
					// Error channel full, discard
				}

				// Wait before reconnecting
				select {
				case <-time.After(5 * time.Second):
					// Try again
				case <-t.doneCh:
					return
				}
			}
		}
	}
}

// connectToSSE establishes a connection to the SSE server using the unified MCP endpoint
func (t *Transport) connectToSSE() error {
	// Connect to the MCP endpoint directly (not the old /sse endpoint)
	mcpURL := t.url

	// For backward compatibility with 2024-11-05 servers:
	// Try the unified MCP endpoint first, fallback to old /sse endpoint if needed

	// Remove any existing path suffix to get the base URL
	if strings.HasSuffix(mcpURL, DefaultEventsPath) {
		mcpURL = strings.TrimSuffix(mcpURL, DefaultEventsPath)
	}
	if strings.HasSuffix(mcpURL, DefaultMCPEndpoint) {
		mcpURL = strings.TrimSuffix(mcpURL, DefaultMCPEndpoint)
	}

	// Ensure we have the MCP endpoint path
	if !strings.HasSuffix(mcpURL, "/") {
		mcpURL += "/"
	}
	mcpURL = strings.TrimSuffix(mcpURL, "/") + DefaultMCPEndpoint

	// Log connection attempt
	t.GetLogger().Debug("Connecting to MCP endpoint", "url", mcpURL)

	req, err := http.NewRequest("GET", mcpURL, nil)
	if err != nil {
		errMsg := fmt.Errorf("Failed to create MCP request: %w", err)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg.Error())
		}
		return errMsg
	}

	// Set headers for SSE request
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Context that can be canceled when Stop is called
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register cancel function to be called when doneCh is closed
	go func() {
		select {
		case <-t.doneCh:
			cancel()
		case <-ctx.Done():
			// Context already canceled
			return
		}
	}()

	req = req.WithContext(ctx)

	if debugHandler := t.GetDebugHandler(); debugHandler != nil {
		debugHandler("Sending MCP connection request...")
	}
	t.GetLogger().Debug("Sending MCP request")

	resp, err := t.client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("MCP request failed: %v", err)
		t.GetLogger().Debug("MCP request failed", "error", err)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg)
		}
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	// Handle different response types based on MCP specification
	if resp.StatusCode == http.StatusMethodNotAllowed {
		// Server doesn't support GET on MCP endpoint, try fallback to old /sse endpoint
		return t.connectToLegacySSE()
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("MCP request returned status code %d", resp.StatusCode)
		t.GetLogger().Debug("MCP request returned error status", "status_code", resp.StatusCode)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg)
		}
		return errors.New(errMsg)
	}

	// Check if server returned SSE stream
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return errors.New("server did not return SSE stream")
	}

	t.GetLogger().Debug("MCP SSE connection established, parsing events")

	// Store the MCP URL for POST requests
	t.mcpURL.Store(&mcpURL)
	t.connected.Store(true)
	t.GetLogger().Debug("MCP endpoint set", "endpoint", mcpURL)

	// Parse SSE events
	reader := bufio.NewReader(resp.Body)
	var buf bytes.Buffer
	var eventType string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				t.GetLogger().Debug("SSE connection closed (EOF)")
				break
			}
			t.GetLogger().Debug("Error reading SSE stream", "error", err)
			return err
		}

		line = bytes.TrimSpace(line)
		t.GetLogger().Debug("SSE line received", "line", string(line))

		// Skip comment lines
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}

		// Handle event type
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
			t.GetLogger().Debug("Event type", "type", eventType)
			continue
		}

		// Handle data lines
		if bytes.HasPrefix(line, []byte("data:")) {
			// Extract the data
			data := bytes.TrimPrefix(line, []byte("data:"))
			data = bytes.TrimSpace(data)
			buf.Write(data)
			t.GetLogger().Debug("Event data", "data", string(data))
		} else if len(line) == 0 && buf.Len() > 0 {
			// Empty line indicates end of event
			msg := buf.Bytes()
			t.GetLogger().Debug("Complete event received", "data", string(msg), "type", eventType)

			// Handle different event types
			if eventType == "endpoint" {
				// Legacy 2024-11-05 behavior: server sends endpoint URL
				// This should not happen with the new unified endpoint pattern
				t.GetLogger().Debug("Received legacy endpoint event", "endpoint", string(msg))
			} else if eventType == "message" || eventType == "" {
				// Regular message, process it
				response, err := t.HandleMessage(msg)
				if err != nil {
					t.GetLogger().Debug("Error handling message", "error", err)
					// Log error but continue processing
					buf.Reset()
					eventType = ""
					continue
				}

				if response != nil {
					t.GetLogger().Debug("Sending response", "response", string(response))
					select {
					case t.readCh <- response:
						// Message sent
					default:
						// Channel full, discard oldest message
						<-t.readCh
						t.readCh <- response
					}
				}
			}

			// Reset buffer and event type for next event
			buf.Reset()
			eventType = ""
		}
	}

	t.GetLogger().Debug("SSE connection closed")
	return errors.New("SSE connection closed")
}

// connectToLegacySSE provides backward compatibility with 2024-11-05 servers
func (t *Transport) connectToLegacySSE() error {
	// Connect to the legacy events endpoint for backward compatibility
	eventsURL := t.url

	// Only append the SSE endpoint if the URL doesn't already end with it
	if !strings.HasSuffix(eventsURL, DefaultEventsPath) &&
		!strings.Contains(eventsURL, DefaultEventsPath+"?") {
		// Append the default events path if not already present
		if !strings.HasSuffix(eventsURL, "/") {
			eventsURL += "/"
		}
		eventsURL = strings.TrimSuffix(eventsURL, "/") + DefaultEventsPath
	}

	// Log connection attempt
	t.GetLogger().Debug("Connecting to legacy SSE server", "url", eventsURL)

	req, err := http.NewRequest("GET", eventsURL, nil)
	if err != nil {
		errMsg := fmt.Errorf("Failed to create legacy SSE request: %w", err)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg.Error())
		}
		return errMsg
	}

	// Set headers for SSE request
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Context that can be canceled when Stop is called
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register cancel function to be called when doneCh is closed
	go func() {
		select {
		case <-t.doneCh:
			cancel()
		case <-ctx.Done():
			// Context already canceled
			return
		}
	}()

	req = req.WithContext(ctx)

	if debugHandler := t.GetDebugHandler(); debugHandler != nil {
		debugHandler("Sending legacy SSE connection request...")
	}
	t.GetLogger().Debug("Sending legacy SSE request")

	resp, err := t.client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("Legacy SSE request failed: %v", err)
		t.GetLogger().Debug("Legacy SSE request failed", "error", err)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg)
		}
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Legacy SSE request returned status code %d", resp.StatusCode)
		t.GetLogger().Debug("Legacy SSE request returned error status", "status_code", resp.StatusCode)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg)
		}
		return errors.New(errMsg)
	}

	t.GetLogger().Debug("Legacy SSE connection established, parsing events")

	// Parse SSE events (legacy format)
	reader := bufio.NewReader(resp.Body)
	var buf bytes.Buffer
	var eventType string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				t.GetLogger().Debug("Legacy SSE connection closed (EOF)")
				break
			}
			t.GetLogger().Debug("Error reading legacy SSE stream", "error", err)
			return err
		}

		line = bytes.TrimSpace(line)
		t.GetLogger().Debug("Legacy SSE line received", "line", string(line))

		// Skip comment lines
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}

		// Handle event type
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
			t.GetLogger().Debug("Legacy event type", "type", eventType)
			continue
		}

		// Handle data lines
		if bytes.HasPrefix(line, []byte("data:")) {
			// Extract the data
			data := bytes.TrimPrefix(line, []byte("data:"))
			data = bytes.TrimSpace(data)
			buf.Write(data)
			t.GetLogger().Debug("Legacy event data", "data", string(data))
		} else if len(line) == 0 && buf.Len() > 0 {
			// Empty line indicates end of event
			msg := buf.Bytes()
			t.GetLogger().Debug("Complete legacy event received", "data", string(msg), "type", eventType)

			// Handle different event types (legacy behavior)
			if eventType == "endpoint" {
				// Store the message endpoint (legacy behavior)
				endpoint := string(msg)
				t.mcpURL.Store(&endpoint)
				t.connected.Store(true)
				t.GetLogger().Debug("Legacy POST endpoint set", "endpoint", endpoint)

				// Notify that connection is established with an encoded JSON response
				// that includes both connected status and the endpoint URL
				jsonResp := fmt.Sprintf(`{"connected":true,"endpoint":"%s"}`, endpoint)
				select {
				case t.readCh <- []byte(jsonResp):
					t.GetLogger().Debug("Sent legacy connected notification with endpoint")
				default:
					t.GetLogger().Debug("Legacy connected notification channel full, skipping")
				}
			} else if eventType == "message" || eventType == "" {
				// Regular message, process it
				response, err := t.HandleMessage(msg)
				if err != nil {
					t.GetLogger().Debug("Error handling legacy message", "error", err)
					// Log error but continue processing
					buf.Reset()
					eventType = ""
					continue
				}

				if response != nil {
					t.GetLogger().Debug("Sending legacy response", "response", string(response))
					select {
					case t.readCh <- response:
						// Message sent
					default:
						// Channel full, discard oldest message
						<-t.readCh
						t.readCh <- response
					}
				}
			}

			// Reset buffer and event type for next event
			buf.Reset()
			eventType = ""
		}
	}

	t.GetLogger().Debug("Legacy SSE connection closed")
	return errors.New("Legacy SSE connection closed")
}

// GetAddr returns the transport's address
func (t *Transport) GetAddr() string {
	return t.addr
}

// GetStoredMCPURL returns the stored MCP URL for POST requests (client mode only)
func (t *Transport) GetStoredMCPURL() string {
	if !t.isClient {
		return ""
	}

	mcpURL := t.mcpURL.Load()
	if mcpURL == nil {
		return ""
	}
	return *mcpURL
}
