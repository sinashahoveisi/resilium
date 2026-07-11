package resilium

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/internal/ratelimit"
	"github.com/sinashahoveisi/resilium/retry"
)

// WithRetry adds retry behavior to the policy using the given config.
// When RetryIf is nil, retries stop immediately on ErrCircuitOpen so an
// open circuit breaker is not hammered through backoff cycles. Exhausted
// retries return ErrMaxAttemptsExceeded wrapping the last error.
func WithRetry(cfg retry.Config) Option {
	return func(p *Policy) {
		retryCfg := cfg
		if retryCfg.RetryIf == nil {
			retryCfg.RetryIf = func(err error) bool {
				if errors.Is(err, ErrCircuitOpen) || errors.Is(err, circuitbreaker.ErrCircuitOpen) {
					return false
				}
				return true
			}
		}
		maxAttempts := retryCfg.MaxAttempts
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		p.middlewares = append(p.middlewares, func(next OperationFunc) OperationFunc {
			return func(ctx context.Context) (any, error) {
				var attempt int
				result, err := retry.Do(ctx, retryCfg, func(ctx context.Context) (any, error) {
					r, e := next(ctx)
					if e != nil {
						attempt++
						shouldRetry := retryCfg.RetryIf == nil || retryCfg.RetryIf(e)
						if shouldRetry && attempt < maxAttempts {
							p.onRetry(attempt, e)
						}
					}
					return r, e
				})
				if errors.Is(err, retry.ErrMaxAttemptsExceeded) {
					return nil, fmt.Errorf("%w: %w", ErrMaxAttemptsExceeded, err)
				}
				return result, err
			}
		})
	}
}

// WithCircuitBreaker adds circuit-breaking behavior to the policy.
// Each Policy holds one CircuitBreaker instance shared across Execute calls
// on that policy; use separate policies (or circuitbreaker.Do with a shared
// breaker) for different dependencies. Set cfg.Name to identify the breaker
// in OnCircuitOpen, OnCircuitClose, and logger output. Open calls return
// ErrCircuitOpen.
func WithCircuitBreaker(cfg circuitbreaker.Config) Option {
	return func(p *Policy) {
		cb := circuitbreaker.New(cfg)
		name := cfg.Name
		p.middlewares = append(p.middlewares, func(next OperationFunc) OperationFunc {
			return func(ctx context.Context) (any, error) {
				before := cb.State()
				result, err := circuitbreaker.Do(ctx, cb, func(ctx context.Context) (any, error) {
					return next(ctx)
				})
				after := cb.State()
				if before != circuitbreaker.StateOpen && after == circuitbreaker.StateOpen {
					p.onCircuitOpen(name)
				}
				if before != circuitbreaker.StateClosed && after == circuitbreaker.StateClosed {
					p.onCircuitClose(name)
				}
				if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
					return nil, fmt.Errorf("%w", ErrCircuitOpen)
				}
				return result, err
			}
		})
	}
}

// WithTimeout bounds execution time of the wrapped operation using
// context.WithTimeout. When the deadline is exceeded, returns
// ErrTimeout wrapping context.DeadlineExceeded. Parent context
// cancellation returns context.Canceled and is not mapped to ErrTimeout.
func WithTimeout(d time.Duration) Option {
	return func(p *Policy) {
		p.middlewares = append(p.middlewares, func(next OperationFunc) OperationFunc {
			return func(ctx context.Context) (any, error) {
				timeoutCtx, cancel := context.WithTimeout(ctx, d)
				defer cancel()

				result, err := next(timeoutCtx)
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
					p.onTimeout()
					return nil, fmt.Errorf("%w: %w", ErrTimeout, context.DeadlineExceeded)
				}
				return result, err
			}
		})
	}
}

// WithRateLimit bounds how often the wrapped operation may run using a
// token-bucket limiter. requestsPerSecond is the sustained refill rate;
// burst is the maximum number of tokens that can accumulate (allowing
// short bursts without rejecting). A typical starting point is burst equal
// to requestsPerSecond (rounded up) or a small fixed value such as 5–10.
// Rejected calls return ErrRateLimited immediately without blocking.
func WithRateLimit(requestsPerSecond float64, burst int) Option {
	return func(p *Policy) {
		bucket := ratelimit.New(requestsPerSecond, burst)
		p.middlewares = append(p.middlewares, func(next OperationFunc) OperationFunc {
			return func(ctx context.Context) (any, error) {
				if !bucket.Allow() {
					p.onRateLimited()
					return nil, ErrRateLimited
				}
				return next(ctx)
			}
		})
	}
}

// WithLogger attaches structured logging to policy events (retries,
// circuit state transitions, timeouts, rate-limit rejections). A nil
// logger defaults to slog.Default(). Logging is implemented via Hooks
// merged with any hooks from WithHooks.
func WithLogger(logger *slog.Logger) Option {
	return func(p *Policy) {
		if logger == nil {
			logger = slog.Default()
		}
		p.logger = logger
		p.hooks = mergeHooks(p.hooks, loggerHooks(logger))
	}
}

// Hooks lets callers observe policy events without wiring a full logger
// or metrics backend. Callbacks are invoked from the middleware that
// triggers them; they must not block for long. A Policy is safe for
// concurrent Execute calls; hook implementations should be thread-safe
// if they mutate shared state.
type Hooks struct {
	// OnRetry is called when a failed attempt will be retried. attempt is
	// 1-indexed (1 = first failure that triggers a retry). It is not
	// called on the final failed attempt when no retry follows.
	OnRetry func(attempt int, err error)
	// OnCircuitOpen is called when a circuit breaker transitions to open.
	// name is circuitbreaker.Config.Name when set via WithCircuitBreaker,
	// otherwise "".
	OnCircuitOpen func(name string)
	// OnCircuitClose is called when a circuit breaker transitions to closed
	// (typically after a successful half-open trial).
	OnCircuitClose func(name string)
	// OnTimeout is called when WithTimeout detects a deadline exceeded.
	OnTimeout func()
	// OnRateLimited is called when WithRateLimit rejects a call because
	// no token was available.
	OnRateLimited func()
}

// WithHooks attaches the given hooks to the policy, chaining with any
// hooks already registered (e.g. from WithLogger). Later registrations
// run after earlier ones for the same event.
func WithHooks(h Hooks) Option {
	return func(p *Policy) {
		p.hooks = mergeHooks(p.hooks, h)
	}
}
