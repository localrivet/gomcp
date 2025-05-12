// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json"
	"fmt"
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
	Role    string  `json:"role"`    // Must be "user" or "assistant" per schema
	Content Content `json:"content"` // Must be a single Content object per schema
}

// MarshalJSON implements custom marshalling for PromptMessage to ensure proper format.
func (pm PromptMessage) MarshalJSON() ([]byte, error) {
	type Alias PromptMessage

	// Format for schema compliance: content is a single object, not an array
	return json.Marshal(struct {
		Role    string  `json:"role"`
		Content Content `json:"content"`
	}{
		Role:    pm.Role,
		Content: pm.Content,
	})
}

// UnmarshalJSON implements custom unmarshalling for PromptMessage.
func (pm *PromptMessage) UnmarshalJSON(data []byte) error {
	// First check if we're dealing with the old format (array of content)
	var oldFormat struct {
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	}

	if err := json.Unmarshal(data, &oldFormat); err == nil && len(oldFormat.Content) > 0 {
		// We found old array format, convert by using first content item
		pm.Role = oldFormat.Role

		// Get the first content item and parse it
		var typeDetect struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(oldFormat.Content[0], &typeDetect); err != nil {
			// Try default to text content if type detection fails
			var tc TextContent
			if errText := json.Unmarshal(oldFormat.Content[0], &tc); errText == nil && tc.Type == "text" {
				pm.Content = tc
				return nil
			}
			return fmt.Errorf("failed to detect content type in prompt message: %w", err)
		}

		// Process based on content type
		switch typeDetect.Type {
		case "text":
			var tc TextContent
			if err := json.Unmarshal(oldFormat.Content[0], &tc); err != nil {
				return fmt.Errorf("failed to unmarshal TextContent in prompt message: %w", err)
			}
			pm.Content = tc
		case "image":
			var ic ImageContent
			if err := json.Unmarshal(oldFormat.Content[0], &ic); err != nil {
				return fmt.Errorf("failed to unmarshal ImageContent in prompt message: %w", err)
			}
			pm.Content = ic
		case "audio":
			var ac AudioContent
			if err := json.Unmarshal(oldFormat.Content[0], &ac); err != nil {
				return fmt.Errorf("failed to unmarshal AudioContent in prompt message: %w", err)
			}
			pm.Content = ac
		case "resource":
			var erc EmbeddedResourceContent
			if err := json.Unmarshal(oldFormat.Content[0], &erc); err != nil {
				return fmt.Errorf("failed to unmarshal EmbeddedResourceContent in prompt message: %w", err)
			}
			pm.Content = erc
		default:
			// We don't log directly in the protocol package to avoid import cycles
			// and because the protocol package should be independent of logging implementations.
			// Callers should handle unknown content types appropriately.
			return fmt.Errorf("unknown content type: %s", typeDetect.Type)
		}

		return nil
	}

	// New format with single content object
	var newFormat struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}

	if err := json.Unmarshal(data, &newFormat); err != nil {
		return fmt.Errorf("failed to unmarshal prompt message: %w", err)
	}

	pm.Role = newFormat.Role

	// Detect content type
	var typeDetect struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(newFormat.Content, &typeDetect); err != nil {
		return fmt.Errorf("failed to detect content type: %w", err)
	}

	// Process content based on type
	switch typeDetect.Type {
	case "text":
		var tc TextContent
		if err := json.Unmarshal(newFormat.Content, &tc); err != nil {
			return fmt.Errorf("failed to unmarshal TextContent: %w", err)
		}
		pm.Content = tc
	case "image":
		var ic ImageContent
		if err := json.Unmarshal(newFormat.Content, &ic); err != nil {
			return fmt.Errorf("failed to unmarshal ImageContent: %w", err)
		}
		pm.Content = ic
	case "audio":
		var ac AudioContent
		if err := json.Unmarshal(newFormat.Content, &ac); err != nil {
			return fmt.Errorf("failed to unmarshal AudioContent: %w", err)
		}
		pm.Content = ac
	case "resource":
		var erc EmbeddedResourceContent
		if err := json.Unmarshal(newFormat.Content, &erc); err != nil {
			return fmt.Errorf("failed to unmarshal EmbeddedResourceContent: %w", err)
		}
		pm.Content = erc
	default:
		return fmt.Errorf("unknown content type: %s", typeDetect.Type)
	}

	return nil
}

// Prompt represents a prompt template available from the server.
type Prompt struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`                  // Human-readable name
	Description string                 `json:"description,omitempty"` // Longer description
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
	Name      string                 `json:"name,omitempty"` // Added for 2025-03-26 schema
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// GetPromptResult defines the result for 'prompts/get'.
type GetPromptResult struct {
	Messages    []PromptMessage `json:"messages"`
	Description string          `json:"description,omitempty"`
}
