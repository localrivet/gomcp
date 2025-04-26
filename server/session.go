// Package server provides the MCP server implementation.
package server

import (
	"context"  // Added for SSEContextFunc
	"net/http" // Added for SSEContextFunc
	// "github.com/localrivet/gomcp/types" // Import types if needed elsewhere in this file
)

// ClientSession interface moved to types/session.go

// SSEContextFunc is a function type used by the SSEServer to allow
// customization of the context passed to the core MCPServer's HandleMessage method,
// based on the incoming HTTP request for client->server messages.
// This allows injecting values from HTTP headers (like auth tokens) into the context.
type SSEContextFunc func(ctx context.Context, r *http.Request) context.Context

// Note: The ClientSession interface focuses on server-to-client communication needs
// for asynchronous messages. Client-to-server requests are handled separately
// (e.g., via an HTTP endpoint calling MCPServer.HandleMessage).
