package draft

import (
	"fmt"
	"regexp"
	"strings"
)

// ResourceDefinition represents an MCP resource definition
type ResourceDefinition struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	PathPattern string      `json:"path_pattern"`
	Metadata    interface{} `json:"metadata,omitempty"`
	Versioned   bool        `json:"versioned,omitempty"`
	Streamable  bool        `json:"streamable,omitempty"`
	Cacheable   bool        `json:"cacheable,omitempty"`  // New in draft
	Deprecated  bool        `json:"deprecated,omitempty"` // New in draft
	Category    string      `json:"category,omitempty"`   // New in draft
	Tags        []string    `json:"tags,omitempty"`       // New in draft
	Format      string      `json:"format,omitempty"`     // New in draft - e.g., "text", "json", "binary"
	Schema      interface{} `json:"schema,omitempty"`     // New in draft - for data validation
	Permission  string      `json:"permission,omitempty"` // New in draft - "read", "write", "read-write"
}

// ResourceMetadata represents metadata for a resource (new in draft)
type ResourceMetadata struct {
	Author         string                 `json:"author,omitempty"`
	Version        string                 `json:"version,omitempty"`
	Created        string                 `json:"created,omitempty"`
	Modified       string                 `json:"modified,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Category       string                 `json:"category,omitempty"`
	ContentType    string                 `json:"content_type,omitempty"`
	Encoding       string                 `json:"encoding,omitempty"`
	MaxSize        int64                  `json:"max_size,omitempty"`
	Cost           *ResourceCost          `json:"cost,omitempty"`
	Security       *SecurityInfo          `json:"security,omitempty"`
	Support        *SupportInfo           `json:"support,omitempty"`
	DataLifecycle  *DataLifecycle         `json:"data_lifecycle,omitempty"`
	CustomMetadata map[string]interface{} `json:"custom_metadata,omitempty"`
}

// DataLifecycle represents the lifecycle of resource data (new in draft)
type DataLifecycle struct {
	RetentionPeriod string `json:"retention_period,omitempty"` // e.g., "30d", "1y", "forever"
	ArchiveAfter    string `json:"archive_after,omitempty"`
	DeleteAfter     string `json:"delete_after,omitempty"`
	Immutable       bool   `json:"immutable,omitempty"`
	Versioning      bool   `json:"versioning,omitempty"`
}

// ValidateResourceDefinition validates a resource definition
func ValidateResourceDefinition(resource ResourceDefinition) error {
	if resource.Type == "" {
		return ErrInvalidResourceDefinition("resource type is required")
	}
	if resource.Name == "" {
		return ErrInvalidResourceDefinition("resource name is required")
	}
	if resource.Description == "" {
		return ErrInvalidResourceDefinition("resource description is required")
	}
	if resource.PathPattern == "" {
		return ErrInvalidResourceDefinition("resource path pattern is required")
	}

	// Validate path pattern format
	if !strings.HasPrefix(resource.PathPattern, "/") {
		return ErrInvalidResourceDefinition("resource path pattern must start with /")
	}

	// Check for valid path parameter syntax
	if err := validatePathPattern(resource.PathPattern); err != nil {
		return err
	}

	// Additional validation for draft-specific features
	if resource.Deprecated && resource.Format == "" {
		return ErrInvalidResourceDefinition("resource format should be specified for deprecated resources")
	}

	return nil
}

// validatePathPattern validates a path pattern for proper parameter syntax
func validatePathPattern(pattern string) error {
	// Check for balanced curly braces
	openCount := strings.Count(pattern, "{")
	closeCount := strings.Count(pattern, "}")
	if openCount != closeCount {
		return ErrInvalidResourceDefinition("unbalanced curly braces in path pattern")
	}

	// Validate parameter format
	paramRegex := regexp.MustCompile(`\{([^{}]+)\}`)
	matches := paramRegex.FindAllStringSubmatch(pattern, -1)

	for _, match := range matches {
		paramName := match[1]
		// Check if parameter has proper format (alphanumeric + underscore)
		validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
		if !validNameRegex.MatchString(paramName) {
			return ErrInvalidResourceDefinition(fmt.Sprintf("invalid parameter name: %s", paramName))
		}
	}

	return nil
}

// ExtractPathParams extracts path parameters from a concrete path based on a pattern
func ExtractPathParams(pattern string, path string) (map[string]string, error) {
	params := make(map[string]string)

	// Convert pattern to regex
	regexPattern := regexp.QuoteMeta(pattern)
	paramRegex := regexp.MustCompile(`\\\{([^{}]+)\\\}`)
	regexPattern = paramRegex.ReplaceAllString(regexPattern, `([^/]+)`)
	regexPattern = "^" + regexPattern + "$"

	// Extract parameter names
	paramNames := []string{}
	matches := paramRegex.FindAllStringSubmatch(regexp.QuoteMeta(pattern), -1)
	for _, match := range matches {
		paramNames = append(paramNames, match[1])
	}

	// Match path against regex
	re := regexp.MustCompile(regexPattern)
	match := re.FindStringSubmatch(path)
	if match == nil {
		return nil, ErrResourcePathMismatch
	}

	// Populate parameters
	for i, paramName := range paramNames {
		params[paramName] = match[i+1]
	}

	return params, nil
}

// MatchResourcePath determines if a path matches a given resource pattern and extracts parameters
func MatchResourcePath(pattern string, path string) (bool, map[string]string) {
	params, err := ExtractPathParams(pattern, path)
	if err != nil {
		return false, nil
	}
	return true, params
}

// ResourceMatch represents a successful resource pattern match
type ResourceMatch struct {
	Definition ResourceDefinition
	Params     map[string]string
}

// FindMatchingResource finds the first resource definition that matches a given path
func FindMatchingResource(resources []ResourceDefinition, path string) (*ResourceMatch, error) {
	for _, resource := range resources {
		matches, params := MatchResourcePath(resource.PathPattern, path)
		if matches {
			return &ResourceMatch{
				Definition: resource,
				Params:     params,
			}, nil
		}
	}
	return nil, ErrResourceNotFound
}

// ErrInvalidResourceDefinition represents an error for an invalid resource definition
type ErrInvalidResourceDefinition string

func (e ErrInvalidResourceDefinition) Error() string {
	return string(e)
}

// ErrResourceNotFound represents a resource not found error
var ErrResourceNotFound = fmt.Errorf("resource not found")

// ErrResourcePathMismatch represents a path that doesn't match the pattern
var ErrResourcePathMismatch = fmt.Errorf("resource path does not match pattern")
