# GoMCP Examples

This directory contains example MCP servers and clients demonstrating the usage of the `gomcp` library.

## Running the Examples

The examples are designed to communicate over standard input/output (stdio). To run them, you typically need two terminals or use shell piping to connect the server's stdout to the client's stdin and vice-versa.

### Multi-Tool Example (`server/` and `client/`)

This is the main example demonstrating multiple tools.

**Using Piping:**

```bash
# This command runs the multi-tool server and the standard client
go run ./examples/server/*.go | go run ./examples/client/main.go
```

**Expected Output:**

When run successfully, you will see log messages printed to **stderr** from both the server and the client, detailing the steps:

1.  Server starts.
2.  Client starts.
3.  Handshake occurs.
4.  Client requests tool definitions.
5.  Server sends tool definitions (echo, calculator, filesystem).
6.  Client prints received definitions.
7.  Client uses the `echo` tool.
8.  Server executes `echo`, sends result.
9.  Client prints `echo` result.
10. Client uses the `calculator` tool (add, divide by zero, missing arg).
11. Server executes `calculator`, sends results/errors.
12. Client prints `calculator` results/errors.
13. Client uses the `filesystem` tool (list, write, read, read non-existent, write outside sandbox).
14. Server executes `filesystem`, sends results/errors.
15. Client prints `filesystem` results/errors.
16. Client finishes.
17. Server detects client disconnection (EOF) and finishes.

## Examples Included

- **`server/` & `client/`**: The primary example demonstrating a server offering multiple tools (`echo`, `calculator`, `filesystem`) and a client that uses them all. The `filesystem` tool operates within a `./fs_sandbox` directory.
- **`auth-server/` & `auth-client/`**: Demonstrates a simple API key authentication mechanism (via environment variable `MCP_API_KEY=test-key-123`) required for the server to start.
- **`rate-limit-server/` & `rate-limit-client/`**: Builds on the auth example, adding simple global rate limiting (2 requests/sec, burst 4) to the server's tool. The client sends requests rapidly to demonstrate hitting the limit.

### Auth Example (`auth-server/` and `auth-client/`)

**Using Piping:**

```bash
# Set the required API key and run the auth server and client
export MCP_API_KEY="test-key-123"
go run ./examples/auth-server/main.go | go run ./examples/auth-client/main.go

# Example of running with the wrong key (server will fail to start)
export MCP_API_KEY="wrong-key"
go run ./examples/auth-server/main.go | go run ./examples/auth-client/main.go
```

### Rate Limit Example (`rate-limit-server/` and `rate-limit-client/`)

**Using Piping:**

```bash
# Set the required API key and run the rate-limited server and client
export MCP_API_KEY="test-key-123"
go run ./examples/rate-limit-server/main.go | go run ./examples/rate-limit-client/main.go
```
