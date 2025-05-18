// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// SamplingCache provides caching for sampling operations to improve performance.
// It is safe for concurrent use by multiple goroutines.
type SamplingCache struct {
	configCache      map[string]*SamplingConfig  // Cache of commonly used configurations by key
	responseCache    map[string]SamplingResponse // Cache of sampling responses by request hash
	responseTTL      time.Duration               // Time-to-live for response cache entries
	maxEntries       int                         // Maximum number of cache entries
	mu               sync.RWMutex                // Mutex for thread safety
	lastCleanup      time.Time                   // Last time the cache was cleaned up
	cleanupInterval  time.Duration               // How often to clean up expired entries
	entryTimestamps  map[string]time.Time        // When each entry was cached
	configReuseCount map[string]int              // Counter for how often each config is reused
}

// NewSamplingCache creates a new sampling cache with the specified capacity and TTL.
func NewSamplingCache(capacity int, ttl time.Duration) *SamplingCache {
	if capacity <= 0 {
		capacity = 100 // Default capacity
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute // Default TTL
	}

	return &SamplingCache{
		configCache:      make(map[string]*SamplingConfig, capacity),
		responseCache:    make(map[string]SamplingResponse, capacity),
		responseTTL:      ttl,
		maxEntries:       capacity,
		lastCleanup:      time.Now(),
		cleanupInterval:  time.Minute,
		entryTimestamps:  make(map[string]time.Time, capacity),
		configReuseCount: make(map[string]int, capacity),
	}
}

// GetConfig retrieves a cached configuration by key, if it exists.
func (c *SamplingCache) GetConfig(key string) (*SamplingConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, found := c.configCache[key]
	if found {
		c.configReuseCount[key]++
	}
	return config, found
}

// SetConfig caches a configuration with the specified key.
func (c *SamplingCache) SetConfig(key string, config *SamplingConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up if we're at capacity
	if len(c.configCache) >= c.maxEntries {
		c.evictLeastUsedConfig()
	}

	c.configCache[key] = config
	c.configReuseCount[key] = 1
}

// GetResponse retrieves a cached response for a request, if it exists and hasn't expired.
func (c *SamplingCache) GetResponse(params SamplingCreateMessageParams, version string) (SamplingResponse, bool) {
	key := c.computeRequestHash(params, version)

	c.mu.RLock()
	defer c.mu.RUnlock()

	resp, found := c.responseCache[key]
	if !found {
		return SamplingResponse{}, false
	}

	// Check if the entry has expired
	timestamp, ok := c.entryTimestamps[key]
	if !ok || time.Since(timestamp) > c.responseTTL {
		return SamplingResponse{}, false
	}

	return resp, true
}

// SetResponse caches a response for a request.
func (c *SamplingCache) SetResponse(params SamplingCreateMessageParams, version string, response SamplingResponse) {
	key := c.computeRequestHash(params, version)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up if necessary
	if time.Since(c.lastCleanup) > c.cleanupInterval {
		c.cleanup()
	}

	// Evict if at capacity
	if len(c.responseCache) >= c.maxEntries {
		c.evictOldestResponse()
	}

	c.responseCache[key] = response
	c.entryTimestamps[key] = time.Now()
}

// Clear empties the cache.
func (c *SamplingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.configCache = make(map[string]*SamplingConfig, c.maxEntries)
	c.responseCache = make(map[string]SamplingResponse, c.maxEntries)
	c.entryTimestamps = make(map[string]time.Time, c.maxEntries)
	c.configReuseCount = make(map[string]int, c.maxEntries)
	c.lastCleanup = time.Now()
}

// cleanup removes expired entries from the response cache.
func (c *SamplingCache) cleanup() {
	now := time.Now()
	for key, timestamp := range c.entryTimestamps {
		if now.Sub(timestamp) > c.responseTTL {
			delete(c.responseCache, key)
			delete(c.entryTimestamps, key)
		}
	}
	c.lastCleanup = now
}

// evictOldestResponse removes the oldest response from the cache.
func (c *SamplingCache) evictOldestResponse() {
	var oldestKey string
	var oldestTime time.Time

	// Initialize with the first entry
	for key, timestamp := range c.entryTimestamps {
		oldestKey = key
		oldestTime = timestamp
		break
	}

	// Find the oldest entry
	for key, timestamp := range c.entryTimestamps {
		if timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = timestamp
		}
	}

	// Remove the oldest entry
	if oldestKey != "" {
		delete(c.responseCache, oldestKey)
		delete(c.entryTimestamps, oldestKey)
	}
}

