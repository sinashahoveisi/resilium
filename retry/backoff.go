package retry

import (
	"math/rand/v2"
	"time"
)

// WithJitter wraps a BackoffFunc and adds random jitter up to the given
// fraction (0.0–1.0) of the computed delay, to avoid thundering-herd
// retries across many clients.
func WithJitter(fn BackoffFunc, fraction float64) BackoffFunc {
	return func(attempt int) time.Duration {
		delay := fn(attempt)
		if fraction <= 0 || delay <= 0 {
			return delay
		}
		if fraction > 1 {
			fraction = 1
		}
		jitter := time.Duration(float64(delay) * fraction * (2*rand.Float64() - 1))
		result := delay + jitter
		if result < 0 {
			return 0
		}
		return result
	}
}

// LinearBackoff returns a BackoffFunc that increases delay linearly:
// base * attempt, capped at max.
func LinearBackoff(base, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		if attempt < 1 {
			attempt = 1
		}
		delay := time.Duration(attempt) * base
		if delay > max {
			return max
		}
		return delay
	}
}
