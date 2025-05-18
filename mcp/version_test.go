package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestVersionDetection(t *testing.T) {
	detector := NewVersionDetector()

	tests := []struct {
		name          string
		message       map[string]interface{}
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name: "explicit draft version",
			message: map[string]interface{}{
				"type":    "request",
				"version": "draft",
				"id":      "123",
			},
			expected:    VersionDraft,
			expectError: false,
		},
		{
			name: "explicit 2025-03-26 version",
			message: map[string]interface{}{
				"type":    "request",
				"version": "2025-03-26",
				"id":      "123",
			},
			expected:    Version20250326,
			expectError: false,
		},
		{
			name: "explicit 2024-11-05 version",
			message: map[string]interface{}{
				"type":    "request",
				"version": "2024-11-05",
				"id":      "123",
			},
			expected:    Version20241105,
			expectError: false,
		},
		{
			name: "no version specified",
			message: map[string]interface{}{
				"type": "request",
				"id":   "123",
			},
			expected:    VersionDraft, // Default version
			expectError: false,
		},
		{
			name: "unsupported version",
			message: map[string]interface{}{
				"type":    "request",
				"version": "2023-01-01",
				"id":      "123",
			},
			expectError:   true,
			errorContains: "unsupported version",
		},
		{
			name: "malformed message",
			message: map[string]interface{}{
				"invalid": true,
			},
			expected:    VersionDraft, // Default version used for malformed messages
			expectError: false,
		},
		{
			name: "v prefix in version",
			message: map[string]interface{}{
				"type":    "request",
				"version": "v2025-03-26",
				"id":      "123",
			},
			expected:    Version20250326,
			expectError: false,
		},
		{
			name: "latest keyword",
			message: map[string]interface{}{
				"type":    "request",
				"version": "latest",
				"id":      "123",
			},
			expected:    VersionDraft, // Latest is draft
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert message to JSON
			messageJSON, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal test message: %v", err)
			}

			// Detect version
			version, err := detector.DetectVersion(messageJSON)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message does not contain expected text. Got: %s, Want: %s", err.Error(), tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if version != tt.expected {
				t.Errorf("Incorrect version. Got: %s, Want: %s", version, tt.expected)
			}
		})
	}
}

func TestVersionNegotiation(t *testing.T) {
	detector := NewVersionDetector()

	tests := []struct {
		name           string
		clientVersions []string
		serverVersions []string
		expected       string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "both support all versions",
			clientVersions: []string{VersionDraft, Version20250326, Version20241105},
			serverVersions: []string{VersionDraft, Version20250326, Version20241105},
			expected:       VersionDraft, // Highest common version
			expectError:    false,
		},
		{
			name:           "client prefers older, server has all",
			clientVersions: []string{Version20241105, Version20250326},
			serverVersions: []string{VersionDraft, Version20250326, Version20241105},
			expected:       Version20241105, // Client's preferred version that server supports
			expectError:    false,
		},
		{
			name:           "server prefers older, client has all",
			clientVersions: []string{VersionDraft, Version20250326, Version20241105},
			serverVersions: []string{Version20250326, Version20241105},
			expected:       Version20250326, // Highest common version
			expectError:    false,
		},
		{
			name:           "no common versions",
			clientVersions: []string{"2023-01-01"},
			serverVersions: []string{VersionDraft, Version20250326, Version20241105},
			expectError:    true,
			errorContains:  "no common version",
		},
		{
			name:           "client sends empty list",
			clientVersions: []string{},
			serverVersions: []string{VersionDraft, Version20250326, Version20241105},
			expectError:    true,
			errorContains:  "client did not provide any versions",
		},
		{
			name:           "server sends empty list",
			clientVersions: []string{VersionDraft, Version20250326, Version20241105},
			serverVersions: []string{},
			expectError:    true,
			errorContains:  "server did not provide any versions",
		},
		{
			name:           "different version formats",
			clientVersions: []string{"v2025-03-26", "v2024-11-05"},
			serverVersions: []string{"2025-03-26", "2024-11-05"},
			expected:       "v2025-03-26", // Original client version string preserved
			expectError:    false,
		},
		{
			name:           "latest keyword",
			clientVersions: []string{"latest"},
			serverVersions: []string{VersionDraft},
			expected:       "latest", // Original client version string preserved
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Perform version negotiation
			version, err := detector.NegotiateVersion(tt.clientVersions, tt.serverVersions)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message does not contain expected text. Got: %s, Want: %s", err.Error(), tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if version != tt.expected {
				t.Errorf("Incorrect negotiated version. Got: %s, Want: %s", version, tt.expected)
			}
		})
	}
}

