package v20250618

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
)

// TestSSETransport_draft tests SSE transport compliance with MCP 2025-06-18 specification
func TestSSETransport_draft(t *testing.T) {
	// Get a dynamic port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()

	// Create server with SSE transport
	srv := server.NewServer("sse-test-server")
	srv.AsSSE(address)

	// Register a test tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (interface{}, error) {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": args.Message},
			},
		}, nil
	})

	// Start server
	go func() {
		if err := srv.Run(); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	t.Run("Unified MCP endpoint supports GET for SSE", func(t *testing.T) {
		testUnifiedEndpointSSE_draft(t, address)
	})

	t.Run("Unified MCP endpoint supports POST for messages", func(t *testing.T) {
		testUnifiedEndpointPOST_draft(t, address)
	})

	t.Run("Single request support", func(t *testing.T) {
		testSingleRequests_draft(t, address)
	})

	t.Run("SSE streaming for requests", func(t *testing.T) {
		testSSEStreaming_draft(t, address)
	})

	t.Run("Session management", func(t *testing.T) {
		testSessionManagement_draft(t, address)
	})

	t.Run("Error handling", func(t *testing.T) {
		testErrorHandling_draft(t, address)
	})
}

// testUnifiedEndpointSSE_draft tests that the unified MCP endpoint supports GET for SSE
func testUnifiedEndpointSSE_draft(t *testing.T, address string) {
	// Connect to unified MCP endpoint for SSE
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	req, err := http.NewRequest("GET", mcpURL, nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to MCP endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Verify response is SSE
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("Expected Content-Type to contain text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// For draft, we shouldn't receive an "endpoint" event since the client
	// already knows the endpoint from the URL they connected to
	reader := bufio.NewReader(resp.Body)

	receivedEndpointEvent := false

	// Create a channel to receive read results
	type readResult struct {
		line []byte
		err  error
	}
	readCh := make(chan readResult, 1)

	// Create a context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start a goroutine to read from the stream
	go func() {
		for {
			line, err := reader.ReadBytes('\n')
			select {
			case readCh <- readResult{line: line, err: err}:
				if err != nil {
					return // Exit on error (including EOF)
				}
			case <-ctx.Done():
				return // Exit if context is cancelled
			}
		}
	}()

	// Read events for a short time to check if endpoint event is sent
	// We expect NO events to be sent for draft unified endpoint pattern
	readTimeout := time.NewTimer(500 * time.Millisecond) // Short timeout to check for events
	defer readTimeout.Stop()

	for {
		select {
		case <-readTimeout.C:
			// Timeout reached without receiving endpoint event - this is expected for draft
			if receivedEndpointEvent {
				t.Error("Should not receive endpoint event in draft unified endpoint pattern")
			}
			return
		case <-ctx.Done():
			// Context timeout - also acceptable
			if receivedEndpointEvent {
				t.Error("Should not receive endpoint event in draft unified endpoint pattern")
			}
			return
		case result := <-readCh:
			if result.err != nil {
				if result.err == io.EOF {
					// EOF is acceptable - connection closed
					return
				}
				t.Fatalf("Error reading SSE stream: %v", result.err)
			}

			line := bytes.TrimSpace(result.line)

			// Check if we receive an endpoint event (which we shouldn't)
			if bytes.HasPrefix(line, []byte("event: endpoint")) {
				receivedEndpointEvent = true
				t.Error("Should not receive endpoint event in draft unified endpoint pattern")
				return
			}

			// If we receive any other events, that's unexpected for this test but not necessarily an error
			if len(line) > 0 && bytes.HasPrefix(line, []byte("event:")) {
				t.Logf("Received unexpected event: %s", string(line))
			}
		}
	}
}

// testUnifiedEndpointPOST_draft tests that the unified MCP endpoint supports POST for messages
func testUnifiedEndpointPOST_draft(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Test initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	reqBody, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Client must include Accept header for both JSON and SSE
	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Server can respond with either JSON or SSE stream
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type to be application/json or text/event-stream, got %s", contentType)
	}

	// If it's JSON response, parse and validate
	if strings.Contains(contentType, "application/json") {
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}

		// Validate JSON-RPC response
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc '2.0', got %v", response["jsonrpc"])
		}
		if response["id"] != float64(1) {
			t.Errorf("Expected id 1, got %v", response["id"])
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T", response["result"])
		}

		// Validate draft specific response
		if result["protocolVersion"] != "2025-06-18" {
			t.Errorf("Expected protocolVersion 'draft', got %v", result["protocolVersion"])
		}
	}
}

