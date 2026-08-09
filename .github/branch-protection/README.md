# Recommended branch protection

For the default branch:

- require pull requests and at least one approving review;
- dismiss stale approvals and require review of the latest push;
- require conversation resolution;
- require signed commits or vigilant-mode verification where practical;
- require linear history;
- block force pushes and deletion;
- require `CI / test`, `CI / race`, `CI / build`, `Security / dependency-review`, and `Docs / validate` after those workflows exist;
- require branches to be up to date before merge;
- restrict bypass to emergency maintainers and audit every bypass;
- require deployment/release environment approval for public releases;
- enable private vulnerability reporting, secret scanning, push protection, Dependabot alerts, and dependency graph.

Repository settings cannot be guaranteed from source control. A maintainer must apply and periodically audit these settings. Do not mark checks required until their workflow names have run on the default branch, or merging can deadlock.
