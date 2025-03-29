package mcp

import (
	"bytes"
	"encoding/json"
	"errors" // Import errors package for errors.Is
	"io"
	"strings"
	"testing"
)

// TestSendMessage verifies that SendMessage correctly marshals and writes a message
// with a newline delimiter.
func TestSendMessage(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConnection(&bytes.Buffer{}, &buf) // Reader not used here

	payload := HandshakeRequestPayload{
		SupportedProtocolVersions: []string{"1.0"},
		ClientName:                "TestClient",
	}

	err := conn.SendMessage(MessageTypeHandshakeRequest, payload)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
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

	if receivedMsg.MessageType != MessageTypeHandshakeRequest {
		t.Errorf("Expected message type %q, got %q", MessageTypeHandshakeRequest, receivedMsg.MessageType)
	}
	if receivedMsg.ProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected protocol version %q, got %q", CurrentProtocolVersion, receivedMsg.ProtocolVersion)
	}
	if receivedMsg.MessageID == "" {
		t.Error("Expected non-empty MessageID, got empty string")
	}

	// Unmarshal and verify payload
	var receivedPayload HandshakeRequestPayload
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

	if len(receivedPayload.SupportedProtocolVersions) != 1 || receivedPayload.SupportedProtocolVersions[0] != "1.0" {
		t.Errorf("Expected supported versions %v, got %v", []string{"1.0"}, receivedPayload.SupportedProtocolVersions)
	}
	if receivedPayload.ClientName != "TestClient" {
		t.Errorf("Expected client name %q, got %q", "TestClient", receivedPayload.ClientName)
	}
}

// TestReceiveMessage verifies that ReceiveMessage correctly reads a newline-delimited
// JSON message and returns the generic Message struct with RawMessage payload.
func TestReceiveMessage(t *testing.T) {
	// Prepare a message to be received
	msgToSend := Message{
		ProtocolVersion: CurrentProtocolVersion,
		MessageID:       "test-uuid",
		MessageType:     MessageTypeHandshakeResponse,
		Payload: HandshakeResponsePayload{
			SelectedProtocolVersion: CurrentProtocolVersion,
			ServerName:              "TestServer",
		},
	}
	jsonData, _ := json.Marshal(msgToSend)
	inputJson := string(jsonData) + "\n" // Add newline

	conn := NewConnection(strings.NewReader(inputJson), &bytes.Buffer{}) // Writer not used

	receivedMsg, err := conn.ReceiveMessage()
	if err != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}

	// Verify basic fields
	if receivedMsg.MessageType != MessageTypeHandshakeResponse {
		t.Errorf("Expected message type %q, got %q", MessageTypeHandshakeResponse, receivedMsg.MessageType)
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

	var receivedPayload HandshakeResponsePayload
	err = UnmarshalPayload(rawPayload, &receivedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal RawMessage payload: %v", err)
	}

	if receivedPayload.SelectedProtocolVersion != CurrentProtocolVersion {
		t.Errorf("Expected selected version %q, got %q", CurrentProtocolVersion, receivedPayload.SelectedProtocolVersion)
	}
	if receivedPayload.ServerName != "TestServer" {
		t.Errorf("Expected server name %q, got %q", "TestServer", receivedPayload.ServerName)
	}
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
	if !strings.Contains(outputStr, MessageTypeError) || !strings.Contains(outputStr, "InvalidJSON") {
		t.Errorf("Expected server to send back an InvalidJSON Error message, got: %q", outputStr)
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
	if !strings.Contains(outputStr, MessageTypeError) || !strings.Contains(outputStr, "InvalidMessage") {
		t.Errorf("Expected server to send back an InvalidMessage Error message, got: %q", outputStr)
	}
}
