# ADR 0008: Secure attachments and data redaction

- Status: Accepted
- Date: 2026-08-07

## Context

Runner-provided paths and messages can expose arbitrary files, follow symlinks, exhaust memory/disk, or leak secrets into CI artifacts.

## Decision

Accept regular files within the project, including Gauge’s contained screenshot directory; refuse symlinks/external paths by default; enforce per-file size limits; stream copies; sniff MIME; and generate UUID attachment names. Aggregate/redact messages and apply mask/hide/history policy before identity generation. Make raw protobuf dumps opt-in and visibly sensitive.

## Alternatives

Blind copy trusts the test repository too much. Loading every file into memory is simple but unsafe for large suites. Regex secret scanning gives false confidence and unpredictable transformations.

## Consequences

External/symlink workflows need explicit configuration. Literal redaction requires operators to provide sensitive values and is defense-in-depth, not source sanitization.

## Compatibility impact

Runners reporting screenshot basenames are supported through `gauge_screenshots_dir` without bypassing containment.

## Security impact

This is a primary trust-boundary decision; weakening options must remain explicit and diagnosed by `doctor`.

## Test implications

Missing, oversized, directory, symlink, external, MIME, basename-resolution, redaction, and legacy-byte cases are required.
