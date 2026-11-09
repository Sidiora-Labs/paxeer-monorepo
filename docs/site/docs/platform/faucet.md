# Hosted faucet

Private-network only.

The hosted faucet is for private or disposable networks only. It has no public
faucet hostname on Paxeer X Network and is not a mainnet-beta allocation service.
The private claim routes and deployment templates below do not establish a
public funding service.

## Public funding and network identity

Paxeer X is one BFT network, with EVM chain ID `125` (`0x7d`). The kernel is
part of the network and proposes blocks; a receipt is final once its block is
committed. Fund public accounts through transfers on that network and read the
resulting account balances directly. A private faucet grant is not public
custody credit or evidence of a mainnet balance.

Use the [public node endpoints](../reference/public-rpc.md) at their HTTPS root:
`https://api-mainnet-beta.paxeer.network` or
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. Check `eth_chainId` before signing;
use `eth_getBalance` for an EVM address's native balance.

The node serves the `px` namespace at its root and answers from node state.
`px_getBalances` and `px_getCapabilities` are node reads and do not require a
hosted gateway.
Only `px_register` and `px_requestFunds` are hosted operations.
After parameter validation, node calls to either operation return JSON-RPC
`-32004` with `error.data.code: "hosted_gateway_service_required"`.

On a configured gateway, `px_getBalances` reads chain bank balances
and returns `account`, `height`, and `balances` entries with `denom` and exact
decimal-string `amount`; an unbound DID is refused rather than assigned a
balance (`services/platform/hosted/gateway/src/paxeer.rs`).

The former router funding flow and `lx_requestFunds` method are retired. The
current [gateway OpenRPC contract](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/services/platform/hosted/gateway/openrpc.json)
defines `px_requestFunds` for an authenticated starter-credit claim when its
upstreams are configured. Its presence in the contract does not make this
private faucet available on public nodes. See
[Hosted gateway JSON-RPC](gateway-rpc.md) for gateway context.

## Private deployment

The crate is `layerx-platform-faucet`, and its binary is `layerx-faucet`
(`services/platform/hosted/faucet/Cargo.toml`). The implementation is
`services/platform/hosted/faucet/src/main.rs`. It authenticates claims, reserves
quotas and idempotency state in Redis, and asks testnet-control to fund them.
Session minting is described in [Hosted identity](identity.md), journey
admission in [Hosted testnet control](../operators/beta-control.md), and treasury
funding in [Hosted core](hosted-core.md).

`services/platform/hosted/testnet/deployment.yaml` contains the faucet
Deployment, Service, Redis StatefulSet and network policies. The faucet image
is `ghcr.io/sidiora-labs/layerx-faucet:0.1.0`; its entrypoint is
`/usr/local/bin/layerx-faucet` and its user/group is `4020:4020`
(`docker/platform-faucet/Dockerfile`). The Deployment has three replicas and
listens on `0.0.0.0:9443`. Service `layerx-faucet-public` is **ClusterIP**, with
port `443` targeting container port `9443`. The word `public` in this internal
Service name does not mean a public network faucet exists.

The source manifest has no faucet Ingress. The private-cluster renderer in
`services/platform/hosted/tests/beta-cluster.sh` appends one with a `/` backend
path, host selected by `LAYERX_BETA_FAUCET_HOST`, and Service
`layerx-faucet-public`. Its historical public-host default is not a supported
public faucet endpoint; configure a private hostname before using that
renderer. Cluster bring-up port-forwards the Service on local port `19445`
by default and exports `LAYERX_FAUCET_URL` for private smoke clients.

## TLS

Inbound HTTPS uses rustls without client-certificate authentication. Set
`LAYERX_FAUCET_TLS_CERT_DER` to a server certificate DER file and
`LAYERX_FAUCET_TLS_KEY_DER` to a PKCS#8 private-key DER file. Application
credentials still apply to claim routes.

The server accepts HTTP/1.1 and caps the entire request, including headers and
body, at 16 KiB. It rejects query strings, duplicate headers,
`Transfer-Encoding`, and a missing `Host`. At most 128 connections are live;
further accepted connections are dropped without an HTTP response. Socket
read/write timeouts are 8 seconds.

Outbound identity and funding endpoints must use `https://`; Redis must use
`rediss://`. Endpoint hosts must be DNS names, not literal IP addresses. Both
clients use `native_tls`, add the CA from `LAYERX_OUTBOUND_CA_DER`, and require
at least TLS 1.2. Connections have a 3-second timeout per attempted address
and 8-second I/O timeouts. Outbound HTTPS does not present a client
certificate. Redis authenticates with its configured ACL username and
password; the supplied Redis configuration sets `tls-auth-clients no`.

