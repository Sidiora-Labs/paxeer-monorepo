<!-- Source: https://docs.paxeer.app/json-rpc-unsupported/ -->

# Unsupported JSON-RPC Methods

Ethereum JSON-RPC methods that are explicitly unsupported on Paxeer EVM.

**Source:** `rpc/info.go`, `rpc/filter.go`, `rpc/tracers.go`, `rpc/state.go` and `rpc/tx.go` in the Paxeer node source.

## Overview

Some Ethereum JSON-RPC methods are **registered** on Paxeer's EVM endpoint but return a **JSON-RPC error** instead of a result. This gives clients and tools a stable, documented failure (code `-32000`) rather than "method not found" (`-32601`).

## Explicitly Unsupported Methods

| Method | Typical Error Message | Reason |
| --- | --- | --- |
| `eth_blobBaseFee` | "blobs not supported on this chain" | EIP-4844 blob transactions not supported |
| `eth_newPendingTransactionFilter` | "eth_newPendingTransactionFilter is not supported on Pax EVM RPC" | Only on nodes without a receipt-log feed; sequencer and standby nodes serve it (see below) |
| `debug_getRawBlock` | "debug_getRawBlock is not supported on Pax EVM RPC" | Raw RLP block payloads not served |
| `debug_getRawHeader` | "debug_getRawHeader is not supported on Pax EVM RPC" | Raw RLP header payloads not served |
| `debug_getRawReceipts` | "debug_getRawReceipts is not supported on Pax EVM RPC" | Raw RLP receipt payloads not served |

### Synchronization and raw transactions

`eth_syncing` returns `false` when consensus reports the node caught up, and otherwise a progress object with `startingBlock`, `currentBlock` and `highestBlock`. `debug_getRawTransaction` returns raw transaction bytes by hash, or a *transaction not found* error. The raw header, block and receipt-list methods in the table return explicit unsupported errors.

## Error Response Format

Unsupported methods return a standard JSON-RPC error:

```
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32000,
    "message": "blobs not supported on this chain"
  }
}
```

## Behavior Notes

### eth_syncing

`eth_syncing` is implemented from the consensus status: `false` when the node is caught up, otherwise `startingBlock` (earliest held block), `currentBlock` and `highestBlock`.

### eth_newPendingTransactionFilter

On sequencer and standby nodes, which hold the receipt-log feed, this filter is served: each poll returns the hashes of transactions receipted since the last poll, in sequence order. Full-transaction mode (`fullTx: true`) is refused — the filter serves transaction hashes only. On every other node the call returns the `-32000` unsupported error. Independently, the node tracks pending nonces, and `eth_getTransactionCount(address, "pending")` uses the local pending nonce or forwards the request to the appropriate EVM proxy.

### debug_getRaw* Methods

Raw RLP-encoded headers, blocks and receipt lists are not served on this RPC surface; use the standard `eth_*` endpoints, which return structured JSON. Raw transaction bytes are the exception: `debug_getRawTransaction` is implemented and returns them by hash.

### eth_blobBaseFee

The current implementation rejects EIP-4844 blob transactions in transaction admission. The blob fee RPC returns the explicit unsupported error shown above.

## Broader Compatibility Notes

Beyond explicitly unsupported methods, Paxeer's RPC diverges from Ethereum in several conceptual areas:

### Block Tags and Pending Nonces

- Committed reads resolve `latest`, `safe` and `finalized` through publication watermarks. Supported `pending` reads use the open execution layer when configured; `eth_getProof` refuses pending state.

- Pending nonce queries have a separate path. Do not assume `pending` and `latest` return the same nonce.

- A BFT commit provides finality under the consensus protocol’s assumptions. RPC freshness and finality are separate properties.

### Storage Proofs Use a Different Format

- `eth_getProof` is implemented by `StateAPI.GetProof`.

- Its result contains `address`, `hexValues` and `storageProof`. Storage proofs contain ICS23 proof operations through the application root rather than Ethereum Merkle Patricia trie proofs.

- The response excludes Ethereum per-account proof fields because the store has no Ethereum per-account root. Ethereum proof verifiers cannot consume this response unchanged.

- The requested height must be readable and the configured store must support proof queries. Historical retention and endpoint availability still apply.

### No Proof-of-Work

- `eth_mining` and `eth_hashrate` have no implementation on any registered API, so they return method-not-found (`-32601`), not the registered `-32000` unsupported error. Do not branch on `-32000` for them.

- Paxeer uses BFT consensus, not PoW

See [JSON-RPC API](https://docs.paxeer.app/json-rpc) for full compatibility details.

## Implementation references

The node sources and contract checks are in this repository:

```
rpc/info_test.go
rpc/state_test.go
rpc/tracers_test.go
rpc/filter_test.go
```

For provider-specific method support, check the endpoint configuration and inspect the JSON-RPC result or error returned by each request.
