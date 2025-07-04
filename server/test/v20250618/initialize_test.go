package v20250618

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestInitializeResponse_V20250618(t *testing.T) {
	// Create a server with some tools and resources to ensure capabilities are declared correctly
	s := server.NewServer("test-server")

	// Register a tool
	s.Tool("test-tool", "A test tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "test result", nil
	})

	// Register a resource
	s.Resource("/test/{id}", "A test resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{"id": "123"}, nil
	})

	// Register a prompt
	s.Prompt("test-prompt", "A test prompt", server.User("Hello {{name}}"))

	tests := []struct {
		name            string
		clientVersion   string
		expectedVersion string
	}{
		{
			name:            "2025-06-18 version request",
			clientVersion:   "2025-06-18",
			expectedVersion: "2025-06-18",
		},
		{
			name:            "latest version request in draft context",
			clientVersion:   "latest",
			expectedVersion: "2025-06-18", // In draft test context, latest resolves to draft
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create initialize request
			initRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]interface{}{
					"protocolVersion": tt.clientVersion,
					"capabilities": map[string]interface{}{
						"roots": map[string]interface{}{
							"listChanged": true,
						},
					},
					"clientInfo": map[string]interface{}{
						"name":    "test-client",
						"version": "1.0.0",
					},
				},
			}

			// Send request and get response
			requestBytes, err := json.Marshal(initRequest)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			responseBytes, err := server.HandleMessage(s.GetServer(), requestBytes)
			if err != nil {
				t.Fatalf("Failed to handle initialize message: %v", err)
			}

			// Parse response
			var response map[string]interface{}
			if err := json.Unmarshal(responseBytes, &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Validate basic JSON-RPC structure
			if response["jsonrpc"] != "2.0" {
				t.Errorf("Expected jsonrpc to be '2.0', got %v", response["jsonrpc"])
			}
			if response["id"] != float64(1) {
				t.Errorf("Expected id to be 1, got %v", response["id"])
			}

			// Get the result
			result, ok := response["result"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected result to be an object, got %T", response["result"])
			}

			// Validate protocol version
			protocolVersion, ok := result["protocolVersion"].(string)
			if !ok {
				t.Fatalf("Expected protocolVersion to be a string, got %T", result["protocolVersion"])
			}
			if protocolVersion != tt.expectedVersion {
				t.Errorf("Expected protocolVersion %s, got %s", tt.expectedVersion, protocolVersion)
			}

			// Validate server info
			serverInfo, ok := result["serverInfo"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected serverInfo to be an object, got %T", result["serverInfo"])
			}
			if name, ok := serverInfo["name"].(string); !ok || name == "" {
				t.Errorf("Expected serverInfo.name to be a non-empty string, got %v", serverInfo["name"])
			}
			if version, ok := serverInfo["version"].(string); !ok || version == "" {
				t.Errorf("Expected serverInfo.version to be a non-empty string, got %v", serverInfo["version"])
			}

			// Validate capabilities structure
			capabilities, ok := result["capabilities"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected capabilities to be an object, got %T", result["capabilities"])
			}

			// Validate that capabilities only contain declaration flags, not actual data
			validateCapabilityFlags(t, capabilities)

			// Since we registered tools, resources, and prompts, these capabilities should be present
			validateToolsCapability(t, capabilities, true)
			validateResourcesCapability(t, capabilities, true)
			validatePromptsCapability(t, capabilities, true)
			validateLoggingCapability(t, capabilities, true)
		})
	}
}

func validateCapabilityFlags(t *testing.T, capabilities map[string]interface{}) {
	// Capabilities should only contain declaration flags, never actual data arrays

	// Check tools capability
	if tools, exists := capabilities["tools"]; exists {
		toolsCap, ok := tools.(map[string]interface{})
		if !ok {
			t.Errorf("Expected tools capability to be an object, got %T", tools)
			return
		}

		// Should have listChanged flag
		if _, ok := toolsCap["listChanged"].(bool); !ok {
			t.Errorf("Expected tools.listChanged to be a boolean, got %T", toolsCap["listChanged"])
		}

		// Should NOT have actual tools array
		if _, hasTools := toolsCap["tools"]; hasTools {
			t.Errorf("tools capability should not contain actual tools array in initialize response")
		}
	}

	// Check resources capability
	if resources, exists := capabilities["resources"]; exists {
		resourcesCap, ok := resources.(map[string]interface{})
		if !ok {
			t.Errorf("Expected resources capability to be an object, got %T", resources)
			return
		}

		// Should have subscribe and listChanged flags
		if _, ok := resourcesCap["subscribe"].(bool); !ok {
			t.Errorf("Expected resources.subscribe to be a boolean, got %T", resourcesCap["subscribe"])
		}
		if _, ok := resourcesCap["listChanged"].(bool); !ok {
			t.Errorf("Expected resources.listChanged to be a boolean, got %T", resourcesCap["listChanged"])
		}

		// Should NOT have actual resources array
		if _, hasResources := resourcesCap["resources"]; hasResources {
			t.Errorf("resources capability should not contain actual resources array in initialize response")
		}
	}

	// Check prompts capability
	if prompts, exists := capabilities["prompts"]; exists {
		promptsCap, ok := prompts.(map[string]interface{})
		if !ok {
			t.Errorf("Expected prompts capability to be an object, got %T", prompts)
			return
		}

		// Should have listChanged flag
		if _, ok := promptsCap["listChanged"].(bool); !ok {
			t.Errorf("Expected prompts.listChanged to be a boolean, got %T", promptsCap["listChanged"])
		}

		// Should NOT have actual prompts array
		if _, hasPrompts := promptsCap["prompts"]; hasPrompts {
			t.Errorf("prompts capability should not contain actual prompts array in initialize response")
		}
	}
}

