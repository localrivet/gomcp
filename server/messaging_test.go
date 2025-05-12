package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForResponse polls the mock session for a response with a specific ID within a timeout.
func waitForResponse(t *testing.T, session *mockClientSession, requestID interface{}, timeout time.Duration) *protocol.JSONRPCResponse {
	t.Helper()
	startTime := time.Now()
	// Convert target ID to a canonical form for comparison (e.g., string or float64)
	var targetIDValue interface{} = requestID
	if idNum, ok := requestID.(int); ok {
		targetIDValue = float64(idNum) // Use float64 for numeric comparison
	} else if idNum, ok := requestID.(float64); ok {
		targetIDValue = idNum
	} // Add other numeric types if needed, otherwise treat as string

	// Use the provided timeout, but ensure it doesn't exceed 3 seconds for testing efficiency
	maxTimeout := 3 * time.Second
	if timeout <= 0 || timeout > maxTimeout {
		timeout = maxTimeout
	}

	for time.Since(startTime) < timeout {
		for _, resp := range session.GetSentResponses() {
			// Try to compare response ID with target ID, handling potential number types
			var respIDValue interface{}
			if rawID, ok := resp.ID.(json.RawMessage); ok {
				// Attempt to unmarshal RawMessage as number first, then string
				var numVal float64
				if err := json.Unmarshal(rawID, &numVal); err == nil {
					respIDValue = numVal
				} else {
					// Fallback: treat as string if not a number
					var strVal string
					if err := json.Unmarshal(rawID, &strVal); err == nil {
						respIDValue = strVal
					} else {
						// Could not determine type, use raw for potential %v comparison
						respIDValue = resp.ID
					}
				}
			} else {
				// If ID is not RawMessage, use its direct value (likely string or number)
				respIDValue = resp.ID
			}

			// Perform comparison based on determined types
			if reflect.DeepEqual(targetIDValue, respIDValue) {
				return &resp // Return pointer to the found response
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Timeout reached
	responses := session.GetSentResponses()
	ids := make([]string, len(responses))
	for i, r := range responses {
		// Use more careful string formatting for logging IDs
		if rawID, ok := r.ID.(json.RawMessage); ok {
			ids[i] = string(rawID)
		} else {
			ids[i] = fmt.Sprintf("%v", r.ID)
		}
	}
	t.Fatalf("Timed out waiting for response with ID '%v' after %v. Got responses for IDs: %v", requestID, timeout, ids)
	return nil // Should be unreachable due to Fatalf
}

// waitForNotification polls the mock session for a specific number of notifications.
// Potentially useful later.
func waitForNotification(t *testing.T, session *mockClientSession, expectedCount int, timeout time.Duration) {
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

// --- Test Setup Helpers --- ADDED HERE ---

// Placeholder for setupTestServer
func setupTestServer(t *testing.T) (*server.Server, *mockClientSession, func()) {
	t.Helper()
	// Create a minimal server instance needed for tests
	srv := server.NewServer("Test Server") // Use the exported NewServer
	mh := server.NewMessageHandler(srv)    // Use the exported NewMessageHandler
	srv.MessageHandler = mh
	srv.SubscriptionManager = server.NewSubscriptionManager() // Takes no arguments
	srv.TransportManager = server.NewTransportManager()       // Initialize TransportManager

	// Create a mock session
	session := newMockClientSession("test-session-setup")

	// Register session (important for callbacks, etc.)
	srv.TransportManager.RegisterSession(session, &protocol.ClientCapabilities{})

	// Define cleanup function
	cleanup := func() {
		// Add any necessary cleanup, e.g., stopping the server if it were running
		// srv.Shutdown() // Assuming a Shutdown method exists or is added - Commented out
		t.Log("Test server cleanup executed.")
	}

	return srv, session, cleanup
}

// Placeholder for createRequest
func createRequest(t *testing.T, method string, params interface{}) protocol.JSONRPCRequest {
	t.Helper()
	// Marshal params to json.RawMessage
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params for request creation: %v", err)
	}

	// Generate a unique ID for testing
	requestID := fmt.Sprintf("test-req-%s", uuid.NewString())

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      requestID,
		Params:  json.RawMessage(paramsBytes),
	}
	return req
}

// --- Test Cases ---

func TestHandleMessage_ValidRequest(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Create a valid JSON-RPC request (e.g., ping or a simple known method)
	// 3. Marshal the request to JSON bytes
	// 4. Call mh.HandleMessage(session, jsonBytes)
	// 5. Verify no error returned from HandleMessage
	// 6. Verify session.GetSentResponses() contains the expected success response
	// 7. Verify session.GetSentNotifications() is empty
}

func TestHandleMessage_ValidNotification(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Create a valid JSON-RPC notification (e.g., initialized)
	// 3. Marshal to JSON bytes
	// 4. Call mh.HandleMessage(session, jsonBytes)
	// 5. Verify no error returned
	// 6. Verify session.GetSentResponses() is empty
	// 7. Verify session.GetSentNotifications() is empty (usually no response to notifications)
}

func TestHandleMessage_InvalidJSON(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler                                              // Get handler from server
	invalidJSON := []byte(`{"jsonrpc": "2.0", "method": "foo", "id": 1,`) // Malformed

	err := mh.HandleMessage(session, invalidJSON)
	if err == nil {
		t.Fatalf("Expected an error for invalid JSON, but got nil")
	}

	// For Parse Error, the request ID might be unknown, so the response ID is typically null.
	// We can wait for *any* response or specifically check for a response with a null ID.
	// Let's check for null ID here.
	waitForResponse(t, session, nil, 100*time.Millisecond)

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response for invalid JSON, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error == nil {
		t.Fatalf("Expected an error response payload, got nil")
	}
	if resp.Error.Code != protocol.CodeParseError {
		t.Errorf("Expected error code %d (ParseError), got %d", protocol.CodeParseError, resp.Error.Code)
	}
	if resp.ID != nil {
		// Per JSON-RPC spec, ID should be null for parse errors if it couldn't be determined
		t.Errorf("Expected null ID for ParseError, got %v", resp.ID)
	}
}

func TestHandleMessage_InvalidRequestObject(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server
	// Valid JSON, but invalid Request (e.g., missing jsonrpc or method)
	invalidReqJSON := []byte(`{"id": 1}`)

	err := mh.HandleMessage(session, invalidReqJSON)
	// HandleMessage itself might return nil if it sends an error response successfully
	if err != nil {
		// We might get an unmarshal error here depending on how HandleMessage checks
		t.Logf("HandleMessage returned error (expected if it couldn't determine type): %v", err)
	}

	// The request ID was `1` in the invalid JSON `{"id": 1}`
	waitForResponse(t, session, 1, 100*time.Millisecond) // ID is 1 here

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		// It should detect it's not a valid req/notif/resp and send an error
		t.Fatalf("Expected 1 response for invalid request object, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error == nil {
		t.Fatalf("Expected an error response payload, got nil")
	}
	// The error code might vary depending on where validation fails.
	// Expecting InvalidRequest is reasonable.
	if resp.Error.Code != protocol.CodeInvalidRequest {
		t.Errorf("Expected error code %d (InvalidRequest), got %d", protocol.CodeInvalidRequest, resp.Error.Code)
	}
	// ID should match the invalid request's ID if parsable. Compare marshalled JSON form.
	expectedIDJSON, _ := json.Marshal(1)
	receivedIDJSON, err := json.Marshal(resp.ID)
	if err != nil {
		t.Fatalf("Could not marshal received ID: %v", err)
	}
	if string(receivedIDJSON) != string(expectedIDJSON) {
		t.Errorf("Expected ID %s, got %s (%v)", string(expectedIDJSON), string(receivedIDJSON), resp.ID)
	}
}

func TestHandleMessage_MethodNotFound(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server
	req := protocol.JSONRPCRequest{
		Method: "nonexistent/method",
		ID:     "req-1",
		Params: nil,
	}
	reqBytes, _ := json.Marshal(req)

	err := mh.HandleMessage(session, reqBytes)
	if err != nil {
		// Should be nil, as the error is sent in the response
		t.Fatalf("HandleMessage failed unexpectedly: %v", err)
	}

	waitForResponse(t, session, req.ID, 100*time.Millisecond) // Pass req.ID

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error == nil {
		t.Fatalf("Expected an error response payload, got nil")
	}
	if resp.Error.Code != protocol.CodeMethodNotFound {
		t.Errorf("Expected error code %d (MethodNotFound), got %d", protocol.CodeMethodNotFound, resp.Error.Code)
	}
	if resp.ID != "req-1" {
		t.Errorf("Expected ID 'req-1', got %v", resp.ID)
	}
}

// --- Ping Test (Compliance Plan I.1) ---

func TestHandleMessage_Ping(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server

	// Ping request parameters are empty according to schema
	params := struct{}{}
	req := protocol.JSONRPCRequest{
		Method: protocol.MethodPing,
		ID:     "ping-req-1",
		// Params should be omitted or null for ping according to schema examples, but server should handle empty object too
		Params: params, // Send empty object explicitly
	}
	reqBytes, _ := json.Marshal(req)

	handleErr := mh.HandleMessage(session, reqBytes)
	if handleErr != nil {
		t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
	}

	waitForResponse(t, session, req.ID, 100*time.Millisecond) // Pass req.ID

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
	resp := responses[0]

	// Expect success (Error == nil) once the handler is implemented.
	// Initially, this will fail as the method is not found.
	if resp.Error != nil {
		if resp.Error.Code == protocol.CodeMethodNotFound {
			t.Logf("Received expected MethodNotFound error (code: %d). Test should pass once ping handler is implemented.", resp.Error.Code)
			// Mark test as skipped until implemented?
			// For now, let it fail to drive implementation.
			t.Fatalf("Method 'ping' not implemented yet (received error: %d %s)", resp.Error.Code, resp.Error.Message)
		} else {
			t.Fatalf("Expected success or MethodNotFound, but got unexpected error: %+v", resp.Error)
		}
	}

	if resp.ID != "ping-req-1" {
		t.Errorf("Expected ID 'ping-req-1', got %v", resp.ID)
	}

	// Verify the result is an empty object (or null, depending on final implementation)
	// Since Go struct{}{} marshals to {}, check for that.
	resultBytes, _ := json.Marshal(resp.Result)
	if string(resultBytes) != "{}" {
		t.Errorf("Expected result to be an empty object '{}', got %s", string(resultBytes))
	}
}

// --- Resource Templates Test (Compliance Plan I.2) ---

func TestHandleMessage_ResourcesListTemplates(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server

	// resources/list_templates request has no parameters
	req := protocol.JSONRPCRequest{
		Method: protocol.MethodResourcesListTemplates,
		ID:     "tmpl-req-1",
		Params: nil, // Or json.RawMessage(`null`)
	}
	reqBytes, _ := json.Marshal(req)

	handleErr := mh.HandleMessage(session, reqBytes)
	if handleErr != nil {
		// Should succeed in sending the response, even if it's MethodNotFound initially
		t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
	}

	waitForResponse(t, session, req.ID, 100*time.Millisecond) // Pass req.ID

	responses := session.GetSentResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
	resp := responses[0]

	// Expect success (Error == nil) once the handler is implemented.
	if resp.Error != nil {
		if resp.Error.Code == protocol.CodeMethodNotFound {
			t.Logf("Received expected MethodNotFound error (code: %d). Test should pass once %s handler is implemented.", resp.Error.Code, req.Method)
			t.Fatalf("Method '%s' not implemented yet (received error: %d %s)", req.Method, resp.Error.Code, resp.Error.Message)
		} else {
			t.Fatalf("Expected success or MethodNotFound, but got unexpected error: %+v", resp.Error)
		}
	}

	if resp.ID != "tmpl-req-1" {
		t.Errorf("Expected ID 'tmpl-req-1', got %v", resp.ID)
	}

	// Verify the result structure (ListResourceTemplatesResult)
	var result protocol.ListResourceTemplatesResult
	resultBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal ListResourceTemplatesResult: %v", err)
	}

	// For now, expect an empty list until templates are actually added/retrieved
	if len(result.ResourceTemplates) != 0 {
		t.Errorf("Expected empty ResourceTemplates list initially, got %d items", len(result.ResourceTemplates))
	}

	// TODO: Add tests later where templates *are* registered and expected in the result.
}

// --- Prompts Get Test (Compliance Plan I.3) ---

func TestHandleMessage_PromptsGet(t *testing.T) {
	// TODO: Implement test (should fail with MethodNotFound initially)
	// 1. Setup MH, Server, Session
	// 2. Register a prompt via server.Registry
	// 3. Create a prompts/get request with the prompt ID
	// 4. Marshal and send via HandleMessage
	// 5. Verify MethodNotFound initially
	// 6. Once implemented, verify success response with correct prompt data
	t.Skip("Test not implemented")
}

// --- Tool Call Test (Covers existing handler + progress notification - Compliance Plan V) ---

func TestHandleMessage_ToolsCall_Success(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Register a tool via server.Registry (e.g., simple echo tool)
	// 3. Create a tools/call request for the registered tool
	// 4. Marshal and send via HandleMessage
	// 5. Verify success response with correct tool result
	t.Skip("Test not implemented")
}

func TestHandleMessage_ToolsCall_WithProgress(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Register a tool
	// 3. Create a tools/call request with a _meta.progressToken
	// 4. Marshal and send via HandleMessage
	// 5. Verify that session.SendNotification() was called with $/progress updates
	// 6. Verify success response
	t.Skip("Test not implemented")
}

func TestHandleMessage_ToolsCall_ToolNotFound(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Create a tools/call request for a tool NOT registered
	// 3. Marshal and send via HandleMessage
	// 4. Verify error response with code ErrorCodeMCPToolNotFound
	t.Skip("Test not implemented")
}

func TestHandleMessage_ToolsCall_InvalidParams(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Register a tool
	// 3. Create a tools/call request with mismatched/invalid arguments
	// 4. Marshal and send via HandleMessage
	// 5. Verify error response with code ErrorCodeInvalidParams
	t.Skip("Test not implemented")
}

// --- Resource Read Test (Covers existing handler) ---

func TestHandleMessage_ResourcesRead_Success(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server

	// --- Setup Temporary Files and Resources ---
	tempDir := t.TempDir()

	// 1. Text Resource
	textFileContent := "Hello, World! This is a test."
	textFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(textFilePath, []byte(textFileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp text file: %v", err)
	}
	textFileURI := "file://" + textFilePath
	srv.Resource(textFileURI, server.WithFileContent(textFilePath), server.WithName("Test Text File"), server.WithMimeType("text/plain"))

	// 2. Audio Resource (Dummy Content)
	audioFileContent := []byte{0xCA, 0xFE, 0xBA, 0xBE} // Dummy bytes
	audioFilePath := filepath.Join(tempDir, "test.audio")
	if err := os.WriteFile(audioFilePath, audioFileContent, 0644); err != nil {
		t.Fatalf("Failed to create temp audio file: %v", err)
	}
	audioFileURI := "file://" + audioFilePath
	srv.Resource(audioFileURI, server.WithFileContent(audioFilePath), server.WithName("Test Audio File"), server.WithMimeType("audio/octet-stream"))

	// --- Test Cases ---
	tests := []struct {
		name          string
		uri           string
		expectedKind  string
		verifyContent func(t *testing.T, contents []protocol.ResourceContents)
	}{
		{
			name:         "Read Text Resource",
			uri:          textFileURI,
			expectedKind: string(protocol.ResourceKindFile),
			verifyContent: func(t *testing.T, contents []protocol.ResourceContents) {
				if len(contents) != 1 {
					t.Fatalf("Expected 1 content part, got %d", len(contents))
				}
				textContent, ok := contents[0].(protocol.TextResourceContents)
				if !ok {
					t.Fatalf("Expected TextResourceContents, got %T", contents[0])
				}
				if textContent.Text != textFileContent {
					t.Errorf("Expected text content '%s', got '%s'", textFileContent, textContent.Text)
				}
			},
		},
		{
			name:         "Read Audio Resource",
			uri:          audioFileURI,
			expectedKind: string(protocol.ResourceKindAudio),
			verifyContent: func(t *testing.T, contents []protocol.ResourceContents) {
				if len(contents) != 1 {
					t.Fatalf("Expected 1 content part, got %d", len(contents))
				}
				audioContent, ok := contents[0].(protocol.AudioResourceContents)
				if !ok {
					t.Fatalf("Expected AudioResourceContents, got %T", contents[0])
				}
				expectedBase64 := base64.StdEncoding.EncodeToString(audioFileContent)
				if audioContent.Audio != expectedBase64 {
					t.Errorf("Expected audio base64 '%s', got '%s'", expectedBase64, audioContent.Audio)
				}
			},
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session.ClearMessages() // Clear messages from previous test iteration
			requestID := fmt.Sprintf("read-req-%d", i)

			params := protocol.ReadResourceRequestParams{URI: tc.uri}
			// Marshal params to json.RawMessage before putting in request
			paramsBytes, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			req := protocol.JSONRPCRequest{
				Method: protocol.MethodReadResource,
				ID:     requestID,
				Params: json.RawMessage(paramsBytes), // Assign raw message
			}
			reqBytes, _ := json.Marshal(req)

			handleErr := mh.HandleMessage(session, reqBytes)
			if handleErr != nil {
				t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
			}

			// Wait for the response (handleRequest runs in a goroutine)
			var responses []protocol.JSONRPCResponse
			startTime := time.Now()
			for time.Since(startTime) < 200*time.Millisecond { // Increased timeout slightly
				responses = session.GetSentResponses()
				if len(responses) > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			if len(responses) != 1 {
				t.Fatalf("Expected 1 response, got %d within timeout", len(responses))
			}
			resp := responses[0]

			if resp.Error != nil {
				t.Fatalf("Expected success, but got error: %+v", resp.Error)
			}
			if resp.ID != requestID {
				t.Errorf("Expected ID '%s', got %v", requestID, resp.ID)
			}

			// Verify the result structure
			result, ok := resp.Result.(protocol.ReadResourceResult) // Direct type assertion
			assert.True(t, ok, "Response result is not of type protocol.ReadResourceResult, type: %T", resp.Result)

			// Verify resource metadata in result
			if result.Resource.URI != tc.uri {
				t.Errorf("Expected resource URI '%s', got '%s'", tc.uri, result.Resource.URI)
			}
			if result.Resource.Kind != tc.expectedKind {
				t.Errorf("Expected resource kind '%s', got '%s'", tc.expectedKind, result.Resource.Kind)
			}

			// Verify content using the provided function
			tc.verifyContent(t, result.Contents)
		})
	}
}

func TestHandleMessage_ResourcesRead_NotFound(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Create a resources/read request for a non-existent URI
	// 3. Marshal and send via HandleMessage
	// 4. Verify error response with code ErrorCodeMCPResourceNotFound
	t.Skip("Test not implemented")
}

// --- Resource List Test (Covers existing handler) ---
func TestHandleMessage_ResourcesList(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Register multiple resources
	// 3. Create a resources/list request
	// 4. Marshal and send via HandleMessage
	// 5. Verify success response containing the registered resources
	t.Skip("Test not implemented")
}

// --- Resource Subscribe/Unsubscribe Test (Covers existing handler) ---

func TestHandleMessage_ResourcesSubscribe(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Create a resources/subscribe request for one or more URIs
	// 3. Marshal and send via HandleMessage
	// 4. Verify success response (empty result)
	// 5. Optionally, check SubscriptionManager state
	t.Skip("Test not implemented")
}

func TestHandleMessage_ResourcesUnsubscribe(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Subscribe to some URIs first
	// 3. Create a resources/unsubscribe request
	// 4. Marshal and send via HandleMessage
	// 5. Verify success response (empty result)
	// 6. Optionally, check SubscriptionManager state
	t.Skip("Test not implemented")
}

// --- Prompts List Test (Covers existing handler) ---

func TestHandleMessage_PromptsList(t *testing.T) {
	// TODO: Implement test
	// 1. Setup MH, Server, Session
	// 2. Register multiple prompts
	// 3. Create a prompts/list request
	// 4. Marshal and send via HandleMessage
	// 5. Verify success response containing the registered prompts
	t.Skip("Test not implemented")
}

// --- Completion Complete Test (Old Spec) (Compliance Plan I.4) ---

// TestHandleMessage_CompletionComplete_Success tests argument autocompletion.
func TestHandleMessage_CompletionComplete_Success(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server

	// Set capability
	srv.ImplementsCompletions = true

	// Register some test data
	srv.Resource("file:///home/user/project/main.go", server.WithTextContent("package main\n\nfunc main() {}"), server.WithName("main.go"))
	srv.Resource("file:///home/user/project/other.go", server.WithTextContent("package other"), server.WithName("other.go"))
	srv.Resource("file:///home/user/data/file.txt", server.WithTextContent("some data"), server.WithName("file.txt"))
	srv.Registry.AddPrompt(protocol.Prompt{Name: "Code Review", URI: "prompt://code-review"})
	srv.Registry.AddPrompt(protocol.Prompt{Name: "Code Generation", URI: "prompt://code-gen"})

	tests := []struct {
		name            string
		params          protocol.CompleteRequest
		expectedValues  []string
		expectedTotal   int
		expectedHasMore bool
	}{
		{
			name: "Complete prompt title",
			params: protocol.CompleteRequest{
				Ref: protocol.CompletionReference{
					Type: protocol.RefTypePrompt,
				},
				Argument: protocol.CompletionArgument{
					Name:  "promptRef", // Arg name ignored for now
					Value: "Code",      // Partial value
				},
			},
			expectedValues:  []string{"Code Generation", "Code Review"}, // Sorted alphabetically
			expectedTotal:   2,
			expectedHasMore: false,
		},
		{
			name: "Complete resource URI",
			params: protocol.CompleteRequest{
				Ref: protocol.CompletionReference{
					Type: protocol.RefTypeResource,
				},
				Argument: protocol.CompletionArgument{
					Name:  "uri",                        // Arg name ignored for now
					Value: "file:///home/user/project/", // Partial value
				},
			},
			expectedValues:  []string{"file:///home/user/project/main.go", "file:///home/user/project/other.go"}, // Sorted
			expectedTotal:   2,
			expectedHasMore: false,
		},
		{
			name: "No match",
			params: protocol.CompleteRequest{
				Ref: protocol.CompletionReference{
					Type: protocol.RefTypePrompt,
				},
				Argument: protocol.CompletionArgument{
					Name:  "promptRef",
					Value: "nonexistent",
				},
			},
			expectedValues:  []string{},
			expectedTotal:   0,
			expectedHasMore: false,
		},
		{
			name: "Empty value matches all",
			params: protocol.CompleteRequest{
				Ref: protocol.CompletionReference{
					Type: protocol.RefTypeResource,
				},
				Argument: protocol.CompletionArgument{
					Name:  "uri",
					Value: "", // Empty value
				},
			},
			// Sorted alphabetically
			expectedValues:  []string{"file:///home/user/data/file.txt", "file:///home/user/project/main.go", "file:///home/user/project/other.go"},
			expectedTotal:   3,
			expectedHasMore: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session.ClearMessages() // Clear before each subtest
			req := createRequest(t, protocol.MethodCompletionComplete, tc.params)
			reqBytes, _ := json.Marshal(req)

			handleErr := mh.HandleMessage(session, reqBytes)
			if handleErr != nil {
				t.Fatalf("HandleMessage failed unexpectedly: %v", handleErr)
			}

			respMsg := waitForResponse(t, session, req.ID, 100*time.Millisecond)
			require.NotNil(t, respMsg, "Expected a response")
			require.Nil(t, respMsg.Error, "Expected no error in response")
			require.NotNil(t, respMsg.Result, "Expected result in response")

			// Verify the result structure (CompleteResult)
			var result protocol.CompleteResult
			// Handle if Result is already unmarshalled into map[string]interface{}
			resultBytes, err := json.Marshal(respMsg.Result)
			require.NoError(t, err, "Failed to remarshal result")
			err = json.Unmarshal(resultBytes, &result)
			require.NoError(t, err, "Failed to unmarshal CompleteResult")

			// Check actual values
			assert.Equal(t, tc.expectedValues, result.Completion.Values, "Mismatch in completion values")
			require.NotNil(t, result.Completion.Total, "Expected Total field to be present")
			assert.Equal(t, tc.expectedTotal, *result.Completion.Total, "Mismatch in total count")
			require.NotNil(t, result.Completion.HasMore, "Expected HasMore field to be present")
			assert.Equal(t, tc.expectedHasMore, *result.Completion.HasMore, "Mismatch in hasMore flag")
		})
	}
}

// --- Logging Set Level Test (Covers existing handler - Compliance Plan VI) ---

func TestHandleMessage_LoggingSetLevel(t *testing.T) {
	// 1. Setup MH, Server, Session (ensure Logging capability is enabled)
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server
	// Assume client announced logging capability during initialize
	// (or ensure server doesn't check caps for this specific method if not required by spec)

	// 2. Create a logging/set_level request
	targetLevel := protocol.LogLevelDebug // Example: set to Debug
	params := protocol.SetLevelRequestParams{
		Level: targetLevel,
	}
	paramsBytes, _ := json.Marshal(params)
	req := protocol.JSONRPCRequest{
		Method: protocol.MethodLoggingSetLevel,
		ID:     "log-set-1",
		Params: json.RawMessage(paramsBytes),
	}
	reqBytes, _ := json.Marshal(req)

	// 3. Marshal and send via HandleMessage
	handleErr := mh.HandleMessage(session, reqBytes)
	require.NoError(t, handleErr, "HandleMessage failed unexpectedly")

	// 4. Verify success response (empty object)
	respMsg := waitForResponse(t, session, req.ID, 100*time.Millisecond)
	require.NotNil(t, respMsg, "Expected a response")
	require.Nil(t, respMsg.Error, "Expected no error in response")
	require.NotNil(t, respMsg.Result, "Expected result in response")

	resultBytes, _ := json.Marshal(respMsg.Result)
	require.JSONEq(t, `{}`, string(resultBytes), "Expected empty object result")

	// 5. Optionally, verify logging level was set on the server/logger if possible
	// This requires accessing the server's logger state or capturing output.
	// For now, we primarily test that the handler executes and returns success.
	// We can manually check the server log output during test runs for: "[LogX] Log level set to: debug"
	// require.Equal(t, targetLevel, srv.GetCurrentLogLevel()) // Need a getter on server
}

// --- Notification Handling Tests (Compliance Plan II & III) ---

func TestHandleNotification_Initialized(t *testing.T) {
	t.Skip("Test not implemented")
	// TODO: Test initialized notification handling
}

func TestHandleNotification_Cancelled(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server
	testReqID := "req-to-cancel-123"

	// 1. Simulate an active request by adding its cancel func
	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel() // Good practice to call cancel eventually
	mh.AddCancelFuncForTest(testReqID, cancel)

	// Verify it was added
	if _, ok := mh.GetCancelFuncForTest(testReqID); !ok {
		t.Fatalf("Test setup failed: Cancel func for %s not added", testReqID)
	}

	// 2. Create $/cancelled notification parameters
	params := protocol.CancelledParams{
		ID: testReqID, // Target the request ID we added
	}
	paramsBytes, _ := json.Marshal(params)

	// 3. Create the notification object
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0", // Correct field name
		Method:  protocol.MethodCancelled,
		Params:  json.RawMessage(paramsBytes),
	}

	// 4. Call the main message handler with the marshalled notification
	notificationBytes, _ := json.Marshal(notification)
	err := mh.HandleMessage(session, notificationBytes)
	if err != nil {
		t.Fatalf("HandleMessage returned unexpected error for cancel notification: %v", err)
	}

	// Give the handler goroutine time to process (handleNotification runs in a goroutine)
	time.Sleep(50 * time.Millisecond)

	// 5. Verify the request context was cancelled
	select {
	case <-reqCtx.Done():
		// Success: context is cancelled
		t.Logf("Context for request %s successfully cancelled.", testReqID)
	case <-time.After(100 * time.Millisecond): // Timeout to prevent test hanging
		t.Errorf("Context for request %s was not cancelled within timeout", testReqID)
	}

	// 6. Verify the cancel func was removed from the map
	if _, ok := mh.GetCancelFuncForTest(testReqID); ok {
		t.Errorf("Cancel func for %s was not removed from the map after cancellation", testReqID)
	}
}

