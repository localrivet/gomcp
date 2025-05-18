// Package udp provides a UDP implementation of the MCP transport.
//
// This package implements the Transport interface using UDP (User Datagram Protocol),
// suitable for high-throughput, low-latency communication where occasional packet
// loss is acceptable. It includes optional reliability mechanisms that can be
// configured based on application requirements.
package udp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/transport"
)

const (
	// DefaultMaxPacketSize is the maximum UDP packet size we'll use.
	// It's set conservatively to avoid fragmentation at the IP layer.
	DefaultMaxPacketSize = 1400

	// DefaultReadBufferSize is the default size for UDP read buffers.
	DefaultReadBufferSize = 4096

	// DefaultWriteBufferSize is the default size for UDP write buffers.
	DefaultWriteBufferSize = 4096

	// DefaultReadTimeout is the default timeout for read operations.
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout is the default timeout for write operations.
	DefaultWriteTimeout = 10 * time.Second

	// DefaultReconnectDelay is the default delay before reconnection attempts.
	DefaultReconnectDelay = 1 * time.Second

	// DefaultMaxRetries is the default maximum number of retries for operations.
	DefaultMaxRetries = 3

	// MaxConcurrentReassembly is the maximum number of messages that can be
	// reassembled concurrently.
	MaxConcurrentReassembly = 100

	// DefaultFragmentTTL is the default time-to-live for message fragments.
	// If a fragment isn't reassembled within this time, it will be discarded.
	DefaultFragmentTTL = 30 * time.Second

	// HeaderSize is the size of the UDP packet header in bytes.
	// The header format is:
	// - Magic (2 bytes): To identify our protocol
	// - Flags (1 byte): For packet type and control flags
	// - MessageID (4 bytes): Unique ID for the message
	// - FragmentIndex (2 bytes): Index of this fragment
	// - TotalFragments (2 bytes): Total number of fragments
	// - Checksum (4 bytes): CRC32 checksum of the payload
	HeaderSize = 15

	// MagicBytes are the first two bytes of every packet to identify our protocol
	MagicByte1 = 0x4D // 'M'
	MagicByte2 = 0x43 // 'C'

	// Flag values for the packet header
	FlagData           = 0x00 // Data packet
	FlagAck            = 0x01 // Acknowledgment
	FlagReliable       = 0x02 // Request reliable delivery
	FlagLastFragment   = 0x04 // Last fragment of a message
	FlagSingleFragment = 0x08 // Message fits in a single fragment
)

// ErrMessageTooLarge is returned when a message exceeds the maximum size.
var ErrMessageTooLarge = errors.New("message too large")

// ErrTimeout is returned when an operation times out.
var ErrTimeout = errors.New("operation timed out")

// ErrNotInitialized is returned when an operation is attempted before initialization.
var ErrNotInitialized = errors.New("transport not initialized")

// ErrInvalidPacket is returned when a packet doesn't match our protocol.
var ErrInvalidPacket = errors.New("invalid packet format")

// ErrReassemblyFailed is returned when message reassembly fails.
var ErrReassemblyFailed = errors.New("message reassembly failed")

// PacketHeader represents the header of a UDP packet in our protocol.
type PacketHeader struct {
	Magic          [2]byte // Protocol identifier
	Flags          byte    // Packet flags
	MessageID      uint32  // Unique message identifier
	FragmentIndex  uint16  // Index of this fragment
	TotalFragments uint16  // Total number of fragments
	Checksum       uint32  // CRC32 checksum of payload
}

// encodeHeader serializes a packet header to bytes.
func encodeHeader(header *PacketHeader) []byte {
	buf := make([]byte, HeaderSize)
	buf[0] = header.Magic[0]
	buf[1] = header.Magic[1]
	buf[2] = header.Flags
	binary.BigEndian.PutUint32(buf[3:7], header.MessageID)
	binary.BigEndian.PutUint16(buf[7:9], header.FragmentIndex)
	binary.BigEndian.PutUint16(buf[9:11], header.TotalFragments)
	binary.BigEndian.PutUint32(buf[11:15], header.Checksum)
	return buf
}

