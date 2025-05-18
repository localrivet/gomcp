// Package test provides utilities for testing the MCP implementation.
package test

import (
	"github.com/localrivet/gomcp/server"
)

// ServerWithHandler is a wrapper for Server that exposes the handleMessage method for testing
// This is kept for backward compatibility with existing tests
type ServerWithHandler struct {
	Server server.Server
}

// HandleMessage exposes the handleMessage method of serverImpl for testing
func (s *ServerWithHandler) HandleMessage(message []byte) ([]byte, error) {
	return server.HandleMessage(s.Server.GetServer(), message)
}

// WrapServer wraps a Server to expose its handleMessage method for testing
func WrapServer(s server.Server) *ServerWithHandler {
	return &ServerWithHandler{Server: s}
}
