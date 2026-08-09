# Label taxonomy

| Label | Purpose |
| --- | --- |
| `type:bug` | incorrect existing behavior |
| `type:feature` | new capability |
| `type:compatibility` | Gauge/runner/Allure/platform matrix gap |
| `type:documentation` | documentation only |
| `type:security` | public coordination after private triage; never initial vulnerability disclosure |
| `area:protocol` | Gauge gRPC/protobuf lifecycle |
| `area:mapping` | Allure hierarchy/status/identity |
| `area:attachments` | files, messages, privacy |
| `area:packaging` | binary, ZIP, distribution |
| `area:ci` | automation and compatibility matrix |
| `status:needs-triage` | not yet accepted/scoped |
| `status:blocked-upstream` | cannot proceed without an upstream change |
| `status:ready` | implementation-ready |
| `priority:critical` | security/data loss/release integrity |
| `priority:high`, `priority:normal`, `priority:low` | scheduling signal |
| `good first issue` | bounded and documented contributor entry point |

Labels are documentation until created in repository settings. Automation must not assume a label exists before bootstrap.
