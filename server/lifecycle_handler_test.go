package server_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// --- Mock Client Session (REMOVED - Defined in messaging_test.go) ---

// --- Test Setup Helper (Adapted from messaging_test.go) ---

// func setupTestLifecycleHandler() (*server.LifecycleHandler, *server.Server, *mockClientSession) {
// 	// LifecycleHandler needs the server, so we still create a minimal server
// 	srv := server.NewServer("Test Lifecycle Server")
// 	// We get the handler via the server instance, assuming it's accessible or we use the MH setup
// 	// mh := server.NewMessageHandler(srv) // Need MH to access LifecycleHandler indirectly for now - REMOVED
// 	// TODO: Expose LifecycleHandler directly or test via MessageHandler
// 	// For now, we'll test via MessageHandler by sending 'initialize' message
//
// 	session := newMockClientSession("test-lifecycle-session-1")
// 	return nil, srv, session // Return nil for handler for now, test via MH
// }

// Use the setup function from messaging_test.go as we test via HandleMessage
func setupTestMessageHandlerForLifecycle() (*server.MessageHandler, *server.Server, *mockClientSession) {
	srv := server.NewServer("Test Lifecycle Server")
	mh := server.NewMessageHandler(srv)
	// newMockClientSession is defined in messaging_test.go (same package)
	session := newMockClientSession("test-lifecycle-session-1")
	return mh, srv, session
}

// --- Initialize Tests (Moved from messaging_test.go) ---

