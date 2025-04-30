package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/localrivet/gomcp/auth"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// SecureEchoArgs defines arguments for the secure-echo tool.
type SecureEchoArgs struct {
	Message string `json:"message" description:"The message to echo." required:"true"`
}

// secureEchoHandler implements the logic for the secure-echo tool.
// This handler will only be called if authentication succeeds via the auth hook
func secureEchoHandler(args SecureEchoArgs) (protocol.Content, error) {
	log.Printf("Executing secure-echo tool with message: %s", args.Message)

	if args.Message == "" {
		return nil, errors.New("message cannot be empty")
	}

	log.Printf("Securely Echoing message: %s", args.Message)
	successContent := protocol.TextContent{Type: "text", Text: args.Message}
	return successContent, nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Get JWKS URL from environment variable or use a default for testing
	jwksURL := os.Getenv("JWKS_URL")
	if jwksURL == "" {
		// This is just an example - in production you would require a real JWKS URL
		log.Println("JWKS_URL not set, using a mock TokenValidator for demonstration")
	}

	// Configure the server with JWT auth
	log.Println("Starting Auth Example MCP Server with JWT Authentication...")

	// Create a validator (using either real JWKS or a mock for demonstration)
	var tokenValidator auth.TokenValidator
	var authSetupErr error

	if jwksURL != "" {
		// Set up a real JWKS validator
		jwksConfig := auth.JWKSConfig{
			JWKSURL:          jwksURL,
			ExpectedIssuer:   os.Getenv("JWT_ISSUER"),   // Optional
			ExpectedAudience: os.Getenv("JWT_AUDIENCE"), // Optional
			ClockSkew:        5 * time.Second,           // Allow 5 seconds of clock skew
			RefreshInterval:  12 * time.Hour,            // Refresh JWKS cache daily
		}
		tokenValidator, authSetupErr = auth.NewJWKSTokenValidator(jwksConfig, http.DefaultClient)
	} else {
		// Example only: For demo purposes, create a simple mock validator
		// that accepts a fixed token (you should never do this in production)
		tokenValidator = &MockTokenValidator{validToken: "test-token-123"}
	}

	if authSetupErr != nil {
		log.Fatalf("Failed to create token validator: %v", authSetupErr)
	}

	// Configure the auth hook
	authConfig := auth.AuthHookConfig{
		Validator:   tokenValidator,
		TokenHeader: "Authorization", // Standard header name
		TokenPrefix: "Bearer ",       // Standard bearer token prefix
	}

	authHook, err := auth.NewAuthenticationHook(authConfig)
	if err != nil {
		log.Fatalf("Failed to create authentication hook: %v", err)
	}

	// Create server with auth hook
	srv := server.NewServer("GoAuthServer-JWT",
		server.WithBeforeHandleMessageHook(authHook),
	)

	// Register the secure echo tool
	err = server.AddTool(
		srv,
		"secure-echo",
		"Echoes back the provided message (Requires JWT Auth).",
		secureEchoHandler,
	)
	if err != nil {
		log.Fatalf("Failed to register secure-echo tool: %v", err)
	}

	// Run the server using the ServeStdio helper
	log.Println("Server setup complete. Listening on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}

// MockTokenValidator is for DEMO PURPOSES ONLY - used when no real JWKS URL is provided
// In a real application, you would use the JWKSTokenValidator instead
type MockTokenValidator struct {
	validToken string
}

func (m *MockTokenValidator) ValidateToken(ctx context.Context, tokenString string) (auth.Principal, error) {
	if tokenString == m.validToken {
		// Create a simple principal with minimal claims
		return &mockPrincipal{
			claims: map[string]interface{}{
				"sub":  "mock-user-id",
				"name": "Mock User",
			},
		}, nil
	}

	return nil, &protocol.MCPError{
		ErrorPayload: protocol.ErrorPayload{
			Code:    protocol.ErrorCodeMCPAuthenticationFailed,
			Message: "Invalid token",
		},
	}
}

type mockPrincipal struct {
	claims map[string]interface{}
}

func (p *mockPrincipal) GetClaims() interface{} {
	return p.claims
}

func (p *mockPrincipal) GetSubject() string {
	sub, _ := p.claims["sub"].(string)
	return sub
}

// Removed manual tool definition (secureEchoTool)
