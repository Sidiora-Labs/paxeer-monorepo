# EVM JSON-RPC

The `rpc/` directory holds the `evmrpc` Go package, its supporting packages,
contract fixtures and tests. It serves EVM JSON-RPC over HTTP and WebSocket
inside `paxd`. Paxeer mainnet-beta is one BFT network with EVM chain ID `125`
(`0x7d`); the kernel participates in block proposal and execution. Receipts
become final when their block is committed. An open block can already contain
executed, receipted transactions before commitment. Select `latest` for
committed state or `pending` for an open proposal view on a node with an
open-state provider, subject to the method-specific behavior below.

## Layout

- [config/](config/) — RPC configuration types, option decoding, defaults and the application configuration template.
- [ethbloom/](ethbloom/) — Ethereum log bloom matching and exact address/topic matching.
- [rpcutils/](rpcutils/) — Ethereum transaction sender recovery and signature test vectors.
- [solidity/](solidity/) — Solidity contracts and precompile interfaces used by RPC tests.
- [stats/](stats/) — HTTP and WebSocket request statistics trackers.
- [testdata/](testdata/) — Method registration snapshots and account-history response fixtures.
- [tests/](tests/) — RPC integration tests, application setup helpers and compiled contract fixtures.
- [AGENTS.md](AGENTS.md) — Interface behavior and implementation requirements.
- [server.go](server.go) — HTTP and WebSocket service registration and node dependency wiring.
- [server_options.go](server_options.go) — Pending snapshot and transaction forwarding options.
- [rpcstack.go](rpcstack.go) — HTTP server lifecycle, tenant routing and transport configuration.
- [endpoints.go](endpoints.go) — Module availability checks and HTTP timeout validation.
- [websockets.go](websockets.go) — WebSocket connection-attempt metrics.
- [info.go](info.go) — Chain identity, fees, synchronization status and RPC information.
- [net.go](net.go) — Network identity and listening status.
- [net_peers.go](net_peers.go) — Peer count reads.
- [web3.go](web3.go) — Client version and hashing methods.
- [echo.go](echo.go) — Echo service.
- [txpool.go](txpool.go) — Mempool transaction inspection.
- [block.go](block.go) — Block objects, block receipts and namespace-specific transaction views.
- [state.go](state.go) — Account state, storage and committed state proofs.
- [tx.go](tx.go) — Transaction lookup, nonces, receipt encoding and transaction indices.
- [raw_tx.go](raw_tx.go) — Raw signed transaction lookup by hash.
- [raw_tx_index.go](raw_tx_index.go) — Raw transaction lookup by block and index.
- [send.go](send.go) — Raw transaction submission and local receipt-log delivery.
- [send_sync.go](send_sync.go) — Synchronous raw transaction submission.
- [sequencer_receipt.go](sequencer_receipt.go) — Signed receipt progress and receipt-key APIs.
- [simulate.go](simulate.go) — Calls, gas estimation and access-list simulation.
- [watermark_manager.go](watermark_manager.go) — Block/state availability and pending snapshot resolution.
- [filter.go](filter.go) — Polling filters, log queries and block caching.
- [subscribe.go](subscribe.go) — WebSocket head, log and pending transaction subscriptions.
- [notifier.go](notifier.go) — Committed block header delivery to subscriptions.
- [domain_scope.go](domain_scope.go) — Tenant scope carried through RPC requests.
- [domain_index.go](domain_index.go) — Tenant-specific transaction and receipt indexing.
- [domain_ws_apis.go](domain_ws_apis.go) — Tenant-aware WebSocket API wrappers.
- [pax_legacy.go](pax_legacy.go) — Legacy method catalogue and allowlist handling.
- [pax_legacy_http.go](pax_legacy_http.go) — HTTP legacy method gating and batch handling.
- [pax_filter.go](pax_filter.go) — Legacy combined-view filter methods.
- [association.go](association.go) — Cosmos/EVM address association and transaction hash mapping.
- [px_account.go](px_account.go) — Native account reads and node-backed EVM calls.
- [px_assets.go](px_assets.go) — Asset listing and indexer access.
- [px_history.go](px_history.go) — Account history reads.
- [px_discovery.go](px_discovery.go) — Capability probes and route catalogue discovery.
- [px_kernel.go](px_kernel.go) — Commit, validator and kernel-backed reads.
- [px_inference.go](px_inference.go) — Committed inference market reads.
- [px_params.go](px_params.go) — Native RPC parameter validation and request handling.
- [px_route_catalogue.json](px_route_catalogue.json) — Embedded product route catalogue.
- [generate_px_catalogue.py](generate_px_catalogue.py) — Catalogue generation from the product route contract.
- [tracers.go](tracers.go) — Historical execution tracing and debug APIs.
- [trace_baker.go](trace_baker.go) — Background trace generation and retention.
- [trace_profile.go](trace_profile.go) — Profiled transaction tracing.
- [worker_pool.go](worker_pool.go) — Shared worker pool for block and log reads.
- [worker_pool_metrics.go](worker_pool_metrics.go) — Worker and log-query metrics and debug output.
- [metrics.go](metrics.go) — RPC request metrics.
- [ip_rate_limit.go](ip_rate_limit.go) — Per-client HTTP rate limiting.
- [served_methods.go](served_methods.go) — Enumeration of registered calls and subscription selectors.
- [setup_test.go](setup_test.go) — Application and RPC fixtures for package tests.

