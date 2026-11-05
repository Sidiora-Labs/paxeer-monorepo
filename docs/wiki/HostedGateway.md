# Hosted gateway

`layerx-gateway` is the hosted HTTP and receipt-verification boundary for
Paxeer X Network. The crate is `layerx-platform-gateway`, with its binary in
[`src/main.rs`](../../services/platform/hosted/gateway/src/main.rs).
The kernel runs inside the network and proposes blocks. A receipt is final
once its block is committed. Hosted identity, indexing and product APIs do
not form another execution or settlement network. Sub-chains are tenant
domains in the database, launchable without new infrastructure.

## Public nodes and hosted gateway

The public node JSON-RPC endpoints are the root paths of
`https://api-mainnet-beta.paxeer.network` and `https://api1.mainnet-beta.paxeer.network`
through `https://api14.mainnet-beta.paxeer.network`. EVM chain ID is `125`
(`0x7d`). Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and
`txpool`. Hosted `px_*` methods belong to the gateway, not the node API.
Obtain the gateway origin and enabled product routes from its operator.
The checked-in Ingress names the public node hostname; that manifest alone
does not establish a deployed public gateway there.

The unified router and its `/rpc`, `/rpc/schema` and `/rpc/ws` endpoints are retired.
There is no separate LayerX interface or sequencer service in the merged network.
`node.hyperpaxeer.com` is the build-time library mirror, not a JSON-RPC endpoint.

The current gateway dispatcher serves JSON-RPC at `POST /` with
`Content-Type: application/json`. It accepts one request or a batch of 1–32
requests. JSON-RPC replies and errors use HTTP 200; notifications produce no
reply, and an all-notification request returns HTTP 204. Invalid JSON is
`-32700`; an invalid request or batch is `-32600`. All `lx_*` names now answer
`-32601 Method not found`. The removed `/rpc/schema` route is unavailable;
read the checked-in OpenRPC file instead.

Use this request against a public node to identify the network:

