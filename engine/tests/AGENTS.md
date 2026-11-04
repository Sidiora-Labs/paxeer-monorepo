# State Tests Documentation

## Overview

This directory compares the Giga and V2 EVM execution paths through local node
applications. [state_test.go](state_test.go) selects the configuration;
[state_harness_test.go](state_harness_test.go) builds pre-state, submits encoded
transactions through `ProcessBlock`, and compares results. These are execution
modes inside the node, not separate networks. Paxeer mainnet-beta is one BFT
network whose kernel participates in block proposal; its EVM chain ID is `125`.
These local comparisons do not establish public-network consensus or finality.

- **Fixture archive:** `engine/tests/data/fixtures_general_state_tests.tgz`
- **Extracted fixtures:** `engine/tests/data/GeneralStateTests/`
- **Existing exclusion inventory:** [data/skip_list.json](data/skip_list.json)

[harness/loader.go](harness/loader.go) extracts the archive with `tar` when the
fixture directory is absent. Run the commands below from the repository root;
Go runs the package tests with `engine/tests` as their working directory. No
public endpoint is needed.

[harness/builder.go](harness/builder.go) rebuilds and signs transactions using
`config.DefaultChainID` from
[modules/evm/config/config.go](../../modules/evm/config/config.go), currently
`713714`. This local harness default must not be used as the mainnet chain ID.
The harness prefers `Cancun` posts; if absent, it takes the first fork encountered
in the fixture's Go map. That fallback is not a deterministic fork selection.

## Running Tests

### Basic Commands

Select a real category explicitly. The test calls `t.Skip` when `STATE_TEST_DIR`
is unset; the bare `go test` command does **not** run every category. The following
commands bypass existing exclusions and request the available fixture checks.
They are examples of explicit selections, not recorded passing gates.

```bash
# Compare the stExample category without the exclusion inventory.
STATE_TEST_DIR=stExample STATE_TEST_NAME= IGNORE_SKIP_LIST=true \
  VERIFY_ETHEREUM_SPEC=true USE_REGULAR_STORE=false \
  go test -count=1 -v -run '^TestGigaVsV2_StateTests$' ./engine/tests

# Compare only the add11.json fixture, not add11_yml.json.
STATE_TEST_DIR=stExample STATE_TEST_NAME='add11.json::' IGNORE_SKIP_LIST=true \
  VERIFY_ETHEREUM_SPEC=true USE_REGULAR_STORE=false \
  go test -count=1 -v -run '^TestGigaVsV2_StateTests$' ./engine/tests

# Diagnostic alternative: Giga executor with the regular KVStore.
STATE_TEST_DIR=stExample STATE_TEST_NAME='add11.json::' IGNORE_SKIP_LIST=true \
  VERIFY_ETHEREUM_SPEC=true USE_REGULAR_STORE=true \
  go test -count=1 -v -run '^TestGigaVsV2_StateTests$' ./engine/tests
```

The switches in [state_test.go](state_test.go) and
[state_harness_test.go](state_harness_test.go) behave as follows:

| Variable | Behavior |
| --- | --- |
| `STATE_TEST_DIR` | Category relative to `GeneralStateTests`; required to execute comparisons. |
| `STATE_TEST_NAME` | Case-sensitive substring of the loader's relative-path/JSON-key name, not a regular expression. Empty means no name filter. |
| `IGNORE_SKIP_LIST=true` | Bypasses both category and individual exclusions. The JSON inventory is still loaded and must decode successfully. |
| `VERIFY_ETHEREUM_SPEC=true` | Enables checks of explicit `post.state` accounts when present. It is off by default. |
| `USE_REGULAR_STORE=true` | Selects `ModeGigaWithRegularStore`; otherwise the comparison uses `ModeGigaSequential`. V2 remains sequential. |

Boolean switches enable their behavior only for the literal value `true`.
`-count=1` avoids reusing Go's cached results.

### Confirming That Cases Executed

Inspect the subtest output and `=== Overall: ... ===` summary. For a repair,
record the intended case names and expected count before running, and require
that all those cases executed with zero failures and zero skips. The summary's
`total` includes skipped cases; `passed + failed` counts recorded comparisons.
A fixture JSON key can produce several indexed post cases. The `Found ... state
test files` message actually counts loaded JSON-key entries, not executed cases.

A zero exit code alone is insufficient. The harness can skip an excluded
category or an empty directory; an unmatched name filter or fixtures with no
post cases can also produce zero comparisons. A missing directory instead
fails loading. If the summary is absent, empty, or smaller than the declared
inventory, do not report qualification. Inspect the full log for setup failures,
panics and `SKIP` entries as well.

### Limits of the Comparison

The ordinary comparison checks result codes and gas, then balances in `uhpx`,
nonces and code for pre-state addresses, plus storage keys present in pre-state.
It does not enumerate every newly created account or storage key.

With `VERIFY_ETHEREUM_SPEC=true`, explicit `post.state` entries supply additional
balance, nonce, code and storage expectations. The current implementation logs
V2 fixture differences and returns a failure for Giga fixture differences; it
does not check the fixture's `hash` or `logs` fields. No fixture-state check runs
when `post.state` is empty. Balance verification currently compares the bank's
`uhpx` integer directly with the fixture balance, so review units and the chain
ID/fork adaptations when investigating an oracle failure. Do not change fixture
expectations merely to make a local result pass.

The exception branch can report success when only one executor fails, and the
common-error branch accepts two execution errors without classifying them.
Those outcomes do not prove matching expected exceptions. Keep these limits
visible when interpreting a green run; enabling fixture checks does not repair
them or prove full Ethereum conformance.

## Running Tests in Parallel (Best Practices)

