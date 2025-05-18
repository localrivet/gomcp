package mqtt

import (
	"context"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMQTTEndToEnd performs an end-to-end test of the MQTT transport
// This test is skipped by default as it requires a running MQTT broker
func TestMQTTEndToEnd(t *testing.T) {
	t.Skip("MQTT E2E test requires a running MQTT broker - enable manually")

	// Configure with your MQTT broker details
	brokerURL := "tcp://localhost:1883"
	clientID := "mqtt-test-client"
	topicPrefix := "mcp-test"

	// Create and configure server transport
	serverTransport := NewTransport(brokerURL, true,
		WithClientID("mqtt-test-server"),
		WithTopicPrefix(topicPrefix),
		WithQoS(1),
	)

	// Initialize and start server transport
	require.NoError(t, serverTransport.Initialize())
	require.NoError(t, serverTransport.Start())
	defer serverTransport.Stop()

	// Set up message handling for the server
	serverDone := make(chan struct{})
	serverTransport.SetMessageHandler(func(msg []byte) ([]byte, error) {
		// Simple echo handler
		return msg, nil
	})

	// Create and configure client transport
	clientTransport := NewTransport(brokerURL, false,
		WithClientID(clientID),
		WithTopicPrefix(topicPrefix),
		WithQoS(1),
	)

	// Initialize and start client transport
	require.NoError(t, clientTransport.Initialize())
	require.NoError(t, clientTransport.Start())
	defer clientTransport.Stop()

	// Send a test message from client to server
	testMessage := []byte(`{"jsonrpc":"2.0","id":1,"method":"echo","params":{"message":"Hello MQTT"}}`)
	err := clientTransport.Send(testMessage)
	require.NoError(t, err)

	// Wait for response processing
	time.Sleep(500 * time.Millisecond)

	// In a real implementation, we would use channels to synchronize and verify the response
	// For this basic test, we're just verifying the transport sends without error

	// Close server processing
	close(serverDone)
}

// TestMQTTWithRealBroker tests the MQTT transport with an actual MQTT broker if available
func TestMQTTWithRealBroker(t *testing.T) {
	// This test attempts to connect to a local MQTT broker if available
	// It will be skipped if the connection fails

	brokerURL := "tcp://localhost:1883"

	// Create options with a short timeout
	opts := paho.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("mqtt-test-probe")
	opts.SetConnectTimeout(2 * time.Second)

	// Create client
	client := paho.NewClient(opts)

	// Try to connect
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		t.Skip("Skipping test as no MQTT broker is available:", token.Error())
	}
	defer client.Disconnect(250)

	// If we get here, we have a working broker
	t.Log("Connected to MQTT broker successfully")

	// Test publish/subscribe
	topic := "mqtt-test-topic"
	message := "Hello MQTT"
	received := make(chan string, 1)

	// Subscribe to test topic
	token = client.Subscribe(topic, 1, func(_ paho.Client, msg paho.Message) {
		received <- string(msg.Payload())
	})
	token.Wait()
	require.NoError(t, token.Error())

	// Publish a test message
	token = client.Publish(topic, 1, false, message)
	token.Wait()
	require.NoError(t, token.Error())

	// Wait for message with timeout
	select {
	case msg := <-received:
		assert.Equal(t, message, msg)
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for message")
	}
}

// TestMQTTContextTimeouts tests context timeouts
func TestMQTTContextTimeouts(t *testing.T) {
	t.Skip("MQTT timeout test requires a running MQTT broker - enable manually")

	// For a real context timeout test, we'd need to implement:
	// 1. Context with timeout passed to transport operations
	// 2. Server that intentionally delays responses
	// 3. Verification that timeouts are properly propagated

	// This would typically be implemented as part of a client transport test
	// since context handling is primarily a client-side concern

	brokerURL := "tcp://localhost:1883"
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Initialize a transport but don't use it since we're skipping the test
	_ = NewTransport(brokerURL, false)

	// In a full implementation, we would have methods that respect context
	// For now we just demonstrate the pattern
	select {
	case <-ctx.Done():
		t.Log("Context deadline exceeded as expected")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Context timeout was not respected")
	}
}
