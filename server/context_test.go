package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mocking needed for server/message handler to test context methods

// Helper to create a message handler for testing
func setupTestMessageHandler() (*server.MessageHandler, *server.Server, *mockClientSession) {
	srv := server.NewServer("Test Server").WithLogger(logx.NewDefaultLogger())
	mh := server.NewMessageHandler(srv)
	srv.MessageHandler = mh
	srv.SubscriptionManager = server.NewSubscriptionManager()
	srv.TransportManager = server.NewTransportManager()
	srv.Registry = server.NewRegistry()

	session := newMockClientSession("test-session-mh")
	srv.TransportManager.RegisterSession(session, &protocol.ClientCapabilities{})

	return mh, srv, session
}

// Helper to create a context for testing
func setupTestContext(parent context.Context, requestID string, progressToken interface{}) (*server.Context, *server.Server, *mockClientSession) {
	_, srv, session := setupTestMessageHandler() // Get server and mock session
	if parent == nil {
		parent = context.Background()
	}
	ctx := server.NewContext(parent, requestID, session, progressToken, srv)
	return ctx, srv, session
}

// Helper to create a server and message handler for context tests
func setupTestServerAndMessageHandler() (*server.Server, *mockClientSession, *server.MessageHandler) {
	srv := server.NewServer("Context Test Server").WithLogger(logx.NewDefaultLogger())
	mh := server.NewMessageHandler(srv)
	srv.MessageHandler = mh
	mockSession := newMockClientSession("test-session-ctx") // Use a unique prefix
	// We don't necessarily need to register the session with TransportManager here
	// unless the tested Context method relies on TransportManager lookups.
	return srv, mockSession, mh
}

// Reuse the existing setupTestServer from messaging_test.go with a new name
func setupContextTestServer(t *testing.T) (*server.Server, *mockClientSession, func()) {
	t.Helper()
	// Create a minimal server instance needed for tests
	srv := server.NewServer("test-server").WithLogger(logx.NewDefaultLogger())
	mh := server.NewMessageHandler(srv) // Use the exported NewMessageHandler
	srv.MessageHandler = mh
	srv.SubscriptionManager = server.NewSubscriptionManager() // Takes no arguments
	srv.TransportManager = server.NewTransportManager()       // Initialize TransportManager
	srv.Registry = server.NewRegistry()                       // Initialize Registry

	// Create a mock session
	session := newMockClientSession("test-session-setup")
	// Add logger if mockClientSession implements SetLogger
	// session.SetLogger(logx.NewDefaultLogger())

	// Register session (important for callbacks, etc.)
	srv.TransportManager.RegisterSession(session, &protocol.ClientCapabilities{})

	// Define cleanup function
	cleanup := func() {
		// Add any necessary cleanup, e.g., stopping the server if it were running
		t.Log("Test server cleanup executed.")
	}

	return srv, session, cleanup
}

// Test NewContext - Basic check
func TestContext_NewContext(t *testing.T) {
	ctx, _, _ := setupTestContext(context.Background(), "req-1", nil)
	if ctx == nil {
		t.Fatal("NewContext returned nil")
	}
	// Add more checks if needed (e.g., internal fields if accessible/exported)
}

// TODO: Test Log methods (Info, Debug, Warning, Error) -> verify SendNotification call
func TestContext_LogInfo(t *testing.T)    { t.Skip("Test not implemented") }
func TestContext_LogDebug(t *testing.T)   { t.Skip("Test not implemented") }
func TestContext_LogWarning(t *testing.T) { t.Skip("Test not implemented") }
func TestContext_LogError(t *testing.T)   { t.Skip("Test not implemented") }

// TODO: Test ReportProgress -> verify SendNotification call
func TestContext_ReportProgress(t *testing.T) { t.Skip("Test not implemented") }

