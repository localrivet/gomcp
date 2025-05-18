// Package server provides the server-side implementation of the MCP protocol.
// It offers a comprehensive API for building and running MCP servers that can
// register tools, resources, and prompt templates for client interaction.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/transport"
	"github.com/localrivet/gomcp/transport/mqtt"
	"github.com/localrivet/gomcp/transport/nats"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/transport/udp"
	"github.com/localrivet/gomcp/transport/unix"
)

// Server represents an MCP server with fluent configuration methods.
// It provides a builder-style API for configuring all aspects of an MCP server
// including tools, resources, prompts, and transport options.
type Server interface {
	// Run starts the server and blocks until it exits.
	//
	// This method initializes the server, starts listening for connections,
	// and processes incoming requests. It blocks until the server encounters
	// an error or is explicitly stopped.
	//
	// Example:
	//  if err := server.Run(); err != nil {
	//      log.Fatalf("Server error: %v", err)
	//  }
	Run() error

	// Tool registers a tool with the server.
	//
	// The name parameter is the unique identifier for the tool. The description
	// parameter provides human-readable documentation. The handler parameter is
	// a function that implements the tool's logic.
	//
	// Tool handlers can have one of the following signatures:
	//  func(ctx *Context) (interface{}, error)
	//  func(ctx *Context, args T) (interface{}, error)
	//  func(ctx *Context) error
	//  func(ctx *Context, args T) error
	//
	// Where T is any struct type that can be unmarshaled from JSON. The struct
	// fields should use `json` tags to map to JSON property names.
	//
	// Example:
	//  server.Tool("add", "Add two numbers", func(ctx *server.Context, args struct {
	//      X float64 `json:"x" required:"true"`
	//      Y float64 `json:"y" required:"true"`
	//  }) (float64, error) {
	//      return args.X + args.Y, nil
	//  })
	Tool(name string, description string, handler interface{}) Server

	// WithAnnotations adds annotations to a tool.
	//
	// Annotations provide additional metadata about a tool, such as examples,
	// parameter descriptions, or usage notes.
	//
	// Example:
	//  server.Tool("greet", "Greet a user", greetHandler).
	//      WithAnnotations("greet", map[string]interface{}{
	//          "examples": []map[string]interface{}{
	//              {
	//                  "description": "Greet a user by name",
	//                  "args": map[string]interface{}{
	//                      "name": "Alice",
	//                  },
	//              },
	//          },
	//      })
	WithAnnotations(toolName string, annotations map[string]interface{}) Server

	// Resource registers a resource with the server.
	//
	// The path parameter defines the resource path, which can include path
	// parameters in the format {paramName}. The description parameter provides
	// human-readable documentation. The handler parameter is a function that
	// implements the resource's logic.
	//
	// Example:
	//  server.Resource("/users/{id}", "Get user information", func(ctx *server.Context, params struct {
	//      ID string `path:"id"`
	//  }) (interface{}, error) {
	//      return getUserById(params.ID)
	//  })
	Resource(path string, description string, handler interface{}) Server

	// Prompt registers a prompt with the server.
	//
	// The name parameter is the unique identifier for the prompt. The description
	// parameter provides human-readable documentation. The template parameters
	// define the prompt template and can be either a string or a function that
	// generates the template.
	//
	// Example:
	//  server.Prompt("greeting", "Greeting template",
	//      "Hello, {{name}}! Welcome to {{service}}.")
	Prompt(name string, description string, template ...interface{}) Server

	// Root sets the allowed root paths.
	//
	// Root paths are the entry points for resource navigation. At least one
	// root path must be defined for resources to be accessible.
	//
	// Example:
	//  server.Root("/api/v1", "/api/v2")
	Root(paths ...string) Server

	// AsStdio configures the server to use Standard I/O for communication.
	//
	// This is useful for child processes or integration with other MCP systems.
	// An optional logFile parameter can be provided to redirect stdio logs.
	//
	// Example:
	//  server.AsStdio("./mcp-server.log")
	AsStdio(logFile ...string) Server

	// AsWebsocket configures the server to use WebSocket for communication.
	//
	// The address parameter specifies the host and port to listen on.
	//
	// Example:
	//  server.AsWebsocket("localhost:8080")
	AsWebsocket(address string) Server

	// AsWebsocketWithPaths configures the server to use WebSocket for communication with custom paths.
	//
	// Example:
	//
	//	server.AsWebsocketWithPaths("localhost:8080", "/api/v1", "/socket")
	AsWebsocketWithPaths(address, pathPrefix, wsPath string) Server

	// AsSSE configures the server to use Server-Sent Events for communication.
	//
	// The address parameter specifies the host and port to listen on.
	//
	// Example:
	//  server.AsSSE("localhost:8080")
	AsSSE(address string) Server

	// AsHTTP configures the server to use HTTP for communication.
	//
	// The address parameter specifies the host and port to listen on.
	//
	// Example:
	//  server.AsHTTP("localhost:8080")
	AsHTTP(address string) Server

	// AsHTTPWithPaths configures the server to use HTTP for communication with custom paths.
	//
	// Example:
	//
	//	server.AsHTTPWithPaths("localhost:8080", "/api/v1", "/rpc")
	AsHTTPWithPaths(address, pathPrefix, apiPath string) Server

	// AsUnixSocket configures the server to use Unix Domain Sockets for communication.
	//
	// Unix Domain Sockets provide high-performance inter-process communication for
	// processes running on the same machine.
	//
	// Example:
	//
	//	server.AsUnixSocket("/tmp/mcp.sock")
	//	// With options:
	//	server.AsUnixSocket("/tmp/mcp.sock", unix.WithPermissions(0600))
	AsUnixSocket(socketPath string, options ...unix.UnixSocketOption) Server

	// AsUDP configures the server to use UDP for communication.
	//
	// UDP provides low-latency communication with minimal overhead,
	// suitable for high-throughput scenarios where occasional packet
	// loss is acceptable.
	//
	// Example:
	//
	//	server.AsUDP(":8080")
	//	// With options:
	//	server.AsUDP(":8080", udp.WithMaxPacketSize(2048))
	AsUDP(address string, options ...udp.UDPOption) Server

	// AsMQTT configures the server to use MQTT for communication
	// with optional configuration options.
	//
	// MQTT provides a publish/subscribe-based communication model,
	// suitable for IoT applications and distributed systems with
	// potentially intermittent connectivity.
	//
	// Example:
	//
	//	server.AsMQTT("tcp://broker.example.com:1883")
	//	// With options:
	//	server.AsMQTT("tcp://broker.example.com:1883",
	//	    mqtt.WithQoS(1),
	//	    mqtt.WithCredentials("username", "password"),
	//	    mqtt.WithTopicPrefix("custom/topic/prefix"))
	AsMQTT(brokerURL string, options ...mqtt.MQTTOption) Server

	// AsMQTTWithClientID configures the server to use MQTT with a specific client ID
	// along with optional configuration options.
	//
	// This is useful when you need to control the client ID to implement
	// features like persistent sessions or shared subscriptions.
	//
	// Example:
	//
	//	server.AsMQTTWithClientID("tcp://broker.example.com:1883", "mcp-server-1")
	//	// With options:
	//	server.AsMQTTWithClientID("tcp://broker.example.com:1883", "mcp-server-1",
	//	    mqtt.WithQoS(2),
	//	    mqtt.WithTopicPrefix("my-org/mcp"))
	AsMQTTWithClientID(brokerURL string, clientID string, options ...mqtt.MQTTOption) Server

	// AsMQTTWithTLS configures the server to use MQTT with TLS security
	// along with optional configuration options.
	//
	// This is recommended for production environments to encrypt
	// communications between the server and the MQTT broker.
	//
	// Example:
	//
	//	server.AsMQTTWithTLS("ssl://broker.example.com:8883",
	//	    mqtt.TLSConfig{
	//	        CertFile: "/path/to/cert.pem",
	//	        KeyFile: "/path/to/key.pem",
	//	        CAFile: "/path/to/ca.pem",
	//	    })
	AsMQTTWithTLS(brokerURL string, tlsConfig mqtt.TLSConfig, options ...mqtt.MQTTOption) Server

	// AsNATS configures the server to use NATS for communication
	// with optional configuration options.
	//
	// NATS provides a high-performance, cloud native communication system,
	// suitable for microservices architectures, IoT messaging, and
	// event-driven applications.
	//
	// Example:
	//
	//	server.AsNATS("nats://localhost:4222")
	//	// With options:
	//	server.AsNATS("nats://localhost:4222",
	//	    nats.WithCredentials("username", "password"),
	//	    nats.WithSubjectPrefix("custom/subject/prefix"))
	AsNATS(serverURL string, options ...nats.NATSOption) Server

	// AsNATSWithClientID configures the server to use NATS with a specific client ID
	// along with optional configuration options.
	//
	// Example:
	//
	//	server.AsNATSWithClientID("nats://localhost:4222", "mcp-server-1")
	//	// With options:
	//	server.AsNATSWithClientID("nats://localhost:4222", "mcp-server-1",
	//	    nats.WithSubjectPrefix("my-org/mcp"))
	AsNATSWithClientID(serverURL string, clientID string, options ...nats.NATSOption) Server

	// AsNATSWithToken configures the server to use NATS with token authentication
	// along with optional configuration options.
	//
	// Example:
	//
	//	server.AsNATSWithToken("nats://localhost:4222", "s3cr3t-t0k3n")
	//	// With options:
	//	server.AsNATSWithToken("nats://localhost:4222", "s3cr3t-t0k3n",
	//	    nats.WithClientID("mcp-server-1"),
	//	    nats.WithSubjectPrefix("my-org/mcp"))
	AsNATSWithToken(serverURL string, token string, options ...nats.NATSOption) Server

	// GetServer returns the underlying server implementation.
	//
	// This is primarily used for advanced configuration or testing.
	GetServer() *serverImpl

	// GetRoots returns the list of registered root paths.
	GetRoots() []string

	// IsPathInRoots checks if a path is within any registered root.
	IsPathInRoots(path string) bool

	// WithSamplingConfig sets the sampling configuration for the server.
	//
	// Sampling configuration controls how sampling requests are handled,
	// including rate limits, caching, and other performance parameters.
	//
	// Example:
	//  config := server.NewSamplingConfig().WithRateLimit(10)
	//  server.WithSamplingConfig(config)
	WithSamplingConfig(config *SamplingConfig) Server

	// WithSamplingController sets a custom sampling controller for the server.
	//
	// This is an advanced method for applications that need fine-grained
	// control over sampling behavior.
	WithSamplingController(controller *SamplingController) Server

	// Logger returns the server's configured logger.
	//
	// This can be used to access the logger for custom logging needs.
	Logger() *slog.Logger
}

