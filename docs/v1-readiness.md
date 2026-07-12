# v1.0.0 readiness assessment

Assessment of exported API surfaces as of Phase 3. **No tag or semver
commitment is implied** — this document supports a manual freeze decision.

Legend: **Stable** = shape is ready to freeze; **Concern** = would want
changes or more real-world usage before committing to backward compatibility.

---

## github.com/sinashahoveisi/resilium

| Export | Stability | Notes |
| ------ | --------- | ----- |
| `type Operation[T]` | Stable | Core generic callback; unlikely to change. |
| `type Middleware` | Stable | Internal composition primitive; stable since Phase 1. |
| `type OperationFunc` | Stable | Untyped middleware chain building block. |
| `type Policy` | Stable | Struct fields unexported; safe to extend internally. |
| `type Option` | Stable | Standard functional options pattern. |
| `func New(...Option) *Policy` | Stable | |
| `func Execute[T](ctx, *Policy, Operation[T]) (T, error)` | Stable | Core API; error/type-assertion edge case is internal-only. |
| `func WithRetry(retry.Config) Option` | Stable | Default RetryIf for ErrCircuitOpen is intentional; document in migrations if changed. |
| `func WithCircuitBreaker(circuitbreaker.Config) Option` | Stable | Optional `circuitbreaker.Config.Name` identifies the breaker in hooks and logs; empty name preserves prior behavior. |
| `func WithTimeout(time.Duration) Option` | Stable | ErrTimeout + DeadlineExceeded dual errors.Is documented. |
| `func WithRateLimit(float64, int) Option` | Stable | Two-parameter burst API is correct; burst semantics should stay documented (Phase 2 signature change already shipped pre-release). |
| `func WithLogger(*slog.Logger) Option` | Stable | nil → slog.Default(). |
| `func WithHooks(Hooks) Option` | Stable | Duration/success callbacks intentionally deferred post-v1.0 as additive struct fields (backward compatible for keyed literals). |
| `type Hooks` | Stable | Same deferral decision: new callback fields may ship in v1.x without breaking callers using keyed struct literals. |
| `Hooks.OnRetry` | Stable | Semantics clarified: fires only when retry will follow. |
| `Hooks.OnCircuitOpen` | Stable | `name` is `circuitbreaker.Config.Name` when set via WithCircuitBreaker. |
| `Hooks.OnCircuitClose` | Stable | Same as OnCircuitOpen for `name`. |
| `Hooks.OnTimeout` | Stable | |
| `Hooks.OnRateLimited` | Stable | Added Phase 2; completes rate-limit observability. |
| `var ErrCircuitOpen` | Stable | Sentinel errors should not change. |
| `var ErrTimeout` | Stable | |
| `var ErrMaxAttemptsExceeded` | Stable | |
| `var ErrRateLimited` | Stable | |

---

## github.com/sinashahoveisi/resilium/retry

| Export | Stability | Notes |
| ------ | --------- | ----- |
| `var ErrMaxAttemptsExceeded` | Stable | Parallel sentinel to root; intentional for import-cycle avoidance. |
| `type BackoffFunc` | Stable | |
| `type Config` | Stable | Field semantics documented; MaxAttempts < 1 → 1. |
| `Config.MaxAttempts` | Stable | |
| `Config.Backoff` | Stable | nil → 100ms fixed. |
| `Config.RetryIf` | Stable | |
| `func FixedBackoff(time.Duration) BackoffFunc` | Stable | |
| `func ExponentialBackoff(base, max time.Duration) BackoffFunc` | Stable | |
| `func LinearBackoff(base, max time.Duration) BackoffFunc` | Stable | In backoff.go. |
| `func WithJitter(BackoffFunc, float64) BackoffFunc` | Stable | In backoff.go. |
| `func Do[T](ctx, Config, op) (T, error)` | Stable | Standalone entry point; behavior well-tested. |

---

## github.com/sinashahoveisi/resilium/circuitbreaker

| Export | Stability | Notes |
| ------ | --------- | ----- |
| `var ErrCircuitOpen` | Stable | |
| `type Config` | Stable | WindowSize default 20 documented. |
| `Config.FailureThreshold` | Stable | |
| `Config.MinRequests` | Stable | |
| `Config.OpenDuration` | Stable | |
| `Config.WindowSize` | Stable | Phase 2 addition; sliding window semantics should not revert. |
| `Config.Name` | Stable | Optional identifier for hooks/logs; empty string when unset. |
| `type State` | Stable | |
| `StateClosed`, `StateOpen`, `StateHalfOpen` | Stable | iota order is API. |
| `(State) String()` | Stable | |
| `type CircuitBreaker` | Stable | Safe for concurrent use. |
| `func New(Config) *CircuitBreaker` | Stable | |
| `(cb *CircuitBreaker) State() State` | Stable | Lazy open→half-open transition. |
| `func Do[T](ctx, *CircuitBreaker, op) (T, error)` | Stable | |

---

## github.com/sinashahoveisi/resilium/otel (submodule)

| Export | Stability | Notes |
| ------ | --------- | ----- |
| `func Metrics(metric.Meter) resilium.Hooks` | Concern | Counter names and attributes are public contract once tagged. No duration histogram yet — metric set may grow. Errors from `meter.Int64Counter` ignored (`_, _`) — acceptable for hooks but limits diagnosability. |

---

## Non-exported / internal

| Package | Notes |
| ------- | ----- |
| `internal/ratelimit` | Not public API; can change freely. Documented for maintainers. |

---

## Gaps before v1.0.0 tag

1. ~~**Benchmarks**~~ — **Closed.** Comparative microbenchmarks vs sony/gobreaker and avast/retry-go in [`benchmarks/README.md`](../benchmarks/README.md).
2. **Real-world usage** — limited production feedback on policy ordering and hook semantics.
3. ~~**Hooks extensibility (duration/success)**~~ — **Deferred post-v1.0** by design; additive struct fields are backward compatible.
4. ~~**Named circuit breakers**~~ — **Closed.** `circuitbreaker.Config.Name` threads through hooks and logging.
5. **otel module versioning** — requires pinned pseudo-version / semver sync with root on each release (documented in otel/go.mod).
6. **Announcement / semver policy** — semver policy written in [`docs/versioning.md`](./versioning.md); announcement post still open.

---

## Recommendation

The **core execution model** (`New`, `Execute`, `With*`, subpackage `Do` functions,
sentinel errors) is suitable for v1.0.0 freeze with documentation as written.

**Defer or explicitly mark experimental:** otel metric names — unless you accept
additive-only changes post-v1 for metrics (new counters are compatible). Hooks
duration/success callbacks are explicitly deferred to v1.x as additive fields.