```sh
curl https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

For hosted methods, send the request to the operator-provided gateway root:

```sh
curl "${LAYERX_GATEWAY_URL%/}/" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"px_getReceiptKeys","params":[]}'
```

See the [OpenRPC contract](../../services/platform/hosted/gateway/openrpc.json)
and [`src/rpc.rs`](../../services/platform/hosted/gateway/src/rpc.rs) for the
current request contract. [Public JSON-RPC](PublicRpc.md) and
[Public payment API](PublicAPI.md) provide related background; older examples
of activity submission there do not restore the removed methods on this gateway.

## Hosted JSON-RPC methods

OpenRPC and `ROUTER_METHODS` in
[`src/lib.rs`](../../services/platform/hosted/gateway/src/lib.rs) declare
17 method names: the following 16 `px_*` methods and `pax_sequencerReceipt`.
This count excludes the EVM methods relayed to a configured node.
Parameters are positional; use `[]` explicitly for methods without arguments.

| Method | Parameters | Result or dependency |
| --- | --- | --- |
| `px_register` | `[signer_public_key, registration_signature]` | Self-service identity principal after Ed25519 proof of key possession; both arguments are lowercase hex, 32 and 64 bytes respectively |
| `px_requestFunds` | `[did, signer_public_key]` | Authenticated starter-credit claim through identity and faucet; requires a Bearer session that authorizes the signer |
| `px_resolveAccount` | `[account]` | EVM address or `did:layerx:<64 hex>` resolution through the address precompile; unbound fields are null |
| `px_getAccount` | `[account]` | Resolution and EVM balance, nonce and height from one chain snapshot |
| `px_getBalances` | `[account]` | Bank balances at one height; amounts are exact decimal strings in base units |
| `px_listAssets` | `[]` | Up to 16 indexer assets, sorted by asset ID, with chain metadata at the returned height; unavailable metadata remains null |
| `px_getNetwork` | `[]` | Network identifier, EVM chain ID and head, plus consensus head; missing consensus yields null with `consensus_unavailable` |
| `px_getCommit` | `[height]` | Consensus `signedHeader` at a positive decimal-string height |
| `px_getValidators` | `[height]` | Full validator set collected across consensus pages |
| `px_getReceiptKeys` | `[]` | Configured receipt signing keys and inclusive height ranges; absent configuration is `-32003 receipt_keys_unavailable` |
| `px_getSequenceBatch` | `[height]` | Transaction zero of the consensus block, checked as a sequence batch, with verified Merkle inclusion proof |
| `px_getHistory` | `[account, cursor, limit, kind]` | Indexed transfer history of an EVM address, including asset metadata and indexer freshness |
| `px_getUnifiedHistory` | `[account, cursor, limit, kind]` | Account resolution and merged indexed history for its bound account references |
| `px_listHolderRecords` | `[address, kind, cursor, limit]` | Indexed `budget` or `grant` records held by an EVM address |
| `px_getCapabilities` | `[]` | Seven precompile availability flags, their addresses, probe time and chain height |
| `px_getRouteCatalogue` | `[]` | Authenticated product route catalogue with dependency readiness; requires `LayerX-Key` |
| `pax_sequencerReceipt` | `[txHash]` | Node response relayed unchanged for a `0x`-prefixed 32-byte hash; unknown hashes return null |

History cursors are opaque indexer cursors. Holder-record cursors are 32
lowercase hex characters; holder page size defaults to 50 and is bounded at
100. Use the OpenRPC schemas for optional argument types and result fields.
A native account ID is not a DID public key: retain the original EVM address
or explicit DID selector for repeated resolution, balance and history reads.

The gateway relays permitted `eth_*`, `net_*` and `web3_*` methods through its
configured node pool. Node-side signing methods are refused; ordinary HTTP
relay does not implement `eth_subscribe` or `eth_unsubscribe`. Submit signed
transactions using `eth_sendRawTransaction` or `eth_sendRawTransactionSync`.
The `submitted`, `receipted` and `committed` rungs of `pax_sequencerReceipt`
describe progress within this one network. A signed receipt before block
commit is not finality evidence. See [Commitment levels](CommitmentLevels.md).

Capability probes use `eth_call` at one height, rather than `eth_getCode`,
since native precompiles need not carry bytecode. The probed surfaces are
exchange `0x1015`, bridge `0x1016`, launchpad `0x1017`, ledger `0x101a`, pay
`0x101b`, auth `0x101c` and Programs `0x101d` (20-byte addresses padded with
leading zeros). Account resolution uses `0x1004`; bank reads use `0x1001`.
A transaction addressed to an unavailable probed surface is refused with
`-32003 Surface unavailable` and `data.code=surface_unavailable`. Reads and
history are not gated by those flags. The probe cache defaults to 15 seconds.

## TLS

The default listener uses rustls without client-certificate authentication.
`LAYERX_GATEWAY_TLS_CERT_DER` supplies the server certificate and
`LAYERX_GATEWAY_TLS_KEY_DER` supplies the PKCS#8 private key. Explicit
`LAYERX_GATEWAY_LISTENER=plain` selects a plain listener and refuses either
certificate variable being set. The hosted manifest uses HTTPS. The binary
bounds requests at 8 MiB, live connections at 256, and requests per connection
at 128; the checked-in nginx Ingress has a smaller `512k` body limit.

Outbound HTTPS uses `native_tls`, TLS 1.2 or later and
`LAYERX_GATEWAY_OUTBOUND_CA_DER`. Configured HTTPS endpoints must use DNS
hosts rather than literal IP addresses. Kernel backend configuration requires
a PKCS#12 client identity. Without it, a paired service client identity may
be configured for other HTTPS backends; it cannot coexist with the kernel
client identity. Connect and I/O timeouts are 3 and 8 seconds.

Redis requires `rediss://`, the outbound CA and ACL username/password
files. Its checked-in server configuration has `tls-auth-clients no`;
Redis authentication uses `AUTH`. See
[`src/http.rs`](../../services/platform/hosted/gateway/src/http.rs),
[`src/store.rs`](../../services/platform/hosted/gateway/src/store.rs) and the
[deployment manifest](../../services/platform/hosted/gateway/deployment.yaml).

## Tokens

