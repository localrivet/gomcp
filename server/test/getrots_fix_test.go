package test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestGetRootsFix verifies that the GetRoots() method works correctly
// in both MCP session environments and test environments with metadata fallback
func TestGetRootsFix(t *testing.T) {
	// Create a temporary workspace for testing
	workspace, err := os.MkdirTemp("", "gomcp-test-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	t.Run("test_environment_with_metadata", func(t *testing.T) {
		// Create server context with workspace roots in metadata (test scenario)
		ctx := &server.Context{
			Logger: slog.Default(),
			Metadata: map[string]interface{}{
				"roots": []string{workspace},
			},
		}

		// Test GetRoots() method
		roots := ctx.GetRoots()

		t.Logf("Expected workspace: %s", workspace)
		t.Logf("GetRoots() returned: %v", roots)
		t.Logf("Length: %d", len(roots))

		// Verify the fix works
		if len(roots) == 0 {
			t.Errorf("FAILED: GetRoots() still returns empty slice instead of [%s]", workspace)
		}

		// Verify correct roots are returned
		expectedRoots := []string{workspace}
		if len(roots) != len(expectedRoots) {
			t.Errorf("Expected %d roots, got %d", len(expectedRoots), len(roots))
		}

		if len(roots) > 0 && roots[0] != expectedRoots[0] {
			t.Errorf("Expected root %s, got %s", expectedRoots[0], roots[0])
		}
	})

	t.Run("mcp_session_roots_take_priority", func(t *testing.T) {
		// Create a server with actual roots
		svr := server.NewServer("test-server")
		mcpRoot := "/path/to/mcp/root"
		svr.Root(mcpRoot)

		// Create a proper context using NewContext (the one way to do things)
		// Use a dummy request for context creation
		dummyRequest := `{"jsonrpc":"2.0","id":1,"method":"test"}`
		ctx, err := server.NewContext(context.Background(), []byte(dummyRequest), svr.GetServer())
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}

		// Add metadata with workspace roots (should be ignored since server has roots)
		ctx.Metadata["roots"] = []string{workspace}

		// Test GetRoots() method
		roots := ctx.GetRoots()

		// Should return MCP session roots, not metadata roots
		if len(roots) != 1 || roots[0] != mcpRoot {
			t.Errorf("Expected MCP session root [%s], got %v", mcpRoot, roots)
		}
	})

	t.Run("empty_metadata_returns_empty", func(t *testing.T) {
		// Create context with no roots in metadata
		ctx := &server.Context{
			Logger:   slog.Default(),
			Metadata: map[string]interface{}{},
		}

		// Test GetRoots() method
		roots := ctx.GetRoots()

		// Should return empty slice
		if len(roots) != 0 {
			t.Errorf("Expected empty roots, got %v", roots)
		}
	})

	t.Run("nil_metadata_returns_empty", func(t *testing.T) {
		// Create context with nil metadata
		ctx := &server.Context{
			Logger:   slog.Default(),
			Metadata: nil,
		}

		// Test GetRoots() method
		roots := ctx.GetRoots()

		// Should return empty slice
		if len(roots) != 0 {
			t.Errorf("Expected empty roots, got %v", roots)
		}
	})

	t.Run("invalid_metadata_type_returns_empty", func(t *testing.T) {
		// Create context with invalid roots type in metadata
		ctx := &server.Context{
			Logger: slog.Default(),
			Metadata: map[string]interface{}{
				"roots": "not-a-slice", // Wrong type
			},
		}

		// Test GetRoots() method
		roots := ctx.GetRoots()

		// Should return empty slice
		if len(roots) != 0 {
			t.Errorf("Expected empty roots for invalid type, got %v", roots)
		}
	})

	t.Run("multiple_roots_in_metadata", func(t *testing.T) {
		// Create multiple test workspaces
		workspace2, err := os.MkdirTemp("", "gomcp-test2-*")
		if err != nil {
			t.Fatalf("Failed to create second workspace: %v", err)
		}
		defer os.RemoveAll(workspace2)

		// Create context with multiple workspace roots in metadata
		expectedRoots := []string{workspace, workspace2}
		ctx := &server.Context{
			Logger: slog.Default(),
			Metadata: map[string]interface{}{
				"roots": expectedRoots,
			},
		}

		// Test GetRoots() method
		roots := ctx.GetRoots()

		// Verify all roots are returned
		if len(roots) != len(expectedRoots) {
			t.Errorf("Expected %d roots, got %d", len(expectedRoots), len(roots))
		}

		for i, expectedRoot := range expectedRoots {
			if i >= len(roots) || roots[i] != expectedRoot {
				t.Errorf("Expected root[%d]=%s, got %s", i, expectedRoot, roots[i])
			}
		}
	})
}

