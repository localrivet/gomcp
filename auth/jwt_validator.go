// Package auth provides interfaces and structures for handling authentication
// and authorization within the MCP server. This file implements a TokenValidator
// based on JWTs and JWKS.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v2/jwk" // Common library for JWK/JWKS handling
	"github.com/localrivet/gomcp/protocol"
)

// JWKSConfig holds configuration for the JWKS-based validator.
type JWKSConfig struct {
	// JWKSURL is the URL of the JSON Web Key Set endpoint. (Required)
	JWKSURL string
	// ExpectedIssuer is the required value for the 'iss' claim. (Optional)
	ExpectedIssuer string
	// ExpectedAudience is the required value for the 'aud' claim. (Optional)
	ExpectedAudience string
	// ClockSkew defines the acceptable time difference for validating expiry ('exp') and not before ('nbf') claims. Defaults to 0.
	ClockSkew time.Duration
	// RefreshInterval defines how often to refresh the JWK set from the URL. Defaults to 1 hour.
	RefreshInterval time.Duration
}

// JWKSTokenValidator implements the TokenValidator interface using a JWKS endpoint.
type JWKSTokenValidator struct {
	config     JWKSConfig
	jwkCache   *jwk.Cache // Use lestrrat-go's caching mechanism
	httpClient *http.Client
}

// NewJWKSTokenValidator creates a new validator instance.
func NewJWKSTokenValidator(config JWKSConfig, client *http.Client) (*JWKSTokenValidator, error) {
	if config.JWKSURL == "" {
		return nil, fmt.Errorf("JWKSURL is required in JWKSConfig")
	}
	if config.RefreshInterval <= 0 {
		config.RefreshInterval = 1 * time.Hour // Default refresh interval
	}
	if client == nil {
		client = http.DefaultClient
	}

	// Create a JWK cache that automatically refreshes
	cache := jwk.NewCache(context.Background())
	err := cache.Register(config.JWKSURL, jwk.WithMinRefreshInterval(config.RefreshInterval), jwk.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to register JWKS URL %s with cache: %w", config.JWKSURL, err)
	}

	// Trigger initial fetch, handle potential error during setup
	_, err = cache.Refresh(context.Background(), config.JWKSURL)
	if err != nil {
		return nil, fmt.Errorf("failed initial JWKS fetch from %s: %w", config.JWKSURL, err)
	}

	return &JWKSTokenValidator{
		config:     config,
		jwkCache:   cache,
		httpClient: client,
	}, nil
}

// jwtPrincipal implements the Principal interface for JWT claims.
type jwtPrincipal struct {
	claims jwt.MapClaims
}

func (p *jwtPrincipal) GetClaims() interface{} {
	return p.claims
}

func (p *jwtPrincipal) GetSubject() string {
	sub, _ := p.claims.GetSubject() // Handles potential error internally
	return sub
}

// ValidateToken implements the TokenValidator interface.
func (v *JWKSTokenValidator) ValidateToken(ctx context.Context, tokenString string) (Principal, error) {
	// 1. Parse the JWT, providing the Keyfunc to fetch the key from JWKS
	token, err := jwt.Parse(tokenString, v.keyFunc)
	if err != nil {
		// Distinguish between parsing errors and validation errors (like expired)
		return nil, &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.ErrorCodeMCPAuthenticationFailed,
				Message: fmt.Sprintf("Invalid token format or signature: %v", err),
			},
		}
	}

	// 2. Basic validation (signature already checked by Parse with keyFunc)
	if !token.Valid {
		return nil, &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.ErrorCodeMCPAuthenticationFailed,
				Message: "Token is invalid (potentially expired, inactive, or signature mismatch)",
			},
		}
	}

	// 3. Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.ErrorCodeMCPAuthenticationFailed,
				Message: "Invalid token claims format",
			},
		}
	}

	// 4. Validate standard claims (iss, aud, exp, nbf) using jwt.Validator
	var validationOptions []jwt.ParserOption
	if v.config.ExpectedIssuer != "" {
		validationOptions = append(validationOptions, jwt.WithIssuer(v.config.ExpectedIssuer))
	}
	if v.config.ExpectedAudience != "" {
		validationOptions = append(validationOptions, jwt.WithAudience(v.config.ExpectedAudience))
	}
	if v.config.ClockSkew > 0 {
		validationOptions = append(validationOptions, jwt.WithLeeway(v.config.ClockSkew))
	}

	validator := jwt.NewValidator(validationOptions...)
	err = validator.Validate(claims)
	if err != nil {
		return nil, &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.ErrorCodeMCPAuthenticationFailed,
				Message: fmt.Sprintf("Token validation failed: %v", err),
			},
		}
	}

	// 5. Return the principal
	principal := &jwtPrincipal{claims: claims}
	return principal, nil
}

// keyFunc is used by jwt.Parse to fetch the public key from the JWKS cache.
func (v *JWKSTokenValidator) keyFunc(token *jwt.Token) (interface{}, error) {
	// Fetch the key set from the cache
	keySet, err := v.jwkCache.Get(context.Background(), v.config.JWKSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWK set from cache for %s: %w", v.config.JWKSURL, err)
	}

	// Get the Key ID ('kid') from the token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("JWT header missing 'kid' field")
	}

	// Look up the key in the set
	key, found := keySet.LookupKeyID(kid)
	if !found {
		// Key not found, try refreshing the cache once in case it's new
		_, refreshErr := v.jwkCache.Refresh(context.Background(), v.config.JWKSURL)
		if refreshErr != nil {
			// Log the refresh error but return the original "key not found" error
			// log.Printf("Failed to refresh JWKS cache after key lookup failure: %v", refreshErr)
			return nil, fmt.Errorf("key with kid '%s' not found in JWKS at %s (refresh attempted)", kid, v.config.JWKSURL)
		}
		// Retry lookup after refresh
		keySet, err = v.jwkCache.Get(context.Background(), v.config.JWKSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get JWK set from cache after refresh for %s: %w", v.config.JWKSURL, err)
		}
		key, found = keySet.LookupKeyID(kid)
		if !found {
			return nil, fmt.Errorf("key with kid '%s' not found in JWKS at %s (even after refresh)", kid, v.config.JWKSURL)
		}
	}

	// Extract the raw public key material
	var rawKey interface{}
	if err := key.Raw(&rawKey); err != nil {
		return nil, fmt.Errorf("failed to get raw public key material for kid '%s': %w", kid, err)
	}

	// Check if the algorithm used in the token matches the key type (optional but good practice)
	// Example: Check if token uses RS256 and key is RSA, or ES256 and key is ECDSA etc.
	// ... implementation depends on specific requirements ...

	return rawKey, nil
}

// Ensure JWKSTokenValidator implements the interface
var _ TokenValidator = (*JWKSTokenValidator)(nil)
