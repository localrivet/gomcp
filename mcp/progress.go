// Package mcp provides shared types for the MCP protocol implementation.
package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ProgressNotificationParams represents the parameters for a progress notification
// following the MCP JSON-RPC 2.0 specification pattern
type ProgressNotificationParams struct {
	// ProgressToken is a unique identifier for this progress tracking session
	ProgressToken string `json:"progressToken"`

	// Progress is the current progress value (required)
	Progress float64 `json:"progress"`

	// Total is the total expected value (optional, for percentage calculations)
	Total *float64 `json:"total,omitempty"`

	// Message is an optional human-readable progress message
	// Note: Only supported in draft and 2025-03-26 versions, not in 2024-11-05
	Message string `json:"message,omitempty"`
}

// ProgressNotification represents a complete progress notification message
// This includes the JSON-RPC 2.0 envelope and the progress-specific parameters
type ProgressNotification struct {
	// JSONRPC version (always "2.0")
	JSONRPC string `json:"jsonrpc"`

	// Method is always "notifications/progress"
	Method string `json:"method"`

	// Params contains the progress notification parameters
	Params ProgressNotificationParams `json:"params"`

	// ID is optional for notifications but can be included for tracking
	ID interface{} `json:"id,omitempty"`

	// protocolVersion tracks which MCP version this notification is for
	// This is not serialized but used internally for validation
	protocolVersion string `json:"-"`
}

// NewProgressNotification creates a new progress notification message
// The protocolVersion parameter determines which fields are included (draft, 2024-11-05, 2025-03-26)
func NewProgressNotification(progressToken string, progress float64, total *float64, message string) *ProgressNotification {
	return NewProgressNotificationForVersion(progressToken, progress, total, message, "draft")
}

// NewProgressNotificationForVersion creates a new progress notification for a specific protocol version
func NewProgressNotificationForVersion(progressToken string, progress float64, total *float64, message string, protocolVersion string) *ProgressNotification {
	params := ProgressNotificationParams{
		ProgressToken: progressToken,
		Progress:      progress,
		Total:         total,
	}

	// Only include message field for versions that support it
	if protocolVersion == "draft" || protocolVersion == "2025-03-26" || protocolVersion == "2025-06-18" {
		params.Message = message
	}

	return &ProgressNotification{
		JSONRPC:         "2.0",
		Method:          "notifications/progress",
		Params:          params,
		protocolVersion: protocolVersion,
	}
}

// ToJSON serializes the progress notification to JSON bytes
// This respects the protocol version and excludes unsupported fields
func (pn *ProgressNotification) ToJSON() ([]byte, error) {
	// Create a copy for serialization to avoid modifying the original
	notification := *pn

	// For 2024-11-05, ensure message field is not included
	if pn.protocolVersion == "2024-11-05" {
		notification.Params.Message = ""
	}

	return json.Marshal(notification)
}

// FromJSON deserializes JSON bytes into a progress notification
func (pn *ProgressNotification) FromJSON(data []byte) error {
	return json.Unmarshal(data, pn)
}

// SetProtocolVersion sets the protocol version for this notification
func (pn *ProgressNotification) SetProtocolVersion(version string) {
	pn.protocolVersion = version
}

// GetProtocolVersion returns the protocol version for this notification
func (pn *ProgressNotification) GetProtocolVersion() string {
	return pn.protocolVersion
}

// Validate checks if the progress notification is valid according to MCP specifications
func (pn *ProgressNotification) Validate() error {
	if pn.JSONRPC != "2.0" {
		return fmt.Errorf("invalid JSON-RPC version: %s", pn.JSONRPC)
	}

	if pn.Method != "notifications/progress" {
		return fmt.Errorf("invalid method for progress notification: %s", pn.Method)
	}

	if pn.Params.ProgressToken == "" {
		return fmt.Errorf("progress token is required")
	}

	if pn.Params.Progress < 0 {
		return fmt.Errorf("progress value cannot be negative: %f", pn.Params.Progress)
	}

	if pn.Params.Total != nil && *pn.Params.Total < 0 {
		return fmt.Errorf("total value cannot be negative: %f", *pn.Params.Total)
	}

	if pn.Params.Total != nil && pn.Params.Progress > *pn.Params.Total {
		return fmt.Errorf("progress value (%f) cannot exceed total value (%f)", pn.Params.Progress, *pn.Params.Total)
	}

	// Validate message field based on protocol version
	if pn.protocolVersion == "2024-11-05" && pn.Params.Message != "" {
		return fmt.Errorf("message field is not supported in protocol version 2024-11-05")
	}

	return nil
}

