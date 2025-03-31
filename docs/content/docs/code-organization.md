---
title: Package Organization
weight: 90
cascade:
  type: docs
---

## Goal

The primary goal of `gomcp` is to provide idiomatic Go tools for building applications that communicate using the Model Context Protocol (MCP). This includes:

- **MCP Servers:** Applications that expose tools or resources to MCP clients (often language models or agents).
- **MCP Clients:** Applications that connect to MCP servers to utilize their offered tools and resources.

## Core Components

The library is structured into several key packages:

1.  **`protocol/`**:

    - Defines Go structs mapping to MCP concepts (e.g., `Tool`, `Resource`, `Prompt`, `ClientCapabilities`, `ServerCapabilities`).
    - Defines Go structs for specific request parameters and results (e.g., `InitializeRequestParams`, `InitializeResult`, `CallToolParams`, `CallToolResult`).
    - Defines Go structs for JSON-RPC 2.0 base messages (`JSONRPCRequest`, `JSONRPCResponse`, `JSONRPCNotification`) and error payloads (`ErrorPayload`).
    - Includes constants for MCP method names (e.g., `MethodInitialize`, `MethodCallTool`, `MethodCancelled`) and the supported protocol version (`CurrentProtocolVersion`).
    - Uses standard Go `encoding/json` tags.
    - For detailed descriptions of the protocol messages and structures, see the [Protocols]({{< ref "protocols" >}}) section, including:
      - [Messages]({{< ref "protocols/protocol_messages" >}})
      - [Tools]({{< ref "protocols/protocol_tools" >}})
      - [Resources]({{< ref "protocols/protocol_resources" >}})
      - [Prompts]({{< ref "protocols/protocol_prompts" >}})

2.  **`server/`**:

    - Defines the `Server` struct, containing the core transport-agnostic MCP server logic.
    - `NewServer` initializes a server instance, taking server info and options (like a logger).
    - `RegisterTool`, `RegisterResource`, `RegisterPrompt` allow adding capabilities dynamically. These methods trigger `list_changed` notifications if supported.
    - `RegisterNotificationHandler` allows handling client-sent notifications (e.g., `$/cancelled`).
    - `HandleMessage` is the main entry point for processing incoming raw messages (typically called by a transport implementation). It handles the initialization sequence and dispatches requests/notifications to internal handlers.
    - Internal handlers (`handle...`) are responsible for unmarshalling parameters, performing actions (like calling a registered `ToolHandlerFunc`), and generating responses/errors.
    - Includes `Send...` methods for server-initiated notifications (`SendProgress`, `SendResourceChanged`, etc.), which are typically invoked via a `ClientSession` interface implemented by the transport layer.

3.  **`client/`**:

    - Defines the `Client` struct for managing the client-side connection, currently implemented using the SSE+HTTP hybrid transport model.
    - `NewClient` initializes a client instance, requiring the server's base URL and other options.
    - `RegisterRequestHandler`, `RegisterNotificationHandler` allow handling server-sent requests/notifications received over SSE.
    - `Connect` establishes the SSE connection and performs the MCP initialization handshake (sending `initialize` via HTTP POST, receiving response via SSE, sending `initialized` via HTTP POST).
    - Provides methods for sending specific MCP requests (e.g., `ListTools`, `CallTool`, `SubscribeResources`). These methods typically send the request via HTTP POST and wait for the response via the SSE connection.
    - Manages pending requests and dispatches incoming SSE messages (responses, notifications, requests) appropriately.
    - Includes `Send...` methods for client-initiated notifications (`SendCancellation`, `SendRootsListChanged`).

4.  **`transport/`**:

    - Contains different transport implementations.
    - **`stdio/`**: Provides a `StdioTransport` that implements the `types.Transport` interface for communication over standard input/output (newline-delimited JSON). Useful for simple cases or testing.
    - **`sse/`**: Provides an `SSEServer` that handles the server-side of the SSE+HTTP hybrid transport (SSE for server->client, HTTP POST for client->server). The `client` package uses an SSE client library (`github.com/r3labs/sse/v2`) internally to connect to this.
    - _(Other transports like WebSockets or TCP could be added here in the future.)_

5.  **`types/`**:
    - Defines core interfaces like `Transport` and `Logger` used across packages.

## Communication Flow

The library supports multiple communication methods:

### Stdio Transport (`transport/stdio`)

```
+--------------+        Stdio Pipe         +-------------+
|              |  <--- JSON Lines ----    |             |
|  MCP Client  |        (Stdin)           |  MCP Server |
| (App/Script) |                          | (App/Script)|
|              |    ---- JSON Lines --->  |             |
+--------------+        (Stdout)          +-------------+
```

- Simple, direct communication via stdin/stdout.
- Suitable for local inter-process communication or basic examples.
- The `StdioTransport` handles reading/writing newline-delimited JSON.

### SSE + HTTP Hybrid Transport (`transport/sse` + `client`)

```
+--------------+                          +-----------------+
|              | ---- HTTP POST Req ---> |                 |
|  MCP Client  | (e.g., initialize,      |    MCP Server   |
| (Using client|  callTool, initialized) | (Using sse pkg) |
|    package)  |                          |                 |
|              | <--- HTTP POST Resp ---  |                 |
+--------------+ (e.g., callTool result) +-----------------+
       |                                       ^
       | Establish & Maintain SSE Connection   | SSE Events
       +--------<---- SSE Events --------------+ (e.g., endpoint,
                 (e.g., initialize result,       message (notifications,
                  notifications, server reqs))     server requests))
```

- **Client -> Server:** Requests (`initialize`, `callTool`) and Notifications (`initialized`, `$/cancelled`) are sent via HTTP POST requests to a specific message endpoint on the server. Responses to these requests (like `callTool` results) are sent back in the HTTP response body.
- **Server -> Client:** The client establishes a persistent Server-Sent Events (SSE) connection. The server sends asynchronous messages (like `initialize` results, notifications, or server-to-client requests) over this SSE connection.
- This is the primary transport used by the `client` package.

## Next Steps & Future Development

The core library is now compliant with the defined features of the MCP 2025-03-26 specification. Future work includes:

- **Example Updates:** Ensure all examples in `examples/` are up-to-date with the latest library structure and demonstrate features like cancellation, progress, and subscriptions effectively across different transports.
- **Testing:** Add more comprehensive unit and integration tests, especially covering notifications, subscriptions, cancellation, concurrency, and different transport layers.
- **Progress Reporting:** Address the issue where the `ToolHandlerFunc` doesn't have direct access to the `sessionID` needed for `server.SendProgress`. This might require API changes or alternative patterns.
- **Protocol Enhancements:** Implement optional fields mentioned in the spec (e.g., `trace`, `workspaceFolders`, filtering options, content annotations).
- **Error Handling:** Refine error reporting and potentially add more specific MCP error codes for implementation-defined errors.
- **Alternative Transports:** Add examples or support for transports beyond stdio and SSE (e.g., WebSockets, TCP).
- **Documentation:** Enhance GoDoc comments and keep `/docs` guides up-to-date.
