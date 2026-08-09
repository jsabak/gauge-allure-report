# ADR 0007: Typed configuration and safe output cleaning

- Status: Accepted
- Date: 2026-08-07

## Context

Reporter output is routinely cleaned in CI, making a path/configuration mistake potentially destructive. YAML and environment configuration must be predictable across runners.

## Decision

Use typed strict YAML plus documented environment precedence. Mark owned result directories and clean only recognized top-level Allure files. Refuse roots, home/project directories, `.git`, symlinks, shallow/suspicious paths, and non-owned nonempty destinations unless a narrow explicit opt-in is set.

## Alternatives

Recursive deletion is simple but unsafe. Requiring an empty directory every run prevents normal trend workflows. Lenient unknown config fields hide operator mistakes.

## Consequences

Some legacy/shared result directories need migration or `allow_clean_non_owned`. Unrelated files are preserved and may require operator cleanup.

## Compatibility impact

Configuration keys, defaults, and precedence are a versioned surface with JSON Schema.

## Security impact

The design directly mitigates path traversal and broad deletion. The opt-in weakens ownership only, not target/pattern guards.

## Test implications

Tests cover precedence, invalid types/fields, ownership, forbidden targets, symlinks, preserved files, timestamped non-clean mode, and atomic writes.
