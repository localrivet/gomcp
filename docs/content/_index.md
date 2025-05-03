---
title: GoMCP - Go Model Context Protocol Library
weight: 1
next: /get-started/
cascade:
  type: docs
---

## What is MCP?

The [Model Context Protocol (MCP)](https://modelcontextprotocol.io) lets you build servers that expose data and functionality to LLM applications in a secure, standardized way. Think of it like a web API, but specifically designed for LLM interactions. MCP servers can:

- Expose data through **Resources** (think GET endpoints; load info into context)
- Provide functionality through **Tools** (think POST/PUT endpoints; execute actions)
- Define interaction patterns through **Prompts** (reusable templates)
- And more!

GoMCP provides a robust, idiomatic Go interface for building and interacting with these servers.

## Why GoMCP?

The MCP protocol is powerful but implementing it involves details like server setup, protocol handlers, content types, and error management. GoMCP handles these protocol details, letting you focus on building your application's capabilities.

GoMCP aims to be:

- **Robust:** Implements the MCP specification thoroughly.
- **Performant:** Leverages Go's concurrency and efficiency.
- **Idiomatic:** Feels natural to Go developers.
- **Complete:** Provides a full implementation of the core MCP specification for both servers and clients.
- **Flexible:** Supports multiple transport layers and offers hooks for customization.

## Key Features

- **Full MCP Compliance:** Supports all core features of the `2025-03-26` and `2024-11-05` specifications.
- **Protocol Version Negotiation:** Automatically negotiates the highest compatible protocol version during the handshake.
- **Transport Agnostic Core:** Server (`server/`) and Client (`client/`) logic is decoupled from the underlying transport.
- **Multiple Transports:** Includes implementations for common communication methods:
  - **Streamable HTTP (SSE + POST):** The primary network transport (`transport/sse/`).
  - **WebSocket:** For persistent, bidirectional network communication (`transport/websocket/`).
  - **Standard Input/Output (Stdio):** Ideal for local inter-process communication (`transport/stdio/`).
  - **TCP:** For raw TCP socket communication (`transport/tcp/`).
- **Helper Utilities:** Provides helpers for argument parsing (`util/schema`), progress reporting (`util/progress`), response creation (`server.Text`, `server.JSON`), etc.
- **Extensible Hooks:** Offers a hook system (`hooks/`) for intercepting and modifying behavior.
- **`2025-03-26` Enhancements:** Includes support for the Authorization Framework, JSON-RPC Batching, Tool Annotations, Audio Content, Completions, and more.
