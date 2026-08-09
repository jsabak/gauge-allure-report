# ADR 0010: Deterministic Gauge packaging and repository distribution

* Status: Accepted
* Date: 2026-08-07

## Context

Gauge expects a platform archive with `plugin.json` and a binary below `bin/`, plus platform-specific naming and metadata. Generic Go archive defaults place binaries at the root.

## Decision

Use a small audited Go packager/verifier as the authoritative release configuration. Cross-compile with `CGO_ENABLED=0`, inject version metadata, create exact Gauge ZIPs, verify entries/modes/version, produce checksums/SBOM/provenance, and publish only from reviewed semantic tags with explicit permissions. Prepare—but never auto-push—the Gauge repository metadata.

## Alternatives

Default GoReleaser archives violate the directory contract. Hand-built ZIP commands vary by OS. Automatically pushing upstream metadata would require overly broad credentials.

## Consequences

The custom packager needs tests and maintenance but makes the archive contract explicit. Public availability starts only after GitHub release and Gauge repository acceptance.

## Compatibility impact

Only built and smoke-tested architectures are advertised. Archive naming changes require upstream metadata changes.

## Security impact

Exact inventories reject source/secrets/test output. OIDC provenance and draft-release review reduce artifact substitution risk.

## Test implications

Every package is verified, version-executed, installed in isolated `GAUGE_HOME`, and used for a real Gauge run.
