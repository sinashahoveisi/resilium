# Roadmap

## Phase 1 — MVP
- [x] `retry.Do` with fixed, linear, and exponential backoff (+ jitter)
- [x] `circuitbreaker.Do` with closed/open/half-open state machine
- [x] Core `resilium.New` / `Execute` composition (retry + circuit breaker)
- [x] Unit tests with >90% coverage on `retry` and `circuitbreaker`
- [x] Benchmarks vs. sony/gobreaker and avast/retry-go

## Phase 2 — Differentiation
- [x] `WithTimeout` and `WithRateLimit` middlewares
- [x] Deterministic, documented policy composition order
- [x] `resilium/otel` submodule for OpenTelemetry metrics
- [x] `WithHooks` / `WithLogger` wired through all middlewares
- [x] Policy-order documentation with worked examples

## Phase 3 — Production readiness
- [x] Full godoc coverage on all exported identifiers
- [x] `docs/` guide: choosing thresholds, common pitfalls
- [x] golangci-lint clean, CI matrix on Go 1.22 and 1.23
- [ ] v1.0.0 API freeze and semver commitment
- [ ] Announcement post with benchmark comparisons

## Under consideration
- Bulkhead / concurrency-limiting policy
- Hedged requests (fire a backup request after a delay)
- Adapter for `net/http.RoundTripper`
