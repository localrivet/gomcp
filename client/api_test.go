package client

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientConnect(t *testing.T) {
	// Create a server handler that properly handles initialization
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			// Correctly format the initialize result as JSON
			result := protocol.InitializeResult{
				ProtocolVersion: protocol.CurrentProtocolVersion,
				ServerInfo: protocol.Implementation{
					Name:    "Test Server",
					Version: "1.0.0",
				},
				Capabilities: protocol.ServerCapabilities{},
			}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				t.Logf("Failed to marshal initialize result: %v", err)
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32603,
						Message: "Internal error: " + err.Error(),
					},
				}
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultBytes),
			}
		} else if req.Method == "initialized" {
			// Handle the initialized notification
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage("null"),
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Test 1: Basic successful connection
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Test 2: Connection when already connected
	err = client.Connect(ctx)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))

	// Test 3: Connection with a failing transport
	failingTransport, err := NewInMemoryTransport(logger, func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32603,
				Message: "Internal error",
			},
		}
	})
	require.NoError(t, err)

	failingClient, err := newClientWithTransport(config, failingTransport)
	require.NoError(t, err)

	err = failingClient.Connect(ctx)
	assert.Error(t, err)
	assert.False(t, failingClient.IsConnected())

	// Test 4: Connection with a cancelled context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = client.Close() // Close previous connection
	require.NoError(t, err)

	err = client.Connect(cancelCtx)
	assert.Error(t, err)
	assert.False(t, client.IsConnected())
}

func TestClientClose(t *testing.T) {
	// Create a server handler that properly handles initialization
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			// Correctly format the initialize result as JSON
			result := protocol.InitializeResult{
				ProtocolVersion: protocol.CurrentProtocolVersion,
				ServerInfo: protocol.Implementation{
					Name:    "Test Server",
					Version: "1.0.0",
				},
				Capabilities: protocol.ServerCapabilities{},
			}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				t.Logf("Failed to marshal initialize result: %v", err)
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32603,
						Message: "Internal error: " + err.Error(),
					},
				}
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultBytes),
			}
		} else if req.Method == "initialized" {
			// Handle the initialized notification
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage("null"),
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Test 1: Close when not connected
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// Test 2: Close after connecting
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// Test 3: Close multiple times should be idempotent
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestClientIsConnected(t *testing.T) {
	// Create a server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			// Correctly format the initialize result as JSON
			result := protocol.InitializeResult{
				ProtocolVersion: protocol.CurrentProtocolVersion,
				ServerInfo: protocol.Implementation{
					Name:    "Test Server",
					Version: "1.0.0",
				},
				Capabilities: protocol.ServerCapabilities{},
			}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				t.Logf("Failed to marshal initialize result: %v", err)
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32603,
						Message: "Internal error: " + err.Error(),
					},
				}
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultBytes),
			}
		} else if req.Method == "initialized" {
			// Handle the initialized notification
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage("null"),
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Test 1: Should be false initially
	assert.False(t, client.IsConnected())

	// Test 2: Should be true after connecting
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Test 3: Should be false after closing
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// Test 4: Should reflect transport status
	// Get the client implementation to simulate transport disconnect
	clientImpl, ok := client.(*clientImpl)
	require.True(t, ok, "Expected *clientImpl")

	// Force the client to think it's connected, but the transport is still disconnected
	clientImpl.connectionState = connectionStateConnected
	assert.False(t, client.IsConnected(), "IsConnected should check transport status")
}

