package mqtt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTransport(t *testing.T) {
	// Create a new transport
	trans := NewTransport("tcp://localhost:1883", true)
	assert.NotNil(t, trans)
	assert.Equal(t, "tcp://localhost:1883", trans.brokerURL)
	assert.True(t, trans.isServer)
	assert.Equal(t, DefaultTopicPrefix, trans.topicPrefix)
	assert.Equal(t, DefaultServerTopic, trans.serverTopic)
	assert.Equal(t, DefaultClientTopic, trans.clientTopic)
	assert.Equal(t, byte(1), trans.qos)
	assert.True(t, trans.cleanSession)
}

func TestOptionsApply(t *testing.T) {
	// Create a new transport with options
	trans := NewTransport("tcp://localhost:1883", true,
		WithClientID("test-client"),
		WithQoS(2),
		WithCredentials("username", "password"),
		WithTopicPrefix("custom"),
		WithServerTopic("req"),
		WithClientTopic("resp"),
		WithCleanSession(false),
	)

	assert.NotNil(t, trans)
	assert.Equal(t, "test-client", trans.clientID)
	assert.Equal(t, byte(2), trans.qos)
	assert.Equal(t, "username", trans.username)
	assert.Equal(t, "password", trans.password)
	assert.Equal(t, "custom", trans.topicPrefix)
	assert.Equal(t, "req", trans.serverTopic)
	assert.Equal(t, "resp", trans.clientTopic)
	assert.False(t, trans.cleanSession)
}

func TestTopicFormatting(t *testing.T) {
	trans := NewTransport("tcp://localhost:1883", true)

	// Test server topic formatting
	assert.Equal(t, "mcp/requests", trans.getServerTopic(""))
	assert.Equal(t, "mcp/requests/client1", trans.getServerTopic("client1"))

	// Test client topic formatting
	assert.Equal(t, "mcp/responses/#", trans.getClientTopic("all"))
	assert.Equal(t, "mcp/responses/client1", trans.getClientTopic("client1"))

	// Test with custom prefix
	trans = NewTransport("tcp://localhost:1883", true, WithTopicPrefix("custom"))
	assert.Equal(t, "custom/requests", trans.getServerTopic(""))
	assert.Equal(t, "custom/responses/client1", trans.getClientTopic("client1"))
}

// Note: Integration tests requiring an actual MQTT broker would be in separate files
// and typically skipped unless explicitly enabled
