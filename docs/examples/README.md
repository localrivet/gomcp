# GOMCP Examples

This section provides a variety of examples demonstrating common usage patterns for GOMCP.

## Basic Examples

- [Hello World Server](hello-world-server.md) - Minimal server implementation
- [Hello World Client](hello-world-client.md) - Minimal client implementation
- [Calculator Server](calculator-server.md) - Simple tool implementation example
- [Calculator Client](calculator-client.md) - Simple tool calling example

## Transport Examples

All available transports have corresponding examples in the `/examples` directory:

- [HTTP](http-example.md) - Standard request/response over HTTP
- [WebSocket](websocket-example.md) - Bidirectional communication for web applications
- [SSE (Server-Sent Events)](sse-example.md) - Server-to-client streaming
- [Unix Socket](unix-example.md) - High-performance interprocess communication
- [UDP](udp-example.md) - Low-overhead communication with optional reliability
- [MQTT](mqtt-example.md) - Publish/subscribe messaging for IoT and lightweight applications
- [NATS](nats-example.md) - Cloud-native, high-performance messaging
- [Standard I/O](stdio-example.md) - Communication via stdin/stdout for CLI integration
- [gRPC](grpc-example.md) - gRPC transport example (client-side, with conceptual server implementation)

Each transport has specific strengths and is suitable for different use cases:

| Transport    | Bidirectional | Streaming |   Connection   | Best For                                        |
| ------------ | :-----------: | :-------: | :------------: | ----------------------------------------------- |
| HTTP         |      No       |    No     |   Stateless    | Simple integrations, REST-like services         |
| WebSocket    |      Yes      |    Yes    |   Persistent   | Web applications, real-time updates             |
| SSE          |      No       |    Yes    |   Persistent   | Server-to-client updates, monitoring            |
| Unix Socket  |      Yes      |    Yes    |   Persistent   | High-performance local IPC                      |
| UDP          |      Yes      |    No     | Connectionless | High-throughput, latency-sensitive applications |
| MQTT         |      Yes      |    No     |   Persistent   | IoT, telemetry, pub/sub patterns                |
| NATS         |      Yes      |    Yes    |   Persistent   | Microservices, cloud-native applications        |
| Standard I/O |      Yes      |    Yes    |     Direct     | CLI tools, child processes                      |
| gRPC         |      Yes      |    Yes    |   Persistent   | Service-to-service communication                |

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

# Run a specific example (e.g., HTTP example)
go run http/http_example.go

# For examples that require a broker (MQTT, NATS)
# Make sure you have the broker running first:
# docker run -d -p 1883:1883 eclipse-mosquitto  # For MQTT
# docker run -d -p 4222:4222 nats               # For NATS
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
