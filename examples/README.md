# GoMCP Examples

This directory contains example MCP servers and clients demonstrating the usage of the `gomcp` library.

## Running the Examples

The examples are designed to communicate over standard input/output (stdio). To run them, you typically need two terminals or use shell piping to connect the server's stdout to the client's stdin and vice-versa.

**General Pattern:**

```bash
# In one terminal (or background process):
go run ./examples/server/*.go

# In another terminal:
go run ./examples/client/main.go
```

**Using Piping (Simpler):**

This command runs both the server and client, connecting them directly:

```bash
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

- **`server/`**: Contains `main.go` which runs an MCP server offering multiple tools:
  - `echo` (defined in `server/main.go`)
  - `calculator` (defined in `server/calculator.go`)
  - `filesystem` (defined in `server/filesystem.go`) - Operates within a `./fs_sandbox` directory created automatically. **Note:** This is a simplified example; real filesystem tools need significant security hardening.
- **`client/`**: Contains `main.go` which runs an MCP client that connects to the server, requests tool definitions, and then calls each available tool with various arguments to demonstrate success and error handling.
