package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/localrivet/gomcp/protocol"
	// "github.com/localrivet/gomcp/transport/sse" // Ensure this is removed if unused
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/types/conversion"
)

// TestNewSSEClient tests the constructor and basic setup.
// Note: This test now primarily checks URL parsing and option setup,
// as the actual connection logic is within the transport and tested elsewhere
// or via end-to-end tests.
func TestNewSSEClient(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		basePath      string
		expectedError bool
		errorContains string
	}{
		{
			name:          "Valid URL and path",
			baseURL:       "http://localhost:8080",
			basePath:      "/mcp",
			expectedError: false,
		},
		{
			name:          "Invalid URL scheme", // SSE client constructor doesn't validate scheme anymore
			baseURL:       "ws://localhost:8080",
			basePath:      "/mcp",
			expectedError: false, // Error would happen during transport connection attempt
		},
		{
			name:          "Invalid URL format",
			baseURL:       "://invalid",
			basePath:      "/mcp",
			expectedError: true,
			errorContains: "invalid base URL",
		},
		{
			name:          "Empty base path",
			baseURL:       "http://localhost:8080",
			basePath:      "",
			expectedError: false,
		},
		{
			name:          "Path without leading slash",
			baseURL:       "http://localhost:8080",
			basePath:      "mcp",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ClientOptions{
				Logger: NewNilLogger(),
				// Transport will be created by NewSSEClient
			}

			// We expect NewSSEClient itself to fail only on base URL parsing now.
			// Transport creation errors are deferred.
			_, err := NewSSEClient("test-sse-client", tt.baseURL, tt.basePath, opts)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error creating client but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q but got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error creating client: %v", err)
				}
				// Further checks on the created client/transport could be added if needed,
				// but the core logic is tested via handshake tests in client_test.go
			}
		})
	}
}

