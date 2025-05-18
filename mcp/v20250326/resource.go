package v20250326

import (
	"fmt"
	"strings"
)

// ResourceDefinition represents an MCP resource definition
type ResourceDefinition struct {
	Path        string           `json:"path"`
	Description string           `json:"description"`
	Methods     []string         `json:"methods,omitempty"`
	Parameters  []ResourceParam  `json:"parameters,omitempty"`
	Metadata    ResourceMetadata `json:"metadata,omitempty"`
	Cache       *CacheConfig     `json:"cache,omitempty"`     // Added for v20250326
	Versioned   bool             `json:"versioned,omitempty"` // Added for v20250326
}

// ResourceParam represents a parameter in a resource path
type ResourceParam struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Format      string `json:"format,omitempty"`     // Added for v20250326
	Validation  string `json:"validation,omitempty"` // Added for v20250326
}

// ResourceMetadata represents metadata for a resource
type ResourceMetadata struct {
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	Cost        *ResourceCost          `json:"cost,omitempty"` // Added for v20250326
}

// CacheConfig represents caching configuration for a resource (v20250326)
type CacheConfig struct {
	Enabled    bool   `json:"enabled"`
	TTL        int    `json:"ttl_seconds,omitempty"`   // Time to live in seconds
	MaxSize    int    `json:"max_size_kb,omitempty"`   // Maximum cache size in KB
	Invalidate string `json:"invalidate_on,omitempty"` // Event that invalidates cache
	VaryBy     string `json:"vary_by,omitempty"`       // Parameter to vary cache by
	Strategy   string `json:"strategy,omitempty"`      // Cache strategy (e.g., "LRU", "MRU")
}

// ValidateResourceDefinition validates a resource definition
func ValidateResourceDefinition(resource ResourceDefinition) error {
	if resource.Path == "" {
		return ErrInvalidResourceDefinition("resource path is required")
	}
	if resource.Description == "" {
		return ErrInvalidResourceDefinition("resource description is required")
	}

	// Check for valid path format and parameter names
	pathParams := extractPathParams(resource.Path)
	declaredParams := make(map[string]bool)

	for _, param := range resource.Parameters {
		declaredParams[param.Name] = true
	}

	for _, paramName := range pathParams {
		if !declaredParams[paramName] {
			return ErrInvalidResourceDefinition(fmt.Sprintf("path parameter '%s' not declared in parameters", paramName))
		}
	}

	// Validate methods, if specified
	if len(resource.Methods) > 0 {
		validMethods := map[string]bool{
			"GET":     true,
			"POST":    true,
			"PUT":     true,
			"DELETE":  true,
			"PATCH":   true,
			"OPTIONS": true,
			"HEAD":    true,
		}

		for _, method := range resource.Methods {
			if !validMethods[strings.ToUpper(method)] {
				return ErrInvalidResourceDefinition(fmt.Sprintf("invalid HTTP method: %s", method))
			}
		}
	}

	// Validate cache configuration if present
	if resource.Cache != nil && resource.Cache.Enabled {
		if resource.Cache.TTL < 0 {
			return ErrInvalidResourceDefinition("cache TTL cannot be negative")
		}
		if resource.Cache.MaxSize < 0 {
			return ErrInvalidResourceDefinition("cache max size cannot be negative")
		}
	}

	return nil
}

// extractPathParams extracts parameter names from a path
// e.g., "/users/{id}/posts/{postId}" -> ["id", "postId"]
func extractPathParams(path string) []string {
	var params []string
	segments := strings.Split(path, "/")

	for _, segment := range segments {
		if len(segment) > 2 && segment[0] == '{' && segment[len(segment)-1] == '}' {
			paramName := segment[1 : len(segment)-1]
			params = append(params, paramName)
		}
	}

	return params
}

// ErrInvalidResourceDefinition represents an error for an invalid resource definition
type ErrInvalidResourceDefinition string

func (e ErrInvalidResourceDefinition) Error() string {
	return string(e)
}
