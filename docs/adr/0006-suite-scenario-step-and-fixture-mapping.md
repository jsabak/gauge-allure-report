# ADR 0006: Map suites, scenarios, steps, and fixtures natively

- Status: Accepted
- Date: 2026-08-07

## Context

Gauge has suite/spec/scenario hooks, context, teardown, concepts, step hooks, and table-driven scenarios. Allure separates test results, nested steps, and fixture containers.

## Decision

Emit one test per scenario/table-row attempt; preserve concepts as nested steps; group Context/Steps/Teardown by default; map suite/spec/scenario lifecycle to containers referencing child UUIDs. Keep step hooks inside their step via attachments rather than creating a container per step.

## Alternatives

Flattening loses behavior structure. One test per step inflates counts. Step containers multiply files/associations and do not match Allure’s intended nested-step model.

## Consequences

Containers make broad fixture scope visible. Successful hook presence/timing is limited by what the reporter protocol supplies; failures/messages/screenshots remain authoritative.

## Compatibility impact

Grouping can be configured for context/teardown; hook mode is fixed to fixtures in 0.1.

## Security impact

Fixture attachments follow the same bounded policy as step attachments.

## Test implications

Golden tests and E2E cover all scopes, nested concepts, parallel child association, context/teardown modes, and hook failures.
