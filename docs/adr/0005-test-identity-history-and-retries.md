# ADR 0005: Stable test identity, history, and retries

* Status: Accepted
* Date: 2026-08-07

## Context

Allure needs stable logical identifiers across machines while keeping table rows distinct in history and attempts distinct as files.

## Decision

Canonicalize project-relative `/` paths and scenario ID or span/heading. Derive `testCaseId` with SHA-256 of that identity. Derive `historyId` from the test case ID and sorted privacy-processed logical parameters. Use random UUID v4 per attempt. Exclude stream, runner, absolute checkout path, and retry number; represent retry as an excluded parameter/label.

## Alternatives

Random IDs lose history. Name-only IDs collide. Including absolute paths or stream IDs is unstable. Including retry in history prevents Allure grouping.

## Consequences

Moving/renaming a specification or scenario intentionally changes test identity. Table values affect history but not test case identity.

## Compatibility impact

Identity rules are a user-visible contract; changes require migration notes and golden vectors.

## Security impact

Masked/hidden raw values are removed before hashing, avoiding recoverable secret-derived identifiers.

## Test implications

Golden SHA vectors, cross-platform path tests, duplicate names, table rows, retry attempts, privacy policy, and fuzzing are required.
