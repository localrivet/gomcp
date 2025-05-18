package v20241105

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
}

// ResourceParam represents a parameter in a resource path
type ResourceParam struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ResourceMetadata represents metadata for a resource
type ResourceMetadata struct {
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
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
