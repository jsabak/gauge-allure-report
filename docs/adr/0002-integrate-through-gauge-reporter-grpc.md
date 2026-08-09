# ADR 0002: Integrate through Gauge reporter gRPC

- Status: Accepted
- Date: 2026-08-07

## Context

Gauge exposes a reporter plugin lifecycle that is separate from language runners. XML conversion would lose hooks, nesting, retries, tables, and attachments and would add an intermediate format.

## Decision

Implement the current 12-method Gauge Reporter gRPC service, bind an ephemeral loopback port, print the exact startup handshake, and consume only Gauge Core protocol/environment data.

## Alternatives

Converting JUnit/XML, instrumenting Python steps with Allure calls, modifying `gauge-python`, or forking Gauge Core were rejected because they are lossy, runner-specific, invasive, or operationally expensive.

## Consequences

Native lifecycle data is available, but the project must track proto/Core producer behavior and be nil-safe for partial messages. Stdout is reserved for the handshake.

## Compatibility impact

`plugin.json` declares `grpc_support` and execution scope. Protocol changes require compatibility fixtures and may raise the minimum Gauge version.

## Security impact

Loopback-only binding avoids a remote service. Large-message limits still require memory/stress testing and bounded downstream fields.

## Test implications

Tests cover every RPC path, nil/unknown payloads, chunk flags, startup/install behavior, and real Core E2E.