// TestWorkspacePathResolution demonstrates how the fix enables proper workspace path resolution
func TestWorkspacePathResolution(t *testing.T) {
	// Create a temporary workspace
	workspace, err := os.MkdirTemp("", "gomcp-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	// Create a test file in the workspace
	testFile := "test.txt"
	testFilePath := filepath.Join(workspace, testFile)
	if err := os.WriteFile(testFilePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("workspace_path_resolution_now_works", func(t *testing.T) {
		// Create context with workspace roots in metadata
		ctx := &server.Context{
			Logger: slog.Default(),
			Metadata: map[string]interface{}{
				"roots": []string{workspace},
			},
		}

		// This function simulates how tools typically resolve workspace paths
		resolveWorkspacePath := func(ctx *server.Context, inputPath string) (string, error) {
			workspaceRoots := ctx.GetRoots() // ✅ Now returns workspace from metadata

			if len(workspaceRoots) == 0 {
				// Falls back to current directory - this was the problem before
				return filepath.Abs(inputPath)
			}

			// Should resolve relative to workspace root
			primaryRoot := workspaceRoots[0]
			return filepath.Join(primaryRoot, inputPath), nil
		}

		// Test path resolution
		resolvedPath, err := resolveWorkspacePath(ctx, testFile)
		if err != nil {
			t.Fatalf("Failed to resolve workspace path: %v", err)
		}

		// Should resolve to the test file in the workspace
		if resolvedPath != testFilePath {
			t.Errorf("Expected resolved path %s, got %s", testFilePath, resolvedPath)
		}

		// Verify the file actually exists at the resolved path
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			t.Errorf("Resolved path does not exist: %s", resolvedPath)
		}
	})
}

// TestGetPrimaryRootFix verifies that GetPrimaryRoot also works with the metadata fallback
func TestGetPrimaryRootFix(t *testing.T) {
	workspace, err := os.MkdirTemp("", "gomcp-primary-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	t.Run("primary_root_from_metadata", func(t *testing.T) {
		ctx := &server.Context{
			Logger: slog.Default(),
			Metadata: map[string]interface{}{
				"roots": []string{workspace, "/other/root"},
			},
		}

		primaryRoot := ctx.GetPrimaryRoot()
		if primaryRoot != workspace {
			t.Errorf("Expected primary root %s, got %s", workspace, primaryRoot)
		}
	})
}

// TestInRootsFix verifies that InRoots also works with the metadata fallback
func TestInRootsFix(t *testing.T) {
	workspace, err := os.MkdirTemp("", "gomcp-inroots-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	t.Run("in_roots_with_metadata", func(t *testing.T) {
		// Create a server with no roots set
		svr := server.NewServer("test-server")

		// Create a proper context using NewContext (the one way to do things)
		dummyRequest := `{"jsonrpc":"2.0","id":1,"method":"test"}`
		ctx, err := server.NewContext(context.Background(), []byte(dummyRequest), svr.GetServer())
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}

		// Add metadata with workspace roots
		ctx.Metadata["roots"] = []string{workspace}

		// Test path within workspace
		testPath := filepath.Join(workspace, "test.txt")
		if !ctx.InRoots(testPath) {
			t.Errorf("Expected path %s to be in roots", testPath)
		}

		// Test path outside workspace
		outsidePath := "/tmp/outside.txt"
		if ctx.InRoots(outsidePath) {
			t.Errorf("Expected path %s to NOT be in roots", outsidePath)
		}
	})
}
