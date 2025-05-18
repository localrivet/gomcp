package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Known MCP specification versions
const (
	VersionDraft    = "draft"
	Version20241105 = "2024-11-05"
	Version20250326 = "2025-03-26"
)

// SupportedVersions is a list of all supported MCP specification versions in order of preference (newest first)
var SupportedVersions = []string{
	VersionDraft,    // Draft is always the most preferred as it has the newest features
	Version20250326, // Next is the latest stable version
	Version20241105, // Then the previous stable version
}

// VersionDetector detects and negotiates MCP versions
type VersionDetector struct {
	DefaultVersion string   // Default version to use when none is specified
	Supported      []string // Supported versions in order of preference (newest first)
}

// NewVersionDetector creates a new version detector with default settings
func NewVersionDetector() *VersionDetector {
	return &VersionDetector{
		DefaultVersion: SupportedVersions[0],
		Supported:      SupportedVersions,
	}
}

// DetectVersion determines the appropriate MCP version based on a message
func (d *VersionDetector) DetectVersion(message []byte) (string, error) {
	// Try to extract version from message
	var msg struct {
		Version string `json:"version"`
		Type    string `json:"type"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return "", fmt.Errorf("failed to parse message for version detection: %w", err)
	}

	// If version is present and non-empty, validate it
	if msg.Version != "" {
		return d.ValidateVersion(msg.Version)
	}

	// If no version is present, use the default version
	return d.DefaultVersion, nil
}

// ValidateVersion checks if a version is supported and returns a normalized version string
func (d *VersionDetector) ValidateVersion(version string) (string, error) {
	// Normalize the version string
	normalizedVersion := NormalizeVersion(version)

	// Check if the normalized version is supported
	for _, supported := range d.Supported {
		if NormalizeVersion(supported) == normalizedVersion {
			return supported, nil
		}
	}

	return "", fmt.Errorf("unsupported version: %s", version)
}

// NormalizeVersion normalizes a version string for comparison
func NormalizeVersion(version string) string {
	// Convert to lowercase
	version = strings.ToLower(version)

	// Remove any "v" prefix
	version = strings.TrimPrefix(version, "v")

	// Handle special cases
	if version == "latest" || version == "current" {
		return NormalizeVersion(SupportedVersions[0])
	}

	if version == "stable" {
		// Stable is the latest non-draft version
		for _, v := range SupportedVersions {
			if v != VersionDraft {
				return NormalizeVersion(v)
			}
		}
	}

	// Handle date format normalization (YYYY-MM-DD)
	if len(version) == 10 && version[4] == '-' && version[7] == '-' {
		return version
	}

	return version
}

// NegotiateVersion handles version negotiation between client and server
func (d *VersionDetector) NegotiateVersion(clientVersions []string, serverVersions []string) (string, error) {
	// If either list is empty, return an error
	if len(clientVersions) == 0 {
		return "", fmt.Errorf("client did not provide any versions")
	}
	if len(serverVersions) == 0 {
		return "", fmt.Errorf("server did not provide any versions")
	}

	// Normalize all versions for comparison
	normalizedClientVersions := make(map[string]string)
	for _, v := range clientVersions {
		normalizedClientVersions[NormalizeVersion(v)] = v
	}

	normalizedServerVersions := make(map[string]string)
	for _, v := range serverVersions {
		normalizedServerVersions[NormalizeVersion(v)] = v
	}

	// Find common versions (preserving client's original version strings)
	commonVersions := []string{}
	for _, clientVersion := range clientVersions {
		normalizedClient := NormalizeVersion(clientVersion)
		for normalized := range normalizedServerVersions {
			if normalizedClient == normalized {
				commonVersions = append(commonVersions, clientVersion)
				break
			}
		}
	}

	if len(commonVersions) == 0 {
		return "", fmt.Errorf("no common version between client %v and server %v", clientVersions, serverVersions)
	}

	// Return the client's most preferred version (first in their list) that the server supports
	return commonVersions[0], nil
}

// IsVersionCompatible checks if two versions are compatible
func (d *VersionDetector) IsVersionCompatible(v1, v2 string) bool {
	// Normalize both versions
	nv1 := NormalizeVersion(v1)
	nv2 := NormalizeVersion(v2)

	// Same normalized version is always compatible
	if nv1 == nv2 {
		return true
	}

	// Draft is compatible with the latest stable version
	if (nv1 == NormalizeVersion(VersionDraft) && nv2 == NormalizeVersion(SupportedVersions[1])) ||
		(nv2 == NormalizeVersion(VersionDraft) && nv1 == NormalizeVersion(SupportedVersions[1])) {
		return true
	}

	// Otherwise, different versions are not compatible
	return false
}

// GetVersionAdapter returns an adapter for converting between versions
// This is a placeholder for future version adaptation functionality
func (d *VersionDetector) GetVersionAdapter(fromVersion, toVersion string) (interface{}, error) {
	// For now, just check if the conversion is possible
	if !d.IsVersionCompatible(fromVersion, toVersion) {
		return nil, fmt.Errorf("no adapter available for converting from %s to %s", fromVersion, toVersion)
	}

	// In the future, this would return an actual adapter object
	// For now, we'll return a simple struct that could be expanded later
	return &VersionAdapter{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}, nil
}

// VersionAdapter is a placeholder for future version adaptation functionality
type VersionAdapter struct {
	FromVersion string
	ToVersion   string
}

// AdaptMessage is a placeholder for adapting a message from one version to another
func (a *VersionAdapter) AdaptMessage(message []byte) ([]byte, error) {
	// This is a placeholder - in a complete implementation, this would
	// actually transform the message between versions
	return message, nil
}

// GetSupportedVersions returns a list of all versions supported by this library
func GetSupportedVersions() []string {
	return SupportedVersions
}

// GetCompatibilityMatrix returns a map showing which versions are compatible with each other
func GetCompatibilityMatrix() map[string][]string {
	detector := NewVersionDetector()
	matrix := make(map[string][]string)

	for _, v1 := range SupportedVersions {
		compatible := []string{}
		for _, v2 := range SupportedVersions {
			if detector.IsVersionCompatible(v1, v2) {
				compatible = append(compatible, v2)
			}
		}
		matrix[v1] = compatible
	}

	return matrix
}
