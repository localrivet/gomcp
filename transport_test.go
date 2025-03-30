package gomcp

import (
	"bytes"
	"encoding/json"
	"errors" // Import errors package for errors.Is
	"fmt"
	"io"
	"strings"
	"testing"
)

// TestSendMessage verifies that SendMessage correctly marshals and writes a message
// with a newline delimiter.
func TestSendMessage(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConnection(&bytes.Buffer{}, &buf) // Reader not used here

	// Use InitializeRequestParams as payload example
	payload := InitializeRequestParams{
		ProtocolVersion: CurrentProtocolVersion,
		ClientInfo:      Implementation{Name: "TestClient", Version: "0.1"},
		Capabilities:    ClientCapabilities{},
	}

	// Send using SendRequest
	reqID, err := conn.SendRequest(MethodInitialize, payload)
	if err != nil {
		t.Fatalf("SendRequest failed for InitializeRequest: %v", err)
	}
	if reqID == "" {
		t.Fatal("SendRequest returned an empty request ID")
	}

	// Check the output buffer
	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("Expected output to end with newline, got %q", output)
	}

	// Trim newline for JSON parsing
	jsonData := strings.TrimSuffix(output, "\n")

	// Unmarshal into JSONRPCRequest and verify fields
	var receivedReq JSONRPCRequest
	err = json.Unmarshal([]byte(jsonData), &receivedReq)
	if err != nil {
		t.Fatalf("Failed to unmarshal sent message into JSONRPCRequest: %v\nOriginal JSON: %s", err, jsonData)
	}

	// Check JSON-RPC fields
	if receivedReq.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc %q, got %q", "2.0", receivedReq.JSONRPC)
	}
	if receivedReq.Method != MethodInitialize {
		t.Errorf("Expected method %q, got %q", MethodInitialize, receivedReq.Method)
	}
	if receivedReq.ID == nil { // ID should not be nil for requests
		t.Error("Expected non-nil ID, got nil")
	}
	// Check if ID matches the one returned by SendRequest (optional but good)
	if idStr, ok := receivedReq.ID.(string); !ok || idStr != reqID {
		t.Errorf("Expected ID %q, got %q", reqID, receivedReq.ID)
	}

	// Unmarshal and verify params (InitializeRequestParams)
	var receivedParams InitializeRequestParams
	// Params field is interface{}, needs type assertion or re-marshal/unmarshal
	paramsMap, ok := receivedReq.Params.(map[string]interface{})
	if !ok {
		t.Fatalf("Params is not a map[string]interface{}, type is %T", receivedReq.Params)
	}
	paramsBytes, err := json.Marshal(paramsMap)
	if err != nil {
		t.Fatalf("Failed to re-marshal params map: %v", err)
	}
	err = json.Unmarshal(paramsBytes, &receivedParams)
	if err != nil {
		t.Fatalf("Failed to unmarshal params map into struct: %v", err)
	}

	// Check fields of InitializeRequestParams
	if receivedParams.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected params protocol version %q, got %q", CurrentProtocolVersion, receivedParams.ProtocolVersion)
	}
	if receivedParams.ClientInfo.Name != "TestClient" {
		t.Errorf("Expected client name %q, got %q", "TestClient", receivedParams.ClientInfo.Name)
	}
	// Add checks for other fields if necessary (e.g., Capabilities)
}

// TestReceiveMessage verifies that ReceiveMessage correctly reads a newline-delimited
// JSON message and returns the raw byte slice.
func TestReceiveRawMessage(t *testing.T) { // Renamed
	// Prepare a JSON-RPC Response to be received
	testID := "req-123"
	initResultPayload := InitializeResult{
		ProtocolVersion: CurrentProtocolVersion,
		ServerInfo:      Implementation{Name: "TestServer", Version: "0.1"},
		Capabilities:    ServerCapabilities{},
	}
	respToSend := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      testID,
		Result:  initResultPayload, // Embed the result directly
	}
	jsonData, _ := json.Marshal(respToSend)
	inputJson := string(jsonData) + "\n" // Add newline

	conn := NewConnection(strings.NewReader(inputJson), &bytes.Buffer{}) // Writer not used

	// ReceiveMessage currently tries to parse into gomcp.Message, which will fail
	// for a standard JSON-RPC response.
	// ReceiveRawMessage should return the raw bytes.
	rawBytes, err := conn.ReceiveRawMessage() // Use ReceiveRawMessage
	if err != nil {
		t.Fatalf("ReceiveRawMessage failed: %v", err)
	}

	// Verify the received bytes match the input (excluding newline)
	expectedBytes := []byte(strings.TrimSuffix(inputJson, "\n"))
	// Trim newline from received bytes before comparing
	receivedTrimmed := bytes.TrimSuffix(rawBytes, []byte("\n"))
	if !bytes.Equal(receivedTrimmed, expectedBytes) {
		t.Errorf("Expected raw bytes %q, got %q", string(expectedBytes), string(receivedTrimmed))
	}
	// Further parsing into JSONRPCResponse would happen in the caller (client/server)
}

