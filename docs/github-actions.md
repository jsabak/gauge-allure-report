# GitHub Actions

The repository workflows pin actions by full commit SHA and grant read-only permissions unless a job explicitly releases or attests artifacts. A consumer workflow can install a release ZIP and upload native results even when Gauge fails:

```yaml
name: gauge
on: [push, pull_request]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
      - name: Install reporter
        run: gauge install allure-report --file ./allure-report-0.1.0-linux.x86_64.zip
      - name: Run Gauge
        run: gauge run specs
      - name: Upload native results
        if: always()
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7
        with:
          name: allure-results
          path: reports/allure-results
          if-no-files-found: error
```

The reporter auto-detects `GITHUB_ACTIONS`, repository, run number/attempt, workflow, server URL, and run ID to create `executor.json`. Only explicitly allowlisted environment values enter `environment.properties`.

For a hosted report, download the artifact in a trusted workflow and generate static HTML with a pinned Allure CLI. Do not expose pull-request secrets to untrusted fork code and do not use `pull_request_target` to execute a fork checkout.

See [examples/github-actions/gauge.yml](../examples/github-actions/gauge.yml) and the repository’s own `.github/workflows` for build, E2E, nightly compatibility, security, documentation, release preparation, and tagged release jobs.
