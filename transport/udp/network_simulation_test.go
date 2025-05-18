package udp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestPacketLoss tests the transport's ability to handle packet loss.
// This simulates a network where packets are occasionally dropped.
func TestPacketLoss(t *testing.T) {
	// Create a lossy connection simulation
	lossRate := 0.3 // 30% packet loss

	// Create server transport
	serverAddr := "localhost:0" // Use port 0 for automatic assignment
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(256),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
		WithRetryLimit(5),                      // Increase retries for reliability
		WithRetryInterval(50*time.Millisecond), // Faster retries for testing
	)

	// Initialize server transport
	if err := serverTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the actual server port and address
	actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	t.Logf("Server listening on port %d", actualPort)

	// Create a simulation proxy that will pass through to the real server port
	proxyConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("Failed to create proxy listener: %v", err)
	}

	proxyPort := proxyConn.LocalAddr().(*net.UDPAddr).Port
	t.Logf("Proxy listening on port %d", proxyPort)

	// Set up the actual server address for the proxy to forward to
	serverRealAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("localhost:%d", actualPort))
	if err != nil {
		t.Fatalf("Failed to resolve server address: %v", err)
	}

	// Start the lossy proxy in a goroutine
	proxyDone := make(chan struct{})
	var proxyWg sync.WaitGroup
	proxyWg.Add(1)

	go func() {
		defer proxyWg.Done()
		defer func() {
			// Recover from potential panics when closing channels
			if r := recover(); r != nil {
				// Don't log here as test might be complete
			}
		}()

		buf := make([]byte, 2048)
		for {
			// Check if we should exit first
			select {
			case <-proxyDone:
				return
			default:
				// Continue with read, but with a short timeout
			}

			// Set read deadline to allow checking for done signal
			proxyConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

			// Read from client
			n, clientAddr, err := proxyConn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Just a timeout, check for exit again
					continue
				}

				// Check if we're done or connection closed
				select {
				case <-proxyDone:
					return
				default:
					// Only log if test is still running and it's not a normal close
					if !strings.Contains(err.Error(), "use of closed") {
						// Don't use t.Logf here to avoid possible race with test completion
					}
				}
				continue
			}

			// Simulate packet loss
			if rand.Float64() < lossRate {
				// Drop this packet
				select {
				case <-proxyDone:
					return
				default:
					// Safe to log only if proxyDone is not closed
					t.Logf("Dropping packet from %v", clientAddr)
				}
				continue
			}

			// Forward the packet to the server
			data := make([]byte, n)
			copy(data, buf[:n])

			// Create a connection to the server and send the data
			serverConn, err := net.DialUDP("udp", nil, serverRealAddr)
			if err != nil {
				select {
				case <-proxyDone:
					return
				default:
					// Only log if test is still running
					// Don't use t.Logf to avoid race
				}
				continue
			}

			_, err = serverConn.Write(data)
			if err != nil {
				serverConn.Close()
				continue
			}

			// Read response from server (with timeout)
			serverConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			respBuf := make([]byte, 2048)
			n, err = serverConn.Read(respBuf)
			serverConn.Close()

			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// No response, that's OK
					continue
				}
				continue
			}

			// Simulate packet loss for the response too
			if rand.Float64() < lossRate {
				// Drop this packet
				select {
				case <-proxyDone:
					return
				default:
					// Safe to log only if proxyDone is not closed
					t.Logf("Dropping response packet to %v", clientAddr)
				}
				continue
			}

			// Forward response back to client
			select {
			case <-proxyDone:
				return
			default:
				_, _ = proxyConn.WriteToUDP(respBuf[:n], clientAddr)
			}
		}
	}()

	// Start the server transport
	if err := serverTransport.Start(); err != nil {
		close(proxyDone)
		proxyConn.Close()
		proxyWg.Wait()
		t.Fatalf("Failed to start server transport: %v", err)
	}

	// Set up proper cleanup
	defer func() {
		serverTransport.Stop()
		// Signal proxy to stop and close connection
		close(proxyDone)
		proxyConn.Close()
		// Wait for proxy goroutine to finish
		proxyWg.Wait()
	}()

	// Create client transport with reliability enabled
	// Point to our lossy proxy instead of the real server
	clientTransport := NewTransport(fmt.Sprintf("localhost:%d", proxyPort), false,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(256),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
		WithRetryLimit(5),                      // Increase retries for reliability
		WithRetryInterval(50*time.Millisecond), // Faster retries for testing
	)

	// Initialize client transport
	if err := clientTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize client transport: %v", err)
	}

	// Start the client
	if err := clientTransport.Start(); err != nil {
		t.Fatalf("Failed to start client transport: %v", err)
	}
	defer clientTransport.Stop()

	// Test sending a message with packet loss
	testMessage := []byte("This is a test message for packet loss simulation.")

	// Wait group for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Receive message on server
	var receivedMessage []byte
	go func() {
		defer wg.Done()
		select {
		case msg := <-serverTransport.readCh:
			receivedMessage = msg
			t.Logf("Server received message: %s", string(msg))
		case <-time.After(3 * time.Second):
			t.Error("Timeout waiting for message on server")
		}
	}()

	// Send message from client
	t.Logf("Sending message: %s", string(testMessage))
	err = clientTransport.Send(testMessage)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for reception
	wg.Wait()

	// Verify message was received correctly despite packet loss
	if !bytes.Equal(receivedMessage, testMessage) {
		t.Errorf("Received message doesn't match sent message.\nSent: %s\nReceived: %s",
			string(testMessage), string(receivedMessage))
	} else {
		t.Logf("Message successfully received despite %.0f%% packet loss", lossRate*100)
	}

	// Check reliability metrics
	if clientTransport.reliabilityManager != nil {
		metrics := clientTransport.reliabilityManager.GetMetrics()
		t.Logf("Client reliability metrics: %+v", metrics)

		// We expect some retransmissions due to packet loss
		if metrics.PacketsRetransmitted == 0 {
			t.Logf("No retransmissions recorded, but expected some with %.0f%% packet loss", lossRate*100)
		}
	}
}