func TestHandleNotification_ClientMessage(t *testing.T) {
	t.Parallel()
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server

	// 1. Create notifications/message parameters
	testLevel := protocol.LogLevelWarn
	testMessage := "This is a warning log from the client."
	params := protocol.LoggingMessageParams{
		Level:   testLevel,
		Message: testMessage,
		// Logger: protocol.StringPtr("client-component"), // Optional logger name
		// Data: map[string]interface{}{"detail": 123}, // Optional structured data
	}
	paramsBytes, _ := json.Marshal(params)

	// 2. Create the notification object
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  protocol.MethodNotificationMessage,
		Params:  json.RawMessage(paramsBytes),
	}

	// 3. Call the main message handler
	notificationBytes, _ := json.Marshal(notification)
	err := mh.HandleMessage(session, notificationBytes)
	if err != nil {
		t.Fatalf("HandleMessage returned unexpected error for %s notification: %v", protocol.MethodNotificationMessage, err)
	}

	// 4. Verification (Basic - Handler doesn't panic)
	// TODO: Enhance test to capture logger output (e.g., inject mock logger)
	// For now, we mainly verified that the handler runs without error.
	// We can visually inspect the test output for the expected log line:
	t.Logf("Sent %s notification. Check test output for server log message.", protocol.MethodNotificationMessage)

	// Ensure no response was sent for a notification
	if len(session.GetSentResponses()) > 0 {
		t.Errorf("Expected no responses for a notification, but got %d", len(session.GetSentResponses()))
	}
}