// decodeHeader parses a packet header from bytes.
func decodeHeader(data []byte) (*PacketHeader, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("packet too small for header: %w", ErrInvalidPacket)
	}

	// Check magic bytes
	if data[0] != MagicByte1 || data[1] != MagicByte2 {
		return nil, fmt.Errorf("invalid magic bytes: %w", ErrInvalidPacket)
	}

	header := &PacketHeader{
		Magic:          [2]byte{data[0], data[1]},
		Flags:          data[2],
		MessageID:      binary.BigEndian.Uint32(data[3:7]),
		FragmentIndex:  binary.BigEndian.Uint16(data[7:9]),
		TotalFragments: binary.BigEndian.Uint16(data[9:11]),
		Checksum:       binary.BigEndian.Uint32(data[11:15]),
	}

	return header, nil
}

// FragmentInfo holds information about a received message fragment.
type FragmentInfo struct {
	ReceivedTime time.Time
	Data         []byte
}

// Transport implements the transport.Transport interface for UDP.
// It supports both client and server modes and provides optional reliability.
type Transport struct {
	transport.BaseTransport
	addr               string        // UDP address (host:port)
	isServer           bool          // Whether this is a server
	conn               *net.UDPConn  // UDP connection
	maxPacketSize      int           // Maximum packet size
	readBufferSize     int           // Read buffer size
	writeBufferSize    int           // Write buffer size
	readTimeout        time.Duration // Read timeout
	writeTimeout       time.Duration // Write timeout
	reconnectDelay     time.Duration // Delay before reconnection
	maxRetries         int           // Maximum number of retries
	fragmentTTL        time.Duration // Time-to-live for fragments
	nextMessageID      uint32        // Next message ID to use
	reliabilityEnabled bool          // Whether reliability mechanisms are enabled
	connMu             sync.Mutex    // Mutex for connection access

	// Reliability fields
	reliabilityLevel     ReliabilityLevel    // Level of reliability guarantees
	retransmitStrategy   RetransmitStrategy  // Strategy for retransmission timing
	initialRetryInterval time.Duration       // Initial retry interval
	maxRetryInterval     time.Duration       // Maximum retry interval
	slidingWindowSize    int                 // Size of the sliding window (ReliabilityFull)
	reliabilityManager   *ReliabilityManager // Manager for reliability mechanisms

	// For server mode
	clientAddrs   map[string]*net.UDPAddr // Map of client addresses
	clientAddrsMu sync.RWMutex            // Mutex for client addresses

	// For message fragmentation and reassembly
	fragments       map[uint32]map[uint16]*FragmentInfo // MessageID -> FragmentIndex -> FragmentInfo
	fragmentsMu     sync.Mutex                          // Mutex for fragments
	reassemblyQueue chan uint32                         // Queue of message IDs ready for reassembly

	// Channels for async operation
	readCh chan []byte   // Channel for received messages
	errCh  chan error    // Channel for errors
	doneCh chan struct{} // Channel for shutdown signal

	running   bool       // Whether the transport is running
	runningMu sync.Mutex // Mutex for running state
}

// UDPOption is a function that configures a Transport.
// These options allow customizing the behavior of the UDP transport.
type UDPOption func(*Transport)

// WithMaxPacketSize sets the maximum packet size.
func WithMaxPacketSize(size int) UDPOption {
	return func(t *Transport) {
		if size > 0 {
			t.maxPacketSize = size
		}
	}
}

// WithReadBufferSize sets the read buffer size.
func WithReadBufferSize(size int) UDPOption {
	return func(t *Transport) {
		if size > 0 {
			t.readBufferSize = size
		}
	}
}

