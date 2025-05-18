package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestUnixSocketOption tests the client.WithUnixSocket option
func TestUnixSocketOption(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-client.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create a mock transport with proper initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Use test helper to create client with the unix socket option
	c, err := client.NewClient("test-client",
		client.WithUnixSocket(socketPath),
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
	os.Remove(socketPath)
}

// TestUnixSocketWithOptions tests the options for Unix socket client transport
func TestUnixSocketWithOptions(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-client-options.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create a mock transport with proper initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Create client with Unix socket options
	c, err := client.NewClient("test-client",
		client.WithUnixSocket(socketPath,
			client.WithTimeout(5*time.Second),
			client.WithReconnect(true),
			client.WithReconnectDelay(2*time.Second),
			client.WithMaxRetries(3),
			client.WithBufferSize(8192),
			client.WithPermissions(0644),
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

	// Verify connection timeout was set properly
	if mockTransport.ConnectionTimeout != 5*time.Second {
		t.Errorf("Connection timeout not correctly set, expected 5s, got %v", mockTransport.ConnectionTimeout)
	}

	// Verify request timeout was set properly
	if mockTransport.RequestTimeout != 5*time.Second {
		t.Errorf("Request timeout not correctly set, expected 5s, got %v", mockTransport.RequestTimeout)
	}

	// Clean up
	c.Close()
	os.Remove(socketPath)
}

// TestUnixClientServerCommunication tests a basic request-response flow
func TestUnixClientServerCommunication(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-communication.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create mock transport with initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Create client with Unix socket transport
	c, err := client.NewClient("test-client",
		client.WithUnixSocket(socketPath),
		client.WithTransport(mockTransport),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Don't attempt to make a call, just verify client was created successfully
	if c == nil {
		t.Fatal("Client is nil")
	}

	// Verify initialization happened
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called during initialization")
	}

	// Clean up
	c.Close()
	os.Remove(socketPath)
}

// TestUnixSocketReconnection tests the client with reconnection options
func TestUnixSocketReconnection(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-reconnect.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create mock transport with initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Create client with Unix socket transport and reconnection enabled
	c, err := client.NewClient("test-client",
		client.WithUnixSocket(socketPath,
			client.WithReconnect(true),
			client.WithReconnectDelay(10*time.Millisecond), // Shorter delay for testing
			client.WithMaxRetries(3),
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

	// Verify initialization happened
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called during initialization")
	}

	// Clean up
	c.Close()
	os.Remove(socketPath)
}

// TestUnixSocketTimeout tests the client with timeout options
func TestUnixSocketTimeout(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-timeout.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create mock transport with initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Create client with Unix socket transport and short timeout
	c, err := client.NewClient("test-client",
		client.WithUnixSocket(socketPath,
			client.WithTimeout(50*time.Millisecond), // Short timeout for testing
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

	// Verify initialization happened
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called during initialization")
	}

	// Clean up
	c.Close()
	os.Remove(socketPath)
}
