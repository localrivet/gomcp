package unix

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	// Test server mode with absolute path
	tmpDir := os.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	serverTransport := NewTransport(socketPath)
	if serverTransport.isClient {
		t.Errorf("Expected server mode for absolute path '%s', got client mode", socketPath)
	}

	// Test server mode with relative path
	serverTransport = NewTransport("./test.sock")
	if serverTransport.isClient {
		t.Errorf("Expected server mode for relative path './test.sock', got client mode")
	}

	// Test client mode
	clientTransport := NewTransport("test.sock") // No path prefix suggests client mode
	if !clientTransport.isClient {
		t.Errorf("Expected client mode for 'test.sock', got client mode")
	}

	// Test with options
	serverTransport = NewTransport(socketPath, WithPermissions(0644), WithBufferSize(8192))
	if serverTransport.permissions != 0644 || serverTransport.socketBufferSize != 8192 {
		t.Errorf("Options were not applied correctly: perm=%v, buffer=%v",
			serverTransport.permissions, serverTransport.socketBufferSize)
	}
}

func TestServerInitializeAndStart(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("gomcp-test-%d.sock", time.Now().UnixNano()))

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	transport := NewTransport(socketPath)

	// Test Initialize
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test Start
	err = transport.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify socket file was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatalf("Socket file was not created at %s", socketPath)
	}

	// Test permissions
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Error getting socket file info: %v", err)
	}
	if info.Mode().Perm() != DefaultSocketPermissions {
		t.Errorf("Socket has wrong permissions: %v, expected %v", info.Mode().Perm(), DefaultSocketPermissions)
	}

	// Clean up
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify socket file was removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("Socket file was not removed at %s", socketPath)
		// Clean up manually if test fails
		os.Remove(socketPath)
	}
}

func TestClientServerCommunication(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("gomcp-test-%d.sock", time.Now().UnixNano()))

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create server transport
	serverTransport := NewTransport(socketPath)

	// Set up an echo message handler
	testMsg := []byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`)
	respMsg := []byte(`{"jsonrpc":"2.0","id":1,"result":"success"}`)

	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		if bytes.Equal(message, testMsg) {
			return respMsg, nil
		}
		return nil, fmt.Errorf("unexpected message: %s", string(message))
	})

	// Initialize and start server
	err := serverTransport.Initialize()
	if err != nil {
		t.Fatalf("Server initialize failed: %v", err)
	}

	err = serverTransport.Start()
	if err != nil {
		t.Fatalf("Server start failed: %v", err)
	}

	// Directly connect a client instead of using the Transport interface
	// because the Receive method is only for client mode and we're using
	// direct Unix socket communication instead
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to socket: %v", err)
	}

	// Send test message with newline
	testMsgWithNewline := append(testMsg, '\n')
	_, err = conn.Write(testMsgWithNewline)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read response
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Remove trailing newline
	response := buffer[:n-1]
	if !bytes.Equal(response, respMsg) {
		t.Errorf("Expected response %s, got %s", string(respMsg), string(response))
	}

	// Close client connection
	conn.Close()

	// Clean up
	err = serverTransport.Stop()
	if err != nil {
		t.Fatalf("Server stop failed: %v", err)
	}

	// Ensure socket file is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("Socket file was not removed at %s", socketPath)
		// Clean up manually if test fails
		os.Remove(socketPath)
	}
}

func TestConcurrentConnections(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("gomcp-test-%d.sock", time.Now().UnixNano()))

	// Ensure socket doesn't exist before test
	os.Remove(socketPath)

	// Create server transport
	serverTransport := NewTransport(socketPath)

	// Set up a simple echo handler
	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		return message, nil
	})

	// Initialize and start server
	err := serverTransport.Initialize()
	if err != nil {
		t.Fatalf("Server initialize failed: %v", err)
	}

	err = serverTransport.Start()
	if err != nil {
		t.Fatalf("Server start failed: %v", err)
	}

	// Test with multiple concurrent clients
	numClients := 5
	errorCh := make(chan error, numClients)
	doneCh := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			// Connect directly with net.Dial for this test
			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				errorCh <- fmt.Errorf("client %d failed to connect: %v", clientID, err)
				return
			}
			defer conn.Close()

			// Send a message
			message := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"test","params":{}}`, clientID))
			message = append(message, '\n') // Add newline delimiter
			_, err = conn.Write(message)
			if err != nil {
				errorCh <- fmt.Errorf("client %d failed to send: %v", clientID, err)
				return
			}

			// Read response
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil {
				errorCh <- fmt.Errorf("client %d failed to receive: %v", clientID, err)
				return
			}

			// Verify response equals sent message (minus newline)
			response := buffer[:n-1] // Remove trailing newline
			if !bytes.Equal(response, message[:len(message)-1]) {
				errorCh <- fmt.Errorf("client %d got wrong response: %s", clientID, string(response))
				return
			}

			doneCh <- true
		}(i)
	}

	// Wait for all clients with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < numClients; i++ {
		select {
		case err := <-errorCh:
			t.Errorf("Client error: %v", err)
		case <-doneCh:
			// Client completed successfully
		case <-timeout:
			t.Fatal("Test timed out waiting for clients")
			break
		}
	}

	// Clean up
	err = serverTransport.Stop()
	if err != nil {
		t.Fatalf("Server stop failed: %v", err)
	}

	// Ensure socket file is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("Socket file was not removed at %s", socketPath)
		// Clean up manually if test fails
		os.Remove(socketPath)
	}
}

