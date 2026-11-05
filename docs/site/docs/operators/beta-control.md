# Hosted control

`layerx-testnet-control` is a private-network utility for disposable hosted
status, journey admission, funding and reset. It has no public control DNS
name or Ingress in `services/platform/hosted/testnet/deployment.yaml`. Its
“public” listener means unauthenticated within the permitted private network.
It is not a protocol node or an authority over public-network finality.

Paxeer mainnet-beta is one BFT network: the kernel is part of `paxd` and
proposes blocks, EVM chain ID is `125` (`0x7d`), and a receipt is final once its
block is committed. Public JSON-RPC uses the root of
`https://api-mainnet-beta.paxeer.network` or the numbered nodes `api1` through
`api14.mainnet-beta.paxeer.network`; see [Public RPC endpoints](../reference/public-rpc.md).
The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and
`txpool`. Hosted `px_` methods are declared in
`services/platform/hosted/gateway/openrpc.json`; see
[Hosted gateway JSON-RPC](../platform/gateway-rpc.md). Public funding uses
native transfers through node JSON-RPC. The custody-credit/router funding
instructions are retired.

For native inputs, deployment limits, readiness and retained volumes, see
[Disposable beta cluster](beta-cluster.md). Tenant domains are database
namespaces on the same network. The private utility's `network_id` `402` is
its configured application namespace identifier, distinct from EVM chain ID
`125`; it is not evidence of a second public chain.

## Deployment and TLS

The crate is `layerx-platform-testnet`, with library `src/lib.rs` and binary
`src/main.rs` under `services/platform/hosted/testnet/`. The Deployment has
two replicas and runs `/usr/local/bin/layerx-testnet-control` from
`ghcr.io/sidiora-labs/layerx-testnet-control:0.1.0` as UID/GID `4020`.

| Listener | Default bind | ClusterIP Service |
| --- | --- | --- |
| Unauthenticated status | `0.0.0.0:9443` | `layerx-testnet-public`, port `443` → `9443` |
| Authenticated admin | `0.0.0.0:9444` | `layerx-testnet-admin`, port `443` → `9444` |

