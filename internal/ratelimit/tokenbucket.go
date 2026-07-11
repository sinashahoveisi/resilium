// Package ratelimit provides a minimal token-bucket rate limiter for
// internal use by resilium policies.
package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket limits how often Allow returns true using a token-bucket
// algorithm. It never blocks; callers check Allow and reject if false.
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64 // tokens added per second
	burst  float64 // maximum stored tokens
	tokens float64
	last   time.Time
}

// New creates a TokenBucket that refills at rate tokens per second with
// the given burst capacity. If rate <= 0, Allow always returns false.
// Burst values below 1 are treated as 1.
func New(rate float64, burst int) *TokenBucket {
	if burst < 1 {
		burst = 1
	}
	return &TokenBucket{
		rate:   rate,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// Allow reports whether a token is available and consumes one if so.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.rate <= 0 {
		return false
	}

	now := time.Now()
	elapsed := now.Sub(tb.last).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.burst {
		tb.tokens = tb.burst
	}
	tb.last = now

	if tb.tokens < 1 {
		return false
	}
	tb.tokens--
	return true
}