// TestSSEClientEndToEnd simulates a basic end-to-end connection and handshake
// using the actual SSETransport and a mock HTTP server.
func TestSSEClientEndToEnd(t *testing.T) {
	protocols := []struct {
		name            string
		version         string
		needsEndpoint   bool
		explicitVersion bool // Whether to explicitly set the version or let it use the default
	}{
		{
			name:            "Current Protocol 2025-03-26",
			version:         protocol.CurrentProtocolVersion,
			needsEndpoint:   false,
			explicitVersion: true,
		},
		{
			name:            "Old Protocol 2024-11-05",
			version:         protocol.OldProtocolVersion,
			needsEndpoint:   true,
			explicitVersion: true,
		},
		{
			name:            "Default Protocol (should be 2024-11-05)",
			version:         protocol.OldProtocolVersion, // Expected version
			needsEndpoint:   true,
			explicitVersion: false, // Don't explicitly set it
		},
	}

	for _, proto := range protocols {
		t.Run(proto.name, func(t *testing.T) {
			// --- Mock HTTP Server Setup ---
			mcpPath := "/mcp-test" // Define the single MCP endpoint path for the test
			serverSessionID := "mock-e2e-session-123"

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				reqSessionID := r.Header.Get("Mcp-Session-Id") // Get session from header

				t.Logf("Mock Server E2E: Received %s %s (Header Session: %s)", r.Method, path, reqSessionID)

				if path != mcpPath {
					http.NotFound(w, r)
					return
				}

				// --- Handle POST Requests (Client -> Server) ---
				if r.Method == http.MethodPost {
					var req protocol.JSONRPCRequest
					// Use io.ReadAll to avoid potential issues with Decode closing the body prematurely
					bodyBytes, err := io.ReadAll(r.Body)
					if err != nil {
						t.Logf("Mock Server E2E: Failed to read POST body: %v", err)
						http.Error(w, "Failed to read body", http.StatusInternalServerError)
						return
					}
					if err := json.Unmarshal(bodyBytes, &req); err != nil {
						t.Logf("Mock Server E2E: Bad JSON in POST: %v", err)
						http.Error(w, "Bad JSON", http.StatusBadRequest)
						return
					}
					t.Logf("Mock Server E2E: Received POST %s (ID: %v)", req.Method, req.ID)

					// Handle Initialize Request
					if req.Method == protocol.MethodInitialize {
						if reqSessionID != "" {
							t.Logf("Mock Server E2E: Error - InitializeRequest should not have Mcp-Session-Id header")
							http.Error(w, "Initialize should not have session ID", http.StatusBadRequest)
							return
						}

						// Unmarshal to get capabilities and version requested
						var initParams protocol.InitializeRequestParams
						if err := protocol.UnmarshalPayload(req.Params, &initParams); err != nil {
							t.Logf("Mock Server E2E: Failed to parse InitializeRequestParams: %v", err)
							http.Error(w, "Bad params", http.StatusBadRequest)
							return
						}

						t.Logf("Mock Server E2E: Client requested protocol version: %s", initParams.ProtocolVersion)

						// Server should respond with proto.version, but log both to understand negotiations
						t.Logf("Mock Server E2E: Server responding with protocol version: %s", proto.version)

						// Respond with InitializeResult and Session ID header
						initResult := protocol.InitializeResult{
							ProtocolVersion: proto.version, // Use the requested protocol version
							Capabilities:    protocol.ServerCapabilities{},
							ServerInfo:      protocol.Implementation{Name: "MockE2EServer", Version: "1.0"},
						}
						resp := protocol.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: initResult}
						respBytes, _ := json.Marshal(resp)

						w.Header().Set("Content-Type", "application/json") // Respond with JSON for Initialize
						w.Header().Set("Mcp-Session-Id", serverSessionID)  // Set the session ID header
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(respBytes)
						t.Logf("Mock Server E2E: Sent InitializeResponse (ID: %v) with Session ID: %s", req.ID, serverSessionID)
						return
					}

					// Handle other POST requests (require valid session ID)
					if reqSessionID == "" || reqSessionID != serverSessionID {
						t.Logf("Mock Server E2E: Invalid or missing session ID for %s. Expected '%s', got '%s'", req.Method, serverSessionID, reqSessionID)
						http.Error(w, "Invalid or missing Mcp-Session-Id header", http.StatusBadRequest)
						return
					}

					if req.Method == protocol.MethodInitialized {
						t.Logf("Mock Server E2E: Received Initialized notification (Session: %s)", reqSessionID)
						w.WriteHeader(http.StatusAccepted) // Respond with 202 Accepted
						t.Logf("Mock Server E2E: Sent 202 Accepted for Initialized.")
					} else {
						// Handle other potential client requests/notifications if needed
						t.Logf("Mock Server E2E: Received unexpected POST method %s", req.Method)
						http.Error(w, "Unexpected method", http.StatusBadRequest)
					}
					return
				}

				// --- Handle GET Requests (Server -> Client SSE Stream) ---
				if r.Method == http.MethodGet {
					flusher, ok := w.(http.Flusher)
					if !ok {
						http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					w.Header().Set("Cache-Control", "no-cache")
					// Send initial headers and potentially a comment to keep connection alive
					_, _ = w.Write([]byte(": stream open\n\n"))
					flusher.Flush() // Use flusher
					t.Logf("Mock Server E2E: Established SSE stream for GET request (Session: %s)", reqSessionID)

					// For old protocol version, we need to send the endpoint event first
					if proto.needsEndpoint {
						// For 2024-11-05 protocol, must send endpoint event immediately
						// The proper format for an endpoint URL is a complete URL, not just the host
						scheme := "http"
						if r.TLS != nil {
							scheme = "https"
						}
						// Important: The endpoint URL is just the plain URL string without quotes or JSON formatting
						endpointURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, mcpPath)
						_, _ = w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", endpointURL)))
						flusher.Flush()
						t.Logf("Mock Server E2E: Sent 'endpoint' event for 2024-11-05 protocol: %s", endpointURL)
					}

					// Send an initial valid event to ensure the client library proceeds
					_, _ = w.Write([]byte("event: message\ndata: {\"status\":\"ready\"}\n\n"))
					flusher.Flush()
					t.Logf("Mock Server E2E: Sent initial 'message' event.")

					// Send a properly formatted JSON-RPC notification that the client expects
					testNotif := protocol.JSONRPCNotification{
						JSONRPC: "2.0",
						Method:  "test/serverNotification",
						Params:  map[string]interface{}{"message": "This is a test notification"},
					}
					notifData, _ := json.Marshal(testNotif)
					_, _ = w.Write([]byte("event: message\ndata: " + string(notifData) + "\n\n"))
					flusher.Flush()
					t.Logf("Mock Server E2E: Sent test notification message")

					// Instead of just waiting for context to complete, send periodic keepalive
					// while listening for context done
					ticker := time.NewTicker(1 * time.Second)
					defer ticker.Stop()

					for {
						select {
						case <-ticker.C:
							// Send a comment as keepalive
							_, _ = w.Write([]byte(": keepalive\n\n"))
							flusher.Flush()
						case <-r.Context().Done():
							t.Logf("Mock Server E2E: GET stream context done (client likely closed).")
							return // End the handler when client disconnects
						}
					}
				}

				// Method Not Allowed for other HTTP methods
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}))
			defer server.Close()
			// --- End Mock Server Setup ---

			opts := ClientOptions{
				Logger: logx.NewDefaultLogger(), // Use DefaultLogger instead of NilLogger to get debug logs
				// Only set protocol version explicitly if the test case requires it
			}

			// Only set protocol version if explicitVersion is true
			if proto.explicitVersion {
				opts.PreferredProtocolVersion = conversion.StrPtr(proto.version)
			}

			// Use NewSSEClient, providing the base path for the single MCP endpoint
			client, err := NewSSEClient("TestSSEClientE2E", server.URL, mcpPath, opts)
			if err != nil {
				t.Fatalf("NewSSEClient failed: %v", err)
			}

			// Connect and verify state with a shorter timeout to avoid lengthy hangs
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			t.Logf("Running test with protocol %s (needsEndpoint=%v, explicitVersion=%v)",
				proto.version, proto.needsEndpoint, proto.explicitVersion)

			t.Logf("TestSSEClientEndToEnd (%s): Calling client.Connect()...", proto.name)
			err = client.Connect(ctx) // This performs the POST Initialize, GET SSE, POST Initialized flow
			if err != nil {
				t.Fatalf("Client.Connect failed: %v", err)
			}
			t.Logf("TestSSEClientEndToEnd (%s): client.Connect() returned successfully.", proto.name)

			// Defer Close call
			defer func() {
				t.Logf("TestSSEClientEndToEnd (%s): Calling client.Close() via defer...", proto.name)
				client.Close()
				t.Logf("TestSSEClientEndToEnd (%s): client.Close() returned.", proto.name)
			}()

			// Verify state after successful connection
			if !client.IsInitialized() {
				t.Error("Client should be initialized after successful connect")
			}
			// The IsInitialized check above implicitly verifies that the handshake,
			// including session ID exchange via headers, was successful.
			// We don't need to check the transport type or access its internal fields here.
		})
	}
}

// TestSSEClientReconnection is harder to test reliably without more control
// over the mock server or transport layer failures. The r3labs/sse client handles
// reconnection internally based on its strategy. We might need more specific
// transport-level tests or integration tests for robust reconnection validation.
// func TestSSEClientReconnection(t *testing.T) { ... }
