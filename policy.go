package resilium

import (
	"log/slog"
	"time"

	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/retry"
)

// WithRetry adds retry behavior to the policy using the given config.
//
// TODO: implement — wrap the operation so failures are retried according
// to cfg.MaxAttempts and cfg.Backoff, respecting context cancellation.
func WithRetry(cfg retry.Config) Option {
	return func(p *Policy) {
		// placeholder
	}
}

// WithCircuitBreaker adds circuit-breaking behavior to the policy.
//
// TODO: implement — short-circuit calls once the failure threshold in
// cfg is exceeded, and transition through closed/open/half-open states.
func WithCircuitBreaker(cfg circuitbreaker.Config) Option {
	return func(p *Policy) {
		// placeholder
	}
}

// WithTimeout bounds the total execution time of the wrapped operation.
//
// TODO: implement — derive a context.WithTimeout(ctx, d) around the call.
func WithTimeout(d time.Duration) Option {
	return func(p *Policy) {
		// placeholder
	}
}

// WithRateLimit bounds how often the wrapped operation may run.
//
// TODO: implement — likely backed by golang.org/x/time/rate or an
// internal token-bucket implementation to keep the core module
// dependency-free.
func WithRateLimit(requestsPerSecond float64) Option {
	return func(p *Policy) {
		// placeholder
	}
}

// WithLogger attaches structured logging to policy events (retries,
// circuit state transitions, timeouts).
//
// TODO: implement — store logger on Policy and call it from each
// middleware's decision points.
func WithLogger(logger *slog.Logger) Option {
	return func(p *Policy) {
		// placeholder
	}
}

// Hooks lets callers observe policy events without wiring a full logger
// or metrics backend.
type Hooks struct {
	OnRetry       func(attempt int, err error)
	OnCircuitOpen func(name string)
	OnCircuitClose func(name string)
	OnTimeout     func()
}

// WithHooks attaches the given hooks to the policy.
//
// TODO: implement — invoke the relevant hook from each middleware.
func WithHooks(h Hooks) Option {
	return func(p *Policy) {
		// placeholder
	}
}
