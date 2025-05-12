package client

import (
	"net/url"
	"strings"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// CreateClient creates a new client with automatic transport detection
func CreateClient(nameOrURL string, logger logx.Logger, options ...ClientOption) (Client, error) {
	// Apply options first to get configuration
	config := &ClientConfig{
		Name:                     nameOrURL,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// If the name looks like a URL, try to detect the transport type
	if strings.HasPrefix(nameOrURL, "http://") || strings.HasPrefix(nameOrURL, "https://") {
		parsedURL, err := url.Parse(nameOrURL)
		if err != nil {
			return nil, NewClientError("invalid URL", 0, err)
		}

		// Default to SSE transport for HTTP URLs
		return CreateSSEClient(parsedURL.String(), "/mcp", logger, options...)
	}

	// Otherwise, assume it's a command name for stdio
	return CreateStdioClient(nameOrURL, logger, options...)
}

// CreateSSEClient creates a client that connects via HTTP/SSE
func CreateSSEClient(baseURL, basePath string, logger logx.Logger, options ...ClientOption) (Client, error) {
	// Apply client options
	config := &ClientConfig{
		Name:                     "SSEClient",
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create transport options from client options
	transportOpts := []TransportOption{}

	// Apply authentication if provided
	if config.AuthProvider != nil {
		transportOpts = append(transportOpts, WithTransportAuth(config.AuthProvider))
	}

	// Apply retry strategy if provided
	if config.RetryStrategy != nil {
		transportOpts = append(transportOpts, WithTransportRetryStrategy(config.RetryStrategy))
	}

	// Apply timeout if provided
	if config.DefaultTimeout > 0 {
		transportOpts = append(transportOpts, WithRequestTimeout(config.DefaultTimeout))
	}

	// Create the transport
	transport, err := NewSSETransport(baseURL, basePath, logger, transportOpts...)
	if err != nil {
		return nil, err
	}

	// Create the client with the transport
	return newClientWithTransport(config, transport)
}

// CreateWebSocketClient creates a client that connects via WebSocket
func CreateWebSocketClient(serverURL string, logger logx.Logger, options ...ClientOption) (Client, error) {
	// Apply client options
	config := &ClientConfig{
		Name:                     "WebSocketClient",
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Parse the URL to extract path (basePath) and determine if we need to add /mcp path
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, NewClientError("invalid URL", 0, err)
	}

	// Default path to /mcp if none provided
	basePath := parsedURL.Path
	if basePath == "" || basePath == "/" {
		basePath = "/mcp"
	}

	// Create transport options from client options
	transportOpts := []TransportOption{}

	// Apply authentication if provided
	if config.AuthProvider != nil {
		transportOpts = append(transportOpts, WithTransportAuth(config.AuthProvider))
	}

	// Apply retry strategy if provided
	if config.RetryStrategy != nil {
		transportOpts = append(transportOpts, WithTransportRetryStrategy(config.RetryStrategy))
	}

	// Apply timeout if provided
	if config.DefaultTimeout > 0 {
		transportOpts = append(transportOpts, WithRequestTimeout(config.DefaultTimeout))
	}

	// Create the transport
	transport, err := NewWebSocketTransport(serverURL, basePath, logger, transportOpts...)
	if err != nil {
		return nil, err
	}

	// Create the client with the transport
	return newClientWithTransport(config, transport)
}

// CreateStdioClient creates a client that connects via stdio
func CreateStdioClient(name string, logger logx.Logger, options ...ClientOption) (Client, error) {
	// Apply client options
	config := &ClientConfig{
		Name:                     "StdioClient-" + name,
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Parse the command string to extract the program and arguments
	args := strings.Fields(name)
	if len(args) == 0 {
		return nil, NewClientError("invalid command", 0, nil)
	}

	command := args[0]
	cmdArgs := args[1:]

	// Create transport options from client options
	transportOpts := []TransportOption{}

	// Apply timeout if provided
	if config.DefaultTimeout > 0 {
		transportOpts = append(transportOpts, WithRequestTimeout(config.DefaultTimeout))
	}

	// Create the transport
	transport, err := NewStdioTransport(command, cmdArgs, logger, transportOpts...)
	if err != nil {
		return nil, err
	}

	// Create the client with the transport
	return newClientWithTransport(config, transport)
}

// CreateInMemoryClient creates a client that connects to an in-memory server
func CreateInMemoryClient(server interface{}, logger logx.Logger, options ...ClientOption) (Client, error) {
	// Apply client options
	config := &ClientConfig{
		Name:                     "InMemoryClient",
		Logger:                   logger,
		PreferredProtocolVersion: LatestProtocolVersion,
	}

	for _, option := range options {
		option(config)
	}

	// Create transport options from client options
	transportOpts := []TransportOption{}

	// Apply timeout if provided
	if config.DefaultTimeout > 0 {
		transportOpts = append(transportOpts, WithRequestTimeout(config.DefaultTimeout))
	}

	// Create a server handler based on the server parameter
	var serverHandler ServerRequestHandler

	// Handle different types of server parameters
	switch s := server.(type) {
	case ServerRequestHandler:
		// If directly passed a ServerRequestHandler
		serverHandler = s
	case []protocol.Tool:
		// If passed a list of tools
		serverHandler = DefaultServerHandler(s, nil, nil)
	case struct {
		Tools     []protocol.Tool
		Resources []protocol.Resource
		Prompts   []protocol.Prompt
	}:
		// If passed a struct with tools, resources, and prompts
		serverHandler = DefaultServerHandler(s.Tools, s.Resources, s.Prompts)
	default:
		// Default empty server
		serverHandler = DefaultServerHandler(nil, nil, nil)
	}

	// Create the transport
	transport, err := NewInMemoryTransport(logger, serverHandler, transportOpts...)
	if err != nil {
		return nil, err
	}

	// Create the client with the transport
	return newClientWithTransport(config, transport)
}

// newClientWithTransport creates a new client with the given transport
func newClientWithTransport(config *ClientConfig, transport ClientTransport) (Client, error) {
	client := &clientImpl{
		config:                 *config,
		transport:              transport,
		serverInfo:             protocol.Implementation{},
		serverCapabilities:     protocol.ServerCapabilities{},
		connectionState:        connectionStateDisconnected,
		notificationHandlers:   make(map[string][]NotificationHandler),
		progressHandlers:       []ProgressHandler{},
		resourceUpdateHandlers: make(map[string][]ResourceUpdateHandler),
		logHandlers:            []LogHandler{},
		connectionHandlers:     []ConnectionStatusHandler{},
		pendingRequests:        make(map[string]chan *protocol.JSONRPCResponse),
		done:                   make(chan struct{}),
	}

	// Set up notification handler on the transport
	transport.SetNotificationHandler(client.handleNotification)

	// Create protocol handler based on preferred version
	client.protocolHandler = newProtocolHandler(config.PreferredProtocolVersion)

	return client, nil
}
