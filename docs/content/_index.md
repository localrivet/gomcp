---
title: GoMCP - Go Model Context Protocol Library
weight: 1
cascade:
  type: docs
---

**GoMCP** is a comprehensive Go library designed to provide idiomatic tools for building applications that communicate using the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/).

## Why GoMCP?

The Model Context Protocol enables powerful interactions between language models (or other agents) and external tools or data sources. GoMCP was built to make implementing both MCP clients (consumers of tools/resources) and MCP servers (providers of tools/resources) straightforward and efficient in Go. It handles the underlying JSON-RPC 2.0 messaging and provides clear interfaces for defining capabilities and managing communication transports.

## Specification

GoMCP aims to be fully compliant with the official **Model Context Protocol specification (Protocol Revision: 2025-03-26)**. It supports core features like initialization, tool definition and execution, resource discovery and access, prompts, progress reporting, and cancellation.

## Getting Started

Dive into the documentation to learn more:

- **[Documentation Home]({{< ref "docs" >}}):** Start here for an overview and detailed guides.
  - **[Getting Started]({{< ref "docs/getting-started" >}}):** Installation and your first server/client.
  - **[Server Implementation]({{< ref "docs/create-server" >}}):** Building applications that provide MCP capabilities.
  - **[Client Implementation]({{< ref "docs/create-client" >}}):** Building applications that consume MCP capabilities.
  - **[Protocols]({{< ref "docs/protocols" >}}):** Detailed breakdown of MCP messages and structures.
- **[Examples]({{< ref "examples" >}}):** Explore runnable examples demonstrating various features and transports.
- **[Go Reference](https://pkg.go.dev/github.com/localrivet/gomcp):** Detailed Go package documentation.

## Repository

- [GitHub Repository](https://github.com/localrivet/gomcp)
