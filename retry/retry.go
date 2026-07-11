// Package retry provides retry policies with configurable backoff
// strategies, used as a resilium middleware but also usable standalone.
package retry

import (
	"context"
	"time"
)

// BackoffFunc returns the delay to wait before the given retry attempt
// (1-indexed: attempt 1 is the delay before the first retry).
type BackoffFunc func(attempt int) time.Duration

// Config configures retry behavior.
type Config struct {
	// MaxAttempts is the total number of attempts, including the first
	// (non-retry) call. A value of 3 means: 1 initial call + up to 2 retries.
	MaxAttempts int

	// Backoff computes the delay between attempts. Defaults to a fixed
	// 100ms delay if nil.
	Backoff BackoffFunc

	// RetryIf decides whether a given error should trigger a retry.
	// If nil, all non-nil errors are retried.
	RetryIf func(err error) bool
}

// FixedBackoff returns a BackoffFunc with a constant delay.
func FixedBackoff(d time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		return d
	}
}

// ExponentialBackoff returns a BackoffFunc that doubles the delay each
// attempt, starting at base and capped at max.
//
// TODO: implement exponential growth with optional jitter.
func ExponentialBackoff(base, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		// placeholder
		return base
	}
}

// Do runs op, retrying according to cfg until it succeeds, attempts are
// exhausted, or ctx is cancelled.
//
// TODO: implement the retry loop, respecting cfg.RetryIf and ctx
// cancellation between attempts.
func Do[T any](ctx context.Context, cfg Config, op func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	return zero, nil
}
