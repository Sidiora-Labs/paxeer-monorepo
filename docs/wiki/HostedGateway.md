# Hosted gateway

`layerx-gateway` is the hosted HTTP and receipt-verification boundary for
Paxeer X Network. The crate is `layerx-platform-gateway`, with its binary in
[`src/main.rs`](../../services/platform/hosted/gateway/src/main.rs).
The LayerX kernel inside `paxd` executes and orders each transaction and builds
each block; validators check and commit it. A receipt is final once validators
commit its block. Hosted identity, indexing and product APIs do
not form another execution or settlement network. Sub-chains are tenant
domains in the database, launchable without new infrastructure.

## Public nodes and hosted gateway

Use a root node URL from [Public JSON-RPC](PublicRpc.md) for chain access.
EVM chain ID is `125` (`0x7d`). The node's registered methods are defined in
[`rpc/server.go`](../../rpc/server.go) and filtered by node configuration.
Native `px_*` reads are served by the node; hosted identity and faucet
operations use `px_register` and `px_requestFunds` on the configured gateway.

The current gateway dispatcher serves JSON-RPC at `POST /` with
`Content-Type: application/json`. It accepts one request or a batch of 1–32
requests. JSON-RPC replies and errors use HTTP 200; notifications produce no
reply, and an all-notification request returns HTTP 204. Invalid JSON is
`-32700`; an invalid request or batch is `-32600`. All `lx_*` names now answer
`-32601 Method not found`. `GET /openrpc.json` serves the gateway schema.

Set the shell variable `PAXEER_RPC_URL` to the selected node root URL to identify
the network:

