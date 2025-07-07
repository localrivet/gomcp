// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
)

// Connect establishes a connection to the server.
func (c *clientImpl) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// If no transport has been set, select an appropriate one based on the URL
	if c.transport == nil {
		// Select transport based on URL scheme
		url := c.url
		switch {
		case url == "stdio:///" || url == "stdio://" || url == "stdio:":
			WithStdio()(c)
		case len(url) > 5 && url[:5] == "http:":
			WithHTTP(url)(c)
		case len(url) > 6 && url[:6] == "https:":
			WithHTTP(url)(c)
		case len(url) > 3 && url[:3] == "ws:":
			WithWebsocket(url)(c)
		case len(url) > 4 && url[:4] == "wss:":
			WithWebsocket(url)(c)
		case len(url) > 4 && url[:4] == "sse:":
			WithSSE(url)(c)
		case len(url) > 8 && url[:8] == "unix:///":
			WithUnixSocket(url[8:])(c)
		default:
			// Default to stdio for non-URL patterns (client names, empty strings, etc.)
			// This is the most common MCP transport and matches user expectations
			WithStdio()(c)
			c.logger.Debug("no explicit transport configured, defaulting to stdio", "url", url)
		}
	}

	// Set the timeout on the transport
	c.transport.SetConnectionTimeout(c.connectionTimeout)
	c.transport.SetRequestTimeout(c.requestTimeout)

	// Connect to the server
	if err := c.transport.Connect(); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.connected = true

	// Initialize the connection by negotiating the protocol version
	if err := c.initialize(); err != nil {
		if disconnectErr := c.transport.Disconnect(); disconnectErr != nil {
			slog.Default().Error("Failed to disconnect transport after initialization failure", "error", disconnectErr)
		}
		c.connected = false
		return fmt.Errorf("failed to initialize connection: %w", err)
	}

	return nil
}

// initialize performs the initial version negotiation with the server.
func (c *clientImpl) initialize() error {
	// Determine which protocol version to send
	var protocolVersion string

	// If a negotiated version was already set (via WithProtocolVersion),
	// use that single version
	if c.negotiatedVersion != "" {
		protocolVersion = c.negotiatedVersion
	} else {
		// Otherwise use the default (most preferred) version
		protocolVersion = c.versionDetector.DefaultVersion
	}

	// Create the initialize request
	requestID := c.generateRequestID()
	params := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities":    c.capabilities,
		"clientInfo": map[string]interface{}{
			"name":    "GoMCP Client",
			"version": "1.0.0",
		},
	}
	initRequest := mcp.NewRequest(requestID, "initialize", params)

	// Convert the request to JSON
	requestJSON, err := initRequest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	// Send the request to the server
	ctx, cancel := context.WithTimeout(c.ctx, c.connectionTimeout)
	defer cancel()

	responseJSON, err := c.transport.SendWithContext(ctx, requestJSON)
	if err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string                 `json:"jsonrpc"`
		ID      int64                  `json:"id"`
		Result  map[string]interface{} `json:"result,omitempty"`
		Error   *struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	// Check for error response
	if response.Error != nil {
		return fmt.Errorf("server returned error: %s (code %d)", response.Error.Message, response.Error.Code)
	}

	// Extract the negotiated protocol version
	serverProtocolVersion, ok := response.Result["protocolVersion"].(string)
	if !ok {
		return errors.New("server did not provide a protocol version")
	}

	// Validate the protocol version
	if _, err := c.versionDetector.ValidateVersion(serverProtocolVersion); err != nil {
		return fmt.Errorf("server returned invalid protocol version: %w", err)
	}

	c.negotiatedVersion = serverProtocolVersion

	// Extract and store server capabilities
	if capabilitiesData, exists := response.Result["capabilities"]; exists {
		if capabilitiesJSON, err := json.Marshal(capabilitiesData); err == nil {
			var serverCapabilities ServerCapabilities
			if err := json.Unmarshal(capabilitiesJSON, &serverCapabilities); err == nil {
				c.serverCapabilities = &serverCapabilities
			} else {
				c.logger.Warn("failed to parse server capabilities", "error", err)
			}
		}
	}

	// Extract and store server info
	if serverInfoData, exists := response.Result["serverInfo"]; exists {
		if serverInfoJSON, err := json.Marshal(serverInfoData); err == nil {
			var serverInfo ServerInfo
			if err := json.Unmarshal(serverInfoJSON, &serverInfo); err == nil {
				c.serverInfo = &serverInfo
			} else {
				c.logger.Warn("failed to parse server info", "error", err)
			}
		}
	}

	// Extract server instructions (2025-03-26 only)
	if instructions, exists := response.Result["instructions"]; exists {
		if instructionsStr, ok := instructions.(string); ok {
			c.serverInstructions = instructionsStr
		}
	}

	c.initialized = true

	c.logger.Info("initialized client connection",
		"url", c.url,
		"protocolVersion", c.negotiatedVersion,
		"serverName", func() string {
			if c.serverInfo != nil {
				return c.serverInfo.Name
			}
			return "unknown"
		}(),
		"serverVersion", func() string {
			if c.serverInfo != nil {
				return c.serverInfo.Version
			}
			return "unknown"
		}())

	// Send initialized notification
	if err := c.sendInitializedNotification(); err != nil {
		c.logger.Warn("failed to send initialized notification", "error", err)
		// We don't fail the initialization process if this fails
	}

	// Setup notification handler
	c.registerNotificationHandler()

	return nil
}

