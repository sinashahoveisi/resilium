// Package circuitbreaker implements the circuit breaker pattern as a
// resilium middleware, usable standalone as well.
//
// A CircuitBreaker is safe for concurrent use. State transitions are
// computed lazily (open→half-open) when State or Do is called.
package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when a call is rejected because the circuit
// breaker is open or a half-open trial is already in flight. Callers in
// the root resilium package can map this to resilium.ErrCircuitOpen via
// errors.Is.
var ErrCircuitOpen = errors.New("circuitbreaker: circuit open")

// Config configures a circuit breaker.
type Config struct {
	// FailureThreshold is the failure ratio (0.0–1.0) that trips the
	// breaker from closed to open, once MinRequests outcomes are in the
	// sliding window.
	FailureThreshold float64

	// MinRequests is the minimum number of outcomes in the sliding window
	// before the failure ratio is evaluated.
	MinRequests int

	// OpenDuration is how long the breaker stays open before moving to
	// half-open and allowing a trial request through.
	OpenDuration time.Duration

	// WindowSize is the number of most recent request outcomes kept in the
	// sliding window when evaluating FailureThreshold (e.g. 20–100).
	// Defaults to 20 when zero or negative.
	WindowSize int
}

// State represents the circuit breaker's current state.
type State int

const (
	// StateClosed allows requests through and tracks outcomes in the
	// sliding window.
	StateClosed State = iota
	// StateOpen rejects all requests until OpenDuration elapses.
	StateOpen
	// StateHalfOpen allows exactly one trial request; success closes the
	// breaker, failure reopens it.
	StateHalfOpen
)

// String returns a human-readable name for the state.
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
// them once failures exceed the configured threshold within the sliding
// window. Safe for concurrent use by multiple goroutines.
type CircuitBreaker struct {
	cfg                   Config
	mu                    sync.Mutex
	window                window
	transition            transition
	halfOpenTrialInFlight bool
}

// New creates a CircuitBreaker with the given config. WindowSize defaults
// to 20 when zero or negative.
func New(cfg Config) *CircuitBreaker {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = defaultWindowSize
	}
	return &CircuitBreaker{
		cfg:    cfg,
		window: newWindow(cfg.WindowSize),
		transition: transition{
			state:   StateClosed,
			sinceAt: time.Now(),
		},
	}
}

// State returns the breaker's current state, lazily transitioning from
// open to half-open when OpenDuration has elapsed.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeTransitionToHalfOpen(time.Now())
	return cb.transition.state
}

// Do runs op if the breaker allows it, recording the outcome to
// influence future state transitions. Returns ErrCircuitOpen when the
// breaker is open or when a half-open trial is already in progress.
// Returns ctx.Err() if ctx is cancelled before the call starts.
func Do[T any](ctx context.Context, cb *CircuitBreaker, op func(ctx context.Context) (T, error)) (T, error) {
	var zero T

	if err := ctx.Err(); err != nil {
		return zero, err
	}

	if err := cb.beforeCall(); err != nil {
		return zero, err
	}

	result, err := op(ctx)
	cb.afterCall(err)

	return result, err
}

func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.maybeTransitionToHalfOpen(now)

	switch cb.transition.state {
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		if cb.halfOpenTrialInFlight {
			return ErrCircuitOpen
		}
		cb.halfOpenTrialInFlight = true
		return nil
	default:
		return nil
	}
}

func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.transition.state {
	case StateHalfOpen:
		cb.halfOpenTrialInFlight = false
		if err != nil {
			cb.transition.state = StateOpen
			cb.transition.sinceAt = now
		} else {
			cb.transition.state = StateClosed
			cb.transition.sinceAt = now
			cb.window.reset()
		}
	case StateClosed:
		if err != nil {
			cb.window.recordFailure()
		} else {
			cb.window.recordSuccess()
		}
		cb.maybeTrip(now)
	}
}

func (cb *CircuitBreaker) maybeTransitionToHalfOpen(now time.Time) {
	if cb.transition.state != StateOpen {
		return
	}
	if now.Sub(cb.transition.sinceAt) >= cb.cfg.OpenDuration {
		cb.transition.state = StateHalfOpen
		cb.transition.sinceAt = now
		cb.halfOpenTrialInFlight = false
	}
}

func (cb *CircuitBreaker) maybeTrip(now time.Time) {
	if cb.transition.state != StateClosed {
		return
	}
	if cb.window.total() >= cb.cfg.MinRequests &&
		cb.window.failureRatio() >= cb.cfg.FailureThreshold {
		cb.transition.state = StateOpen
		cb.transition.sinceAt = now
	}
}