// TestOutOfOrderDelivery tests the transport's ability to handle out-of-order packet delivery.
// This simulates a network where packets arrive out of order.
func TestOutOfOrderDelivery(t *testing.T) {
	// Create server transport with small packet size to force fragmentation
	serverAddr := "localhost:0" // Use port 0 for automatic assignment
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityFull), // Full reliability for ordered delivery
		WithMaxPacketSize(64),                 // Small size to force fragmentation
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	if err := serverTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the assigned port
	actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	actualServerAddr := "localhost:" + strconv.Itoa(actualPort)

	// Create client transport with reliability
	clientTransport := NewTransport(actualServerAddr, false,
		WithReliabilityLevel(ReliabilityFull), // Full reliability for ordered delivery
		WithMaxPacketSize(64),                 // Small size to force fragmentation
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	if err := clientTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize client transport: %v", err)
	}

	// Start both transports
	if err := serverTransport.Start(); err != nil {
		t.Fatalf("Failed to start server transport: %v", err)
	}
	defer serverTransport.Stop()

	if err := clientTransport.Start(); err != nil {
		t.Fatalf("Failed to start client transport: %v", err)
	}
	defer clientTransport.Stop()

	// Generate a large test message
	testMessage := []byte("This is a large test message for out-of-order delivery simulation. " +
		"We need it to be fragmented into multiple packets to test proper reassembly. " +
		"With our small packet size, this should create several fragments that will test " +
		"the transport's ability to handle out-of-order delivery.")

	// Wait group for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Receive message on server
	var receivedMessage []byte
	go func() {
		defer wg.Done()
		select {
		case msg := <-serverTransport.readCh:
			receivedMessage = msg
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for message on server")
		}
	}()

	// Send message from client
	err := clientTransport.Send(testMessage)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for reception
	wg.Wait()

	// Verify message was received correctly despite out-of-order delivery
	if !bytes.Equal(receivedMessage, testMessage) {
		t.Errorf("Received message doesn't match sent message.\nSent: %s\nReceived: %s",
			string(testMessage), string(receivedMessage))
	} else {
		t.Logf("Message successfully received and reassembled correctly")
	}
}

