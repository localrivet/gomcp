package server

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/localrivet/gomcp/transport/stdio"
)

// AsStdio configures the server to use the Standard I/O transport.
// Optionally specify a log file path to direct all logs there instead of stderr.
// This is important for MCP communication over stdin/stdout to prevent log messages
// from corrupting the JSON-RPC protocol.
func (s *serverImpl) AsStdio(logFile ...string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Configure logging to avoid stdout/stderr
	if len(logFile) > 0 && logFile[0] != "" {
		// Ensure directory exists
		logDir := filepath.Dir(logFile[0])
		if logDir != "." {
			os.MkdirAll(logDir, 0755)
		}

		// Open log file
		if f, err := os.OpenFile(logFile[0], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
			// Create a new logger with the file output
			s.logger = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}))
		} else {
			// If we can't open the log file, disable logging
			s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		}
	} else {
		// No log file specified, disable logging to avoid breaking stdio transport
		s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	s.transport = stdio.NewTransport()
	return s
}
