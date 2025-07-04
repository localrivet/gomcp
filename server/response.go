package server

import (
	"encoding/json"
)

// Response types for MCP protocol messages
// These structs ensure proper JSON marshaling and prevent character escaping issues

// ToolListResponse represents the response for tools/list requests
type ToolListResponse struct {
	Tools      []ToolInfo `json:"tools"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// ToolInfo represents information about a single tool
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema interface{}            `json:"inputSchema"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ToolCallResponse represents the response for tools/call requests
type ToolCallResponse struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError"`
}

// ContentItem represents a single content item in tool/prompt responses
type ContentItem struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL string      `json:"imageUrl,omitempty"`
	AltText  string      `json:"altText,omitempty"`
	URL      string      `json:"url,omitempty"`
	Title    string      `json:"title,omitempty"`
	MimeType string      `json:"mimeType,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	Filename string      `json:"filename,omitempty"`
}

// PromptListResponse represents the response for prompts/list requests
type PromptListResponse struct {
	Prompts    []PromptInfo `json:"prompts"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

// PromptInfo represents information about a single prompt
type PromptInfo struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptGetResponse represents the response for prompts/get requests
type PromptGetResponse struct {
	Description string          `json:"description"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage represents a single message in a prompt response
type PromptMessage struct {
	Role    string        `json:"role"`
	Content PromptContent `json:"content"`
}

// ContentType represents the type of content in a prompt
type ContentType string

// ContentTypeText is used for plain text content
const ContentTypeText ContentType = "text"

// PromptContent represents the content of a prompt message
type PromptContent struct {
	Type ContentType `json:"type"`
	Text string      `json:"text,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ResourceListResponse represents the response for resources/list requests
type ResourceListResponse struct {
	Resources  []ResourceInfo `json:"resources"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

// ResourceInfo represents information about a single resource
type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ResourceTemplatesListResponse represents the response for resources/templates/list requests
type ResourceTemplatesListResponse struct {
	ResourceTemplates []ResourceTemplateInfo `json:"resourceTemplates"`
	NextCursor        string                 `json:"nextCursor,omitempty"`
}

// ResourceTemplateInfo represents information about a single resource template
type ResourceTemplateInfo struct {
	URITemplate string                 `json:"uriTemplate"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ResourceReadResponse represents the response for resources/read requests
type ResourceReadResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents a single resource content item
type ResourceContent struct {
	URI     string        `json:"uri"`
	Text    string        `json:"text,omitempty"`
	Content []ContentItem `json:"content,omitempty"`
}

// InitializeResponse represents the response for initialize requests
type InitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	Instructions    string                 `json:"instructions,omitempty"`
}

// ServerInfo represents server information in initialize responses
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RootsListResponse represents the response for roots/list requests
type RootsListResponse struct {
	Roots []RootInfo `json:"roots"`
}

// RootInfo represents information about a single root
type RootInfo struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// EmptyResponse represents an empty success response
type EmptyResponse struct{}

// Helper functions to create responses

// NewToolListResponse creates a new ToolListResponse
func NewToolListResponse(tools []ToolInfo, nextCursor string) *ToolListResponse {
	return &ToolListResponse{
		Tools:      tools,
		NextCursor: nextCursor,
	}
}

// NewToolCallResponse creates a new ToolCallResponse
func NewToolCallResponse(content []ContentItem, isError bool) *ToolCallResponse {
	return &ToolCallResponse{
		Content: content,
		IsError: isError,
	}
}

// NewPromptListResponse creates a new PromptListResponse
func NewPromptListResponse(prompts []PromptInfo, nextCursor string) *PromptListResponse {
	return &PromptListResponse{
		Prompts:    prompts,
		NextCursor: nextCursor,
	}
}

// NewPromptGetResponse creates a new PromptGetResponse
func NewPromptGetResponse(description string, messages []PromptMessage) *PromptGetResponse {
	return &PromptGetResponse{
		Description: description,
		Messages:    messages,
	}
}

// NewResourceListResponse creates a new ResourceListResponse
func NewResourceListResponse(resources []ResourceInfo, nextCursor string) *ResourceListResponse {
	return &ResourceListResponse{
		Resources:  resources,
		NextCursor: nextCursor,
	}
}

// NewResourceTemplatesListResponse creates a new ResourceTemplatesListResponse
func NewResourceTemplatesListResponse(templates []ResourceTemplateInfo, nextCursor string) *ResourceTemplatesListResponse {
	return &ResourceTemplatesListResponse{
		ResourceTemplates: templates,
		NextCursor:        nextCursor,
	}
}

// NewResourceReadResponse creates a new ResourceReadResponse
func NewResourceReadResponse(contents []ResourceContent) *ResourceReadResponse {
	return &ResourceReadResponse{
		Contents: contents,
	}
}

// NewInitializeResponse creates a new InitializeResponse
func NewInitializeResponse(protocolVersion string, serverInfo ServerInfo, capabilities map[string]interface{}, instructions ...string) *InitializeResponse {
	response := &InitializeResponse{
		ProtocolVersion: protocolVersion,
		ServerInfo:      serverInfo,
		Capabilities:    capabilities,
	}

	// Add instructions if provided
	if len(instructions) > 0 && instructions[0] != "" {
		response.Instructions = instructions[0]
	}

	return response
}

// NewRootsListResponse creates a new RootsListResponse
func NewRootsListResponse(roots []RootInfo) *RootsListResponse {
	return &RootsListResponse{
		Roots: roots,
	}
}

// Helper functions to create content items

// NewTextContent creates a new text content item
func NewTextContent(text string) ContentItem {
	return ContentItem{
		Type: "text",
		Text: text,
	}
}

// NewImageContent creates a new image content item
func NewImageContent(imageURL, altText string) ContentItem {
	return ContentItem{
		Type:     "image",
		ImageURL: imageURL,
		AltText:  altText,
	}
}

// NewLinkContent creates a new link content item
func NewLinkContent(url, title string) ContentItem {
	return ContentItem{
		Type:  "link",
		URL:   url,
		Title: title,
	}
}

// NewFileContent creates a new file content item
func NewFileContent(mimeType string, data interface{}, filename string) ContentItem {
	return ContentItem{
		Type:     "file",
		MimeType: mimeType,
		Data:     data,
		Filename: filename,
	}
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
		Data:     blob,
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

	// Add URL field if available (used in draft version as audioUrl)
	if a.URL != "" {
		contentItem["audioUrl"] = a.URL
	}

	// Add Data field if available (used in v20250326 version)
	if a.Data != "" {
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

// ShutdownResponse represents the response for shutdown requests
type ShutdownResponse struct {
	Success bool `json:"success"`
}

// NewShutdownResponse creates a new shutdown response
func NewShutdownResponse(success bool) *ShutdownResponse {
	return &ShutdownResponse{
		Success: success,
	}
}
