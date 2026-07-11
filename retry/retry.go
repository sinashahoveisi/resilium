// Package retry provides retry policies with configurable backoff
// strategies, used as a resilium middleware but also usable standalone.
package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrMaxAttemptsExceeded is returned when all retry attempts are exhausted.
// The last underlying error is wrapped. Callers in the root resilium package
// can map this to resilium.ErrMaxAttemptsExceeded via errors.Is.
var ErrMaxAttemptsExceeded = errors.New("retry: max attempts exceeded")

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
func ExponentialBackoff(base, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		if attempt < 1 {
			attempt = 1
		}
		delay := base
		for i := 1; i < attempt; i++ {
			if delay > max/2 {
				return max
			}
			delay *= 2
		}
		if delay > max {
			return max
		}
		return delay
	}
}

// Do runs op, retrying according to cfg until it succeeds, attempts are
// exhausted, or ctx is cancelled.
func Do[T any](ctx context.Context, cfg Config, op func(ctx context.Context) (T, error)) (T, error) {
	var zero T

	maxAttempts := cfg.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	backoff := cfg.Backoff
	if backoff == nil {
		backoff = FixedBackoff(100 * time.Millisecond)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		result, err := op(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		shouldRetry := cfg.RetryIf == nil || cfg.RetryIf(err)
		if !shouldRetry {
			return zero, err
		}

		if attempt >= maxAttempts {
			break
		}

		delay := backoff(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return zero, ctx.Err()
		case <-timer.C:
		}
	}

	return zero, fmt.Errorf("%w: %v", ErrMaxAttemptsExceeded, lastErr)
}
