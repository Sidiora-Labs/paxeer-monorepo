<!-- Source: https://docs.paxeer.app/relay-archive/ -->

# Chain History & Archives

Read retained chain history from full or archive nodes and verify exported receipt evidence.

## Full and archive nodes

The chain node is `paxd`. Initialize with `--mode full` for recent query history or `--mode archive` to retain all available state history and blocks. The archive profile retains state history and blocks by setting its retention counts to zero; the generated application configuration includes `ss-keep-recent` under `[state-store]` and `min-retain-blocks`. Retention applies to the history the node has synchronized or restored.

Choose retention for the queries your application needs. A recent read can succeed while a request for pruned state fails. A snapshot supplies only the history included in that snapshot.

## Read chain history through node RPC

Use `eth_getBlockByNumber`, `eth_getTransactionReceipt` and bounded `eth_getLogs` queries at the node’s root JSON-RPC endpoint. Keep one transaction namespace throughout an indexer and retain block coordinates with each result. Use `pax_sequencerReceipt` to distinguish a signed execution result from one covered by a committed block.

## Verify receipt archives

The mirror tooling under `services/interop/crates/layerx-mirror/` reads Ethereum or Solana archives under configured source trust. It checks archive commitments, committed block coordinates and receipt signatures. Preserve receipt-key pins and source provenance with the archive, and use its reported freshness when deciding whether the evidence is current.

## Continue reading

For chain history and state, run a `paxd` full or archive node (see [Run a Node](https://docs.paxeer.app/run-node) and [Configuration](https://docs.paxeer.app/configuration)). For receipts and proofs, see [Receipts & Proofs](https://docs.paxeer.app/proofs) and [Data Availability](https://docs.paxeer.app/data-availability). For external mirrors, see [External Mirrors](https://docs.paxeer.app/mirrors).

**Source:** `node/params/config.go`, `rpc/block.go`, `rpc/tx.go`, `rpc/filter.go`, `rpc/sequencer_receipt.go` and `services/interop/crates/layerx-mirror/src/verify.rs`.
