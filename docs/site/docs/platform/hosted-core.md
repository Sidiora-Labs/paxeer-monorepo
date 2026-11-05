# Hosted core

Hosted core is the TLS HTTP service implemented by the `layerx-platform-core`
crate and `layerx-core-boundary` binary in `services/platform/hosted/core`.
It provides hosted reads and testnet administration backed by the merged
Paxeer network. `paxd` runs the kernel as part of the network and proposes
blocks. Receipts become final when their block is committed.

The public node JSON-RPC is at the root of
`https://api-mainnet-beta.paxeer.network` and the numbered
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network` hosts, with EVM chain ID `125`
(`0x7d`). See [Public RPC endpoints](../reference/public-rpc.md). The node
namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`.
Hosted gateway `px_` methods are declared in
`services/platform/hosted/gateway/openrpc.json`; configure that gateway where
core needs those methods. A direct node URL alone does not supply the gateway
contract. Sub-chains are tenant domains in the database, not additional node
infrastructure.

## Deployment and trust

`docker/platform-core/Dockerfile` builds the boundary and runs it as user
`4020:4020`. The deployment definition in
`services/platform/hosted/node/deployment.yaml` runs the core container as
`4021:4020`, alongside the supervised `paxd` container and treasury signer.
It sources `/run/layerx/node/core.env` before starting the boundary.
These are repository deployment definitions, not evidence of a live deployment.

Core and admin bind separately, by default on `0.0.0.0:9443` and
`0.0.0.0:9444`. Both use TLS. The core plane can validate a presented client
certificate against its optional CA, but allows clients without certificates.
The admin plane uses no TLS client authentication; its privileged routes
require a bearer token. Keep the admin listener on the trusted service network.

Core creates `SequencerEndpoint` and `PxEndpoint` clients from the configured
HTTPS endpoint and DER trust anchor. The retained `SEQUENCER` configuration
names identify this network transport; they do not require a separate service.
Funding uses a separate loopback node JSON-RPC connection. The old LNI socket,
replica listener and batch-authority readiness dependency are retired.

## Config keys

`config()` in `services/platform/hosted/core/src/main.rs` reads these keys.

| Variable | Role |
| --- | --- |
| `LAYERX_CORE_LISTEN` | Core TLS bind; default `0.0.0.0:9443` |
| `LAYERX_CORE_ADMIN_LISTEN` | Admin TLS bind; default `0.0.0.0:9444` |
| `LAYERX_CORE_TLS_CERT_DER`, `LAYERX_CORE_TLS_KEY_DER` | Core certificate and PKCS#8 key file paths |
| `LAYERX_CORE_ADMIN_TLS_CERT_DER`, `LAYERX_CORE_ADMIN_TLS_KEY_DER` | Admin certificate and PKCS#8 key file paths |
| `LAYERX_CORE_CLIENT_CA_DER` | Optional CA file for presented core client certificates |
| `LAYERX_CORE_SEQUENCER_ENDPOINT` | Required HTTPS network/gateway endpoint |
| `LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER` | Required DER CA file for the endpoint; nonempty, at most 1 MiB |
| `LAYERX_CORE_NETWORK_ID` | Required nonzero `u32` deployment network identifier; distinct from EVM chain ID |
| `LAYERX_CORE_SEQUENCER_ID` | Required 32-byte hex identity retained for receipt/recovery compatibility |
| `LAYERX_CORE_TREASURY_ASSET` | Required nonzero 32-byte hex asset identifier retained for receipt/recovery compatibility |
| `LAYERX_CORE_TREASURY_SIGNER_SOCKET` | Required treasury signer Unix socket; startup obtains its public key and fails if connection fails |
| `LAYERX_CORE_ADMIN_TOKEN_FILE` | Required admin bearer token file |
| `LAYERX_CORE_STATE_DIR` | Required durable directory; contains `journal`, `funding-intents` and `funding-receipts` |
| `LAYERX_CORE_SUPERVISOR_SOCKET` | Required Unix socket path for testnet reset |
| `LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE` | Optional bearer token for internal receipt events and gateway readiness |
| `LAYERX_CORE_WALLET_CAPS_TOKEN_FILE` | Optional wallet capability token file loaded by configuration |
| `LAYERX_CORE_FEE_LIMIT` | Retained fee-limit setting; default `1000` |
| `LAYERX_CORE_RECEIPT_DEADLINE_MS` | Receipt deadline; default `15000` ms |

Native funding configuration is read in `src/chain_fund.rs`:

| Variable | Role |
| --- | --- |
| `LAYERX_CORE_CHAIN_RPC_URL` | Plain HTTP loopback node URL with explicit port; no path other than trailing slashes, query or credentials; deployment value `http://127.0.0.1:8545` |
| `LAYERX_CORE_FUNDING_SENDER_KEY_FILE` | EVM funding sender key file; choose this or the KMS key ID |
| `LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID` | EVM funding sender KMS key ID; ambiguous signer sources are refused |

