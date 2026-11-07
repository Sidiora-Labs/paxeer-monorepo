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

Public node JSON-RPC is served at the root of
`https://api-mainnet-beta.paxeer.network` and
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. The node namespaces are `debug`,
`echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`; method availability depends
on node configuration. Use [Public RPC endpoints](../reference/public-rpc.md)
for node HTTP and WebSocket examples.

The hosted gateway is a separate API producer serving the same network.
Obtain its origin and enabled capabilities from its operator; a checked-in
Ingress manifest is not evidence of a deployed public gateway. The gateway
accepts JSON-RPC at `POST /` with `Content-Type: application/json`. A hosted
`px_` method must not be assumed available on every public node.

The former unified router and its `/rpc`, `/rpc/schema` and `/rpc/ws` public contract are retired.
The separate LayerX interface and standalone daemon deployment model are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.

## Hosted JSON-RPC contract

OpenRPC declares **17 methods**, including the node receipt relay below.
Arguments are positional arrays. Methods without arguments use `params: []`.

| Method | Arguments | Result or purpose |
| --- | --- | --- |
| `px_register` | `signer_public_key`, `registration_signature` | Self-service identity registration bound to proof of possession of an Ed25519 key |
| `px_requestFunds` | `did`, `signer_public_key` | Authenticated starter-credit claim through the configured faucet |
| `px_resolveAccount` | `account` | EVM address, Pax address and native identity bindings; unbound fields are null |
| `px_getAccount` | `account` | Resolution, EVM balance and nonce at one chain height |
| `px_getBalances` | `account` | Bound account's bank denoms and decimal-string base-unit amounts at one height |
| `px_listAssets` | none | At most 16 indexed assets, sorted by asset ID, with chain metadata where available |
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

Account selectors are an EVM address or `did:layerx:<64 hex>`.
`layerx_account` is a distinct native account reference: retain the original
EVM/DID selector for subsequent reads. A bare native account ID is refused
with `native_account_reverse_resolution_unsupported`. Account resolution
uses addr at `0x0000000000000000000000000000000000001004`; bank reads use
`0x0000000000000000000000000000000000001001`. These are precompiles on the
same chain. History comes from the configured indexer and can trail chain
state; its cursors must be passed through unchanged. The current history
implementation orders positions oldest first and dispatches unified history
with only the resolved EVM side. The OpenRPC prose describing newest-first
pages and inclusion of every native account overstates this implementation;
clients must not assume those behaviors.

The gateway also relays admitted `eth_`, `net_` and `web3_` requests to its
configured node pool. Node-side signing/account methods are refused, and
HTTP `eth_subscribe`/`eth_unsubscribe` calls are not relayed. The current
`src/capabilities.rs::chain_rpc` still rewrites bare upstream origins to a
retired prefixed path, which conflicts with the public node-root contract.
This implementation mismatch must be resolved before relying on a gateway
relay configured with public node roots. Use public
node WebSockets for node subscriptions. JSON-RPC batches contain at most
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
64-byte signature over the registration binding described in OpenRPC.
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
| `GET /v1/status` | Gateway, kernel boundary, chain and indexer status |
| `GET /metrics` | Event producer failure and overflow counters |
| `GET /internal/v1/principal` | Authenticated gateway key's principal digest |
| `POST /v1/settle` | Authenticated merchant receipt settlement check |
| `GET /v1/receipts/{id}` | Principal-owned receipt lookup and verification |
| `GET /v1/state` | Verified backend state; refuses missing, stale or invalid proof evidence |

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
| `LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS` | Capability cache lifetime; default 15 seconds |
| `LAYERX_GATEWAY_REDIS_URL` | Durable Redis TLS endpoint |
| `LAYERX_GATEWAY_REDIS_USERNAME_FILE`, `LAYERX_GATEWAY_REDIS_PASSWORD_FILE` | Redis ACL credential files |
| `LAYERX_GATEWAY_NETWORK_ID` | Hosted network identifier |
| `LAYERX_GATEWAY_LXP_WIRE_VERSION` | Numeric current state-commitment protocol version |
| `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID` | Signed protocol network identifier; distinct from EVM chain ID |
| `LAYERX_GATEWAY_IDEMPOTENCY_SECONDS` | Retention from 3600 to 2592000 seconds; default 604800 |
| `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | Service credential for self-registration |
| `LAYERX_GATEWAY_FAUCET_URL`, `LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE`, `LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT` | Hosted starter-credit claim configuration |
| `LAYERX_GATEWAY_TRUSTED_PROXIES` | Trusted proxy addresses/CIDRs for client identity attribution |
| `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | Protected route bindings and allowed origins |

