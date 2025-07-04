package embedded

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	transport := NewTransport()

	if transport.bufferSize != 100 {
		t.Errorf("Expected default buffer size 100, got %d", transport.bufferSize)
	}

	if transport.timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", transport.timeout)
	}
}

func TestNewTransportWithOptions(t *testing.T) {
	transport := NewTransport(
		WithBufferSize(50),
		WithTimeout(10*time.Second),
	)

	if transport.bufferSize != 50 {
		t.Errorf("Expected buffer size 50, got %d", transport.bufferSize)
	}

	if transport.timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", transport.timeout)
	}
}

func TestNewTransportPair(t *testing.T) {
	server, client := NewTransportPair()

	// Both should have the same configuration
	if server.bufferSize != client.bufferSize {
		t.Errorf("Buffer sizes don't match: server=%d, client=%d", server.bufferSize, client.bufferSize)
	}

	// They should share the same done channel
	if server.done != client.done {
		t.Error("Server and client should share the same done channel")
	}
}

func TestInitializeAndStart(t *testing.T) {
	transport := NewTransport()

	// Test Initialize
	if err := transport.Initialize(); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	// Channels should be initialized
	if transport.serverToClient == nil {
		t.Error("serverToClient channel not initialized")
	}

	// Test Start
	if err := transport.Start(); err != nil {
		t.Errorf("Start failed: %v", err)
	}

	if !transport.IsStarted() {
		t.Error("Transport should be started")
	}

	// Test double start
	if err := transport.Start(); err == nil {
		t.Error("Expected error on double start")
	}

	// Clean up
	defer transport.Stop()
}

func TestSendReceive(t *testing.T) {
	t.Logf("🔧 Creating transport pair...")
	server, client := NewTransportPair()

	// Initialize and start both
	t.Logf("🔧 Initializing server...")
	if err := server.Initialize(); err != nil {
		t.Fatalf("Server initialize failed: %v", err)
	}
	t.Logf("🔧 Initializing client...")
	if err := client.Initialize(); err != nil {
		t.Fatalf("Client initialize failed: %v", err)
	}
	t.Logf("🔧 Starting server...")
	if err := server.Start(); err != nil {
		t.Fatalf("Server start failed: %v", err)
	}
	t.Logf("🔧 Starting client...")
	if err := client.Start(); err != nil {
		t.Fatalf("Client start failed: %v", err)
	}

	defer func() {
		t.Logf("🔧 Stopping transports...")
		server.Stop()
		client.Stop()
	}()

	// Test client to server communication
	testMessage := []byte("Hello from client")

	t.Logf("🔧 Sending message from client...")
	if err := client.Send(testMessage); err != nil {
		t.Errorf("Client send failed: %v", err)
	}

	t.Logf("🔧 Waiting for server to receive message...")
	// Server should receive the message
	received, err := server.Receive()
	if err != nil {
		t.Errorf("Server receive failed: %v", err)
	}

	t.Logf("🔧 Received: %s", string(received))
	if string(received) != string(testMessage) {
		t.Errorf("Expected %s, got %s", string(testMessage), string(received))
	}
}

func TestMessageHandler(t *testing.T) {
	server, client := NewTransportPair()

	// Set up echo handler on server - use atomic for race-free access
	var handlerCalled atomic.Bool
	server.SetMessageHandler(func(message []byte) ([]byte, error) {
		handlerCalled.Store(true)
		// Echo the message back
		return message, nil
	})

	// Initialize and start
	server.Initialize()
	client.Initialize()
	server.Start()
	client.Start()

	defer func() {
		server.Stop()
		client.Stop()
	}()

	// Send message from client
	testMessage := []byte("test message")
	if err := client.Send(testMessage); err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Give some time for message processing
	time.Sleep(50 * time.Millisecond)

	if !handlerCalled.Load() {
		t.Error("Message handler was not called")
	}

	// The message handler should have been called and sent a response
	// Client should receive the response via Receive()
	response, err := client.Receive()
	if err != nil {
		t.Errorf("Failed to receive response: %v", err)
	} else if string(response) != string(testMessage) {
		t.Errorf("Expected echo %s, got %s", string(testMessage), string(response))
	}
}

