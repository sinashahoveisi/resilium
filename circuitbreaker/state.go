package circuitbreaker

import "time"

const defaultWindowSize = 20

// window tracks the most recent request outcomes in a fixed-size sliding
// ring buffer. Only the last WindowSize results are considered when
// computing the failure ratio; older outcomes are discarded as new ones
// arrive. The window is reset when the breaker closes after a successful
// half-open trial.
type window struct {
	buf      []bool // true = success, false = failure
	next     int    // index of the next slot to write
	count    int    // number of filled slots (0..len(buf))
	failures int    // failures among the filled slots
}

func newWindow(size int) window {
	return window{buf: make([]bool, size)}
}

func (w *window) total() int {
	return w.count
}

func (w *window) failureRatio() float64 {
	if w.count == 0 {
		return 0
	}
	return float64(w.failures) / float64(w.count)
}

func (w *window) reset() {
	w.next = 0
	w.count = 0
	w.failures = 0
}

func (w *window) recordSuccess() {
	w.record(true)
}

func (w *window) recordFailure() {
	w.record(false)
}

func (w *window) record(success bool) {
	if len(w.buf) == 0 {
		return
	}
	if w.count == len(w.buf) {
		if !w.buf[w.next] {
			w.failures--
		}
	} else {
		w.count++
	}
	w.buf[w.next] = success
	if !success {
		w.failures++
	}
	w.next = (w.next + 1) % len(w.buf)
}

// transition holds bookkeeping for when the breaker last changed state,
// used to know when an open breaker should move to half-open.
type transition struct {
	state   State
	sinceAt time.Time
}