// --- Notification Sending Tests (Compliance Plan V) ---

func TestNotificationSending_Progress(t *testing.T) {
	// TODO: Implement test verifying SendProgress calls session.SendNotification
	// This might involve calling a method that triggers progress (like tools/call with token)
	// or calling MessageHandler.SendProgress directly if made accessible for testing.
	t.Skip("Test not implemented")
}

func TestNotificationSending_Logging(t *testing.T) {
	// TODO: Implement test verifying SendLoggingMessage calls session.SendNotification
	// Call MessageHandler.SendLoggingMessage directly if accessible.
	t.Skip("Test not implemented")
}

func TestNotificationSending_ListChanged(t *testing.T) {
	srv, session, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	// Simulate client supporting prompt list changes
	clientCaps := protocol.ClientCapabilities{
		Prompts: &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			ListChanged: true,
		},
		// Add other caps as needed for different list types (resources, tools)
	}
	// **Register session with TransportManager AND store capabilities**
	srv.TransportManager.RegisterSession(session, &clientCaps) // Register session with its capabilities
	session.StoreClientCapabilities(clientCaps)                // Ensure mock session also knows capabilities

	// --- Test Prompt List Changed ---
	session.ClearMessages()
	t.Log("Triggering prompt change...")
	srv.Prompt("Test Prompt 1", "A test prompt") // This should trigger the callback

	// Wait for the notification
	waitForNotification(t, session, 1, 100*time.Millisecond)

	notifications := session.GetSentNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}
	notif := notifications[0]
	if notif.Method != protocol.MethodNotifyPromptsListChanged {
		t.Errorf("Expected method %s, got %s", protocol.MethodNotifyPromptsListChanged, notif.Method)
	}
	// Params should be null or omitted for list_changed
	if notif.Params != nil && string(notif.Params.(json.RawMessage)) != "null" {
		t.Errorf("Expected nil or null params, got %s", string(notif.Params.(json.RawMessage)))
	}

	// --- TODO: Test Resource List Changed ---
	// 1. Set ImplementsResourceListChanged in clientCaps
	// 2. Register session again with updated caps? // Re-registering might not be needed if caps can be updated
	// 3. Clear messages
	// 4. Add/Remove resource using srv.Resource()
	// 5. Verify notifications/resources/list_changed received

	// --- TODO: Test Tool List Changed ---
	// 1. Set ImplementsToolListChanged in clientCaps
	// 2. Register session again with updated caps?
	// 3. Clear messages
	// 4. Add/Remove tool using srv.Tool()
	// 5. Verify notifications/tools/list_changed received

	// --- TODO: Test Client Does NOT Support Notification ---
	// 1. Set corresponding ListChanged flag to false in caps
	// 2. Register session
	// 3. Trigger change
	// 4. Verify NO notification is received (check count after a short delay)
}

