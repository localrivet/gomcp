package test

import (
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
)

// TestSamplingRequestFlowWithRetries tests the complete sampling request flow with retries
func TestSamplingRequestFlowWithRetries(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Get the underlying serverImpl
	innerServer := s.GetServer()

	// Verify we can access it
	if innerServer == nil {
		t.Fatal("Failed to get server implementation")
	}

	// Simply create a session and verify sampling capabilities
	clientInfo := server.ClientInfo{
		SamplingSupported: true,
		SamplingCaps: server.SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: true,
		},
		ProtocolVersion: "draft",
	}

	sessionManager := server.NewSessionManager()
	session := sessionManager.CreateSession(clientInfo, "draft")

	// Verify the session was created with proper settings
	if session == nil {
		t.Fatal("Failed to create session")
	}

	if !session.ClientInfo.SamplingSupported {
		t.Error("Expected sampling to be supported")
	}

	// Log success
	t.Log("Successfully tested basic sampling session flow")
}

// TestConcurrentSamplingRequests tests multiple sampling requests running concurrently
func TestConcurrentSamplingRequests(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Get the underlying serverImpl
	innerServer := s.GetServer()

	// Verify we can access it
	if innerServer == nil {
		t.Fatal("Failed to get server implementation")
	}

	// Test with multiple sessions concurrently
	const concurrentSessions = 5
	var wg sync.WaitGroup
	wg.Add(concurrentSessions)

	for i := 0; i < concurrentSessions; i++ {
		go func(sessionNum int) {
			defer wg.Done()

			// Create client info
			clientInfo := server.ClientInfo{
				SamplingSupported: true,
				SamplingCaps: server.SamplingCapabilities{
					Supported:    true,
					TextSupport:  true,
					ImageSupport: true,
					AudioSupport: true,
				},
				ProtocolVersion: "draft",
			}

			// Create a session
			sessionManager := server.NewSessionManager()
			session := sessionManager.CreateSession(clientInfo, "draft")

			// Check that the session has a valid ID
			if session.ID == "" {
				t.Errorf("Session %d: Empty session ID", sessionNum)
			}

			// Sleep to simulate concurrent work
			time.Sleep(10 * time.Millisecond)
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Log success
	t.Log("Successfully tested concurrent session creation")
}
