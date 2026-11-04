# Hosted core

`layerx-platform-core` is the crate and `layerx-core-boundary` is its binary
([crate manifest](../../services/platform/hosted/core/Cargo.toml)). It provides
hosted HTTPS reads and administrative funding/reset operations for the merged
Paxeer network. The kernel runs inside the network and proposes blocks through
`paxd`; a receipt is final once its block is committed. Core is an HTTP boundary,
not a second execution or settlement system.
The standalone `layerxd` daemon and its LNI/replica operating model are retired.

Public node JSON-RPC uses the root HTTPS URL of `api1.mainnet-beta.paxeer.network`
through `api14.mainnet-beta.paxeer.network`, or
`api-mainnet-beta.paxeer.network`, with EVM chain id **125** (`0x7d`). The node
namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`.
The hosted gateway implements the `px_` contract and accepts JSON-RPC at
`POST /`; those methods are not an additional public node namespace. Its declared
contract is [openrpc.json](../../services/platform/hosted/gateway/openrpc.json),
and its dispatch lives in
[gateway main.rs](../../services/platform/hosted/gateway/src/main.rs) and its
`rpc.rs` module. Submit signed chain
transactions with `eth_sendRawTransaction`; consult `px_getCapabilities` for
hosted surface availability. Sub-chains are tenant domains in the database,
without a new node deployment for each tenant.

Core has separate TLS listeners, defaulting to `0.0.0.0:9443` and
`0.0.0.0:9444`. Core routes use the first; admin routes use the second. Configuring
a core client CA validates certificates that clients present but still allows
clients without a certificate. The admin TLS listener uses no client-certificate
authentication; its protected routes require a bearer token. These are the
`server_tls_config`, `config` and `platform_core` functions in
[core main.rs](../../services/platform/hosted/core/src/main.rs).

The [node deployment](../../services/platform/hosted/node/deployment.yaml) runs
`paxd`, the treasury signer and `core-boundary`. The core container runs as user
4021, group 4020, under `generation_client.py core`, which supplies generation
configuration before starting the binary. HTTPS probes use `/readyz` and
`/livez`. The manifest is a hosted testnet configuration, not a declaration that
all hosted facilities are deployed on the public network.

---

## Config keys

The authoritative startup parser is `config` in
[main.rs](../../services/platform/hosted/core/src/main.rs).

| Variable | Role |
| --- | --- |
| `LAYERX_CORE_LISTEN` | Core TLS bind; default `0.0.0.0:9443` |
| `LAYERX_CORE_ADMIN_LISTEN` | Admin TLS bind; default `0.0.0.0:9444` |
| `LAYERX_CORE_TLS_CERT_DER`, `LAYERX_CORE_TLS_KEY_DER` | Required core certificate and PKCS#8 private-key file paths, both DER |
| `LAYERX_CORE_ADMIN_TLS_CERT_DER`, `LAYERX_CORE_ADMIN_TLS_KEY_DER` | Required admin certificate and PKCS#8 private-key file paths, both DER |
| `LAYERX_CORE_CLIENT_CA_DER` | Optional core client CA file; client certificates remain optional |
| `LAYERX_CORE_NETWORK_ID` | Required nonzero `u32` protocol network id; distinct from the EVM chain id |
| `LAYERX_CORE_SEQUENCER_ENDPOINT` | Required HTTPS JSON-RPC endpoint used by `SequencerEndpoint` and `PxEndpoint`; the deployment selects the hosted gateway |
| `LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER` | Required DER trust anchor for that endpoint; nonempty, at most 1 MiB |
| `LAYERX_CORE_SEQUENCER_ID` | Required 32-byte hex identity retained by the funding recovery contract; its name does not imply a separate service |
| `LAYERX_CORE_TREASURY_SIGNER_SOCKET` | Required treasury signer socket; startup connects before opening listeners |
| `LAYERX_CORE_TREASURY_ASSET` | Required nonzero 32-byte hex asset id retained for funding recovery |
| `LAYERX_CORE_ADMIN_TOKEN_FILE` | Required admin bearer secret file |
| `LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE` | Optional bearer secret for internal readiness and receipt-event reads |
| `LAYERX_CORE_WALLET_CAPS_TOKEN_FILE` | Optional token loaded by the startup parser; not a public authorization mechanism |
| `LAYERX_CORE_SUPERVISOR_SOCKET` | Required supervisor socket path for administrative reset |
| `LAYERX_CORE_STATE_DIR` | Required durable directory; startup creates `journal/`, `funding-intents/` and `funding-receipts/` with mode `0700` |
| `LAYERX_CORE_FEE_LIMIT` | Parsed `u64`, default `1000`, retained in core configuration; not the gas-price setting for native funding |
| `LAYERX_CORE_RECEIPT_DEADLINE_MS` | Receipt wait for retained ledger-funding recovery; default `15000` milliseconds |
| `LAYERX_CORE_CHAIN_RPC_URL` | Funding JSON-RPC URL; plaintext HTTP on `127.0.0.1` or `localhost`, explicit port, no non-root path; deployment uses `http://127.0.0.1:8545` |
| `LAYERX_CORE_FUNDING_SENDER_KEY_FILE` or `LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID` | Alternative EVM sender sources; configuring both is refused |

