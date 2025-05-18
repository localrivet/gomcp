package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// MockTransport is a mock transport for testing with enhanced capabilities
type MockTransport struct {
	// Connection state
	ConnectCalled      bool
	DisconnectCalled   bool
	ConnectionAttempts int
	Connected          bool
	mu                 sync.Mutex

	// Request/Response handling
	LastSentMessage []byte
	RequestHistory  []RequestRecord
	ResponseQueue   []ResponseConfig
	DefaultResponse *ResponseConfig
	ResponseHistory [][]byte // Track responses sent back to client

	// Network simulation
	NetworkConditions NetworkConditions
	RequestTimeout    time.Duration
	ConnectionTimeout time.Duration
	ErrorRate         float64 // Probability of returning an error on any request

	// Handlers
	NotificationHandlerFunc func(method string, params []byte)

	// RequestInterceptor can modify outgoing requests for testing
	// If set, every request sent through Send/SendWithContext will be processed by this function
	RequestInterceptor func(request []byte) []byte
}

// RequestRecord stores information about a sent request
type RequestRecord struct {
	Message   []byte
	Raw       []byte // The raw request message (same as Message)
	Timestamp time.Time
	ID        interface{}
	Method    string
}

// ResponseConfig defines a response configuration
type ResponseConfig struct {
	Data         []byte
	Error        error
	Delay        time.Duration
	Condition    func([]byte) bool // If set, this response only applies if the condition is true
	Probability  float64           // Probability (0-1) of this response being used (for simulating flakiness)
	AfterUseFunc func()            // Called after this response is used
}

// NetworkConditions allows configuration of simulated network conditions
type NetworkConditions struct {
	Latency         time.Duration // Base latency for all operations
	LatencyJitter   time.Duration // Random variation in latency
	PacketLossRate  float64       // Probability (0-1) of packet loss
	DisconnectAfter int           // Number of requests after which to simulate disconnection, 0 = never
	RequestCount    int           // Internal counter of processed requests
}

// NewMockTransport creates a new mock transport with default settings
func NewMockTransport() *MockTransport {
	return &MockTransport{
		RequestHistory: make([]RequestRecord, 0),
		ResponseQueue:  make([]ResponseConfig, 0),
		NetworkConditions: NetworkConditions{
			Latency:         0,
			LatencyJitter:   0,
			PacketLossRate:  0,
			DisconnectAfter: 0,
		},
		RequestTimeout:    30 * time.Second,
		ConnectionTimeout: 10 * time.Second,
	}
}

// WithDefaultResponse sets a default response to use when the queue is empty
func (m *MockTransport) WithDefaultResponse(data []byte, err error) *MockTransport {
	m.DefaultResponse = &ResponseConfig{
		Data:  data,
		Error: err,
	}
	return m
}

// WithNetworkConditions configures simulated network conditions
func (m *MockTransport) WithNetworkConditions(conditions NetworkConditions) *MockTransport {
	m.NetworkConditions = conditions
	return m
}

// QueueResponse adds a response to the queue (responses are used in FIFO order)
func (m *MockTransport) QueueResponse(data []byte, err error) *MockTransport {
	m.ResponseQueue = append(m.ResponseQueue, ResponseConfig{
		Data:  data,
		Error: err,
	})
	return m
}

// QueueResponseWithDelay adds a response with a specified delay
func (m *MockTransport) QueueResponseWithDelay(data []byte, err error, delay time.Duration) *MockTransport {
	m.ResponseQueue = append(m.ResponseQueue, ResponseConfig{
		Data:  data,
		Error: err,
		Delay: delay,
	})
	return m
}

// QueueConditionalResponse adds a response that is only used if the condition is met
func (m *MockTransport) QueueConditionalResponse(data []byte, err error, condition func([]byte) bool) *MockTransport {
	m.ResponseQueue = append(m.ResponseQueue, ResponseConfig{
		Data:      data,
		Error:     err,
		Condition: condition,
	})
	return m
}