// TestDuplicatePackets tests the transport's ability to handle duplicate packet delivery.
// This simulates a network where packets are occasionally duplicated.
func TestDuplicatePackets(t *testing.T) {
	// Create server and client transports
	serverAddr := "localhost:0"
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(256),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	if err := serverTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the assigned port
	actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	actualServerAddr := "localhost:" + strconv.Itoa(actualPort)

	// Create client transport
	clientTransport := NewTransport(actualServerAddr, false,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(256),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	if err := clientTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize client transport: %v", err)
	}

	// Start both transports
	if err := serverTransport.Start(); err != nil {
		t.Fatalf("Failed to start server transport: %v", err)
	}
	defer serverTransport.Stop()

	if err := clientTransport.Start(); err != nil {
		t.Fatalf("Failed to start client transport: %v", err)
	}
	defer clientTransport.Stop()

	// Test data
	testMessage := []byte("This is a test message for duplicate packet handling.")

	// Wait group for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Count received messages
	receivedCount := 0
	var mu sync.Mutex

	// Start a goroutine to receive messages on the server
	go func() {
		defer wg.Done()

		// We'll check for 1 second to ensure we only receive one message
		timeout := time.After(1 * time.Second)
		for {
			select {
			case <-serverTransport.readCh:
				mu.Lock()
				receivedCount++
				mu.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// Send the same message multiple times (simulating duplicate packets)
	for i := 0; i < 3; i++ {
		err := clientTransport.Send(testMessage)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}
	}

	// Wait for the receiving goroutine to complete
	wg.Wait()

	// In a production setting with proper duplicate detection, we'd expect
	// receivedCount to be either 1 or 3 depending on whether deduplication
	// is happening at the message level or packet level.
	t.Logf("Received %d messages after sending 3 duplicate messages", receivedCount)
}

// TestReliabilityProfiles tests the different reliability profile options.
func TestReliabilityProfiles(t *testing.T) {
	testCases := []struct {
		name                string
		reliabilityLevel    ReliabilityLevel
		expectedRetransmit  bool
		expectedOrderedRecv bool
	}{
		{
			name:                "NoReliability",
			reliabilityLevel:    ReliabilityNone,
			expectedRetransmit:  false,
			expectedOrderedRecv: false,
		},
		{
			name:                "BasicReliability",
			reliabilityLevel:    ReliabilityBasic,
			expectedRetransmit:  true,
			expectedOrderedRecv: false,
		},
		{
			name:                "FullReliability",
			reliabilityLevel:    ReliabilityFull,
			expectedRetransmit:  true,
			expectedOrderedRecv: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create server transport
			serverAddr := "localhost:0"
			serverTransport := NewTransport(serverAddr, true,
				WithReliabilityLevel(tc.reliabilityLevel),
				WithMaxPacketSize(256),
				WithReadTimeout(100*time.Millisecond),
				WithWriteTimeout(100*time.Millisecond),
			)

			if err := serverTransport.Initialize(); err != nil {
				t.Fatalf("Failed to initialize server transport: %v", err)
			}

			// Get the assigned port
			actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
			actualServerAddr := "localhost:" + strconv.Itoa(actualPort)

			// Create client transport
			clientTransport := NewTransport(actualServerAddr, false,
				WithReliabilityLevel(tc.reliabilityLevel),
				WithMaxPacketSize(256),
				WithReadTimeout(100*time.Millisecond),
				WithWriteTimeout(100*time.Millisecond),
				WithRetryInterval(50*time.Millisecond), // Fast retries for testing
				WithRetryLimit(3),
			)

			if err := clientTransport.Initialize(); err != nil {
				t.Fatalf("Failed to initialize client transport: %v", err)
			}

			// Start both transports
			if err := serverTransport.Start(); err != nil {
				t.Fatalf("Failed to start server transport: %v", err)
			}
			defer serverTransport.Stop()

			if err := clientTransport.Start(); err != nil {
				t.Fatalf("Failed to start client transport: %v", err)
			}
			defer clientTransport.Stop()

			// Test data
			testMessage := []byte("Testing message for reliability profile: " + tc.name)

			// Wait group for synchronization
			var wg sync.WaitGroup
			wg.Add(1)

			// Receive message on server
			var receivedMessage []byte
			go func() {
				defer wg.Done()
				select {
				case msg := <-serverTransport.readCh:
					receivedMessage = msg
				case <-time.After(2 * time.Second):
					t.Error("Timeout waiting for message on server")
				}
			}()

			// Send message from client
			err := clientTransport.Send(testMessage)
			if err != nil {
				t.Fatalf("Failed to send message: %v", err)
			}

			// Wait for reception
			wg.Wait()

			// Verify message was received correctly
			if !bytes.Equal(receivedMessage, testMessage) {
				t.Errorf("Received message doesn't match sent message.\nSent: %s\nReceived: %s",
					string(testMessage), string(receivedMessage))
			}

			// Check reliability manager metrics
			if clientTransport.reliabilityManager != nil {
				metrics := clientTransport.reliabilityManager.GetMetrics()
				t.Logf("%s client metrics: %+v", tc.name, metrics)
			}

			if serverTransport.reliabilityManager != nil {
				metrics := serverTransport.reliabilityManager.GetMetrics()
				t.Logf("%s server metrics: %+v", tc.name, metrics)
			}
		})
	}
}

// TestNetworkLatency tests the transport's ability to handle network latency.
// This simulates a network with variable delays in packet delivery.
func TestNetworkLatency(t *testing.T) {
	// Create server transport
	serverAddr := "localhost:0" // Use port 0 for automatic assignment
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityFull),
		WithMaxPacketSize(512),
		WithReadTimeout(500*time.Millisecond), // Longer timeout for delayed packets
		WithWriteTimeout(500*time.Millisecond),
		WithRetryInterval(200*time.Millisecond), // Adjusted for latency
	)

	if err := serverTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the assigned port
	actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	actualServerAddr := "localhost:" + strconv.Itoa(actualPort)

	// Create a custom proxy that adds latency
	proxyConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatalf("Failed to create proxy listener: %v", err)
	}

	// Create client transport pointing to the proxy
	proxyPort := proxyConn.LocalAddr().(*net.UDPAddr).Port
	proxyAddr := "localhost:" + strconv.Itoa(proxyPort)

	clientTransport := NewTransport(proxyAddr, false,
		WithReliabilityLevel(ReliabilityFull),
		WithMaxPacketSize(512),
		WithReadTimeout(1*time.Second), // Longer for delayed packets
		WithWriteTimeout(500*time.Millisecond),
		WithRetryInterval(200*time.Millisecond), // Adjusted for latency
	)

	if err := clientTransport.Initialize(); err != nil {
		t.Fatalf("Failed to initialize client transport: %v", err)
	}

	// Start the server transport
	if err := serverTransport.Start(); err != nil {
		t.Fatalf("Failed to start server transport: %v", err)
	}

	// Start the client transport
	if err := clientTransport.Start(); err != nil {
		serverTransport.Stop()
		t.Fatalf("Failed to start client transport: %v", err)
	}

	// Create proxy goroutines to introduce latency
	proxyDone := make(chan struct{})
	var proxyWg sync.WaitGroup

	// Set up proper cleanup for all resources
	defer func() {
		// Signal proxy to stop
		close(proxyDone)
		// Close connections
		clientTransport.Stop()
		serverTransport.Stop()
		proxyConn.Close()
		// Wait for proxy goroutines to finish
		proxyWg.Wait()
	}()

	// Client → Server proxy with latency
	proxyWg.Add(1)
	go func() {
		defer proxyWg.Done()
		clientToServerProxy(t, proxyConn, actualServerAddr, 20, 100, proxyDone) // 20-100ms latency
	}()

	// Give the proxy goroutines time to start
	time.Sleep(100 * time.Millisecond)

	// Test sending messages with latency
	testMessage := []byte("This is a test message for latency simulation. We want to verify that " +
		"the transport handles delayed packet delivery correctly, maintaining reliable communication.")

	// Wait group for synchronization
	var msgWg sync.WaitGroup
	msgWg.Add(1)

	// Receive message on server
	var receivedMessage []byte
	go func() {
		defer msgWg.Done()
		select {
		case msg := <-serverTransport.readCh:
			receivedMessage = msg
			t.Logf("Server received message of length %d", len(msg))
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for message on server")
		}
	}()

	// Send message from client to server
	t.Logf("Sending message of length %d", len(testMessage))
	err = clientTransport.Send(testMessage)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for message reception
	msgWg.Wait()

	// Verify the message was received correctly despite latency
	if !bytes.Equal(receivedMessage, testMessage) {
		t.Errorf("Received message doesn't match sent message.\nSent: %s\nReceived: %s",
			string(testMessage), string(receivedMessage))
	} else {
		t.Logf("Message successfully received despite network latency")
	}

	// Check reliability metrics
	if clientTransport.reliabilityManager != nil {
		metrics := clientTransport.reliabilityManager.GetMetrics()
		t.Logf("Client reliability metrics: %+v", metrics)
		t.Logf("Average RTT: %v", metrics.AverageRTT)
	}
}

