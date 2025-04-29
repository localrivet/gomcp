// Package server provides the MCP server implementation.
// This file contains server options related to authentication.
package server

import (
	"github.com/localrivet/gomcp/auth" // Import the auth package
)

// WithAuth is a ServerOption to configure and enable authentication using a provided validator and checker.
// It registers the necessary hooks on the server.
func WithAuth(validator auth.TokenValidator, checker auth.PermissionChecker) ServerOption {
	return func(s *Server) {
		if validator == nil {
			s.logger.Warn("WithAuth called with nil TokenValidator. Authentication will not be enabled.")
			return
		}
		if checker == nil {
			s.logger.Info("WithAuth called with nil PermissionChecker. Using AllowAllPermissionChecker.")
			checker = &auth.AllowAllPermissionChecker{} // Use default allow-all if none provided
		}

		// Create the authentication hook using the provided validator
		// Assuming default header/prefix for now, could make this configurable via WithAuth args
		authHookConfig := auth.AuthHookConfig{
			Validator:   validator,
			TokenHeader: "Authorization", // Default
			TokenPrefix: "Bearer ",       // Default
		}
		authHook, err := auth.NewAuthenticationHook(authHookConfig)
		if err != nil {
			s.logger.Error("Failed to create authentication hook: %v. Authentication may not function correctly.", err)
			// Decide whether to proceed or halt server startup? For now, log and continue.
			return
		}

		// Register the hook using the existing ServerOption function
		hookOpt := WithBeforeHandleMessageHook(authHook)
		hookOpt(s) // Apply the hook registration option

		// TODO: Implement and register permission checking hook using the 'checker'
		s.logger.Info("Authentication hook registered.")

		// Store the permission checker for use in request handlers
		s.permissionChecker = checker
		s.logger.Info("Permission checker configured.")
	}
}

// WithJWTAuth is a convenience ServerOption specifically for JWT/JWKS authentication.
// It creates the JWKSTokenValidator internally.
func WithJWTAuth(config auth.JWKSConfig, checker auth.PermissionChecker) ServerOption {
	return func(s *Server) {
		// Create the validator
		// Using nil for httpClient uses http.DefaultClient
		validator, err := auth.NewJWKSTokenValidator(config, nil)
		if err != nil {
			s.logger.Error("Failed to initialize JWT Validator: %v. JWT Authentication will not be enabled.", err)
			return
		}
		s.logger.Info("JWT Validator initialized with JWKS URL: %s", config.JWKSURL)

		// Use the general WithAuth option to register hooks
		authOpt := WithAuth(validator, checker)
		authOpt(s) // Apply the general auth option
	}
}

// Helper method `registerHook` and constants removed as we now use the specific hook options.
