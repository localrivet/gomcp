package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/gobwas/ws"
	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/websocket"
	"github.com/localrivet/gomcp/types"
)

// runWebsocketTransport is a placeholder for the WebSocket transport implementation.
// It should handle multiple connections internally.
// RunWebsocketTransport runs the MCP server using WebSocket as its transport.
func (tm *TransportManager) runWebsocketTransport(s *Server) error {
	s.logger.Info("Starting WebSocket transport on %s at path %s...", tm.websocketAddr, tm.websocketPath)

	mux := http.NewServeMux()
	mux.HandleFunc(tm.websocketPath, func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("Received WebSocket upgrade request from %s for path %s", r.RemoteAddr, r.URL.Path)

		// Upgrade connection using gobwas/ws
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			s.logger.Error("WebSocket upgrade failed for %s: %v", r.RemoteAddr, err)
			// ws.UpgradeHTTP handles writing the error response
			return
		}
		s.logger.Info("WebSocket connection established with %s", conn.RemoteAddr())

		// Create a unique connection ID
		connectionID := uuid.NewString()

		// Create the MCP WebSocket transport wrapper using the upgraded net.Conn
		// Use the main server logger for the transport options.
		transportOpts := types.TransportOptions{Logger: s.logger}
		mcpTransport := websocket.NewWebSocketTransport(conn, ws.StateServerSide, transportOpts)

		// Create a new websocketSession, including the server logger
		session := &websocketSession{
			connectionID: connectionID,
			transport:    mcpTransport,
			logger:       s.logger, // Pass the server logger
		}

		// Session registration is deferred until after initialize
		// tm.sessionsMu.Lock()
		// tm.Sessions[session.SessionID()] = session
		// tm.sessionsMu.Unlock()
		// s.logger.Info("Registered WebSocket session %s for %s", session.SessionID(), conn.RemoteAddr())

		// Start the message handling loop for this session in a new goroutine
		go func() {
			initialized := false // Track if initialize succeeded
			defer func() {
				s.logger.Info("Removing WebSocket session %s for %s", session.SessionID(), conn.RemoteAddr())
				tm.RemoveSession(session.SessionID()) // This also removes capabilities
				s.SubscriptionManager.UnsubscribeAll(session.SessionID())
				session.Close() // Ensure the session (and underlying connection) is closed
			}()

			ctx := context.Background() // Base context for message handling loop

			for {
				// Receive raw message using the transport
				rawMessage, err := session.transport.Receive(ctx)
				if err != nil {
					// Check for expected closure errors
					if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || err.Error() == "transport is closed" {
						s.logger.Info("WebSocket session %s closed.", session.SessionID())
					} else {
						// Log unexpected errors
						s.logger.Error("WebSocket session %s receive error: %v", session.SessionID(), err)
					}
					break // Exit loop on any error or closure
				}

				if len(rawMessage) == 0 {
					s.logger.Warn("WebSocket session %s received empty message, skipping.", session.SessionID())
					continue
				}

				// Handle the first message specifically: it MUST be initialize
				if !initialized {
					var req protocol.JSONRPCRequest
					if err := json.Unmarshal(rawMessage, &req); err != nil || req.Method != protocol.MethodInitialize {
						s.logger.Error("WebSocket session %s: First message was not a valid initialize request: %v", session.SessionID(), err)
						// Send error and close? Or just close?
						errorResponse := protocol.NewErrorResponse(req.ID, protocol.CodeInvalidRequest, fmt.Sprintf("Invalid Request: %v", err), nil)
						_ = session.SendResponse(*errorResponse) // Ignore error during forced close
						break                                    // Exit loop, defer will close
					}

					var params protocol.InitializeRequestParams
					if err := json.Unmarshal(req.Params.(json.RawMessage), &params); err != nil {
						s.logger.Error("WebSocket session %s: Failed to unmarshal initialize params: %v", session.SessionID(), err)
						errorResponse := protocol.NewErrorResponse(req.ID, protocol.CodeInvalidParams, fmt.Sprintf("Invalid initialize parameters: %v", err), nil)
						_ = session.SendResponse(*errorResponse) // Ignore error during forced close
						break                                    // Exit loop
					}

					// Call InitializeHandler
					result, caps, initErr := s.MessageHandler.lifecycleHandler.InitializeHandler(params)
					if initErr != nil {
						s.logger.Error("WebSocket session %s: InitializeHandler failed: %v", session.SessionID(), initErr)
						// Send error response
						mcpErr, _ := initErr.(*protocol.MCPError)
						errorResponse := protocol.NewErrorResponse(req.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
						_ = session.SendResponse(*errorResponse) // Ignore error
						break                                    // Exit loop
					}

					// Send success response
					successResponse := protocol.NewSuccessResponse(req.ID, result)
					if err := session.SendResponse(*successResponse); err != nil {
						s.logger.Error("WebSocket session %s: Failed to send initialize success response: %v", session.SessionID(), err)
						break // Exit loop
					}

					// Store negotiated version and capabilities in the session
					session.SetNegotiatedVersion(result.ProtocolVersion)
					session.StoreClientCapabilities(*caps)

					// NOW register the fully initialized session with TransportManager
					tm.RegisterSession(session, caps)
					s.logger.Info("Registered fully initialized WebSocket session %s for %s", session.SessionID(), conn.RemoteAddr())
					initialized = true // Mark as initialized
					continue           // Go to next loop iteration to wait for next message
				}

				// Handle subsequent messages using the core server logic
				if handlerErr := s.MessageHandler.HandleMessage(session, rawMessage); handlerErr != nil {
					s.logger.Error("Error handling message from WebSocket session %s: %v", session.SessionID(), handlerErr)
					// Error responses are handled within MessageHandler now
				}
			}
			s.logger.Info("WebSocket message handling loop finished for session %s", session.SessionID())
		}() // End of goroutine
	}) // End of HandleFunc

	// Start the HTTP server
	httpServer := &http.Server{
		Addr:    tm.websocketAddr,
		Handler: mux,
		// TODO: Set appropriate timeouts for WebSocket if needed
	}

	s.logger.Info("WebSocket HTTP server listening on %s...", tm.websocketAddr)
	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed { // Ignore ErrServerClosed on graceful shutdown
		s.logger.Error("WebSocket HTTP server ListenAndServe error: %v", err)
	}
	return err // Return the error from ListenAndServe
}