// evictLeastUsedConfig removes the least used configuration from the cache.
func (c *SamplingCache) evictLeastUsedConfig() {
	var leastUsedKey string
	var leastUsedCount int = -1

	// Find the least used configuration
	for key, count := range c.configReuseCount {
		if leastUsedCount == -1 || count < leastUsedCount {
			leastUsedKey = key
			leastUsedCount = count
		}
	}

	// Remove the least used configuration
	if leastUsedKey != "" {
		delete(c.configCache, leastUsedKey)
		delete(c.configReuseCount, leastUsedKey)
	}
}

// computeRequestHash generates a deterministic hash for a sampling request.
func (c *SamplingCache) computeRequestHash(params SamplingCreateMessageParams, version string) string {
	// Create a string representation of the request
	requestStr := fmt.Sprintf("%s|%d|%s|",
		version,
		params.MaxTokens,
		params.SystemPrompt)

	// Add all messages
	for _, msg := range params.Messages {
		requestStr += fmt.Sprintf("%s|%s|%s|", msg.Role, msg.Content.Type, msg.Content.Text)
		// For non-text content, add a truncated hash of the data
		if msg.Content.Data != "" {
			data, _ := base64.StdEncoding.DecodeString(msg.Content.Data)
			if len(data) > 256 {
				// Use first 256 bytes for large data
				data = data[:256]
			}
			hash := sha256.Sum256(data)
			requestStr += hex.EncodeToString(hash[:]) + "|"
		}
	}

	// Compute the hash
	hash := sha256.Sum256([]byte(requestStr))
	return hex.EncodeToString(hash[:])
}

// ImageOptimizer provides utilities for optimizing image data for sampling requests.
type ImageOptimizer struct {
	maxDimension    int    // Maximum image dimension
	quality         int    // JPEG quality (1-100)
	defaultMimetype string // Default mimetype for optimized images
}

// NewImageOptimizer creates a new image optimizer with the specified settings.
func NewImageOptimizer(maxDimension int, quality int) *ImageOptimizer {
	if maxDimension <= 0 {
		maxDimension = 1024 // Default max dimension
	}
	if quality <= 0 || quality > 100 {
		quality = 85 // Default quality
	}

	return &ImageOptimizer{
		maxDimension:    maxDimension,
		quality:         quality,
		defaultMimetype: "image/jpeg",
	}
}

// OptimizeImageData optimizes image data for inclusion in sampling messages.
// This is a placeholder that would typically use an image processing library
// to resize and recompress the image.
func (o *ImageOptimizer) OptimizeImageData(data string, mimeType string) (string, string, error) {
	// In a real implementation, this would:
	// 1. Decode the base64 data
	// 2. Parse the image
	// 3. Resize if needed
	// 4. Recompress with appropriate quality
	// 5. Re-encode to base64

	// This is just a placeholder
	return data, mimeType, nil
}

// ContentSizeAnalyzer analyzes and manages content size in sampling operations.
type ContentSizeAnalyzer struct {
	// Constants for size limitations
	MaxTextBytes     int
	MaxImageBytes    int
	MaxAudioBytes    int
	WarningThreshold float64 // 0.0-1.0 percentage of max before warning
}

// NewContentSizeAnalyzer creates a new analyzer with default limits.
func NewContentSizeAnalyzer() *ContentSizeAnalyzer {
	return &ContentSizeAnalyzer{
		MaxTextBytes:     32 * 1024,       // 32KB
		MaxImageBytes:    1 * 1024 * 1024, // 1MB
		MaxAudioBytes:    5 * 1024 * 1024, // 5MB
		WarningThreshold: 0.8,             // 80%
	}
}

// AnalyzeContent checks content size against limits and returns warnings/errors.
func (a *ContentSizeAnalyzer) AnalyzeContent(content SamplingMessageContent) (bool, string) {
	var sizeBytes int
	var maxBytes int

	// Determine size based on content type
	switch content.Type {
	case "text":
		sizeBytes = len(content.Text)
		maxBytes = a.MaxTextBytes
	case "image", "audio":
		sizeBytes = len(content.Data)
		if content.Type == "image" {
			maxBytes = a.MaxImageBytes
		} else {
			maxBytes = a.MaxAudioBytes
		}
	default:
		return false, fmt.Sprintf("unknown content type: %s", content.Type)
	}

	// Check if exceeds max
	if sizeBytes > maxBytes {
		return false, fmt.Sprintf("%s content exceeds maximum size (%d > %d bytes)",
			content.Type, sizeBytes, maxBytes)
	}

	// Check if approaching warning threshold
	ratio := float64(sizeBytes) / float64(maxBytes)
	if ratio >= a.WarningThreshold {
		return true, fmt.Sprintf("%s content approaching maximum size (%.1f%% of limit)",
			content.Type, ratio*100)
	}

	return true, ""
}