// ValidateProgressIncrease validates that progress values increase as required by MCP specs
func (pn *ProgressNotification) ValidateProgressIncrease(previousProgress float64) error {
	if pn.Params.Progress < previousProgress {
		return fmt.Errorf("progress value (%f) must not decrease from previous value (%f)", pn.Params.Progress, previousProgress)
	}
	return nil
}

// GetPercentage calculates the percentage completion if total is available
func (pn *ProgressNotification) GetPercentage() *float64 {
	if pn.Params.Total == nil || *pn.Params.Total == 0 {
		return nil
	}

	percentage := (pn.Params.Progress / *pn.Params.Total) * 100
	return &percentage
}

// IsComplete returns true if the progress indicates completion
func (pn *ProgressNotification) IsComplete() bool {
	if pn.Params.Total != nil {
		return pn.Params.Progress >= *pn.Params.Total
	}
	// If no total is specified, consider 100% as complete
	return pn.Params.Progress >= 100.0
}

// ProgressListener represents a function that handles progress notifications
type ProgressListener func(*ProgressNotification) error

// ProgressChannel represents a bidirectional communication channel for progress notifications
type ProgressChannel struct {
	mu        sync.RWMutex
	listeners map[string][]ProgressListener // Maps progress tokens to listeners
	active    bool
}

// NewProgressChannel creates a new progress communication channel
func NewProgressChannel() *ProgressChannel {
	return &ProgressChannel{
		listeners: make(map[string][]ProgressListener),
		active:    true,
	}
}

// Subscribe adds a listener for progress notifications with a specific token
func (pc *ProgressChannel) Subscribe(progressToken string, listener ProgressListener) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.active {
		return
	}

	if pc.listeners[progressToken] == nil {
		pc.listeners[progressToken] = make([]ProgressListener, 0)
	}

	pc.listeners[progressToken] = append(pc.listeners[progressToken], listener)
}

// SubscribeAll adds a listener for all progress notifications (use empty string as token)
func (pc *ProgressChannel) SubscribeAll(listener ProgressListener) {
	pc.Subscribe("", listener)
}

// Unsubscribe removes all listeners for a specific progress token
func (pc *ProgressChannel) Unsubscribe(progressToken string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	delete(pc.listeners, progressToken)
}

// Publish sends a progress notification to all relevant listeners
func (pc *ProgressChannel) Publish(notification *ProgressNotification) error {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if !pc.active {
		return fmt.Errorf("progress channel is not active")
	}

	// Validate the notification before publishing
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid progress notification: %w", err)
	}

	var lastError error

	// Send to token-specific listeners
	if listeners, exists := pc.listeners[notification.Params.ProgressToken]; exists {
		for _, listener := range listeners {
			if err := listener(notification); err != nil {
				lastError = err
			}
		}
	}

	// Send to global listeners (empty token)
	if globalListeners, exists := pc.listeners[""]; exists {
		for _, listener := range globalListeners {
			if err := listener(notification); err != nil {
				lastError = err
			}
		}
	}

	return lastError
}

// Close deactivates the progress channel and clears all listeners
func (pc *ProgressChannel) Close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.active = false
	pc.listeners = make(map[string][]ProgressListener)
}

// IsActive returns whether the progress channel is active
func (pc *ProgressChannel) IsActive() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.active
}

// GetListenerCount returns the number of listeners for a specific token
func (pc *ProgressChannel) GetListenerCount(progressToken string) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if listeners, exists := pc.listeners[progressToken]; exists {
		return len(listeners)
	}
	return 0
}

// GetTotalListenerCount returns the total number of listeners across all tokens
func (pc *ProgressChannel) GetTotalListenerCount() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	total := 0
	for _, listeners := range pc.listeners {
		total += len(listeners)
	}
	return total
}

