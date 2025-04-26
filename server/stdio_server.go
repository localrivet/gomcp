package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// --- Stdio Helper ---

// stdioSession provides a basic implementation of ClientSession for stdio.
// It's kept internal to the server package as it's specific to ServeStdio.
type stdioSession struct {
	id                 string
	transport          types.Transport
	clientCapabilities protocol.ClientCapabilities // Store capabilities if needed
	negotiatedVersion  string                      // Store version if needed
	initMu             sync.RWMutex                // Mutex for safe access
	initialized        bool
	logger             types.Logger // Add logger for internal use
}

func newStdioSession(id string, transport types.Transport, logger types.Logger) *stdioSession {
	return &stdioSession{
		id:        id,
		transport: transport,
		logger:    logger,
	}
}

func (s *stdioSession) SessionID() string { return s.id }
func (s *stdioSession) SendNotification(notification protocol.JSONRPCNotification) error {
	msg, err := json.Marshal(notification)
	if err != nil {
		s.logger.Error("StdioSession %s: Error marshaling notification: %v", s.id, err)
		return err
	}
	return s.transport.Send(msg)
}
func (s *stdioSession) SendResponse(response protocol.JSONRPCResponse) error {
	msg, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("StdioSession %s: Error marshaling response: %v", s.id, err)
		return err
	}
	return s.transport.Send(msg)
}
func (s *stdioSession) Close() error      { return nil } // No-op for stdio
func (s *stdioSession) Initialize()       { s.initialized = true }
func (s *stdioSession) Initialized() bool { return s.initialized }
func (s *stdioSession) StoreClientCapabilities(caps protocol.ClientCapabilities) { /* Store if needed */
}
func (s *stdioSession) GetClientCapabilities() protocol.ClientCapabilities {
	return s.clientCapabilities
}
func (s *stdioSession) SetNegotiatedVersion(version string) { s.negotiatedVersion = version }
func (s *stdioSession) GetNegotiatedVersion() string        { return s.negotiatedVersion }

// Ensure stdioSession implements the types.ClientSession interface
var _ types.ClientSession = (*stdioSession)(nil)

// ServeStdio runs the MCP server, listening for messages on standard input
// and sending responses/notifications to standard output. It blocks until
// the input stream is closed, an error occurs, or the context is cancelled
// (e.g., by SIGINT).
func ServeStdio(srv *Server) error {
	logger := srv.logger // Use the server's configured logger
	logger.Info("Starting server on stdio...")

	// Create stdio transport
	transport := stdio.NewStdioTransport() // Assumes stdio package is imported

	// Create and register the internal stdio session
	session := newStdioSession("stdio-session", transport, logger)
	if err := srv.RegisterSession(session); err != nil {
		logger.Error("ServeStdio: Failed to register session: %v", err)
		return fmt.Errorf("failed to register session: %w", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	// Set up context cancellation for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt) // Assumes os and os/signal packages are imported
	defer stop()

	logger.Info("Server listening on stdio. Press Ctrl+C to exit.")

	// Run the main message loop
	for {
		select {
		case <-ctx.Done():
			logger.Info("ServeStdio: Context cancelled, shutting down...")
			return ctx.Err()
		default:
			// Receive raw message
			rawMsg, err := transport.ReceiveWithContext(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) { // Assumes io package is imported
					logger.Info("ServeStdio: Input closed (EOF), shutting down...")
					return nil // Clean shutdown on EOF
				}
				if errors.Is(err, context.Canceled) { // Assumes errors package is imported
					logger.Info("ServeStdio: Context cancelled during receive, shutting down...")
					return nil // Clean shutdown on context cancellation
				}
				logger.Error("ServeStdio: Error receiving message: %v", err)
				// Decide whether to continue or return based on error severity
				// For stdio, most receive errors are likely fatal.
				return fmt.Errorf("error receiving message: %w", err)
			}

			// Let the server handle the message
			responses := srv.HandleMessage(ctx, session.SessionID(), rawMsg)

			// Send responses
			for _, response := range responses {
				if response == nil {
					continue
				}
				respBytes, err := json.Marshal(response) // Use standard json.Marshal
				if err != nil {
					logger.Error("ServeStdio: Error marshaling response: %v", err)
					continue // Skip this response, but log the error
				}

				if err := transport.Send(respBytes); err != nil {
					logger.Error("ServeStdio: Error sending response: %v", err)
					// If sending fails, it's likely fatal for stdio
					return fmt.Errorf("failed to send response: %w", err)
				}
			}
		}
	}
}
