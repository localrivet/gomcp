// Package progress provides utilities for reporting progress in MCP tool handlers.
package progress

import (
	"context"
	"fmt"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types" // Import types for ClientSession
)

// ProgressReporter helps report progress in tool handlers.
// It needs access to the server instance to send progress notifications.
type ProgressReporter struct {
	token   interface{}         // Changed to interface{}
	server  ServerLogic         // Use an interface for the server dependency
	session types.ClientSession // Use types.ClientSession
	ctx     context.Context
}

// ServerLogic defines the methods the ProgressReporter needs from the server.
type ServerLogic interface {
	SendProgress(sessionID string, params protocol.ProgressParams) error
}

// NewProgressReporter creates a new progress reporter.
// token is now interface{}
func NewProgressReporter(ctx context.Context, token interface{}, server ServerLogic, session types.ClientSession) *ProgressReporter { // Use types.ClientSession and ServerLogic interface
	return &ProgressReporter{
		token:   token,  // Store interface{}
		server:  server, // Store the interface
		session: session,
		ctx:     ctx,
	}
}

// Report sends a progress update with the given message.
func (p *ProgressReporter) Report(message string) error {
	tokenStr, ok := p.getTokenAsString()
	if !ok {
		return nil // No token or invalid type
	}
	return p.server.SendProgress(p.session.SessionID(), protocol.ProgressParams{
		Token: tokenStr,
		Value: message,
	})
}

// Reportf sends a progress update with a formatted message.
func (p *ProgressReporter) Reportf(format string, args ...interface{}) error {
	return p.Report(fmt.Sprintf(format, args...))
}

// ReportProgress sends a progress update with percentage.
func (p *ProgressReporter) ReportProgress(message string, percentage int) error {
	tokenStr, ok := p.getTokenAsString()
	if !ok {
		return nil // No token or invalid type
	}
	return p.server.SendProgress(p.session.SessionID(), protocol.ProgressParams{
		Token: tokenStr,
		Value: map[string]interface{}{
			"message":    message,
			"percentage": percentage,
		},
	})
}

// getTokenAsString converts the stored interface{} token to a string.
// It handles nil, string, and numeric types according to the spec.
func (p *ProgressReporter) getTokenAsString() (string, bool) {
	if p.token == nil {
		return "", false
	}
	switch v := p.token.(type) {
	case string:
		return v, true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		// Convert numeric types to string
		return fmt.Sprintf("%v", v), true
	default:
		// Invalid type according to spec
		// Log this? For now, just return false.
		return "", false
	}
}
