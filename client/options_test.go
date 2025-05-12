package client

import (
	"context"
	"testing"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
)

// mockClient is used to test options
type mockClient struct {
	config  *ClientConfig
	logger  logx.Logger
	timeout time.Duration
	auth    AuthProvider
}

func newMockClient() *mockClient {
	return &mockClient{
		config: &ClientConfig{
			Logger:                   logx.NewDefaultLogger(),
			PreferredProtocolVersion: protocol.CurrentProtocolVersion,
			DefaultTimeout:           30 * time.Second,
		},
		logger: logx.NewDefaultLogger(),
	}
}

func (m *mockClient) applyOptions(opts ...ClientOption) {
	for _, opt := range opts {
		opt(m.config)
	}

	// Apply options to mock client
	m.logger = m.config.Logger
	m.timeout = m.config.DefaultTimeout
	m.auth = m.config.AuthProvider
}

func TestWithPreferredProtocolVersion(t *testing.T) {
	client := newMockClient()

	// Default should be current protocol version
	assert.Equal(t, protocol.CurrentProtocolVersion, client.config.PreferredProtocolVersion)

	// Apply option
	client.applyOptions(WithPreferredProtocolVersion("2024-01-01"))

	// Check if protocol version was set
	assert.Equal(t, "2024-01-01", client.config.PreferredProtocolVersion)
}

func TestWithLogger(t *testing.T) {
	client := newMockClient()

	// Create a custom logger
	logger := logx.NewDefaultLogger()

	// Apply option
	client.applyOptions(WithLogger(logger))

	// Check if logger was set
	assert.Equal(t, logger, client.config.Logger)
	assert.Equal(t, logger, client.logger)
}

func TestWithTimeout(t *testing.T) {
	client := newMockClient()

	// Default should be non-zero
	assert.NotZero(t, client.config.DefaultTimeout)

	// Apply option
	timeout := 45 * time.Second
	client.applyOptions(WithTimeout(timeout))

	// Check if timeout was set
	assert.Equal(t, timeout, client.config.DefaultTimeout)
	assert.Equal(t, timeout, client.timeout)
}

func TestWithRetryStrategy(t *testing.T) {
	client := newMockClient()

	// Default should be nil
	assert.Nil(t, client.config.RetryStrategy)

	// Create a backoff strategy
	backoff := NewExponentialBackoff(
		100*time.Millisecond,
		5*time.Second,
		3,
	)

	// Apply option
	client.applyOptions(WithRetryStrategy(backoff))

	// Check if strategy was set
	assert.Equal(t, backoff, client.config.RetryStrategy)
}

// Simple middleware implementation for testing
type testMiddleware struct{}

func (t *testMiddleware) BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
	return req, nil
}

func (t *testMiddleware) AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	return resp, nil
}

func TestWithMiddleware(t *testing.T) {
	client := newMockClient()

	// Default should be empty
	assert.Empty(t, client.config.Middleware)

	// Create test middleware
	middleware1 := &testMiddleware{}
	middleware2 := &testMiddleware{}

	// Apply option for one middleware
	client.applyOptions(WithMiddleware(middleware1))
	assert.Len(t, client.config.Middleware, 1)

	// Apply option for another middleware
	client.applyOptions(WithMiddleware(middleware2))
	assert.Len(t, client.config.Middleware, 2)
}

func TestWithAuth(t *testing.T) {
	client := newMockClient()

	// Default should be nil
	assert.Nil(t, client.config.AuthProvider)

	// Create auth provider
	auth := NewBearerAuth("test-token")

	// Apply option
	client.applyOptions(WithAuth(auth))

	// Check if auth was set
	assert.Equal(t, auth, client.config.AuthProvider)
	assert.Equal(t, auth, client.auth)
}

func TestWithFollowRedirects(t *testing.T) {
	client := newMockClient()

	// Apply option
	client.applyOptions(WithFollowRedirects(true))

	// Check if the custom option was set
	assert.NotNil(t, client.config.TransportOptions.Custom)
	if client.config.TransportOptions.Custom != nil {
		follow, ok := client.config.TransportOptions.Custom["follow_redirects"]
		assert.True(t, ok)
		assert.True(t, follow.(bool))
	}
}
