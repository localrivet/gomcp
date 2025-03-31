---
title: Basic Usage
weight: 40
layout: docs
---

# Basic Usage

The core logic for building MCP servers and clients resides in the `server`, `client`, and `protocol` packages within the `gomcp` library.

For concrete examples demonstrating how to:

- Implement MCP servers using various transports (Stdio, SSE, WebSocket).
- Integrate servers with different Go HTTP frameworks (Gin, Echo, Fiber, Chi, etc.).
- Implement MCP clients.
- Load server configuration from files (JSON, YAML, TOML).
- Handle features like authentication or rate limiting.

Please refer to the directories within the **[`examples/`](../examples/)** folder.

Each example directory is structured as a self-contained Go module where applicable. Detailed instructions for running specific examples can be found in the **[`examples/README.md`](../examples/README.md)** file.
