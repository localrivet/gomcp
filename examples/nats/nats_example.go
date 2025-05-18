// Package main provides an example of using the NATS transport for gomcp
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/nats"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// This example assumes you have a local NATS server running
	// such as NATS Server on the default port
	serverURL := "nats://localhost:4222"

	// Start the server in a goroutine
	startServer(serverURL)

	// Wait a bit for the server to initialize
	time.Sleep(1 * time.Second)

	// Start the client
	go runClient(serverURL)

	// Wait for termination signal
	<-signals
	fmt.Println("\nShutdown signal received, exiting...")
}

func startServer(serverURL string) {
	// Create a new server
	srv := server.NewServer("nats-example-server")

	// Configure the server with NATS transport
	srv.AsNATS(serverURL,
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

	// Start the server in a goroutine
	go func() {
		fmt.Println("Starting NATS server on", serverURL)
		if err := srv.Run(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

func runClient(serverURL string) {
	// Create a new client with NATS transport
	c, err := client.NewClient("nats-example-client",
		client.WithNATS(serverURL,
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

	// Wait a moment to allow printing of results
	time.Sleep(500 * time.Millisecond)
}
