package server

import (
	"log"

	"github.com/localrivet/gomcp/protocol"
)

// LifecycleHandler provides methods for handling MCP lifecycle messages.
type LifecycleHandler struct {
	server *Server // Reference back to the main Server struct
}

// NewLifecycleHandler creates a new LifecycleHandler instance.
func NewLifecycleHandler(srv *Server) *LifecycleHandler {
	return &LifecycleHandler{
		server: srv,
	}
}

// InitializeHandler handles the 'initialize' request.
// It performs version negotiation and returns server capabilities.
// It now also returns the negotiated client capabilities.
func (h *LifecycleHandler) InitializeHandler(params protocol.InitializeRequestParams) (protocol.InitializeResult, *protocol.ClientCapabilities, error) {
	// 1. Version Negotiation
	clientSchemaVersion := params.ProtocolVersion // Use correct field name
	negotiatedVersion := ""                       // The version the server chooses
	// Use correct constants
	supportedVersions := []string{protocol.OldProtocolVersion, protocol.CurrentProtocolVersion}

	// Check if client requested a specific supported version
	for _, supported := range supportedVersions {
		if clientSchemaVersion == supported {
			negotiatedVersion = supported
			break
		}
	}

	// If no specific match, default to the latest supported version
	if negotiatedVersion == "" {
		negotiatedVersion = protocol.CurrentProtocolVersion // Default to latest
		log.Printf("Client requested version '%s', defaulting to latest supported: %s", clientSchemaVersion, negotiatedVersion)
	} else {
		log.Printf("Client requested version '%s', negotiated version: %s", clientSchemaVersion, negotiatedVersion)
	}

	// TODO: Store the negotiatedVersion per session if needed for future requests?
	// Currently handled by checking inside request handlers.

	// 2. Prepare Server Capabilities based on negotiated version and server implementation
	serverCaps := protocol.ServerCapabilities{}

	// Resources (Subscribe, ListChanged)
	if h.server.ImplementsResourceSubscription || h.server.ImplementsResourceListChanged {
		serverCaps.Resources = &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
		if h.server.ImplementsResourceSubscription {
			serverCaps.Resources.Subscribe = true
		}
		if h.server.ImplementsResourceListChanged {
			serverCaps.Resources.ListChanged = true
		}
	}

	// Prompts (ListChanged)
	if h.server.ImplementsPromptListChanged {
		serverCaps.Prompts = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
		serverCaps.Prompts.ListChanged = true
	}

	// Tools (ListChanged)
	if h.server.ImplementsToolListChanged {
		serverCaps.Tools = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
		serverCaps.Tools.ListChanged = true
	}

	// Add 2025-03-26 specific capabilities if negotiated
	if negotiatedVersion == protocol.CurrentProtocolVersion {
		// Logging Capability (Presence indicates support for logging/set_level and notifications/message)
		if h.server.ImplementsLogging { // Server flag for overall logging support
			serverCaps.Logging = &struct{}{}
		}

		// Completions (Sampling) Capability (Presence indicates support for sampling/create_message)
		if h.server.ImplementsCompletions { // Server flag for completion/sampling support
			serverCaps.Completions = &struct{}{}
		}

		// Authorization Capability (Presence indicates support for auth flow)
		if h.server.ImplementsAuthorization { // Server flag for auth support
			serverCaps.Authorization = &struct{}{}
		}
	}

	// Note: Basic capabilities like implementing tools/call, resources/list, etc.,
	// are implicitly supported if the server offers the respective top-level capabilities
	// (e.g., Tools, Resources). Finer-grained flags like ImplementsToolCall are not
	// part of the standard protocol.ServerCapabilities structure.
	// Support for $/progress, $/cancelled is also generally assumed.

	// 3. Prepare Result
	result := protocol.InitializeResult{
		ProtocolVersion: negotiatedVersion, // Use correct field name
		ServerInfo: protocol.Implementation{
			Name:    h.server.name, // Use server name
			Version: ServerVersion,
		},
		Capabilities: serverCaps,
	}

	// Log the client capabilities received
	log.Printf("Client capabilities: %+v", params.Capabilities)

	// Return result, client capabilities, and no error
	return result, &params.Capabilities, nil
}

// ShutdownHandler handles the 'shutdown' request.
func (h *LifecycleHandler) ShutdownHandler() error {
	log.Println("Handling shutdown request")
	// Signal the transport manager to stop accepting new connections
	h.server.TransportManager.Shutdown()
	// The actual server termination will happen after the response is sent and 'exit' is received.
	return nil // Return nil for success
}

// ExitHandler handles the 'exit' notification.
func (h *LifecycleHandler) ExitHandler() {
	log.Println("Handling exit notification")
	// Trigger server termination by closing the done channel
	close(h.server.done)
}

// InitializedHandler handles the 'initialized' notification.
func (h *LifecycleHandler) InitializedHandler() {
	log.Println("Handling initialized notification")
	// The client is now ready to receive requests and notifications.
}