## Build and test

This directory belongs to the root [Go module](../go.mod) and builds as part
of `paxd`. Run these targets from the repository root; both are defined in the
root [Makefile](../Makefile):

```sh
make paxd-build
make test-paxd-domain
```

`paxd-build` prepares the native libraries and delegates the node build to
[chain.mk](../chain.mk), producing `build/paxd`. `test-paxd-domain` prepares
the native libraries, runs the host tests, then runs
`CGO_ENABLED=1 go test ./node/... ./modules/evm/keeper/... ./rpc/...`.
The RPC tests include the `tests/` integration package and the supporting Go
packages listed above. [tests/README.md](tests/README.md) describes its server
helpers and contract fixtures.

## Configuration

The node supplies the `[evm]` section of `app.toml` through application options
to `ReadConfig` in [config/config.go](config/config.go). That file defines the
configuration structure, defaults and `ConfigTemplate`. A checked-in example
is [docker/localnode/config/app.toml](../docker/localnode/config/app.toml).
The option names are:

| Area | Keys under `[evm]` |
| --- | --- |
| HTTP and WebSocket | `http_address`, `http_enabled`, `http_port`, `ws_enabled`, `ws_port`, `cors_origins`, `ws_origins` |
| Requests and timeouts | `batch_request_limit`, `batch_response_max_size`, `read_timeout`, `read_header_timeout`, `write_timeout`, `idle_timeout`, `send_raw_tx_sync_timeout` |
| Simulation | `simulation_gas_limit`, `simulation_evm_timeout`, `max_concurrent_simulation_calls` |
| Filters and subscriptions | `filter_timeout`, `max_log_no_block`, `max_blocks_for_log`, `max_subscriptions_new_head`, `max_subscriptions_logs` |
| Method access and submission | `deny_list`, `enabled_legacy_pax_apis`, `enable_test_api`, `enable_unsafe_keyring_rpc`, `max_tx_pool_txs`, `slow` |
| Tracing | `max_concurrent_trace_calls`, `max_trace_lookback_blocks`, `trace_timeout`, `enable_parallelized_block_trace` |
| Trace storage | `trace_bake_enabled`, `trace_bake_workers`, `trace_bake_queue_size`, `trace_bake_tracers`, `trace_bake_window_blocks`, `trace_bake_use_snapshot`, `trace_bake_snapshot_window` |
| Workers and statistics | `worker_pool_size`, `worker_queue_size`, `rpc_stats_interval` |
| Client rate limits | `ip_rate_limit_rps`, `ip_rate_limit_burst` |
| Native reads | `px_indexer_url`, `px_inference_program`, `px_inference_reader` |

