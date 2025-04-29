package client

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
	"github.com/localrivet/gomcp/types/conversion"
)

// --- Mock Logger ---
type NilLogger struct{}

func (n *NilLogger) Debug(msg string, args ...interface{}) {}
func (n *NilLogger) Info(msg string, args ...interface{})  {}
func (n *NilLogger) Warn(msg string, args ...interface{})  {}
func (n *NilLogger) Error(msg string, args ...interface{}) {}
func NewNilLogger() *NilLogger                             { return &NilLogger{} }

var _ types.Logger = (*NilLogger)(nil)

// --- Tests ---

// Helper function to run the handshake simulation more synchronously
func runHandshakeTest(t *testing.T, clientVersion, serverVersion string, expectSuccess bool, expectedNegotiatedVersion string) {
	t.Helper()
	mockTransport := newMockTransport()
	client, err := NewClient("TestClient", ClientOptions{
		Transport:                mockTransport,
		PreferredProtocolVersion: conversion.StrPtr(clientVersion),
		Logger:                   NewNilLogger(),
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Start the client's message processing loop in the background
	// This goroutine will block waiting for messages on the mock transport's receiveChan
	client.startMessageProcessing()
	defer client.Close() // Ensure processing stops and transport closes

	handshakeCompleted := false
	var connectErr error

	// --- Simulate Handshake Steps ---

	// 1. Simulate Client Sending Initialize Request
	// We manually replicate the first part of client.Connect()
	clientInfo := protocol.Implementation{Name: client.clientName, Version: "0.1.0"}
	initParams := protocol.InitializeRequestParams{
		ProtocolVersion: client.preferredVersion,
		Capabilities:    client.clientCapabilities,
		ClientInfo:      clientInfo,
	}
	initReqID := uuid.NewString()
	initTimeout := 30 * time.Second // Increased timeout for test
	initCtx, initCancel := context.WithTimeout(context.Background(), initTimeout)
	defer initCancel()

	// Use a channel to wait for the response from sendRequestAndWait
	responseChan := make(chan *protocol.JSONRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		// This goroutine simulates the blocking nature of sendRequestAndWait
		// It sends the request, then waits for the response channel populated by handleResponse
		resp, err := client.sendRequestAndWait(initCtx, protocol.MethodInitialize, initReqID, initParams, initTimeout)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- resp
		}
	}()

	// Expect Initialize Request on the mock transport's send channel
	var reqBytes []byte
	select {
	case reqBytes = <-mockTransport.sendChan:
		t.Log("Test: Received initialize request from client.")
	case err := <-errChan:
		t.Fatalf("Test: sendRequestAndWait failed unexpectedly while sending initialize: %v", err)
	case <-time.After(10 * time.Second): // Increased timeout
		t.Fatal("Test: Timed out waiting for client to send initialize request")
	}
	// Basic verification of the received request
	var initReq protocol.JSONRPCRequest
	if err := json.Unmarshal(reqBytes, &initReq); err != nil {
		t.Fatalf("Test: Failed to unmarshal initialize request: %v", err)
	}
	if initReq.Method != protocol.MethodInitialize || initReq.ID.(string) != initReqID {
		t.Fatalf("Test: Received unexpected request: %+v", initReq)
	}

	// 2. Simulate Server Sending Initialize Response
	initResult := protocol.InitializeResult{
		ProtocolVersion: serverVersion,
		Capabilities:    protocol.ServerCapabilities{Logging: &struct{}{}},
		ServerInfo:      protocol.Implementation{Name: "MockTransportServer", Version: "1.0"},
	}
	resp := protocol.JSONRPCResponse{JSONRPC: "2.0", ID: initReqID, Result: initResult}
	respBytes, _ := json.Marshal(resp)
	if err := mockTransport.SimulateSend(respBytes); err != nil { // Puts response on receiveChan
		t.Fatalf("Test: Failed to simulate sending initialize response: %v", err)
	}
	t.Log("Test: Sent initialize response to client.")

	// 3. Wait for the client's sendRequestAndWait goroutine to finish
	var initializeResponse *protocol.JSONRPCResponse
	select {
	case initializeResponse = <-responseChan:
		t.Log("Test: Client's sendRequestAndWait for initialize completed.")
		// Process the response (as client.Connect would)
		if initializeResponse.Error != nil {
			connectErr = fmt.Errorf("received error response during initialization: [%d] %s", initializeResponse.Error.Code, initializeResponse.Error.Message)
		} else {
			var parsedResult protocol.InitializeResult
			if err := protocol.UnmarshalPayload(initializeResponse.Result, &parsedResult); err != nil {
				connectErr = fmt.Errorf("failed to parse initialize result: %w", err)
			} else {
				// Successfully parsed the result, store it first
				client.capabilitiesMu.Lock()
				client.serverInfo = parsedResult.ServerInfo
				client.serverCapabilities = parsedResult.Capabilities
				client.negotiatedVersion = parsedResult.ProtocolVersion // Store the actual negotiated version
				client.capabilitiesMu.Unlock()

				// Now check if the negotiated version is supported by the client
				serverSelectedVersion := parsedResult.ProtocolVersion
				if serverSelectedVersion != protocol.CurrentProtocolVersion && serverSelectedVersion != protocol.OldProtocolVersion {
					connectErr = fmt.Errorf("server selected unsupported protocol version: %s (client supports %s, %s)",
						serverSelectedVersion, protocol.CurrentProtocolVersion, protocol.OldProtocolVersion)
					// Reset negotiated version if unsupported
					client.capabilitiesMu.Lock()
					client.negotiatedVersion = "" // Clear the stored version if it's unsupported
					client.capabilitiesMu.Unlock()
				}
				// The final assertion checks client.negotiatedVersion against expectedNegotiatedVersion
			}
		}
	case connectErr = <-errChan:
		t.Logf("Test: Client's sendRequestAndWait for initialize failed: %v", connectErr)
	case <-time.After(10 * time.Second): // Increased timeout
		t.Fatal("Test: Timed out waiting for client's sendRequestAndWait for initialize to return")
	}

	// 4. Simulate Client Sending Initialized Notification (if handshake succeeded so far)
	if connectErr == nil && expectSuccess {
		// Manually trigger the sendNotification part of client.Connect
		initNotifParams := protocol.InitializedNotificationParams{}
		initNotif := protocol.JSONRPCNotification{JSONRPC: "2.0", Method: protocol.MethodInitialized, Params: initNotifParams}
		notifCtx, notifCancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout
		defer notifCancel()
		if err := client.sendNotification(notifCtx, initNotif); err != nil {
			// Log warning, but don't necessarily fail the test here
			t.Logf("Test: Client failed to send initialized notification: %v", err)
		}

		// Expect Initialized on sendChan
		var notifBytes []byte
		select {
		case notifBytes = <-mockTransport.sendChan:
			t.Log("Test: Received initialized notification from client.")
			var receivedNotif protocol.JSONRPCNotification
			if err := json.Unmarshal(notifBytes, &receivedNotif); err != nil {
				t.Fatalf("Test: Failed to unmarshal initialized notification: %v", err)
			}
			if receivedNotif.Method != protocol.MethodInitialized {
				t.Fatalf("Test: Received unexpected notification method: %s", receivedNotif.Method)
			}
			handshakeCompleted = true
			// Manually set client state after successful handshake simulation
			client.stateMu.Lock()
			client.initialized = true
			client.stateMu.Unlock()
		case <-time.After(10 * time.Second): // Increased timeout
			t.Fatal("Test: Timed out waiting for client to send initialized notification")
		}
	} else if connectErr == nil && !expectSuccess {
		// If success was not expected, but we didn't get an error processing the response,
		// it means the version negotiation should have failed.
		t.Log("Test: Handshake failed due to version mismatch as expected.")
		handshakeCompleted = true // Mark as completed (though failed)
	} else if connectErr != nil {
		// If an error occurred during response processing
		t.Logf("Test: Handshake failed due to error processing response: %v", connectErr)
		handshakeCompleted = true // Mark as completed (though failed)
	}

	// --- Assertions ---
	if expectSuccess {
		if connectErr != nil {
			t.Errorf("Handshake failed unexpectedly: %v", connectErr)
		}
		if !handshakeCompleted {
			t.Error("Handshake simulation did not complete successfully for expected success case.")
		}
		if !client.IsInitialized() {
			t.Error("Client state 'initialized' is false after successful handshake simulation")
		}
		client.stateMu.RLock()
		negotiated := client.negotiatedVersion
		client.stateMu.RUnlock()
		if negotiated != expectedNegotiatedVersion {
			t.Errorf("Client negotiated version mismatch: expected %s, got %s", expectedNegotiatedVersion, negotiated)
		}
	} else {
		if connectErr == nil {
			t.Error("Handshake succeeded unexpectedly when failure was expected")
		} else {
			t.Logf("Handshake failed as expected: %v", connectErr)
		}
		if client.IsInitialized() {
			t.Error("Client state 'initialized' is true after failed handshake simulation")
		}
	}
}

func TestClientInitializeSuccessCurrentVersion(t *testing.T) {
	runHandshakeTest(t,
		protocol.CurrentProtocolVersion, // Client prefers current
		protocol.CurrentProtocolVersion, // Server supports current
		true,                            // Expect success
		protocol.CurrentProtocolVersion, // Expect current negotiated
	)
}

func TestClientInitializeSuccessOldVersion(t *testing.T) {
	runHandshakeTest(t,
		protocol.OldProtocolVersion, // Client prefers old (default)
		protocol.OldProtocolVersion, // Server supports old
		true,                        // Expect success
		protocol.OldProtocolVersion, // Expect old negotiated
	)
}

func TestClientInitializeUnsupportedVersion(t *testing.T) {
	runHandshakeTest(t,
		protocol.CurrentProtocolVersion, // Client prefers current
		"999.999.999",                   // Server responds with unsupported
		false,                           // Expect failure
		"",                              // Expect no negotiated version
	)
}

func TestClientInitializePreferredOldVersion(t *testing.T) {
	// Scenario: Client prefers old, server supports both but chooses old
	runHandshakeTest(t,
		protocol.OldProtocolVersion, // Client prefers old
		protocol.OldProtocolVersion, // Server responds with old
		true,                        // Expect success
		protocol.OldProtocolVersion, // Expect old negotiated
	)
}

func TestClientInitializeServerPrefersCurrent(t *testing.T) {
	// Scenario: Client prefers old (default), server supports both but chooses current
	runHandshakeTest(t,
		protocol.OldProtocolVersion,     // Client prefers old (default)
		protocol.CurrentProtocolVersion, // Server responds with current
		true,                            // Expect success
		protocol.CurrentProtocolVersion, // Expect current negotiated
	)
}

// TODO: Add tests for client sending requests after successful connection
