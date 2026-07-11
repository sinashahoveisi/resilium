package retry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	t.Parallel()

	errTransient := errors.New("transient")
	errPermanent := errors.New("permanent")

	tests := []struct {
		name      string
		cfg       Config
		op        func(t *testing.T) func(context.Context) (string, error)
		want      string
		wantErr   error
		wantIs    error
		errSubstr string
	}{
		{
			name: "succeeds on first try",
			cfg:  Config{MaxAttempts: 3},
			op: func(t *testing.T) func(context.Context) (string, error) {
				return func(ctx context.Context) (string, error) {
					return "ok", nil
				}
			},
			want: "ok",
		},
		{
			name: "succeeds after retries",
			cfg: Config{
				MaxAttempts: 4,
				Backoff:     FixedBackoff(time.Millisecond),
			},
			op: func(t *testing.T) func(context.Context) (string, error) {
				var calls atomic.Int32
				return func(ctx context.Context) (string, error) {
					if calls.Add(1) < 3 {
						return "", errTransient
					}
					return "recovered", nil
				}
			},
			want: "recovered",
		},
		{
			name: "exhausts attempts and wraps error",
			cfg: Config{
				MaxAttempts: 3,
				Backoff:     FixedBackoff(time.Millisecond),
			},
			op: func(t *testing.T) func(context.Context) (string, error) {
				return func(ctx context.Context) (string, error) {
					return "", errTransient
				}
			},
			wantIs:    ErrMaxAttemptsExceeded,
			errSubstr: "transient",
		},
		{
			name: "respects RetryIf and returns immediately",
			cfg: Config{
				MaxAttempts: 5,
				Backoff:     FixedBackoff(time.Millisecond),
				RetryIf:     func(err error) bool { return errors.Is(err, errTransient) },
			},
			op: func(t *testing.T) func(context.Context) (string, error) {
				var calls atomic.Int32
				return func(ctx context.Context) (string, error) {
					calls.Add(1)
					return "", errPermanent
				}
			},
			wantErr: errPermanent,
		},
		{
			name: "respects context cancellation before attempt",
			cfg: Config{
				MaxAttempts: 3,
				Backoff:     FixedBackoff(time.Millisecond),
			},
			op: func(t *testing.T) func(context.Context) (string, error) {
				return func(ctx context.Context) (string, error) {
					return "", errTransient
				}
			},
			wantErr: context.Canceled,
		},
		{
			name: "respects context cancellation mid-backoff",
			cfg: Config{
				MaxAttempts: 3,
				Backoff:     FixedBackoff(50 * time.Millisecond),
			},
			op: func(t *testing.T) func(context.Context) (string, error) {
				var calls atomic.Int32
				return func(ctx context.Context) (string, error) {
					if calls.Add(1) == 1 {
						return "", errTransient
					}
					return "unexpected", nil
				}
			},
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.wantErr == context.Canceled {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}
			if tt.wantErr == context.DeadlineExceeded {
				dctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
				ctx = dctx
			}

			got, err := Do(ctx, tt.cfg, tt.op(t))

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Do() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("Do() error = %v, want errors.Is(..., %v)", err, tt.wantIs)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("Do() error = %v, want substring %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Do() unexpected error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Do() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExponentialBackoff(t *testing.T) {
	t.Parallel()

	fn := ExponentialBackoff(100*time.Millisecond, 800*time.Millisecond)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 100 * time.Millisecond},
		{attempt: 2, want: 200 * time.Millisecond},
		{attempt: 3, want: 400 * time.Millisecond},
		{attempt: 4, want: 800 * time.Millisecond},
		{attempt: 5, want: 800 * time.Millisecond},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			t.Parallel()
			if got := fn(tt.attempt); got != tt.want {
				t.Fatalf("ExponentialBackoff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinearBackoff(t *testing.T) {
	t.Parallel()

	fn := LinearBackoff(50*time.Millisecond, 200*time.Millisecond)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 50 * time.Millisecond},
		{attempt: 2, want: 100 * time.Millisecond},
		{attempt: 3, want: 150 * time.Millisecond},
		{attempt: 4, want: 200 * time.Millisecond},
		{attempt: 10, want: 200 * time.Millisecond},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			t.Parallel()
			if got := fn(tt.attempt); got != tt.want {
				t.Fatalf("LinearBackoff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithJitter(t *testing.T) {
	t.Parallel()

	base := FixedBackoff(100 * time.Millisecond)
	fn := WithJitter(base, 0.5)

	for attempt := 1; attempt <= 3; attempt++ {
		for i := 0; i < 20; i++ {
			got := fn(attempt)
			if got < 50*time.Millisecond || got > 150*time.Millisecond {
				t.Fatalf("WithJitter() = %v, want within [50ms, 150ms]", got)
			}
		}
	}

	t.Run("zero fraction returns base delay", func(t *testing.T) {
		t.Parallel()
		fn := WithJitter(base, 0)
		if got := fn(1); got != 100*time.Millisecond {
			t.Fatalf("WithJitter(0) = %v, want 100ms", got)
		}
	})

	t.Run("negative jitter result clamped to zero", func(t *testing.T) {
		t.Parallel()
		alwaysMax := func(attempt int) time.Duration { return 1 * time.Nanosecond }
		fn := WithJitter(alwaysMax, 1.0)
		for i := 0; i < 50; i++ {
			if got := fn(1); got < 0 {
				t.Fatalf("WithJitter() returned negative duration: %v", got)
			}
		}
	})
}

func TestBackoffEdgeAttempts(t *testing.T) {
	t.Parallel()

	if got := ExponentialBackoff(time.Millisecond, time.Second)(0); got != time.Millisecond {
		t.Fatalf("ExponentialBackoff attempt 0 = %v, want 1ms", got)
	}
	if got := LinearBackoff(time.Millisecond, time.Second)(0); got != time.Millisecond {
		t.Fatalf("LinearBackoff attempt 0 = %v, want 1ms", got)
	}
}

func TestDoMaxAttemptsMinimum(t *testing.T) {
	t.Parallel()

	_, err := Do(context.Background(), Config{MaxAttempts: 0}, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, errors.New("fail")
	})
	if !errors.Is(err, ErrMaxAttemptsExceeded) {
		t.Fatalf("Do() error = %v, want ErrMaxAttemptsExceeded", err)
	}
}

func TestDefaultBackoffWhenNil(t *testing.T) {
	t.Parallel()

	start := time.Now()
	var calls atomic.Int32

	_, err := Do(context.Background(), Config{MaxAttempts: 2}, func(ctx context.Context) (struct{}, error) {
		if calls.Add(1) == 1 {
			return struct{}{}, errors.New("fail")
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 90*time.Millisecond {
		t.Fatalf("default backoff too short: %v", elapsed)
	}
}
