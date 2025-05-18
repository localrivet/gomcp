package server

import (
	"io"
	"log/slog"
)

// NewTestLogger creates a logger suitable for testing with minimal output.
// This function returns a structured logger that discards all output by default,
// which is useful for keeping test output clean while still allowing the option
// to enable logs during test development.
//
// Returns:
//   - A configured slog.Logger instance with output directed to io.Discard
func NewTestLogger() *slog.Logger {
	// Create a no-op logger for tests to avoid cluttering test output
	// You can set this to os.Stdout during development if you want to see logs
	var output io.Writer = io.Discard

	// Uncomment to enable test logs
	// output = os.Stdout

	return slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Debug level to capture all logs
	}))
}
