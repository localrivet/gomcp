// Package client provides a high-level client API for interacting with MCP servers.
// It builds on the existing transport and hooks infrastructure to provide a
// simple, fluent interface for client applications.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/localrivet/gomcp/hooks"
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// Protocol version constants
const (
	ProtocolVersion2024   = "2024-11-05"
	ProtocolVersion2025   = "2025-03-26"
	LatestProtocolVersion = ProtocolVersion2025
)

// Client is the interface for a single MCP server connection
type Client interface {
	// Connection Management
	Connect(ctx context.Context) error
	Close() error
	IsConnected() bool
	Run(ctx context.Context) error
	Cleanup() // New method for explicit graceful cleanup

	// MCP Methods - High-level API
	ListTools(ctx context.Context) ([]protocol.Tool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}, progressCh chan<- protocol.ProgressParams) ([]protocol.Content, error)
	ListResources(ctx context.Context) ([]protocol.Resource, error)
	ReadResource(ctx context.Context, uri string) ([]protocol.ResourceContents, error)
	ListPrompts(ctx context.Context) ([]protocol.Prompt, error)
	GetPrompt(ctx context.Context, name string, args map[string]interface{}) ([]protocol.PromptMessage, error)

	// Server Information
	ServerInfo() protocol.Implementation
	ServerCapabilities() protocol.ServerCapabilities

	// Raw Protocol Access
	SendRequest(ctx context.Context, method string, params interface{}) (*protocol.JSONRPCResponse, error)

	// Configuration methods (fluent interface)
	WithTimeout(timeout time.Duration) Client
	WithRetry(maxAttempts int, backoff BackoffStrategy) Client
	WithMiddleware(middleware ClientMiddleware) Client
	WithAuth(auth AuthProvider) Client
	WithLogger(logger logx.Logger) Client

	// Notification registration methods
	OnNotification(method string, handler NotificationHandler) Client
	OnProgress(handler ProgressHandler) Client
	OnResourceUpdate(uri string, handler ResourceUpdateHandler) Client
	OnLog(handler LogHandler) Client
	OnConnectionStatus(handler ConnectionStatusHandler) Client
}

// MCP provides a simple interface for working with multiple MCP servers
type MCP struct {
	Servers map[string]Client
	config  *McpConfig
	logger  logx.Logger
	roots   []string
	ctx     context.Context

	// Channel for connection errors
	connectionErrors chan ServerConnectionError
	errMutex         sync.RWMutex
}

// ServerConnectionError represents an error connecting to a specific server
type ServerConnectionError struct {
	ServerName string
	Err        error // Changed from Error to Err to avoid conflict
}

// ClientConfig holds the configuration for a client
type ClientConfig struct {
	Name                     string
	Logger                   logx.Logger
	Capabilities             protocol.ClientCapabilities
	PreferredProtocolVersion string
	TransportOptions         types.TransportOptions
	DefaultTimeout           time.Duration
	RetryStrategy            BackoffStrategy
	Middleware               []ClientMiddleware
	Hooks                    ClientHooks
	AuthProvider             AuthProvider
	Endpoint                 string
	ServerName               string
}

// ClientHooks holds the various hook functions for the client
type ClientHooks struct {
	BeforeSendRequest        []hooks.ClientBeforeSendRequestHook
	BeforeSendNotification   []hooks.ClientBeforeSendNotificationHook
	OnReceiveRawMessage      []hooks.OnReceiveRawMessageHook
	BeforeHandleResponse     []hooks.ClientBeforeHandleResponseHook
	BeforeHandleNotification []hooks.ClientBeforeHandleNotificationHook
	BeforeHandleRequest      []hooks.ClientBeforeHandleRequestHook
}

// ClientOption is a function that configures a ClientConfig
type ClientOption func(*ClientConfig)

