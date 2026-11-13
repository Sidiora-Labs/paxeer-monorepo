# Hosted control

Private-network only.

Hosted control is retained for private or disposable networks. It has no public
control hostname on Paxeer mainnet-beta. Public accounts receive native chain
transfers; the private funding and reset APIs below are not a public funding
service.

Paxeer is one BFT network running `paxd`, with the kernel inside the network
proposing blocks. EVM chain ID is `125` (`0x7d`), and a receipt is final when its
block is committed. Use a root node URL from
[Public JSON-RPC](PublicRpc.md) for chain calls. Hosted identity and faucet
operations use the configured gateway application.

The retained binary `layerx-testnet-control`, from crate
`layerx-platform-testnet`, exposes status and journey admission on one TLS
listener and authenticated fund/reset proxies on another. These operations
coordinate hosted services; they do not define another execution or finality
layer. The `layerx-testnet` names and network ID `402` below identify this
private compatibility harness, not the public EVM chain ID. Sub-chains on the
merged network are tenant domains in the database, not additional control
clusters.

The implementation is in
[`services/platform/hosted/testnet/src/main.rs`](../../services/platform/hosted/testnet/src/main.rs),
with defaults and release validation in
[`src/lib.rs`](../../services/platform/hosted/testnet/src/lib.rs).
Read [Hosted faucet](HostedFaucet.md) for claim admission,
[Hosted core](HostedCore.md) for the funding backend, and
[Hosted identity](HostedIdentity.md) for sessions.

## Deployment and listeners

[`services/platform/hosted/testnet/deployment.yaml`](../../services/platform/hosted/testnet/deployment.yaml)
contains the control Deployment, faucet Deployment, Redis, Services, CronJobs,
and NetworkPolicies. Control has two replicas, image
`ghcr.io/sidiora-labs/layerx-testnet-control:0.1.0`, and runs as UID/GID `4020`.
The container runs `/usr/local/bin/layerx-testnet-control` directly. Its image
also carries the operator scripts under `/opt/layerx/`.

| Listener | Default bind | Service |
| --- | --- | --- |
| Public status | `0.0.0.0:9443` | `layerx-testnet-public`, port `443` → `public-tls` |
| Admin | `0.0.0.0:9444` | `layerx-testnet-admin`, ClusterIP port `443` → `admin-tls` |

Here “public” names the listener with unauthenticated status reads inside the
private installation. The source manifest has no control Ingress. Configure
operator-supplied private HTTPS origins with the `LAYERX_TESTNET_*_URL` keys
below. [Hosted node](HostedNode.md) describes the native node deployment.

## TLS

Both listeners use rustls with a DER server certificate and PKCS#8 DER key,
and no client-certificate authentication. Admin authorization is a Bearer
check. The binary permits at most 128 concurrent connections across the two
listeners and bounds HTTP messages at 64 KiB. It rejects query strings,
`Transfer-Encoding`, duplicate headers, and requests missing `Host`.

Outbound HTTPS uses `native_tls`, the configured CA certificate, a minimum
TLS version of 1.2, a 3-second connection timeout, and an 8-second I/O timeout.
Component endpoints require `https://`; the Redis probe requires `rediss://`
without a path. The control client does not present a client certificate.

## Tokens

| Credential | Configuration | Purpose |
| --- | --- | --- |
| Control admin token | `LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE` | Incoming admin Bearer, compared at equal length in constant time |
| Backend admin token | `LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE` | Outgoing Bearer to core-admin |
| Scheduled reset token | `LAYERX_TESTNET_RESET_TOKEN_FILE` in the reset job | Bearer to control admin; the job mounts the control-admin Secret |
| Status publication token | `LAYERX_STATUS_TOKEN_FILE` in the publisher job | Bearer to the separately operated status publisher |

