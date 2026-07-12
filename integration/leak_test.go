package integration_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium"
	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/retry"
)

const goroutineLeakTolerance = 2

func settleGoroutines() {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
}

func countGoroutines() int {
	settleGoroutines()
	return runtime.NumGoroutine()
}

func TestIntegrationNoGoroutineLeak(t *testing.T) {
	srv := newSwitchableServer()

	successPolicy := resilium.New(
		resilium.WithTimeout(100*time.Millisecond),
		resilium.WithRetry(retry.Config{
			MaxAttempts: 3,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
	)

	timeoutPolicy := resilium.New(
		resilium.WithTimeout(15 * time.Millisecond),
	)

	retryExhaustPolicy := resilium.New(
		resilium.WithRetry(retry.Config{
			MaxAttempts: 3,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
	)

	cbPolicy := resilium.New(
		resilium.WithRetry(retry.Config{
			MaxAttempts: 5,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
		resilium.WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenDuration:     50 * time.Millisecond,
		}),
	)

	baseline := countGoroutines()

	ctx := context.Background()
	const iterations = 3000

	for i := 0; i < iterations; i++ {
		switch i % 4 {
		case 0:
			srv.SetMode(modeOK)
			_, _ = resilium.Execute(ctx, successPolicy, func(ctx context.Context) (struct{}, error) {
				if err := requireHTTPOK(ctx, srv.URL("/")); err != nil {
					return struct{}{}, err
				}
				return struct{}{}, nil
			})
		case 1:
			srv.SetMode(modeStatus500)
			_, _ = resilium.Execute(ctx, retryExhaustPolicy, func(ctx context.Context) (struct{}, error) {
				code, err := httpGet(ctx, srv.URL("/"))
				if err != nil {
					return struct{}{}, err
				}
				return struct{}{}, errFromHTTPStatus(code)
			})
		case 2:
			srv.SetSlowDelay(200 * time.Millisecond)
			_, _ = resilium.Execute(ctx, timeoutPolicy, func(ctx context.Context) (struct{}, error) {
				code, err := httpGet(ctx, srv.URL("/slow"))
				if err != nil {
					return struct{}{}, err
				}
				return struct{}{}, errFromHTTPStatus(code)
			})
		case 3:
			srv.SetMode(modeStatus500)
			_, _ = resilium.Execute(ctx, cbPolicy, func(ctx context.Context) (struct{}, error) {
				code, err := httpGet(ctx, srv.URL("/"))
				if err != nil {
					return struct{}{}, err
				}
				return struct{}{}, errFromHTTPStatus(code)
			})
		}
	}

	srv.Close()
	closeTestHTTPIdle()
	settleGoroutines()
	after := runtime.NumGoroutine()
	delta := after - baseline
	t.Logf("goroutines: baseline=%d after=%d delta=%+d (tolerance=%d)", baseline, after, delta, goroutineLeakTolerance)

	if delta > goroutineLeakTolerance {
		t.Fatalf("goroutine leak suspected: baseline=%d after=%d delta=%+d (tolerance=%d)",
			baseline, after, delta, goroutineLeakTolerance)
	}
}

func TestIntegrationLeakBaselineSanity(t *testing.T) {
	if testing.Short() {
		t.Skip("baseline sanity is covered by full leak test")
	}
	before := runtime.NumGoroutine()
	settleGoroutines()
	after := runtime.NumGoroutine()
	if after > before+goroutineLeakTolerance {
		t.Fatalf("settle increased goroutines %d -> %d", before, after)
	}
}

func TestIntegrationLeakSentinelCoverage(t *testing.T) {
	srv := newSwitchableServer()
	defer srv.Close()

	timeoutPolicy := resilium.New(resilium.WithTimeout(10 * time.Millisecond))
	srv.SetSlowDelay(100 * time.Millisecond)
	_, err := resilium.Execute(context.Background(), timeoutPolicy, func(ctx context.Context) (struct{}, error) {
		_, err := httpGet(ctx, srv.URL("/slow"))
		return struct{}{}, err
	})
	if !errors.Is(err, resilium.ErrTimeout) {
		t.Fatalf("timeout sentinel not hit: %v", err)
	}
}
