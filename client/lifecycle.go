// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
			return errors.New("no transport configured, use WithTransport option")
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
		c.transport.Disconnect()
		c.connected = false
		return fmt.Errorf("failed to initialize connection: %w", err)
	}

	return nil
}

// initialize performs the initial version negotiation with the server.
func (c *clientImpl) initialize() error {
	// Determine which protocol version(s) to send
	var protocolVersion interface{}
	
	// If a negotiated version was already set (via WithProtocolVersion),
	// use that single version instead of the full array
	if c.negotiatedVersion != "" {
		protocolVersion = c.negotiatedVersion
	} else {
		// Otherwise use the full list of supported versions
		protocolVersion = c.versionDetector.Supported
	}
	
	// Create the initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.generateRequestID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    c.capabilities,
			"clientInfo": map[string]interface{}{
				"name":    "GoMCP Client",
				"version": "1.0.0",
			},
		},
	}

	// Convert the request to JSON
	requestJSON, err := json.Marshal(initRequest)
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
	protocolVersion, ok := response.Result["protocolVersion"].(string)
	if !ok {
		return errors.New("server did not provide a protocol version")
	}

	// Validate the protocol version
	if _, err := c.versionDetector.ValidateVersion(protocolVersion.(string)); err != nil {
		return fmt.Errorf("server returned invalid protocol version: %w", err)
	}

	c.negotiatedVersion = protocolVersion.(string)
	c.initialized = true

	c.logger.Info("initialized client connection",
		"url", c.url,
		"protocolVersion", c.negotiatedVersion)

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
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	// Convert to JSON
	notificationJSON, err := json.Marshal(notification)
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

	// Send a shutdown request if we're initialized
	if c.initialized {
		shutdownRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      c.generateRequestID(),
			"method":  "shutdown",
		}

		// Convert to JSON
		requestJSON, err := json.Marshal(shutdownRequest)
		if err != nil {
			c.logger.Error("failed to marshal shutdown request", "error", err)
		} else {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(c.ctx, c.connectionTimeout)
			defer cancel()

			// Send the request
			_, err := c.transport.SendWithContext(ctx, requestJSON)
			if err != nil {
				c.logger.Error("failed to send shutdown request", "error", err)
			}
		}
	}

	// Disconnect from the server
	err := c.transport.Disconnect()
	c.connected = false
	c.initialized = false

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
				errorResponse := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      request.ID,
					"error": map[string]interface{}{
						"code":    -32601,
						"message": "Method not found",
					},
				}
				responseJSON, _ := json.Marshal(errorResponse)
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
