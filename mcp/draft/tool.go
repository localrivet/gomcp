package draft

import (
	"encoding/json"
	"fmt"
)

// ToolDefinition represents an MCP tool definition
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Metadata    ToolMetadata    `json:"metadata,omitempty"`
	Streamable  bool            `json:"streamable,omitempty"`
	Cacheable   bool            `json:"cacheable,omitempty"`  // New in draft
	Versioned   bool            `json:"versioned,omitempty"`  // New in draft
	Deprecated  bool            `json:"deprecated,omitempty"` // New in draft
	Category    string          `json:"category,omitempty"`   // New in draft
	Tags        []string        `json:"tags,omitempty"`       // New in draft
}

// ToolInputSchema represents a JSON Schema for tool input
type ToolInputSchema struct {
	Type                 string                    `json:"type"`
	Properties           map[string]PropertyDetail `json:"properties"`
	Required             []string                  `json:"required,omitempty"`
	AdditionalProperties *bool                     `json:"additionalProperties,omitempty"` // New in draft
	OneOf                []json.RawMessage         `json:"oneOf,omitempty"`                // New in draft
	AnyOf                []json.RawMessage         `json:"anyOf,omitempty"`                // New in draft
	AllOf                []json.RawMessage         `json:"allOf,omitempty"`                // New in draft
}

// PropertyDetail represents a JSON Schema property definition
type PropertyDetail struct {
	Type        string        `json:"type"`
	Description string        `json:"description,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`
	Format      string        `json:"format,omitempty"`
	Minimum     *float64      `json:"minimum,omitempty"`
	Maximum     *float64      `json:"maximum,omitempty"`
	MinLength   *int          `json:"minLength,omitempty"`
	MaxLength   *int          `json:"maxLength,omitempty"`
	Pattern     string        `json:"pattern,omitempty"`
	Default     interface{}   `json:"default,omitempty"`
	Examples    []interface{} `json:"examples,omitempty"`
	Deprecated  bool          `json:"deprecated,omitempty"`       // New in draft
	Sensitive   bool          `json:"sensitive,omitempty"`        // New in draft
	ReadOnly    bool          `json:"readOnly,omitempty"`         // New in draft
	WriteOnly   bool          `json:"writeOnly,omitempty"`        // New in draft
	ContentType string        `json:"contentMediaType,omitempty"` // New in draft - for binary/media data
}

// ToolMetadata represents metadata for a tool
type ToolMetadata struct {
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	Cost        *ResourceCost          `json:"cost,omitempty"`
	Performance *PerformanceMetrics    `json:"performance,omitempty"`
	Security    *SecurityInfo          `json:"security,omitempty"`   // New in draft
	Support     *SupportInfo           `json:"support,omitempty"`    // New in draft
	License     string                 `json:"license,omitempty"`    // New in draft
	Homepage    string                 `json:"homepage,omitempty"`   // New in draft
	Repository  string                 `json:"repository,omitempty"` // New in draft
	Deprecated  *DeprecationInfo       `json:"deprecated,omitempty"` // New in draft
	Examples    []ToolExample          `json:"examples,omitempty"`   // New in draft
}

// ResourceCost represents the cost of using a resource or tool
type ResourceCost struct {
	Type     string  `json:"type"`               // e.g., "credits", "tokens", "api_calls"
	Amount   float64 `json:"amount"`             // Estimated cost amount
	Currency string  `json:"currency,omitempty"` // Optional currency code
	Free     bool    `json:"free,omitempty"`     // New in draft - indicates free usage
	Tier     string  `json:"tier,omitempty"`     // New in draft - pricing tier
}

// PerformanceMetrics represents performance characteristics of a tool
type PerformanceMetrics struct {
	AvgLatency      int   `json:"avg_latency_ms,omitempty"`          // Average latency in milliseconds
	P95Latency      int   `json:"p95_latency_ms,omitempty"`          // 95th percentile latency in milliseconds
	MaxThroughput   int   `json:"max_throughput_qps,omitempty"`      // Maximum throughput in queries per second
	AvgResponseSize int64 `json:"avg_response_size_bytes,omitempty"` // New in draft
	CpuUsage        int   `json:"cpu_usage_pct,omitempty"`           // New in draft - CPU usage percentage
	MemoryUsage     int64 `json:"memory_usage_bytes,omitempty"`      // New in draft - Memory usage in bytes
}

// SecurityInfo represents security information for a tool (new in draft)
type SecurityInfo struct {
	RequiresAuth   bool     `json:"requires_auth"`
	AuthMethods    []string `json:"auth_methods,omitempty"`
	DataEncrypted  bool     `json:"data_encrypted,omitempty"`
	DataResidency  []string `json:"data_residency,omitempty"`
	Certifications []string `json:"certifications,omitempty"` // e.g., "SOC2", "GDPR"
	DataRetention  string   `json:"data_retention,omitempty"` // e.g., "30d", "90d", "forever"
	PrivacyPolicy  string   `json:"privacy_policy,omitempty"`
	TermsOfService string   `json:"terms_of_service,omitempty"`
}

// SupportInfo represents support information for a tool (new in draft)
type SupportInfo struct {
	Email     string   `json:"email,omitempty"`
	URL       string   `json:"url,omitempty"`
	Docs      string   `json:"docs,omitempty"`
	Status    string   `json:"status,omitempty"` // "stable", "beta", "alpha", "experimental"
	Languages []string `json:"languages,omitempty"`
}

// DeprecationInfo represents deprecation information for a tool (new in draft)
type DeprecationInfo struct {
	Since       string `json:"since,omitempty"`        // Version or date when deprecated
	RemovalDate string `json:"removal_date,omitempty"` // When it will be removed
	Alternative string `json:"alternative,omitempty"`  // Recommended alternative
	Message     string `json:"message,omitempty"`      // Deprecation message
}

// ToolExample represents an example of tool usage (new in draft)
type ToolExample struct {
	Description string                 `json:"description"`
	Arguments   map[string]interface{} `json:"arguments"` // Example input arguments
	Result      interface{}            `json:"result,omitempty"`
}

// ValidateToolDefinition validates a tool definition
func ValidateToolDefinition(tool ToolDefinition) error {
	if tool.Name == "" {
		return ErrInvalidToolDefinition("tool name is required")
	}
	if tool.Description == "" {
		return ErrInvalidToolDefinition("tool description is required")
	}
	if len(tool.Schema) == 0 {
		return ErrInvalidToolDefinition("tool schema is required")
	}

	// Validate schema format
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(tool.Schema, &schemaObj); err != nil {
		return ErrInvalidToolDefinition(fmt.Sprintf("invalid schema JSON: %v", err))
	}

	// Additional validation for deprecated tools
	if tool.Deprecated && tool.Metadata.Deprecated == nil {
		return ErrInvalidToolDefinition("deprecated tools should include deprecation metadata")
	}

	return nil
}

// ErrInvalidToolDefinition represents an error for an invalid tool definition
type ErrInvalidToolDefinition string

func (e ErrInvalidToolDefinition) Error() string {
	return string(e)
}
