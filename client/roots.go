// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"encoding/json"
	"fmt"
)

// AddRoot adds a filesystem root to be exposed to the server.
func (c *clientImpl) AddRoot(uri string, name string) error {
	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()

	// Check if the root already exists
	for _, root := range c.roots {
		if root.URI == uri {
			return fmt.Errorf("root with URI %s already exists", uri)
		}
	}

	// Add the root to our local cache
	c.roots = append(c.roots, Root{
		URI:  uri,
		Name: name,
	})

	// Only send the actual request if we're initialized
	if c.IsInitialized() {
		// Use the sendRequest method to handle the JSON-RPC request
		params := map[string]interface{}{
			"uri":  uri,
			"name": name,
		}

		_, err := c.sendRequest("roots/add", params)
		if err != nil {
			// Rollback the local addition on error
			for i, root := range c.roots {
				if root.URI == uri {
					c.roots = append(c.roots[:i], c.roots[i+1:]...)
					break
				}
			}
			return fmt.Errorf("failed to add root: %w", err)
		}
	}

	// Enable roots capability if not already enabled
	if !c.capabilities.Roots.ListChanged {
		c.capabilities.Roots.ListChanged = true
	}

	return nil
}

// RemoveRoot removes a filesystem root.
func (c *clientImpl) RemoveRoot(uri string) error {
	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()

	// Find the root in our local cache
	var foundIndex = -1
	for i, root := range c.roots {
		if root.URI == uri {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return fmt.Errorf("root with URI %s not found", uri)
	}

	// Only send the actual request if we're initialized
	if c.IsInitialized() {
		params := map[string]interface{}{
			"uri": uri,
		}

		_, err := c.sendRequest("roots/remove", params)
		if err != nil {
			return fmt.Errorf("failed to remove root: %w", err)
		}
	}

	// Remove the root from our local cache
	c.roots = append(c.roots[:foundIndex], c.roots[foundIndex+1:]...)

	return nil
}

// GetRoots returns the current list of roots.
func (c *clientImpl) GetRoots() ([]Root, error) {
	// If we're initialized, get the roots from the server
	if c.IsInitialized() {
		result, err := c.sendRequest("roots/list", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get roots: %w", err)
		}

		// Parse the result based on the format
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected result format")
		}

		rootsData, ok := resultMap["roots"]
		if !ok {
			return nil, fmt.Errorf("roots field not found in response")
		}

		rootsList, ok := rootsData.([]interface{})
		if !ok {
			return nil, fmt.Errorf("roots field is not an array")
		}

		roots := make([]Root, 0, len(rootsList))
		for _, item := range rootsList {
			rootMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			uri, ok := rootMap["uri"].(string)
			if !ok {
				continue
			}

			name, ok := rootMap["name"].(string)
			if !ok {
				name = uri // Use URI as fallback name
			}

			root := Root{
				URI:  uri,
				Name: name,
			}

			// Add metadata if available (for v2025-03-26)
			if metadata, ok := rootMap["metadata"].(map[string]interface{}); ok {
				root.Metadata = metadata
			}

			roots = append(roots, root)
		}

		// Update our local cache
		c.rootsMu.Lock()
		c.roots = roots
		c.rootsMu.Unlock()

		return roots, nil
	}

	// If not initialized, return the cached roots
	c.rootsMu.RLock()
	defer c.rootsMu.RUnlock()

	// Return a copy to prevent modifications
	roots := make([]Root, len(c.roots))
	copy(roots, c.roots)

	return roots, nil
}

// handleRootsList handles a roots/list request from the server.
func (c *clientImpl) handleRootsList(requestID int64) error {
	c.rootsMu.RLock()
	roots := make([]Root, len(c.roots))
	copy(roots, c.roots)
	c.rootsMu.RUnlock()

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result": map[string]interface{}{
			"roots": roots,
		},
	}

	// Convert to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal roots/list response: %w", err)
	}

	// Send the response
	_, err = c.transport.Send(responseJSON)
	if err != nil {
		return fmt.Errorf("failed to send roots/list response: %w", err)
	}

	return nil
}

// sendRootsListChangedNotification sends a notification that the roots list has changed.
func (c *clientImpl) sendRootsListChangedNotification() error {
	// Check if we support the roots/list_changed notification
	if !c.capabilities.Roots.ListChanged {
		return nil
	}

	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/roots/list_changed",
	}

	// Convert to JSON
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal roots list changed notification: %w", err)
	}

	// Send the notification
	_, err = c.transport.Send(notificationJSON)
	if err != nil {
		return fmt.Errorf("failed to send roots list changed notification: %w", err)
	}

	return nil
}
