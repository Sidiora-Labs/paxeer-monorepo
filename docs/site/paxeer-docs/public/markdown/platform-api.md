<!-- Source: https://docs.paxeer.app/platform-api/ -->

# Platform API

Accounts, balances, history, assets, registration, funding and consensus evidence over the hosted gateway, plus the Agent API for policy-governed writes.

## Choose an interface

Use the node JSON-RPC root for signed chain transactions and EVM reads. Configure the hosted gateway origin separately with coordinates supplied by its operator. Its `px_` methods provide account, balance, asset, history and consensus reads. EVM chain ID `125` identifies the network. The gateway serves `GET /openrpc.json` for its method contract and authenticated `GET /v1/routes` for product route discovery. Source: `services/platform/hosted/gateway/openrpc.json` and `services/platform/hosted/gateway/src/rpc.rs`.

| Surface | Transport | Purpose |
| --- | --- | --- |
| Gateway RPC | `POST /` | Seventeen `px_` / `pax_sequencerReceipt` methods: accounts, balances, assets, history, registration, funding and consensus reads. |
| Node JSON-RPC | `POST /` | `eth_` and `net_` calls at the node root, including `eth_sendRawTransaction` for signed EVM transactions. See [EVM JSON-RPC](https://docs.paxeer.app/json-rpc). |
| Agent API | Contract major 2, minor 0 | Scoped identities, disclosures, approvals, budgets and event streams. |

The gateway exposes hosted reads and route discovery; the Agent API provides governed preparation and tracking. Use signed native transactions for network execution. The node reports receipt progress through `pax_sequencerReceipt`, and a receipt is final when its containing block is committed.

## Authentication and signing

Public account and consensus reads do not require a gateway key. Route discovery requires a gateway API key in the `Authorization` header, using the `LayerX-Key` scheme. Funding requests require an identity bearer session and the matching signer public key. Registration proves possession of its signing key. Authentication and scope requirements follow the individual route; transaction signatures authorize the exact chain action. Use [scoped identities and credentials](https://docs.paxeer.app/identity) and [budget controls](https://docs.paxeer.app/agent-budgets).

## Query gateway state

```
POST /
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]}
```

Parameters are positional, in the order listed. The hosted dispatcher serves the following seventeen methods. JSON-RPC batches contain between 1 and 32 requests. Unsupported methods return `-32601`; unavailable configured backends return explicit errors.

| Method | Parameters | Purpose |
| --- | --- | --- |
| `px_getNetwork` | none | Configured network identity, node head and consensus availability. |
| `px_getAccount` | `account`; or `did`, `asset` | Account resolution and node state; the two-argument form reads a DID account for one Asset. |
| `px_resolveAccount` | `account` | Resolve an account reference to its account record. |
| `px_getBalances` | `account` | Balances held by one account, read through the bank precompile. |
| `px_listAssets` | optional `cursor` | Native asset list. |
| `px_register` | `signer_public_key`, `registration_signature`, optional `email_or_verification_token` | Key-bound principal registration or signup verification. |
| `px_requestFunds` | `did`, `signer_public_key` | One bounded testnet funding request, authenticated by identity session. |
| `pax_sequencerReceipt` | `txHash` | The node's sequencer receipt and commitment rung, relayed unchanged. |
| `px_getCommit`, `px_getValidators`, `px_getSequenceBatch` | `height` as a positive decimal string | Signed commit, validator set and sequence batch at one block height. |
| `px_getReceiptKeys` | none | Published receipt signing keys for local receipt verification. |
| `px_getHistory`, `px_getUnifiedHistory` | `account`, optional `cursor`, `limit`, `kind` | Paged account history from the indexer read API; the unified variant tags the side each record answers for. |
| `px_listHolderRecords` | `address`, `kind`, optional `cursor`, `limit` | Paged holder records of one kind for one address. |
| `px_getCapabilities` | none | Which product surfaces are currently available on this endpoint. |
| `px_getRouteCatalogue` | none | Authenticated product route catalogue and dependency readiness. |

Node-side signing methods (`eth_accounts`, `eth_coinbase`, `eth_sendTransaction`, `eth_sign`, `eth_signTransaction`, `eth_signTypedData`, `eth_signTypedData_v4`, `eth_mining`) and `eth_subscribe` / `eth_unsubscribe` are refused at the gateway rather than relayed. A raw transaction addressed to a precompile surface that `px_getCapabilities` reports false is refused with `-32003 surface_unavailable`; check capabilities before submitting.

## Submit through the Agent API

Governed writes use the Agent API pipeline (prepare, sign, submit, track, wait). Native wallet writes submit signed EVM transactions to the node root. The steps are:

1. Verify the network with `px_getNetwork` and read the current account state.
2. Prepare canonical activity bytes with an actor, authority, time bounds, fee limit, payload and stable idempotency key.
3. Inspect the decoded disclosure and sign the exact signing preimage.
4. Submit and retain the submission reference, transaction hash and receipt.
5. Verify the receipt signature under independently trusted receipt keys, compare published keys through `px_getReceiptKeys`, and verify the commit and trusted validator set at the landing height before releasing goods, funds or dependent work.

### Idempotency and uncertain outcomes

A timeout does not establish rejection. Reconcile the existing submission and reuse its idempotency key rather than creating a second payment. Agent mutations return the same result for the same body and key; a changed body under the same key returns `IdempotencyConflict`.

## Follow activity

Poll `px_getHistory` or `px_getUnifiedHistory` with a retained cursor for account activity, follow EVM heads and logs on the [chain WebSocket endpoints](https://docs.paxeer.app/public-rpc), and use [agent subscriptions](https://docs.paxeer.app/subscriptions) for durable, acknowledged tenant-scoped streams.

### Agent event streams

Agent subscriptions support create, list, health, acknowledge, pause, resume and delete. Their persisted cursor enables recovery. Scope evaluates tenant, capability, then filters for agents, accounts, activity types, modules, assets, counterparties and result classes. Delivery is at least once: deduplicate by `deduplication_id` and handle Event, Gap and Truncated variants.

## Agent API pipeline

The wire envelope contains contract major and minor as big-endian u16, request ID as big-endian u64, operation as UTF-8 and a canonical-map payload. Amounts, sequences, limits and timestamps are decimal strings. Settlement domain is Paxeer.

| Operation | Inputs and outcome |
| --- | --- |
| `prepare` | Actor, authority, account sequence, timestamp bound, idempotency key, fee limit, payload and payload hash produce unsigned canonical bytes, signing preimage, disclosure, preparation reference and expiry. |
| `sign`, `submit` | Preparation reference and signature bind the signed activity and tracked submission. |
| `track` | Submission reference returns state, evidence, rung and transitions. |
| `wait` | Submission reference, requested rung and deadline return the actual rung and whether the deadline elapsed. |
| Reads | `read.balance`, `read.account`, `read.module_state`, `read.history`, `read.batch`, `read.checkpoint`, `read.proof_bundle`. |
| Availability and projection | `availability.fetch`, `export.offline`, `project`. Projection is an estimate rather than a verified read. |

Submission progresses through Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed and Expired. Unknown is a first-class pending state. On the wire, progress is reported as one of three rungs — `submitted`, `receipted` and `committed` — and `wait` reports the actual rung with `deadline_elapsed` when its deadline expires. A `receipted` result proves execution; `committed` establishes block finality.

## Errors and retries

| Code | Meaning | Action |
| --- | --- | --- |
| `-32600` | Invalid request envelope. | Correct the JSON-RPC framing. |
| `-32601` | Method not found in the hosted dispatcher. | Use the hosted method catalogue or send node methods to the node root. |
| `-32003` | `surface_unavailable`: a raw transaction targets a precompile surface reported false by `px_getCapabilities`. | Check capabilities; do not resubmit to a disabled surface. |

Agent errors distinguish TransportFailure, Deadline, ProtocolIncompatibility, UnavailableCapability, CoreRejection, VerificationFailure, PolicyRefusal, CapabilityRefusal, BudgetRefusal, RateLimit, IdempotencyConflict and InternalFault. Resolve authority and verification failures before retrying.

Continue with [CLI workflows](https://docs.paxeer.app/cli), [SDK integration](https://docs.paxeer.app/sdk) and [EVM JSON-RPC](https://docs.paxeer.app/json-rpc).
