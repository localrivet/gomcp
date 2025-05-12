// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json" // Added for UnmarshalJSON
	"fmt"           // Added for UnmarshalJSON
	"log"           // Added for UnmarshalJSON
)

// --- Resource Access Structures ---

// Resource represents a piece of context available from the server.
type Resource struct {
	URI         string                 `json:"uri"`                   // Unique identifier (e.g., "file:///path/to/file", "git://...?rev=...")
	Kind        string                 `json:"kind,omitempty"`        // e.g., "file", "git_commit", "api_spec"
	Name        string                 `json:"name"`                  // Human-readable name (required by MCP spec)
	Description string                 `json:"description,omitempty"` // Longer description
	Version     string                 `json:"version,omitempty"`     // Opaque version string (changes when content changes)
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Additional arbitrary metadata
	Size        *int                   `json:"size,omitempty"`        // Optional size in bytes (added in 2025-03-26)
	Annotations Annotations            `json:"annotations,omitempty"` // Added in 2025-03-26
}

// ResourceTemplate describes a template for creating resources (Placeholder structure)
type ResourceTemplate struct {
	Kind        string                 `json:"kind"`                  // Kind of resource the template creates
	Name        string                 `json:"name"`                  // Human-readable name (required by MCP spec)
	Description string                 `json:"description,omitempty"` // Longer description
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Default metadata or parameters needed
	URITemplate string                 `json:"uriTemplate"`           // URI template for constructing resource URLs
}

// ResourceContents defines the interface for different types of resource content.
type ResourceContents interface {
	GetContentType() string
}

// TextResourceContents holds text-based resource content.
type TextResourceContents struct {
	ContentType string `json:"mimeType"` // e.g., "text/plain", "application/json"
	Text        string `json:"text"`     // Changed from Content to Text with json tag "text"
	URI         string `json:"uri"`      // Required by schema
}

func (trc TextResourceContents) GetContentType() string { return trc.ContentType }

// BlobResourceContents holds binary resource content (base64 encoded).
type BlobResourceContents struct {
	ContentType string `json:"mimeType"` // e.g., "image/png", "application/octet-stream"
	Blob        string `json:"blob"`     // Base64 encoded string
	URI         string `json:"uri"`      // Required by schema
}

func (brc BlobResourceContents) GetContentType() string { return brc.ContentType }

type AudioResourceContents struct {
	ContentType string `json:"mimeType"` // e.g., "audio/mpeg", "audio/wav"
	Audio       string `json:"audio"`    // Base64 encoded string
	URI         string `json:"uri"`      // Required by schema
}

func (arc AudioResourceContents) GetContentType() string { return arc.ContentType }

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

// ListResourceTemplatesRequestParams defines parameters for 'resources/list_templates'. (Empty)
type ListResourceTemplatesRequestParams struct{}

// ListResourceTemplatesResult defines the result for 'resources/templates/list'.
type ListResourceTemplatesResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}

// ReadResourceRequestParams defines parameters for 'resources/read'.
type ReadResourceRequestParams struct {
	URI     string `json:"uri"`
	Version string `json:"version,omitempty"`
}

// ReadResourceResult defines the result for 'resources/read'.
type ReadResourceResult struct {
	Resource Resource           `json:"resource"`
	Contents []ResourceContents `json:"contents"` // Actual content (Text, Blob, or Audio) - Array for 2024-11-05 and 2025-03-26
}

// UnmarshalJSON implements custom unmarshalling for ReadResourceResult to handle the Contents interface slice.
func (r *ReadResourceResult) UnmarshalJSON(data []byte) error {
	// 1. Define an auxiliary type to prevent recursion
	type Alias ReadResourceResult
	aux := &struct {
		Contents []json.RawMessage `json:"contents"` // Unmarshal Contents into RawMessage first
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	// 2. Unmarshal into the auxiliary type
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal base ReadResourceResult: %w", err)
	}

	// 3. Iterate over RawMessages and unmarshal into concrete types
	r.Contents = make([]ResourceContents, 0, len(aux.Contents)) // Initialize the slice
	for _, raw := range aux.Contents {
		var actualContent ResourceContents
		var unmarshalErr error

		// Try unmarshalling as TextResourceContents
		var tc TextResourceContents
		unmarshalErr = json.Unmarshal(raw, &tc)
		if unmarshalErr == nil && tc.Text != "" && tc.URI != "" { // Check if required fields are present
			actualContent = tc
		} else {
			// Try unmarshalling as BlobResourceContents
			var bc BlobResourceContents
			unmarshalErr = json.Unmarshal(raw, &bc)
			if unmarshalErr == nil && bc.Blob != "" && bc.URI != "" { // Check if required fields are present
				actualContent = bc
			} else {
				// Try unmarshalling as AudioResourceContents
				var ac AudioResourceContents
				unmarshalErr = json.Unmarshal(raw, &ac)
				if unmarshalErr == nil && ac.Audio != "" && ac.URI != "" { // Check if required fields are present
					actualContent = ac
				} else {
					// None matched or all had errors
					log.Printf("Warning: Could not determine resource content type for: %s", string(raw))
					continue // Skip this content part
				}
			}
		}

		r.Contents = append(r.Contents, actualContent)
	}

	return nil
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
