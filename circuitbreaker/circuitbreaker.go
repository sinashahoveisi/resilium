// Package circuitbreaker implements the circuit breaker pattern as a
// resilium middleware, usable standalone as well.
package circuitbreaker

import (
	"context"
	"sync"
	"time"
)

// Config configures a circuit breaker.
type Config struct {
	// FailureThreshold is the failure ratio (0.0–1.0) that trips the
	// breaker from closed to open, once MinRequests is reached.
	FailureThreshold float64

	// MinRequests is the minimum number of requests in the rolling
	// window before the failure ratio is evaluated.
	MinRequests int

	// OpenDuration is how long the breaker stays open before moving to
	// half-open and allowing a trial request through.
	OpenDuration time.Duration
}

// State represents the circuit breaker's current state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker guards calls to an unreliable dependency, rejecting
// them once failures exceed the configured threshold.
type CircuitBreaker struct {
	cfg Config
	mu  sync.Mutex
	// TODO: rolling window counters, current state, last state-change time
}

// New creates a CircuitBreaker with the given config.
func New(cfg Config) *CircuitBreaker {
	return &CircuitBreaker{cfg: cfg}
}

// State returns the breaker's current state.
//
// TODO: implement — return the real tracked state.
func (cb *CircuitBreaker) State() State {
	return StateClosed
}

// Do runs op if the breaker allows it, recording the outcome to
// influence future state transitions.
//
// TODO: implement — check state, reject with an open-circuit error if
// appropriate, otherwise run op and record success/failure.
func Do[T any](ctx context.Context, cb *CircuitBreaker, op func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	return zero, nil
}
