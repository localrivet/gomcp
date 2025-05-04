# gomcp Server Composition Parity Plan

This document outlines the plan for implementing server composition features in gomcp to achieve feature parity with FastMCP's server composition capabilities.

## Overview

FastMCP supports two methods of server composition:

1. **Static Composition** (`import_server`): One-time copy of components with prefixing
2. **Dynamic Composition** (`mount`): Live link where the main server delegates requests to the subserver

These features enable modular development, code reuse, and better organization of MCP applications. Adding these features to gomcp will bring it to full feature parity with FastMCP.

## Feature Comparison with FastMCP

| Feature                | FastMCP                                       | Current gomcp | Goal                        |
| ---------------------- | --------------------------------------------- | ------------- | --------------------------- |
| Static Import          | `import_server()` method                      | Not supported | Add `ImportServer()` method |
| Dynamic Mount          | `mount()` method                              | Not supported | Add `Mount()` method        |
| Prefix Customization   | Custom separators for tools/resources/prompts | N/A           | Support custom separators   |
| Direct vs. Proxy Mount | `as_proxy` parameter                          | N/A           | Support both mounting modes |

## Technical Design

### 1. Core Types and Interfaces

```go
// MountedServer represents a server that's been mounted with a prefix
type MountedServer struct {
    Server            *Server
    Prefix            string
    ToolSeparator     string
    ResourceSeparator string
    PromptSeparator   string
}

// Interface for consistent access to server components
type ServerComponents interface {
    GetTools() (map[string]Tool, error)
    GetResources() (map[string]protocol.Resource, error)
    GetPrompts() (map[string]protocol.Prompt, error)
}
```

### 2. Static Import Implementation

For `ImportServer()`, we need to:

- Copy tools, resources, and prompts from source server to destination
- Apply prefixing using configurable separators
- Ensure one-time import (changes to source server aren't reflected)

### 3. Dynamic Mount Implementation

For `Mount()`, we need to:

- Store reference to mounted server with prefix info
- Add a delegate mechanism to route requests to the mounted server
- Implement prefix/separator handling for all component types
- Support both direct and proxy mounting modes

### 4. Integration with Server Methods

Add methods to the `Server` struct:

```go
// ImportServer performs static composition, copying components from srcServer
func (s *Server) ImportServer(prefix string, srcServer *Server, options ...ImportOption) *Server

// Mount performs dynamic composition, creating a live link to srcServer
func (s *Server) Mount(prefix string, srcServer *Server, options ...MountOption) *Server

// Unmount removes a previously mounted server
func (s *Server) Unmount(prefix string) *Server
```

## Implementation Phases

### Phase 1: Core Structure & Static Import

1. Create composition.go file with core types
2. Implement `MountedServer` and related interfaces
3. Add `ImportServer()` method to Server
4. Update Registry to handle prefixed components
5. Add tests for basic static import functionality

### Phase 2: Dynamic Mount Support

1. Implement `Mount()` and `Unmount()` methods
2. Add dynamic routing for requests to mounted servers
3. Update MessageHandler to delegate requests to mounted servers
4. Handle resources/tools list change notifications between mounted servers
5. Add tests for dynamic mount functionality

### Phase 3: Advanced Features

1. Add direct vs proxy mounting modes
2. Implement custom separators for different component types
3. Support cascading mounts (mount a server that has mounts)
4. Add comprehensive documentation and examples

## Testing Strategy

1. Unit Tests:

   - Test prefixing logic for different component types
   - Test import and mount with various separator configurations
   - Test error handling for invalid mount/import operations

2. Integration Tests:

   - Test full request flow through mounted servers
   - Test dynamically adding components to mounted servers
   - Test cascading mounts (server1 mounts server2 which mounts server3)

3. Example Applications:
   - Create demo applications showing both composition methods
   - Create examples with mixed static/dynamic composition

## Documentation

1. Add godoc comments for all new methods and types
2. Create examples in documentation showing both import and mount
3. Add a server composition section to the main documentation
4. Document best practices for server composition

## Acceptance Criteria

1. Both import and mount work as expected in all test cases
2. API is ergonomic and follows Go conventions
3. Performance overhead is minimal, especially for dynamic mounting
4. Full compatibility with other gomcp features
5. Examples demonstrating real-world use cases
