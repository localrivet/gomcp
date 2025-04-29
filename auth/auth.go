// Package auth defines interfaces and structures for handling authentication
// and authorization within the MCP server, focusing initially on OAuth 2.1 JWTs.
package auth

import (
	"context"

	"github.com/localrivet/gomcp/protocol" // For error codes
	// For ClientSession
)

// Principal represents the authenticated entity (e.g., user, client application)
// after successful token validation. It can carry claims from the token.
type Principal interface {
	// GetClaims returns the claims associated with the principal.
	// The specific type of claims depends on the token format (e.g., map[string]interface{} for JWT).
	GetClaims() interface{}
	// GetSubject returns a unique identifier for the principal (e.g., 'sub' claim from JWT).
	GetSubject() string
}

// TokenValidator defines the interface for validating access tokens.
// Implementations will handle specific token types (e.g., JWT) and validation methods (e.g., JWKS).
type TokenValidator interface {
	// ValidateToken attempts to validate the given token string.
	// It returns the authenticated Principal if validation is successful,
	// or an error (potentially a *protocol.ErrorPayload for specific JSON-RPC errors) otherwise.
	ValidateToken(ctx context.Context, tokenString string) (Principal, error)
}

// PermissionChecker defines the interface for checking if a principal
// is authorized to perform a specific MCP action.
type PermissionChecker interface {
	// CheckPermission verifies if the given principal has the necessary permissions
	// for the specified MCP method and parameters.
	// It should return nil if authorized, or an error (e.g., a *protocol.ErrorPayload
	// with ErrorCodePermissionDenied) if not.
	CheckPermission(ctx context.Context, principal Principal, method string, params interface{}) error
}

// --- Context Handling ---

// principalKey is the context key for storing the authenticated Principal.
type principalKeyType struct{}

var principalKey = principalKeyType{}

// ContextWithPrincipal returns a new context with the given Principal embedded.
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

// PrincipalFromContext retrieves the Principal from the context, if present.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey).(Principal)
	return principal, ok
}

// --- Default Implementations (Optional) ---

// AllowAllPermissionChecker is a simple implementation that grants all permissions.
// Useful for testing or servers that don't require fine-grained checks after authentication.
type AllowAllPermissionChecker struct{}

func (c *AllowAllPermissionChecker) CheckPermission(ctx context.Context, principal Principal, method string, params interface{}) error {
	// Always allow if a principal exists (meaning authentication succeeded)
	if principal == nil {
		// This case shouldn't typically happen if the auth hook runs first,
		// but handle defensively.
		return &protocol.MCPError{ // Use MCPError which implements error
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.ErrorCodeMCPAuthenticationFailed, // Use correct constant
				Message: "No authenticated principal found in context",
			},
		}
	}
	return nil // Allow access
}

// Ensure AllowAllPermissionChecker implements the interface
var _ PermissionChecker = (*AllowAllPermissionChecker)(nil)
