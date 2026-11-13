<!-- Source: https://docs.paxeer.app/public-rpc/ -->

# Public RPC endpoints

Connect wallets and applications to Paxeer X chain ID 125 over HTTPS and WebSocket, and choose the correct interface for chain or kernel requests.

## Choose the execution domain

The Paxeer X chain exposes Ethereum JSON-RPC for contracts, balances, logs and transaction broadcasts. The unified router connects the wider network, including LayerX-native requests. An EVM transaction hash and a LayerX activity identifier identify different execution paths; use the matching receipt and commitment interface.

| Interface | Endpoint | Purpose |
| --- | --- | --- |
| Unified router | `https://api-mainnet-beta.paxeer.network` | Network routing and unified interfaces. |
| Chain HTTPS | `https://api1.mainnet-beta.paxeer.network` | Ethereum JSON-RPC requests and broadcasts. |
| Chain WebSocket | `wss://api1.mainnet-beta.paxeer.network/ws` | Ethereum subscriptions and persistent RPC sessions. |

The numbered public full-node names run from `api1.mainnet-beta.paxeer.network` through `api16.mainnet-beta.paxeer.network`. Each supports HTTPS and WebSocket. Numbers identify nodes rather than a preference order. WebSocket connections also accept the root path.

## Verify the chain before signing

| Setting | Value |
| --- | --- |
| EVM chain ID | 125 / `0x7d` |
| Consensus identifier | `hyperpax_125-1` |
| Native gas coin | `uhpx` (bond denom); bank metadata displays PAX at exponent 6 over `uhpx`. EVM amounts use wei, tracked as a sub-`uhpx` remainder. |

```
curl https://api1.mainnet-beta.paxeer.network   -H 'content-type: application/json'   -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Check that the response is `0x7d`, then query `eth_blockNumber` to check head progress. Signing uses the configured chain ID for replay protection.

## Subscribe to chain heads

```
wscat -c wss://api1.mainnet-beta.paxeer.network/ws
{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}
```

Reconnect a dropped WebSocket and recreate subscriptions. Store your last processed block number so you can query missing logs after reconnecting. Deduplicate logs by transaction hash and log index; handle the chain's documented finality semantics before irreversible downstream actions.

## Fallback and request limits

Each endpoint fronts a full node and may briefly trail another node. Compare heads across two nodes when freshness matters. Spread sustained polling across endpoints or use subscriptions. The public-node configuration rejects request bodies above 10 MB and allows up to one hour of WebSocket inactivity.

After a broadcast timeout, query the signed transaction's hash before deciding whether to rebroadcast the identical transaction. An interrupted transport does not establish rejection. LayerX writes use their own idempotency and pending-outcome workflow.

## Continue building

- [Ethereum methods and compatibility](https://docs.paxeer.app/json-rpc)

- [Unsupported Ethereum methods](https://docs.paxeer.app/json-rpc-unsupported)

- [LayerX and platform API reference](https://docs.paxeer.app/platform-api)

- [Execution and settlement commitments](https://docs.paxeer.app/finality)

- [Wallet and frontend integration](https://docs.paxeer.app/building-a-frontend)
