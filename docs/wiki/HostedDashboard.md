# Hosted dashboard

`layerx-dashboard` is the session-authenticated developer read surface
for keys, quota, the gateway request log, webhook delivery health, and
receipt-backed test payments
(`services/platform/hosted/dashboard/src/main.rs`;
`services/platform/hosted/dashboard/src/model.rs`). The crate is
`layerx-platform-dashboard`. The companion Next.js package
`@layerx/developer-dashboard` provides the browser UI; its Fly configuration
is checked in separately.

The dashboard API serves reads. Every API path except `GET /healthz` requires a
developer session. Protocol facts that the dashboard renders carry the
verification level the human plane displays; settlement is claimed only
where a verified LayerX receipt stands behind the fact
(`services/platform/hosted/dashboard/src/model.rs`).

Webhook registration and delivery live on
[Hosted webhooks](HostedWebhooks.md). Gateway keys and JSON-RPC live on
[Hosted gateway](HostedGateway.md) and [Public JSON-RPC](PublicRpc.md).

---

## Deployment

The browser UI, dashboard API and webhooks use separate services:

- The browser UI's Fly template names app `paxeer-dashboard-web`, uses
  `docker/platform-dashboard-web/Dockerfile`, and serves port `3000`
  (`services/platform/hosted/dashboard/web/fly.toml`). Its
  `next.config.ts` rewrites `/v1/dashboard/*` to `DASHBOARD_API_ORIGIN`,
  which is required in production. Development defaults to loopback HTTP
  port `9445`.
- The dashboard API image uses `docker/platform-dashboard/Dockerfile`.
  `services/platform/hosted/dashboard/railway.env.example` selects
  `PORT=9445`, `LAYERX_DASHBOARD_LISTEN=[::]:9445` and
  `LAYERX_DASHBOARD_LISTENER=plain`. The API and `GET /healthz` share
  that listener. Webhook state and gateway state use separately configured
  Redis TLS stores; developer sessions use the identity service.
- Webhooks has separate public and ingress roles, configured through
  `services/platform/hosted/webhooks/env-init.sh` and
  `services/platform/hosted/webhooks/railway.env.example`.
  See [Hosted webhooks](HostedWebhooks.md).

The dashboard image starts `layerx-env-files` before `layerx-dashboard`
as uid `65532`. The file variables in the dashboard env example hold
base64 values decoded under `/run/layerx`, using
`docker/platform-dashboard/files.tsv` and
`LAYERX_FILES_OWNER=65532:65532`:

| File variables | Paths under `/run/layerx` |
| --- | --- |
| `DASHBOARD_CLIENT_P12`, `DASHBOARD_CLIENT_PASSWORD` | `client/identity.p12`, `client/password` |
| `DASHBOARD_CLIENT_CA_DER` | `ca/internal.der` |
| `DASHBOARD_WEBHOOKS_REDIS_USERNAME`, `DASHBOARD_WEBHOOKS_REDIS_PASSWORD` | `redis/username`, `redis/password` |
| `DASHBOARD_GATEWAY_REDIS_USERNAME`, `DASHBOARD_GATEWAY_REDIS_PASSWORD` | `dashboard-redis/username`, `dashboard-redis/password` |
| `DASHBOARD_IDENTITY_TOKEN` | `tokens/identity` |

The API listens on `LAYERX_DASHBOARD_LISTEN`, default
`0.0.0.0:9445` (`services/platform/hosted/dashboard/src/main.rs`). At most 256
connections are live. The `tls` listener uses
`LAYERX_DASHBOARD_TLS_CERT_DER` and `LAYERX_DASHBOARD_TLS_KEY_DER`.

---

## Authentication

`GET /healthz` is unauthenticated and returns
`{"ready": <bool>}` with HTTP 200 when ready and 503 otherwise.

Every `/v1/dashboard/*` route calls the same developer identity
introspector as webhooks: `Authorization: Bearer` or cookie
`__Host-layerx-session` (`services/platform/hosted/dashboard/src/main.rs`).
Failed authentication is `401 session_required`. The dashboard API is GET-only.

The browser UI presents sign-in after a 401. A developer creates and keeps a
non-exportable Ed25519 private key in this browser's IndexedDB, uses the key
already held here, or enters a session token. The public key has a copy control.
When key sign-in is configured, one primary action creates and uses a key when
none is held, or signs in with the held key. A browser without Ed25519 offers
the token path. Without the runtime tenant or gateway origin, the view says that
key sign-in is not configured while the session token path remains available.

On the browser UI host, `POST /session/registration` forwards a registration
proof to `px_register`. `POST /session` forwards a fresh key proof to
`px_openSession`, or checks a supplied token against the dashboard API overview
with Bearer authorization. The gateway origin comes from the server runtime
variable `DASHBOARD_GATEWAY_ORIGIN`. The tenant comes from the server runtime
variable `DASHBOARD_GATEWAY_TENANT` and must contain 1 to 95 lower case letters,
digits, dots, underscores or hyphens. The tenant is read for each request, so it
is not compiled into the build. Registration bindings and session digests include
that runtime tenant. Invalid or absent tenant values are treated as unset. The API origin matches the build-time `DASHBOARD_API_ORIGIN` rewrite.
Without the gateway origin or a valid tenant, key registration and proof sign-in
return 503 with `sign_in_not_configured`; token sign-in remains available. The
page explains this refusal.

