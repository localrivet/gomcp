package server

import (
	"encoding/json"
)

// ContentItem represents a single content item in a response.
// In the MCP protocol, responses are structured as collections of typed content items,
// allowing for rich, multimodal responses that can include text, images, links, files,
// JSON data, and binary blobs with appropriate metadata.
type ContentItem struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL string      `json:"imageUrl,omitempty"`
	AltText  string      `json:"altText,omitempty"`
	URL      string      `json:"url,omitempty"`
	Title    string      `json:"title,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	MimeType string      `json:"mimeType,omitempty"`
	Filename string      `json:"filename,omitempty"`
	Blob     string      `json:"blob,omitempty"` // Add blob support for MCP Inspector validation
}

// TextContent creates a new text content item.
// This function creates a properly formatted text content item for inclusion
// in MCP responses, handling edge cases like empty text to ensure protocol compliance.
//
// Parameters:
//   - text: The text content to include in the response
//
// Returns:
//   - A ContentItem of type "text" properly formatted for the MCP protocol
func TextContent(text string) ContentItem {
	// If text is empty, set it to a space to satisfy the MCP Inspector validation
	if text == "" {
		text = " "
	}

	return ContentItem{
		Type: "text",
		Text: text, // This will satisfy the MCP Inspector validation
	}
}

// ImageContent creates a new image content item.
// This function creates a properly formatted image content item for inclusion in MCP responses.
//
// Parameters:
//   - imageURL: The URL where the image can be accessed
//   - altText: Accessibility description of the image content
//   - optMimeType: Optional MIME type of the image (e.g., "image/png")
//
// Returns:
//   - A ContentItem of type "image" properly formatted for the MCP protocol
func ImageContent(imageURL string, altText string, optMimeType ...string) ContentItem {
	content := ContentItem{
		Type:     "image",
		ImageURL: imageURL,
	}

	if altText != "" {
		content.AltText = altText
	}

	// Add mime type if provided
	if len(optMimeType) > 0 && optMimeType[0] != "" {
		content.MimeType = optMimeType[0]
	}

	return content
}

// LinkContent creates a new link content item.
// This function creates a properly formatted link content item for inclusion in MCP responses.
//
// Parameters:
//   - url: The target URL of the link
//   - title: The display text or title for the link
//
// Returns:
//   - A ContentItem of type "link" properly formatted for the MCP protocol
func LinkContent(url, title string) ContentItem {
	return ContentItem{
		Type:  "link",
		URL:   url,
		Title: title,
	}
}

// FileContent creates a content item of type "file"
func FileContent(fileURL string, filename string, mimeType string) ContentItem {
	content := ContentItem{
		Type:     "file",
		URL:      fileURL,
		Filename: filename,
	}

	if mimeType != "" {
		content.MimeType = mimeType
	}

	return content
}

// JSONContent creates a content item of type "json"
func JSONContent(data interface{}) ContentItem {
	return ContentItem{
		Type: "json",
		Data: data,
	}
}

// BlobContent creates a new blob content item.
func BlobContent(blob string, mimeType string) ContentItem {
	return ContentItem{
		Type:     "blob",
		Blob:     blob,
		MimeType: mimeType,
	}
}

// ResourceResponse is a standard response for MCP resources.
// It ensures the response format follows the MCP protocol.
type ResourceResponse struct {
	Content  []ContentItem          `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	IsError  bool                   `json:"isError,omitempty"`
}

// NewResourceResponse creates a new resource response with the given content items.
func NewResourceResponse(items ...ContentItem) ResourceResponse {
	return ResourceResponse{
		Content: items,
	}
}

// WithMetadata adds metadata to the resource response.
func (r ResourceResponse) WithMetadata(metadata map[string]interface{}) ResourceResponse {
	r.Metadata = metadata
	return r
}

// AsError marks the response as an error.
func (r ResourceResponse) AsError() ResourceResponse {
	r.IsError = true
	return r
}

// SimpleTextResponse creates a simple text response map
func SimpleTextResponse(text string) map[string]interface{} {
	return TextResource{Text: text}.ToResourceResponse()
}

// ResourceConverter allows custom types to be converted to resource responses
type ResourceConverter interface {
	ToResourceResponse() map[string]interface{}
}

