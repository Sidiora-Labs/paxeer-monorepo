# Hosted gateway

`layerx-gateway`, from the `layerx-platform-gateway` crate in
`services/platform/hosted/gateway/`, provides hosted account, history,
registration and evidence APIs for Paxeer X Network. Its declared JSON-RPC
contract is `services/platform/hosted/gateway/openrpc.json`; dispatch is in
`src/rpc.rs`, `src/paxeer.rs`, `src/chain_reads.rs` and `src/history.rs`.

## Network and endpoints

Paxeer mainnet-beta is one BFT chain, run by `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel is part of the network and proposes blocks. A receipt is
final when its block is committed. Sub-chains are tenant domains in the
database and can be launched without provisioning another chain.

Public node JSON-RPC uses the operator-provided node root. The node namespaces
are `debug`,
`echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`; method availability depends
on node configuration. Use [Public RPC endpoints](../reference/public-rpc.md)
for node HTTP and WebSocket examples.

The hosted gateway is a separate API producer serving the same network.
Obtain its origin and enabled capabilities from its operator; a checked-in
Ingress manifest is not evidence of a deployed public gateway. The gateway
accepts JSON-RPC at `POST /` with `Content-Type: application/json`. A hosted
`px_` method must not be assumed available on every public node.

## Hosted JSON-RPC contract

The gateway serves **17 methods**, including the node receipt relay below,
and publishes their schema at `GET /openrpc.json`. The checked-in OpenRPC
document also includes five node inference methods, which are filtered from
the gateway schema. Arguments are positional arrays; methods without
arguments use `params: []`.

| Method | Arguments | Result or purpose |
| --- | --- | --- |
| `px_register` | `signer_public_key`, `registration_signature`, optional `email_or_verification_token` | Identity registration, email signup or verification with Ed25519 proof of possession |
| `px_requestFunds` | `did`, `signer_public_key` | Authenticated starter-credit claim through the configured faucet |
| `px_resolveAccount` | `account` | EVM address, Pax address and native identity bindings; unbound fields are null |
| `px_getAccount` | `account` | Resolution, EVM balance and nonce at one chain height |
| `px_getBalances` | `account` | Bound account's bank denoms and decimal-string base-unit amounts at one height |
| `px_listAssets` | optional `cursor`, `limit` | At most 16 indexed assets in registration order, with chain metadata where available |
| `px_getNetwork` | none | Network identifier, EVM head and optional consensus head |
| `px_getHistory` | `account`, optional `cursor`, `limit`, `kind` | Indexed transfers of one EVM address in indexer position order |
| `px_getUnifiedHistory` | `account`, optional `cursor`, `limit`, `kind` | Resolved account history with side tags; current dispatch queries the EVM side |
| `px_listHolderRecords` | `address`, `kind`, optional `cursor`, `limit` | Indexed `budget` or `grant` holder records for an EVM address |
| `px_getCommit` | `height` | Consensus signed header and commit |
| `px_getValidators` | `height` | Full consensus validator set, collecting all pages |
| `px_getSequenceBatch` | `height` | Transaction zero of a consensus block and its verified Merkle inclusion proof |
| `px_getReceiptKeys` | none | Configured receipt signing keys and inclusive height ranges |
| `pax_sequencerReceipt` | `txHash` | Node receipt and `submitted`, `receipted` or `committed` rung, relayed unchanged |
| `px_getCapabilities` | none | Decoded precompile availability at a chain height |
| `px_getRouteCatalogue` | none | Hosted route contracts and dependency readiness |

For example, post this body to the operator-provided gateway root:

```json
{"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]}
```

Consensus `height` arguments are positive decimal `u64` strings, such as
`"123"`. Receipt `txHash` arguments are `0x`-prefixed 32-byte hex strings.
Receipt keys have `0x`-prefixed 32-byte `keyId` and `publicKey` fields;
`firstHeight` and `lastHeight` are JSON integers. Preserve these encodings
when building clients.

Asset cursors are canonical positive decimal signed-64-bit registration-row
IDs. `px_listAssets` returns `height`, `assets`, `joined_limit: 16`,
`next_cursor` and `order: "registration"`. The runtime accepts a limit from
1 to 16, defaulting to 16; the checked-in schema declares the optional cursor.

Account selectors are an EVM address or `did:layerx:<64 hex>`.
`layerx_account` is a distinct native account reference: retain the original
EVM/DID selector for subsequent reads. A bare native account ID is refused
with `native_account_reverse_resolution_unsupported`. Account resolution
uses addr at `0x0000000000000000000000000000000000001004`; bank reads use
`0x0000000000000000000000000000000000001001`. These are precompiles on the
same chain. History comes from the configured indexer and can trail chain
state; its cursors must be passed through unchanged. The current history
implementation orders positions oldest first and dispatches unified history
with only the resolved EVM side.

The gateway also relays admitted `eth_`, `net_` and `web3_` requests to its
configured node pool. Node-side signing/account methods are refused, and
HTTP `eth_subscribe`/`eth_unsubscribe` calls are not relayed. Upstream URLs
with a bare origin use the root path; explicit configured paths are preserved.
Use public node WebSockets for node subscriptions. JSON-RPC batches contain at most
32 entries; a batch containing only notifications returns HTTP 204.

`px_getCapabilities` probes exchange (`0x1015`), bridge (`0x1016`), launchpad
(`0x1017`), ledger (`0x101a`), pay (`0x101b`), auth (`0x101c`) and programs
(`0x101d`), with addresses zero-padded to 20 bytes. Failed or malformed view
calls report the surface as false. Raw transaction submissions addressed to
an unavailable surface are refused with `-32003` and
`data.code: "surface_unavailable"`; reads and history remain available. Capability
results describe decoded views, not successful execution of every operation.

## Tokens and scopes

Registration requires a lowercase 32-byte Ed25519 public key and a lowercase
64-byte signature over the registration binding in `src/rpc_register.rs`.
The binding is SHA-256 of the domain `layerx-register-binding-v1`, configured
tenant and raw public key, each prefixed with its unsigned 64-bit big-endian
byte length. Two arguments provision the principal directly. An optional
email starts signup; a verification token (`vfy_` plus 64 lowercase hex
characters) completes verification and returns session data.
`px_requestFunds` requires a Bearer session authorising the supplied DID's
signer. Its result must contain funding evidence and a settled starter-credit
record. This hosted claim is not an unauthenticated public faucet or a
custody deposit. Ordinary funding uses transfers on the native network; see
[Developer platform](index.md).

Key management uses a Bearer identity session:

| Method and path | Purpose |
| --- | --- |
| `POST /v1/keys` | Issue a key; requires `Idempotency-Key`, signer public key, scopes and quota |
| `GET /v1/keys` | List this principal's keys |
| `DELETE /v1/keys/{id}` | Revoke an owned key |
| `POST /v1/keys/{id}/rotate` | Rotate an owned key with `Idempotency-Key` |

Protected gateway REST routes use `Authorization: LayerX-Key {id}:{secret}`.
Authentication checks the salted secret digest and revocation state in Redis.
A key's principal comes from that durable record. Caller-supplied
`x-layerx-principal` and `x-layerx-api-key` headers are refused before
routing. Session tokens and gateway keys are not forwarded as service tokens.

The key issuer accepts a sorted, nonempty list of at most six scopes:
`activity:write`, `program:call`, `program:read`, `program:simulate`,
`receipt:read` and `state:read`. Scope issuance does not activate retired
routes. The remaining production REST parser requires `receipt:read` for
settlement and receipt reads, and `state:read` for state reads. Missing scopes
return `403 insufficient_scope`. Quotas have `1..=1_000_000` requests per
`1..=2_592_000` seconds.

## Public routes

`src/main.rs::route_with_peer` is the ingress dispatcher. In addition to root
JSON-RPC, key management and configured product routes, it admits:

| Method and path | Behavior |
| --- | --- |
| `GET /livez` | Process liveness, service name and package version |
| `GET /readyz` | Product-scope dependency readiness |
| `GET /readyz/core` | Core-scope dependency readiness |
| `GET /v1/status` | Gateway, durable-store, event-producer, chain, consensus and indexer status |
| `GET /metrics` | Event producer failure and overflow counters |
| `GET /internal/v1/principal` | Authenticated gateway key's principal digest |
| `POST /v1/settle` | Authenticated merchant receipt settlement check |
| `GET /v1/receipts/{id}` | Principal-owned receipt lookup and verification |
| `GET /v1/state` | Validator-authenticated committed state; refuses missing, stale or invalid evidence |

`POST /v1/activities` and the `/v1/programs` route family are retired ingress:
requests return `404 not_found`. All `lx_` JSON-RPC methods, including the
former submission, fee, receipt and checkpoint calls, return `-32601 Method
not found`. Submit signed EVM transactions to node-root
`eth_sendRawTransaction`; recover evidence by the original transaction hash.

Additional hosted product routes are configured through
`LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` and exposed by `px_getRouteCatalogue`.
The binding file must identify chain `125` and match the gateway network and
wire version. Route listings and readiness observations do not prove an
end-to-end product journey.

## TLS and configuration

The default listener uses rustls without client certificate authentication.
`LAYERX_GATEWAY_LISTENER=plain` supports a private hop behind TLS termination;
that mode refuses configured inbound certificate/key variables. The default
listen address is `[::]:9443`; `LAYERX_GATEWAY_LISTEN` overrides it, otherwise
`PORT` selects `[::]:<port>`.

Outbound component endpoints require canonical HTTPS URLs with DNS hosts.
The HTTP client uses a three-second connect timeout and eight-second I/O
timeout. Its TLS connector uses the configured CA and an optional client
identity, with TLS 1.2 as the minimum. Redis requires `rediss://`, TLS and ACL
username/password authentication.

| Configuration | Meaning |
| --- | --- |
| `LAYERX_GATEWAY_TLS_CERT_DER`, `LAYERX_GATEWAY_TLS_KEY_DER` | Inbound DER certificate and PKCS#8 private key for TLS mode |
| `LAYERX_GATEWAY_OUTBOUND_CA_DER` | Outbound HTTPS and Redis trust certificate |
| `LAYERX_GATEWAY_PAXEER_RPC_URLS` | JSON array of two to eight distinct HTTPS node URLs; use node roots |
| `LAYERX_GATEWAY_CONSENSUS_RPC_URL` | Consensus endpoint for commit, validator and batch reads |
| `LAYERX_GATEWAY_INDEXER_URL` | Indexer read endpoint |
| `LAYERX_GATEWAY_RECEIPT_KEYS_FILE` | Receipt key list used for publication and verification |
| `LAYERX_GATEWAY_STATE_VALIDATORS_FILE` | Pinned consensus chain ID and validator set for committed state verification |
| `LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS` | Capability cache lifetime; default 15 seconds |
| `LAYERX_GATEWAY_REDIS_URL` | Durable Redis TLS endpoint |
| `LAYERX_GATEWAY_REDIS_USERNAME_FILE`, `LAYERX_GATEWAY_REDIS_PASSWORD_FILE` | Redis ACL credential files |
| `LAYERX_GATEWAY_NETWORK_ID` | Hosted network identifier |
| `LAYERX_GATEWAY_LXP_WIRE_VERSION` | Numeric current state-commitment protocol version |
| `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID` | Signed protocol network identifier; distinct from EVM chain ID |
| `LAYERX_GATEWAY_IDEMPOTENCY_SECONDS` | Retention from 3600 to 2592000 seconds; default 604800 |
| `LAYERX_GATEWAY_IDENTITY_URL`, `LAYERX_GATEWAY_IDENTITY_TOKEN_FILE` | Identity endpoint and service credential; configured together |
| `LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` | Optional key-provisioning binding key for the identity client |
| `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | Service credential for self-registration; requires identity configuration |
| `LAYERX_GATEWAY_TENANT` | Required registration tenant when provisioning is enabled; 1–95 lowercase letters, digits or `-_.` |
| `LAYERX_GATEWAY_FAUCET_URL`, `LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE`, `LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT` | Hosted starter-credit claim configuration |
| `LAYERX_GATEWAY_TRUSTED_PROXIES` | Trusted proxy addresses/CIDRs for client identity attribution |
| `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | Protected route bindings and allowed origins |

Identity configuration is independent of the optional kernel bundle. It can
be supplied without `LAYERX_GATEWAY_COMPONENT_URL`.

Configuring `LAYERX_GATEWAY_COMPONENT_URL` enables the kernel bundle: component,
authority and program-registry URLs and their `*_TOKEN_FILE` credentials,
plus `LAYERX_GATEWAY_MODULE_REGISTRY_FILE`. `LAYERX_GATEWAY_PUBLIC_CORE_URL`
optionally selects public core reads. Partial kernel configuration is refused.
These boundaries do not create another chain or finality stage.

Outbound service mTLS accepts either
`LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12` with
`LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE`, or
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12` with
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PASSWORD_FILE`. Each pair must be
complete, and the pairs are mutually exclusive. A kernel bundle requires a
client identity. Use the configuration parser in `src/main.rs` for required
combinations.

## Redis state and ACL

`src/store.rs` persists keys, revocation, quotas, idempotency records, activity
ownership, TAP replay bindings and audit events. Important key families are
`gateway:key:*`, `gateway:quota:*`, `gateway:idem:*`, `gateway:pending`,
`gateway:tap:nonce:*` and `gateway:tap:binding:*`. The audit stream is
`gateway:audit`, with hash-chain head `gateway:audit:head`. Audit identities
are digests; credentials and payloads must not be used as audit labels.

The checked-in Redis manifest enables append-only persistence,
`appendfsync always` and `maxmemory-policy noeviction`, with ACL credentials
provided separately. Preserve the store and idempotency records through
restarts. A persistence error is reported separately from invalid credentials;
it must not be treated as evidence that an operation never happened.

## Typed refusals

JSON-RPC errors are carried inside the JSON-RPC envelope, often with HTTP 200.
Inspect `error.code` and `error.data`, rather than treating HTTP success as
operation success.

| JSON-RPC code | Meaning on these paths |
| --- | --- |
| `-32700` | Invalid JSON |
| `-32600` | Invalid request, empty batch or batch above 32 entries |
| `-32601` | Unknown or retired method, including every `lx_` name |
| `-32602` | Invalid method arguments; also `no_sequence_batch` at a height without a batch |
| `-32001` | Unavailable account/chain read, registration or pending faucet claim; inspect `data.code` |
| `-32002` | Registration or funding authorisation refusal |
| `-32003` | Unavailable consensus evidence, receipt keys or precompile surface |
| `-32005` | Public-read, registration or funding rate limit |
| `-32010` | Kernel-dependent method unavailable; data names `code`, `backend` and `reason` |

A kernel refusal uses `code: "kernel_unavailable"` and reason
`not_configured` or `unreachable`. The backend names include
`core_agent_boundary`, `public_core`, `independent_receipt_authority`,
`identity` and `program_registry`; these are diagnostic identifiers.

Protected REST refusals use `{"ok":false,"error":{"code":"..."}}`.
Common examples are `401 api_key_required`, `403 insufficient_scope`,
`429 quota_exceeded` and `503 persistence_unavailable`. Keep retry identity
stable after transport failure or a pending outcome.

## Receipt verification before success

A receipt signature is checked against independently trusted signing keys and
their height ranges. A public key returned beside a receipt is not its own
trust anchor. `px_getReceiptKeys` publishes the gateway's configured list;
missing configuration returns `receipt_keys_unavailable` rather than invented
keys.

`GET /v1/state` reads consensus status and the commit at its reported head.
`LAYERX_GATEWAY_STATE_VALIDATORS_FILE` pins the chain ID and validator set;
its JSON document contains `chain_id` and 1–256 `validators`, each with an
Ed25519 `public_key` in hex and positive integer `voting_power`. The file is
bounded to 65536 bytes and rejects unknown fields and duplicate keys.

The gateway authenticates the header and validator commit, matches the
status height, block hash and application hash, requires `catching_up: false`,
and accepts timestamps at most 120 seconds old or five seconds in the future.
Redis retains the observed height/hash to refuse regression or divergence.
The response includes `canonical_state_root`, `state_root`, `block_height`,
`block_hash`, `timestamp_ms`, `commitment_proof` and
`verification: "bft-signed-committed-header"`.

Missing pins return `503 state_authority_not_configured`; missing node or
commit evidence returns `503 state_node_unavailable` or
`503 state_proof_unavailable`. Invalid evidence returns
`502 state_proof_unverified`; stale or regressing evidence returns
`502 state_proof_stale`.

`POST /v1/settle` binds the merchant request to canonical receipt evidence,
checks receipt authorisation, then queries `pax_sequencerReceipt` and
`eth_getTransactionReceipt` for matching committed inclusion. Pending inclusion
returns HTTP 202. A refusal may be a typed result within HTTP 200: inspect the
result before delivering a resource. This merchant check concerns the same
chain's transaction, not a second settlement network.

For transaction receipt states and payment evidence, see
[Commitment levels](../protocol/commitment-levels.md). Simulation results,
queue acknowledgements and capability flags do not prove committed execution.

## Readiness

`GET /readyz/core` requires a configured, ready chain and all configured
backends to be ready. `GET /readyz` additionally requires
`product_routes.complete: true`. They return HTTP 200 when those conditions
hold and HTTP 503 otherwise. Chain readiness requires `eth_chainId: "0x7d"`;
consensus readiness requires `catching_up: false`.

The `backends` map reports `durable_store`, an enabled `event_producer`,
`paxeer_chain`, `consensus` and `indexer`. Unconfigured consensus and indexer
entries remain explicit and do not alone fail readiness; an unconfigured
chain does. `components` contains `durable_store`. Inspect
`capabilities.chain_serving` and the backend entries for feature availability.

Readiness includes protocol/network identity, observation and expiry times,
and `scope` (`product` or `core`). Product scope includes `product_routes`;
core scope reports it as null. The document is valid for 30 seconds. Readiness
does not attest to a completed payment or program execution.

`GET /v1/status` has a separate summary. Its `hosted_gateway` field is
`available` when every listed backend is ready, `degraded` when that condition
fails but Redis is ready, and `unavailable` otherwise. Unconfigured listed
backends prevent the `available` summary.

## Tests and build entry points

Gateway contract coverage lives alongside the implementation: `src/rpc.rs`
checks retired-method refusal and the exact OpenRPC method set;
`src/chain_reads.rs` covers consensus reads; `src/paxeer.rs` covers chain and
account reads; `src/history.rs` covers indexed pagination; and
`tests/tap_nonce.rs` covers durable authentication and replay handling.
Workspace entry points are declared in `services/platform/Makefile.inc`,
including `platform-build`, `platform-test`, `platform-hosted-smoke` and
`platform-hosted-topology-check`. Hosted checks require configured services.

## Version-2 module registry

When kernel backends are configured, startup reads
`LAYERX_GATEWAY_MODULE_REGISTRY_FILE` and validates schema version 2, unique
nonzero lowercase asset IDs, bounded asset metadata, known module IDs and
valid activity ordinals. This registry is configuration for hosted
verification, not a public submission API. Module declarations do not restore
the retired activity or Programs REST ingress.

[Developer platform](index.md) ·
[Commitment levels](../protocol/commitment-levels.md) · [Home](../index.md)
