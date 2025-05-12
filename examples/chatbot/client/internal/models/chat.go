package models

import (
	"time"
)

// ChatMessage represents a message in the chat history
type ChatMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// ToolCall represents a tool call made by the LLM
type ToolCall struct {
	Name      string
	Arguments map[string]interface{}
	Response  string
}
