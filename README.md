# resilium

**Composable resilience policies for Go — retry, circuit breaker, timeout, and rate limiting, unified behind one type-safe API.**

[![Go Reference](https://pkg.go.dev/badge/github.com/sinashahoveisi/resilium.svg)](https://pkg.go.dev/github.com/sinashahoveisi/resilium)
[![Go Report Card](https://goreportcard.com/badge/github.com/sinashahoveisi/resilium)](https://goreportcard.com/report/github.com/sinashahoveisi/resilium)
[![CI](https://github.com/sinashahoveisi/resilium/actions/workflows/ci.yml/badge.svg)](https://github.com/sinashahoveisi/resilium/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Why resilium?

Most Go projects end up hand-wiring retry logic, circuit breakers, and timeouts separately — often with inconsistent behavior, no shared observability, and no clear execution order between them. resilium fixes that with a single composable pipeline:

```go
policy := resilium.New(
    resilium.WithTimeout(5*time.Second),
    resilium.WithRetry(retry.Config{
        MaxAttempts: 3,
        Backoff:     retry.ExponentialBackoff(100*time.Millisecond, 2*time.Second),
    }),
    resilium.WithCircuitBreaker(circuitbreaker.Config{
        FailureThreshold: 0.5,
        MinRequests:      10,
        OpenDuration:     30 * time.Second,
        WindowSize:       50, // defaults to 20 if unset
    }),
)

user, err := resilium.Execute(ctx, policy, func(ctx context.Context) (User, error) {
    return fetchUser(ctx, userID)
})
```

No wrapper soup, no interface{} juggling — results are fully typed via generics.

## Features

- **Composable policies** — combine retry, circuit breaker, timeout, and rate limiting in any order; each works standalone too
- **Generics-first** — `Execute[T]` returns your actual type, not `interface{}`
- **Context-aware** — cancellation and deadlines propagate correctly through every policy
- **Observable by default** — structured logging hooks and OpenTelemetry metrics integration included, not bolted on
- **Zero required dependencies** — the core module has no third-party dependencies; OpenTelemetry support is an optional submodule

## Installation

```bash
go get github.com/sinashahoveisi/resilium
```

Requires Go 1.22 or later (uses generics and the standard `log/slog` package).

## Quick start

The canonical runnable example lives at [`examples/basic/main.go`](./examples/basic/main.go):

```bash
go run ./examples/basic
```

It combines retry, timeout, and rate limiting:

```go
policy := resilium.New(
    resilium.WithRetry(retry.Config{
        MaxAttempts: 3,
        Backoff:     retry.ExponentialBackoff(100*time.Millisecond, 1*time.Second),
    }),
    resilium.WithTimeout(2*time.Second),
    resilium.WithRateLimit(10, 10), // 10 req/s sustained, burst up to 10
)
result, err := resilium.Execute(ctx, policy, callFlakyService)
```

See [`examples/`](./examples) for additional circuit breaker and combined-policy examples.

## Policy execution order

When you combine policies, order matters. resilium applies them in the order you list them — outermost first:

```go
resilium.New(
    resilium.WithTimeout(5*time.Second),   // outermost: bounds total time, including retries
    resilium.WithRetry(retryConfig),       // retries the call below on failure
    resilium.WithCircuitBreaker(cbConfig), // innermost: guards the actual call
)
```

This is deliberate rather than automatic, because the "correct" order depends on your use case (e.g. do you want a circuit breaker to see every retry attempt, or just the overall outcome?). See [docs/policy-order.md](./docs/policy-order.md) for ordering examples and [docs/guide.md](./docs/guide.md) for threshold tuning and common pitfalls.

> **Note:** the circuit breaker evaluates failures over a sliding
> window of the last `WindowSize` requests (default 20), not a
> cumulative all-time counter. This means a long-running healthy
> service that suddenly starts failing trips the breaker within
> `WindowSize` failures — not after thousands of accumulated
> historical successes dilute the ratio.

## Observability

```go
policy := resilium.New(
    resilium.WithLogger(slog.Default()),
    resilium.WithHooks(resilium.Hooks{
        OnRetry:       func(attempt int, err error) { /* ... */ },
        OnCircuitOpen: func(name string) { /* ... */ },
    }),
)
```

OpenTelemetry metrics are available via the optional `resilium/otel` submodule:

```go
import otelresilium "github.com/sinashahoveisi/resilium/otel"

policy := resilium.New(
    resilium.WithHooks(otelresilium.Metrics(otel.Meter("my-service"))),
)
```

See [`otel/README.md`](./otel/README.md). The core module stays dependency-free.

## Status

resilium is under active development. The API may change before v1.0. See [CHANGELOG.md](./CHANGELOG.md) for release notes and [ROADMAP.md](./ROADMAP.md) for what's planned.

## Comparison with alternatives

|                        | resilium | sony/gobreaker | avast/retry-go | failsafe-go |
| ---------------------- | -------- | -------------- | -------------- | ----------- |
| Generics               | ✅       | ❌             | ✅             | ✅          |
| Circuit breaker        | ✅       | ✅             | ❌             | ✅          |
| Retry                  | ✅       | ❌             | ✅             | ✅          |
| Composable policies    | ✅       | ❌             | ❌             | ✅          |
| Built-in OTel metrics  | ✅       | ❌             | ❌             | ❌          |
| Zero core dependencies | ✅       | ✅             | ✅             | ❌          |

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a PR — it covers the development setup, testing requirements, and commit conventions.

## License

MIT — see [LICENSE](./LICENSE).
