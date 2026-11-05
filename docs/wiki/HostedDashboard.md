# Hosted dashboard

`layerx-dashboard` is the session-authenticated developer read surface
for keys, quota, the gateway request log, webhook delivery health, and
receipt-backed test payments
(`services/platform/hosted/dashboard/src/main.rs`;
`services/platform/hosted/dashboard/src/model.rs`). The crate is
`layerx-platform-dashboard`. A companion Next.js process,
`layerx-dashboard-web`, is the browser UI, served as its own Fly app.

It does not accept writes. Every path except `GET /healthz` requires a
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

- `paxeer-dashboard-web` remains a Fly app
  (`services/platform/hosted/dashboard/web/fly.toml`), built from
  `docker/platform-dashboard-web/Dockerfile` and served on port `3000`.
  Its `/v1/dashboard/*` rewrite targets
  `http://dashboard.railway.internal:9445`
  (`services/platform/hosted/dashboard/web/next.config.ts`).
- Railway service `dashboard` is built from
  `docker/platform-dashboard/Dockerfile`. Its configuration is
  `services/platform/hosted/dashboard/railway.env.example`, with
  `PORT=9445`, `LAYERX_DASHBOARD_LISTEN=[::]:9445` and
  `LAYERX_DASHBOARD_LISTENER=plain`. The API and `GET /healthz` share
  that plain HTTP listener. It reads webhook state from
  `rediss://redis-internal.railway.internal:6379`, gateway keys, quota
  and request logs from `rediss://redis-router.railway.internal:6379`,
  and developer sessions from `https://identity.railway.internal:9443`.
- Railway services `webhooks-public` and `webhooks-ingress` use the
  `env-init-prebuilt` or `env-init` target of
  `docker/platform-webhooks/Dockerfile`, with settings from
  `services/platform/hosted/webhooks/railway.env.example`.
  The `layerx-env-files` entrypoint runs
  `services/platform/hosted/webhooks/env-init.sh` to select the role's
  listener, health listener and credential paths from `LAYERX_ROLE`,
  and its instance id from the replica id or hostname; see
  [Hosted webhooks](HostedWebhooks.md).

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
Failed authentication is `401 session_required`. Mutating cookie
requests are not served; the dashboard is GET-only.

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
