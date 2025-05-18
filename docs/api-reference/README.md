# API Reference

This section provides comprehensive documentation for the GOMCP API.

## Root Package

- [gomcp](gomcp.md) - Core package with version information and entry points

## Main Packages

- [client](client.md) - Client-side implementation of the MCP protocol
- [server](server.md) - Server-side implementation of the MCP protocol
- [mcp](mcp.md) - Core protocol types and version handling

## Transport Packages

- [transport](transport.md) - Transport interface and common utilities
- [transport/stdio](transport-stdio.md) - Standard I/O transport
- [transport/ws](transport-ws.md) - WebSocket transport
- [transport/sse](transport-sse.md) - Server-Sent Events transport
- [transport/http](transport-http.md) - HTTP transport

## Utility Packages

- [util/schema](util-schema.md) - JSON Schema generation and validation
- [util/conversion](util-conversion.md) - Type conversion utilities

## API Structure

### Client API

The Client API provides methods for consuming MCP services:

```go
// Create a client
client, err := client.NewClient("ws://localhost:8080/mcp")

// Call a tool
result, err := client.CallTool("toolName", args)

// Get a resource
resource, err := client.GetResource("/path/to/resource")

// Get a prompt
prompt, err := client.GetPrompt("promptName", variables)
```

### Server API

The Server API provides a fluent interface for creating MCP servers:

```go
// Create a server
server := server.NewServer("serverName").AsStdio()

// Add a tool
server.Tool("toolName", "description", toolHandler)

// Add a resource
server.Resource("/path/to/resource", "description", resourceHandler)

// Add a prompt
server.Prompt("promptName", "description", template)

// Start the server
server.Run()
```

## Generating Documentation

API documentation is automatically generated from source code comments. For local documentation:

```bash
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
```

Then visit [http://localhost:6060/pkg/github.com/localrivet/gomcp/](http://localhost:6060/pkg/github.com/localrivet/gomcp/).
