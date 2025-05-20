// Package main provides an example of using the SSE (Server-Sent Events) transport for gomcp
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Define the SSE server address with a dynamic port
	// Create a listener on a random available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}
	address := listener.Addr().String()
	listener.Close() // Close the listener as we just needed it to get a free port

	fmt.Printf("Using dynamic address: %s\n", address)

	// Start the server in a goroutine
	startServer(address)

	// Wait a bit longer for the server to initialize
	time.Sleep(2 * time.Second)

	// Start the client
	go runClient(address)

	// Wait for termination signal
	<-signals
	fmt.Println("\nShutdown signal received, exiting...")
}

func startServer(address string) {
	// Create a new server
	srv := server.NewServer("sse-example-server")

	// Configure the server with SSE transport (default paths)
	srv.AsSSE(address)

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
		fmt.Println("Starting SSE server on", address)
		if err := srv.Run(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

func runClient(address string) {
	// Use explicit http:// scheme for the SSE server
	// Do NOT include the /sse path - the transport will handle that
	serverURL := fmt.Sprintf("http://%s", address)

	fmt.Printf("Connecting to SSE server at URL: %s\n", serverURL)

	// Create a new client with the SSE server URL
	// For SSE connections, the oldest protocol version is automatically used
	// for maximum compatibility, unless explicitly overridden
	c, err := client.NewClient("",
		client.WithSSE(serverURL),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call the echo tool - connection happens automatically
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from SSE client!",
	})
	if err != nil {
		log.Fatalf("Echo call failed: %v", err)
	}
	fmt.Printf("Echo result: %v\n", echoResult)

	// Wait a moment to allow printing of results
	time.Sleep(500 * time.Millisecond)
}