Secret-file loading strips trailing CR/LF and rejects empty values or values over
4096 bytes. Treasury DID signatures use
[custody.rs](../../services/platform/hosted/core/src/custody.rs); the EVM funding
sender is configured separately through
[chain_fund.rs](../../services/platform/hosted/core/src/chain_fund.rs).

The retired LNI socket configuration is no longer read. Although the deployment
still carries `LAYERX_CORE_NODE_URL`, `LAYERX_CORE_NODE_BEARER_TOKEN_FILE`,
`LAYERX_CORE_REPLICA_URL` and `LAYERX_CORE_REPLICA_BEARER_TOKEN_FILE`, `config`
initializes those internal endpoint/token fields empty. They do not establish a
replica connection or a second execution path.

---

## Public routes

The core-plane dispatcher is `core_route` / `protocol_route` in
[main.rs](../../services/platform/hosted/core/src/main.rs), with read handlers in
[public_reads.rs](../../services/platform/hosted/core/src/public_reads.rs).
Successful reads normally use `{"ok":true,"result":...,"trace":...}`;
liveness and readiness return their own documents. Query strings are rejected on
most routes; retired compatibility routes have their own handling.

| Method | Path | Current behavior |
| --- | --- | --- |
| `GET` | `/livez` | `200 {"live":true}`; does not probe dependencies |
| `GET` | `/readyz` | Native committed-state and journal checks, plus independent funding status |
| `GET` | `/v1/sequencer` | Compatibility name for the network read: `network_id`, `paxeer`, `layerx`, `latest_committed_height` |
| `GET` | `/v1/state` | Native committed `height`, `block_hash`, `state_root`, `commitment: "native_app_hash"` |
| `GET` | `/v1/accounts/{selector}` or `/v1/accounts/{selector}/balance` | Same native account snapshot; selector is a `0x` 20-byte EVM address or nonzero `did:layerx:` 32-byte key |
| `GET` | `/v1/dids/{did}/sequence` | Bound EVM address's pending nonce, with `verification: "sequencer_pending_nonce"`; unknown DID is `404 unknown_did` |
| `GET` | `/v1/dids/{did}/accounts` | DID account enumeration through the configured network endpoint |
| `GET` | `/v1/assets` or `/v1/assets/{id}` | Asset list or detail; detail id must be nonzero 32-byte hex |
| `POST` | `/v1/fees/estimate` | JSON containing exactly `canonical_hex`; unsigned chain-125 transaction fee bound |
| `GET` | `/v1/node-info` | Network, receipt-key and checkpoint metadata using compatibility field names |
| `GET` | `/v1/batches/{number}` | Canonical nonzero decimal block number; result contains `batch_number`, not a signed batch header |
| `GET` | `/v1/checkpoints/{id}` | Confirms the current checkpoint hash; result contains `checkpoint_id`, not a proof bundle |
| `GET` | `/v1/receipts/{hash}` | Authenticated receipt bytes with its native landing `block_hash` and `state_root`; hash is 32-byte lowercase hex |
| `GET` | `/v1/proofs/{activity\|receipt}/{hash}` | Validates available receipt/landing evidence, then refuses unavailable native inclusion material |
| `GET` | `/v1/proofs/account/{hash}/{selector}` | Refuses unavailable native account inclusion material after receipt/selector validation |

