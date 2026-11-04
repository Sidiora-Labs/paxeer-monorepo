# Data availability

Paxeer mainnet-beta is one BFT chain, run by `paxd`, with EVM chain id `125` (`0x7d`). The kernel is embedded in the network and participates in block proposal and execution. A receipt is final when its block is committed. Reading data from a node and establishing that it belongs to a committed block are separate checks.

## Committed execution data

The embedded kernel host in `modules/lxphost` executes against the chain's SDK context. It restores module key/value records and canonical account records from that context's store, applies the activity, and writes resulting records and sequence counters back to it. Its batch number comes from the block height and its timestamp from the block time. These records belong to the chain's state lifecycle.

Kernel receipts carry canonical encoded bytes, execution results and events, and previous and resulting state roots. A state root commits to state; the root alone does not supply the records needed to reconstruct that state. Consumers must retain the receipt bytes and any execution data their verification requires, and validate the relevant commitment rather than trusting a decoded JSON view alone.

EVM transaction receipts and logs are served through the node's Ethereum JSON-RPC methods, including `eth_getTransactionReceipt`, `eth_getBlockReceipts` and `eth_getLogs`. A pending transaction or execution result obtained before block commitment is not final. See [Sidiora](sidiora.md) for token and fee behavior within this same execution and commitment lifecycle.

## Reading from the network

Use a node's root HTTPS URL, for example `https://api1.mainnet-beta.paxeer.network`, and check that `eth_chainId` returns `0x7d`. Public node hosts are `api1` through `api14.mainnet-beta.paxeer.network` and `api-mainnet-beta.paxeer.network`; see [Public RPC endpoints](../reference/public-rpc.md).

The hosted gateway has its own method contract in `services/platform/hosted/gateway/openrpc.json`. Its hosted `px_` methods must not be assumed to be available on every public node. Select the endpoint that implements the required method and verify its response according to that contract.

## Retention and recovery

Finality does not promise that every node retains all historical data. Block, state and receipt stores have distinct availability bounds. `rpc/watermark_manager.go` checks the heights that backing stores can safely serve; historical queries can fail because a height has been pruned or has not yet become available. Receipt availability is checked separately from block availability, so finding a block does not establish that its receipts remain queryable.

Receipt storage is configured by `storage/config/receipt_config.go` and wired by `node/receipt_store_config.go`:

| Setting | Behavior |
| --- | --- |
| `receipt-store.rs-backend` | `pebbledb` by default; `pebble` is also accepted |
| `receipt-store.db-directory` | Explicit directory; when empty, the application derives it from the node home and backend |
| `receipt-store.async-write-buffer` | Queue length; zero or a negative value selects synchronous writes |
| `receipt-store.prune-interval-seconds` | Interval between pruning passes |
| `min-retain-blocks` | Supplies the receipt store's retained-version count; zero keeps all receipt history |

The backend key is `receipt-store.rs-backend`; the misnamed `receipt-store.backend` is rejected. Retaining receipt history does not by itself retain historical state or blocks. Recovery and historical verification require the corresponding chain stores and records, with retention configured for the consumer's needs.

## Retired bundle protocol

The pre-merge five-class DA bundle, its 65,536-byte chunks, tag-18 selectors, tag-19 chunk stream and tag-20 terminator are retired. The sealed-candidate retrieval and checkpoint-certificate finalization procedure described by that protocol is also retired.

The old `lxp_batch_availability_root` commitment, `da-bodies.log`, sibling bundle directory and `LAYERX_NODE_DA_RETAIN_BATCHES` retention setting are retired parts of that model. They are not configuration or retrieval instructions for the merged node. The archive exclusion checks in `modules/lxphost/retired_da_bundle_test.go`, `retired_da_kernel_test.go`, `retired_da_service_test.go` and `retired_da_verified_test.go` document removal of those implementations. Current availability and finality follow committed chain data and storage behavior.