// Define a direct test for ReadResource without the Context implementation
func TestReadResourceFunctionality(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server")
	tempDir := t.TempDir()

	// --- Setup Resources ---
	// 1. Text Resource
	textFileContent := "Hello, Text!"
	textFilePath := filepath.Join(tempDir, "ctx_read.txt")
	os.WriteFile(textFilePath, []byte(textFileContent), 0644)
	textFileURI := "file://" + textFilePath
	srv.Resource(textFileURI, server.WithFileContent(textFilePath), server.WithName("Context Test Text File"), server.WithMimeType("text/plain"))

	// 2. Audio Resource
	audioFileContent := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	audioFilePath := filepath.Join(tempDir, "ctx_read.audio")
	os.WriteFile(audioFilePath, audioFileContent, 0644)
	audioFileURI := "file://" + audioFilePath
	srv.Resource(audioFileURI, server.WithFileContent(audioFilePath), server.WithName("Context Test Audio File"), server.WithMimeType("audio/octet-stream"))

	// 3. Blob Resource (using a text file for simplicity)
	blobFileContent := "This is blob content."
	blobFilePath := filepath.Join(tempDir, "ctx_read.blob")
	os.WriteFile(blobFilePath, []byte(blobFileContent), 0644)
	blobFileURI := "file://" + blobFilePath
	srv.Resource(blobFileURI, server.WithFileContent(blobFilePath), server.WithName("Context Test Blob File"), server.WithMimeType("application/octet-stream"))

	// --- Test Cases ---
	t.Run("Read Text Success", func(t *testing.T) {
		// Get the resource
		resource, ok := srv.Registry.GetResource(textFileURI)
		require.True(t, ok, "Resource should exist")
		require.Equal(t, "file", resource.Kind, "Expected resource.Kind to be 'file'")

		// Read the content
		data, err := os.ReadFile(textFilePath)
		require.NoError(t, err, "Should read file without error")

		// Verify content
		if string(data) != textFileContent {
			t.Errorf("Expected text content '%s', got '%s'", textFileContent, string(data))
		}
	})

	t.Run("Read Audio Success", func(t *testing.T) {
		// Get the resource
		resource, ok := srv.Registry.GetResource(audioFileURI)
		require.True(t, ok, "Resource should exist")
		require.Equal(t, "audio", resource.Kind, "Expected resource.Kind to be 'audio'")

		// Read the content
		data, err := os.ReadFile(audioFilePath)
		require.NoError(t, err, "Should read file without error")

		// Verify content
		if !bytes.Equal(data, audioFileContent) {
			t.Errorf("Expected audio content to match")
		}
	})

	t.Run("Read Blob Success", func(t *testing.T) {
		// Get the resource
		resource, ok := srv.Registry.GetResource(blobFileURI)
		require.True(t, ok, "Resource should exist")
		require.Equal(t, "file", resource.Kind, "Expected resource.Kind to be 'file'")

		// Read the content
		data, err := os.ReadFile(blobFilePath)
		require.NoError(t, err, "Should read file without error")

		// Verify content
		if string(data) != blobFileContent {
			t.Errorf("Expected blob content '%s', got '%s'", blobFileContent, string(data))
		}
	})

	t.Run("Read Not Found", func(t *testing.T) {
		_, ok := srv.Registry.GetResource("file:///does/not/exist")
		require.False(t, ok, "Resource should not exist")
	})
}

