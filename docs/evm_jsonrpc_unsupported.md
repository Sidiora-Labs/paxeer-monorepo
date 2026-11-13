# Pax EVM JSON-RPC: explicitly unsupported methods

This page documents the EVM JSON-RPC handlers in this monorepo. Paxeer is one network: the kernel runs inside the node and proposes blocks, and a receipt is final once its block is committed. Mainnet-beta uses chain ID **125** (`0x7d`). Send public JSON-RPC requests to the root of the configured node URL. See [public RPC endpoints](site/docs/reference/public-rpc.md) for the node list.

Some Ethereum JSON-RPC methods are **registered** on Pax's EVM endpoint but return a **JSON-RPC error** instead of a result, so clients and tools get a stable, documented failure (code **`-32000`**) rather than “method not found” (`-32601`). This code identifies an explicit refusal here; other execution or lookup errors can also use `-32000`, so inspect `error.message`.

## Methods

The following registered methods return code `-32000` with these messages:

| Method | `error.message` |
| -------- | ------------------------- |
| `eth_blobBaseFee` | `blobs not supported on this chain` |
| `debug_getRawBlock` | `debug_getRawBlock is not supported on Pax EVM RPC` |
| `debug_getRawHeader` | `debug_getRawHeader is not supported on Pax EVM RPC` |
| `debug_getRawReceipts` | `debug_getRawReceipts is not supported on Pax EVM RPC` |

## Behavior notes

### Pending transaction filters

`eth_newPendingTransactionFilter` is conditionally supported. On the proposer and standby, an attached `ReceiptLogFeed` supplies hashes of transactions receipted before block commit. A node without that feed returns code `-32000` with `eth_newPendingTransactionFilter is not supported on Pax EVM RPC`. Committed-block finality does not eliminate this pending receipt feed.

- Omit the optional `fullTx` argument or pass `false` to create a filter of transaction hashes. On a node with the feed, `true` returns code `-32000` with `eth_newPendingTransactionFilter serves transaction hashes only`.
- Poll `eth_getFilterChanges` with the returned filter ID on the same node and transport. It drains hashes receipted since installation or the last poll, in receipt sequence order, and returns `[]` when there are no changes. Installation does not replay existing pending transactions, and block commit does not replay hashes already delivered.
- A filter installed in a tenant domain retains that scope. Polling it from a different domain returns `filter belongs to another chain`.
- Use `eth_uninstallFilter` to remove the filter. Filters live in memory, expire after the configured inactivity timeout, and do not survive a server restart. Polling an unknown or expired ID returns `filter does not exist`; install a new filter to resume delivery of future receipts.
- A filter buffers at most 10,000 receipted transactions between polls. Exceeding that limit makes polling fail with `pending filter fell behind the receipted transactions; uninstall it and install a new one`. Reinstalling starts a new stream; reconcile any missed history through committed blocks and receipts.
- `eth_getFilterLogs` does not apply to transaction-hash filters and returns `pending transaction filter has no logs`.

### Raw transaction payloads

`debug_getRawTransaction` **is supported over HTTP**. Given a known transaction hash, it returns the hex-encoded signed Ethereum transaction envelope produced by `Transaction.MarshalBinary()`, including the type prefix for typed transactions. It delegates lookup to the same implementation as `eth_getRawTransactionByHash`.

The two methods differ when lookup finds no transaction: `debug_getRawTransaction` returns code `-32000` with `transaction <hash> not found`, while `eth_getRawTransactionByHash` returns `null`. Lookup errors propagate to the caller. Raw block, header and receipt payloads remain unsupported as listed above.

## Implementation and coverage

The error type and code are defined in [`rpc_unsupported.go`](../rpc/rpc_unsupported.go). The current method behavior is implemented in [`filter.go`](../rpc/filter.go), [`tracers.go`](../rpc/tracers.go), [`raw_tx.go`](../rpc/raw_tx.go) and [`info.go`](../rpc/info.go); namespace registration is in [`server.go`](../rpc/server.go).

Regression coverage includes [`pending_filter_test.go`](../rpc/pending_filter_test.go), [`debug_raw_tx_test.go`](../rpc/debug_raw_tx_test.go) and [`raw_tx_test.go`](../rpc/raw_tx_test.go). Integration fixtures for the always-unsupported methods are under `tests/integration/evm_module/rpc_io_test/testdata/`: `eth_blobBaseFee/blobs-not-supported-error.iox` and `debug_getRawBlock`, `debug_getRawHeader`, `debug_getRawReceipts` with `not-supported.iox` in each directory.

For broader compatibility rules (pending tags, uncles, trie, PoW, etc.), see [`rpc/AGENTS.md`](../rpc/AGENTS.md).