// QueueFlaky adds a response that is used with the given probability
func (m *MockTransport) QueueFlaky(data []byte, err error, probability float64) *MockTransport {
	m.ResponseQueue = append(m.ResponseQueue, ResponseConfig{
		Data:        data,
		Error:       err,
		Probability: probability,
	})
	return m
}

// ClearResponses removes all queued responses
func (m *MockTransport) ClearResponses() *MockTransport {
	m.ResponseQueue = make([]ResponseConfig, 0)
	return m
}

// ClearHistory clears the request history
func (m *MockTransport) ClearHistory() *MockTransport {
	m.RequestHistory = make([]RequestRecord, 0)
	return m
}

// GetRequestByID finds a request in the history by its ID
func (m *MockTransport) GetRequestByID(id interface{}) *RequestRecord {
	for i := len(m.RequestHistory) - 1; i >= 0; i-- {
		if m.RequestHistory[i].ID == id {
			return &m.RequestHistory[i]
		}
	}
	return nil
}

// GetRequestsByMethod finds requests in the history by method
func (m *MockTransport) GetRequestsByMethod(method string) []RequestRecord {
	var result []RequestRecord
	for _, rec := range m.RequestHistory {
		if rec.Method == method {
			result = append(result, rec)
		}
	}
	return result
}

// Connect implements the Transport interface
func (m *MockTransport) Connect() error {
	// Record connection attempt
	{
		m.mu.Lock()
		m.ConnectCalled = true
		m.ConnectionAttempts++
		m.mu.Unlock()
	}

	// Simulate network conditions - outside of lock
	var delay time.Duration
	if m.NetworkConditions.Latency > 0 {
		delay = m.NetworkConditions.Latency
		if m.NetworkConditions.LatencyJitter > 0 {
			jitter := time.Duration(rand.Int63n(int64(m.NetworkConditions.LatencyJitter)))
			delay += jitter
		}
		time.Sleep(delay)
	}

	// Simulate connection failures
	if m.NetworkConditions.PacketLossRate > 0 {
		if rand.Float64() < m.NetworkConditions.PacketLossRate {
			return &MockError{Message: "simulated connection failure"}
		}
	}

	// Set connected state
	m.mu.Lock()
	m.Connected = true
	m.mu.Unlock()

	return nil
}

