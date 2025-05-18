package test

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
)

func TestNATSClient(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS server
	t.Skip("NATS integration test requires a running NATS server - enable manually")

	serverURL := "nats://localhost:4222"

	// Create client with NATS transport
	c, err := client.NewClient("test-client",
		client.WithNATS(serverURL,
			client.WithNATSClientID("test-client-1"),
			client.WithNATSSubjectPrefix("mcp-test"),
		),
	)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	// Test if client is connected
	assert.False(t, c.IsConnected()) // Should be false before any operations

	// We would test actual operations here, but that requires a running server
	// which we don't have in automated tests

	// Clean up
	err = c.Close()
	assert.NoError(t, err)
}

func TestNATSClientWithBroker(t *testing.T) {
	// Attempt to connect to a real broker and skip if it fails
	serverURL := "nats://localhost:4222"

	// Create client with NATS transport
	c, err := client.NewClient("test-client",
		client.WithNATS(serverURL),
	)
	if err != nil {
		t.Skipf("Skipping test as NATS broker connection failed: %v", err)
	}
	assert.NotNil(t, c)
	c.Close()
}

func TestNATSClientWithTimeout(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS server
	t.Skip("NATS integration test requires a running NATS server - enable manually")

	serverURL := "nats://localhost:4222"

	// Create client with NATS transport and a short request timeout
	c, err := client.NewClient("test-client",
		client.WithNATS(serverURL),
		client.WithRequestTimeout(100*time.Millisecond),
	)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	// We would test timeout behavior here, but that requires a running server
	// which we don't have in automated tests

	// Clean up
	err = c.Close()
	assert.NoError(t, err)
}

func TestNATSClientWithServerAndCredentials(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS server with authentication
	t.Skip("NATS integration test requires a broker with authentication - enable manually")

	serverURL := "nats://localhost:4222"

	// Create client with NATS transport and credentials
	c, err := client.NewClient("test-client",
		client.WithNATS(serverURL,
			client.WithNATSCredentials("username", "password"),
		),
	)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	// Clean up
	err = c.Close()
	assert.NoError(t, err)
}
