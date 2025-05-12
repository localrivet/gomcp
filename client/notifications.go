package client

import (
	"github.com/localrivet/gomcp/protocol"
)

// handleProgress processes progress notifications
func (c *clientImpl) handleProgress(params protocol.ProgressParams) error {
	for _, handler := range c.progressHandlers {
		if err := handler(&params); err != nil {
			c.config.Logger.Error("Progress handler error: %v", err)
		}
	}
	return nil
}

// handleResourceUpdate processes resource update notifications
func (c *clientImpl) handleResourceUpdate(uri string) error {
	if handlers, ok := c.resourceUpdateHandlers[uri]; ok {
		for _, handler := range handlers {
			if err := handler(uri); err != nil {
				c.config.Logger.Error("Resource update handler error for URI %s: %v", uri, err)
			}
		}
	}
	return nil
}

// handleLog processes log notifications
func (c *clientImpl) handleLog(level protocol.LoggingLevel, message string) error {
	for _, handler := range c.logHandlers {
		if err := handler(level, message); err != nil {
			c.config.Logger.Error("Log handler error: %v", err)
		}
	}
	return nil
}

// handleConnectionStatus processes connection status changes
func (c *clientImpl) handleConnectionStatus(connected bool) error {
	for _, handler := range c.connectionHandlers {
		if err := handler(connected); err != nil {
			c.config.Logger.Error("Connection status handler error: %v", err)
		}
	}
	return nil
}

// Implementation of the notification registration methods for clientImpl

func (c *clientImpl) OnNotification(method string, handler NotificationHandler) Client {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	// Add the handler to the appropriate method
	if _, ok := c.notificationHandlers[method]; !ok {
		c.notificationHandlers[method] = []NotificationHandler{}
	}
	c.notificationHandlers[method] = append(c.notificationHandlers[method], handler)

	return c
}

func (c *clientImpl) OnProgress(handler ProgressHandler) Client {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	c.progressHandlers = append(c.progressHandlers, handler)

	return c
}

func (c *clientImpl) OnResourceUpdate(uri string, handler ResourceUpdateHandler) Client {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if _, ok := c.resourceUpdateHandlers[uri]; !ok {
		c.resourceUpdateHandlers[uri] = []ResourceUpdateHandler{}
	}
	c.resourceUpdateHandlers[uri] = append(c.resourceUpdateHandlers[uri], handler)

	return c
}

func (c *clientImpl) OnLog(handler LogHandler) Client {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	c.logHandlers = append(c.logHandlers, handler)

	return c
}

func (c *clientImpl) OnConnectionStatus(handler ConnectionStatusHandler) Client {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	c.connectionHandlers = append(c.connectionHandlers, handler)

	return c
}
