// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

// --- Resource Access Structures ---

// Resource represents a piece of context available from the server.
type Resource struct {
	URI         string                 `json:"uri"`                   // Unique identifier (e.g., "file:///path/to/file", "git://...?rev=...")
	Kind        string                 `json:"kind,omitempty"`        // e.g., "file", "git_commit", "api_spec"
	Title       string                 `json:"title,omitempty"`       // Human-readable title
	Description string                 `json:"description,omitempty"` // Longer description
	Version     string                 `json:"version,omitempty"`     // Opaque version string (changes when content changes)
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Additional arbitrary metadata
	Size        *int                   `json:"size,omitempty"`        // Optional size in bytes (added in 2025-03-26)
}

// ResourceContents defines the interface for different types of resource content.
type ResourceContents interface {
	GetContentType() string
}

// TextResourceContents holds text-based resource content.
type TextResourceContents struct {
	ContentType string `json:"contentType"` // e.g., "text/plain", "application/json"
	Content     string `json:"content"`
}

func (trc TextResourceContents) GetContentType() string { return trc.ContentType }

// BlobResourceContents holds binary resource content (base64 encoded).
type BlobResourceContents struct {
	ContentType string `json:"contentType"` // e.g., "image/png", "application/octet-stream"
	Blob        string `json:"blob"`        // Base64 encoded string
}

func (brc BlobResourceContents) GetContentType() string { return brc.ContentType }

// ListResourcesRequestParams defines parameters for 'resources/list'.
type ListResourcesRequestParams struct {
	Filter map[string]interface{} `json:"filter,omitempty"`
	Cursor string                 `json:"cursor,omitempty"`
}

// ListResourcesResult defines the result for 'resources/list'.
type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// ReadResourceRequestParams defines parameters for 'resources/read'.
type ReadResourceRequestParams struct {
	URI     string `json:"uri"`
	Version string `json:"version,omitempty"`
}

// ReadResourceResult defines the result for 'resources/read'.
type ReadResourceResult struct {
	Resource Resource         `json:"resource"`
	Contents ResourceContents `json:"contents"` // Actual content (Text or Blob)
}

// SubscribeResourceParams defines parameters for 'resources/subscribe'.
type SubscribeResourceParams struct {
	URIs []string `json:"uris"`
}

// SubscribeResourceResult defines the result for 'resources/subscribe'. (Currently empty)
type SubscribeResourceResult struct{}

// UnsubscribeResourceParams defines parameters for 'resources/unsubscribe'.
type UnsubscribeResourceParams struct {
	URIs []string `json:"uris"`
}

// UnsubscribeResourceResult defines the result for 'resources/unsubscribe'. (Currently empty)
type UnsubscribeResourceResult struct{}

// ResourceUpdatedParams defines parameters for 'notifications/resources/updated'.
type ResourceUpdatedParams struct {
	Resource Resource `json:"resource"`
}