| Credential | Accepted use |
| --- | --- |
| `Authorization: LayerX-Key {id}:{secret}` | Durable key authentication for protected gateway HTTP routes and the product catalogue |
| `Authorization: Bearer {session}` | Issue, list, revoke or rotate keys; authenticated `px_requestFunds` |
| Component, authority, identity and registry token files | Outbound Bearer authorization to the configured backend; caller credentials are not substituted for these tokens |
| PKCS#12 identity and password file | Outbound HTTPS client identity |
| Redis username and password files | Redis ACL authentication |

`authenticate_gateway_key` looks up the key in Redis and checks a
length-prefixed SHA-256 digest over `gateway-key-v1`, the stored salt and the
presented secret using a constant-time comparison. Issued secrets have the
`lxp_live_` prefix followed by 64 hex characters derived by the issuance
path. Revoked keys and digest mismatches are authentication failures;
a store failure remains a persistence error.

Key management introspects the caller's session through
`POST /v1/sessions/introspect`, sending the session token in the JSON body
and the configured identity service token in the outbound Authorization
header. Issue and rotation require that the session authorize the requested
signer. Secrets are derived for replayable issuance; Redis stores their
digests. Caller-supplied `x-layerx-principal` and `x-layerx-api-key` headers
are refused as `400 untrusted_identity_header` before normal authentication.

## Scopes

Key issuance accepts a sorted, non-empty list of at most six scope strings:
`activity:write`, `program:call`, `program:read`, `program:simulate`,
`receipt:read` and `state:read`. Acceptance of a scope does not enable a
removed route. The current `ProductionRoute` permission mapping is:

| Scope | Routes |
| --- | --- |
| `receipt:read` | `POST /v1/settle`, `GET /v1/receipts/{id}` |
| `state:read` | `GET /v1/state` |

Missing scope is `403 insufficient_scope`. Key quotas use finite fixed
windows: 1–1,000,000 requests over 1–2,592,000 seconds. Protected state,
receipt and settlement reads consume the Redis-backed quota. Public chain
reads use a separate in-process budget of 120 reads/second overall and 30
per client. One hosted request can perform several metered upstream reads.
Forwarded client addresses are trusted only from configured trusted proxies.

## Public routes

The route order in `route_with_peer` is authoritative. The old
`/v1/activities` and `/v1/programs` families are removed and return 404,
including requests with query strings. The production parser admits only
settlement, state and receipt routes; emulator administration is excluded.
Additional product proxies use the checked-in
[route catalogue](../../tools/paxeer-x/route-catalogue.json) and operator
bindings in [`src/routes.rs`](../../services/platform/hosted/gateway/src/routes.rs).
Their catalogue entries describe request contracts, not proof that a feature
is deployed or has completed an end-to-end operation.

| Method and path | Authentication and behavior |
| --- | --- |
| `POST /` | JSON-RPC; public reads, with method-specific authentication for catalogue and funding |
| `GET /livez` | No credential; `status=live`, `service=layerx-gateway`, package version |
| `GET /readyz` | No credential; backend readiness and product dependency report |
| `GET /readyz/core` | No credential; backend readiness with product route report omitted |
| `GET /v1/status` | No credential; service availability summary |
| `GET /metrics` | Event producer failure and overflow counters |
| `POST /v1/keys` | Bearer session, `Idempotency-Key`; JSON signer, scopes and quota settings |
| `GET /v1/keys` | Bearer session; principal's key metadata |
| `DELETE /v1/keys/{id}` | Bearer session; revoke an owned key |
| `POST /v1/keys/{id}/rotate` | Bearer session and `Idempotency-Key`; replace an owned key |
| `GET /internal/v1/principal` | `LayerX-Key`; durable principal digest |
| `GET /v1/routes` | `LayerX-Key`; catalogue plus dependency readiness |
| `GET /v1/routes/{service}` | `LayerX-Key`; service routes and health; private services are refused |
| `GET /v1/receipts/{id}` | `LayerX-Key`, `receipt:read`, configured component; owner-scoped receipt verification |
| `GET /v1/state` | `LayerX-Key`, `state:read`, configured component; verified current head evidence |
| `POST /v1/settle` | `LayerX-Key`, `receipt:read`, configured component; JSON merchant evidence checked against chain commitment |

Receipt path identifiers are exactly 64 ASCII hex characters, including
uppercase. Settlement consumes the seller's existing evidence; it does not
submit another execution. Its `settled`, `pending` and `refused` outcomes
must be read from the response body: HTTP 200 alone does not mean settlement
succeeded. Pending chain inclusion returns HTTP 202.

