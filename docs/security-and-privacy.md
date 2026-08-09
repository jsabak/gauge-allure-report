# Security and privacy

## Threat model

Specifications, runner diagnostics, protobuf fields, attachment paths, environment variables, and CI metadata are untrusted. A malicious test repository may attempt path traversal, symlink escape, oversized files, secret exfiltration, output deletion, or protocol resource exhaustion.

## Controls

* gRPC binds to an ephemeral loopback address only; receive/send limits are 1 GiB to match large Gauge suites without exposing a network service.
* Output preparation rejects broad/sensitive targets and never recursively deletes. Only recognized top-level Allure artifacts in an owned directory are eligible.
* JSON and attachments are written to temporary files, synchronized, and atomically replaced. Windows replacement uses `MoveFileEx` with replace/write-through semantics.
* Attachments use `Lstat`, regular-file enforcement, project containment, symlink refusal, a configurable byte limit, MIME sniffing, and streamed copy.
* Gauge-managed screenshot search roots do not bypass containment.
* Environment output is an exact allowlist; common CI provider fields are selected explicitly for `executor.json`.
* Message attachments support literal redaction. Masked/hidden parameter values do not enter history identity.
* Diagnostic protobuf dumping is disabled by default and clearly marked sensitive.
* Failure messages and traces are bounded to 16 KiB and 256 KiB respectively.
* Unknown statuses and protocol gaps are never represented as passed tests.

## Operator guidance

Treat Allure results as potentially sensitive build artifacts. Restrict artifact readers, retention, and public report hosting. Do not allow external attachments or symlinks in untrusted pull requests. Redact tokens at their source as well as with reporter configuration; redaction is literal, not a general secret scanner.

Use `strict: true` in controlled release conformance jobs and best-effort mode in ordinary test jobs where a report is preferable to a missing report. Do not enable diagnostic dumps in production without reviewing the Gauge suite payload.

## Reporting vulnerabilities

Do not open a public issue with exploit details. Follow [SECURITY.md](../SECURITY.md) and use a private GitHub Security Advisory. Include affected versions, reproduction, impact, and suggested mitigation without real credentials.
