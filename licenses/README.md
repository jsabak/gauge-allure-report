# Dependency licenses

`go-modules.csv` is the reviewed allowlist for modules reachable from production and test packages. `scripts/check-licenses.*` fails when that compile-time module set changes or a license leaves the approved permissive set. License identifiers were verified against the license files in the pinned module archives.

Release SBOMs are generated separately and are authoritative for each published artifact. A dependency update must refresh both `go.sum` and this allowlist after license review.
