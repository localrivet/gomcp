// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json"
	"fmt"
	"log"
)

// --- Prompt Structures ---

// PromptArgument defines an input parameter for a prompt template.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // e.g., "string", "number", "boolean"
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage represents a single message within a prompt sequence.
type PromptMessage struct {
	Role    string    `json:"role"`    // e.g., "system", "user", "assistant"
	Content []Content `json:"content"` // Defined in messages.go
}

// UnmarshalJSON implements custom unmarshalling for PromptMessage to handle the Content interface slice.
func (pm *PromptMessage) UnmarshalJSON(data []byte) error {
	type Alias PromptMessage
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(pm),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal base PromptMessage: %w", err)
	}
	pm.Content = make([]Content, 0, len(aux.Content))
	for _, raw := range aux.Content {
		var typeDetect struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typeDetect); err != nil {
			var tc TextContent // Defined in messages.go
			if errText := json.Unmarshal(raw, &tc); errText == nil && tc.Type == "text" {
				pm.Content = append(pm.Content, tc)
				continue
			}
			return fmt.Errorf("failed to detect content type in prompt message: %w", err)
		}
		var actualContent Content
		switch typeDetect.Type {
		case "text":
			var tc TextContent // Defined in messages.go
			if err := json.Unmarshal(raw, &tc); err != nil {
				return fmt.Errorf("failed to unmarshal TextContent in prompt message: %w", err)
			}
			actualContent = tc
		case "image":
			var ic ImageContent // Defined in messages.go
			if err := json.Unmarshal(raw, &ic); err != nil {
				return fmt.Errorf("failed to unmarshal ImageContent in prompt message: %w", err)
			}
			actualContent = ic
		case "audio":
			var ac AudioContent // Defined in messages.go
			if err := json.Unmarshal(raw, &ac); err != nil {
				return fmt.Errorf("failed to unmarshal AudioContent in prompt message: %w", err)
			}
			actualContent = ac
		case "resource":
			var erc EmbeddedResourceContent // Defined in messages.go
			if err := json.Unmarshal(raw, &erc); err != nil {
				return fmt.Errorf("failed to unmarshal EmbeddedResourceContent in prompt message: %w", err)
			}
			actualContent = erc
		default:
			log.Printf("Warning: Unknown content type '%s' encountered in prompt message", typeDetect.Type)
			continue
		}
		pm.Content = append(pm.Content, actualContent)
	}
	return nil
}

// Prompt represents a prompt template available from the server.
type Prompt struct {
	URI         string                 `json:"uri"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Arguments   []PromptArgument       `json:"arguments,omitempty"`
	Messages    []PromptMessage        `json:"messages"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PromptReference allows referencing a prompt, potentially with arguments.
type PromptReference struct {
	URI       string                 `json:"uri"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ListPromptsRequestParams defines parameters for 'prompts/list'.
type ListPromptsRequestParams struct {
	Filter map[string]interface{} `json:"filter,omitempty"`
	Cursor string                 `json:"cursor,omitempty"`
}

// ListPromptsResult defines the result for 'prompts/list'.
type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// GetPromptRequestParams defines parameters for 'prompts/get'.
type GetPromptRequestParams struct {
	URI       string                 `json:"uri"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// GetPromptResult defines the result for 'prompts/get'.
type GetPromptResult struct {
	Prompt Prompt `json:"prompt"`
}
