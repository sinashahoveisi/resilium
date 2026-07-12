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
- [x] v1.0.0 API freeze and semver commitment — tagged `v1.0.0`, see [`docs/versioning.md`](./docs/versioning.md)
- [x] Announcement post with benchmark comparisons — [published on Medium](https://medium.com/@sinashahoveisi/introducing-resilium-composable-type-safe-resilience-for-go-ebf18a662468)

## Post-v1.0 (tracked, not yet scheduled)

- [ ] Duration/success callbacks on `Hooks` (additive, backward compatible — see [`docs/v1-readiness.md`](./docs/v1-readiness.md))
- [ ] Real-world usage feedback period before any v2 discussion

## Under consideration

- Bulkhead / concurrency-limiting policy
- Hedged requests (fire a backup request after a delay)
- Adapter for `net/http.RoundTripper`