// Option represents a server configuration option.
// Server options are used to customize the behavior and configuration of a server instance
// when it is created with NewServer.
type Option func(*serverImpl)

// serverImpl is the concrete implementation of the Server interface.
type serverImpl struct {
	// name is the unique identifier for this server instance, used in logs and server info.
	name string

	// tools is a map of registered tool handlers keyed by tool name.
	tools map[string]*Tool

	// resources is a map of registered resource handlers keyed by path pattern.
	resources map[string]*Resource

	// prompts is a map of registered prompt templates keyed by prompt name.
	prompts map[string]*Prompt

	// roots is a slice of registered root paths for resource navigation.
	roots []string

	// transport is the communication transport used by this server (stdio, websocket, etc.).
	transport transport.Transport

	// logger is the structured logger used for server logs.
	logger *slog.Logger

	// versionDetector handles MCP protocol version detection and negotiation.
	versionDetector *mcp.VersionDetector

	// mu protects concurrent access to server state.
	mu sync.RWMutex

	// protocolVersion is the negotiated MCP protocol version for this server.
	protocolVersion string

	// requestTracker manages pending requests and matches responses to requests.
	requestTracker *requestTracker

	// sessionManager handles client session creation, retrieval, and management.
	sessionManager *SessionManager

	// defaultSession is a session used for simple implementations that don't track
	// multiple client sessions explicitly.
	defaultSession *ClientSession

	// lastRequestID tracks the last used request ID for generating unique request IDs.
	// This is used in the sampling.go file to generate sequential request identifiers
	// for JSON-RPC requests, particularly for sampling operations.
	lastRequestID int64

	// Sampling configuration and controller
	// samplingConfig defines the parameters for sampling behavior (rate limits, caching, etc.).
	samplingConfig *SamplingConfig

	// samplingController manages sampling requests and applies sampling configuration.
	samplingController *SamplingController
}

