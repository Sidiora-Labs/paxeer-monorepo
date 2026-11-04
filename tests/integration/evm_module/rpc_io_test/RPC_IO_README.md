# EVM RPC .io / .iox tests

Integration tests for `paxd` EVM JSON-RPC response kinds and explicit fixture directives. The suite discovers `.io` and `.iox` files recursively under [`testdata/`](testdata/) and runs them as named subtests against a live endpoint. See the [integration overview](../../README.md) for the merged network and the [TypeScript RPC suite](../../rpc_tests/README.md) for value, schema and geth error comparisons.

## `.io` vs `.iox`

- **`.io`** - vanilla JSON-RPC fixtures (`>>` / `<<`) with no harness directives.
- **`.iox`** - same line format, plus harness extensions: `@ bind` / `<< @ ref_pair`, `@ expect_body_contains`, `@ expect_response_header`, and the `__SEED__` / `__REVERTER__` placeholders.

## How to run

1. Start the local cluster from the repo root: `make docker-cluster-start` (EVM RPC on port 8545; add `DOCKER_DETACH=true` to run it in the background).
2. Run the script from the repo root:
   ```bash
   ./tests/integration/evm_module/scripts/evm_rpc_tests.sh
   ```

Before `go test`, the [script](../scripts/evm_rpc_tests.sh) uses `docker exec` to attempt sender association, send one EVM transfer and attempt two deployments (a minimal contract and a reverter). Successful receipt lookups export the seed bindings described below. Deployment failures can leave bindings unset and fixtures skipped; inspect the named outcomes. The script runs this package with `PAX_EVM_IO_RUN_INTEGRATION=1`, then, only if that command succeeds, runs the sibling WebSocket package with `PAX_EVM_WS_RUN_INTEGRATION=1`. The CLI submissions use the local Cosmos chain ID `pax`. Use a disposable cluster with a funded signing key.

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

For extra `[DEBUG]` output, set `PAX_EVM_IO_DEBUG_FILES` to comma-separated fixture paths relative to `testdata/`, such as `eth_call/call-contract.io`. Matching names restrict execution; if none match, the runner retains the full discovered list. An unset `PAX_EVM_IO_RUN_INTEGRATION` causes the integration test to skip when invoking the Go package directly.

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

Use this suite to compare per-fixture response kinds and directives under different local executor configurations. It cannot establish full RPC or execution parity.

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
   ```
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
3. Match each `TestEVMRPCSpec/<fixture-path>` subtest by name, including its PASS, FAIL or SKIP status and pair-level diagnostics. Equal Total, Passed, Failed and Skipped counts can hide different failing or skipped fixtures and **do not prove parity**. Investigate every identity or outcome difference; skips leave coverage unproven.

Use equivalent genesis, configuration and fixture state. Each script invocation sends new transactions and may choose different seed/deployment blocks, so retain the seed inputs from the setup/DEBUG output when interpreting a comparison. `PAX_EVM_IO_TX_CONTAINER` must seed the chain actually queried by its RPC URL. Even identical per-fixture passes establish only the assertions this harness makes, not equal return values, error envelopes or execution results.

## Test mix

| Kind | Description |
| ---- | ----------- |
| `.io` | Request/response fixtures curated from [ethereum/execution-apis](https://github.com/ethereum/execution-apis), plus Paxeer additions. |
| `.iox` | Fixtures using bindings, placeholders and directives, including unsupported-method and legacy-deprecation cases. |

The file list is discovered at runtime; the summary counts files, not individual
request/response pairs, and its pass rate excludes skipped files. See
[`testdata/README.md`](testdata/README.md) and the
[unsupported-method reference](../../../../docs/evm_jsonrpc_unsupported.md).

### Fixtures replaced by self-contained versions

Some execution-apis fixtures depended on contracts at fixed addresses that only exist on the execution-apis test chain. They are replaced by fixtures that use the script-deployed reverter:

| Upstream fixture | Replacement |
| ---------------- | ----------- |
| `eth_call/call-revert-abi-error.io` | `eth_call/call-revert-abi-error-pax.iox` |
| `eth_call/call-revert-abi-panic.io` | `eth_call/call-revert-abi-panic-pax.iox` |
| `eth_estimateGas/estimate-call-abi-error.io` | `eth_estimateGas/estimate-call-abi-error-pax.iox` |
| `eth_estimateGas/estimate-failed-call.io` | `eth_estimateGas/estimate-call-abi-error-pax.iox` and `estimate-call-abi-panic-pax.iox` |

## What is checked

For an ordinary expected response, `specOnly` checks the presence of `result`
and `error` against the fixture and requires a `jsonrpc` field. It does not
compare result values, error codes/messages/data, the ID, or the value of the
`jsonrpc` field. Every call must return HTTP 200 and valid JSON.

Extensions add specific assertions:

- `@ bind txHash = result.transactions.0.hash` captures a JSON path for later
  `${txHash}` substitution. Bindings are local to a fixture file.
- `<< @ ref_pair 1` compares `result.number` and `result.hash` to the specified
  earlier response (one-based); it does not deep-compare blocks.
- `@ expect_body_contains text` checks a raw response-body substring.
- `@ expect_response_header Header-Name` checks header presence, not its value.

A pair with no expected response accepts an object containing `result` or
`error`. A nonempty batch response in that branch requires a body or header
directive. Merely expecting an error does not assert a particular error code;
use the fixture's explicit directives to understand the actual coverage.

## Outcomes

- **Pass** - all pairs and declared directives in the file pass.
- **Skip** - the file has no pairs, a required `${binding}` is missing, or it uses `__REVERTER__` without `PAX_EVM_IO_REVERTER_ADDRESS`. A skip can occur after earlier pairs ran.
- **Fail** - a parse, transport, HTTP status, response-kind, reference or directive check fails. On a response-kind mismatch the runner logs the actual `error.code` and `error.message` (or a short `result` snippet), which tells apart not implemented (`-32601`), invalid params (`-32602`), a disabled endpoint, or another spec mismatch.

### What "seed" means here

**Seed** is the minimal-contract deployment block found by the script before the tests. It gives data-dependent fixtures a known block to query; inspect the exported values rather than assuming setup succeeded.

1. The script sends an EVM transfer, attempts the minimal-contract deployment and polls for its receipt.
2. The script sets `PAX_EVM_IO_SEED_BLOCK` to that block number (hex) and `PAX_EVM_IO_DEPLOY_TX_HASH` to the deploy tx hash.
3. In `.iox` fixtures, `__SEED__` is replaced by that block number (or by `"latest"` if the seed is not set).
4. Fixtures can bind `${txHash}` from a first request (for example `eth_getBlockByNumber(__SEED__, true)` -> `result.transactions.0.hash`); `${deployTxHash}` and `${txHash}` are both pre-filled from `PAX_EVM_IO_DEPLOY_TX_HASH` when set; subsequent bindings may replace them.
5. The script also deploys a reverter contract and sets `PAX_EVM_IO_REVERTER_ADDRESS`; `__REVERTER__` is replaced by that address. The reverter answers empty calldata or `0x01` with `Error("user error")` and `0x02` with a panic.