[worker_pool_metrics.go](worker_pool_metrics.go) reads `EVM_DEBUG_METRICS`
to control periodic metrics output to standard output.

## Documentation

- [RPC method catalogue](../docs/system/rpc.md) — Registered methods, transports and configuration-dependent availability.
- [Ethereum compatibility and unsupported methods](../docs/evm_jsonrpc_unsupported.md) — Explicit refusals and pending filter behavior.
- [JSON-RPC documentation site page](../docs/site/paxeer-docs/src/app/json-rpc/page.tsx) — Namespace overview and client semantics.
- [Query the EVM](../docs/site/paxeer-docs/src/app/querying-evm/page.tsx) — Read-only client examples.
- [Public RPC documentation site page](../docs/site/paxeer-docs/src/app/public-rpc/page.tsx) — Node root URLs and WebSocket endpoints.

Public JSON-RPC uses the node's root path. HTTP and WebSocket registrations
are defined in [server.go](server.go); configuration and method availability
differ between the transports. [AGENTS.md](AGENTS.md) describes implementation
requirements and compatibility differences.

### Understanding the RPC Architecture

#### Eth_ Endpoints

Use supported `eth_*` methods for EVM applications and Ethereum tooling.
The ordinary transaction view filters Cosmos-native transactions, while
EVM-originating transactions can also contain synthetic CosmWasm event logs.
Chain-generated venue receipts are also included by the block-receipt/log
paths when present; an EVM view is not a promise that every receipt represents
a user-signed Ethereum transaction. See [block.go](block.go), [tx.go](tx.go),
[filter.go](filter.go) and [utils.go](utils.go).

HTTP also registers `net`, `web3`, `txpool`, `debug`, `echo` and `px`, plus
the chain-specific APIs below. WebSocket registers `eth`, `net`, `web3`, `txpool`, `echo`,
`px` and `pax_sequencerReceipt`, including `eth_subscribe` for heads, logs and pending transaction hashes.
The `test` namespace requires `enable_test_api` and a non-live chain ID.
An operator's `deny_list` can further restrict methods.

The node's receipt APIs are `pax_sequencerReceipt` and `px_getReceiptKeys`,
registered in [server.go](server.go), with provider/forwarding behavior
defined by [sequencer_receipt.go](sequencer_receipt.go). The `px` namespace
(`px_account.go`, `px_assets.go`, `px_history.go`, `px_discovery.go`,
`px_kernel.go`) answers account, asset, history, capability, commit and
validator reads from node state. `px_register` and `px_requestFunds` answer
`-32004` with data code `hosted_gateway_service_required`; the hosted
operations are declared in
[openrpc.json](../services/platform/hosted/gateway/openrpc.json).

#### Pax_ Endpoints