Account reads compare the joined account with balance and nonce at one height,
then recheck chain id and head. A changed head returns
`503 account_snapshot_changed`; inconsistent values return
`502 account_snapshot_mismatch`. Balance is a decimal string in wei with
`balance_decimals: 18`; `next_sequence` is the native nonce. A legacy 32-byte
account hash is not a supported account selector.

Fee estimation validates a nonzero gas limit and priority fee no greater than
max fee, then returns `fee = gas_limit * max_fee_per_gas` as a decimal string,
`parameter_version: 0`, empty `canonical_schedule`, and `canonical_bytes`.
It does not reserve funds or provide a committed fee-schedule proof.

## Admin routes

On the admin listener, `GET /livez` and `GET /readyz` precede bearer checks.
Every other request requires `Authorization: Bearer <secret>` matched in constant
time against `LAYERX_CORE_ADMIN_TOKEN_FILE`. Query strings are refused.

| Method | Path | Contract |
| --- | --- | --- |
| `POST` | `/admin/v1/testnet/fund` | JSON `funding_id`, `did`, `public_key`, `amount`; unknown fields refused; requires `Idempotency-Key` |
| `POST` | `/admin/v1/testnet/reset` | Exact body `{}` and `Idempotency-Key`; delegates reset to the supervisor |
| `POST` | `/admin/v1/testnet/funding-intents` | Authenticated recovery import; exact intent schema and matching idempotency header are validated |
| `GET` | `/admin/v1/testnet/funding-intents/{key}` | Authenticated recovery export for the selected funding intent |

Fund/reset require `Content-Type: application/json`. Missing bearer returns
`401 unauthorized`; missing content type or idempotency header returns
`400 content_type_required` or `400 idempotency_key_required`.
Recovery payloads are internal formats implemented by `funding_intent_route` and
[funding_native.rs](../../services/platform/hosted/core/src/funding_native.rs),
not a public transaction-submission API.

## Transaction submission and receipt lookup

The old Activity and program lifecycle HTTP submission routes are retired.
Core no longer dispatches `/v1/activities`, `/v1/programs/call`, deploy, upgrade,
wind-down or simulate requests; they fall through to `404 not_found`. The hosted
gateway also rejects Activity/program HTTP paths. Use signed native chain
transactions and the gateway's declared JSON-RPC contract instead.

`receipt_route` looks up authenticated node receipts using the published receipt
keys. `await_receipt` reads committed state at the transaction's landing height;
`validate_receipt_landing` checks transaction position/hash, block number,
execution status and any covering-block commitment. The exposed state root is
the native app hash, not a replica's batch root. Authentication or landing
failures return `503 receipt_unavailable`. The current lookup also reports
absent or merely submitted transactions as unavailable once its deadline is
exhausted; callers must not assume an absent receipt always yields 404.

## Readiness

`readiness` requires successful chain-id/head reads, native committed block and
app-hash evidence, and a durable writable journal probe. Its successful document
contains `ready: true`, `network_id: "eip155:<chain_id>"`, `paxeer`,
`latest_committed_height`, `block_hash`, `app_hash`, `wire_version: "3"`,
`synchronous_receipts: true` and `state_snapshot: true`. There is no replica
probe. Failures include `503 node_unavailable`, `committed_chain_unavailable`
and `journal_unavailable`, with a five-second retry hint.

A separate `funding` object reports the funding chain/sender dependency. Funding
can be unavailable while the outer readiness response remains 200. Inspect that
object before treating core as ready to fund. `GET /internal/readyz` exposes the
same checks only with the receipt-events bearer token; missing configuration
returns `503 gateway_readiness_not_configured`.

## State reads, proofs and retired contracts

`GET /v1/protocol/account-state/head` is retained as a compatibility read of
native committed state. Other retired contracts explicitly return
`410 retired_http_contract` for their supported GET form:

- `/v1/programs/account-state/changes`
- `/v1/receipts/{id}/account-state` and `/v1/programs/{id}/account-state`
- `/v1/batches/{id}/receipt-authority`
- `/v1/programs/receipts/by-idempotency/{key}`
- `/v1/programs/events/{topic}/{from_sequence}/{limit}`

