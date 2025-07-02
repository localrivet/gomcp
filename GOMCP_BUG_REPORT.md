# GoMCP Bug Report: server.Context.GetRoots() Returns Empty in Test Environments

## ✅ RESOLVED

**Status:** Fixed in commits 867e23b and ae86f4e  
**Date:** 2025-01-02  
**Solution:** Enhanced `Context.GetRoots()` and `Context.InRoots()` to support metadata fallback for test environments, plus fixed MCP notification handling

### Fix Summary

The issues have been resolved through two complementary fixes:

1. **Enhanced GetRoots/InRoots for test environments** (commit 867e23b)
2. **Fixed MCP notification handling** (commit ae86f4e)

#### Changes Made

1. **Enhanced `Context.GetRoots()` method** (`server/context.go`):
   - First tries MCP session roots (existing behavior)
   - Falls back to checking `ctx.Metadata["roots"]` for test environments
   - Returns empty slice if neither source has roots

2. **Enhanced `Context.InRoots()` method** (`server/context.go`):
   - Uses same fallback logic as GetRoots for consistency
   - Implements proper path validation for metadata-sourced roots
   - Maintains security boundaries in all environments

3. **Fixed MCP notification handling** (`server/message.go`):
   - Fixed accidentally commented out `handleInitializedNotification()` call
   - Ensures proper MCP specification compliance
   - Queued notifications are now correctly sent after `notifications/initialized`

#### Test Coverage

**Comprehensive test suite added** (`server/test/getrots_fix_test.go`):
- ✅ Test environment metadata fallback
- ✅ MCP session roots priority
- ✅ InRoots workspace validation  
- ✅ Mixed environment scenarios

**MCP compliance tests fixed**:
- ✅ TestInitializationSequenceCompliance
- ✅ TestNotificationTimingRace
- ✅ TestResourceNotificationSent
- ✅ TestRootsListMCPCompliance

#### Demo Application

**Working demonstration** (`examples/getrots_demo/`):
- Shows GetRoots working in test environments
- Demonstrates MCP session priority
- Validates workspace boundary enforcement
- Confirms path resolution accuracy

### Verification Results

✅ **All project tests pass**  
✅ **MCP specification compliance verified**  
✅ **Backward compatibility maintained**  
✅ **Security boundaries preserved**  
✅ **Performance impact minimal**

### Key Benefits

- **Test Environment Support**: GetRoots now works properly in test scenarios
- **MCP Compliance**: Proper notification timing per specification
- **Workspace Isolation**: Enhanced security for workspace-aware tools  
- **Backward Compatible**: Existing MCP session behavior unchanged
- **Performance**: Minimal overhead, efficient fallback logic

---

## Summary

The `server.Context.GetRoots()` method returns an empty slice when called in test environments or when workspace roots are not properly initialized through the MCP protocol, even when roots are provided through other means (e.g., metadata). This breaks workspace path resolution for tools that depend on workspace isolation.

## Environment

- **GoMCP Version**: Latest (as of 2025-01-02)
- **Go Version**: 1.21+
- **Platform**: macOS (darwin 24.5.0)
- **Context**: Testing MCP server tools that require workspace path resolution

## Problem Description

### Expected Behavior
When creating a `server.Context` with workspace roots (either through MCP session initialization or test setup), `ctx.GetRoots()` should return the configured workspace root paths.

### Actual Behavior
`ctx.GetRoots()` returns an empty slice `[]` in test environments, causing tools to fall back to current working directory behavior instead of respecting workspace boundaries.

### Impact
- **Security**: Workspace isolation is bypassed
- **Functionality**: File operations occur in wrong directories
- **Testing**: Unable to properly test workspace-aware tools

## Reproduction Steps

### 1. Create a Test Context

```go
package main

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "github.com/localrivet/gomcp/server"
)

func main() {
    // Create a temporary workspace
    workspace, _ := os.MkdirTemp("", "test-workspace-*")
    defer os.RemoveAll(workspace)
    
    // Create server context (typical test setup)
    ctx := &server.Context{
        Logger: slog.Default(),
        Metadata: map[string]interface{}{
            "roots": []string{workspace},
        },
    }
    
    // Check what GetRoots() returns
    roots := ctx.GetRoots()
    fmt.Printf("Expected roots: [%s]\n", workspace)
    fmt.Printf("Actual roots: %v\n", roots)
    fmt.Printf("Length: %d\n", len(roots))
}
```

### 2. Run the Test

```bash
go run main.go
```

### 3. Observe Output

```
Expected roots: [/var/folders/p3/3g61cfqj15vb1zjgq9gt0tnr0000gn/T/test-workspace-123456]
Actual roots: []
Length: 0
```

## Detailed Test Case

Here's a comprehensive test that demonstrates the issue:

