# MQTT Transport Example

The MQTT (Message Queuing Telemetry Transport) transport provides a lightweight publish/subscribe messaging pattern. This transport is ideal for IoT applications, telemetry data collection, and applications with constrained resources or networks.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using MQTT transport
- Creating an MCP client that connects to the MQTT broker
- Registering and calling a simple echo tool
- Configuring MQTT-specific options like QoS and topic prefixes

## Prerequisites

This example requires an MQTT broker (such as Mosquitto) running on localhost port 1883. You can start one using Docker:

```bash
docker run -d -p 1883:1883 eclipse-mosquitto
```

## Server Configuration

```go
// Create a new server
srv := server.NewServer("mqtt-example-server")

// Configure the server with MQTT transport
srv.AsMQTT(brokerURL,
    mqtt.WithClientID("mcp-example-server"),
    mqtt.WithQoS(1),
    mqtt.WithTopicPrefix("mcp-example"),
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
// Create a new client with MQTT transport
c, err := client.NewClient("mqtt-example-client",
    client.WithMQTT(brokerURL,
        client.WithMQTTClientID("mcp-example-client"),
        client.WithMQTTQoS(1),
        client.WithMQTTTopicPrefix("mcp-example"),
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
    "message": "Hello from MQTT client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## MQTT Configuration Options

The MQTT transport supports several configuration options:

### Server Options

- `mqtt.WithClientID(string)` - Set a unique client ID for the MQTT connection
- `mqtt.WithQoS(byte)` - Set the Quality of Service level (0, 1, or 2)
- `mqtt.WithTopicPrefix(string)` - Set a custom topic prefix for MCP messages
- `mqtt.WithCredentials(username, password string)` - Set authentication credentials
- `mqtt.WithTLS(config *TLSConfig)` - Configure TLS/SSL encryption

### Client Options

- `client.WithMQTTClientID(string)` - Set a unique client ID for the MQTT connection
- `client.WithMQTTQoS(byte)` - Set the Quality of Service level (0, 1, or 2)
- `client.WithMQTTTopicPrefix(string)` - Set a custom topic prefix for MCP messages
- `client.WithMQTTCredentials(username, password string)` - Set authentication credentials
- `client.WithMQTTTLS(config *mqtt.TLSConfig)` - Configure TLS/SSL encryption

## Advantages of MQTT Transport

- **Lightweight protocol**: Low bandwidth overhead
- **Publish/subscribe pattern**: Efficient message distribution
- **QoS levels**: Reliable message delivery options
- **Designed for constrained networks**: Works well in environments with limited connectivity
- **Widely supported**: Many client libraries and broker implementations available

## Limitations of MQTT Transport

- **Requires a broker**: Depends on an external MQTT broker
- **Limited payload size**: Some brokers restrict message size
- **Not inherently bidirectional**: Requires careful topic design for request-response patterns

## Running the Example

```bash
# Ensure an MQTT broker is running
docker run -d -p 1883:1883 eclipse-mosquitto

# Run the example directly
go run examples/mqtt/mqtt_example.go
```

## Complete Example

See the full example in [examples/mqtt/mqtt_example.go](https://github.com/localrivet/gomcp/tree/main/examples/mqtt/mqtt_example.go) in the GOMCP repository.
