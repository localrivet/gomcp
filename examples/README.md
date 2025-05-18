# GoMCP Examples

This directory contains examples for using the GoMCP library.

## Server Example

The server example demonstrates how to create and run an MCP server with stdio transport.

### Features Demonstrated

- Creating a server with `server.NewServer()`
- Configuring file-based logging with `slog.DefaultConfig()` and `slog.NewLoggerWithConfig()`
- Registering tools, resources, and prompts
- Setting root paths for filesystem access
- Using stdio transport with `srv.AsStdio()`

### Running the Server

```bash
# Build the server example
go build -o examples/server/server examples/server/main.go

# Run the server
./examples/server/server
```

The server will:

- Log messages to the `logs` directory (created automatically)
- Use text format for better human readability
- Print a brief message to the console to confirm startup

The server exposes:

- A tool called "echo" that echoes back a message
- A resource at "/hello" that returns a simple greeting
- A prompt called "greeting" that formats a greeting with a name
- Root paths set to the examples and server directories

### Testing with JSON-RPC Messages

Since this is a stdio-based server, you can interact with it by sending JSON-RPC messages to its standard input. Here are some example messages you can paste into the terminal:

#### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": { "protocolVersion": "draft" }
}
```

#### Call the Echo Tool

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": { "name": "echo", "args": { "message": "Hello from JSON-RPC!" } }
}
```

#### Get the Hello Resource

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "resources/read",
  "params": { "uri": "/hello" }
}
```

#### Render the Greeting Prompt

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "prompts/render",
  "params": { "name": "greeting", "args": { "name": "Example User" } }
}
```

#### List Tools

```json
{ "jsonrpc": "2.0", "id": 5, "method": "tools/list" }
```

Copy and paste these messages one at a time into the terminal where the server is running. The server will process each message and return a JSON-RPC response.

### Viewing Logs

The logs will be in the `logs` directory with filenames like:

```
mcp-example-2023-08-10-15-30-45.log
```

You can view the logs with any text editor or using commands like:

```bash
cat logs/mcp-example-*.log
```
