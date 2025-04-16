// Package response provides utilities for creating MCP tool responses.
package response

import (
	"encoding/json"

	"github.com/localrivet/gomcp/protocol"
)

// Error creates an error response with the given message.
func Error(msg string) ([]protocol.Content, bool) {
	return []protocol.Content{
		protocol.TextContent{Type: "text", Text: msg},
	}, true
}

// JSON creates a JSON response from the given value.
func JSON(v interface{}) ([]protocol.Content, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return Error("Failed to marshal response: " + err.Error())
	}
	return []protocol.Content{
		protocol.TextContent{Type: "json", Text: string(b)},
	}, false
}

// Text creates a text response.
func Text(msg string) ([]protocol.Content, bool) {
	return []protocol.Content{
		protocol.TextContent{Type: "text", Text: msg},
	}, false
}

// Success creates a success response with the given message.
func Success(msg string) ([]protocol.Content, bool) {
	return Text(msg)
}
