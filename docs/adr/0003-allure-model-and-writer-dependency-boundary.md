# ADR 0003: Use the official Allure model behind a writer boundary

* Status: Accepted
* Date: 2026-08-07

## Context

Hand-maintaining result JSON risks schema drift. The official Go commons module provides result/container types, while its filesystem choices do not meet this project’s guarded-cleaning and Windows atomic-replacement requirements.

## Decision

Pin `github.com/allure-framework/allure-go/commons` for model types. Keep a narrow local `output.Writer` boundary and implement deterministic JSON, metadata, atomic replacement, and cleaning locally.

## Alternatives

Copying model structs would fork the schema. Using an entire upstream adapter would couple lifecycle and filesystem policy. Untyped maps would weaken compile-time checks and golden review.

## Consequences

Model upgrades are explicit and small. The local writer is more code but owns critical durability/safety semantics.

## Compatibility impact

Default output remains official Allure 2 input compatible with Allure 2 and 3. Model bumps require generator conformance.

## Security impact

The boundary prevents an upstream writer from bypassing containment/cleaning policy and makes dependency review focused.

## Test implications

Golden JSON, writer fault injection, atomic replacement, permission, and dual-generator tests gate changes.
