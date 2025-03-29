package mcp

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

	// Send using MethodInitialize
	err := conn.SendMessage(MethodInitialize, payload)
	if err != nil {
		t.Fatalf("SendMessage failed for InitializeRequest: %v", err)
	}

	// Check the output buffer
	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("Expected output to end with newline, got %q", output)
	}

	// Trim newline for JSON parsing
	jsonData := strings.TrimSuffix(output, "\n")

	// Unmarshal and verify basic fields
	var receivedMsg Message
	err = json.Unmarshal([]byte(jsonData), &receivedMsg)
	if err != nil {
		t.Fatalf("Failed to unmarshal sent message: %v\nOriginal JSON: %s", err, jsonData)
	}

	// Check the method name used in the message
	if receivedMsg.MessageType != MethodInitialize {
		t.Errorf("Expected message type (method) %q, got %q", MethodInitialize, receivedMsg.MessageType)
	}
	if receivedMsg.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected protocol version %q, got %q", CurrentProtocolVersion, receivedMsg.ProtocolVersion)
	}
	if receivedMsg.MessageID == "" {
		t.Error("Expected non-empty MessageID, got empty string")
	}

	// Unmarshal and verify payload (now InitializeRequestParams)
	var receivedPayload InitializeRequestParams
	// When json.Unmarshal decodes into an interface{} (like Message.Payload),
	// it uses map[string]interface{} for JSON objects. We need to handle this.
	// One way is to re-marshal the map and unmarshal into the target struct.
	payloadMap, ok := receivedMsg.Payload.(map[string]interface{})
	if !ok {
		// If the payload wasn't a JSON object, this might fail. Adjust if needed.
		t.Fatalf("Payload is not a map[string]interface{}, type is %T", receivedMsg.Payload)
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		t.Fatalf("Failed to re-marshal payload map: %v", err)
	}
	err = json.Unmarshal(payloadBytes, &receivedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload map into struct: %v", err)
	}

	// Check fields of InitializeRequestParams
	if receivedPayload.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected payload protocol version %q, got %q", CurrentProtocolVersion, receivedPayload.ProtocolVersion)
	}
	if receivedPayload.ClientInfo.Name != "TestClient" {
		t.Errorf("Expected client name %q, got %q", "TestClient", receivedPayload.ClientInfo.Name)
	}
	// Add checks for other fields if necessary (e.g., Capabilities)
}

// TestReceiveMessage verifies that ReceiveMessage correctly reads a newline-delimited
// JSON message and returns the generic Message struct with RawMessage payload.
func TestReceiveMessage(t *testing.T) {
	// Prepare a message to be received (simulate InitializeResponse)
	// Note: Real InitializeResponse is JSON-RPC, not MCP Message format.
	// This test verifies ReceiveMessage can parse the *content*, assuming
	// the transport layer somehow provides it within the Message struct for now.
	initResultPayload := InitializeResult{
		ProtocolVersion: CurrentProtocolVersion,
		ServerInfo:      Implementation{Name: "TestServer", Version: "0.1"},
		Capabilities:    ServerCapabilities{}, // Add basic capabilities
	}
	// We still wrap it in Message for ReceiveMessage to parse currently
	msgToSend := Message{
		ProtocolVersion: CurrentProtocolVersion,
		MessageID:       "test-uuid",          // JSON-RPC response needs matching ID
		MessageType:     "InitializeResponse", // Conceptual type for now
		Payload:         initResultPayload,
	}
	jsonData, _ := json.Marshal(msgToSend)
	inputJson := string(jsonData) + "\n" // Add newline

	conn := NewConnection(strings.NewReader(inputJson), &bytes.Buffer{}) // Writer not used

	receivedMsg, err := conn.ReceiveMessage()
	if err != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}

	// Verify basic fields
	// TODO: Update this check once SendMessage/ReceiveMessage handle JSON-RPC properly.
	// For now, check the conceptual type we sent.
	if receivedMsg.MessageType != "InitializeResponse" { // Check conceptual type
		t.Errorf("Expected message type %q, got %q", "InitializeResponse", receivedMsg.MessageType)
	}
	if receivedMsg.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected protocol version %q, got %q", CurrentProtocolVersion, receivedMsg.ProtocolVersion)
	}
	if receivedMsg.MessageID != "test-uuid" {
		t.Errorf("Expected message ID %q, got %q", "test-uuid", receivedMsg.MessageID)
	}

	// Verify payload is RawMessage and can be unmarshalled
	rawPayload, ok := receivedMsg.Payload.(json.RawMessage)
	if !ok {
		t.Fatalf("Expected payload to be json.RawMessage, got %T", receivedMsg.Payload)
	}

	// Unmarshal into InitializeResult
	var receivedPayload InitializeResult
	err = UnmarshalPayload(rawPayload, &receivedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal RawMessage payload into InitializeResult: %v", err)
	}

	// Check fields of InitializeResult
	if receivedPayload.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected payload protocol version %q, got %q", CurrentProtocolVersion, receivedPayload.ProtocolVersion)
	}
	if receivedPayload.ServerInfo.Name != "TestServer" {
		t.Errorf("Expected server name %q, got %q", "TestServer", receivedPayload.ServerInfo.Name)
	}
	// Check other fields like Capabilities if necessary
}

