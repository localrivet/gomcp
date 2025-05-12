package client

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientError(t *testing.T) {
	// Create a new client error
	message := "Test error message"
	code := 1001
	cause := errors.New("underlying error")
	err := NewClientError(message, code, cause)

	// Check if it's a client error
	clientErr, ok := err.(*ClientError)
	assert.True(t, ok)
	assert.Equal(t, code, clientErr.Code)
	assert.Equal(t, message, clientErr.Message)
	assert.Equal(t, cause, clientErr.Cause)

	// Check error message
	assert.Equal(t, fmt.Sprintf("%s (code=%d): %v", message, code, cause), err.Error())

	// Without cause
	err = NewClientError(message, code, nil)
	assert.Equal(t, fmt.Sprintf("%s (code=%d)", message, code), err.Error())
}

func TestConnectionError(t *testing.T) {
	// Create a new connection error
	endpoint := "http://example.com"
	message := "Connection refused"
	cause := errors.New("network error")
	err := NewConnectionError(endpoint, message, cause)

	// Check error properties
	connErr, ok := err.(*ConnectionError)
	assert.True(t, ok)
	assert.Equal(t, message, connErr.Message)
	assert.Equal(t, cause, connErr.Cause)
	assert.Equal(t, endpoint, connErr.Endpoint)

	// Check error type detection
	assert.True(t, IsConnectionError(err))
	assert.False(t, IsTimeoutError(err))
	assert.False(t, IsServerError(err))
	assert.False(t, IsTransportError(err))
}

func TestTransportError(t *testing.T) {
	// Create a new transport error
	transport := "websocket"
	message := "Transport error"
	cause := errors.New("connection closed")
	err := NewTransportError(transport, message, cause)

	// Check error properties
	transportErr, ok := err.(*TransportError)
	assert.True(t, ok)
	assert.Equal(t, message, transportErr.Message)
	assert.Equal(t, cause, transportErr.Cause)
	assert.Equal(t, transport, transportErr.Transport)

	// Check error type detection
	assert.True(t, IsTransportError(err))
	assert.False(t, IsTimeoutError(err))
	assert.False(t, IsConnectionError(err))
	assert.False(t, IsServerError(err))
}

func TestTimeoutError(t *testing.T) {
	// Create a new timeout error
	operation := "request"
	timeout := 5 * time.Second
	cause := ErrRequestTimeout
	err := NewTimeoutError(operation, timeout, cause)

	// Check error properties
	timeoutErr, ok := err.(*TimeoutError)
	assert.True(t, ok)
	assert.Contains(t, timeoutErr.Message, "timed out after")
	assert.Equal(t, cause, timeoutErr.Cause)
	assert.Equal(t, operation, timeoutErr.Operation)
	assert.Equal(t, timeout, timeoutErr.Timeout)

	// Check error type detection
	assert.True(t, IsTimeoutError(err))
	assert.False(t, IsConnectionError(err))
	assert.False(t, IsTransportError(err))
	assert.False(t, IsServerError(err))
}

func TestServerError(t *testing.T) {
	// Create a new server error
	method := "call_tool"
	serverID := "test-server"
	code := 500
	message := "Internal server error"
	cause := ErrServerError
	err := NewServerError(method, serverID, code, message, cause)

	// Check error properties
	serverErr, ok := err.(*ServerError)
	assert.True(t, ok)
	assert.Equal(t, message, serverErr.Message)
	assert.Equal(t, code, serverErr.Code)
	assert.Equal(t, cause, serverErr.Cause)
	assert.Equal(t, method, serverErr.Method)
	assert.Equal(t, serverID, serverErr.ServerID)

	// Check error type detection
	assert.True(t, IsServerError(err))
	assert.False(t, IsTimeoutError(err))
	assert.False(t, IsConnectionError(err))
	assert.False(t, IsTransportError(err))
}

func TestErrorTypes(t *testing.T) {
	// Test standard errors
	assert.True(t, IsTimeoutError(ErrRequestTimeout))
	assert.True(t, IsConnectionError(ErrNotConnected))
	assert.True(t, IsConnectionError(ErrAlreadyConnected))
	assert.True(t, IsTransportError(ErrTransportFailure))
	assert.True(t, IsServerError(ErrServerError))

	// Standard error is not a special type
	stdErr := errors.New("standard error")
	assert.False(t, IsTimeoutError(stdErr))
	assert.False(t, IsConnectionError(stdErr))
	assert.False(t, IsTransportError(stdErr))
	assert.False(t, IsServerError(stdErr))
}

func TestUnwrap(t *testing.T) {
	// Test unwrap functionality
	cause := errors.New("original error")
	clientErr := NewClientError("wrapper", 0, cause)

	// Unwrap should return the original error
	unwrapped := errors.Unwrap(clientErr)
	assert.Equal(t, cause, unwrapped)

	// Is should work with wrapped errors
	assert.True(t, errors.Is(clientErr, cause))

	// Test with standard errors
	assert.True(t, errors.Is(NewClientError("wrapper", 0, ErrNotConnected), ErrNotConnected))
	assert.True(t, errors.Is(NewTimeoutError("op", time.Second, ErrRequestTimeout), ErrRequestTimeout))
	assert.True(t, errors.Is(NewServerError("method", "server", 0, "msg", ErrServerError), ErrServerError))
}
