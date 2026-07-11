package benchmarks_test

import (
	"context"
	"errors"
	"testing"

	avastretry "github.com/avast/retry-go/v4"
	"github.com/sinashahoveisi/resilium/retry"
)

// Comparison notes (not apples-to-apples):
// - avast/retry-go defaults to Attempts(10) and exponential backoff with jitter;
//   we set Attempts, Delay(0), and FixedDelay to match resilium's zero-delay
//   fixed backoff for measuring retry-loop overhead, not sleep time.
// - avast/retry-go wraps errors in a retry.Error slice unless LastErrorOnly(true);
//   resilium returns ErrMaxAttemptsExceeded wrapping the last error.
// - resilium.Do is generic and context-first; avast/retry-go uses Option funcs.

const successAttempt = 3

func BenchmarkResiliumRetrySuccessOnNth(b *testing.B) {
	ctx := context.Background()
	cfg := retry.Config{
		MaxAttempts: successAttempt,
		Backoff:     retry.FixedBackoff(0),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := 0
		_, err := retry.Do(ctx, cfg, func(ctx context.Context) (struct{}, error) {
			attempt++
			if attempt < successAttempt {
				return struct{}{}, errors.New("fail")
			}
			return struct{}{}, nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAvastRetrySuccessOnNth(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := 0
		err := avastretry.Do(func() error {
			attempt++
			if attempt < successAttempt {
				return errors.New("fail")
			}
			return nil
		},
			avastretry.Attempts(successAttempt),
			avastretry.Delay(0),
			avastretry.DelayType(avastretry.FixedDelay),
			avastretry.LastErrorOnly(true),
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResiliumRetryExhausted(b *testing.B) {
	ctx := context.Background()
	cfg := retry.Config{
		MaxAttempts: successAttempt,
		Backoff:     retry.FixedBackoff(0),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := retry.Do(ctx, cfg, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, errors.New("fail")
		})
		if !errors.Is(err, retry.ErrMaxAttemptsExceeded) {
			b.Fatalf("err = %v, want ErrMaxAttemptsExceeded", err)
		}
	}
}

func BenchmarkAvastRetryExhausted(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := avastretry.Do(func() error {
			return errors.New("fail")
		},
			avastretry.Attempts(successAttempt),
			avastretry.Delay(0),
			avastretry.DelayType(avastretry.FixedDelay),
			avastretry.LastErrorOnly(true),
		)
		if err == nil {
			b.Fatal("expected error")
		}
	}
}
