package test

import (
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestGRPCTransport(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Create a new server
	s := server.NewServer("test-server")

	// Get access to the server implementation
	sImpl := s.GetServer()
	if sImpl == nil {
		t.Fatal("Failed to get server implementation")
	}

	// Configure with gRPC transport on a random port
	sImpl.AsGRPC(":0")

	// Add a test tool
	s = s.Tool("echo", "Echo test", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
		return args, nil
	})

	// This is a minimal test that verifies we can configure the server with gRPC
	// A full integration test would involve starting the server and connecting with a client
}