A successful sign-in returns 204 and sets `__Host-layerx-session` with `Secure`,
`HttpOnly`, `SameSite=Strict`, `Path=/` and no Domain. Its `Max-Age` ends at the
proof session's `expires_at`, or twelve hours after a token is accepted.
`DELETE /session` returns 204 and clears the cookie with the same attributes
and `Max-Age=0`. The shell's sign-out control returns to sign-in.

These web routes require an Origin matching either the parsed HTTPS origin of
the Host header, including its port, or the origin of the request URL for direct
access. An absent, multi-value or malformed Host header supplies no HTTPS origin.
Missing or non-matching Origins return 403 with `origin_refused` before the body
is read.

Requests require `Content-Type: application/json` with a body bound of 8192 bytes;
upstream calls have a five-second time limit. Response bodies and error text
contain no session token.

---

## Routes

All listed routes are `GET`. Unknown paths and non-GET methods on
owned routes are `404 not_found`.

| Path | Result type | Fields |
| --- | --- | --- |
| `/v1/dashboard/overview` | `Overview` | `principal`, `generated_at`, `usage`, `keys`, `requests`, `recent_requests`, `deliveries`, `endpoints`, `dead_letters`, `payments` |
| `/v1/dashboard/keys` | `KeyView[]` | per key: `key_id`, `principal`, `disabled`, `requests_per_window`, `window_seconds`, `used_in_window`, `remaining_in_window`, `window_started_at`, `window_resets_at`, `window_lapsed`, `utilisation_per_mille` |
| `/v1/dashboard/usage` | `UsageSummary` | `keys`, `live_keys`, `disabled_keys`, `requests_allowed`, `requests_used`, `requests_remaining`, `utilisation_per_mille` |
| `/v1/dashboard/requests` | `RequestRecord[]` | `at`, `operation` (optional), `operation_digest`, `outcome`, `verification` |
| `/v1/dashboard/webhooks` | endpoint health list | webhook `EndpointHealth` records |
| `/v1/dashboard/webhook-deliveries` | delivery records | optional query `endpoint=`; `limit` |
| `/v1/dashboard/webhook-dead-letters` | dead-letter records | `limit` |
| `/v1/dashboard/test-payments` | `PaymentView[]` | `event`, `subject`, `subject_sequence`, `occurred_at`, `amount`, `asset`, `verification`, `settlement_verification`, `receipt_digest`, `settled`, `facts` |
| `/v1/dashboard/receipts/{activity}` | `ReceiptView` | `activity_id`, `event`, `receipt_digest`, `verification`, `settled` |

`limit` defaults to 50 and is clamped to `1..=200`
(`services/platform/hosted/dashboard/src/main.rs`).
`webhook-deliveries` accepts optional `endpoint` as an `EndpointId`.

`RequestOutcome` wire words are `pending`, `completed`,
`rate-limited`, and `refused`. A request line never presents above
`unverified` because the gateway audit trail records digests rather
than receipts (`services/platform/hosted/dashboard/src/model.rs`).

The gateway writes the principal request index in the same script as each
common audit entry and recovers retained entries in the background at startup.
`DIGEST` is the principal's audit digest, before the first colon in the event.

| Store key | Writer and dashboard reading |
| --- | --- |
| `gateway:principal:DIGEST:audit` | Gateway stream containing the common audit IDs and fields for this principal |
| `gateway:principal:DIGEST:audit:version` | Gateway sets `1`; the dashboard accepts version `1` and refuses other values or entries without a version |

A principal with neither a version nor index entries has an empty request list.
Its overview has empty key and recent-request lists when it has no keys.

Receipt lookups read `webhooks:principal:DIGEST:activities` and its
`webhooks:principal:DIGEST:activities:version` key. Webhooks writes both as one
index update; version `1` is the supported format. A principal with neither a
version nor the requested activity field receives `receipt_not_found`, while an
activity field without version metadata is refused as unavailable.

A payment's `settled` flag is true only when the `state` fact equals
`settled`, that fact is at least `receipt-verified`, and a receipt
digest stands behind that exact fact.

---

## Refusals

| HTTP | Code | When |
| --- | --- | --- |
| 400 | `invalid_request` | Malformed request |
| 401 | `session_required` | Missing or inactive session |
| 403 | `principal_refused` | Gateway store refused the principal |
| 404 | `receipt_not_found` | Unknown activity on `/receipts/{activity}` |
| 404 | `not_found` | Unknown path or non-GET |
| 503 | `state_unavailable` | Corrupt store, webhook or I/O failure |
| 503 | `encoding_failed` | Response JSON encoding failed |

---

## Source contract

- `services/platform/hosted/dashboard/src/main.rs` owns TLS, routing, paging, and session checks.
- `services/platform/hosted/dashboard/src/model.rs` owns the view models above.
- `services/platform/hosted/dashboard/src/service.rs` reads gateway Redis (keys, quota, audit) and the webhooks Redis reader.

[Home](Home.md)
