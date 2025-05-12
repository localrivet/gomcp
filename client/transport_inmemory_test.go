package client

import (
	"context"
	"testing"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryTransport(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a minimal server handler for testing
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		}
	}

	// Create an in-memory transport
	transport, err := NewInMemoryTransport(logger, serverHandler)

	// Check if transport was created successfully
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Check transport type
	assert.Equal(t, TransportTypeInMemory, transport.GetTransportType())

	// Check initial connection state
	assert.False(t, transport.IsConnected())
}

func TestInMemoryTransportConnect(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a minimal server handler for testing
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		}
	}

	// Create an in-memory transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Connect should succeed
	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)

	// Transport should now be connected
	assert.True(t, transport.IsConnected())

	// Connecting again should fail
	err = transport.Connect(ctx)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestInMemoryTransportClose(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a minimal server handler for testing
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		}
	}

	// Create and connect transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)

	// Close the transport
	err = transport.Close()
	require.NoError(t, err)

	// Transport should now be disconnected
	assert.False(t, transport.IsConnected())

	// Closing again should succeed (idempotent)
	err = transport.Close()
	assert.NoError(t, err)
}

func TestInMemoryTransportIsConnected(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a minimal server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		}
	}

	// Create transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Should not be connected initially
	assert.False(t, transport.IsConnected())

	// Connect
	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)

	// Should be connected after Connect
	assert.True(t, transport.IsConnected())

	// Close
	err = transport.Close()
	require.NoError(t, err)

	// Should not be connected after Close
	assert.False(t, transport.IsConnected())
}

func TestInMemoryTransportSendRequest(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a server handler that echoes the request method and ID
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"method": req.Method},
		}
	}

	// Create and connect transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)

	// Send a test request
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-id",
		Method:  "test-method",
		Params:  nil,
	}

	resp, err := transport.SendRequest(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "test-id", resp.ID)

	// Test sending when not connected
	err = transport.Close()
	require.NoError(t, err)

	_, err = transport.SendRequest(ctx, req)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestInMemoryTransportSendRequestAsync(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a server handler that echoes the request method
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"method": req.Method},
		}
	}

	// Create and connect transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Create a response channel
	responseCh := make(chan *protocol.JSONRPCResponse, 1)

	// Send a test request async
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "async-test-id",
		Method:  "async-test-method",
		Params:  nil,
	}

	err = transport.SendRequestAsync(ctx, req, responseCh)
	require.NoError(t, err)

	// Wait for the response
	var resp *protocol.JSONRPCResponse
	select {
	case resp = <-responseCh:
		// Got response
	case <-ctx.Done():
		t.Fatal("Timed out waiting for async response")
	}

	// Verify response
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "async-test-id", resp.ID)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "async-test-method", result["method"])

	// Test sending when not connected
	disconnectedTransport, _ := NewInMemoryTransport(logger, serverHandler)
	err = disconnectedTransport.SendRequestAsync(ctx, req, responseCh)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))

	// Test sending with nil response channel (should not error)
	err = transport.SendRequestAsync(ctx, req, nil)
	assert.NoError(t, err)
}

func TestInMemoryTransportSetNotificationHandler(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a minimal server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		}
	}

	// Create and connect transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Channel to receive notifications
	notificationCh := make(chan *protocol.JSONRPCNotification, 5)

	// Set notification handler
	transport.SetNotificationHandler(func(notification *protocol.JSONRPCNotification) error {
		notificationCh <- notification
		return nil
	})

	// Access the unexported method via type assertion
	inmemTransport, ok := transport.(*inMemoryTransport)
	require.True(t, ok, "Expected inMemoryTransport")

	// Send a notification using the unexported method
	method := "test.notification"
	params := map[string]interface{}{"event": "test-event"}
	err = inmemTransport.SendNotification(method, params)
	require.NoError(t, err)

	// Wait for the notification
	var notification *protocol.JSONRPCNotification
	select {
	case notification = <-notificationCh:
		// Got notification
	case <-ctx.Done():
		t.Fatal("Timed out waiting for notification")
	}

	// Verify notification
	require.NotNil(t, notification)
	assert.Equal(t, "2.0", notification.JSONRPC)
	assert.Equal(t, method, notification.Method)
	notifyParams, ok := notification.Params.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-event", notifyParams["event"])

	// Test changing the notification handler
	newNotificationCh := make(chan *protocol.JSONRPCNotification, 5)
	transport.SetNotificationHandler(func(notification *protocol.JSONRPCNotification) error {
		newNotificationCh <- notification
		return nil
	})

	// Send another notification
	err = inmemTransport.SendNotification("test.notification.2", map[string]interface{}{"event": "test-event-2"})
	require.NoError(t, err)

	// Wait for the notification on the new channel
	select {
	case notification = <-newNotificationCh:
		// Got notification on new handler
		assert.Equal(t, "test.notification.2", notification.Method)
	case <-ctx.Done():
		t.Fatal("Timed out waiting for notification on new handler")
	}

	// Verify old handler doesn't receive it
	select {
	case notification = <-notificationCh:
		t.Fatal("Notification received on old handler")
	default:
		// Expected - old handler shouldn't receive notifications
	}
}

