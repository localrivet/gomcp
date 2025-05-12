package sse

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/protocol"
)

// findAvailablePort finds an available TCP port on the local machine.
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to listen on port 0: %w", err)
	}
	defer listener.Close() // Ensure the listener is closed

	// Get the port from the listener's address
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("could not get TCP address from listener")
	}
	return addr.Port, nil
}

// TestURLResolution tests that the SSE transport correctly resolves URLs
// when using the 2024-11-05 protocol which uses relative URLs
func TestURLResolution(t *testing.T) {
	// Find an available port for the test URLs
	availablePort, err := findAvailablePort()
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	baseHost := fmt.Sprintf("127.0.0.1:%d", availablePort)
	baseScheme := "http"

	testCases := []struct {
		name string
		// Use placeholders that will be replaced dynamically
		serverBaseURLTemplate       string
		relativeEndpoint            string
		expectedResolvedURLTemplate string
		expectError                 bool
	}{
		{
			name:                        "Absolute Path URL",
			serverBaseURLTemplate:       "%s://%s", // Scheme://Host
			relativeEndpoint:            "/message?sessionId=123",
			expectedResolvedURLTemplate: "%s://%s/message?sessionId=123",
			expectError:                 false,
		},
		{
			name:                        "Relative Path URL Without Leading Slash",
			serverBaseURLTemplate:       "%s://%s",
			relativeEndpoint:            "message?sessionId=123",
			expectedResolvedURLTemplate: "%s://%s/message?sessionId=123",
			expectError:                 false,
		},
		{
			name:                        "Absolute URL",
			serverBaseURLTemplate:       "%s://%s",
			relativeEndpoint:            "",
			expectedResolvedURLTemplate: "",
			expectError:                 true, // Expect an error because test client can't reach the other local port
		},
		{
			name:                        "URL With Path in Base",
			serverBaseURLTemplate:       "%s://%s/api", // Path in base
			relativeEndpoint:            "/message?sessionId=123",
			expectedResolvedURLTemplate: "%s://%s/message?sessionId=123", // Expect resolution to replace base path
			expectError:                 false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Construct dynamic URLs for this test case
			dynamicServerBaseURL := fmt.Sprintf(tc.serverBaseURLTemplate, baseScheme, baseHost)
			dynamicExpectedResolvedURL := fmt.Sprintf(tc.expectedResolvedURLTemplate, baseScheme, baseHost)
			// Handle absolute expected URLs
			if !strings.Contains(tc.expectedResolvedURLTemplate, "%s") {
				dynamicExpectedResolvedURL = tc.expectedResolvedURLTemplate
			}

			// --- Special handling for Absolute URL case ---
			absoluteTestHost := ""
			if tc.name == "Absolute URL" {
				absolutePort, err := findAvailablePort()
				if err != nil {
					t.Fatalf("Failed to find available port for absolute URL test: %v", err)
				}
				absoluteTestHost = fmt.Sprintf("127.0.0.1:%d", absolutePort)
				tc.relativeEndpoint = fmt.Sprintf("%s://%s/message?sessionId=123", baseScheme, absoluteTestHost)
				dynamicExpectedResolvedURL = tc.relativeEndpoint // Expected URL is the absolute one itself
			}
			// --- End special handling ---

			// Create a test server that will verify the resolved URL
			var receivedURL string
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedURL = "http://" + r.Host + r.URL.String()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer testServer.Close()

			// Parse the test server URL
			serverURL, err := url.Parse(testServer.URL)
			if err != nil {
				t.Fatalf("Failed to parse test server URL: %v", err)
			}

			// Create transport with old protocol version using the dynamic base URL
			transport, err := NewSSETransport(SSETransportOptions{
				BaseURL:         dynamicServerBaseURL,
				BasePath:        "/mcp",
				Logger:          NewNilLogger(),
				ProtocolVersion: protocol.OldProtocolVersion,
			})
			if err != nil {
				t.Fatalf("Failed to create transport: %v", err)
			}

			// Manually set the message endpoint to our test value
			transport.messageEndpointURL = tc.relativeEndpoint

			// Create a mock message to send (just needs to be valid JSON)
			message := `{"jsonrpc":"2.0","method":"ping","id":1}`

			// Replace the HTTP client with one using our test server
			originalURL, _ := url.Parse(transport.serverBaseURL)
			transport.serverBaseURL = testServer.URL
			transport.httpClient = testServer.Client()

			// Set sessionID to avoid requirement checks
			transport.sessionID = "test-session"

			// Send the message, which should resolve the URL
			err = transport.Send(context.Background(), []byte(message))

			// Verify error expectations
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err == nil {
				// Check for accurate URL resolution by comparing the resolved URL format
				// against what we'd expect after substituting the test server host
				expectedURL := strings.Replace(dynamicExpectedResolvedURL,
					originalURL.Host, serverURL.Host, 1)
				expectedURL = strings.Replace(expectedURL,
					originalURL.Scheme, serverURL.Scheme, 1)

				if receivedURL != expectedURL {
					t.Errorf("URL resolution failed.\nBase: %s\nRel: %s\nExpected: %s\nReceived: %s",
						dynamicServerBaseURL, tc.relativeEndpoint, expectedURL, receivedURL)
				}
			}

			// For the Absolute URL case, check that the error indicates the correct *different* local host was tried
			if tc.name == "Absolute URL" && err != nil {
				if !strings.Contains(err.Error(), absoluteTestHost) {
					t.Errorf("Expected error for Absolute URL case to mention host '%s', but got: %v", absoluteTestHost, err)
				}
			}
		})
	}
}
