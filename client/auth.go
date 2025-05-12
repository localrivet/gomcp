package client

import (
	"encoding/base64"
	"fmt"
)

// bearerAuth implements AuthProvider with Bearer token authentication
type bearerAuth struct {
	token string
}

// NewBearerAuth creates a new Bearer token auth provider
func NewBearerAuth(token string) AuthProvider {
	return &bearerAuth{token: token}
}

// GetAuthHeaders implements AuthProvider.GetAuthHeaders
func (a *bearerAuth) GetAuthHeaders() map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", a.token),
	}
}

// GetAuthToken implements AuthProvider.GetAuthToken
func (a *bearerAuth) GetAuthToken() string {
	return a.token
}

// basicAuth implements AuthProvider with Basic authentication
type basicAuth struct {
	username string
	password string
	token    string // computed base64 token
}

// NewBasicAuth creates a new Basic auth provider
func NewBasicAuth(username, password string) AuthProvider {
	token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	return &basicAuth{
		username: username,
		password: password,
		token:    token,
	}
}

// GetAuthHeaders implements AuthProvider.GetAuthHeaders
func (a *basicAuth) GetAuthHeaders() map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Basic %s", a.token),
	}
}

// GetAuthToken implements AuthProvider.GetAuthToken
func (a *basicAuth) GetAuthToken() string {
	return a.token
}

// customHeaderAuth implements AuthProvider with custom headers
type customHeaderAuth struct {
	headers map[string]string
	token   string
}

// NewCustomHeaderAuth creates a new custom header auth provider
func NewCustomHeaderAuth(headers map[string]string, token string) AuthProvider {
	return &customHeaderAuth{
		headers: headers,
		token:   token,
	}
}

// GetAuthHeaders implements AuthProvider.GetAuthHeaders
func (a *customHeaderAuth) GetAuthHeaders() map[string]string {
	return a.headers
}

// GetAuthToken implements AuthProvider.GetAuthToken
func (a *customHeaderAuth) GetAuthToken() string {
	return a.token
}

// noAuth implements AuthProvider with no authentication
type noAuth struct{}

// NewNoAuth creates a new no-auth provider
func NewNoAuth() AuthProvider {
	return &noAuth{}
}

// GetAuthHeaders implements AuthProvider.GetAuthHeaders
func (a *noAuth) GetAuthHeaders() map[string]string {
	return map[string]string{}
}

// GetAuthToken implements AuthProvider.GetAuthToken
func (a *noAuth) GetAuthToken() string {
	return ""
}