// sendInitializedNotification sends the initialized notification to the server.
func (c *clientImpl) sendInitializedNotification() error {
	// Create notification using structured type
	notification := mcp.NewNotification("notifications/initialized", nil)

	// Convert to JSON
	notificationJSON, err := notification.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal initialized notification: %w", err)
	}

	// Send the notification
	_, err = c.transport.Send(notificationJSON)
	if err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

// Close closes the client connection.
func (c *clientImpl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	var err error

	// Send a shutdown request if we're initialized
	if c.initialized {
		requestID := c.generateRequestID()
		shutdownRequest := mcp.NewRequest(requestID, "shutdown", nil)

		// Convert to JSON
		requestJSON, marshalErr := shutdownRequest.Marshal()
		if marshalErr != nil {
			c.logger.Error("failed to marshal shutdown request", "error", marshalErr)
		} else {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(c.ctx, c.connectionTimeout)
			defer cancel()

			// Send the request
			_, sendErr := c.transport.SendWithContext(ctx, requestJSON)
			if sendErr != nil {
				c.logger.Error("failed to send shutdown request", "error", sendErr)
			}
		}
	}

	// Disconnect from transport
	if err := c.transport.Disconnect(); err != nil {
		slog.Default().Error("Failed to disconnect transport", "error", err)
	}

	c.connected = false
	c.initialized = false

	// Emit client disconnected event
	go func() {
		if err := events.Publish[events.ClientDisconnectedEvent](c.events, events.TopicClientDisconnected, events.ClientDisconnectedEvent{
			URL: c.url,
		}); err != nil {
			c.logger.Warn("failed to publish client disconnected event", "error", err)
		}
	}()

	// Cancel the client context
	c.cancel()

	// If we have a server registry and server name, stop the server
	if c.serverRegistry != nil && c.serverName != "" {
		if stopErr := c.serverRegistry.StopServer(c.serverName); stopErr != nil {
			c.logger.Error("failed to stop server", "server", c.serverName, "error", stopErr)
			// Don't override the original error if there was one
			if err == nil {
				err = stopErr
			}
		}
	}

	return err
}

// registerNotificationHandler registers the client's notification handler.
func (c *clientImpl) registerNotificationHandler() {
	c.transport.RegisterNotificationHandler(func(method string, params []byte) {
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int64           `json:"id,omitempty"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}

		if err := json.Unmarshal(params, &request); err != nil {
			c.logger.Error("failed to parse server message", "error", err)
			return
		}

		// Handle request methods
		if request.ID != 0 {
			switch request.Method {
			case "roots/list":
				if err := c.handleRootsList(request.ID); err != nil {
					c.logger.Error("failed to handle roots/list request", "error", err)
				}
			case "sampling/createMessage":
				if err := c.handleSamplingCreateMessage(request.ID, request.Params); err != nil {
					c.logger.Error("failed to handle sampling/createMessage request", "error", err)
				}
			default:
				c.logger.Warn("received unsupported request method", "method", request.Method)
				// Send method not found error
				errorResponse := mcp.NewErrorResponse(request.ID, -32601, "Method not found", nil)
				responseJSON, _ := errorResponse.Marshal()
				_, err := c.transport.Send(responseJSON)
				if err != nil {
					c.logger.Error("failed to send error response", "error", err)
				}
			}
			return
		}

		// Handle notification methods
		switch request.Method {
		// Handle server notifications here
		default:
			c.logger.Debug("received notification", "method", request.Method)
		}
	})
}
