package server

import (
	"sync"
	"time"
)

// SessionID is a unique identifier for a client session.
// It's used to track and manage individual client connections to the server.
type SessionID string

// ClientSession represents a session with a client.
// It encapsulates all client-specific information including capabilities,
// negotiated protocol version, and session metadata needed for managing
// the client connection lifecycle.
type ClientSession struct {
	ID              SessionID         // Unique session identifier
	ClientInfo      ClientInfo        // Information about the client
	Created         time.Time         // When the session was created
	LastActive      time.Time         // Last time the session was active
	ProtocolVersion string            // Negotiated protocol version
	Metadata        map[string]string // Additional session metadata
}

// SessionManager manages client sessions.
// It provides methods for creating, retrieving, updating, and closing
// client sessions, ensuring proper lifecycle management and thread safety.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[SessionID]*ClientSession
	nextID   int64
}

// NewSessionManager creates a new session manager.
// It initializes the internal data structures needed for tracking client sessions.
//
// Returns:
//   - A new SessionManager instance ready for use
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[SessionID]*ClientSession),
	}
}

// CreateSession creates a new client session.
// This method generates a unique session ID, initializes a new session with
// the provided client information, and adds it to the session manager.
//
// Parameters:
//   - clientInfo: Information about the client's capabilities and features
//   - protocolVersion: The negotiated MCP protocol version for this client
//
// Returns:
//   - A new ClientSession instance configured for the client
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

// GetSession retrieves a session by ID.
// This method looks up a client session using its unique identifier.
//
// Parameters:
//   - id: The unique identifier of the session to retrieve
//
// Returns:
//   - The ClientSession if found
//   - A boolean indicating whether the session exists
func (sm *SessionManager) GetSession(id SessionID) (*ClientSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	return session, exists
}

// UpdateSession updates an existing session.
// This method applies custom updates to a session while maintaining thread safety,
// and automatically updates the session's last active timestamp.
//
// Parameters:
//   - id: The unique identifier of the session to update
//   - update: A function that receives the session and applies updates to it
//
// Returns:
//   - A boolean indicating whether the session was found and updated
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

// CloseSession removes a session.
// This method deletes a client session from the session manager,
// typically called when a client disconnects or times out.
//
// Parameters:
//   - id: The unique identifier of the session to close
//
// Returns:
//   - A boolean indicating whether the session was found and removed
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

// DetectClientCapabilities infers client capabilities from the protocol version.
// This function analyzes the protocol version to determine which features
// and content types the client is likely to support, particularly for sampling operations.
//
// Parameters:
//   - protocolVersion: The MCP protocol version negotiated with the client
//
// Returns:
//   - A SamplingCapabilities struct describing the client's supported features
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

// UpdateClientCapabilities updates the capabilities of a client session.
// This method modifies a session's recorded capabilities, typically called
// when new information about client support becomes available.
//
// Parameters:
//   - id: The unique identifier of the session to update
//   - caps: The new sampling capabilities to set for the client
//
// Returns:
//   - A boolean indicating whether the session was found and updated
func (sm *SessionManager) UpdateClientCapabilities(id SessionID, caps SamplingCapabilities) bool {
	return sm.UpdateSession(id, func(session *ClientSession) {
		session.ClientInfo.SamplingCaps = caps
		session.ClientInfo.SamplingSupported = caps.Supported
	})
}

// generateUniqueID creates a unique session identifier.
// This is a simplified implementation that combines the current timestamp
// with a sequence number to create reasonably unique identifiers.
//
// Parameters:
//   - id: A sequence number to incorporate into the ID
//
// Returns:
//   - A string containing the unique session identifier
func generateUniqueID(id int64) string {
	// In a real implementation, this would generate a secure random ID
	// For this example, we'll just use a simple string representation
	return time.Now().Format("20060102150405") + "-" + time.Now().Format("000000000") + "-" + time.Now().Format("000000000000000")
}
