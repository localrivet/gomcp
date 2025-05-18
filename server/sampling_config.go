package server

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SamplingConfig defines server-side configuration options for sampling operations.
// This comprehensive configuration structure controls all aspects of sampling behavior,
// including rate limiting, timeouts, retry policies, prioritization, and protocol-specific settings.
type SamplingConfig struct {
	// Rate limiting settings
	MaxRequestsPerMinute  int  // Maximum number of sampling requests per minute
	MaxConcurrentRequests int  // Maximum number of concurrent sampling requests
	MaxTokensPerRequest   int  // Maximum tokens allowed per request
	PerClientRateLimit    bool // Whether to apply rate limits per client

	// Timeout settings
	DefaultTimeout time.Duration // Default timeout for sampling requests
	MaxTimeout     time.Duration // Maximum allowed timeout for sampling requests

	// Retry settings
	DefaultMaxRetries    int           // Default number of retries for failed requests
	DefaultRetryInterval time.Duration // Default interval between retries

	// Priority settings
	EnablePrioritization bool // Whether to enable request prioritization
	DefaultPriority      int  // Default priority for sampling requests (1-10, 10 being highest)

	// Resource allocation
	ResourceQuota map[string]int // Resource quotas for different content types

	// Error handling
	GracefulDegradation bool // Whether to enable graceful degradation on errors

	// Protocol-specific settings
	ProtocolDefaults map[string]*ProtocolSamplingConfig // Protocol-specific settings
}

// ProtocolSamplingConfig defines protocol-specific sampling configuration.
// Different protocol versions have different capabilities and constraints,
// and this structure captures those version-specific settings.
type ProtocolSamplingConfig struct {
	MaxTokens             int
	SupportedContentTypes map[string]bool
	StreamingSupported    bool
}

// NewDefaultSamplingConfig creates a new sampling configuration with sensible defaults.
// This function provides a pre-configured SamplingConfig with reasonable values
// for all settings, suitable for most server deployments without customization.
//
// Returns:
//   - A pointer to a new SamplingConfig struct with default values
func NewDefaultSamplingConfig() *SamplingConfig {
	return &SamplingConfig{
		MaxRequestsPerMinute:  120,  // 2 requests per second on average
		MaxConcurrentRequests: 10,   // Max 10 concurrent requests
		MaxTokensPerRequest:   8192, // Max tokens for the latest protocol
		PerClientRateLimit:    true, // Apply rate limits per client

		DefaultTimeout: 30 * time.Second,
		MaxTimeout:     120 * time.Second,

		DefaultMaxRetries:    2,
		DefaultRetryInterval: 1 * time.Second,

		EnablePrioritization: true,
		DefaultPriority:      5,

		ResourceQuota: map[string]int{
			"text":  8192,
			"image": 4096,
			"audio": 4096,
		},

		GracefulDegradation: true,

		ProtocolDefaults: map[string]*ProtocolSamplingConfig{
			"draft": {
				MaxTokens: 2048,
				SupportedContentTypes: map[string]bool{
					"text":  true,
					"image": true,
					"audio": true,
				},
				StreamingSupported: false,
			},
			"2024-11-05": {
				MaxTokens: 4096,
				SupportedContentTypes: map[string]bool{
					"text":  true,
					"image": true,
				},
				StreamingSupported: false,
			},
			"2025-03-26": {
				MaxTokens: 8192,
				SupportedContentTypes: map[string]bool{
					"text":  true,
					"image": true,
					"audio": true,
				},
				StreamingSupported: true,
			},
		},
	}
}

// SamplingController manages sampling operations with rate limiting and prioritization.
// This component enforces rate limits, tracks request statistics, manages prioritization,
// and validates requests against protocol-specific constraints to ensure reliable
// and fair resource allocation for sampling operations.
type SamplingController struct {
	config          *SamplingConfig
	requestCount    map[string]int     // Requests per client in current minute
	concurrentCount int                // Current concurrent requests
	requestQueue    []*samplingRequest // Prioritized request queue
	rateLimiterTick *time.Ticker       // Ticker for rate limiting resets
	mu              sync.RWMutex
	logger          *slog.Logger // Logger instance
}

