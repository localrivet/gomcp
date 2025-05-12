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
		level:  protocol.LogLevelInfo, // Default to INFO level
	}
}

// NewLogger creates a new logger instance based on the configuration.
// logType can be a log level string like "debug", "info", "warning", "error"
// or a custom prefix string for backward compatibility
func NewLogger(logType string) Logger {
	logger := &DefaultLogger{
		logger: log.New(os.Stderr, "[GoMCP] ", log.LstdFlags|log.Ltime|log.Lmsgprefix),
		level:  protocol.LogLevelInfo, // Default to INFO level
	}

	// Try to parse logType as a log level
	switch logType {
	case "debug":
		logger.level = protocol.LogLevelDebug
	case "info":
		logger.level = protocol.LogLevelInfo
	case "warning", "warn":
		logger.level = protocol.LogLevelWarn
	case "error":
		logger.level = protocol.LogLevelError
	}

	return logger
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if isLevelEnabled(l.level, protocol.LogLevelDebug) {
		l.logger.Printf("DEBUG: "+msg, args...)
	}
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if isLevelEnabled(l.level, protocol.LogLevelInfo) {
		l.logger.Printf("INFO: "+msg, args...)
	}
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if isLevelEnabled(l.level, protocol.LogLevelWarn) {
		l.logger.Printf("WARN: "+msg, args...)
	}
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Always log errors regardless of level
	l.logger.Printf("ERROR: "+msg, args...)
}

// Helper function to determine if logging should occur at a given level
func isLevelEnabled(configuredLevel, msgLevel protocol.LoggingLevel) bool {
	// Order of levels from least to most severe:
	// debug, info, notice, warning, error, critical, alert, emergency

	switch configuredLevel {
	case protocol.LogLevelDebug:
		// If configured for debug, log everything
		return true
	case protocol.LogLevelInfo:
		// If configured for info, don't log debug
		return msgLevel != protocol.LogLevelDebug
	case protocol.LogLevelWarn:
		// If configured for warning, only log warning and more severe
		return msgLevel != protocol.LogLevelDebug &&
			msgLevel != protocol.LogLevelInfo &&
			msgLevel != protocol.LogLevelNotice
	case protocol.LogLevelError, protocol.LogLevelCritical,
		protocol.LogLevelAlert, protocol.LogLevelEmergency:
		// If configured for error or more severe, only log that level and above
		return msgLevel == protocol.LogLevelError ||
			msgLevel == protocol.LogLevelCritical ||
			msgLevel == protocol.LogLevelAlert ||
			msgLevel == protocol.LogLevelEmergency
	default:
		// Default to info level behavior for unknown levels
		return msgLevel != protocol.LogLevelDebug
	}
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

	// Set the new level
	l.level = level
	l.logger.Printf("[LogX] Log level set to: %s", string(l.level))
}

// SetLogLevelFromString sets the logging level from a string representation
// This is a utility function to help external callers set the log level
func SetLogLevelFromString(logger Logger, levelStr string) {
	var level protocol.LoggingLevel

	switch levelStr {
	case "debug":
		level = protocol.LogLevelDebug
	case "info":
		level = protocol.LogLevelInfo
	case "notice":
		level = protocol.LogLevelNotice
	case "warn", "warning":
		level = protocol.LogLevelWarn
	case "error":
		level = protocol.LogLevelError
	case "critical":
		level = protocol.LogLevelCritical
	case "alert":
		level = protocol.LogLevelAlert
	case "emergency":
		level = protocol.LogLevelEmergency
	default:
		// Default to INFO for unknown level strings
		level = protocol.LogLevelInfo
	}

	logger.SetLevel(level)
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
	a.mu.Lock()
	defer a.mu.Unlock()
	if isLevelEnabled(a.level, protocol.LogLevelDebug) {
		a.logger.Printf("DEBUG: "+format, v...)
	}
}

// Info logs an info message
func (a *StandardLoggerAdapter) Info(format string, v ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if isLevelEnabled(a.level, protocol.LogLevelInfo) {
		a.logger.Printf("INFO: "+format, v...)
	}
}

// Warn logs a warning message
func (a *StandardLoggerAdapter) Warn(format string, v ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if isLevelEnabled(a.level, protocol.LogLevelWarn) {
		a.logger.Printf("WARN: "+format, v...)
	}
}

// Error logs an error message
func (a *StandardLoggerAdapter) Error(format string, v ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Always log errors regardless of level
	a.logger.Printf("ERROR: "+format, v...)
}

// SetLevel sets the logging level
func (a *StandardLoggerAdapter) SetLevel(level protocol.LoggingLevel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.level = level
	a.logger.Printf("[LogX] Log level set to: %s", string(level))
}

// Ensure StandardLoggerAdapter implements Logger
var _ Logger = (*StandardLoggerAdapter)(nil)