```sh
curl "$PAXEER_RPC_URL" \
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
current request contract. See [Public JSON-RPC](PublicRpc.md) and
[Public payment API](PublicAPI.md) for related chain access and payment material.

## Hosted JSON-RPC methods

The served schema and `ROUTER_METHODS` in
[`src/lib.rs`](../../services/platform/hosted/gateway/src/lib.rs) declare
18 method names: the following 17 `px_*` methods and `pax_sequencerReceipt`.
This count excludes the EVM methods relayed to a configured node. The checked-in
OpenRPC file also describes five `px_getInference*` node reads; the gateway
schema omits those methods and its dispatcher returns method-not-found for them.
Parameters are positional; use `[]` explicitly for methods without arguments.

| Method | Parameters | Result or dependency |
| --- | --- | --- |
| `px_openSession` | `[signer_public_key, proof_time, session_signature]` | Developer session from a fresh Ed25519 proof over the configured tenant, signer key and canonical decimal Unix time; returns tenant, sub, token, csrf_token and expires_at |
| `px_register` | `[signer_public_key, registration_signature]` | Self-service identity principal after Ed25519 proof of key possession; both arguments are lowercase hex, 32 and 64 bytes respectively |
| `px_requestFunds` | `[did, signer_public_key]` | Authenticated starter-credit claim through identity and faucet; requires a Bearer session that authorizes the signer |
| `px_resolveAccount` | `[account]` | EVM address or `did:layerx:<64 hex>` resolution through the address precompile; unbound fields are null |
| `px_getAccount` | `[account]` | Resolution and EVM balance, nonce and height from one chain snapshot |
| `px_getBalances` | `[account]` | Bank balances at one height; amounts are exact decimal strings in base units |
| `px_listAssets` | `[cursor?, limit?]` | Registered assets in registration order, with `next_cursor` and chain metadata at the returned height; unavailable metadata remains null |
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

`px_openSession` signs the length-prefixed SHA-256 digest of
`layerx-session-open-v1`, the configured tenant, the 32-byte signer key and
`proof_time` in decimal seconds. The proof time may be exactly 120 seconds
before or after the gateway clock. The gateway first asks identity to confirm
the principal and then asks it to mint a session without choosing a lifetime;
identity supplies the lifetime and the five returned fields. Each signer/time
pair is accepted once while it remains in the bounded replay set.

Asset-list cursors are canonical decimal strings; omit the cursor or use null
for the first page. The optional integer limit is 1–16, default 16, and the
response reports `order: "registration"`.

History cursors are opaque indexer cursors. Holder-record cursors are 32
lowercase hex characters; holder page size defaults to 50 and is bounded at
100. Use the OpenRPC schemas for optional argument types and result fields.
A native account ID is not a DID public key: retain the original EVM address
or explicit DID selector for repeated resolution, balance and history reads.

The gateway relays permitted `eth_*`, `net_*` and `web3_*` methods through its
configured node pool. Node-side signing methods are refused; ordinary HTTP
relay refuses `eth_subscribe` and `eth_unsubscribe`; the root WebSocket listener
relays subscriptions. Submit signed
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
| `GET /openrpc.json` | No credential; served JSON-RPC schema |
| `GET /` with WebSocket upgrade | EVM subscription relay through the configured node pool |
| `GET /livez` | No credential; `status=live`, `service=layerx-gateway`, package version |
| `GET /readyz` | No credential; backend readiness and product dependency report |
| `GET /readyz/core` | No credential; backend readiness with product route report omitted |
| `GET /v1/status` | No credential; service availability summary |
| `GET /metrics` | Event producer failure and overflow counters |
| `GET /v1/chains` | Public indexer read; list chains |
| `GET /v1/chains/:id` | Public indexer read; chain registry record |
| `GET /v1/chains/:id/summary` | Public indexer read; chain summary |
| `GET /v1/chains/:id/transfers` | Public indexer read; paged chain transfers |
| `GET /v1/chains/:id/events` | Public indexer read; paged chain events |
| `POST /v1/keys` | Bearer session, `Idempotency-Key`; JSON signer, scopes and quota settings |
| `GET /v1/keys` | Bearer session; principal's key metadata |
| `DELETE /v1/keys/{id}` | Bearer session; revoke an owned key |
| `POST /v1/keys/{id}/rotate` | Bearer session and `Idempotency-Key`; replace an owned key |
| `GET /internal/v1/principal` | `LayerX-Key`; durable principal digest |
| `GET /v1/routes` | `LayerX-Key`; catalogue plus dependency readiness. Top-level keys: `schema`, `canonical_origin`, `chain_id`, `rpc_records`, `preserve_api1_original_host`, `wallet_authentication`, `wallet_placement`, `services`, `routes`, `denied_surfaces`, `node_signing_methods`, `rpc_paths`, `node_rpc`, `hosted_gateway` and `readiness` |
| `GET /v1/routes/{service}` | `LayerX-Key`; service routes and health; private services are refused |
| `GET /v1/receipts/{id}` | `LayerX-Key`, `receipt:read`, configured component; owner-scoped receipt verification |
| `GET /v1/state` | `LayerX-Key`, `state:read`, configured component; verified current head evidence |
| `POST /v1/settle` | `LayerX-Key`, `receipt:read`, configured component; JSON merchant evidence checked against chain commitment |

The five chain reads use `LAYERX_GATEWAY_INDEXER_URL` through the same client
as the history methods and require no credential. They share the public-read
budget of 120 reads per second overall and 30 per client; exhaustion returns
`429 public_read_rate_limit` with a one-second retry interval. Caller headers
are not forwarded to the indexer.

`:id` must contain only decimal ASCII digits and fit an unsigned 32-bit number;
invalid selectors return `400 invalid_read_selector` before any indexer request.
Transfers and events pass their query string unchanged, bounded at 512 bytes
using letters, digits, `-._~=&` and complete `%XX` escapes for cursor, limit and
kind parameters. The indexer validates parameter names, duplicates and values.
The list, record and summary routes accept only an empty query. Queries outside
these boundaries return `400 invalid_read_selector`; other methods return
`405 method_not_allowed`, subject to the gateway's existing CORS preflight rule.
The chain-by-name route is not served.

Indexer JSON documents and statuses pass through unchanged, including selector
refusals and `404` for unknown chains. Non-JSON replies return
`502 invalid_indexer_response`. An absent indexer returns
`503 indexer_not_configured`; an unreachable indexer returns
`503 indexer_unreachable`. Gateway refusals use its JSON error envelope.
The route bindings' allowed origins govern CORS headers and preflight replies;
an unlisted origin returns `403 origin_not_allowed`.

The explorer proxy admits these public backend routes:

- `GET /explorer/backend/api/v2/advanced-filters`
- `GET /explorer/backend/api/v2/advanced-filters/csv`
- `GET /explorer/backend/api/v2/advanced-filters/methods`
- `GET /explorer/backend/api/v2/paxeer-x/feed/markets`
- `GET /explorer/backend/api/v2/paxeer-x/feed/markets/{market_id}/updates`
- `GET /explorer/backend/api/v2/paxeer-x/feed/updates`
- `GET /explorer/backend/api/v2/paxeer-x/feed/updates/{hash}`

Its feed pages are available for `GET` and `HEAD`: `/explorer/feed`,
`/explorer/feed/market/[id]` and `/explorer/feed/update/[hash]`.

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
| `LAYERX_GATEWAY_INDEXER_URL` | Optional history, asset and public chain-read indexer origin |
| `LAYERX_GATEWAY_RECEIPT_KEYS_FILE` | Optional pinned receipt-key list; required for receipt verification and publication |
| `LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS` | Probe cache lifetime, 1–300 seconds; default 15 |
| `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | Optional product upstream bindings; catalogue presence alone is insufficient |
| `LAYERX_GATEWAY_TRUSTED_PROXIES` | Trusted proxy CIDRs for client-address attribution |
| `LAYERX_GATEWAY_IDENTITY_URL`, `LAYERX_GATEWAY_IDENTITY_TOKEN_FILE` | Identity service HTTPS address and outbound token file |
| `LAYERX_GATEWAY_TENANT` | Trusted registration/session tenant, 1–95 lowercase letters, digits, dots, underscores or hyphens |
| `LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12`, `LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE` | Paired outbound client identity bundle and password file |
| `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | Self-service registration authorization |
| `LAYERX_GATEWAY_FAUCET_URL`, `LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE`, `LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT` | Faucet integration and expected starter-credit amount |
| `LAYERX_GATEWAY_IDEMPOTENCY_SECONDS` | Retention 3,600–2,592,000 seconds; default 604,800 |

Optional kernel backend integration is selected by
`LAYERX_GATEWAY_COMPONENT_URL`. Its required group comprises component,
authority and program-registry URLs and their respective `*_TOKEN_FILE`
variables, `LAYERX_GATEWAY_PUBLIC_CORE_URL`, a client identity and
`LAYERX_GATEWAY_MODULE_REGISTRY_FILE`. Identity configuration can also be used
independently for registration and sessions; the optional
`LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` requires the identity URL.
These are hosted backend configuration names retained in the implementation.

Outbound service mTLS may alternatively use the paired
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PKCS12` and
`LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_PASSWORD_FILE` variables. The two
client-identity pairs cannot be configured together.

