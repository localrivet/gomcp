package server

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/localrivet/gomcp/transport/stdio"
)

// AsStdio configures the server to use the Standard I/O transport.
// The stdio transport uses standard input and output streams for communication,
// making it ideal for command-line tools, language server protocols, and
// child processes that communicate with parent processes.
//
// Parameters:
//   - logFile: Optional path to a file where standard I/O logging should be redirected.
//     If not provided, logs will be written to io.Discard to prevent log messages
//     from corrupting the JSON-RPC protocol communication over stdin/stdout.
//
// Returns:
//   - The server instance for method chaining
//
// This is the default transport for MCP servers and is particularly suitable for
// CLI applications and integration with development environments.
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
