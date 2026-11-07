# Hosted faucet

Private-network only.

The retained hosted faucet is for private or disposable networks. It has no
public faucet hostname on Paxeer mainnet-beta and is not a general-use source
of mainnet funds. An approved public allocation is an operator-arranged native
chain transfer; the private service described here is not a prerequisite for
using public accounts.

Paxeer mainnet-beta is one BFT network running `paxd`, with the kernel inside
the network proposing blocks, EVM chain ID `125` (`0x7d`), and finality when a
block is committed. Public JSON-RPC is at the root of
`https://api-mainnet-beta.paxeer.network` and the numbered node hosts
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. The `px_` methods described below
belong to the hosted gateway application, not those public node endpoints.
See [Public JSON-RPC](PublicRpc.md).

`layerx-faucet`, from crate `layerx-platform-faucet`, authenticates claims,
reserves quota and idempotency state in Redis, and asks
[Hosted control](HostedBetaControl.md) to fund the recipient through core.
The implementation is
[`services/platform/hosted/faucet/src/main.rs`](../../services/platform/hosted/faucet/src/main.rs).
The `layerx-testnet` names and response labels identify the retained private
harness. They do not describe another execution or settlement network.

## Deployment

The Deployment, Services, Redis StatefulSet, Secrets references, and
NetworkPolicies live in
[`services/platform/hosted/testnet/deployment.yaml`](../../services/platform/hosted/testnet/deployment.yaml).
The faucet has three replicas, image
`ghcr.io/sidiora-labs/layerx-faucet:0.1.0`, UID/GID `4020`, and listens on
`0.0.0.0:9443`. Service `layerx-faucet-public` is **ClusterIP**, mapping port
443 to container port 9443. “Public” in this Service name does not publish a
mainnet faucet. The source manifest has no faucet Ingress.

The cluster helper can render a faucet Ingress and port-forward the Service;
the default local port is `19445`, exported as `LAYERX_FAUCET_URL`. Such a
private deployment does not establish a public endpoint. The library defaults
in `testnet/src/lib.rs` use private cluster Service origins.

## TLS

