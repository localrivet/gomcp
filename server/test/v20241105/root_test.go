package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestRootHandling20241105 tests root path handling against 2024-11-05 specification
func TestRootHandling20241105(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-v20241105")

	// Set some root paths - the 2024-11-05 specification maintains the same concept of filesystem roots
	srv.Root("/path/to/workspace", "/path/to/config")

	// Add another root with a separate call to verify multiple calls work
	srv.Root("/path/to/resources")

	// Create a JSON-RPC request for the roots/list method
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "roots/list"
	}`)

	// In the 2024-11-05 specification, the roots/list method is a client method,
	// so we'd expect the server to respond with a method not found error
	responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Roots []map[string]interface{} `json:"roots"`
		} `json:"result,omitempty"`
		Error interface{} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// We should get an error for this client-side method
	if response.Error == nil {
		t.Errorf("Expected error for roots/list, but got nil error")
	}

	// Verify we have all the roots we registered
	rootPaths := srv.GetServer().GetRoots()
	if len(rootPaths) != 3 {
		t.Errorf("Expected 3 roots after multiple calls, got %d", len(rootPaths))
	}

	// Test path validation with paths from different roots
	testPaths := []struct {
		path           string
		shouldBeInRoot bool
		description    string
	}{
		{"/path/to/workspace/src/main.go", true, "file in first root"},
		{"/path/to/config/settings.json", true, "file in second root"},
		{"/path/to/resources/images/logo.png", true, "file in third root added separately"},
		{"/path/outside/root/file.txt", false, "file outside any root"},
		{"/path/to/workspace", true, "root path itself"},
		{"/path/to/workspace/../private/secrets.txt", false, "attempted path traversal"},
	}

	for _, tp := range testPaths {
		t.Run(tp.description, func(t *testing.T) {
			result := srv.GetServer().IsPathInRoots(tp.path)
			if result != tp.shouldBeInRoot {
				t.Errorf("For path %s, expected IsPathInRoots to be %v, got %v",
					tp.path, tp.shouldBeInRoot, result)
			}
		})
	}
}