// ProgressToken represents a unique token for tracking progress of long-running operations
type ProgressToken struct {
	// Token is the unique string identifier
	Token string

	// RequestID is the ID of the request this progress token is associated with
	RequestID string

	// CreatedAt is when this token was created
	CreatedAt time.Time

	// LastUpdate is when this token was last used for progress reporting
	LastUpdate time.Time

	// IsActive indicates if this token is still active
	IsActive bool

	// LastProgress tracks the last reported progress value to enforce increasing requirement
	LastProgress float64

	// ProtocolVersion tracks which MCP version this token is for
	ProtocolVersion string
}

// ProgressTokenManager manages progress tokens in a thread-safe manner
type ProgressTokenManager struct {
	mu     sync.RWMutex
	tokens map[string]*ProgressToken
}

// NewProgressTokenManager creates a new progress token manager
func NewProgressTokenManager() *ProgressTokenManager {
	return &ProgressTokenManager{
		tokens: make(map[string]*ProgressToken),
	}
}

// GenerateToken creates a new unique progress token
func (ptm *ProgressTokenManager) GenerateToken(requestID string) string {
	return ptm.GenerateTokenForVersion(requestID, "draft")
}

// GenerateTokenForVersion creates a new unique progress token for a specific protocol version
func (ptm *ProgressTokenManager) GenerateTokenForVersion(requestID string, protocolVersion string) string {
	// Generate a cryptographically secure random token
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based token if crypto/rand fails
		return fmt.Sprintf("progress_%d_%s", time.Now().UnixNano(), requestID)
	}

	token := "progress_" + hex.EncodeToString(bytes)

	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	// Store the token
	ptm.tokens[token] = &ProgressToken{
		Token:           token,
		RequestID:       requestID,
		CreatedAt:       time.Now(),
		LastUpdate:      time.Now(),
		IsActive:        true,
		LastProgress:    -1, // Initialize to -1 so first progress value (0 or positive) is always valid
		ProtocolVersion: protocolVersion,
	}

	return token
}

// ValidateToken checks if a progress token is valid and active
func (ptm *ProgressTokenManager) ValidateToken(token string) bool {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	progressToken, exists := ptm.tokens[token]
	if !exists {
		return false
	}

	return progressToken.IsActive
}

// UpdateToken updates the last update time for a token
func (ptm *ProgressTokenManager) UpdateToken(token string) error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	progressToken, exists := ptm.tokens[token]
	if !exists {
		return fmt.Errorf("progress token not found: %s", token)
	}

	if !progressToken.IsActive {
		return fmt.Errorf("progress token is inactive: %s", token)
	}

	progressToken.LastUpdate = time.Now()
	return nil
}

// UpdateTokenWithProgress updates the token with a new progress value, validating it increases
func (ptm *ProgressTokenManager) UpdateTokenWithProgress(token string, progress float64) error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	progressToken, exists := ptm.tokens[token]
	if !exists {
		return fmt.Errorf("progress token not found: %s", token)
	}

	if !progressToken.IsActive {
		return fmt.Errorf("progress token is inactive: %s", token)
	}

	// Validate that progress increases as required by MCP specification
	if progress < progressToken.LastProgress {
		return fmt.Errorf("progress value (%f) must not decrease from previous value (%f)", progress, progressToken.LastProgress)
	}

	progressToken.LastProgress = progress
	progressToken.LastUpdate = time.Now()
	return nil
}

// GetLastProgress returns the last reported progress value for a token
func (ptm *ProgressTokenManager) GetLastProgress(token string) (float64, error) {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	progressToken, exists := ptm.tokens[token]
	if !exists {
		return 0, fmt.Errorf("progress token not found: %s", token)
	}

	return progressToken.LastProgress, nil
}

// DeactivateToken marks a token as inactive (operation completed or cancelled)
func (ptm *ProgressTokenManager) DeactivateToken(token string) error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	progressToken, exists := ptm.tokens[token]
	if !exists {
		return fmt.Errorf("progress token not found: %s", token)
	}

	progressToken.IsActive = false
	return nil
}

// GetToken retrieves a progress token by its string value
func (ptm *ProgressTokenManager) GetToken(token string) (*ProgressToken, error) {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	progressToken, exists := ptm.tokens[token]
	if !exists {
		return nil, fmt.Errorf("progress token not found: %s", token)
	}

	// Return a copy to prevent external modification
	return &ProgressToken{
		Token:           progressToken.Token,
		RequestID:       progressToken.RequestID,
		CreatedAt:       progressToken.CreatedAt,
		LastUpdate:      progressToken.LastUpdate,
		IsActive:        progressToken.IsActive,
		LastProgress:    progressToken.LastProgress,
		ProtocolVersion: progressToken.ProtocolVersion,
	}, nil
}

