<!-- Source: https://docs.paxeer.app/platform-api/ -->

# Platform API

Accounts, balances, history, assets, registration, funding and consensus evidence over the hosted gateway, plus the Agent API for policy-governed writes.

## Choose an interface

The canonical gateway origin is `https://api-mainnet-beta.paxeer.network`. Use the gateway `px_` JSON-RPC methods for account, balance, asset, history and consensus reads, the EVM relay for chain transactions, and the Agent API for preparation, policy and tracking. EVM chain ID 125 identifies the EVM environment. Fetch `GET /rpc/schema` for the gateway's OpenRPC document (*LayerX public JSON-RPC*, version 0.1.0), which is the method contract for the table below.

| Surface | Transport | Purpose |
| --- | --- | --- |
| Gateway RPC | `POST /rpc` | Seventeen `px_` / `pax_sequencerReceipt` methods: accounts, balances, assets, history, registration, funding and consensus reads. |
| EVM relay | `POST /rpc` | `eth_`, `net_` and `web3_` methods relayed to the chain node, including `eth_sendRawTransaction`. See [EVM JSON-RPC](https://docs.paxeer.app/json-rpc). |
| Agent API | Contract major 2, minor 0 | Scoped identities, disclosures, approvals, budgets and event streams. |

The former `lx_` native RPC namespace is switched off. Every `lx_*` method — including `lx_getNodeInfo`, `lx_getAccount`, `lx_sendActivity` and the `lx_subscribe` / `lx_unsubscribe` / `lx_subscription` WebSocket methods — returns `-32601 Method not found`. There is no gateway equivalent of `lx_sendActivity`, `lx_estimateFee`, `lx_getProof` or `lx_getCheckpoint` in the current catalogue; use the Agent API write pipeline for governed submission and the EVM relay for raw transactions.

## Authentication and signing

Public gateway reads do not require a gateway key. State-changing JSON-RPC uses `Authorization: LayerX-Key <key-id>:<key-secret>` with `activity:write`. Hosted identity sessions use `Authorization: Bearer <session-token>`. The gateway credential admits an operation; a cryptographic activity signature authorizes the exact protocol action. Use [scoped identities and credentials](https://docs.paxeer.app/identity) and [budget controls](https://docs.paxeer.app/agent-budgets).

## Query gateway state

```
POST /rpc
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]}
```

Parameters are positional, in the order listed. The router recognises exactly the following seventeen methods; any other method returns `-32601`. Batches may mix `px_` and relayed EVM calls.

| Method | Parameters | Purpose |
| --- | --- | --- |
| `px_getNetwork` | none | Network handshake and heads for the unified endpoint. |
| `px_getAccount` | `account` | Authenticated account object for one account. |
| `px_resolveAccount` | `account` | Resolve an account reference to its account record. |
| `px_getBalances` | `account` | Balances held by one account, read through the bank precompile. |
| `px_listAssets` | none | Native asset list. |
| `px_register` | `signer_public_key`, `registration_signature` | Self-registered identity principal. |
| `px_requestFunds` | `did`, `signer_public_key` | One bounded testnet funding request, authenticated by identity session. |
| `pax_sequencerReceipt` | `txHash` | The node's sequencer receipt and commitment rung, relayed unchanged. |
| `px_getCommit`, `px_getValidators`, `px_getSequenceBatch` | `height` | Signed commit, validator set and sequence batch at one block height. |
| `px_getReceiptKeys` | none | Published receipt signing keys for local receipt verification. |
| `px_getHistory`, `px_getUnifiedHistory` | `account`, optional `cursor`, `limit`, `kind` | Paged account history from the indexer read API; the unified variant tags the side each record answers for. |
| `px_listHolderRecords` | `address`, `kind`, optional `cursor`, `limit` | Paged holder records of one kind for one address. |
| `px_getCapabilities` | none | Which product surfaces are currently available on this endpoint. |
| `px_getRouteCatalogue` | none | Authenticated product route catalogue and dependency readiness. |

Node-side signing methods (`eth_accounts`, `eth_coinbase`, `eth_sendTransaction`, `eth_sign`, `eth_signTransaction`, `eth_signTypedData`, `eth_signTypedData_v4`, `eth_mining`) and `eth_subscribe` / `eth_unsubscribe` are refused at the gateway rather than relayed. A raw transaction addressed to a precompile surface that `px_getCapabilities` reports false is refused with `-32003 surface_unavailable`; check capabilities before submitting.

## Submit through the Agent API

The gateway exposes no native activity-submission RPC. Governed writes go through the Agent API pipeline (prepare, sign, submit, track, wait) or the EVM relay as a signed raw transaction. The steps are:

1. Verify the network with `px_getNetwork` and read the current account state.
2. Prepare canonical activity bytes with an actor, authority, time bounds, fee limit, payload and stable idempotency key.
3. Inspect the decoded disclosure and sign the exact signing preimage.
4. Submit and retain the submission reference, transaction hash and receipt.
5. Verify the evidence — receipt signature against `px_getReceiptKeys`, then commit and validator set at the landing height — before releasing goods, funds or dependent work.

### Idempotency and uncertain outcomes

A timeout does not establish rejection. Reconcile the existing submission and reuse its idempotency key rather than creating a second payment. Agent mutations return the same result for the same body and key; a changed body under the same key returns `IdempotencyConflict`.

## Follow activity

There is no gateway subscription RPC: the `lx_` subscription methods answer method-not-found on WebSocket connections as well as HTTPS. Poll `px_getHistory` or `px_getUnifiedHistory` with a retained cursor for account activity, follow EVM heads and logs on the [chain WebSocket endpoints](https://docs.paxeer.app/public-rpc), and use [agent subscriptions](https://docs.paxeer.app/subscriptions) for durable, acknowledged tenant-scoped streams.

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

Submission progresses through Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed and Expired. Unknown is a first-class pending state. On the wire, progress is reported as one of three rungs — `submitted`, `receipted` and `committed` — and a requested rung above the achieved rung is refused rather than downgraded. Finer internal verification ranks are not part of the Agent API vocabulary.

## Errors and retries

| Code | Meaning | Action |
| --- | --- | --- |
| `-32600` | Invalid request envelope. | Correct the JSON-RPC framing. |
| `-32601` | Method not found, including every switched-off `lx_*` method. | Use the published `px_` catalogue or the EVM relay. |
| `-32003` | `surface_unavailable`: a raw transaction targets a precompile surface reported false by `px_getCapabilities`. | Check capabilities; do not resubmit to a disabled surface. |

Agent errors distinguish TransportFailure, Deadline, ProtocolIncompatibility, UnavailableCapability, CoreRejection, VerificationFailure, PolicyRefusal, CapabilityRefusal, BudgetRefusal, RateLimit, IdempotencyConflict and InternalFault. Resolve authority and verification failures before retrying.

Continue with [CLI workflows](https://docs.paxeer.app/cli), [SDK integration](https://docs.paxeer.app/sdk) and [EVM JSON-RPC](https://docs.paxeer.app/json-rpc).