// websocketSession represents a WebSocket client session and implements the types.ClientSession interface.
type websocketSession struct {
	connectionID string
	transport    *websocket.WebSocketTransport // Use the transport package's type
	initialized  atomic.Bool
	// Add fields for negotiated version and client capabilities if needed
	negotiatedVersion  string
	clientCapabilities protocol.ClientCapabilities
	logger             types.Logger // Add logger
}

// Ensure websocketSession implements the types.ClientSession interface
var _ types.ClientSession = (*websocketSession)(nil)

func (s *websocketSession) SessionID() string {
	return s.connectionID
}

// SendNotification sends a JSON-RPC notification to the WebSocket client.
func (s *websocketSession) SendNotification(notification protocol.JSONRPCNotification) error {
	// Marshal the notification to JSON
	data, err := json.Marshal(notification)
	if err != nil {
		s.logger.Error("WebSocket Session %s: Error marshalling notification %s: %v", s.connectionID, notification.Method, err)
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	// Use the WebSocket transport to send the raw JSON data
	return s.transport.Send(context.Background(), data) // Use context.Background() for now, refine later if needed
}

// SendResponse sends a JSON-RPC response to the WebSocket client.
func (s *websocketSession) SendResponse(response protocol.JSONRPCResponse) error {
	// Marshal the response to JSON
	data, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("WebSocket Session %s: Error marshalling response for ID %v: %v", s.connectionID, response.ID, err)
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	// Use the WebSocket transport to send the raw JSON data
	return s.transport.Send(context.Background(), data) // Use context.Background() for now, refine later if needed
}

// Close signals the WebSocket session to terminate.
func (s *websocketSession) Close() error {
	// Use the WebSocket transport to close the connection
	s.logger.Info("WebSocket Session %s: Close called.", s.connectionID)
	return s.transport.Close()
}

func (s *websocketSession) Initialize() {
	s.initialized.Store(true)
	s.logger.Info("WebSocket Session %s: Initialized.", s.connectionID)
}

func (s *websocketSession) Initialized() bool {
	return s.initialized.Load()
}

func (s *websocketSession) SetNegotiatedVersion(version string) {
	s.negotiatedVersion = version
	s.logger.Info("WebSocket Session %s: Negotiated protocol version set to %s", s.connectionID, version)
}

func (s *websocketSession) GetNegotiatedVersion() string {
	return s.negotiatedVersion
}

func (s *websocketSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	s.clientCapabilities = caps
	s.logger.Info("WebSocket Session %s stored client capabilities", s.connectionID)
}

func (s *websocketSession) GetClientCapabilities() protocol.ClientCapabilities {
	return s.clientCapabilities
}

// SendRequest marshals the request and sends it over the WebSocket.
func (m *websocketSession) SendRequest(request protocol.JSONRPCRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		m.logger.Error("websocketSession SendRequest: Failed to marshal request ID %v: %v", request.ID, err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	// Use the underlying transport's Send method
	// Use context.Background() as there's no specific parent request context here
	return m.transport.Send(context.Background(), data)
}

// GetWriter returns the underlying writer.
func (s *websocketSession) GetWriter() io.Writer {
	// Return the underlying writer from the WebSocket transport
	return s.transport.GetWriter() // Expose the underlying net.Conn
}
