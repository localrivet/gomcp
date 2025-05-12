---
title: 'Examples'
weight: 60
---

The [github.com/localrivet/gomcp/examples] directory contains various client/server pairs demonstrating specific features, transports, and integrations:

- **Basic Usage:**
  - `basic/`: Simple stdio communication with multiple tools.
  - `hello-demo/`: Minimal example showcasing tool, prompt, and resource registration (stdio).
  - `client_helpers_demo/`: Demonstrates client-side helpers for configuration loading and tool calls.
- **Network Transports:**
  - `http/`: Integration with various Go HTTP frameworks using the **Streamable HTTP (SSE)** transport (includes Gin, Echo, Chi, Fiber, etc.).
  - `websocket/`: Demonstrates the **WebSocket** transport.
- **Configuration & Deployment:**
  - `configuration/`: Loading server configuration from files (JSON, YAML, TOML).
  - `cmd/`: Generic command-line client and server implementations configurable for different transports.
- **Advanced Features:**
  - `auth/`: Simple API key authentication hook example (stdio).
  - `rate-limit/`: Example of rate limiting client requests (stdio).
  - `kitchen-sink/`: Comprehensive server example combining multiple features (stdio).
  - `code-assistant/`: Example server providing code review/documentation tools.
  - `meta-tool-demo/`: Server demonstrating a tool that calls other tools.

**Running Examples:**

1.  Navigate to an example's server directory (e.g., `cd examples/websocket/server`).
2.  Run the server: `go run .`
3.  In another terminal, navigate to the corresponding client directory (e.g., `cd examples/websocket/client`).
4.  Run the client: `go run .`

_(Check the specific README within each example directory for more detailed instructions if available.)_
