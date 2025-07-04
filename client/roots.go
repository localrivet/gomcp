// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/localrivet/gomcp/mcp"
)

// rootsRequest represents a request to the roots manager
type rootsRequest struct {
	operation string
	uri       string
	name      string
	response  chan rootsResponse
}

// rootsResponse represents a response from the roots manager
type rootsResponse struct {
	roots []Root
	err   error
}

// rootsManager manages roots using the actor pattern - single goroutine owns all data
type rootsManager struct {
	requests chan rootsRequest
	done     chan struct{}
	client   *clientImpl // for sending notifications
}

// newRootsManager creates and starts a new roots manager
func newRootsManager(client *clientImpl) *rootsManager {
	rm := &rootsManager{
		requests: make(chan rootsRequest, 100), // buffered for better performance
		done:     make(chan struct{}),
		client:   client,
	}
	go rm.run()
	return rm
}

// run is the main actor loop - runs in its own goroutine
func (rm *rootsManager) run() {
	var roots []Root // this slice is owned by this goroutine only

	for {
		select {
		case req := <-rm.requests:
			rm.handleRequest(req, &roots)
		case <-rm.done:
			return
		}
	}
}

// handleRequest processes a single request
func (rm *rootsManager) handleRequest(req rootsRequest, roots *[]Root) {
	var resp rootsResponse

	switch req.operation {
	case "add":
		resp.err = rm.addRoot(roots, req.uri, req.name)
	case "remove":
		resp.err = rm.removeRoot(roots, req.uri)
	case "get":
		// Return a copy to prevent external modifications
		resp.roots = make([]Root, len(*roots))
		copy(resp.roots, *roots)
	default:
		resp.err = fmt.Errorf("unknown operation: %s", req.operation)
	}

	// Send response back
	select {
	case req.response <- resp:
	case <-rm.done:
		return
	}
}

// addRoot adds a root to the slice (runs in actor goroutine)
func (rm *rootsManager) addRoot(roots *[]Root, uri, name string) error {
	// Convert to file:// URI if needed
	var err error
	uri, err = ensureFileURI(uri)
	if err != nil {
		return err
	}

	// Check if the root already exists
	for _, root := range *roots {
		if root.URI == uri {
			return fmt.Errorf("root with URI %s already exists", uri)
		}
	}

	// Add the root
	*roots = append(*roots, Root{
		URI:  uri,
		Name: name,
	})

	// Enable roots capability if not already enabled
	if !rm.client.capabilities.Roots.ListChanged.Load() {
		rm.client.capabilities.Roots.ListChanged.Store(true)
	}

	// Send notification (outside of critical section since we own the data)
	rm.client.sendRootsListChangedNotification()

	return nil
}

// removeRoot removes a root from the slice (runs in actor goroutine)
func (rm *rootsManager) removeRoot(roots *[]Root, uri string) error {
	// Convert to file:// URI if needed
	uri, err := ensureFileURI(uri)
	if err != nil {
		return err
	}

	// Find the root
	var foundIndex = -1
	for i, root := range *roots {
		if root.URI == uri {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return fmt.Errorf("root with URI %s not found", uri)
	}

	// Remove the root
	*roots = append((*roots)[:foundIndex], (*roots)[foundIndex+1:]...)

	// Send notification (outside of critical section since we own the data)
	rm.client.sendRootsListChangedNotification()

	return nil
}

// stop shuts down the roots manager
func (rm *rootsManager) stop() {
	close(rm.done)
}

// AddRoot adds a filesystem root to be exposed to the server.
// Accepts either filesystem paths or file:// URIs - automatically converts paths to file:// URIs as required by MCP specification.
func (c *clientImpl) AddRoot(uri string, name string) error {
	req := rootsRequest{
		operation: "add",
		uri:       uri,
		name:      name,
		response:  make(chan rootsResponse, 1),
	}

	select {
	case c.rootsManager.requests <- req:
		// Wait for response
		resp := <-req.response
		return resp.err
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// RemoveRoot removes a filesystem root.
// Accepts either filesystem paths or file:// URIs - automatically converts paths to file:// URIs as required by MCP specification.
func (c *clientImpl) RemoveRoot(uri string) error {
	req := rootsRequest{
		operation: "remove",
		uri:       uri,
		response:  make(chan rootsResponse, 1),
	}

	select {
	case c.rootsManager.requests <- req:
		// Wait for response
		resp := <-req.response
		return resp.err
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// GetRoots returns the current list of roots.
// According to MCP protocol, clients manage their own roots.
// The server requests roots via "roots/list" calls TO the client, not FROM the client.
func (c *clientImpl) GetRoots() ([]Root, error) {
	req := rootsRequest{
		operation: "get",
		response:  make(chan rootsResponse, 1),
	}

	select {
	case c.rootsManager.requests <- req:
		// Wait for response
		resp := <-req.response
		return resp.roots, resp.err
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// handleRootsList handles a roots/list request from the server.
func (c *clientImpl) handleRootsList(requestID int64) error {
	roots, err := c.GetRoots()
	if err != nil {
		return err
	}

	// Create response using structured type
	result := map[string]interface{}{
		"roots": roots,
	}
	response := mcp.NewSuccessResponse(requestID, result)

	responseData, err := response.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal roots/list response: %w", err)
	}

	_, err = c.transport.Send(responseData)
	return err
}

// sendRootsListChangedNotification sends a notification that the roots list has changed
func (c *clientImpl) sendRootsListChangedNotification() {
	if !c.capabilities.Roots.ListChanged.Load() {
		return // Don't send notifications if capability is not enabled
	}

	// Don't send notifications if transport is nil (not connected yet)
	if c.transport == nil {
		return
	}

	// Create notification using structured type
	notification := mcp.NewNotification("notifications/roots/list_changed", nil)

	notificationData, err := notification.Marshal()
	if err != nil {
		// Log error but don't fail the operation
		return
	}

	// Send notification asynchronously (fire-and-forget as per MCP spec)
	if c.transport != nil {
		go func() {
			defer func() {
				// Recover from any panics in the transport layer
				if r := recover(); r != nil {
					// Notification failed, but don't crash the application
				}
			}()

			// Send notification - if it hangs, it won't affect the main operation
			_, _ = c.transport.Send(notificationData)
		}()
	}
}

// ensureFileURI converts a filesystem path to a file:// URI as required by MCP specification.
// If the input is already a valid file:// URI, it returns it unchanged.
// Returns an error if the input cannot be converted to a valid file:// URI.
func ensureFileURI(path string) (string, error) {
	// Check if it's already a file:// URI
	if strings.HasPrefix(path, "file://") {
		return path, nil
	}

	// Check if it's a valid URI with a different scheme
	if u, err := url.Parse(path); err == nil && u.Scheme != "" && u.Scheme != "file" {
		return "", fmt.Errorf("invalid URI scheme '%s': root URIs must use file:// scheme", u.Scheme)
	}

	// Convert filesystem path to file:// URI
	if !filepath.IsAbs(path) {
		// Make relative paths absolute
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to convert relative path to absolute: %w", err)
		}
	}

	// Convert to file:// URI
	return "file://" + filepath.ToSlash(path), nil
}
