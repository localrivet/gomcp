# Version Compatibility

This document outlines compatibility information for GOMCP, including supported MCP protocol versions and Go versions.

## MCP Protocol Versions

GOMCP supports multiple versions of the MCP protocol:

| Protocol Version | Status    | Notes                                        |
| ---------------- | --------- | -------------------------------------------- |
| `draft`          | Supported | Bleeding edge, may have breaking changes     |
| `2024-11-05`     | Supported | Stable release with core functionality       |
| `2025-03-26`     | Supported | Latest stable release with enhanced features |

### Version Features

- **draft**: Latest development version, includes experimental features (audio sampling, etc.)
- **2024-11-05**: First stable release, includes tools, resources, prompts, and text/image sampling
- **2025-03-26**: Adds support for audio sampling and improved error handling

### Default Version

By default, GOMCP clients attempt to negotiate the protocol version with the server. If not specified, the latest stable version is used.

To explicitly set a protocol version:

```go
// Client-side
client, err := client.NewClient("ws://localhost:8080/mcp",
    client.WithProtocolVersion("2024-11-05"),
)

// Server-side (affects default if client doesn't specify)
server := server.NewServer("example",
    server.WithDefaultProtocolVersion("2024-11-05"),
)
```

## Go Version Compatibility

| GOMCP Version | Minimum Go Version | Recommended Go Version |
| ------------- | ------------------ | ---------------------- |
| v1.0.x        | Go 1.20            | Go 1.21+               |
| v1.1.x        | Go 1.21            | Go 1.21+               |

## Dependency Compatibility

| Dependency        | Versions Tested | Notes                        |
| ----------------- | --------------- | ---------------------------- |
| gorilla/websocket | v1.5.0+         | Used for WebSocket transport |
| go-chi/chi/v5     | v5.0.0+         | Used for HTTP routing        |

## API Stability

- **Stable APIs**: Core client and server interfaces
- **Evolving APIs**: Sampling interfaces, protocol internals
- **Experimental APIs**: Advanced configuration options

## Breaking Changes

Major breaking changes between versions:

### v1.0.x to v1.1.x

- Context API updated to conform to Go standard library patterns
- Sampling API consolidated for better consistency
- Tool registration now supports more return types

### Pre-1.0 to v1.0.0

- Complete API redesign for better usability
- Transport interfaces standardized
- Protocol version negotiation added

## Upgrade Guide

When upgrading between GOMCP versions:

1. Review the changelog for breaking changes
2. Update imports and API usage patterns
3. Test with both old and new protocol versions
4. Update client/server protocol version if needed

For detailed upgrade instructions, see [Upgrading from v1.0.x to v1.1.x](upgrading-1.0-to-1.1.md).
