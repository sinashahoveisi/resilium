# Versioning policy

resilium follows [Semantic Versioning 2.0.0](https://semver.org/) (`MAJOR.MINOR.PATCH`).
Tags apply per Go module. **This policy takes effect at v1.0.0** — it does not
retroactively classify pre-release changes.

---

## Pre-1.0 vs post-1.0

During Phase 1–3 (pre-v1.0.0), breaking API changes were allowed without a major
version bump. Examples that would **not** be repeated after v1.0.0:

- **`WithRateLimit` signature change** — `WithRateLimit(requestsPerSecond float64)`
  → `WithRateLimit(requestsPerSecond float64, burst int)` (Phase 2). Post-v1.0,
  this would require **v2.0.0**.

Once **v1.0.0** is tagged, the rules below apply to all subsequent releases of
each published module.

---

## Major bump (breaking change)

Requires **MAJOR** version increment (e.g. `v1.x.x` → `v2.0.0`).

### Exported API surface

Applies to all published modules: root `resilium`, `resilium/retry`,
`resilium/circuitbreaker`, and `resilium/otel`.

- **Remove or rename** any exported function, type, constant, variable, or struct field.
- **Change an exported function signature** — parameters, return types, or generic constraints.

  **Example (would be MAJOR post-v1.0):** Phase 2 changed
  `WithRateLimit(rps float64)` to `WithRateLimit(rps float64, burst int)`.
  Existing call sites would fail to compile; that is a breaking change requiring
  `v2.0.0`, not a minor release.

- **Change documented behavior of a sentinel error** when existing `errors.Is` /
  `errors.As` logic would break.

  **Example:** If `ErrCircuitOpen` started wrapping a different error type, or
  began being returned from `WithRetry` exhaustion (not just the circuit breaker),
  callers filtering on `errors.Is(err, ErrCircuitOpen)` could mis-classify failures.

- **Change a default that alters runtime behavior** for callers who leave a field
  at the zero value.

  **Examples:**
  - Changing default `circuitbreaker.Config` `WindowSize` from **20** to another value.
  - Changing implicit rate-limit burst when callers pass only `requestsPerSecond`.
  - Changing default `retry.Config` backoff from **100ms fixed** to exponential.

### Deprecation before removal

Exported symbols are **deprecated for at least one minor release** before removal
in a major release:

1. Add a `// Deprecated: use X instead. Will be removed in vN.0.0.` godoc comment.
2. Mention in [CHANGELOG.md](../CHANGELOG.md).
3. Remove only in the next **major** release.

---

## Minor bump (additive, backward compatible)

Requires **MINOR** version increment (e.g. `v1.0.x` → `v1.1.0`).

- **New exported** functions, types, constants, or `Option` constructors
  (e.g. a future `WithBulkhead`).
- **New fields on existing exported structs** when zero value preserves prior behavior
  and keyed struct literals remain valid.

  **Canonical example:** `circuitbreaker.Config.Name` (added pre-v1.0). Existing code:

  ```go
  circuitbreaker.Config{
      FailureThreshold: 0.5,
      MinRequests:      10,
      OpenDuration:     30 * time.Second,
  }
  ```

  continues to compile and behave identically (`Name == ""`). New callers may set
  `Name: "payments"` for hook identification.

- **New optional hook callbacks on `Hooks`** — per [v1-readiness.md](./v1-readiness.md),
  duration/success callbacks are intentionally deferred post-v1 as additive fields.
  Callers using keyed fields (`Hooks{OnRetry: ...}`) are unaffected by new fields.

- **New middleware / `With*` options** that do not change behavior of existing options.

- **New metrics or attributes in `resilium/otel`** when existing counter/histogram
  names and attribute keys are unchanged (new instruments are additive).

---

## Patch bump

Requires **PATCH** version increment (e.g. `v1.0.0` → `v1.0.1`).

- **Bug fixes** that restore documented behavior without changing the public contract.
- **Performance improvements** with no observable semantic change.
- **Documentation-only** changes (README, `docs/`, godoc clarifications).
- **Test-only** changes.

---

## Explicit commitments and exceptions

### `internal/` packages — not semver-covered

Packages under `internal/` (e.g. `internal/ratelimit`) are **not public API**.
They may change in any release without semver consideration. Consumers must use
`resilium.WithRateLimit`, not import `internal/ratelimit` directly.

### `benchmarks/` module — not semver-covered

The `benchmarks/` module exists for comparative microbenchmarks only. It is not
part of the supported public API and is not versioned for external consumers.

### `resilium/otel` — independent module version, aligned compatibility

`resilium/otel` is a **separate Go module** with its own tags
(`github.com/sinashahoveisi/resilium/otel/vX.Y.Z`).

**Policy:** otel versions **independently** on its own semver line, but each otel
release documents which root `resilium` versions it supports (via `require` in
`otel/go.mod`).

**Why not lock otel major to root major?**

- otel can ship metric additions (minor) without a root release.
- Root can patch without forcing an otel tag.
- `go get` on otel already resolves root by version; independent tags avoid
  unnecessary coordinated major bumps when only one module changes.

**Tradeoff:** Consumers must check otel release notes / `go.mod` for compatible
root versions. We accept this in exchange for cleaner module boundaries.

**Breaking otel changes** (renamed metric instruments, removed hooks adapters) bump
**otel major**, not root major, unless root API also breaks.

### Go version support

- **Minimum Go version:** 1.22 (generics, `log/slog`).
- **CI matrix:** Go **1.22** and **1.23** (see [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)).
- **Support policy:** We support the **two most recent Go minor releases** tested
  in CI. When Go 1.24 is added to CI, 1.22 may be dropped in a minor release with
  CHANGELOG notice; dropping a supported Go version is **not** a major semver bump
  for resilium (consumers on older Go toolchains may need to stay on an older tag).

### Modules included in root semver

Tags on `github.com/sinashahoveisi/resilium` cover the root module and its
**published subpackages in the same module**:

- `resilium` (root)
- `resilium/retry`
- `resilium/circuitbreaker`

These share one `go.mod` and one tag line.

---

## Release checklist (maintainers)

1. Update [CHANGELOG.md](../CHANGELOG.md) with user-facing changes and semver bucket.
2. Tag root (and otel separately if otel changed).
3. Regenerate pseudo-version pins in `otel/go.mod` / `benchmarks/go.mod` after root tag.
4. For deprecations: ensure one minor release with `Deprecated` comments before removal.

---

## Related docs

- [v1-readiness.md](./v1-readiness.md) — API stability assessment
- [CHANGELOG.md](../CHANGELOG.md) — release history
- [CONTRIBUTING.md](../CONTRIBUTING.md) — how to propose API changes
