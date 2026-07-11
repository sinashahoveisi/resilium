package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	t.Parallel()

	errFail := errors.New("dependency failed")

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "stays closed under threshold",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      4,
					OpenDuration:     10 * time.Millisecond,
				})

				for i := 0; i < 3; i++ {
					if _, err := Do(context.Background(), cb, succeed[string]); err != nil {
						t.Fatalf("unexpected error on success: %v", err)
					}
				}
				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				if state := cb.State(); state != StateClosed {
					t.Fatalf("State() = %v, want closed", state)
				}
			},
		},
		{
			name: "opens after threshold breach",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      2,
					OpenDuration:     10 * time.Millisecond,
				})

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}
				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				if state := cb.State(); state != StateOpen {
					t.Fatalf("State() = %v, want open", state)
				}
			},
		},
		{
			name: "rejects calls while open",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      1,
					OpenDuration:     50 * time.Millisecond,
				})

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				_, err := Do(context.Background(), cb, succeed[string])
				if !errors.Is(err, ErrCircuitOpen) {
					t.Fatalf("Do() error = %v, want ErrCircuitOpen", err)
				}
			},
		},
		{
			name: "transitions to half-open after OpenDuration",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      1,
					OpenDuration:     15 * time.Millisecond,
				})

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				time.Sleep(20 * time.Millisecond)

				if state := cb.State(); state != StateHalfOpen {
					t.Fatalf("State() = %v, want half-open", state)
				}
			},
		},
		{
			name: "closes again on successful half-open trial",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      1,
					OpenDuration:     10 * time.Millisecond,
				})

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				time.Sleep(15 * time.Millisecond)

				if _, err := Do(context.Background(), cb, succeed[string]); err != nil {
					t.Fatalf("half-open trial failed: %v", err)
				}

				if state := cb.State(); state != StateClosed {
					t.Fatalf("State() = %v, want closed", state)
				}

				got, err := Do(context.Background(), cb, succeed[string])
				if err != nil || got != "ok" {
					t.Fatalf("call after close: got (%q, %v)", got, err)
				}
			},
		},
		{
			name: "reopens on failed half-open trial",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      1,
					OpenDuration:     10 * time.Millisecond,
				})

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				time.Sleep(15 * time.Millisecond)

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected half-open trial failure")
				}

				if state := cb.State(); state != StateOpen {
					t.Fatalf("State() = %v, want open", state)
				}

				_, err := Do(context.Background(), cb, succeed[string])
				if !errors.Is(err, ErrCircuitOpen) {
					t.Fatalf("Do() error = %v, want ErrCircuitOpen", err)
				}
			},
		},
		{
			name: "allows only one half-open trial at a time",
			run: func(t *testing.T) {
				cb := New(Config{
					FailureThreshold: 0.5,
					MinRequests:      1,
					OpenDuration:     10 * time.Millisecond,
				})

				if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
					t.Fatal("expected failure")
				}

				time.Sleep(15 * time.Millisecond)

				trialInFlight := make(chan struct{})
				releaseTrial := make(chan struct{})
				secondDone := make(chan error, 1)

				go func() {
					_, err := Do(context.Background(), cb, func(ctx context.Context) (string, error) {
						close(trialInFlight)
						<-releaseTrial
						return "ok", nil
					})
					if err != nil {
						t.Errorf("trial goroutine error = %v", err)
					}
				}()

				go func() {
					<-trialInFlight
					_, err := Do(context.Background(), cb, succeed[string])
					secondDone <- err
				}()

				err := <-secondDone
				if !errors.Is(err, ErrCircuitOpen) {
					t.Fatalf("second goroutine error = %v, want ErrCircuitOpen", err)
				}

				close(releaseTrial)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func succeed[T any](ctx context.Context) (T, error) {
	var zero T
	if any(zero) == any("") {
		return any("ok").(T), nil
	}
	return zero, nil
}

func fail[T any](err error) func(context.Context) (T, error) {
	return func(ctx context.Context) (T, error) {
		var zero T
		return zero, err
	}
}

func TestStateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Fatalf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestWindow(t *testing.T) {
	t.Parallel()

	w := newWindow(3)
	if w.total() != 0 || w.failureRatio() != 0 {
		t.Fatal("empty window should have zero totals")
	}

	w.recordSuccess()
	w.recordSuccess()
	w.recordFailure()

	if w.total() != 3 {
		t.Fatalf("total() = %d, want 3", w.total())
	}
	if ratio := w.failureRatio(); ratio < 0.333 || ratio > 0.334 {
		t.Fatalf("failureRatio() = %v, want ~0.333", ratio)
	}

	// Fourth outcome evicts the oldest (success); window becomes success, fail, fail.
	w.recordFailure()
	if w.total() != 3 {
		t.Fatalf("after eviction total() = %d, want 3", w.total())
	}
	if ratio := w.failureRatio(); ratio < 0.666 || ratio > 0.667 {
		t.Fatalf("after eviction failureRatio() = %v, want ~0.667", ratio)
	}

	w.reset()
	if w.total() != 0 {
		t.Fatalf("after reset total() = %d, want 0", w.total())
	}
}

func TestSlidingWindowRegression(t *testing.T) {
	t.Parallel()

	const windowSize = 20
	errFail := errors.New("dependency failed")

	cb := New(Config{
		FailureThreshold: 0.5,
		MinRequests:      10,
		WindowSize:       windowSize,
		OpenDuration:     time.Second,
	})

	for i := 0; i < windowSize+1000; i++ {
		if _, err := Do(context.Background(), cb, succeed[string]); err != nil {
			t.Fatalf("success %d: %v", i, err)
		}
	}

	failuresBeforeOpen := 0
	for i := 0; i < windowSize+1; i++ {
		if _, err := Do(context.Background(), cb, fail[string](errFail)); err == nil {
			t.Fatal("expected failure")
		}
		failuresBeforeOpen++
		if cb.State() == StateOpen {
			break
		}
	}

	if cb.State() != StateOpen {
		t.Fatal("breaker should have opened")
	}
	if failuresBeforeOpen > windowSize {
		t.Fatalf("opened after %d failures, want at most %d (sliding window)", failuresBeforeOpen, windowSize)
	}
	if failuresBeforeOpen != 10 {
		t.Fatalf("opened after %d failures, want 10 for threshold 0.5 with min 10 in a full window", failuresBeforeOpen)
	}
}
