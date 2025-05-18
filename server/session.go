package server

import (
	"sync"
	"time"
)

// SessionID is a unique identifier for a client session
type SessionID string

// ClientSession represents a session with a client
type ClientSession struct {
	ID              SessionID         // Unique session identifier
	ClientInfo      ClientInfo        // Information about the client
	Created         time.Time         // When the session was created
	LastActive      time.Time         // Last time the session was active
	ProtocolVersion string            // Negotiated protocol version
	Metadata        map[string]string // Additional session metadata
}

// SessionManager manages client sessions
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[SessionID]*ClientSession
	nextID   int64
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[SessionID]*ClientSession),
	}
}

// CreateSession creates a new client session
func (sm *SessionManager) CreateSession(clientInfo ClientInfo, protocolVersion string) *ClientSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate a new session ID
	sm.nextID++
	sessionID := SessionID(generateUniqueID(sm.nextID))

	// Create a new session
	session := &ClientSession{
		ID:              sessionID,
		ClientInfo:      clientInfo,
		Created:         time.Now(),
		LastActive:      time.Now(),
		ProtocolVersion: protocolVersion,
		Metadata:        make(map[string]string),
	}

	// Store the session
	sm.sessions[sessionID] = session

	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id SessionID) (*ClientSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	return session, exists
}

// UpdateSession updates an existing session
func (sm *SessionManager) UpdateSession(id SessionID, update func(*ClientSession)) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return false
	}

	// Apply the update
	update(session)

	// Update the last active time
	session.LastActive = time.Now()

	return true
}

// CloseSession removes a session
func (sm *SessionManager) CloseSession(id SessionID) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.sessions[id]
	if !exists {
		return false
	}

	delete(sm.sessions, id)
	return true
}

// DetectClientCapabilities infers client capabilities from the protocol version
// and any other available information
func DetectClientCapabilities(protocolVersion string) SamplingCapabilities {
	// Initialize capabilities based on protocol version
	caps := SamplingCapabilities{
		Supported:    true,
		TextSupport:  true,
		ImageSupport: true,
		AudioSupport: false, // Assume no audio support by default
	}

	// Update based on protocol version
	switch protocolVersion {
	case "draft", "2025-03-26":
		// These versions support all content types
		caps.AudioSupport = true
	case "2024-11-05":
		// This version only supports text and image
		caps.AudioSupport = false
	default:
		// Unknown version, default to most restrictive
		caps.ImageSupport = false
		caps.AudioSupport = false
	}

	return caps
}

// UpdateClientCapabilities updates the capabilities of a client session
// based on protocol version and any other information available
func (sm *SessionManager) UpdateClientCapabilities(id SessionID, caps SamplingCapabilities) bool {
	return sm.UpdateSession(id, func(session *ClientSession) {
		session.ClientInfo.SamplingCaps = caps
		session.ClientInfo.SamplingSupported = caps.Supported
	})
}

// generateUniqueID creates a unique session identifier (simplified implementation)
func generateUniqueID(id int64) string {
	// In a real implementation, this would generate a secure random ID
	// For this example, we'll just use a simple string representation
	return time.Now().Format("20060102150405") + "-" + time.Now().Format("000000000") + "-" + time.Now().Format("000000000000000")
}