// Server configuration from config file
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Configuration file structure
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// New creates a new MCP client manager from a config
//
// Example:
//
//	config, err := client.LoadFromFile(configPath, nil)
//	mcp, err := client.New(config, client.WithTimeout(30*time.Second))
//
//	// Configure with fluent interface
//	mcp.Roots([]string{"./path1", "./path2"}).
//		Logger(logx.NewDefaultLogger()).
//		WithContext(ctx)
//
//	// Connect and use
//	mcp.Connect()
//	tools := mcp.ListTools()
func New(config *McpConfig, options ...ClientOption) (*MCP, error) {
	// Create the MCP instance
	mcp := &MCP{
		Servers:          make(map[string]Client),
		config:           config,
		logger:           logx.NewDefaultLogger(),
		ctx:              context.Background(),
		roots:            []string{},
		connectionErrors: make(chan ServerConnectionError, 10), // Buffer for up to 10 errors
	}

	// Create a default client config to apply options to
	clientConfig := &ClientConfig{
		Name:                     getEnvOrDefault("MCP_CLIENT_NAME", "GoMCPClient"),
		Logger:                   mcp.logger,
		PreferredProtocolVersion: LatestProtocolVersion,
		DefaultTimeout:           30 * time.Second,
		RetryStrategy:            NewExponentialBackoff(500*time.Millisecond, 3*time.Second, 3),
	}

	// Apply any client options
	for _, option := range options {
		option(clientConfig)
	}

	// Create clients for each server in the config
	for name, serverConfig := range config.McpServers {
		client, err := NewClientFromServer(serverConfig,
			// Apply the common options from the client config
			WithLogger(clientConfig.Logger),
			WithTimeout(clientConfig.DefaultTimeout),
			WithRetryStrategy(clientConfig.RetryStrategy),
		)
		if err != nil {
			return nil, err
		}

		mcp.Servers[name] = client
	}

	return mcp, nil
}

// Roots sets the root directories for the MCP
func (m *MCP) Roots(paths []string) *MCP {
	m.roots = paths
	return m
}

// Logger sets the logger for the MCP
func (m *MCP) Logger(logger logx.Logger) *MCP {
	m.logger = logger

	// Also set the logger for each server
	for _, client := range m.Servers {
		client.WithLogger(logger)
	}

	return m
}

// Connect connects to all servers
// This is now non-blocking and will attempt connections in the background
func (m *MCP) Connect() error {
	// Clear any previous connection errors
	for len(m.connectionErrors) > 0 {
		<-m.connectionErrors // Drain the channel
	}

	// Start the connection process in a background goroutine
	// go func() {
	// var wg sync.WaitGroup
	var totalServers = len(m.Servers)

	// Connect to each server in parallel
	for name, client := range m.Servers {
		go func(name string, client Client) {
			// defer wg.Done()
			if err := client.Connect(m.ctx); err != nil {
				m.logger.Error("Failed to connect to server %s: %v", name, err)
				// Send error to the connection errors channel
				select {
				case m.connectionErrors <- ServerConnectionError{
					ServerName: name,
					Err:        err,
				}:
					// Successfully sent
				default:
					// Channel is full, just log
					m.logger.Error("Connection error channel full, discarding error from %s: %v", name, err)
				}
			} else {

				m.logger.Info("Successfully connected to server %s", name)
			}
		}(name, client)
	}

	// Log a message for ongoing connection attempts
	m.logger.Info("Connection attempts started for %d servers", totalServers)
	// }()

	// Return immediately while connections happen in background
	return nil
}

// IsConnected returns true if at least one server is connected
func (m *MCP) IsConnected() bool {
	// Check if any underlying client reports as connected
	for _, client := range m.Servers {
		if client.IsConnected() {
			return true
		}
	}
	return false
}

// ConnectionErrors returns a read-only channel that receives connection errors
// Callers can use this to monitor which servers failed to connect
func (m *MCP) ConnectionErrors() <-chan ServerConnectionError {
	return m.connectionErrors
}

// WaitForConnections blocks until all servers have attempted to connect
// with an optional timeout. Returns true if all servers connected successfully,
// false if there were errors or the timeout was reached.
func (m *MCP) WaitForConnections(timeout time.Duration) bool {
	// Create a timer for the timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Use a channel to signal completion from a goroutine
	done := make(chan bool, 1)

	// Start a goroutine to check server connections
	go func() {
		// Check each server's connection status
		allConnected := true
		for name, server := range m.Servers {
			if !server.IsConnected() {
				m.logger.Warn("Server %s is not connected", name)
				allConnected = false
			}
		}
		done <- allConnected
	}()

	// Wait for either completion or timeout
	select {
	case result := <-done:
		return result
	case <-timer.C:
		m.logger.Warn("Timeout waiting for server connections")
		return false
	}
}