func TestClientRun(t *testing.T) {
	// Create a server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			// Correctly format the initialize result as JSON
			result := protocol.InitializeResult{
				ProtocolVersion: protocol.CurrentProtocolVersion,
				ServerInfo: protocol.Implementation{
					Name:    "Test Server",
					Version: "1.0.0",
				},
				Capabilities: protocol.ServerCapabilities{},
			}

			resultBytes, err := json.Marshal(result)
			if err != nil {
				t.Logf("Failed to marshal initialize result: %v", err)
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32603,
						Message: "Internal error: " + err.Error(),
					},
				}
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultBytes),
			}
		} else if req.Method == "initialized" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage("null"),
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Initialize with a retry strategy for testing reconnection
	retryStrategy := NewExponentialBackoff(10*time.Millisecond, 100*time.Millisecond, 3)

	// Create configuration with retry
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
		RetryStrategy:            retryStrategy,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Test 1: Run with auto-connect and short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run the client in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- client.Run(ctx)
	}()

	// Wait for the context to be cancelled or an error
	select {
	case err := <-errChan:
		// Should get context cancelled error or nil
		assert.True(t, err == nil || err == context.DeadlineExceeded, "Expected nil or context deadline error")
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Run did not return after context cancellation")
	}

	// Test 2: Run should handle connection failures
	// Create a transport that fails after initial connection
	var transportConnected bool
	failingTransport := &testTransport{
		connect: func(ctx context.Context) error {
			if !transportConnected {
				transportConnected = true
				return nil
			}
			return fmt.Errorf("simulated connection failure")
		},
		isConnected: func() bool {
			return transportConnected
		},
		close: func() error {
			transportConnected = false
			return nil
		},
		sendRequest: func(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
			if req.Method == "initialize" {
				// Correctly format the initialize result as JSON
				result := protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Transport",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				}

				resultBytes, err := json.Marshal(result)
				if err != nil {
					return nil, err
				}

				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  json.RawMessage(resultBytes),
				}, nil
			}
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage("null"),
			}, nil
		},
		setNotificationHandler: func(handler NotificationHandler) {},
	}

	failingClient, err := newClientWithTransport(config, failingTransport)
	require.NoError(t, err)

	// Short context for quick test
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer shortCancel()

	// Run should handle reconnection attempts
	err = failingClient.Run(shortCtx)
	// Either context deadline or nil error (clean shutdown)
	assert.True(t, err == nil || err == context.DeadlineExceeded, "Expected nil or context deadline error")
}

// testTransport is a minimal transport implementation for testing
type testTransport struct {
	connect                func(ctx context.Context) error
	isConnected            func() bool
	close                  func() error
	sendRequest            func(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error)
	setNotificationHandler func(handler NotificationHandler)
}

func (t *testTransport) Connect(ctx context.Context) error {
	return t.connect(ctx)
}

func (t *testTransport) IsConnected() bool {
	return t.isConnected()
}

func (t *testTransport) Close() error {
	return t.close()
}

func (t *testTransport) SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	return t.sendRequest(ctx, req)
}

func (t *testTransport) SetNotificationHandler(handler NotificationHandler) {
	t.setNotificationHandler(handler)
}

func (t *testTransport) SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error {
	// Simple implementation - just send the response to the channel
	resp, err := t.sendRequest(ctx, req)
	if err != nil {
		return err
	}
	responseCh <- resp
	return nil
}

func (t *testTransport) GetTransportType() TransportType {
	return TransportTypeInMemory
}

func (t *testTransport) GetTransportInfo() map[string]interface{} {
	return map[string]interface{}{
		"type": "test",
	}
}

