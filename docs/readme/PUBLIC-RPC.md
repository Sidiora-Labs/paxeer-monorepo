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

WebSocket upgrades for `api-mainnet-beta.paxeer.network` and the fourteen
numbered hosts `api1` through `api14.mainnet-beta.paxeer.network` are served at
the root of each host, the same path as JSON-RPC.
There is no WebSocket upgrade at the retired `/rpc/ws` route.

## The px namespace

The node serves the `px` namespace at the root path beside `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`, answering from node state. The
retired `lx_*` methods are not served. `px_register` and `px_requestFunds`
validate their parameters and answer `-32004` with error data code
`hosted_gateway_service_required`: registration and funding are operations of
the operator-deployed [hosted gateway](../site/docs/platform/gateway-rpc.md),
whose contract is
[`services/platform/hosted/gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).
