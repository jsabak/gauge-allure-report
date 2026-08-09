# SPDX-License-Identifier: Apache-2.0

import base64
import os
from pathlib import Path

from getgauge.exceptions import SkipScenarioException
from getgauge.python import (
    Messages,
    Screenshots,
    after_scenario,
    after_spec,
    after_step,
    after_suite,
    before_scenario,
    before_spec,
    before_step,
    before_suite,
    continue_on_failure,
    custom_screenshot_writer,
    step,
)


_TINY_PNG = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
)
_retry_attempts = {}


@custom_screenshot_writer
def write_screenshot():
    target = Path(os.environ.get("gauge_screenshots_dir", ".")) / "tiny.png"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(_TINY_PNG)
    return str(target.resolve())


@before_suite
def suite_setup():
    Messages.write_message("before suite message")


@after_suite
def suite_teardown():
    Messages.write_message("after suite message")


@before_spec
def spec_setup():
    Messages.write_message("before spec message")


@after_spec
def spec_teardown():
    Messages.write_message("after spec message")


@before_scenario
def scenario_setup():
    Messages.write_message("before scenario message")


@after_scenario
def scenario_teardown():
    Messages.write_message("after scenario message")


@before_scenario("<hook-failure>")
def deliberately_failing_hook():
    raise RuntimeError("deliberate scenario hook failure")


@before_step
def step_setup():
    Messages.write_message("before step message")


@after_step
def step_teardown():
    Messages.write_message("after step message")


@step("A passing step")
def passing_step():
    pass


@step("A passing value <value>")
def passing_value(value):
    assert value, "expected a non-empty value"


@step("Assertion fails")
def assertion_fails():
    assert 1 == 2, "expected 1 to equal 2"


@step("Unexpected exception occurs")
def unexpected_exception():
    raise RuntimeError("deliberate unexpected Python exception")


@step("Skip this scenario")
def skip_this_scenario():
    raise SkipScenarioException("deliberate programmatic skip")


@continue_on_failure([AssertionError])
@step("Recoverable assertion fails")
def recoverable_failure():
    assert False, "expected recoverable assertion"


@step("Retry once under key <key>")
def retry_once(key):
    _retry_attempts[key] = _retry_attempts.get(key, 0) + 1
    assert _retry_attempts[key] > 1, "expected the first retry attempt to fail"


@step("Emit a message and screenshot")
def message_and_screenshot():
    Messages.write_message("step message with harmless local data")
    Screenshots.capture_screenshot()


@step("Context is ready")
def context_ready():
    pass


@step("Teardown is recorded")
def teardown_recorded():
    pass


@step("Observe account <account_id> password <password> token <token> build <build_timestamp>")
def observe_spec_row(account_id, password, token, build_timestamp):
    assert all((account_id, password, token, build_timestamp)), "expected complete table row"


@step("Observe currency <currency> amount <amount>")
def observe_scenario_row(currency, amount):
    assert currency and int(amount) > 0, "expected a valid scenario row"


@step("Observe static value <value>")
def observe_static(value):
    assert value == "literal", "expected literal static parameter"


@step("Observe multiline text")
def observe_multiline(text):
    assert "first line" in text and "second line" in text, "expected both multiline values"


@step("Observe item table <table>")
def observe_table(table):
    assert table.headers == ["item", "count"], "expected table headers"
    assert len(table.rows) == 2, "expected two table rows"
