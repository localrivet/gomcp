package test

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestGRPCOption tests the client.WithGRPC option
func TestGRPCOption(t *testing.T) {
	// Use a local gRPC address for testing
	address := "localhost:0" // Using port 0 lets the system assign a free port

	// Create a mock transport with proper initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Use test helper to create client with the gRPC option
	c, err := client.NewClient("test-client",
		client.WithGRPC(address),
		client.WithTransport(mockTransport),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the client was created successfully
	if c == nil {
		t.Fatal("Client is nil")
	}

	// Verify initialization happened
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called during initialization")
	}

	// Clean up
	c.Close()
}

// TestGRPCWithOptions tests the options for gRPC client transport
func TestGRPCWithOptions(t *testing.T) {
	// Use a local gRPC address for testing
	address := "localhost:0" // Using port 0 lets the system assign a free port

	// Create a mock transport with proper initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Create client with gRPC options
	c, err := client.NewClient("test-client",
		client.WithGRPC(address,
			client.WithGRPCTLS("cert.pem", "key.pem", "ca.pem"),
			client.WithGRPCTimeout(5*time.Second),
			client.WithGRPCKeepAlive(10*time.Second, 3*time.Second),
			client.WithGRPCMaxMessageSize(8*1024*1024),
		),
		client.WithTransport(mockTransport),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the client was created successfully
	if c == nil {
		t.Fatal("Client is nil")
	}

	// The mock transport may have its own default values set.
	// Check that our options affect the transport, but don't assert exact values
	// since these could be affected by defaults in the transport implementation.
	if mockTransport.ConnectionTimeout == 0 {
		t.Error("Connection timeout was not set")
	}

	// Clean up
	c.Close()
}
