package test

import (
	"path/filepath"
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestRootPathRegistration(t *testing.T) {
	// Create a new server
	svr := server.NewServer("test-server")

	// Initial state should have no roots
	if len(svr.GetServer().GetRoots()) != 0 {
		t.Errorf("Expected no roots initially, got %d", len(svr.GetServer().GetRoots()))
	}

	// Register a single root
	svr.Root("/path/to/root1")

	// Check if root was registered
	if len(svr.GetServer().GetRoots()) != 1 {
		t.Errorf("Expected 1 root, got %d", len(svr.GetServer().GetRoots()))
	}

	normalized := filepath.Clean("/path/to/root1")
	if svr.GetServer().GetRoots()[0] != normalized {
		t.Errorf("Expected root path to be %s, got %s", normalized, svr.GetServer().GetRoots()[0])
	}

	// Register multiple roots in a single call
	svr.Root("/path/to/root2", "/path/to/root3")

	// Check if roots were registered
	if len(svr.GetServer().GetRoots()) != 3 {
		t.Errorf("Expected 3 roots, got %d", len(svr.GetServer().GetRoots()))
	}

	// Try to register a duplicate root
	svr.Root("/path/to/root1")

	// Check that duplicate wasn't added
	if len(svr.GetServer().GetRoots()) != 3 {
		t.Errorf("Expected still 3 roots after duplicate, got %d", len(svr.GetServer().GetRoots()))
	}

	// Check fluent interface
	same := svr.Root("/path/to/root4")
	if same != svr {
		t.Error("Expected Root() to return the same server instance")
	}

	// Check if the new root was added
	if len(svr.GetServer().GetRoots()) != 4 {
		t.Errorf("Expected 4 roots, got %d", len(svr.GetServer().GetRoots()))
	}
}

func TestGetRoots(t *testing.T) {
	// Create a new server
	svr := server.NewServer("test-server")

	// Register some roots
	svr.Root("/path/to/root1", "/path/to/root2")

	// Get the roots
	roots := svr.GetServer().GetRoots()

	// Check if the roots are correct
	if len(roots) != 2 {
		t.Errorf("Expected 2 roots, got %d", len(roots))
	}

	// Check that modifying the returned slice doesn't affect the original
	roots = append(roots, "/path/to/root3")
	if len(svr.GetServer().GetRoots()) != 2 {
		t.Errorf("Expected internal roots to still be 2, got %d", len(svr.GetServer().GetRoots()))
	}
}

func TestIsPathInRoots(t *testing.T) {
	// Create a new server
	svr := server.NewServer("test-server")

	// Test with no roots
	if svr.IsPathInRoots("/path/to/file") {
		t.Error("Expected path not to be in roots when no roots are registered")
	}

	// Register some roots
	svr.Root("/path/to/root1", "/path/to/root2")

	tests := []struct {
		path     string
		expected bool
		name     string
	}{
		{"/path/to/root1/file.txt", true, "direct child of root1"},
		{"/path/to/root2/subdir/file.txt", true, "nested in root2"},
		{"/path/to/root1", true, "exactly root1"},
		{"/path/to/root3/file.txt", false, "not in any root"},
		{"/path/to/file.txt", false, "sibling to roots"},
		{"/path/to/root1/../file.txt", false, "path traversal attempt"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := svr.IsPathInRoots(test.path)
			if result != test.expected {
				t.Errorf("Expected IsPathInRoots(%s) to be %v, got %v",
					test.path, test.expected, result)
			}
		})
	}
}