// SimpleJSONResponse creates a JSON resource response
func SimpleJSONResponse(data interface{}) map[string]interface{} {
	if data == nil {
		return SimpleTextResponse("null")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		resp := SimpleTextResponse("Error: Failed to convert to JSON")
		resp["isError"] = true
		return resp
	}

	return JSONResource{Data: json.RawMessage(jsonData)}.ToResourceResponse()
}

// ==========================================
// Specialized Resource Response Types
// ==========================================

// TextResource represents a simple text resource
type TextResource struct {
	Text string
}

// ToResourceResponse converts TextResource to ResourceResponse
func (tr TextResource) ToResourceResponse() map[string]interface{} {
	content := TextContent(tr.Text)
	return map[string]interface{}{
		"content": []ContentItem{content},
	}
}

// ImageResource represents an image resource
type ImageResource struct {
	URL      string
	AltText  string
	MimeType string
}

// ToResourceResponse converts ImageResource to ResourceResponse
func (ir ImageResource) ToResourceResponse() map[string]interface{} {
	// Ensure we have an alt text
	if ir.AltText == "" {
		ir.AltText = "Image"
	}

	// Create a properly structured image content item
	imageContent := map[string]interface{}{
		"type":     "image",
		"imageUrl": ir.URL,
		"altText":  ir.AltText,
	}

	// Add the mime type to the metadata
	mimeType := "image/jpeg" // Default to a common image type
	if ir.MimeType != "" {
		mimeType = ir.MimeType
	}

	return map[string]interface{}{
		"content": []interface{}{imageContent},
		"metadata": map[string]interface{}{
			"mimeType": mimeType,
		},
	}
}

// LinkResource represents a link resource
type LinkResource struct {
	URL   string
	Title string
}

// ToResourceResponse converts LinkResource to ResourceResponse
func (lr LinkResource) ToResourceResponse() map[string]interface{} {
	// Ensure we have a title
	if lr.Title == "" {
		lr.Title = "Link"
	}

	// Create a properly structured link content item
	linkContent := map[string]interface{}{
		"type":  "link",
		"url":   lr.URL,
		"title": lr.Title,
	}

	return map[string]interface{}{
		"content": []interface{}{linkContent},
	}
}

// FileResource represents a file resource
type FileResource struct {
	URL      string
	Filename string
	MimeType string
}

// ToResourceResponse converts FileResource to ResourceResponse
func (fr FileResource) ToResourceResponse() map[string]interface{} {
	fileContent := FileContent(fr.URL, fr.Filename, fr.MimeType)
	return map[string]interface{}{
		"content": []ContentItem{fileContent},
		"metadata": map[string]interface{}{
			"mimeType": fr.MimeType,
		},
	}
}

// JSONResource represents a JSON resource
type JSONResource struct {
	Data interface{}
}

// ToResourceResponse converts JSONResource to ResourceResponse
func (jr JSONResource) ToResourceResponse() map[string]interface{} {
	jsonContent := JSONContent(jr.Data)
	return map[string]interface{}{
		"content": []ContentItem{jsonContent},
	}
}

// AudioResource represents an audio resource to be returned from a handler
type AudioResource struct {
	// URL is the URL of the audio file. Required for all versions.
	URL string
	// Data is the base64-encoded audio data. Used in 2025-03-26 version.
	Data string
	// MimeType is the MIME type of the audio file. Required for all versions.
	MimeType string
	// AltText is an optional descriptive text for the audio
	AltText string
}

// ToResourceResponse converts the AudioResource to a protocol-specific representation
func (a AudioResource) ToResourceResponse() map[string]interface{} {
	// Create a map that will be version-specific formatted
	contentItem := map[string]interface{}{
		"type":     "audio",
		"mimeType": a.MimeType, // Required in all versions
	}

	// Add URL field for draft version which uses audioUrl
	if a.URL != "" {
		contentItem["audioUrl"] = a.URL
	}

	// Add Data field for v20250326 version, but only if URL is not provided
	// This ensures we don't add both URL and Data which could cause confusion
	if a.Data != "" && a.URL == "" {
		contentItem["data"] = a.Data
	}

	// Add alt text if provided
	if a.AltText != "" {
		contentItem["altText"] = a.AltText
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{contentItem},
	}
}