The account-state/receipt-authority selectors require 64 hex characters. Event
selectors require a nonempty lowercase hex topic of at most 64 decoded bytes,
a canonical decimal sequence and a limit from 1 through 256. Invalid event
selectors return 400 rather than implying a working event stream.

Proof reads currently refuse `native_account_inclusion_unavailable` or
`native_receipt_inclusion_unavailable` with HTTP 503 when inclusion material is
missing. A native block/app hash alone does not prove an account balance or a
receipt's inclusion. Conflicting landing/header evidence returns
`502 proof_landing_mismatch` or `502 proof_root_mismatch`.

## Typed capability refusals

`/v1/accounts`, `/v1/programs/registry` and its descendants, and
`/v1/programs/activities/` descendants return `503 capability_unavailable` with
a 3600-second retry hint. The by-idempotency prefix is handled first as retired.
Do not infer support from a shared prefix or a compatibility field name.

`GET /internal/v1/receipt-events/{hash}` requires the receipt-events bearer token.
It returns available receipt event facts, 202 while pending, or a typed
unavailable response. This internal feed does not replace public receipt
verification.

## Admin funding and durable recovery

Funding sends one native value-bearing EIP-1559 transaction on chain 125.
`chain_fund.rs` resolves the recipient's registered EVM binding through the
address precompile at `0x0000000000000000000000000000000000001004`, signs with the
configured funding sender, submits the raw transaction and validates its native
transaction receipt. Missing identity binding returns
`409 identity_binding_unavailable`. The treasury DID socket authenticates funding
recovery records; it is distinct from the EVM sender's transaction key.

The fund body uses a positive `u64` amount, a bounded funding id, and a DID matching
`did:layerx:<public_key>`; it cannot name the treasury DID. Confirmation returns
`state: "funded"`; an unconfirmed submission returns 202 with
`state: "pending"` and its `transaction_id`.

The journal binds each idempotency key to the method, path and body digest.
A different request under the same key returns `409 idempotency_conflict`.
Native funding stages persist signed transaction bytes and authenticated intents
before submission, and archive confirmed receipts for restart recovery. Pending,
unknown and retryable results can be retried with the same key; they are not
permanent success records. Retained version-one ledger intents follow their
original chain recovery path rather than being reinterpreted as native value
transfers. Keep the complete state directory durable across restarts.

Reset uses the supervisor request/status protocol and the same durable
idempotency handling. A pending reset is not proof that reset completed.

## Failure HTTP statuses

Refusals have `{"error":{"code":"...","retry":"never"}}`, or
`retry: "after"` with `retry_after_seconds` and a `Retry-After` header.
Read the route-specific code together with its HTTP status.

| Status | Examples |
| --- | --- |
| 400 | `invalid_request`, `unsupported_account_selector`, `invalid_fee_request`, `invalid_idempotency_key` |
| 401 | `unauthorized` |
| 404 | `not_found`, `unknown_did` |
| 405 | `method_not_allowed` on recognized routes |
| 409 | `idempotency_conflict`, `identity_binding_unavailable`, `funding_sender_conflict`, `outcome_unknown` |
| 410 | `retired_http_contract` |
| 502 | `account_snapshot_mismatch`, `proof_landing_mismatch`, `proof_root_mismatch`, `invalid_asset_symbol` |
| 503 | `node_unavailable`, `committed_chain_unavailable`, `journal_unavailable`, `receipt_unavailable`, `treasury_unavailable`, `capability_unavailable` |

## Boundary test sources

The current coverage is under
[core/tests](../../services/platform/hosted/core/tests), rather than the retired
standalone-daemon boundary harness. Relevant sources include:

- [repair_22_31_354.rs](../../services/platform/hosted/core/tests/repair_22_31_354.rs):
  native committed-state readiness, explicit retired routes, native asset reads
  and binding receipt evidence to its actual landing block.
- [chain_fund.rs](../../services/platform/hosted/core/tests/chain_fund.rs):
  native funding, journal replay, startup-outage recovery and rejection of
  retired Activity/program routes.
- [signer.rs](../../services/platform/hosted/core/tests/signer.rs): treasury signer
  transport and refusal coverage.

These source references describe coverage; deployment readiness requires its own
recorded qualification result.

[Hosted gateway](HostedGateway.md) · [Hosted node](HostedNode.md) ·
[Public JSON-RPC](PublicRpc.md) · [Home](Home.md)
