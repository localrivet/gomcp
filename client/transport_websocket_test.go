package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebSocketTransport(t *testing.T) {
	// Create a test server for URL construction
	server := httptest.NewServer(nil)
	defer server.Close()

	// Convert the server URL to WebSocket URL
	wsURL := "ws://" + server.Listener.Addr().String()

	// Create a logger for the transport
	logger := logx.NewDefaultLogger()

	// Test 1: Create with minimal parameters
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Check transport type
	assert.Equal(t, TransportTypeWebSocket, transport.GetTransportType())

	// Check transport info
	info := transport.GetTransportInfo()
	assert.NotNil(t, info)
	assert.Contains(t, info["baseURL"], wsURL)

	// Test 2: Create with additional options
	headers := http.Header{}
	headers.Add("X-Test-Header", "test-value")

	transport, err = NewWebSocketTransport(wsURL, "/mcp", logger,
		WithHeaders(headers),
		WithConnectTimeout(5*time.Second),
		WithWebSocketCompression(true),
	)
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Test 3: Invalid URL - it seems the URL validation happens at connection time, not creation
	invalidTransport, invalidErr := NewWebSocketTransport("invalid://url", "/mcp", logger)
	// The constructor likely doesn't validate the URL format, only the Connect method would
	require.NoError(t, invalidErr)
	require.NotNil(t, invalidTransport)

	// If we attempted to connect with this transport, it would fail
	// We don't test that here to avoid network calls in unit tests

	// Test 4: Empty base path should be handled
	transport, err = NewWebSocketTransport(wsURL, "", logger)
	require.NoError(t, err)
	require.NotNil(t, transport)
}

func TestWebSocketTransportConnect(t *testing.T) {
	// Create a test WebSocket server
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections for testing
		},
	}

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request should be upgraded to WebSocket
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()

			// Keep the connection open for the test
			// In a real test, we would handle received messages here
			<-r.Context().Done()
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Convert the server URL to WebSocket URL
	wsURL := "ws://" + server.Listener.Addr().String()
	logger := logx.NewDefaultLogger()

	// Test 1: Successful connection
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Connect to the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, transport.IsConnected())

	// Clean up
	defer transport.Close()

	// Test 2: Connection to non-existent endpoint should fail
	badPathTransport, err := NewWebSocketTransport(wsURL, "/nonexistent", logger)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	err = badPathTransport.Connect(ctx2)
	assert.Error(t, err)
	assert.False(t, badPathTransport.IsConnected())

	// Test 3: Already connected - try to connect again
	err = transport.Connect(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrAlreadyConnected)

	// Test 4: Invalid URL
	invalidTransport, _ := NewWebSocketTransport("invalid://url", "/mcp", logger)
	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()

	err = invalidTransport.Connect(ctx3)
	assert.Error(t, err)
	assert.False(t, invalidTransport.IsConnected())

	// Test 5: Context timeout
	timeoutTransport, _ := NewWebSocketTransport(wsURL, "/mcp", logger)
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer timeoutCancel()

	// Sleep to ensure the context times out
	time.Sleep(10 * time.Millisecond)
	err = timeoutTransport.Connect(timeoutCtx)
	assert.Error(t, err)
	// Check if the error is related to context timeout
	assert.True(t,
		strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "i/o timeout"),
		"Expected a timeout-related error, got: %s", err.Error())
}

func TestWebSocketTransportClose(t *testing.T) {
	// Create a test WebSocket server
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()
			<-r.Context().Done()
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()
	logger := logx.NewDefaultLogger()

	// Test 1: Close an open connection
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, transport.IsConnected())

	// Close the connection
	err = transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.IsConnected())

	// Test 2: Close an already closed connection (should not error)
	err = transport.Close()
	assert.NoError(t, err)

	// Test 3: Close a never-connected transport
	neverConnectedTransport, _ := NewWebSocketTransport(wsURL, "/mcp", logger)
	err = neverConnectedTransport.Close()
	assert.NoError(t, err)
}

