// End-to-end integration tests for github.com/sinashahoveisi/resilium.
//
// These live in integration/ (package integration_test) rather than the repo
// root so the root directory stays focused on library source. They import
// resilium as an external consumer would — the same capability as root-level
// package resilium_test, without requiring co-location with resilium.go.
//
// Unit and white-box tests for the root package remain in resilium_test.go at
// the repo root. Subpackages follow the same split (e.g. retry/*_test.go).
package integration_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium"
	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/retry"
)

func TestIntegrationIntermittent500EventuallySucceeds(t *testing.T) {
	srv := newSwitchableServer()
	defer srv.Close()

	srv.SetMode(modeFailFirstN)
	srv.SetFailUntil(2) // first two hits return 500, third succeeds

	var attempts atomic.Int32
	policy := resilium.New(
		resilium.WithTimeout(500*time.Millisecond),
		resilium.WithRetry(retry.Config{
			MaxAttempts: 5,
			Backoff:     retry.FixedBackoff(5 * time.Millisecond),
		}),
	)

	ctx := context.Background()
	_, err := resilium.Execute(ctx, policy, func(ctx context.Context) (string, error) {
		attempts.Add(1)
		code, err := httpGet(ctx, srv.URL("/"))
		if err != nil {
			return "", err
		}
		if code != http.StatusOK {
			return "", errFromHTTPStatus(code)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want success", err)
	}

	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempt count = %d, want 3 (2 failures then success)", got)
	}
	if got := srv.HitCount(); got != 3 {
		t.Fatalf("server hits = %d, want 3", got)
	}
}

func TestIntegrationSustained500TripsCircuitBreakerFastFail(t *testing.T) {
	srv := newSwitchableServer()
	defer srv.Close()
	srv.SetMode(modeStatus500)

	policy := resilium.New(
		resilium.WithRetry(retry.Config{
			MaxAttempts: 10,
			Backoff:     retry.FixedBackoff(50 * time.Millisecond),
		}),
		resilium.WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenDuration:     time.Second,
			WindowSize:       10,
		}),
	)

	ctx := context.Background()
	op := func(ctx context.Context) (struct{}, error) {
		code, err := httpGet(ctx, srv.URL("/"))
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, errFromHTTPStatus(code)
	}

	_, err := resilium.Execute(ctx, policy, op)
	if err == nil {
		t.Fatal("expected error on sustained 500s")
	}

	hitsAfterTrip := srv.HitCount()
	if hitsAfterTrip > 3 {
		t.Fatalf("server hits after trip = %d, want at most 3 (breaker should open before retry storm)", hitsAfterTrip)
	}

	for i := 0; i < 5; i++ {
		hitsBefore := srv.HitCount()
		start := time.Now()
		_, err := resilium.Execute(ctx, policy, op)
		elapsed := time.Since(start)
		hitsAfter := srv.HitCount()

		if !errors.Is(err, resilium.ErrCircuitOpen) {
			t.Fatalf("call %d: error = %v, want ErrCircuitOpen", i, err)
		}
		if hitsAfter != hitsBefore {
			t.Fatalf("call %d: server hits increased %d -> %d while breaker open", i, hitsBefore, hitsAfter)
		}
		if elapsed > 25*time.Millisecond {
			t.Fatalf("call %d: fast-fail took %v, want near-zero (no backoff while open)", i, elapsed)
		}
	}
}

