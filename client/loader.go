package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ConfigProcessor is a function that can modify or extend a McpConfig after loading
type ConfigProcessor func(*McpConfig) error

// McpConfig is the top-level configuration structure
type McpConfig struct {
	McpServers map[string]McpServer `json:"mcpServers"`
	// Custom properties for extension
	CustomProperties map[string]interface{} `json:"customProperties,omitempty"`
}

// McpServer defines a server configuration
type McpServer struct {
	// Connection can be established either via a command or URL
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`

	// Optional server identity
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// Transport specific options
	Transport string `json:"transport,omitempty"` // Can be "websocket", "sse", "stdio", "auto"
	BasePath  string `json:"basePath,omitempty"`  // For URL-based transports
	Timeout   int    `json:"timeout,omitempty"`   // Connection timeout in seconds

	// Allow extensions by embedding arbitrary JSON properties
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// LoadFromFile loads an MCP configuration from a file path
// The optional processor callback can be used to customize the loaded config
func LoadFromFile(path string, processor ConfigProcessor) (*McpConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return LoadFromJSON(data, processor)
}

// LoadFromJSON parses MCP configuration from JSON data
// The optional processor callback can be used to customize the loaded config
func LoadFromJSON(data []byte, processor ConfigProcessor) (*McpConfig, error) {
	var config McpConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply processor if provided
	if processor != nil {
		if err := processor(&config); err != nil {
			return nil, fmt.Errorf("error in config processor: %w", err)
		}
	}

	return &config, nil
}

// LoadDefaultConfig looks for and loads the default configuration file
// (~/.cursor/mcp.json)
// The optional processor callback can be used to customize the loaded config
func LoadDefaultConfig(processor ConfigProcessor) (*McpConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".cursor", "mcp.json")
	return LoadFromFile(configPath, processor)
}

// GetServer retrieves a server configuration by name
func (c *McpConfig) GetServer(name string) (McpServer, error) {
	server, exists := c.McpServers[name]
	if !exists {
		return McpServer{}, fmt.Errorf("server '%s' not found in configuration", name)
	}

	return server, nil
}

// AddServer adds a server to the configuration
func (c *McpConfig) AddServer(name string, server McpServer) {
	if c.McpServers == nil {
		c.McpServers = make(map[string]McpServer)
	}
	c.McpServers[name] = server
}

// RemoveServer removes a server from the configuration
func (c *McpConfig) RemoveServer(name string) bool {
	if c.McpServers == nil {
		return false
	}

	if _, exists := c.McpServers[name]; !exists {
		return false
	}

	delete(c.McpServers, name)
	return true
}

// NewClientFromServer creates a new MCP client from a server configuration
func NewClientFromServer(server McpServer, options ...ClientOption) (Client, error) {
	// Apply server-specific configuration options
	serverOptions := append([]ClientOption{}, options...)

	// Use server name from config if provided
	serverName := "mcp-server"
	if server.Name != "" {
		serverName = server.Name
	}

	// If transport is explicitly set, use that
	if server.Transport != "" {
		switch strings.ToLower(server.Transport) {
		case "websocket", "ws":
			if server.URL == "" {
				return nil, fmt.Errorf("websocket transport requires a URL")
			}
			return NewWebSocketClient(serverName, server.URL, serverOptions...)

		case "sse", "http":
			if server.URL == "" {
				return nil, fmt.Errorf("SSE/HTTP transport requires a URL")
			}
			// Use basePath from config if provided, otherwise default to "/"
			basePath := "/"
			if server.BasePath != "" {
				basePath = server.BasePath
			}
			return NewSSEClient(serverName, server.URL, basePath, serverOptions...)

		case "stdio":
			if server.Command == "" {
				return nil, fmt.Errorf("stdio transport requires a command")
			}
			return NewStdioClient(server.Command, append(serverOptions, WithStdioArgs(server.Args))...)

		case "auto":
			// Fall through to auto-detection logic below

		default:
			return nil, fmt.Errorf("unknown transport type: %s", server.Transport)
		}
	}

	// URL is provided but no explicit transport type
	if server.URL != "" {
		// Parse the URL to potentially extract the basePath
		basePath := "/"
		var baseURL string

		if server.BasePath != "" {
			basePath = server.BasePath
		} else {
			// Extract the path from the URL if not empty
			if parsedURL, err := url.Parse(server.URL); err == nil && parsedURL.Path != "" && parsedURL.Path != "/" {
				// Store the original URL without the path
				baseURL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
				basePath = parsedURL.Path

				// Ensure basePath has a trailing slash for SSE connections
				if !strings.HasSuffix(basePath, "/") {
					basePath = basePath + "/"
				}

				// Update the URL to use just the scheme and host
				server.URL = baseURL
			}
		}

		// Determine the transport type based on URL
		if strings.HasPrefix(server.URL, "ws://") || strings.HasPrefix(server.URL, "wss://") {
			// For explicit WebSocket URLs
			return NewWebSocketClient(serverName, server.URL, serverOptions...)
		} else if strings.HasPrefix(server.URL, "http://") || strings.HasPrefix(server.URL, "https://") {
			// For HTTP URLs, use SSE transport
			// Redirect following is handled automatically by the SSE transport
			return NewSSEClient(serverName, server.URL, basePath, serverOptions...)
		} else {
			// For any other URL types, let the system auto-detect
			return NewClient(server.URL, serverOptions...)
		}
	}

	// Command is provided but no URL or explicit transport
	if server.Command != "" {
		return NewStdioClient(
			server.Command,
			append(serverOptions, WithStdioArgs(server.Args))...,
		)
	}

	// Neither URL nor command is provided
	return nil, fmt.Errorf("server configuration must specify either URL or Command")
}

// NewClientFromConfig creates a new MCP client from a named server in the config
func NewClientFromConfig(config *McpConfig, serverName string, options ...ClientOption) (Client, error) {
	server, err := config.GetServer(serverName)
	if err != nil {
		return nil, err
	}

	return NewClientFromServer(server, options...)
}

// NewClientFromJSON creates a client directly from JSON configuration
func NewClientFromJSON(jsonData []byte, serverName string, processor ConfigProcessor, options ...ClientOption) (Client, error) {
	config, err := LoadFromJSON(jsonData, processor)
	if err != nil {
		return nil, err
	}

	return NewClientFromConfig(config, serverName, options...)
}

// Quick helper to create a client from the default config and server name
func NewClientFromServerName(serverName string, processor ConfigProcessor, options ...ClientOption) (Client, error) {
	config, err := LoadDefaultConfig(processor)
	if err != nil {
		return nil, err
	}

	return NewClientFromConfig(config, serverName, options...)
}

// WithStdioArgs is a helper option for stdio clients
func WithStdioArgs(args []string) ClientOption {
	return func(config *ClientConfig) {
		// Create the Custom map if it doesn't exist
		if config.TransportOptions.Custom == nil {
			config.TransportOptions.Custom = make(map[string]interface{})
		}
		// Store the args in the custom options map
		config.TransportOptions.Custom["stdio_args"] = args
	}
}

// CreateWebSocketServerConfig creates a McpServer configuration for a WebSocket server
func CreateWebSocketServerConfig(name, url string, basePath string) McpServer {
	return McpServer{
		Name:        name,
		URL:         url,
		BasePath:    basePath,
		Transport:   "websocket",
		Description: fmt.Sprintf("WebSocket server at %s", url),
	}
}

// CreateSSEServerConfig creates a McpServer configuration for an SSE/HTTP server
func CreateSSEServerConfig(name, url string, basePath string) McpServer {
	return McpServer{
		Name:        name,
		URL:         url,
		BasePath:    basePath,
		Transport:   "sse",
		Description: fmt.Sprintf("SSE/HTTP server at %s", url),
	}
}

// CreateStdioServerConfig creates a McpServer configuration for a stdio server
func CreateStdioServerConfig(name, command string, args []string) McpServer {
	return McpServer{
		Name:        name,
		Command:     command,
		Args:        args,
		Transport:   "stdio",
		Description: fmt.Sprintf("Stdio server running %s", command),
	}
}
