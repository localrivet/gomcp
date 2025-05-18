# MCP Specification Reference

This section provides access to the Model Context Protocol (MCP) specifications that GOMCP implements.

## Available Specifications

GOMCP implements the following MCP specifications:

| Version                     | Documentation                                                                                   | Status   |
| --------------------------- | ----------------------------------------------------------------------------------------------- | -------- |
| [draft](draft.md)           | [Official Draft Spec](https://github.com/microsoft/mcp/tree/main/specification/draft)           | Evolving |
| [2024-11-05](2024-11-05.md) | [Official 2024-11-05 Spec](https://github.com/microsoft/mcp/tree/main/specification/2024-11-05) | Stable   |
| [2025-03-26](2025-03-26.md) | [Official 2025-03-26 Spec](https://github.com/microsoft/mcp/tree/main/specification/2025-03-26) | Stable   |

## Key Concepts

### Transport Agnostic

MCP is designed to be transport agnostic. GOMCP implements several transport layers:

- Standard I/O
- WebSocket
- HTTP
- Server-Sent Events (SSE)

### Protocol Concepts

The MCP protocol defines several key interaction patterns:

#### Tools

Functions that can be called by the client with structured arguments:

```json
{
  "jsonrpc": "2.0",
  "method": "tools/execute",
  "params": {
    "name": "add",
    "args": {
      "x": 1,
      "y": 2
    }
  },
  "id": 1
}
```

#### Resources

RESTful resources that can be accessed using path-based navigation:

```json
{
  "jsonrpc": "2.0",
  "method": "resources/get",
  "params": {
    "path": "/users/123"
  },
  "id": 2
}
```

#### Prompts

Template-based text generation:

```json
{
  "jsonrpc": "2.0",
  "method": "prompts/execute",
  "params": {
    "name": "greeting",
    "variables": {
      "name": "Alice",
      "service": "GOMCP"
    }
  },
  "id": 3
}
```

#### Sampling

Real-time content generation (text, images, audio):

```json
{
  "jsonrpc": "2.0",
  "method": "sampling/start",
  "params": {
    "contentType": "text",
    "prompt": "Hello, how can I help you?"
  },
  "id": 4
}
```

## JSON-RPC 2.0

MCP uses JSON-RPC 2.0 as its message format:

- Request: `{"jsonrpc": "2.0", "method": "...", "params": {...}, "id": 1}`
- Response: `{"jsonrpc": "2.0", "result": {...}, "id": 1}`
- Error: `{"jsonrpc": "2.0", "error": {"code": -32000, "message": "..."}, "id": 1}`
- Notification: `{"jsonrpc": "2.0", "method": "...", "params": {...}}`

## Error Codes

MCP defines standard error codes:

| Code   | Message          | Description                 |
| ------ | ---------------- | --------------------------- |
| -32700 | Parse error      | Invalid JSON                |
| -32600 | Invalid request  | Request object is not valid |
| -32601 | Method not found | Method does not exist       |
| -32602 | Invalid params   | Invalid method parameters   |
| -32603 | Internal error   | Internal JSON-RPC error     |
| -32000 | Server error     | Generic server error        |

## Protocol Flow

1. Client connects to server
2. Client sends `initialize` request to negotiate protocol version
3. Server responds with capabilities
4. Client makes tool/resource/prompt/sampling requests
5. Client can send `shutdown` to terminate gracefully

For complete protocol details, refer to the linked specifications.
