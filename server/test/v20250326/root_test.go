package v20250326

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestRootHandling20250326 tests root path handling against 2025-03-26 specification
func TestRootHandling20250326(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-v20250326")

	// In the 2025-03-26 specification, roots are still filesystem boundaries
	// But may have additional features or constraints
	multipleRoots := []string{
		"/path/to/main/repo",
		"/path/to/submodules",
		"/path/to/dependencies",
	}
	srv.Root(multipleRoots...)

	// Test the capability - roots/list remains a client method
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "roots/list"
	}`)

	responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Error   interface{} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// In the 2025-03-26 spec, this remains a client method so we expect an error
	if response.Error == nil {
		t.Errorf("Expected error for roots/list, but got nil error")
	}

	// Verify we registered all the roots
	rootPaths := srv.GetServer().GetRoots()
	if len(rootPaths) != 3 {
		t.Errorf("Expected 3 roots, got %d: %v", len(rootPaths), rootPaths)
	}

	// Test file operation security - in 2025-03-26, strict path validation is required
	t.Run("file operation security", func(t *testing.T) {
		// Create a table of paths to test with expected results
		tests := []struct {
			path        string
			allowed     bool
			description string
		}{
			// Standard paths
			{"/path/to/main/repo/src/main.go", true, "Valid file in first root"},
			{"/path/to/submodules/lib/utils.go", true, "Valid file in second root"},
			{"/path/to/dependencies/vendor/module.js", true, "Valid file in third root"},
			{"/path/outside/roots/file.txt", false, "File outside all roots"},

			// Edge cases
			{"/path/to/main/repo", true, "Root path itself"},
			{"/path/to/main/repo/", true, "Root path with trailing slash"},

			// Security tests - these should be blocked according to 2025-03-26 spec
			{"/path/to/main/repo/../secrets.txt", false, "Simple path traversal"},
			{"/path/to/main/repo/../../etc/passwd", false, "Deep path traversal"},
			{"/path/to/main/repo/symlink", true, "Potential symlink (path itself is valid)"},
		}

		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				result := srv.GetServer().IsPathInRoots(test.path)
				if result != test.allowed {
					t.Errorf("Path '%s': expected allowed=%v, got %v",
						test.path, test.allowed, result)
				}
			})
		}
	})

	// Test adding more roots after initial setup
	srv.Root("/path/to/additional/resources")

	rootPaths = srv.GetServer().GetRoots()
	if len(rootPaths) != 4 {
		t.Errorf("Expected 4 roots after adding another, got %d", len(rootPaths))
	}

	// Test with a new path that should be valid with the additional root
	newPath := "/path/to/additional/resources/config.json"
	if !srv.GetServer().IsPathInRoots(newPath) {
		t.Errorf("Expected %s to be within roots after adding new root", newPath)
	}
}