Legacy `pax_*` block, receipt and log APIs include relevant Cosmos transactions
with synthetic EVM receipts, including CosmWasm CW20/CW721 events. They remain
useful when maintaining an existing pointer-contract indexer or reconciling
historical mixed EVM/Cosmos data, subject to the
[legacy gate](#legacy-gate-and-deprecation). Prefer supported `eth_*` and
`debug_*` APIs for new integrations.

`pax2_*` adds bank transfers to the legacy block payload. It exposes seven
HTTP methods: `pax2_getBlockByNumber`, `pax2_getBlockByHash`, their
`ExcludeTraceFail` variants, `pax2_getBlockReceipts`,
`pax2_getBlockTransactionCountByNumber` and
`pax2_getBlockTransactionCountByHash`. There is no `pax2` transaction or filter
API. Bank-transfer inclusion in block payloads does not imply a receipt for
every bank transfer.

#### Key Differences

| View | Transaction coverage | Index interpretation |
| --- | --- | --- |
| `eth_*` | EVM-visible transactions and applicable synthetic receipts/logs | The EVM view of the relevant method |
| Legacy `pax_*` | Adds Cosmos transactions that have synthetic EVM receipts | The combined filtered view |
| Legacy `pax2_*` | Adds bank transfers to block payloads | The block method's filtered view |

Sub-chains are tenant domains in the database and do not require new node
infrastructure. [rpcstack.go](rpcstack.go) resolves `/chain/<id>` to a domain
scope on the same server; [domain_scope.go](domain_scope.go) carries that scope.
Root-path mainnet requests retain chain ID `125`.

### Pending

`pending` is method- and provider-dependent. It is not universally an alias
for `latest`. In [watermark_manager.go](watermark_manager.go), `latest`, `safe`
and `finalized` resolve to the latest queryable committed height. A node wired
with `NewPendingOption` can additionally expose the proposer or standby's
open execution snapshot. This is a provider capability, retained during the
idle seal-to-open interval, rather than a promise that a block is always open.
Omitting the block tag (or passing null) to `eth_call`, `eth_estimateGas` or
`eth_createAccessList` selects pending on that node and latest on a full node
without the provider. An explicit pending tag on a full node still refuses;
clients requiring committed state should pass `latest`.

#### State and simulation reads

`eth_getBalance`, `eth_getCode`, `eth_getStorageAt` and pending simulation
reads use a frozen open-block snapshot after the last accepted transaction.
The simulation header uses that block's height, time, gas limit and base fee,
and the committed parent's hash. See [state.go](state.go) and
[simulate.go](simulate.go).

If the provider reports that no block is open, `pendingSnapshot` waits up to
`PendingOpenWait` (two seconds), honoring request cancellation. If the wait
expires, these reads resolve to the latest queryable committed state and
header. When the next block opens, they use its snapshot. A node without a
pending provider returns `-32000`, `pending served by sequencer`, for these
explicit pending requests: `eth_getBalance`, `eth_getCode`, `eth_getStorageAt`,
`eth_call`, `eth_estimateGas` and `eth_createAccessList`. Cancellation ends the
wait with the request error; it does not trigger the idle committed fallback.

#### Block, nonce and send behavior

- `eth_getBlockByNumber("pending", false)` reads the open snapshot directly.
  Its `number` is the open height; `hash` and `stateRoot` are null. It lists
  receipted transaction hashes in order. Passing `true` for full transactions
  returns `-32000`, `pending block lists transaction hashes only`.
- Pending block objects, indexed pending transactions and pending block
  transaction counts read the snapshot directly and report an error if it is
  closed; they do not share the state-read idle fallback. Without a provider,
  `eth_getBlockTransactionCountByNumber("pending")` returns null, while pending
  block-number resolution returns `-32000`, `pending served by sequencer`.
- Pending `eth_getTransactionCount` reads the open-block nonce. With the node's
  committed-state context provider, it waits during an idle interval and falls
  back to the latest queryable committed nonce if the wait expires. Held future
  nonces do not advance this value. Without a local open-state provider, the
  request follows the account's `EvmProxy` route; without that route it returns
  `-32000`, `pending served by sequencer`. A caller with open-state access but
  no committed-state context provider receives `ErrPendingBlockClosed`
  (`-32000`, `no block is open; retry`) when its wait expires.
- On the local acceptance path, `eth_sendRawTransaction` encountering a sealed
  block waits using the same bound and retries acceptance once. It returns the
  hash on success, or the closed-block error if the retry still finds no open
  block. Accepted logs retain their receipt's height, and commit waits for
  in-flight sends to record logs before advancing the receipt log feed.

See [block.go](block.go), [tx.go](tx.go) and [send.go](send.go). The different
paths mean clients must not infer one method's idle behavior from another.

#### Filters, proofs and tracing

`eth_newPendingTransactionFilter` is available when the server has a receipt
log feed. Polling returns hashes receipted since the last poll in sequence
order. Full transaction objects are refused; without the feed the method
returns `-32000` with an unsupported message. See [filter.go](filter.go).

`eth_getProof` always refuses `pending` with `-32000`,
`pending state has no committed root`. Committed proofs use ICS23 operations
through the SDK app root, not Ethereum trie proofs. Pending tracing is also
refused; use a committed block for historical traces. See [state.go](state.go),
[simulate.go](simulate.go) and [tracers.go](tracers.go).

Pending consumer behavior is covered in [repair_m1147_test.go](repair_m1147_test.go),
with further open/idle state and header checks in
[pending_simulate_test.go](pending_simulate_test.go) and
[pending_state_test.go](pending_state_test.go).

### Legacy gate and deprecation

The legacy methods listed in [pax_legacy.go](pax_legacy.go) are deprecated
for new integrations. Use supported `eth_*`,
`debug_*` and documented replacements. `pax_sequencerReceipt` is explicitly
exempt from the legacy gate; it reports transaction progress as `submitted`,
`receipted` or `committed` according to its provider and committed batch state.

On HTTP, legacy calls are controlled by `enabled_legacy_pax_apis` under `[evm]`
in `app.toml`, enforced centrally by `wrapPaxLegacyHTTP` in
[pax_legacy_http.go](pax_legacy_http.go). Only allowlisted methods pass this
gate; `deny_list` remains an independent restriction. The node WebSocket
server does not register the legacy `pax` or `pax2` APIs.

- [DefaultConfig](config/config.go) and its generated template enable
  `pax_getPaxAddress`, `pax_getEVMAddress` and `pax_getCosmosTx`; other legacy
  methods are commented out in the template.
- Disabled calls return JSON-RPC error `-32601` with data
  `"legacy_pax_deprecated"` and a message explaining enablement and deprecation.
  The gate uses HTTP 200 for these JSON-RPC responses.
- Forwarded legacy calls preserve the result shape and carry the
  `Pax-Legacy-RPC-Deprecation` response header. Mixed batches are merged by
  request ID. Notifications have no response entries; an all-notification
  batch returns an empty HTTP body.
- The [Docker localnet config](../docker/localnode/config/app.toml) enables
  all gated methods except `pax_sign`; it is not the production default.

#### Migration by method

| Legacy method(s) | Supported migration and limits |
| --- | --- |
| `pax_getBlockByNumber`, `pax_getBlockByHash`, and the `pax2_*` equivalents | `eth_getBlockByNumber`, `eth_getBlockByHash` for the EVM view; Cosmos-sourced synthetic transactions and added bank transfers remain legacy-only block payloads |
| `pax_getBlockTransactionCountByNumber`, `pax_getBlockTransactionCountByHash`, and the `pax2_*` equivalents | The matching `eth_getBlockTransactionCountByNumber` or `eth_getBlockTransactionCountByHash`; counts follow the EVM view |
| `pax_getBlockReceipts`, `pax2_getBlockReceipts`, `pax_getTransactionReceipt` | `eth_getBlockReceipts`, `eth_getTransactionReceipt` for EVM-visible receipts; no standard replacement supplies the combined Cosmos receipt view |
| `pax_getLogs`, `pax_getFilterLogs`, `pax_getFilterChanges`, `pax_newFilter`, `pax_newBlockFilter`, `pax_uninstallFilter` | The matching `eth_*` log/filter methods; Cosmos-originating synthetic logs remain legacy-only |
| `pax_getTransactionByHash`, `pax_getTransactionByBlockHashAndIndex`, `pax_getTransactionByBlockNumberAndIndex`, `pax_getTransactionCount` | The matching `eth_*` transaction methods, using EVM indices and account nonce semantics |
| `pax_traceBlockByNumberExcludeTraceFail`, `pax_traceBlockByHashExcludeTraceFail` | `debug_traceBlockByNumber`, `debug_traceBlockByHash`; handle untraceable entries explicitly and retain executed failures |
| `pax_getBlockByNumberExcludeTraceFail`, `pax_getBlockByHashExcludeTraceFail`, and the `pax2_*` variants | Standard `eth_getBlockByNumber` or `eth_getBlockByHash`; no standard flag reproduces the legacy exclusion view |
| `pax_getTransactionReceiptExcludeTraceFail`, `pax_getTransactionErrorByHash`, `pax_getVMError` | `eth_getTransactionReceipt` for status/gas and `debug_traceTransaction` for execution errors; no identical legacy error/exclusion shape |
| `pax_getPaxAddress`, `pax_getEVMAddress`, `pax_getCosmosTx`, `pax_getEvmTx`, `pax_associate` | No direct standard Ethereum equivalent for address association or Cosmos/EVM hash mapping; existing clients require the method-specific legacy allowlist |
| `pax_sign` | Sign locally with the application's signer; do not migrate to an unsafe node signing API |

Allowlist defaults do not make legacy methods supported migration targets.
`pax_sequencerReceipt` is the explicit receipt-progress exception, not a
replacement for historical combined receipts or logs. The legacy-only views
below describe compatibility for existing clients and require operator enablement.

### Legacy transaction and tracing views

#### Synthetic Transaction Endpoints

Existing integrations can query CosmWasm token events as EVM-compatible logs
and receipts to index pointer contracts or reconcile token transfers:

| Method | Legacy behavior |
| --- | --- |
| `pax_getFilterLogs`, `pax_getLogs` | Include synthetic Cosmos-sourced logs in the combined view |
| `pax_getBlockByNumber`, `pax_getBlockByHash` | Include supported synthetic transactions in block data |
| `pax_getBlockReceipts` | Include the corresponding synthetic receipts in the combined view |

EVM-originating transactions can expose synthetic event logs through
`eth_getLogs` and `eth_getTransactionReceipt`. For Cosmos-originating
CosmWasm transactions, use the enabled legacy combined view to obtain their
synthetic receipts/logs. Do not assume a Cosmos hash is a signed Ethereum
transaction or that `eth_getTransactionReceipt` can encode every synthetic
receipt. There is no `pax_getTransactionByReceipt` method.

#### Tracing Failure Management Endpoints

Some transactions consume a nonce but fail before entering the VM, producing
a stub receipt with zero effective gas price and zero gas used. The
`ExcludeTraceFail` views exclude these and synthetic receipts without EVM
execution, using `isReceiptUntraceable` in [utils.go](utils.go). Reverted and
out-of-gas transactions did execute and remain visible. These APIs do not
select only successful transactions.

| Method | Behavior |
| --- | --- |
| `pax_traceBlockByNumberExcludeTraceFail`, `pax_traceBlockByHashExcludeTraceFail` | Omit untraceable entries from block tracing |
| `pax_getBlockByNumberExcludeTraceFail`, `pax_getBlockByHashExcludeTraceFail` | Omit synthetic and untraceable entries from block data |
| `pax_getTransactionReceiptExcludeTraceFail` | Refuse a receipt classified as untraceable; retain executed failures |

Use regular supported `eth_*` and `debug_*` methods when diagnosing execution
failures. Historical traces must reflect the original error and actual gas
usage, as described in [AGENTS.md](AGENTS.md#debug_-prefixed-endpoints).

### Transaction Index Mismatches

Transaction and log indices depend on the method's filtered view. For example,
suppose a block contains EVM transaction A, a Cosmos transaction C with a
synthetic receipt, and EVM transaction B. Ignoring any additional synthetic
venue receipts, the receipt views are:

| Transaction | `eth_getBlockReceipts` index | `pax_getBlockReceipts` index |
| --- | --- | --- |
| A | 0 | 0 |
| C | Excluded | 1 |
| B | 1 | 2 |

This example does not mean every Cosmos transaction has a synthetic receipt
or appears in the legacy view. [utils.go](utils.go) filters messages and
receipts; [tx.go](tx.go) translates stored consensus positions to the selected
view in `GetEvmTxIndex` and `encodeReceipt`. Venue synthetic receipts have
their own assigned positions and should not be renumbered by clients.

#### Receipts and Logs

EVM-originating synthetic events remain part of their EVM transaction's logs.
Cosmos-originating CosmWasm events require the combined legacy log view.
Receipt encoding and log-query normalization use the relevant namespace's
transaction mapping and log offsets; do not compare indices across namespaces
as though they were absolute consensus positions.

#### Best Practices

- Use one namespace consistently for receipts, logs and transaction indices.
- Use transaction hashes to correlate entries across views; keep the source
  namespace when persisting indices.
- Include executed failures when investigating failures or accounting for gas.
- Treat pending data as a view that changes during block execution; stable
  historical data requires an available committed height.
