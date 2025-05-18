// Package main provides an example of using UDP transport for gomcp
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
	"github.com/localrivet/gomcp/transport/udp"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Define the UDP host and port
	address := "localhost:8083"

	// Start the server in a goroutine
	startServer(address)

	// Wait a bit for the server to initialize
	time.Sleep(1 * time.Second)

	// Start the client
	go runClient(address)

	// Wait for termination signal
	<-signals
	fmt.Println("\nShutdown signal received, exiting...")
}

func startServer(address string) {
	// Create a new server
	srv := server.NewServer("udp-example-server")

	// Configure the server with UDP transport
	// UDP is an unreliable protocol by default, so we enable reliability for this example
	srv.AsUDP(address,
		udp.WithReliability(true),           // Enable reliability
		udp.WithMaxPacketSize(4096),         // 4KB packet size
		udp.WithFragmentTTL(10*time.Second), // Fragment expiry time
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
		fmt.Println("Starting UDP server on", address)
		if err := srv.Run(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

func runClient(address string) {
	// Create a new client with UDP transport
	c, err := client.NewClient("udp-example-client",
		client.WithUDP(address,
			client.WithReliability(true),           // Enable reliability
			client.WithMaxPacketSize(4096),         // 4KB packet size
			client.WithFragmentTTL(10*time.Second), // Fragment expiry time
		),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call the echo tool - connection happens automatically
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from UDP client!",
	})
	if err != nil {
		log.Fatalf("Echo call failed: %v", err)
	}
	fmt.Printf("Echo result: %v\n", echoResult)

	// Wait a moment to allow printing of results
	time.Sleep(500 * time.Millisecond)
}
