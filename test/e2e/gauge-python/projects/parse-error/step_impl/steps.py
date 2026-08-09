# SPDX-License-Identifier: Apache-2.0

from getgauge.python import step


@step("A harmless implemented step")
def harmless_step():
    pass
