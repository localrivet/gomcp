package server

import (
	"sync"

	"github.com/localrivet/gomcp/logx"
)

// SubscriptionManager maintains mappings between resource URIs and clients interested in updates.
type SubscriptionManager struct {
	// Maps URIs to a set of connection IDs (using a map for O(1) lookups)
	subscriptions map[string]map[string]bool
	// Maps connection IDs to a set of URIs (for efficient cleanup on disconnect)
	connectionURIs map[string]map[string]bool
	mu             sync.RWMutex
	logger         logx.Logger // Logger instance
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions:  make(map[string]map[string]bool),
		connectionURIs: make(map[string]map[string]bool),
		logger:         logx.NewDefaultLogger(), // Initialize with default logger
	}
}

// SetLogger updates the logger used by the subscription manager.
func (sm *SubscriptionManager) SetLogger(logger logx.Logger) {
	sm.logger = logger
}

// Subscribe adds a subscription for a given connection ID to a resource URI.
func (sm *SubscriptionManager) Subscribe(uri, connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.subscriptions[uri]; !ok {
		sm.subscriptions[uri] = make(map[string]bool)
	}
	sm.subscriptions[uri][connectionID] = true

	if sm.logger != nil {
		sm.logger.Debug("Connection %s subscribed to resource %s", connectionID, uri)
	}
}

// Unsubscribe removes a subscription for a given connection ID from a resource URI.
func (sm *SubscriptionManager) Unsubscribe(uri, connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if connectionIDs, ok := sm.subscriptions[uri]; ok {
		delete(connectionIDs, connectionID)
		if len(connectionIDs) == 0 {
			delete(sm.subscriptions, uri)
		}

		if sm.logger != nil {
			sm.logger.Debug("Connection %s unsubscribed from resource %s", connectionID, uri)
		}
	}
}

// GetSubscribedConnectionIDs returns a list of connection IDs subscribed to a resource URI.
func (sm *SubscriptionManager) GetSubscribedConnectionIDs(uri string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	connectionIDs := []string{}
	if ids, ok := sm.subscriptions[uri]; ok {
		for id := range ids {
			connectionIDs = append(connectionIDs, id)
		}
	}
	return connectionIDs
}

// UnsubscribeAll removes all subscriptions for a given connection ID.
func (sm *SubscriptionManager) UnsubscribeAll(connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	urisToRemove := []string{}
	for uri, connectionIDs := range sm.subscriptions {
		if _, ok := connectionIDs[connectionID]; ok {
			delete(connectionIDs, connectionID)
			if len(connectionIDs) == 0 {
				urisToRemove = append(urisToRemove, uri)
			}
		}
	}
	for _, uri := range urisToRemove {
		delete(sm.subscriptions, uri)
	}

	if sm.logger != nil && len(urisToRemove) > 0 {
		sm.logger.Debug("Connection %s unsubscribed from all resources (%d total)", connectionID, len(urisToRemove))
	}
}

// IsSubscribed checks if a specific connection ID is subscribed to a given URI.
func (sm *SubscriptionManager) IsSubscribed(uri string, connectionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if subs, ok := sm.subscriptions[uri]; ok {
		_, exists := subs[connectionID]
		return exists
	}
	return false
}
