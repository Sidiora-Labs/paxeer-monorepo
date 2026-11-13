# EVM RPC .io / .iox tests

Integration tests for `paxd` EVM JSON-RPC method-aware schemas, deterministic expectations and explicit fixture directives. The suite discovers `.io` and `.iox` files recursively under [`testdata/`](testdata/) and runs them as named subtests against a live endpoint. See the [integration overview](../../README.md) for the merged network and the [TypeScript RPC suite](../../rpc_tests/README.md) for value, schema and geth error comparisons.

## `.io` vs `.iox`

- **`.io`** - vanilla JSON-RPC fixtures (`>>` / `<<`) with no harness directives.
- **`.iox`** - same line format, plus harness extensions: `@ bind` / `<< @ ref_pair`, `@ expect_body_contains`, `@ expect_response_header`, and the `__SEED__` / `__REVERTER__` placeholders.

## How to run

1. Start the local cluster from the repo root: `make docker-cluster-start` (EVM RPC on port 8545; add `DOCKER_DETACH=true` to run it in the background).
2. Run the script from the repo root:
   ```bash
   ./tests/integration/evm_module/scripts/evm_rpc_tests.sh
   ```

Before `go test`, the [script](../scripts/evm_rpc_tests.sh) uses `docker exec` to associate the sender, send one EVM transfer and deploy a minimal contract and a reverter. It waits for sender sequence advancement between submissions and requires successful deployment receipts with matching hashes, nonzero contract addresses and positive block heights. These receipts supply the seed bindings described below. Any setup failure stops the script before either suite runs. The script runs this package with `PAX_EVM_IO_RUN_INTEGRATION=1`, then, only if that command succeeds, runs the sibling WebSocket package with `PAX_EVM_WS_RUN_INTEGRATION=1`. The CLI submissions use the local Cosmos chain ID `pax`. Use a disposable cluster with a funded signing key.

Script knobs:

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| `PAX_EVM_RPC_URL` | `http://localhost:8545` | RPC endpoint under test |
| `PAX_EVM_IO_TX_CONTAINER` | `pax-node-0` | Container used to send the seed transactions |
| `PAX_EVM_IO_TX_FROM` | `admin` | Key that signs the seed transactions |
| `PAX_EVM_IO_TX_PASSWORD` | Docker development keyring password | Must match the signing key inside the container |
| `PAX_EVM_IO_TX_RECIPIENT` | fixed test address | Recipient of the seed EVM transfer |
| `PAX_EVM_IO_PROJECT_ROOT` | `/pax-protocol/pax-chain` | Repository path inside the container containing the contract hex files |
| `PAX_EVM_IO_KEYRING_BACKEND` | unset | Passed as `--keyring-backend` when set |

The RPC URL must reach the same chain from both the host Go process and the container CLI/receipt queries. The Go package alone defaults to `http://127.0.0.1:8545`; the script uses `http://localhost:8545`.

For extra `[DEBUG]` output, set `PAX_EVM_IO_DEBUG_FILES` to comma-separated fixture paths relative to `testdata/`, such as `eth_call/call-contract.io`. Every selected name must exist; unknown or duplicate names fail. Matching names restrict execution to the selected fixtures. The full fixture inventory is validated before selection. When invoking the Go package directly, `PAX_EVM_IO_RUN_INTEGRATION` must equal `1`; any other value skips the integration test.

### Legacy `pax_*` gating

Gated legacy `pax_*` / `pax2_*` methods are controlled by `[evm].enabled_legacy_pax_apis` in `app.toml`. A blocked method returns HTTP 200 with JSON-RPC error code `-32601` and `error.data` equal to `legacy_pax_deprecated`. The default `paxd init` allowlist is `pax_getPaxAddress`, `pax_getEVMAddress` and `pax_getCosmosTx` (see [`rpc/config/config.go`](../../../../rpc/config/config.go)). The docker localnet `app.toml` enables every gated method except `pax_sign`.

Deprecation is asserted in `testdata/pax_legacy_deprecation/`:

- `pax_sign-disabled.iox` - gate errors carry `error.data` `legacy_pax_deprecated`.
- `deprecation-success.iox` - allowlisted calls succeed and carry the `Pax-Legacy-RPC-Deprecation` response header (body unchanged).
- `batch-nonobject-tail-gate.iox` - a JSON-RPC batch with `pax_sign` and a trailing non-object entry; asserts `legacy_pax_deprecated`, `Invalid Request` and `-32600` in the raw body (see [`rpc/pax_legacy_http.go`](../../../../rpc/pax_legacy_http.go)).

Directives used by these fixtures:

- `@ expect_body_contains substring` - response body must contain the substring.
- `@ expect_response_header Header-Name` - response must include that HTTP header (case-insensitive lookup).

### Comparing legacy vs giga (RPC parity)

Use this suite to compare per-case outcomes, method-aware schemas, deterministic values and directives under different local executor configurations. It cannot establish full RPC or execution parity.

The suite does not detect which executor a node uses. It only sends JSON-RPC to `PAX_EVM_RPC_URL`, so you choose the executor by how you start the node or which URL you pass.

Run a local cluster with giga enabled:

```bash
# All 4 nodes use giga (and OCC), foreground:
GIGA_EXECUTOR=true GIGA_OCC=true make docker-cluster-start

# Same, in the background so you can run the RPC test script:
GIGA_EXECUTOR=true GIGA_OCC=true DOCKER_DETACH=true make docker-cluster-start
# Wait until build/generated/launch.complete has 4 lines, then:
./tests/integration/evm_module/scripts/evm_rpc_tests.sh
```

Without `GIGA_EXECUTOR` and `GIGA_OCC`, the cluster uses the legacy (V2) executor. The node image applies these in `docker/localnode/scripts/step4_config_override.sh`.

Set `LEGACY_RPC_URL` and `GIGA_RPC_URL` to the endpoints under comparison, and
select a matching seed container for each invocation. Then:

1. Run the suite against the legacy endpoint and retain the complete verbose output and exit status:
   ```bash
   PAX_EVM_RPC_URL="$LEGACY_RPC_URL" ./tests/integration/evm_module/scripts/evm_rpc_tests.sh
   ```
   The report looks like:
   ```text
   ========== Pax EVM RPC .io/.iox test report ==========
     Total:  ...
     Passed: ...
     Failed: ...
     Skipped: ...
     Pass rate: ...%
   ```
2. Run the same fixture revision against the giga endpoint and retain its complete output and exit status:
   ```bash
   PAX_EVM_RPC_URL="$GIGA_RPC_URL" ./tests/integration/evm_module/scripts/evm_rpc_tests.sh
   ```
3. Match every `<fixture-path>#pair-N` identity from `case_inventory.json`, the named `TestEVMRPCSpec/<fixture-path>` outcomes, and pair-level diagnostics. Require the same executed identities and successful assertions on both endpoints. Compare the declared method schemas, exact deterministic fields and error codes as well as directives; kind-only checking cannot establish conformance. Equal Total, Passed, Failed and Skipped counts can hide different failing or skipped fixtures and **do not prove parity**. Investigate every identity or outcome difference; skips leave coverage unproven.

Use equivalent genesis, configuration and fixture state. Each script invocation sends new transactions and may choose different seed/deployment blocks, so retain the seed inputs from the setup/DEBUG output when interpreting a comparison. `PAX_EVM_IO_TX_CONTAINER` must seed the chain actually queried by its RPC URL. Even identical per-case passes establish only the assertions declared by the fixtures. Empty expected result objects request schema validation and do not establish equality of dynamic fields. Additional endpoint-to-endpoint comparisons are needed to claim equality of those values or full execution results.

## Test mix

