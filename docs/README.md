# GOMCP Documentation

GOMCP is a Go implementation of the Model Context Protocol (MCP), designed to enable seamless integration between Go applications and Large Language Models (LLMs).

## Documentation Sections

- [Getting Started](getting-started/README.md) - Quick guide to start using GOMCP
- [Installation](installation/README.md) - How to install and set up GOMCP
- [Tutorials](tutorials/README.md) - Step-by-step guides for client and server usage
- [API Reference](api-reference/README.md) - Detailed API documentation for all packages
- [Examples](examples/README.md) - Example code and usage patterns
- [Troubleshooting](troubleshooting/README.md) - Common issues and solutions
- [Version Compatibility](version-compatibility/README.md) - Information about protocol versions
- [MCP Specification Reference](spec-reference/README.md) - MCP protocol specification details

## Core Features

- Full MCP protocol implementation
- Client and server components
- Multiple transport options:
  - HTTP - Standard request/response over HTTP
  - WebSocket - Bidirectional communication for web applications
  - Server-Sent Events (SSE) - Server-to-client streaming
  - Unix Socket - High-performance interprocess communication
  - UDP - Low-overhead communication with optional reliability
  - MQTT - Publish/subscribe messaging for IoT and lightweight applications
  - NATS - Cloud-native, high-performance messaging
  - Standard I/O - Communication via stdin/stdout for CLI integration
  - gRPC - (Client-side support, with server integration in development)
- Automatic protocol version negotiation
- Comprehensive type safety
- Support for all MCP operations: tools, resources, prompts, and sampling
- Flexible configuration options

## License

This project is licensed under the [LICENSE](../LICENSE) file in the root directory of this source tree.
