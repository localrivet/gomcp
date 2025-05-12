package server

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// TransportType represents the type of transport.
type TransportType int

const (
	TransportNone TransportType = iota
	TransportStdio
	TransportWebsocket
	TransportSSE
)

// TransportManager handles the server's transport mechanisms.
type TransportManager struct {
	// Field to track the selected transport
	selectedTransport TransportType
	// Map to store active client sessions (sessionID -> session)
	Sessions map[string]types.ClientSession
	// Add a map to store capabilities per session ID
	Capabilities map[string]*protocol.ClientCapabilities
	sessionsMu   sync.RWMutex

	// Transport configuration fields
	addr          string
	websocketAddr string
	websocketPath string
	sseAddr       string
	sseBasePath   string

	// Added for SetLogger functionality
	logger logx.Logger
}

// NewTransportManager creates a new TransportManager.
func NewTransportManager() *TransportManager {
	return &TransportManager{
		Sessions:     make(map[string]types.ClientSession),
		Capabilities: make(map[string]*protocol.ClientCapabilities),
	}
}

// RegisterSession adds a new client session to the manager.
func (tm *TransportManager) RegisterSession(session types.ClientSession, caps *protocol.ClientCapabilities) {
	tm.sessionsMu.Lock()
	defer tm.sessionsMu.Unlock()
	tm.Sessions[session.SessionID()] = session
	tm.Capabilities[session.SessionID()] = caps // Store capabilities

	// Use logger if available, otherwise fall back to standard log
	if tm.logger != nil {
		tm.logger.Info("Registered session: %s with caps: %v", session.SessionID(), caps != nil)
	}
}

// RemoveSession removes a client session from the manager.
func (tm *TransportManager) RemoveSession(sessionID string) {
	tm.sessionsMu.Lock()
	defer tm.sessionsMu.Unlock()
	delete(tm.Sessions, sessionID)
	delete(tm.Capabilities, sessionID) // Also remove capabilities

	// Use logger if available, otherwise fall back to standard log
	if tm.logger != nil {
		tm.logger.Info("Removed session: %s", sessionID)
	}
}

// GetSession retrieves a client session by its ID.
// It now also returns the client's capabilities.
func (tm *TransportManager) GetSession(sessionID string) (types.ClientSession, *protocol.ClientCapabilities, bool) {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()
	session, ok := tm.Sessions[sessionID]
	if !ok {
		return nil, nil, false
	}
	caps := tm.Capabilities[sessionID] // Retrieve capabilities (will be nil if not found)
	return session, caps, ok           // Return session, caps (potentially nil), and session found status
}

// GetAllSessionIDs returns a slice of all active session IDs.
func (tm *TransportManager) GetAllSessionIDs() []string {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()
	ids := make([]string, 0, len(tm.Sessions))
	for id := range tm.Sessions {
		ids = append(ids, id)
	}
	return ids
}

// Deprecated: SendMessage sends a raw message to a specific connection ID.
// Prefer using session.SendResponse or session.SendNotification.
func (tm *TransportManager) SendMessage(sessionID string, message []byte) error {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()
	session, ok := tm.Sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// We cannot reliably send raw bytes via the ClientSession interface without knowing the specific transport.
	// Attempting a generic JSON-RPC notification as a placeholder, but this is wrong.
	if tm.logger != nil {
		tm.logger.Warn("[DEPRECATED SendMessage] Attempting to send raw bytes to session %s. This needs refactoring!", sessionID)
	}

	// Create a placeholder notification - THIS IS LIKELY NOT WHAT THE CALLER INTENDED
	var genericPayload interface{}
	if err := json.Unmarshal(message, &genericPayload); err != nil {
		genericPayload = string(message) // Fallback to string if not JSON
	}
	placeholderNotif := protocol.NewNotification("server/raw_message", genericPayload)

	// Use the session's SendNotification method
	err := session.SendNotification(*placeholderNotif)
	if err != nil {
		if tm.logger != nil {
			tm.logger.Error("Error sending placeholder notification via session %s: %v", sessionID, err)
		}
		return fmt.Errorf("error sending via session: %w", err)
	}

	return nil
}

// AsStdio configures the server to use standard I/O as its transport.
func (tm *TransportManager) AsStdio(s *Server) *Server {
	tm.selectedTransport = TransportStdio
	// TODO: Implement stdio transport configuration
	return s
}

// AsWebsocket configures the server to use WebSocket as its transport.
func (tm *TransportManager) AsWebsocket(s *Server, addr string, path string) *Server {
	tm.selectedTransport = TransportWebsocket
	tm.websocketAddr = addr
	tm.websocketPath = path
	return s
}

// AsSSE configures the server to use Server-Sent Events as its transport.
func (tm *TransportManager) AsSSE(s *Server, addr string, basePath string) *Server {
	tm.selectedTransport = TransportSSE
	tm.sseAddr = addr
	tm.sseBasePath = basePath
	return s
}

// Run starts the selected transport.
func (tm *TransportManager) Run(s *Server) error {
	switch tm.selectedTransport {
	case TransportStdio:
		return runStdioTransport(tm, s) // Delegate to stdio_transport.go

	case TransportWebsocket:
		return tm.runWebsocketTransport(s) // Delegate to websocket_transport.go
	case TransportSSE:
		return tm.runSseTransport(s) // Delegate to sse_transport.go
	case TransportNone:
		return fmt.Errorf("no transport configured")
	default:
		return fmt.Errorf("unknown transport type")
	}
}

// runSseTransport is a placeholder for the SSE transport implementation.
// It should handle multiple connections internally.

// Shutdown signals the transport manager to stop accepting new connections.
func (tm *TransportManager) Shutdown() {
	if tm.logger != nil {
		tm.logger.Info("TransportManager received shutdown signal")
	}
	// TODO: Implement logic to stop listening for new connections for each transport
	// For StdIO, this might not be necessary as it reads from stdin until EOF.
}

// SetLogger sets the logger for the transport manager and propagates it to all active sessions
func (tm *TransportManager) SetLogger(logger logx.Logger) {
	tm.logger = logger

	// Propagate logger to active sessions if they implement LoggerSetter
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()

	for _, session := range tm.Sessions {
		if loggerSetter, ok := session.(interface{ SetLogger(logger logx.Logger) }); ok {
			loggerSetter.SetLogger(logger)
		}
	}
}
