# Gauge Python

Gauge Core and Gauge Python are separate processes. Core sends step execution requests to the Python runner and independently sends reporter lifecycle messages to `allure-report`. Your step files therefore need only `getgauge`; do not install `allure-pytest` for this reporter.

## Minimal project

```json
{
  "Language": "python",
  "Plugins": ["allure-report"]
}
```

```text
project/
├── manifest.json
├── requirements.txt
├── env/default/python.properties
├── specs/example.spec
└── step_impl/steps.py
```

Pin the runner package to the plugin version:

```text
getgauge==0.5.1
```

Install and run:

```console
gauge install python
gauge install allure-report
python -m pip install -r requirements.txt
gauge run specs
```

See the runnable [example](../examples/gauge-python/) and exhaustive [E2E fixture](../test/e2e/gauge-python/projects/main/).

## Messages, screenshots, skips, and recoverable failures

The ordinary Gauge APIs flow through the reporter protocol:

```python
from getgauge.exceptions import SkipScenarioException
from getgauge.python import Messages, Screenshots, continue_on_failure, step

@step("Record a message")
def record_message():
    Messages.write_message("diagnostic text")
    Screenshots.capture_screenshot()

@step("Skip locally")
def skip_locally():
    raise SkipScenarioException("precondition is not available")

@continue_on_failure([AssertionError])
@step("Verify softly")
def verify_softly():
    assert False, "expected verification"
```

Register a `custom_screenshot_writer` when no desktop screenshot plugin is suitable. Gauge Python returns only the screenshot basename, so write the file into `gauge_screenshots_dir`; the reporter searches that Gauge-managed directory securely.

## Failure classification caveat

Gauge Python 0.5.1 sets `ProtoExecutionResult.errorType=ASSERTION` for every caught exception. An `AssertionError` diagnostic maps to `failed`; a `RuntimeError` or another unexpected diagnostic maps to `broken`; recoverable execution maps to `failed`. Ambiguous diagnostics are conservatively `broken`.

## Parallel execution and retries

Use normal Gauge flags:

```console
gauge run specs -p -n 2 --max-retries-count 2 --retry-only retry
```

`--max-retries-count` counts total attempts. Core’s final suite stores that one-based count while lifecycle callbacks expose `currentRetry` as zero-based; the reporter normalizes both forms. Fixture associations use protocol stream/runner identity and test UUIDs, not process-global Python state.

## Supported Python

Gauge Python’s upstream CI currently exercises Python 3.10 and 3.14 on Linux, Windows, and macOS; 0.5.1 was locally verified here on Python 3.14.6. See [compatibility.yaml](../compatibility.yaml) for evidence rather than assuming every intermediate interpreter/OS combination has been run locally.