// --- Initialize Tests (REMOVED - Moved to lifecycle_handler_test.go) ---
// TODO: Add placeholder tests for methods in compliance plan (ping, prompts/get, etc.)

// --- Duplicate SendRequest removed below ---
/* // Removing duplicate
// SendRequest stores the request for test inspection.
func (m *mockClientSession) SendRequest(request protocol.JSONRPCRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	m.sentRequests = append(m.sentRequests, request)
	return nil
}
*/

func TestHandleMessage_ToolsCall(t *testing.T) {
	t.Parallel()
	const toolName = "test_tool"
	const toolInput = `{}`
	const requestID = "req-tools-call-1"

	srv, mockSession, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler // Get handler from server
	// Ensure session is registered for TransportManager lookups if needed
	srv.TransportManager.RegisterSession(mockSession, &protocol.ClientCapabilities{}) // Register with empty caps

	// Add a mock tool using the server's Tool method
	testToolFn := func(ctx *server.Context, input json.RawMessage) (interface{}, error) {
		// Simulate successful execution
		if string(input) != toolInput {
			return nil, fmt.Errorf("unexpected input: %s, expected: %s", string(input), toolInput)
		}
		return map[string]string{"result": "success"}, nil // Return a map, handler will marshal
	}
	srv.Tool(toolName, "A test tool", testToolFn) // Use server method to register

	// Prepare parameters
	toolCallParams := protocol.CallToolRequestParams{
		ToolCall: &protocol.ToolCall{
			ID:       "call-1",
			ToolName: toolName,
			Input:    json.RawMessage(toolInput),
		},
	}
	paramsBytes, err := json.Marshal(toolCallParams) // Use standard marshal
	if err != nil {
		t.Fatalf("Failed to marshal tool call params: %v", err)
	}

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  protocol.MethodCallTool,
		Params:  json.RawMessage(paramsBytes),
	}
	reqBytes, _ := json.Marshal(req)

	// --- Test Success ---
	t.Run("Success", func(t *testing.T) {
		err := mh.HandleMessage(mockSession, reqBytes)
		require.NoError(t, err)

		respMsg := waitForResponse(t, mockSession, requestID, 200*time.Millisecond)
		require.NotNil(t, respMsg, "Expected a response")
		require.Nil(t, respMsg.Error, "Expected no error in response")
		require.NotNil(t, respMsg.Result, "Expected result in response")

		// Directly assert the type of the result
		result, ok := respMsg.Result.(protocol.CallToolResult)
		require.True(t, ok, "Result is not of type CallToolResult")

		// Now check the fields of the asserted result
		require.Equal(t, "call-1", result.ToolCallID)
		require.JSONEq(t, `{"result": "success"}`, string(result.Output))
		require.Nil(t, result.Error, "Expected no tool error")
	})

	// --- Test Tool Not Found ---
	t.Run("ToolNotFound", func(t *testing.T) {
		mockSession.ClearMessages() // Clear previous response

		// Prepare params for non-existent tool
		toolCallParamsNotFound := protocol.CallToolRequestParams{
			ToolCall: &protocol.ToolCall{
				ID:       "call-2",
				ToolName: "non_existent_tool",
				Input:    json.RawMessage("{}"),
			},
		}
		paramsNotFoundBytes, err := json.Marshal(toolCallParamsNotFound) // Use standard marshal
		if err != nil {
			t.Fatalf("Failed to marshal not found params: %v", err)
		}

		reqNotFound := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "req-tools-call-notfound",
			Method:  protocol.MethodCallTool,
			Params:  json.RawMessage(paramsNotFoundBytes),
		}
		reqNotFoundBytes, _ := json.Marshal(reqNotFound)

		err = mh.HandleMessage(mockSession, reqNotFoundBytes)
		require.NoError(t, err)

		respMsg := waitForResponse(t, mockSession, "req-tools-call-notfound", 200*time.Millisecond)
		require.NotNil(t, respMsg, "Expected a response")
		require.Nil(t, respMsg.Error, "Expected no top-level JSON-RPC error") // Error is inside the result
		require.NotNil(t, respMsg.Result, "Expected result (containing error) in response")

		// Directly assert the type of the result
		result, ok := respMsg.Result.(protocol.CallToolResult)
		require.True(t, ok, "Error Result is not of type CallToolResult")

		// Now check the fields of the asserted result
		require.Equal(t, "call-2", result.ToolCallID)
		require.Nil(t, result.Output, "Expected no tool output")
		require.NotNil(t, result.Error, "Expected tool error")
		require.Equal(t, protocol.CodeMCPToolNotFound, result.Error.Code) // Check specific error code
		require.Contains(t, result.Error.Message, "not found")
	})

	// --- Test Invalid Params ---
	t.Run("InvalidParams", func(t *testing.T) {
		mockSession.ClearMessages() // Clear previous response
		reqInvalidParams := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "req-tools-call-invalid",
			Method:  protocol.MethodCallTool,
			Params:  json.RawMessage(`{"invalid": "structure"}`), // Not CallToolRequestParams
		}
		reqInvalidParamsBytes, _ := json.Marshal(reqInvalidParams)

		err := mh.HandleMessage(mockSession, reqInvalidParamsBytes)
		require.NoError(t, err) // Handler should handle gracefully

		respMsg := waitForResponse(t, mockSession, "req-tools-call-invalid", 200*time.Millisecond)
		require.NotNil(t, respMsg, "Expected a response")
		require.NotNil(t, respMsg.Error, "Expected error in response")
		require.Nil(t, respMsg.Result, "Expected no result")
		require.Equal(t, protocol.CodeInvalidParams, respMsg.Error.Code) // Corrected constant
	})
}

