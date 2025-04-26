// Package progress provides utilities for reporting progress in MCP tool handlers.
package progress

import (
	"context"
	"fmt"

	"github.com/localrivet/gomcp/protocol"
	// "github.com/localrivet/gomcp/server" // No longer needed directly
	"github.com/localrivet/gomcp/types" // Import types for ClientSession
)

// ProgressReporter helps report progress in tool handlers.
// It needs access to the server instance to send progress notifications.
type ProgressReporter struct {
	token   *protocol.ProgressToken
	server  ServerLogic         // Use an interface for the server dependency
	session types.ClientSession // Use types.ClientSession
	ctx     context.Context
}

// ServerLogic defines the methods the ProgressReporter needs from the server.
type ServerLogic interface {
	SendProgress(sessionID string, params protocol.ProgressParams) error
}

// NewProgressReporter creates a new progress reporter.
func NewProgressReporter(ctx context.Context, token *protocol.ProgressToken, server ServerLogic, session types.ClientSession) *ProgressReporter { // Use types.ClientSession and ServerLogic interface
	return &ProgressReporter{
		token:   token,
		server:  server, // Store the interface
		session: session,
		ctx:     ctx,
	}
}

// Report sends a progress update with the given message.
func (p *ProgressReporter) Report(message string) error {
	if p.token == nil {
		return nil
	}
	return p.server.SendProgress(p.session.SessionID(), protocol.ProgressParams{
		Token: string(*p.token),
		Value: message,
	})
}

// Reportf sends a progress update with a formatted message.
func (p *ProgressReporter) Reportf(format string, args ...interface{}) error {
	return p.Report(fmt.Sprintf(format, args...))
}

// ReportProgress sends a progress update with percentage.
func (p *ProgressReporter) ReportProgress(message string, percentage int) error {
	if p.token == nil {
		return nil
	}
	return p.server.SendProgress(p.session.SessionID(), protocol.ProgressParams{
		Token: string(*p.token),
		Value: map[string]interface{}{
			"message":    message,
			"percentage": percentage,
		},
	})
}
