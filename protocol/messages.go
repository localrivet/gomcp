// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json"
	"fmt"
	"log"
)

// Annotations provides optional annotations for the client (2025-03-26).
type Annotations struct {
	Audience []string `json:"audience,omitempty"` // Describes who the intended customer is
	Priority *float64 `json:"priority,omitempty"` // Importance of the data (0-1)
}

// --- Initialization Sequence Structures ---

// Implementation describes the name and version of an MCP implementation (client or server).
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities describes features the client supports.
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      *struct{}              `json:"logging,omitempty"` // Added based on ServerCapabilities
	Prompts      *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"prompts,omitempty"` // Added based on ServerCapabilities
	Resources *struct {
		Subscribe   bool `json:"subscribe,omitempty"`
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"resources,omitempty"` // Added based on ServerCapabilities
	Tools *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"tools,omitempty"` // Added based on ServerCapabilities
	Roots *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"roots,omitempty"`
	Sampling      *SamplingCapability `json:"sampling,omitempty"` // Corrected type
	Authorization *struct {           // Added for 2025-03-26
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
	// Syslog levels based on RFC 5424, used in 2025-03-26 spec
	LogLevelEmergency LoggingLevel = "emergency"
	LogLevelAlert     LoggingLevel = "alert"
	LogLevelCritical  LoggingLevel = "critical"
	LogLevelError     LoggingLevel = "error"
	LogLevelWarn      LoggingLevel = "warning" // Renamed from 'warn'
	LogLevelNotice    LoggingLevel = "notice"
	LogLevelInfo      LoggingLevel = "info"
	LogLevelDebug     LoggingLevel = "debug"
	// LogLevelTrace is not a standard syslog level
)

// SetLevelRequestParams defines parameters for 'logging/set_level'.
type SetLevelRequestParams struct {
	Level LoggingLevel `json:"level"`
}