// testSingleRequests_draft tests single request support in draft (no batching requirement)
func testSingleRequests_draft(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// First initialize
	initAndInitialized_draft(t, mcpURL)

	// Create single tool call request
	toolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "Single request test",
			},
		},
	}

	reqBody, err := json.Marshal(toolRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Response can be either JSON or SSE stream
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// JSON response
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Validate response
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc '2.0', got %v", response["jsonrpc"])
		}
		if response["id"] != float64(1) {
			t.Errorf("Expected id 1, got %v", response["id"])
		}

		// Validate tool response content
		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T", response["result"])
		}

		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content array, got %T", result["content"])
		}

		if len(content) == 0 {
			t.Fatal("Expected at least one content item")
		}

		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be object, got %T", content[0])
		}

		if contentItem["type"] != "text" {
			t.Errorf("Expected content type 'text', got %v", contentItem["type"])
		}
		if contentItem["text"] != "Single request test" {
			t.Errorf("Expected text 'Single request test', got %v", contentItem["text"])
		}
	} else if strings.Contains(contentType, "text/event-stream") {
		// SSE response - should receive response event
		reader := bufio.NewReader(resp.Body)
		foundResponse := false

		// Set timeout
		done := make(chan bool, 1)
		go func() {
			time.Sleep(3 * time.Second)
			done <- true
		}()

		for !foundResponse {
			select {
			case <-done:
				if !foundResponse {
					t.Error("Timeout waiting for SSE response")
				}
				return
			default:
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						return
					}
					t.Fatalf("Error reading SSE stream: %v", err)
				}

				line = bytes.TrimSpace(line)

				if bytes.HasPrefix(line, []byte("event: message")) {
					// Read data line
					dataLine, err := reader.ReadBytes('\n')
					if err != nil {
						t.Fatalf("Error reading response data: %v", err)
					}
					dataLine = bytes.TrimSpace(dataLine)

					if bytes.HasPrefix(dataLine, []byte("data: ")) {
						responseData := bytes.TrimPrefix(dataLine, []byte("data: "))
						var response map[string]interface{}
						if err := json.Unmarshal(responseData, &response); err == nil {
							if response["jsonrpc"] == "2.0" && response["id"] == float64(1) {
								foundResponse = true
							}
						}
					}
				}
			}
		}
	}
}

// testSSEStreaming_draft tests SSE streaming behavior for requests
func testSSEStreaming_draft(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// First initialize
	initAndInitialized_draft(t, mcpURL)

	// Send a request that should potentially stream responses
	toolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "Test streaming",
			},
		},
	}

	reqBody, err := json.Marshal(toolRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// The server can choose to stream or return immediate JSON
	// Both are valid according to the spec
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type to be application/json or text/event-stream, got %s", contentType)
	}

	// If streaming, ensure we get a proper response
	if strings.Contains(contentType, "text/event-stream") {
		reader := bufio.NewReader(resp.Body)
		foundResponse := false

		// Set timeout
		done := make(chan bool, 1)
		go func() {
			time.Sleep(3 * time.Second)
			done <- true
		}()

		for !foundResponse {
			select {
			case <-done:
				if !foundResponse {
					t.Error("Timeout waiting for streamed response")
				}
				return
			default:
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						return
					}
					t.Fatalf("Error reading SSE stream: %v", err)
				}

				line = bytes.TrimSpace(line)

				if bytes.HasPrefix(line, []byte("event: message")) {
					// Read data line
					dataLine, err := reader.ReadBytes('\n')
					if err != nil {
						t.Fatalf("Error reading response data: %v", err)
					}
					dataLine = bytes.TrimSpace(dataLine)

					if bytes.HasPrefix(dataLine, []byte("data: ")) {
						responseData := bytes.TrimPrefix(dataLine, []byte("data: "))
						var response map[string]interface{}
						if err := json.Unmarshal(responseData, &response); err == nil {
							if response["jsonrpc"] == "2.0" && response["id"] == float64(1) {
								foundResponse = true
							}
						}
					}
				}
			}
		}
	}
}