// CleanupExpiredTokens removes tokens that haven't been updated for the specified duration
func (ptm *ProgressTokenManager) CleanupExpiredTokens(expiration time.Duration) int {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	now := time.Now()
	removed := 0

	for token, progressToken := range ptm.tokens {
		if now.Sub(progressToken.LastUpdate) > expiration {
			delete(ptm.tokens, token)
			removed++
		}
	}

	return removed
}

// GetActiveTokens returns a list of all active progress tokens
func (ptm *ProgressTokenManager) GetActiveTokens() []string {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	var activeTokens []string
	for token, progressToken := range ptm.tokens {
		if progressToken.IsActive {
			activeTokens = append(activeTokens, token)
		}
	}

	return activeTokens
}

// ExtractProgressTokenFromRequest extracts a progress token from request metadata
// This follows the MCP specification pattern where progress tokens are in params._meta.progressToken
func ExtractProgressTokenFromRequest(requestBytes []byte) (string, error) {
	// Parse the request to look for progress token in the correct location per MCP spec
	var request struct {
		Params struct {
			Meta map[string]interface{} `json:"_meta,omitempty"`
		} `json:"params,omitempty"`
	}

	if err := json.Unmarshal(requestBytes, &request); err != nil {
		return "", fmt.Errorf("failed to parse request for progress token: %w", err)
	}

	// Check params._meta.progressToken as per MCP specification
	if request.Params.Meta != nil {
		if token, ok := request.Params.Meta["progressToken"].(string); ok && token != "" {
			return token, nil
		}
		// Also check for integer tokens as the spec allows both string and integer
		if tokenInt, ok := request.Params.Meta["progressToken"].(float64); ok {
			return fmt.Sprintf("%.0f", tokenInt), nil
		}
	}

	// No progress token found
	return "", nil
}

// ProgressReporter provides a user-friendly API for progress reporting with automatic token management
type ProgressReporter struct {
	// Immutable fields (set once, never changed)
	token              string
	requestID          string
	tokenManager       *ProgressTokenManager
	notificationSender ProgressNotificationSender
	parent             *ProgressReporter
	childWeight        float64
	startTime          time.Time
	protocolVersion    string

	// Mutable fields protected by atomic operations
	current     int64 // Using int64 for atomic operations (scaled by 1000 for precision)
	isActive    int32 // 1 for true, 0 for false
	isCompleted int32 // 1 for true, 0 for false

	// Fields requiring mutex protection
	mu             sync.RWMutex
	total          *float64
	message        string
	lastUpdateTime time.Time
	children       map[string]*ProgressReporter
}

// ProgressNotificationSender is an interface for sending progress notifications
// This allows the ProgressReporter to work with different server implementations
type ProgressNotificationSender interface {
	SendProgressNotification(progressToken string, progress float64, total *float64, message string) error
}

// ProgressReporterConfig contains configuration options for creating a ProgressReporter
type ProgressReporterConfig struct {
	// RequestID is the ID of the request this progress is associated with
	RequestID string

	// Total is the expected total value for progress (optional)
	Total *float64

	// InitialMessage is the initial progress message
	InitialMessage string

	// TokenManager is used for token lifecycle management (optional, will create default if nil)
	TokenManager *ProgressTokenManager

	// NotificationSender is used to send progress notifications (optional)
	NotificationSender ProgressNotificationSender

	// Parent is the parent ProgressReporter for hierarchical progress (optional)
	Parent *ProgressReporter

	// ChildWeight is how much this reporter contributes to parent's progress (0.0-1.0, default 1.0)
	ChildWeight float64

	// ProtocolVersion specifies which MCP version to use (draft, 2024-11-05, 2025-03-26)
	ProtocolVersion string
}