The backend and control-admin tokens are distinct credentials. The faucet
reads the control-admin credential through `LAYERX_TESTNET_ADMIN_TOKEN_FILE`.
Control reads secret files at startup, strips trailing CR/LF, and rejects
empty values or values over 4096 bytes. Public status routes require no Bearer.
Identity is probed with an unauthenticated readiness GET; control does not
introspect user sessions.

## Public routes

`public_route` accepts GET only; other methods and unknown paths return
`404 not_found`.

| Path | Result |
| --- | --- |
| `/livez` | `200 {"status":"live"}` |
| `/readyz` | Readiness document; `200` when ready, otherwise `503` |
| `/v1/status` | `200` status document, including degraded state when applicable |
| `/v1/parameters` | `200` private harness parameters |
| `/v1/journeys/funding` | Funding admission document; `200` or `503` |
| `/v1/journeys/payment` | Payment admission document; `200` or `503` |
| `/v1/journeys/receipt-inspection` | Receipt inspection admission document; `200` or `503` |

Neither programs nor settlement has a journey route. Parameters contain
`network: "layerx-testnet"`, `network_id: 402`, `evm_chain_id: 125`, the running
`package_semver`, `lxp_wire_protocol_version`, and `reset_schedule`.
`LXP_WIRE_PROTOCOL_VERSION` comes from
`layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION`; it is checked
independently of the package version.

The status document has `service: "layerx-hosted-testnet"`, `state`,
`package_semver`, `lxp_wire_protocol_version`, `network_id`, and `components`.
The four components are `testnet`, `gateway`, `core`, and `paxeer`, each with
`name` and `state`. `testnet` is ready only when the release gate and identity,
faucet, core-admin, receipt-authority, and Redis dependencies are ready;
otherwise it is degraded. The other components are ready or unavailable
according to their individual probes. Overall state reflects all dependencies
and journeys. This shape differs from gateway status.

## Journeys and probes

Control currently probes eight dependencies and exposes three journeys.
A journey admits work only when every dependency in its set is ready.

| Journey | Dependencies |
| --- | --- |
| `funding` | `identity`, `faucet`, `redis`, `core_admin`, `core` |
| `payment` | `identity`, `gateway`, `core`, `receipt_authority` |
| `receipt_inspection` | `gateway`, `receipt_authority`, `core` |

Admission returns `admitted`, `journey`, `ready`, `dependencies`, and `failing`.
A ready response has HTTP 200, `admitted: true`, `ready: true`, and an empty
`failing` list. Degraded admission returns HTTP 503 and names the failed
dependencies.

Identity, faucet, core, receipt authority, gateway, and Paxeer are probed with
unauthenticated `GET /readyz` and must return HTTP 200. Core-admin is probed by
TLS handshake only. Redis is probed over TLS with `PING`; `+PONG` or `-NOAUTH`
counts as reachable. The latter proves an ACL-protected listener, not that the
faucet can authenticate or persist a claim. Registry is not in the current
dependency or journey sets.

Global readiness requires all eight dependencies, all three journeys, and the
package/wire release gate. For example, a failed Paxeer probe degrades global
readiness without failing any of the three journey dependency sets. The
readiness document adds `release`, `dependencies`, and `journeys` to the
release identity fields. Startup also validates package and wire agreement
against `platform_testnet()` before serving.

## Admin routes

Only the admin listener serves these routes; its listener has no health GETs.
Every request first requires the control-admin Bearer, exact
`Content-Type: application/json`, and `Idempotency-Key`. Keys are 1–128 ASCII
letters, digits, or characters from `-_.:`.

| Method and path | Body | Behavior |
| --- | --- | --- |
| `POST /admin/v1/testnet/fund` | `funding_id`, `did`, `public_key`, `amount` | Validate, admit the funding journey, then proxy to core-admin at the same path |
| `POST /admin/v1/testnet/reset` | Exact bytes `{}` | Proxy to core-admin at the same path without journey admission |

