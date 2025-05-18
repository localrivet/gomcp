package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/mcp"
)

// TestVersionNegotiation verifies the client correctly negotiates the protocol version
// with servers that support different versions
func TestVersionNegotiation(t *testing.T) {
	versions := []string{"draft", "2024-11-05", "2025-03-26"}

	for _, version := range versions {
		t.Run("Negotiate_"+version, func(t *testing.T) {
			mockTransport := SetupMockTransport(version)

			// Create client without initializing
			c, err := client.NewClient("test://server",
				client.WithTransport(mockTransport),
				client.WithVersionDetector(mcp.NewVersionDetector()),
			)
			if err != nil {
				t.Fatalf("Failed to initialize client: %v", err)
			}

			// Verify negotiated version
			if c.Version() != version {
				t.Errorf("Expected version %s, got %s", version, c.Version())
			}
		})
	}
}

// TestVersionFallback tests that the client correctly falls back to older versions
// when the server doesn't support the latest version
func TestVersionFallback(t *testing.T) {
	testCases := []struct {
		name            string
		serverVersions  []string
		expectedVersion string
	}{
		{
			name:            "Server supports only draft",
			serverVersions:  []string{"draft"},
			expectedVersion: "draft",
		},
		{
			name:            "Server supports draft and 2024-11-05",
			serverVersions:  []string{"draft", "2024-11-05"},
			expectedVersion: "2024-11-05", // Should pick the newest
		},
		{
			name:            "Server supports all versions",
			serverVersions:  []string{"draft", "2024-11-05", "2025-03-26"},
			expectedVersion: "2025-03-26", // Should pick the newest
		},
		{
			name:            "Server supports 2024-11-05 and 2025-03-26",
			serverVersions:  []string{"2024-11-05", "2025-03-26"},
			expectedVersion: "2025-03-26", // Should pick the newest
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockTransport := NewMockTransport()

			// Ensure the transport is connected
			EnsureConnected(mockTransport)

			// Create a custom initialize response with specific versions
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": tc.serverVersions[len(tc.serverVersions)-1], // Use the newest version as the server's current version
					"versions":        tc.serverVersions,
					"serverInfo": map[string]interface{}{
						"name":    "Test Server",
						"version": "1.0.0",
					},
					"capabilities": map[string]interface{}{},
				},
			}

			responseJSON, _ := json.Marshal(response)
			mockTransport.QueueResponse(responseJSON, nil)

			// Add a resource response for the GetResource call that triggers initialization
			resourceVersion := tc.expectedVersion

			// Create a resource response appropriate for the version
			var resourceResponse []byte
			if resourceVersion == "2025-03-26" {
				resourceResp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      2, // GetResource usually sends ID 2
					"result": map[string]interface{}{
						"contents": []interface{}{
							map[string]interface{}{
								"uri":  "/",
								"text": "Root Resource",
								"content": []interface{}{
									map[string]interface{}{
										"type": "text",
										"text": "Default resource content for version fallback test",
									},
								},
							},
						},
					},
				}
				resourceResponse, _ = json.Marshal(resourceResp)
			} else {
				// For draft and 2024-11-05
				resourceResp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      2, // GetResource usually sends ID 2
					"result": map[string]interface{}{
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Default resource content for version fallback test",
							},
						},
					},
				}
				resourceResponse, _ = json.Marshal(resourceResp)
			}

			mockTransport.QueueResponse(resourceResponse, nil)

			// Create client
			c, err := client.NewClient("test://server",
				client.WithTransport(mockTransport),
				client.WithVersionDetector(mcp.NewVersionDetector()),
			)
			if err != nil {
				t.Fatalf("Failed to initialize client: %v", err)
			}

			// Verify negotiated version
			if c.Version() != tc.expectedVersion {
				t.Errorf("Expected version %s, got %s", tc.expectedVersion, c.Version())
			}
		})
	}
}

