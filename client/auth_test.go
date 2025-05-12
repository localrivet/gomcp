package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBearerAuth(t *testing.T) {
	// Test creating a bearer auth provider
	token := "test-token"
	auth := NewBearerAuth(token)

	// Test GetAuthHeaders
	headers := auth.GetAuthHeaders()
	assert.Equal(t, "Bearer test-token", headers["Authorization"])

	// Test GetAuthToken
	assert.Equal(t, token, auth.GetAuthToken())
}

func TestBasicAuth(t *testing.T) {
	// Test creating a basic auth provider
	username := "testuser"
	password := "testpass"
	auth := NewBasicAuth(username, password)

	// Test GetAuthHeaders
	headers := auth.GetAuthHeaders()
	assert.Contains(t, headers["Authorization"], "Basic ")

	// Test GetAuthToken
	assert.NotEmpty(t, auth.GetAuthToken())
}

func TestCustomHeaderAuth(t *testing.T) {
	// Test creating a custom header auth provider
	headers := map[string]string{
		"X-API-Key":    "test-key",
		"X-Client-ID":  "test-client",
		"Content-Type": "application/json",
	}
	token := "test-token"
	auth := NewCustomHeaderAuth(headers, token)

	// Test GetAuthHeaders
	authHeaders := auth.GetAuthHeaders()
	assert.Equal(t, "test-key", authHeaders["X-API-Key"])
	assert.Equal(t, "test-client", authHeaders["X-Client-ID"])
	assert.Equal(t, "application/json", authHeaders["Content-Type"])

	// Test GetAuthToken
	assert.Equal(t, token, auth.GetAuthToken())
}

func TestNoAuth(t *testing.T) {
	// Test creating a no-auth provider
	auth := NewNoAuth()

	// Test GetAuthHeaders
	headers := auth.GetAuthHeaders()
	assert.Empty(t, headers)

	// Test GetAuthToken
	assert.Empty(t, auth.GetAuthToken())
}
