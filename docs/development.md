# Development

## Prerequisites

* Go 1.26.x
* Gauge Core for E2E
* Python 3.10 or 3.14 plus Gauge Python 0.5.1 for the primary fixture
* Node.js 24 plus Gauge JavaScript 5.0.7 for the independence smoke test
* Java as required by Allure 2 CLI

Clone, download modules, and run the fast checks:

```console
go mod download
go test ./...
go vet ./...
go test -race ./...
```

On Windows the scripts automatically place Go caches under `.tools/cache` when a sandboxed profile cannot write the default user cache.

## Commands

| Command | Purpose |
| --- | --- |
| `make test` | unit/golden tests and coverage |
| `make cover-core` | enforce the per-package core coverage floor |
| `make test-race` | race detector |
| `make licenses` | validate the compiled Go dependency license allowlist |
| `make build` or `scripts/build.ps1` | local binary with build metadata |
| `scripts/install-local.ps1` | exact ZIP packaging, verification, isolated-capable Gauge install |
| `scripts/test-e2e.ps1 -Runner python` | exhaustive Python fixture plus expected diagnostics and both Allure generators |
| `scripts/test-e2e.ps1 -Runner js` | runner-independence smoke plus generators |
| `scripts/package-all.ps1` | all supported cross-compiled Gauge ZIPs |
| `go run ./build/validatefiles` | parse all repository JSON and YAML files |

`-SkipAllure` makes local fixture iteration faster but is not release conformance. `-PythonCommand` can select an isolated interpreter.
The conformance scripts set the upstream-supported `ALLURE_NO_ANALYTICS=true` opt-out before invoking either generator; test data and repository metadata are never sent as Allure telemetry.

## Test design

Golden tests protect exact Allure JSON. Table-driven tests cover every status enum and error kind, invalid/missing timestamps, identity vectors, retry normalization, parser/metadata behavior, safe cleaning, atomic output, and attachment policy. Fuzz targets seed identifiers, tags, and table serialization. The collector test drives concurrent scenario callbacks and final reconciliation.

The E2E harness copies fixtures to `.tools/e2e/<id>` so test runs never write reports into tracked source. Failures retain that directory for investigation.

## Adding protocol behavior

1. Confirm the field against the pinned `gauge-proto` and current Gauge Core producer.
2. Add a nil-safe normalization/mapping test with a realistic protobuf payload.
3. Decide whether live or final suite data is authoritative.
4. Add a regression to the Python fixture when the runner can produce the field.
5. Update mapping docs, compatibility notes, and an ADR if the policy changes.

Do not import a language runner into production. Keep stdout reserved for the Gauge handshake. Add interfaces only at I/O, clock, UUID, or another meaningful boundary.

## Contribution quality

Format Go with `gofmt`, keep SPDX identifiers on source files, update `go.sum`, add DCO sign-off, and do not commit generated reports, toolchains, caches, or credentials. See [CONTRIBUTING.md](../CONTRIBUTING.md).
