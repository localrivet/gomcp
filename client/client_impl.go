package client

import (
	"sync"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// clientImpl implements the Client interface
type clientImpl struct {
	config          ClientConfig
	transport       ClientTransport
	protocolHandler ProtocolHandler

	serverInfo         protocol.Implementation
	serverCapabilities protocol.ServerCapabilities
	negotiatedVersion  string

	connectionState int // 0=disconnected, 1=connecting, 2=connected
	connectMu       sync.RWMutex

	// Notification handlers
	notificationHandlers   map[string][]NotificationHandler
	progressHandlers       []ProgressHandler
	resourceUpdateHandlers map[string][]ResourceUpdateHandler
	logHandlers            []LogHandler
	connectionHandlers     []ConnectionStatusHandler

	// Request tracking
	pendingRequests   map[string]chan *protocol.JSONRPCResponse
	pendingRequestsMu sync.RWMutex

	// Shutdown signaling
	done chan struct{}
}

// NewClient creates a new client with automatic transport detection
func NewClient(nameOrURL string, options ...ClientOption) (Client, error) {
	// Default logger if none provided in options
	logger := logx.NewDefaultLogger()

	// Apply options to get any configured logger
	config := &ClientConfig{
		Name:                     nameOrURL,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create a client using the factories based on URL format
	return CreateClient(nameOrURL, config.Logger, options...)
}

// NewSSEClient creates a client that connects via HTTP/SSE
func NewSSEClient(name, baseURL, basePath string, options ...ClientOption) (Client, error) {
	// Default logger if none provided in options
	logger := logx.NewDefaultLogger()

	// Apply options to get any configured logger
	config := &ClientConfig{
		Name:                     name,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create client with SSE transport
	return CreateSSEClient(baseURL, basePath, config.Logger, options...)
}

// NewWebSocketClient creates a client that connects via WebSocket
func NewWebSocketClient(name, serverURL string, options ...ClientOption) (Client, error) {
	// Default logger if none provided in options
	logger := logx.NewDefaultLogger()

	// Apply options to get any configured logger
	config := &ClientConfig{
		Name:                     name,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create WebSocket client
	return CreateWebSocketClient(serverURL, config.Logger, options...)
}

// NewStdioClient creates a client that connects via stdio
func NewStdioClient(name string, options ...ClientOption) (Client, error) {
	// Default logger if none provided in options
	logger := logx.NewDefaultLogger()

	// Apply options to get any configured logger
	config := &ClientConfig{
		Name:                     name,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create a Stdio client
	return CreateStdioClient(name, config.Logger, options...)
}

// NewInMemoryClient creates a client that connects to an in-memory server
func NewInMemoryClient(name string, server interface{}, options ...ClientOption) (Client, error) {
	// Default logger if none provided in options
	logger := logx.NewDefaultLogger()

	// Apply options to get any configured logger
	config := &ClientConfig{
		Name:                     name,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create an in-memory client
	return CreateInMemoryClient(server, config.Logger, options...)
}