func TestVersionCompatibility(t *testing.T) {
	detector := NewVersionDetector()

	tests := []struct {
		name     string
		version1 string
		version2 string
		expected bool
	}{
		{
			name:     "same version - draft",
			version1: VersionDraft,
			version2: VersionDraft,
			expected: true,
		},
		{
			name:     "same version - 2025-03-26",
			version1: Version20250326,
			version2: Version20250326,
			expected: true,
		},
		{
			name:     "draft and latest stable",
			version1: VersionDraft,
			version2: Version20250326,
			expected: true,
		},
		{
			name:     "latest stable and draft",
			version1: Version20250326,
			version2: VersionDraft,
			expected: true,
		},
		{
			name:     "different versions - not compatible",
			version1: Version20250326,
			version2: Version20241105,
			expected: false,
		},
		{
			name:     "draft and older version - not compatible",
			version1: VersionDraft,
			version2: Version20241105,
			expected: false,
		},
		{
			name:     "v prefix and without",
			version1: "v2025-03-26",
			version2: "2025-03-26",
			expected: true,
		},
		{
			name:     "latest keyword and actual version",
			version1: "latest",
			version2: VersionDraft,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compatible := detector.IsVersionCompatible(tt.version1, tt.version2)
			if compatible != tt.expected {
				t.Errorf("Unexpected compatibility result. Got: %v, Want: %v", compatible, tt.expected)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"draft", "draft"},
		{"DRAFT", "draft"},
		{"2025-03-26", "2025-03-26"},
		{"2024-11-05", "2024-11-05"},
		{"v2025-03-26", "2025-03-26"},
		{"V2025-03-26", "2025-03-26"},
		{"latest", NormalizeVersion(SupportedVersions[0])},
		{"current", NormalizeVersion(SupportedVersions[0])},
		{"stable", NormalizeVersion(SupportedVersions[1])},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeVersion(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetSupportedVersions(t *testing.T) {
	versions := GetSupportedVersions()
	if !reflect.DeepEqual(versions, SupportedVersions) {
		t.Errorf("GetSupportedVersions() returned %v, want %v", versions, SupportedVersions)
	}
}

func TestGetCompatibilityMatrix(t *testing.T) {
	matrix := GetCompatibilityMatrix()

	// Check that all supported versions are in the matrix
	for _, version := range SupportedVersions {
		if _, ok := matrix[version]; !ok {
			t.Errorf("Version %s missing from compatibility matrix", version)
		}
	}

	// Check a few specific compatibility rules
	if !containsVersion(matrix[VersionDraft], VersionDraft) {
		t.Errorf("Draft should be compatible with itself")
	}

	if !containsVersion(matrix[VersionDraft], Version20250326) {
		t.Errorf("Draft should be compatible with latest stable version")
	}

	if containsVersion(matrix[Version20250326], Version20241105) {
		t.Errorf("Latest stable version should not be compatible with older stable version")
	}
}

func TestVersionAdapter(t *testing.T) {
	detector := NewVersionDetector()

	tests := []struct {
		name        string
		fromVersion string
		toVersion   string
		expectError bool
	}{
		{
			name:        "compatible versions",
			fromVersion: VersionDraft,
			toVersion:   Version20250326,
			expectError: false,
		},
		{
			name:        "incompatible versions",
			fromVersion: Version20250326,
			toVersion:   Version20241105,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := detector.GetVersionAdapter(tt.fromVersion, tt.toVersion)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if adapter == nil {
				t.Errorf("Expected adapter but got nil")
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s string, substr string) bool {
	return strings.Contains(s, substr)
}

// Helper function to check if a slice of strings contains a specific version
func containsVersion(versions []string, version string) bool {
	for _, v := range versions {
		if v == version {
			return true
		}
	}
	return false
}