// GetName returns the server's name.
//
// The server name is set during initialization and is typically used
// in logging and protocol messages.
func (s *serverImpl) GetName() string {
	return s.name
}

// GetTools returns a map of all registered tools.
//
// The map keys are tool names, and the values are the corresponding Tool objects
// containing metadata and handler functions.
func (s *serverImpl) GetTools() map[string]*Tool {
	return s.tools
}

// GetResources returns a map of all registered resources.
//
// The map keys are resource path patterns, and the values are the corresponding
// Resource objects containing metadata and handler functions.
func (s *serverImpl) GetResources() map[string]*Resource {
	return s.resources
}

// GetPrompts returns a map of all registered prompts.
//
// The map keys are prompt names, and the values are the corresponding Prompt
// objects containing metadata and template functions.
func (s *serverImpl) GetPrompts() map[string]*Prompt {
	return s.prompts
}

// GetTransport returns the server's configured transport.
//
// The transport is responsible for communication between the server and clients
// (e.g., stdio, WebSocket, HTTP).
func (s *serverImpl) GetTransport() transport.Transport {
	return s.transport
}

// WithSamplingConfig sets the sampling configuration for the server.
//
// This method configures how the server handles sampling requests, including
// rate limits, caching behavior, and other performance parameters.
//
// Example:
//
//	config := server.NewSamplingConfig().
//	    WithRateLimit(10).
//	    WithCacheSize(100)
//	server.WithSamplingConfig(config)
func (s *serverImpl) WithSamplingConfig(config *SamplingConfig) Server {
	s.samplingConfig = config
	return s
}