// NewProgressReporter creates a new ProgressReporter with the given configuration
func NewProgressReporter(config ProgressReporterConfig) *ProgressReporter {
	if config.TokenManager == nil {
		config.TokenManager = NewProgressTokenManager()
	}

	if config.ChildWeight <= 0 {
		config.ChildWeight = 1.0
	}

	if config.ProtocolVersion == "" {
		config.ProtocolVersion = "draft" // Default to draft version
	}

	token := config.TokenManager.GenerateTokenForVersion(config.RequestID, config.ProtocolVersion)

	reporter := &ProgressReporter{
		token:              token,
		requestID:          config.RequestID,
		tokenManager:       config.TokenManager,
		notificationSender: config.NotificationSender,
		parent:             config.Parent,
		childWeight:        config.ChildWeight,
		startTime:          time.Now(),
		protocolVersion:    config.ProtocolVersion,
		// Atomic fields initialized to zero values
		current:     0, // 0.0 * 1000 = 0
		isActive:    1, // true
		isCompleted: 0, // false
		// Mutex-protected fields
		total:          config.Total,
		message:        config.InitialMessage,
		lastUpdateTime: time.Now(),
		children:       make(map[string]*ProgressReporter),
	}

	// Register with parent if provided
	if config.Parent != nil {
		config.Parent.mu.Lock()
		config.Parent.children[reporter.token] = reporter
		config.Parent.mu.Unlock()
	}

	return reporter
}

// NewSimpleProgressReporter creates a basic ProgressReporter with minimal configuration
func NewSimpleProgressReporter(requestID string, total *float64) *ProgressReporter {
	return NewProgressReporter(ProgressReporterConfig{
		RequestID: requestID,
		Total:     total,
	})
}

// NewSimpleProgressReporterForVersion creates a basic ProgressReporter for a specific protocol version
func NewSimpleProgressReporterForVersion(requestID string, total *float64, protocolVersion string) *ProgressReporter {
	return NewProgressReporter(ProgressReporterConfig{
		RequestID:       requestID,
		Total:           total,
		ProtocolVersion: protocolVersion,
	})
}

// GetToken returns the progress token for this reporter
func (pr *ProgressReporter) GetToken() string {
	return pr.token // Immutable, no lock needed
}

// GetRequestID returns the request ID associated with this reporter
func (pr *ProgressReporter) GetRequestID() string {
	return pr.requestID // Immutable, no lock needed
}

// IsActive returns whether the progress reporter is active
func (pr *ProgressReporter) IsActive() bool {
	return atomic.LoadInt32(&pr.isActive) == 1
}

// IsCompleted returns whether the progress has been completed
func (pr *ProgressReporter) IsCompleted() bool {
	return atomic.LoadInt32(&pr.isCompleted) == 1
}

// getCurrentFloat returns the current progress as a float64
func (pr *ProgressReporter) getCurrentFloat() float64 {
	return float64(atomic.LoadInt64(&pr.current)) / 1000.0
}

// setCurrentFloat sets the current progress from a float64
func (pr *ProgressReporter) setCurrentFloat(value float64) {
	atomic.StoreInt64(&pr.current, int64(value*1000))
}

// Update updates the progress with a new current value and optional message
func (pr *ProgressReporter) Update(current float64, message ...string) error {
	if !pr.IsActive() {
		return fmt.Errorf("progress reporter is not active")
	}

	if pr.IsCompleted() {
		return fmt.Errorf("progress reporter is already completed")
	}

	// Validate progress value
	if current < 0 {
		return fmt.Errorf("progress value cannot be negative: %f", current)
	}

	// Check total constraint (need lock for this)
	pr.mu.RLock()
	total := pr.total
	pr.mu.RUnlock()

	if total != nil && current > *total {
		return fmt.Errorf("progress value (%f) cannot exceed total (%f)", current, *total)
	}

	// Validate that progress increases as required by MCP specification
	previousCurrent := pr.getCurrentFloat()
	if current < previousCurrent {
		return fmt.Errorf("progress value (%f) must not decrease from previous value (%f)", current, previousCurrent)
	}

	// Update progress atomically
	pr.setCurrentFloat(current)

	// Update message and timestamp (need lock for these)
	pr.mu.Lock()
	if len(message) > 0 && message[0] != "" {
		pr.message = message[0]
	}
	pr.lastUpdateTime = time.Now()
	msg := pr.message
	pr.mu.Unlock()

	// Update token manager with progress validation
	if pr.tokenManager != nil {
		if err := pr.tokenManager.UpdateTokenWithProgress(pr.token, current); err != nil {
			return fmt.Errorf("failed to update token with progress: %w", err)
		}
	}

	// Send notification if sender is available
	if pr.notificationSender != nil {
		// Create protocol-version-aware notification
		notification := NewProgressNotificationForVersion(pr.token, current, total, msg, pr.protocolVersion)
		if err := notification.Validate(); err != nil {
			return fmt.Errorf("invalid progress notification: %w", err)
		}

		if err := pr.notificationSender.SendProgressNotification(pr.token, current, total, msg); err != nil {
			return fmt.Errorf("failed to send progress notification: %w", err)
		}
	}

	return nil
}

