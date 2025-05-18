package nats

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNATSWithRealBroker(t *testing.T) {
	// Try to connect to a real NATS broker, skip the test if it fails
	serverURL := "nats://localhost:4222"

	// Create server transport
	serverTransport := NewTransport(serverURL, true,
		WithClientID("test-server"),
		WithSubjectPrefix("mcp-test"),
	)

	// Try to initialize the transport, skip if it fails
	err := serverTransport.Initialize()
	if err != nil {
		t.Skipf("Skipping test as no NATS broker is available: %v", err)
		return
	}

	// Create a message handler
	messageReceived := make(chan []byte, 1)
	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		messageReceived <- message
		response := map[string]interface{}{
			"id":     1,
			"result": "success",
		}
		responseJSON, _ := json.Marshal(response)
		return responseJSON, nil
	})

	// Start the server transport
	err = serverTransport.Start()
	assert.NoError(t, err)
	defer serverTransport.Stop()

	// Create client transport
	clientTransport := NewTransport(serverURL, false,
		WithClientID("test-client"),
		WithSubjectPrefix("mcp-test"),
	)

	// Initialize the client transport
	err = clientTransport.Initialize()
	assert.NoError(t, err)
	defer clientTransport.Stop()

	// Start the client transport
	err = clientTransport.Start()
	assert.NoError(t, err)

	// Create a test message
	message := map[string]interface{}{
		"id":     1,
		"method": "test",
		"params": map[string]interface{}{
			"hello": "world",
		},
	}
	messageJSON, _ := json.Marshal(message)

	// Send the message from client to server
	err = clientTransport.Send(messageJSON)
	assert.NoError(t, err)

	// Wait for the message to be received by the server
	select {
	case received := <-messageReceived:
		// Validate the received message
		var receivedMsg map[string]interface{}
		err = json.Unmarshal(received, &receivedMsg)
		assert.NoError(t, err)
		assert.Equal(t, float64(1), receivedMsg["id"]) // JSON numbers are parsed as float64
		assert.Equal(t, "test", receivedMsg["method"])
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for server to receive message")
	}
}

func TestNATSFullE2E(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS broker
	t.Skip("NATS E2E test requires a running NATS broker - enable manually")

	// This test requires a running NATS broker on localhost:4222
	serverURL := "nats://localhost:4222"

	// Create server transport
	serverTransport := NewTransport(serverURL, true,
		WithClientID("e2e-server"),
		WithSubjectPrefix("mcp-e2e"),
	)

	// Initialize and start the server transport
	err := serverTransport.Initialize()
	assert.NoError(t, err)
	err = serverTransport.Start()
	assert.NoError(t, err)
	defer serverTransport.Stop()

	// Create a message handler
	messageReceived := make(chan []byte, 1)
	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		messageReceived <- message
		response := map[string]interface{}{
			"id":     1,
			"result": "success",
		}
		responseJSON, _ := json.Marshal(response)
		return responseJSON, nil
	})

	// Create client transport
	clientTransport := NewTransport(serverURL, false,
		WithClientID("e2e-client"),
		WithSubjectPrefix("mcp-e2e"),
	)

	// Initialize and start the client transport
	err = clientTransport.Initialize()
	assert.NoError(t, err)
	err = clientTransport.Start()
	assert.NoError(t, err)
	defer clientTransport.Stop()

	// Create a test message
	message := map[string]interface{}{
		"id":     1,
		"method": "test",
		"params": map[string]interface{}{
			"hello": "world",
		},
	}
	messageJSON, _ := json.Marshal(message)

	// Send the message from client to server
	err = clientTransport.Send(messageJSON)
	assert.NoError(t, err)

	// Wait for the message to be received by the server
	select {
	case received := <-messageReceived:
		// Validate the received message
		var receivedMsg map[string]interface{}
		err = json.Unmarshal(received, &receivedMsg)
		assert.NoError(t, err)
		assert.Equal(t, float64(1), receivedMsg["id"]) // JSON numbers are parsed as float64
		assert.Equal(t, "test", receivedMsg["method"])
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for server to receive message")
	}
}
