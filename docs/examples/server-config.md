# Server Configuration

GOMCP provides a convenient way to manage and connect to external MCP servers. This feature is particularly useful for applications that need to:

1. Launch and manage external MCP servers (like Node.js, Python, or other Go services)
2. Handle server process lifecycle (starting, stopping, etc.)
3. Maintain connections to multiple MCP servers
4. Configure servers with specific environment variables and arguments

## Server Configuration Format

Server configurations use a standard JSON format:

```json
{
  "mcpServers": {
    "calculator": {
      "command": "node",
      "args": ["./calculator-server.js"],
      "env": {
        "NODE_ENV": "development"
      }
    },
    "task-master-ai": {
      "command": "npx",
      "args": ["-y", "--package=task-master-ai", "task-master-ai"],
      "env": {
        "ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}"
      }
    }
  }
}
```

Each server entry includes:

- **command**: The executable to run (e.g., `node`, `python`, `docker`)
- **args**: Command-line arguments for the executable
- **env**: Environment variables to set for the process
- **url** (optional): Explicit URL for connecting to the server if not using stdio

## Environment Variable Handling

The server configuration system supports environment variable substitution in the `env` section:

```json
"env": {
  "ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
  "DEBUG": "true"
}
```

When the `${VARIABLE_NAME}` syntax is used:

1. The system will look for the variable in the current process environment
2. If found, the value from the environment will be used
3. If not found, the literal string `${VARIABLE_NAME}` will be passed

This makes it easy to securely pass sensitive API keys and credentials without hardcoding them in your configuration files.

## Using Server Configurations

GOMCP provides two primary ways to use server configurations:

### 1. Loading from a Configuration File (WithServerConfig)

You can load server configurations from a JSON file:

```go
// Create a client connected to a server defined in a config file
client, err := client.NewClient("",
    client.WithServerConfig("config.json", "task-master-ai"),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer client.Close() // This will also stop the server process
```

This approach:

- Loads the configuration from the specified file
- Starts the named server process
- Creates and connects a client to the server
- Automatically stops the server when `client.Close()` is called

### 2. Programmatic Configuration (WithServers)

You can define server configurations directly in your code:

```go
// Define server configuration
config := client.ServerConfig{
    MCPServers: map[string]client.ServerDefinition{
        "calculator": {
            Command: "node",
            Args: []string{"./calculator-server.js"},
            Env: map[string]string{"NODE_ENV": "development"},
        },
    },
}

// Create a client using the configuration
client, err := client.NewClient("",
    client.WithServers(config, "calculator"),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer client.Close() // This will also stop the server process
```

This approach:

- Uses an in-memory configuration
- Is useful for dynamic configurations or testing
- Otherwise behaves identically to the file-based approach

## Advanced Usage: Server Registry

For more control, you can use the `ServerRegistry` directly:

```go
// Create a registry
registry := client.NewServerRegistry()

// Load configuration
if err := registry.LoadConfig("config.json"); err != nil {
    log.Fatalf("Failed to load configuration: %v", err)
}

// Get client for a specific server
calculator, err := registry.GetClient("calculator")
if err != nil {
    log.Fatalf("Failed to get calculator client: %v", err)
}

// Use the client
result, err := calculator.CallTool("add", map[string]interface{}{
    "x": 5,
    "y": 10,
})

// Stop a specific server
if err := registry.StopServer("calculator"); err != nil {
    log.Printf("Warning: Failed to stop calculator server: %v", err)
}

// Stop all servers
if err := registry.StopAll(); err != nil {
    log.Printf("Warning: Failed to stop all servers: %v", err)
}
```

This approach:

- Gives you direct control over the server lifecycle
- Allows you to manage multiple servers independently
- Is useful for more complex applications that need to start and stop servers dynamically

## Common Scenarios

### Connecting to Task Master

```go
client, err := client.NewClient("",
    client.WithServerConfig("config.json", "task-master-ai"),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}

// Call Task Master tools
result, err := client.CallTool("add_task", map[string]interface{}{
    "prompt": "Implement user authentication using JWT",
})
```

The corresponding configuration might look like:

```json
{
  "mcpServers": {
    "task-master-ai": {
      "command": "npx",
      "args": ["-y", "--package=task-master-ai", "task-master-ai"],
      "env": {
        "ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}",
        "PROJECT_ROOT": "${PWD}"
      }
    }
  }
}
```

### Connecting to Docker-based Servers

```go
config := client.ServerConfig{
    MCPServers: map[string]client.ServerDefinition{
        "customer-service": {
            Command: "docker",
            Args: []string{
                "run",
                "-i",
                "--rm",
                "-e", "DATABASE_URL=postgresql://postgres:postgres@host.docker.internal:5432/customers",
                "-e", "STDIO=true",
                "customer-service:latest",
            },
        },
    },
}

client, err := client.NewClient("",
    client.WithServers(config, "customer-service"),
)
```

### Managing Multiple Servers

```go
registry := client.NewServerRegistry()
if err := registry.LoadConfig("servers.json"); err != nil {
    log.Fatalf("Failed to load server config: %v", err)
}

// Get all available servers
serverNames, _ := registry.GetServerNames()
for _, name := range serverNames {
    fmt.Printf("Available server: %s\n", name)
}

// Use specific servers as needed
taskmaster, _ := registry.GetClient("task-master-ai")
context7, _ := registry.GetClient("context7")

// When done, stop all servers
registry.StopAll()
```

## Integration with Larger Applications

Server management can be integrated into larger applications that need to manage multiple MCP servers with different capabilities:

```go
type AppConfig struct {
    // Application configuration
    Port int
    LogLevel string

    // MCP server configuration
    MCPServers client.ServerConfig
}

func NewApplication(config AppConfig) *Application {
    app := &Application{
        port: config.Port,
        logLevel: config.LogLevel,
    }

    // Start MCP servers based on configuration
    registry := client.NewServerRegistry()
    if err := registry.ApplyConfig(config.MCPServers); err != nil {
        log.Fatalf("Failed to start MCP servers: %v", err)
    }
    app.serverRegistry = registry

    // Get specific clients needed by the application
    aiClient, _ := registry.GetClient("ai-service")
    dbClient, _ := registry.GetClient("database-service")

    // Initialize application services with these clients
    app.aiService = NewAIService(aiClient)
    app.dbService = NewDatabaseService(dbClient)

    return app
}

func (a *Application) Shutdown() {
    // Shutdown MCP servers cleanly
    a.serverRegistry.StopAll()
}
```

## Best Practices

1. **Always use defer client.Close()** to ensure proper cleanup of server processes
2. **Provide explicit environment variables** in the configuration
3. **Use environment variable substitution** (`${VAR_NAME}`) for sensitive values
4. **Use a logger** with appropriate configuration to help debug issues
5. **Handle errors** when starting servers and creating clients
6. **Avoid hardcoding API keys** in your source code or configuration files
7. **Consider process isolation** - each server runs in its own process for stability
8. **Test server configurations** before deploying to production
9. **Monitor server processes** for unexpected termination
10. **Use timeouts** for operations to avoid hanging indefinitely

## Security Considerations

1. **Environment variables** should be used for sensitive information
2. **Process isolation** ensures one server can't affect others if it crashes
3. **Input validation** should be performed on all data passed to servers
4. **Timeouts** should be set for all operations to prevent hanging
5. **Error handling** should be robust to prevent information leakage
6. **Logging** should not contain sensitive information
7. **Child processes** inherit file descriptors, so be careful with file permissions