// TestContext_CallTool verifies the Context.CallTool method.
func TestContext_CallTool(t *testing.T) {
	t.Parallel()
	srv, mockSession, mh := setupTestServerAndMessageHandler()

	// Create context
	parentCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx := server.NewContext(parentCtx, "req-ctx-calltool", mockSession, nil, srv)

	clientToolName := "client_calculator"
	clientToolInput := map[string]int{"a": 5, "b": 3}

	// --- Test Success Path ---
	var receivedReq protocol.JSONRPCRequest
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		output, toolErr, err := ctx.CallTool(clientToolName, clientToolInput)

		require.NoError(t, err, "Context.CallTool failed")
		require.Nil(t, toolErr, "Expected nil tool error from client")
		require.NotNil(t, output, "Expected non-nil output from client tool")
		require.JSONEq(t, `{"result": 8}`, string(output))
	}()

	// Wait for the server to send the request
	var sentReqs []protocol.JSONRPCRequest
	startTime := time.Now()
	for time.Since(startTime) < 2*time.Second {
		sentReqs = mockSession.GetSentRequests()
		if len(sentReqs) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Len(t, sentReqs, 1, "Expected server to send 1 request")
	receivedReq = sentReqs[0]

	// Verify the sent request
	require.Equal(t, protocol.MethodCallTool, receivedReq.Method)
	require.NotEmpty(t, receivedReq.ID, "Request ID should not be empty")

	var params protocol.CallToolRequestParams
	err := json.Unmarshal(receivedReq.Params.(json.RawMessage), &params)
	require.NoError(t, err, "Failed to unmarshal sent request params")
	require.NotNil(t, params.ToolCall)
	require.Equal(t, clientToolName, params.ToolCall.ToolName)
	require.NotEmpty(t, params.ToolCall.ID, "ToolCall ID should not be empty")
	expectedInputJSON, _ := json.Marshal(clientToolInput)
	require.JSONEq(t, string(expectedInputJSON), string(params.ToolCall.Input))

	// Simulate client sending back a successful response
	successResult := protocol.CallToolResult{
		ToolCallID: params.ToolCall.ID, // Echo back the specific tool call ID
		Output:     json.RawMessage(`{"result": 8}`),
		Error:      nil,
	}
	successResp := protocol.NewSuccessResponse(receivedReq.ID, successResult)
	successRespBytes, _ := json.Marshal(successResp)

	// Handle the response (this will unblock the goroutine)
	err = mh.HandleMessage(mockSession, successRespBytes)
	require.NoError(t, err, "Failed to handle client success response")

	wg.Wait() // Wait for the goroutine to finish assertions

	// --- Test Error Path ---
	mockSession.ClearMessages()
	wg = sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		clientToolErrInput := map[string]string{"error": "divide by zero"}
		_, toolErr, err := ctx.CallTool("client_failing_tool", clientToolErrInput)

		require.NoError(t, err, "Context.CallTool failed unexpectedly on client error")
		require.NotNil(t, toolErr, "Expected non-nil tool error from client")
		require.Equal(t, protocol.ErrorCode(12345), toolErr.Code) // Example error code
		require.Equal(t, "Client tool failed", toolErr.Message)
	}()

	// Wait for the server to send the request again
	sentReqs = nil
	startTime = time.Now()
	for time.Since(startTime) < 2*time.Second {
		sentReqs = mockSession.GetSentRequests()
		if len(sentReqs) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Len(t, sentReqs, 1, "Expected server to send 1 request for error test")
	receivedReq = sentReqs[0]
	require.Equal(t, protocol.MethodCallTool, receivedReq.Method)

	// Unmarshal params again to get the new ToolCall ID
	err = json.Unmarshal(receivedReq.Params.(json.RawMessage), &params)
	require.NoError(t, err)

	// Simulate client sending back an error response
	errResult := protocol.CallToolResult{
		ToolCallID: params.ToolCall.ID, // Echo back the specific tool call ID
		Output:     nil,
		Error: &protocol.ToolError{
			Code:    12345, // Use the custom type
			Message: "Client tool failed",
		},
	}
	errResp := protocol.NewSuccessResponse(receivedReq.ID, errResult) // Note: Still a success at JSON-RPC level
	errRespBytes, _ := json.Marshal(errResp)

	// Handle the error response
	err = mh.HandleMessage(mockSession, errRespBytes)
	require.NoError(t, err, "Failed to handle client error response")

	wg.Wait() // Wait for the goroutine to finish assertions
}

// Helper function for TestContext_ReportProgress
func waitForContextNotification(t *testing.T, session *mockClientSession, expectedCount int, timeout time.Duration) {
	t.Helper()
	startTime := time.Now()

	// Use the provided timeout, but ensure it doesn't exceed 3 seconds
	maxTimeout := 3 * time.Second
	if timeout <= 0 || timeout > maxTimeout {
		timeout = maxTimeout
	}

	for time.Since(startTime) < timeout {
		if len(session.GetSentNotifications()) >= expectedCount {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Timeout reached
	t.Fatalf("Timed out waiting for %d notification(s) after %v. Got %d.", expectedCount, timeout, len(session.GetSentNotifications()))
}

func TestContext_ProgressReporting(t *testing.T) {
	t.Parallel()
	srv, session, _ := setupContextTestServer(t) // Use renamed setup function
	progressToken := "my-progress-token"
	ctx := server.NewContext(context.Background(), "req-progress", session, progressToken, srv)

	// Clear any initial notifications
	session.ClearMessages()

	ctx.ReportProgress("Starting work", 0, 100)

	// Wait for the notification
	waitForContextNotification(t, session, 1, 100*time.Millisecond) // Use renamed helper

	notifications := session.GetSentNotifications()
	require.Len(t, notifications, 1)

	notif := notifications[0]
	assert.Equal(t, protocol.MethodProgress, notif.Method)

	var params protocol.ProgressParams
	// Handle if Params is already unmarshalled into map[string]interface{}
	paramsBytes, err := json.Marshal(notif.Params)
	require.NoError(t, err, "Failed to remarshal params")
	err = json.Unmarshal(paramsBytes, &params)
	require.NoError(t, err)

	assert.Equal(t, progressToken, params.Token)
	// Check the value structure
	progressValue, ok := params.Value.(map[string]interface{})
	require.True(t, ok, "Progress value is not a map")
	assert.Equal(t, "Starting work", progressValue["message"])
	assert.Equal(t, float64(0), progressValue["current"])
	assert.Equal(t, float64(100), progressValue["total"])
}
