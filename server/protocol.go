// Package server provides the server-side implementation of the MCP protocol.
package server

import (
	"encoding/json"
	"fmt"
)

// ValidateProtocolVersion validates that the requested protocol version is supported
// and returns the validated version or an error.
func (s *serverImpl) ValidateProtocolVersion(clientVersion string) (string, error) {
	// If no version specified, use the default version
	if clientVersion == "" {
		defaultVersion := s.versionDetector.DefaultVersion
		s.logger.Debug("client did not specify protocol version, using default", "version", defaultVersion)
		return defaultVersion, nil
	}

	// Validate the version
	validatedVersion, err := s.versionDetector.ValidateVersion(clientVersion)
	if err != nil {
		return "", fmt.Errorf("unsupported protocol version: %s", clientVersion)
	}

	s.logger.Debug("using validated protocol version", "requestedVersion", clientVersion, "validatedVersion", validatedVersion)
	return validatedVersion, nil
}

// ExtractProtocolVersion extracts the protocol version from the initialize request
func ExtractProtocolVersion(params json.RawMessage) (string, error) {
	if params == nil {
		return "", nil
	}

	// First try parsing as a proper struct
	var initParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}

	if err := json.Unmarshal(params, &initParams); err == nil {
		return initParams.ProtocolVersion, nil
	}

	// If that fails, try to get the raw value and convert it
	var rawParams map[string]interface{}
	if err := json.Unmarshal(params, &rawParams); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	if protocolVersion, exists := rawParams["protocolVersion"]; exists {
		switch v := protocolVersion.(type) {
		case string:
			return v, nil
		case float64: // JSON numbers are unmarshaled to float64
			return fmt.Sprintf("%.0f", v), nil
		case int:
			return fmt.Sprintf("%d", v), nil
		case bool:
			return fmt.Sprintf("%t", v), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	}

	return "", nil
}
