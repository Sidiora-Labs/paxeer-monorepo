# Platform contract

The [hosted gateway](../../../services/platform/hosted/gateway) provides application routes and root-path JSON-RPC. Its hosted methods are declared by [openrpc.json](../../../services/platform/hosted/gateway/openrpc.json). Chain execution and finality belong to the Paxeer node.

## gateway-explorer-proxy

[explorer_proxy.rs](../../../services/platform/hosted/gateway/src/explorer_proxy.rs) allowlists explorer requests by method and path. Archive receipt reads use:

| Method | Path |
|---|---|
| GET | `/api/v2/paxeer-x/archive/receipts` |
| GET | `/api/v2/paxeer-x/archive/receipts/{id}` |

The proxy forwards admitted requests to the configured explorer origin. Header admission, cookies, redirects and WebSocket handling follow the route's policy. These explorer paths are distinct from node JSON-RPC and from the node's committed EVM receipt method, `eth_getTransactionReceipt`.
