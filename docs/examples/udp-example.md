# UDP Transport Example

The UDP (User Datagram Protocol) transport provides low-overhead, connectionless communication with optional reliability features. This transport is ideal for high-throughput scenarios where occasional packet loss is acceptable or where minimal latency is critical.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using UDP transport
- Creating an MCP client that connects to the UDP server
- Registering and calling a simple echo tool
- Configuring UDP-specific options like packet size and reliability

## Server Configuration

```go
// Create a new server
srv := server.NewServer("udp-example-server")

// Configure the server with UDP transport and custom options
srv.AsUDP(":8082",
    udp.WithMaxPacketSize(4096),
    udp.WithReliability(true),
    udp.WithRetryCount(3),
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
// Create a new client with the UDP server address
c, err := client.NewClient("udp-client",
    client.WithUDP("localhost:8082",
        client.WithUDPMaxPacketSize(4096),
        client.WithUDPReliability(true),
        client.WithUDPRetryCount(3),
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
    "message": "Hello from UDP client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## UDP Configuration Options

The UDP transport supports several configuration options:

### Server Options

- `udp.WithMaxPacketSize(size int)` - Set the maximum packet size for UDP datagrams
- `udp.WithReliability(enabled bool)` - Enable optional reliability features (sequence numbers, acknowledgments)
- `udp.WithRetryCount(count int)` - Set the number of retries for message delivery
- `udp.WithBufferSize(size int)` - Configure socket buffer size

### Client Options

- `client.WithUDPMaxPacketSize(size int)` - Set the maximum packet size for UDP datagrams
- `client.WithUDPReliability(enabled bool)` - Enable optional reliability features
- `client.WithUDPRetryCount(count int)` - Set the number of retries for message delivery
- `client.WithUDPTimeout(duration time.Duration)` - Set timeout for acknowledgment reception

## Advantages of UDP Transport

- **Low Overhead**: Minimal protocol overhead, ideal for high-throughput applications
- **Low Latency**: No connection establishment or termination
- **Multicast/Broadcast**: Support for one-to-many communication patterns
- **Resilient**: No connection state to maintain, simplifies failure recovery
- **Configurable Reliability**: Optional reliability layer for applications that need it

## Limitations of UDP Transport

- **Potential Packet Loss**: By default, no guarantee of delivery
- **No Ordering Guarantee**: Messages may arrive out of order
- **Size Limitations**: Limited by MTU sizes on network paths
- **Firewalls and NAT**: Some environments restrict UDP traffic

## Running the Example

```bash
# First terminal: Run the server
go run examples/udp/udp_example.go server

# Second terminal: Run the client
go run examples/udp/udp_example.go client
```

## Complete Example

See the full example in [examples/udp/udp_example.go](https://github.com/localrivet/gomcp/tree/main/examples/udp/udp_example.go) in the GOMCP repository.