// samplingRequest represents a queued sampling request with priority information.
// This internal structure is used by the SamplingController to track individual
// requests in the prioritization queue, including metadata needed for scheduling.
type samplingRequest struct {
	sessionID    SessionID
	priority     int
	contentTypes []string
	tokenCount   int
	timestamp    time.Time
}

// NewSamplingController creates a new controller with the specified configuration.
// This function initializes all components of the sampling controller, including
// the rate limiter and request tracking system, and starts the background goroutine
// for periodically resetting rate limits.
//
// Parameters:
//   - config: The sampling configuration to use (or nil for default configuration)
//   - logger: A structured logger for recording controller events
//
// Returns:
//   - A fully initialized SamplingController ready for managing sampling requests
func NewSamplingController(config *SamplingConfig, logger *slog.Logger) *SamplingController {
	if config == nil {
		config = NewDefaultSamplingConfig()
	}

	controller := &SamplingController{
		config:       config,
		requestCount: make(map[string]int),
		logger:       logger,
	}

	// Start the rate limiter ticker
	controller.rateLimiterTick = time.NewTicker(time.Minute)
	go controller.resetRateLimits()

	return controller
}

// resetRateLimits resets rate limits every minute.
// This method runs as a background goroutine and periodically clears the
// per-client request count map to enforce per-minute rate limits.
// It continues running until the controller is stopped.
func (sc *SamplingController) resetRateLimits() {
	for range sc.rateLimiterTick.C {
		sc.mu.Lock()
		sc.requestCount = make(map[string]int)
		sc.mu.Unlock()

		sc.logger.Debug("sampling rate limits reset")
	}
}

// CanProcessRequest checks if a sampling request can be processed based on rate limits.
// This method evaluates both global concurrent request limits and per-client
// rate limits (if enabled) to determine if a new request should be accepted or rejected.
//
// Parameters:
//   - sessionID: The client session ID for per-client rate limiting
//
// Returns:
//   - true if the request can be processed, false if it would exceed rate limits
func (sc *SamplingController) CanProcessRequest(sessionID SessionID) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Check global concurrent limit
	if sc.concurrentCount >= sc.config.MaxConcurrentRequests {
		return false
	}

	// Check per-client rate limit if enabled
	if sc.config.PerClientRateLimit {
		sessionKey := string(sessionID)
		if sessionKey == "" {
			sessionKey = "default"
		}

		if sc.requestCount[sessionKey] >= sc.config.MaxRequestsPerMinute {
			return false
		}
	}

	return true
}

// RecordRequest records a sampling request for rate limiting purposes.
// This method updates the concurrent request counter and per-client request
// counter (if enabled) when a new sampling request begins processing.
//
// Parameters:
//   - sessionID: The client session ID for per-client rate tracking
func (sc *SamplingController) RecordRequest(sessionID SessionID) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Increment concurrent count
	sc.concurrentCount++

	// Record per-client request
	if sc.config.PerClientRateLimit {
		sessionKey := string(sessionID)
		if sessionKey == "" {
			sessionKey = "default"
		}

		sc.requestCount[sessionKey]++
	}
}

// CompleteRequest marks a request as completed, updating rate limiting counters.
// This method decrements the concurrent request counter when a sampling request
// finishes processing, regardless of whether it succeeded or failed.
//
// Parameters:
//   - sessionID: The client session ID for the completed request
func (sc *SamplingController) CompleteRequest(sessionID SessionID) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Decrement concurrent count
	if sc.concurrentCount > 0 {
		sc.concurrentCount--
	}
}

