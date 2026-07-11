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

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/sinashahoveisi/resilium"
    "github.com/sinashahoveisi/resilium/retry"
)

func main() {
    policy := resilium.New(
        resilium.WithRetry(retry.Config{
            MaxAttempts: 3,
            Backoff:     retry.ExponentialBackoff(100*time.Millisecond, 1*time.Second),
        }),
    )

    result, err := resilium.Execute(context.Background(), policy, func(ctx context.Context) (string, error) {
        return callFlakyService(ctx)
    })
    if err != nil {
        fmt.Println("failed after retries:", err)
        return
    }
    fmt.Println("got:", result)
}
```

See [`examples/`](./examples) for circuit breaker, timeout, and combined-policy examples.

## Policy execution order

When you combine policies, order matters. resilium applies them in the order you list them — outermost first:

```go
resilium.New(
    resilium.WithTimeout(5*time.Second),   // outermost: bounds total time, including retries
    resilium.WithRetry(retryConfig),       // retries the call below on failure
    resilium.WithCircuitBreaker(cbConfig), // innermost: guards the actual call
)
```

This is deliberate rather than automatic, because the "correct" order depends on your use case (e.g. do you want a circuit breaker to see every retry attempt, or just the overall outcome?). The [docs](./docs/policy-order.md) walk through common configurations.

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

OpenTelemetry metrics are available via the `resilium/otel` submodule, kept separate so the core package stays dependency-free.

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
