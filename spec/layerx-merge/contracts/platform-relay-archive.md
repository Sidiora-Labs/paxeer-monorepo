# Platform receipt retrieval contract

Receipt retrieval uses the node and hosted explorer interfaces. Signed execution evidence and committed EVM receipts represent different stages of a transaction; finality follows block commitment on the single Paxeer network.

## relay-archive-http-routes

The [gateway explorer proxy](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) allows `GET /api/v2/paxeer-x/archive/receipts` and `GET /api/v2/paxeer-x/archive/receipts/{id}` against the configured explorer upstream. The [platform contract](platform.md#gateway-explorer-proxy) describes these paths.

## relay-archive-jsonrpc-methods

Node JSON-RPC is served at the configured node root. `eth_getTransactionReceipt` returns committed EVM receipt data. `pax_sequencerReceipt` serves the signed receipt and its rung where the receipt source or forwarding option is configured; see [sequencer](sequencer.md#rungs).

## relay-archive-config-keys

Gateway configuration belongs to the [hosted gateway](platform-gateway.md) and its [implementation](../../../services/platform/hosted/gateway). Node receipt configuration belongs to [node/sequencer/config.go](../../../node/sequencer/config.go).

## relay-archive-env-vars

Environment bindings are component-specific; use the [hosted component catalogue](platform-hosted.md) for their declarations.

## relay-archive-error-codes

JSON-RPC failures follow the [RPC error model](rpc.md#error-model). Explorer proxy admission and upstream failures are HTTP responses, rather than kernel execution receipts.

## relay-archive-file-formats

The signed receipt wire format and replicated WAL are described in [sequencer](sequencer.md#sequencer-receipt). Committed EVM receipts use the [receipt store](../../../storage/ledger_db/receipt).

## relay-archive-cli-commands

The node command parser is under [daemon/paxd/cmd](../../../daemon/paxd/cmd). Receipt retrieval does not require a separate archive command from this page.

<a id="platform-relay-archive-contract"></a>
