package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple echo script to use for testing
const echoScriptContent = `#!/bin/bash
while read line; do
  echo "$line"
done
`

// Helper to create a temporary echo script for testing
func createTempEchoScript(t *testing.T) string {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "echo.sh")
	err := os.WriteFile(scriptPath, []byte(echoScriptContent), 0755)
	require.NoError(t, err, "Failed to create temp echo script")
	return scriptPath
}

func TestNewStdioTransport(t *testing.T) {
	// Create a logger for testing
	logger := logx.NewDefaultLogger()

	// Test with valid arguments
	transport, err := NewStdioTransport("echo", []string{}, logger)
	assert.NoError(t, err, "Should not return an error with valid args")
	assert.NotNil(t, transport, "Should return a valid transport")

	// Test with options
	transport, err = NewStdioTransport("echo", []string{}, logger, WithRequestTimeout(5*time.Second))
	assert.NoError(t, err, "Should not return an error with options")
	assert.NotNil(t, transport, "Should return a valid transport with options")
}

func TestStdioTransportConnect(t *testing.T) {
	logger := logx.NewDefaultLogger()
	scriptPath := createTempEchoScript(t)

	// Test successful connection
	t.Run("SuccessfulConnect", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = transport.Connect(ctx)
		assert.NoError(t, err, "Should connect successfully")
		assert.True(t, transport.IsConnected(), "Should be connected after Connect")

		// Clean up
		err = transport.Close()
		assert.NoError(t, err, "Failed to close transport")
	})

	// Test connect when already connected
	t.Run("AlreadyConnected", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Connect first time
		err = transport.Connect(ctx)
		require.NoError(t, err, "Should connect successfully first time")

		// Try to connect again
		err = transport.Connect(ctx)
		assert.Error(t, err, "Should return error when already connected")
		assert.True(t, transport.IsConnected(), "Should still be connected")

		// Clean up
		err = transport.Close()
		assert.NoError(t, err, "Failed to close transport")
	})

	// Test connect with invalid command
	t.Run("InvalidCommand", func(t *testing.T) {
		transport, err := NewStdioTransport("non_existent_command", []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = transport.Connect(ctx)
		assert.Error(t, err, "Should return error with invalid command")
		assert.False(t, transport.IsConnected(), "Should not be connected")
	})
}

func TestStdioTransportClose(t *testing.T) {
	logger := logx.NewDefaultLogger()
	scriptPath := createTempEchoScript(t)

	// Test closing a connected transport
	t.Run("CloseConnected", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = transport.Connect(ctx)
		require.NoError(t, err, "Failed to connect")

		err = transport.Close()
		assert.NoError(t, err, "Should close without error")
		assert.False(t, transport.IsConnected(), "Should not be connected after close")
	})

	// Test closing an unconnected transport
	t.Run("CloseUnconnected", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		err = transport.Close()
		assert.NoError(t, err, "Closing unconnected transport should not error")
		assert.False(t, transport.IsConnected(), "Should not be connected")
	})

	// Test close multiple times
	t.Run("CloseMultipleTimes", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = transport.Connect(ctx)
		require.NoError(t, err, "Failed to connect")

		err = transport.Close()
		assert.NoError(t, err, "First close should succeed")

		err = transport.Close()
		assert.NoError(t, err, "Second close should be no-op")
	})
}

func TestStdioTransportIsConnected(t *testing.T) {
	logger := logx.NewDefaultLogger()
	scriptPath := createTempEchoScript(t)

	// Test various states
	t.Run("ConnectionStates", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		// Initial state
		assert.False(t, transport.IsConnected(), "Should not be connected initially")

		// After connecting
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = transport.Connect(ctx)
		require.NoError(t, err, "Failed to connect")
		assert.True(t, transport.IsConnected(), "Should be connected after Connect")

		// After closing
		err = transport.Close()
		require.NoError(t, err, "Failed to close")
		assert.False(t, transport.IsConnected(), "Should not be connected after Close")
	})
}

func TestStdioTransportSendRequest(t *testing.T) {
	logger := logx.NewDefaultLogger()
	scriptPath := createTempEchoScript(t)

	// Test sending a request when not connected
	t.Run("SendWhenNotConnected", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		req := &protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "test.method",
			ID:      1,
		}

		ctx := context.Background()
		_, err = transport.SendRequest(ctx, req)
		assert.Error(t, err, "Should error when not connected")
	})

	// Skip actual request sending test as it requires a more complex echo script
	// that can parse and respond to JSON-RPC
	t.Skip("Full request/response testing requires additional infrastructure")
}

func TestStdioTransportSendRequestAsync(t *testing.T) {
	logger := logx.NewDefaultLogger()
	scriptPath := createTempEchoScript(t)

	// Test sending async request when not connected
	t.Run("SendAsyncWhenNotConnected", func(t *testing.T) {
		transport, err := NewStdioTransport(scriptPath, []string{}, logger)
		require.NoError(t, err, "Failed to create transport")

		req := &protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "test.method",
			ID:      1,
		}

		ctx := context.Background()
		responseChannel := make(chan *protocol.JSONRPCResponse, 1)
		err = transport.SendRequestAsync(ctx, req, responseChannel)
		assert.Error(t, err, "Should error when not connected")
	})

	// Skip more complex async tests as they require a script that can parse JSON-RPC
	t.Skip("Full async testing requires additional infrastructure")
}

func TestStdioTransportSetNotificationHandler(t *testing.T) {
	logger := logx.NewDefaultLogger()

	transport, err := NewStdioTransport("echo", []string{}, logger)
	require.NoError(t, err, "Failed to create transport")

	// Set notification handler
	handler := func(notification *protocol.JSONRPCNotification) error {
		// Just a dummy handler for testing
		return nil
	}

	// Set the handler
	transport.SetNotificationHandler(handler)

	// Cannot easily test the handler without a specific echo script that sends notifications
	// This test just ensures the method doesn't panic
}

func TestStdioTransportGetTransportType(t *testing.T) {
	logger := logx.NewDefaultLogger()

	transport, err := NewStdioTransport("echo", []string{}, logger)
	require.NoError(t, err, "Failed to create transport")

	// Verify transport type
	assert.Equal(t, TransportTypeStdio, transport.GetTransportType(), "Should return stdio transport type")
}

func TestStdioTransportGetTransportInfo(t *testing.T) {
	logger := logx.NewDefaultLogger()

	transport, err := NewStdioTransport("echo", []string{}, logger)
	require.NoError(t, err, "Failed to create transport")

	info := transport.GetTransportInfo()
	assert.NotNil(t, info, "Should return non-nil transport info")

	// Check specific fields returned by the implementation
	assert.Equal(t, "echo", info["command"], "Should contain the command")
	assert.Contains(t, info, "args", "Should contain args field")
	assert.Contains(t, info, "connected", "Should contain connected field")
	assert.Contains(t, info, "pid", "Should contain pid field")

	// The "type" field is returned by transport interface but might be added by wrapping struct
	// Currently not included in the actual implementation
}

func TestStdioTransportGetPID(t *testing.T) {
	// This tests an internal method that isn't directly accessible through the interface
	// Skip this test since we can't test it directly without type assertion to unexported type
	t.Skip("Cannot directly test getPID method")
}

func TestStdioTransportReceiveLoop(t *testing.T) {
	// This tests an internal method that isn't directly accessible through the interface
	// Skip this test since we can't test it directly without type assertion to unexported type
	t.Skip("Cannot directly test receiveLoop method")
}
