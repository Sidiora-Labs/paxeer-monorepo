# Public node and hosted gateway JSON-RPC

Paxeer mainnet-beta is one BFT network, EVM chain ID `125` (`0x7d`). The kernel
runs inside `paxd`, proposes blocks and executes within the network. A receipt
is final once its block is committed. The public node and an operator-hosted
gateway expose different method contracts over that same network.

## Public node endpoints

Send JSON-RPC to the root of `https://api-mainnet-beta.paxeer.network` or
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. The public namespaces are `debug`,
`echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`; individual methods depend on
node configuration. The full node list and root WebSocket URLs are in
[Public RPC endpoints](../reference/public-rpc.md).

```bash
curl --fail-with-body --max-time 15 \
  https://api1.mainnet-beta.paxeer.network \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain ID result is `"0x7d"`. Check it before signing or
broadcasting, and compare `eth_blockNumber` across nodes when choosing a node
near the head. An unavailable endpoint does not prove a transaction failed.

The pre-merge router and its `/rpc`, `/rpc/ws` and `/rpc/schema` paths are retired.
There is no separate LayerX interface to configure.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.
Sub-chains are tenant domains in the database; creating one does not deploy a
second chain or change the public EVM chain ID.

## Hosted gateway endpoint and request shape

An operator-deployed gateway accepts `POST /` at its own configured origin.
Obtain that origin from the operator. The `px_*` methods below are hosted
contracts, not additional methods promised on every public node. The checked-in
OpenRPC document has no `servers` entry establishing a public gateway origin.

The gateway's method contract is
`services/platform/hosted/gateway/openrpc.json` (OpenRPC 1.3.2).
Compatibility handlers for retired prefixed RPC paths remain in the hosted
source; new clients should use the root. All `lx_*` methods are retired and
return `-32601` before any upstream call, including the former activity,
batch, checkpoint, proof and subscription methods.

Requests use JSON-RPC 2.0 and `Content-Type: application/json`. Hosted method
parameters are positional arrays. The gateway accepts a single request or a
batch of 1–32 requests within its 8 MiB request bound. Valid notifications
without an `id` produce no response; a notification-only exchange returns HTTP
204. JSON-RPC results and errors use HTTP 200. Clients must inspect `error`.

Example hosted read, sent to the operator's gateway root:

```json
{"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]}
```

## Hosted methods

These are the 17 declared method names. Optional trailing arguments can be
omitted; consult OpenRPC for each method's exact selectors and schemas.

| Method | Positional parameters | Result |
| --- | --- | --- |
| `px_register` | `[signer_public_key, registration_signature]` | Hosted identity principal record |
| `px_requestFunds` | `[did, signer_public_key]` | Confirmed faucet claim with `funded`, `funding_id`, `transaction_id`, decimal-string `amount`, `network` and `starter_credit` |
| `px_resolveAccount` | `[account]` | `evm_address`, `pax_address`, `layerx_did`, `layerx_account` and `bound` |
| `px_getAccount` | `[account]` | Resolved `account` and `paxeer` address, hex balance/nonce and integer height; `paxeer` can be null |
| `px_getBalances` | `[account]` | Resolved `account`, integer `height` and `balances` with `denom` and exact decimal-string `amount` |
| `px_listAssets` | `[]` | Integer `height`, joined `assets` and `joined_limit` |
| `px_getNetwork` | `[]` | `network_id`, `paxeer` chain ID/head and `consensus`; a missing consensus snapshot is null with `reason: "consensus_unavailable"` |
| `px_getHistory` | `[account, cursor?, limit?, kind?]` | Transfer `items`, `next_cursor`, `indexedHeight`, `chainHeight` and `freshness` for the resolved address |
| `px_getUnifiedHistory` | `[account, cursor?, limit?, kind?]` | Resolved `account`, merged `accounts`, history `items` tagged with `side`, cursor and freshness fields |
| `px_listHolderRecords` | `[address, kind, cursor?, limit?]` | Holder-record `items`, `next_cursor`, `indexedHeight` and `chainHeight`; kind is `budget` or `grant` |
| `px_getCapabilities` | `[]` | Boolean `exchange`, `bridge`, `launchpad`, `ledger`, `pay`, `auth`, `programs`; `addresses`, integer `probed_at` and decimal-string `rpc_height` |
| `px_getRouteCatalogue` | `[]` | `schema: "paxeer-x.routes.v1"`, `chain_id: 125`, `services`, `routes` and dependency `readiness` |
| `px_getCommit` | `[height]` | Decimal-string `height` and `signedHeader` with consensus `header` and `commit` |
| `px_getValidators` | `[height]` | Decimal-string `height` and `total`, plus `validators` with `address`, `pubKey` and decimal-string `votingPower` |
| `px_getReceiptKeys` | `[]` | Signing `keys` with `keyId`, `publicKey` and inclusive integer `firstHeight`/`lastHeight` |
| `px_getSequenceBatch` | `[height]` | Decimal-string `height`, hex `tx` and inclusion `proof` containing `total`, `index`, `leafHash` and `aunts` |
| `pax_sequencerReceipt` | `[txHash]` | Node receipt lifecycle object or null when the transaction is unknown |

Account resolution reads `addr` at
`0x0000000000000000000000000000000000001004`; bank balances come from
`0x0000000000000000000000000000000000001001`. Null unbound references do not
establish zero balances. `px_getBalances` requires a bound EVM address.
`px_listAssets` joins at most 16 records; `joined_limit` makes that bound
explicit rather than claiming an unbounded inventory.

History cursors are the indexer's 32 lowercase hexadecimal characters encoding
its block/transaction/log position. Pass `next_cursor` through unchanged, or
use null/omit it for the first page. Limits are integers 1–100, default 50.
History amounts are exact decimal strings in asset base units. Indexer height
and chain height describe freshness; a stale or absent indexer is not proof
that an account has no activity.

Consensus read heights are positive decimal `u64` strings, unlike EVM hex
quantities. Receipt key IDs and public keys encode 32 bytes with a `0x` prefix.
The sequence-batch proof covers transaction index zero of the requested block;
a block without a sequence batch returns `-32602` with
`data.code: "no_sequence_batch"`.

## Authentication and onboarding

Public balance, history, capability and consensus reads do not require a
gateway key. `px_register` proves possession of the Ed25519 signer key: its
public key is 64 lowercase hex characters and the signature is 128 lowercase
hex characters, both unprefixed. The signature covers the registration binding
implemented in `src/rpc_register.rs`. Registration is idempotent for that
signer and creates a hosted principal; it does not establish an on-chain DID
binding or complete [Human onboarding](../concepts/identity.md#human-onboarding).

`px_requestFunds` requires a hosted identity session:

```text
Authorization: Bearer <identity session token>
```

The session must authorize the named signer. The gateway derives faucet
idempotency from the principal, DID and signer key. A retry resolves the same
claim. Incomplete funded evidence returns `-32603`, and a pending upstream
claim remains `-32001` with `data.code: "faucet_claim_pending"`.

`px_getRouteCatalogue` requires gateway key authentication even though most
reads are public. Its credential format is:

```text
Authorization: LayerX-Key <key-id>:lxp_live_<64-hex-secret>
```

Missing credentials, disabled keys and mismatched authority are refusals.
Requests carrying caller-supplied `x-layerx-principal` or `x-layerx-api-key`
headers are rejected with HTTP 400 `untrusted_identity_header`.

## Submission and committed receipts

Transactions use the EVM path: sign the exact chain-125 transaction with its
pending nonce and fee bounds, then submit signed bytes with
`eth_sendRawTransaction`, or use the configured gas station's sponsored flow.
The gateway relays supported `eth_`, `net_` and `web3_` calls to its node pool.
It refuses node-side account/signing methods such as `eth_sendTransaction`
and `eth_sign`; callers supply their own signing authority.

`px_getCapabilities` probes the native surfaces. Raw sends addressed to an
unavailable exchange, bridge, launchpad, ledger, pay, auth or programs
precompile are refused with `-32003` and `data.code: "surface_unavailable"`.
A false capability is not a route to a fallback execution network.

Read `pax_sequencerReceipt` using the original `0x`-prefixed 32-byte transaction
hash. The gateway relays the node's lifecycle object unchanged:

| Wire rung | Meaning |
| --- | --- |
| `submitted` | Accepted transaction with `txHash` and hex `nonce`; no execution outcome yet |
| `receipted` | Signed execution receipt and outcome, without committed block evidence |
| `committed` | Receipt covered by a sequence batch in a committed block, including `blockNumber` and `blockHash` |

A committed receipt must still report successful execution (`status: "0x1"`)
for the intended operation. `"0x0"` is a failed execution. The method name is
retained; it does not designate a separate running service. Signing keys,
transaction inclusion and consensus evidence support independent verification.
Neither an acknowledgement nor a timeout establishes a successful outcome.
Resume lookups under the same transaction and journey identity; see
[Human journeys](../human/journeys.md#done-means-committed-success).

The HTTPS relay refuses `eth_subscribe` and `eth_unsubscribe` with `-32004`.
Use the node's root WebSocket endpoint for supported EVM subscriptions. New-head
notifications prompt reads; they do not replace committed receipt verification.

## Errors and limits

These codes describe gateway handling; relayed node errors retain their node
meaning. Method-specific `error.data.code` distinguishes the cause.

| Code | Gateway meaning |
| ---: | --- |
| `-32700` | Invalid JSON |
| `-32600` | Invalid JSON-RPC envelope or batch size |
| `-32601` | Unknown, retired or non-relayable method |
| `-32602` | Invalid parameters or selector; also no sequence batch at a requested height |
| `-32603` | Malformed upstream response or incomplete required evidence |
| `-32001` | Unavailable read, missing configuration or pending faucet claim |
| `-32002` | Authentication or authorization refusal |
| `-32003` | Unavailable chain evidence or precompile surface |
| `-32004` | EVM subscriptions are not relayed over HTTPS |
| `-32005` | Rate-limit refusal on methods using this code |
| `-32010` | Required hosted backend unavailable; `data.code: "kernel_unavailable"` names its backend and reason |

The shared hosted read limiter admits at most 120 reads per second per process
and 30 per attributed client. One hosted operation can consume several upstream
reads. REST limiter refusals use HTTP 429 `public_read_rate_limit` with
`Retry-After: 1`; JSON-RPC error mapping depends on the method. Registration and
faucet claims also have separate 30-per-minute process limits. A wrong JSON
content type returns HTTP 415 `json_content_type_required`.

## Source contract and configuration

All source paths below are relative to `services/platform/hosted/gateway/`:

- `openrpc.json`: declared method parameters, result encodings and errors.
- `src/main.rs` and `src/rpc.rs`: root dispatch, request bound, batch and notification behavior.
- `src/paxeer.rs` and `src/evm.rs`: account resolution, bank reads and EVM relay policy.
- `src/chain_reads.rs`: commit, validator, receipt-key and sequence-batch evidence.
- `src/history.rs` and `src/capabilities.rs`: indexed reads and native surface probes.
- `src/rpc_register.rs`, `src/rpc_faucet.rs` and `src/routes.rs`: identity, funding and authenticated route catalogue.

`LAYERX_GATEWAY_PAXEER_RPC_URLS` is a JSON array of two to eight distinct node
URLs. The relay checks chain ID `125` before using an endpoint.
`LAYERX_GATEWAY_CONSENSUS_RPC_URL` configures consensus reads;
`LAYERX_GATEWAY_RECEIPT_KEYS_FILE` supplies the receipt signing-key list.
Missing upstreams or invalid evidence produce explicit unavailable responses,
not fabricated balances, receipts or finality.

[Human web application](../human/web-app.md) · [Home](../index.md)
