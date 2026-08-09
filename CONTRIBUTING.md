# Contributing

Thank you for improving Gauge Allure Report. Start with a focused issue for behavior or compatibility changes; small fixes may go directly to a pull request.

## Development flow

1. Fork and create a branch from the default branch.
2. Keep changes focused and preserve runner independence.
3. Add or update unit/golden/E2E tests and documentation.
4. Run `go test ./...`, `go vet ./...`, and `go test -race ./...`.
5. Run the relevant Gauge fixture and package verifier for protocol/packaging changes.
6. Add a changelog entry when users are affected.
7. Sign every commit with the DCO line below.

```text
Signed-off-by: Your Name <your.email@example.com>
```

Use `git commit -s`. The sign-off certifies the [Developer Certificate of Origin](DCO). A corporate CLA is not required.

## Pull requests

Describe motivation, user-visible behavior, compatibility/security impact, tests run, and anything intentionally not run. Do not mix formatting or dependency churn with a functional change. Maintainers may request an ADR for identity, lifecycle, protocol, output, security, or compatibility policy.

Generated reports, `.tools`, caches, credentials, personal test data, and unreviewed binary assets must not be committed. New dependencies require a license/security rationale and an updated inventory.

Review follows [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) and [GOVERNANCE.md](GOVERNANCE.md). Vulnerabilities follow [SECURITY.md](SECURITY.md), not public issues.