func validateToolsCapability(t *testing.T, capabilities map[string]interface{}, shouldExist bool) {
	tools, exists := capabilities["tools"]
	if shouldExist && !exists {
		t.Errorf("Expected tools capability to be present")
		return
	}
	if !shouldExist && exists {
		t.Errorf("Expected tools capability to not be present")
		return
	}
	if !exists {
		return
	}

	toolsCap, ok := tools.(map[string]interface{})
	if !ok {
		t.Errorf("Expected tools capability to be an object, got %T", tools)
		return
	}

	// Must have listChanged
	if listChanged, ok := toolsCap["listChanged"].(bool); !ok || !listChanged {
		t.Errorf("Expected tools.listChanged to be true, got %v", toolsCap["listChanged"])
	}
}

func validateResourcesCapability(t *testing.T, capabilities map[string]interface{}, shouldExist bool) {
	resources, exists := capabilities["resources"]
	if shouldExist && !exists {
		t.Errorf("Expected resources capability to be present")
		return
	}
	if !shouldExist && exists {
		t.Errorf("Expected resources capability to not be present")
		return
	}
	if !exists {
		return
	}

	resourcesCap, ok := resources.(map[string]interface{})
	if !ok {
		t.Errorf("Expected resources capability to be an object, got %T", resources)
		return
	}

	// Must have subscribe and listChanged
	if subscribe, ok := resourcesCap["subscribe"].(bool); !ok || !subscribe {
		t.Errorf("Expected resources.subscribe to be true, got %v", resourcesCap["subscribe"])
	}
	if listChanged, ok := resourcesCap["listChanged"].(bool); !ok || !listChanged {
		t.Errorf("Expected resources.listChanged to be true, got %v", resourcesCap["listChanged"])
	}
}

func validatePromptsCapability(t *testing.T, capabilities map[string]interface{}, shouldExist bool) {
	prompts, exists := capabilities["prompts"]
	if shouldExist && !exists {
		t.Errorf("Expected prompts capability to be present")
		return
	}
	if !shouldExist && exists {
		t.Errorf("Expected prompts capability to not be present")
		return
	}
	if !exists {
		return
	}

	promptsCap, ok := prompts.(map[string]interface{})
	if !ok {
		t.Errorf("Expected prompts capability to be an object, got %T", prompts)
		return
	}

	// Must have listChanged
	if listChanged, ok := promptsCap["listChanged"].(bool); !ok || !listChanged {
		t.Errorf("Expected prompts.listChanged to be true, got %v", promptsCap["listChanged"])
	}
}

func validateLoggingCapability(t *testing.T, capabilities map[string]interface{}, shouldExist bool) {
	logging, exists := capabilities["logging"]
	if shouldExist && !exists {
		t.Errorf("Expected logging capability to be present")
		return
	}
	if !shouldExist && exists {
		t.Errorf("Expected logging capability to not be present")
		return
	}
	if !exists {
		return
	}

	loggingCap, ok := logging.(map[string]interface{})
	if !ok {
		t.Errorf("Expected logging capability to be an object, got %T", logging)
	}

	// Logging capability can be empty object or have configuration
	_ = loggingCap // Just validate it's an object
}

func TestInitializeResponseWithoutRegistrations_V20250618(t *testing.T) {
	// Create a server without any registrations
	s := server.NewServer("test-server")

	// Create initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	// Send request and get response
	requestBytes, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	responseBytes, err := server.HandleMessage(s.GetServer(), requestBytes)
	if err != nil {
		t.Fatalf("Failed to handle initialize message: %v", err)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Get the result
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be an object, got %T", response["result"])
	}

	// Validate capabilities structure
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected capabilities to be an object, got %T", result["capabilities"])
	}

	// Since no tools/resources/prompts are registered, those capabilities should not be present
	// But logging should always be present
	validateToolsCapability(t, capabilities, false)
	validateResourcesCapability(t, capabilities, false)
	validatePromptsCapability(t, capabilities, false)
	validateLoggingCapability(t, capabilities, true)
}