```go
package main

import (
    "log/slog"
    "os"
    "path/filepath"
    "testing"
    "github.com/localrivet/gomcp/server"
)

func TestGetRootsIssue(t *testing.T) {
    // Create a temporary workspace
    workspace, err := os.MkdirTemp("", "gomcp-test-*")
    if err != nil {
        t.Fatalf("Failed to create workspace: %v", err)
    }
    defer os.RemoveAll(workspace)

    t.Run("demonstrate_getrots_issue", func(t *testing.T) {
        // Create server context with workspace roots in metadata
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
        
        // This assertion fails
        if len(roots) == 0 {
            t.Errorf("ISSUE: GetRoots() returns empty slice instead of [%s]", workspace)
        }
        
        // Expected behavior
        expectedRoots := []string{workspace}
        if len(roots) != len(expectedRoots) || (len(roots) > 0 && roots[0] != expectedRoots[0]) {
            t.Errorf("Expected roots %v, got %v", expectedRoots, roots)
        }
    })
}
```

## Root Cause Analysis

### Current Implementation Issue

The `GetRoots()` method appears to be designed only for real MCP client sessions where:
1. Client sends workspace roots during MCP initialization
2. Server automatically detects and stores these roots through the MCP protocol
3. `GetRoots()` returns the MCP session roots

### Test Environment Problem

In test environments, we create `server.Context` directly without going through the full MCP initialization process, so:
- No MCP client session exists
- No workspace roots are registered through the MCP protocol
- `GetRoots()` has no roots to return

### Impact on Tool Development

This makes it impossible to properly test MCP tools that require workspace isolation:

```go
// This is how tools typically resolve workspace paths
func ResolveWorkspacePath(ctx *server.Context, inputPath string) (string, error) {
    workspaceRoots := ctx.GetRoots()  // ❌ Returns [] in tests
    
    if len(workspaceRoots) == 0 {
        // Falls back to current directory - WRONG!
        return filepath.Abs(inputPath)
    }
    
    // Should resolve relative to workspace root
    primaryRoot := workspaceRoots[0]
    return filepath.Join(primaryRoot, inputPath), nil
}
```

## Proposed Solutions

### Option 1: Enhance GetRoots() for Test Environments

Allow `GetRoots()` to check multiple sources for workspace roots:

```go
func (ctx *Context) GetRoots() []string {
    // First, try MCP session roots (current behavior)
    if sessionRoots := ctx.getSessionRoots(); len(sessionRoots) > 0 {
        return sessionRoots
    }
    
    // Fallback: Check metadata for test environments
    if ctx.Metadata != nil {
        if roots, ok := ctx.Metadata["roots"]; ok {
            if rootSlice, ok := roots.([]string); ok {
                return rootSlice
            }
        }
    }
    
    return []string{}
}
```

### Option 2: Provide Test Helper

Create a test helper function to properly initialize contexts with workspace roots:

```go
// In gomcp/server package
func NewTestContext(workspaceRoots []string, logger *slog.Logger) *Context {
    ctx := &Context{
        Logger: logger,
        Metadata: make(map[string]interface{}),
    }
    
    // Properly initialize workspace roots for testing
    ctx.setWorkspaceRoots(workspaceRoots)
    
    return ctx
}
```

### Option 3: Expose SetRoots Method

Allow direct setting of workspace roots for test scenarios:

```go
// Add method to Context
func (ctx *Context) SetRoots(roots []string) {
    ctx.workspaceRoots = roots
}
```

## Workaround (Current)

For now, developers must work around this by:

1. **Avoiding GetRoots() in tests**: Create separate test paths that don't rely on workspace roots
2. **Mocking the entire context**: Create custom context implementations for testing
3. **Using alternative approaches**: Pass workspace roots through other means

## Expected Fix

The ideal fix would be **Option 1** - enhancing `GetRoots()` to check metadata as a fallback for test environments. This would:

- ✅ Maintain backward compatibility
- ✅ Enable proper testing of workspace-aware tools
- ✅ Not break existing MCP protocol behavior
- ✅ Follow the principle of graceful degradation

## Test Verification

After the fix, this test should pass:

```go
func TestGetRootsFixed(t *testing.T) {
    workspace, _ := os.MkdirTemp("", "test-*")
    defer os.RemoveAll(workspace)
    
    ctx := &server.Context{
        Logger: slog.Default(),
        Metadata: map[string]interface{}{
            "roots": []string{workspace},
        },
    }
    
    roots := ctx.GetRoots()
    
    if len(roots) != 1 || roots[0] != workspace {
        t.Errorf("Expected [%s], got %v", workspace, roots)
    }
}
```

## Additional Context

This issue was discovered while implementing comprehensive workspace path resolution for the `godevtools` project, which provides development tools for MCP servers. The inability to properly test workspace isolation is a significant blocker for ensuring security and correctness of file operations.

The issue affects any MCP tool that needs to:
- Validate file paths against workspace boundaries
- Resolve relative paths to workspace roots
- Implement security restrictions based on workspace access
- Test workspace-aware functionality

## Files Affected

In the gomcp codebase, this likely affects:
- `server/context.go` (or similar) - where `GetRoots()` is implemented
- Any session management code that handles workspace root initialization
- Test utilities and documentation

## Priority

**High** - This prevents proper testing of workspace-aware MCP tools and has security implications for workspace isolation. 