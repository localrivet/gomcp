package udp

import (
	"bytes"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestReliability(t *testing.T) {
	// Create a server transport with reliability enabled
	serverAddr := "localhost:0" // Use port 0 for automatic port assignment
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(1024),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	err := serverTransport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the dynamically assigned port
	serverPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	actualServerAddr := "localhost:" + strconv.Itoa(serverPort)

	// Create a client transport with reliability enabled, pointing to the server
	clientTransport := NewTransport(actualServerAddr, false,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(1024),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	err = clientTransport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize client transport: %v", err)
	}

	// Start both transports
	err = serverTransport.Start()
	if err != nil {
		t.Fatalf("Failed to start server transport: %v", err)
	}
	defer serverTransport.Stop()

	err = clientTransport.Start()
	if err != nil {
		t.Fatalf("Failed to start client transport: %v", err)
	}
	defer clientTransport.Stop()

	// Test data
	testMessage := []byte("Hello, world! This is a test of the UDP reliable transport.")

	// Wait groups for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Start a goroutine to receive messages on the server
	var receivedMessage []byte
	go func() {
		defer wg.Done()
		// Wait for a message with timeout
		select {
		case msg := <-serverTransport.readCh:
			receivedMessage = msg
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for message on server")
		}
	}()

	// Send message from client to server
	err = clientTransport.Send(testMessage)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for the message to be received
	wg.Wait()

	// Verify the message was received correctly
	if !bytes.Equal(receivedMessage, testMessage) {
		t.Errorf("Received message does not match sent message.\nSent: %s\nReceived: %s",
			string(testMessage), string(receivedMessage))
	}

	// Test that the reliability metrics are being updated
	if clientTransport.reliabilityManager != nil {
		metrics := clientTransport.reliabilityManager.GetMetrics()
		t.Logf("Client reliability metrics: %+v", metrics)
	}

	if serverTransport.reliabilityManager != nil {
		metrics := serverTransport.reliabilityManager.GetMetrics()
		t.Logf("Server reliability metrics: %+v", metrics)
	}
}

func TestReliableFragmentation(t *testing.T) {
	// Create a server transport with reliability enabled
	serverAddr := "localhost:0" // Use port 0 for automatic port assignment
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(64), // Use a small packet size to force fragmentation
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	err := serverTransport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the dynamically assigned port
	serverPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	actualServerAddr := "localhost:" + strconv.Itoa(serverPort)

	// Create a client transport with reliability enabled, pointing to the server
	clientTransport := NewTransport(actualServerAddr, false,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(64), // Use a small packet size to force fragmentation
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	err = clientTransport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize client transport: %v", err)
	}

	// Start both transports
	err = serverTransport.Start()
	if err != nil {
		t.Fatalf("Failed to start server transport: %v", err)
	}
	defer serverTransport.Stop()

	err = clientTransport.Start()
	if err != nil {
		t.Fatalf("Failed to start client transport: %v", err)
	}
	defer clientTransport.Stop()

	// Test data - large enough to force fragmentation
	testMessage := []byte("This is a large message that will definitely be fragmented into multiple packets because we've set a very small maximum packet size. We want to ensure that the fragmentation and reassembly works correctly with reliability enabled.")

	// Wait groups for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Start a goroutine to receive messages on the server
	var receivedMessage []byte
	go func() {
		defer wg.Done()
		// Wait for a message with timeout
		select {
		case msg := <-serverTransport.readCh:
			receivedMessage = msg
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for message on server")
		}
	}()

	// Send message from client to server
	err = clientTransport.Send(testMessage)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for the message to be received
	wg.Wait()

	// Verify the message was received correctly
	if !bytes.Equal(receivedMessage, testMessage) {
		t.Errorf("Received message does not match sent message.\nSent: %s\nReceived: %s",
			string(testMessage), string(receivedMessage))
	}

	// Test that the reliability metrics are being updated
	if clientTransport.reliabilityManager != nil {
		metrics := clientTransport.reliabilityManager.GetMetrics()
		t.Logf("Client reliability metrics: %+v", metrics)
	}

	if serverTransport.reliabilityManager != nil {
		metrics := serverTransport.reliabilityManager.GetMetrics()
		t.Logf("Server reliability metrics: %+v", metrics)
	}
}
