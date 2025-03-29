// examples/rate-limit-client/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mcp "github.com/localrivet/gomcp"
)

// Helper function to use a tool (copied from other client example)
// Includes basic error checking for MCP errors.
func useTool(conn *mcp.Connection, toolName string, args map[string]interface{}) (interface{}, error) {
	log.Printf("Sending UseToolRequest for tool '%s'...", toolName)
	reqPayload := mcp.UseToolRequestPayload{ToolName: toolName, Arguments: args}
	err := conn.SendMessage(mcp.MessageTypeUseToolRequest, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to send UseToolRequest for '%s': %w", toolName, err)
	}

	// log.Println("Waiting for UseToolResponse...") // Reduce log noise for rapid requests
	var responseMsg *mcp.Message
	var receiveErr error
	done := make(chan struct{})
	go func() { defer close(done); responseMsg, receiveErr = conn.ReceiveMessage() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for UseToolResponse for '%s'", toolName)
	}
	if receiveErr != nil {
		return nil, fmt.Errorf("failed to receive UseToolResponse for '%s': %w", toolName, receiveErr)
	}
	if responseMsg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		if err := mcp.UnmarshalPayload(responseMsg.Payload, &errPayload); err == nil {
			return nil, fmt.Errorf("tool '%s' failed with MCP Error: [%d] %s", toolName, errPayload.Code, errPayload.Message) // Use %d for int code
		}
		return nil, fmt.Errorf("tool '%s' failed with an unparsable MCP Error payload", toolName)
	}
	if responseMsg.MessageType != mcp.MessageTypeUseToolResponse {
		return nil, fmt.Errorf("expected UseToolResponse for '%s', got %s", toolName, responseMsg.MessageType)
	}
	var responsePayload mcp.UseToolResponsePayload
	err = mcp.UnmarshalPayload(responseMsg.Payload, &responsePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal UseToolResponse payload for '%s': %w", toolName, err)
	}
	return responsePayload.Result, nil
}

// runClientLogic performs handshake and tests the rate limited tool.
func runClientLogic(conn *mcp.Connection, clientName string) error {
	// --- Handshake ---
	log.Println("Sending HandshakeRequest...")
	hsReqPayload := mcp.HandshakeRequestPayload{SupportedProtocolVersions: []string{mcp.CurrentProtocolVersion}, ClientName: clientName}
	err := conn.SendMessage(mcp.MessageTypeHandshakeRequest, hsReqPayload)
	if err != nil {
		return fmt.Errorf("failed to send HandshakeRequest: %w", err)
	}
	msg, err := conn.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive HandshakeResponse: %w", err)
	}
	if msg.MessageType == mcp.MessageTypeError {
		var errPayload mcp.ErrorPayload
		_ = mcp.UnmarshalPayload(msg.Payload, &errPayload)
		return fmt.Errorf("handshake failed with MCP Error: [%d] %s", errPayload.Code, errPayload.Message) // Use %d for int code
	}
	if msg.MessageType != mcp.MessageTypeHandshakeResponse {
		return fmt.Errorf("expected HandshakeResponse, got %s", msg.MessageType)
	}
	var hsRespPayload mcp.HandshakeResponsePayload
	err = mcp.UnmarshalPayload(msg.Payload, &hsRespPayload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal HandshakeResponse payload: %w", err)
	}
	if hsRespPayload.SelectedProtocolVersion != mcp.CurrentProtocolVersion {
		return fmt.Errorf("server selected unsupported protocol version: %s", hsRespPayload.SelectedProtocolVersion)
	}
	log.Printf("Handshake successful with server: %s", hsRespPayload.ServerName)
	// --- End Handshake ---

	// --- Test Rate Limiting ---
	log.Println("\n--- Testing Rate Limited Echo Tool ---")
	toolName := "limited-echo"
	successCount := 0
	rateLimitErrors := 0
	otherErrors := 0
	totalRequests := 10 // Number of requests to send rapidly

	log.Printf("Sending %d requests rapidly to test rate limit...", totalRequests)
	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		message := fmt.Sprintf("Request %d", i+1)
		args := map[string]interface{}{"message": message}
		_, err := useTool(conn, toolName, args) // Result not checked here, only error

		if err != nil {
			// log.Printf("Attempt %d: ERROR - %v", i+1, err) // Reduce log noise
			if strings.Contains(err.Error(), "RateLimitExceeded") {
				rateLimitErrors++
			} else {
				log.Printf("Attempt %d: UNEXPECTED ERROR - %v", i+1, err) // Log unexpected errors
				otherErrors++
			}
		} else {
			// log.Printf("Attempt %d: SUCCESS", i+1) // Reduce log noise
			successCount++
		}
		// No artificial delay - attempt to hit the burst/rate limit
	}
	duration := time.Since(startTime)

	log.Printf("\nRate Limit Test Summary (%v elapsed):", duration)
	log.Printf("  Total Requests: %d", totalRequests)
	log.Printf("  Successful:     %d", successCount)
	log.Printf("  Rate Limited:   %d", rateLimitErrors)
	log.Printf("  Other Errors:   %d", otherErrors)

	// Basic validation: Expect some successes and some rate limit errors
	// The exact numbers depend on timing, but both should ideally be non-zero
	if successCount == 0 {
		log.Println("WARNING: Expected some successful requests, but got none.")
	}
	if rateLimitErrors == 0 {
		log.Println("WARNING: Expected some rate limit errors, but got none (increase totalRequests or check server limits?).")
	}
	if otherErrors > 0 {
		log.Println("ERROR: Encountered unexpected errors during rate limit test.")
		// Return an error if unexpected errors occurred
		return fmt.Errorf("encountered %d unexpected errors during rate limit test", otherErrors)
	}

	log.Println("Client finished.")
	return nil // Indicate overall success if no unexpected errors
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Rate Limit Example MCP Client...")

	clientName := "GoRateLimitClient"
	conn := mcp.NewStdioConnection()

	// Run the core client logic
	err := runClientLogic(conn, clientName)
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}
}