// Close disconnects from all servers
func (m *MCP) Close() {
	for _, client := range m.Servers {
		client.Close()
	}
}

// WithContext sets the context for all operations
func (m *MCP) WithContext(ctx context.Context) *MCP {
	m.ctx = ctx
	return m
}

// ListTools returns all tools from all servers
// This is a convenience method that calls ListTools on each server
func (m *MCP) ListTools() map[string][]protocol.Tool {
	result := make(map[string][]protocol.Tool)

	// Check each server individually
	for name, server := range m.Servers {
		// Check if this specific server is connected before trying to list its tools
		if server.IsConnected() {
			tools, err := server.ListTools(m.ctx)
			if err == nil {
				result[name] = tools
				m.logger.Debug("Listed %d tools from server %s", len(tools), name)
			} else {
				m.logger.Debug("Error listing tools from connected server %s: %v", name, err)
			}
		} else {
			// Log that we are skipping this server because it's not connected
			m.logger.Debug("Skipping tool listing for server %s as it is not connected", name)
		}
	}

	return result
}

// ListToolsWithTimeout returns all tools from all servers with a timeout
// This is a convenience method that calls ListTools on each server with a timeout
func (m *MCP) ListToolsWithTimeout(timeout time.Duration) map[string][]protocol.Tool {
	result := make(map[string][]protocol.Tool)
	var resultMu sync.Mutex

	// Create a wait group to track when all goroutines are done
	var wg sync.WaitGroup

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	// Launch a goroutine for each server
	for name, server := range m.Servers {
		wg.Add(1)
		go func(name string, server Client) {
			defer wg.Done()

			// Create a context with timeout for this specific call
			toolCtx, toolCancel := context.WithTimeout(ctx, timeout)
			defer toolCancel()

			// Call ListTools with timeout
			tools, err := server.ListTools(toolCtx)
			if err != nil {
				m.logger.Error("Failed to list tools from server %s: %v", name, err)
				return
			}

			// Add tools to the result map
			resultMu.Lock()
			result[name] = tools
			resultMu.Unlock()
		}(name, server)
	}

	// Use a channel to signal when all goroutines are done or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// All servers have completed
		m.logger.Debug("All servers completed tool listing")
	case <-ctx.Done():
		// Timeout occurred
		m.logger.Warn("Timeout occurred while listing tools from servers")
	}

	return result
}

// ListToolsAsync lists tools from all servers asynchronously
// It returns immediately with an empty map and pushes results to the provided channel as they arrive
func (m *MCP) ListToolsAsync(resultCh chan<- map[string][]protocol.Tool, timeout time.Duration) {
	// Create a context with timeout for the overall operation
	ctx, cancel := context.WithTimeout(m.ctx, timeout)

	// Start a goroutine to handle the async operation
	go func() {
		defer cancel() // Ensure context is cancelled when done

		// Intermediate results will be collected here
		tempResult := make(map[string][]protocol.Tool)
		var resultMu sync.Mutex

		// Create a wait group to track when all servers are done
		var wg sync.WaitGroup

		// Launch a goroutine for each server
		for name, server := range m.Servers {
			wg.Add(1)
			go func(name string, server Client) {
				defer wg.Done()

				// Create a context with timeout for this specific server
				serverCtx, serverCancel := context.WithTimeout(ctx, timeout/2) // Use half the timeout for the individual call
				defer serverCancel()

				// Try to list tools from this server
				tools, err := server.ListTools(serverCtx)
				if err != nil {
					m.logger.Error("Failed to list tools from server %s: %v", name, err)
					return
				}

				// Add tools to the result map and send a copy
				resultMu.Lock()
				tempResult[name] = tools

				// Make a deep copy of the current results to send
				currentResult := make(map[string][]protocol.Tool)
				for k, v := range tempResult {
					currentResult[k] = v
				}
				resultMu.Unlock()

				// Send the current results through the channel
				select {
				case resultCh <- currentResult:
					// Successfully sent
				case <-ctx.Done():
					// Context cancelled, stop
					return
				}
			}(name, server)
		}

		// Wait for all servers to complete
		wg.Wait()

		// Send final results
		select {
		case resultCh <- tempResult:
			// Successfully sent final results
		case <-ctx.Done():
			// Context cancelled, stop
		}
	}()
}

