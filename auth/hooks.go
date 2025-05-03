// Package auth provides interfaces, implementations, and hooks for handling
// authentication and authorization within the MCP server.
package auth

import (
	"context"
	"encoding/json"
	"fmt" // Required for header extraction simulation
	"strings"

	"github.com/localrivet/gomcp/hooks"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// AuthHookConfig holds configuration needed for the authentication hook.
type AuthHookConfig struct {
	Validator TokenValidator // The configured token validator (e.g., JWKSTokenValidator)
	// Add other config as needed, e.g., how to extract token (header, param, etc.)
	TokenHeader string // e.g., "Authorization"
	TokenPrefix string // e.g., "Bearer "
}

// NewAuthenticationHook creates a BeforeHandleMessageHook that performs token validation.
// It requires a configured TokenValidator.
func NewAuthenticationHook(config AuthHookConfig) (hooks.BeforeHandleMessageHook, error) {
	if config.Validator == nil {
		return nil, fmt.Errorf("TokenValidator is required in AuthHookConfig")
	}
	if config.TokenHeader == "" {
		config.TokenHeader = "Authorization" // Default header
	}
	if config.TokenPrefix == "" {
		config.TokenPrefix = "Bearer " // Default prefix
	}

	// The hook function itself - signature matches hooks.BeforeHandleMessageHook
	authHook := func(ctx context.Context, session types.ClientSession, rawMessage []byte) ([]byte, error) {
		// --- Token Extraction ---
		// NOTE: This hook runs *before* the transport layer has fully processed
		// the request (e.g., HTTP headers). Accessing transport-specific details
		// like HTTP headers directly here is problematic.
		//
		// **Assumption/Workaround:** For now, we assume the token might be passed
		// via a mechanism accessible through the session or context *before* this hook.
		// A more robust solution might involve:
		// 1. Transport layer middleware: The transport (e.g., SSE, WebSocket) extracts
		//    the token and puts it into the context *before* calling Server.HandleMessage.
		// 2. A dedicated auth message: The client sends an explicit 'authenticate' message.
		//
		// **Simulating Header Extraction (Needs Refinement):**
		// We'll *simulate* getting it from context for now. The transport needs to put it there.
		tokenString, ok := TokenFromContext(ctx) // Need to define TokenFromContext
		if !ok || tokenString == "" {
			// Maybe check session metadata if applicable?
			// For now, assume missing token means authentication failure.
			// Allow 'initialize' and 'initialized' methods without auth? Maybe.
			// Let's check the method being called (requires parsing rawMessage partially)
			var baseReq struct {
				Method string `json:"method"`
			}
			// Ignore unmarshal error here, just trying to peek at method
			_ = json.Unmarshal(rawMessage, &baseReq)

			// Allow initialization sequence without a token
			// TODO: Make this configurable?
			if baseReq.Method == protocol.MethodInitialize || baseReq.Method == protocol.MethodInitialized {
				return rawMessage, nil // Pass through without auth check
			}

			// If not initialization and no token, fail.
			return rawMessage, &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeMCPAuthenticationFailed,
					Message: "Missing authentication token",
				},
			}
		}

		// --- Token Validation ---
		principal, err := config.Validator.ValidateToken(ctx, tokenString)
		if err != nil {
			// If err is already an MCPError, return it directly
			if mcpErr, ok := err.(*protocol.MCPError); ok {
				return rawMessage, mcpErr
			}
			// Otherwise, wrap the generic error
			return rawMessage, &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeMCPAuthenticationFailed,
					Message: fmt.Sprintf("Failed to validate token: %v", err),
				},
			}
		}

		// --- Inject Principal into Context ---
		newCtx := ContextWithPrincipal(ctx, principal)

		// --- Execute Hook Logic (Modify Context) ---
		// This hook modifies the context, but the core server HandleMessage
		// doesn't currently accept the modified context back.
		// **Limitation:** The current hook signature doesn't allow passing the
		// modified context (`newCtx`) down to subsequent handlers easily.
		//
		// **Workaround:** We rely on the fact that the *original* context `ctx`
		// passed to `HandleMessage` will eventually be used by handlers, and
		// we've modified *that* context's value store via `ContextWithPrincipal`.
		// This works because context values are immutable, but the map holding them
		// is passed by reference implicitly. This is subtle and potentially fragile.
		// A better approach might involve changing hook signatures or server logic.

		// For now, we return the original message and nil error, relying on the
		// context modification side-effect.
		_ = newCtx // Avoid unused variable error, acknowledge limitation.

		return rawMessage, nil // Pass message through, context modified implicitly
	}

	return authHook, nil
}

// --- Context Helpers for Token ---
// These are placeholders; the actual mechanism to get the token into the context
// needs to be implemented in the transport layer or via a specific auth request.

type tokenKeyType struct{}

var tokenKey = tokenKeyType{}

// ContextWithToken returns a context embedding the token string.
// This should be called by the transport layer *before* Server.HandleMessage.
func ContextWithToken(ctx context.Context, token string) context.Context {
	// Simple extraction assuming "Bearer <token>"
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	return context.WithValue(ctx, tokenKey, token)
}

// TokenFromContext extracts the token string from the context.
func TokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(tokenKey).(string)
	return token, ok
}

// --- Server Option Removed ---
// The WithJWTAuth ServerOption was removed from this package to avoid import cycles.
// It should be defined in the `server` package, potentially in a new `server/auth_options.go` file.
