package v20250326

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
	Streamable  bool            `json:"streamable,omitempty"` // Added for v20250326
}

// ToolInputSchema represents a JSON Schema for tool input
type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertyDetail `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
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
	Examples    []interface{} `json:"examples,omitempty"` // Added for v20250326
}

// ToolMetadata represents metadata for a tool
type ToolMetadata struct {
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	Cost        *ResourceCost          `json:"cost,omitempty"`        // Added for v20250326
	Performance *PerformanceMetrics    `json:"performance,omitempty"` // Added for v20250326
}

// ResourceCost represents the cost of using a resource or tool (v20250326)
type ResourceCost struct {
	Type     string  `json:"type"`               // e.g., "credits", "tokens", "api_calls"
	Amount   float64 `json:"amount"`             // Estimated cost amount
	Currency string  `json:"currency,omitempty"` // Optional currency code
}

// PerformanceMetrics represents performance characteristics of a tool (v20250326)
type PerformanceMetrics struct {
	AvgLatency    int `json:"avg_latency_ms,omitempty"`     // Average latency in milliseconds
	P95Latency    int `json:"p95_latency_ms,omitempty"`     // 95th percentile latency in milliseconds
	MaxThroughput int `json:"max_throughput_qps,omitempty"` // Maximum throughput in queries per second
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

	return nil
}

// ErrInvalidToolDefinition represents an error for an invalid tool definition
type ErrInvalidToolDefinition string

func (e ErrInvalidToolDefinition) Error() string {
	return string(e)
}