Kernel-dependent hosted boundaries are optional gateway backends. Configuring
`LAYERX_GATEWAY_COMPONENT_URL` enables their configuration bundle, including
`LAYERX_GATEWAY_AUTHORITY_URL`, `LAYERX_GATEWAY_IDENTITY_URL`,
`LAYERX_GATEWAY_PROGRAM_REGISTRY_URL` and the corresponding
`*_TOKEN_FILE` credentials. `LAYERX_GATEWAY_PUBLIC_CORE_URL` configures public
kernel reads. The bundle uses `LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12`,
`LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE`,
`LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` and
`LAYERX_GATEWAY_MODULE_REGISTRY_FILE`. Partial kernel configuration is refused.
These service boundaries do not create an additional chain or finality stage.

A deployment without that bundle may supply
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12` and
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PASSWORD_FILE` together for service
mTLS. The service identity pair conflicts with the kernel client identity.
Use the runtime configuration parser in `src/main.rs` as the source for
required combinations; deployment YAML values are not mainnet defaults.

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

`GET /v1/state` verifies the backend head receipt, batch signature and
inclusion proof against configured receipt keys before returning state facts.
Unavailable or noncurrent evidence returns
`503 principal_state_proof_unavailable`; invalid proof returns
`502 state_proof_unverified`. This signed head evidence does not replace
committed-block verification.

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

`GET /readyz` and `GET /readyz/core` report configured dependency health.
They return HTTP 200 when all configured backends are ready, otherwise 503.
Unconfigured backends do not make that aggregate fail. Inspect the individual
`backends` and `capabilities.chain_serving` entries before assuming a feature
is usable. Chain readiness requires an `eth_chainId` result of `0x7d`;
consensus readiness requires a valid status with `catching_up: false`.

Readiness includes protocol/network identity, observation and expiry times,
and `scope` (`product` or `core`). Product scope additionally includes
`product_routes`. The document is valid for 30 seconds. A dependency probe
or a ready route listing does not attest to a completed payment or program
execution.

With no kernel bundle, kernel component entries are omitted. With a bundle,
its five backends and `principal_state_boundary` are included; the latter is
currently `unavailable`. `GET /v1/status` has a different summary shape:
`hosted_gateway` is `degraded` when Redis is ready and `unavailable` otherwise.
It does not report that field as `available`.

## Tests and build entry points

Gateway contract coverage lives alongside the implementation: `src/rpc.rs`
checks retired-method refusal and the exact OpenRPC method set;
`src/chain_reads.rs` covers consensus reads; `src/paxeer.rs` covers chain and
account reads; `src/history.rs` covers indexed pagination; and
`tests/tap_nonce.rs` covers durable authentication and replay handling.
Historical scripts that submit to retired ingress are not current integration
instructions.

Workspace entry points are declared in `services/platform/Makefile.inc`,
including `platform-build`, `platform-test`, `platform-hosted-smoke` and
`platform-hosted-topology-check`. Hosted checks require real configured
services; a source listing does not establish that those checks passed.

## Version-2 module registry

When kernel backends are configured, startup reads
`LAYERX_GATEWAY_MODULE_REGISTRY_FILE` and validates schema version 2, unique
nonzero lowercase asset IDs, bounded asset metadata, known module IDs and
valid activity ordinals. This registry is configuration for hosted
verification, not a public submission API. Module declarations do not restore
the retired activity or Programs REST ingress. Do not use the former daemon
socket preparation procedure as a merged-network bootstrap step.

[Developer platform](index.md) ·
[Commitment levels](../protocol/commitment-levels.md) · [Home](../index.md)