Funding JSON rejects unknown fields. `funding_id` must satisfy the key rules;
`did` must start with `did:` and be at most 512 bytes; `public_key` is 64 ASCII
hex digits; `amount` is a positive `u64`. Core imposes additional identity
binding checks described below.

The proxy keeps the request body and idempotency key but uses the backend
admin Bearer. It passes through upstream HTTP 200, 202, and 400–499 responses.
Other outcomes, including transport failures and upstream 5xx, become
`503 core_unavailable` with `retry: "after"`.

## Native treasury funding path

The private faucet posts its reserved `funding_id`, DID, signer public key,
and configured amount to control admin. After validation and journey
admission, control forwards to the configured core-admin origin. Set
`LAYERX_TESTNET_CORE_ADMIN_URL` to the configured private HTTPS origin.

For new funding, core resolves the signer key to a nonzero chain address using
the addr precompile at `0x0000000000000000000000000000000000001004`, then signs
and submits an EIP-1559 native value transfer on chain `125`. It persists the
prepared transfer and funding intent for retries, obtains the transaction
receipt, and archives the result. This path is implemented in
[`core/src/chain_fund.rs`](../../services/platform/hosted/core/src/chain_fund.rs)
and [`core/src/funding_native.rs`](../../services/platform/hosted/core/src/funding_native.rs).
The older Asset SEND path is retained for replaying existing journal records;
it is not the construction path for new native funding.

Core additionally requires `did` to equal `did:layerx:` followed by the
lowercase public key and refuses the treasury DID. An unbound chain address
is refused. Passing control's field validation alone does not establish that
the recipient can be funded. Core's native sender uses
`LAYERX_CORE_CHAIN_RPC_URL` and either
`LAYERX_CORE_FUNDING_SENDER_KEY_FILE` or
`LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID`.

