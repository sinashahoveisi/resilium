# resilium OpenTelemetry integration

Optional metrics for [resilium](https://github.com/sinashahoveisi/resilium) via OpenTelemetry. This is a **separate Go module** so the core `resilium` package stays dependency-free.

## Install

```bash
go get github.com/sinashahoveisi/resilium/otel
go get go.opentelemetry.io/otel/metric
```

## Usage

```go
import (
    otelresilium "github.com/sinashahoveisi/resilium/otel"
    "go.opentelemetry.io/otel"
)

meter := otel.Meter("my-service")
policy := resilium.New(
    resilium.WithRetry(retryCfg),
    resilium.WithCircuitBreaker(cbCfg),
    resilium.WithHooks(otelresilium.Metrics(meter)),
)
```

Metrics emitted: `resilium.retry.attempts`, `resilium.circuit_breaker.open`, `resilium.circuit_breaker.close`, `resilium.rate_limited`.

## Local development

This module depends on the root `github.com/sinashahoveisi/resilium` module. The repo root includes a [`go.work`](../go.work) file that links both modules for local development — use it when working from a checkout. The committed `otel/go.mod` does **not** include a `replace` directive, so external `go get` consumers resolve a tagged release of the root module once published.
