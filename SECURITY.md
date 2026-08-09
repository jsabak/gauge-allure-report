# Security policy

## Supported versions

| Version | Security fixes |
| --- | --- |
| latest `0.x` release | supported |
| older pre-1.0 releases | best effort; upgrade is normally required |
| unreleased default branch | not a stable support target |

## Report privately

Use **GitHub → Security → Advisories → New draft security advisory** for the repository. If that channel is unavailable, contact `jan.sabak@amberteam.pl` with the subject `SECURITY: gauge-allure-report`. Do not file a public issue.

Include affected version/platform, reproduction, impact, attack prerequisites, relevant logs with secrets removed, and any proposed mitigation. Do not attach real credentials or sensitive test results.

The project aims to acknowledge a report within 5 business days, coordinate validation and remediation privately, and publish a security advisory with credit when a fix is available. Timelines depend on severity and maintainer availability; no bounty program is promised.

## Scope priorities

Path traversal, unsafe deletion, symlink escape, attachment exfiltration, secret leakage, arbitrary code execution, exposed network listeners, release artifact substitution, and protocol resource exhaustion are in scope. Ordinary test failures, publicly known dependency issues without project impact, and social engineering without a technical flaw are generally out of scope.
