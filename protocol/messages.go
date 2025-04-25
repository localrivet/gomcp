// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json"
	"fmt"
	"log"
)

// --- Initialization Sequence Structures ---

// Implementation describes the name and version of an MCP implementation (client or server).
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities describes features the client supports.
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Roots        *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"roots,omitempty"`
	Sampling      *struct{} `json:"sampling,omitempty"`
	Authorization *struct { // Added for 2025-03-26
		// Define specific authorization capabilities if needed by the spec
	} `json:"authorization,omitempty"`
}

// ServerCapabilities describes features the server supports.
type ServerCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      *struct{}              `json:"logging,omitempty"`
	Prompts      *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"prompts,omitempty"`
	Resources *struct {
		Subscribe   bool `json:"subscribe,omitempty"`
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"resources,omitempty"`
	Tools *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"tools,omitempty"`
	Authorization *struct { // Added for 2025-03-26
		// Define specific authorization capabilities if needed by the spec
	} `json:"authorization,omitempty"`
	Completions *struct { // Added for 2025-03-26
		// Define specific completion capabilities if needed by the spec
	} `json:"completions,omitempty"`
}

// InitializeRequestParams defines the parameters for the 'initialize' request.
type InitializeRequestParams struct {
	ProtocolVersion  string             `json:"protocolVersion"`
	Capabilities     ClientCapabilities `json:"capabilities"`
	ClientInfo       Implementation     `json:"clientInfo"`
	Trace            *string            `json:"trace,omitempty"`
	WorkspaceFolders []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

// WorkspaceFolder represents a workspace folder as defined by LSP.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// InitializeResult defines the result payload for a successful 'initialize' response.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// InitializedNotificationParams is the payload for the 'initialized' notification (empty).
type InitializedNotificationParams struct{}

// --- Content Structures ---

// Content defines the interface for different types of content in results/prompts.
type Content interface {
	GetType() string
}

// ContentAnnotations defines optional metadata for content parts.
type ContentAnnotations struct {
	Title    *string  `json:"title,omitempty"`
	Audience []string `json:"audience,omitempty"`
	Priority *float64 `json:"priority,omitempty"`
}

// TextContent represents textual content.
type TextContent struct {
	Type        string              `json:"type"` // Should always be "text"
	Text        string              `json:"text"`
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

func (tc TextContent) GetType() string { return tc.Type }

// ImageContent represents image content.
type ImageContent struct {
	Type        string              `json:"type"` // Should always be "image"
	Data        string              `json:"data"`
	MediaType   string              `json:"mediaType"`
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

func (ic ImageContent) GetType() string { return ic.Type }

// AudioContent represents audio content.
type AudioContent struct {
	Type        string              `json:"type"` // Should always be "audio"
	Data        string              `json:"data"`
	MediaType   string              `json:"mediaType"`
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

func (ac AudioContent) GetType() string { return ac.Type }

// EmbeddedResourceContent represents an embedded resource.
type EmbeddedResourceContent struct {
	Type        string              `json:"type"` // Should always be "resource"
	Resource    Resource            `json:"resource"`
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

func (erc EmbeddedResourceContent) GetType() string { return erc.Type }

// --- Logging Structures ---

// LoggingLevel defines the possible logging levels.
type LoggingLevel string

const (
	LogLevelError LoggingLevel = "error"
	LogLevelWarn  LoggingLevel = "warn"
	LogLevelInfo  LoggingLevel = "info"
	LogLevelDebug LoggingLevel = "debug"
	LogLevelTrace LoggingLevel = "trace"
)

// SetLevelRequestParams defines parameters for 'logging/set_level'.
type SetLevelRequestParams struct {
	Level LoggingLevel `json:"level"`
}

// LoggingMessageParams defines parameters for 'notifications/message'.
type LoggingMessageParams struct {
	Level   LoggingLevel `json:"level"`
	Message string       `json:"message"`
}

// --- Sampling Structures ---

// SamplingMessage represents a message in the context provided for sampling.
type SamplingMessage struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
	Name    *string   `json:"name,omitempty"`
}

// UnmarshalJSON implements custom unmarshalling for SamplingMessage to handle the Content interface slice.
func (sm *SamplingMessage) UnmarshalJSON(data []byte) error {
	type Alias SamplingMessage
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(sm),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal base SamplingMessage: %w", err)
	}
	sm.Content = make([]Content, 0, len(aux.Content))
	for _, raw := range aux.Content {
		var typeDetect struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typeDetect); err != nil {
			return fmt.Errorf("failed to detect content type in sampling message: %w", err)
		}
		var actualContent Content
		switch typeDetect.Type {
		case "text":
			var tc TextContent
			if err := json.Unmarshal(raw, &tc); err != nil {
				return fmt.Errorf("failed to unmarshal TextContent in sampling message: %w", err)
			}
			actualContent = tc
		case "image":
			var ic ImageContent
			if err := json.Unmarshal(raw, &ic); err != nil {
				return fmt.Errorf("failed to unmarshal ImageContent in sampling message: %w", err)
			}
			actualContent = ic
		case "audio":
			var ac AudioContent
			if err := json.Unmarshal(raw, &ac); err != nil {
				return fmt.Errorf("failed to unmarshal AudioContent in sampling message: %w", err)
			}
			actualContent = ac
		case "resource":
			var erc EmbeddedResourceContent
			if err := json.Unmarshal(raw, &erc); err != nil {
				return fmt.Errorf("failed to unmarshal EmbeddedResourceContent in sampling message: %w", err)
			}
			actualContent = erc
		default:
			log.Printf("Warning: Unknown content type '%s' encountered in sampling message", typeDetect.Type)
			continue
		}
		sm.Content = append(sm.Content, actualContent)
	}
	return nil
}

// ModelPreferences specifies desired model characteristics.
type ModelPreferences struct {
	ModelURI    string   `json:"modelUri,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`
	TopK        *int     `json:"topK,omitempty"`
}

// ModelHint provides information about the model used for a response.
type ModelHint struct {
	ModelURI     string  `json:"modelUri"`
	InputTokens  *int    `json:"inputTokens,omitempty"`
	OutputTokens *int    `json:"outputTokens,omitempty"`
	FinishReason *string `json:"finishReason,omitempty"`
}

// CreateMessageRequestParams defines parameters for 'sampling/create_message'.
type CreateMessageRequestParams struct {
	Context     []SamplingMessage `json:"context"`
	Preferences *ModelPreferences `json:"preferences,omitempty"`
}

// CreateMessageResult defines the result for 'sampling/create_message'.
type CreateMessageResult struct {
	Message   SamplingMessage `json:"message"`
	ModelHint *ModelHint      `json:"modelHint,omitempty"`
}

// --- Roots Structures ---

// Root represents a root context or workspace available on the client.
type Root struct {
	URI         string                 `json:"uri"`
	Kind        string                 `json:"kind,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ListRootsRequestParams defines parameters for 'roots/list'. (Currently empty)
type ListRootsRequestParams struct{}

// ListRootsResult defines the result for 'roots/list'.
type ListRootsResult struct {
	Roots []Root `json:"roots"`
}

// --- Cancellation and Progress Structures ---

// CancelledParams defines the parameters for the '$/cancelled' notification.
type CancelledParams struct {
	ID interface{} `json:"id"`
}

// ProgressParams defines the parameters for the '$/progress' notification.
type ProgressParams struct {
	Token   string      `json:"token"`
	Value   interface{} `json:"value"`
	Message *string     `json:"message,omitempty"` // Added for 2025-03-26
}

// ProgressToken is an identifier for reporting progress.
type ProgressToken string

// RequestMeta contains metadata associated with a request, like a progress token.
type RequestMeta struct {
	ProgressToken *ProgressToken `json:"progressToken,omitempty"`
}

// --- List Changed Notification Structures ---

// ToolsListChangedParams defines parameters for 'notifications/tools/list_changed'.
type ToolsListChangedParams struct{}

// ResourcesListChangedParams defines parameters for 'notifications/resources/list_changed'.
type ResourcesListChangedParams struct{}

// PromptsListChangedParams defines parameters for 'notifications/prompts/list_changed'.
type PromptsListChangedParams struct{}

// RootsListChangedParams defines parameters for 'notifications/roots/list_changed'.
type RootsListChangedParams struct{}