func TestClientListTools(t *testing.T) {
	// Create test tools
	expectedTools := []protocol.Tool{
		{
			Name:        "test-tool1",
			Description: "First test tool",
			InputSchema: protocol.ToolInputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDetail{
					"param1": {
						Type:        "string",
						Description: "First parameter",
					},
				},
			},
		},
		{
			Name:        "test-tool2",
			Description: "Second test tool",
			InputSchema: protocol.ToolInputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDetail{
					"param2": {
						Type:        "number",
						Description: "Second parameter",
					},
				},
			},
		},
	}

	// Create a server handler that returns our test tools
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == protocol.MethodListTools {
			resp := &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"tools": expectedTools,
				},
			}
			return resp
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    protocol.CodeMethodNotFound,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}

	// Use test client instead of regular client
	client, err := newTestClient(config, transport)
	require.NoError(t, err)

	// Print the client implementation type
	t.Logf("Client type: %T", client)

	// Mark client as connected (bypass normal initialization)
	clientImpl, ok := client.(*clientImpl)
	require.True(t, ok, "Expected *clientImpl")
	clientImpl.connectionState = connectionStateConnected

	// Connect the transport (but skip the client's initialization)
	ctx := context.Background()
	err = transport.Connect(ctx)
	require.NoError(t, err)

	// Call ListTools and check results
	t.Logf("Calling ListTools")
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Logf("ListTools error: %v", err)
		// Try with a direct SendRequest to debug
		resp, sendErr := client.SendRequest(ctx, protocol.MethodListTools, nil)
		if sendErr != nil {
			t.Logf("SendRequest error: %v", sendErr)
		} else {
			// Try parsing the response ourselves
			t.Logf("Raw response received: %+v", resp)
			if resp.Result != nil {
				t.Logf("Result type: %T", resp.Result)

				var resultBytes []byte

				// Handle different result types
				switch r := resp.Result.(type) {
				case json.RawMessage:
					resultBytes = r
				case []byte:
					resultBytes = r
				default:
					var err error
					resultBytes, err = json.Marshal(r)
					if err != nil {
						t.Logf("Failed to marshal result: %v", err)
					}
				}

				if len(resultBytes) > 0 {
					t.Logf("Result JSON: %s", string(resultBytes))

					// Try direct unmarshaling
					var tools []protocol.Tool
					unmarshalErr := json.Unmarshal(resultBytes, &tools)
					if unmarshalErr != nil {
						t.Logf("Unmarshal error: %v", unmarshalErr)
					} else {
						t.Logf("Successfully unmarshaled %d tools directly", len(tools))
					}
				}
			}
		}
	}
	require.NoError(t, err)
	require.NotNil(t, tools)

	// Verify the returned tools match the expected values
	require.Len(t, tools, len(expectedTools))
	for i, tool := range tools {
		assert.Equal(t, expectedTools[i].Name, tool.Name)
		assert.Equal(t, expectedTools[i].Description, tool.Description)
		assert.Equal(t, expectedTools[i].InputSchema.Type, tool.InputSchema.Type)

		// Check first property in each tool
		if i == 0 {
			prop := tool.InputSchema.Properties["param1"]
			assert.Equal(t, "string", prop.Type)
			assert.Equal(t, "First parameter", prop.Description)
		} else if i == 1 {
			prop := tool.InputSchema.Properties["param2"]
			assert.Equal(t, "number", prop.Type)
			assert.Equal(t, "Second parameter", prop.Description)
		}
	}

	// Test error case: client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.ListTools(ctx)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestClientCallTool(t *testing.T) {
	// Create a server handler that handles tool calls
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		} else if req.Method == "callTool" {
			// Parse the parameters to verify they are correct
			var params protocol.CallToolRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32700,
						Message: "Parse error: " + err.Error(),
					},
				}
			}

			// Return a successful tool call result
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.CallToolResult{
					ToolCallID: params.ToolCall.ID,
					Output:     json.RawMessage(`{"content": [{"type": "text", "text": "Tool execution result"}]}`),
				},
			}
		}

		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test: Basic tool call
	args := map[string]interface{}{
		"param1": "value1",
	}

	result, err := client.CallTool(ctx, "test-tool", args, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result, 1)

	// Type assert to access fields
	textContent, ok := result[0].(protocol.TextContent)
	require.True(t, ok, "Expected TextContent type")
	assert.Equal(t, "text", textContent.Type)
	assert.Equal(t, "Tool execution result", textContent.Text)

	// Test error case: client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.CallTool(ctx, "test-tool", args, nil)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestClientListResources(t *testing.T) {
	// Create test resources
	expectedResources := []protocol.Resource{
		{
			URI:         "file:///test/resource1.txt",
			Kind:        string(protocol.ResourceKindFile),
			Name:        "First test resource",
			Description: "A text file resource",
		},
		{
			URI:         "file:///test/resource2.md",
			Kind:        string(protocol.ResourceKindFile),
			Name:        "Second test resource",
			Description: "A markdown file resource",
		},
	}

	// Create a server handler that returns our test resources
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "resources/list" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.ListResourcesResult{
					Resources: expectedResources,
				},
			}
		} else if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Call ListResources and check results
	resources, err := client.ListResources(ctx)
	require.NoError(t, err)
	require.NotNil(t, resources)

	// Verify the returned resources match the expected values
	require.Len(t, resources, len(expectedResources))
	for i, resource := range resources {
		assert.Equal(t, expectedResources[i].URI, resource.URI)
		assert.Equal(t, expectedResources[i].Kind, resource.Kind)
		assert.Equal(t, expectedResources[i].Name, resource.Name)
		assert.Equal(t, expectedResources[i].Description, resource.Description)
	}

	// Test error case: client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.ListResources(ctx)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestClientReadResource(t *testing.T) {
	// Create test resource content
	expectedContents := []protocol.TextResourceContents{
		{
			ContentType: string(protocol.ResourceKindText),
			Text:        "This is the content of the resource",
		},
	}

	// Create a server handler for resource reading
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "resources/read" {
			// Get the URI from the params
			var params protocol.ReadResourceRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32700,
						Message: "Parse error: " + err.Error(),
					},
				}
			}

			// Verify the URI
			if params.URI != "file:///test/resource1.txt" {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    protocol.CodeMCPResourceNotFound,
						Message: "Resource not found: " + params.URI,
					},
				}
			}

			// Return the resource content
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.ReadResourceResult{
					Resource: protocol.Resource{
						URI:  "file:///test/resource1.txt",
						Kind: string(protocol.ResourceKindFile),
					},
					Contents: []protocol.ResourceContents{expectedContents[0]},
				},
			}
		} else if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test 1: Read an existing resource
	contents, err := client.ReadResource(ctx, "file:///test/resource1.txt")
	require.NoError(t, err)
	require.NotNil(t, contents)
	require.Len(t, contents, 1)

	// Type assert to get the concrete type
	textContent, ok := contents[0].(protocol.TextResourceContents)
	require.True(t, ok, "Expected TextResourceContents type")
	assert.Equal(t, string(protocol.ResourceKindText), textContent.ContentType)
	assert.Equal(t, "This is the content of the resource", textContent.Text)

	// Test 2: Read a non-existent resource
	_, err = client.ReadResource(ctx, "file:///non-existent.txt")
	assert.Error(t, err)
	// Check the error is a resource not found error (since we don't have a specific function for this)
	mcpErr, ok := err.(*ClientError)
	require.True(t, ok, "Expected ClientError type")
	assert.Equal(t, int(protocol.CodeMCPResourceNotFound), mcpErr.Code)

	// Test 3: Error case - client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.ReadResource(ctx, "file:///test/resource1.txt")
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestClientListPrompts(t *testing.T) {
	// Create test prompts
	expectedPrompts := []protocol.Prompt{
		{
			URI:         "test-prompt1",
			Name:        "First test prompt",
			Description: "A simple test prompt",
			Arguments: []protocol.PromptArgument{
				{
					Name:        "arg1",
					Description: "First argument",
					Type:        "string",
					Required:    true,
				},
			},
			Messages: []protocol.PromptMessage{
				{
					Role: "user",
					Content: protocol.TextContent{
						Type: "text",
						Text: "You are a helpful assistant",
					},
				},
			},
		},
		{
			URI:         "test-prompt2",
			Name:        "Second test prompt",
			Description: "Another test prompt",
			Messages: []protocol.PromptMessage{
				{
					Role: "user",
					Content: protocol.TextContent{
						Type: "text",
						Text: "Hello world",
					},
				},
			},
		},
	}

	// Create a server handler that returns our test prompts
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "prompts/list" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.ListPromptsResult{
					Prompts: expectedPrompts,
				},
			}
		} else if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Call ListPrompts and check results
	prompts, err := client.ListPrompts(ctx)
	require.NoError(t, err)
	require.NotNil(t, prompts)

	// Verify the returned prompts match the expected values
	require.Len(t, prompts, len(expectedPrompts))
	for i, prompt := range prompts {
		assert.Equal(t, expectedPrompts[i].URI, prompt.URI)
		assert.Equal(t, expectedPrompts[i].Name, prompt.Name)
		assert.Equal(t, expectedPrompts[i].Description, prompt.Description)

		// Check if arguments match
		if len(expectedPrompts[i].Arguments) > 0 {
			require.Len(t, prompt.Arguments, len(expectedPrompts[i].Arguments))
			assert.Equal(t, expectedPrompts[i].Arguments[0].Name, prompt.Arguments[0].Name)
			assert.Equal(t, expectedPrompts[i].Arguments[0].Type, prompt.Arguments[0].Type)
		}

		// Check if messages match
		require.Len(t, prompt.Messages, len(expectedPrompts[i].Messages))
		assert.Equal(t, expectedPrompts[i].Messages[0].Role, prompt.Messages[0].Role)

		// Check content
		textContent, ok := expectedPrompts[i].Messages[0].Content.(protocol.TextContent)
		require.True(t, ok)

		actualTextContent, ok := prompt.Messages[0].Content.(protocol.TextContent)
		require.True(t, ok)

		assert.Equal(t, textContent.Text, actualTextContent.Text)
	}

	// Test error case: client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.ListPrompts(ctx)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestClientGetPrompt(t *testing.T) {
	// Create test prompt
	// testPrompt := protocol.Prompt{
	// 	URI:         "test-prompt",
	// 	Title:       "Test prompt",
	// 	Description: "A test prompt with arguments",
	// 	Arguments: []protocol.PromptArgument{
	// 		{
	// 			Name:        "name",
	// 			Description: "User's name",
	// 			Type:        "string",
	// 			Required:    true,
	// 		},
	// 	},
	// 	Messages: []protocol.PromptMessage{
	// 		{
	// 			Role: "system",
	// 			Content: protocol.TextContent{
	// 				Type: "text",
	// 				Text: "You are a helpful assistant",
	// 			},
	// 		},
	// 		{
	// 			Role: "user",
	// 			Content: []protocol.Content{
	// 				protocol.TextContent{
	// 					Type: "text",
	// 					Text: "Hello, my name is {{name}}",
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// Expected messages after argument substitution
	expectedMessages := []protocol.PromptMessage{
		{
			Role: "user",
			Content: protocol.TextContent{
				Type: "text",
				Text: "You are a helpful assistant",
			},
		},
		{
			Role: "user",
			Content: protocol.TextContent{
				Type: "text",
				Text: "Hello, my name is John",
			},
		},
	}

	// Create a server handler for prompt retrieval
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "prompts/get" {
			// Get the prompt name and arguments from the params
			var params protocol.GetPromptRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32700,
						Message: "Parse error: " + err.Error(),
					},
				}
			}

			// Verify the prompt URI
			if params.URI != "test-prompt" {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32602,
						Message: "Invalid prompt URI: " + params.URI,
					},
				}
			}

			// Check if required arguments are provided
			nameVal, ok := params.Arguments["name"]
			if !ok || nameVal != "John" {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32602,
						Message: "Missing or invalid required argument: name",
					},
				}
			}

			// Return the prompt messages with substituted arguments
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  expectedMessages,
			}
		} else if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test 1: Get a prompt with valid arguments
	args := map[string]interface{}{
		"name": "John",
	}

	messages, err := client.GetPrompt(ctx, "test-prompt", args)
	require.NoError(t, err)
	require.NotNil(t, messages)
	require.Len(t, messages, 2)

	// Verify the returned messages match the expected values
	assert.Equal(t, expectedMessages[0].Role, messages[0].Role)
	assert.Equal(t, expectedMessages[1].Role, messages[1].Role)

	// Check content of the first message
	firstTextContent, ok := messages[0].Content.(protocol.TextContent)
	require.True(t, ok, "Expected TextContent type")
	assert.Equal(t, "You are a helpful assistant", firstTextContent.Text)

	// Check content of the second message (with substituted argument)
	secondTextContent, ok := messages[1].Content.(protocol.TextContent)
	require.True(t, ok, "Expected TextContent type")
	assert.Equal(t, "Hello, my name is John", secondTextContent.Text)

	// Test 2: Error case - missing required argument
	invalidArgs := map[string]interface{}{
		"wrongName": "John",
	}

	_, err = client.GetPrompt(ctx, "test-prompt", invalidArgs)
	assert.Error(t, err)

	// Test 3: Error case - client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.GetPrompt(ctx, "test-prompt", args)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestClientServerInfo(t *testing.T) {
	// Create a server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{
						Logging: &struct{}{},
					},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Test 1: No server info before connecting
	serverInfo := client.ServerInfo()
	assert.Empty(t, serverInfo.Name)
	assert.Empty(t, serverInfo.Version)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test 2: Get server info after connecting
	serverInfo = client.ServerInfo()
	assert.Equal(t, "Test Server", serverInfo.Name)
	assert.Equal(t, "1.0.0", serverInfo.Version)

	// Close the connection
	err = client.Close()
	require.NoError(t, err)
}