Start with one bounded selection in one process. The suite creates real node
applications and stores for each comparison; concurrent processes consume more
resources. There is no enforced safe process count in this harness, and the old
category timeout and pass-rate lists are not current measurements.

Use the owning task's declared timeout and resource budget. Do not extend a
timeout to conceal a hang, or substitute a smaller selection for a recorded
failing inventory. If separately authorized diagnostic runs are parallelized,
keep their logs separate and collect every process's exit status.

## Extracting Failing Test Names

Keep the complete log in a private evidence directory. If using `tee`, enable
Bash `pipefail` so its success cannot hide a failing `go test`. For example, with
`STATE_LOG_DIR` set to an existing private directory:

```bash
set -o pipefail
STATE_TEST_DIR=stExample STATE_TEST_NAME= IGNORE_SKIP_LIST=true \
  VERIFY_ETHEREUM_SPEC=true USE_REGULAR_STORE=false \
  go test -count=1 -v -run '^TestGigaVsV2_StateTests$' ./engine/tests \
  2>&1 | tee "${STATE_LOG_DIR:?set a private evidence directory}/stExample.log"
```

Read the exit status immediately and retain the exact command and revision.
These searches help navigate that log; they do not establish a passing result:

```bash
rg -- '--- (PASS|FAIL|SKIP):|=== Overall:' "$STATE_LOG_DIR/stExample.log"
rg '^\s+- stExample/' "$STATE_LOG_DIR/stExample.log"
rg 'result_code|gas_mismatch|state_mismatch|balance_mismatch|error_mismatch' \
  "$STATE_LOG_DIR/stExample.log"
```

The summary lists at most 50 individual failures. Use the full subtest output
for a complete inventory; counting `PASS` lines also counts parent tests and is
not an executed-case count.

## Skip List Format

The existing [data/skip_list.json](data/skip_list.json) describes exclusions from
diagnostic runs. It is not a repair ledger or evidence that excluded behavior
works. Do not add failures to it to obtain a green qualification run.

The inventory uses these fields; [harness/skip.go](harness/skip.go) consumes
only the first two:

| Field | Meaning |
| --- | --- |
| `skipped_tests` | Map from test identifier to reason. |
| `skipped_categories` | Entire category names excluded by default. |
| `skipped_category_reasons` | Existing explanatory metadata; the loader does not decode or enforce it. |

### Test Name Format for `skipped_tests`

The full identifier is `category/relPath/FullJSONKey`, followed by `/index` only
when the selected fork has multiple post entries. `relPath` is relative to the
category directory and includes the JSON filename. `FullJSONKey` is the literal
key inside that file and can itself contain slashes and brackets. Preserve it
exactly when recording a failure; use the harness output rather than guessing.

Matching also accepts a bare test identifier for compatibility. A
`category/shortName` entry can match names beginning with `shortName.json` or
`shortName/`, or exactly equal to `shortName`. Category exclusions take precedence.
These are exact/prefix rules, not glob or regular-expression matching, and a
short entry can exclude several cases.

### Per-test vs Per-category Skipping

Both forms exclude execution. A run using either form can support diagnosis of
the remaining cases, but cannot qualify an excluded failing target under
requirement `22998.1`. Use `IGNORE_SKIP_LIST=true` to expose existing exclusions
while repairing their real execution paths. Remove a resolved exclusion only
after the original active case passes with unchanged assertions; retain its
failure identity and evidence in the owning repair record.

## Failure Types

The comparison declares these diagnostic labels:

| Type | Description |
| --- | --- |
| `result_code` | V2 and Giga transaction result codes differ. |
| `gas_mismatch` | Reported gas usage differs. |
| `state_mismatch` | A compared storage value differs. |
| `balance_mismatch` | A compared balance differs. |
| `code_mismatch` | Compared account code differs. |
| `nonce_mismatch` | Compared account nonce differs. |
| `error_mismatch` | The expected-exception branch found both executors successful. |
| `v2_error` | V2 returned an execution error while Giga did not. |
| `giga_error` | Giga returned an execution error while V2 did not. |
| `unknown` | Other failure, including transaction construction/encoding or result-count mismatch. |

Inventory reason strings are free-form metadata and need not be one of these
labels. A label alone does not identify the underlying defect.

## Test Categories Status

Consult the fixture archive for available categories and the exclusion inventory
for existing diagnostic exclusions. Neither establishes a current pass rate.
Report category status only with a named revision, explicit selection, executed
counts and the corresponding log; historical percentages are not qualification
evidence.

## Workflow for Analyzing New Categories

1. Declare the category, literal JSON keys, selected fork/post cases, expected
   executed count and original failing command. Check that the fixtures exist
   and contain those posts. Preserve the recorded failing selection.
2. Run the explicit selection with exclusions bypassed and available fixture
   checks enabled, subject to the owning task's gate and timeout rules. Retain
   the full log and exit code; a diagnostic store-mode run is additional evidence,
   not a substitute for the original execution configuration.
3. Investigate divergences in the transaction builder, node execution paths or
   comparison/oracle code. Repair the responsible implementation within the
   owning task's scope. Do not add exclusions, skip/delete tests, weaken
   assertions, or relabel an unexpected failure as expected.
4. Rerun the original recorded command under the task's bounded repair policy.
   Require every declared case to execute and pass without skips or weakened
   expectations. Check fixture-oracle coverage and the exception/error limits
   above before describing what the result proves.
5. Record the verified revision, exact command and environment, exit code,
   expected/executed/passed/failed/skipped counts, and private log location in the
   owning repair evidence. Resolve an existing exclusion only with that evidence.
   If execution is empty, incomplete or blocked by a harness limitation, report
   it as unqualified and leave the unresolved behavior visible.
