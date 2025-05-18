package test

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/udp"
)

// TestUDPTransport tests the basic configuration of a server with UDP transport
func TestUDPTransport(t *testing.T) {
	// Create a new server instance
	s := server.NewServer("test-server")

	// Configure it to use UDP
	s = s.AsUDP(":0") // Use port 0 to let the system assign a free port

	// Verify the server was configured successfully
	if s == nil {
		t.Fatal("Server is nil after configuration")
	}

	// Use options as well
	s = server.NewServer("test-server-with-options")
	s = s.AsUDP(":0",
		udp.WithMaxPacketSize(2048),
		udp.WithReadTimeout(2*time.Second),
		udp.WithReliability(true),
	)

	// Verify the server was configured successfully with options
	if s == nil {
		t.Fatal("Server with options is nil after configuration")
	}
}
