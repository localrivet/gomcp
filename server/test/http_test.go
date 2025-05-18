package test

import (
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestAsHTTP(t *testing.T) {
	s := server.NewServer("test")

	// Configure as HTTP server with dynamic port
	address := ":0"
	s = s.AsHTTP(address)

	// Check that the server was configured properly
	// We can only indirectly test that the HTTP transport was set correctly
	// by verifying the server implements the expected interfaces

	// Test that the server can be converted to AsHTTP
	// (If AsHTTP is already called, this will work)
	httpServer := s.AsHTTP(address)
	if httpServer == nil {
		t.Fatal("AsHTTP returned nil")
	}
}
