// Package main provides an example of using Unix Socket transport for gomcp
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/unix"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Define the Unix socket path - using a temporary directory for this example
	socketPath := filepath.Join(os.TempDir(), "gomcp-unix-example.sock")

	// Remove the socket file if it already exists
	if _, err := os.Stat(socketPath); err == nil {
		if err := os.Remove(socketPath); err != nil {
			log.Fatalf("Failed to remove existing socket file: %v", err)
		}
	}

	// Start the server in a goroutine
	startServer(socketPath)

	// Wait a bit for the server to initialize
	time.Sleep(1 * time.Second)

	// Start the client
	go runClient(socketPath)

	// Wait for termination signal
	<-signals
	fmt.Println("\nShutdown signal received, exiting...")

	// Clean up the socket file
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove socket file: %v\n", err)
	}
}

func startServer(socketPath string) {
	// Create a new server
	srv := server.NewServer("unix-example-server")

	// Configure the server with Unix Socket transport
	srv.AsUnixSocket(socketPath,
		unix.WithPermissions(0600), // Only owner can read/write
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
		fmt.Println("Starting Unix Socket server on", socketPath)
		if err := srv.Run(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

func runClient(socketPath string) {
	// Create a new client with Unix Socket transport
	c, err := client.NewClient("unix-example-client",
		client.WithUnixSocket(socketPath),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call the echo tool - connection happens automatically
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from Unix Socket client!",
	})
	if err != nil {
		log.Fatalf("Echo call failed: %v", err)
	}
	fmt.Printf("Echo result: %v\n", echoResult)

	// Wait a moment to allow printing of results
	time.Sleep(500 * time.Millisecond)
}
