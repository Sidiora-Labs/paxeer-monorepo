<!-- Source: https://docs.paxeer.app/public-rpc/ -->

# Public RPC endpoints

Connect wallets and applications to Paxeer mainnet-beta, EVM chain ID 125, using node JSON-RPC at the endpoint root.

## Connect to the node

Paxeer is one BFT network. The kernel runs inside `paxd`, and the proposer executes transactions and proposes their block. The node exposes Ethereum JSON-RPC for contracts, balances, logs and broadcasts. A receipt is final once its covering block commits.

| Interface | Endpoint | Purpose |
| --- | --- | --- |
| Node HTTP | `PAXEER_RPC_URL` | Your configured root JSON-RPC URL for reads and broadcasts. |
| Node WebSocket | `PAXEER_WS_URL` | Your configured WebSocket URL for subscriptions and RPC sessions. |
| Hosted gateway | `services/platform/hosted/gateway/openrpc.json` | Contract for the hosted gateway’s methods, parameters, results and errors. |

Set the example variables to the endpoints supplied for your deployment. These are client-side variable names, not node configuration keys. HTTP JSON-RPC requests use the endpoint root. Node listener defaults and enabled methods are defined in `rpc/config/config.go` and `rpc/server.go`; public provider limits depend on its configuration.

## Verify the chain before signing

| Setting | Value |
| --- | --- |
| EVM chain ID | 125 / `0x7d` |
| Consensus identifier | `hyperpax_125-1` |
| Native gas coin | `uhpx` (bond denom); bank metadata displays PAX at exponent 6 over `uhpx`. EVM amounts use wei, tracked as a sub-`uhpx` remainder. |

```
curl "$PAXEER_RPC_URL"   -H 'content-type: application/json'   -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Check that the response is `0x7d`, then query `eth_blockNumber` to check head progress. Signing uses the configured chain ID for replay protection.

## Subscribe to chain heads

```
{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}
```

Send the subscription request over your configured WebSocket connection. Reconnect a dropped connection and recreate subscriptions. Store your last processed block number so you can query missing logs after reconnecting. Deduplicate logs by transaction hash and log index; require covering-block commitment before treating an outcome as final.

## Fallback and request limits

A node can trail another node. Compare block heights when freshness matters and use subscriptions where available. Bound event queries: `evm.max_blocks_for_log` defaults to 2000 and `evm.max_log_no_block` defaults to 10000. Operators can configure different limits.

After a broadcast timeout, query the signed transaction's hash before deciding whether to rebroadcast the identical transaction. An interrupted transport does not establish rejection. Use `pax_sequencerReceipt` to distinguish submitted, receipted and committed transactions. The node’s governed receipt-key list is available through `px_getReceiptKeys`.

## Continue building

- [Ethereum methods and compatibility](https://docs.paxeer.app/json-rpc)

- [Unsupported Ethereum methods](https://docs.paxeer.app/json-rpc-unsupported)

- [Hosted platform API reference](https://docs.paxeer.app/platform-api)

- [Receipt and block commitment](https://docs.paxeer.app/finality)

- [Wallet and frontend integration](https://docs.paxeer.app/building-a-frontend)
