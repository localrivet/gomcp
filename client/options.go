package client

import (
	"time"

	"github.com/localrivet/gomcp/logx"
)

// WithPreferredProtocolVersion sets the preferred protocol version for the client
func WithPreferredProtocolVersion(version string) ClientOption {
	return func(config *ClientConfig) {
		config.PreferredProtocolVersion = version
	}
}

// WithLogger sets the logger for the client
func WithLogger(logger logx.Logger) ClientOption {
	return func(config *ClientConfig) {
		config.Logger = logger
	}
}

// WithTimeout sets the default timeout for requests
func WithTimeout(timeout time.Duration) ClientOption {
	return func(config *ClientConfig) {
		config.DefaultTimeout = timeout
	}
}

// WithRetryStrategy sets the retry strategy for the client
func WithRetryStrategy(strategy BackoffStrategy) ClientOption {
	return func(config *ClientConfig) {
		config.RetryStrategy = strategy
	}
}

// WithMiddleware adds middleware to the client
func WithMiddleware(middleware ClientMiddleware) ClientOption {
	return func(config *ClientConfig) {
		config.Middleware = append(config.Middleware, middleware)
	}
}

// WithAuth sets the auth provider for the client
func WithAuth(auth AuthProvider) ClientOption {
	return func(config *ClientConfig) {
		config.AuthProvider = auth
	}
}

// WithFollowRedirects configures whether to follow HTTP redirects for SSE connections.
// This is especially useful when the server redirects from the base path to the SSE endpoint.
func WithFollowRedirects(follow bool) ClientOption {
	return func(config *ClientConfig) {
		// Create the Custom map if it doesn't exist
		if config.TransportOptions.Custom == nil {
			config.TransportOptions.Custom = make(map[string]interface{})
		}
		config.TransportOptions.Custom["follow_redirects"] = follow
	}
}
