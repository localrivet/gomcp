# Getting Started with GOMCP

This guide will help you get started with GOMCP, a Go implementation of the Model Context Protocol (MCP).

## Prerequisites

- Go 1.20 or later
- Basic familiarity with Go programming

## Quick Start

### Installing GOMCP

```bash
go get github.com/localrivet/gomcp
```

### Basic Server Example

Create a file named `server.go`:

```go
package main

import (
	"log"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a new server
	s := server.NewServer("example-server").AsStdio()

	// Register a tool
	s.Tool("add", "Add two numbers", func(ctx *server.Context, args struct {
		X float64 `json:"x" required:"true"`
		Y float64 `json:"y" required:"true"`
	}) (float64, error) {
		return args.X + args.Y, nil
	})

	// Start the server
	if err := s.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

### Basic Client Example

Create a file named `client.go`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a new client
	c, err := client.NewClient("stdio:///")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call the "add" tool
	result, err := c.CallTool("add", map[string]interface{}{
		"x": 5,
		"y": 3,
	})
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	fmt.Printf("Result: %v\n", result)
}
```

## Next Steps

- See the [Tutorials](../tutorials/README.md) for more detailed guides
- Check the [Examples](../examples/README.md) for common usage patterns
- Read the [API Reference](../api-reference/README.md) for detailed documentation
