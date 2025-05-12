# GoMCP Demo Client

This client application demonstrates how to connect to and interact with the GoMCP demo server using a fluent interface.

## Features

The demo client showcases the following features:

- **Fluent API Design** - Method chaining for elegant, readable code
- Connecting to the demo server using either SSE or WebSocket transport
- Listing available tools, resources, and prompts
- Reading various resource types (static, dynamic, and parameterized)
- Calling tools with different argument types
- Handling progress updates from long-running operations
- Subscribing to notifications and events

## Usage

### Prerequisites

Make sure the demo server is running. You can start it with:

```bash
cd cmd/demo/server
go run main.go
```

### Running the Client

To run the client with the default SSE transport:

```bash
cd cmd/demo/client
go run main.go
```

To run the client with WebSocket transport:

```bash
cd cmd/demo/client
go run main.go websocket  # or "ws" for short
```

## Fluent Interface Design

The client uses a fluent interface with method chaining to create concise, readable code:

```go
demoClient.
    WithSSETransport().
    WithEventHandlers().
    Connect().
    ListTools().
    ListResources().
    ListPrompts().
    ReadResource("resource://greeting", "greeting resource").
    CallTool("add", args, "add tool")
```

This approach mirrors the server's fluent design, making both components consistent in style.

## Demonstrated Functionality

### Transport Types

The client supports connecting to the server using:

- Server-Sent Events (SSE) - Default and most widely supported
- WebSocket - For bidirectional communication

### Resource Access

The client demonstrates reading different types of resources:

- Static resources: `resource://greeting`, `data://config`
- Template resources with parameters: `weather://London/current`
- Repository information: `repos://localrivet/gomcp/info`
- Documentation with wildcard paths: `docs://api/reference`

### Tool Calls

The client calls various tools:

- `add` - Simple tool that adds two numbers
- `process_items` - Tool that processes items and reports progress
- `analyze_data` - Tool with complex input structure

### Event Handling

The client sets up handlers for various events:

- Connection status changes
- Progress updates
- Server log messages

## Code Structure

- **DemoClient Type**: A wrapper around the standard client that provides the fluent interface
- **Transport Methods**: WithSSETransport() and WithWebSocketTransport() for transport selection
- **Action Methods**: Each method performs an operation and returns the client for chaining
- **Helper Functions**: Utilities for displaying resource contents and tool results

## Extending the Client

You can extend this client by:

1. Adding more fluent methods for additional functionality
2. Creating specialized resource or tool helper methods
3. Enhancing the error handling
4. Adding custom middleware support
5. Implementing resource subscriptions
