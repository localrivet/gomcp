// Package logx provides a standard logger implementation for the gomcp project.
package logx

import (
	"log"
	"os"

	"github.com/localrivet/gomcp/types"
)

// DefaultLogger provides a basic logger implementation using the standard log package.
type DefaultLogger struct {
	logger *log.Logger
}

// NewDefaultLogger creates a new logger writing to stderr with standard flags.
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		logger: log.New(os.Stderr, "[GoMCP] ", log.LstdFlags|log.Ltime|log.Lmsgprefix),
	}
}

// NewLogger creates a new logger with a specific prefix.
func NewLogger(prefix string) *DefaultLogger {
	if prefix == "" {
		prefix = "[GoMCP] "
	} else if prefix[len(prefix)-1] != ' ' {
		prefix += " "
	}
	return &DefaultLogger{
		logger: log.New(os.Stderr, prefix, log.LstdFlags|log.Ltime|log.Lmsgprefix),
	}
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	l.logger.Printf("DEBUG: "+msg, args...)
}
func (l *DefaultLogger) Info(msg string, args ...interface{}) { l.logger.Printf("INFO: "+msg, args...) }
func (l *DefaultLogger) Warn(msg string, args ...interface{}) { l.logger.Printf("WARN: "+msg, args...) }
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("ERROR: "+msg, args...)
}

// Ensure interface compliance
var _ types.Logger = (*DefaultLogger)(nil)
