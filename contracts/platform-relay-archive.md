# Chain history and archive access

Paxeer is one BFT network: the kernel participates in `paxd` block proposal, and a receipt is final when its block commits. Mainnet-beta uses EVM chain ID 125 (`0x7d`). Full and archive `paxd` nodes provide retained chain history. Tenant domains are database namespaces within the network.

Use [hosted history and indexing](platform-hosted.md#indexer-http-routes) for application projections and [the gateway](platform-gateway.md#gateway-jsonrpc-methods) for hosted history methods. An indexer's ingestion position and a node's retained state describe different data availability boundaries.

## Relay archive config keys

Node history retention is configured in `config/app.toml`. [State-store configuration](../storage/config/ss_config.go) defines retained historical state; `state-store.ss-keep-recent` defaults to 100000 versions, and zero retains all versions. The pruning interval `state-store.ss-prune-interval` defaults to 600 seconds,
and the `StateStoreConfig.KeepLastVersion` field defaults to true. The emitted key names
are declared in the [TOML template](../storage/config/toml.go). [Receipt-store configuration](../storage/config/receipt_config.go) selects the receipt database independently. Configure the hosted indexer through its own [configuration reader](../services/platform/hosted/indexer/src/config.rs).

## Relay archive CLI commands

`paxd start` runs the native node. The [hosted node wrapper](../services/platform/hosted/node/native_runtime.py) prepares and supervises node generations and checks committed health. The hosted indexer also supports `backfill --cutover-height N`, with optional `--range-blocks N`, as described in [hosted indexing](platform-hosted.md#indexer-http-routes).

## Relay archive env vars

The hosted node reads `LAYERX_NODE_CONSENSUS_RPC_URL`, `LAYERX_NODE_RPC_URL` and `LAYERX_NODE_CHAIN_ID`. The consensus chain identifier is distinct from numeric EVM chain ID 125. Node and indexer configuration belong to separate processes; see [hosted configuration](platform-hosted.md#env-vars) and the linked readers for their required inputs.

## Relay archive error codes

Handle node JSON-RPC errors according to the [RPC contract](rpc.md#public-json-rpc) and hosted history errors according to the [gateway contract](platform-gateway.md#gateway-error-codes). Indexer ingestion distinguishes `source_unavailable`, `source_decode_failed`, `store_unavailable`, `source_configuration_invalid`, `source_integrity_failed` and `reorg_beyond_finality`. Integrity and beyond-finality reorganization errors halt ingestion.

## Relay archive file formats

Node historical state and receipt databases use the backends selected by [storage configuration](../storage/config). The hosted indexer's [store](../services/platform/hosted/indexer/src/store.rs) owns its projection records and ingestion positions. Read chain data through node APIs and application projections through the indexer API.

## Relay archive HTTP routes

Public Ethereum JSON-RPC uses the configured node HTTPS origin's root path. Hosted indexer HTTP reads include `/v1/chains`, `/v1/assets`, `/v1/history/…` and `/v1/holders/…`, as registered by [api.rs](../services/platform/hosted/indexer/src/api.rs). The indexer has its own configured listener; see [the route contract](platform-hosted.md#indexer-http-routes).

## Relay archive JSON-RPC methods

Use node `eth_` methods for retained block, transaction, log and receipt data. Hosted `px_getHistory`, `px_getUnifiedHistory` and `px_listHolderRecords` are declared in [gateway OpenRPC](../services/platform/hosted/gateway/openrpc.json). History cursors are opaque: pass returned values through unchanged and preserve the indexer's ordering and completeness information. For protobuf query and transaction services, use [the protobuf API contract](proto-api.md).
