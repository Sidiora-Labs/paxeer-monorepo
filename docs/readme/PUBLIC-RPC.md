# Public RPC

Paxeer X Network serves JSON-RPC at the root path of its public node names.
The EVM chain ID is 125 (`eth_chainId` returns `0x7d`); the consensus chain ID
is `hyperpax_125-1`.

## Public JSON-RPC

| Surface | URL |
| --- | --- |
| HTTPS JSON-RPC | `https://api-mainnet-beta.paxeer.network` |

Send requests directly to the node's root path.
There is no separate public LayerX interface or router.
The `/rpc`, `/rpc/ws` and `/rpc/schema` routes are retired; use the root path.

```bash
curl -s https://api-mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

## Node names

Use `api1` through `api14.mainnet-beta.paxeer.network` or
`api-mainnet-beta.paxeer.network`, all at the root path.

| Node | HTTPS JSON-RPC |
| --- | --- |
| 1 | `https://api1.mainnet-beta.paxeer.network` |
| 2 | `https://api2.mainnet-beta.paxeer.network` |
| 3 | `https://api3.mainnet-beta.paxeer.network` |
| 4 | `https://api4.mainnet-beta.paxeer.network` |
| 5 | `https://api5.mainnet-beta.paxeer.network` |
| 6 | `https://api6.mainnet-beta.paxeer.network` |
| 7 | `https://api7.mainnet-beta.paxeer.network` |
| 8 | `https://api8.mainnet-beta.paxeer.network` |
| 9 | `https://api9.mainnet-beta.paxeer.network` |
| 10 | `https://api10.mainnet-beta.paxeer.network` |
| 11 | `https://api11.mainnet-beta.paxeer.network` |
| 12 | `https://api12.mainnet-beta.paxeer.network` |
| 13 | `https://api13.mainnet-beta.paxeer.network` |
| 14 | `https://api14.mainnet-beta.paxeer.network` |
| Mainnet beta | `https://api-mainnet-beta.paxeer.network` |

**Retention:** `api2.mainnet-beta.paxeer.network` is the archive node and keeps
full history; the other thirteen numbered nodes prune. On a pruned node,
`pax_sequencerReceipt` and state queries for heights outside that node's
retention return JSON-RPC error `-32000`. This is expected retention behavior,
not a defect. Send historical receipt and state queries to
`https://api2.mainnet-beta.paxeer.network` at the root path, on EVM chain ID 125.

Names are numbered, not ranked. Pick any one and fall back to another if it
stops answering; compare `eth_blockNumber` across two names when you need the
newest head.

WebSocket upgrades were observed at
`wss://api-mainnet-beta.paxeer.network` and
`wss://api-mainnet-beta.paxeer.network/ws`.
There is no WebSocket upgrade at `/rpc/ws`.

## Hosted gateway methods

`px_*` and `lx_*` methods belong to the operator-deployed
[hosted gateway](../site/docs/platform/gateway-rpc.md). At the public node's
root, `px_getReceiptKeys` and `px_getHistory` return JSON-RPC error `-32601`
(method not available). Obtain a gateway URL from its operator for those
methods; the node names do not expose a public gateway endpoint.