// LoggingMessageParams defines parameters for 'notifications/message'.
// This struct now includes fields for both 2024-11-05 (Level, Message)
// and 2025-03-26 (Level, Logger, Data).
// Consuming code must check negotiatedVersion to determine which fields are expected/valid.
type LoggingMessageParams struct {
	Level   LoggingLevel `json:"level"`
	Message string       `json:"message,omitempty"` // Kept for 2024-11-05 compatibility
	Logger  *string      `json:"logger,omitempty"`  // Added for 2025-03-26
	Data    interface{}  `json:"data,omitempty"`    // Added for 2025-03-26 (JSON object)
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

// ModelPreferences represents the 2024-11-05 structure.
type ModelPreferences struct {
	ModelURI    string   `json:"modelUri,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`
	TopK        *int     `json:"topK,omitempty"`
}

// --- START: Added for 2025-03-26 Sampling ---

// ModelHintV20250326 represents a model hint for 2025-03-26.
type ModelHintV20250326 struct {
	Name string `json:"name"`
}

// ModelPreferencesV20250326 represents the 2025-03-26 structure.
type ModelPreferencesV20250326 struct {
	Hints                []ModelHintV20250326 `json:"hints,omitempty"`
	CostPriority         *float64             `json:"costPriority,omitempty"`         // 0-1
	SpeedPriority        *float64             `json:"speedPriority,omitempty"`        // 0-1
	IntelligencePriority *float64             `json:"intelligencePriority,omitempty"` // 0-1
}

// ModelHint represents the 2024-11-05 structure.
type ModelHint struct {
	ModelURI     string  `json:"modelUri"`
	InputTokens  *int    `json:"inputTokens,omitempty"`
	OutputTokens *int    `json:"outputTokens,omitempty"`
	FinishReason *string `json:"finishReason,omitempty"`
}

// --- Note: 2025-03-26 sampling result includes model/stopReason directly ---

// CreateMessageRequestParams represents the 2024-11-05 structure.
type CreateMessageRequestParams struct {
	Context     []SamplingMessage `json:"context"` // Renamed to 'messages' in 2025-03-26
	Preferences *ModelPreferences `json:"preferences,omitempty"`
}

// CreateMessageRequestParamsV20250326 represents the 2025-03-26 structure.
type CreateMessageRequestParamsV20250326 struct {
	Messages         []SamplingMessage          `json:"messages"`                   // Renamed from 'context'
	ModelPreferences *ModelPreferencesV20250326 `json:"modelPreferences,omitempty"` // Use new struct
	SystemPrompt     *string                    `json:"systemPrompt,omitempty"`     // Added field
	MaxTokens        *int                       `json:"maxTokens,omitempty"`        // Added field
}

// CreateMessageResult represents the 2024-11-05 structure.
type CreateMessageResult struct {
	Message   SamplingMessage `json:"message"`
	ModelHint *ModelHint      `json:"modelHint,omitempty"`
}

// CreateMessageResultV20250326 represents the 2025-03-26 structure.
type CreateMessageResultV20250326 struct {
	Role       string    `json:"role"`                 // Added field (e.g., "assistant")
	Content    []Content `json:"content"`              // Added field (replaces Message.Content)
	Model      *string   `json:"model,omitempty"`      // Added field (replaces ModelHint.ModelURI)
	StopReason *string   `json:"stopReason,omitempty"` // Added field (replaces ModelHint.FinishReason)
	// InputTokens and OutputTokens are not part of the 2025-03-26 result structure.
}

// --- END: Added for 2025-03-26 Sampling ---

// SamplingCapability defines capabilities related to sampling.
type SamplingCapability struct {
	Enabled bool `json:"enabled,omitempty"` // Indicates if the client supports handling sampling/request
	// Add other sampling-related capabilities here if needed in the future
}

// SamplingRequestParams defines the parameters for a 'sampling/request'.
// Note: This structure aligns with the 2025-03-26 spec's 'sampling/create_message' request.
// If supporting 2024-11-05 sampling is needed, separate handling might be required.
type SamplingRequestParams struct {
	Messages         []SamplingMessage          `json:"messages"`
	ModelPreferences *ModelPreferencesV20250326 `json:"modelPreferences,omitempty"`
	SystemPrompt     *string                    `json:"systemPrompt,omitempty"`
	MaxTokens        *int                       `json:"maxTokens,omitempty"`
	Meta             *RequestMeta               `json:"_meta,omitempty"` // Allow progress reporting
}

// SamplingResult defines the result payload for a successful 'sampling/request' response.
// Note: This structure aligns with the 2025-03-26 spec's 'sampling/create_message' result.
type SamplingResult struct {
	Role       string       `json:"role"`
	Content    []Content    `json:"content"`
	Model      *string      `json:"model,omitempty"`
	StopReason *string      `json:"stopReason,omitempty"`
	Meta       *RequestMeta `json:"_meta,omitempty"` // Allow progress reporting
}

// UnmarshalJSON implements custom unmarshalling for SamplingResult to handle the Content interface slice.
func (sr *SamplingResult) UnmarshalJSON(data []byte) error {
	// Define an alias with Content as RawMessage to parse base fields first
	type Alias SamplingResult
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(sr),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal base SamplingResult: %w", err)
	}

	sr.Content = make([]Content, 0, len(aux.Content))
	for _, raw := range aux.Content {
		// Detect the type first
		var typeDetect struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typeDetect); err != nil {
			// Handle case where content might just be a string (implicitly text)
			var text string
			if errStr := json.Unmarshal(raw, &text); errStr == nil {
				sr.Content = append(sr.Content, TextContent{Type: "text", Text: text})
				continue
			}
			return fmt.Errorf("failed to detect content type in sampling result: %w, raw: %s", err, string(raw))
		}

		var actualContent Content
		switch typeDetect.Type {
		case "text":
			var tc TextContent
			if err := json.Unmarshal(raw, &tc); err != nil {
				return fmt.Errorf("failed to unmarshal TextContent in sampling result: %w", err)
			}
			actualContent = tc
		case "image":
			var ic ImageContent
			if err := json.Unmarshal(raw, &ic); err != nil {
				return fmt.Errorf("failed to unmarshal ImageContent in sampling result: %w", err)
			}
			actualContent = ic
		case "audio":
			var ac AudioContent
			if err := json.Unmarshal(raw, &ac); err != nil {
				return fmt.Errorf("failed to unmarshal AudioContent in sampling result: %w", err)
			}
			actualContent = ac
		case "resource":
			var erc EmbeddedResourceContent
			if err := json.Unmarshal(raw, &erc); err != nil {
				return fmt.Errorf("failed to unmarshal EmbeddedResourceContent in sampling result: %w", err)
			}
			actualContent = erc
		default:
			log.Printf("Warning: Unknown content type '%s' encountered in sampling result", typeDetect.Type)
			// Optionally store as raw data or skip
			continue
		}
		sr.Content = append(sr.Content, actualContent)
	}

	return nil
}

// --- Roots Structures ---

// Root represents a root context or workspace available on the client.
type Root struct {
	URI         string                 `json:"uri"`
	Kind        string                 `json:"kind,omitempty"`
	Name        string                 `json:"name"` // Human-readable name
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
	ID     interface{} `json:"id"`
	Reason *string     `json:"reason,omitempty"` // Added for 2025-03-26
}

// ProgressParams defines the parameters for the '$/progress' notification.
type ProgressParams struct {
	Token   string      `json:"token"`
	Value   interface{} `json:"value"`
	Message *string     `json:"message,omitempty"` // Ensure omitempty is present (relevant for 2025-03-26)
}

// ProgressToken is an identifier for reporting progress.
// It can be a string or a number according to the spec.
// type ProgressToken string // Removed, using interface{} now

// RequestMeta contains metadata associated with a request, like a progress token.
type RequestMeta struct {
	// Use interface{} to accept string or number, per spec.
	ProgressToken interface{} `json:"progressToken,omitempty"`
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

// BoolPtr is a helper function to return a pointer to a boolean value.
func BoolPtr(b bool) *bool {
	return &b
}

// StringPtr is a helper function to return a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}

// IntPtr is a helper function to return a pointer to an int value.
func IntPtr(i int) *int {
	return &i
}