| Kind | Description |
| ---- | ----------- |
| `.io` | Request/response fixtures curated from [ethereum/execution-apis](https://github.com/ethereum/execution-apis), plus Paxeer additions. |
| `.iox` | Fixtures using bindings, placeholders and directives, including unsupported-method and legacy-deprecation cases. |

The file list is discovered at runtime and checked against the committed
`case_inventory.json`, which declares every fixture/pair identity (one-based
`#pair-N`). Update that inventory deliberately when adding or removing pairs.
The summary counts files; the executed inventory tracks individual pairs.
An enabled run must execute every selected identity exactly once without skips. See
[`testdata/README.md`](testdata/README.md) and the
[unsupported-method reference](../../../../docs/evm_jsonrpc_unsupported.md).

### Self-contained revert fixtures

The revert fixtures query the script-deployed reverter through `__REVERTER__`:

- `eth_call/call-revert-abi-error-pax.iox`
- `eth_call/call-revert-abi-panic-pax.iox`
- `eth_estimateGas/estimate-call-abi-error-pax.iox`
- `eth_estimateGas/estimate-call-abi-panic-pax.iox`

## What is checked

`specOnly` and the runner use the same method-aware validator. Ordinary
responses must have `jsonrpc: "2.0"`, an ID equal to the request ID, and exactly
one of `result` and `error`. Method schemas check result types, canonical hex
quantities (zero is `0x0`, with no leading zeroes), even-length hex data, fixed
hash/address sizes, and required object fields. Fee history also checks fee and
ratio lengths, ratios in [0, 1], and requested reward widths. Unknown success
methods fail until a schema is declared.

All scalar, null and array expectations remain exact. Every populated object
field is compared recursively, including deterministic nested values. An empty
expected result object permits dynamic object contents only after the method's
required schema passes. It is not a wildcard for arbitrary result types.
Errors need an integer JSON-RPC code and a nonempty message; code 3 and its hex
revert data are accepted only for execution methods. Error fixtures must declare
an exact code. Declared messages and data also compare exactly. An empty error
object is incomplete evidence and fails. Every call must return HTTP 200.

Extensions add specific assertions:

- `@ bind txHash = result.transactions.0.hash` captures a JSON path for later
  `${txHash}` substitution. Bindings are local to a fixture file.
- `<< @ ref_pair 1` compares `result.number` and `result.hash` to the specified
  earlier response (one-based); it does not deep-compare blocks.
- `@ expect_body_contains text` checks a raw response-body substring.
- `@ expect_response_header Header-Name` checks header presence, not its value.

A bare `<<` still ends a pair. For a single response it requires the same
method schema and envelope validation, but declares no deterministic values.
A nonempty batch response requires explicit body or header directives; those
substring/header assertions alone do not prove each batch element's schema or
values. Use explicit per-method cases for conformance claims. Every request
marker must have exactly one response marker; overwritten requests, unmatched
requests at EOF, and orphan response markers fail parsing.

## Outcomes

- **Pass** - every selected pair, declared expectation, binding and directive passes, and executed identities match the declared inventory exactly.
- **Skip** - the integration opt-in does not equal `1`. An enabled run rejects missing fixtures, empty files, required bindings or reverter setup.
- **Fail** - inventory, setup, parsing, transport, HTTP status, schema, deterministic-value, reference or directive checks fail. Pair identities and the actual response accompany comparison failures.

### What "seed" means here

**Seed** is the minimal-contract deployment block found by the script before the tests. It gives data-dependent fixtures a known block to query; inspect the exported values rather than assuming setup succeeded.

1. The script sends an EVM transfer, deploys the minimal contract and requires its successful committed receipt.
2. The script sets `PAX_EVM_IO_SEED_BLOCK` to that block number (hex) and `PAX_EVM_IO_DEPLOY_TX_HASH` to the deploy tx hash.
3. In `.iox` fixtures, `__SEED__` is replaced by that block number (or by `"latest"` if the seed is not set).
4. Fixtures can bind `${txHash}` from a first request (for example `eth_getBlockByNumber(__SEED__, true)` -> `result.transactions.0.hash`); `${deployTxHash}` and `${txHash}` are both pre-filled from `PAX_EVM_IO_DEPLOY_TX_HASH` when set; subsequent bindings may replace them.
5. The script also requires a successful reverter deployment receipt and sets `PAX_EVM_IO_REVERTER_ADDRESS`; `__REVERTER__` is replaced by that address. The reverter answers empty calldata or `0x01` with `Error("user error")` and `0x02` with a panic.
