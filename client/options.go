// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"log/slog"
	"time"

	"github.com/localrivet/gomcp/mcp"
)

// Option is a client configuration option.
type Option func(*clientImpl)

// WithLogger sets the client's logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *clientImpl) {
		c.logger = logger
	}
}

// WithTransport sets the client's transport.
func WithTransport(transport Transport) Option {
	return func(c *clientImpl) {
		c.transport = transport
	}
}

// WithVersionDetector sets the client's version detector.
func WithVersionDetector(detector *mcp.VersionDetector) Option {
	return func(c *clientImpl) {
		c.versionDetector = detector
	}
}

// WithRequestTimeout sets the client's request timeout.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *clientImpl) {
		c.requestTimeout = timeout
		if c.transport != nil {
			c.transport.SetRequestTimeout(timeout)
		}
	}
}

// WithConnectionTimeout sets the client's connection timeout.
func WithConnectionTimeout(timeout time.Duration) Option {
	return func(c *clientImpl) {
		c.connectionTimeout = timeout
		if c.transport != nil {
			c.transport.SetConnectionTimeout(timeout)
		}
	}
}

// WithRoots sets the initial roots for the client.
func WithRoots(roots []Root) Option {
	return func(c *clientImpl) {
		// Add each root via the actor pattern after client is fully initialized
		go func() {
			for _, root := range roots {
				_ = c.AddRoot(root.URI, root.Name)
			}
		}()
		// Enable roots capability if roots are provided
		c.capabilities.Roots.ListChanged = true
	}
}

// WithRootsCapability enables or disables the roots capability.
func WithRootsCapability(enabled bool, listChanged bool) Option {
	return func(c *clientImpl) {
		if enabled {
			c.capabilities.Roots.ListChanged = listChanged
		} else {
			// Clear roots capability
			c.capabilities.Roots = RootsCapability{}
		}
	}
}

// WithSamplingCapability enables or disables the sampling capability.
func WithSamplingCapability(enabled bool, config map[string]interface{}) Option {
	return func(c *clientImpl) {
		if enabled && config != nil {
			c.capabilities.Sampling = config
		} else if enabled {
			c.capabilities.Sampling = map[string]interface{}{}
		} else {
			c.capabilities.Sampling = nil
		}
	}
}

// WithExperimentalCapability adds an experimental capability.
func WithExperimentalCapability(name string, config interface{}) Option {
	return func(c *clientImpl) {
		if c.capabilities.Experimental == nil {
			c.capabilities.Experimental = make(map[string]interface{})
		}
		c.capabilities.Experimental[name] = config
	}
}

// WithProtocolVersion sets a specific protocol version for the client to use.
// This bypasses the normal negotiation process and forces the client to use this version.
// This is useful for testing or when you know exactly which version the server expects.
func WithProtocolVersion(version string) Option {
	return func(c *clientImpl) {
		c.negotiatedVersion = version
	}
}

// WithOldestProtocolVersion sets the client to use the oldest supported protocol version.
// This is useful for maximum compatibility with servers that might not support newer versions.
func WithOldestProtocolVersion() Option {
	return func(c *clientImpl) {
		// Get the last element in the supported versions slice, which is the oldest
		if len(mcp.SupportedVersions) > 0 {
			c.negotiatedVersion = mcp.SupportedVersions[len(mcp.SupportedVersions)-1]
		}
	}
}

// WithProtocolNegotiation enables or disables protocol version negotiation.
func WithProtocolNegotiation(enabled bool) Option {
	return func(c *clientImpl) {
		// Store protocol negotiation setting in the client
		// This would be used during connection establishment
		c.mu.Lock()
		defer c.mu.Unlock()

		// Store the setting directly in the client for use during initialization
		if c.capabilities.Experimental == nil {
			c.capabilities.Experimental = make(map[string]interface{})
		}

		c.capabilities.Experimental["protocol_negotiation"] = enabled
	}
}

// ServerConfigOption configures server registry behavior
type ServerConfigOption func(*serverConfigParams)

type serverConfigParams struct {
	registryLogger *slog.Logger
}

// WithServerRegistryLogger sets a logger for the server registry.
// Use this when you want logging from the server management, but make sure the logger doesn't write to
// stdout/stderr if using stdio transport to avoid interfering with the MCP communication.
func WithServerRegistryLogger(logger *slog.Logger) ServerConfigOption {
	return func(p *serverConfigParams) {
		p.registryLogger = logger
	}
}