// ConnectWithContext implements the Transport interface
func (m *MockTransport) ConnectWithContext(ctx context.Context) error {
	if ctx == nil {
		return m.Connect()
	}

	connCh := make(chan error, 1)
	go func() {
		connCh <- m.Connect()
	}()

	select {
	case err := <-connCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Disconnect implements the Transport interface
func (m *MockTransport) Disconnect() error {
	// Use a simple lock/unlock to avoid nested locks which can cause deadlocks
	m.mu.Lock()
	// Only log and set the state if we're currently connected
	if m.Connected {
		m.Connected = false
		m.DisconnectCalled = true
		fmt.Println("[MockTransport] Disconnect called")
	}
	m.mu.Unlock()
	return nil
}

// withResponse is a helper function that captures the response that was sent
func (m *MockTransport) withResponse(response []byte, err error) ([]byte, error) {
	if err == nil {
		m.mu.Lock()
		m.ResponseHistory = append(m.ResponseHistory, response)
		m.mu.Unlock()
	}
	return response, err
}

// Send implements the Transport interface
func (m *MockTransport) Send(message []byte) ([]byte, error) {
	// A simplified approach with minimal locking to avoid deadlocks

	// Store the original message for debugging
	m.LastSentMessage = append([]byte{}, message...)

	// Apply request interceptor if set
	if m.RequestInterceptor != nil {
		message = m.RequestInterceptor(message)
	}

	// Track request
	record := m.recordRequest(message)

	// Check for notification/initialized message - special handler
	// This handles a common case for test failures
	var req map[string]interface{}
	if err := json.Unmarshal(message, &req); err == nil {
		reqMethod, isMethod := req["method"].(string)
		if isMethod && reqMethod == "notifications/initialized" {
			// Auto-handle this message with an empty success response
			response := []byte(`{"jsonrpc":"2.0","result":null}`)

			// Store in response history
			m.mu.Lock()
			m.ResponseHistory = append(m.ResponseHistory, response)
			m.RequestHistory = append(m.RequestHistory, record)
			m.mu.Unlock()

			return response, nil
		}
	}

	// Basic connection check
	if !m.Connected {
		return nil, fmt.Errorf("connection closed")
	}

	// Apply simple network simulations
	if m.NetworkConditions.Latency > 0 {
		time.Sleep(m.NetworkConditions.Latency)
	}

	if m.NetworkConditions.PacketLossRate > 0 && rand.Float64() < m.NetworkConditions.PacketLossRate {
		return nil, fmt.Errorf("simulated packet loss")
	}

	// Find an appropriate response
	var response []byte
	var responseErr error

	// Access shared state with lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to request history
	m.RequestHistory = append(m.RequestHistory, record)

	// First try conditional responses
	for i, cfg := range m.ResponseQueue {
		if cfg.Condition == nil || cfg.Condition(message) {
			response = cfg.Data
			responseErr = cfg.Error

			// Remove this response from the queue
			if i < len(m.ResponseQueue) {
				m.ResponseQueue = append(m.ResponseQueue[:i], m.ResponseQueue[i+1:]...)
			}

			// Apply ID replacement for JSON-RPC responses
			if response != nil &&
				bytes.HasPrefix(bytes.TrimSpace(message), []byte(`{"jsonrpc":"2.0"`)) &&
				bytes.HasPrefix(bytes.TrimSpace(response), []byte(`{"jsonrpc":"2.0"`)) {
				// Parse request to extract ID
				var req map[string]interface{}
				if err := json.Unmarshal(message, &req); err == nil {
					if reqID, ok := req["id"]; ok {
						// Create ID replacement string
						idStr := fmt.Sprintf(`"id":%v`, reqID)
						// Replace ID in response
						response = bytes.ReplaceAll(response, []byte(`"id":0`), []byte(idStr))
					}
				}
			}

			// Add to response history
			m.ResponseHistory = append(m.ResponseHistory, response)

			return response, responseErr
		}
	}

	// If no conditional response matched and we have a default
	if m.DefaultResponse != nil {
		response = m.DefaultResponse.Data
		responseErr = m.DefaultResponse.Error

		// Apply ID replacement for JSON-RPC responses
		if response != nil &&
			bytes.HasPrefix(bytes.TrimSpace(message), []byte(`{"jsonrpc":"2.0"`)) &&
			bytes.HasPrefix(bytes.TrimSpace(response), []byte(`{"jsonrpc":"2.0"`)) {
			// Parse request to extract ID
			var req map[string]interface{}
			if err := json.Unmarshal(message, &req); err == nil {
				if reqID, ok := req["id"]; ok {
					// Create ID replacement string
					idStr := fmt.Sprintf(`"id":%v`, reqID)
					// Replace ID in response
					response = bytes.ReplaceAll(response, []byte(`"id":0`), []byte(idStr))
				}
			}
		}

		// Add to response history
		m.ResponseHistory = append(m.ResponseHistory, response)

		return response, responseErr
	}

	return nil, fmt.Errorf("no response available for request: %s", string(message))
}

// SendWithContext implements the Transport interface
func (m *MockTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Apply request interceptor if set
	if m.RequestInterceptor != nil {
		message = m.RequestInterceptor(message)
	}

	if ctx == nil {
		return m.Send(message)
	}

	// Make a buffered channel to avoid goroutine leaks
	respCh := make(chan struct {
		data []byte
		err  error
	}, 1)

	// Use a separate goroutine to handle the Send operation
	go func() {
		data, err := m.Send(message)
		select {
		case respCh <- struct {
			data []byte
			err  error
		}{data, err}:
		case <-ctx.Done():
			// Context was cancelled before we could send the response
			// Don't block trying to send on the channel
		}
	}()

	// Wait for either the response or context cancellation
	select {
	case resp := <-respCh:
		return resp.data, resp.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SetRequestTimeout implements the Transport interface
func (m *MockTransport) SetRequestTimeout(timeout time.Duration) {
	m.RequestTimeout = timeout
}

// SetConnectionTimeout implements the Transport interface
func (m *MockTransport) SetConnectionTimeout(timeout time.Duration) {
	m.ConnectionTimeout = timeout
}

// RegisterNotificationHandler implements the Transport interface
func (m *MockTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	m.NotificationHandlerFunc = handler
}

// recordRequest extracts and records information about a request
func (m *MockTransport) recordRequest(message []byte) RequestRecord {
	var request map[string]interface{}
	record := RequestRecord{
		Message:   append([]byte{}, message...), // Create a copy to avoid races
		Raw:       append([]byte{}, message...), // Create a copy to avoid races
		Timestamp: time.Now(),
	}

	// Try to extract ID and method
	if err := json.Unmarshal(message, &request); err == nil {
		record.ID = request["id"]
		if method, ok := request["method"].(string); ok {
			record.Method = method
		}
	}

	return record
}

// MockError is a mock error for testing
type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}

// CreateInitializeResponse creates a mock initialize response for the specified version
func CreateInitializeResponse(version string, capabilities map[string]interface{}) []byte {
	if capabilities == nil {
		capabilities = map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": true,
			},
		}
	}

	initializeResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": version,
			"serverInfo": map[string]interface{}{
				"name":    "Test Server",
				"version": "1.0.0",
			},
			"capabilities": capabilities,
		},
	}

	responseJSON, _ := json.Marshal(initializeResponse)
	return responseJSON
}

