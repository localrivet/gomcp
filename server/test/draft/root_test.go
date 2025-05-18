// Package draft contains tests specifically for the draft version of the MCP specification
package draft

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestRootHandlingDraft tests root path handling against draft specification
func TestRootHandlingDraft(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-draft")

	// Set some root paths - this uses the draft specification's concept of filesystem roots
	srv.Root("/path/to/project", "/path/to/another/directory")

	// Create a JSON-RPC request for the roots/list method (client-side method that server would call)
	// We'll simulate the server receiving this request to verify the roots capability
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "roots/list"
	}`)

	// Process the request directly
	responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	// Parse the response - in the draft spec, this should list roots
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

	// Per the specification, server should respond with a method not supported error
	// for client-side methods like roots/list
	if response.Error == nil {
		t.Errorf("Expected error for roots/list, but got nil error")
	}

	// Test if our internal GetRoots method works correctly
	rootPaths := srv.GetServer().GetRoots()
	if len(rootPaths) != 2 {
		t.Errorf("Expected 2 roots, got %d", len(rootPaths))
	}

	// Test access to paths within and outside roots
	// Draft specification requires respecting filesystem roots
	inRootPath := "/path/to/project/file.txt"
	outsideRootPath := "/path/outside/root/file.txt"

	if !srv.GetServer().IsPathInRoots(inRootPath) {
		t.Errorf("Expected %s to be within roots", inRootPath)
	}

	if srv.GetServer().IsPathInRoots(outsideRootPath) {
		t.Errorf("Expected %s to be outside roots", outsideRootPath)
	}
}