## Tokens

The caller's session Bearer is sent to identity in the introspection JSON
body, never forwarded as an upstream `Authorization` credential.

| Credential | Use |
| --- | --- |
| Caller `Authorization: Bearer <session>` | Authenticates `POST /v1/faucet/claims` through identity introspection |
| `LAYERX_IDENTITY_SERVICE_TOKEN_FILE` | Bearer authorization for identity; introspection body is `{"token":"<caller session>"}` |
| `LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE` | Optional inbound shared secret for `POST /v1/faucet/service-claims` |
| `LAYERX_TESTNET_ADMIN_TOKEN_FILE` | Outbound Bearer authorization for testnet-control funding |
| `LAYERX_FAUCET_REDIS_USERNAME_FILE` and `LAYERX_FAUCET_REDIS_PASSWORD_FILE` | Redis `AUTH`; not HTTP credentials |

Direct-claim authentication requires a nonempty Bearer of at most 4096 bytes,
identity HTTP 200, and JSON `active: true` with a valid `sub`. The subject is
1–512 ASCII alphanumeric characters or `-`, `_`, `.`, `:`. A non-200 identity
response, inactive session or invalid subject produces `401 identity_required`;
transport or response-decoding failure produces `503 identity_unavailable`.
Request headers and bodies are zeroized when dropped.

The supplied identity URL ends in `/v1/introspect`. Identity's faucet response
shape is `active` plus `sub`
(`services/platform/hosted/identity/src/main.rs`, `introspection_shape`).
Secret files have trailing CR/LF removed and must contain 1–4096 bytes.

## Public routes

These are the client-facing routes of the **private** faucet. Probes require
no credential and run before claim admission.

| Method and path | Inputs | Result |
| --- | --- | --- |
| `GET /livez` | None | `200`, `{"status":"live","service":"faucet"}` |
| `GET /readyz` | None | Redis `PING`/`PONG`: `200`, `{"status":"ready","service":"faucet"}`; otherwise `503 dependency_unavailable` |
| `POST /v1/faucet/claims` | Session Bearer, `Idempotency-Key`, exact `Content-Type: application/json`, JSON `did` and `public_key` | `200` funded body, `202` pending body, or typed refusal |
| `POST /v1/faucet/service-claims` | Service Bearer, same headers, JSON `principal`, `did` and `public_key` | Same funding outcomes |

Other methods or paths return `404 not_found`. The service-claim route also
returns `404 not_found` when `LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE` is unset.
When configured, that route compares the presented secret in constant time
for equal-length tokens. It trusts the authenticated service to supply the
principal; it does not introspect a caller session itself.

Claim request types deny unknown fields. A DID must start with `did:` and
contain only the identifier characters above, at most 512 bytes. The public
key is exactly 64 ASCII hexadecimal digits without a `0x` prefix. A service
claim must be a JSON object and also name a valid principal of at most 512
bytes. `Idempotency-Key` is a 1–128 character identifier using the same
character set. These are faucet validation rules; downstream funding can
impose additional DID/key binding checks.

Both claim routes reject `forwarded`, `x-forwarded-for`, `x-real-ip`,
`x-layerx-client-ip` and `x-layerx-principal` as
`400 untrusted_identity_header`. Admission counts the TCP peer IP, including
for service claims. Content type and forwarded-header checks run before
admission; admission runs before idempotency, authentication and body checks.
Thus an otherwise refused claim can still consume a request-admission slot.

A private-network request, after obtaining its identity session and admitting
the funding journey, follows this shape. Set the URL, CA and identity values
from that private deployment; reuse the key when retrying this claim:

```sh
jq -n --arg did "$LAYERX_TEST_SOURCE_DID" \
  --arg public_key "$LAYERX_TEST_SOURCE_PUBLIC_KEY" \
  '{did:$did, public_key:$public_key}' > faucet-request.json
curl --fail --silent --show-error --max-time 30 \
  --cacert "$LAYERX_TEST_CA_FILE" \
  --header "Authorization: Bearer $(tr -d '\r\n' < "$LAYERX_TEST_AUTH_TOKEN_FILE")" \
  --header 'Idempotency-Key: faucet-private-claim-01' \
  --header 'Content-Type: application/json' \
  --request POST "$LAYERX_FAUCET_URL/v1/faucet/claims" \
  --data-binary @faucet-request.json
```

