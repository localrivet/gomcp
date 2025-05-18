// Package gomcp provides a Go implementation of the Model Context Protocol (MCP).
//
// # Overview
//
// The Model Context Protocol (MCP) is a standardized communication protocol designed to
// facilitate interaction between applications and Large Language Models (LLMs). This library
// provides a complete Go implementation of the protocol with support for all specification
// versions (2024-11-05, 2025-03-26, and draft) with automatic version detection and negotiation.
//
// # Core Features
//
// - Full MCP protocol implementation
// - Client and server components
// - Multiple transport options (stdio, HTTP, WebSocket, Server-Sent Events)
// - Automatic protocol version negotiation
// - Comprehensive type safety
// - Support for all MCP operations: tools, resources, prompts, and sampling
// - Flexible configuration options
// - Process management for external MCP servers
// - Server configuration file support
//
// # Organization
//
// The library is organized into the following main packages:
//
//   - github.com/localrivet/gomcp/client: Client implementation for consuming MCP services
//   - github.com/localrivet/gomcp/server: Server implementation for hosting MCP services
//   - github.com/localrivet/gomcp/transport: Transport layer implementations
//   - github.com/localrivet/gomcp/mcp: Core protocol definitions and version handling
//
// # Basic Usage
//
// ## Client Example
//
//	import "github.com/localrivet/gomcp/client"
//
//	// Create a new client
//	c, err := client.NewClient("my-client",
//	  client.WithProtocolVersion("2025-03-26"),
//	  client.WithProtocolNegotiation(true),
//	)
//	if err != nil {
//	  log.Fatalf("Failed to create client: %v", err)
//	}
//
//	// Call a tool on the MCP server
//	result, err := c.CallTool("say_hello", map[string]interface{}{
//	  "name": "World",
//	})
//
// ## Server Configuration Example
//
//	// Connect to an external MCP server defined in a config file
//	c, err := client.NewClient("",
//	  client.WithServerConfig("mcpservers.json", "task-master-ai"),
//	)
//	if err != nil {
//	  log.Fatalf("Failed to create client: %v", err)
//	}
//	defer c.Close() // Will also stop the server process
//
//	// The mcpservers.json file defines how to start and connect to the server:
//	// {
//	//   "mcpServers": {
//	//     "task-master-ai": {
//	//       "command": "npx",
//	//       "args": ["-y", "--package=task-master-ai", "task-master-ai"],
//	//       "env": { "ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}" }
//	//     }
//	//   }
//	// }
//
// ## Server Example
//
//	import "github.com/localrivet/gomcp/server"
//
//	// Create a new server
//	s := server.NewServer("example-server",
//	  server.WithLogger(logger),
//	).AsStdio()
//
//	// Register a tool
//	s.Tool("say_hello", "Greet someone", func(ctx *server.Context, args struct {
//	  Name string `json:"name"`
//	}) (string, error) {
//	  return fmt.Sprintf("Hello, %s!", args.Name), nil
//	})
//
//	// Start the server
//	s.Run()
//
// # Process Management
//
// GOMCP includes robust functionality for managing external MCP server processes:
//
//   - Server configuration loading from JSON files
//   - Automatic process management (start/stop) for external servers
//   - Environment variable handling and substitution
//   - Support for different types of servers (NPX-based, Docker, etc.)
//   - Clean process termination on client close
//
// For more information, see the docs/examples/server-config.md documentation.
//
// # Specification Compliance
//
// This library implements the Model Context Protocol as defined at:
// https://github.com/microsoft/modelcontextprotocol
//
// For detailed documentation, examples, and specifications, see:
// https://modelcontextprotocol.github.io/
//
// For more examples, see the examples directory in this repository.
//
// # Versioning
//
// gomcp follows semantic versioning. The current version is available through the Version constant.
package gomcp

// Version is the current version of the gomcp library
const Version = "0.1.0"
