package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/server"
)

func TestValidateProtocolVersion(t *testing.T) {
	// Create a server instance for testing
	s := server.NewServer("test-server")
	versionDetector := mcp.NewVersionDetector()

	tests := []struct {
		name          string
		version       string
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid version - draft",
			version:     "draft",
			expected:    "draft",
			expectError: false,
		},
		{
			name:        "valid version - 2025-03-26",
			version:     "2025-03-26",
			expected:    "2025-03-26",
			expectError: false,
		},
		{
			name:        "valid version - 2024-11-05",
			version:     "2024-11-05",
			expected:    "2024-11-05",
			expectError: false,
		},
		{
			name:        "empty version uses default",
			version:     "",
			expected:    versionDetector.DefaultVersion,
			expectError: false,
		},
		{
			name:          "unsupported version",
			version:       "2023-01-01",
			expectError:   true,
			errorContains: "unsupported protocol version",
		},
		{
			name:        "version with v prefix",
			version:     "v2025-03-26",
			expected:    "2025-03-26",
			expectError: false,
		},
		{
			name:        "latest keyword",
			version:     "latest",
			expected:    "draft", // Latest is draft
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Access ValidateProtocolVersion through type assertion
			validator := s.(interface {
				ValidateProtocolVersion(string) (string, error)
			})

			result, err := validator.ValidateProtocolVersion(tt.version)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error does not contain expected text. Got: %v, Want contains: %s", err, tt.errorContains)
					return
				}
				return
			}

			// If not expecting error but got one
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check result
			if result != tt.expected {
				t.Errorf("Incorrect version. Got: %s, Want: %s", result, tt.expected)
			}
		})
	}
}

func TestExtractProtocolVersion(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]interface{}
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid params with protocol version",
			params: map[string]interface{}{
				"protocolVersion": "2025-03-26",
			},
			expected:    "2025-03-26",
			expectError: false,
		},
		{
			name:        "nil params",
			params:      nil,
			expected:    "",
			expectError: false,
		},
		{
			name:        "empty params",
			params:      map[string]interface{}{},
			expected:    "",
			expectError: false,
		},
		{
			name: "invalid params type",
			params: map[string]interface{}{
				"protocolVersion": 123,
			},
			expected:    "123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert params to JSON
			var paramsJSON json.RawMessage
			if tt.params != nil {
				jsonBytes, err := json.Marshal(tt.params)
				if err != nil {
					t.Fatalf("Failed to marshal test params: %v", err)
				}
				paramsJSON = jsonBytes
			}

			// Extract protocol version
			result, err := server.ExtractProtocolVersion(paramsJSON)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error does not contain expected text. Got: %v, Want contains: %s", err, tt.errorContains)
					return
				}
				return
			}

			// If not expecting error but got one
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check result
			if result != tt.expected {
				t.Errorf("Incorrect version. Got: %s, Want: %s", result, tt.expected)
			}
		})
	}
}
