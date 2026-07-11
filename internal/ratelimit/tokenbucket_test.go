package ratelimit_test

import (
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium/internal/ratelimit"
)

func TestTokenBucketAllow(t *testing.T) {
	t.Parallel()

	tb := ratelimit.New(10, 1)

	if !tb.Allow() {
		t.Fatal("first Allow() should succeed with full bucket")
	}
	if tb.Allow() {
		t.Fatal("second immediate Allow() should fail with burst 1")
	}

	time.Sleep(110 * time.Millisecond)
	if !tb.Allow() {
		t.Fatal("Allow() should succeed after refill at 10/s")
	}
}

func TestTokenBucketNonPositiveRate(t *testing.T) {
	t.Parallel()

	tb := ratelimit.New(0, 1)
	if tb.Allow() {
		t.Fatal("Allow() should fail when rate is zero")
	}
}

func TestTokenBucketNeverBlocks(t *testing.T) {
	t.Parallel()

	tb := ratelimit.New(1, 1)
	_ = tb.Allow()

	start := time.Now()
	if tb.Allow() {
		t.Fatal("expected rejection")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("Allow() blocked for %v, want immediate rejection", elapsed)
	}
}

func TestTokenBucketRefillAccuracy(t *testing.T) {
	t.Parallel()

	const rate = 500.0
	const burst = 50
	tb := ratelimit.New(rate, burst)

	start := time.Now()
	allowed := 0
	calls := 0
	for time.Since(start) < 2*time.Second {
		calls++
		if tb.Allow() {
			allowed++
		}
	}
	elapsed := time.Since(start).Seconds()

	// Steady-state allowance ≈ burst + rate×elapsed (tokens cap at burst between calls).
	expected := float64(burst) + rate*elapsed
	tolerance := expected * 0.15
	if allowed < int(expected-tolerance) || allowed > int(expected+tolerance) {
		t.Fatalf("allowed %d calls over %.2fs (%d Allow() calls), want ~%.0f ±15%%",
			allowed, elapsed, calls, expected)
	}
}

func TestTokenBucketBurstAllowsRapidSuccession(t *testing.T) {
	t.Parallel()

	tb := ratelimit.New(100, 3)
	allowed := 0
	for i := 0; i < 3; i++ {
		if tb.Allow() {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("burst 3: allowed %d rapid calls, want 3", allowed)
	}
	if tb.Allow() {
		t.Fatal("fourth immediate call should be rejected with burst 3")
	}
}