// SetupMockTransport sets up a mock transport with a successful initialize response for the specified version
func SetupMockTransport(version string) *MockTransport {
	m := NewMockTransport()

	// Set the transport to connected state to prevent "connection closed" errors
	m.mu.Lock()
	m.Connected = true
	m.mu.Unlock()

	// Configure the mock transport to support the version's protocol

	// Setup the initialize response
	initResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": version,
			"serverInfo": map[string]interface{}{
				"name":    "Test Server",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{},
		},
	}

	// Add version-specific capabilities and supported versions
	resultMap := initResponse["result"].(map[string]interface{})
	capabilities := resultMap["capabilities"].(map[string]interface{})

	// Include version support based on MCP specification
	var supportedVersions []string
	switch version {
	case "2025-03-26":
		// 2025-03-26 supports all previous versions
		supportedVersions = []string{"draft", "2024-11-05", "2025-03-26"}
		capabilities["enhancedResources"] = true
		capabilities["multipleRoots"] = true
	case "2024-11-05":
		// 2024-11-05 supports itself and draft
		supportedVersions = []string{"draft", "2024-11-05"}
		capabilities["rootsNotifications"] = true
	case "draft":
		// Draft only supports itself
		supportedVersions = []string{"draft"}
		capabilities["experimental"] = map[string]interface{}{
			"featureX": true,
		}
	}

	// Add the versions to the result
	resultMap["versions"] = supportedVersions

	// Marshal to JSON and queue as the response to the initialize request
	initJSON, _ := json.Marshal(initResponse)

	// Queue response with condition for initialize requests
	m.QueueConditionalResponse(initJSON, nil,
		func(msg []byte) bool {
			var req map[string]interface{}
			if err := json.Unmarshal(msg, &req); err != nil {
				return false
			}

			method, ok := req["method"].(string)
			return ok && method == "initialize"
		})

	// Create a proper version-specific resource response for basic resource/get requests
	var defaultResourceResponse []byte
	switch version {
	case "2025-03-26":
		// 2025-03-26 format uses 'contents' array
		resourceResp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      0, // Will be overridden by actual request ID
			"result": map[string]interface{}{
				"contents": []interface{}{
					map[string]interface{}{
						"uri":  "/",
						"text": "Root Resource",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Default resource content",
							},
						},
					},
				},
			},
		}
		defaultResourceResponse, _ = json.Marshal(resourceResp)
	case "2024-11-05", "draft":
		// 2024-11-05 and draft format uses 'content' array
		resourceResp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      0, // Will be overridden by actual request ID
			"result": map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Default resource content",
					},
				},
			},
		}
		defaultResourceResponse, _ = json.Marshal(resourceResp)
	}

	// Queue resource/get response
	m.QueueConditionalResponse(
		defaultResourceResponse,
		nil,
		IsRequestMethod("resource/get"),
	)

	// Add response for notifications/initialized
	m.QueueConditionalResponse(
		[]byte(`{"jsonrpc":"2.0","result":null}`),
		nil,
		IsRequestMethod("notifications/initialized"),
	)

	// Queue roots operations responses
	rootsListResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result": map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "/test/root",
					"name": "Test Root",
				},
			},
		},
	}
	rootsListJSON, _ := json.Marshal(rootsListResponse)
	m.QueueConditionalResponse(
		rootsListJSON,
		nil,
		IsRequestMethod("roots/list"),
	)

	// Queue roots/add response
	rootsAddResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result":  map[string]interface{}{},
	}
	rootsAddJSON, _ := json.Marshal(rootsAddResponse)
	m.QueueConditionalResponse(
		rootsAddJSON,
		nil,
		IsRequestMethod("roots/add"),
	)

	// Queue roots/remove response
	rootsRemoveResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result":  map[string]interface{}{},
	}
	rootsRemoveJSON, _ := json.Marshal(rootsRemoveResponse)
	m.QueueConditionalResponse(
		rootsRemoveJSON,
		nil,
		IsRequestMethod("roots/remove"),
	)

	// Add default response for tool/call or tool/execute
	defaultToolResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result": map[string]interface{}{
			"output": "Default tool response",
		},
	}
	toolJSON, _ := json.Marshal(defaultToolResponse)
	m.QueueConditionalResponse(
		toolJSON,
		nil,
		func(msg []byte) bool {
			return IsRequestMethod("tool/execute")(msg) || IsRequestMethod("tool/call")(msg)
		},
	)

	// Add default response for prompt/get
	defaultPromptResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result": map[string]interface{}{
			"prompt":   "{{variable}} default prompt",
			"rendered": "Default rendered prompt",
		},
	}
	promptJSON, _ := json.Marshal(defaultPromptResponse)
	m.QueueConditionalResponse(
		promptJSON,
		nil,
		IsRequestMethod("prompt/get"),
	)

	// Add default response for sampling/createMessage
	defaultSamplingResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result": map[string]interface{}{
			"role": "assistant",
			"content": map[string]interface{}{
				"type": "text",
				"text": "This is a default sampling response",
			},
			"model":      "test-model",
			"stopReason": "endTurn",
		},
	}
	samplingJSON, _ := json.Marshal(defaultSamplingResponse)
	m.QueueConditionalResponse(
		samplingJSON,
		nil,
		IsRequestMethod("sampling/createMessage"),
	)

	// Set a generic fallback response
	m.WithDefaultResponse(
		[]byte(`{"jsonrpc":"2.0","id":0,"result":null}`),
		nil,
	)

	return m
}

