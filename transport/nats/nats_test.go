package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTransport(t *testing.T) {
	transport := NewTransport("nats://localhost:4222", true)
	assert.NotNil(t, transport)
	assert.Equal(t, "nats://localhost:4222", transport.serverURL)
	assert.True(t, transport.isServer)
	assert.Equal(t, DefaultSubjectPrefix, transport.subjectPrefix)
	assert.Equal(t, DefaultServerSubject, transport.serverSubject)
	assert.Equal(t, DefaultClientSubject, transport.clientSubject)
}

func TestOptionsApply(t *testing.T) {
	transport := NewTransport("nats://localhost:4222", false,
		WithClientID("test-client"),
		WithCredentials("user", "pass"),
		WithSubjectPrefix("custom"),
		WithServerSubject("srv"),
		WithClientSubject("cli"),
	)

	assert.Equal(t, "test-client", transport.clientID)
	assert.Equal(t, "user", transport.username)
	assert.Equal(t, "pass", transport.password)
	assert.Equal(t, "custom", transport.subjectPrefix)
	assert.Equal(t, "srv", transport.serverSubject)
	assert.Equal(t, "cli", transport.clientSubject)

	// Test token option
	transport = NewTransport("nats://localhost:4222", false,
		WithToken("secret-token"),
	)
	assert.Equal(t, "secret-token", transport.token)

	// Test TLS option
	tlsConfig := TLSConfig{
		CertFile:   "cert.pem",
		KeyFile:    "key.pem",
		CAFile:     "ca.pem",
		ServerName: "localhost",
		SkipVerify: false,
	}
	transport = NewTransport("nats://localhost:4222", false,
		WithTLS(tlsConfig),
	)
	assert.NotNil(t, transport.tlsConfig)
	assert.Equal(t, "cert.pem", transport.tlsConfig.CertFile)
	assert.Equal(t, "key.pem", transport.tlsConfig.KeyFile)
	assert.Equal(t, "ca.pem", transport.tlsConfig.CAFile)
	assert.Equal(t, "localhost", transport.tlsConfig.ServerName)
	assert.False(t, transport.tlsConfig.SkipVerify)
}

func TestTopicFormatting(t *testing.T) {
	transport := NewTransport("nats://localhost:4222", true,
		WithSubjectPrefix("mcp"),
		WithServerSubject("requests"),
		WithClientSubject("responses"),
	)

	// Test server subject formatting
	assert.Equal(t, "mcp.requests", transport.getServerSubject(""))
	assert.Equal(t, "mcp.requests.client1", transport.getServerSubject("client1"))

	// Test client subject formatting
	assert.Equal(t, "mcp.responses.client2", transport.getClientSubject("client2"))
	assert.Equal(t, "mcp.responses.>", transport.getClientSubject("all"))
}

func TestNATSEndToEnd(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS server
	t.Skip("NATS E2E test requires a running NATS server - enable manually")

	// To run this test, ensure you have a NATS server running on localhost:4222
	// and remove the t.Skip line above
}