// WithSamplingController sets a custom sampling controller for the server.
//
// This is an advanced method for applications that need fine-grained control
// over sampling behavior beyond what is provided by the standard SamplingConfig.
func (s *serverImpl) WithSamplingController(controller *SamplingController) Server {
	s.samplingController = controller
	return s
}

// NewServer creates a new MCP server with the given name and options.
//
// The server is initialized with default settings that can be customized using
// the provided options. By default, the server uses stdio transport and default
// logging configuration.
//
// The name parameter is a human-readable identifier for the server, used in logs
// and server information.
//
// Example:
//
//	// Create a basic server with default settings
//	server := server.NewServer("my-service")
//
//	// Create a server with custom logger and sampling configuration
//	customLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	samplingConfig := server.NewSamplingConfig().WithRateLimit(100)
//
//	server := server.NewServer("my-service",
//	    server.WithLogger(customLogger),
//	    server.WithSamplingConfig(samplingConfig),
//	)
func NewServer(name string, options ...Option) Server {
	s := &serverImpl{
		name:            name,
		tools:           make(map[string]*Tool),
		resources:       make(map[string]*Resource),
		prompts:         make(map[string]*Prompt),
		versionDetector: mcp.NewVersionDetector(),
		logger:          slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		sessionManager:  NewSessionManager(),
	}

	// Set the default transport to stdio
	s.transport = stdio.NewTransport()

	// Initialize sampling configuration with defaults
	s.samplingConfig = NewDefaultSamplingConfig()
	s.samplingController = NewSamplingController(s.samplingConfig, s.logger)

	// Apply options
	for _, option := range options {
		option(s)
	}

	// Set up default session for simple implementations
	defaultClientInfo := ClientInfo{
		SamplingSupported: true,
		SamplingCaps: SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: false, // Will be updated based on protocol version
		},
		ProtocolVersion: s.protocolVersion,
	}
	s.defaultSession = s.sessionManager.CreateSession(defaultClientInfo, "draft")

	return s
}

