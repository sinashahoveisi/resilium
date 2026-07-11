# Roadmap

## Phase 1 — MVP
- [ ] `retry.Do` with fixed, linear, and exponential backoff (+ jitter)
- [ ] `circuitbreaker.Do` with closed/open/half-open state machine
- [ ] Core `resilium.New` / `Execute` composition (retry + circuit breaker)
- [ ] Unit tests with >90% coverage on `retry` and `circuitbreaker`
- [ ] Benchmarks vs. sony/gobreaker and avast/retry-go

## Phase 2 — Differentiation
- [ ] `WithTimeout` and `WithRateLimit` middlewares
- [ ] Deterministic, documented policy composition order
- [ ] `resilium/otel` submodule for OpenTelemetry metrics
- [ ] `WithHooks` / `WithLogger` wired through all middlewares
- [ ] Policy-order documentation with worked examples

## Phase 3 — Production readiness
- [ ] Full godoc coverage on all exported identifiers
- [ ] `docs/` guide: choosing thresholds, common pitfalls
- [ ] golangci-lint clean, CI matrix on Go 1.22 and 1.23
- [ ] v1.0.0 API freeze and semver commitment
- [ ] Announcement post with benchmark comparisons

## Under consideration
- Bulkhead / concurrency-limiting policy
- Hedged requests (fire a backup request after a delay)
- Adapter for `net/http.RoundTripper`
