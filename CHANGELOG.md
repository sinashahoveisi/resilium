# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](./docs/versioning.md).

## [1.0.0] - 2026-07-12

First stable release. API is now covered by the [versioning policy](./docs/versioning.md).

### Added

**Core**
- `resilium.New` / `resilium.Execute[T]` — composable, generics-first policy execution with type-safe results.
- `resilium.Policy` and functional options (`Option`) for building composable middleware stacks.

**Retry**
- `retry.Do` with configurable `MaxAttempts`, `RetryIf` filtering, and context-aware backoff waits.
- `retry.FixedBackoff`, `retry.ExponentialBackoff`, `retry.LinearBackoff`, `retry.WithJitter`.
- `resilium.WithRetry` — automatically skips retrying `ErrCircuitOpen` unless a custom `RetryIf` is supplied.

**Circuit breaker**
- `circuitbreaker.Do` and `circuitbreaker.CircuitBreaker` with a closed → open → half-open → closed state machine.
- Sliding-window failure tracking (`Config.WindowSize`, default 20) instead of cumulative all-time counters.
- `circuitbreaker.Config.Name` — optional identifier surfaced in hooks and logs for distinguishing multiple breakers.
- `resilium.WithCircuitBreaker`.

**Timeout & rate limiting**
- `resilium.WithTimeout` — bounds operation time; maps `context.DeadlineExceeded` to `ErrTimeout` without masking `context.Canceled`.
- `resilium.WithRateLimit(requestsPerSecond, burst)` — fail-fast token bucket, never blocks the caller.

**Observability**
- `resilium.Hooks` (`OnRetry`, `OnCircuitOpen`, `OnCircuitClose`, `OnTimeout`, `OnRateLimited`) and `resilium.WithHooks`.
- `resilium.WithLogger` — structured `log/slog` logging for all policy events.
- `resilium/otel` submodule — OpenTelemetry metrics adapter (`otelresilium.Metrics`), kept as a separate Go module so the core stays dependency-free.

**Documentation**
- [`docs/policy-order.md`](./docs/policy-order.md) — worked examples of middleware composition order.
- [`docs/guide.md`](./docs/guide.md) — threshold tuning guidance and common pitfalls.
- [`docs/versioning.md`](./docs/versioning.md) — semantic versioning policy.
- [`docs/v1-readiness.md`](./docs/v1-readiness.md) — API stability assessment.
- Full godoc coverage on all exported identifiers.

**Testing**
- Unit tests across `resilium`, `retry`, `circuitbreaker`, and `internal/ratelimit` (90%+ coverage on core packages).
- `integration/` — end-to-end tests against a real HTTP server: intermittent failures, sustained outages tripping the breaker, slow-server timeouts, breaker recovery, rate-limit burst/refill, and combined-policy chaos scenarios.
- Goroutine-leak and concurrent-load sanity checks.
- `benchmarks/` — isolated module comparing resilium against `sony/gobreaker` and `avast/retry-go`.

### Notes

- Minimum Go version: 1.22 (generics, `log/slog`). CI tests Go 1.22 and 1.23.
- `internal/` and `benchmarks/` are not covered by semver — see [`docs/versioning.md`](./docs/versioning.md).
- `resilium/otel` versions independently from the root module.