## Claim reservation and funding

`reserve` uses one Redis `EVAL` to enforce amount quotas and reserve an
idempotency record. It hashes the authenticated identity, DID, public-key
text and configured amount, with a zero byte after each part.
`funding_id` hashes the idempotency key and that request digest.

| Reservation outcome | Behavior |
| --- | --- |
| Fresh reservation | Request funding, then persist completion or return pending |
| Same digest, still reserved | Retry upstream with the same `funding_id` |
| Same digest, funded | Return `200` with the stored response |
| Same key, different digest | `409 idempotency_conflict` |
| Identity, address or network amount cap exceeded | `429 identity_quota`, `address_quota` or `network_quota` |

The address quota hashes a lowercase public key, so upper/lowercase spellings
share the cap. The request digest retains the submitted spelling: retries
must preserve the original fields. Direct claims use the identity subject
and TCP peer for their quota keys. Service claims use the supplied principal
for the identity quota and `service:<principal>` for the network amount quota;
they still share the peer-based request-admission limiter. Replicas sharing
Redis share these limits.

`fund` posts `funding_id`, `did`, `public_key` and numeric `amount` to
`LAYERX_TESTNET_FUNDING_URL`, using the control-admin Bearer and
`Idempotency-Key: <funding_id>`. Testnet-control authenticates, checks funding
journey readiness and forwards the request to its configured core admin
(`services/platform/hosted/testnet/src/main.rs`, `admin_route`).

Upstream HTTP 200 with a matching `funding_id` and `state: "funded"` becomes:

```json
{
  "funded": true,
  "funding_id": "<funding id>",
  "transaction_id": "<upstream transaction id, or null>",
  "amount": "1000000",
  "network": "layerx-testnet"
}
```

`amount` is the configured fixed claim amount as a decimal string, not a
caller-selected amount. `transaction_id` is copied from the upstream and can
be null. The literal `network: "layerx-testnet"` identifies this private
service response; it does not assert a public chain ID or mainnet allocation.
The faucet checks the upstream funding response, rather than independently
reading the destination's chain balance. Check committed account state for
the resulting credit.

Upstream HTTP 4xx triggers quota rollback and `503 funding_rejected` if
rollback succeeds. Transport failure, other statuses, malformed or mismatched
funding evidence, completion failure, or rollback failure leaves the outcome
pending. HTTP `202` carries
`{"state":"still_checking","retry":"after","retry_after_seconds":10}`
and `Retry-After: 10`. Retry with the original key and fields; pending does
not mean no transfer occurred. Redis retains the response for funded replays
within the configured idempotency TTL.

## Optional gateway starter credit

A private operator can configure the gateway's faucet integration using
`LAYERX_GATEWAY_FAUCET_URL`, `LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE` and
`LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT`. The amount must be a canonical positive
decimal no greater than `9007199254740991` and must match the faucet's fixed
claim amount. The shared service token must match the faucet's inbound token.
The gateway also needs its identity backend and registration service token
(`services/platform/hosted/gateway/src/rpc_faucet.rs`).

`px_requestFunds` takes positional `[did, signer_public_key]`. The key must be
a nonzero 32-byte lowercase hexadecimal Ed25519 key without a prefix and must
be authorized by the Bearer session. The gateway obtains the principal's
starter-credit grant from identity, derives idempotency from the principal
and grant, and calls `POST /v1/faucet/service-claims`. Retries reuse that grant.
It requires complete funded evidence, including a nonempty transaction ID,
checks the exact funded amount, and settles the identity starter-credit
record before returning the funded body with `starter_credit` added.

| JSON-RPC code | Condition |
| --- | --- |
| `-32602` | Invalid parameters; faucet HTTP 400 or 409 |
| `-32002` | Missing session, unbound signer, or faucet HTTP 401, 403 or 404 |
| `-32001` | Unconfigured faucet, dependency failure, or pending claim (`data.code: faucet_claim_pending`) |
| `-32005` | Gateway claim-rate limit (`data.code: faucet_rate_limit`) or faucet HTTP 429 |
| `-32603` | Invalid/incomplete upstream evidence or a funded-amount mismatch |
| `-32010` | Identity backend unavailable at gateway dispatch (`kernel_unavailable`) |

