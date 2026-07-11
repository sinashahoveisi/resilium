package resilium

import "errors"

// ErrCircuitOpen is returned when a call is rejected because its
// circuit breaker is in the open state.
var ErrCircuitOpen = errors.New("resilium: circuit breaker is open")

// ErrTimeout is returned when an operation exceeds its configured
// timeout before completing.
var ErrTimeout = errors.New("resilium: operation timed out")

// ErrMaxAttemptsExceeded is returned when retry attempts are exhausted
// without a successful result. The last underlying error is wrapped.
var ErrMaxAttemptsExceeded = errors.New("resilium: max retry attempts exceeded")

// ErrRateLimited is returned when a call is rejected by a rate-limit
// policy because it would exceed the configured request rate.
var ErrRateLimited = errors.New("resilium: rate limit exceeded")