func TestClientServerCapabilities(t *testing.T) {
	// Create a server handler with specific capabilities
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{
						Logging: &struct{}{},
						Resources: &struct {
							Subscribe   bool `json:"subscribe,omitempty"`
							ListChanged bool `json:"listChanged,omitempty"`
						}{
							Subscribe:   true,
							ListChanged: true,
						},
						Tools: &struct {
							ListChanged bool `json:"listChanged,omitempty"`
						}{
							ListChanged: true,
						},
					},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Test 1: Empty capabilities before connecting
	capabilities := client.ServerCapabilities()
	assert.Nil(t, capabilities.Logging)
	assert.Nil(t, capabilities.Resources)
	assert.Nil(t, capabilities.Tools)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test 2: Get capabilities after connecting
	capabilities = client.ServerCapabilities()

	// Check logging capability
	assert.NotNil(t, capabilities.Logging)

	// Check resources capability
	assert.NotNil(t, capabilities.Resources)
	assert.True(t, capabilities.Resources.Subscribe)
	assert.True(t, capabilities.Resources.ListChanged)

	// Check tools capability
	assert.NotNil(t, capabilities.Tools)
	assert.True(t, capabilities.Tools.ListChanged)

	// Close the connection
	err = client.Close()
	require.NoError(t, err)
}

