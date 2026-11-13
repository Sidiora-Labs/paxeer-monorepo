# State comparison tests

This package compares the V2 and Giga execution paths inside a Paxeer node.
The harness signs and encodes fixture transactions, submits them through
`ProcessBlock`, reads keepers and receipt codecs, and checks both executors
against explicit fixture expectations. It runs in process without a live service.

## Select a fixture

Run from the repository root:

```bash
STATE_TEST_ROOT=data/qualification STATE_TEST_DIR=transfer \
  STATE_TEST_NAME=transferValid STATE_TEST_FORK=Cancun STATE_TEST_COUNT=1 \
  IGNORE_SKIP_LIST=true VERIFY_ETHEREUM_SPEC=true USE_REGULAR_STORE=false \
  go test -count=1 -v -run '^TestGigaVsV2_StateTests$' ./engine/tests/
```

The fixture root is relative to `engine/tests`, the Go package working directory.
This selects `transfer/paxeerTransfer.json/transferValid` and requires
`selected=1 executed=1 required=1 skipped=0`. The fixture specifies a zero-price
transfer of 123 wei with sender/recipient balances, nonces, code, storage and the
empty RLP logs hash.

`STATE_TEST_ROOT` is optional. When unset, the loader uses
`data/GeneralStateTests`, extracting `data/fixtures_general_state_tests.tgz`
when that directory is absent. An explicit root selects that directory directly.

`STATE_TEST_DIR`, `STATE_TEST_NAME`, `STATE_TEST_FORK` and `STATE_TEST_COUNT` are
required. The name is a literal substring of the full loader/post-case name.
A name ending in `/` followed by a decimal post index must match the complete
loader/post-case name exactly; `/4` does not also select `/40` through `/49`.
Only `Cancun` post selections are supported. Missing posts, empty selections,
count mismatches and Go subtest filters that prevent execution cause an error.
Each selected identifier is printed as `Nonexcluded fixture`, followed by
selected, executed, required and skipped counts.
The loader adds a post index only when a fixture has multiple posts for the
selected fork; a single-post fixture is selected without a trailing `/0`.

Both `IGNORE_SKIP_LIST` and `VERIFY_ETHEREUM_SPEC` must be the literal `true`.
Category and case exclusions are not consulted by this entry point.
`USE_REGULAR_STORE=true` selects Giga execution with the regular KV store.

## Execution configuration

Local `pax-test` transactions are re-signed for EVM chain ID `125`, matching the
keeper's default chain ID. Public `hyperpax_125-1` also maps to EVM chain ID
`125`. The fixtures execute in local test state.

`NewStateTestContext` sets `MinimumFeePerGas` to zero in both EVM keepers.
Execution uses the fixed block time `time.Unix(1_800_000_000, 0).UTC()` and the
node test wrapper's block header. The fixture's `env` fields do not configure
that execution context. The node's fork schedule, opcode gas costs and selected
state store apply; selecting `Cancun` chooses fixture posts.

## Fixture oracle

For successful execution, each executor must match explicit `post.state` and
`post.logs`. Balances combine bank `uhpx * 10^12` with the fractional wei store.
Checks cover balances, nonces, code, declared storage keys, and the Keccak hash
of RLP-encoded receipt logs. The harness also compares both executors' result
codes, gas usage and post-state.

Ethereum's `post.hash` is an MPT commitment. Paxeer uses a Cosmos store
commitment, so this harness compares explicit accounts and logs instead of that
root. It does not enumerate undeclared accounts or storage slots. Successful
execution without explicit post-state cannot pass the fixture oracle.

Expected exceptions require matching classified outcomes from both executors.
One-sided or differently classified outcomes, unknown exception kinds, missing
results, unexpected execution errors and block-processing errors do not satisfy
an expected exception.

See [state_test.go](state_test.go), [state_harness_test.go](state_harness_test.go)
and [the fixture loader](harness/loader.go) for selection and execution.