// IsRequestMethod checks if a given request uses the specified method
func IsRequestMethod(method string) func([]byte) bool {
	return func(message []byte) bool {
		var req map[string]interface{}
		if err := json.Unmarshal(message, &req); err != nil {
			return false
		}
		reqMethod, ok := req["method"].(string)
		return ok && reqMethod == method
	}
}

// RequestHasID checks if a given request has the specified ID
func RequestHasID(id interface{}) func([]byte) bool {
	return func(message []byte) bool {
		var req map[string]interface{}
		if err := json.Unmarshal(message, &req); err != nil {
			return false
		}
		reqID, ok := req["id"]
		if !ok {
			return false
		}

		// Handle different numeric types
		switch typedID := id.(type) {
		case int:
			floatID, ok := reqID.(float64)
			return ok && int(floatID) == typedID
		case float64:
			floatReqID, ok := reqID.(float64)
			return ok && floatReqID == typedID
		default:
			return fmt.Sprintf("%v", reqID) == fmt.Sprintf("%v", id)
		}
	}
}

// Network condition simulation methods

// SetLatency configures the latency simulation
func (m *MockTransport) SetLatency(latencyMs int, jitterPercent int) *MockTransport {
	m.mu.Lock()
	defer m.mu.Unlock()

	latency := time.Duration(latencyMs) * time.Millisecond
	m.NetworkConditions.Latency = latency

	if jitterPercent > 0 {
		jitter := (time.Duration(jitterPercent) * latency) / 100
		m.NetworkConditions.LatencyJitter = jitter
	}

	return m
}

