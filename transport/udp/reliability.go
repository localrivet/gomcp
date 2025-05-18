// Package udp provides a UDP implementation of the MCP transport.
package udp

import (
	"hash/crc32"
	"sync"
	"time"
)

// ReliabilityLevel defines different levels of reliability guarantees.
type ReliabilityLevel int

const (
	// ReliabilityNone provides no additional reliability beyond UDP.
	ReliabilityNone ReliabilityLevel = iota

	// ReliabilityBasic provides basic acknowledgments and retries.
	ReliabilityBasic

	// ReliabilityFull provides full reliability with ordered delivery.
	ReliabilityFull
)

// AckType defines the type of acknowledgment.
type AckType byte

const (
	// AckSingle acknowledges a single message.
	AckSingle AckType = iota

	// AckCumulative acknowledges all messages up to a sequence number.
	AckCumulative
)

// RetransmitStrategy defines how retransmission timing is calculated.
type RetransmitStrategy int

const (
	// RetransmitFixed uses a fixed interval for retransmissions.
	RetransmitFixed RetransmitStrategy = iota

	// RetransmitExponential uses exponential backoff for retransmissions.
	RetransmitExponential
)

// AckInfo represents information about an acknowledgment.
type AckInfo struct {
	Type      AckType   // Type of acknowledgment
	MessageID uint32    // Message ID being acknowledged
	Timestamp time.Time // When the ack was received
}

// PendingMessage represents a message waiting for acknowledgment.
type PendingMessage struct {
	MessageID       uint32             // Unique message identifier
	Data            []byte             // Original message data
	FirstSentTime   time.Time          // When the message was first sent
	LastSentTime    time.Time          // When the message was last sent
	RetryCount      int                // Number of retransmission attempts
	RetryLimit      int                // Maximum number of retries
	NextRetryTime   time.Time          // When to retry next
	RetryInterval   time.Duration      // Current retry interval
	InitialInterval time.Duration      // Initial retry interval
	MaxInterval     time.Duration      // Maximum retry interval
	Strategy        RetransmitStrategy // Retransmission strategy
	Fragments       [][]byte           // Message fragments if fragmented
	Acknowledged    bool               // Whether the message has been acknowledged
}

// ReliabilityMetrics tracks statistics about the reliability layer.
type ReliabilityMetrics struct {
	PacketsSent          int64           // Total packets sent
	PacketsRetransmitted int64           // Packets that needed retransmission
	AcksReceived         int64           // Acknowledgments received
	MessagesFailed       int64           // Messages that failed after all retries
	AverageRTT           time.Duration   // Average round-trip time
	rtts                 []time.Duration // Recent RTTs for calculation
	rttsMu               sync.Mutex      // Mutex for RTTs
	maxRTTs              int             // Maximum number of RTTs to track
}

// ReliabilityManager handles reliable message delivery for UDP transport.
type ReliabilityManager struct {
	transport            *Transport                 // Reference to the transport
	level                ReliabilityLevel           // Current reliability level
	pendingMessages      map[uint32]*PendingMessage // Messages waiting for ack
	pendingMu            sync.Mutex                 // Mutex for pending messages
	lastAcks             map[string]AckInfo         // Last ack from each client (server mode)
	lastAcksMu           sync.RWMutex               // Mutex for last acks
	retransmitCh         chan uint32                // Channel for message IDs to retransmit
	initialRetryInterval time.Duration              // Initial retry interval
	maxRetryInterval     time.Duration              // Maximum retry interval
	retryStrategy        RetransmitStrategy         // Strategy for retry timing
	retryLimit           int                        // Maximum number of retries
	slidingWindowSize    int                        // Size of the sliding window (ReliabilityFull)
	metrics              ReliabilityMetrics         // Metrics for monitoring
	running              bool                       // Whether the manager is running
	done                 chan struct{}              // Shutdown signal
}

// NewReliabilityManager creates a new reliability manager.
func NewReliabilityManager(transport *Transport, level ReliabilityLevel) *ReliabilityManager {
	rm := &ReliabilityManager{
		transport:            transport,
		level:                level,
		pendingMessages:      make(map[uint32]*PendingMessage),
		lastAcks:             make(map[string]AckInfo),
		retransmitCh:         make(chan uint32, 100),
		initialRetryInterval: 500 * time.Millisecond,
		maxRetryInterval:     10 * time.Second,
		retryStrategy:        RetransmitExponential,
		retryLimit:           5,
		slidingWindowSize:    16,
		metrics: ReliabilityMetrics{
			maxRTTs: 100,
		},
		done: make(chan struct{}),
	}

	return rm
}