func TestWebSocketTransportIsConnected(t *testing.T) {
	// Create a test WebSocket server
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Create channel to detect server shutdown
	serverShutdown := make(chan struct{})

	// Create a mutex for synchronizing access to the serverIsClosing flag
	var serverIsClosing bool
	var serverMu sync.Mutex

	// Setup a goroutine that will be notified when server is closing
	go func() {
		<-serverShutdown
		serverMu.Lock()
		serverIsClosing = true
		serverMu.Unlock()
	}()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			serverMu.Lock()
			closing := serverIsClosing
			serverMu.Unlock()

			if closing {
				// If we're in the closing state, return an error to simulate server disconnect
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Normal connection handling
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()
			<-r.Context().Done()
		}
	}))
	defer func() {
		// Signal we're shutting down the server
		close(serverShutdown)
		server.Close()
	}()

	wsURL := "ws://" + server.Listener.Addr().String()
	logger := logx.NewDefaultLogger()

	// Test 1: Never connected transport
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)
	assert.False(t, transport.IsConnected())

	// Test 2: Connected transport
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, transport.IsConnected())

	// Test 3: Transport after closing
	err = transport.Close()
	require.NoError(t, err)
	assert.False(t, transport.IsConnected())

	// Test 4: Simulate server closure in a simpler way
	// Instead of closing the server directly, which doesn't reliably break connections,
	// we'll manually close the WebSocket connection in a separate test

	// Create a special server for this test that will close connections
	closingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			// Intentionally short deadline to force connection to close quickly
			time.AfterFunc(200*time.Millisecond, func() {
				conn.Close()
			})
			<-r.Context().Done()
		}
	}))
	defer closingServer.Close()

	closingWsURL := "ws://" + closingServer.Listener.Addr().String()
	closingTransport, _ := NewWebSocketTransport(closingWsURL, "/mcp", logger)

	err = closingTransport.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, closingTransport.IsConnected())

	// Wait for the connection to be closed by the server
	time.Sleep(500 * time.Millisecond)

	// Make a few Ping calls to ensure the transport detects the disconnect
	for i := 0; i < 3; i++ {
		// Cast to websocketTransport to access the Send method directly
		if wst, ok := closingTransport.(*websocketTransport); ok {
			wst.transport.Send(context.Background(), []byte("ping"))
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Transport should detect it's no longer connected
	assert.False(t, closingTransport.IsConnected(), "Transport should detect server disconnection")
}

func TestWebSocketTransportSendRequest(t *testing.T) {
	// Create a test server that echoes back requests as responses
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Track received requests
	receivedRequests := make(chan []byte, 10)

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()

			// Read messages and echo them back as responses
			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					break
				}

				// Store the received request
				receivedRequests <- message

				// Parse the message to determine if it's a request
				var baseMsg map[string]interface{}
				if err := json.Unmarshal(message, &baseMsg); err != nil {
					t.Logf("Failed to parse message: %v", err)
					continue
				}

				// Check if it's a JSON-RPC request
				if _, hasMethod := baseMsg["method"]; hasMethod {
					// Create a response
					responseID := baseMsg["id"]
					response := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      responseID,
						"result":  map[string]interface{}{"echo": true},
					}

					responseBytes, _ := json.Marshal(response)

					// Add newline to match the client's behavior
					responseBytes = append(responseBytes, '\n')

					// Send the response back
					err = conn.WriteMessage(messageType, responseBytes)
					if err != nil {
						t.Logf("Failed to send response: %v", err)
						break
					}
				}
			}
		}
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()
	logger := logx.NewDefaultLogger()

	// Create and connect the transport
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Test 1: Successful request/response
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-request-1",
		Method:  "test.method",
		Params:  map[string]interface{}{"param": "value"},
	}

	resp, err := transport.SendRequest(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, req.ID, resp.ID)

	// Verify we received the expected request
	select {
	case receivedReq := <-receivedRequests:
		var reqObj protocol.JSONRPCRequest
		err := json.Unmarshal(receivedReq, &reqObj)
		require.NoError(t, err)
		assert.Equal(t, req.ID, reqObj.ID)
		assert.Equal(t, req.Method, reqObj.Method)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for request")
	}

	// Test 2: Request with context timeout
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer timeoutCancel()

	// Create a server that never responds
	nonRespondingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			// Read request but don't respond
			_, _, _ = conn.ReadMessage()

			// Wait for connection to be closed
			<-r.Context().Done()
		}
	}))
	defer nonRespondingServer.Close()

	nonRespondingURL := "ws://" + nonRespondingServer.Listener.Addr().String()
	timeoutTransport, _ := NewWebSocketTransport(nonRespondingURL, "/mcp", logger)

	// Connect to the non-responding server
	connCtx, connCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connCancel()

	err = timeoutTransport.Connect(connCtx)
	require.NoError(t, err)
	defer timeoutTransport.Close()

	timeoutReq := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "timeout-request",
		Method:  "test.timeout",
		Params:  map[string]interface{}{},
	}

	_, err = timeoutTransport.SendRequest(timeoutCtx, timeoutReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")

	// Test 3: Not connected
	disconnectedTransport, _ := NewWebSocketTransport(wsURL, "/mcp", logger)

	req = &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-request-2",
		Method:  "test.method",
		Params:  map[string]interface{}{},
	}

	_, err = disconnectedTransport.SendRequest(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestWebSocketTransportSendRequestAsync(t *testing.T) {
	// Create a test server that echoes back requests as responses
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Create channel to signal when the server has received a request
	requestReceived := make(chan []byte, 1)

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()

			// Read and process messages
			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					break
				}

				// Signal that a request was received and store it
				select {
				case requestReceived <- message:
					// Successfully stored
				default:
					// Channel is full, which is fine
				}

				// Parse the message to determine if it's a request
				var baseMsg map[string]interface{}
				if err := json.Unmarshal(message, &baseMsg); err != nil {
					t.Logf("Failed to parse message: %v", err)
					continue
				}

				// Check if it's a JSON-RPC request
				if _, hasMethod := baseMsg["method"]; hasMethod {
					// Create a response
					responseID := baseMsg["id"]
					response := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      responseID,
						"result":  map[string]interface{}{"echo": true, "async": true},
					}

					responseBytes, _ := json.Marshal(response)

					// Add newline to match the client's behavior
					responseBytes = append(responseBytes, '\n')

					// Send the response back immediately
					err = conn.WriteMessage(messageType, responseBytes)
					if err != nil {
						t.Logf("Failed to send response: %v", err)
						break
					}
				}
			}
		}
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()
	logger := logx.NewDefaultLogger()

	// Create and connect the transport
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Test 1: Successful async request with response channel
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "async-request-1",
		Method:  "test.async.method",
		Params:  map[string]interface{}{"param": "value"},
	}

	// Create a buffered channel to receive the response
	responseCh := make(chan *protocol.JSONRPCResponse, 5)

	// Send the async request
	err = transport.SendRequestAsync(ctx, req, responseCh)
	require.NoError(t, err)

	// Wait to ensure the request is received by the server
	select {
	case receivedRequest := <-requestReceived:
		t.Logf("Server received request: %s", string(receivedRequest))
	case <-time.After(500 * time.Millisecond):
		t.Logf("Request might not have been received by server yet, continuing anyway")
	}

	// Wait for the response
	var resp *protocol.JSONRPCResponse
	select {
	case resp = <-responseCh:
		require.NotNil(t, resp)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, req.ID, resp.ID)

		// Verify the result contains the expected fields
		result, ok := resp.Result.(map[string]interface{})
		require.True(t, ok, "Result is not a map: %v", resp.Result)
		assert.Equal(t, true, result["echo"])
		assert.Equal(t, true, result["async"])

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for async response")
	}

	// Test 2: Async request with nil response channel (fire and forget)
	req = &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "async-request-2",
		Method:  "test.async.method",
		Params:  map[string]interface{}{"param": "fire-and-forget"},
	}

	// Send the async request without a response channel
	err = transport.SendRequestAsync(ctx, req, nil)
	require.NoError(t, err)

	// Verify the request was received
	select {
	case receivedRequest := <-requestReceived:
		t.Logf("Server received fire-and-forget request: %s", string(receivedRequest))
	case <-time.After(500 * time.Millisecond):
		t.Logf("Fire-and-forget request might not have been received yet")
	}

	// Test 3: Not connected
	disconnectedTransport, _ := NewWebSocketTransport(wsURL, "/mcp", logger)

	req = &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "async-request-3",
		Method:  "test.async.method",
		Params:  map[string]interface{}{},
	}

	err = disconnectedTransport.SendRequestAsync(ctx, req, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Test 4: Validation error (marshal error by sending something that can't be marshaled)
	invalidReq := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      make(chan int), // channels can't be marshaled to JSON
		Method:  "test.async.method",
		Params:  map[string]interface{}{},
	}

	err = transport.SendRequestAsync(ctx, invalidReq, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal")
}

func TestWebSocketTransportSetNotificationHandler(t *testing.T) {
	// Create a test server that sends notifications
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Channel to signal when the server should send a notification
	sendNotification := make(chan struct{}, 1)

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()

			// Loop to send notifications when requested
			for {
				select {
				case <-sendNotification:
					// Create a notification
					notification := map[string]interface{}{
						"jsonrpc": "2.0",
						"method":  "test.notification",
						"params":  map[string]interface{}{"event": "test-event"},
					}

					notificationBytes, _ := json.Marshal(notification)

					// Add newline to match expected format
					notificationBytes = append(notificationBytes, '\n')

					// Send the notification
					err = conn.WriteMessage(websocket.TextMessage, notificationBytes)
					if err != nil {
						t.Logf("Failed to send notification: %v", err)
						return
					}

				case <-r.Context().Done():
					return
				}
			}
		}
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()
	logger := logx.NewDefaultLogger()

	// Create and connect the transport
	transport, err := NewWebSocketTransport(wsURL, "/mcp", logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Channel to receive notifications
	notifyCh := make(chan *protocol.JSONRPCNotification, 5)

	// Set the notification handler
	transport.SetNotificationHandler(func(notification *protocol.JSONRPCNotification) error {
		notifyCh <- notification
		return nil
	})

	// Trigger a notification from the server
	sendNotification <- struct{}{}

	// Wait for the notification
	select {
	case notification := <-notifyCh:
		require.NotNil(t, notification)
		assert.Equal(t, "2.0", notification.JSONRPC)
		assert.Equal(t, "test.notification", notification.Method)

		// Check the params
		params, ok := notification.Params.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test-event", params["event"])

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}

	// Test changing the notification handler
	newNotifyCh := make(chan *protocol.JSONRPCNotification, 5)

	// Set a new notification handler
	transport.SetNotificationHandler(func(notification *protocol.JSONRPCNotification) error {
		newNotifyCh <- notification
		return nil
	})

	// Trigger another notification
	sendNotification <- struct{}{}

	// Wait for the notification on the new channel
	select {
	case notification := <-newNotifyCh:
		require.NotNil(t, notification)
		assert.Equal(t, "test.notification", notification.Method)

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for notification on new handler")
	}

	// Ensure the old channel didn't receive anything
	select {
	case notification := <-notifyCh:
		t.Fatalf("Unexpected notification received on old handler: %v", notification)
	case <-time.After(100 * time.Millisecond):
		// This is expected, no notification should be received on the old channel
	}
}

func TestWebSocketTransportGetTransportType(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a WebSocket transport
	transport, err := NewWebSocketTransport("ws://example.com", "/mcp", logger)
	require.NoError(t, err)

	// Check the transport type
	transportType := transport.GetTransportType()
	assert.Equal(t, TransportTypeWebSocket, transportType)
}

func TestWebSocketTransportGetTransportInfo(t *testing.T) {
	baseURL := "ws://example.com"
	basePath := "/mcp"
	logger := logx.NewDefaultLogger()

	// Create a WebSocket transport
	transport, err := NewWebSocketTransport(baseURL, basePath, logger)
	require.NoError(t, err)

	// Check the transport info
	info := transport.GetTransportInfo()
	require.NotNil(t, info)

	// Verify the expected fields
	assert.Contains(t, info, "baseURL")
	assert.Contains(t, info, "basePath")
	assert.Contains(t, info, "connected")

	assert.Contains(t, info["baseURL"], baseURL)
	assert.Equal(t, basePath, info["basePath"])
	assert.Equal(t, false, info["connected"])

	// Test with different base path
	customPath := "/custom/path"
	transportCustomPath, err := NewWebSocketTransport(baseURL, customPath, logger)
	require.NoError(t, err)

	infoCustomPath := transportCustomPath.GetTransportInfo()
	assert.Equal(t, customPath, infoCustomPath["basePath"])
}

func TestWebSocketTransportReceiveLoop(t *testing.T) {
	// The receiveLoop method is internal and tested indirectly through the SendRequest
	// and notification handler tests. It's difficult to test directly without exposing
	// implementation details.
	t.Skip("Tested indirectly through other test cases")
}
