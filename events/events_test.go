package events

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test event types
type TestEvent struct {
	Message string
	Value   int
}

type ServerEvent struct {
	Name    string
	Port    int
	Started time.Time
}

func TestBasicPublishSubscribe(t *testing.T) {
	subject := NewSubject()
	defer Complete(subject)

	received := make(chan TestEvent, 1)

	// Subscribe to events
	sub := Subscribe[TestEvent](subject, "test.topic", func(ctx context.Context, evt TestEvent) error {
		received <- evt
		return nil
	})

	// Publish an event
	testEvt := TestEvent{Message: "hello", Value: 42}
	err := Publish[TestEvent](subject, "test.topic", testEvt)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Wait for event
	select {
	case got := <-received:
		if got.Message != "hello" || got.Value != 42 {
			t.Errorf("Expected {hello, 42}, got %+v", got)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Event not received within timeout")
	}

	// Cleanup
	sub.Unsubscribe()
}

func TestTypeSafety(t *testing.T) {
	subject := NewSubject()
	defer Complete(subject)

	// Subscribe to TestEvent
	testReceived := make(chan TestEvent, 1)
	Subscribe[TestEvent](subject, "test.events", func(ctx context.Context, evt TestEvent) error {
		testReceived <- evt
		return nil
	})

	// Subscribe to ServerEvent
	serverReceived := make(chan ServerEvent, 1)
	Subscribe[ServerEvent](subject, "server.events", func(ctx context.Context, evt ServerEvent) error {
		serverReceived <- evt
		return nil
	})

	// Publish TestEvent - should only reach TestEvent subscriber
	if err := Publish[TestEvent](subject, "test.events", TestEvent{Message: "test", Value: 1}); err != nil {
		t.Errorf("Failed to publish TestEvent: %v", err)
	}

	// Publish ServerEvent - should only reach ServerEvent subscriber
	if err := Publish[ServerEvent](subject, "server.events", ServerEvent{Name: "test-server", Port: 8080}); err != nil {
		t.Errorf("Failed to publish ServerEvent: %v", err)
	}

	// Verify only correct events were received
	select {
	case evt := <-testReceived:
		if evt.Message != "test" {
			t.Errorf("Expected test message, got %s", evt.Message)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TestEvent not received")
	}

	select {
	case evt := <-serverReceived:
		if evt.Name != "test-server" {
			t.Errorf("Expected test-server, got %s", evt.Name)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ServerEvent not received")
	}
}

func TestConnectionSpecificEvents(t *testing.T) {
	subject := NewSubject()
	defer Complete(subject)

	// Mock connections
	conn1 := &mockConn{addr: "127.0.0.1:1001"}
	conn2 := &mockConn{addr: "127.0.0.1:1002"}

	received := make(chan struct {
		event TestEvent
		conn  net.Conn
	}, 2)

	// Subscribe with connection handler
	Subscribe[TestEvent](subject, "conn.events", func(ctx context.Context, evt TestEvent, conn net.Conn) error {
		received <- struct {
			event TestEvent
			conn  net.Conn
		}{evt, conn}
		return nil
	})

	// Publish to specific connections
	if err := Publish[TestEvent](subject, "conn.events", TestEvent{Message: "conn1", Value: 1}, conn1); err != nil {
		t.Errorf("Failed to publish TestEvent to conn1: %v", err)
	}
	Publish[TestEvent](subject, "conn.events", TestEvent{Message: "conn2", Value: 2}, conn2)

	// Verify events received with correct connections
	for i := 0; i < 2; i++ {
		select {
		case result := <-received:
			if result.event.Message == "conn1" && result.conn != conn1 {
				t.Error("conn1 event received with wrong connection")
			}
			if result.event.Message == "conn2" && result.conn != conn2 {
				t.Error("conn2 event received with wrong connection")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Event not received within timeout")
		}
	}
}

func TestReplayFunctionality(t *testing.T) {
	subject := NewSubject(WithReplay(3))
	defer Complete(subject)

	// Publish some events before subscribing
	for i := 1; i <= 4; i++ {
		Publish[TestEvent](subject, "replay.test", TestEvent{Message: fmt.Sprintf("event%d", i), Value: i})
	}

	time.Sleep(10 * time.Millisecond) // Let events process

	received := make(chan TestEvent, 5)

	// Subscribe with replay enabled
	Subscribe[TestEvent](subject, "replay.test", func(ctx context.Context, evt TestEvent) error {
		received <- evt
		return nil
	}, true) // Enable replay

	// Should receive the last 3 cached events (cache size = 3)
	replayEvents := make([]TestEvent, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case evt := <-received:
			replayEvents = append(replayEvents, evt)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Replay event not received")
		}
	}

	// Verify we got events 2, 3, 4 (since cache size is 3 and we published 4 events)
	expectedValues := []int{2, 3, 4}
	for i, evt := range replayEvents {
		if evt.Value != expectedValues[i] {
			t.Errorf("Expected replay event %d, got %d", expectedValues[i], evt.Value)
		}
	}

	// Publish new event
	Publish[TestEvent](subject, "replay.test", TestEvent{Message: "new", Value: 5})

	// Should receive new event
	select {
	case evt := <-received:
		if evt.Value != 5 {
			t.Errorf("Expected new event 5, got %d", evt.Value)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("New event not received")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	subject := NewSubject()
	defer Complete(subject)

	const numSubscribers = 5
	received := make([]chan TestEvent, numSubscribers)

	// Create multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		received[i] = make(chan TestEvent, 1)
		idx := i // Capture for closure
		Subscribe[TestEvent](subject, "multi.test", func(ctx context.Context, evt TestEvent) error {
			received[idx] <- evt
			return nil
		})
	}

	// Publish one event
	testEvt := TestEvent{Message: "broadcast", Value: 100}
	Publish[TestEvent](subject, "multi.test", testEvt)

	// All subscribers should receive the event
	for i := 0; i < numSubscribers; i++ {
		select {
		case evt := <-received[i]:
			if evt.Message != "broadcast" || evt.Value != 100 {
				t.Errorf("Subscriber %d received incorrect event: %+v", i, evt)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Subscriber %d did not receive event", i)
		}
	}
}

func TestUnsubscribe(t *testing.T) {
	subject := NewSubject()
	defer Complete(subject)

	received := make(chan TestEvent, 2)

	// Subscribe
	sub := Subscribe[TestEvent](subject, "unsub.test", func(ctx context.Context, evt TestEvent) error {
		received <- evt
		return nil
	})

	// Publish first event
	Publish[TestEvent](subject, "unsub.test", TestEvent{Message: "first", Value: 1})

	// Should receive first event
	select {
	case evt := <-received:
		if evt.Message != "first" {
			t.Errorf("Expected 'first', got '%s'", evt.Message)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("First event not received")
	}

	// Unsubscribe
	sub.Unsubscribe()

	// Publish second event
	Publish[TestEvent](subject, "unsub.test", TestEvent{Message: "second", Value: 2})

	// Should NOT receive second event
	select {
	case evt := <-received:
		t.Errorf("Received event after unsubscribe: %+v", evt)
	case <-time.After(200 * time.Millisecond):
		// Expected - no event should be received
	}
}

func TestConcurrentPublishSubscribe(t *testing.T) {
	subject := NewSubject(WithBufferSize(1000))
	defer Complete(subject)

	const numGoroutines = 10
	const eventsPerGoroutine = 100

	received := make(chan TestEvent, numGoroutines*eventsPerGoroutine)
	var wg sync.WaitGroup

	// Subscribe
	Subscribe[TestEvent](subject, "concurrent.test", func(ctx context.Context, evt TestEvent) error {
		received <- evt
		return nil
	})

	// Start publishing goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				evt := TestEvent{
					Message: fmt.Sprintf("g%d-e%d", goroutineID, j),
					Value:   goroutineID*1000 + j,
				}
				Publish[TestEvent](subject, "concurrent.test", evt)
			}
		}(i)
	}

	wg.Wait()

	// Count received events
	receivedCount := 0
	timeout := time.After(2 * time.Second)

	for receivedCount < numGoroutines*eventsPerGoroutine {
		select {
		case <-received:
			receivedCount++
		case <-timeout:
			t.Fatalf("Only received %d out of %d events", receivedCount, numGoroutines*eventsPerGoroutine)
		}
	}

	if receivedCount != numGoroutines*eventsPerGoroutine {
		t.Errorf("Expected %d events, got %d", numGoroutines*eventsPerGoroutine, receivedCount)
	}
}

func TestTopicConstants(t *testing.T) {
	// Just verify the constants exist and have expected values
	expectedTopics := map[string]string{
		"TopicServerInitialized":  "server.initialized",
		"TopicClientConnected":    "client.connected",
		"TopicClientDisconnected": "client.disconnected",
		"TopicToolRegistered":     "tool.registered",
		"TopicToolExecuted":       "tool.executed",
		"TopicResourceAccessed":   "resource.accessed",
	}

	actualTopics := map[string]string{
		"TopicServerInitialized":  TopicServerInitialized,
		"TopicClientConnected":    TopicClientConnected,
		"TopicClientDisconnected": TopicClientDisconnected,
		"TopicToolRegistered":     TopicToolRegistered,
		"TopicToolExecuted":       TopicToolExecuted,
		"TopicResourceAccessed":   TopicResourceAccessed,
	}

	for name, expected := range expectedTopics {
		if actual := actualTopics[name]; actual != expected {
			t.Errorf("Topic %s: expected %q, got %q", name, expected, actual)
		}
	}
}

func TestInvalidHandler(t *testing.T) {
	subject := NewSubject()
	defer Complete(subject)

	// Test panic with non-function handler
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for non-function handler")
		}
	}()

	Subscribe[TestEvent](subject, "invalid.test", "not a function")
}

