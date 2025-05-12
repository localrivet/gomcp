package client

import (
	"math"
	"math/rand"
	"time"
)

// ExponentialBackoff implements BackoffStrategy with exponential delay between attempts
type ExponentialBackoff struct {
	initialDelay time.Duration
	maxDelay     time.Duration
	factor       float64
	jitter       float64
	maxAttempts  int
	resetOnPause bool
	randomSource *rand.Rand
}

// NewExponentialBackoff creates a new exponential backoff strategy
func NewExponentialBackoff(initialDelay, maxDelay time.Duration, maxAttempts int) *ExponentialBackoff {
	return &ExponentialBackoff{
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
		factor:       2.0,
		jitter:       0.2,
		maxAttempts:  maxAttempts,
		resetOnPause: true,
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// WithFactor sets the exponential factor (default 2.0)
func (b *ExponentialBackoff) WithFactor(factor float64) *ExponentialBackoff {
	b.factor = factor
	return b
}

// WithJitter sets the jitter factor to randomize delays (default 0.2 - 20%)
func (b *ExponentialBackoff) WithJitter(jitter float64) *ExponentialBackoff {
	b.jitter = jitter
	return b
}

// WithResetOnPause controls whether the backoff resets after a long pause
func (b *ExponentialBackoff) WithResetOnPause(reset bool) *ExponentialBackoff {
	b.resetOnPause = reset
	return b
}

// NextDelay implements BackoffStrategy.NextDelay
func (b *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate exponential backoff with base factor
	delayFloat := float64(b.initialDelay) * math.Pow(b.factor, float64(attempt-1))

	// Apply jitter if configured
	if b.jitter > 0 {
		// Calculate jitter range (delay * jitter)
		jitterRange := delayFloat * b.jitter

		// Generate random jitter value between -jitterRange/2 and +jitterRange/2
		jitterAmount := (b.randomSource.Float64() - 0.5) * jitterRange

		// Apply jitter to delay
		delayFloat += jitterAmount
	}

	// Ensure we don't exceed max delay
	if delayFloat > float64(b.maxDelay) {
		delayFloat = float64(b.maxDelay)
	}

	return time.Duration(delayFloat)
}

// MaxAttempts implements BackoffStrategy.MaxAttempts
func (b *ExponentialBackoff) MaxAttempts() int {
	return b.maxAttempts
}

// ConstantBackoff implements BackoffStrategy with fixed delay between attempts
type ConstantBackoff struct {
	delay        time.Duration
	maxAttempts  int
	jitter       float64
	randomSource *rand.Rand
}

// NewConstantBackoff creates a new constant backoff strategy
func NewConstantBackoff(delay time.Duration, maxAttempts int) *ConstantBackoff {
	return &ConstantBackoff{
		delay:        delay,
		maxAttempts:  maxAttempts,
		jitter:       0.1,
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// WithJitter sets the jitter factor to randomize delays (default 0.1 - 10%)
func (b *ConstantBackoff) WithJitter(jitter float64) *ConstantBackoff {
	b.jitter = jitter
	return b
}

// NextDelay implements BackoffStrategy.NextDelay
func (b *ConstantBackoff) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delayFloat := float64(b.delay)

	// Apply jitter if configured
	if b.jitter > 0 {
		// Calculate jitter range (delay * jitter)
		jitterRange := delayFloat * b.jitter

		// Generate random jitter value between -jitterRange/2 and +jitterRange/2
		jitterAmount := (b.randomSource.Float64() - 0.5) * jitterRange

		// Apply jitter to delay
		delayFloat += jitterAmount
	}

	return time.Duration(delayFloat)
}

// MaxAttempts implements BackoffStrategy.MaxAttempts
func (b *ConstantBackoff) MaxAttempts() int {
	return b.maxAttempts
}

// NoBackoff implements BackoffStrategy with no delay between attempts
type NoBackoff struct {
	maxAttempts int
}

// NewNoBackoff creates a new no-backoff strategy
func NewNoBackoff(maxAttempts int) *NoBackoff {
	return &NoBackoff{maxAttempts: maxAttempts}
}

// NextDelay implements BackoffStrategy.NextDelay
func (b *NoBackoff) NextDelay(attempt int) time.Duration {
	return 0
}

// MaxAttempts implements BackoffStrategy.MaxAttempts
func (b *NoBackoff) MaxAttempts() int {
	return b.maxAttempts
}
