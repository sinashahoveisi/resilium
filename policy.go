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
func WithCircuitBreaker(cfg circuitbreaker.Config) Option {
	return func(p *Policy) {
		cb := circuitbreaker.New(cfg)
		p.middlewares = append(p.middlewares, func(next OperationFunc) OperationFunc {
			return func(ctx context.Context) (any, error) {
				before := cb.State()
				result, err := circuitbreaker.Do(ctx, cb, func(ctx context.Context) (any, error) {
					return next(ctx)
				})
				after := cb.State()
				if before != circuitbreaker.StateOpen && after == circuitbreaker.StateOpen {
					p.onCircuitOpen("")
				}
				if before != circuitbreaker.StateClosed && after == circuitbreaker.StateClosed {
					p.onCircuitClose("")
				}
				if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
					return nil, fmt.Errorf("%w", ErrCircuitOpen)
				}
				return result, err
			}
		})
	}
}

// WithTimeout bounds the total execution time of the wrapped operation.
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
					if p.logger != nil {
						p.logger.Warn("resilium rate limited")
					}
					return nil, ErrRateLimited
				}
				return next(ctx)
			}
		})
	}
}

// WithLogger attaches structured logging to policy events (retries,
// circuit state transitions, timeouts).
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
// or metrics backend.
type Hooks struct {
	OnRetry        func(attempt int, err error)
	OnCircuitOpen  func(name string)
	OnCircuitClose func(name string)
	OnTimeout      func()
	OnRateLimited  func()
}

// WithHooks attaches the given hooks to the policy.
func WithHooks(h Hooks) Option {
	return func(p *Policy) {
		p.hooks = mergeHooks(p.hooks, h)
	}
}