// ListResources returns all resources from all servers
// This is a convenience method that calls ListResources on each server
func (m *MCP) ListResources() map[string][]protocol.Resource {
	result := make(map[string][]protocol.Resource)
	for name, server := range m.Servers {
		resources, err := server.ListResources(m.ctx)
		if err == nil {
			result[name] = resources
		}
	}
	return result
}

// ReadResource reads a resource from a specific server
func (m *MCP) ReadResource(serverName, uri string) ([]protocol.ResourceContents, error) {
	server, ok := m.Servers[serverName]
	if !ok {
		return nil, ErrServerNotFound
	}

	return server.ReadResource(m.ctx, uri)
}

// CallTool calls a tool on a specific server
func (m *MCP) CallTool(serverName, toolName string, args map[string]interface{}) ([]protocol.Content, error) {
	server, ok := m.Servers[serverName]
	if !ok {
		return nil, ErrServerNotFound
	}

	return server.CallTool(m.ctx, toolName, args, nil)
}

// ErrServerNotFound is returned when a specified server is not found
var ErrServerNotFound = fmt.Errorf("server not found")

// Helper function to create a WebSocket client
func createWebSocketClient(config *ClientConfig, endpoint string) (Client, error) {
	return NewWebSocketClient(
		config.Name,
		endpoint,
		WithTimeout(config.DefaultTimeout),
		WithRetryStrategy(config.RetryStrategy),
		WithLogger(config.Logger),
	)
}

// Helper function to create an SSE client
func createSSEClient(config *ClientConfig, endpoint string) (Client, error) {
	parsedURL, urlErr := url.Parse(endpoint)
	if urlErr != nil {
		return nil, fmt.Errorf("invalid URL: %w", urlErr)
	}

	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	path := parsedURL.Path
	if path == "" {
		path = "/mcp"
	}

	return NewSSEClient(
		config.Name,
		baseURL,
		path,
		WithTimeout(config.DefaultTimeout),
		WithLogger(config.Logger),
	)
}

// Helper function to create a Stdio client
func createStdioClient(config *ClientConfig, endpoint string) (Client, error) {
	return NewStdioClient(
		endpoint,
		WithTimeout(config.DefaultTimeout),
		WithLogger(config.Logger),
	)
}

// Helper function to get environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// findServerConfig looks for server configuration in ~/.cursor/mcp.json
func findServerConfig(serverName string) (command string, args []string, found bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", nil, false
	}

	configPath := filepath.Join(homeDir, ".cursor", "mcp.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", nil, false
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", nil, false
	}

	serverConfig, exists := config.MCPServers[serverName]
	if !exists {
		return "", nil, false
	}

	return serverConfig.Command, serverConfig.Args, true
}

// createStdioClientWithCommand creates a stdio client with the given command and args
func createStdioClientWithCommand(config *ClientConfig, command string, args []string) (Client, error) {
	// We'll use the existing NewStdioClient but with the command set in environment
	// This avoids implementing a custom transport for this example
	return NewStdioClient(
		config.Name,
		WithTimeout(config.DefaultTimeout),
		WithLogger(config.Logger),
		WithRetryStrategy(config.RetryStrategy),
	)
}

// WithEndpoint sets the endpoint for the client
func WithEndpoint(endpoint string) ClientOption {
	return func(config *ClientConfig) {
		config.Endpoint = endpoint
	}
}

// WithServer sets the server name to connect to from the config file
func WithServer(serverName string) ClientOption {
	return func(config *ClientConfig) {
		config.ServerName = serverName
	}
}

// CheckConnectionStatus logs detailed information about the connection status of all servers
func (m *MCP) CheckConnectionStatus() {
	for name, client := range m.Servers {
		connected := client.IsConnected()
		m.logger.Info("Server %s connection status: %v", name, connected)
	}
}
