package resilium

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
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
	if got := opCalls.Load(); got != 2 {
		t.Fatalf("op called %d times, want 2 (no retries after circuit opens)", got)
	}
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

func TestWithTimeoutDeadline(t *testing.T) {
	t.Parallel()

	policy := New(WithTimeout(30 * time.Millisecond))
	_, err := Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return "late", nil
		}
	})

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("errors.Is(err, ErrTimeout) = false, err = %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("errors.Is(err, context.DeadlineExceeded) = false, err = %v", err)
	}
}

func TestWithTimeoutOperationErrorNotMapped(t *testing.T) {
	t.Parallel()

	opErr := errors.New("dependency failed")
	policy := New(WithTimeout(time.Second))
	_, err := Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
		return "", opErr
	})

	if errors.Is(err, ErrTimeout) {
		t.Fatalf("operation error incorrectly mapped to ErrTimeout: %v", err)
	}
	if !errors.Is(err, opErr) {
		t.Fatalf("Execute() error = %v, want %v", err, opErr)
	}
}

func TestWithTimeoutParentCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	policy := New(WithTimeout(time.Second))
	_, err := Execute(ctx, policy, func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	if errors.Is(err, ErrTimeout) {
		t.Fatalf("parent cancel incorrectly mapped to ErrTimeout: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
}

func TestWithRateLimitFailFast(t *testing.T) {
	t.Parallel()

	policy := New(WithRateLimit(1000, 1))
	op := func(ctx context.Context) (struct{}, error) { return struct{}{}, nil }

	if _, err := Execute(context.Background(), policy, op); err != nil {
		t.Fatalf("first call: %v", err)
	}

	start := time.Now()
	_, err := Execute(context.Background(), policy, op)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second call error = %v, want ErrRateLimited", err)
	}
	if elapsed > 5*time.Millisecond {
		t.Fatalf("rate limit blocked for %v, want immediate rejection", elapsed)
	}
}

func TestWithHooks(t *testing.T) {
	var retries atomic.Int32
	var opened atomic.Int32
	var closed atomic.Int32
	var timedOut atomic.Int32

	hooks := Hooks{
		OnRetry:        func(attempt int, err error) { retries.Add(1) },
		OnCircuitOpen:  func(name string) { opened.Add(1) },
		OnCircuitClose: func(name string) { closed.Add(1) },
		OnTimeout:      func() { timedOut.Add(1) },
	}

	t.Run("OnRetry", func(t *testing.T) {
		retries.Store(0)
		policy := New(
			WithHooks(hooks),
			WithRetry(retry.Config{
				MaxAttempts: 3,
				Backoff:     retry.FixedBackoff(time.Millisecond),
			}),
		)
		attempts := 0
		_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("fail")
			}
			return "ok", nil
		})
		if retries.Load() != 2 {
			t.Fatalf("OnRetry calls = %d, want 2", retries.Load())
		}
	})

	t.Run("OnRetryExhaustion", func(t *testing.T) {
		retries.Store(0)
		const maxAttempts = 5
		policy := New(
			WithHooks(hooks),
			WithRetry(retry.Config{
				MaxAttempts: maxAttempts,
				Backoff:     retry.FixedBackoff(time.Millisecond),
			}),
		)
		_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
			return "", errors.New("fail")
		})
		if got := retries.Load(); got != maxAttempts-1 {
			t.Fatalf("OnRetry calls = %d, want %d on persistent failure", got, maxAttempts-1)
		}
	})

	t.Run("OnRateLimited", func(t *testing.T) {
		var rateLimited atomic.Int32
		policy := New(
			WithHooks(Hooks{OnRateLimited: func() { rateLimited.Add(1) }}),
			WithRateLimit(1000, 1),
		)
		op := func(ctx context.Context) (struct{}, error) { return struct{}{}, nil }
		_, _ = Execute(context.Background(), policy, op)
		_, _ = Execute(context.Background(), policy, op)
		if rateLimited.Load() != 1 {
			t.Fatalf("OnRateLimited calls = %d, want 1", rateLimited.Load())
		}
	})

	t.Run("OnCircuitOpen", func(t *testing.T) {
		opened.Store(0)
		policy := New(
			WithHooks(hooks),
			WithCircuitBreaker(circuitbreaker.Config{
				FailureThreshold: 0.5,
				MinRequests:      1,
				OpenDuration:     time.Second,
			}),
		)
		_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
			return "", errors.New("fail")
		})
		if opened.Load() != 1 {
			t.Fatalf("OnCircuitOpen calls = %d, want 1", opened.Load())
		}
	})

	t.Run("OnCircuitClose", func(t *testing.T) {
		closed.Store(0)
		policy := New(
			WithHooks(hooks),
			WithCircuitBreaker(circuitbreaker.Config{
				FailureThreshold: 0.5,
				MinRequests:      1,
				OpenDuration:     10 * time.Millisecond,
			}),
		)
		_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
			return "", errors.New("fail")
		})
		time.Sleep(15 * time.Millisecond)
		_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
			return "ok", nil
		})
		if closed.Load() != 1 {
			t.Fatalf("OnCircuitClose calls = %d, want 1", closed.Load())
		}
	})

	t.Run("OnTimeout", func(t *testing.T) {
		timedOut.Store(0)
		policy := New(
			WithHooks(hooks),
			WithTimeout(20*time.Millisecond),
		)
		_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		})
		if timedOut.Load() != 1 {
			t.Fatalf("OnTimeout calls = %d, want 1", timedOut.Load())
		}
	})
}

func TestWithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	policy := New(
		WithLogger(logger),
		WithRetry(retry.Config{
			MaxAttempts: 2,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
	)
	_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
		return "", errors.New("fail")
	})

	out := buf.String()
	if !strings.Contains(out, "resilium retry") {
		t.Fatalf("log output = %q, want retry debug log", out)
	}
}

func TestWithRateLimitLoggerWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	policy := New(
		WithLogger(logger),
		WithRateLimit(1000, 1),
	)
	op := func(ctx context.Context) (struct{}, error) { return struct{}{}, nil }
	_, _ = Execute(context.Background(), policy, op)
	_, _ = Execute(context.Background(), policy, op)

	if !strings.Contains(buf.String(), "resilium rate limited") {
		t.Fatalf("log output = %q, want rate limit warn log", buf.String())
	}
}

func TestPolicyOrderRetryBeforeCircuitBreaker(t *testing.T) {
	var cbEvaluations atomic.Int32

	policy := New(
		WithRetry(retry.Config{MaxAttempts: 3, Backoff: retry.FixedBackoff(time.Millisecond)}),
		WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      100,
			OpenDuration:     time.Second,
		}),
	)

	attempts := 0
	_, _ = Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
		attempts++
		cbEvaluations.Add(1)
		return "", errors.New("fail")
	})

	if cbEvaluations.Load() != 3 {
		t.Fatalf("circuit breaker evaluations = %d, want 3 (innermost, per attempt)", cbEvaluations.Load())
	}
}