## Config keys

The following keys are read by the current binary. A backend variable or a
checked-in service hostname does not establish a public network endpoint.

| Key | Role |
| --- | --- |
| `LAYERX_GATEWAY_LISTEN` | Socket address; otherwise `PORT` selects `[::]:{port}`, then default `[::]:9443` |
| `LAYERX_GATEWAY_LISTENER` | `tls` (default) or `plain` |
| `LAYERX_GATEWAY_TLS_CERT_DER`, `LAYERX_GATEWAY_TLS_KEY_DER` | TLS server certificate and PKCS#8 key |
| `LAYERX_GATEWAY_OUTBOUND_CA_DER` | Required HTTPS and Redis trust certificate |
| `LAYERX_GATEWAY_REDIS_URL` | Required Redis TLS origin |
| `LAYERX_GATEWAY_REDIS_USERNAME_FILE`, `LAYERX_GATEWAY_REDIS_PASSWORD_FILE` | Required Redis credentials |
| `LAYERX_GATEWAY_NETWORK_ID` | Required bounded network label |
| `LAYERX_GATEWAY_LXP_WIRE_VERSION` | Required numeric current state-commitment protocol version |
| `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID` | Required signed-activity protocol network number; distinct from EVM chain ID |
| `LAYERX_GATEWAY_PAXEER_RPC_URLS` | Optional JSON array of 2–8 distinct HTTPS node endpoints |
| `LAYERX_GATEWAY_CONSENSUS_RPC_URL` | Optional consensus JSON-RPC endpoint for commits, validators, batch inclusion and head |
| `LAYERX_GATEWAY_INDEXER_URL` | Optional history and asset indexer origin |
| `LAYERX_GATEWAY_RECEIPT_KEYS_FILE` | Optional pinned receipt-key list; required for receipt verification and publication |
| `LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS` | Probe cache lifetime, 1–300 seconds; default 15 |
| `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | Optional product upstream bindings; catalogue presence alone is insufficient |
| `LAYERX_GATEWAY_TRUSTED_PROXIES` | Trusted proxy CIDRs for client-address attribution |
| `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | Self-service registration authorization |
| `LAYERX_GATEWAY_FAUCET_URL`, `LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE`, `LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT` | Faucet integration and expected starter-credit amount |
| `LAYERX_GATEWAY_IDEMPOTENCY_SECONDS` | Retention 3,600–2,592,000 seconds; default 604,800 |

Optional kernel backend integration is selected by
`LAYERX_GATEWAY_COMPONENT_URL`. Its group comprises component, authority,
identity and program-registry URLs and their respective `*_TOKEN_FILE`
variables, `LAYERX_GATEWAY_PUBLIC_CORE_URL`,
`LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12`,
`LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE`,
`LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` and
`LAYERX_GATEWAY_MODULE_REGISTRY_FILE`. Setting members of this group without
the component URL is a startup error. These are hosted backend configuration
names retained in the implementation, not a second network topology.

Without that group, outbound service mTLS may use the paired
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12` and
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PASSWORD_FILE` variables.

There is a remaining configuration limitation in `capabilities::chain_rpc`:
it rewrites a bare node origin to the retired `/rpc` path, although public
nodes serve JSON-RPC at the root.
`Endpoint::parse` also normalizes a single trailing slash to an empty base
path, so adding that slash does not bypass the rewrite. Check the deployed
upstream routing rather than assuming a bare public node URL works in this
build. The checked-in testnet manifest also supplies only one node-pool
entry, while the current parser requires at least two. These are existing
source/manifest limitations, not instructions to use the retired topology.

## Redis state and ACL

The checked-in deployment runs Redis 8.2.1 with TLS port 6379, a 50 GiB
persistent volume, `appendonly yes`, `appendfsync always`,
`maxmemory-policy noeviction`, `protected-mode yes` and an ACL file.
The gateway shares durable authentication, quota and operation state across
replicas; do not clear that state to recover an uncertain operation.

