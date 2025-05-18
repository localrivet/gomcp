// Package client provides the client-side implementation of the MCP protocol.
package client

// This file contains helper functions for testing the sampling functionality.
// These functions are exported but should only be used for testing.

// ParseSamplingResponseForTest exposes parseSamplingResponse for testing
func ParseSamplingResponseForTest(data []byte) (*SamplingResponse, error) {
	return parseSamplingResponse(data)
}

// ValidateSamplingResponseForVersionForTest exposes validateSamplingResponseForVersion for testing
func ValidateSamplingResponseForVersionForTest(response *SamplingResponse, version string) error {
	return validateSamplingResponseForVersion(response, version)
}

// ParseStreamingSamplingResponseForTest exposes parseStreamingSamplingResponse for testing
func ParseStreamingSamplingResponseForTest(data []byte) (*StreamingSamplingResponse, error) {
	return parseStreamingSamplingResponse(data)
}

// ValidateStreamingSamplingResponseForVersionForTest exposes validateStreamingSamplingResponseForVersion for testing
func ValidateStreamingSamplingResponseForVersionForTest(response *StreamingSamplingResponse, version string) error {
	return validateStreamingSamplingResponseForVersion(response, version)
}

// IsStreamingSupportedForVersionForTest exposes isStreamingSupportedForVersion for testing
func IsStreamingSupportedForVersionForTest(version string) bool {
	return isStreamingSupportedForVersion(version)
}
