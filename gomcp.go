// Package gomcp provides a convenient entry point to the Go implementation
// of the Model Context Protocol (MCP).
//
// This package provides simple fluent entry points for creating servers and clients.
// For more specialized functionality, import the specific subpackages directly:
// - github.com/localrivet/gomcp/server
// - github.com/localrivet/gomcp/client
// - github.com/localrivet/gomcp/protocol
// - github.com/localrivet/gomcp/transport/*
package gomcp

import (
	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

// Server creates a new MCP server with the given name.
// This is a convenience function that wraps server.NewServer.
func NewServer(name string) *server.Server {
	return server.NewServer(name)
}

// Client creates a new MCP client with the given name/URL.
// This uses automatic transport detection based on the format of nameOrURL.
// This is a convenience function that wraps client.NewClient.
func NewClient(nameOrURL string, opts ...client.ClientOption) (client.Client, error) {
	return client.NewClient(nameOrURL, opts...)
}
