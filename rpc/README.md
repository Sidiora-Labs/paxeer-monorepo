# EVM JSON-RPC

This package (`evmrpc`) serves the EVM JSON-RPC interface inside `paxd`.
Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The
kernel is part of the network and proposes blocks. Receipts become final when
their block is committed; an open block can already contain executed,
receipted transactions before that commitment. Select `latest` for committed
state; select `pending` for an open proposal view on a node with an open-state
provider, subject to the method-specific idle and refusal behavior below.

Public JSON-RPC uses the root path of
`https://api-mainnet-beta.paxeer.network` and the numbered nodes
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. See the
[public endpoint reference](../docs/site/docs/reference/public-rpc.md) for
the full list and WebSocket URLs. `node.hyperpaxeer.com` is the hpx library mirror,
not an RPC endpoint. The former router and its prefixed routes are retired.

See [AGENTS.md](AGENTS.md) for implementation requirements and compatibility
differences. [server.go](server.go) defines the HTTP and WebSocket registrations;
configuration and method availability differ between the two transports.

## Understanding the RPC Architecture

### Eth_ Endpoints

Use supported `eth_*` methods for EVM applications and Ethereum tooling.
The ordinary transaction view filters Cosmos-native transactions, while
EVM-originating transactions can also contain synthetic CosmWasm event logs.
Chain-generated venue receipts are also included by the block-receipt/log
paths when present; an EVM view is not a promise that every receipt represents
a user-signed Ethereum transaction. See [block.go](block.go), [tx.go](tx.go),
[filter.go](filter.go) and [utils.go](utils.go).

HTTP also registers `net`, `web3`, `txpool`, `debug` and `echo`, plus the
chain-specific APIs below. WebSocket registers `eth`, `net`, `web3` and `echo`,
including `eth_subscribe` for heads, logs and pending transaction hashes.
The `test` namespace requires `enable_test_api` and a non-live chain ID.
An operator's `deny_list` can further restrict methods.

The node's receipt APIs are `pax_sequencerReceipt` and `px_getReceiptKeys`,
registered on HTTP in [server.go](server.go), with provider/forwarding behavior
defined by [sequencer_receipt.go](sequencer_receipt.go). The broader hosted
gateway `px_*` contract is declared in
[openrpc.json](../services/platform/hosted/gateway/openrpc.json); its methods
are not automatically node methods.

### Pax_ Endpoints

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

### Key Differences

| View | Transaction coverage | Index interpretation |
| --- | --- | --- |
| `eth_*` | EVM-visible transactions and applicable synthetic receipts/logs | The EVM view of the relevant method |
| Legacy `pax_*` | Adds Cosmos transactions that have synthetic EVM receipts | The combined filtered view |
| Legacy `pax2_*` | Adds bank transfers to block payloads | The block method's filtered view |

Sub-chains are tenant domains in the database and do not require new node
infrastructure. [rpcstack.go](rpcstack.go) resolves `/chain/<id>` to a domain
scope on the same server; [domain_scope.go](domain_scope.go) carries that scope.
Root-path mainnet requests retain chain ID `125`.

## Pending

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

### State and simulation reads

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

### Block, nonce and send behavior

- `eth_getBlockByNumber("pending", false)` reads the open snapshot directly.
  Its `number` is the open height; `hash` and `stateRoot` are null. It lists
  receipted transaction hashes in order. Passing `true` for full transactions
  returns `-32000`, `pending block lists transaction hashes only`.
- Pending block objects, indexed pending transactions and pending block
  transaction counts read the snapshot directly and report an error if it is
  closed; they do not share the state-read idle fallback. Without a provider,
  `eth_getBlockTransactionCountByNumber("pending")` returns null, while pending
  block-number resolution returns `-32000`, `pending served by sequencer`.
- Pending `eth_getTransactionCount` uses `PendingContext` and waits for an open
  block. If none opens within the bound, it returns `ErrPendingBlockClosed`
  (`-32000`, `no block is open; retry`).
- On the local acceptance path, `eth_sendRawTransaction` encountering a sealed
  block waits using the same bound and retries acceptance once. It returns the
  hash on success, or the closed-block error if the retry still finds no open
  block. Accepted logs retain their receipt's height, and commit waits for
  in-flight sends to record logs before advancing the receipt log feed.

See [block.go](block.go), [tx.go](tx.go) and [send.go](send.go). The different
paths mean clients must not infer one method's idle behavior from another.

### Filters, proofs and tracing

`eth_newPendingTransactionFilter` is available when the server has a receipt
log feed. Polling returns hashes receipted since the last poll in sequence
order. Full transaction objects are refused; without the feed the method
returns `-32000` with an unsupported message. See [filter.go](filter.go).

`eth_getProof` always refuses `pending` with `-32000`,
`pending state has no committed root`. Committed proofs use ICS23 operations
through the SDK app root, not Ethereum trie proofs. Pending tracing is also
refused; use a committed block for historical traces. See [state.go](state.go),
[simulate.go](simulate.go) and [tracers.go](tracers.go).

The consumer regression coverage is [repair_m1147_test.go](repair_m1147_test.go),
with further open/idle state and header checks in
[pending_simulate_test.go](pending_simulate_test.go) and
[pending_state_test.go](pending_state_test.go).

## Legacy gate and deprecation

The legacy methods listed in [pax_legacy.go](pax_legacy.go) are deprecated
and scheduled for removal. New integrations should use supported `eth_*`,
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

### Migration by method

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

## Pax_ Endpoints

### 1. Synthetic Transaction Endpoints

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

### 2. Tracing Failure Management Endpoints

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

## Transaction Index Mismatches

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

### Receipts and Logs

EVM-originating synthetic events remain part of their EVM transaction's logs.
Cosmos-originating CosmWasm events require the combined legacy log view.
Receipt encoding and log-query normalization use the relevant namespace's
transaction mapping and log offsets; do not compare indices across namespaces
as though they were absolute consensus positions.

### Best Practices

- Use one namespace consistently for receipts, logs and transaction indices.
- Use transaction hashes to correlate entries across views; keep the source
  namespace when persisting indices.
- Include executed failures when investigating failures or accounting for gas.
- Treat pending data as a view that changes during block execution; stable
  historical data requires an available committed height.
