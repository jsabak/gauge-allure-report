# Configuration

Configuration precedence is environment override → YAML → built-in default. The default YAML path is `.gauge/allure-report.yaml`; `GAUGE_ALLURE_CONFIG` selects another path relative to the Gauge project root. Unknown YAML fields and invalid enum values fail startup before the reporter handshake.

## YAML reference

| Key | Default | Meaning |
| --- | ---: | --- |
| `results_dir` | derived | destination directory |
| `clean_results` | `true` | safely remove known artifacts from an owned destination |
| `allow_clean_non_owned` | `false` | allow known-file cleanup in a non-owned nonempty directory; weakens safety |
| `compatibility` | `allure2-and-3` | conservative model, or `allure3` for future opt-in extras |
| `write_mode` | `streaming` | write scenario snapshots live, or only at `suite-final` |
| `strict` | `false` | fail an RPC on mapping/output errors instead of collecting warnings |
| `log_level` | `info` | `debug`, `info`, `warn`, or `error` on stderr |
| `attach_screenshots` | `true` | import Gauge screenshot files/bytes |
| `attach_messages` | `true` | aggregate Gauge messages by scope |
| `attach_tables` | `true` | attach deterministic table JSON |
| `max_attachment_bytes` | `104857600` | per-attachment byte limit |
| `follow_symlinks` | `false` | permit attachment symlinks after resolution checks |
| `allow_external_attachments` | `false` | permit files outside project root |
| `context_mode` | `grouped-steps` | `grouped-steps`, `flat`, or `fixtures` |
| `teardown_mode` | `grouped-steps` | `grouped-steps`, `flat`, or `fixtures` |
| `hook_mode` | `fixtures` | only supported hook mapping in 0.1 |
| `behavior_from_spec` | `false` | add specification heading as Allure `feature` |
| `issue_link_pattern` | empty | URL with `{}` placeholder |
| `tms_link_pattern` | empty | URL with `{}` placeholder |
| `generic_link_pattern` | empty | optional generic link pattern |
| `environment_allowlist` | `[]` | exact environment keys copied to `environment.properties` |
| `promote_parameters` | `[]` | step parameter names copied to test parameters |
| `mask_parameters` | `[]` | names shown as `[MASKED]` and excluded from raw identity |
| `hide_parameters` | `[]` | names shown as `[HIDDEN]` and excluded from raw identity |
| `exclude_history_parameters` | `[]` | mark parameters excluded from history |
| `redact` | `[]` | literal values replaced by `[REDACTED]` in message attachments |
| `executor_auto` | `true` | auto-detect CI provider |
| `executor` | `{}` | safe explicit name/type/build/report overrides |
| `categories_file` | empty | project-relative strict Allure categories JSON, max 1 MiB, no symlink |
| `categories_profile` | `none` | `none` or `gauge-default` |
| `source_link_template` | empty | source URL template with path then line placeholders |
| `maximum_nesting_depth` | `100` | concept recursion guard, range 1–1000 |
| `diagnostic_proto_dump` | `false` | attach final suite protobuf JSON; may expose secrets |

The machine-readable contract is [allure-report.schema.json](../schemas/allure-report.schema.json).

## Environment overrides

String overrides: `GAUGE_ALLURE_RESULTS_DIR`, `GAUGE_ALLURE_COMPATIBILITY`, `GAUGE_ALLURE_WRITE_MODE`, `GAUGE_ALLURE_LOG_LEVEL`, `GAUGE_ALLURE_CONTEXT_MODE`, `GAUGE_ALLURE_TEARDOWN_MODE`, `GAUGE_ALLURE_HOOK_MODE`, `GAUGE_ALLURE_CATEGORIES_FILE`, `GAUGE_ALLURE_CATEGORIES_PROFILE`, `ALLURE_LINK_ISSUE_PATTERN`, `ALLURE_LINK_TMS_PATTERN`, `ALLURE_LINK_LINK_PATTERN`.

Boolean overrides: `GAUGE_ALLURE_CLEAN_RESULTS`, `GAUGE_ALLURE_ALLOW_CLEAN_NON_OWNED`, `GAUGE_ALLURE_STRICT`, `GAUGE_ALLURE_ATTACH_SCREENSHOTS`, `GAUGE_ALLURE_ATTACH_MESSAGES`, `GAUGE_ALLURE_ATTACH_TABLES`, `GAUGE_ALLURE_FOLLOW_SYMLINKS`, `GAUGE_ALLURE_ALLOW_EXTERNAL_ATTACHMENTS`, `GAUGE_ALLURE_EXECUTOR_AUTO`, `GAUGE_ALLURE_DIAGNOSTIC_PROTO_DUMP`.

List overrides (comma or semicolon separated): `GAUGE_ALLURE_ENV_ALLOWLIST`, `GAUGE_ALLURE_PROMOTE_PARAMETERS`, `GAUGE_ALLURE_MASK_PARAMETERS`, `GAUGE_ALLURE_HIDE_PARAMETERS`, `GAUGE_ALLURE_EXCLUDE_HISTORY_PARAMETERS`, `GAUGE_ALLURE_REDACT`.

Numeric overrides: `GAUGE_ALLURE_MAX_ATTACHMENT_BYTES`, `GAUGE_ALLURE_MAX_NESTING_DEPTH`. Gauge’s `overwrite_reports` is honored only when the dedicated cleaning override is absent.

## Output safety

The writer creates `.gauge-allure-report-owned` in its destination. Cleaning refuses filesystem/project/home roots, `.git`, suspiciously shallow paths, symlink destinations, and non-owned nonempty directories by default. It deletes only `*-result.json`, `*-container.json`, `*-attachment.*`, `environment.properties`, `executor.json`, and `categories.json`; unrelated files and subdirectories are never recursively removed.

Validate without executing tests:

```console
allure-report validate-config .gauge/allure-report.yaml
allure-report doctor
```
