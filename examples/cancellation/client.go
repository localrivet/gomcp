// Package main provides a client example for cancellation in MCP.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a new client
	c, err := client.NewClient("cancellation-example-client")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}

	// Connect to the server (assuming it's running with stdio transport)
	conn, err := client.ConnectStdio()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	c.Connect(conn)

	// Start the long-running task
	fmt.Println("Starting long-running task...")
	requestID := "task-123"

	// Create a channel to receive the response
	responseCh := make(chan interface{})
	errorCh := make(chan error)

	// Call the tool in a goroutine
	go func() {
		result, err := c.Call("longRunningTask", map[string]interface{}{
			"duration": 10,
		}, client.WithRequestID(requestID))

		if err != nil {
			errorCh <- err
			return
		}

		responseCh <- result
	}()

	// Wait for 2 seconds
	time.Sleep(2 * time.Second)

	// Send cancellation
	fmt.Println("Sending cancellation request...")

	// Create and send cancellation notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params": map[string]interface{}{
			"requestId": requestID,
			"reason":    "User requested cancellation",
		},
	}

	notificationBytes, _ := json.Marshal(notification)
	err = conn.Write(notificationBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send cancellation: %v\n", err)
	}

	// Wait for result or timeout
	select {
	case result := <-responseCh:
		fmt.Printf("Received result (shouldn't happen if cancelled): %v\n", result)
	case err := <-errorCh:
		fmt.Printf("Received error (expected if cancelled): %v\n", err)
	case <-time.After(15 * time.Second):
		fmt.Println("Timeout waiting for response")
	}

	fmt.Println("Client example completed")
}