// TestReceiveRawMessageEOF tests that ReceiveRawMessage returns an error containing io.EOF
// when the reader reaches EOF.
func TestReceiveRawMessageEOF(t *testing.T) { // Renamed function
	conn := NewConnection(strings.NewReader(""), &bytes.Buffer{}) // Empty reader

	_, err := conn.ReceiveRawMessage() // Use ReceiveRawMessage
	if err == nil {
		t.Fatal("Expected an error on EOF, got nil")
	}

	// Use errors.Is for robust EOF checking (requires Go 1.13+)
	if !errors.Is(err, io.EOF) {
		// Check wrapped error message as fallback
		if !strings.Contains(err.Error(), io.EOF.Error()) {
			t.Errorf("Expected error wrapping io.EOF, got: %v", err)
		}
	}
}

// TestReceiveRawMessageInvalidJSON tests receiving malformed JSON.
func TestReceiveRawMessageInvalidJSON(t *testing.T) { // Renamed
	invalidJson := "{not json\n"
	var serverOutput bytes.Buffer // Capture error message sent back
	conn := NewConnection(strings.NewReader(invalidJson), &serverOutput)

	_, err := conn.ReceiveRawMessage() // Use ReceiveRawMessage
	if err == nil {
		t.Fatal("Expected an error on invalid JSON, got nil")
	}
	// Check if the error is the expected one from ReceiveRawMessage
	if !strings.Contains(err.Error(), "received invalid JSON") {
		t.Errorf("Expected error containing 'received invalid JSON', got: %v", err)
	}

	// Check if the server attempted to send an Error message back
	outputStr := serverOutput.String()
	// Check for the numeric code in the output string
	// Note: The check for MessageTypeError might become obsolete if we switch to pure JSON-RPC error format
	if !strings.Contains(outputStr, fmt.Sprintf(`"code":%d`, ErrorCodeParseError)) {
		t.Errorf("Expected server to send back an Error message with code %d, got: %q", ErrorCodeParseError, outputStr)
	}
}

// TestReceiveRawMessageMissingFields tests receiving valid JSON but missing required JSON-RPC fields.
// Note: ReceiveRawMessage itself doesn't validate fields anymore, so this test mainly checks if valid JSON is read.
// The validation logic is now expected in the caller (client/server).
func TestReceiveRawMessageMissingFields(t *testing.T) { // Renamed
	missingFieldsJson := `{"params": {}}` + "\n" // Missing jsonrpc, method/id
	var serverOutput bytes.Buffer                // Capture potential error message sent back
	conn := NewConnection(strings.NewReader(missingFieldsJson), &serverOutput)

	rawBytes, err := conn.ReceiveRawMessage() // Use ReceiveRawMessage
	if err != nil {
		t.Fatalf("ReceiveRawMessage failed unexpectedly for valid JSON: %v", err)
	}

	// Verify the raw bytes were read correctly
	expectedBytes := []byte(strings.TrimSuffix(missingFieldsJson, "\n"))
	// Trim newline from received bytes before comparing
	receivedTrimmed := bytes.TrimSuffix(rawBytes, []byte("\n"))
	if !bytes.Equal(receivedTrimmed, expectedBytes) {
		t.Errorf("Expected raw bytes %q, got %q", string(expectedBytes), string(receivedTrimmed))
	}

	// The serverOutput buffer should be empty because ReceiveRawMessage doesn't send errors for missing fields.
	if outputStr := serverOutput.String(); outputStr != "" {
		t.Errorf("Expected no error message to be sent by ReceiveRawMessage for missing fields, but got: %q", outputStr)
	}
}
