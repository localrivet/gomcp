// Package mcp contains the core types and interfaces for the Model Context Protocol.
//
// This package defines the fundamental types used across all MCP specification versions
// and provides utilities for working with them.
package mcp

// SpecVersion represents an MCP specification version
type SpecVersion string

// Supported MCP specification versions
const (
	SpecVersion20241105 SpecVersion = "2024-11-05"
	SpecVersion20250326 SpecVersion = "2025-03-26"
	SpecVersionDraft    SpecVersion = "draft"
)

// VersionSupport represents which MCP specification versions are supported
type VersionSupport struct {
	V20241105 bool
	V20250326 bool
	Draft     bool
}

// AllVersions returns a VersionSupport with all versions enabled
func AllVersions() VersionSupport {
	return VersionSupport{
		V20241105: true,
		V20250326: true,
		Draft:     true,
	}
}