func TestLifecycle_Initialize_NegotiateCurrentVersion(t *testing.T) {
	mh, srv, session := setupTestMessageHandlerForLifecycle()

	// Set server capabilities
	srv.ImplementsResourceSubscription = true
	srv.ImplementsLogging = true
	srv.ImplementsCompletions = true

	// Build params *without* Capabilities first, marshal, then add capabilities manually
	baseParams := &struct {
		ProtocolVersion string                  `json:\"protocolVersion\"`
		ClientInfo      protocol.Implementation `json:\"clientInfo\"`
		Capabilities    map[string]interface{}  `json:\"capabilities\"` // Use map for flexibility
	}{
		ProtocolVersion: protocol.CurrentProtocolVersion,
		ClientInfo: protocol.Implementation{
			Name:    "test-client",
			Version: "1.0",
		},
		// Leave Capabilities nil for now
	}

	// Manually construct the capabilities map
	clientCapsMap := map[string]interface{}{
		"resources": map[string]interface{}{ // Matches anonymous struct fields/tags
			"subscribe": true,
		},
		"logging": map[string]interface{}{},
		"sampling": map[string]interface{}{ // Matches SamplingCapability fields/tags
			"enabled": true,
		},
	}
	baseParams.Capabilities = clientCapsMap

	// Construct request
	req := protocol.JSONRPCRequest{
		Method: "initialize",
		ID:     "init-req-1",
		Params: baseParams, // Pass the modified params struct
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal initialize request: %v", err)
	}

	handleErr := mh.HandleMessage(session, reqBytes)
	if handleErr != nil {
		t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
	}

	// Wait for the response
	waitForResponse(t, session, req.ID, 100*time.Millisecond)

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
	resp := responses[0]

	if resp.Error != nil {
		t.Fatalf("Expected success response, but got error: %+v", resp.Error)
	}
	if resp.ID != "init-req-1" {
		t.Errorf("Expected ID 'init-req-1', got %v", resp.ID)
	}

	// Unmarshal the result into InitializeResult
	var result protocol.InitializeResult
	resultBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	if result.ProtocolVersion != protocol.CurrentProtocolVersion {
		t.Errorf("Expected negotiated version %s, got %s", protocol.CurrentProtocolVersion, result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "Test Lifecycle Server" { // Updated server name
		t.Errorf("Expected server name 'Test Lifecycle Server', got %s", result.ServerInfo.Name)
	}

	// Verify negotiated capabilities
	if result.Capabilities.Resources == nil || !result.Capabilities.Resources.Subscribe {
		t.Error("Expected Resources.Subscribe capability to be enabled")
	}
	if result.Capabilities.Resources.ListChanged { // Server didn't implement this in default setup
		// TODO: Check srv.ImplementsResourceListChanged default or set explicitly
		t.Error("Expected Resources.ListChanged capability to be disabled based on default server setup")
	}
	if result.Capabilities.Logging == nil { // Server implemented, client supported
		t.Error("Expected Logging capability to be enabled")
	}
	if result.Capabilities.Completions == nil { // Server implemented, client supported
		t.Error("Expected Completions capability to be enabled")
	}
}

func TestLifecycle_Initialize_NegotiateOldVersion(t *testing.T) {
	mh, srv, session := setupTestMessageHandlerForLifecycle()

	// Set server capabilities
	srv.ImplementsResourceSubscription = true
	srv.ImplementsLogging = true // Logging is 2025-03-26 only, should not be negotiated

	// Build params *without* Capabilities first, marshal, then add capabilities manually
	baseParams := &struct {
		ProtocolVersion string                  `json:\"protocolVersion\"`
		ClientInfo      protocol.Implementation `json:\"clientInfo\"`
		Capabilities    map[string]interface{}  `json:\"capabilities\"` // Use map for flexibility
	}{
		ProtocolVersion: protocol.OldProtocolVersion,
		ClientInfo: protocol.Implementation{
			Name:    "test-client-old",
			Version: "0.9",
		},
	}

	// Manually construct the capabilities map
	clientCapsMap := map[string]interface{}{
		"resources": map[string]interface{}{ // Matches anonymous struct fields/tags
			"subscribe": true,
		},
		// No logging/sampling for old client
	}
	baseParams.Capabilities = clientCapsMap

	// Construct request
	req := protocol.JSONRPCRequest{
		Method: "initialize",
		ID:     "init-req-2",
		Params: baseParams,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal initialize request: %v", err)
	}

	handleErr := mh.HandleMessage(session, reqBytes)
	if handleErr != nil {
		t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
	}

	// Wait for the response
	waitForResponse(t, session, req.ID, 100*time.Millisecond)

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
	resp := responses[0]

	if resp.Error != nil {
		t.Fatalf("Expected success response, but got error: %+v", resp.Error)
	}
	if resp.ID != "init-req-2" {
		t.Errorf("Expected ID 'init-req-2', got %v", resp.ID)
	}

	var result protocol.InitializeResult
	resultBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	if result.ProtocolVersion != protocol.OldProtocolVersion {
		t.Errorf("Expected negotiated version %s, got %s", protocol.OldProtocolVersion, result.ProtocolVersion)
	}

	// Verify negotiated capabilities (only old ones should be possible)
	if result.Capabilities.Resources == nil || !result.Capabilities.Resources.Subscribe {
		t.Error("Expected Resources.Subscribe capability to be enabled")
	}
	if result.Capabilities.Logging != nil { // Should not be present for old version
		t.Error("Expected Logging capability to be disabled for old version negotiation")
	}
	if result.Capabilities.Completions != nil { // Should not be present for old version
		t.Error("Expected Completions capability to be disabled for old version negotiation")
	}
}

func TestLifecycle_Initialize_InvalidParams(t *testing.T) {
	mh, _, session := setupTestMessageHandlerForLifecycle()

	// Send params as an incompatible type (e.g., string instead of object)
	req := protocol.JSONRPCRequest{
		Method: "initialize",
		ID:     "init-req-invalid",
		Params: json.RawMessage(`"not-an-object"`),
	}
	reqBytes, _ := json.Marshal(req)

	handleErr := mh.HandleMessage(session, reqBytes)
	if handleErr != nil {
		// HandleMessage should succeed in sending the error response
		t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
	}

	// Wait for the response
	waitForResponse(t, session, req.ID, 100*time.Millisecond)

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response for invalid params, got %d", len(responses))
	}
	resp := responses[0]

	if resp.Error == nil {
		t.Fatalf("Expected error response, but got success")
	}
	if protocol.ErrorCode(resp.Error.Code) != protocol.CodeInvalidParams {
		t.Errorf("Expected error code %d (InvalidParams), got %d", protocol.CodeInvalidParams, resp.Error.Code)
	}
	if resp.ID != "init-req-invalid" {
		t.Errorf("Expected ID 'init-req-invalid', got %v", resp.ID)
	}
}

// --- Shutdown Test (Related to Compliance Plan I) ---

func TestLifecycle_Shutdown(t *testing.T) {
	mh, _, session := setupTestMessageHandlerForLifecycle()

	// Shutdown request has no parameters
	req := protocol.JSONRPCRequest{
		Method: "shutdown",
		ID:     "shutdown-req-1",
		Params: nil, // Or json.RawMessage(`null`)
	}
	reqBytes, _ := json.Marshal(req)

	handleErr := mh.HandleMessage(session, reqBytes)
	if handleErr != nil {
		// Should succeed in sending the response
		t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
	}

	// Wait for the response
	waitForResponse(t, session, req.ID, 100*time.Millisecond)

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
	resp := responses[0]

	if resp.Error != nil {
		t.Fatalf("Expected success response, but got error: %+v", resp.Error)
	}
	if resp.ID != "shutdown-req-1" {
		t.Errorf("Expected ID 'shutdown-req-1', got %v", resp.ID)
	}

	// Per JSON-RPC spec, the Result for shutdown should be null.
	if resp.Result != nil {
		resultBytes, _ := json.Marshal(resp.Result)
		t.Errorf("Expected result to be null, got %s", string(resultBytes))
	}

	// TODO: Optionally, verify that TransportManager.Shutdown() was called.
	// This might require enhancing the test setup or using mocks/spies for TransportManager.
}

// --- Exit Notification Test (Compliance Plan II.1) ---

func TestHandleNotification_Exit(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Create an 'exit' notification
	// 3. Marshal and send via HandleMessage
	// 4. Verify no response/notification sent back
	// 5. Verify server shutdown sequence was initiated (e.g., close(server.done) was called)
	//    - This likely requires adding a way to observe the 'done' channel or server state.
	t.Skip("Test not implemented")
}

// TODO: Add tests for ExitHandler (requires handling 'exit' notification)
