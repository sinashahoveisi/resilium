package circuitbreaker

import "time"

// window tracks rolling request outcomes used to decide state
// transitions. Kept separate from CircuitBreaker for testability.
//
// TODO: implement a fixed-size or time-bucketed rolling window of
// success/failure counts.
type window struct {
	successes int
	failures  int
}

func (w *window) total() int {
	return w.successes + w.failures
}

func (w *window) failureRatio() float64 {
	if w.total() == 0 {
		return 0
	}
	return float64(w.failures) / float64(w.total())
}

func (w *window) reset() {
	w.successes = 0
	w.failures = 0
}

// transition holds bookkeeping for when the breaker last changed state,
// used to know when an open breaker should move to half-open.
//
// TODO: wire this into CircuitBreaker.
type transition struct {
	state    State
	sinceAt  time.Time
}