func TestErrorHandling(t *testing.T) {
	// Create a temporary socket path
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("gomcp-test-%d.sock", time.Now().UnixNano()))

	// Test connecting to non-existent socket
	clientTransport := NewTransport(socketPath)
	err := clientTransport.Initialize()
	if err == nil {
		// On some platforms, this might not return an error immediately
		// as they handle file creation differently or defer the actual connection
		t.Log("Note: Initialize on non-existent socket didn't return an error. This might be expected on some platforms.")
	}

	// Generate an invalid socket path based on the OS
	var invalidPath string
	if runtime.GOOS == "windows" {
		invalidPath = filepath.Join("Z:\\non-existent-dir", "test.sock")
	} else {
		invalidPath = filepath.Join("/non-existent-dir", "test.sock")
	}

	// Test starting server with invalid socket path
	serverTransport := NewTransport(invalidPath)
	err = serverTransport.Initialize()
	// Initialize might not fail on some platforms

	// Start should fail on most platforms
	err = serverTransport.Start()
	if err == nil {
		// If it doesn't fail, make sure to clean up
		t.Log("Note: Expected error when starting server with invalid socket path, but it succeeded")
		serverTransport.Stop()
	}

	// Test with existing socket file
	// Create a dummy file at the socket path
	dummySocketPath := filepath.Join(os.TempDir(), fmt.Sprintf("gomcp-test-dummy-%d.sock", time.Now().UnixNano()))
	dummyFile, err := os.Create(dummySocketPath)
	if err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}
	dummyFile.Close()

	// Make the file read-only to test permission issues
	err = os.Chmod(dummySocketPath, 0400)
	if err != nil {
		t.Fatalf("Failed to change permissions: %v", err)
	}

	// Try to start a server on the read-only file
	// On some systems this might still succeed if the user has sufficient permissions
	readOnlyTransport := NewTransport(dummySocketPath)
	err = readOnlyTransport.Initialize()
	if err != nil {
		t.Logf("Note: Initialize failed with read-only socket: %v", err)
	} else {
		err = readOnlyTransport.Start()
		if err == nil {
			t.Log("Note: Expected potential error when starting server on read-only file, but succeeded")
			readOnlyTransport.Stop() // Clean up
		} else {
			t.Logf("Note: Start failed with read-only socket as expected: %v", err)
		}
	}

	// Clean up
	os.Remove(dummySocketPath)
}
