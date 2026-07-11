# Contributing to resilium

Thanks for considering a contribution. This project is early-stage, so
input on the API design is just as valuable as code.

## Development setup

```bash
git clone https://github.com/sinashahoveisi/resilium.git
cd resilium
go test ./...
```

No external dependencies are required to build or test the core module.

## Before opening a PR

- Run `go vet ./...` and `go test -race ./...` — both must pass.
- If you're changing public API, update the relevant doc comments and
  the README example if it's affected.
- Add tests for new behavior. PRs that add exported functions without
  tests will be asked for tests before merge.
- Keep the core `resilium` module dependency-free. New third-party
  dependencies (e.g. for OpenTelemetry) belong in a separate submodule
  under a subdirectory with its own `go.mod`.

## Commit messages

Use a short imperative summary line, e.g. `retry: add jitter support`.
A brief body explaining _why_ is welcome for anything non-obvious.

## Design discussions

For anything that changes public API shape (not just internals), please
open an issue first to discuss before submitting a large PR. This saves
rework on both sides.

## Reporting bugs

Include a minimal reproduction and your Go version (`go version`).
