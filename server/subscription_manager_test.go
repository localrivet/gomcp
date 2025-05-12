package server_test

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	// Add necessary imports later

	"github.com/localrivet/gomcp/server"
	"github.com/stretchr/testify/require"
)

// TODO: Test Subscribe
func TestSubscriptionManager_Subscribe(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test Unsubscribe
func TestSubscriptionManager_Unsubscribe(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test UnsubscribeAll
func TestSubscriptionManager_UnsubscribeAll(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test GetSubscribedConnectionIDs
func TestSubscriptionManager_GetSubscribedConnectionIDs(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test notification sending on resource updates (integration with Registry callback?)
// This might be better tested via the Registry callback test or an integration test.
func TestSubscriptionManager_NotificationIntegration(t *testing.T) { t.Skip("Test not implemented") }

func TestSubscriptionManager_SubscribeUnsubscribe(t *testing.T) {
	t.Parallel()
	sm := server.NewSubscriptionManager()
	uri1 := "file:///test1.txt"
	uri2 := "file:///test2.doc"
	conn1 := "session-1"
	conn2 := "session-2"
	conn3 := "session-3"

	// Initial state: no subscriptions
	require.Empty(t, sm.GetSubscribedConnectionIDs(uri1))
	require.Empty(t, sm.GetSubscribedConnectionIDs(uri2))
	require.False(t, sm.IsSubscribed(uri1, conn1))

	// Subscribe conn1 to uri1
	sm.Subscribe(uri1, conn1)
	require.ElementsMatch(t, []string{conn1}, sm.GetSubscribedConnectionIDs(uri1))
	require.Empty(t, sm.GetSubscribedConnectionIDs(uri2))
	require.True(t, sm.IsSubscribed(uri1, conn1))
	require.False(t, sm.IsSubscribed(uri1, conn2))
	require.False(t, sm.IsSubscribed(uri2, conn1))

	// Subscribe conn1 to uri1 again (idempotent)
	sm.Subscribe(uri1, conn1)
	require.ElementsMatch(t, []string{conn1}, sm.GetSubscribedConnectionIDs(uri1))

	// Subscribe conn2 to uri1
	sm.Subscribe(uri1, conn2)
	require.ElementsMatch(t, []string{conn1, conn2}, sm.GetSubscribedConnectionIDs(uri1))
	require.True(t, sm.IsSubscribed(uri1, conn1))
	require.True(t, sm.IsSubscribed(uri1, conn2))

	// Subscribe conn1 to uri2
	sm.Subscribe(uri2, conn1)
	require.ElementsMatch(t, []string{conn1, conn2}, sm.GetSubscribedConnectionIDs(uri1))
	require.ElementsMatch(t, []string{conn1}, sm.GetSubscribedConnectionIDs(uri2))
	require.True(t, sm.IsSubscribed(uri2, conn1))
	require.False(t, sm.IsSubscribed(uri2, conn2))

	// Unsubscribe conn1 from uri1
	sm.Unsubscribe(uri1, conn1)
	require.ElementsMatch(t, []string{conn2}, sm.GetSubscribedConnectionIDs(uri1))
	require.ElementsMatch(t, []string{conn1}, sm.GetSubscribedConnectionIDs(uri2))
	require.False(t, sm.IsSubscribed(uri1, conn1))
	require.True(t, sm.IsSubscribed(uri1, conn2))
	require.True(t, sm.IsSubscribed(uri2, conn1))

	// Unsubscribe conn2 from uri1 (removes uri1 entry)
	sm.Unsubscribe(uri1, conn2)
	require.Empty(t, sm.GetSubscribedConnectionIDs(uri1))
	require.ElementsMatch(t, []string{conn1}, sm.GetSubscribedConnectionIDs(uri2))
	require.False(t, sm.IsSubscribed(uri1, conn2))

	// Unsubscribe non-existent
	sm.Unsubscribe("file:///nonexistent", conn1)
	sm.Unsubscribe(uri2, conn3)
	require.ElementsMatch(t, []string{conn1}, sm.GetSubscribedConnectionIDs(uri2)) // Unchanged
}

func TestSubscriptionManager_Concurrency(t *testing.T) {
	t.Parallel()
	sm := server.NewSubscriptionManager()
	numGoroutines := 50
	numOpsPerGoroutine := 100
	uri := "file:///concurrent.txt"

	var wg sync.WaitGroup
	// wg.Add(numGoroutines * 2) // REMOVED: Incorrect initial add

	// Phase 1: Concurrent Subscriptions
	wg.Add(numGoroutines) // ADDED: Add count for subscribe phase
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			connID := fmt.Sprintf("session-%d", id)
			for j := 0; j < numOpsPerGoroutine; j++ {
				sm.Subscribe(uri, connID)
			}
		}(i)
	}

	// Wait for subscriptions to likely finish (no perfect sync needed here)
	wg.Wait() // Wait for subscribe phase only

	// Check intermediate state
	require.Len(t, sm.GetSubscribedConnectionIDs(uri), numGoroutines)

	// Phase 2: Concurrent Unsubscriptions
	wg.Add(numGoroutines) // ADDED: Add count for unsubscribe phase
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			connID := fmt.Sprintf("session-%d", id)
			for j := 0; j < numOpsPerGoroutine; j++ {
				sm.Unsubscribe(uri, connID)
			}
		}(i)
	}

	wg.Wait() // Wait for unsubscribes

	// Final state: should be empty
	require.Empty(t, sm.GetSubscribedConnectionIDs(uri))
}

// Helper to sort string slices for reliable comparison
func sortStrings(s []string) {
	sort.Strings(s)
}