A grant amount that cannot be represented canonically in the faucet's range
returns `-32001`, message `Starter credit unavailable`, and `data.code`
`starter_credit_amount_unrepresentable`. A representable amount differing
from the configured claim amount returns `-32603`, message
`Starter credit amount mismatch`, and `data.code`
`starter_credit_amount_mismatch`. Both checks run before the service claim
and before consuming the gateway's 30-claims-per-minute process-local slot.
A different amount in the funded response also produces the mismatch error.
Other identity starter-credit refusals retain the identity response in
`error.data`; consult the OpenRPC contract and `grant_result` for that mapping.

## Config keys

These keys are read by `config` in the faucet implementation. Amounts and
caps are integer funding units, not decimal token display amounts.

| Key | Role or default |
| --- | --- |
| `LAYERX_FAUCET_LISTEN` | Socket address; `0.0.0.0:9443` |
| `LAYERX_FAUCET_TLS_CERT_DER` | Server certificate DER file |
| `LAYERX_FAUCET_TLS_KEY_DER` | PKCS#8 server private-key DER file |
| `LAYERX_OUTBOUND_CA_DER` | CA DER file for HTTPS and Redis |
| `LAYERX_IDENTITY_INTROSPECTION_URL` | HTTPS URL including introspection path |
| `LAYERX_IDENTITY_SERVICE_TOKEN_FILE` | Identity service Bearer file |
| `LAYERX_FAUCET_SERVICE_CLAIM_TOKEN_FILE` | Optional inbound service Bearer file; unset disables service claims |
| `LAYERX_TESTNET_FUNDING_URL` | HTTPS control-admin funding URL including path |
| `LAYERX_TESTNET_ADMIN_TOKEN_FILE` | Control-admin Bearer file |
| `LAYERX_FAUCET_REDIS_URL` | Redis TLS URL |
| `LAYERX_FAUCET_REDIS_USERNAME_FILE` | Redis ACL username file |
| `LAYERX_FAUCET_REDIS_PASSWORD_FILE` | Redis ACL password file |
| `LAYERX_FAUCET_CLAIM_AMOUNT` | Fixed claim amount; `1000000` |
| `LAYERX_FAUCET_IDENTITY_LIMIT` | Amount cap per identity/window; `10000000` |
| `LAYERX_FAUCET_ADDRESS_LIMIT` | Amount cap per public key/window; `10000000` |
| `LAYERX_FAUCET_NETWORK_LIMIT` | Amount cap per peer or service principal/window; `50000000` |
| `LAYERX_FAUCET_NETWORK_REQUEST_LIMIT` | Requests per TCP peer/admission window; `60` |
| `LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS` | Admission window; `60` |
| `LAYERX_FAUCET_WINDOW_SECONDS` | Amount-quota window; `86400` |
| `LAYERX_FAUCET_IDEMPOTENCY_SECONDS` | Idempotency TTL; `604800` |

The quota window and amount must be positive; idempotency retention must
cover at least one quota window. Each amount cap must accommodate a full
claim and be at most `9007199254740991` so Redis Lua arithmetic remains exact.
Admission bounds of zero cause admission to fail as
`503 persistence_unavailable`. The Deployment leaves claim amount and quota
window unset, using their code defaults.

Configure operator-supplied private endpoints; replace the placeholders below:

| Dependency | Configuration | Example URL |
| --- | --- | --- |
| Identity | `LAYERX_IDENTITY_INTROSPECTION_URL` | `https://OPERATOR-IDENTITY-ORIGIN:9443/v1/introspect` |
| Funding | `LAYERX_TESTNET_FUNDING_URL` | `https://OPERATOR-CONTROL-ADMIN-ORIGIN/admin/v1/testnet/fund` |
| Redis | `LAYERX_FAUCET_REDIS_URL` | `rediss://OPERATOR-FAUCET-REDIS:6379` |

The Deployment mounts `layerx-faucet-tls` at `/run/layerx/tls`,
`layerx-testnet-identity-client` at `/run/layerx/identity`,
`layerx-testnet-control-admin` at `/run/layerx/control-admin`,
`layerx-gateway-faucet-client` at `/run/layerx/gateway-faucet`, and
`layerx-faucet-redis-client` at `/run/layerx/redis-auth`.

## Redis state and ACL

The supplied StatefulSet `layerx-faucet-redis` runs `redis:8.2.1-alpine` on TLS
port 6379 with `appendonly yes`, `appendfsync always`,
`maxmemory-policy noeviction`, `protected-mode yes` and ACL file
`/run/layerx/auth/users.acl`. Its Service is headless. Cluster bring-up writes
`user default off` and an enabled `layerx-faucet` user with `~* &* +@all`.