Inbound rustls uses `LAYERX_TESTNET_TLS_CERT_DER` and
`LAYERX_TESTNET_TLS_KEY_DER` (PKCS#8); neither listener requests a client
certificate. Outbound HTTPS uses `LAYERX_OUTBOUND_CA_DER` with a TLS 1.2
minimum and no client certificate. Component URLs must use HTTPS; Redis uses
`rediss://` without a path. Connect timeout is 3 seconds, I/O timeout is
8 seconds, requests are bounded at 64 KiB, and at most 128 connections are
live across both listeners. The parser refuses query strings,
`Transfer-Encoding`, duplicate headers and a missing `Host`.

NetworkPolicy admits admin TCP `9444` from the faucet and reset job. Status
TCP `9443` admits `ingress-nginx` and the status-publisher pod. An allowed
ingress namespace does not itself provision a public edge. Deployment health
probes use the status listener. Keep control and its reset credentials inside
the disposable environment.

## Tokens

| Key | Purpose |
| --- | --- |
| `LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE` | Inbound admin Bearer; mounted at `/run/layerx/control-admin/token` |
| `LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE` | Outbound Bearer to core-admin; mounted at `/run/layerx/backend-admin/token` |
| `LAYERX_TESTNET_RESET_TOKEN_FILE` | Reset script's Bearer to control; the CronJob mounts the control-admin token |
| `LAYERX_STATUS_TOKEN_FILE` | Status publisher's Bearer to its external publishing destination |

Control compares the inbound Bearer in constant time. Token files have
trailing CR/LF stripped and must contain 1–4096 bytes. Inbound and backend
admin credentials have different roles. The faucet uses
`LAYERX_TESTNET_ADMIN_TOKEN_FILE` to call control. Identity readiness is an
unauthenticated probe; control does not require an identity service token.

## Public routes

These routes require no Bearer on the private status listener. Non-GET
methods and unknown paths return `404 not_found`.

| Route | Result |
| --- | --- |
| `GET /livez` | `200 {"status":"live"}` |
| `GET /readyz` | Readiness document, `200` when ready, otherwise `503` |
| `GET /v1/status` | Redacted four-component status, HTTP `200` even when degraded |
| `GET /v1/parameters` | `network`, `network_id`, `evm_chain_id`, `package_semver`, `lxp_wire_protocol_version`, `reset_schedule` |
| `GET /v1/journeys/funding` | Funding admission, HTTP `200` or `503` |
| `GET /v1/journeys/payment` | Payment admission, HTTP `200` or `503` |
| `GET /v1/journeys/receipt-inspection` | Receipt-inspection admission, HTTP `200` or `503` |

The current `Journey::ALL` contains three journeys. Programs and settlement
journey paths return `404`; they are not admission routes.

Parameters report `network: "layerx-testnet"`, `network_id: 402` and
`evm_chain_id: 125`. Package semver comes from the crate; wire version comes
from `LXP_WIRE_PROTOCOL_VERSION`, which uses the shared state-commitment
protocol constant. Do not infer the wire version from package semver.
`reset_schedule` is `09:00 UTC on the first Tuesday of every month`.

Status keys are `service`, `state`, `package_semver`,
`lxp_wire_protocol_version`, `network_id` and `components`. Service is
`layerx-hosted-testnet`; components are exactly `testnet`, `gateway`, `core`
and `paxeer`, each with `name` and `state`. The `testnet` component reflects
the release gate; the other three reflect their matching dependency probes.
This redacted document is not the full readiness document and is not a
committed-block proof.

## Journeys and probes

`Dependency::ALL` in `testnet/src/main.rs` contains eight dependencies.
The names below are retained private compatibility service names. They do
not describe independent execution, settlement or receipt-finality networks.

| Journey | Dependencies |
| --- | --- |
| `funding` | `identity`, `faucet`, `redis`, `core_admin`, `core` |
| `payment` | `identity`, `gateway`, `core`, `receipt_authority` |
| `receipt_inspection` | `gateway`, `receipt_authority`, `core` |

A ready admission response has `admitted: true`, `ready: true`, and empty
`failing`. A degraded response has HTTP `503`, `admitted: false` and the
failing dependency names. Global readiness additionally requires all eight
dependencies and the package/wire release gate. Thus a `paxeer` probe failure
can degrade global readiness while all journeys remain admitted. The program
registry is not a current probe dependency.

| Dependency | Probe |
| --- | --- |
| `identity`, `faucet`, `core`, `receipt_authority`, `gateway`, `paxeer` | Unauthenticated `GET /readyz`, requires HTTP `200` |
| `core_admin` | TLS handshake only |
| `redis` | TLS then `PING`; `+PONG` or `-NOAUTH` proves listener reachability |

A successful Redis probe does not prove that faucet ACL credentials work.
Control does not write Redis; faucet claim quota and idempotency reservations
belong to the faucet's Redis store. Core's native node health is separate
from control's aggregate dependency reachability.

## Admin routes

Admin routes use the admin listener only. Requests require a valid inbound
Bearer, exact `Content-Type: application/json` and an `Idempotency-Key` of
1–128 ASCII alphanumeric characters or `-`, `_`, `.`, `:`. There is no admin
health route; authentication and header checks precede route dispatch.

| Route | Body and behavior |
| --- | --- |
| `POST /admin/v1/testnet/fund` | JSON `funding_id`, `did`, `public_key`, `amount`; unknown fields are refused. Requires the funding journey, then proxies to core-admin at the same path |
| `POST /admin/v1/testnet/reset` | Body must be exactly the two bytes `{}`. Proxies to core-admin at the same path without a journey probe |

Control requires a valid `funding_id`, a `did` beginning with `did:` and at
most 512 bytes, a 64-character ASCII hexadecimal public key, and nonzero
amount. Core additionally requires `did:layerx:<lowercase-public-key>` and
refuses the treasury DID. Use the same request body and idempotency key for
retries; control passes both to core-admin with the backend admin token.

Upstream `200`, `202` and `4xx` responses pass through. Other upstream
outcomes become `503 core_unavailable` with `retry: "after"`. This includes
backend `5xx`; inspect core's private evidence when diagnosing the original
cause. Control does not add a `Retry-After` header.

## Private funding and recovery

The faucet posts the funding command to control. Control validates and
proxies it; core's `fund`, `fund_send`, `funding_native.rs` and `chain_fund.rs`
own the native transfer and its durable recovery. New funding prepares an
EVM value transfer, records an authorized version-2 intent, submits the signed
transaction and checks its receipt. The recipient is the address bound to
the identity public key through the identity precompile at
`0x0000000000000000000000000000000000001004`. A missing or zero binding is
`409 identity_binding_unavailable` in the funding implementation.

The sender needs native funds for the value and gas. There is no custody
deposit or LNI SEND bootstrap for new funding. A pending confirmation returns
`202` with `state: "pending"` and the transaction ID. The faucet treats
pending as indeterminate, not funded. Core archives confirmed native receipts
and preserves authenticated older intent records for replay through their
original native interface. It does not convert an old intent into a second
value transfer.

Retain core's funding journals and archives across restarts. After an
indeterminate response, retry the same private funding identifier, body and
idempotency key. Changing the key can create another transfer. A reset is a
disposable-generation operation mediated by the node supervisor; core retains
funding receipt evidence before requesting it. It is not an operation for
public mainnet, and resetting an application namespace is not a public chain
rollback.

## Configuration

The source of truth for required inputs is `config()` in
`testnet/src/main.rs`; mount definitions are in `testnet/deployment.yaml`.

| Keys | Role |
| --- | --- |
| `LAYERX_TESTNET_PUBLIC_LISTEN`, `LAYERX_TESTNET_ADMIN_LISTEN` | Listener overrides; defaults `0.0.0.0:9443` and `0.0.0.0:9444` |
| `LAYERX_TESTNET_TLS_CERT_DER`, `LAYERX_TESTNET_TLS_KEY_DER`, `LAYERX_OUTBOUND_CA_DER` | Server certificate, PKCS#8 key and outbound CA |
| `LAYERX_PENDING_PACKAGE_SEMVER`, `LAYERX_PENDING_WIRE_PROTOCOL_VERSION` | Required pending release identity; mismatches refuse startup |
| `LAYERX_TESTNET_CORE_URL`, `LAYERX_TESTNET_CORE_ADMIN_URL` | Private core application and admin HTTPS origins |
| `LAYERX_TESTNET_GATEWAY_URL`, `LAYERX_TESTNET_PAXEER_URL` | Gateway and Paxeer-boundary HTTPS origins |
| `LAYERX_TESTNET_IDENTITY_URL`, `LAYERX_TESTNET_FAUCET_URL` | Identity and faucet HTTPS origins |
| `LAYERX_TESTNET_RECEIPT_AUTHORITY_URL` | Private receipt-verification compatibility service |
| `LAYERX_TESTNET_REDIS_URL` | TLS Redis origin |
| `LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE`, `LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE` | Distinct outbound and inbound admin credentials |

The manifest uses Service port `443` for gateway and faucet, `9443` for
core, identity, receipt authority and Paxeer boundary, `9444` for core-admin,
and `6379` for Redis. `LAYERX_TESTNET_REGISTRY_URL` remains in the manifest
but is not read by current control configuration. Library defaults in
`platform_testnet()` use private service origins. The Deployment runs the
binary directly; the older `run-testnet.sh` wrapper is not its entrypoint.

## Cluster CronJobs

`layerx-testnet-reset` is scheduled Tuesdays at `09:00 UTC`.
`reset-testnet.sh` additionally checks that it is Tuesday and the day of the
month is at most seven. It POSTs `{}` with one idempotency key per calendar
month and requires `state: "reset"` and a non-empty `reset_id`. A pending
reset is not a completed reset.

`layerx-testnet-status-publisher` runs every two minutes. Its
`render-status.sh` fetches `/v1/status`, checks the redacted four-component
shape and PUTs it to `LAYERX_STATUS_PUBLISH_URL` using the status token. The
publishing service is external to this repository's deployment. The beta
helper omits that job when `LAYERX_BETA_STATUS_PUBLISH_URL` is unset and
reports the missing owner input.

## Typed refusals

JSON refusals use `{"error":{"code":…}}`. Readiness and journey failures
instead return their readiness/admission documents with HTTP `503`.

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Request framing rejected |
| `400` | `content_type_required` | Admin content type missing or different |
| `400` | `idempotency_key_required` | Admin idempotency key missing |
| `400` | `invalid_idempotency_key` | Admin key outside the accepted alphabet or length |
| `400` | `invalid_argument` | Funding fields invalid or reset body differs from `{}` |
| `401` | `unauthorized` | Admin Bearer missing or mismatched |
| `404` | `not_found` | Unknown route or unsupported method after applicable checks |
| `503` | `journey_degraded` | Funding journey failed; includes journey, failing names and retry hint |
| `503` | `core_unavailable` | Backend transport failed or status was not `200`, `202` or `4xx` |
| `500` | `serialization_failure` | JSON serialization failed |

Backend `4xx` codes may also pass through unchanged. For deployment and
recovery, use [Disposable beta cluster](beta-cluster.md); for claim quotas
and faucet responses, see [Hosted faucet](../platform/faucet.md).

[Home](../index.md)
