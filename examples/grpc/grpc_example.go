// Package main provides an example of using gRPC transport for gomcp
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
)

// NOTE: This is a conceptual example only.
// The gomcp project currently doesn't provide a fully integrated
// gRPC transport in the server interface like other transports.
// A full implementation would require an AsGRPC method in the server interface.

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Define the gRPC host and port
	address := "localhost:50051"

	fmt.Println("=== Conceptual gRPC Example ===")
	fmt.Println("Note: This is a demonstration of how a gRPC transport would be")
	fmt.Println("used if fully implemented in the gomcp project.")
	fmt.Println()
	fmt.Println("Server would be configured as:")
	fmt.Println("  srv := server.NewServer(\"grpc-example-server\")")
	fmt.Println("  srv.AsGRPC(\"" + address + "\")")
	fmt.Println()

	// Server is just a concept in this example
	// (We would start a real server here)
	fmt.Println("Starting conceptual gRPC client...")

	// Show client code example
	runClientExample(address)

	// Wait for termination signal or timeout
	fmt.Println("Press Ctrl+C to exit or wait for 10 seconds...")
	select {
	case <-signals:
		fmt.Println("\nShutdown signal received, exiting...")
	case <-time.After(10 * time.Second):
		fmt.Println("Example timeout reached, exiting...")
	}
}

func runClientExample(address string) {
	// This is a conceptual example of how the gRPC client would be created
	fmt.Println("Client would be created as:")
	fmt.Println("  client.NewClient(\"grpc-example-client\",")
	fmt.Println("    client.WithGRPC(\"" + address + "\",")
	fmt.Println("      client.WithGRPCTimeout(5*time.Second),")
	fmt.Println("    ),")
	fmt.Println("    client.WithConnectionTimeout(5*time.Second),")
	fmt.Println("    client.WithRequestTimeout(30*time.Second),")
	fmt.Println("  )")
	fmt.Println()

	// Demonstrate tool call
	fmt.Println("Tool call would be executed as:")
	fmt.Println("  client.CallTool(\"echo\", map[string]interface{}{")
	fmt.Println("    \"message\": \"Hello from gRPC client!\",")
	fmt.Println("  })")
	fmt.Println()

	// Try to create an actual client to validate the WithGRPC option exists
	// This will fail since there's no gRPC server running, but it validates
	// that the client API accepts these options
	_, err := client.NewClient("grpc-example-client",
		client.WithGRPC(address),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		fmt.Printf("Note: Creating a real client failed as expected: %v\n", err)
		fmt.Println("A real implementation would require a running gRPC server.")
	}
}
