package jsonrpc

// This file contains struct definitions for JSON-RPC messages
// to provide type safety and documentation in test code

// JSONRPC defines the basic structure of a JSON-RPC message
type JSONRPC struct {
	Version string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError defines the structure of a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolParams defines parameters for tool execution
type ToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ResourceParams defines parameters for resource retrieval
type ResourceParams struct {
	Path    string                 `json:"path"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// PromptParams defines parameters for prompt retrieval
type PromptParams struct {
	Name      string                 `json:"name"`
	Variables map[string]interface{} `json:"variables"`
}

// RootAddParams defines parameters for adding a root
type RootAddParams struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// RootRemoveParams defines parameters for removing a root
type RootRemoveParams struct {
	Path string `json:"path"`
}

// ToolResult defines the result of a tool execution
type ToolResult struct {
	Output   interface{}            `json:"output"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PromptResult defines the result of a prompt retrieval
type PromptResult struct {
	Prompt   string                 `json:"prompt"`
	Rendered string                 `json:"rendered"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RootsListResult defines the result of a roots/list request
type RootsListResult struct {
	Roots []RootItem `json:"roots"`
}

// RootItem defines a root resource
type RootItem struct {
	URI      string                 `json:"uri"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContentItem defines a content item in a resource
type ContentItem struct {
	Type     string      `json:"type"`
	Text     interface{} `json:"text,omitempty"`
	URL      string      `json:"url,omitempty"`
	AltText  string      `json:"altText,omitempty"`
	Code     string      `json:"code,omitempty"`
	Language string      `json:"language,omitempty"`
	Headers  []string    `json:"headers,omitempty"`
	Rows     [][]string  `json:"rows,omitempty"`
}

// ResourceItem defines a single resource in a resource response
type ResourceItem struct {
	URI      string                 `json:"uri"`
	Text     string                 `json:"text"`
	Content  []ContentItem          `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceResult20241105 defines the result of a resource retrieval for v20241105
type ResourceResult20241105 struct {
	Content []ContentItem `json:"content"`
}

// ResourceResult20250326 defines the result of a resource retrieval for v20250326
type ResourceResult20250326 struct {
	Contents []ResourceItem `json:"contents"`
}

// InitializeParams defines the parameters for an initialize request
type InitializeParams struct {
	ClientInfo ClientInfoObj `json:"clientInfo"`
	Versions   []string      `json:"versions"`
}

// ClientInfoObj defines the client information
type ClientInfoObj struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfoObj defines the server information
type ServerInfoObj struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult defines the result of an initialize request
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      ServerInfoObj          `json:"serverInfo"`
	Capabilities    map[string]interface{} `json:"capabilities"`
}

// SamplingMessageContent represents the content of a sampling message
type SamplingMessageContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// SamplingMessage represents a message in a sampling conversation
type SamplingMessage struct {
	Role    string                 `json:"role"`
	Content SamplingMessageContent `json:"content"`
}

// SamplingModelHint represents a hint for model selection
type SamplingModelHint struct {
	Name string `json:"name"`
}

// SamplingModelPreferences represents model preferences for a sampling request
type SamplingModelPreferences struct {
	Hints                []SamplingModelHint `json:"hints,omitempty"`
	CostPriority         *float64            `json:"costPriority,omitempty"`
	SpeedPriority        *float64            `json:"speedPriority,omitempty"`
	IntelligencePriority *float64            `json:"intelligencePriority,omitempty"`
}

// SamplingCreateMessageParams represents the parameters for a sampling/createMessage request
type SamplingCreateMessageParams struct {
	Messages         []SamplingMessage        `json:"messages"`
	ModelPreferences SamplingModelPreferences `json:"modelPreferences"`
	SystemPrompt     string                   `json:"systemPrompt,omitempty"`
	MaxTokens        int                      `json:"maxTokens,omitempty"`
}

// SamplingResponse represents the response to a sampling/createMessage request
type SamplingResponse struct {
	Role       string                 `json:"role"`
	Content    SamplingMessageContent `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`
}

// SamplingStreamingResponse represents a streaming response to a sampling request
type SamplingStreamingResponse struct {
	Chunk      *SamplingStreamingChunk      `json:"chunk,omitempty"`
	Completion *SamplingStreamingCompletion `json:"completion,omitempty"`
}

// SamplingStreamingChunk represents a chunk of a streaming response
type SamplingStreamingChunk struct {
	ChunkID string                 `json:"chunkId"`
	Role    string                 `json:"role,omitempty"`
	Content SamplingMessageContent `json:"content"`
}

// SamplingStreamingCompletion represents the completion of a streaming response
type SamplingStreamingCompletion struct {
	Model      string `json:"model,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
}