// --- Test for V2024 Tool Call Compliance ---

func TestHandleMessage_ToolsCall_V2024Compat(t *testing.T) {
	t.Parallel()
	const toolName = "test_tool_v2024"
	const toolInput = `{}`
	const requestID = "req-tools-call-v2024-1"

	srv, mockSession, cleanup := setupTestServer(t) // Use new setup
	defer cleanup()
	mh := srv.MessageHandler                                                          // Get handler from server
	mockSession.SetNegotiatedVersion(protocol.OldProtocolVersion)                     // Set old version!
	srv.TransportManager.RegisterSession(mockSession, &protocol.ClientCapabilities{}) // Register with empty caps

	// Add a mock tool
	testToolFn := func(ctx *server.Context, input json.RawMessage) (interface{}, error) {
		return map[string]string{"result": "v2024_success"}, nil
	}
	srv.Tool(toolName, "A test tool (V2024)", testToolFn)

	// Prepare parameters
	toolCallParams := protocol.CallToolRequestParams{
		ToolCall: &protocol.ToolCall{
			ID:       "call-v2024-1",
			ToolName: toolName,
			Input:    json.RawMessage(toolInput),
		},
	}
	paramsBytes, err := json.Marshal(toolCallParams)
	require.NoError(t, err)

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  protocol.MethodCallTool,
		Params:  json.RawMessage(paramsBytes),
	}
	reqBytes, _ := json.Marshal(req)

	// --- Test Success (V2024 Format) ---
	t.Run("SuccessV2024", func(t *testing.T) {
		err := mh.HandleMessage(mockSession, reqBytes)
		require.NoError(t, err)

		respMsg := waitForResponse(t, mockSession, requestID, 200*time.Millisecond)
		require.NotNil(t, respMsg, "Expected a response")
		require.Nil(t, respMsg.Error, "Expected no top-level error in response")
		require.NotNil(t, respMsg.Result, "Expected result in response")

		// Directly assert the type of the result
		result, ok := respMsg.Result.(protocol.CallToolResultV2024)
		require.True(t, ok, "Result is not of type CallToolResultV2024")

		// Now check the fields of the asserted result
		require.Equal(t, "call-v2024-1", result.ToolCallID)
		require.False(t, result.IsError, "Expected IsError to be false")
		require.Len(t, result.Content, 1, "Expected 1 content item")

		// Check the content (assuming helper converted map to JSON string in TextContent)
		textContent, ok := result.Content[0].(protocol.TextContent)
		require.True(t, ok, "Expected TextContent")
		require.JSONEq(t, `{"result": "v2024_success"}`, textContent.Text)
	})

	// --- Test Error (V2024 Format) ---
	t.Run("ErrorV2024", func(t *testing.T) {
		mockSession.ClearMessages()

		// Register a tool that returns an error
		const errorToolName = "error_tool_v2024"
		srv.Tool(errorToolName, "Tool that errors (V2024)", func(ctx *server.Context, input json.RawMessage) (interface{}, error) {
			return nil, fmt.Errorf("tool failed intentionally")
		})

		// Prepare params
		toolCallParamsErr := protocol.CallToolRequestParams{
			ToolCall: &protocol.ToolCall{
				ID:       "call-err-v2024-1",
				ToolName: errorToolName,
				Input:    json.RawMessage(toolInput),
			},
		}
		paramsErrBytes, err := json.Marshal(toolCallParamsErr)
		require.NoError(t, err)
		reqErr := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "req-tool-err-v2024",
			Method:  protocol.MethodCallTool,
			Params:  json.RawMessage(paramsErrBytes),
		}
		reqErrBytes, _ := json.Marshal(reqErr)

		err = mh.HandleMessage(mockSession, reqErrBytes)
		require.NoError(t, err)

		respMsg := waitForResponse(t, mockSession, "req-tool-err-v2024", 200*time.Millisecond)
		require.NotNil(t, respMsg, "Expected a response")
		require.Nil(t, respMsg.Error, "Expected no top-level error in response")
		require.NotNil(t, respMsg.Result, "Expected result in response")

		// Directly assert the type of the result
		result, ok := respMsg.Result.(protocol.CallToolResultV2024)
		require.True(t, ok, "Error Result is not of type CallToolResultV2024")

		// Now check the fields of the asserted result
		require.Equal(t, "call-err-v2024-1", result.ToolCallID)
		require.True(t, result.IsError, "Expected IsError to be true")
		require.Len(t, result.Content, 1, "Expected 1 content item for error message")

		textContent, ok := result.Content[0].(protocol.TextContent)
		require.True(t, ok, "Expected TextContent for error")
		require.Contains(t, textContent.Text, "tool failed intentionally")
	})
}