func TestInMemoryTransportGetTransportType(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create transport with no handler
	transport, err := NewInMemoryTransport(logger, nil)
	require.NoError(t, err)

	// Check transport type
	transportType := transport.GetTransportType()
	assert.Equal(t, TransportTypeInMemory, transportType)
}

func TestInMemoryTransportGetTransportInfo(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create a minimal server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  nil,
		}
	}

	// Create transport
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Get transport info
	info := transport.GetTransportInfo()

	// Verify info
	assert.NotNil(t, info)
	assert.Contains(t, info, "connected")
	assert.Contains(t, info, "hasHandler")
	assert.Contains(t, info, "serverType")
}

func TestInMemoryTransportSendNotification(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create transport with no handler
	transport, err := NewInMemoryTransport(logger, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Access the unexported SendNotification method via type assertion
	inmemTransport, ok := transport.(*inMemoryTransport)
	require.True(t, ok, "Expected inMemoryTransport")

	// Test 1: Send notification with no handler
	// Should buffer the notification
	err = inmemTransport.SendNotification("test.notification", map[string]interface{}{"event": "test-event"})
	require.NoError(t, err)

	// Set up a notification handler to receive buffered notifications
	notificationCh := make(chan *protocol.JSONRPCNotification, 5)
	notificationReceived := make(chan struct{})

	transport.SetNotificationHandler(func(notification *protocol.JSONRPCNotification) error {
		notificationCh <- notification
		notificationReceived <- struct{}{}
		return nil
	})

	// Wait for the buffered notification to be delivered
	select {
	case <-notificationReceived:
		// Notification was received
	case <-ctx.Done():
		t.Fatal("Timed out waiting for buffered notification")
	}

	// Check the notification
	notification := <-notificationCh
	assert.Equal(t, "test.notification", notification.Method)
	params, ok := notification.Params.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-event", params["event"])

	// Test 2: Send notification with handler
	// Should be delivered immediately
	err = inmemTransport.SendNotification("test.notification.2", map[string]interface{}{"event": "test-event-2"})
	require.NoError(t, err)

	// Wait for the notification
	select {
	case <-notificationReceived:
		// Notification was received
	case <-ctx.Done():
		t.Fatal("Timed out waiting for direct notification")
	}

	// Check the notification
	notification = <-notificationCh
	assert.Equal(t, "test.notification.2", notification.Method)
	params, ok = notification.Params.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-event-2", params["event"])

	// Test 3: Send notification when disconnected
	err = transport.Close()
	require.NoError(t, err)

	// Should buffer the notification
	err = inmemTransport.SendNotification("test.notification.3", map[string]interface{}{"event": "test-event-3"})
	require.NoError(t, err)
}

func TestDefaultServerHandler(t *testing.T) {
	// Create test tools, resources, and prompts
	tools := []protocol.Tool{
		{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: protocol.ToolInputSchema{
				Type: "object",
			},
		},
	}

	resources := []protocol.Resource{
		{
			URI:         "test-resource",
			Description: "A test resource",
		},
	}

	prompts := []protocol.Prompt{
		{
			URI:         "test-prompt",
			Description: "A test prompt",
		},
	}

	// Create default server handler
	handler := DefaultServerHandler(tools, resources, prompts)

	// Test initialize request
	initReq := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "init-id",
		Method:  "initialize",
		Params: protocol.InitializeRequestParams{
			ProtocolVersion: protocol.CurrentProtocolVersion,
		},
	}

	initResp := handler(initReq)
	require.NotNil(t, initResp)
	assert.Equal(t, "init-id", initResp.ID)
	assert.Nil(t, initResp.Error)

	// Test listTools request
	toolsReq := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "tools-id",
		Method:  protocol.MethodListTools,
	}

	toolsResp := handler(toolsReq)
	require.NotNil(t, toolsResp)
	assert.Equal(t, "tools-id", toolsResp.ID)
	assert.Nil(t, toolsResp.Error)
}
