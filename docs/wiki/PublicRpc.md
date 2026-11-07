# Public node and hosted gateway JSON-RPC

The public JSON-RPC of Paxeer X Network is served at the root path of
`https://api-mainnet-beta.paxeer.network` and `api1` through
`api14.mainnet-beta.paxeer.network` (EVM chain ID `125`, hex `0x7d`). There is
no router, no `/rpc` path and no separate LayerX interface. The proposer is the
executor: a transaction is final once its block is committed.

See [Commitment levels](CommitmentLevels.md), [Assets](Assets.md) and the
[Payments developer path](PaymentsQuickstart.md). The complete endpoint list,
including the archive node, is in
[Public RPC endpoints](../site/docs/reference/public-rpc.md).

## Node endpoints

| Transport | URL |
| --- | --- |
| HTTPS JSON-RPC | `https://api<N>.mainnet-beta.paxeer.network/` for `<N>` from 1 to 14, or `https://api-mainnet-beta.paxeer.network/` |
| WebSocket | `wss://api<N>.mainnet-beta.paxeer.network/` at the same root |

```sh
curl --max-time 15 -sS https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The result is `"0x7d"`. The node namespaces are `debug`, `echo`, `eth`, `net`,
`pax`, `px`, `test` and `txpool`; `test` is registered only on non-live chain
IDs, and availability depends on node configuration. WebSocket subscriptions
use `eth_subscribe` with `newHeads`, `logs` or `newPendingTransactions`.
The retired `lx_*` methods are not served by the node or the hosted gateway.

## The px namespace

The node answers these methods at the root from node state
([`rpc/server.go`](../../rpc/server.go)). The hosted gateway serves the same
names under the contract in
[`services/platform/hosted/gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).
Parameters are positional.

| Method | Positional parameters | Result |
| --- | --- | --- |
| `px_resolveAccount` | `[account]` | `evm_address`, `pax_address`, `layerx_did`, `layerx_account` and `bound`; unbound references are null |
| `px_getAccount` | `[account]` | Resolved `account` and `paxeer` with `address`, hex `balance` and `nonce`, and integer `height` |
| `px_getBalances` | `[account]` | Resolved `account`, integer `height` and `balances` with `denom` and decimal-string `amount` |
| `px_listAssets` | `[]` at the node; `[cursor?]` at the hosted gateway | Registered assets; the hosted gateway pages them with `next_cursor` |
| `px_getNetwork` | `[]` | `network_id`, `paxeer` chain ID and head, and the consensus head |
| `px_getHistory` | `[account, cursor?, limit?, kind?]` | Indexed transfer `items`, `next_cursor`, `indexedHeight`, `chainHeight` and `freshness` |
| `px_getUnifiedHistory` | `[account, cursor?, limit?, kind?]` | Resolved `account`, merged `accounts` and history `items` tagged with `side` |
| `px_listHolderRecords` | `[address, kind, cursor?, limit?]` | Holder-record `items`, `next_cursor`, `indexedHeight` and `chainHeight` |
| `px_getCapabilities` | `[]` | Availability of the native precompile surfaces, `probed_at` and `rpc_height` |
| `px_getRouteCatalogue` | `[]` | Route catalogue with `schema: "paxeer-x.routes.v1"` and `chain_id: 125` |
| `px_getCommit` | `[height]` | Decimal-string `height` and `signedHeader` with consensus `header` and `commit` |
| `px_getValidators` | `[height]` | Decimal-string `height` and `total`, plus `validators` |
| `px_getReceiptKeys` | `[]` | Receipt signing `keys` with `keyId`, `publicKey`, `firstHeight` and `lastHeight` |
| `px_getSequenceBatch` | `[height]` | Decimal-string `height`, hex `tx` and inclusion `proof` |
| `px_register` | `[signer_public_key, registration_signature]` | Hosted operation; the node answers `-32004` |
| `px_requestFunds` | `[did, signer_public_key]` | Hosted operation; the node answers `-32004` |

Consensus read heights are positive decimal `u64` strings, unlike EVM hex
quantities. History cursors are opaque; pass `next_cursor` through unchanged.

`px_register` and `px_requestFunds` are operations of the hosted identity and
faucet services. The node validates their parameters and answers `-32004` with
error data code `hosted_gateway_service_required`. Send them to the
operator-provided hosted gateway origin; `px_requestFunds` requires a hosted
identity session (`Authorization: Bearer <identity session token>`). See
[Hosted faucet](HostedFaucet.md).

## Submitting transactions and reading receipts

Sign a type-2 (EIP-1559) transaction for chain `125` locally and submit the
signed bytes with `eth_sendRawTransaction`. Neither the node nor the hosted
gateway signs for the caller. Read the outcome with `pax_sequencerReceipt`
using the `0x`-prefixed 32-byte transaction hash:

| Rung | Meaning |
| --- | --- |
| `submitted` | Accepted transaction, no execution outcome yet |
| `receipted` | Signed execution receipt, without committed block evidence |
| `committed` | Receipt covered by a committed block, with `blockNumber` and `blockHash` |

A committed receipt must also report successful execution (`status: "0x1"`).
Verify the receipt signature against keys from `px_getReceiptKeys` that are
valid at its height, and verify committed evidence against a trusted consensus
context. See [Protocol](Protocol.md#from-transaction-to-committed-receipt).

## Hosted gateway

The hosted gateway is operator-deployed and accepts `POST /` at its own origin;
obtain that origin and its credentials from the operator. The retired `/rpc`,
`/rpc/schema`, `/rpc/ws` and `/rpc/evm/ws` paths answer HTTP 404. A WebSocket
upgrade at the gateway root is the authenticated EVM subscription relay. The
gateway relays admitted `eth_*`, `net_*` and `web3_*` calls and
`pax_sequencerReceipt` to its configured node pool, refuses node-side signing
methods with `-32601`, and refuses `eth_subscribe` over HTTPS with `-32004`.

Requests use JSON-RPC 2.0 with `Content-Type: application/json`, a single
request or a batch of 1 to 32 entries, within an 8 MiB request bound.
Notification-only requests return HTTP 204. The full method, error and
authentication contract is in
[Hosted gateway JSON-RPC](../site/docs/platform/gateway-rpc.md) and the
[gateway README](../../services/platform/hosted/gateway/README.md).

## Source contract

- [`rpc/server.go`](../../rpc/server.go) and
  [`rpc/testdata/served_methods_http.golden`](../../rpc/testdata/served_methods_http.golden):
  node registrations.
- [`rpc/sequencer_receipt.go`](../../rpc/sequencer_receipt.go): receipt rungs
  and receipt keys.
- [`services/platform/hosted/gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json):
  hosted method contract.
