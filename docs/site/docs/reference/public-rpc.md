# Public RPC endpoints

Paxeer X Network is one BFT network with EVM chain ID `125` (`0x7d`). The
kernel runs inside the node and participates in block proposal and execution.
Receipts become final when their block is committed; see
[Checkpoints](../concepts/checkpoints.md).

The public node JSON-RPC is served over HTTPS at the root path. WebSocket
clients can also use the root path. The canonical alias below is a direct
node endpoint, with the same chain identity as the numbered nodes.

## Endpoints

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
full history; the other thirteen numbered nodes prune. Historical query
availability depends on the node's block, state and receipt stores; see
[Data availability](../protocol/data-availability.md#retention-and-recovery).
`pax_sequencerReceipt` returns `null` for an unknown transaction and can return
`-32000` when no local receipt source or forwarding is configured. Send
historical receipt and state queries to
`https://api2.mainnet-beta.paxeer.network` at the root path, on EVM chain ID 125.
There is no router or separate LayerX interface.

Nodes are numbered, not ranked. Keep more than one configured, use bounded
timeouts and compare `eth_blockNumber` when choosing a node near the head.
Check `eth_chainId` before signing or broadcasting. A slow or unreachable
endpoint does not establish that a submitted transaction failed.

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
[EVM JSON-RPC differences](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/docs/evm_jsonrpc_unsupported.md).

The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [Hosted gateway JSON-RPC](../platform/gateway-rpc.md). A hosted
method is not automatically available on every public node.

The former unified router and its prefixed RPC routes are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint.
Sub-chains are tenant domains in the database; creating one does not require
a new chain endpoint or change the public EVM chain ID.

## Limits and conduct

Spread polling load across nodes or use subscriptions. Keep log queries
bounded and reconnect subscriptions after disconnects. After a broadcast
timeout, query the original transaction hash before retrying.

The checked-in node proxy template sets `client_max_body_size 10m` and a
3600-second read/send timeout on `/ws`, with 300 seconds on the root path.
Deployed limits and endpoint availability depend on node configuration.
Use TLS certificate verification and client-side deadlines for all calls.