func TestJSONRPCCommunication(t *testing.T) {
	server, client := NewTransportPair()

	// Set up JSON-RPC handler on server
	server.SetMessageHandler(func(message []byte) ([]byte, error) {
		// Parse JSON-RPC request
		var req struct {
			JSONRPC string      `json:"jsonrpc"`
			Method  string      `json:"method"`
			Params  interface{} `json:"params"`
			ID      interface{} `json:"id"`
		}

		if err := json.Unmarshal(message, &req); err != nil {
			return nil, err
		}

		// Create response
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "Hello from server",
		}

		return json.Marshal(response)
	})

	// Initialize and start
	server.Initialize()
	client.Initialize()
	server.Start()
	client.Start()

	defer func() {
		server.Stop()
		client.Stop()
	}()

	// Send JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"params":  map[string]string{"message": "hello"},
		"id":      1,
	}

	requestBytes, _ := json.Marshal(request)
	if err := client.Send(requestBytes); err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Client should receive the JSON-RPC response via Receive()
	responseBytes, err := client.Receive()
	if err != nil {
		t.Errorf("Failed to receive response: %v", err)
		return
	}

	var response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Result  string      `json:"result"`
	}

	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if response.Result != "Hello from server" {
		t.Errorf("Expected 'Hello from server', got %s", response.Result)
	}

	if response.ID != float64(1) { // JSON unmarshals numbers as float64
		t.Errorf("Expected ID 1, got %v", response.ID)
	}
}

func TestConcurrentAccess(t *testing.T) {
	server, client := NewTransportPair()

	// Set up echo handler
	server.SetMessageHandler(func(message []byte) ([]byte, error) {
		return message, nil
	})

	server.Initialize()
	client.Initialize()
	server.Start()
	client.Start()

	defer func() {
		server.Stop()
		client.Stop()
	}()

	// Send multiple messages concurrently
	const numMessages = 10
	var wg sync.WaitGroup

	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			message := []byte("message " + string(rune('0'+id)))
			if err := client.Send(message); err != nil {
				t.Errorf("Send %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Give time for all messages to be processed
	time.Sleep(100 * time.Millisecond)

	// Check that all messages were processed
	stats := server.GetChannelStats()
	t.Logf("Channel stats: %+v", stats)
}

func TestStop(t *testing.T) {
	transport := NewTransport()
	transport.Initialize()
	transport.Start()

	if !transport.IsStarted() {
		t.Error("Transport should be started")
	}

	if err := transport.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if transport.IsStarted() {
		t.Error("Transport should be stopped")
	}

	// Test send after stop
	if err := transport.Send([]byte("test")); err == nil {
		t.Error("Expected error when sending after stop")
	}
}

func TestSendTimeout(t *testing.T) {
	transport := NewTransport(WithTimeout(100 * time.Millisecond))
	transport.Initialize()
	transport.Start()
	defer transport.Stop()

	// Fill the channel to capacity to trigger timeout
	for i := 0; i < transport.bufferSize+1; i++ {
		err := transport.Send([]byte("test message"))
		if err != nil && err.Error() == "send timeout" {
			// Expected timeout error
			return
		}
	}

	t.Error("Expected send timeout error")
}

func TestGetChannelStats(t *testing.T) {
	server, client := NewTransportPair()
	server.Initialize()
	client.Initialize()
	server.Start()
	client.Start()
	defer func() {
		server.Stop()
		client.Stop()
	}()

	// Send a few messages
	client.Send([]byte("msg1"))
	client.Send([]byte("msg2"))

	// Check stats
	stats := server.GetChannelStats()

	expectedKeys := []string{"serverToClient", "clientToServer", "serverErrors", "clientErrors"}
	for _, key := range expectedKeys {
		if _, exists := stats[key]; !exists {
			t.Errorf("Expected key %s in stats", key)
		}
	}
}