Core reports `state: "pending"` with HTTP 202 while confirmation is unavailable.
The faucet reports this as `202 still_checking`; it only completes the claim
when core returns HTTP 200 with matching `funding_id` and `state: "funded"`.
Control itself keeps no claim journal or Redis state. See
[claim reservation and funding](HostedFaucet.md#claim-reservation-and-funding)
for replay and rollback behavior.

## Config keys

These are read by control's `config()`; paths shown are the manifest mounts.

| Key | Role or default |
| --- | --- |
| `LAYERX_TESTNET_PUBLIC_LISTEN` | `0.0.0.0:9443` |
| `LAYERX_TESTNET_ADMIN_LISTEN` | `0.0.0.0:9444` |
| `LAYERX_TESTNET_TLS_CERT_DER` | `/run/layerx/tls/server.crt.der` |
| `LAYERX_TESTNET_TLS_KEY_DER` | `/run/layerx/tls/server.key.der` |
| `LAYERX_OUTBOUND_CA_DER` | `/run/layerx/tls/ca.crt.der` |
| `LAYERX_PENDING_PACKAGE_SEMVER` | Required pending package; ConfigMap `package-semver` |
| `LAYERX_PENDING_WIRE_PROTOCOL_VERSION` | Required pending `u16`; ConfigMap `lxp-wire-protocol-version` |
| `LAYERX_TESTNET_CORE_URL` | Core HTTPS origin |
| `LAYERX_TESTNET_CORE_ADMIN_URL` | Core-admin HTTPS origin |
| `LAYERX_TESTNET_GATEWAY_URL` | Gateway HTTPS origin |
| `LAYERX_TESTNET_PAXEER_URL` | Paxeer-boundary HTTPS origin |
| `LAYERX_TESTNET_IDENTITY_URL` | Identity HTTPS origin |
| `LAYERX_TESTNET_FAUCET_URL` | Faucet HTTPS origin |
| `LAYERX_TESTNET_RECEIPT_AUTHORITY_URL` | Receipt-authority HTTPS origin |
| `LAYERX_TESTNET_REDIS_URL` | Redis `rediss://` endpoint |
| `LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE` | `/run/layerx/backend-admin/token` |
| `LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE` | `/run/layerx/control-admin/token` |

The parameters handler returns the first-Tuesday reset schedule from
`platform_testnet()`. The Deployment starts the binary directly.

## Cluster CronJobs

[`reset-testnet.sh`](../../services/platform/hosted/testnet/reset-testnet.sh)
runs only on Tuesday and UTC days 1–7. CronJob `layerx-testnet-reset` schedules
it at `0 9 * * 2` in `Etc/UTC`, with concurrency forbidden. The script sends
exactly `{}` to control admin, with key `scheduled-reset-YYYYMM`, and requires
`state: "reset"` plus a nonempty `reset_id`. Its inputs are
`LAYERX_TESTNET_ADMIN_URL`, `LAYERX_TESTNET_RESET_TOKEN_FILE`, and
`LAYERX_TESTNET_CA_FILE`. This is a disposable-network operation, not a public
mainnet reset procedure.

[`render-status.sh`](../../services/platform/hosted/testnet/render-status.sh)
fetches `LAYERX_TESTNET_STATUS_URL`, validates the four-component status shape,
and PUTs it to `LAYERX_STATUS_PUBLISH_URL` using `LAYERX_STATUS_TOKEN_FILE` and
`LAYERX_TESTNET_CA_FILE`. The manifest schedules
`layerx-testnet-status-publisher` every two minutes. The publisher destination
is configured separately through `LAYERX_STATUS_PUBLISH_URL`.

## Callers the NetworkPolicy admits

The manifest admits control admin TCP 9444 from faucet and reset-job pods.
The status listener TCP 9443 admits ingress-controller and status-publisher
traffic. Egress permits trusted-boundary pods on 9443/9444/9445, gateway and
faucet on 9443, faucet Redis on 6379, program registry on 9420, and DNS on 53.
The retained registry policy edge does not add a registry readiness probe.
Deployment health checks use the public listener: readiness every 5 seconds
and liveness every 15 seconds.

## Typed refusals

Control errors use `{"error":{"code":…}}`, sometimes with `retry` or journey
details. Its HTTP writer does not emit `Retry-After`, including when a proxied
body contains retry advice. The [faucet](HostedFaucet.md#typed-refusals) does
emit that header when it has a delay.

| HTTP | Code | Condition |
| --- | --- | --- |
| 400 | `invalid_request` | HTTP framing failed |
| 400 | `content_type_required` | Admin content type is not exactly `application/json` |
| 400 | `idempotency_key_required` | Missing admin idempotency key |
| 400 | `invalid_idempotency_key` | Invalid admin key |
| 400 | `invalid_argument` | Invalid fund fields or reset body |
| 401 | `unauthorized` | Missing or mismatched admin Bearer |
| 404 | `not_found` | Unknown method/path on the selected listener |
| 503 | `journey_degraded` | Funding dependencies failed; includes `journey`, `failing`, `retry: "after"` |
| 503 | `core_unavailable` | Core proxy transport or non-passthrough status failure |
| 500 | `serialization_failure` | Response serialization failed |

Readiness and journey GETs return their own documents with HTTP 503 when
degraded, rather than the generic error envelope.

## Tests

The existing crate tests cover release validation, dependency indexing,
journey dependency sets and readiness, degraded dependencies, public status
shape, private endpoint defaults, EVM chain identity, and Redis URL parsing.
`platform-test-tooling` in
[`services/platform/Makefile.inc`](../../services/platform/Makefile.inc) runs
both the control and faucet crate tests through the runtime-clock wrapper and
checks the operator scripts' syntax.

[`hosted-smoke.sh`](../../services/platform/hosted/testnet/tests/hosted-smoke.sh)
exercises readiness, parameter identity, funding, payment, and receipt
inspection against configured private services. These are operational test
entry points, not evidence that a public control service is deployed.

[Hosted faucet](HostedFaucet.md) · [Home](Home.md)