An absent or unusable funding configuration is reported in the funding
readiness subsection and refuses funding requests; it is not a substitute for
core network readiness. Secret-file reads strip trailing CR/LF and reject
empty values or values longer than 4096 bytes. The treasury signing key stays
behind `SocketSigner`; its authorization role is separate from the EVM funding
sender that pays for native value transfers.

The legacy `LAYERX_CORE_NODE_URL`, `LAYERX_CORE_NODE_BEARER_TOKEN_FILE`,
`LAYERX_CORE_REPLICA_URL` and `LAYERX_CORE_REPLICA_BEARER_TOKEN_FILE` entries
remain in the deployment YAML, but current `config()` does not read them.
They do not configure active transports.

## Public routes

Routes are selected by `core_route`, `protocol_route` and
`src/public_reads.rs`. Successful read envelopes generally contain
`ok`, `result` and `trace`; health responses are direct JSON documents.

| Method | Path | Current behavior |
| --- | --- | --- |
| `GET` | `/livez` | `200` with `{"live":true}` |
| `GET` | `/readyz` | Network committed-state and journal checks described below |
| `GET` | `/v1/sequencer` | Compatibility route returning network identity and committed head from `px_getNetwork` |
| `GET` | `/v1/state` | Native committed height, block hash and app hash as `state_root`, with `commitment: "native_app_hash"` |
| `GET` | `/v1/protocol/account-state/head` | Compatibility alias for the native committed-state read |
| `GET` | `/v1/accounts/{selector}` or `/v1/accounts/{selector}/balance` | Native balance and nonce joined with account identity |
| `GET` | `/v1/dids/{did}/sequence` | Bound EVM address's pending nonce, with `verification: "sequencer_pending_nonce"` |
| `GET` | `/v1/dids/{did}/accounts` | Hosted DID account listing |
| `GET` | `/v1/assets` or `/v1/assets/{id}` | Asset listing or asset metadata |
| `POST` | `/v1/fees/estimate` | JSON object with exactly one `canonical_hex` field; invalid body is `400 invalid_fee_request` |
| `GET` | `/v1/node-info` | Network head and receipt-key information |
| `GET` | `/v1/batches/{number}` | Native block lookup by canonical nonzero decimal height; returns `batch_number` as a compatibility field |
| `GET` | `/v1/checkpoints/{id}` | Nonzero 32-byte hex hash matched against the current committed checkpoint; unavailable or mismatched evidence is refused |
| `GET` | `/v1/proofs/{kind}/{id}` or `/v1/proofs/account/{activity}/{account}` | Evidence lookup; unsupported selectors or unavailable evidence return typed refusals |
| `GET` | `/v1/receipts/{id}` | Authenticated receipt lookup; `id` is 32 bytes encoded as lowercase hex |

Account selectors and balance units are explained in [Accounts](../concepts/accounts.md).
Core checks the gateway identity join against EVM balance and nonce at a fixed
height. A changing head is `503 account_snapshot_changed`; inconsistent values
are `502 account_snapshot_mismatch`. Missing native evidence is refused rather
than reported as a zero balance.

The old public activity and program submission routes, including
`/v1/activities`, `/v1/programs/call`, `/v1/programs/deploy`,
`/v1/programs/upgrade`, `/v1/programs/wind-down` and `/v1/programs/simulate`,
are no longer registered by core and fall through to `404 not_found`.
Use the current node or hosted gateway transaction contract for submission;
consult its OpenRPC contract and [Public payment API](public-api.md).

Retired account-state change, per-receipt/program account-state,
receipt-authority, program-event and by-idempotency receipt HTTP contracts
return `410 retired_http_contract` for supported GET forms. The native head
alias above remains available. The unavailable account collection and program
registry/activity collection paths return `503 capability_unavailable` with
`retry_after_seconds: 3600`. Method and selector validation can produce `400`
or `405` before a retirement response; these paths are not legacy relays.

## Readiness

`GET /readyz` reads chain ID and block height, obtains the native committed
block and app hash, and writes and syncs a journal probe. Its failures are
`503 node_unavailable`, `503 committed_chain_unavailable` or
`503 journal_unavailable`, each with a five-second retry hint.

Success includes `ready`, `network_id` in `eip155:<chain ID>` form, `paxeer`,
`latest_committed_height`, `block_hash`, `app_hash`, `wire_version: "3"`,
`synchronous_receipts: true` and `state_snapshot: true`. It also includes a
`funding` object. Core readiness can return `200` while `funding.ready` is
false; inspect that object before using testnet funding.

