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

// Debug logs a message at DEBUG level
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	if !l.IsLevelEnabled(protocol.LogLevelDebug) {
		return // Skip logging if debug level is not enabled
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("DEBUG: "+msg, args...)
}

// Info logs a message at INFO level
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	if !l.IsLevelEnabled(protocol.LogLevelInfo) {
		return // Skip logging if info level is not enabled
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("INFO: "+msg, args...)
}

// Warn logs a message at WARN level
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	if !l.IsLevelEnabled(protocol.LogLevelWarn) {
		return // Skip logging if warn level is not enabled
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("WARN: "+msg, args...)
}

// Error logs a message at ERROR level
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	// We always log errors regardless of level
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("ERROR: "+msg, args...)
}

// Helper function to determine if logging should occur at a given level
func isLevelEnabled(configuredLevel, msgLevel protocol.LoggingLevel) bool {
	// The client code uses this comparison:
	// if a.level >= protocol.LogLevelDebug { ... }

	// Convert string levels to numeric severity for comparison
	configuredSeverity := levelToSeverity(configuredLevel)
	msgSeverity := levelToSeverity(msgLevel)

	// Lower severity number = less restrictive = more messages
	// Higher severity number = more restrictive = fewer messages
	//
	// We should log the message if the configured level's
	// severity is less than or equal to the message level's severity
	return configuredSeverity <= msgSeverity
}

// Helper to map protocol level to an internal severity
// IMPORTANT: In client code "debug" is considered MORE permissive than "error"
// The comparison used in client is: if a.level >= protocol.LogLevelDebug { log... }
// So debug must be numerically higher than error for this to work
func levelToSeverity(level protocol.LoggingLevel) int {
	switch level {
	case protocol.LogLevelDebug:
		return 100 // Most permissive (logs everything)
	case protocol.LogLevelInfo:
		return 80
	case protocol.LogLevelNotice:
		return 70
	case protocol.LogLevelWarn:
		return 50
	case protocol.LogLevelError:
		return 40
	case protocol.LogLevelCritical:
		return 30
	case protocol.LogLevelAlert:
		return 20
	case protocol.LogLevelEmergency:
		return 10 // Least permissive (logs almost nothing)
	default:
		return 80 // Default to INFO level
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
	IsLevelEnabled(level protocol.LoggingLevel) bool
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
	if !a.IsLevelEnabled(protocol.LogLevelDebug) {
		return // Skip logging if debug level is not enabled
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.logger.Printf("DEBUG: "+format, v...)
}

// Info logs an info message
func (a *StandardLoggerAdapter) Info(format string, v ...interface{}) {
	if !a.IsLevelEnabled(protocol.LogLevelInfo) {
		return // Skip logging if info level is not enabled
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.logger.Printf("INFO: "+format, v...)
}

// Warn logs a warning message
func (a *StandardLoggerAdapter) Warn(format string, v ...interface{}) {
	if !a.IsLevelEnabled(protocol.LogLevelWarn) {
		return // Skip logging if warn level is not enabled
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.logger.Printf("WARN: "+format, v...)
}

// Error logs an error message
func (a *StandardLoggerAdapter) Error(format string, v ...interface{}) {
	// We always log errors regardless of level
	a.mu.Lock()
	defer a.mu.Unlock()
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

// Implementation of IsLevelEnabled for DefaultLogger
func (l *DefaultLogger) IsLevelEnabled(level protocol.LoggingLevel) bool {
	configuredSeverity := levelToSeverity(l.level)
	msgSeverity := levelToSeverity(level)
	return configuredSeverity <= msgSeverity
}

// Implementation of IsLevelEnabled for StandardLoggerAdapter
func (a *StandardLoggerAdapter) IsLevelEnabled(level protocol.LoggingLevel) bool {
	configuredSeverity := levelToSeverity(a.level)
	msgSeverity := levelToSeverity(level)
	return configuredSeverity <= msgSeverity
}

// PrintLevelDebugInfo logs information about how log level severities work
// This is a helper for debugging purposes
func PrintLevelDebugInfo(logger *log.Logger) {
	if logger == nil {
		logger = log.New(os.Stderr, "[LogX Debug] ", log.LstdFlags)
	}

	logger.Printf("Log Level Severity Values (higher = more permissive):")
	logger.Printf("  DEBUG: %d", levelToSeverity(protocol.LogLevelDebug))
	logger.Printf("  INFO: %d", levelToSeverity(protocol.LogLevelInfo))
	logger.Printf("  NOTICE: %d", levelToSeverity(protocol.LogLevelNotice))
	logger.Printf("  WARN: %d", levelToSeverity(protocol.LogLevelWarn))
	logger.Printf("  ERROR: %d", levelToSeverity(protocol.LogLevelError))
	logger.Printf("  CRITICAL: %d", levelToSeverity(protocol.LogLevelCritical))
	logger.Printf("  ALERT: %d", levelToSeverity(protocol.LogLevelAlert))
	logger.Printf("  EMERGENCY: %d", levelToSeverity(protocol.LogLevelEmergency))
}
