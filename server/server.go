package server

import (
	"strings"
	"sync"

	// "github.com/localrivet/gomcp/logx" // No longer needed here
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

const ServerVersion = "0.1.0" // Define server version constant

// ResourceDetails holds the details for adding a resource.
type ResourceDetails struct {
	URI         string
	Kind        string
	Name        string
	Description string
}

// RootDetails holds the details for adding a root.
type RootDetails struct {
	URI         string
	Kind        string
	Name        string
	Description string
}

// Server represents the main MCP server instance.
type Server struct {
	name   string
	*Hooks // Embed Hooks struct

	// Logging
	logger       logx.Logger           // Use interface type
	loggingLevel protocol.LoggingLevel // Store the current level requested by client

	// Transport Management
	TransportManager *TransportManager

	// State Management
	Registry            *Registry
	SubscriptionManager *SubscriptionManager
	MessageHandler      *MessageHandler

	// Capability Flags (reflect server implementation status)
	ImplementsResourceSubscription bool
	ImplementsResourceListChanged  bool
	ImplementsPromptListChanged    bool
	ImplementsToolListChanged      bool
	ImplementsLogging              bool
	ImplementsCompletions          bool // Tracks if completion/complete (V2024) is implemented
	ImplementsSampling             bool // Tracks if sampling/createMessage (V2025) can be sent
	ImplementsAuthorization        bool

	// Used by LifecycleHandler for graceful shutdown
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once

	// Add mutexes for concurrent access if necessary
	done chan struct{}  // Channel to signal server termination
	wg   sync.WaitGroup // WaitGroup to wait for goroutines to finish
}

// NewServer creates a new Server instance.
func NewServer(name string) *Server {
	srv := &Server{
		name: name,
		// Initialize the embedded Hooks struct.
		Hooks: NewHooks(),

		// Initialize capability tracking fields based on current implementation status
		ImplementsResourceSubscription: true,  // Phase 3.3
		ImplementsResourceListChanged:  false, // Phase 3.3 (infrastructure in place, BUT notification sending logic missing)
		ImplementsPromptListChanged:    true,  // Phase 3.4 (infrastructure in place)
		ImplementsToolListChanged:      true,  // Set to true as callback exists
		ImplementsLogging:              true,  // Phase 5.3 (basic handlers added)
		ImplementsCompletions:          true,  // Phase 4.1 (V2024 method handled)
		ImplementsSampling:             true,  // Phase 5.1 (Context.CreateMessage implemented)
		ImplementsAuthorization:        false, // Phase 7.1 (TODO)

		// Initialize logger using the interface
		logger:       logx.NewLogger("stdout"), // Returns Logger interface
		loggingLevel: protocol.LogLevelInfo,    // Default level

		// Initialize other components
		Registry:            NewRegistry(),
		SubscriptionManager: NewSubscriptionManager(),
		TransportManager:    NewTransportManager(),
		shutdownSignal:      make(chan struct{}),
		done:                make(chan struct{}),
	}

	// Initialize the logger using the factory function
	// We store it as the interface type types.Logger
	srv.logger = logx.NewLogger(name) // Initialize logger with server name prefix

	// Setup registry callbacks to notify MessageHandler about changes
	srv.Registry.SetResourceChangedCallback(func(uri string) {
		// Get all subscribers for this URI
		subscribers := srv.SubscriptionManager.GetSubscribedConnectionIDs(uri) // Corrected method name

		// Handle resource list change notification for all sessions supporting it
		notifyListChangedSessions := []string{}
		allSessions := srv.TransportManager.GetAllSessionIDs()
		for _, sessionID := range allSessions {
			_, caps, ok := srv.TransportManager.GetSession(sessionID)
			if ok && caps != nil && caps.Resources != nil && caps.Resources.ListChanged {
				notifyListChangedSessions = append(notifyListChangedSessions, sessionID)
			}
		}
		if len(notifyListChangedSessions) > 0 {
			srv.handleResourceListChange(notifyListChangedSessions) // Pass slice of IDs
		}

		// Handle resource update notification only for direct subscribers
		if len(subscribers) > 0 {
			srv.handleResourceUpdate(subscribers, uri) // Pass subscriber slice and URI
		}
	})
	srv.Registry.SetPromptChangedCallback(func() {
		srv.logger.Debug("Prompt change callback triggered")
		notifySessionIDs := []string{}
		allSessionIDs := srv.TransportManager.GetAllSessionIDs()
		for _, sessionID := range allSessionIDs {
			_, caps, ok := srv.TransportManager.GetSession(sessionID)
			if ok && caps != nil && caps.Prompts != nil && caps.Prompts.ListChanged {
				notifySessionIDs = append(notifySessionIDs, sessionID)
			}
		}
		if len(notifySessionIDs) > 0 {
			srv.handlePromptListChange(notifySessionIDs)
		} else {
			srv.logger.Debug("Prompt change callback: No sessions support prompt list changes.")
		}
	})
	srv.Registry.SetToolChangedCallback(func() {
		srv.logger.Debug("Tool change callback triggered")
		notifySessionIDs := []string{}
		allSessionIDs := srv.TransportManager.GetAllSessionIDs()
		for _, sessionID := range allSessionIDs {
			_, caps, ok := srv.TransportManager.GetSession(sessionID)
			if ok && caps != nil && caps.Tools != nil && caps.Tools.ListChanged {
				notifySessionIDs = append(notifySessionIDs, sessionID)
			}
		}
		if len(notifySessionIDs) > 0 {
			srv.handleToolListChange(notifySessionIDs)
		}
	})

	// Initialize the embedded MessageHandler struct.
	srv.MessageHandler = NewMessageHandler(srv)

	// Initialize the embedded SubscriptionManager.
	srv.SubscriptionManager = NewSubscriptionManager()

	return srv
}

// --- Notification Sending Helpers (Callbacks call these) ---

// handleResourceListChange sends notifications/resources/list_changed to the provided session IDs.
func (s *Server) handleResourceListChange(sessionIDs []string) {
	if len(sessionIDs) == 0 {
		return
	}
	s.logger.Info("Sending notifications/resources/list_changed to %d sessions", len(sessionIDs))
	notification := protocol.NewNotification(protocol.MethodNotifyResourcesListChanged, nil)
	s.MessageHandler.SendNotificationToConnections(sessionIDs, notification)
}

// handleResourceUpdate sends notifications/resources/updated to the provided session IDs for the given URI.
func (s *Server) handleResourceUpdate(sessionIDs []string, uri string) {
	if len(sessionIDs) == 0 {
		return
	}
	resource, exists := s.Registry.GetResource(uri)
	if !exists {
		// Resource was likely deleted, list_changed notification already covers this.
		s.logger.Debug("Resource %s not found for update notification (likely deleted).", uri)
		return
	}
	s.logger.Info("Sending notifications/resources/updated for %s to %d sessions", uri, len(sessionIDs))
	params := protocol.ResourceUpdatedParams{Resource: resource}
	notification := protocol.NewNotification(protocol.MethodNotifyResourceUpdated, params)
	s.MessageHandler.SendNotificationToConnections(sessionIDs, notification)
}

// handlePromptListChange sends notifications/prompts/list_changed to the provided session IDs.
func (s *Server) handlePromptListChange(sessionIDs []string) {
	if len(sessionIDs) == 0 {
		return
	}
	s.logger.Info("Sending notifications/prompts/list_changed to %d sessions: %v", len(sessionIDs), sessionIDs)
	notification := protocol.NewNotification(protocol.MethodNotifyPromptsListChanged, nil)
	s.MessageHandler.SendNotificationToConnections(sessionIDs, notification)
}

// handleToolListChange sends notifications/tools/list_changed to the provided session IDs.
func (s *Server) handleToolListChange(sessionIDs []string) {
	if len(sessionIDs) == 0 {
		return
	}
	s.logger.Info("Sending notifications/tools/list_changed to %d sessions", len(sessionIDs))
	notification := protocol.NewNotification(protocol.MethodNotifyToolsListChanged, nil)
	s.MessageHandler.SendNotificationToConnections(sessionIDs, notification)
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	s.logger.Info("Server received close signal")
	// Close the done channel to signal termination
	close(s.done)
	// Wait for all goroutines (transports, handlers) to finish
	s.wg.Wait()
	s.logger.Info("Server shut down gracefully")
	return nil
}

// AsStdio configures the server to use standard I/O as its transport.
func (s *Server) AsStdio() *Server {
	s.TransportManager.AsStdio(s) // Call the embedded method, passing self
	return s
}

// AsWebsocket configures the server to use WebSocket as its transport.
func (s *Server) AsWebsocket(addr string, path string) *Server {
	s.TransportManager.AsWebsocket(s, addr, path) // Call the embedded method, passing self and parameters
	return s
}

// AsSSE configures the server to use Server-Sent Events as its transport.
func (s *Server) AsSSE(addr string, basePath string) *Server {
	s.TransportManager.AsSSE(s, addr, basePath) // Call the embedded method, passing self and parameters
	return s
}

// Resource registers a new static resource or dynamic resource template with the server's registry.
// It uses a flexible options pattern for configuration.
//
// Example:
//
//	server.Resource("/static/data.txt", WithTextContent("Hello"), WithMimeType("text/plain"))
//	server.Resource("/api/users/{id}", WithHandler(getUserHandler), WithDescription("Fetches user data"))
func (s *Server) Resource(uri string, options ...ResourceOption) *Server {
	// Create a default config
	config := newResourceConfig()

	// Apply all provided options
	for _, opt := range options {
		opt(&config)
	}

	// --- Phase 2 Implementation ---
	// TODO: Add more validation for the config (e.g., ensure only one content source is set)

	// Determine if URI is a template (basic check for now)
	isTemplate := strings.Contains(uri, "{") && strings.Contains(uri, "}")

	if isTemplate {
		// Ensure a handler is provided for templates
		if config.HandlerFn == nil {
			s.logger.Error("Registration error for template URI '%s': WithHandler must be provided for resource templates.", uri)
			// Decide on error handling: log and ignore, return error from Resource?, panic?
			// For now, log and ignore.
		} else {
			err := s.Registry.RegisterResourceTemplate(uri, config)
			if err != nil {
				// Handle registration error - log it for now
				s.logger.Error("Failed to register resource template '%s': %v", uri, err)
				// Depending on desired server behavior, could panic or return the error
			}
		}
	} else { // Static resource
		// Ensure no handler is provided for static resources (or handle appropriately)
		if config.HandlerFn != nil {
			s.logger.Warn("Registration warning for static URI '%s': WithHandler was provided but URI is not a template. Handler will be ignored.", uri)
			// config.HandlerFn = nil // Optionally clear the handler
		}
		// Ensure some content source is provided for static resources
		if config.Content == nil && config.FilePath == "" && config.DirPath == "" && config.URL == "" {
			s.logger.Error("Registration error for static URI '%s': No content source (WithTextContent, WithFileContent, etc.) provided.", uri)
			// Log and ignore for now
		} else {
			err := s.Registry.RegisterStaticResource(uri, config)
			if err != nil {
				// Handle registration error - log it for now
				s.logger.Error("Failed to register static resource '%s': %v", uri, err)
				// Depending on desired server behavior, could panic or return the error
			}
		}
	}

	return s
}

// Root registers a new root with the server's registry.
func (s *Server) Root(root protocol.Root) *Server {
	s.Registry.AddRoot(root) // Delegate to embedded Registry
	return s
}

// Prompt registers a new prompt with the server's registry.
func (s *Server) Prompt(name, description string, messages ...protocol.PromptMessage) *Server {
	s.Registry.AddPrompt(protocol.Prompt{
		URI:         name, // Set URI to the same as name for lookup consistency
		Name:        name,
		Description: description,
		Messages:    messages,
	})
	return s
}

// Tool registers a new tool with the server's registry.
// The fn parameter should be a function with a single struct argument and returns (any, error).
func (s *Server) Tool(
	name, desc string,
	fn any, // Using any for now due to Go method generic limitations
) *Server {
	s.Registry.Tool(name, desc, fn)
	return s
}

// NotifyResourceUpdated triggers a resource update notification for subscribers.
func (s *Server) NotifyResourceUpdated(uri string) {
	subscribers := s.SubscriptionManager.GetSubscribedConnectionIDs(uri) // Corrected method name
	if len(subscribers) > 0 {
		s.handleResourceUpdate(subscribers, uri)
	}
}

// WithLogger sets a custom logger for the server.
// This allows using an external logger implementation that implements the logx.Logger interface.
func (s *Server) WithLogger(logger logx.Logger) *Server {
	if logger != nil {
		s.logger = logger

		// Propagate logger to components that have their own logger instance
		if s.Registry != nil {
			s.Registry.SetLogger(logger)
		}

		if s.MessageHandler != nil {
			s.MessageHandler.SetLogger(logger)
		}

		if s.SubscriptionManager != nil {
			s.SubscriptionManager.SetLogger(logger)
		}

		if s.TransportManager != nil {
			s.TransportManager.SetLogger(logger)
		}
	}
	return s
}

// Run starts the configured transport(s) and blocks until the server is closed.
func (s *Server) Run() error {
	s.logger.Info("Starting server...")
	// Add 1 to WaitGroup for the main server loop/signal handling
	s.wg.Add(1)

	// Start configured transports in goroutines
	if err := s.TransportManager.Run(s); err != nil { // Corrected method name
		s.logger.Error("Failed to start transports: %v", err)
		return err // Return error if transports fail to start
	}

	// Wait for termination signal (e.g., from Close() or OS signal)
	<-s.done

	// Signal transports to stop
	s.TransportManager.Shutdown() // Corrected method name

	// Signal completion of the main server loop
	s.wg.Done()
	s.logger.Info("Server Run loop finished")
	return nil
}

// TODO: Add methods for AddPrompt, AddTool, RunStdio, etc.