// SamplingPerformanceMetrics tracks performance metrics for sampling operations.
type SamplingPerformanceMetrics struct {
	RequestCount      int64         // Total requests sent
	SuccessCount      int64         // Successful requests
	ErrorCount        int64         // Failed requests
	TotalResponseTime time.Duration // Total time spent on responses
	MaxResponseTime   time.Duration // Maximum response time
	MinResponseTime   time.Duration // Minimum response time
	CacheHits         int64         // Number of cache hits
	CacheMisses       int64         // Number of cache misses
	BytesSent         int64         // Total bytes sent
	BytesReceived     int64         // Total bytes received
	mu                sync.Mutex    // Mutex for thread safety
}

// NewSamplingPerformanceMetrics creates a new performance metrics tracker.
func NewSamplingPerformanceMetrics() *SamplingPerformanceMetrics {
	return &SamplingPerformanceMetrics{
		MinResponseTime: time.Hour, // Start with a high value
	}
}

// RecordRequest records metrics for a sampling request.
func (m *SamplingPerformanceMetrics) RecordRequest(
	success bool,
	responseTime time.Duration,
	cacheHit bool,
	bytesSent int,
	bytesReceived int) {

	m.mu.Lock()
	defer m.mu.Unlock()

	m.RequestCount++
	if success {
		m.SuccessCount++
	} else {
		m.ErrorCount++
	}

	m.TotalResponseTime += responseTime
	if responseTime > m.MaxResponseTime {
		m.MaxResponseTime = responseTime
	}
	if responseTime < m.MinResponseTime {
		m.MinResponseTime = responseTime
	}

	if cacheHit {
		m.CacheHits++
	} else {
		m.CacheMisses++
	}

	m.BytesSent += int64(bytesSent)
	m.BytesReceived += int64(bytesReceived)
}

// GetAverageResponseTime returns the average response time.
func (m *SamplingPerformanceMetrics) GetAverageResponseTime() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RequestCount == 0 {
		return 0
	}
	return time.Duration(int64(m.TotalResponseTime) / m.RequestCount)
}

// GetMetrics returns a map of all metrics.
func (m *SamplingPerformanceMetrics) GetMetrics() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := map[string]interface{}{
		"requestCount":       m.RequestCount,
		"successCount":       m.SuccessCount,
		"errorCount":         m.ErrorCount,
		"successRate":        float64(0),
		"averageResponseMs":  int64(0),
		"maxResponseMs":      m.MaxResponseTime.Milliseconds(),
		"minResponseMs":      m.MinResponseTime.Milliseconds(),
		"cacheHits":          m.CacheHits,
		"cacheMisses":        m.CacheMisses,
		"cacheHitRate":       float64(0),
		"totalBytesSent":     m.BytesSent,
		"totalBytesReceived": m.BytesReceived,
	}

	// Calculate derived metrics
	if m.RequestCount > 0 {
		metrics["successRate"] = float64(m.SuccessCount) / float64(m.RequestCount)
		metrics["averageResponseMs"] = int64(m.TotalResponseTime) / m.RequestCount / int64(time.Millisecond)
	}

	totalCacheAttempts := m.CacheHits + m.CacheMisses
	if totalCacheAttempts > 0 {
		metrics["cacheHitRate"] = float64(m.CacheHits) / float64(totalCacheAttempts)
	}

	return metrics
}

// Reset clears all metrics.
func (m *SamplingPerformanceMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RequestCount = 0
	m.SuccessCount = 0
	m.ErrorCount = 0
	m.TotalResponseTime = 0
	m.MaxResponseTime = 0
	m.MinResponseTime = time.Hour
	m.CacheHits = 0
	m.CacheMisses = 0
	m.BytesSent = 0
	m.BytesReceived = 0
}

// SamplingOptimizationOptions holds configuration for sampling optimizations
type SamplingOptimizationOptions struct {
	CacheCapacity int
	CacheTTL      time.Duration
	LogWarnings   bool
	MaxTextBytes  int
	MaxImageBytes int
	MaxAudioBytes int
	ImageMaxDim   int
	ImageQuality  int
}

