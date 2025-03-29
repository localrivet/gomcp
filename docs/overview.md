# GoMCP Library Overview

This document provides a higher-level overview of the `gomcp` library architecture and core concepts.

## Goal

The primary goal of `gomcp` is to provide idiomatic Go tools for building applications that communicate using the Model Context Protocol (MCP). This includes:

- **MCP Servers:** Applications that expose tools or resources to MCP clients (often language models or agents).
- **MCP Clients:** Applications that connect to MCP servers to utilize their offered tools and resources.

## Core Components (Root Package)

The main library code resides in the root package (`github.com/localrivet/gomcp`).

1.  **`protocol.go`**:

    - Defines Go structs that map directly to the JSON message types specified by the MCP standard (e.g., `Message`, `HandshakeRequest`, `HandshakeResponse`, `ErrorPayload`).
    - Includes constants for message type strings (`MessageTypeHandshakeRequest`, etc.) and the supported protocol version (`CurrentProtocolVersion`).
    - Uses standard Go `encoding/json` tags for serialization/deserialization.

2.  **`transport.go`**:

    - Handles the low-level communication mechanics.
    - Provides a `Connection` struct abstracting the underlying I/O (currently hardcoded to `os.Stdin` and `os.Stdout`).
    - Implements `SendMessage` which takes a message type and payload struct, marshals it to newline-delimited JSON, and writes it to the output stream. It also handles generating unique `message_id`s.
    - Implements `ReceiveMessage` which reads a newline-delimited line from the input stream, unmarshals it into a generic `Message` struct (keeping the payload as `json.RawMessage`), and performs basic validation.
    - Includes a helper `UnmarshalPayload` to decode the `json.RawMessage` payload into a specific target struct based on the message type determined by the caller.

3.  **`server.go`**:

    - Defines the `Server` struct, which orchestrates the server-side logic.
    - `NewServer` initializes a server instance (currently using the stdio `Connection`).
    - `Run` is the main entry point. It first calls `handleHandshake` and then enters a loop (currently basic) to receive messages.
    - `handleHandshake` implements the server's part of the handshake: receiving `HandshakeRequest`, validating the protocol version, and sending either `HandshakeResponse` or an `Error`.
    - **Future Work:** The `Run` loop needs to be expanded with a dispatcher to route incoming messages (like `ToolDefinitionRequest`, `UseToolRequest`) to appropriate handlers. These handlers would contain the application-specific logic for defining and executing tools.

4.  **`client.go`**:
    - Defines the `Client` struct for managing the client-side connection.
    - `NewClient` initializes a client instance.
    - `Connect` implements the client's part of the handshake: sending `HandshakeRequest` and processing the server's `HandshakeResponse` or `Error`. It stores the `serverName` upon success.
    - **Future Work:** Methods need to be added to send specific requests (e.g., `RequestToolDefinitions()`, `UseTool(name string, args map[string]interface{})`) and handle their corresponding responses or potential errors. A mechanism for handling server-sent notifications might also be needed.

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

- Implement remaining MCP message types in `protocol.go`.
- Implement server-side handling for tool definitions and execution.
- Implement client-side methods for requesting definitions and using tools.
- Add support for resource access messages.
- Add support for server-sent notifications.
- Add comprehensive unit and integration tests.
- Improve error handling and reporting.
- Consider adding support for alternative transports (e.g., WebSockets, TCP).
- Enhance documentation (GoDoc comments, more detailed usage guides in `/docs`).
