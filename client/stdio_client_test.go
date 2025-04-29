package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/protocol"
	// Stdio transport is implicitly tested via NewStdioClient -> NewClient
)

// TestNewStdioClient verifies the constructor.
// Assumes NewStdioClient correctly creates the StdioTransport internally.
func TestNewStdioClient(t *testing.T) {
	// This test becomes less meaningful as transport creation is internal.
	// We rely on end-to-end or integration tests.
	// For now, just check if the client creation succeeds.
	opts := ClientOptions{
		Logger: NewNilLogger(),
		// Transport is created by NewStdioClient
	}
	client, err := NewStdioClient("test-stdio-client", opts)
	if err != nil {
		// This might fail if the underlying StdioTransport creation fails
		// (e.g., cannot access os.Stdin/os.Stdout in certain test environments).
		// Consider if this test is runnable in the CI environment.
		t.Fatalf("NewStdioClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("NewStdioClient returned nil client")
	}
	// We expect the client to be non-nil, but not yet connected/initialized.
	if client.IsInitialized() {
		t.Error("Newly created client should not be initialized")
	}
	// Close is difficult to test reliably here without mocking stdin/stdout.
	// Rely on other tests for Close behavior.
}

// TestStdioClientMessageProcessing simulates sending/receiving a notification.
// This requires mocking the transport layer as direct stdio is hard to test.
func TestStdioClientMessageProcessing(t *testing.T) {
	mockTransport := newMockTransport()
	opts := ClientOptions{
		Logger:    NewNilLogger(),
		Transport: mockTransport, // Inject mock transport
	}
	client, err := NewClient("test-stdio-client-mock", opts)
	if err != nil {
		t.Fatalf("NewClient with mock transport failed: %v", err)
	}

	// Simulate successful connection/initialization manually for this test
	client.stateMu.Lock()
	client.initialized = true
	client.negotiatedVersion = protocol.CurrentProtocolVersion // Assume current for test
	client.stateMu.Unlock()

	notificationReceived := make(chan struct{})
	method := "test/notification"
	err = client.RegisterNotificationHandler(method, func(ctx context.Context, params interface{}) error {
		t.Logf("Notification handler called for %s", method)
		close(notificationReceived)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to register notification handler: %v", err)
	}

	// Simulate server sending a notification
	notifParams := map[string]string{"data": "hello"}
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  notifParams,
	}
	notifBytes, _ := json.Marshal(notification)

	// Start the client's processing loop in background
	go client.startMessageProcessing() // Needs to run to process simulated message

	// Simulate receiving the notification via the mock transport
	if err := mockTransport.SimulateSend(notifBytes); err != nil {
		t.Fatalf("Failed to simulate sending notification: %v", err)
	}

	// Wait for the handler to be called
	select {
	case <-notificationReceived:
		t.Log("Notification successfully processed by handler.")
	case <-time.After(2 * time.Second): // Reduced timeout
		t.Fatal("Timed out waiting for notification to be processed")
	}

	// Clean up
	if err := client.Close(); err != nil {
		t.Errorf("client.Close() failed: %v", err)
	}
}

// TestStdioClientGracefulShutdown is difficult to test reliably without
// mocking stdin/stdout pipes and process signals. Skipping for now.
// func TestStdioClientGracefulShutdown(t *testing.T) { ... }

// TestStdioClientWithRealStdio is problematic in automated tests as it
// requires interacting with the actual stdin/stdout of the test runner.
// Skipping this test. Manual testing might be needed.
// func TestStdioClientWithRealStdio(t *testing.T) { ... }
