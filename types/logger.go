// Package types defines core interfaces and common types used across the GoMCP library.
package types

// Logger defines the interface for logging within the GoMCP library.
// It allows for different logging implementations to be used.
type Logger interface {
	// Debug logs a debug message.
	Debug(msg string, args ...interface{})

	// Info logs an informational message.
	Info(msg string, args ...interface{})

	// Warn logs a warning message.
	Warn(msg string, args ...interface{})

	// Error logs an error message.
	Error(msg string, args ...interface{})
}
