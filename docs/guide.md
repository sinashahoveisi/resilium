# Choosing resilience thresholds

Practical guidance for tuning resilium policies. For middleware ordering,
see [policy-order.md](./policy-order.md).

## Circuit breaker: FailureThreshold, MinRequests, WindowSize

The breaker trips when **at least `MinRequests` outcomes** are in the sliding
window **and** `failures / total >= FailureThreshold`. Only the last
`WindowSize` outcomes count (default 20 if unset in `circuitbreaker.New`).

### Low-volume internal service (~10 req/min)

A nightly sync job or admin API with sporadic traffic.

```go
circuitbreaker.Config{
    FailureThreshold: 0.5,
    MinRequests:      3,
    WindowSize:       10,
    OpenDuration:     30 * time.Second,
}
```

With low volume, use a **small window** (10) and **low MinRequests** (3) so a
short burst of failures (e.g. 2 of 3) trips quickly. A 50% threshold avoids
opening on a single flaky call while still reacting within a few failures.

### High-volume public API (~1,000 req/s)

User-facing HTTP handlers where you want to shed load without overreacting to
noise.

```go
circuitbreaker.Config{
    FailureThreshold: 0.3,
    MinRequests:      50,
    WindowSize:       100,
    OpenDuration:     15 * time.Second,
}
```

Use a **larger window** (100) and **higher MinRequests** (50) so momentary blips
(5–10 failures in 100 calls) stay below 30%. `OpenDuration` can be shorter
because traffic will exercise half-open quickly.

### Bursty batch job (spikes every hour)

Workers that process batches; failures during a batch should trip, but idle
periods should not retain stale success history forever.

```go
circuitbreaker.Config{
    FailureThreshold: 0.5,
    MinRequests:      10,
    WindowSize:       20, // default
    OpenDuration:     60 * time.Second,
}
```

`WindowSize: 20` ensures a healthy batch's successes do not dilute failures
when the next batch starts failing — only the last 20 outcomes matter. Pair
with retry outermost if each batch item is retried individually.

---

## Retry: MaxAttempts and backoff

### Transient network blips (timeouts, connection reset)

The dependency is usually healthy; failures are short-lived.

```go
retry.Config{
    MaxAttempts: 4,
    Backoff:     retry.WithJitter(retry.ExponentialBackoff(50*time.Millisecond, 2*time.Second), 0.2),
}
```

Use **moderate MaxAttempts** (3–5), **exponential backoff with jitter**, and a
cap so total wait stays under your outer timeout. Jitter spreads retries when
many clients fail together.

### Downstream service is down (503, connection refused)

Retrying aggressively wastes resources and keeps the circuit closed longer.

```go
retry.Config{
    MaxAttempts: 2,
    Backoff:     retry.FixedBackoff(100 * time.Millisecond),
    RetryIf: func(err error) bool {
        // Example: only retry clearly transient errors; skip permanent ones.
        return !errors.Is(err, context.DeadlineExceeded)
    },
}
```

Use **few attempts** (2–3) and **RetryIf** to skip permanent errors. Combine
with **circuit breaker innermost** so each attempt is recorded and the breaker
opens after sustained per-attempt failures (see [policy-order.md](./policy-order.md)).

### Fixed deadline per call (RPC timeout)

Match retry budget to upstream timeout; see policy-order for **timeout
innermost** vs **outermost**.

---

## Rate limiter: burst sizing

`WithRateLimit(requestsPerSecond, burst)` uses a token bucket: sustained rate
is `requestsPerSecond`, and up to `burst` calls can succeed in a short spike
before rejections (`ErrRateLimited`, fail-fast, no blocking).

| Traffic pattern | Suggested starting point |
| ---------------- | ------------------------ |
| Steady API (~100 rps) | `WithRateLimit(100, 100)` — burst ≈ 1 s of traffic |
| Mostly steady with occasional spikes | `WithRateLimit(50, 10)` — allow 10 rapid calls, then 50/s sustained |
| Strict protection (avoid any burst) | `WithRateLimit(100, 1)` — ~10 ms minimum spacing at 100 rps |

Place **RateLimit outermost** so one logical `Execute` consumes one token, not
one per retry attempt. See [policy-order.md §4](./policy-order.md#4-ratelimit-outermost--retry-avoid-retry-bursts).

---

## Common pitfalls

### Retry + circuit breaker: why ErrCircuitOpen is not retried by default

`WithRetry` installs a default `RetryIf` that returns **false** for
`ErrCircuitOpen` unless you provide your own `RetryIf`. Without this, an open
breaker would cause retry to sleep through every backoff cycle until
`MaxAttempts` is exhausted — adding latency without calling the dependency.
If you override `RetryIf`, preserve the open-circuit check unless you intend
otherwise.

### WithTimeout outermost vs innermost

- **Outermost** (`New(WithTimeout, WithRetry, …)`): one deadline covers all
  attempts and backoff waits. Use for **total caller SLA**.
- **Innermost** (`New(WithRetry, WithTimeout)`): each attempt gets a fresh
  timeout. Total time can reach `MaxAttempts × timeout + backoff`. Use for
  **per-request downstream deadlines**.

See [policy-order.md §1 and §2](./policy-order.md) for worked examples.

### Rate limit innermost with retry

`New(WithRetry, WithRateLimit)` runs `Allow()` on **every retry attempt**, so
one logical operation can consume multiple tokens. Usually wrong unless you
deliberately want stricter per-attempt throttling.

### Hooks: OnRetry timing

`OnRetry` fires **before backoff**, only when another attempt **will** follow
(not on the final failed attempt). Dashboards measuring "time until retry"
should account for backoff happening after the hook.

### Sharing circuit breakers across policies

Each `WithCircuitBreaker` creates a **new** breaker per `Policy`. To share state
across call sites, use `circuitbreaker.Do` with a shared `*CircuitBreaker`
instead of the policy middleware.

---

## Further reading

- [Policy execution order](./policy-order.md) — middleware ordering with examples
- [v1-readiness.md](./v1-readiness.md) — API stability assessment before tagging v1.0.0
