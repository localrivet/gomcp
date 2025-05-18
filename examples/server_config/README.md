# MCP Server Configuration Example

This example demonstrates how to configure and manage MCP servers using the configuration file approach.

## Server Configuration Format

The server configuration is stored in a JSON file with the following structure:

```json
{
  "mcpServers": {
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"]
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "env": {
        "DEBUG": "true"
      }
    }
  }
}
```

Each server entry contains:

- `command`: The executable to run the server
- `args`: Command line arguments to pass to the server
- `env` (optional): Environment variables to set for the server process

## Using the Server Registry

The library provides a `ServerRegistry` to manage multiple MCP servers:

```go
import "github.com/localrivet/gomcp/client"

// Create a new server registry
registry := client.NewServerRegistry()

// Load server configurations from a file
err := registry.LoadConfig("path/to/mcpservers.json")

// Get all configured server names
serverNames, _ := registry.GetServerNames()

// Get a client for a specific server
client, _ := registry.GetClient("memory")

// Call methods on the client
result, _ := client.CallTool("echo", map[string]interface{}{
    "message": "Hello from GOMCP!",
})

// Close client and stop server when done
client.Close()

// Or stop all servers at once
registry.StopAll()
```

## Built-in Server Types

This example includes configurations for two types of servers:

1. **Memory Server** (`@modelcontextprotocol/server-memory`): A simple in-memory MCP server
2. **Filesystem Server** (`@modelcontextprotocol/server-filesystem`): An MCP server that serves files from a directory

You can extend this to support any MCP-compatible server including custom implementations.

## Running the Example

To run this example:

```bash
go run examples/server_config/main.go
```

The example will:

1. Create a sample configuration file if one doesn't exist
2. Start the configured servers
3. Connect clients to the servers
4. Execute some sample operations
5. Clean up all resources on exit (Ctrl+C)

## Key Components

The main components demonstrated in this example:

- `client.NewServerRegistry()`: Creates a registry for managing MCP servers
- `registry.LoadConfig(path)`: Loads a configuration file and starts the defined servers
- `registry.GetClient(name)`: Gets a client for a specific server
- `registry.StopAll()`: Gracefully shuts down all servers

## Integration with Other Applications

This functionality can be integrated into applications that need to manage multiple MCP servers:

```go
// Create registry
registry := client.NewServerRegistry()

// Load server config file
registry.LoadConfig("/path/to/mcpservers.json")

// Get client for a specific server
fsClient, _ := registry.GetClient("filesystem")
result, _ := fsClient.CallTool("listFiles", map[string]interface{}{"path": "/"})

// When finished
registry.StopAll()
```

## Notes

- The registry starts each server as a child process and pipes communication through stdin/stdout.
- Environment variables from the parent process are inherited, with additional ones from the config added.
- All servers are automatically shut down when the registry is stopped or the program exits.
