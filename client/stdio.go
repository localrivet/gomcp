package client

import (
	"io"
	"time"
)

// StdioOption is a function that configures a Stdio transport.
// These options allow customizing the behavior of the Stdio client connection.
type StdioOption func(*stdioConfig)

// stdioConfig holds configuration for Stdio transport.
// These settings control the behavior of the Stdio client connection.
type stdioConfig struct {
	in            io.Reader
	out           io.Writer
	appendNewline bool
	timeout       time.Duration
}

// WithStdioInput sets a custom input reader for the Stdio transport.
func WithStdioInput(in io.Reader) StdioOption {
	return func(cfg *stdioConfig) {
		cfg.in = in
	}
}

// WithStdioOutput sets a custom output writer for the Stdio transport.
func WithStdioOutput(out io.Writer) StdioOption {
	return func(cfg *stdioConfig) {
		cfg.out = out
	}
}

// WithStdioNewline configures whether to append a newline character to messages.
func WithStdioNewline(appendNewline bool) StdioOption {
	return func(cfg *stdioConfig) {
		cfg.appendNewline = appendNewline
	}
}

// WithStdioTimeout sets the timeout for Stdio operations.
func WithStdioTimeout(timeout time.Duration) StdioOption {
	return func(cfg *stdioConfig) {
		cfg.timeout = timeout
	}
}

// WithStdio configures the client to use Standard I/O for communication
// with optional configuration options.
//
// This is particularly useful for CLI applications or when communicating with child processes.
//
// Parameters:
// - options: Optional configuration settings (custom I/O, timeout, etc.)
//
// Example:
//
//	client.New(
//	    client.WithStdio(),
//	    // or with options:
//	    client.WithStdio(
//	        client.WithStdioInput(customInput),
//	        client.WithStdioOutput(customOutput),
//	        client.WithStdioNewline(false))
//	)
func WithStdio(options ...StdioOption) Option {
	return func(c *clientImpl) {
		// Create default config
		cfg := &stdioConfig{
			in:            nil, // nil means use os.Stdin
			out:           nil, // nil means use os.Stdout
			appendNewline: true,
			timeout:       30 * time.Second,
		}

		// Apply options
		for _, option := range options {
			option(cfg)
		}

		var transport *StdioTransport

		// Create the appropriate transport based on config
		if cfg.in != nil && cfg.out != nil {
			// Custom I/O
			transport = NewStdioTransportWithIO(cfg.in, cfg.out)
		} else {
			// Default I/O
			transport = NewStdioTransport()
		}

		// Set newline option if needed
		if !cfg.appendNewline {
			transport.transport.SetNewline(false)
		}

		// Set the transport
		c.transport = transport

		// Set timeouts if specified
		if cfg.timeout > 0 {
			c.requestTimeout = cfg.timeout
			c.connectionTimeout = cfg.timeout
			transport.SetRequestTimeout(cfg.timeout)
			transport.SetConnectionTimeout(cfg.timeout)
		}
	}
}
