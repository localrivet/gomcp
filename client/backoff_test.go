package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExponentialBackoff(t *testing.T) {
	// Test creating a new exponential backoff strategy
	initialDelay := 100 * time.Millisecond
	maxDelay := 5 * time.Second
	maxAttempts := 5

	backoff := NewExponentialBackoff(initialDelay, maxDelay, maxAttempts)

	// Test WithFactor
	backoff.WithFactor(1.5)

	// Test WithJitter
	backoff.WithJitter(0.1)

	// Test WithResetOnPause
	backoff.WithResetOnPause(true)

	// Test MaxAttempts
	assert.Equal(t, maxAttempts, backoff.MaxAttempts())

	// Test NextDelay - first attempt should return initialDelay (with jitter)
	firstDelay := backoff.NextDelay(1)
	assert.True(t, firstDelay >= 90*time.Millisecond, "First delay should be approximately initialDelay with jitter")
	assert.True(t, firstDelay <= 110*time.Millisecond, "First delay should be approximately initialDelay with jitter")

	// Test NextDelay - exponential growth
	secondDelay := backoff.NextDelay(2)
	assert.True(t, secondDelay > firstDelay, "Second delay should be greater than first delay")

	// Test NextDelay - max delay cap
	finalDelay := backoff.NextDelay(10) // Way beyond maxAttempts
	assert.True(t, finalDelay <= maxDelay, "Delay should never exceed maxDelay")

	// Test NextDelay - invalid attempt
	zeroDelay := backoff.NextDelay(0)
	assert.Equal(t, time.Duration(0), zeroDelay, "Delay for attempt 0 should be 0")
}

func TestConstantBackoff(t *testing.T) {
	// Test creating a new constant backoff strategy
	delay := 200 * time.Millisecond
	maxAttempts := 3

	backoff := NewConstantBackoff(delay, maxAttempts)

	// Test WithJitter
	backoff.WithJitter(0.1)

	// Test MaxAttempts
	assert.Equal(t, maxAttempts, backoff.MaxAttempts())

	// Test NextDelay - all attempts should return similar delay (with jitter)
	firstDelay := backoff.NextDelay(1)
	assert.True(t, firstDelay >= 180*time.Millisecond, "Delay should be approximately delay with jitter")
	assert.True(t, firstDelay <= 220*time.Millisecond, "Delay should be approximately delay with jitter")

	secondDelay := backoff.NextDelay(2)
	assert.True(t, secondDelay >= 180*time.Millisecond, "Delay should be approximately delay with jitter")
	assert.True(t, secondDelay <= 220*time.Millisecond, "Delay should be approximately delay with jitter")

	// Test NextDelay - invalid attempt
	zeroDelay := backoff.NextDelay(0)
	assert.Equal(t, time.Duration(0), zeroDelay, "Delay for attempt 0 should be 0")
}

func TestNoBackoff(t *testing.T) {
	// Test creating a new no-backoff strategy
	maxAttempts := 2

	backoff := NewNoBackoff(maxAttempts)

	// Test MaxAttempts
	assert.Equal(t, maxAttempts, backoff.MaxAttempts())

	// Test NextDelay - all attempts should return 0
	assert.Equal(t, time.Duration(0), backoff.NextDelay(1))
	assert.Equal(t, time.Duration(0), backoff.NextDelay(2))
	assert.Equal(t, time.Duration(0), backoff.NextDelay(3)) // Beyond maxAttempts
}
