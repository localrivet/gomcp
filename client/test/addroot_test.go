//go:build !race

package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAddRoot_Success(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2024-11-05")

	// Test AddRoot with valid file:// URI (as required by MCP specification)
	testPath := "file:///Users/almatuck/workspaces/rnd/mcp-client"
	testName := "projectRoot"

	err := c.AddRoot(testPath, testName)
	if err != nil {
		t.Fatalf("AddRoot failed: %v", err)
	}

	// Verify that AddRoot updated the client's local root list
	roots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed: %v", err)
	}

	if len(roots) != 1 {
		t.Fatalf("Expected 1 root, got %d", len(roots))
	}

	if roots[0].URI != testPath {
		t.Errorf("Expected root URI %s, got %s", testPath, roots[0].URI)
	}

	if roots[0].Name != testName {
		t.Errorf("Expected root name %s, got %s", testName, roots[0].Name)
	}

	// Wait for asynchronous notification to be sent
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for roots/list_changed notification")
	}

	// Verify that a notifications/roots/list_changed was sent (not roots/add)
	notifications := mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 roots/list_changed notification, got %d", len(notifications))
	}

	// Verify no roots/add requests were sent (this method doesn't exist in MCP protocol)
	addRequests := mockTransport.GetRequestsByMethod("roots/add")
	if len(addRequests) != 0 {
		t.Errorf("Expected 0 roots/add requests (method doesn't exist in MCP), got %d", len(addRequests))
	}
}

func TestAddRoot_FileURI(t *testing.T) {
	c, _ := SetupClientWithMockTransport(t, "2024-11-05")

	// Test with file:// URI (should work - MCP supports any valid URI)
	testURI := "file:///Users/almatuck/workspaces/rnd/mcp-client"
	testName := "projectRoot"

	err := c.AddRoot(testURI, testName)
	if err != nil {
		t.Fatalf("AddRoot with file:// URI failed: %v", err)
	}

	// Verify the root was added with the exact URI
	roots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed: %v", err)
	}

	if len(roots) != 1 {
		t.Fatalf("Expected 1 root, got %d", len(roots))
	}

	if roots[0].URI != testURI {
		t.Errorf("Expected root URI %s, got %s", testURI, roots[0].URI)
	}
}

func TestAddRoot_DuplicateCheck(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2024-11-05")

	testPath := "file:///Users/almatuck/workspaces/rnd/mcp-client"
	testName := "projectRoot"

	// Add the same root twice
	err := c.AddRoot(testPath, testName)
	if err != nil {
		t.Fatalf("First AddRoot failed: %v", err)
	}

	err = c.AddRoot(testPath, "differentName")
	if err == nil {
		t.Fatal("Expected AddRoot to fail for duplicate URI, but it succeeded")
	}

	if err.Error() != fmt.Sprintf("root with URI %s already exists", testPath) {
		t.Errorf("Expected duplicate error message, got: %s", err.Error())
	}

	// Wait for asynchronous notification to be sent
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for roots/list_changed notification")
	}

	// Verify only one notification was sent (for the successful add)
	notifications := mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 roots/list_changed notification, got %d", len(notifications))
	}

	// Verify no roots/add requests were sent
	addRequests := mockTransport.GetRequestsByMethod("roots/add")
	if len(addRequests) != 0 {
		t.Errorf("Expected 0 roots/add requests (method doesn't exist in MCP), got %d", len(addRequests))
	}
}