Inbound TLS uses rustls with `LAYERX_FAUCET_TLS_CERT_DER` and
`LAYERX_FAUCET_TLS_KEY_DER` (PKCS#8 DER), without client-certificate
authentication. The binary bounds concurrent connections at 128 and bodies at
16 KiB. Its HTTP/1.1 parser rejects query strings, `Transfer-Encoding`, duplicate
headers, and a missing `Host`.

Outbound HTTPS and Redis use the configured DER CA certificate and TLS 1.2 or
later, with a 3-second connect timeout and 8-second I/O timeout. Configured
upstreams require DNS hostnames and the appropriate `https://` or `rediss://`
scheme. HTTPS clients do not present a client certificate. Redis uses TLS
followed by ACL username/password `AUTH`; the manifest sets
`tls-auth-clients no`.

## Tokens

| Credential | Purpose |
| --- | --- |
| Caller session Bearer | Authenticate `POST /v1/faucet/claims` by introspection |
| `LAYERX_IDENTITY_SERVICE_TOKEN_FILE` | Outgoing Bearer to the identity introspection endpoint |
| `LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE` | Optional incoming Bearer for service claims, compared at equal length in constant time |
| `LAYERX_TESTNET_ADMIN_TOKEN_FILE` | Outgoing Bearer to control admin funding |
| `LAYERX_FAUCET_REDIS_USERNAME_FILE` and `LAYERX_FAUCET_REDIS_PASSWORD_FILE` | Redis ACL credentials |

The caller's session token is sent in the introspection JSON body
`{"token":…}`; the outgoing Authorization header uses the identity service
token. The configured path is `/v1/introspect`. Authentication requires a
nonempty `Bearer ` token of at most 4096 bytes, identity HTTP 200, `active: true`,
and a valid `sub` identifier of at most 512 bytes. Non-200 introspection,
inactive sessions, or invalid subjects produce `401 identity_required`;
transport or JSON failures produce `503 identity_unavailable`. See
[Hosted identity](HostedIdentity.md).

Secret files are read at startup, stripped of trailing CR/LF, and rejected if
empty or over 4096 bytes. Request headers and bodies are zeroized on drop.
The gateway service token is not a user session token; service claims trust
the authenticated service to supply the principal.

## Public routes

These routes are on the private faucet listener. Probes require no Bearer.

| Method and path | Inputs | Result |
| --- | --- | --- |
| `GET /livez` | None | `200 {"status":"live","service":"faucet"}` |
| `GET /readyz` | None | Redis `PING` must return `PONG`; `200` ready or `503 dependency_unavailable` |
| `POST /v1/faucet/claims` | Session Bearer, idempotency key, JSON `did` and `public_key` | `200` funded, `202` still checking, or typed refusal |
| `POST /v1/faucet/service-claims` | Service Bearer, idempotency key, JSON `principal`, `did`, `public_key` | Same funding outcomes |

Claims require exact `Content-Type: application/json` and an
`Idempotency-Key` of 1–128 ASCII letters, digits, or characters from `-_.:`.
`did` must begin with `did:` and be an identifier of at most 512 bytes using
the same character set. `public_key` must be 64 ASCII hex digits. Both request
structs reject unknown fields; the service route also explicitly requires a
JSON object and a valid principal of at most 512 bytes. Core requires a DID
matching the key for successful funding, as described below.

Preflight rejects `forwarded`, `x-forwarded-for`, `x-real-ip`,
`x-layerx-client-ip`, and `x-layerx-principal` with
`400 untrusted_identity_header`. It then enforces admission by TCP peer
address before checking the idempotency key or authenticating the claim.
Service claims use this same preflight. Without
`LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE`, the service route returns
`404 not_found`. Unknown methods and paths also return `404 not_found`.

Direct claims use the introspected subject for the identity quota and the
TCP peer for the funding network quota. Service claims use their authenticated
service's supplied principal for the identity quota and `service:<principal>`
for the funding network quota. Admission request counts still use the TCP
peer for both routes. A principal shares its identity quota across both
routes, and the address quota normalizes the public key to lowercase.

For a configured private deployment, a direct claim follows this shape. The
Bearer is read from an existing curl configuration file containing the session
Authorization header; use the private deployment's CA and URL.

```sh
jq -n --arg did "$LAYERX_TEST_SOURCE_DID" \
  --arg public_key "$LAYERX_TEST_SOURCE_PUBLIC_KEY" \
  '{did:$did, public_key:$public_key}' > faucet-request.json
curl --silent --show-error --max-time 30 \
  --cacert "$LAYERX_TEST_CA_FILE" --config "$SESSION_CURL_CONFIG" \
  --request POST "$LAYERX_FAUCET_URL/v1/faucet/claims" \
  --header 'Idempotency-Key: private-faucet-claim-01' \
  --header 'Content-Type: application/json' --data-binary @faucet-request.json
```

Keep the same key and payload when checking an indeterminate claim. HTTP 202
is not funding success. A direct HTTP 200 body contains `funded: true`,
`funding_id`, `transaction_id` (nullable in the faucet's upstream type),
`amount` as a decimal string, and `network: "layerx-testnet"`. Success is not
wrapped in `{"ok":true}`.

## Hosted gateway and starter credits

The current gateway contract is `px_requestFunds`, declared in
[`gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json)
and implemented in
[`gateway/src/rpc_faucet.rs`](../../services/platform/hosted/gateway/src/rpc_faucet.rs).
The old `lx_requestFunds` method is retired. This application method is usable
only on an operator-configured hosted gateway with a faucet backend; it is not
a public node funding API.

Params are exactly `[did, signer_public_key]`. The key is nonzero, lowercase
64-digit Ed25519 hex without `0x` and must be allowed by the caller's session.
The gateway requests the principal's grant from identity
`POST /v1/starter-credits`, derives an idempotency key from the principal and
grant, and posts to the faucet's `/v1/faucet/service-claims`. It settles the
grant through `POST /v1/starter-credits/settle` after validating funded evidence
and the amount. Retries reuse the same grant identity.

Gateway configuration requires `LAYERX_GATEWAY_FAUCET_URL`,
`LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE`, and
`LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT` together when enabled. The amount must be
canonical positive decimal, no greater than `9007199254740991`, and match
`LAYERX_FAUCET_CLAIM_AMOUNT` and the identity grant. Registration credentials
and a working identity backend are also required. The gateway and faucet
mount the same Secret `layerx-gateway-faucet-client` for service claims.

The gateway only returns a funded result when it has a nonempty funding ID,
transaction ID, amount, and network. It adds `starter_credit` to the result.
This is stricter than the direct faucet HTTP body's nullable transaction ID.
The `layerx faucet` CLI request builder, SDK `RpcClient::request_funds`, and
MCP `faucet.request` use the `px_requestFunds` method; their configured
transport must address the hosted application. See [CLI](Cli.md) and
[Hosted gateway](HostedGateway.md).

| JSON-RPC code | Machine-readable reason or condition |
| --- | --- |
| `-32602` | Invalid params; upstream claim HTTP 400/409 |
| `-32002` | `session_required`, `signer_not_bound`; upstream claim HTTP 401/403/404 |
| `-32001` | `faucet_not_configured`, `registration_not_configured`, `identity_unavailable`, or `faucet_unavailable` |
| `-32001` | `faucet_claim_pending`, with pending state and upstream document for HTTP 202 |
| `-32005` | `faucet_rate_limit` or upstream claim HTTP 429 |
| `-32603` | Invalid upstream response or incomplete funded evidence |
| `-32010` | `kernel_unavailable` when gateway identity backend admission fails |

An unrepresentable grant returns `-32001`, message `Starter credit unavailable`,
with `error.data.code: "starter_credit_amount_unrepresentable"`, before a
service claim or per-minute slot is consumed. A grant that differs from the
configured claim amount, or a funded amount that differs from the grant,
returns `-32603`, message `Starter credit amount mismatch`, with
`error.data.code: "starter_credit_amount_mismatch"`. The gateway does not
truncate or resize grants. It limits service-claim attempts to 30 per minute
per gateway process; Redis supplies the faucet's shared quota state.

Identity grant refusals preserve the identity error document in `error.data`:
HTTP 403/409 maps to `-32002`, 429 to `-32005`, 400/404 to `-32602`, and other
failures to `-32001`. The faucet HTTP errors below have a different envelope
from these JSON-RPC errors.

## Claim reservation and funding

`RESERVE_SCRIPT` makes quota reservation and idempotency lookup atomic in
Redis. The request digest is SHA-256 over identity, DID, public key, and amount,
with a zero byte after each part. The funding ID is SHA-256 over idempotency
key and request digest using the same part encoding. The idempotency key is
shared across the faucet replicas, not scoped to a single process.

| Reservation outcome | Behavior |
| --- | --- |
| Fresh or pending, same digest | Call funding with the same `funding_id` |
| Funded, same digest | Return the stored HTTP 200 body |
| Same idempotency key, different digest | `409 idempotency_conflict` |
| Identity, address, or network amount cap exceeded | `429 identity_quota`, `address_quota`, or `network_quota` |

The faucet posts `funding_id`, `did`, `public_key`, and numeric `amount` to
`LAYERX_TESTNET_FUNDING_URL` using the control-admin Bearer and the funding ID
as the upstream idempotency key. Set this URL to an operator-supplied private
endpoint, for example
`https://OPERATOR-CONTROL-ADMIN-ORIGIN/admin/v1/testnet/fund`.
Control validates and proxies the command to core-admin.

New core funding resolves the signer key's bound chain address and sends a
native EIP-1559 value transfer on chain `125`. It journals the transfer for
retries and verifies receipt binding. The private DID must be `did:layerx:`
plus the lowercase public key, and cannot be the treasury DID; a valid faucet
request can still fail these deeper checks. See
[control's native treasury funding path](HostedBetaControl.md#native-treasury-funding-path)
and [Hosted core](HostedCore.md).

Upstream HTTP 200 with matching `funding_id` and `state: "funded"` is stored
before returning the funded body. An upstream 4xx triggers quota rollback and
`503 funding_rejected` when rollback succeeds. Other statuses, malformed
responses, transport failures, completion failures, or rollback failures
return HTTP 202 with `state: "still_checking"`, `retry: "after"`,
`retry_after_seconds: 10`, and `Retry-After: 10`. A pending response keeps the
reservation. The default amount `1000000` is a count of native base units on
the new transfer path, not a promise of that many whole tokens.

## Config keys

Defaults come from the faucet's `config()`. Mounted paths are from the shared
testnet deployment manifest.

| Key | Role or default |
| --- | --- |
| `LAYERX_FAUCET_LISTEN` | `0.0.0.0:9443` |
| `LAYERX_FAUCET_TLS_CERT_DER` | `/run/layerx/tls/server.crt.der` |
| `LAYERX_FAUCET_TLS_KEY_DER` | `/run/layerx/tls/server.key.der` |
| `LAYERX_OUTBOUND_CA_DER` | `/run/layerx/tls/ca.crt.der` |
| `LAYERX_IDENTITY_INTROSPECTION_URL` | HTTPS URL including `/v1/introspect` |
| `LAYERX_IDENTITY_SERVICE_TOKEN_FILE` | `/run/layerx/identity/token` |
| `LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE` | Optional; `/run/layerx/gateway-faucet/token` in the manifest |
| `LAYERX_TESTNET_FUNDING_URL` | HTTPS control-admin funding URL including path |
| `LAYERX_TESTNET_ADMIN_TOKEN_FILE` | `/run/layerx/control-admin/token` |
| `LAYERX_FAUCET_REDIS_URL` | Operator-supplied, for example `rediss://OPERATOR-FAUCET-REDIS:6379` |
| `LAYERX_FAUCET_REDIS_USERNAME_FILE` | `/run/layerx/redis-auth/username` |
| `LAYERX_FAUCET_REDIS_PASSWORD_FILE` | `/run/layerx/redis-auth/password` |
| `LAYERX_FAUCET_IDENTITY_LIMIT` | `10000000` amount units per window |
| `LAYERX_FAUCET_ADDRESS_LIMIT` | `10000000` amount units per window |
| `LAYERX_FAUCET_NETWORK_LIMIT` | `50000000` amount units per window |
| `LAYERX_FAUCET_NETWORK_REQUEST_LIMIT` | `60` admission requests per peer/window |
| `LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS` | `60` |
| `LAYERX_FAUCET_WINDOW_SECONDS` | `86400` |
| `LAYERX_FAUCET_IDEMPOTENCY_SECONDS` | `604800` |
| `LAYERX_FAUCET_CLAIM_AMOUNT` | `1000000` |

The claim amount must be positive and fit every quota limit; each limit must
be no greater than `9007199254740991`, the exact Redis Lua integer bound used
by the implementation. The quota window must be positive and idempotency
retention must cover it. The manifest leaves claim amount and quota window
at their code defaults.

## Redis state and ACL

StatefulSet `layerx-faucet-redis` runs Redis `8.2.1-alpine`, with TLS port 6379,
a 20 GiB PVC, `appendonly yes`, `appendfsync always`,
`maxmemory-policy noeviction`, and `protected-mode yes`. Plaintext Redis is
disabled. ACL rules are mounted at `/run/layerx/auth/users.acl`. The cluster
helper disables the default user and provisions the `layerx-faucet` ACL user.
All faucet replicas share this store.

| Key | Contents |
| --- | --- |
| `faucet:admission:{window}:{digest}` | TCP-peer request count with admission-window expiry |
| `faucet:quota:{window}:identity:{digest}` | Identity amount counter with remaining-window expiry |
| `faucet:quota:{window}:address:{digest}` | Counter for the lowercase signer public key |
| `faucet:quota:{window}:network:{digest}` | Counter for TCP peer or service-principal scope |
| `faucet:idem:{digest}` | Request digest, state, funding ID, quota keys, amount, and funded response |
| `faucet:audit`, `faucet:audit:head` | Audit stream and chained reservation head |

Reservation events carry `event`, `result`, optional `funding_id`, and
`chain`; completion and rejection events carry their result and funding ID.
Session and service credentials are not audit fields. Idempotency hashes
expire according to the configured retention; the durable core journal also
protects retries of prepared transfers. Preserve the Redis PVC and core state
when restarting private services.

## Callers the NetworkPolicy admits

The source manifest permits faucet TCP 9443 ingress from IP blocks and control
and gateway pods. The ClusterIP Service itself does not assign a public
address. Egress permits trusted-boundary pods on 9443, control on 9444, Redis
on 6379, and DNS on 53. Redis admits only faucet and control pods on 6379.

Deployment readiness probes run every 5 seconds and liveness probes every
15 seconds. Faucet readiness proves Redis `PING`, not identity or funding
availability. Control's [funding journey](HostedBetaControl.md#journeys-and-probes)
checks the wider dependency set.

## Typed refusals

HTTP refusals use `{"error":{"code":…,"retry":…}}`. When a delay is known,
`retry` is `after`, `error.retry_after_seconds` carries the delay, and the HTTP
`Retry-After` header is set. Otherwise `retry` is `never`. HTTP 200 and 202
bodies use their own shapes.

| HTTP | Code | Condition |
| --- | --- | --- |
| 400 | `invalid_request` | Invalid HTTP framing |
| 400 | `content_type_required` | Claim content type is not exactly `application/json` |
| 400 | `untrusted_identity_header` | Forwarded identity or IP header supplied |
| 400 | `idempotency_key_required` | Missing claim key |
| 400 | `invalid_idempotency_key` | Malformed claim key |
| 400 | `invalid_argument` | Invalid JSON or claim fields |
| 401 | `service_token_required` | Missing or mismatched service Bearer |
| 401 | `identity_required` | Missing or rejected user identity |
| 404 | `not_found` | Unknown route, or service claims disabled |
| 409 | `idempotency_conflict` | Key already used with another digest |
| 429 | `network_request_rate` | Peer admission request cap exceeded |
| 429 | `identity_quota` | Identity amount cap exceeded |
| 429 | `address_quota` | Destination signer amount cap exceeded |
| 429 | `network_quota` | Peer/service scope amount cap exceeded |
| 503 | `dependency_unavailable` | Readiness Redis check failed |
| 503 | `persistence_unavailable` | Admission or reservation Redis operation failed |
| 503 | `identity_unavailable` | Introspection transport, encoding, or JSON failed |
| 503 | `funding_rejected` | Funding upstream returned 4xx and rollback succeeded |

Quota refusals advise retry at the next quota window. Request admission
refusals use the configured admission-window delay. Dependency, persistence,
identity, and funding-rejected responses use a 10-second delay. A 202
`still_checking` response is an indeterminate outcome, not a typed refusal or
a completed allocation.

## Tests

Existing faucet tests cover service-token admission, strict service claim
fields, principal-scoped quotas, public-key case normalization, quota numeric
bounds, and typed retry advice. The retained `testnet/tests/faucet-quota.py`
corpus exercises the production Lua scripts against Redis. Gateway tests in
`rpc_faucet.rs` cover params, method dispatch, grant validation, amount
representability, and funding evidence.

[`hosted-smoke.sh`](../../services/platform/hosted/testnet/tests/hosted-smoke.sh)
checks readiness, admits the funding journey, and claims funds for configured
private identities before its payment journey. `platform-test-tooling` in
[`services/platform/Makefile.inc`](../../services/platform/Makefile.inc) runs
the faucet crate tests through the runtime-clock wrapper. These test entry
points do not demonstrate public faucet availability.

[Hosted control](HostedBetaControl.md) · [Home](Home.md)
