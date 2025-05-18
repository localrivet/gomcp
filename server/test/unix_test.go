package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/unix"
)

// TestUnixSocketTransport tests the basic configuration of a server with Unix Domain Socket transport
func TestUnixSocketTransport(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-server.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create a new server instance
	s := server.NewServer("test-server")

	// Configure it to use Unix Domain Sockets
	s = s.AsUnixSocket(socketPath)

	// Verify the server was configured successfully
	if s == nil {
		t.Fatal("Server is nil after configuration")
	}

	// Use options as well to test the full API
	s = s.AsUnixSocket(socketPath,
		unix.WithPermissions(0600),
		unix.WithBufferSize(8192),
	)

	// Verify the server was configured successfully
	if s == nil {
		t.Fatal("Server is nil after configuration with options")
	}

	// Clean up
	os.Remove(socketPath)
}

// TestUnixSocketConnection tests that a server can successfully listen on a Unix socket
func TestUnixSocketConnection(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-connection.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create a server
	s := server.NewServer("test-unix-connection")

	// Configure with Unix socket transport
	s = s.AsUnixSocket(socketPath, unix.WithPermissions(0666)) // Permissive for testing

	// Start the server in a goroutine
	go func() {
		if err := s.Run(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait a brief moment for the server to start
	time.Sleep(100 * time.Millisecond)

	// Check if the socket file was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatalf("Socket file was not created at %s", socketPath)
	}

	// Clean up
	os.Remove(socketPath)
}

// TestUnixSocketClientServer tests a client connecting to a server through Unix Domain Sockets
// and using a properly named tool (lowercase with underscore)
func TestUnixSocketClientServer(t *testing.T) {
	t.Skip("Skipping full client-server test for now") // Remove this line to enable the test

	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), "gomcp-test-unix-client-server.sock")

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create a server with a tool handler
	s := server.NewServer("test-unix-server")

	// Register a simple test tool using the correct naming convention (lowercase with underscore)
	s = s.Tool("test_echo", "Echo back the message", func(ctx *server.Context) (interface{}, error) {
		// Extract the message parameter from the request
		if ctx.Request == nil || ctx.Request.ToolArgs == nil {
			return map[string]string{"error": "no arguments provided"}, nil
		}

		// Get the message parameter
		message, ok := ctx.Request.ToolArgs["message"].(string)
		if !ok {
			return map[string]string{"echo": "no message provided"}, nil
		}

		return map[string]string{"echo": message}, nil
	})

	// Configure server with Unix socket transport
	s = s.AsUnixSocket(socketPath, unix.WithPermissions(0666)) // Permissive for testing

	// Start the server
	go func() {
		if err := s.Run(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(200 * time.Millisecond)

	// Create a client using the same socket
	c, err := client.NewClient("unix-test-client",
		client.WithUnixSocket(socketPath),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Make a request to the test tool with proper name
	result, err := c.CallTool("test_echo", map[string]interface{}{
		"message": "Hello Unix Socket!",
	})

	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}

	// Verify result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	echo, ok := resultMap["echo"].(string)
	if !ok || echo != "Hello Unix Socket!" {
		t.Errorf("Expected echo response 'Hello Unix Socket!', got %v", resultMap)
	}

	// Clean up
	c.Close()
	os.Remove(socketPath)
}
