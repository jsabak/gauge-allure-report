# SPDX-License-Identifier: Apache-2.0

"""Semantic assertions for native Allure result directories."""

import argparse
import json
from pathlib import Path


def load_many(root, suffix):
    return [json.loads(path.read_text(encoding="utf-8")) for path in sorted(root.glob(f"*{suffix}"))]


def walk_steps(steps):
    for value in steps or []:
        yield value
        yield from walk_steps(value.get("steps"))


def attachment_sources(results, containers):
    sources = []
    for result in results:
        sources.extend(item.get("source") for item in result.get("attachments", []))
        for step in walk_steps(result.get("steps")):
            sources.extend(item.get("source") for item in step.get("attachments", []))
    for container in containers:
        for fixture in container.get("befores", []) + container.get("afters", []):
            sources.extend(item.get("source") for item in fixture.get("attachments", []))
            for step in walk_steps(fixture.get("steps")):
                sources.extend(item.get("source") for item in step.get("attachments", []))
    return [source for source in sources if source]


def assert_main_python(root, results, containers):
    statuses = {item.get("status") for item in results}
    assert {"passed", "failed", "broken", "skipped"} <= statuses, statuses
    assert len(results) >= 15, len(results)  # 14 final scenarios plus the failed retry attempt
    assert len(containers) >= 3
    for item in results:
        assert item.get("uuid") and item.get("testCaseId") and item.get("historyId")
        assert item.get("start", 0) <= item.get("stop", 0)

    passing = next(item for item in results if item.get("name") == "Passing scenario" and item.get("status") == "passed")
    assert [step["name"] for step in passing.get("steps", [])] == ["Context", "Steps", "Teardown"]
    nested = next(item for item in results if item.get("name") == "Nested concepts")
    flattened = list(walk_steps(nested.get("steps")))
    assert any(step.get("name", "").startswith("Outer operation") and step.get("steps") for step in flattened)
    assert any(step.get("name", "").startswith("Inner operation") and step.get("steps") for step in flattened)

    retry_results = [item for item in results if item.get("name") == "Retry followed by pass"]
    assert len(retry_results) == 2
    assert len({item["historyId"] for item in retry_results}) == 1
    assert any(any(p.get("name") == "retry" and p.get("excluded") for p in item.get("parameters", [])) for item in retry_results)

    spec_rows = [item for item in results if item.get("name") == "Specification table row"]
    assert len(spec_rows) == 2
    parameters = [p for item in spec_rows for p in item.get("parameters", [])]
    assert any(p.get("name") == "password" and p.get("value") == "[MASKED]" for p in parameters)
    assert any(p.get("name") == "token" and p.get("value") == "[HIDDEN]" for p in parameters)
    assert any(p.get("name") == "build_timestamp" and p.get("excluded") for p in parameters)

    assert any(any(label.get("name") == "owner" and label.get("value") == "reporter-team" for label in item.get("labels", [])) for item in results)
    assert any(any(link.get("type") == "issue" for link in item.get("links", [])) for item in results)
    children = {child for container in containers for child in container.get("children", [])}
    assert {item["uuid"] for item in results if item.get("name") != "Retry followed by pass"} <= children

    sources = attachment_sources(results, containers)
    assert sources
    assert all((root / source).is_file() for source in sources)
    assert (root / "environment.properties").is_file()
    assert "TEST_REGION=local-e2e" in (root / "environment.properties").read_text(encoding="utf-8")
    assert (root / "categories.json").is_file()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=["main-python", "main-js", "parse", "missing", "no-match"])
    parser.add_argument("results")
    args = parser.parse_args()
    root = Path(args.results)
    if args.mode == "no-match" and not root.exists():
        print("Gauge Core did not start reporters for the zero-match filter (known boundary).")
        return
    assert root.is_dir(), root
    results = load_many(root, "-result.json")
    containers = load_many(root, "-container.json")
    assert results, "no Allure test results"
    if args.mode == "main-python":
        assert_main_python(root, results, containers)
    elif args.mode == "main-js":
        assert len(results) == 1 and results[0].get("status") == "passed"
        assert len(containers) >= 3
    elif args.mode == "parse":
        assert any(item.get("status") == "broken" and "Parse error" in item.get("name", "") for item in results)
    elif args.mode == "missing":
        assert any(item.get("status") == "broken" and "Validation error" in item.get("name", "") for item in results)
    else:
        assert any(item.get("status") == "skipped" and "No matching" in item.get("name", "") for item in results)
    print(f"validated {args.mode}: {len(results)} results, {len(containers)} containers")


if __name__ == "__main__":
    main()