| Key family | Purpose |
| --- | --- |
| `gateway:key:{id}` | Principal digest, signer, salted credential digest, scopes, quota, epoch and disabled flag |
| `gateway:principal:{digest}:keys` | Principal's key IDs, bounded at 128 |
| `gateway:quota:{key_id}:{window}` | Fixed-window usage |
| `gateway:idem:{scope}` | Durable operation digest, state, response, receipt and continuation |
| `gateway:pending` | Pending operation references |
| `gateway:activity:{id}`, `gateway:activity-operation:{id}` | Activity owner and operation pointer |
| `gateway:audit`, `gateway:audit:head` | Hash-chained audit stream and head |
| `gateway:tap:nonce:{scope}`, `gateway:tap:binding:{digest}` | TAP replay protection and credential binding |
| `gateway:events:*` | Durable event outbox, observations and sequence tracking |

Stored continuations are bounded at 2 MiB in at most 16 chunks. The presence
of operation storage does not make the removed public submission routes
available. Retain the original signed transaction and lookup selector while
its outcome is uncertain; resolve chain progress through the receipt APIs.

## Callers the NetworkPolicy admits

The [manifest](../../services/platform/hosted/gateway/deployment.yaml)
configures three gateway replicas, HTTPS container port 9443 and Service
port 443, with a PDB minimum of two available replicas. The HPA spans 3–30
replicas at 65% CPU utilization. These are deployment settings, not measured
service availability.

Gateway ingress admits nginx, testnet control, selected internal payment and
Programs pods, node and registry pods. Egress permits Redis, the Paxeer
boundary, registry, trusted-boundary services, faucet and DNS; a separate
policy admits event delivery to payments and webhooks. Redis ingress admits
gateway pods and the developer dashboard API. Review these policies when
binding additional product upstreams.

## Typed refusals

Ordinary HTTP refusals carry `{"ok":false,"error":{"code":...}}`.
JSON-RPC refusals instead use `error.code`, `error.message` and, where
provided, machine-readable `error.data`. Inspect both transport status and
the response envelope.

| Surface | Refusal | Meaning |
| --- | --- | --- |
| HTTP | `401 api_key_required` | Missing, invalid or revoked durable gateway key |
| HTTP | `401 session_required` | Missing or inactive Bearer session |
| HTTP | `403 insufficient_scope` | Authenticated key lacks route permission |
| HTTP | `403 signer_not_owned` | Session does not authorize the signer requested for key issuance/rotation |
| HTTP | `404 receipt_not_found` | Receipt absent or not owned by this principal |
| HTTP | `409 idempotency_conflict` | Key issuance collides with different retained key material |
| HTTP | `429 quota_exceeded` | Protected read quota exhausted |
| HTTP | `503 persistence_unavailable` | Redis failure; not an authentication refusal |
| HTTP | `503 principal_state_proof_unavailable` | Upstream state head unavailable or not current |
| HTTP | `502 state_proof_unverified` | State proof cannot be verified |
| JSON-RPC | `-32601 Method not found` | Unknown or removed method, including all `lx_*` names |
| JSON-RPC | `-32602 Invalid params` | Method's argument contract not met |
| JSON-RPC | `-32001` | Chain/history/funding read unavailable; inspect message and data |
| JSON-RPC | `-32002 Authentication required` | Product catalogue key authentication failed |
| JSON-RPC | `-32003 Chain read unavailable` | Missing receipt keys, consensus evidence or invalid consensus response |
| JSON-RPC | `-32003 Surface unavailable` | Submission target failed its precompile capability probe |
| JSON-RPC | `-32005 Read unavailable` | Public read budget exhausted |
| JSON-RPC | `-32010 Kernel unavailable` | Required hosted backend absent or unreachable; data names backend and reason |

## Receipt verification before success

`verify_live_receipt` selects a pinned authorization for the authority facts
and its inclusive batch-height range, verifies the receipt and binds the
expected activity ID. Receipt HTTP reads also check the durable principal
owner and match component bytes to the authority evidence before returning
them. A backend status word cannot construct a `VerifiedOperation`.

`LAYERX_GATEWAY_RECEIPT_KEYS_FILE` is a JSON array whose entries contain
`keyId`, `publicKey`, `firstHeight` and `lastHeight` (snake-case aliases are
accepted). The file's key ID and public key are base64-encoded 32-byte values;
`px_getReceiptKeys` publishes them as `0x`-prefixed hex. Empty lists, duplicate
IDs and invalid or reversed ranges are refused. Do not substitute a key from
an untrusted receipt for this operator-pinned authorization.