### Native deployment inputs

The [platform gateway definition](../../deploy/projects/platform/gateway.kvx)
sets `LAYERX_FILES_TABLE=/etc/layerx/native-files.tsv` and `LAYERX_ROLE=identity`.
It sets `LAYERX_GATEWAY_INDEXER_URL` to the private HTTPS origin of the project
indexer, using the indexer's listener port. `LAYERX_GATEWAY_CONSENSUS_RPC_URL`
comes from the operator input
`PLATFORM_GATEWAY_LAYERX_GATEWAY_CONSENSUS_RPC_URL`, an absolute HTTPS address
with a DNS host that may hold a path; consensus calls use exactly that path.
It reads `LAYERX_GATEWAY_IDENTITY_URL` from the operator input
`PLATFORM_GATEWAY_LAYERX_GATEWAY_IDENTITY_URL`, an absolute HTTPS identity
service address, and `LAYERX_GATEWAY_TENANT` from the operator input
`PLATFORM_GATEWAY_LAYERX_GATEWAY_TENANT` with the tenant format above.

The [native file table](../../docker/platform-gateway/native-files.tsv) selects
six additional rows for this role, each with mode `0600`. The project reads
each file input from a launch input named `PLATFORM_GATEWAY_` followed by the
file input name. The image stages the files for the gateway's runtime user.

| File input | Destination | Gateway variable | Input category and format |
| --- | --- | --- | --- |
| `ENDPOINT_ROUTE_BINDINGS` | `/run/secrets/route-bindings.json` | `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | `operator`: product route bindings JSON document encoded as padded base64 |
| `ENDPOINT_IDENTITY_TOKEN` | `/run/secrets/identity-token` | `LAYERX_GATEWAY_IDENTITY_TOKEN_FILE` | `operator`: base64 of token text issued by the identity service |
| `ENDPOINT_IDENTITY_PROVISIONING_TOKEN` | `/run/secrets/identity-provisioning-token` | `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | `operator`: base64 of provisioning token text issued by identity |
| `ENDPOINT_KEY_PROVISIONING_KEY` | `/run/secrets/key-provisioning-key` | `LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` | `generated`: base64 of 64 hexadecimal characters, generated once and equal on every replica |
| `ENDPOINT_CLIENT_P12` | `/run/secrets/client.p12` | `LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12` | `generated`: base64 PKCS#12 bundle written by the launcher |
| `ENDPOINT_CLIENT_PASSWORD` | `/run/secrets/client-password` | `LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE` | `generated`: base64 client password written by the launcher |

