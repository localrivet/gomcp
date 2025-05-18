# GOMCP Examples

This section provides a variety of examples demonstrating common usage patterns for GOMCP.

## Basic Examples

- [Hello World Server](hello-world-server.md) - Minimal server implementation
- [Hello World Client](hello-world-client.md) - Minimal client implementation
- [Calculator Server](calculator-server.md) - Simple tool implementation example
- [Calculator Client](calculator-client.md) - Simple tool calling example

## Transport Examples

- [WebSocket Server](websocket-server.md) - Server using WebSocket transport
- [WebSocket Client](websocket-client.md) - Client using WebSocket transport
- [HTTP Server](http-server.md) - Server using HTTP transport
- [HTTP Client](http-client.md) - Client using HTTP transport
- [SSE Server](sse-server.md) - Server using Server-Sent Events transport
- [SSE Client](sse-client.md) - Client using Server-Sent Events transport

## Advanced Examples

- [Resources Example](resources-example.md) - Working with resources
- [Prompts Example](prompts-example.md) - Working with prompts
- [Sampling Example](sampling-example.md) - Working with sampling
- [Multiple Tools Server](multiple-tools-server.md) - Server with multiple tools
- [Error Handling](error-handling.md) - Proper error handling patterns
- [Timeouts and Retries](timeouts-retries.md) - Working with timeouts and retries

## Real-world Examples

- [LLM Integration](llm-integration.md) - Integrating with LLMs
- [Web Application](web-application.md) - Using GOMCP in a web application
- [CLI Tool](cli-tool.md) - Building a command-line tool with GOMCP

## Running Examples

Most examples in this directory can be run directly:

```bash
# Clone the repository
git clone https://github.com/localrivet/gomcp.git
cd gomcp/examples

# Run a specific example (e.g., calculator server)
go run calculator/server/main.go

# In another terminal, run the client
go run calculator/client/main.go
```

## Example Code Structure

Examples follow a consistent structure:

1. Imports and setup
2. Configuration
3. Main implementation
4. Error handling
5. Cleanup

## Source Code

Full source code for all examples is available in the [examples](https://github.com/localrivet/gomcp/tree/main/examples) directory of the GOMCP repository.
