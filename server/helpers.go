package server

import (
	"github.com/localrivet/gomcp/protocol"
)

// Text creates a protocol.Content of type text.
func Text(msg string) protocol.Content {
	// Create a TextContent struct and return it as a Content interface
	return protocol.TextContent{
		Type: "text",
		Text: msg,
	}
}

// Define helper functions for prompt messages
func System(msg string) protocol.PromptMessage {
	// Assuming Text exists and works with protocol.Content
	return protocol.PromptMessage{Role: "system", Content: []protocol.Content{Text(msg)}}
}

func User(msg string) protocol.PromptMessage {
	// Assuming Text exists and works with protocol.Content
	return protocol.PromptMessage{Role: "user", Content: []protocol.Content{Text(msg)}}
}
