package client

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/localrivet/gomcp/transport/sse" // Import SSE transport
	// Import types for TransportOptions
)

// NewSSEClient creates a new MCP client using the Streamable HTTP transport (SSE + POST).
// The baseURL should include the scheme and host (e.g., "http://localhost:8080").
// The basePath argument defines the single path for the MCP endpoint (e.g., "/mcp").
func NewSSEClient(clientName string, baseURL string, basePath string, opts ClientOptions) (*Client, error) {
	// Normalize base URL (remove trailing slash)
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Normalize base path (ensure leading slash, remove trailing)
	if basePath != "" {
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		basePath = strings.TrimSuffix(basePath, "/")
	}
	// If basePath is empty, the endpoint is just the baseURL

	// Parse and validate the base URL
	_, err := url.Parse(baseURL) // Validate baseURL format
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Create the SSE Transport using the updated options
	transportOpts := sse.SSETransportOptions{
		BaseURL:  baseURL,  // Pass the normalized base URL
		BasePath: basePath, // Pass the normalized base path for the single MCP endpoint
		Logger:   opts.Logger,
	}

	// If a preferred protocol version is specified in client options, use it for the transport
	// Note: If not specified, transport will use the default (2024-11-05)
	if opts.PreferredProtocolVersion != nil {
		transportOpts.ProtocolVersion = *opts.PreferredProtocolVersion
	}

	sseTransport, err := sse.NewSSETransport(transportOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE transport: %w", err)
	}

	// Assign the created transport to the main client options
	opts.Transport = sseTransport

	// Create the generic client with the SSE transport
	return NewClient(clientName, opts)
}
