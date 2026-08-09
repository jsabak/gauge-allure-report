# ADR 0009: Gauge, Allure 2, and Allure 3 compatibility policy

* Status: Accepted
* Date: 2026-08-07

## Context

Gauge protocol/releases and both Allure generator generations evolve independently. Advertising unverified combinations would make reports unreliable.

## Decision

Maintain a machine-readable evidence matrix. Default to official Allure 2 result fields accepted by both Allure 2 and 3. Gate optional Allure 3-only behavior behind `compatibility: allure3`. Test a bounded minimum/latest Gauge matrix, upstream Gauge Python endpoint versions, advertised OS packages, and pinned current Allure generators.

## Alternatives

Latest-only testing misses regressions at the declared minimum. An unbounded version matrix is slow and unmaintainable. Emitting speculative fields can break Allure 2.

## Consequences

The advertised set may be narrower than theoretical compatibility. Upstream behavior such as zero-match early exit is documented rather than hidden.

## Compatibility impact

Raising minimum versions or removing packages requires release notes; nightly failures do not silently change policy.

## Security impact

Pinned generators/actions reduce surprise; compatibility updates still require dependency/security review.

## Test implications

Required fast checks and a bounded nightly matrix record exact evidence in `compatibility.yaml`.
