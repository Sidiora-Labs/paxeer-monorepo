# State comparison qualification

This package compares the real V2 and Giga execution paths inside one Paxeer
node. The harness signs and encodes fixture transactions, submits them through
`ProcessBlock`, reads the real keepers and receipt codecs, and checks both
executors against independent committed expectations. It needs no live service.

## Required bounded command

Run from the repository root:

```bash
STATE_TEST_ROOT=data/qualification STATE_TEST_DIR=transfer \
  STATE_TEST_NAME=transferValid STATE_TEST_FORK=Cancun STATE_TEST_COUNT=1 \
  IGNORE_SKIP_LIST=true VERIFY_ETHEREUM_SPEC=true USE_REGULAR_STORE=false \
  go test -count=1 -v -run '^TestGigaVsV2_StateTests$' ./engine/tests/
```

The fixture root is relative to `engine/tests`, the Go package working directory.
This command selects exactly `transfer/paxeerTransfer.json/transferValid`, executes
one comparison through both executors, and requires `selected=1 executed=1
required=1 skipped=0`. The unchanged fixture specifies a zero-price transfer of
123 wei and complete sender/recipient balances, nonces, code, storage and the
empty RLP logs hash. It is a Paxeer fixture, not a claim that the historical
Ethereum inventory has been repaired. `TestRepairM_1180_RequiredSelection` drives
the same suite entry point and command configuration inside the bounded task gate.

`STATE_TEST_DIR`, `STATE_TEST_NAME`, `STATE_TEST_FORK` and `STATE_TEST_COUNT` are
required. The name is a literal substring of the full loader/post-case name.
A name ending in `/` followed by a decimal post index must match the complete
loader/post-case name exactly; `/4` does not also select `/40` through `/49`.
Only the declared `Cancun` post selection is supported; there is no map-order
fork fallback. Missing posts, empty selections, count mismatches and Go subtest
filters that prevent execution fail qualification. Every selected identifier is
printed as `Nonexcluded fixture`, followed by selected/executed/required counts.
Both `IGNORE_SKIP_LIST` and `VERIFY_ETHEREUM_SPEC` must be the literal `true`.
Neither category nor case exclusions are consulted during qualification.
`USE_REGULAR_STORE=true` selects an additional diagnostic store mode; it cannot
replace the recorded original execution configuration for a repair.

## Supported oracle and adaptations

Local `pax-test` transactions are re-signed for chain ID `713714`, matching the
real keeper, rather than Ethereum fixture chain ID 1. Public `hyperpax_125-1`
maps to chain ID 125. This bounded command certifies the local harness only.
Paxeer's real fork activation schedule, fee charging, custom opcode gas costs
and state store stay enabled. Cancun identifies the required fixture posts;
it does not override Paxeer's production fork schedule or block environment.
Fixtures using Ethereum-specific environmental, fee or gas assumptions can fail;
keep their expectations unchanged and route the discrepancy to the owner.

The supported fixture gate requires explicit `post.state` and `post.logs`.
Balances combine bank `uhpx * 10^12` with the fractional wei store. Both executors
must match fixture balances, nonces, code and every declared storage key, as well
as the Keccak hash of RLP-encoded receipt logs. Differences from either executor
fail. Agreement between two wrong local states is insufficient.

Ethereum's `post.hash` is an MPT commitment and is not Paxeer's Cosmos store
commitment; this gate checks explicit accounts and logs, not that incompatible
root. It does not claim exhaustive enumeration of undeclared accounts or slots.
A fixture without explicit state cannot pass as oracle-qualified.
Expected exceptions require both executors to fail with the same classified,
expected kind. A one-sided failure, different kind, unknown kind, missing result,
or unexpected execution error fails. Infrastructure errors are never expected
fixture exceptions.

## Diagnostic inventory and owned repairs

`data/skip_list.json` preserves 101 historical case exclusions and 23 category
exclusions. It is diagnostic history, not a qualification input or a passing
inventory. `data/state_repairs.json` owns every entry in domain 5 and retains its
literal identity and reason. Case entries include counted active replay commands.
The historical file contains no original failure commands or verified revisions;
those fields remain explicitly missing. Recover them from private audit evidence.
Category entries also need their original bounded fixture selection reconstructed.
No historical exclusion is resolved by the new bounded transfer selection.

Never add an exclusion as a repair. Never change a fixture expectation, skip or
delete a test, accept an unexpected error, or disable the independent oracle to
obtain a pass. Remove only a resolved exclusion whose original active selection
passes unchanged. For each resolved entry retain the original failure command,
exact nonexcluded fixture/post inventory, expected count, executed count, passed /
failed / skipped counts, verified revision, exit status and private evidence log.
The authoritative revision and gate evidence come from the runner's `hf-build`
and `hf-done` ledger. A build or an empty green test run is not qualification.

## Task gate and scope

Task 22.33.1180 runs only `TestRepairM_1180_*` through `hf-done` after `hf-build`.
Those tests use committed fixtures, real applications/stores/codecs and the same
suite selection logic. They do not invoke another Go toolchain or test process.
Fixture archive extraction uses Go's archive/gzip packages, not an external tar.
See `repair_m1180.kvx` for finding-to-test mappings, unresolved production wiring
requests and the baseline-order conflict. Do not edit another domain's module
to repair an inventory entry under this task's scope.