func TestAddRoot_PathConversion(t *testing.T) {
	c, _ := SetupClientWithMockTransport(t, "2024-11-05")

	// Test that AddRoot automatically converts paths to file:// URIs (one way of doing things)
	testCases := []struct {
		input      string
		expected   string // For absolute paths, exact match; for relative, we'll check differently
		name       string
		isRelative bool
	}{
		{"/absolute/path", "file:///absolute/path", "absolute path", false},
		{"./relative/path", "", "relative path", true}, // We'll check this differently
		{"file:///already/valid", "file:///already/valid", "already valid file URI", false},
	}

	for _, tc := range testCases {
		err := c.AddRoot(tc.input, "testName")
		if err != nil {
			t.Errorf("AddRoot failed for %s '%s': %v", tc.name, tc.input, err)
			continue
		}

		// Verify the root was stored with the correct file:// URI format
		roots, err := c.GetRoots()
		if err != nil {
			t.Fatalf("GetRoots failed: %v", err)
		}

		if tc.isRelative {
			// For relative paths, just verify they were converted to a file:// URI with absolute path
			found := false
			for _, root := range roots {
				if strings.HasPrefix(root.URI, "file://") && strings.HasSuffix(root.URI, "/relative/path") {
					found = true
					t.Logf("Relative path '%s' correctly converted to absolute file URI: '%s'", tc.input, root.URI)
					break
				}
			}
			if !found {
				t.Errorf("Expected relative path '%s' to be converted to absolute file:// URI, but not found. Actual roots: %v",
					tc.input, roots)
			}
		} else {
			// For absolute paths and already valid URIs, check exact match
			found := false
			for _, root := range roots {
				if root.URI == tc.expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected root with URI '%s' (converted from '%s'), but not found. Actual roots: %v",
					tc.expected, tc.input, roots)
			}
		}

		// Clean up for next test
		c.RemoveRoot(tc.input) // Should work with original input too
	}
}

func TestAddRoot_NotificationFormat(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2024-11-05")

	testPath := "file:///Users/almatuck/workspaces/rnd/mcp-client"
	testName := "projectRoot"

	err := c.AddRoot(testPath, testName)
	if err != nil {
		t.Fatalf("AddRoot failed: %v", err)
	}

	// Wait for asynchronous notification to be sent
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for roots/list_changed notification")
	}

	// Check the notification format
	notifications := mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	var notification map[string]interface{}
	err = json.Unmarshal(notifications[0].Message, &notification)
	if err != nil {
		t.Fatalf("Failed to parse notification: %v", err)
	}

	// Verify notification structure
	if notification["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got %v", notification["jsonrpc"])
	}

	if notification["method"] != "notifications/roots/list_changed" {
		t.Errorf("Expected method 'notifications/roots/list_changed', got %v", notification["method"])
	}

	// Notifications should not have an ID field
	if _, hasID := notification["id"]; hasID {
		t.Error("Notification should not have an 'id' field")
	}

	t.Logf("Notification format: %s", string(notifications[0].Message))
}

func TestAddRoot_GetRoots_LocalOnly(t *testing.T) {
	c, _ := SetupClientWithMockTransport(t, "2024-11-05")

	// Add multiple roots (all must be valid file:// URIs)
	roots := []struct {
		uri  string
		name string
	}{
		{"file:///path/one", "First"},
		{"file:///path/two", "Second"},
		{"file:///path/three", "Third"},
	}

	for _, root := range roots {
		err := c.AddRoot(root.uri, root.name)
		if err != nil {
			t.Fatalf("AddRoot failed for %s: %v", root.uri, err)
		}
	}

	// GetRoots should return all roots from local cache (no server requests)
	retrievedRoots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed: %v", err)
	}

	if len(retrievedRoots) != len(roots) {
		t.Fatalf("Expected %d roots, got %d", len(roots), len(retrievedRoots))
	}

	// Verify all roots are present with correct data
	for i, expectedRoot := range roots {
		if retrievedRoots[i].URI != expectedRoot.uri {
			t.Errorf("Root %d: expected URI %s, got %s", i, expectedRoot.uri, retrievedRoots[i].URI)
		}
		if retrievedRoots[i].Name != expectedRoot.name {
			t.Errorf("Root %d: expected name %s, got %s", i, expectedRoot.name, retrievedRoots[i].Name)
		}
	}
}
