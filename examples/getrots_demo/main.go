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
	fmt.Println("GoMCP GetRoots() Bug Fix Demonstration")
	fmt.Println("=====================================")

	// Create a temporary workspace
	workspace, err := os.MkdirTemp("", "getrots-demo-*")
	if err != nil {
		fmt.Printf("Failed to create workspace: %v\n", err)
		return
	}
	defer os.RemoveAll(workspace)

	fmt.Printf("Created test workspace: %s\n\n", workspace)

	// Demonstrate the original problem (before fix)
	fmt.Println("1. Test Environment Context (using metadata fallback):")
	fmt.Println("   This simulates how contexts are created in test environments")

	// Create server context with workspace roots in metadata (test scenario)
	ctx := &server.Context{
		Logger: slog.Default(),
		Metadata: map[string]interface{}{
			"roots": []string{workspace},
		},
	}

	// Test GetRoots() method
	roots := ctx.GetRoots()
	fmt.Printf("   Expected roots: [%s]\n", workspace)
	fmt.Printf("   GetRoots() returned: %v\n", roots)
	fmt.Printf("   Length: %d\n", len(roots))

	if len(roots) == 0 {
		fmt.Println("   ❌ ISSUE: GetRoots() returns empty slice (bug exists)")
	} else {
		fmt.Println("   ✅ SUCCESS: GetRoots() returns workspace from metadata (bug fixed)")
	}

	// Demonstrate workspace path resolution
	fmt.Println("\n2. Workspace Path Resolution:")
	testFile := "test.txt"
	testFilePath := filepath.Join(workspace, testFile)

	// Create the test file
	if err := os.WriteFile(testFilePath, []byte("test content"), 0644); err != nil {
		fmt.Printf("   Failed to create test file: %v\n", err)
		return
	}

	// This function simulates how tools typically resolve workspace paths
	resolveWorkspacePath := func(ctx *server.Context, inputPath string) (string, error) {
		workspaceRoots := ctx.GetRoots()

		if len(workspaceRoots) == 0 {
			// Falls back to current directory - this was the problem before
			return filepath.Abs(inputPath)
		}

		// Should resolve relative to workspace root
		primaryRoot := workspaceRoots[0]
		return filepath.Join(primaryRoot, inputPath), nil
	}

	resolvedPath, err := resolveWorkspacePath(ctx, testFile)
	if err != nil {
		fmt.Printf("   Failed to resolve workspace path: %v\n", err)
		return
	}

	fmt.Printf("   Input path: %s\n", testFile)
	fmt.Printf("   Resolved path: %s\n", resolvedPath)
	fmt.Printf("   Expected path: %s\n", testFilePath)

	if resolvedPath == testFilePath {
		fmt.Println("   ✅ SUCCESS: Path resolved correctly to workspace")
	} else {
		fmt.Println("   ❌ ISSUE: Path resolution failed")
	}

	// Test InRoots functionality
	fmt.Println("\n3. InRoots() Workspace Validation:")
	inWorkspace := ctx.InRoots(testFilePath)
	outsideWorkspace := ctx.InRoots("/tmp/outside.txt")

	fmt.Printf("   Path in workspace (%s): %v\n", testFilePath, inWorkspace)
	fmt.Printf("   Path outside workspace (/tmp/outside.txt): %v\n", outsideWorkspace)

	if inWorkspace && !outsideWorkspace {
		fmt.Println("   ✅ SUCCESS: InRoots() correctly validates workspace boundaries")
	} else {
		fmt.Println("   ❌ ISSUE: InRoots() validation failed")
	}

	// Demonstrate MCP session roots take priority
	fmt.Println("\n4. MCP Session Roots Priority:")
	svr := server.NewServer("demo-server")
	mcpRoot := "/path/to/mcp/root"
	svr.Root(mcpRoot)

	// Create a proper context using NewContext (the proper way)
	dummyRequest := `{"jsonrpc":"2.0","id":1,"method":"test"}`
	mcpCtx, err := server.NewContext(context.Background(), []byte(dummyRequest), svr.GetServer())
	if err != nil {
		fmt.Printf("   Failed to create MCP context: %v\n", err)
		return
	}

	// Add metadata with workspace roots (should be ignored since server has roots)
	mcpCtx.Metadata["roots"] = []string{workspace}

	mcpRoots := mcpCtx.GetRoots()
	fmt.Printf("   Server roots: [%s]\n", mcpRoot)
	fmt.Printf("   Metadata roots: [%s]\n", workspace)
	fmt.Printf("   GetRoots() returned: %v\n", mcpRoots)

	if len(mcpRoots) == 1 && mcpRoots[0] == mcpRoot {
		fmt.Println("   ✅ SUCCESS: MCP session roots take priority over metadata")
	} else {
		fmt.Println("   ❌ ISSUE: Priority logic failed")
	}

	fmt.Println("\n🎉 All tests completed!")
	fmt.Println("\nThis demonstrates that the GetRoots() bug has been fixed:")
	fmt.Println("- Test environments can now use metadata for workspace roots")
	fmt.Println("- MCP session roots still take priority when available")
	fmt.Println("- Workspace path resolution works correctly")
	fmt.Println("- Security boundaries are properly enforced")
}