State reads verify a signed batch header, receipt inclusion, digest, root,
sequence and timestamp. They publish verified head facts, not an account or
cell listing. Settlement first binds the merchant request, digest and
idempotency evidence to its receipt, then requires committed
`pax_sequencerReceipt` evidence and a matching `eth_getTransactionReceipt`
before returning a successful settled result. The name of that HTTP route
does not imply a separate settlement chain.

## Readiness

`GET /livez` only reports process liveness. `GET /readyz` and
`GET /readyz/core` return HTTP 200 when every configured backend is ready,
otherwise 503; an unavailable clock also returns 503. The product variant
includes a route dependency report, while the core variant omits it.
Current readiness computes completeness from backend serving status; it does
not certify successful execution of every product route.

Redis readiness requires `PING` → `PONG`. Chain readiness requires
`eth_chainId` → `0x7d`. Configured consensus readiness requires a valid
`status` response with `catching_up=false`. With no hosted kernel backends,
the report omits their components, retains chain and consensus availability,
and includes configured durable store/event producer dependencies. An
unconfigured backend is not a serving failure, so HTTP 200 alone does not
prove chain serving; inspect `capabilities.chain_serving` and `backends`.

When kernel backends are configured, the report names
`core_agent_boundary`, `public_core`, `independent_receipt_authority`,
`identity` and `program_registry`. Those are implementation backend labels.
`components.principal_state_boundary` remains `unavailable` in this report;
use the verified state route's response to assess an actual state read.

The deployment probes HTTPS `/readyz` every 5 seconds and `/livez` every
15 seconds. `GET /v1/status` is a different summary: `hosted_gateway` is
`degraded` when Redis is ready and `unavailable` otherwise; it is never an
`available` verdict for all gateway capabilities.

## Tests

The current source contains the following focused checks. They describe test
coverage, not a claim that a hosted deployment has passed them.

| Source | Test or coverage |
| --- | --- |
| `src/lib.rs` | `production_route_refuses_every_program_and_activity_route`; `issued_key_debug_redacts_the_credential`; receipt-key loading and authorization checks |
| `src/rpc.rs` | `lx_names_answer_method_not_found` checks refusal with and without configured backends |
| `src/main.rs` | `receipt_keys_list_loads_and_accepts_primary_and_standby_inclusive_ranges`; `settlement_pending_before_commit_and_before_committed_transaction`; `settlement_refuses_tampered_commit_and_execution_fields` |
| `src/state.rs` | `state_reads_publish_only_the_sequencer_signed_head_facts`; `tampered_or_unpinned_head_documents_are_refused` |
| `src/store.rs` | Durable key state, bounded quotas and continuation storage checks |
| `tests/tap_nonce.rs` | Redis-backed principal binding and exact-retry versus changed-request replay protection |

All source paths in this table are relative to
[`services/platform/hosted/gateway`](../../services/platform/hosted/gateway).
Historical lifecycle and hosted-boundary scripts that submit removed routes
are not current public API examples.

## Make targets

[`services/platform/Makefile.inc`](../../services/platform/Makefile.inc)
defines `platform-build` for platform workspaces and SDK/release checks,
`platform-test` for workspace suites through the runtime-clock wrapper,
`platform-hosted-smoke`, `platform-hosted-topology-check` and
`platform-test-agent-install` for their respective integration workflows.
These broader targets have additional prerequisites; they are not gates
for this documentation rewrite.

## Version-2 module registry

When the optional kernel backend group is enabled, startup reads
`LAYERX_GATEWAY_MODULE_REGISTRY_FILE` and validates schema version 2,
1–256 unique nonzero lowercase-hex asset IDs, bounded currency/symbol
metadata and decimals no greater than 38. It validates module IDs and
ordinals, rejects duplicate registrations and includes Programs lifecycle
ordinals 1, 2 and 7 in that module's registration.

This validation does not enable removed HTTP activity or Programs routes.
The old daemon socket/preparation procedure is retired; it is not a
prerequisite for the merged network's public node RPC.

[Home](Home.md)