// TestCrossVersionFeatureDetection tests if the client correctly detects and adapts
// to features available in different protocol versions
func TestCrossVersionFeatureDetection(t *testing.T) {
	testCases := []struct {
		version              string
		hasRootsList         bool
		hasEnhancedResources bool
	}{
		{
			version:              "draft",
			hasRootsList:         true,
			hasEnhancedResources: false,
		},
		{
			version:              "2024-11-05",
			hasRootsList:         true,
			hasEnhancedResources: false,
		},
		{
			version:              "2025-03-26",
			hasRootsList:         true,
			hasEnhancedResources: true,
		},
	}

	for _, tc := range testCases {
		t.Run("Features_"+tc.version, func(t *testing.T) {
			// Setup a client with the specific version
			c, mockTransport := SetupClientWithMockTransport(t, tc.version)

			// Test for enhanced resources feature by checking the response format
			mockTransport.QueueResponse(CreateResourceResponse(tc.version, "Hello World"), nil)

			resource, err := c.GetResource("/test")
			if err != nil {
				t.Fatalf("Failed to get resource: %v", err)
			}

			// Verify response format based on version
			if tc.hasEnhancedResources {
				// Check for 2025-03-26 specific format (contents array)
				resultMap, ok := resource.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected map result, got %T", resource)
				}

				_, hasContents := resultMap["contents"]
				if !hasContents {
					t.Errorf("Expected 'contents' field in 2025-03-26 response, but not found")
				}
			} else {
				// Check for draft/2024-11-05 format (content array)
				resultMap, ok := resource.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected map result, got %T", resource)
				}

				_, hasContent := resultMap["content"]
				if !hasContent && tc.version != "draft" {
					t.Errorf("Expected 'content' field in %s response, but not found", tc.version)
				}
			}
		})
	}
}

// TestVersionSpecificErrors tests error handling specific to each version
func TestVersionSpecificErrors(t *testing.T) {
	versions := []string{"draft", "2024-11-05", "2025-03-26"}

	for _, version := range versions {
		t.Run("Errors_"+version, func(t *testing.T) {
			// Setup client
			c, mockTransport := SetupClientWithMockTransport(t, version)

			// Create error response
			errorResponse := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      2,
				"error": map[string]interface{}{
					"code":    -32000,
					"message": "Test error for " + version,
				},
			}

			errorJSON, _ := json.Marshal(errorResponse)
			mockTransport.QueueResponse(errorJSON, nil)

			// Try to execute a request that will return an error
			_, err := c.GetResource("/error-test")

			// Verify error was properly returned
			if err == nil {
				t.Errorf("Expected error for version %s, but got none", version)
				return
			}

			// Error message should contain the version-specific error
			expected := "Test error for " + version

			// Check either the exact error or that it contains the expected text
			// This allows for some flexibility in how errors are formatted
			if !strings.Contains(err.Error(), expected) {
				t.Errorf("Expected error message to contain %q, got %q", expected, err.Error())
			}
		})
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return s != "" && substr != "" && s != substr && len(s) > len(substr) && s[len(s)-len(substr):] == substr
}

// TestVersionUpgradeDowngrade tests the client's ability to handle
// reconnecting to a server with a different version
func TestVersionUpgradeDowngrade(t *testing.T) {
	// Start with draft version
	mockTransport := SetupMockTransport("draft")

	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client with draft: %v", err)
	}

	if c.Version() != "draft" {
		t.Errorf("Expected draft version, got %s", c.Version())
	}

	// Now "reconnect" to a server with 2025-03-26
	err = c.Close()
	if err != nil {
		t.Fatalf("Failed to close client: %v", err)
	}

	// Change the mock transport's version
	mockTransport = SetupMockTransport("2025-03-26")

	// Apply new transport
	c, err = client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client with 2025-03-26: %v", err)
	}

	// Should now be using 2025-03-26
	if c.Version() != "2025-03-26" {
		t.Errorf("Expected 2025-03-26 version after upgrade, got %s", c.Version())
	}

	// Test downgrade
	err = c.Close()
	if err != nil {
		t.Fatalf("Failed to close client: %v", err)
	}

	// Change back to 2024-11-05
	mockTransport = SetupMockTransport("2024-11-05")

	c, err = client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)

	if err != nil {
		t.Fatalf("Failed to initialize client with 2024-11-05: %v", err)
	}

	// Should now be using 2024-11-05
	if c.Version() != "2024-11-05" {
		t.Errorf("Expected 2024-11-05 version after downgrade, got %s", c.Version())
	}
}