| Key | Contents |
| --- | --- |
| `faucet:admission:{window}:{digest}` | Request count; peer digest; expires after the admission window duration |
| `faucet:quota:{window}:identity:{digest}` | Reserved amount per identity; expires at quota-window boundary |
| `faucet:quota:{window}:address:{digest}` | Reserved amount per lowercase public key |
| `faucet:quota:{window}:network:{digest}` | Reserved amount per peer or service scope |
| `faucet:idem:{digest}` | Request digest, state, funding ID, quota keys, amount and completed response; idempotency TTL |
| `faucet:audit` and `faucet:audit:head` | Audit stream and reservation hash-chain head |

Reservation events include `event`, `result`, optional `funding_id` and
`chain`; completion and rollback append events without a chain field.
Authentication credentials do not enter these audit records. Preserve the
Redis persistent volume to retain reservations, limits and funded replays
across faucet restarts.

## Callers the NetworkPolicy admits

The supplied policies admit faucet TCP 9443 from IPv4/IPv6 IP blocks and from
testnet-control and gateway pods. That policy does not create a public Service
or Ingress. Faucet egress permits trusted-boundary pods on 9443,
testnet-control on 9444, Redis on 6379 and DNS on UDP/TCP 53. Redis ingress
admits faucet and testnet-control pods on 6379. The binary calls its configured
identity and control endpoints; network policy alone is not proof that those
dependencies are ready.

Deployment readiness probes call HTTPS `/readyz` every 5 seconds and liveness
probes call `/livez` every 15 seconds. Faucet readiness checks **only Redis**,
not identity, testnet-control or the funding journey.

## Typed refusals

HTTP refusals use `{"error":{"code":"…","retry":"never"}}`, or
`retry: "after"` with `retry_after_seconds` and a `Retry-After` header.
Success and pending bodies use their own shapes.

| HTTP | Code | Condition |
| --- | --- | --- |
| 400 | `invalid_request` | Invalid HTTP framing or request bound exceeded |
| 400 | `content_type_required` | Claim content type is not exactly `application/json` |
| 400 | `untrusted_identity_header` | Forwarded identity/IP header supplied |
| 400 | `idempotency_key_required` | Missing idempotency key |
| 400 | `invalid_idempotency_key` | Invalid or oversized key |
| 400 | `invalid_argument` | Invalid claim fields or JSON |
| 401 | `service_token_required` | Missing or incorrect service-claim Bearer |
| 401 | `identity_required` | Missing, invalid or inactive direct-claim session |
| 404 | `not_found` | Unknown route/method, or service claims disabled |
| 409 | `idempotency_conflict` | Existing key names a different digest |
| 429 | `network_request_rate` | Peer request-admission limit exceeded |
| 429 | `identity_quota` | Identity amount cap exceeded |
| 429 | `address_quota` | Public-key amount cap exceeded |
| 429 | `network_quota` | Peer/service-scope amount cap exceeded |
| 503 | `dependency_unavailable` | Readiness Redis check failed |
| 503 | `persistence_unavailable` | Admission/reservation persistence failed |
| 503 | `identity_unavailable` | Identity transport or response decoding failed |
| 503 | `funding_rejected` | Upstream 4xx and successful rollback |

Quota refusals advise retry at the quota-window boundary; request-rate
refusals return the configured admission-window duration. Dependency,
persistence, identity and funding refusals advise a 10-second delay. HTTP
`202 still_checking` is an indeterminate funding outcome, not a refusal.

## Tests

The faucet's `#[cfg(test)]` module covers exact service-token admission,
service-claim fields, principal-scoped network buckets, case-normalized
address quotas, Redis-safe quota bounds, and refusal/retry shapes.
`platform-test-tooling` includes the faucet crate through the repository's
runtime-clock wrapper (`services/platform/Makefile.inc`).

The private hosted smoke script
`services/platform/hosted/testnet/tests/hosted-smoke.sh` checks faucet readiness,
admits the funding journey and submits a session-authenticated claim. It
asserts `funded == true` and a non-null `funding_id`. Its
`make platform-hosted-smoke` entrypoint requires private deployment URLs,
including `LAYERX_FAUCET_URL`, the CA, session token and test identity inputs.
These tests describe private integration coverage, not public availability.

[Home](../index.md)
