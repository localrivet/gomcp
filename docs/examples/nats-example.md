# NATS Transport Example

The NATS transport provides a lightweight, high-performance cloud-native messaging system. This transport is ideal for microservices architecture, event-driven applications, and cloud-native deployments requiring efficient, reliable communication.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using NATS transport
- Creating an MCP client that connects to the NATS broker
- Registering and calling a simple echo tool
- Configuring NATS-specific options like subject prefixes and QoS settings

## Prerequisites

This example requires a NATS broker running on localhost port 4222. You can start one using Docker:

```bash
docker run -d -p 4222:4222 nats
```

## Server Configuration

```go
// Create a new server
srv := server.NewServer("nats-example-server")

// Configure the server with NATS transport
srv.AsNATS("nats://localhost:4222",
    nats.WithClientID("mcp-example-server"),
    nats.WithSubjectPrefix("mcp-example"),
)

// Register a simple echo tool
srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
    Message string `json:"message"`
}) (map[string]interface{}, error) {
    fmt.Printf("Server received: %s\n", args.Message)
    return map[string]interface{}{
        "message": args.Message,
    }, nil
})

// Start the server
if err := srv.Run(); err != nil {
    log.Fatalf("Server error: %v", err)
}
```

## Client Configuration

```go
// Create a new client with NATS transport
c, err := client.NewClient("nats-example-client",
    client.WithNATS("nats://localhost:4222",
        client.WithNATSClientID("mcp-example-client"),
        client.WithNATSSubjectPrefix("mcp-example"),
    ),
    client.WithConnectionTimeout(5*time.Second),
    client.WithRequestTimeout(30*time.Second),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer c.Close()

// Call the echo tool
echoResult, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from NATS client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## NATS Configuration Options

The NATS transport supports several configuration options:

### Server Options

- `nats.WithClientID(string)` - Set a unique client ID for the NATS connection
- `nats.WithSubjectPrefix(string)` - Set a custom subject prefix for MCP messages
- `nats.WithCredentials(username, password string)` - Set authentication credentials
- `nats.WithToken(token string)` - Use token-based authentication
- `nats.WithTLS(config *tls.Config)` - Configure TLS/SSL encryption

### Client Options

- `client.WithNATSClientID(string)` - Set a unique client ID for the NATS connection
- `client.WithNATSSubjectPrefix(string)` - Set a custom subject prefix for MCP messages
- `client.WithNATSCredentials(username, password string)` - Set authentication credentials
- `client.WithNATSToken(token string)` - Use token-based authentication
- `client.WithNATSTLS(config *tls.Config)` - Configure TLS/SSL encryption

## Advantages of NATS Transport

- **High Performance**: Designed for high throughput and low latency
- **Cloud Native**: Built for modern distributed systems architecture
- **Flexible Communication Patterns**: Supports request-reply, publish-subscribe, and streaming
- **Clustering and High Availability**: Built-in support for clustering and fault tolerance
- **Simple Protocol**: Lightweight and easy to implement
- **Language Agnostic**: Clients available in many programming languages
- **Built-in Monitoring**: Includes monitoring and management capabilities

## Limitations of NATS Transport

- **Requires a Broker**: Depends on a NATS server/cluster
- **Message Size Limits**: Default 1MB message size limit (configurable)
- **Learning Curve**: May require understanding NATS-specific concepts for advanced usage

## Running the Example

```bash
# Ensure a NATS broker is running
docker run -d -p 4222:4222 nats

# Run the example directly
go run examples/nats/nats_example.go
```

## Complete Example

See the full example in [examples/nats/nats_example.go](https://github.com/localrivet/gomcp/tree/main/examples/nats/nats_example.go) in the GOMCP repository.