// WithWriteBufferSize sets the write buffer size.
func WithWriteBufferSize(size int) UDPOption {
	return func(t *Transport) {
		if size > 0 {
			t.writeBufferSize = size
		}
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(timeout time.Duration) UDPOption {
	return func(t *Transport) {
		if timeout > 0 {
			t.readTimeout = timeout
		}
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(timeout time.Duration) UDPOption {
	return func(t *Transport) {
		if timeout > 0 {
			t.writeTimeout = timeout
		}
	}
}

// WithReconnectDelay sets the delay before reconnection attempts.
func WithReconnectDelay(delay time.Duration) UDPOption {
	return func(t *Transport) {
		if delay > 0 {
			t.reconnectDelay = delay
		}
	}
}

// WithMaxRetries sets the maximum number of retries for operations.
func WithMaxRetries(retries int) UDPOption {
	return func(t *Transport) {
		if retries > 0 {
			t.maxRetries = retries
		}
	}
}

// WithFragmentTTL sets the time-to-live for message fragments.
func WithFragmentTTL(ttl time.Duration) UDPOption {
	return func(t *Transport) {
		if ttl > 0 {
			t.fragmentTTL = ttl
		}
	}
}

// WithReliability enables or disables reliability mechanisms.
func WithReliability(enabled bool) UDPOption {
	return func(t *Transport) {
		t.reliabilityEnabled = enabled
		if enabled {
			t.reliabilityLevel = ReliabilityBasic
		} else {
			t.reliabilityLevel = ReliabilityNone
		}
	}
}

// NewTransport creates a new UDP transport.
//
// Parameters:
//   - addr: The UDP address in the format "host:port". For server mode, typically ":port".
//     For client mode, "host:port" of the server to connect to.
//   - isServer: Whether this is a server (true) or client (false).
//   - options: Optional configuration settings.
//
// Example:
//
//	// Server mode
//	serverTransport := udp.NewTransport(":8080", true)
//
//	// Client mode
//	clientTransport := udp.NewTransport("example.com:8080", false)
//
//	// With options
//	transport := udp.NewTransport(":8080", true,
//	    udp.WithMaxPacketSize(2048),
//	    udp.WithReadTimeout(5*time.Second),
//	    udp.WithReliability(true))
func NewTransport(addr string, isServer bool, options ...UDPOption) *Transport {
	t := &Transport{
		addr:                 addr,
		isServer:             isServer,
		maxPacketSize:        DefaultMaxPacketSize,
		readBufferSize:       DefaultReadBufferSize,
		writeBufferSize:      DefaultWriteBufferSize,
		readTimeout:          DefaultReadTimeout,
		writeTimeout:         DefaultWriteTimeout,
		reconnectDelay:       DefaultReconnectDelay,
		maxRetries:           DefaultMaxRetries,
		fragmentTTL:          DefaultFragmentTTL,
		clientAddrs:          make(map[string]*net.UDPAddr),
		fragments:            make(map[uint32]map[uint16]*FragmentInfo),
		reassemblyQueue:      make(chan uint32, MaxConcurrentReassembly),
		readCh:               make(chan []byte, 100),
		errCh:                make(chan error, 10),
		doneCh:               make(chan struct{}),
		reliabilityEnabled:   false,
		reliabilityLevel:     ReliabilityNone,
		retransmitStrategy:   RetransmitExponential,
		initialRetryInterval: 500 * time.Millisecond,
		maxRetryInterval:     10 * time.Second,
		slidingWindowSize:    16,
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	return t
}

// Initialize initializes the transport.
// For server mode, it creates a UDP listener.
// For client mode, it establishes a connection to the server.
func (t *Transport) Initialize() error {
	t.runningMu.Lock()
	defer t.runningMu.Unlock()

	if t.running {
		return nil // Already initialized
	}

	var err error
	if t.isServer {
		// Server mode: create a UDP listener
		addr, err := net.ResolveUDPAddr("udp", t.addr)
		if err != nil {
			return fmt.Errorf("failed to resolve UDP address: %w", err)
		}

		t.conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("failed to create UDP listener: %w", err)
		}
	} else {
		// Client mode: connect to the server
		addr, err := net.ResolveUDPAddr("udp", t.addr)
		if err != nil {
			return fmt.Errorf("failed to resolve UDP address: %w", err)
		}

		t.conn, err = net.DialUDP("udp", nil, addr)
		if err != nil {
			return fmt.Errorf("failed to connect to UDP server: %w", err)
		}
	}

	// Set buffer sizes if specified
	if t.readBufferSize > 0 {
		err = t.conn.SetReadBuffer(t.readBufferSize)
		if err != nil {
			return fmt.Errorf("failed to set UDP read buffer size: %w", err)
		}
	}

	if t.writeBufferSize > 0 {
		err = t.conn.SetWriteBuffer(t.writeBufferSize)
		if err != nil {
			return fmt.Errorf("failed to set UDP write buffer size: %w", err)
		}
	}

	// Initialize reliability manager if enabled
	if t.reliabilityEnabled {
		t.reliabilityManager = NewReliabilityManager(t, t.reliabilityLevel)
	}

	return nil
}

// Start starts the transport.
func (t *Transport) Start() error {
	t.runningMu.Lock()
	defer t.runningMu.Unlock()

	if t.running {
		return nil // Already started
	}

	// Initialize if needed
	if t.conn == nil {
		if err := t.Initialize(); err != nil {
			return err
		}
	}

	// Create fresh channels for this session
	t.doneCh = make(chan struct{})
	t.fragments = make(map[uint32]map[uint16]*FragmentInfo)
	t.reassemblyQueue = make(chan uint32, MaxConcurrentReassembly)

	// Start the goroutines for receiving and processing packets
	go t.receivePackets()
	go t.processReassemblyQueue()
	go t.cleanupFragments()

	// Start the reliability manager if enabled
	if t.reliabilityEnabled && t.reliabilityManager != nil {
		t.reliabilityManager.Start()
	}

	t.running = true
	return nil
}

// Stop stops the transport.
func (t *Transport) Stop() error {
	t.runningMu.Lock()
	defer t.runningMu.Unlock()

	if !t.running {
		return nil // Already stopped
	}

	// Signal all goroutines to stop
	close(t.doneCh)

	// Stop the reliability manager if it's running
	if t.reliabilityEnabled && t.reliabilityManager != nil {
		t.reliabilityManager.Stop()
	}

	// Close the connection
	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		if err != nil {
			return fmt.Errorf("failed to close UDP connection: %w", err)
		}
	}

	// Give goroutines a moment to clean up
	// This helps prevent goroutine leaks
	time.Sleep(50 * time.Millisecond)

	// Reset state
	t.running = false

	// Only create new channels when we're starting again, not during shutdown
	// This prevents test goroutines from hanging on old channels

	return nil
}

// Send sends a message over the transport.
func (t *Transport) Send(message []byte) error {
	if t.conn == nil {
		return ErrNotInitialized
	}

	// Generate a unique message ID
	messageID := t.generateMessageID()

	// Check if the message can fit in a single packet
	maxPayloadSize := t.maxPacketSize - HeaderSize
	if len(message) <= maxPayloadSize {
		// This can be sent as a single packet
		err := t.sendSinglePacket(message, messageID)
		if err != nil {
			return err
		}

		// Track message for reliability if enabled
		if t.reliabilityEnabled && t.reliabilityManager != nil {
			t.reliabilityManager.TrackMessage(messageID, message, nil)
		}

		return nil
	}

	// Message needs to be fragmented
	err := t.sendFragmentedMessage(message, messageID, maxPayloadSize)
	if err != nil {
		return err
	}

	// Track fragments for reliability if enabled
	if t.reliabilityEnabled && t.reliabilityManager != nil {
		// We need to store the original fragments for possible retransmission
		// This would be implemented in the sendFragmentedMessage method
		// For now, just track the whole message
		t.reliabilityManager.TrackMessage(messageID, message, nil)
	}

	return nil
}

// sendSinglePacket sends a message as a single packet.
func (t *Transport) sendSinglePacket(message []byte, messageID uint32) error {
	// Create header
	header := &PacketHeader{
		Magic:          [2]byte{MagicByte1, MagicByte2},
		Flags:          FlagSingleFragment,
		MessageID:      messageID,
		FragmentIndex:  0,
		TotalFragments: 1,
		Checksum:       crc32.ChecksumIEEE(message),
	}

	// Add reliability flag if enabled
	if t.reliabilityEnabled {
		header.Flags |= FlagReliable
	}

	// Encode header
	headerBytes := encodeHeader(header)

	// Create packet (header + message)
	packet := make([]byte, len(headerBytes)+len(message))
	copy(packet, headerBytes)
	copy(packet[len(headerBytes):], message)

	// Set write deadline if timeout is specified
	if t.writeTimeout > 0 {
		if err := t.conn.SetWriteDeadline(time.Now().Add(t.writeTimeout)); err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
		defer t.conn.SetWriteDeadline(time.Time{}) // Clear deadline
	}

	// Write packet
	_, err := t.conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	return nil
}

// sendFragmentedMessage fragments a large message and sends it as multiple packets.
func (t *Transport) sendFragmentedMessage(message []byte, messageID uint32, maxPayloadSize int) error {
	// Calculate number of fragments needed
	totalFragments := (len(message) + maxPayloadSize - 1) / maxPayloadSize
	if totalFragments > 65535 {
		return ErrMessageTooLarge
	}

	// Send fragments
	for i := 0; i < totalFragments; i++ {
		// Calculate fragment bounds
		start := i * maxPayloadSize
		end := start + maxPayloadSize
		if end > len(message) {
			end = len(message)
		}

		// Create fragment
		fragment := message[start:end]

		// Create header
		header := &PacketHeader{
			Magic:          [2]byte{MagicByte1, MagicByte2},
			Flags:          0, // Basic data packet
			MessageID:      messageID,
			FragmentIndex:  uint16(i),
			TotalFragments: uint16(totalFragments),
			Checksum:       crc32.ChecksumIEEE(fragment),
		}

		// Set flags
		if i == totalFragments-1 {
			header.Flags |= FlagLastFragment
		}
		if t.reliabilityEnabled {
			header.Flags |= FlagReliable
		}

		// Encode header
		headerBytes := encodeHeader(header)

		// Create packet (header + fragment)
		packet := make([]byte, len(headerBytes)+len(fragment))
		copy(packet, headerBytes)
		copy(packet[len(headerBytes):], fragment)

		// Set write deadline if timeout is specified
		if t.writeTimeout > 0 {
			if err := t.conn.SetWriteDeadline(time.Now().Add(t.writeTimeout)); err != nil {
				return fmt.Errorf("failed to set write deadline: %w", err)
			}
			defer t.conn.SetWriteDeadline(time.Time{}) // Clear deadline
		}

		// Write packet
		_, err := t.conn.Write(packet)
		if err != nil {
			return fmt.Errorf("failed to send fragment %d/%d: %w", i+1, totalFragments, err)
		}

		// Small delay between fragments to prevent overwhelming the network
		// This is especially important for large messages
		if i < totalFragments-1 {
			time.Sleep(time.Microsecond)
		}
	}

	return nil
}

// receivePackets continuously receives UDP packets and processes them.
func (t *Transport) receivePackets() {
	buffer := make([]byte, t.maxPacketSize)

	for {
		select {
		case <-t.doneCh:
			return
		default:
			// Check if connection is still valid
			if t.conn == nil {
				// Connection is gone, check if we should keep running
				select {
				case <-t.doneCh:
					return // Transport is shutting down
				default:
					// Sleep briefly to avoid a tight loop, but not for too long
					time.Sleep(10 * time.Millisecond)
					continue
				}
			}

			// Set read deadline if timeout is specified
			if t.readTimeout > 0 && t.conn != nil {
				err := t.conn.SetReadDeadline(time.Now().Add(t.readTimeout))
				if err != nil {
					select {
					case t.errCh <- fmt.Errorf("failed to set read deadline: %w", err):
					default:
						// Channel full, discard error
					}
				}
			}

			// Read packet
			n, raddr, err := t.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is expected, just try again
					continue
				}

				// Check if we should exit
				select {
				case <-t.doneCh:
					return // Transport is shutting down
				default:
					// Report the error and continue
					select {
					case t.errCh <- fmt.Errorf("failed to read packet: %w", err):
					default:
						// Channel full, discard error
					}
				}

				// Sleep briefly to avoid tight loop on persistent errors
				time.Sleep(10 * time.Millisecond)
				continue
			}

			// Process the packet
			if n > 0 {
				// Make a copy of the data to avoid it being overwritten
				packetData := make([]byte, n)
				copy(packetData, buffer[:n])

				// For server mode, remember client address for responses
				if t.isServer && raddr != nil {
					t.clientAddrsMu.Lock()
					t.clientAddrs[raddr.String()] = raddr
					t.clientAddrsMu.Unlock()
				}

				// Process the packet
				t.processPacket(packetData)
			}
		}
	}
}

// processPacket handles received UDP packets.
func (t *Transport) processPacket(data []byte) {
	// Parse the header
	header, err := decodeHeader(data)
	if err != nil {
		select {
		case t.errCh <- fmt.Errorf("invalid packet header: %w", err):
		default:
			// Channel full, discard error
		}
		return
	}

	// Extract the payload
	payload := data[HeaderSize:]

	// Verify checksum
	if crc32.ChecksumIEEE(payload) != header.Checksum {
		select {
		case t.errCh <- fmt.Errorf("checksum mismatch for message %d, fragment %d",
			header.MessageID, header.FragmentIndex):
		default:
			// Channel full, discard error
		}
		return
	}

	// Handle based on flags
	if header.Flags&FlagAck != 0 {
		// This is an acknowledgment packet
		if t.reliabilityEnabled && t.reliabilityManager != nil {
			// Get the remote address as a string key
			var addrStr string
			if t.isServer {
				// For server, we need the remote address which was captured during receivePackets
				addrStr = t.conn.RemoteAddr().String()
			} else {
				// For client, we're only connected to the server
				addrStr = t.addr
			}

			// Process the acknowledgment
			t.reliabilityManager.HandleAck(header, addrStr)
		}
		return
	}

	// Check if this is a single fragment message
	if header.Flags&FlagSingleFragment != 0 {
		// Send directly to read channel
		select {
		case t.readCh <- payload:
		default:
			select {
			case t.errCh <- errors.New("read channel full, dropping message"):
			default:
				// Both channels full, can't do anything
			}
		}

		// Send acknowledgment if reliable delivery was requested
		if header.Flags&FlagReliable != 0 && t.reliabilityEnabled && t.reliabilityManager != nil {
			ackPacket := t.reliabilityManager.CreateAckPacket(header.MessageID, false)
			_, _ = t.conn.Write(ackPacket)
		}

		return
	}

	// Handle message fragment
	t.handleFragment(header, payload)
}

// handleFragment processes a fragment of a multi-part message.
func (t *Transport) handleFragment(header *PacketHeader, payload []byte) {
	t.fragmentsMu.Lock()
	defer t.fragmentsMu.Unlock()

	messageID := header.MessageID
	fragmentIndex := header.FragmentIndex
	totalFragments := header.TotalFragments

	// Create map for this message if it doesn't exist
	if _, exists := t.fragments[messageID]; !exists {
		t.fragments[messageID] = make(map[uint16]*FragmentInfo)
	}

	// Store the fragment
	t.fragments[messageID][fragmentIndex] = &FragmentInfo{
		ReceivedTime: time.Now(),
		Data:         payload,
	}

	// Send acknowledgment if reliable delivery was requested
	if header.Flags&FlagReliable != 0 && t.reliabilityEnabled && t.reliabilityManager != nil {
		ackPacket := t.reliabilityManager.CreateAckPacket(messageID, false)
		_, _ = t.conn.Write(ackPacket)
	}

	// Check if we have all fragments
	if len(t.fragments[messageID]) == int(totalFragments) {
		// All fragments received, queue for reassembly
		select {
		case t.reassemblyQueue <- messageID:
			// Successfully queued
		default:
			select {
			case t.errCh <- fmt.Errorf("reassembly queue full, dropping message %d", messageID):
			default:
				// Both channels full, can't do anything
			}
		}
	}
}

// cleanupFragments periodically removes expired fragments.
func (t *Transport) cleanupFragments() {
	ticker := time.NewTicker(t.fragmentTTL / 10) // Check 10 times per TTL period
	defer ticker.Stop()

	for {
		select {
		case <-t.doneCh:
			return // Transport is shutting down
		case <-ticker.C:
			t.cleanExpiredFragments()
		}
	}
}

// cleanExpiredFragments removes fragments that have expired.
func (t *Transport) cleanExpiredFragments() {
	now := time.Now()
	expired := []uint32{} // List of expired message IDs

	t.fragmentsMu.Lock()
	defer t.fragmentsMu.Unlock()

	// Find expired message fragments
	for messageID, fragments := range t.fragments {
		// Check the oldest fragment
		oldestTime := now
		for _, fragment := range fragments {
			if fragment.ReceivedTime.Before(oldestTime) {
				oldestTime = fragment.ReceivedTime
			}
		}

		// If the oldest fragment is older than TTL, mark for removal
		if now.Sub(oldestTime) > t.fragmentTTL {
			expired = append(expired, messageID)
		}
	}

	// Remove expired fragments
	for _, messageID := range expired {
		delete(t.fragments, messageID)
	}
}

// processReassemblyQueue handles reassembly of fragmented messages.
func (t *Transport) processReassemblyQueue() {
	for {
		select {
		case <-t.doneCh:
			return // Transport is shutting down
		case messageID, ok := <-t.reassemblyQueue:
			if !ok {
				// Channel is closed
				return
			}

			// Reassemble the message
			message, err := t.reassembleMessage(messageID)
			if err != nil {
				select {
				case t.errCh <- fmt.Errorf("failed to reassemble message %d: %w", messageID, err):
				default:
					// Channel full, discard error
				}
				continue
			}

			// Send the reassembled message to the read channel
			select {
			case t.readCh <- message:
				// Message sent successfully
			default:
				// Read channel is full
				select {
				case t.errCh <- errors.New("read channel full, dropping reassembled message"):
				default:
					// Both channels full, can't do anything
				}
			}

			// Send acknowledgment if reliability is enabled
			if t.reliabilityEnabled && t.reliabilityManager != nil {
				// Create and send a cumulative ack for this message ID
				ackPacket := t.reliabilityManager.CreateAckPacket(messageID, true)
				if t.conn != nil {
					_, _ = t.conn.Write(ackPacket)
				}
			}
		}
	}
}

// reassembleMessage reassembles a fragmented message.
func (t *Transport) reassembleMessage(messageID uint32) ([]byte, error) {
	t.fragmentsMu.Lock()
	defer t.fragmentsMu.Unlock()

	// Get fragments for this message
	fragments, ok := t.fragments[messageID]
	if !ok {
		return nil, fmt.Errorf("no fragments found for message ID %d", messageID)
	}

	// Find total size of message
	var totalSize int
	totalFragments := uint16(len(fragments))

	// Validate we have all fragments
	for i := uint16(0); i < totalFragments; i++ {
		if _, ok := fragments[i]; !ok {
			return nil, fmt.Errorf("missing fragment %d for message ID %d", i, messageID)
		}
		totalSize += len(fragments[i].Data)
	}

	// Reassemble message
	message := make([]byte, totalSize)
	var offset int

	for i := uint16(0); i < totalFragments; i++ {
		fragment := fragments[i]
		copy(message[offset:], fragment.Data)
		offset += len(fragment.Data)
	}

	// Remove fragments now that we've reassembled the message
	delete(t.fragments, messageID)

	return message, nil
}

// Receive receives a message from the transport.
// It returns reassembled messages from the read channel.
func (t *Transport) Receive() ([]byte, error) {
	select {
	case <-t.doneCh:
		return nil, errors.New("transport stopped")
	case err := <-t.errCh:
		return nil, err
	case message := <-t.readCh:
		return message, nil
	}
}

// generateMessageID generates a unique message ID.
func (t *Transport) generateMessageID() uint32 {
	// Simple atomic increment approach
	return atomic.AddUint32(&t.nextMessageID, 1)
}
