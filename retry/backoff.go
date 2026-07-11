package retry

import (
	"math/rand"
	"time"
)

// WithJitter wraps a BackoffFunc and adds random jitter up to the given
// fraction (0.0–1.0) of the computed delay, to avoid thundering-herd
// retries across many clients.
//
// TODO: implement — e.g. delay +/- (delay * fraction * rand).
func WithJitter(fn BackoffFunc, fraction float64) BackoffFunc {
	return func(attempt int) time.Duration {
		base := fn(attempt)
		// placeholder: no jitter applied yet
		_ = rand.Float64()
		return base
	}
}

// LinearBackoff returns a BackoffFunc that increases delay linearly:
// base * attempt, capped at max.
//
// TODO: implement linear growth.
func LinearBackoff(base, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		return base
	}
}
