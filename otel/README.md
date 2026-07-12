# resilium OpenTelemetry integration

Optional metrics for [resilium](https://github.com/sinashahoveisi/resilium) via OpenTelemetry. This is a **separate Go module** so the core `resilium` package stays dependency-free.

**Compatible with:** `resilium` v1.0.0+. This module follows [independent semantic versioning](https://github.com/sinashahoveisi/resilium/blob/main/docs/versioning.md#resiliumotel--independent-module-version-aligned-compatibility) from the root module — check the `require` line in [`go.mod`](./go.mod) for the exact root version each release was built against.

## Install

```bash
go get github.com/sinashahoveisi/resilium@v1.0.0
go get github.com/sinashahoveisi/resilium/otel@v1.0.0
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

Metric names and attribute keys are a public contract as of v1.0.0 — new instruments may be added in future minor releases, but existing names won't change without a major bump. See [`docs/versioning.md`](https://github.com/sinashahoveisi/resilium/blob/main/docs/versioning.md).

## Local development

This module depends on the root `github.com/sinashahoveisi/resilium` module. The repo root includes a [`go.work`](../go.work) file that links both modules for local development — use it when working from a checkout. The committed `otel/go.mod` does **not** include a `replace` directive, so external `go get` consumers resolve a tagged release of the root module directly.

After pushing changes to the root module, regenerate the pin from within `otel/`:

```bash
GOWORK=off GOPROXY=direct go get github.com/sinashahoveisi/resilium@main
go mod tidy
```

## Links

- [Root module README](https://github.com/sinashahoveisi/resilium)
- [Versioning policy](https://github.com/sinashahoveisi/resilium/blob/main/docs/versioning.md)
- [CHANGELOG](https://github.com/sinashahoveisi/resilium/blob/main/CHANGELOG.md)