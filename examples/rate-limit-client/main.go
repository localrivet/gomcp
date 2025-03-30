package main

import (
	"context" // Added
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/client"   // Changed
	"github.com/localrivet/gomcp/protocol" // Added
)

// useTool sends a CallToolRequest using the client and processes the response.
// It returns the result Content slice or an error.
func useTool(ctx context.Context, clt *client.Client, toolName string, args map[string]interface{}) ([]protocol.Content, error) { // Changed types, added ctx
	// log.Printf("Sending CallToolRequest for tool '%s'...", toolName) // Reduce log noise for rapid requests
	reqParams := protocol.CallToolParams{ // Changed type
		Name:      toolName,
		Arguments: args,
	}

	// Call the tool using the client method
	// The client's internal sendRequestAndWait handles timeouts.
	// Pass nil for progressToken as we are not using progress reporting here.
	result, err := clt.CallTool(ctx, reqParams, nil) // Added ctx
	if err != nil {
		// This error could be a transport error, timeout, or an MCP error response
		// Check for specific rate limit error text (adjust if server uses a specific code)
		// Note: A more robust check would involve parsing the protocol.ErrorPayload if available
		if strings.Contains(err.Error(), "Too many requests") || strings.Contains(err.Error(), "RateLimitExceeded") || strings.Contains(err.Error(), fmt.Sprintf("[%d]", protocol.ErrorCodeMCPRateLimitExceeded)) {
			return nil, fmt.Errorf("RateLimitExceeded") // Return a simplified error for counting
		}
		return nil, fmt.Errorf("CallTool '%s' failed: %w", toolName, err)
	}

	// Check if the tool execution itself resulted in an error (IsError flag)
	if result.IsError != nil && *result.IsError {
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", toolName)
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(protocol.TextContent); ok { // Changed type
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", toolName, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", toolName, result.Content[0])
			}
		}
		// Return the content along with the error, as the content might contain error details
		return result.Content, fmt.Errorf("%s", errMsg) // Use format specifier
	}

	// log.Printf("Tool '%s' executed successfully.", toolName) // Reduce log noise
	return result.Content, nil
}

// runClientLogic creates a client, connects, and tests the rate limited tool.
func runClientLogic(ctx context.Context, clientName string) error { // Added ctx
	// Create a new client instance
	// Assuming rate-limit server runs on default 8080 - adjust if needed
	clt, err := client.NewClient(clientName, client.ClientOptions{
		ServerBaseURL: "http://127.0.0.1:8080",
		// Logger: Use default logger
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect and perform initialization
	log.Println("Connecting to server...")
	err = clt.Connect(ctx) // Added ctx
	if err != nil {
		return fmt.Errorf("client failed to connect: %w", err)
	}
	defer clt.Close() // Ensure connection is closed eventually
	log.Printf("Client connected successfully to server: %s (Version: %s)", clt.ServerInfo().Name, clt.ServerInfo().Version)
	// log.Printf("Server Capabilities: %+v", client.ServerCapabilities()) // Optional: Log capabilities

	// --- Test Rate Limiting ---
	log.Println("\n--- Testing Rate Limited Echo Tool ---")
	toolName := "limited-echo"
	successCount := 0
	rateLimitErrors := 0
	otherErrors := 0
	totalRequests := 10 // Number of requests to send rapidly

	log.Printf("Sending %d requests rapidly to test rate limit...", totalRequests)
	startTime := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex // To protect counters

	requestCtx, cancelRequests := context.WithTimeout(ctx, 30*time.Second) // Timeout for the whole batch
	defer cancelRequests()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			message := fmt.Sprintf("Request %d", reqNum)
			args := map[string]interface{}{"message": message}
			// Pass requestCtx to individual tool calls
			_, err := useTool(requestCtx, clt, toolName, args) // Pass client instance and ctx

			mu.Lock()
			if err != nil {
				if err.Error() == "RateLimitExceeded" { // Check for our simplified error
					rateLimitErrors++
				} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					log.Printf("Attempt %d: Request canceled or timed out: %v", reqNum, err)
					otherErrors++ // Count cancellations/timeouts as other errors
				} else {
					log.Printf("Attempt %d: UNEXPECTED ERROR - %v", reqNum, err) // Log unexpected errors
					otherErrors++
				}
			} else {
				successCount++
			}
			mu.Unlock()
			// No artificial delay - attempt to hit the burst/rate limit
		}(i + 1)
	}

	wg.Wait() // Wait for all requests to complete
	duration := time.Since(startTime)

	log.Printf("\nRate Limit Test Summary (%v elapsed):", duration)
	log.Printf("  Total Requests: %d", totalRequests)
	log.Printf("  Successful:     %d", successCount)
	log.Printf("  Rate Limited:   %d", rateLimitErrors)
	log.Printf("  Other Errors:   %d", otherErrors)

	// Basic validation: Expect some successes and some rate limit errors
	// The exact numbers depend on timing, but both should ideally be non-zero for a good test
	if successCount == 0 && totalRequests > 0 && otherErrors == 0 { // Only warn if no successes AND no other errors
		log.Println("WARNING: Expected some successful requests, but got none.")
	}
	// Allow rateLimitErrors to be 0 if totalRequests is small or timing is off
	// if rateLimitErrors == 0 && totalRequests > 5 { // Expect errors if sending more than burst+rate
	// 	log.Println("WARNING: Expected some rate limit errors, but got none (increase totalRequests or check server limits?).")
	// }
	if otherErrors > 0 {
		log.Println("ERROR: Encountered unexpected errors/timeouts during rate limit test.")
		// Return an error if unexpected errors occurred
		return fmt.Errorf("encountered %d unexpected errors/timeouts during rate limit test", otherErrors)
	}

	log.Println("Client operations finished.")
	return nil // Indicate overall success if no unexpected errors
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Rate Limit Example MCP Client...")

	clientName := "GoRateLimitClient-Refactored"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Add timeout context
	defer cancel()

	// Run the core client logic
	err := runClientLogic(ctx, clientName) // Pass context
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}

// Removed unused Ptr helper function
