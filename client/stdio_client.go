package client

import (
	"fmt"
	"io"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// NewStdioClient creates a new MCP client using stdio transport.
// This is useful for command-line tools and testing.
func NewStdioClient(clientName string, opts ClientOptions) (*Client, error) {
	// Ensure logger is initialized if not provided
	logger := opts.Logger
	if logger == nil {
		logger = logx.NewDefaultLogger() // Use logx
		opts.Logger = logger             // Assign back to opts
	}

	// Create stdio transport if not provided in options
	if opts.Transport == nil {
		// Pass logger to transport constructor
		transportOpts := types.TransportOptions{Logger: logger}
		opts.Transport = stdio.NewStdioTransportWithOptions(transportOpts)
	}

	// Remove context/signal handling and message loop from here.
	// The generic NewClient and its Connect method handle the lifecycle.

	// Create the generic client
	client, err := NewClient(clientName, opts)
	if err != nil {
		// If client creation fails, close the transport if we created it
		if transport, ok := opts.Transport.(io.Closer); ok {
			transport.Close()
		}
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
}