// WithLogger sets the server's logger.
//
// This option configures the structured logger used by the server for logging events,
// errors, and debug information.
//
// Example:
//
//	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//	    Level: slog.LevelDebug,
//	})
//	logger := slog.New(jsonHandler)
//
//	server := server.NewServer("my-service",
//	    server.WithLogger(logger),
//	)
func WithLogger(logger *slog.Logger) Option {
	return func(s *serverImpl) {
		s.logger = logger
	}
}

// Logger returns the server's logger.
//
// This method provides access to the server's configured logger for custom logging needs.
// It can be used to log additional information or to reconfigure logging at runtime.
//
// Example:
//
//	// Log a custom message with the server's logger
//	server.Logger().Info("custom event occurred",
//	    "correlation_id", correlationID,
//	    "user_id", userID,
//	)
func (s *serverImpl) Logger() *slog.Logger {
	return s.logger
}

// ProcessInitialize processes an initialize request.
//
// This method handles the initial handshake between client and server, including
// protocol version negotiation, capability exchange, and session creation.
//
// The ctx parameter contains the client's initialization request. The method returns
// a response containing the negotiated protocol version and server capabilities.
func (s *serverImpl) ProcessInitialize(ctx *Context) (interface{}, error) {
	// Extract the client's requested protocol version
	clientProtocolVersion, err := ExtractProtocolVersion(ctx.Request.Params)
	if err != nil {
		return nil, err
	}

	// Validate and potentially normalize the protocol version
	protocolVersion, err := s.ValidateProtocolVersion(clientProtocolVersion)
	if err != nil {
		return nil, err
	}

	// Store the validated protocol version
	s.mu.Lock()
	s.protocolVersion = protocolVersion
	s.mu.Unlock()

	// Determine sampling capabilities based on protocol version
	samplingCaps := DetectClientCapabilities(protocolVersion)

	// Update or create client info
	clientInfo := ClientInfo{
		SamplingSupported: samplingCaps.Supported,
		SamplingCaps:      samplingCaps,
		ProtocolVersion:   protocolVersion,
	}

	// Create a new session for this client
	session := s.sessionManager.CreateSession(clientInfo, protocolVersion)

	// Store the session ID in the context metadata
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]interface{})
	}
	ctx.Metadata["sessionID"] = string(session.ID)

	// For simple implementations that don't track multiple sessions, update the default session
	s.mu.Lock()
	s.defaultSession = session
	s.mu.Unlock()

	// Log the session creation
	s.logger.Info("client connected",
		"sessionID", string(session.ID),
		"protocolVersion", protocolVersion,
		"samplingSupported", samplingCaps.Supported,
		"audioSupport", samplingCaps.AudioSupport)

	// Prepare the sampling capabilities for the response based on protocol version
	samplingCapabilities := map[string]interface{}{
		"supported": true,
		"contentTypes": map[string]bool{
			"text":  true,
			"image": samplingCaps.ImageSupport,
		},
	}

	// Audio is only supported in draft and 2025-03-26 versions
	if protocolVersion == "draft" || protocolVersion == "2025-03-26" {
		samplingCapabilities["contentTypes"].(map[string]bool)["audio"] = samplingCaps.AudioSupport
	}

	// Return response with the validated protocol version
	return map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"logging": map[string]interface{}{},
			"prompts": map[string]interface{}{
				"listChanged": true,
			},
			"resources": map[string]interface{}{
				"subscribe":   true,
				"listChanged": true,
			},
			"tools": map[string]interface{}{
				"listChanged": true,
			},
			"sampling": samplingCapabilities,
		},
		"serverInfo": map[string]interface{}{
			"name":    s.name,
			"version": "1.0.0",
		},
	}, nil
}

