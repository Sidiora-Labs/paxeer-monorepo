# Public RPC

Paxeer X Network serves JSON-RPC at the root path of its public node names.
The EVM chain ID is 125 (`eth_chainId` returns `0x7d`); the consensus chain ID
is `hyperpax_125-1`.

## Public JSON-RPC

| Surface | URL |
| --- | --- |
| HTTPS JSON-RPC | `https://api-mainnet-beta.paxeer.network` |

Send requests directly to the node's root path.

```bash
curl -s https://api-mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

## Node names

Use `api1` through `api14.mainnet-beta.paxeer.network` or
`api-mainnet-beta.paxeer.network`, all at the root path.

| Node | HTTPS JSON-RPC | WebSocket |
| --- | --- | --- |
| Canonical alias | `https://api-mainnet-beta.paxeer.network` | `wss://api-mainnet-beta.paxeer.network` |
| 1 | `https://api1.mainnet-beta.paxeer.network` | `wss://api1.mainnet-beta.paxeer.network` |
| 2 | `https://api2.mainnet-beta.paxeer.network` | `wss://api2.mainnet-beta.paxeer.network` |
| 3 | `https://api3.mainnet-beta.paxeer.network` | `wss://api3.mainnet-beta.paxeer.network` |
| 4 | `https://api4.mainnet-beta.paxeer.network` | `wss://api4.mainnet-beta.paxeer.network` |
| 5 | `https://api5.mainnet-beta.paxeer.network` | `wss://api5.mainnet-beta.paxeer.network` |
| 6 | `https://api6.mainnet-beta.paxeer.network` | `wss://api6.mainnet-beta.paxeer.network` |
| 7 | `https://api7.mainnet-beta.paxeer.network` | `wss://api7.mainnet-beta.paxeer.network` |
| 8 | `https://api8.mainnet-beta.paxeer.network` | `wss://api8.mainnet-beta.paxeer.network` |
| 9 | `https://api9.mainnet-beta.paxeer.network` | `wss://api9.mainnet-beta.paxeer.network` |
| 10 | `https://api10.mainnet-beta.paxeer.network` | `wss://api10.mainnet-beta.paxeer.network` |
| 11 | `https://api11.mainnet-beta.paxeer.network` | `wss://api11.mainnet-beta.paxeer.network` |
| 12 | `https://api12.mainnet-beta.paxeer.network` | `wss://api12.mainnet-beta.paxeer.network` |
| 13 | `https://api13.mainnet-beta.paxeer.network` | `wss://api13.mainnet-beta.paxeer.network` |
| 14 | `https://api14.mainnet-beta.paxeer.network` | `wss://api14.mainnet-beta.paxeer.network` |

**Retention:** `api2.mainnet-beta.paxeer.network` is the archive node and keeps
full history; the other thirteen numbered nodes prune. Historical queries
depend on the node's retained state and receipt availability. Send historical
receipt and state queries to
`https://api2.mainnet-beta.paxeer.network` at the root path, on EVM chain ID 125.

Error `-32000` alone does not identify a retention limit. For example,
`pax_sequencerReceipt` returns that code with `pax_sequencerReceipt served by sequencer`
when the node has neither a local receipt source nor a forwarding endpoint.
An unknown transaction hash returns `null` from a configured local receipt source.

Names are numbered, not ranked. Pick any one and fall back to another if it
stops answering; compare `eth_blockNumber` across two names when you need the
newest head.

WebSocket upgrades for `api-mainnet-beta.paxeer.network` and the fourteen
numbered hosts `api1` through `api14.mainnet-beta.paxeer.network` are served at
the root of each host, the same path as JSON-RPC.

## Usage

Confirm the EVM chain ID; the expected result is `"0x7d"`:

```bash
curl -sS https://api1.mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Read the node's current block height:

```bash
curl -sS https://api1.mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
```

Subscribe over the root WebSocket endpoint:

```text
wscat -c wss://api1.mainnet-beta.paxeer.network
> {"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}
```

The node proxy template in `tools/bringup/archive_state.go`, used by
`tools/bringup/install-archive.sh`, also supports `/ws`. Prefer root URLs for
client configuration. Subscription notifications report new heads; consumers
requiring finality must verify the corresponding committed evidence.

## Method contracts

`rpc/server.go` registers HTTP namespaces `debug`, `echo`, `eth`, `net`,
`pax`, `pax2`, `px`, `txpool` and `web3`. WebSocket registrations are `echo`,
`eth`, `net`, `pax`, `px`, `txpool` and `web3`. Method availability depends on
the node configuration. The optional `test` namespace is restricted to
non-live chain IDs. See
[EVM JSON-RPC differences](../site/paxeer-docs/public/markdown/json-rpc-unsupported.md).

The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet services
declared in [Hosted gateway](../wiki/HostedGateway.md). A hosted method is not
automatically available on every public node.

The former unified router and its prefixed RPC routes are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint.

## Limits and conduct

Spread polling load across nodes or use subscriptions. Keep log queries
bounded and reconnect subscriptions after disconnects. After a broadcast
timeout, query the original transaction hash before retrying.

The checked-in node proxy template sets `client_max_body_size 10m` and a
3600-second read/send timeout on `/ws`, with 300 seconds on the root path.
Deployed limits and endpoint availability depend on node configuration.
Use TLS certificate verification and client-side deadlines for all calls.

## The px namespace

The node serves the `px` namespace at the root path, answering from node state.
HTTP registrations also include `debug`, `echo`, `eth`, `net`, `pax`, `pax2`,
`txpool` and `web3`; method availability depends on node configuration.
The `test` API requires `EnableTestAPI` and a non-live chain ID, so it is not
served on mainnet chain ID `hyperpax_125-1`.

`px_register` and `px_requestFunds`
validate their parameters and answer `-32004` with error data code
`hosted_gateway_service_required`: registration and funding are operations of
the operator-deployed [hosted gateway](../site/paxeer-docs/src/app/platform-api/page.tsx),
whose contract is
[`services/platform/hosted/gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).
