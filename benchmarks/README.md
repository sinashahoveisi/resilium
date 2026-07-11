# Comparative benchmarks

Isolated Go module comparing resilium subpackages to [sony/gobreaker](https://github.com/sony/gobreaker) and [avast/retry-go](https://github.com/avast/retry-go). Third-party dependencies live here only — the core `resilium` module stays dependency-free.

## Local development

The repo root [`go.work`](../go.work) links this module to the root module for local runs without a `replace` directive in `benchmarks/go.mod`. External consumers should `go get` a tagged release once published.

## Reproduce

```bash
cd benchmarks
go test -bench=. -benchmem ./...
```

## Results

Captured on **2026-07-11** with **Go 1.26.3** on **darwin/arm64** (Apple M1 Pro, Darwin 25.5.0).

| Benchmark | ns/op | B/op | allocs/op |
| --------- | ----- | ---- | --------- |
| BenchmarkResiliumCircuitBreakerClosedHappy | 91.4 | 0 | 0 |
| BenchmarkGobreakerClosedHappy | 95.6 | 0 | 0 |
| BenchmarkResiliumCircuitBreakerOpenReject | 51.1 | 0 | 0 |
| BenchmarkGobreakerOpenReject | 52.3 | 0 | 0 |
| BenchmarkResiliumRetrySuccessOnNth | 1174 | 528 | 8 |
| BenchmarkAvastRetrySuccessOnNth | 1553 | 800 | 14 |
| BenchmarkResiliumRetryExhausted | 1601 | 624 | 11 |
| BenchmarkAvastRetryExhausted | 1592 | 880 | 16 |

Raw output:

```
go test -bench=. -benchmem ./...
goos: darwin
goarch: arm64
cpu: Apple M1 Pro
BenchmarkResiliumCircuitBreakerClosedHappy-10    	11039396	        91.40 ns/op	       0 B/op	       0 allocs/op
BenchmarkGobreakerClosedHappy-10                 	12545340	        95.61 ns/op	       0 B/op	       0 allocs/op
BenchmarkResiliumCircuitBreakerOpenReject-10     	24199178	        51.08 ns/op	       0 B/op	       0 allocs/op
BenchmarkGobreakerOpenReject-10                  	24019815	        52.25 ns/op	       0 B/op	       0 allocs/op
BenchmarkResiliumRetrySuccessOnNth-10            	 1000000	      1174 ns/op	     528 B/op	       8 allocs/op
BenchmarkAvastRetrySuccessOnNth-10               	  737142	      1553 ns/op	     800 B/op	      14 allocs/op
BenchmarkResiliumRetryExhausted-10               	  775389	      1601 ns/op	     624 B/op	      11 allocs/op
BenchmarkAvastRetryExhausted-10                  	  764192	      1592 ns/op	     880 B/op	      16 allocs/op
PASS
ok  	github.com/sinashahoveisi/resilium/benchmarks	12.946s
```

Run the command above to refresh on your hardware.

## Interpretation

These microbenchmarks measure **hot-path overhead** of closed-state success, open-state fast rejection, and retry loop iteration with **zero backoff delay**. They do **not** capture:

- Composable policy pipelines (`New` + multiple `With*` middlewares)
- Structured hooks, `slog` logging, or OpenTelemetry metrics
- Sliding-window vs cumulative failure counting semantics (see comments in `*_bench_test.go`)
- Real-world I/O latency inside guarded operations

For feature-level comparison (generics, composition, observability), see the [README comparison table](../README.md#comparison-with-alternatives).

## Non-equivalent configurations

Documented in source comments:

- **Circuit breaker:** resilium sliding window vs gobreaker cumulative counts + interval clearing; resilium checks `context.Context` before each call.
- **Retry:** avast/retry-go configured with `Delay(0)`, `FixedDelay`, and `LastErrorOnly(true)` to approximate resilium's fixed zero backoff and single final error — defaults differ substantially.

Numbers should not be read as proof that either library is universally faster in production workloads.
