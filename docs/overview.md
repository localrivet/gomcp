---
layout: default
title: Overview
nav_order: 2 # Appears after Home (nav_order: 1)
---

# GoMCP Library Overview

This document provides a higher-level overview of the `gomcp` library architecture and core concepts.

## Goal

The primary goal of `gomcp` is to provide idiomatic Go tools for building applications that communicate using the Model Context Protocol (MCP). This includes:

- **MCP Servers:** Applications that expose tools or resources to MCP clients (often language models or agents).
- **MCP Clients:** Applications that connect to MCP servers to utilize their offered tools and resources.

## Core Components (Root Package)

The main library code resides in the root package (`github.com/localrivet/gomcp`).

1.  **`protocol.go`**:

    - Defines Go structs mapping to MCP concepts (e.g., `Tool`, `Resource`, `Prompt`, `ClientCapabilities`, `ServerCapabilities`).
    - Defines Go structs for specific request parameters and results (e.g., `InitializeRequestParams`, `InitializeResult`, `CallToolParams`, `CallToolResult`).
    - Defines Go structs for JSON-RPC 2.0 base messages (`JSONRPCRequest`, `JSONRPCResponse`, `JSONRPCNotification`) and error payloads (`ErrorPayload`).
    - Includes constants for MCP method names (e.g., `MethodInitialize`, `MethodCallTool`, `MethodCancelled`) and the supported protocol version (`CurrentProtocolVersion`).
    - Uses standard Go `encoding/json` tags.

2.  **`transport.go`**:

    - Handles low-level JSON-RPC 2.0 communication mechanics over stdio.
    - Provides a `Connection` struct abstracting `io.Reader` and `io.Writer`.
    - Implements methods for sending specific JSON-RPC message types: `SendRequest`, `SendResponse`, `SendErrorResponse`, `SendNotification`. These handle marshalling, unique ID generation (for requests), and writing newline-delimited JSON.
    - Implements `ReceiveRawMessage` which reads a newline-delimited line and performs basic JSON validation.
    - Includes a helper `UnmarshalPayload` to decode `interface{}` params/results into specific target structs.

3.  **`server.go`**:

    - Defines the `Server` struct, orchestrating server-side logic.
    - `NewServer` initializes a server instance (using stdio `Connection` by default).
    - `RegisterTool`, `RegisterResource`, `RegisterPrompt` allow adding capabilities dynamically. These methods trigger `list_changed` notifications if supported.
    - `RegisterNotificationHandler` allows handling client-sent notifications (e.g., `$/cancelled`).
    - `Run` is the main entry point. It calls `handleInitialize` and then enters the main message loop.
    - `handleInitialize` implements the server's part of the initialization sequence.
    - The `Run` loop receives raw messages, determines if they are requests or notifications, and dispatches them to appropriate internal handlers (e.g., `handleCallToolRequest`, `handleSubscribeResource`) or registered notification handlers.
    - Internal handlers (`handle...`) are responsible for unmarshalling parameters, performing actions (like calling a registered `ToolHandlerFunc`), and sending responses/errors.
    - Includes `Send...` methods for server-initiated notifications (`SendProgress`, `SendResourceChanged`, etc.).

4.  **`client.go`**:
    - Defines the `Client` struct for managing the client-side connection.
    - `NewClient` initializes a client instance.
    - `RegisterRequestHandler`, `RegisterNotificationHandler` allow handling server-sent requests/notifications.
    - `Connect` implements the client's part of the initialization sequence.
    - Provides methods for sending specific MCP requests and waiting for responses (e.g., `ListTools`, `CallTool`, `SubscribeResources`, `Ping`). These methods use `sendRequestAndWait`.
    - `sendRequestAndWait` handles sending the JSON-RPC request, managing pending requests, and waiting for the corresponding response or timeout.
    - `processIncomingMessages` runs in a background goroutine to receive messages, dispatch responses to waiting callers, and dispatch incoming requests/notifications to registered handlers.
    - Includes `Send...` methods for client-initiated notifications (`SendCancellation`, `SendProgress`, `SendRootsListChanged`).

## Communication Flow (Stdio)

The library currently assumes communication over standard input and output:

```
+--------------+        Stdio Pipe         +-------------+
|              |  <--- JSON Lines ----    |             |
|  MCP Client  |        (Stdin)           |  MCP Server |
| (e.g., Agent)|                          | (Tool Host) |
|              |    ---- JSON Lines --->  |             |
+--------------+        (Stdout)          +-------------+
```

- The server reads requests from its `stdin` and writes responses/notifications to its `stdout`.
- The client reads responses/notifications from its `stdin` (which is connected to the server's `stdout`) and writes requests to its `stdout` (which is connected to the server's `stdin`).
- Messages are single JSON objects per line.

## Next Steps & Future Development

The core library is now compliant with the defined features of the MCP 2025-03-26 specification. Future work includes:

- **Example Updates:** Update all examples in `cmd/` and `examples/` to use the latest refactored library and demonstrate features like cancellation, progress, and subscriptions.
- **Testing:** Add more comprehensive unit and integration tests, especially covering notifications, subscriptions, cancellation, and concurrency.
- **Progress Reporting Helpers:** Consider adding server-side utilities to simplify sending progress updates from tool handlers.
- **Protocol Enhancements:** Implement optional fields mentioned in the spec (e.g., `trace`, `workspaceFolders`, filtering options, content annotations).
- **Error Handling:** Refine error reporting and potentially add more specific MCP error codes for implementation-defined errors.
- **Alternative Transports:** Add examples or support for transports beyond stdio (e.g., WebSockets, TCP).
- **Documentation:** Enhance GoDoc comments and keep `/docs` guides up-to-date.