The key issuance and rotation routes require the key provisioning key. The
platform project definition holds it as the generated input
`PLATFORM_GATEWAY_ENDPOINT_KEY_PROVISIONING_KEY`: base64 of 64 hexadecimal
characters, generated once and equal on every replica.

The role selects ten rows including the four `*` rows for outbound CA,
Redis username, Redis password and receipt keys. A native start without the
role and identity settings selects only those four chain files and serves
the chain methods; `px_register` and `px_openSession` answer
`registration_not_configured`.

Supply `PLATFORM_GATEWAY_ENDPOINT_ROUTE_BINDINGS` as the padded base64 encoding
of the complete UTF-8 JSON document. The definition passes it through
`ENDPOINT_ROUTE_BINDINGS`; the native table writes it to
`/run/secrets/route-bindings.json` with mode `0600` for the `identity` role.
The entrypoint reads this protected document, stages any referenced
`health_authorization_file` and rewrites those paths before dropping privileges.
Those authorization files must already exist as protected files; the bindings
input does not create their contents.

The document has this shape; replace `NETWORK` and `WIRE` with the gateway's
`LAYERX_GATEWAY_NETWORK_ID` and decimal-string `LAYERX_GATEWAY_LXP_WIRE_VERSION`:

```json
{
  "schema": "paxeer-x.route-bindings.v1",
  "chain_id": 125,
  "network_id": "NETWORK",
  "wire_version": "WIRE",
  "allowed_origins": [],
  "services": {},
  "upstreams": {},
  "passthrough": [],
  "mcp": []
}
```

`upstreams` and `mcp` are optional and default to empty collections. Other
top-level members are required; unknown fields are refused. The document is
bounded at 1 MiB, with at most 31 service bindings, 64 named upstreams and 32
MCP principal bindings. `allowed_origins` holds distinct HTTPS origins without
wildcards or base paths for browser CORS.

Each `services` key is a service ID from the
[route catalogue](../../tools/paxeer-x/route-catalogue.json). Its binding holds:

| Member | Required value or meaning |
| --- | --- |
| `url` | HTTPS origin with a DNS host, optional port, and no base path, credentials, query or fragment |
| `discovery_url` | Optional HTTPS endpoint advertised after a successful product service probe; private services cannot advertise one |
| `health_path` | Absolute path probed with GET for readiness |
| `identity_path` | Absolute path probed with GET for network and version identity; may equal `health_path` |
| `health_authorization_file` | Optional protected file containing the complete Authorization header value, such as `Bearer ...`, for both probes |
| `ready_pointer`, `ready_value` | JSON pointer into the health response and the exact non-null JSON value required there |
| `network_pointer`, `network_value` | JSON pointer into the identity response and the exact JSON value, either integer `125` or the gateway network label string |
| `version_pointer`, `version_value` | JSON pointer into the identity response and the exact nonempty version string of at most 64 bytes |

Pointers start with `/` and are at most 128 bytes. Probe paths are at most
2048 bytes, use graphic ASCII, and contain no query, fragment, backslash,
percent encoding or `.`/`..` path segment. Both probes require HTTP 200 and a
JSON content type; every configured predicate must match. For `agentd`,
`network_value` must be the gateway network label and `version_value` must
equal its wire version. No default health path or predicate is inferred for
any service.

For `dashboard` and `webhooks`, use the service's HTTPS origin with both
`health_path` and `identity_path` set to `/healthz`. Configure the following
predicates for each binding:

| Service | `ready_pointer` / `ready_value` | `network_pointer` / `network_value` | `version_pointer` / `version_value` |
| --- | --- | --- | --- |
| `dashboard` | `/ready` / `true` | `/network_id` / the gateway network label string, also set as `LAYERX_DASHBOARD_NETWORK_ID` | `/version` / the deployed `layerx-platform-dashboard` package version string |
| `webhooks` | `/ready` / `true` | `/network_id` / the gateway network label string, also set as `LAYERX_WEBHOOKS_NETWORK_ID` | `/version` / the deployed `layerx-platform-webhooks` package version string |

The network settings accept 1–64 printable ASCII bytes without spaces.
Each health response always includes the package version; the network member
is present when configured. Set the network label on every bound service.
Both webhooks roles expose these health members; ingress application TLS
requires a trusted internal client certificate. `/livez` carries no readiness,
network or version predicate.