// DefaultSamplingOptimizationOptions returns default optimization options
func DefaultSamplingOptimizationOptions() *SamplingOptimizationOptions {
	return &SamplingOptimizationOptions{
		CacheCapacity: 200,
		CacheTTL:      10 * time.Minute,
		LogWarnings:   true,
		MaxTextBytes:  32 * 1024,       // 32KB
		MaxImageBytes: 1 * 1024 * 1024, // 1MB
		MaxAudioBytes: 5 * 1024 * 1024, // 5MB
		ImageMaxDim:   1024,
		ImageQuality:  85,
	}
}

// SetupOptimizedSamplingClient creates a client with sampling optimizations.
// This follows a separate constructor pattern rather than a functional option.
func SetupOptimizedSamplingClient(baseClient Client, opts *SamplingOptimizationOptions) Client {
	if opts == nil {
		opts = DefaultSamplingOptimizationOptions()
	}

	// Must assert to get access to the internal fields
	c, ok := baseClient.(*clientImpl)
	if !ok {
		// If not a clientImpl, just return the original client
		return baseClient
	}

	// Create optimization components
	cache := NewSamplingCache(opts.CacheCapacity, opts.CacheTTL)

	// Configure size analyzer
	sizeAnalyzer := NewContentSizeAnalyzer()
	if opts.MaxTextBytes > 0 {
		sizeAnalyzer.MaxTextBytes = opts.MaxTextBytes
	}
	if opts.MaxImageBytes > 0 {
		sizeAnalyzer.MaxImageBytes = opts.MaxImageBytes
	}
	if opts.MaxAudioBytes > 0 {
		sizeAnalyzer.MaxAudioBytes = opts.MaxAudioBytes
	}

	// Create metrics tracker
	metrics := NewSamplingPerformanceMetrics()

	// Store these components for access in other methods
	c.samplingCache = cache
	c.sizeAnalyzer = sizeAnalyzer
	c.samplingMetrics = metrics

	// Register an optimized sampling handler wrapper if there's already a base handler
	if c.samplingHandler != nil {
		baseHandler := c.samplingHandler
		c.samplingHandler = wrapSamplingHandlerWithOptimizations(
			baseHandler, cache, sizeAnalyzer, metrics, opts.LogWarnings, c.Version())
	}

	return baseClient
}

// wrapSamplingHandlerWithOptimizations creates an optimized wrapper around a base sampling handler
func wrapSamplingHandlerWithOptimizations(
	baseHandler SamplingHandler,
	cache *SamplingCache,
	sizeAnalyzer *ContentSizeAnalyzer,
	metrics *SamplingPerformanceMetrics,
	logWarnings bool,
	protocolVersion string) SamplingHandler {

	return func(params SamplingCreateMessageParams) (SamplingResponse, error) {
		// Start timing
		startTime := time.Now()
		bytesSent := estimateRequestSize(params)

		// Check cache first
		cachedResp, found := cache.GetResponse(params, protocolVersion)
		if found {
			responseTime := time.Since(startTime)
			metrics.RecordRequest(true, responseTime, true, bytesSent, estimateResponseSize(cachedResp))
			return cachedResp, nil
		}

		// Analyze content size
		for i, msg := range params.Messages {
			ok, warning := sizeAnalyzer.AnalyzeContent(msg.Content)
			if !ok {
				metrics.RecordRequest(false, time.Since(startTime), false, bytesSent, 0)
				return SamplingResponse{}, fmt.Errorf("content size validation failed for message %d: %s", i, warning)
			}
			if logWarnings && warning != "" {
				// Would log warning in a real implementation
				_ = warning
			}
		}

		// Call the base handler
		response, err := baseHandler(params)

		// Calculate metrics
		responseTime := time.Since(startTime)
		success := err == nil
		bytesReceived := 0
		if success {
			bytesReceived = estimateResponseSize(response)
			// Cache successful responses
			cache.SetResponse(params, protocolVersion, response)
		}
		metrics.RecordRequest(success, responseTime, false, bytesSent, bytesReceived)

		return response, err
	}
}

// estimateRequestSize estimates the size of a sampling request in bytes.
func estimateRequestSize(params SamplingCreateMessageParams) int {
	size := len(params.SystemPrompt)

	for _, msg := range params.Messages {
		size += len(msg.Role)
		size += len(msg.Content.Type)
		size += len(msg.Content.Text)
		size += len(msg.Content.Data)
		size += len(msg.Content.MimeType)
	}

	return size
}

// estimateResponseSize estimates the size of a sampling response in bytes.
func estimateResponseSize(response SamplingResponse) int {
	size := len(response.Role)
	size += len(response.Model)
	size += len(response.StopReason)
	size += len(response.Content.Type)
	size += len(response.Content.Text)
	size += len(response.Content.Data)
	size += len(response.Content.MimeType)

	return size
}
