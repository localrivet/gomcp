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
		c.roots = roots
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

// WithSamplingOptimizations enables sampling optimizations for the client.
func WithSamplingOptimizations(opts *SamplingOptimizationOptions) Option {
	if opts == nil {
		opts = DefaultSamplingOptimizationOptions()
	}

	return func(c *clientImpl) {
		// Create optimization components
		cache := NewSamplingCache(opts.CacheCapacity, opts.CacheTTL)

		// Configure size analyzer
		sizeAnalyzer := NewContentSizeAnalyzer()
		if opts.MaxTextBytes > 0 {
			sizeAnalyzer.MaxTextBytes = opts.MaxTextBytes
		}
		if opts.MaxImageBytes > 0 {
			sizeAnalyzer.MaxImageBytes = opts.MaxImageBytes
		}
		if opts.MaxAudioBytes > 0 {
			sizeAnalyzer.MaxAudioBytes = opts.MaxAudioBytes
		}

		// Create metrics tracker
		metrics := NewSamplingPerformanceMetrics()

		// Store these components for access in other methods
		c.samplingCache = cache
		c.sizeAnalyzer = sizeAnalyzer
		c.samplingMetrics = metrics

		// Register an optimized sampling handler wrapper if there's already a base handler
		if c.samplingHandler != nil {
			baseHandler := c.samplingHandler
			c.samplingHandler = wrapSamplingHandlerWithOptimizations(
				baseHandler, cache, sizeAnalyzer, metrics, opts.LogWarnings, c.Version())
		}
	}
}

// WithServerConfig loads server configurations from a file and connects to a specific named server.
// This is used to integrate with the server registry system to automatically manage server processes.
// If the server requires starting a new process, it will be launched and managed by the registry.
// When the client is closed, the associated server process will be terminated if it was launched by this option.
func WithServerConfig(configPath string, serverName string) Option {
	return func(c *clientImpl) {
		// Create a new server registry
		registry := NewServerRegistry()

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
func WithServers(config ServerConfig, serverName string) Option {
	return func(c *clientImpl) {
		// Create a new server registry
		registry := NewServerRegistry()

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