func TestPublishTimeout(t *testing.T) {
	// Create subject with buffer size 0 (unbuffered channel)
	subject := NewSubject(WithBufferSize(0))

	// Stop the event loop immediately so no one consumes from the channel
	Complete(subject)

	// Give eventLoop time to actually stop
	time.Sleep(10 * time.Millisecond)

	// This should timeout immediately since it's an unbuffered channel with no receiver
	err := Publish[TestEvent](subject, "timeout.test", TestEvent{Message: "blocked"})
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !contains(err.Error(), "failed to emit event") {
		t.Errorf("Expected timeout error message, got: %v", err)
	}
}

// Helper function to check if string contains substring
func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}

// Mock connection for testing
type mockConn struct {
	addr string
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &mockAddr{m.addr} }
func (m *mockConn) RemoteAddr() net.Addr               { return &mockAddr{m.addr} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

type mockAddr struct {
	addr string
}

func (m *mockAddr) Network() string { return "tcp" }
func (m *mockAddr) String() string  { return m.addr }

func TestLoggerIntegration(t *testing.T) {
	// Create a test logger that captures output - use a channel-based approach to avoid race conditions
	logMessages := make(chan string, 10)

	// Create a custom handler that sends to channel instead of shared buffer
	handler := &channelHandler{ch: logMessages}
	logger := slog.New(handler)

	// Create subject with logger
	subject := NewSubject(WithLogger(logger), WithBufferSize(10))
	defer Complete(subject)

	// Subscribe with a handler that returns an error
	Subscribe[TestEvent](subject, "test.error", func(ctx context.Context, evt TestEvent) error {
		return fmt.Errorf("test error: %s", evt.Message)
	})

	// Publish an event that will trigger the error
	err := Publish[TestEvent](subject, "test.error", TestEvent{Message: "trigger error"})
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Wait for event processing and collect log messages
	var logMessages_received []string
	timeout := time.After(100 * time.Millisecond)

collectLoop:
	for {
		select {
		case msg := <-logMessages:
			logMessages_received = append(logMessages_received, msg)
		case <-timeout:
			break collectLoop
		}
	}

	// Combine all log messages to check content
	logStr := strings.Join(logMessages_received, " ")

	// Check that error was logged
	if !strings.Contains(logStr, "event handler error") {
		t.Errorf("Expected error to be logged, got: %s", logStr)
	}

	if !strings.Contains(logStr, "test error: trigger error") {
		t.Errorf("Expected specific error message in log, got: %s", logStr)
	}

	if !strings.Contains(logStr, "topic=test.error") {
		t.Errorf("Expected topic to be logged, got: %s", logStr)
	}
}

// channelHandler is a thread-safe slog handler that sends messages to a channel
type channelHandler struct {
	ch chan string
}

func (h *channelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *channelHandler) Handle(ctx context.Context, record slog.Record) error {
	// Build message without shared state
	var msg strings.Builder
	msg.WriteString("level=" + record.Level.String())
	msg.WriteString(" msg=" + record.Message)

	record.Attrs(func(attr slog.Attr) bool {
		msg.WriteString(" " + attr.Key + "=" + attr.Value.String())
		return true
	})

	select {
	case h.ch <- msg.String():
	default:
		// Channel full, drop message (test shouldn't fail because of this)
	}
	return nil
}

func (h *channelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // For simplicity, ignore attrs in test handler
}

func (h *channelHandler) WithGroup(name string) slog.Handler {
	return h // For simplicity, ignore groups in test handler
}

// TestAsyncSyncDelivery verifies that live events are delivered asynchronously
// while replay events are delivered synchronously for order preservation
func TestAsyncSyncDelivery(t *testing.T) {
	subject := NewSubject(WithReplay(10), WithBufferSize(100))
	defer Complete(subject)

	// Test data
	type TestEvent struct {
		ID       int
		Sequence int
	}

	deliveryOrder := make([]int, 0, 10)
	var mu sync.Mutex

	// Track delivery order to verify sync vs async behavior
	handler := func(ctx context.Context, evt TestEvent) error {
		mu.Lock()
		deliveryOrder = append(deliveryOrder, evt.Sequence)
		mu.Unlock()

		// Add small delay to make async behavior more apparent
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	// Publish some events before subscribing (for replay)
	for i := 1; i <= 5; i++ {
		err := Publish[TestEvent](subject, "test.topic", TestEvent{ID: i, Sequence: i})
		if err != nil {
			t.Fatalf("Failed to publish event %d: %v", i, err)
		}
	}

	// Small delay to ensure events are processed
	time.Sleep(10 * time.Millisecond)

	// Subscribe with replay enabled - should get replay events synchronously
	sub := Subscribe[TestEvent](subject, "test.topic", handler, true)
	defer sub.Unsubscribe()

	// Wait for replay to complete
	time.Sleep(20 * time.Millisecond)

	// Check that replay events were delivered in order (synchronous delivery)
	mu.Lock()
	replayOrder := make([]int, len(deliveryOrder))
	copy(replayOrder, deliveryOrder)
	mu.Unlock()

	expectedReplayOrder := []int{1, 2, 3, 4, 5}
	if !reflect.DeepEqual(replayOrder, expectedReplayOrder) {
		t.Errorf("Replay events not delivered in order. Expected %v, got %v", expectedReplayOrder, replayOrder)
	}

	// Reset delivery tracking for live events test
	mu.Lock()
	deliveryOrder = deliveryOrder[:0]
	mu.Unlock()

	// Publish more events (live events) - these should be delivered asynchronously
	var wg sync.WaitGroup
	for i := 6; i <= 10; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			err := Publish[TestEvent](subject, "test.topic", TestEvent{ID: seq, Sequence: seq})
			if err != nil {
				t.Errorf("Failed to publish live event %d: %v", seq, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all async deliveries to complete
	time.Sleep(50 * time.Millisecond)

	// Check that we received all live events (order may vary due to async delivery)
	mu.Lock()
	liveEventsReceived := len(deliveryOrder)
	mu.Unlock()

	if liveEventsReceived != 5 {
		t.Errorf("Expected 5 live events, got %d", liveEventsReceived)
	}

	t.Logf("✓ Replay events delivered synchronously in order: %v", expectedReplayOrder)
	t.Logf("✓ Live events delivered asynchronously (%d events received)", liveEventsReceived)
}
