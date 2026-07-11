// Package resilium provides composable resilience policies — retry,
// circuit breaker, timeout, and rate limiting — behind a single,
// type-safe execution API.
package resilium

import (
	"context"
	"fmt"
)

// Operation is the unit of work resilium executes. It is generic over
// the result type T so callers get their real type back, not interface{}.
type Operation[T any] func(ctx context.Context) (T, error)

// Middleware wraps an Operation with additional behavior (retry, circuit
// breaking, timeout, etc). Policies are built by composing middlewares.
type Middleware func(next OperationFunc) OperationFunc

// OperationFunc is the untyped form of Operation used internally so that
// middlewares can be composed without needing to know the result type.
type OperationFunc func(ctx context.Context) (any, error)

// Policy is an ordered, composable set of resilience behaviors.
// Construct one with New and the With* option functions.
type Policy struct {
	middlewares []Middleware
}

// Option configures a Policy when passed to New.
type Option func(*Policy)

// New builds a Policy from the given options, applied in the order
// listed. Order matters: see the README section on policy execution
// order for guidance on how to sequence retry, circuit breaker, and
// timeout middlewares.
func New(opts ...Option) *Policy {
	p := &Policy{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Execute runs op through every middleware configured on the policy and
// returns the typed result.
func Execute[T any](ctx context.Context, p *Policy, op Operation[T]) (T, error) {
	var zero T

	wrapped := OperationFunc(func(ctx context.Context) (any, error) {
		return op(ctx)
	})

	for i := len(p.middlewares) - 1; i >= 0; i-- {
		wrapped = p.middlewares[i](wrapped)
	}

	result, err := wrapped(ctx)
	if err != nil {
		return zero, err
	}

	typed, ok := result.(T)
	if !ok {
		return zero, fmt.Errorf("resilium: internal type assertion failed for %T", zero)
	}
	return typed, nil
}
