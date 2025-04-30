package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/protocol"
)

// TestURLResolution tests that the SSE transport correctly resolves URLs
// when using the 2024-11-05 protocol which uses relative URLs
func TestURLResolution(t *testing.T) {
	testCases := []struct {
		name                string
		serverBaseURL       string
		relativeEndpoint    string
		expectedResolvedURL string
		expectError         bool
	}{
		{
			name:                "Absolute Path URL",
			serverBaseURL:       "http://example.com",
			relativeEndpoint:    "/message?sessionId=123",
			expectedResolvedURL: "http://example.com/message?sessionId=123",
			expectError:         false,
		},
		{
			name:                "Relative Path URL Without Leading Slash",
			serverBaseURL:       "http://example.com",
			relativeEndpoint:    "message?sessionId=123",
			expectedResolvedURL: "http://example.com/message?sessionId=123",
			expectError:         false,
		},
		{
			name:                "Absolute URL",
			serverBaseURL:       "http://example.com",
			relativeEndpoint:    "http://api.example.com/message?sessionId=123",
			expectedResolvedURL: "http://api.example.com/message?sessionId=123",
			expectError:         false,
		},
		{
			name:                "URL With Path in Base",
			serverBaseURL:       "http://example.com/api",
			relativeEndpoint:    "/message?sessionId=123",
			expectedResolvedURL: "http://example.com/api/message?sessionId=123",
			expectError:         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			// Create transport with old protocol version
			transport, err := NewSSETransport(SSETransportOptions{
				BaseURL:         tc.serverBaseURL,
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
				expectedURL := strings.Replace(tc.expectedResolvedURL,
					originalURL.Host, serverURL.Host, 1)
				expectedURL = strings.Replace(expectedURL,
					originalURL.Scheme, serverURL.Scheme, 1)

				if receivedURL != expectedURL {
					t.Errorf("URL resolution failed.\nExpected: %s\nReceived: %s",
						expectedURL, receivedURL)
				}
			}
		})
	}
}
