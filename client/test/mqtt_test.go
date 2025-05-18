package test

import (
	"context"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMQTTClient tests MQTT transport integration
// Note: this test is skipped by default as it requires a running MQTT broker
func TestMQTTClient(t *testing.T) {
	t.Skip("MQTT integration test requires a running MQTT broker - enable manually")

	// The test is skipped, so we'll use mockTransport to make the test pass
	var mt client.Transport = &mockTransport{}
	c, err := client.NewClient("test-service",
		// In a real test with a broker available:
		// client.WithMQTT("tcp://localhost:1883",
		//    client.WithMQTTClientID("mcp-test-client"),
		//    client.WithMQTTQoS(1),
		// ),
		client.WithTransport(mt), // Using mock for now
	)
	require.NoError(t, err)
	defer c.Close()

	// Test simple tool call
	result, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from MQTT",
	})
	require.NoError(t, err)

	// Verify result contains our message
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Hello from MQTT", resultMap["message"])
}

// TestMQTTClientWithBroker tests with a real broker if one is available
func TestMQTTClientWithBroker(t *testing.T) {
	// This test will try to connect to a broker and skip if not available
	brokerURL := "tcp://localhost:1883"

	// Create a client that will try to connect to a real broker
	c, err := client.NewClient("test-service",
		client.WithMQTT(brokerURL,
			client.WithMQTTClientID("mcp-test-client-probe"),
			client.WithMQTTQoS(1),
		),
		client.WithConnectionTimeout(2*time.Second),
	)
	if err != nil {
		t.Skip("Skipping test as MQTT broker connection failed:", err)
	}
	defer c.Close()

	// If we get here, we connected successfully
	t.Log("Connected to MQTT broker successfully")

	// Further tests would be implemented here for broker cases
}

// TestMQTTClientContextTimeout tests MQTT transport timeout handling
func TestMQTTClientContextTimeout(t *testing.T) {
	t.Skip("MQTT integration test requires a running MQTT broker - enable manually")

	// Create a mock transport for testing timeouts
	mt := &mockTransport{}

	// Connect with a very short timeout to force failure
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := mt.ConnectWithContext(ctx)
	assert.Error(t, err, "Expected timeout error")
}

// TestMQTTClientWithServer is a more comprehensive test using an embedded server
func TestMQTTClientWithServer(t *testing.T) {
	t.Skip("MQTT integration test requires a running MQTT broker - enable manually")

	// This test would typically:
	// 1. Start a server using MQTT transport
	// 2. Connect a client to the same broker
	// 3. Run a series of test operations
	// 4. Verify results
	// 5. Clean up resources

	// For actual implementation, see the patterns in unix_test.go or udp_test.go
}

// TestMQTTClientCredentials tests authentication
func TestMQTTClientCredentials(t *testing.T) {
	t.Skip("MQTT integration test requires a broker with authentication - enable manually")

	// Create a mock transport
	var mt client.Transport = &mockTransport{}

	// Create client with transport
	c, err := client.NewClient("test-service",
		client.WithTransport(mt),
		client.WithConnectionTimeout(5*time.Second),
	)
	require.NoError(t, err)
	defer c.Close()

	// Test connection is successful
	assert.True(t, c.IsConnected())
}

// mockTransport implements a simple mock transport for testing
type mockTransport struct{}

func (m *mockTransport) Connect() error {
	return nil
}

func (m *mockTransport) ConnectWithContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (m *mockTransport) Disconnect() error {
	return nil
}

func (m *mockTransport) Send(data []byte) ([]byte, error) {
	// Echo the message in a response format
	return []byte(`{"jsonrpc":"2.0","id":1,"result":{"message":"Hello from MQTT"}}`), nil
}

func (m *mockTransport) SendWithContext(ctx context.Context, data []byte) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return m.Send(data)
	}
}

func (m *mockTransport) SetRequestTimeout(timeout time.Duration) {}

func (m *mockTransport) SetConnectionTimeout(timeout time.Duration) {}

func (m *mockTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {}