// Start begins the reliability manager's background processes.
func (rm *ReliabilityManager) Start() {
	if rm.running {
		return
	}

	rm.running = true

	// Start the retransmission worker
	go rm.retransmitWorker()

	// Start the timeout checker
	go rm.timeoutChecker()
}

// Stop halts the reliability manager's background processes.
func (rm *ReliabilityManager) Stop() {
	if !rm.running {
		return
	}

	close(rm.done)
	rm.running = false
}

// TrackMessage starts tracking a message for reliable delivery.
func (rm *ReliabilityManager) TrackMessage(messageID uint32, data []byte, fragments [][]byte) {
	if rm.level == ReliabilityNone {
		return // No reliability needed
	}

	rm.pendingMu.Lock()
	defer rm.pendingMu.Unlock()

	now := time.Now()

	pm := &PendingMessage{
		MessageID:       messageID,
		Data:            data,
		FirstSentTime:   now,
		LastSentTime:    now,
		RetryCount:      0,
		RetryLimit:      rm.retryLimit,
		NextRetryTime:   now.Add(rm.initialRetryInterval),
		RetryInterval:   rm.initialRetryInterval,
		InitialInterval: rm.initialRetryInterval,
		MaxInterval:     rm.maxRetryInterval,
		Strategy:        rm.retryStrategy,
		Fragments:       fragments,
		Acknowledged:    false,
	}

	rm.pendingMessages[messageID] = pm
}

// HandleAck processes an acknowledgment packet.
func (rm *ReliabilityManager) HandleAck(header *PacketHeader, addr string) {
	if rm.level == ReliabilityNone {
		return
	}

	messageID := header.MessageID
	now := time.Now()

	// Update last ack from this client (server mode)
	rm.lastAcksMu.Lock()
	ackType := AckSingle
	if header.Flags&0x10 != 0 { // Check if cumulative flag is set
		ackType = AckCumulative
	}

	rm.lastAcks[addr] = AckInfo{
		Type:      ackType,
		MessageID: messageID,
		Timestamp: now,
	}
	rm.lastAcksMu.Unlock()

	// Mark message as acknowledged
	rm.pendingMu.Lock()
	defer rm.pendingMu.Unlock()

	if pm, exists := rm.pendingMessages[messageID]; exists {
		// Calculate RTT for metrics
		rtt := now.Sub(pm.FirstSentTime)
		rm.updateRTTMetrics(rtt)

		// Mark as acknowledged
		pm.Acknowledged = true
		delete(rm.pendingMessages, messageID)
	}

	// For cumulative acknowledgments, also mark all earlier messages as acknowledged
	if ackType == AckCumulative {
		for id, pm := range rm.pendingMessages {
			if id <= messageID {
				pm.Acknowledged = true
				delete(rm.pendingMessages, id)
			}
		}
	}
}

// CreateAckPacket creates an acknowledgment packet.
func (rm *ReliabilityManager) CreateAckPacket(messageID uint32, cumulative bool) []byte {
	// Create header
	header := &PacketHeader{
		Magic:          [2]byte{MagicByte1, MagicByte2},
		Flags:          FlagAck,
		MessageID:      messageID,
		FragmentIndex:  0,
		TotalFragments: 0,
		Checksum:       0, // Ack packets don't need payload checksum
	}

	// Set cumulative flag if needed
	if cumulative {
		header.Flags |= 0x10 // Custom flag for cumulative ack
	}

	// Encode header
	return encodeHeader(header)
}

// retransmitWorker handles message retransmissions.
func (rm *ReliabilityManager) retransmitWorker() {
	for {
		select {
		case <-rm.done:
			return
		case messageID := <-rm.retransmitCh:
			rm.retransmitMessage(messageID)
		}
	}
}