func TestHandleMessage_ResourcesRead_TemplateSuccess(t *testing.T) {
	srv, mockSession, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Register a template handler
	pattern := "test://data/{itemID}/{format}"
	handler := func(ctx interface{}, format string, itemID string) (string, error) {
		// Simple handler returning formatted string
		if format == "json" {
			return fmt.Sprintf(`{"id": "%s", "value": "Data for %s"}`, itemID, itemID), nil
		}
		return fmt.Sprintf("Data for item %s in format %s", itemID, format), nil
	}
	srv.Resource(pattern, server.WithHandler(handler)) // Use the new Server.Resource method with WithHandler option
	// Error handling for template registration is now internal to Server.Resource

	// 2. Send resources/read request matching the template
	requestURI := "test://data/item123/text"
	params := protocol.ReadResourceRequestParams{URI: requestURI}
	req := createRequest(t, protocol.MethodReadResource, params)
	reqBytes, _ := json.Marshal(req)

	err := srv.MessageHandler.HandleMessage(mockSession, reqBytes)
	assert.NoError(t, err, "HandleMessage returned an error")

	// 3. Wait for and verify the response
	resp := waitForResponse(t, mockSession, req.ID, 1*time.Second)
	assert.Nil(t, resp.Error, "Expected no error in response")
	assert.NotNil(t, resp.Result, "Expected result to be non-nil")

	// 4. Unmarshal and check the result structure
	var result protocol.ReadResourceResult
	// Handle if Result is already unmarshalled
	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err, "Failed to remarshal result")
	err = json.Unmarshal(resultBytes, &result) // Unmarshal from asserted bytes
	assert.NoError(t, err, "Failed to unmarshal result")

	assert.Equal(t, requestURI, result.Resource.URI) // Should reflect the requested URI
	assert.Equal(t, "dynamic", result.Resource.Kind) // Kind should indicate it was dynamically generated
	assert.Len(t, result.Contents, 1, "Expected exactly one content part")

	// 5. Check the content generated by the handler
	textContent, ok := result.Contents[0].(protocol.TextResourceContents)
	assert.True(t, ok, "Expected content to be TextResourceContents")
	expectedContent := "Data for item item123 in format text"
	assert.Equal(t, expectedContent, textContent.Text)
	assert.Equal(t, "text/plain", textContent.ContentType) // Default from conversion
}

func TestHandleMessage_ResourcesRead_TemplateNotFound(t *testing.T) {
	srv, mockSession, cleanup := setupTestServer(t)
	defer cleanup()

	// No templates registered

	requestURI := "test://nonexistent/template/path"
	params := protocol.ReadResourceRequestParams{URI: requestURI}
	req := createRequest(t, protocol.MethodReadResource, params)
	reqBytes, _ := json.Marshal(req)

	err := srv.MessageHandler.HandleMessage(mockSession, reqBytes)
	assert.NoError(t, err, "HandleMessage returned an error")

	resp := waitForResponse(t, mockSession, req.ID, 1*time.Second)
	assert.NotNil(t, resp.Error, "Expected an error response")
	assert.Nil(t, resp.Result, "Expected result to be nil")
	assert.Equal(t, int(protocol.CodeMCPResourceNotFound), int(resp.Error.Code)) // Use int cast
	assert.Contains(t, resp.Error.Message, "Resource not found")
}

// TODO: Add test for template handler returning an error
// TODO: Add test for template handler returning []byte
// TODO: Add test for template handler returning struct (JSON)
// TODO: Add test for template handler requiring int param conversion

// --- Notification Handling Tests ---

// ... existing code ...
