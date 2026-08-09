# Gauge to Allure mapping

## Results and hierarchy

| Gauge concept | Native Allure output |
| --- | --- |
| Suite | container spanning all emitted test UUIDs |
| Specification | container spanning its scenario attempts |
| Scenario or table row attempt | one `{uuid}-result.json` |
| Context | `Context` grouped step by default; flat or fixture by config |
| Scenario steps | `Steps` group by default |
| Teardown | `Teardown` grouped step by default; flat or fixture by config |
| Concept | nested `StepResult`, recursively preserving child steps |
| Suite/spec/scenario hooks | before/after fixture results in containers |
| Step hooks | messages/screenshots attached to the corresponding nested step |
| Parser/validation/protocol failure | named synthetic broken test result |
| Empty delivered suite | named synthetic skipped result, or broken when suite is failed |

Concept depth is limited by `maximum_nesting_depth` (100 by default); exceeding it creates a broken diagnostic step instead of risking a stack panic.

## Status

| Gauge evidence | Allure status |
| --- | --- |
| passed execution | `passed` |
| assertion diagnostic | `failed` |
| recoverable/verification failure | `failed` |
| explicit skip / `SkipScenarioException` | `skipped` |
| unexpected exception | `broken` |
| hook, parser, validation, protocol or reporter integration failure | `broken` |
| unknown/not-executed without a skip reason | `broken` |

Some runners leave protobuf `errorType` at its default `ASSERTION` for every exception. The reporter therefore gives explicit `VERIFICATION` and `recoverableError` precedence, recognizes portable assertion diagnostics, and classifies ambiguous failures as `broken`.

## Identity and retries

Canonical identity is `${project}::${project-relative-spec-path}::${scenario-id-or-span-and-heading}` with normalized `/` separators. Absolute checkout prefixes, stream/runner allocation, table values, and retry number do not affect `testCaseId`.

* `testCaseId`: lowercase SHA-256 hex of canonical scenario identity.
* `historyId`: lowercase SHA-256 hex of the test case ID plus sorted logical parameters after privacy/history policy.
* `uuid`: random UUID v4 per attempt, reused during live/final reconciliation.
* table row values: parameters and history inputs, not test case identity.
* retry: excluded `retry` parameter and `gauge.retry` label; not a history input.

Earlier retry attempts remain separate result files with the same logical identifiers, allowing Allure to group them.

## Parameters and tables

Static, dynamic, special string, multiline string, and table fragments become step parameters. Specification and scenario data-table columns become test parameters. `promote_parameters` copies named step parameters to the test level. `mask_parameters`, `hide_parameters`, and `exclude_history_parameters` apply before history hashing; secret raw values do not enter IDs after masking/hiding.

When `attach_tables` is enabled, full tables are also serialized as deterministic JSON attachments.

## Tags, labels, and links

Every original Gauge tag remains an Allure `tag` label. Reserved tags additionally create metadata:

| Syntax | Additional mapping |
| --- | --- |
| `allure.label.<name>:<value>` | arbitrary nonempty label |
| `allure.link.issue:<key>` | issue link using configured pattern |
| `allure.link.tms:<key>` | TMS link using configured pattern |
| `allure.link.link:<url>` | generic link |
| `allure.id:<value>` | `ALLURE_ID` label |

Duplicates are removed deterministically. Malformed reserved tags remain ordinary tags and produce warnings. Optional `source_link_template` receives the canonical path and line number.

## Timing

Gauge `timestampISO` is parsed as the start; `executionTime` is milliseconds. Live start callbacks override fallback starts when correlated. Invalid/missing timestamps use an injected clock, negative durations clamp to zero, child times clamp to their parent range, and `stop` never precedes `start`.

## Attachments and metadata

Gauge messages become UTF-8 text attachments, screenshots retain sniffed MIME types, and legacy screenshot bytes are supported. `environment.properties`, optional `executor.json`, and optional `categories.json` are written once per run. CI auto-detection supports Jenkins, GitHub Actions, GitLab, Azure Pipelines, TeamCity, CircleCI, and a conservative generic fallback.
