// Package main provides a client example for cancellation in MCP.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a simple logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fmt.Println("Creating MCP client...")

	// Create a new client using stdio transport
	c, err := client.NewClient("stdio:///",
		client.WithStdio(),
		client.WithLogger(logger),
		client.WithProtocolVersion("draft"),
		client.WithRequestTimeout(15*time.Second), // Set timeout at client creation instead of per-request
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Start the long-running task
	fmt.Println("Starting long-running task...")
	requestID := "task-123"

	// Create a channel to receive the response
	responseCh := make(chan interface{})
	errorCh := make(chan error)

	// Call the tool in a goroutine
	go func() {
		// Use the regular API without method chaining
		result, err := c.CallTool("longRunningTask", map[string]interface{}{
			"duration": 10,
		})

		if err != nil {
			errorCh <- err
			return
		}

		responseCh <- result
	}()

	// Wait for 2 seconds
	time.Sleep(2 * time.Second)

	// Send cancellation (if this method isn't directly available, use a standard tool call instead)
	fmt.Println("Sending cancellation request...")

	// Use a regular tool call for cancellation instead of CancelRequest fluent API
	_, err = c.CallTool("cancel", map[string]interface{}{
		"requestId": requestID,
		"reason":    "User requested cancellation",
	})

	if err != nil {
		fmt.Printf("Error cancelling request: %v\n", err)
	} else {
		fmt.Println("Cancellation request sent successfully")
	}

	// For demonstration purposes, we'll just wait for the response
	fmt.Println("Waiting for response or error...")

	// Wait for result or timeout
	select {
	case result := <-responseCh:
		fmt.Printf("Received result (should time out or error if cancelled properly): %v\n", result)
	case err := <-errorCh:
		fmt.Printf("Received error (expected if cancelled): %v\n", err)
	case <-time.After(15 * time.Second):
		fmt.Println("Timeout waiting for response")
	}

	// Get available tools using regular API
	fmt.Println("Checking available tools...")
	// Use standard resource access since there's no direct GetTools method
	tools, err := c.GetResource("/tools")
	if err != nil {
		fmt.Printf("Error getting tools: %v\n", err)
	} else {
		fmt.Printf("Available tools: %v\n", tools)
	}

	fmt.Println("Client example completed")
}