`GET /internal/readyz` runs the same check after authenticating the configured
receipt-events bearer. Missing configuration is
`503 gateway_readiness_not_configured`; a wrong bearer is `401 unauthorized`.
The internal receipt-event route `/internal/v1/receipt-events/{hash}` uses the
same token, returns `202` for pending receipts, and refuses missing evidence
with `503 receipt_events_unavailable`. These internal views are not additional
finality stages.

## Admin routes

`GET /livez` and `GET /readyz` are available on the admin listener before
bearer authorization. Every other admin request requires
`Authorization: Bearer <admin token>`, compared in constant time. Queries are
rejected. The fund and reset POST routes require
`Content-Type: application/json` and a valid `Idempotency-Key`. Keys are
1–128 ASCII letters, digits or characters from `-_.:`.

| Method | Path | Input and outcome |
| --- | --- | --- |
| `POST` | `/admin/v1/testnet/fund` | Exact JSON fields `funding_id`, `did`, `public_key`, `amount`; `200` funded or `202` pending with transaction ID |
| `POST` | `/admin/v1/testnet/reset` | Exact body `{}`; `200` reset or `202` pending with reset ID |
| `GET` | `/admin/v1/testnet/funding-intents/{key}` | Authenticated recovery export for a stored funding intent |
| `POST` | `/admin/v1/testnet/funding-intents` | Authenticated recovery import validated against its request, identity, signatures and any archived receipt |

Funding requires a nonzero `u64` amount in wei, a valid funding ID, and a DID
matching `did:layerx:` plus the lowercase public-key hex. The treasury DID
cannot be the recipient. The native path resolves the recipient through
`getEvmAddrByLayerX` at address precompile
`0x0000000000000000000000000000000000001004`; an unbound recipient is
`409 identity_binding_unavailable`.

The funding sender prepares an EIP-1559 native value transfer on chain `125`
using its pending nonce, gas estimate and fee readings. Core persists the
signed intent before broadcast and reuses it on retry. It checks the native
transaction receipt against that transfer and archives the result. A missing
receipt stays pending; a failed receipt is `422 funding_failed`. A different
configured sender or chain for a saved intent is
`409 funding_sender_conflict`. See `src/chain_fund.rs` and
`src/funding_native.rs` for the transaction and recovery contracts.

Reset talks to the supervisor with versioned JSON status/admission requests
bound to a reset ID, network and request digest. Before admission it retains
funding receipts. Ambiguous or admitted work remains pending and is reconciled
on retry; it does not send an unversioned reset command or rebuild independent
execution infrastructure.

## Durable idempotency and recovery

Keep `LAYERX_CORE_STATE_DIR` on persistent storage and preserve its journal,
funding intents and archived receipts together. The journal key commits to
scope and HTTP idempotency key; the request digest commits to method, path and
body. A changed request under the same key is `409 idempotency_conflict`.

`journal_write` appends newline-delimited records, syncs data and syncs the
parent directory on first creation. `journal_read` recovers the last complete
record and supports the older single-record format. An incomplete trailing
record is repaired on the next append. Fund and reset retries reconcile
pending, unknown and retryable outcomes, while terminal outcomes replay.
Native funding recovery validates the signed transfer, treasury authorization,
request binding and receipt archive before accepting an import.

A timeout or dependency refusal does not prove that a transaction failed.
Retry the identical request with the same key and use the stored transaction
or reset identity to reconcile it. Do not discard durable records to recover
service availability.

## Failure HTTP statuses

Refusals contain `error.code`, `error.retry` and, when retryable,
`error.retry_after_seconds`; the HTTP response also carries `Retry-After`.

| Status | Examples |
| --- | --- |
| `400` | `invalid_request`, `invalid_argument`, `unsupported_account_selector`, `invalid_fee_request`, `content_type_required`, `idempotency_key_required`, `invalid_idempotency_key` |
| `401` | `unauthorized` |
| `404` | `not_found`, `unknown_did` |
| `405` | `method_not_allowed` |
| `409` | `idempotency_conflict`, `outcome_unknown`, `identity_binding_unavailable`, `funding_sender_conflict` |
| `410` | `retired_http_contract` |
| `422` | `funding_failed`, or reset dependency refusals remapped by `admin_result` |
| `502` | `account_snapshot_mismatch`, invalid account evidence |
| `503` | `node_unavailable`, `committed_chain_unavailable`, `journal_unavailable`, `capability_unavailable`, `treasury_unavailable`, `receipt_unavailable` |

There is no blanket conversion of admin 5xx responses to 422. The current
`admin_result` is used for reset and preserves `journal_unavailable` as 503;
other reset dependency 5xx responses become 422. Funding errors follow their
native path's status. Inspect the code and retry hint as well as the status.

[Accounts](../concepts/accounts.md) · [Public RPC endpoints](../reference/public-rpc.md) · [Home](../index.md)