For example, if an upstream actually returns
`{"ready":true,"network_id":"NETWORK","wire_version":"WIRE"}` at `/readyz`,
its binding can be:

```json
{
  "url": "https://service.example.net:9443",
  "health_path": "/readyz",
  "identity_path": "/readyz",
  "ready_pointer": "/ready",
  "ready_value": true,
  "network_pointer": "/network_id",
  "network_value": "NETWORK",
  "version_pointer": "/wire_version",
  "version_value": "WIRE"
}
```

Bindings use HTTPS even for services whose catalogue transports are HTTP,
WebSocket, JSON-RPC, authenticated discovery, MCP or A2A. The catalogue chooses
the transport; a binding has no `transport` member. HTTPS uses the gateway's
`native_tls` client with TLS 1.2 or later, normal certificate and hostname
verification, system trust roots plus the DER certificate from
`LAYERX_GATEWAY_OUTBOUND_CA_DER`, and its configured PKCS#12 client identity
when present. A binding cannot select its own CA or disable verification.

When a proxy route's `upstream` differs from its service ID, put
`{"service":"SERVICE_ID","binding":{...}}` under that name in `upstreams`.
In particular, `webhooks-dashboard` needs both `dashboard` and `webhooks`;
`explorer` needs `services.explorer` and `upstreams.explorer-backend`.
Only named proxy upstreams admitted by the catalogue are accepted.

TLS passthrough is available only for `agentd` and `program-registry` through
`passthrough` entries with `service`, a unique nonzero socket address `listen`,
and HTTPS `upstream` matching the service binding's host and port without a
base path. It forwards the TLS stream without terminating it. MCP additionally
requires per-principal `mcp` entries containing `principal`, `identity_socket`,
`identity_tenant`, `identity_uid`, `identity_gid`, `owner_binding_file`,
`owner_binding_sha256`, `owner_uid`, `owner_gid` and `mcp_version`; readiness
checks the pinned owner document and socket identity. `human-web` readiness
instead uses `human_web_origin` in the separate protected
`LAYERX_GATEWAY_UI_BINDINGS_FILE` document, with schema
`paxeer-x.ui-bindings.v2` and required `wallet_origin`. Its probe requires a
nonempty HTTP 200 HTML response at `/explorer/verify`.

The empty collections in the first example bind no service. With no separate
Human web configuration, every `product_routes.services` entry reports
`configured=false`, `ready=false`, `reason=not_configured`, while
`product_routes.complete=true`: unconfigured services do not block that
aggregate. This does not enable product routes or prove that their dependencies
are deployed. Overall `/readyz` still requires chain serving and every
configured backend to be ready.

`capabilities::chain_rpc` sends upstream node calls to the root path: a
bare node origin gets the base path `/`.

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
| `gateway:principal:{digest}:audit` | Gateway-written principal audit stream with the common stream's IDs and fields; the dashboard reads absent entries and an absent version as empty history |
| `gateway:principal:{digest}:audit:version` | Gateway-written index version `1`; other versions and entries without a version are refused by the dashboard |
| `gateway:audit:principal-index:cursor` | Durable position for background recovery in pages of at most 256 common audit entries |
| `gateway:tap:nonce:{scope}`, `gateway:tap:binding:{digest}` | TAP replay protection and credential binding |
| `gateway:events:*` | Durable event outbox, observations and sequence tracking |

The gateway writes each principal audit entry and version in the same script
as the common audit entry. At startup it recovers principal indexes in the
background from the stored cursor without delaying serving. Recovery reports
its stage on failure and leaves serving available.

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
| JSON-RPC | `-32002 Session refused` | `px_openSession` proof is outside the time span, has an invalid signature or was replayed; inspect `error.data.code` |
| JSON-RPC | `-32005 Session unavailable` | `px_openSession` replay set is full; inspect `error.data.code` |
| JSON-RPC | `-32603 Invalid upstream session response` | Identity did not return a complete session answer for the proven principal |
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
`status` response with `catching_up=false`. A configured indexer must report
fresh, reconciled source observations without failures, with indexed cursors at or
below their source heads by at most one ingestion step; a consensus source whose
node declares its transaction index off reports `index_disabled` without lowering
readiness. An absent indexer reports `not_configured` and does not
prevent readiness. With no hosted kernel backends,
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
These broader targets require their declared integration dependencies.

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
