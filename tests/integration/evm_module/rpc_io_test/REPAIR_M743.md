# Task 22.33.743 repair record

Acceptance: 22998.1. Audit: 22.30.172. Base revision:
`264615cba89456c37eeb19535cb16b81846f6a68`.

| Finding | Proving tests in repair_m743_test.go |
| --- | --- |
| io-aggregate-parity-guide | TestRepairM_743_ParityGuide, TestRepairM_743_EnabledInventory |
| io-unpaired-request | TestRepairM_743_UnpairedRequests |
| io-kind-only-schema | TestRepairM_743_MethodSchemas, TestRepairM_743_ErrorSchemas |
| io-tests-accept-wrong-result | TestRepairM_743_DeterministicValues, TestRepairM_743_MethodSchemas, TestRepairM_743_ErrorSchemas |
| io-enabled-skips | TestRepairM_743_EnabledInventory, TestRepairM_743_EnabledSetup |

These tests use the real parser, substitutions, HTTP client, response consumer
and inventory loader. Committed transcripts supply loopback HTTP responses;
negative cases corrupt one recorded field to demonstrate rejection. The
cross-check fixture exercises a produced block hash through binding,
substitution, HTTP transport and the reference consumer. No live node, opt-in,
credentials, services, subprocess, or native artifacts are required.

Baseline verification was not run: the task's ordered implementation-first
instructions and exclusive hf-build/hf-done gates prohibit an unchanged-tree
verify before implementation. No pre-change failing tests are claimed. The
initial gate ledger had no runs, and no implementation changes were inherited.

Qualification evidence is the runner's private gate ledger and final hf-done
JSON record: it names the exact verified candidate revision, command, exit code
and private log location. The runner publishes that record with this task;
this source record does not claim an unexecuted gate. The build and verify must
refer to the same candidate revision.

No repair proposal requires an out-of-path implementation change, so there are
no wiring requests. The task's scope prohibits editing spec/ and GOTCHA.kvx;
this record retains the baseline-instruction contradiction instead.

Fixtures with placeholder quantities such as `0x`, empty error expectations,
or deterministic values from another chain no longer qualify by response kind
alone. They now fail visibly rather than supplying a false conformance pass.
A debug selection is an explicitly named subset; the full committed inventory
is validated first and unknown debug names fail.