// TestReceiveMessageEOF tests that ReceiveMessage returns an error containing io.EOF
// when the reader reaches EOF.
func TestReceiveMessageEOF(t *testing.T) {
	conn := NewConnection(strings.NewReader(""), &bytes.Buffer{}) // Empty reader

	_, err := conn.ReceiveMessage()
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

// TestReceiveMessageInvalidJSON tests receiving malformed JSON.
func TestReceiveMessageInvalidJSON(t *testing.T) {
	invalidJson := "{not json\n"
	var serverOutput bytes.Buffer // Capture error message sent back
	conn := NewConnection(strings.NewReader(invalidJson), &serverOutput)

	_, err := conn.ReceiveMessage()
	if err == nil {
		t.Fatal("Expected an error on invalid JSON, got nil")
	}
	// Check if it's a json syntax error
	var syntaxError *json.SyntaxError
	if !errors.As(err, &syntaxError) && !strings.Contains(err.Error(), "failed to unmarshal message") {
		t.Errorf("Expected JSON syntax error or unmarshal error, got: %v", err)
	}

	// Check if the server attempted to send an Error message back
	outputStr := serverOutput.String()
	// Check for the numeric code in the output string
	// Note: The check for MessageTypeError might become obsolete if we switch to pure JSON-RPC error format
	if !strings.Contains(outputStr, fmt.Sprintf(`"code":%d`, ErrorCodeParseError)) {
		t.Errorf("Expected server to send back an Error message with code %d, got: %q", ErrorCodeParseError, outputStr)
	}
}

// TestReceiveMessageMissingFields tests receiving valid JSON but missing required MCP fields.
func TestReceiveMessageMissingFields(t *testing.T) {
	missingFieldsJson := `{"payload": {}}` + "\n" // Missing version, id, type
	var serverOutput bytes.Buffer                 // Capture error message sent back
	conn := NewConnection(strings.NewReader(missingFieldsJson), &serverOutput)

	_, err := conn.ReceiveMessage()
	if err == nil {
		t.Fatal("Expected an error on missing fields, got nil")
	}
	if !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("Expected error related to missing fields, got: %v", err)
	}

	// Check if the server attempted to send an Error message back
	outputStr := serverOutput.String()
	// Check for the numeric code in the output string
	if !strings.Contains(outputStr, MessageTypeError) || !strings.Contains(outputStr, fmt.Sprintf(`"code":%d`, ErrorCodeMCPInvalidMessage)) {
		t.Errorf("Expected server to send back an Error message with code %d, got: %q", ErrorCodeMCPInvalidMessage, outputStr)
	}
}
