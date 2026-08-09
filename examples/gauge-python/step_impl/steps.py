# SPDX-License-Identifier: Apache-2.0

from getgauge.python import Messages, step


items = []


@step("Add <item> to the list")
def add_item(item):
    items.append(item)
    Messages.write_message("Added a local example item")


@step("The list contains <item>")
def list_contains(item):
    assert item in items, "expected item to be present"