// Increment increments the progress by the given amount
func (pr *ProgressReporter) Increment(amount float64, message ...string) error {
	current := pr.getCurrentFloat()
	return pr.Update(current+amount, message...)
}

// SetTotal updates the total expected value
func (pr *ProgressReporter) SetTotal(total float64) error {
	if !pr.IsActive() {
		return fmt.Errorf("progress reporter is not active")
	}

	if total < 0 {
		return fmt.Errorf("total value cannot be negative: %f", total)
	}

	current := pr.getCurrentFloat()
	if current > total {
		return fmt.Errorf("current progress (%f) exceeds new total (%f)", current, total)
	}

	pr.mu.Lock()
	pr.total = &total
	pr.mu.Unlock()
	return nil
}

// Complete marks the progress as completed with an optional final message
func (pr *ProgressReporter) Complete(message ...string) error {
	if !pr.IsActive() {
		return fmt.Errorf("progress reporter is not active")
	}

	if pr.IsCompleted() {
		return nil // Already completed
	}

	// Get/set total and current
	pr.mu.Lock()
	if pr.total != nil {
		pr.setCurrentFloat(*pr.total)
	} else {
		pr.setCurrentFloat(100.0)
		total := 100.0
		pr.total = &total
	}

	if len(message) > 0 && message[0] != "" {
		pr.message = message[0]
	}
	pr.lastUpdateTime = time.Now()
	msg := pr.message
	total := pr.total
	pr.mu.Unlock()

	// Mark as completed
	atomic.StoreInt32(&pr.isCompleted, 1)

	current := pr.getCurrentFloat()

	// Send final notification BEFORE deactivating token
	if pr.notificationSender != nil {
		if err := pr.notificationSender.SendProgressNotification(pr.token, current, total, msg); err != nil {
			return fmt.Errorf("failed to send final progress notification: %w", err)
		}
	}

	// Deactivate token AFTER sending notification
	if pr.tokenManager != nil {
		if err := pr.tokenManager.DeactivateToken(pr.token); err != nil {
			// Log error but don't fail the completion
			// TODO: Add proper logging when logger is available
			_ = err // Acknowledge the error to satisfy linter
		}
	}

	return nil
}

// Cancel cancels the progress reporting and deactivates the token
func (pr *ProgressReporter) Cancel(message ...string) error {
	if !pr.IsActive() {
		return nil // Already inactive
	}

	// Mark as inactive
	atomic.StoreInt32(&pr.isActive, 0)

	pr.mu.Lock()
	if len(message) > 0 && message[0] != "" {
		pr.message = message[0]
	}
	pr.lastUpdateTime = time.Now()
	pr.mu.Unlock()

	// Deactivate token
	if pr.tokenManager != nil {
		if err := pr.tokenManager.DeactivateToken(pr.token); err != nil {
			// Log error but don't fail the cancellation
			// TODO: Add proper logging when logger is available
			_ = err // Acknowledge the error to satisfy linter
		}
	}

	return nil
}

// CreateChild creates a child ProgressReporter for hierarchical progress tracking
// Note: Simplified version without automatic parent updates to avoid deadlocks
func (pr *ProgressReporter) CreateChild(requestID string, weight float64, total *float64) *ProgressReporter {
	if !pr.IsActive() {
		return nil
	}

	if weight <= 0 || weight > 1 {
		weight = 1.0 // Default weight
	}

	child := NewProgressReporter(ProgressReporterConfig{
		RequestID:          requestID,
		Total:              total,
		TokenManager:       pr.tokenManager,
		NotificationSender: pr.notificationSender,
		Parent:             pr,
		ChildWeight:        weight,
		ProtocolVersion:    pr.protocolVersion,
	})

	// Add to children map
	pr.mu.Lock()
	pr.children[child.token] = child
	pr.mu.Unlock()

	return child
}