func TestClientSendRequest(t *testing.T) {
	// Create a server handler
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		} else if req.Method == "test/echo" {
			// Echo back the parameters
			var params map[string]interface{}
			if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
				return &protocol.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &protocol.ErrorPayload{
						Code:    -32700,
						Message: "Parse error: " + err.Error(),
					},
				}
			}

			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  params,
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test 1: Send a custom request
	params := map[string]interface{}{
		"message": "Hello, world!",
		"number":  42,
	}

	resp, err := client.SendRequest(ctx, "test/echo", params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify the response matches the expected values
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.ID)
	assert.Nil(t, resp.Error)

	// Check the result content
	var result map[string]interface{}
	err = protocol.UnmarshalPayload(resp.Result, &result)
	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", result["message"])
	assert.Equal(t, float64(42), result["number"]) // JSON numbers are float64

	// Test 2: Send a request to a non-existent method
	resp, err = client.SendRequest(ctx, "test/nonexistent", nil)
	require.NoError(t, err) // The request itself succeeded
	require.NotNil(t, resp)

	// Check the error response
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, int(resp.Error.Code))
	assert.Equal(t, "Method not found", resp.Error.Message)

	// Test 3: Error case - client not connected
	err = client.Close()
	require.NoError(t, err)

	_, err = client.SendRequest(ctx, "test/echo", params)
	assert.Error(t, err)
	assert.True(t, IsConnectionError(err))
}

