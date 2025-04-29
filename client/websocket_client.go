package client

import (
	"context"
	"fmt"
	"io" // Needed for Closer check
	"net/url"
	"strings"

	"github.com/gobwas/ws"
	"github.com/localrivet/gomcp/logx" // Import logx
	"github.com/localrivet/gomcp/transport/websocket"
	"github.com/localrivet/gomcp/types"
)

// NewWebSocketClient creates a new MCP client using WebSocket transport.
// The baseURL should include the scheme and host (e.g., "ws://localhost:8080" or "wss://localhost:8080").
// The basePath argument defines the URL prefix for the WebSocket endpoint
// (e.g., "/mcp", resulting in "/mcp/ws").
func NewWebSocketClient(clientName string, baseURL string, basePath string, opts ClientOptions) (*Client, error) {
	// Ensure logger is initialized if not provided
	logger := opts.Logger
	if logger == nil {
		logger = logx.NewDefaultLogger() // Use logx
		opts.Logger = logger             // Assign back to opts
	}

	// Normalize base URL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Normalize base path
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	// Parse and validate the base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Ensure the URL scheme is ws:// or wss://
	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return nil, fmt.Errorf("invalid URL scheme for WebSocket connection: %s (must be ws:// or wss://)", parsedURL.Scheme)
	}

	// Create WebSocket connection
	wsURL := parsedURL.String() + strings.TrimPrefix(basePath, "/") + "ws"
	conn, _, _, err := ws.Dial(context.Background(), wsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to establish WebSocket connection: %w", err)
	}

	// Create WebSocket transport
	transportOpts := types.TransportOptions{
		Logger: logger,
	}
	transport := websocket.NewWebSocketTransport(conn, ws.StateClientSide, transportOpts)

	// Assign the created transport to the main client options
	opts.Transport = transport
	// opts.ServerBaseURL = &wsURL // Removed - No longer part of ClientOptions

	// Create the generic client
	client, err := NewClient(clientName, opts)
	if err != nil {
		// If client creation fails, close the transport
		if transport, ok := opts.Transport.(io.Closer); ok {
			transport.Close()
		}
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
}