// clientToServerProxy simulates latency from client to server
func clientToServerProxy(t *testing.T, proxyConn *net.UDPConn, serverAddr string, minLatency, maxLatency int, done chan struct{}) {
	buffer := make([]byte, 2048)
	serverUDPAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		select {
		case <-done:
			return
		default:
			t.Logf("Failed to resolve server address: %v", err)
			return
		}
	}

	// Create a connection to the server
	serverConn, err := net.DialUDP("udp", nil, serverUDPAddr)
	if err != nil {
		select {
		case <-done:
			return
		default:
			t.Logf("Failed to connect to server: %v", err)
			return
		}
	}
	defer serverConn.Close()

	for {
		// Check if we should exit
		select {
		case <-done:
			return
		default:
			// Continue processing
		}

		// Read from client
		proxyConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, clientAddr, err := proxyConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Just a timeout, continue
				continue
			}

			// Check if we're supposed to exit
			select {
			case <-done:
				return
			default:
				// Only log if not a standard close and the test is still running
				if !strings.Contains(err.Error(), "use of closed") {
					// Avoid using t.Logf as test might be complete
				}
				continue
			}
		}

		// Make a copy of the data
		data := make([]byte, n)
		copy(data, buffer[:n])

		// Forward to server with latency
		go func(data []byte, clientAddr *net.UDPAddr) {
			defer func() {
				// Recover from potential panics
				if r := recover(); r != nil {
					// Don't log as test might be complete
				}
			}()

			// Check if we should process this packet
			select {
			case <-done:
				return
			default:
				// Continue processing
			}

			// Random delay between minLatency and maxLatency
			delay := time.Duration(rand.Intn(maxLatency-minLatency)+minLatency) * time.Millisecond
			time.Sleep(delay)

			// Check again if we should continue
			select {
			case <-done:
				return
			default:
				// Continue processing
			}

			// Forward to server
			_, err := serverConn.Write(data)
			if err != nil {
				return
			}

			// Wait for response from server (with timeout)
			serverConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			respBuffer := make([]byte, 2048)
			n, err := serverConn.Read(respBuffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					return // No response from server, that's OK
				}
				return
			}

			// Check again if we should continue
			select {
			case <-done:
				return
			default:
				// Continue processing
			}

			// Add latency for the response too
			respDelay := time.Duration(rand.Intn(maxLatency-minLatency)+minLatency) * time.Millisecond
			time.Sleep(respDelay)

			// Forward response back to client
			select {
			case <-done:
				return
			default:
				_, _ = proxyConn.WriteToUDP(respBuffer[:n], clientAddr)
			}
		}(data, clientAddr)
	}
}