// testSessionManagement_draft tests session management features
func testSessionManagement_draft(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	reqBody, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check if server returns a session ID in Mcp-Session-Id header
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		t.Logf("Received session ID: %s", sessionID)

		// Session ID validation according to spec
		if len(sessionID) == 0 {
			t.Error("Session ID cannot be empty")
		}

		// Should only contain visible ASCII characters (0x21 to 0x7E)
		for _, char := range []byte(sessionID) {
			if char < 0x21 || char > 0x7E {
				t.Errorf("Session ID contains invalid character: %x", char)
			}
		}

		// Test subsequent request with session ID
		toolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "echo",
				"arguments": map[string]interface{}{
					"message": "Session test",
				},
			},
		}

		reqBody2, err := json.Marshal(toolRequest)
		if err != nil {
			t.Fatalf("Failed to marshal tool request: %v", err)
		}

		req2, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody2))
		if err != nil {
			t.Fatalf("Failed to create second POST request: %v", err)
		}
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Accept", "application/json, text/event-stream")
		req2.Header.Set("Mcp-Session-Id", sessionID) // Include session ID

		resp2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("Failed to send request with session ID: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with valid session ID, got %d", resp2.StatusCode)
		}
	} else {
		t.Log("Server does not use session management (optional feature)")
	}
}

// testErrorHandling_draft tests error handling scenarios
func testErrorHandling_draft(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Test invalid JSON
	req, err := http.NewRequest("POST", mcpURL, strings.NewReader("invalid json"))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send invalid JSON: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}

	// Test unsupported method
	req2, err := http.NewRequest("PUT", mcpURL, nil)
	if err != nil {
		t.Fatalf("Failed to create PUT request: %v", err)
	}

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Failed to send PUT request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for unsupported method, got %d", resp2.StatusCode)
	}

	// Test notification handling (should return 202)
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	reqBody, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	req3, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create notification request: %v", err)
	}
	req3.Header.Set("Content-Type", "application/json")

	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}
	defer resp3.Body.Close()

	// V20250618 spec: notifications should return 202 Accepted or 200 OK
	if resp3.StatusCode != http.StatusAccepted && resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected status 202 or 200 for notification, got %d", resp3.StatusCode)
	}
}

// Helper functions

func initAndInitialized_draft(t *testing.T, mcpURL string) {
	// Send initialize
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	_, err := sendJSONRPCRequest_draft(t, mcpURL, initRequest)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Send initialized notification
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	reqBody, _ := json.Marshal(initNotification)
	req, _ := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send initialized notification: %v", err)
	}
	defer resp.Body.Close()
}

func sendJSONRPCRequest_draft(t *testing.T, url string, request map[string]interface{}) (map[string]interface{}, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Handle both JSON and SSE responses
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, err
		}
		return response, nil
	} else if strings.Contains(contentType, "text/event-stream") {
		// Read SSE stream until we get the response
		reader := bufio.NewReader(resp.Body)

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, err
			}

			line = bytes.TrimSpace(line)

			if bytes.HasPrefix(line, []byte("event: message")) {
				dataLine, err := reader.ReadBytes('\n')
				if err != nil {
					return nil, err
				}
				dataLine = bytes.TrimSpace(dataLine)

				if bytes.HasPrefix(dataLine, []byte("data: ")) {
					responseData := bytes.TrimPrefix(dataLine, []byte("data: "))
					var response map[string]interface{}
					if err := json.Unmarshal(responseData, &response); err != nil {
						return nil, err
					}
					return response, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("unsupported content type: %s", contentType)
}
