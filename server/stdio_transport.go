package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// stdioSession represents a StdIO client session and implements the types.ClientSession interface.
type stdioSession struct {
	connectionID string
	writer       io.Writer
	initialized  atomic.Bool
	// Add fields for negotiated version and client capabilities if needed by StdIO transport
	negotiatedVersion  string
	clientCapabilities protocol.ClientCapabilities
	logger             types.Logger
	stdioTransport     *stdio.StdioTransport
}

// Ensure stdioSession implements the types.ClientSession interface
var _ types.ClientSession = (*stdioSession)(nil)

func (s *stdioSession) SessionID() string {
	return s.connectionID
}

// SendNotification sends a JSON-RPC notification to the StdIO client.
func (s *stdioSession) SendNotification(notification protocol.JSONRPCNotification) error {
	// For StdIO, format as Content-Length framed message
	data, err := json.Marshal(notification)
	if err != nil {
		log.Printf("StdIO Session %s: Error marshalling notification %s: %v", s.connectionID, notification.Method, err)
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	headers := fmt.Sprintf("Content-Length: %d\r\n", len(data))
	headers += "Content-Type: application/json\r\n" // Assuming JSON messages
	fullMessage := append([]byte(headers), '\r', '\n')
	fullMessage = append(fullMessage, data...)

	_, err = s.writer.Write(fullMessage)
	if err != nil {
		log.Printf("StdIO Session %s: Error writing notification to stdout: %v", s.connectionID, err)
		return fmt.Errorf("error writing notification to stdout: %w", err)
	}
	// Ensure the data is flushed immediately for interactive use
	if f, ok := s.writer.(*os.File); ok {
		f.Sync()
	}
	log.Printf("StdIO Session %s: Sent notification %s", s.connectionID, notification.Method)
	return nil
}

// SendResponse sends a JSON-RPC response to the StdIO client.
func (s *stdioSession) SendResponse(response protocol.JSONRPCResponse) error {
	// For StdIO, format as Content-Length framed message
	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("StdIO Session %s: Error marshalling response for ID %v: %v", s.connectionID, response.ID, err)
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	headers := fmt.Sprintf("Content-Length: %d\r\n", len(data))
	headers += "Content-Type: application/json\r\n" // Assuming JSON messages
	fullMessage := append([]byte(headers), '\r', '\n')
	fullMessage = append(fullMessage, data...)

	_, err = s.writer.Write(fullMessage)
	if err != nil {
		log.Printf("StdIO Session %s: Error writing response to stdout: %v", s.connectionID, err)
		return fmt.Errorf("error writing response to stdout: %w", err)
	}
	// Ensure the data is flushed immediately for interactive use
	if f, ok := s.writer.(*os.File); ok {
		f.Sync()
	}
	log.Printf("StdIO Session %s: Sent response for ID %v", s.connectionID, response.ID)
	return nil
}

// Close signals the StdIO session to terminate.
func (s *stdioSession) Close() error {
	// For StdIO, closing the session might involve closing stdin/stdout,
	// but typically the process exit handles this. This method might be
	// used for cleanup or signaling within the server.
	log.Printf("StdIO Session %s: Close called.", s.connectionID)
	// In a real scenario, you might close the underlying writer if it's closable.
	// if closer, ok := s.writer.(io.Closer); ok {
	// return closer.Close()
	// }
	return nil // Nothing to close for os.Stdout
}

func (s *stdioSession) Initialize() {
	s.initialized.Store(true)
	log.Printf("StdIO Session %s: Initialized.", s.connectionID)
}

func (s *stdioSession) Initialized() bool {
	return s.initialized.Load()
}

func (s *stdioSession) SetNegotiatedVersion(version string) {
	s.negotiatedVersion = version
	log.Printf("StdIO Session %s: Negotiated protocol version set to %s", s.connectionID, version)
}

func (s *stdioSession) GetNegotiatedVersion() string {
	return s.negotiatedVersion
}

func (s *stdioSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	s.clientCapabilities = caps
	log.Printf("StdIO Session %s stored client capabilities", s.connectionID)
}

func (s *stdioSession) GetClientCapabilities() protocol.ClientCapabilities {
	return s.clientCapabilities
}

// NewStdioSession creates a new stdioSession instance.
func NewStdioSession(connectionID string, writer io.Writer, logger types.Logger, transport *stdio.StdioTransport) *stdioSession {
	return &stdioSession{
		connectionID:   connectionID,
		writer:         writer,
		logger:         logger,
		stdioTransport: transport,
	}
}

// SendRequest marshals the request and sends it over Stdout via the StdioTransport.
func (m *stdioSession) SendRequest(request protocol.JSONRPCRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		m.logger.Error("stdioSession SendRequest: Failed to marshal request ID %v: %v", request.ID, err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	// Use the StdioTransport's Send method
	// Use context.Background() as there's no specific parent request context here
	return m.stdioTransport.Send(context.Background(), data)
}

// GetWriter returns the underlying writer.
func (s *stdioSession) GetWriter() io.Writer {
	return s.writer
}

// runStdioTransport runs the MCP server using standard I/O.
func runStdioTransport(tm *TransportManager, s *Server) error {
	s.logger.Info("Starting StdIO transport...")

	// Create the underlying StdioTransport
	// Use NewStdioTransportWithReadWriter and types.TransportOptions
	stdioTransport := stdio.NewStdioTransportWithReadWriter(os.Stdin, os.Stdout, types.TransportOptions{
		Logger: s.logger,
	})

	// Create the stdioSession wrapper, passing the transport
	session := NewStdioSession(
		"stdio-connection", // Fixed ID for StdIO
		os.Stdout,          // Pass the original writer (session doesn't write directly)
		s.logger,
		stdioTransport, // Pass the created transport
	)

	// Store the StdIO session in the TransportManager - DEFER THIS
	// tm.sessionsMu.Lock()
	// tm.Sessions[session.SessionID()] = session
	// tm.sessionsMu.Unlock()
	// s.logger.Info("StdIO session registered: %s", session.SessionID())

	// Ensure session is removed on exit
	defer func() {
		tm.RemoveSession(session.SessionID()) // This removes session and capabilities
		s.SubscriptionManager.UnsubscribeAll(session.SessionID())
		s.logger.Info("StdIO session removed: %s", session.SessionID())
	}()

	reader := bufio.NewReader(os.Stdin)
	initialized := false // Track if initialize succeeded

	for {
		// Read headers (Content-Length and Content-Type)
		headers := make(map[string]string)
		contentLength := -1
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					s.logger.Info("StdIO transport: EOF received, closing connection.")
					// Deferred function will handle cleanup
					return nil // Graceful shutdown
				}
				s.logger.Error("StdIO transport: Error reading header from stdin for session %s: %v", session.SessionID(), err)
				// Deferred function will handle cleanup
				return fmt.Errorf("error reading header from stdin for session %s: %w", session.SessionID(), err)
			}
			line = string(bytes.TrimSpace([]byte(line))) // Correctly convert []byte to string
			if line == "" {
				break // End of headers
			}

			parts := splitHeader(line)
			if len(parts) == 2 {
				headers[parts[0]] = parts[1]
				if parts[0] == "Content-Length" {
					fmt.Sscan(parts[1], &contentLength)
				}
			}
		}

		if contentLength == -1 {
			s.logger.Warn("StdIO transport: Missing Content-Length header for session %s", session.SessionID())
			// TODO: Send a parse error response?
			continue // Skip this message
		}

		// Read message body
		message := make([]byte, contentLength)
		_, err := io.ReadFull(reader, message)
		if err != nil {
			if err == io.EOF {
				s.logger.Info("StdIO transport: EOF received while reading body, closing connection.")
				// Deferred function will handle cleanup
				return nil // Graceful shutdown
			}
			s.logger.Error("StdIO transport: Error reading body from stdin for session %s: %v", session.SessionID(), err)
			// TODO: Send a parse error response?
			// Deferred function will handle cleanup
			return fmt.Errorf("error reading body from stdin for session %s: %w", session.SessionID(), err)
		}

		// Handle the first message specifically: it MUST be initialize
		if !initialized {
			var req protocol.JSONRPCRequest
			if err := json.Unmarshal(message, &req); err != nil || req.Method != protocol.MethodInitialize {
				s.logger.Error("StdIO session %s: First message was not a valid initialize request: %v", session.SessionID(), err)
				// Send error and close?
				errorResponse := protocol.NewErrorResponse(req.ID, protocol.CodeInvalidRequest, fmt.Sprintf("First message must be 'initialize'"), nil)
				_ = session.SendResponse(*errorResponse)              // Ignore error during forced exit
				return fmt.Errorf("first message was not initialize") // Exit
			}

			var params protocol.InitializeRequestParams
			if err := json.Unmarshal(req.Params.(json.RawMessage), &params); err != nil {
				s.logger.Error("StdIO session %s: Failed to unmarshal initialize params: %v", session.SessionID(), err)
				errorResponse := protocol.NewErrorResponse(req.ID, protocol.CodeInvalidParams, fmt.Sprintf("Invalid initialize parameters: %v", err), nil)
				_ = session.SendResponse(*errorResponse)                            // Ignore error during forced exit
				return fmt.Errorf("failed to unmarshal initialize params: %w", err) // Exit
			}

			// Call InitializeHandler
			result, caps, initErr := s.MessageHandler.lifecycleHandler.InitializeHandler(params)
			if initErr != nil {
				s.logger.Error("StdIO session %s: InitializeHandler failed: %v", session.SessionID(), initErr)
				// Send error response
				mcpErr, _ := initErr.(*protocol.MCPError)
				errorResponse := protocol.NewErrorResponse(req.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
				_ = session.SendResponse(*errorResponse)                    // Ignore error
				return fmt.Errorf("initialize handler failed: %w", initErr) // Exit
			}

			// Send success response
			successResponse := protocol.NewSuccessResponse(req.ID, result)
			if err := session.SendResponse(*successResponse); err != nil {
				s.logger.Error("StdIO session %s: Failed to send initialize success response: %v", session.SessionID(), err)
				return fmt.Errorf("failed to send initialize response: %w", err) // Exit
			}

			// Store negotiated version and capabilities in the session
			session.SetNegotiatedVersion(result.ProtocolVersion)
			session.StoreClientCapabilities(*caps)

			// NOW register the fully initialized session with TransportManager
			tm.RegisterSession(session, caps) // Pass the pointer directly
			s.logger.Info("Registered fully initialized StdIO session %s", session.SessionID())
			initialized = true // Mark as initialized
			continue           // Go to next loop iteration to wait for next message
		}

		// Handle subsequent messages in a goroutine
		go func(msg []byte) {
			// Call the MessageHandler with the session
			if handlerErr := s.MessageHandler.HandleMessage(session, msg); handlerErr != nil {
				s.logger.Error("Error handling message from session %s: %v", session.SessionID(), handlerErr)
				// Error responses are handled within MessageHandler now
			}
		}(message)
	}
}

// splitHeader splits a header line into key and value.
func splitHeader(line string) []string {
	parts := make([]string, 0, 2)
	keyEnd := -1
	for i := 0; i < len(line); i++ {
		if line[i] == ':' {
			keyEnd = i
			break
		}
	}
	if keyEnd == -1 {
		return parts // Invalid header line
	}
	parts = append(parts, line[:keyEnd])
	valueStart := keyEnd + 1
	for valueStart < len(line) && (line[valueStart] == ' ' || line[valueStart] == '\t') {
		valueStart++
	}
	parts = append(parts, line[valueStart:])
	return parts
}
