# Roadmap

## 0.1 — native reporter release candidate

* Reporter gRPC lifecycle and hybrid reconciliation
* Allure result/container mapping, secure attachments, metadata
* Python exhaustive E2E, JavaScript smoke, Allure 2/3 conformance
* Cross-platform packages, CI/security/release automation, Gauge repository proposal

## Before 1.0

* Verify and, if evidence requires, revise the true minimum Gauge version
* Expand Linux/macOS install E2E and Python 3.10/3.14 compatibility evidence
* Improve successful hook timing only when Gauge protocol data can distinguish it reliably
* Track upstream zero-match reporter lifecycle behavior
* Validate large-suite memory/latency benchmarks and tune without changing semantics
* Stabilize configuration and identity contracts

## Later candidates

* Optional Allure 3-specific fields behind explicit compatibility mode
* Additional CI executor providers based on documented variables
* Better attachment diagnostics and configurable category profiles
* Upstream protocol proposals for explicit exception kind and complete retry attempt data

Roadmap items are intentions, not delivery commitments. Security and compatibility fixes may reorder them.
