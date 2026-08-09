# Upstream baseline

Research was refreshed on **2026-08-07**. Commit pins below make design claims auditable; release versions are used where an upstream publishes them.

| Project | Baseline | License | What was verified |
| --- | --- | --- | --- |
| `getgauge/gauge` | `330124121881492b7dc3e6fb86fbbc56120d6e0c`; v1.6.35 / installed commit `e967358` | Apache-2.0 | plugin launch handshake, reporter callbacks, retries, duration units, final suite producer |
| `getgauge/gauge-proto` | `8a89f7a070a9590b36fd1087a75c3b5739091602` | Apache-2.0 | current Go module and 12 Reporter RPCs |
| `getgauge/xml-report` | `95abdd32478770370532a3b419f32f1af52513fa`; v1.0.1 | Apache-2.0 | native reporter packaging and minimum Gauge precedent |
| `getgauge/html-report` | `95774404de4f271d1ded94e5720863df49a381e4`; v4.4.6 | Apache-2.0 | current official reporter behavior and installation |
| `getgauge/gauge-python` | `9253d270e023056866a0f6ef30252ceea1a0ff6e`; v0.5.1 | MIT | process boundary, hooks, messages, screenshots, skip/recoverable behavior, error type limitation, Python 3.10/3.14 CI |
| `getgauge/gauge-js` | `04cf3ca5176e1cfe604e1b24a8fa05152faf9ef5`; v5.0.7 | MIT | independent runner process and default error-type behavior |
| `getgauge/gauge-repository` | `ef568445593e308783b7b122749d2e0b714bc75c` | repository metadata did not expose a license | distribution metadata shape; confirm current rules at submission time |
| `allure-framework/allure-go` | commons v1.2.1, release commit `5691b85adfe117ce6bfe6194d379f1e40261df40` | Apache-2.0 | official result/container models used by this project |
| `allure-framework/allure2` | `0d72728005b8f55e61b46a45e1cd428a1d14c22f`; 2.45.0 | Apache-2.0 | current Allure 2 generator target; official ZIP digest verified because the npm wrapper lags upstream |
| `allure-framework/allure3` | `814b17e3f2f5c38bd4f7ac491b43e7ced924e45a`; v3.14.3 | Apache-2.0 | current Allure 3 generator target and Allure 2 input compatibility |

## Protocol findings

The current Reporter service exposes `NotifyExecutionStarting`, `SpecStart`, `ScenarioStart`, `ConceptStart`, `StepStart`, `StepEnd`, `ConceptEnd`, `ScenarioEnd`, `SpecEnd`, `ExecutionEnd`, `SuiteResult`, and `Kill`. The reporter binds loopback/ephemeral and prints `Listening on port:<n>` to stdout.

Current Gauge Core sends one complete `SuiteExecutionResult` on the gRPC reporter path. The proto retains chunk flags and `SuiteExecutionResultItem`, but the inspected Core producer does not use them for this path. A regression test ensures an embedded suite payload is not discarded merely because `chunked=true` is present.

`executionTime` is milliseconds. `ProtoScenario.retriesCount` is the number of attempts (minimum one when executed), while live `ScenarioRetriesInfo.currentRetry` is zero-based. Gauge Python and JavaScript do not reliably distinguish unexpected exceptions via the protobuf error enum; diagnostics are necessary.

## Allure and CI findings

The official native format uses UUID result/container filenames plus referenced attachment sources. Allure 3’s migration guidance retains compatibility with official Allure 2 result files. Jenkins’ current syntax uses `results: [[path: '...']]`; Allure 3 selection adds `allureVersion: '3'`.

## Dependency policy

Runtime modules are pinned in `go.mod`/`go.sum`. The official Allure model is used through a narrow writer boundary; filesystem behavior is implemented locally for atomic replacement and guarded cleaning. Proto code is consumed from the published Gauge Go module rather than copied/generated into the repository.

Primary sources:

* <https://github.com/getgauge/gauge>
* <https://github.com/getgauge/gauge-proto>
* <https://github.com/getgauge/gauge-python>
* <https://github.com/getgauge/gauge-js>
* <https://github.com/getgauge/gauge-repository>
* <https://github.com/allure-framework/allure-go>
* <https://allurereport.org/docs/how-it-works/>
* <https://allurereport.org/docs/integrations-jenkins/>
