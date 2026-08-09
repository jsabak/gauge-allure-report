# SPDX-License-Identifier: Apache-2.0

from getgauge.python import step


@step("A filtered passing step")
def filtered_step():
    pass
