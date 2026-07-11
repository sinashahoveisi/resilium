package resilium

import "errors"

// ErrCircuitOpen is returned when a call is rejected because its
// circuit breaker is in the open state. Use errors.Is to test for it;
// the underlying circuitbreaker.ErrCircuitOpen may also be present in
// the error chain when using the subpackage directly.
var ErrCircuitOpen = errors.New("resilium: circuit breaker is open")

// ErrTimeout is returned when an operation exceeds its configured
// timeout before completing. errors.Is(err, context.DeadlineExceeded)
// also returns true for timeout errors wrapped by WithTimeout.
var ErrTimeout = errors.New("resilium: operation timed out")

// ErrMaxAttemptsExceeded is returned when retry attempts are exhausted
// without a successful result. The last underlying error is wrapped;
// errors.Is(err, retry.ErrMaxAttemptsExceeded) may also be true.
var ErrMaxAttemptsExceeded = errors.New("resilium: max retry attempts exceeded")

// ErrRateLimited is returned when a call is rejected by a rate-limit
// policy because no token was available. WithRateLimit never blocks
// waiting for a token.
var ErrRateLimited = errors.New("resilium: rate limit exceeded")