// WithServerConfig loads server configurations from a file and connects to a specific named server.
// This is used to integrate with the server registry system to automatically manage server processes.
// If the server requires starting a new process, it will be launched and managed by the registry.
// When the client is closed, the associated server process will be terminated if it was launched by this option.
func WithServerConfig(configPath string, serverName string, opts ...ServerConfigOption) Option {
	return func(c *clientImpl) {
		// Process options
		params := &serverConfigParams{}
		for _, opt := range opts {
			opt(params)
		}

		// Create a new server registry with options
		var registryOpts []ServerRegistryOption
		if params.registryLogger != nil {
			registryOpts = append(registryOpts, WithRegistryLogger(params.registryLogger))
		}
		registry := NewServerRegistry(registryOpts...)

		// Load the config
		if err := registry.LoadConfig(configPath); err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to load server config", "path", configPath, "error", err)
			}
			return
		}

		// Get the client for the specified server
		client, err := registry.GetClient(serverName)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to get client from registry", "server", serverName, "error", err)
			}
			return
		}

		// Copy the internal transport from the registry's client to our client
		clientImpl, ok := client.(*clientImpl)
		if ok && clientImpl.transport != nil {
			c.transport = clientImpl.transport

			// Store the registry in the client for cleanup during Close()
			c.serverRegistry = registry
			c.serverName = serverName
		} else if c.logger != nil {
			c.logger.Error("Failed to extract transport from registry client", "server", serverName)
		}
	}
}

// WithServers provides direct server configurations to the client.
// This is similar to WithServerConfig but accepts an in-memory configuration
// instead of loading from a file.
func WithServers(config ServerConfig, serverName string, opts ...ServerConfigOption) Option {
	return func(c *clientImpl) {
		// Process options
		params := &serverConfigParams{}
		for _, opt := range opts {
			opt(params)
		}

		// Create a new server registry with options
		var registryOpts []ServerRegistryOption
		if params.registryLogger != nil {
			registryOpts = append(registryOpts, WithRegistryLogger(params.registryLogger))
		}
		registry := NewServerRegistry(registryOpts...)

		// Apply the config directly
		if err := registry.ApplyConfig(config); err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to apply server config", "error", err)
			}
			return
		}

		// Get the client for the specified server
		client, err := registry.GetClient(serverName)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to get client from registry", "server", serverName, "error", err)
			}
			return
		}

		// Copy the internal transport from the registry's client to our client
		clientImpl, ok := client.(*clientImpl)
		if ok && clientImpl.transport != nil {
			c.transport = clientImpl.transport

			// Store the registry in the client for cleanup during Close()
			c.serverRegistry = registry
			c.serverName = serverName
		} else if c.logger != nil {
			c.logger.Error("Failed to extract transport from registry client", "server", serverName)
		}
	}
}

// WithServersFromConfig provides server configurations from a loaded config to the client.
// This allows connecting to multiple servers from a configuration file.
// The first serverName in the list will be used as the primary transport.
func WithServersFromConfig(config *ServerConfig, serverNames ...string) Option {
	return func(c *clientImpl) {
		if len(serverNames) == 0 {
			if c.logger != nil {
				c.logger.Error("No server names provided to WithServersFromConfig")
			}
			return
		}

		// Create a new server registry
		registry := NewServerRegistry()

		// Apply the config directly
		if err := registry.ApplyConfig(*config); err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to apply server config", "error", err)
			}
			return
		}

		// Use the first server as the primary transport
		primaryServerName := serverNames[0]
		client, err := registry.GetClient(primaryServerName)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("Failed to get primary client from registry", "server", primaryServerName, "error", err)
			}
			return
		}

		// Copy the internal transport from the registry's client to our client
		clientImpl, ok := client.(*clientImpl)
		if ok && clientImpl.transport != nil {
			c.transport = clientImpl.transport

			// Store the registry in the client for cleanup during Close()
			c.serverRegistry = registry
			c.serverName = primaryServerName
		} else if c.logger != nil {
			c.logger.Error("Failed to extract transport from registry client", "server", primaryServerName)
		}

		// Store references to all requested servers for access via the registry
		// Additional servers can be accessed through the registry stored in the client
	}
}
