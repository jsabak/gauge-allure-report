# Releasing

Releases follow SemVer. Version `0.x` may refine configuration/model policy, but changes remain documented in the changelog and compatibility matrix.

## Preparation

1. Confirm `CHANGELOG.md`, `compatibility.yaml`, `plugin.json`, and the Gauge repository metadata use the same version.
2. Run `go mod verify`, unit tests, race tests, vet, coverage, and fuzz seeds.
3. Run the full Gauge Python E2E on latest and minimum Gauge jobs and the JavaScript smoke test.
4. Generate both Allure 2 and Allure 3 reports.
5. Run `scripts/package-all.ps1` and verify every ZIP.
6. Inspect dependency licenses, CodeQL, dependency review, and artifact inventory.

## Packaging

The custom Go packager is authoritative because Gauge requires the binary below `bin/`, while generic release archivers place it at the archive root. Every ZIP contains exactly:

```text
plugin.json
bin/allure-report[.exe]
```

Archive names use `allure-report-<version>-<os>.<x86|x86_64|arm64>.zip`. `plugin.json` is validated against the requested version, Unix executable bits are checked, and unexpected/duplicate entries fail verification.

## Automation

Release Please prepares a version/changelog PR. Merging it creates the tag and GitHub release, then dispatches the protected `release` environment workflow. That workflow repeats tests and E2E conformance, cross-compiles with `CGO_ENABLED=0`, verifies every platform package, and uploads checksums, an SBOM, provenance, and a keyless checksum signature to the existing release. Configure required reviewers on the `release` environment so artifact publication requires an explicit maintainer approval.

Signing uses GitHub OIDC provenance and release checksums; no long-lived signing secret is required. If verification, signing, or attestation fails, the workflow fails before uploading release assets. The maintainer must fix the failure and rerun the protected workflow; incomplete releases must not be announced.

## Gauge repository submission

Only after release URLs and checksums exist, update and validate `gauge-repository/allure-report-install.json`, then follow [gauge-repository-submission.md](gauge-repository-submission.md). Do not claim `gauge install allure-report` works before the upstream metadata is merged.

## Rollback

Never replace assets on an existing version. Mark a defective GitHub release and Gauge repository version as affected, publish a new patch version, and document the migration. If a vulnerability is involved, coordinate privately under [SECURITY.md](../SECURITY.md).
