package server

import (
	"sync"
)

// SubscriptionManager manages resource subscriptions for client connections.
type SubscriptionManager struct {
	// subscriptions maps resource URIs to a set of subscribed connection IDs.
	subscriptions map[string]map[string]struct{}
	mu            sync.RWMutex
}

// NewSubscriptionManager creates a new SubscriptionManager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]map[string]struct{}),
	}
}

// Subscribe adds a subscription for a given connection ID to a resource URI.
func (sm *SubscriptionManager) Subscribe(uri, connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.subscriptions[uri]; !ok {
		sm.subscriptions[uri] = make(map[string]struct{})
	}
	sm.subscriptions[uri][connectionID] = struct{}{}
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
