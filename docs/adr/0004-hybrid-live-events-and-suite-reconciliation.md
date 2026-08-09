# ADR 0004: Hybrid live events and final suite reconciliation

- Status: Accepted
- Date: 2026-08-07

## Context

Live callbacks provide timing and partial-run durability; final `SuiteExecutionResult` contains authoritative skips, parser/validation failures, hooks, attachments, and complete trees.

## Decision

Write scenario snapshots in streaming mode, retain correlation/UUID/timing state under a mutex, then reconcile and atomically replace them from the final suite. Final suite data wins for semantics; correlated live state wins where it is uniquely available.

## Alternatives

Live-only loses final-only data. Suite-only loses partial results on interruption and delays output. Independently writing both creates duplicates.

## Consequences

Correlation logic is central and retry/table semantics must be normalized. Partial results survive many interruptions; finalization remains deterministic and idempotent.

## Compatibility impact

Producer changes to retry counts or callback order can affect keys and require matrix updates.

## Security impact

Atomic replacement avoids partially readable JSON. State is bounded by suite size but needs large-suite observation.

## Test implications

Race/stress tests drive concurrent/out-of-order callbacks, stable UUID reuse, final-only scenarios, interrupted runs, and no duplicates.
