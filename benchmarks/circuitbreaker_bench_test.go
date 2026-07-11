package benchmarks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium/circuitbreaker"
	gobreaker "github.com/sony/gobreaker"
)

// Comparison notes (not apples-to-apples):
// - resilium uses a fixed-size sliding window; gobreaker uses cumulative
//   Counts cleared on Interval in the closed state.
// - resilium.Do checks ctx.Err() before the call; gobreaker.Execute has no
//   context parameter (we pass context.Background() only on the resilium side).
// - Open-state rejection errors differ (ErrCircuitOpen vs ErrOpenState).

func BenchmarkResiliumCircuitBreakerClosedHappy(b *testing.B) {
	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: 0.5,
		MinRequests:      5,
		OpenDuration:     time.Second,
		WindowSize:       20,
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := circuitbreaker.Do(ctx, cb, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGobreakerClosedHappy(b *testing.B) {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func tripResiliumOpen(cb *circuitbreaker.CircuitBreaker) {
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = circuitbreaker.Do(ctx, cb, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, errors.New("trip")
		})
	}
}

func tripGobreakerOpen(cb *gobreaker.CircuitBreaker) {
	for i := 0; i < 6; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("trip")
		})
	}
}

func BenchmarkResiliumCircuitBreakerOpenReject(b *testing.B) {
	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: 0.5,
		MinRequests:      1,
		OpenDuration:     time.Hour,
		WindowSize:       20,
	})
	tripResiliumOpen(cb)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := circuitbreaker.Do(ctx, cb, func(ctx context.Context) (struct{}, error) {
			b.Fatal("operation should not run when breaker is open")
			return struct{}{}, nil
		})
		if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
			b.Fatalf("err = %v, want ErrCircuitOpen", err)
		}
	}
}

func BenchmarkGobreakerOpenReject(b *testing.B) {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
		Timeout: time.Hour,
	})
	tripGobreakerOpen(cb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			b.Fatal("operation should not run when breaker is open")
			return nil, nil
		})
		if !errors.Is(err, gobreaker.ErrOpenState) {
			b.Fatalf("err = %v, want ErrOpenState", err)
		}
	}
}
