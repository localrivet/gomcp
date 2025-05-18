# Troubleshooting Guide

This guide helps you diagnose and solve common issues with GOMCP.

## Common Issues

### Client Connection Issues

#### Client cannot connect to server

**Symptoms:**

- `Failed to connect to MCP server` error message
- Connection timeouts
- `context deadline exceeded` errors

**Possible Causes:**

- Server is not running
- Incorrect URL or path
- Network connectivity issues
- Transport incompatibility
- Protocol version mismatch

**Solutions:**

1. Verify server is running
2. Check URL format (e.g., `ws://localhost:8080/mcp` for WebSocket)
3. Check network connectivity
4. Ensure client and server use compatible transport types
5. Try specifying a protocol version: `client.WithProtocolVersion("2024-11-05")`

### Tool Execution Issues

#### Tool not found

**Symptoms:**

- `tool not found: <toolName>` error

**Possible Causes:**

- Tool name misspelled or not registered on server
- Case sensitivity mismatch

**Solutions:**

1. Verify tool name spelling and case
2. Check server logs for registered tools
3. Use `tools/list` to get all available tools

#### Invalid parameters

**Symptoms:**

- `invalid params` error
- `required parameter missing` error

**Possible Causes:**

- Missing required parameters
- Parameter type mismatch
- Parameter case sensitivity

**Solutions:**

1. Check parameter names and types
2. Ensure all required parameters are provided
3. Verify parameter case matches tool definition

### Resource Issues

#### Resource not found

**Symptoms:**

- `resource not found` error
- 404-like errors

**Possible Causes:**

- Incorrect resource path
- Resource not registered
- Root path not configured

**Solutions:**

1. Verify resource path format
2. Check if resource is registered on server
3. Ensure server has configured root paths

### Transport Issues

#### WebSocket connection issues

**Symptoms:**

- `failed to establish WebSocket connection` error
- `unexpected EOF` errors

**Possible Causes:**

- Incorrect WebSocket URL
- Server not configured for WebSocket
- Proxy or firewall issues

**Solutions:**

1. Verify WebSocket URL (should start with `ws://` or `wss://`)
2. Ensure server is configured with `AsWebsocket()`
3. Check proxy or firewall settings

## Advanced Troubleshooting

### Enabling Debug Logging

For more detailed logging, configure the logger with a debug level:

```go
// Client-side debugging
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
client, err := client.NewClient("ws://localhost:8080/mcp",
    client.WithLogger(logger),
)

// Server-side debugging
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
server := server.NewServer("my-server",
    server.WithLogger(logger),
)
```

### Protocol Version Issues

If you're experiencing protocol compatibility issues:

1. Explicitly set the client protocol version:

   ```go
   client, err := client.NewClient("ws://localhost:8080/mcp",
       client.WithProtocolVersion("2024-11-05"),
   )
   ```

2. Check server logs for protocol negotiation messages

3. Try disabling protocol negotiation if you know the server version:
   ```go
   client, err := client.NewClient("ws://localhost:8080/mcp",
       client.WithProtocolVersion("2024-11-05"),
       client.WithProtocolNegotiation(false),
   )
   ```

## Getting Help

If you're still experiencing issues:

1. Search the [GitHub issues](https://github.com/localrivet/gomcp/issues) for similar problems
2. Check the [API reference](../api-reference/README.md) for correct usage
3. File a new issue with detailed information:
   - Go version
   - GOMCP version
   - Error messages
   - Minimal code example reproducing the issue