// SetPacketLoss configures packet loss simulation
func (m *MockTransport) SetPacketLoss(lossPercent int) *MockTransport {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NetworkConditions.PacketLossRate = float64(lossPercent) / 100.0
	return m
}

// SimulateDisconnect forces the transport to disconnect after a certain number of requests
func (m *MockTransport) SimulateDisconnect() *MockTransport {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Connected = false
	return m
}

// SimulateDisconnectAfter configures the transport to disconnect after a specified number of requests
func (m *MockTransport) SimulateDisconnectAfter(requestCount int) *MockTransport {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NetworkConditions.DisconnectAfter = requestCount
	m.NetworkConditions.RequestCount = 0
	return m
}

// SetErrorRate configures the probability of returning errors on any request
func (m *MockTransport) SetErrorRate(errorPercent int) *MockTransport {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ErrorRate = float64(errorPercent) / 100.0
	return m
}

// GetRequestHistory returns a copy of the request history
func (m *MockTransport) GetRequestHistory() []RequestRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy to avoid race conditions
	history := make([]RequestRecord, len(m.RequestHistory))
	copy(history, m.RequestHistory)
	return history
}

// SimulateNotification simulates a notification from the server
func (m *MockTransport) SimulateNotification(method string, message []byte) {
	// Create a copy of the message to avoid race conditions
	messageCopy := append([]byte{}, message...)

	// Parse the message to extract method and params
	var req map[string]interface{}
	if err := json.Unmarshal(messageCopy, &req); err == nil {
		// If message has a method field, use that instead of the passed method
		if reqMethod, ok := req["method"].(string); ok {
			method = reqMethod
		}

		// If this is actually a request with an ID (not a notification)
		if id, hasID := req["id"]; hasID {
			// It's a request that needs a response, not a notification
			// This happens in sampling tests which use SimulateNotification to send requests
			params, hasParams := req["params"]

			// Create record
			record := RequestRecord{
				Message:   messageCopy,
				Raw:       messageCopy,
				Timestamp: time.Now(),
				ID:        id,
				Method:    method,
			}

			// Add to history
			m.mu.Lock()
			m.RequestHistory = append(m.RequestHistory, record)
			m.mu.Unlock()

			// Get notification handler without locking
			notificationHandler := m.NotificationHandlerFunc
			if notificationHandler != nil {
				// This is the key part: we need to execute the handler synchronously for tests
				// that expect a response to be sent
				if hasParams {
					if paramsJSON, ok := params.(json.RawMessage); ok {
						notificationHandler(method, paramsJSON)
					} else {
						paramsJSON, _ := json.Marshal(params)
						notificationHandler(method, paramsJSON)
					}
				} else {
					notificationHandler(method, nil)
				}
			}
			return
		}
	}

	// Regular notification handling
	record := RequestRecord{
		Message:   messageCopy,
		Raw:       messageCopy,
		Timestamp: time.Now(),
		Method:    method,
	}

	// Add to history
	m.mu.Lock()
	m.RequestHistory = append(m.RequestHistory, record)
	m.mu.Unlock()

	// Get notification handler
	notificationHandler := m.NotificationHandlerFunc
	if notificationHandler != nil {
		// Extract params if available
		var params []byte
		var req map[string]interface{}
		if err := json.Unmarshal(messageCopy, &req); err == nil {
			if p, ok := req["params"]; ok {
				if paramsJSON, ok := p.(json.RawMessage); ok {
					params = paramsJSON
				} else {
					params, _ = json.Marshal(p)
				}
			}
		}

		// Call notification handler synchronously for tests
		notificationHandler(method, params)
	}
}