func TestIntegrationSlowServerTimeoutPerCall(t *testing.T) {
	srv := newSwitchableServer()
	defer srv.Close()
	srv.SetSlowDelay(500 * time.Millisecond)

	policy := resilium.New(
		resilium.WithTimeout(50 * time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := resilium.Execute(ctx, policy, func(ctx context.Context) (string, error) {
		code, err := httpGet(ctx, srv.URL("/slow"))
		if err != nil {
			return "", err
		}
		return "", errFromHTTPStatus(code)
	})
	elapsed := time.Since(start)

	if !errors.Is(err, resilium.ErrTimeout) {
		t.Fatalf("error = %v, want ErrTimeout", err)
	}
	if elapsed >= 200*time.Millisecond {
		t.Fatalf("call took %v, want timeout to cut short well before 500ms server sleep", elapsed)
	}

	done := make(chan error, 1)
	go func() {
		_, err := resilium.Execute(ctx, policy, func(ctx context.Context) (string, error) {
			if err := requireHTTPOK(ctx, srv.URL("/fast")); err != nil {
				return "", err
			}
			return "ok", nil
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("concurrent fast call error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("concurrent fast call timed out")
	}
}

func TestIntegrationCircuitBreakerRecoveryAfterServerHeals(t *testing.T) {
	srv := newSwitchableServer()
	defer srv.Close()
	srv.SetMode(modeStatus500)

	openDuration := 75 * time.Millisecond
	policy := resilium.New(
		resilium.WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      1,
			OpenDuration:     openDuration,
			WindowSize:       5,
		}),
	)

	ctx := context.Background()
	op := func(ctx context.Context) (string, error) {
		code, err := httpGet(ctx, srv.URL("/"))
		if err != nil {
			return "", err
		}
		if code != http.StatusOK {
			return "", errFromHTTPStatus(code)
		}
		return "ok", nil
	}

	_, err := resilium.Execute(ctx, policy, op)
	if err == nil {
		t.Fatal("expected failure while server returns 500")
	}

	_, err = resilium.Execute(ctx, policy, op)
	if !errors.Is(err, resilium.ErrCircuitOpen) {
		t.Fatalf("second call error = %v, want ErrCircuitOpen (breaker open)", err)
	}

	srv.SetMode(modeOK)
	time.Sleep(openDuration + 25*time.Millisecond)

	result, err := resilium.Execute(ctx, policy, op)
	if err != nil {
		t.Fatalf("recovery call error = %v, want success after OpenDuration", err)
	}
	if result != "ok" {
		t.Fatalf("result = %q, want ok", result)
	}

	for i := 0; i < 3; i++ {
		if _, err := resilium.Execute(ctx, policy, op); err != nil {
			t.Fatalf("call %d after recovery: %v", i, err)
		}
	}
}

func TestIntegrationRateLimitBurstAndRefill(t *testing.T) {
	srv := newSwitchableServer()
	defer srv.Close()
	srv.SetMode(modeOK)

	policy := resilium.New(
		resilium.WithRateLimit(20, 2),
	)

	ctx := context.Background()
	op := func(ctx context.Context) (struct{}, error) {
		if err := requireHTTPOK(ctx, srv.URL("/")); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	}

	const attempts = 8
	var limited int
	for i := 0; i < attempts; i++ {
		_, err := resilium.Execute(ctx, policy, op)
		if errors.Is(err, resilium.ErrRateLimited) {
			limited++
		} else if err != nil {
			t.Fatalf("attempt %d: unexpected error %v", i, err)
		}
	}

	if limited == 0 {
		t.Fatal("expected at least one ErrRateLimited in burst exhaustion")
	}
	if hits := srv.HitCount(); hits >= int64(attempts) {
		t.Fatalf("server hits = %d, want fewer than %d attempts due to rate limiting", hits, attempts)
	}

	time.Sleep(150 * time.Millisecond)

	if _, err := resilium.Execute(ctx, policy, op); err != nil {
		t.Fatalf("after refill: %v", err)
	}
}

func TestIntegrationCombinedPolicyStackChaos(t *testing.T) {
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 3*time.Second {
			t.Skip("insufficient test deadline for chaos integration test")
		}
	}

	srv := newSwitchableServer()
	defer srv.Close()
	srv.SetMode(modeChaos)

	policy := resilium.New(
		resilium.WithRateLimit(200, 20),
		resilium.WithTimeout(150*time.Millisecond),
		resilium.WithRetry(retry.Config{
			MaxAttempts: 4,
			Backoff:     retry.FixedBackoff(2 * time.Millisecond),
		}),
		resilium.WithCircuitBreaker(circuitbreaker.Config{
			Name:             "chaos-api",
			FailureThreshold: 0.6,
			MinRequests:      5,
			OpenDuration:     50 * time.Millisecond,
			WindowSize:       10,
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	const calls = 40
	var successes atomic.Int32
	for i := 0; i < calls; i++ {
		_, err := resilium.Execute(ctx, policy, func(ctx context.Context) (string, error) {
			code, err := httpGet(ctx, srv.URL("/"))
			if err != nil {
				return "", err
			}
			if code != http.StatusOK {
				return "", errFromHTTPStatus(code)
			}
			return "ok", nil
		})
		if err == nil {
			successes.Add(1)
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			t.Fatalf("test context expired at call %d: %v", i, err)
		}
	}

	if got := successes.Load(); got < 10 {
		t.Fatalf("successes = %d across %d calls, want substantial success despite chaos", got, calls)
	}
}
