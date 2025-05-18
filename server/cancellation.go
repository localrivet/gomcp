package server

import (
	"encoding/json"
	"fmt"
	"sync"
)

// CancelledNotificationParams contains parameters for a cancelled notification
type CancelledNotificationParams struct {
	RequestID string `json:"requestId"`        // ID of the request being cancelled
	Reason    string `json:"reason,omitempty"` // Optional reason for cancellation
}

// RequestCanceller manages cancellable requests and handles cancellation notifications
type RequestCanceller struct {
	mu            sync.RWMutex
	cancellations map[interface{}]chan struct{} // Maps request IDs to cancellation channels
}

// NewRequestCanceller creates a new request canceller
func NewRequestCanceller() *RequestCanceller {
	return &RequestCanceller{
		cancellations: make(map[interface{}]chan struct{}),
	}
}

// Register registers a request as cancellable and returns a channel that will be closed on cancellation
func (rc *RequestCanceller) Register(requestID interface{}) <-chan struct{} {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Create a cancellation channel for this request
	cancelCh := make(chan struct{})
	rc.cancellations[requestID] = cancelCh
	return cancelCh
}

// Cancel cancels a request by closing its cancellation channel
// Returns true if the request was found and cancelled, false otherwise
func (rc *RequestCanceller) Cancel(requestID interface{}, reason string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	cancelCh, exists := rc.cancellations[requestID]
	if !exists {
		return false
	}

	// Close the cancellation channel to signal cancellation
	close(cancelCh)

	// Remove the request from the map
	delete(rc.cancellations, requestID)
	return true
}

// Deregister removes a request from the cancellation registry without cancelling it
// This should be called when a request completes normally
func (rc *RequestCanceller) Deregister(requestID interface{}) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check if the channel exists
	cancelCh, exists := rc.cancellations[requestID]
	if !exists {
		return
	}

	// Try to close the channel in a way that doesn't panic if it's already closed
	// This helps with race conditions where cancellation and completion happen simultaneously
	select {
	case <-cancelCh:
		// Channel is already closed
	default:
		// Channel is still open, close it
		close(cancelCh)
	}

	// Remove the request from the map
	delete(rc.cancellations, requestID)
}

// IsCancelled checks if a request has been cancelled
// Returns true if the request is cancelled, false otherwise
func (rc *RequestCanceller) IsCancelled(requestID interface{}) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	cancelCh, exists := rc.cancellations[requestID]
	if !exists {
		return false
	}

	// Check if the channel is closed (cancelled)
	select {
	case <-cancelCh:
		return true // Channel is closed, request is cancelled
	default:
		return false // Channel is open, request is not cancelled
	}
}

// HandleCancelledNotification processes a notifications/cancelled notification
func (s *serverImpl) HandleCancelledNotification(message []byte) error {
	// Parse the notification
	var notification struct {
		JSONRPC string                      `json:"jsonrpc"`
		Method  string                      `json:"method"`
		Params  CancelledNotificationParams `json:"params"`
	}

	if err := json.Unmarshal(message, &notification); err != nil {
		return fmt.Errorf("failed to parse cancelled notification: %w", err)
	}

	// Extract the request ID
	requestID := notification.Params.RequestID
	reason := notification.Params.Reason

	// Cancel the request
	if requestID == "" {
		return fmt.Errorf("invalid request ID in cancelled notification")
	}

	if cancelled := s.requestCanceller.Cancel(requestID, reason); cancelled {
		s.logger.Info("request cancelled", "requestId", requestID, "reason", reason)
	} else {
		s.logger.Debug("cancellation requested for unknown request", "requestId", requestID)
	}

	return nil
}

// SendCancelledNotification sends a notifications/cancelled notification
func (s *serverImpl) SendCancelledNotification(requestID string, reason string) error {
	// Create the notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params": map[string]interface{}{
			"requestId": requestID,
		},
	}

	// Add reason if provided
	if reason != "" {
		notification["params"].(map[string]interface{})["reason"] = reason
	}

	// Convert to JSON
	message, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal cancelled notification: %w", err)
	}

	// Send the notification
	if s.transport != nil {
		if err := s.transport.Send(message); err != nil {
			return fmt.Errorf("failed to send cancelled notification: %w", err)
		}
	} else {
		s.logger.Warn("no transport configured, cancellation not sent", "requestId", requestID)
	}

	return nil
}

// CancelRequestWithError cancels a request and returns an error with the given reason
func (s *serverImpl) CancelRequestWithError(requestID string, reason string) error {
	// Send the cancellation notification
	if err := s.SendCancelledNotification(requestID, reason); err != nil {
		return err
	}

	// Cancel the request locally
	s.requestCanceller.Cancel(requestID, reason)

	return fmt.Errorf("request cancelled: %s", reason)
}
