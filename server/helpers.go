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
// Note: "system" role is not valid according to MCP schemas - only "user" and "assistant" are allowed
func System(msg string) protocol.PromptMessage {
	// Using "user" role instead of "system" to comply with schemas
	return protocol.PromptMessage{
		Role:    "user",
		Content: Text(msg), // Single Content object, not an array
	}
}

func User(msg string) protocol.PromptMessage {
	return protocol.PromptMessage{
		Role:    "user",
		Content: Text(msg), // Single Content object, not an array
	}
}

func Assistant(msg string) protocol.PromptMessage {
	return protocol.PromptMessage{
		Role:    "assistant",
		Content: Text(msg), // Single Content object, not an array
	}
}
