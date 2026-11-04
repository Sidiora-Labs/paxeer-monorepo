# EVM RPC SPECS

The `evmrpc` package in `rpc/` serves JSON-RPC inside `paxd`. Paxeer
mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`); the kernel
participates in block proposal and execution. A receipt is final when its
block is committed. See [README.md](README.md) for the interface guide and
[public endpoints](../docs/site/docs/reference/public-rpc.md) for root-path URLs.

## Pending and committed reads

Do not equate `pending` with `latest` universally. The committed tags `latest`,
`safe` and `finalized` resolve through the available-height watermarks.
`pending` depends on the method and the node's configured provider:

- **State and simulation:** With a `PendingSnapshot`, balance, code, storage
  and simulation reads use a frozen view after the last accepted transaction
  in the open block. During the seal-to-open interval,
  `WatermarkManager.pendingSnapshot` waits up to `PendingOpenWait` (two seconds)
  and respects request cancellation. If no block opens, these reads use the
  latest queryable committed height and its state/header. They resume using
  open state when a block opens. See [watermark_manager.go](watermark_manager.go),
  [state.go](state.go) and [simulate.go](simulate.go).
- **Nonce and acceptance:** Pending `eth_getTransactionCount` uses `PendingContext`;
  expiration of that wait returns `ErrPendingBlockClosed`, JSON-RPC `-32000`,
  `no block is open; retry`. The local acceptance path for
  `eth_sendRawTransaction` waits the same way after `ErrLayerNotOpen` and retries
  acceptance once. It returns the transaction hash on success. See
  [tx.go](tx.go) and [send.go](send.go).
- **Pending block objects:** [block.go](block.go) reads the snapshot directly;
  it does not apply the state-read idle fallback. The open block has its own
  height and timestamp, null `hash` and `stateRoot`, and transaction hashes only.
  Requesting full transactions returns `-32000`,
  `pending block lists transaction hashes only`. A closed snapshot produces
  an error. Pending block transaction counts also read the snapshot directly;
  without a provider they return null.
- **Nodes without an open-state provider:** Pending state, simulation and
  block resolution return `ErrPendingServedBySequencer` (`-32000`,
  `pending served by sequencer`). This names the proposer runtime within the
  network. It does not make pending a committed tag.
- **Proofs:** `eth_getProof` refuses `pending` on every node with `-32000`,
  `pending state has no committed root`.

Keep accepted logs at the signed receipt's height even after the next block
opens. Commit must wait for in-flight sends to record their logs before the
receipt log feed advances. Pending filters use that feed: with it,
`eth_newPendingTransactionFilter` returns receipted hashes in sequence order;
without it, the method returns an explicit unsupported error. Full transaction
objects are refused. See [filter.go](filter.go) and [send.go](send.go).

## Ethereum compatibility

Check the actual handler and its registration in [server.go](server.go) before
claiming that an Ethereum method is available or unsupported:

- **Uncles:** There are no uncle blocks. Uncle lookup methods return null;
  uncle count methods return zero for an existing queryable block and null
  for a missing block. See [uncle.go](uncle.go) and [uncle_count.go](uncle_count.go).
- **Proof format:** State uses SDK stores rather than an Ethereum state trie.
  Committed `eth_getProof` returns `address`, `hexValues` and `storageProof`
  with ICS23 proof operations, not Ethereum account/storage trie proofs.
  See [state.go](state.go).
- **Proof of work:** `eth_mining` and `eth_hashrate` are not registered.
- **Blobs:** EIP-4844 blob transactions are unsupported. `eth_blobBaseFee`
  returns `-32000`, `blobs not supported on this chain`.
- **Raw debug methods:** `debug_getRawBlock`, `debug_getRawHeader` and
  `debug_getRawReceipts` explicitly return `-32000` unsupported errors.
  `debug_getRawTransaction` retrieves the binary EVM transaction envelope;
  it is implemented. See [tracers.go](tracers.go) and [raw_tx.go](raw_tx.go).
- **Syncing:** `eth_syncing` returns false when consensus is caught up, or
  `startingBlock`, `currentBlock` and `highestBlock` while catching up.
  The starting height is the earliest local block; the highest height is
  the maximum of peer and local heights. See [info.go](info.go).

## `pax_` and `pax2_` prefixed endpoints

Preserve the legacy distinction between the EVM view and the view including
Cosmos transactions with synthetic EVM receipts. It affects transaction and
log indices; [README.md](README.md#transaction-index-mismatches) explains the
mapping. `pax2` adds bank transfers to block payloads and has seven block-related
methods, with no transaction or filter API.

The legacy methods enumerated in [pax_legacy.go](pax_legacy.go) are deprecated;
do not recommend them for new integrations. Prefer supported `eth_*` and
`debug_*` methods. `pax_sequencerReceipt` is explicitly exempt from this legacy
gate; its contract lives in [sequencer_receipt.go](sequencer_receipt.go).

On HTTP, `[evm].enabled_legacy_pax_apis` in `app.toml` controls the legacy
allowlist through `HTTPConfig.PaxLegacyAllowlist` and `wrapPaxLegacyHTTP` in
[pax_legacy_http.go](pax_legacy_http.go). Keep this logic centralized rather
than duplicating it in handlers. The independent `deny_list` still applies;
allowlisting does not override it. These legacy namespaces are not registered
by `NewEVMWebSocketServer`.

Defaults enable `pax_getPaxAddress`, `pax_getEVMAddress` and `pax_getCosmosTx`;
other legacy methods are commented in the generated configuration template.
The [Docker localnet configuration](../docker/localnode/config/app.toml)
enables all gated methods except `pax_sign`. Disabled calls return JSON-RPC
`-32601`, explanatory deprecation text and data `"legacy_pax_deprecated"`.
Gate responses use HTTP 200. Forwarded calls retain their result shape and
carry the `Pax-Legacy-RPC-Deprecation` header.

Allowed single-object bodies pass through unchanged. Mixed batches can be
subset-forwarded and merged by request ID. Notifications produce no response
entries; an all-notification batch has an empty HTTP response body. Preserve
coverage in [pax_legacy_test.go](pax_legacy_test.go) and
[integration fixtures](../tests/integration/evm_module/rpc_io_test/testdata/pax_legacy_deprecation/).

## `debug_` prefixed endpoints

Historical `debug_trace*` calls must replay actual execution, including the
original error and gas used. Reverts and out-of-gas transactions executed in
the VM and must remain visible in `ExcludeTraceFail` views; those views omit
synthetic receipts and pre-VM stub receipts as defined by `isReceiptUntraceable`
in [utils.go](utils.go). They are not success-only views.

## Consistency

Keep responses for an available historical height stable as the chain grows
or the node is upgraded. Use one namespace's transaction and log index mapping
consistently, and respect block, state and receipt availability watermarks.