// GetRequestOptions returns appropriate request options based on configuration.
// This method calculates timeout and retry parameters for a sampling request
// based on its priority level, applying adjustments according to the controller's
// configuration and prioritization settings.
//
// Parameters:
//   - priority: The priority level of the request (1-10, with 10 being highest)
//
// Returns:
//   - RequestSamplingOptions containing configured timeout and retry settings
func (sc *SamplingController) GetRequestOptions(priority int) RequestSamplingOptions {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	timeout := sc.config.DefaultTimeout
	maxRetries := sc.config.DefaultMaxRetries
	retryInterval := sc.config.DefaultRetryInterval

	// Adjust timeout based on priority if prioritization is enabled
	if sc.config.EnablePrioritization {
		// Higher priority = more generous timeout
		priorityFactor := float64(priority) / 10.0
		adjustedTimeout := time.Duration(float64(timeout) * (0.5 + priorityFactor))

		// Cap at maximum timeout
		if adjustedTimeout > sc.config.MaxTimeout {
			adjustedTimeout = sc.config.MaxTimeout
		}

		timeout = adjustedTimeout
	}

	return RequestSamplingOptions{
		Timeout:       timeout,
		MaxRetries:    maxRetries,
		RetryInterval: retryInterval,
	}
}

// Stop stops the sampling controller and cleans up resources.
// This method should be called when shutting down the server to ensure
// proper cleanup of background goroutines and other resources.
func (sc *SamplingController) Stop() {
	if sc.rateLimiterTick != nil {
		sc.rateLimiterTick.Stop()
	}
}

// ValidateForProtocol validates sampling parameters against protocol constraints.
// This method checks that a sampling request conforms to the limitations of
// the specified protocol version, including token count limits and supported content types.
//
// Parameters:
//   - protocol: The protocol version to validate against
//   - messages: The conversation messages to validate
//   - maxTokens: The requested maximum token count
//
// Returns:
//   - nil if valid, or an error describing the validation failure
func (sc *SamplingController) ValidateForProtocol(protocol string, messages []SamplingMessage, maxTokens int) error {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	protocolConfig, exists := sc.config.ProtocolDefaults[protocol]
	if !exists {
		return fmt.Errorf("unsupported protocol version: %s", protocol)
	}

	// Validate token count
	if maxTokens > protocolConfig.MaxTokens {
		return fmt.Errorf("maxTokens exceeds maximum value (%d) for protocol version %s",
			protocolConfig.MaxTokens, protocol)
	}

	// Validate content types
	for i, msg := range messages {
		if !protocolConfig.SupportedContentTypes[msg.Content.Type] {
			return fmt.Errorf("message %d content type '%s' not supported in protocol version '%s'",
				i, msg.Content.Type, protocol)
		}
	}

	return nil
}

// detectSamplingCapabilities identifies client capabilities based on protocol version.
// This is a private version to avoid conflicting with the existing DetectClientCapabilities
// in sampling.go. It determines which content types (text, image, audio) are supported
// in each protocol version.
//
// Parameters:
//   - version: The protocol version to analyze
//
// Returns:
//   - A SamplingCapabilities struct describing the features supported in this version
func detectSamplingCapabilities(version string) SamplingCapabilities {
	switch version {
	case "draft":
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: true,
		}
	case "2024-11-05":
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: false, // Not supported in this version
		}
	case "2025-03-26":
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: true,
		}
	default:
		// Unknown version, assume minimal capabilities
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: false,
			AudioSupport: false,
		}
	}
}

// WithSamplingConfig returns a server option that sets the sampling configuration.
// This function generates a server configuration option that can be passed
// to NewServer or ServerBuilder to customize sampling behavior.
//
// Parameters:
//   - config: The sampling configuration to apply to the server
//
// Returns:
//   - An Option function that configures sampling when applied to a server
func WithSamplingConfig(config *SamplingConfig) Option {
	return func(s *serverImpl) {
		// Update server with the sampling configuration
		if s.samplingConfig == nil {
			s.samplingConfig = config
			s.samplingController = NewSamplingController(config, s.logger)
		}
	}
}

// GetConcurrentRequestCount returns the current number of concurrent requests.
// This method is useful for monitoring and debugging the server's sampling workload.
//
// Returns:
//   - The current count of active sampling requests being processed
func (sc *SamplingController) GetConcurrentRequestCount() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.concurrentCount
}