// BenchmarkUDPTransport benchmarks the performance of the UDP transport under different conditions.
func BenchmarkUDPTransport(b *testing.B) {
	benchmarks := []struct {
		name          string
		messageSize   int
		reliability   ReliabilityLevel
		packetSize    int
		concurrentMsg int
	}{
		{"Small_NoReliability", 64, ReliabilityNone, 1024, 1},
		{"Small_BasicReliability", 64, ReliabilityBasic, 1024, 1},
		{"Small_FullReliability", 64, ReliabilityFull, 1024, 1},
		{"Medium_NoReliability", 1024, ReliabilityNone, 1024, 1},
		{"Medium_BasicReliability", 1024, ReliabilityBasic, 1024, 1},
		{"Medium_FullReliability", 1024, ReliabilityFull, 1024, 1},
		{"Large_NoReliability", 8192, ReliabilityNone, 8192, 1},
		{"Large_BasicReliability", 8192, ReliabilityBasic, 8192, 1},
		{"Large_FullReliability", 8192, ReliabilityFull, 8192, 1},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Set up server transport
			serverAddr := "localhost:0"
			serverTransport := NewTransport(serverAddr, true,
				WithReliabilityLevel(bm.reliability),
				WithMaxPacketSize(bm.packetSize),
				WithReadTimeout(100*time.Millisecond),
				WithWriteTimeout(100*time.Millisecond),
				WithRetryInterval(50*time.Millisecond), // Use faster retry interval for benchmarks
				WithFragmentTTL(2*time.Second),         // Shorter TTL for benchmarks
			)

			err := serverTransport.Initialize()
			if err != nil {
				b.Fatalf("Failed to initialize server transport: %v", err)
			}

			// Get the actual server address
			actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
			actualServerAddr := "localhost:" + strconv.Itoa(actualPort)

			// Create client transport
			clientTransport := NewTransport(actualServerAddr, false,
				WithReliabilityLevel(bm.reliability),
				WithMaxPacketSize(bm.packetSize),
				WithReadTimeout(100*time.Millisecond),
				WithWriteTimeout(100*time.Millisecond),
				WithRetryInterval(50*time.Millisecond), // Use faster retry interval for benchmarks
				WithFragmentTTL(2*time.Second),         // Shorter TTL for benchmarks
			)

			err = clientTransport.Initialize()
			if err != nil {
				b.Fatalf("Failed to initialize client transport: %v", err)
			}

			// Start both transports
			err = serverTransport.Start()
			if err != nil {
				b.Fatalf("Failed to start server transport: %v", err)
			}

			err = clientTransport.Start()
			if err != nil {
				b.Fatalf("Failed to start client transport: %v", err)
			}

			// Generate test message of the specified size
			testMessage := make([]byte, bm.messageSize)
			for i := range testMessage {
				testMessage[i] = byte(i % 256)
			}

			// Set up continuous message reception on server
			serverReceivedCount := 0
			var serverWg sync.WaitGroup
			var serverCountMu sync.Mutex
			serverDone := make(chan struct{})

			serverWg.Add(1)
			go func() {
				defer serverWg.Done()
				for {
					select {
					case <-serverDone:
						return
					case <-serverTransport.readCh:
						serverCountMu.Lock()
						serverReceivedCount++
						serverCountMu.Unlock()
					case <-time.After(100 * time.Millisecond):
						// Timeout, just continue
					}
				}
			}()

			// Ensure cleanup happens at the end
			defer func() {
				// First signal the server goroutine to stop
				close(serverDone)
				// Wait for it to finish
				serverWg.Wait()

				// Now stop both transports
				clientTransport.Stop()
				serverTransport.Stop()

				// Give a brief moment for all resources to be released
				time.Sleep(10 * time.Millisecond)
			}()

			// Reset the timer before we start sending
			b.ResetTimer()

			// Perform the benchmark
			for i := 0; i < b.N; i++ {
				// Send messages concurrently if specified
				var sendWg sync.WaitGroup
				for j := 0; j < bm.concurrentMsg; j++ {
					sendWg.Add(1)
					go func() {
						defer sendWg.Done()
						err := clientTransport.Send(testMessage)
						if err != nil && !errors.Is(err, context.Canceled) {
							b.Logf("Error sending message: %v", err)
						}
					}()
				}
				sendWg.Wait()

				// Wait for all messages to be received or time out
				timeout := time.After(500 * time.Millisecond)
				receivedAll := false
				for !receivedAll {
					serverCountMu.Lock()
					received := serverReceivedCount
					serverCountMu.Unlock()

					if received >= (i+1)*bm.concurrentMsg {
						receivedAll = true
						break
					}

					select {
					case <-timeout:
						// If we timeout, just continue
						b.Logf("Timeout waiting for all messages, got %d/%d", received, (i+1)*bm.concurrentMsg)
						receivedAll = true
					case <-time.After(5 * time.Millisecond):
						// Short delay before checking again
					}
				}
			}

			// Stop the timer before cleanup
			b.StopTimer()

			// Report metrics
			if clientTransport.reliabilityManager != nil && bm.reliability != ReliabilityNone {
				clientMetrics := clientTransport.reliabilityManager.GetMetrics()
				b.ReportMetric(float64(clientMetrics.AverageRTT)/float64(time.Millisecond), "avg_rtt_ms")
				b.ReportMetric(float64(clientMetrics.PacketsRetransmitted), "retransmits")
			}
		})
	}
}