// GetProgress returns the current progress information
func (pr *ProgressReporter) GetProgress() (current float64, total *float64, message string, isCompleted bool) {
	current = pr.getCurrentFloat()
	isCompleted = pr.IsCompleted()

	pr.mu.RLock()
	total = pr.total
	message = pr.message
	pr.mu.RUnlock()

	return current, total, message, isCompleted
}

// GetPercentage returns the current progress as a percentage (0-100)
func (pr *ProgressReporter) GetPercentage() *float64 {
	current := pr.getCurrentFloat()

	pr.mu.RLock()
	total := pr.total
	pr.mu.RUnlock()

	if total == nil || *total == 0 {
		return nil
	}

	percentage := (current / *total) * 100
	return &percentage
}

// GetDuration returns how long the progress has been running
func (pr *ProgressReporter) GetDuration() time.Duration {
	return time.Since(pr.startTime)
}

// GetTimeSinceLastUpdate returns how long since the last progress update
func (pr *ProgressReporter) GetTimeSinceLastUpdate() time.Duration {
	pr.mu.RLock()
	lastUpdate := pr.lastUpdateTime
	pr.mu.RUnlock()
	return time.Since(lastUpdate)
}

// GetChildCount returns the number of child reporters
func (pr *ProgressReporter) GetChildCount() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.children)
}

// GetChildren returns a copy of the children map for safe iteration
func (pr *ProgressReporter) GetChildren() map[string]*ProgressReporter {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	children := make(map[string]*ProgressReporter)
	for token, child := range pr.children {
		children[token] = child
	}
	return children
}

// GetStatistics returns detailed statistics about this progress reporter
func (pr *ProgressReporter) GetStatistics() map[string]interface{} {
	current := pr.getCurrentFloat()
	isActive := pr.IsActive()
	isCompleted := pr.IsCompleted()

	pr.mu.RLock()
	total := pr.total
	message := pr.message
	lastUpdateTime := pr.lastUpdateTime
	childCount := len(pr.children)
	pr.mu.RUnlock()

	stats := map[string]interface{}{
		"token":               pr.token,
		"requestID":           pr.requestID,
		"current":             current,
		"total":               total,
		"message":             message,
		"isActive":            isActive,
		"isCompleted":         isCompleted,
		"childWeight":         pr.childWeight,
		"startTime":           pr.startTime,
		"lastUpdateTime":      lastUpdateTime,
		"duration":            time.Since(pr.startTime),
		"timeSinceLastUpdate": time.Since(lastUpdateTime),
		"childCount":          childCount,
	}

	if total != nil {
		percentage := (current / *total) * 100
		stats["percentage"] = percentage
	}

	return stats
}

// GetProtocolVersion returns the MCP protocol version for this reporter
func (pr *ProgressReporter) GetProtocolVersion() string {
	return pr.protocolVersion // Immutable, no lock needed
}

// SetProtocolVersion updates the MCP protocol version for this reporter
func (pr *ProgressReporter) SetProtocolVersion(version string) {
	// Note: This is not thread-safe by design since protocol version
	// should typically be set once during creation
	pr.protocolVersion = version
}

// Convenience methods for common progress reporting patterns

// StartOperation is a convenience method that creates a reporter and sends an initial notification
func StartOperation(requestID string, total *float64, initialMessage string, sender ProgressNotificationSender) *ProgressReporter {
	reporter := NewProgressReporter(ProgressReporterConfig{
		RequestID:          requestID,
		Total:              total,
		InitialMessage:     initialMessage,
		NotificationSender: sender,
	})

	// Send initial notification if sender is available
	if sender != nil {
		if err := sender.SendProgressNotification(reporter.token, 0.0, total, initialMessage); err != nil {
			// Log error but don't fail the creation
			// TODO: Add proper logging when logger is available
			_ = err // Acknowledge the error to satisfy linter
		}
	}

	return reporter
}

// UpdateOperation is a convenience method for updating progress with automatic percentage calculation
func UpdateOperation(reporter *ProgressReporter, completed int, total int, message string) error {
	if total <= 0 {
		return fmt.Errorf("total must be positive")
	}

	percentage := (float64(completed) / float64(total)) * 100
	return reporter.Update(percentage, message)
}

// CompleteOperation is a convenience method for completing an operation
func CompleteOperation(reporter *ProgressReporter, finalMessage string) error {
	return reporter.Complete(finalMessage)
}
