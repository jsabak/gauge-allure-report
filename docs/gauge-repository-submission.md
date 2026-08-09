# Gauge plugin repository submission

`gauge-repository/allure-report-install.json` is a prepared candidate, not proof of publication.

## Preconditions

* GitHub repository and `v0.1.0` release exist at the URLs encoded in the metadata.
* Every listed Windows/Linux/macOS architecture asset exists and passes `build/verify`.
* `plugin.json`, install metadata, changelog, and tag agree on the version and Gauge support range.
* Checksums, license, security policy, source tag, and reproducible build instructions are public.
* Minimum and latest Gauge compatibility jobs have passed.

## Submission

1. Fork `getgauge/gauge-repository`.
2. Add the validated install JSON using the upstream repository’s current directory/naming convention.
3. Run the upstream metadata tests and schema checks.
4. Open a focused pull request describing the native reporter protocol, platforms, license, source, and compatibility evidence.
5. Respond to review without changing already-published archives; publish a patch version when asset content must change.

After upstream merge, update README maturity/install wording and verify `gauge install allure-report` in a clean `GAUGE_HOME` on all supported operating systems.

The inferred repository owner and asset URLs must be reconfirmed before submission. This project does not fabricate checksums or claim an upstream merge.
