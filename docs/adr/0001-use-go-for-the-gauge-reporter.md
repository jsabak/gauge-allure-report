# ADR 0001: Use Go for the Gauge reporter

* Status: Accepted
* Date: 2026-08-07

## Context

The plugin is a long-lived gRPC sidecar started by Gauge on Linux, macOS, and Windows. It must be independent of the test language runner and ship as a self-contained Gauge archive.

## Decision

Implement the reporter in Go with `CGO_ENABLED=0`, a small command package, internal packages, pinned modules, and cross-compiled binaries.

## Alternatives

TypeScript would require a Node runtime or bundled runtime. Python would create an apparent/actual dependency on Python and conflict with runner independence. Java would add a JVM requirement and larger distribution.

## Consequences

Single binaries simplify startup, process isolation, concurrency, and packaging. Contributors need a current Go toolchain. Cross-platform filesystem behavior still requires OS-specific atomic replacement code.

## Compatibility impact

The test runner language is irrelevant. Supported package architectures are limited to Go/Gauge combinations actually built and smoke-tested.

## Security impact

Fewer runtime installers reduce supply-chain surface; static binaries still include third-party Go modules and require SBOM/license scanning.

## Test implications

CI builds every advertised target and runs race/unit/E2E tests on representative platforms.