// retransmitMessage resends a message.
func (rm *ReliabilityManager) retransmitMessage(messageID uint32) {
	rm.pendingMu.Lock()
	pm, exists := rm.pendingMessages[messageID]
	if !exists || pm.Acknowledged {
		rm.pendingMu.Unlock()
		return
	}

	// Update retry count and time
	pm.RetryCount++
	now := time.Now()
	pm.LastSentTime = now

	// Calculate next retry time based on strategy
	if pm.Strategy == RetransmitExponential {
		// Exponential backoff: double the interval each time, up to max
		pm.RetryInterval = min(pm.RetryInterval*2, pm.MaxInterval)
	}
	pm.NextRetryTime = now.Add(pm.RetryInterval)

	// Check if we've exceeded retry limit
	if pm.RetryCount > pm.RetryLimit {
		// Message failed after all retries
		delete(rm.pendingMessages, messageID)
		rm.pendingMu.Unlock()

		// Update metrics
		rm.metrics.MessagesFailed++
		return
	}

	// Get a copy of the data while holding the lock
	var data []byte
	if len(pm.Fragments) > 0 {
		// This is a fragmented message
		fragments := make([][]byte, len(pm.Fragments))
		copy(fragments, pm.Fragments)
		rm.pendingMu.Unlock()

		// Send all fragments
		for _, fragment := range fragments {
			// Since we don't have a sendRawPacket method yet, use the conn directly
			if rm.transport.conn != nil {
				_, _ = rm.transport.conn.Write(fragment)
			}
		}
	} else {
		// This is a single packet message
		data = make([]byte, len(pm.Data))
		copy(data, pm.Data)
		rm.pendingMu.Unlock()

		// Recreate and send the packet
		messageID := pm.MessageID
		header := &PacketHeader{
			Magic:          [2]byte{MagicByte1, MagicByte2},
			Flags:          FlagSingleFragment | FlagReliable,
			MessageID:      messageID,
			FragmentIndex:  0,
			TotalFragments: 1,
			Checksum:       crc32.ChecksumIEEE(data),
		}

		headerBytes := encodeHeader(header)
		packet := make([]byte, len(headerBytes)+len(data))
		copy(packet, headerBytes)
		copy(packet[len(headerBytes):], data)

		// Since we don't have a sendRawPacket method yet, use the conn directly
		if rm.transport.conn != nil {
			_, _ = rm.transport.conn.Write(packet)
		}
	}

	// Update metrics
	rm.metrics.PacketsRetransmitted++
}

// timeoutChecker periodically checks for messages that need retransmission.
func (rm *ReliabilityManager) timeoutChecker() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-rm.done:
			return
		case <-ticker.C:
			rm.checkTimeouts()
		}
	}
}

// checkTimeouts identifies messages that have timed out and need retransmission.
func (rm *ReliabilityManager) checkTimeouts() {
	now := time.Now()

	rm.pendingMu.Lock()
	defer rm.pendingMu.Unlock()

	for messageID, pm := range rm.pendingMessages {
		if !pm.Acknowledged && now.After(pm.NextRetryTime) {
			// Send to retransmit channel without blocking
			select {
			case rm.retransmitCh <- messageID:
				// Successfully queued for retransmission
			default:
				// Channel full, will try again on next check
			}
		}
	}
}

// updateRTTMetrics updates the round-trip time metrics.
func (rm *ReliabilityManager) updateRTTMetrics(rtt time.Duration) {
	rm.metrics.rttsMu.Lock()
	defer rm.metrics.rttsMu.Unlock()

	// Add to the RTT list
	rm.metrics.rtts = append(rm.metrics.rtts, rtt)

	// Limit the size of the RTT list
	if len(rm.metrics.rtts) > rm.metrics.maxRTTs {
		rm.metrics.rtts = rm.metrics.rtts[1:]
	}

	// Calculate average RTT
	var total time.Duration
	for _, d := range rm.metrics.rtts {
		total += d
	}

	if len(rm.metrics.rtts) > 0 {
		rm.metrics.AverageRTT = total / time.Duration(len(rm.metrics.rtts))
	}
}

// GetMetrics returns the current reliability metrics.
func (rm *ReliabilityManager) GetMetrics() ReliabilityMetrics {
	return rm.metrics
}

// WithReliabilityLevel sets the reliability level for the transport.
func WithReliabilityLevel(level ReliabilityLevel) UDPOption {
	return func(t *Transport) {
		if level != ReliabilityNone {
			t.reliabilityEnabled = true
			t.reliabilityLevel = level
		} else {
			t.reliabilityEnabled = false
			t.reliabilityLevel = ReliabilityNone
		}
	}
}

// WithRetransmitStrategy sets the retransmission strategy.
func WithRetransmitStrategy(strategy RetransmitStrategy) UDPOption {
	return func(t *Transport) {
		t.retransmitStrategy = strategy
	}
}

// WithRetryLimit sets the maximum number of retries.
func WithRetryLimit(limit int) UDPOption {
	return func(t *Transport) {
		if limit > 0 {
			t.maxRetries = limit
		}
	}
}

// WithRetryInterval sets the initial retry interval.
func WithRetryInterval(interval time.Duration) UDPOption {
	return func(t *Transport) {
		if interval > 0 {
			t.initialRetryInterval = interval
		}
	}
}

// WithMaxRetryInterval sets the maximum retry interval.
func WithMaxRetryInterval(interval time.Duration) UDPOption {
	return func(t *Transport) {
		if interval > 0 {
			t.maxRetryInterval = interval
		}
	}
}

// WithSlidingWindowSize sets the size of the sliding window.
func WithSlidingWindowSize(size int) UDPOption {
	return func(t *Transport) {
		if size > 0 {
			t.slidingWindowSize = size
		}
	}
}