// ProcessShutdown processes a shutdown request.
//
// This method handles graceful shutdown requests from clients. It returns a success
// response to the client and initiates server shutdown.
//
// The ctx parameter contains the shutdown request. The method returns a simple
// response indicating whether the shutdown was initiated successfully.
func (s *serverImpl) ProcessShutdown(ctx *Context) (interface{}, error) {
	// TODO: Implement proper shutdown handling
	go func() {
		s.logger.Info("shutdown requested, will exit soon")
		// Give time for the response to be sent before actually shutting down
		time.Sleep(100 * time.Millisecond)
		// TODO: Implement clean shutdown
	}()
	return map[string]interface{}{"success": true}, nil
}

// Run starts the server and blocks until it exits.
//
// This method initializes the server's transport, sets up message handling,
// and begins processing client requests. It blocks until an error occurs or
// the server is explicitly stopped.
//
// Run returns an error if the server fails to start or encounters a fatal error
// during operation. Common error scenarios include transport initialization failure
// or missing transport configuration.
//
// Example:
//
//	server := server.NewServer("my-service").AsStdio()
//
//	// Add tools, resources, etc.
//	server.Tool("add", "Add two numbers", addHandler)
//
//	// Start the server (this will block until exit)
//	if err := server.Run(); err != nil {
//	    log.Fatalf("Server error: %v", err)
//	}
func (s *serverImpl) Run() error {
	s.mu.RLock()
	t := s.transport
	s.mu.RUnlock()

	if t == nil {
		return fmt.Errorf("no transport configured, use AsStdio(), AsWebsocket(), AsSSE(), or AsHTTP()")
	}

	// Initialize the request tracker
	s.mu.Lock()
	s.requestTracker = newRequestTracker()
	s.mu.Unlock()

	// Set the message handler using the non-exported handleMessage method
	t.SetMessageHandler(s.handleMessage)

	// Initialize the transport
	if err := t.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize transport: %w", err)
	}

	// Start the transport
	if err := t.Start(); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	s.logger.Info("server started", "name", s.name)

	// Block until the transport is done
	// TODO: Implement proper shutdown handling
	select {}
}

// GetServer returns the underlying server implementation.
//
// This method provides access to the concrete serverImpl instance, which can be
// used for advanced configuration and testing. In most cases, interacting with
// the Server interface is preferred.
//
// Example:
//
//	// Access the underlying implementation for testing or advanced configuration
//	impl := server.GetServer()
//	tools := impl.GetTools()
func (s *serverImpl) GetServer() *serverImpl {
	return s
}

// sendNotification sends a notification message to the client.
//
// Notifications are one-way messages from the server to the client that do not
// require a response. They are used for events like resource changes, tool list
// updates, and other asynchronous events.
//
// The method parameter specifies the notification type (e.g., "notifications/tools/list_changed").
// The params parameter contains any additional data to include with the notification.
//
// If the notification cannot be sent, an error is logged but not returned to the caller.
func (s *serverImpl) sendNotification(method string, params interface{}) {
	if s.transport == nil {
		return
	}

	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}

	if params != nil {
		notification["params"] = params
	}

	// Convert to JSON
	message, err := json.Marshal(notification)
	if err != nil {
		s.logger.Error("failed to marshal notification", "error", err)
		return
	}

	// Send the notification
	if err := s.transport.Send(message); err != nil {
		s.logger.Error("failed to send notification", "error", err)
	}
}
