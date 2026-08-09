# Troubleshooting

## Gauge cannot connect to the reporter

Run `allure-report doctor` and confirm the binary starts only with Gauge’s `allure-report_action=execution`. The protocol handshake must be the exact stdout line `Listening on port:<n>`; logs belong on stderr. Check the Gauge project and `GAUGE_HOME/logs/gauge.log` for the reporter’s stderr.

## No result directory for a tag filter

Gauge Core 1.6.35 exits before launching reporters when a tag filter matches zero specifications. No plugin can write a result without a process start or callback. The E2E fixture records this boundary. When Core does deliver an empty final suite, `allure-report` emits a synthetic skipped result; that path has a regression test.

## Missing or unsafe screenshot

By default the file must be regular, no larger than 100 MiB, not a symlink, and inside the project. Gauge Python custom screenshot writers should write into `gauge_screenshots_dir`; the runner reports only a basename. Best-effort mode records a warning and continues; strict mode fails the callback. Avoid enabling `follow_symlinks` or `allow_external_attachments` unless the workspace trust boundary is understood.

## Cleaning was refused

The destination is protected by `.gauge-allure-report-owned`. The reporter refuses roots, home/project directories, `.git`, suspicious paths, and non-owned nonempty directories. Choose a dedicated empty directory. `allow_clean_non_owned: true` only relaxes ownership; it still deletes only recognized top-level Allure files.

## Unexpected exception appears as failed or broken

Runners may not distinguish exception types in the enum. The reporter reads recoverable/verification fields and diagnostics. Ensure runner stack traces are enabled. Assertions should include recognizable assertion diagnostics; ambiguous failures intentionally become `broken`.

## Duplicate attempts

Gauge’s final `retriesCount` is a count of attempts, not the zero-based live retry number. This reporter normalizes it. If duplicates remain, enable `diagnostic_proto_dump` only in a sanitized test run and file a compatibility report with Gauge, runner, OS, parallel/retry flags, and redacted artifacts.

## Configuration fails

Run:

```console
allure-report validate-config .gauge/allure-report.yaml
```

Unknown keys are errors by design. List variables accept commas or semicolons; booleans use Go forms such as `true`/`false`; byte/depth values must be positive integers.

## Allure generation fails

Retain `allure-results` and run the pinned Allure 2 and Allure 3 commands from [allure2-and-allure3.md](allure2-and-allure3.md). Check for truncated JSON, missing referenced attachment files, insufficient Java/Node versions, or an accidentally merged directory from concurrent builds.
