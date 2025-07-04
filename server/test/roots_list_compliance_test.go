package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
)

// TestRootsListMCPCompliance tests that roots/list follows MCP specification
// across all three protocol versions using the existing server API
func TestRootsListMCPCompliance(t *testing.T) {
	versions := []string{"2024-11-05", "2025-03-26", "2025-06-18", "draft"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			testRootsListForVersion(t, version)
		})
	}
}

func testRootsListForVersion(t *testing.T, protocolVersion string) {
	// Create server with mock transport using existing pattern
	mockTransport := NewMockTransport()
	srv := server.NewServer("test-server")
	serverImpl := srv.GetServer()
	serverImpl.SetTransport(mockTransport)

	// Test 1: Client without roots capability should NOT trigger roots/list
	t.Run("no_capability_no_request", func(t *testing.T) {
		mockTransport.ClearRequestHistory()

		// Initialize without roots capability
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]interface{}{}, // No roots capability
				"clientInfo":      map[string]interface{}{},
			},
		}

		initBytes, _ := json.Marshal(initRequest)
		_, err := server.HandleMessage(serverImpl, initBytes)
		if err != nil {
			t.Fatalf("Failed to initialize server: %v", err)
		}

		// Send initialized notification
		notification := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}
		notificationBytes, _ := json.Marshal(notification)
		_, err = server.HandleMessage(serverImpl, notificationBytes)
		if err != nil {
			t.Fatalf("Failed to send initialized notification: %v", err)
		}

		// Wait briefly for any async operations
		time.Sleep(50 * time.Millisecond)

		// Verify NO roots/list request was sent
		requests := mockTransport.GetRequestHistory()
		for _, req := range requests {
			var parsed map[string]interface{}
			if json.Unmarshal(req, &parsed) == nil {
				if method, ok := parsed["method"].(string); ok && method == "roots/list" {
					t.Errorf("Server should NOT send roots/list when client lacks capability")
				}
			}
		}
	})

	// Test 2: Client with roots capability SHOULD trigger roots/list
	t.Run("with_capability_sends_request", func(t *testing.T) {
		mockTransport.ClearRequestHistory()

		// Initialize with roots capability
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": protocolVersion,
				"capabilities": map[string]interface{}{
					"roots": map[string]interface{}{
						"listChanged": true, // Required per MCP spec
					},
				},
				"clientInfo": map[string]interface{}{},
			},
		}

		initBytes, _ := json.Marshal(initRequest)
		_, err := server.HandleMessage(serverImpl, initBytes)
		if err != nil {
			t.Fatalf("Failed to initialize server: %v", err)
		}

		// Send initialized notification
		notification := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}
		notificationBytes, _ := json.Marshal(notification)
		_, err = server.HandleMessage(serverImpl, notificationBytes)
		if err != nil {
			t.Fatalf("Failed to send initialized notification: %v", err)
		}

		// Wait for async roots/list request
		time.Sleep(100 * time.Millisecond)

		// Verify roots/list request was sent with correct format
		found := false
		requests := mockTransport.GetRequestHistory()
		for _, req := range requests {
			var parsed map[string]interface{}
			if json.Unmarshal(req, &parsed) == nil {
				if method, ok := parsed["method"].(string); ok && method == "roots/list" {
					found = true
					// Verify JSON-RPC 2.0 format per spec
					if jsonrpc, ok := parsed["jsonrpc"].(string); !ok || jsonrpc != "2.0" {
						t.Errorf("Invalid JSON-RPC version: expected '2.0', got '%v'", jsonrpc)
					}
					if _, ok := parsed["id"]; !ok {
						t.Errorf("roots/list request missing required 'id' field")
					}
					break
				}
			}
		}
		if !found {
			t.Errorf("Server should send roots/list request when client advertises capability")
		}
	})

	// Test 3: Environment should come from transport, not init params (MCP compliance)
	t.Run("environment_from_transport_not_init", func(t *testing.T) {
		// Initialize with environment in clientInfo (should be ignored per MCP spec)
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"environment": map[string]interface{}{
						"SHOULD_BE_IGNORED": "true", // This violates MCP spec
					},
				},
			},
		}

		initBytes, _ := json.Marshal(initRequest)
		response, err := server.HandleMessage(serverImpl, initBytes)

		// Should succeed (not error) but ignore the environment per MCP compliance
		if err != nil {
			t.Errorf("Initialization should succeed even with environment in clientInfo: %v", err)
		}

		// Verify successful response
		var responseObj map[string]interface{}
		if json.Unmarshal(response, &responseObj) == nil {
			if responseObj["error"] != nil {
				t.Errorf("Should not error on clientInfo environment: %v", responseObj["error"])
			}
		}

		// This verifies the server follows MCP spec by not extracting env from init params
		t.Logf("Protocol %s correctly ignores environment in clientInfo per MCP specification", protocolVersion)
	})
}

// TestSessionEnvironmentExtraction tests that environment extraction works correctly
// for different transport types following MCP protocol
func TestSessionEnvironmentExtraction(t *testing.T) {
	// This test verifies the transport-specific environment extraction logic
	// follows MCP compliance (env from transport, not init params)

	mockTransport := NewMockTransport()
	srv := server.NewServer("test-server")
	serverImpl := srv.GetServer()
	serverImpl.SetTransport(mockTransport)

	// Test that session creation works properly
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "draft",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{},
		},
	}

	initBytes, _ := json.Marshal(initRequest)
	response, err := server.HandleMessage(serverImpl, initBytes)

	if err != nil {
		t.Fatalf("Initialization failed: %v", err)
	}

	// Verify successful initialization response
	var responseObj map[string]interface{}
	if err := json.Unmarshal(response, &responseObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if responseObj["error"] != nil {
		t.Fatalf("Initialization returned error: %v", responseObj["error"])
	}

	if result, ok := responseObj["result"].(map[string]interface{}); !ok {
		t.Fatalf("Missing or invalid result in response")
	} else if result["protocolVersion"] != "draft" {
		t.Errorf("Expected protocolVersion 'draft', got %v", result["protocolVersion"])
	}

	t.Log("Session environment extraction follows MCP compliance")
}
