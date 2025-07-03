package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/localrivet/gomcp/server"
)

func main() {
	fmt.Println("GoMCP GetRoots() and Workspace Security Demonstration")
	fmt.Println("===================================================")

	// Create multiple temporary workspaces to demonstrate multiple roots
	workspace1, err := os.MkdirTemp("", "workspace1-*")
	if err != nil {
		fmt.Printf("Failed to create workspace1: %v\n", err)
		return
	}
	defer os.RemoveAll(workspace1)

	workspace2, err := os.MkdirTemp("", "workspace2-*")
	if err != nil {
		fmt.Printf("Failed to create workspace2: %v\n", err)
		return
	}
	defer os.RemoveAll(workspace2)

	fmt.Printf("Created test workspace1: %s\n", workspace1)
	fmt.Printf("Created test workspace2: %s\n\n", workspace2)

	// Create test files in both workspaces
	testFile1 := filepath.Join(workspace1, "project.txt")
	testFile2 := filepath.Join(workspace2, "config.json")
	if err := os.WriteFile(testFile1, []byte("project data"), 0644); err != nil {
		fmt.Printf("Failed to create test file: %v\n", err)
		return
	}
	if err := os.WriteFile(testFile2, []byte(`{"setting": "value"}`), 0644); err != nil {
		fmt.Printf("Failed to create test file: %v\n", err)
		return
	}

	// 1. Demonstrate test environment metadata fallback
	fmt.Println("1. Test Environment Context (metadata fallback):")
	fmt.Println("   This simulates how contexts are created in test environments")

	testCtx := &server.Context{
		Logger: slog.Default(),
		Metadata: map[string]interface{}{
			"roots": []string{workspace1, workspace2},
		},
	}

	roots := testCtx.GetRoots()
	fmt.Printf("   Metadata roots: [%s, %s]\n", workspace1, workspace2)
	fmt.Printf("   GetRoots() returned: %v\n", roots)
	fmt.Printf("   Count: %d\n", len(roots))

	if len(roots) == 2 {
		fmt.Println("   ✅ SUCCESS: GetRoots() returns multiple workspace roots from metadata")
	} else {
		fmt.Println("   ❌ ISSUE: GetRoots() failed to return metadata roots")
	}

	// 2. Demonstrate proper MCP server usage with real roots
	fmt.Println("\n2. Proper MCP Server Root Configuration:")
	fmt.Println("   This shows how to properly configure a server with multiple roots")

	svr := server.NewServer("demo-server")
	// Add both workspaces as valid roots - this is the proper way
	svr.Root(workspace1, workspace2)

	// Create a proper context using NewContext (the proper way)
	dummyRequest := `{"jsonrpc":"2.0","id":1,"method":"test"}`
	mcpCtx, err := server.NewContext(context.Background(), []byte(dummyRequest), svr.GetServer())
	if err != nil {
		fmt.Printf("   Failed to create MCP context: %v\n", err)
		return
	}

	mcpRoots := mcpCtx.GetRoots()
	fmt.Printf("   Configured server roots: [%s, %s]\n", workspace1, workspace2)
	fmt.Printf("   GetRoots() returned: %v\n", mcpRoots)
	fmt.Printf("   Count: %d\n", len(mcpRoots))

	if len(mcpRoots) == 2 {
		fmt.Println("   ✅ SUCCESS: Server properly configured with multiple roots")
	} else {
		fmt.Println("   ❌ ISSUE: Server root configuration failed")
	}

	// 3. Demonstrate security validation with InRoots()
	fmt.Println("\n3. Security Validation with InRoots():")
	fmt.Println("   This shows how to validate file access against workspace boundaries")

	// Test files within allowed roots
	validPath1 := testFile1
	validPath2 := testFile2
	// Test path outside any root (security risk)
	invalidPath := "/tmp/malicious.txt"
	// Test directory traversal attempt
	traversalPath := filepath.Join(workspace1, "../../../etc/passwd")

	fmt.Printf("   Testing path in workspace1: %s\n", validPath1)
	fmt.Printf("   InRoots() result: %v\n", mcpCtx.InRoots(validPath1))

	fmt.Printf("   Testing path in workspace2: %s\n", validPath2)
	fmt.Printf("   InRoots() result: %v\n", mcpCtx.InRoots(validPath2))

	fmt.Printf("   Testing invalid path: %s\n", invalidPath)
	fmt.Printf("   InRoots() result: %v\n", mcpCtx.InRoots(invalidPath))

	fmt.Printf("   Testing traversal attempt: %s\n", traversalPath)
	fmt.Printf("   InRoots() result: %v\n", mcpCtx.InRoots(traversalPath))

	validCount := 0
	if mcpCtx.InRoots(validPath1) {
		validCount++
	}
	if mcpCtx.InRoots(validPath2) {
		validCount++
	}
	invalidCount := 0
	if mcpCtx.InRoots(invalidPath) {
		invalidCount++
	}
	if mcpCtx.InRoots(traversalPath) {
		invalidCount++
	}

	if validCount == 2 && invalidCount == 0 {
		fmt.Println("   ✅ SUCCESS: Security validation working correctly")
	} else {
		fmt.Println("   ❌ ISSUE: Security validation failed")
	}

	// 4. Demonstrate tool-like usage pattern
	fmt.Println("\n4. Tool Implementation Pattern:")
	fmt.Println("   This shows how a tool should safely handle file operations")

	safeFileReader := func(ctx *server.Context, requestedPath string) ([]byte, error) {
		// Step 1: Get workspace roots
		roots := ctx.GetRoots()
		if len(roots) == 0 {
			return nil, fmt.Errorf("no workspace roots configured")
		}

		// Step 2: Resolve the path (handle relative paths)
		var fullPath string
		if filepath.IsAbs(requestedPath) {
			fullPath = requestedPath
		} else {
			// For relative paths, resolve against the first root
			fullPath = filepath.Join(roots[0], requestedPath)
		}

		// Step 3: Security check - CRITICAL for preventing directory traversal
		if !ctx.InRoots(fullPath) {
			return nil, fmt.Errorf("access denied: path %s is outside allowed workspace roots", fullPath)
		}

		// Step 4: Safe to read the file
		return os.ReadFile(fullPath)
	}

	// Test the safe file reader
	fmt.Printf("   Reading valid file: project.txt\n")
	content1, err := safeFileReader(mcpCtx, "project.txt")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Content: %s\n", string(content1))
		fmt.Println("   ✅ SUCCESS: Safe file access worked")
	}

	fmt.Printf("   Attempting to read outside workspace: ../../../etc/passwd\n")
	_, err = safeFileReader(mcpCtx, "../../../etc/passwd")
	if err != nil {
		fmt.Printf("   Correctly blocked: %v\n", err)
		fmt.Println("   ✅ SUCCESS: Security boundary enforced")
	} else {
		fmt.Println("   ❌ SECURITY ISSUE: Dangerous file access allowed!")
	}

	// 5. Demonstrate priority: MCP roots override metadata
	fmt.Println("\n5. Priority Test: MCP Session Roots vs Metadata:")
	fmt.Println("   This shows that proper MCP roots take priority over test metadata")

	// Add metadata to the MCP context (should be ignored)
	mcpCtx.Metadata = map[string]interface{}{
		"roots": []string{"/fake/metadata/root"},
	}

	priorityRoots := mcpCtx.GetRoots()
	fmt.Printf("   Server roots: [%s, %s]\n", workspace1, workspace2)
	fmt.Printf("   Metadata roots: [/fake/metadata/root]\n")
	fmt.Printf("   GetRoots() returned: %v\n", priorityRoots)

	if len(priorityRoots) == 2 && priorityRoots[0] == workspace1 {
		fmt.Println("   ✅ SUCCESS: MCP session roots correctly take priority")
	} else {
		fmt.Println("   ❌ ISSUE: Priority logic failed")
	}

	fmt.Println("\n🎉 All demonstrations completed!")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("✅ Always use server.Root() to configure workspace boundaries")
	fmt.Println("✅ Always validate paths with ctx.InRoots() before file operations")
	fmt.Println("✅ Test environments can use metadata fallback when needed")
	fmt.Println("✅ Multiple workspace roots are supported and secure")
	fmt.Println("✅ Directory traversal attacks are prevented")
	fmt.Println("✅ MCP session roots take priority over metadata")
}
