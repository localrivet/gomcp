package server

import (
	"io"
	"log/slog"
)

// NewTestLogger creates a logger suitable for testing with minimal output.
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
