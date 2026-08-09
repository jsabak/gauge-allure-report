# ADR 0011: Best-effort and strict error handling

* Status: Accepted
* Date: 2026-08-07

## Context

A reporter failure can erase diagnostics for an otherwise useful test run. Conversely, release conformance and regulated pipelines may need any report integrity error to fail immediately.

## Decision

Default to best effort: write every valid artifact possible, collect bounded warnings, emit synthetic broken diagnostics for missing test-level failures, and summarize. Provide `strict: true` to return mapping/output errors through gRPC. Unknown protocol/status values never become passed.

## Alternatives

Always fail-fast maximizes enforcement but commonly produces no report. Always swallow errors hides corrupted/missing artifacts.

## Consequences

Operators choose availability versus enforcement explicitly. Some warnings cannot be represented globally in Allure, so stderr summary and synthetic results are important.

## Compatibility impact

Error-handling mode does not change valid artifact semantics. Strict mode can change Gauge process outcome when attachments or writes fail.

## Security impact

Best effort must not bypass containment or cleaning; security-policy violations are skipped/warned, never accepted. Strict mode is recommended for release conformance.

## Test implications

Fault-injected writer/attachment/protocol tests verify both modes, aggregation, bounded details, synthetic results, and successful finalization after test failures.