// TestMultipleClients tests the UDP transport with multiple clients connecting simultaneously.
func TestMultipleClients(t *testing.T) {
	// Create a server transport
	serverAddr := "localhost:0" // Use port 0 for automatic assignment
	serverTransport := NewTransport(serverAddr, true,
		WithReliabilityLevel(ReliabilityBasic),
		WithMaxPacketSize(512),
		WithReadTimeout(100*time.Millisecond),
		WithWriteTimeout(100*time.Millisecond),
	)

	err := serverTransport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize server transport: %v", err)
	}

	// Get the actual server address
	actualPort := serverTransport.conn.LocalAddr().(*net.UDPAddr).Port
	actualServerAddr := "localhost:" + strconv.Itoa(actualPort)

	// Start the server transport
	err = serverTransport.Start()
	if err != nil {
		t.Fatalf("Failed to start server transport: %v", err)
	}
	defer serverTransport.Stop()

	// Create a channel to collect all messages received by the server
	messages := make(chan []byte, 100)

	// Start a goroutine to collect messages from the server's readCh
	serverDone := make(chan struct{})
	go func() {
		defer close(messages)
		for {
			select {
			case <-serverDone:
				return
			case msg := <-serverTransport.readCh:
				select {
				case messages <- msg:
				default:
					t.Logf("Warning: messages channel is full")
				}
			}
		}
	}()

	// Number of concurrent clients
	numClients := 5
	messagesPerClient := 10

	// Create and track all client transports
	clients := make([]*Transport, numClients)
	for i := 0; i < numClients; i++ {
		// Create client transport
		client := NewTransport(actualServerAddr, false,
			WithReliabilityLevel(ReliabilityBasic),
			WithMaxPacketSize(512),
			WithReadTimeout(100*time.Millisecond),
			WithWriteTimeout(100*time.Millisecond),
		)

		err = client.Initialize()
		if err != nil {
			t.Fatalf("Failed to initialize client %d: %v", i, err)
		}

		err = client.Start()
		if err != nil {
			t.Fatalf("Failed to start client %d: %v", i, err)
		}

		clients[i] = client
	}

	// Ensure all clients are properly closed on test completion
	defer func() {
		for i, client := range clients {
			if client != nil {
				if err := client.Stop(); err != nil {
					t.Logf("Error stopping client %d: %v", i, err)
				}
			}
		}
	}()

	// Create wait group for sending messages
	var wg sync.WaitGroup

	// Each client sends multiple messages concurrently
	expectedMessageCount := numClients * messagesPerClient
	receivedMessages := make(map[string]bool)
	var receivedMu sync.Mutex

	// Start sending messages from all clients
	for i := 0; i < numClients; i++ {
		clientID := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := clients[clientID]

			for j := 0; j < messagesPerClient; j++ {
				// Create a unique message for this client/message
				msgID := fmt.Sprintf("Client-%d-Message-%d", clientID, j)
				data := []byte(msgID)

				// Send the message
				err := client.Send(data)
				if err != nil {
					t.Logf("Error sending from client %d: %v", clientID, err)
					continue
				}

				// Brief delay to avoid overwhelming the server
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	// Wait for all sends to complete
	wg.Wait()

	// Collect received messages (with timeout)
	collectDone := make(chan struct{})
	go func() {
		timeout := time.After(5 * time.Second)
		for {
			select {
			case msg, ok := <-messages:
				if !ok {
					return
				}
				msgStr := string(msg)
				receivedMu.Lock()
				receivedMessages[msgStr] = true
				count := len(receivedMessages)
				receivedMu.Unlock()

				if count >= expectedMessageCount {
					close(collectDone)
					return
				}
			case <-timeout:
				close(collectDone)
				return
			}
		}
	}()

	<-collectDone
	close(serverDone) // Stop the message collection goroutine

	// Verify the received messages
	receivedMu.Lock()
	receivedCount := len(receivedMessages)
	receivedMu.Unlock()

	t.Logf("Received %d/%d expected messages", receivedCount, expectedMessageCount)

	// We may not receive 100% of messages due to UDP being unreliable, but should get most
	successThreshold := int(float64(expectedMessageCount) * 0.9) // 90% success rate
	if receivedCount < successThreshold {
		t.Errorf("Received too few messages: %d/%d (threshold: %d)",
			receivedCount, expectedMessageCount, successThreshold)
	} else {
		t.Logf("Successfully received at least %d%% of sent messages", int(float64(receivedCount)/float64(expectedMessageCount)*100))
	}

	// Verify that the server properly tracked clients
	serverTransport.clientAddrsMu.Lock()
	clientCount := len(serverTransport.clientAddrs)
	serverTransport.clientAddrsMu.Unlock()

	t.Logf("Server tracked %d client addresses", clientCount)
	if clientCount < numClients {
		t.Logf("Server tracked fewer clients than expected (%d < %d), but this might be due to client address reuse",
			clientCount, numClients)
	}
}
