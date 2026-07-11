package resilium

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/retry"
)

func TestRetryStopsOnOpenCircuitBreaker(t *testing.T) {
	var opCalls atomic.Int32

	policy := New(
		WithRetry(retry.Config{
			MaxAttempts: 5,
			Backoff:     retry.FixedBackoff(50 * time.Millisecond),
		}),
		WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenDuration:     time.Second,
		}),
	)

	start := time.Now()
	_, err := Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
		opCalls.Add(1)
		return "", errors.New("fail")
	})
	elapsed := time.Since(start)

	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Execute() error = %v, want ErrCircuitOpen", err)
	}

	// Attempt 1: op fails. Attempt 2: op fails, CB trips open.
	// Attempt 3: CB rejects immediately; retry must not sleep or call op again.
	if got := opCalls.Load(); got != 2 {
		t.Fatalf("op called %d times, want 2 (no retries after circuit opens)", got)
	}

	// With MaxAttempts=5 and 50ms backoff, exhausting retries would take well over 100ms.
	if elapsed > 120*time.Millisecond {
		t.Fatalf("Execute() took %v, expected immediate stop after open circuit", elapsed)
	}
}

func TestRetryRespectsCustomRetryIfWithCircuitBreaker(t *testing.T) {
	var opCalls atomic.Int32
	var retryIfCalls atomic.Int32

	policy := New(
		WithRetry(retry.Config{
			MaxAttempts: 5,
			Backoff:     retry.FixedBackoff(time.Millisecond),
			RetryIf: func(err error) bool {
				retryIfCalls.Add(1)
				return true
			},
		}),
		WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenDuration:     time.Second,
		}),
	)

	_, err := Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
		opCalls.Add(1)
		return "", errors.New("fail")
	})

	if !errors.Is(err, ErrMaxAttemptsExceeded) {
		t.Fatalf("Execute() error = %v, want ErrMaxAttemptsExceeded when custom RetryIf always retries", err)
	}
	if got := opCalls.Load(); got != 2 {
		t.Fatalf("op called %d times, want 2", got)
	}
	if retryIfCalls.Load() < 3 {
		t.Fatalf("custom RetryIf called %d times, want at least 3 (continues past open circuit)", retryIfCalls.Load())
	}
}
