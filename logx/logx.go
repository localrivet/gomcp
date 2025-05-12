// Package logx provides a standard logger implementation for the gomcp project.
package logx

import (
	"log"
	"os"
	"sync"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// DefaultLogger provides a basic logger implementation using the standard log package.
type DefaultLogger struct {
	logger *log.Logger
	level  protocol.LoggingLevel
	mu     sync.Mutex
}

// NewDefaultLogger creates a new logger writing to stderr with standard flags.
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		logger: log.New(os.Stderr, "[GoMCP] ", log.LstdFlags|log.Ltime|log.Lmsgprefix),
	}
}

// NewLogger creates a new logger instance based on the configuration.
// Currently only supports "stdout".
func NewLogger(logType string) Logger { // Return the interface type
	// Basic implementation using standard log
	// TODO: Add support for file logging, structured logging (e.g., zerolog, zap)
	prefix := "[MCP Log] " // Example prefix
	return &DefaultLogger{
		logger: log.New(os.Stdout, prefix, log.LstdFlags|log.Lshortfile),
		level:  protocol.LogLevelInfo, // Default level
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

// Logger defines the interface for logging.
type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	SetLevel(level protocol.LoggingLevel)
}

// SetLevel updates the logging level for the DefaultLogger.
func (l *DefaultLogger) SetLevel(level protocol.LoggingLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// TODO: Validate level? For now, assume valid levels are passed.
	l.level = level
	l.logger.Printf("[LogX] Log level set to: %s", l.level) // Use internal logger
}

// Helper to map protocol level to an internal severity
func levelToSeverity(level protocol.LoggingLevel) int {
	// Implementation of levelToSeverity function
	return 0 // Placeholder return, actual implementation needed
}

// StandardLoggerAdapter adapts a standard log.Logger to implement the Logger interface
type StandardLoggerAdapter struct {
	logger *log.Logger
	level  protocol.LoggingLevel
	mu     sync.Mutex
}

// NewStandardLoggerAdapter creates a Logger that wraps a standard Go log.Logger
func NewStandardLoggerAdapter(logger *log.Logger) Logger {
	if logger == nil {
		logger = log.New(os.Stderr, "[GoMCP] ", log.LstdFlags)
	}
	return &StandardLoggerAdapter{
		logger: logger,
		level:  protocol.LogLevelInfo,
	}
}

// Debug logs a debug message
func (a *StandardLoggerAdapter) Debug(format string, v ...interface{}) {
	a.logger.Printf("DEBUG: "+format, v...)
}

// Info logs an info message
func (a *StandardLoggerAdapter) Info(format string, v ...interface{}) {
	a.logger.Printf("INFO: "+format, v...)
}

// Warn logs a warning message
func (a *StandardLoggerAdapter) Warn(format string, v ...interface{}) {
	a.logger.Printf("WARN: "+format, v...)
}

// Error logs an error message
func (a *StandardLoggerAdapter) Error(format string, v ...interface{}) {
	a.logger.Printf("ERROR: "+format, v...)
}

// SetLevel sets the logging level
func (a *StandardLoggerAdapter) SetLevel(level protocol.LoggingLevel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.level = level
	a.logger.Printf("[LogX] Log level set to: %s", level)
}

// Ensure StandardLoggerAdapter implements Logger
var _ Logger = (*StandardLoggerAdapter)(nil)
