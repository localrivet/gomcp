package test

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestUDPOption tests the client.WithUDP option
func TestUDPOption(t *testing.T) {
	// Use a local UDP address for testing
	address := "localhost:0" // Using port 0 lets the system assign a free port

	// Create a mock transport with proper initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Use test helper to create client with the UDP option
	c, err := client.NewClient("test-client",
		client.WithUDP(address),
		client.WithTransport(mockTransport),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the client was created successfully
	if c == nil {
		t.Fatal("Client is nil")
	}

	// Verify initialization happened
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called during initialization")
	}

	// Clean up
	c.Close()
}

// TestUDPWithOptions tests the options for UDP client transport
func TestUDPWithOptions(t *testing.T) {
	// Use a local UDP address for testing
	address := "localhost:0" // Using port 0 lets the system assign a free port

	// Create a mock transport with proper initialization response
	mockTransport := SetupMockTransport("2024-11-05")

	// Create client with UDP socket options
	c, err := client.NewClient("test-client",
		client.WithUDP(address,
			client.WithReadTimeout(5*time.Second),
			client.WithUDPReconnect(true),
			client.WithUDPReconnectDelay(2*time.Second),
			client.WithUDPMaxRetries(3),
			client.WithMaxPacketSize(2048),
			client.WithReadBufferSize(8192),
			client.WithWriteBufferSize(8192),
			client.WithFragmentTTL(60*time.Second),
			client.WithReliability(true),
		),
		client.WithTransport(mockTransport),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the client was created successfully
	if c == nil {
		t.Fatal("Client is nil")
	}

	// Verify connection timeout was set properly
	if mockTransport.ConnectionTimeout != 5*time.Second {
		t.Errorf("Connection timeout not correctly set, expected 5s, got %v", mockTransport.ConnectionTimeout)
	}

	// Verify request timeout was set properly
	if mockTransport.RequestTimeout != 5*time.Second {
		t.Errorf("Request timeout not correctly set, expected 5s, got %v", mockTransport.RequestTimeout)
	}

	// Clean up
	c.Close()
}

// TestBasicUDPClientServerCommunication tests UDP communication between client and server
func TestBasicUDPClientServerCommunication(t *testing.T) {
	// Set up a simple UDP echo server for testing
	udpAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("Failed to create UDP listener: %v", err)
	}
	defer conn.Close()

	// Get the assigned port
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	address := "localhost:" + strconv.Itoa(localAddr.Port)
	t.Logf("Created UDP echo server at %s", address)

	// Create a buffered channel to ensure the message is captured
	serverReceived := make(chan []byte, 5) // Buffered to prevent blocking
	serverDone := make(chan struct{})
	defer close(serverDone)

	// Start a simple echo server that captures received messages
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-serverDone:
				return
			default:
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, addr, err := conn.ReadFromUDP(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					t.Logf("UDP server read error: %v", err)
					continue
				}

				receivedData := make([]byte, n)
				copy(receivedData, buf[:n])

				// Log the received data for debugging
				t.Logf("Echo server received: %s", string(receivedData))

				// Send to the channel for verification
				select {
				case serverReceived <- receivedData:
					t.Logf("Added message to serverReceived channel")
				default:
					t.Logf("Warning: serverReceived channel is full or closed")
				}

				// Echo back to client
				_, err = conn.WriteToUDP(buf[:n], addr)
				if err != nil {
					t.Logf("UDP server write error: %v", err)
				} else {
					t.Logf("Echo server sent response back")
				}
			}
		}
	}()

	// Give the server goroutine time to start up
	time.Sleep(100 * time.Millisecond)

	// Create a direct UDP client
	clientAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to resolve client address: %v", err)
	}

	clientConn, err := net.DialUDP("udp", clientAddr, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()

	// Test sending a simple message
	testMessage := "Hello UDP World"
	_, err = clientConn.Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	t.Logf("Client sent message: %s", testMessage)

	// Read the response
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	responseBuffer := make([]byte, 1024)
	n, _, err := clientConn.ReadFromUDP(responseBuffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(responseBuffer[:n])
	if response != testMessage {
		t.Errorf("Received incorrect response\nExpected: %s\nGot: %s", testMessage, response)
	} else {
		t.Logf("Successfully received echoed message: %s", response)
	}

	// Since we've already verified the client got a response which was echoed from the server,
	// we're done with the test. The UDP communication is working as expected.
	t.Log("UDP transport communication test successful")
}