func TestIDGeneration(t *testing.T) {
	// Test generateID function
	id1 := generateID()

	// Small sleep to ensure different timestamps
	time.Sleep(1 * time.Millisecond)

	id2 := generateID()

	// IDs should not be empty
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)

	// IDs should be different
	assert.NotEqual(t, id1, id2)

	// IDs should match expected format: id-{timestamp}
	idPattern := regexp.MustCompile(`^id-\d+$`)
	assert.True(t, idPattern.MatchString(id1), "ID should match format id-{timestamp}")
	assert.True(t, idPattern.MatchString(id2), "ID should match format id-{timestamp}")

	// Test generateProgressID
	progressID1 := generateProgressID()

	// Small sleep to ensure different timestamps
	time.Sleep(1 * time.Millisecond)

	progressID2 := generateProgressID()

	// Progress IDs should not be empty
	assert.NotEmpty(t, progressID1)
	assert.NotEmpty(t, progressID2)

	// Progress IDs should be different
	assert.NotEqual(t, progressID1, progressID2)

	// Progress IDs should match expected format: progress-{timestamp}
	progressPattern := regexp.MustCompile(`^progress-\d+$`)
	assert.True(t, progressPattern.MatchString(progressID1), "Progress ID should match format progress-{timestamp}")
	assert.True(t, progressPattern.MatchString(progressID2), "Progress ID should match format progress-{timestamp}")
}

