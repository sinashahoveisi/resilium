# Policy execution order

resilium applies middleware in the order you pass options to `New`: the **first** option is the **outermost** wrapper, the **last** is **innermost** (closest to your operation). `Execute` builds the chain by wrapping from innermost outward:

```go
// New(WithA, WithB, WithC)  →  A(B(C(op)))
```

This document walks through four common orderings with code and reasoning.

---

## 1. Timeout outermost + Retry + CircuitBreaker innermost (recommended default)

```go
policy := resilium.New(
    resilium.WithTimeout(5*time.Second),
    resilium.WithRetry(retry.Config{MaxAttempts: 3, Backoff: retry.FixedBackoff(100*time.Millisecond)}),
    resilium.WithCircuitBreaker(circuitbreaker.Config{
        FailureThreshold: 0.5,
        MinRequests:      10,
        OpenDuration:     30 * time.Second,
    }),
)

result, err := resilium.Execute(ctx, policy, fetchUser)
```

**What happens:** A single 5-second budget covers the entire execution, including every retry attempt and backoff wait. Each retry attempt passes through the circuit breaker individually, so repeated failures on a sick dependency fill the breaker's sliding window and can open the circuit mid-retry sequence. Use this when you care about **total wall-clock time** for the caller (HTTP handler deadline, user-facing SLA) and want the breaker to react to per-attempt failures.

---

## 2. Retry outermost + Timeout innermost (per-attempt timeout)

```go
policy := resilium.New(
    resilium.WithRetry(retry.Config{MaxAttempts: 3, Backoff: retry.FixedBackoff(100*time.Millisecond)}),
    resilium.WithTimeout(2*time.Second),
)

result, err := resilium.Execute(ctx, policy, callSlowAPI)
```

**What happens:** Each individual attempt gets its own 2-second timeout. A hung call is cut off after 2s, then retry may run another 2s attempt, and so on — total time can be up to roughly `(MaxAttempts × timeout) + backoff`. Use this when each downstream call should respect a **per-request** deadline (e.g. matching an upstream RPC timeout) rather than a single budget for the whole retry sequence.

---

## 3. CircuitBreaker outermost + Retry innermost

```go
policy := resilium.New(
    resilium.WithCircuitBreaker(circuitbreaker.Config{
        FailureThreshold: 0.5,
        MinRequests:      5,
        OpenDuration:     30 * time.Second,
    }),
    resilium.WithRetry(retry.Config{MaxAttempts: 3, Backoff: retry.FixedBackoff(50*time.Millisecond)}),
)

result, err := resilium.Execute(ctx, policy, fetchUser)
```

**What happens:** The circuit breaker sees **one outcome per `Execute` call** — the final result after all retries succeed or fail. Internal retry attempts are invisible to the breaker; only the ultimate success or failure is recorded. Contrast with §1, where the breaker is innermost and records **every attempt**. Use breaker-outermost when retries are an implementation detail and you only want to trip the breaker on sustained end-to-end failures, not on transient blips that retries absorb.

---

## 4. RateLimit outermost + Retry (avoid retry bursts)

```go
policy := resilium.New(
    resilium.WithRateLimit(10, 10), // 10 req/s sustained, burst up to 10; fail-fast when empty
    resilium.WithRetry(retry.Config{MaxAttempts: 3, Backoff: retry.FixedBackoff(100*time.Millisecond)}),
    resilium.WithCircuitBreaker(cbConfig),
)

result, err := resilium.Execute(ctx, policy, fetchUser)
```

**What happens:** Rate limiting runs once at the start of `Execute` when outermost. If no token is available, the call returns `ErrRateLimited` immediately without invoking retry or the operation. Put **RateLimit outermost** so retries do not each consume another token — otherwise a single logical request that retries three times could consume three tokens and create a burst that violates your intended steady-state rate. If RateLimit is **innermost** (`New(WithRetry(...), WithRateLimit(...))`), `Allow()` runs on **every retry attempt**, so one `Execute` can consume up to `MaxAttempts` tokens — stricter than a single outer check, not a bypass.

---

## Quick reference

| Outermost → innermost | Total timeout budget | CB sees each attempt? | Rate limit per Execute |
| --------------------- | -------------------- | --------------------- | ---------------------- |
| Timeout → Retry → CB  | One shared budget    | Yes                   | One check (if outer)   |
| Retry → Timeout → op  | Per attempt          | Depends on CB position | One check (if outer)  |
| CB → Retry → op       | Depends on Timeout   | No (final outcome)    | One check (if outer)   |
| RateLimit → Retry → … | Depends on Timeout   | Depends on CB position | One check              |

See also the [README policy execution order section](../README.md#policy-execution-order).