func TestProgressHandler(t *testing.T) {
	// Create a mock server response handler that can send notifications
	serverHandler := func(req *protocol.JSONRPCRequest) *protocol.JSONRPCResponse {
		if req.Method == "initialize" {
			return &protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: protocol.InitializeResult{
					ProtocolVersion: protocol.CurrentProtocolVersion,
					ServerInfo: protocol.Implementation{
						Name:    "Test Server",
						Version: "1.0.0",
					},
					Capabilities: protocol.ServerCapabilities{},
				},
			}
		}
		return &protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &protocol.ErrorPayload{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}

	// Create and configure the client with in-memory transport
	logger := logx.NewDefaultLogger()
	transport, err := NewInMemoryTransport(logger, serverHandler)
	require.NoError(t, err)

	inmemTransport, ok := transport.(*inMemoryTransport)
	require.True(t, ok, "Expected inMemoryTransport")

	// Create configuration
	config := &ClientConfig{
		Logger:                   logger,
		PreferredProtocolVersion: protocol.CurrentProtocolVersion,
	}
	client, err := newClientWithTransport(config, transport)
	require.NoError(t, err)

	// Connect to the in-memory server
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Set up a channel to receive progress notifications
	progressCh := make(chan protocol.ProgressParams, 10)
	client.OnProgress(func(params *protocol.ProgressParams) error {
		progressCh <- *params
		return nil
	})

	// Send a progress notification
	progressToken := "test-progress-1"
	progressParams := protocol.ProgressParams{
		Token:   progressToken,
		Value:   0,
		Message: protocol.StringPtr("Starting operation"),
	}
	inmemTransport.SendNotification(protocol.MethodProgress, progressParams)

	// Send another progress notification
	progressUpdateParams := protocol.ProgressParams{
		Token:   progressToken,
		Value:   50,
		Message: protocol.StringPtr("Operation in progress"),
	}
	inmemTransport.SendNotification(protocol.MethodProgress, progressUpdateParams)

	// Send a completion notification
	progressCompleteParams := protocol.ProgressParams{
		Token:   progressToken,
		Value:   100,
		Message: protocol.StringPtr("Operation completed"),
	}
	inmemTransport.SendNotification(protocol.MethodProgress, progressCompleteParams)

	// Collect and verify all progress events
	progressEvents := collectProgressEvents(t, progressCh, 3)
	require.Len(t, progressEvents, 3)

	// Verify the progress values
	assert.Equal(t, progressToken, progressEvents[0].Token)
	assert.Equal(t, 0, progressEvents[0].Value)
	assert.Equal(t, "Starting operation", *progressEvents[0].Message)

	assert.Equal(t, progressToken, progressEvents[1].Token)
	assert.Equal(t, 50, progressEvents[1].Value)
	assert.Equal(t, "Operation in progress", *progressEvents[1].Message)

	assert.Equal(t, progressToken, progressEvents[2].Token)
	assert.Equal(t, 100, progressEvents[2].Value)
	assert.Equal(t, "Operation completed", *progressEvents[2].Message)

	// Close the connection
	err = client.Close()
	require.NoError(t, err)
}

// Helper function to collect progress events with a timeout
func collectProgressEvents(t *testing.T, ch <-chan protocol.ProgressParams, expectedCount int) []protocol.ProgressParams {
	var events []protocol.ProgressParams
	timeout := time.After(2 * time.Second)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, event)
			if len(events) >= expectedCount {
				return events
			}
		case <-timeout:
			t.Logf("Timeout waiting for progress events. Got %d of %d expected events", len(events), expectedCount)
			return events
		}
	}
}

func inspectResult(t *testing.T, result interface{}) {
	// Try to see what type the result is
	t.Logf("Result type: %T", result)

	// Check if it's a map
	if mapVal, ok := result.(map[string]interface{}); ok {
		t.Logf("Result is a map with keys: %v", mapVal)
		return
	}

	// Check if it's a JSON RawMessage
	if rawVal, ok := result.(json.RawMessage); ok {
		t.Logf("Result is a JSON RawMessage with value: %s", string(rawVal))
		return
	}

	// Just print the value
	t.Logf("Result value: %v", result)
}
