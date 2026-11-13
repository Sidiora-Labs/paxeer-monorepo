# Hosted identity

`layerx-identity`, from crate `layerx-platform-identity`, is the hosted
principal and session service. It stores principals, sessions, publication-key
bindings and signup state on disk and serves authenticated HTTP over TLS.
Its contract is implemented in
[`src/main.rs`](../../services/platform/hosted/identity/src/main.rs) and
[`src/store.rs`](../../services/platform/hosted/identity/src/store.rs).

Paxeer is one BFT network with EVM chain id **125** (`0x7d`); its kernel
proposes blocks as part of that network. Hosted identity manages application
credentials, while [hosted authority](HostedAuthority.md) verifies receipt and
committed-state evidence. A session grants no block finality. Sub-chains are
tenant domains in the database, and identity binds every principal and session
to a tenant namespace.

## Callers

`LAYERX_IDENTITY_SERVICE_TOKENS_DIR` must contain **nine distinct service token
files**, with exactly these names. Tokens must be 16–4096 bytes after trimming
trailing CR/LF, nonempty and free of ASCII control characters. Missing files,
duplicate tokens, or unknown directory entries refuse startup. Kubernetes
projection entries starting with `..` are ignored.

| Service/token filename | Introspect sessions | Create/update principals | Mint/revoke sessions | Other admitted routes |
| --- | --- | --- | --- | --- |
| `gateway` | Yes | No | No | `GET /internal/readyz` |
| `webhooks` | Yes | No | No | — |
| `dashboard` | Yes | No | No | — |
| `faucet` | Yes | No | No | — |
| `testnet` | Yes | No | No | — |
| `ramp` | Yes, with audience | No | No | — |
| `provisioning` | No | Yes | Yes | Publication-key binding, signup/verification and starter-credit routes |
| `registrar` | No | Yes | No | — |
| `registry` | No | No | No | `GET /internal/v1/principal` resolves a publication key |

The control component's credential filename is **`testnet`**. A file called
`control` is rejected. Registrar can create principals but cannot mint sessions;
registry's resolver credential cannot introspect or provision principals.
All nine files are required even if a caller is not deployed.

Service credentials are supplied through
`LAYERX_IDENTITY_SERVICE_TOKENS_DIR`; keep the complete nine-file set distinct
from end-user sessions.

## Credential and session model

A principal is `{tenant, sub, allowed_signer_public_keys, account, audiences}`.
Principal provisioning requires `tenant`, `sub`, and `allowed_signer_public_keys`.
`account` defaults to null and `audiences` defaults to an empty list.

- `sub`: 1–128 lowercase ASCII letters, digits, `-`, `_`, `.`, or `:`.
- `tenant`: the same bound and alphabet, excluding `:`.
- Signer keys: at most 128 unique 32-byte lowercase hex strings, without `0x`;
  an empty list is accepted.
- `account`: optional nonempty identifier of at most 512 ASCII letters, digits,
  `-`, `_`, `.`, or `:`. Each audience uses that alphabet with a 128-byte bound;
  there are at most 32 unique audiences.

Principals are stored by tenant, then subject. A subject is globally bound to
one tenant: provisioning that subject in another tenant returns
**409 `subject_tenant_conflict`**. Updating a principal within its existing
tenant replaces its keys, account and audiences. Introspection reads the
current principal rather than a stale copy of those grants.

A session is `ses_{id}.{secret}`: `id` is 16 random bytes as 32 lowercase hex
characters, and `secret` is 32 random bytes as 64 lowercase hex characters.
The store keeps the session id, tenant, principal subject, timestamps and
`sha256_hex(secret)`; it does not persist the bearer secret. CSRF is another
32 random bytes encoded as hex. The store keeps its digest and a sealed copy,
which is opened and digest-checked during lookup. Comparisons use constant-time
digest checks.

Session minting accepts `sub`, optional `tenant`, and optional `ttl_seconds`.
When tenant is omitted the service resolves the subject's bound tenant;
a supplied tenant must match. Unknown subjects and tenant mismatches both
return **404 `principal_not_found`**. The response always identifies the tenant.
Default TTL is 86400 seconds; accepted TTLs are 1–2592000 seconds.
Limits per tenant/principal are 4096 live unrevoked sessions and 16384 retained
session records, including expired/revoked history. A second revoke returns
the original `revoked_at`; revocation uses the global session id, with the
session's tenant binding retained in storage.

Unknown, malformed, wrong-secret, revoked or expired nonempty session tokens
produce an inactive introspection shape. Empty or oversized token inputs are
400 `invalid_argument`. Unavailable/corrupt storage produces a typed error,
not an inactive success.

## Request and response contracts

Service credentials go in `Authorization: Bearer ...`; the end-user session
token goes in the JSON body of introspection. JSON requests require the exact
`Content-Type: application/json` and reject unknown fields.

| Method and path | Caller/input | Success body |
| --- | --- | --- |
| `GET /livez` | No service token | `{"status":"live","service":"identity"}` |
| `GET /readyz` | No service token | `{"status":"ready","service":"identity"}` after a durable writable-store probe |
| `GET /internal/readyz` | Gateway, empty body | Same readiness handler |
| `POST /v1/principals` | Provisioning or registrar; principal model above | `tenant`, `sub`, `allowed_signer_public_keys`, `account`, `audiences` |
| `POST /v1/sessions` | Provisioning; `sub`, optional `tenant` and `ttl_seconds` | `session_id`, `tenant`, `sub`, `token`, `csrf_token`, `expires_at` |
| `DELETE /v1/sessions/{id}` | Provisioning; 32-hex id | `session_id`, `revoked: true`, `revoked_at` |
| `POST /v1/sessions/introspect` | One of the six introspection services; `token`, plus `audience` for ramp | Caller-specific shape below |
| `POST /v1/introspect` | Same caller/input as above | Same handler and shape |
| `GET /internal/v1/principal` | Registry bearer and `LayerX-Key` header; empty body | `{"ok":true,"result":{"principal_digest":"..."}}` |
| `POST /v1/publication-keys` | Provisioning bearer and `LayerX-Key`; body `{sub, revoked}` | `{"ok":true}` |

Successful JSON is HTTP **200**, including principal/session creation, with
`Cache-Control: no-store` and `Connection: close`. Expiry and revocation times
are Unix seconds. The registry resolver returns SHA-256 of the bound subject
as `principal_digest`; it does not return a session or signer-key list.
Publication keys must be 1–4096 graphic ASCII bytes. Binding conflicts return
409 `publication_key_binding_refused`; unknown/revoked keys resolve as
404 `principal_not_found`.

| Introspection caller | Exact response fields |
| --- | --- |
| `gateway` | `active`, `sub`, `allowed_signer_public_keys` |
| `webhooks`, `dashboard` | `active`, `sub`, `csrf_token` |
| `faucet`, `testnet` | `active`, `sub` |
| `ramp` | `active`, `principal_id`, `account`, `audience`, `expires_at` |

The ramp field is named **`principal_id`**, containing the principal's `sub`.
Ramp requires a valid audience and a stored account plus an exact audience
grant. Without those grants it returns `active: false`, empty `principal_id`
and `account`, the requested audience, and `expires_at: 0`. Other callers must
omit `audience`. These introspection shapes do not add a tenant field; tenant
is included in provisioning/session responses and used internally for lookup.

HTTP parsing requires HTTP/1.1 and `Host`, refuses query strings, duplicate
headers and `Transfer-Encoding`, and bounds request bytes to 16 KiB. I/O timeout
is eight seconds; at most 128 live connections are admitted across listeners.
Further connections are dropped. On protected routes, `forwarded`,
`x-forwarded-for`, `x-real-ip`, `x-layerx-client-ip`, and `x-layerx-principal`
are refused before service authentication. Request headers and bodies are
zeroized on drop.

### Tenant-aware provisioning and recovery

Send this body to `POST /v1/principals` with a provisioning or registrar bearer:

```json
{
  "tenant": "beta",
  "sub": "did:key:alpha",
  "allowed_signer_public_keys": [],
  "account": "account:alpha",
  "audiences": ["ramp"]
}
```

An empty key list is valid but grants no signer keys to the gateway. For a
signing principal, supply its actual 32-byte public keys as lowercase hex.
Mint with the provisioning bearer and body
`{"tenant":"beta","sub":"did:key:alpha","ttl_seconds":3600}`.
Retain the returned tenant and session id with the session token and CSRF
value. After a restart with the same state and seal key, introspect the token
through an admitted service, then revoke with
`DELETE /v1/sessions/{session_id}` as provisioning. Repeating revocation
returns the same timestamp. Minting with `tenant: "other"` returns 404;
trying to provision the same subject there returns 409 and leaves its
original tenant binding intact.

### Signup and starter credits

Provisioning also owns `POST /v1/signup`, `/v1/signup/verify`,
`/v1/starter-credits`, and `/v1/starter-credits/settle`. Signup accepts
`email`, `client_ip`, `tenant`, `sub`, and `signer_public_key`; it queues an
email verification token and returns `pending`, `tenant`, `sub`, and
`expires_at`, without returning that token. Verification takes `token` and
`sub`, checks the pending subject binding, and returns the session response.

Credit requests take `sub`; settlement additionally requires the 32-hex
`grant_id`. The response contains `sub`, `grant_id`, `asset`, `amount`,
`granted_at`, and `settled_at`. The service records the starter-credit grant
and settlement status; these HTTP responses are not network commit proofs.
See [`signup.rs`](../../services/platform/hosted/identity/src/signup.rs) for
verification, quota, duplicate-credit and grant-matching rules.

## Storage and durability

`LAYERX_IDENTITY_STATE_DIR` contains `snapshot.json`, `journal.log`,
`ready.marker`, and signup audit/outbox material. Opening validates the
snapshot's tenant/subject and session bindings, replays the journal, writes a
new generation snapshot, then truncates and syncs the journal. Generation
tracking prevents an already-snapshotted record from replaying twice after an
interrupted compaction. A future journal generation, malformed complete record,
cross-tenant subject claim, or session with no matching principal refuses open.
Only a torn final record without a newline is discarded. Records are bounded
to 64 KiB.

Mutations append and `sync_all` before changing in-memory state. A journal
write failure poisons later writes and readiness until restart. Replacing
the state-directory or journal inode also makes availability checks refuse.
Readiness writes a temporary `ready.marker`, syncs, renames, and syncs the
directory. `/livez` does not probe storage.

CSRF sealing uses `StoreKey` derived from `LAYERX_IDENTITY_STORE_KEY_FILE`:
labelled SHA-256 keys, a SHA-256 keystream and HMAC-SHA256 tag
([`seal.rs`](../../services/platform/hosted/identity/src/seal.rs)). Preserve
both state and the same seal secret across restart; restoring only the files
cannot make a session usable under an unrelated seal key. Token secrets and
CSRF plaintext stay out of the session snapshot/journal. Retained revocations,
tenant bindings and publication-key bindings survive compaction and restart.

## Typed refusals

Errors have `{"error":{"code":...,"retry":"never"|"after"}}`.
Retryable errors include `retry_after_seconds` and `Retry-After`.

| HTTP | Code | Condition |
| --- | --- | --- |
| 400 | `invalid_request` | Invalid HTTP framing. |
| 400 | `untrusted_identity_header` | Identity forwarding header on a protected route. |
| 400 | `content_type_required` | Missing exact JSON content type. |
| 400 | `invalid_argument` | Invalid/unknown JSON fields, subject, tenant, signer keys, grants, TTL or token bounds; audience supplied by a non-ramp caller. |
| 400 | `audience_required` | Ramp request lacks a valid audience. |
| 400 | `publication_key_required` | Missing or invalid `LayerX-Key`. |
| 401 | `service_token_required` | Missing, oversized or unmatched service bearer. |
| 403 | `service_not_permitted` | Authenticated caller lacks the route's capability. |
| 404 | `not_found` | Unknown method/path. |
| 404 | `principal_not_found` | Unknown subject, mint tenant mismatch, or unresolved publication key. |
| 404 | `session_not_found` | Unknown/non-hex revocation id. |
| 409 | `subject_tenant_conflict` | Subject is already bound to another tenant. |
| 409 | `publication_key_binding_refused` | Refused publication-key reassignment/revival. |
| 429 | `session_bound_reached` | 4096 live sessions per tenant/principal; retry after 60 seconds. |
| 429 | `session_history_capacity_reached` | 16384 retained sessions per tenant/principal; no automatic retry hint. |
| 503 | `store_unavailable` | Store lock, availability, durable write, seal-open/digest, or writable-probe failure; retry after 5 seconds. |
| 503 | `clock_unavailable` | System time before the Unix epoch; retry after 5 seconds. |
| 503 | `encoding_failed` | Response serialization failed; retry after 5 seconds. |
| 503 | `entropy_unavailable` | Randomness or session sealing failed; retry after 5 seconds. |

Signup adds typed verification, email/quota and starter-credit refusals from
`signup::Refused::status`, including `verification_not_found`,
`verification_expired`, `verification_required`, and
`starter_credit_grant_mismatch`.

## Readiness

The TLS listener exposes `/livez`, `/readyz` and gateway-only `/internal/readyz`.
A separate plain HTTP listener exposes **`GET /healthz`**, backed by the same
durable readiness check. Its default address is `[::]:8080`; the TLS default
is `[::]:9443`. Startup opens and probes the store before serving.
The Kubernetes template explicitly binds TLS to `0.0.0.0:9443` and probes HTTPS
`/readyz` every five seconds and `/livez` every fifteen seconds.
A successful readiness response proves local storage availability, not that
an external client possesses a valid session or service token.

## Config keys

| Key | Meaning/default |
| --- | --- |
| `LAYERX_IDENTITY_LISTEN` | TLS bind address; default `[::]:9443`. |
| `LAYERX_IDENTITY_HEALTH_ADDR` | Plain health bind address; default `[::]:8080`. |
| `LAYERX_IDENTITY_TLS_CERT_DER` | Required server certificate DER file. |
| `LAYERX_IDENTITY_TLS_KEY_DER` | Required PKCS#8 DER private-key file. |
| `LAYERX_IDENTITY_CLIENT_CA_DER` | Optional client CA configuration. When set, clients must present a certificate trusted by this CA. Unset means no TLS client authentication. Service bearers remain required. |
| `LAYERX_IDENTITY_STATE_DIR` | Required persistent state directory. |
| `LAYERX_IDENTITY_SERVICE_TOKENS_DIR` | Required complete directory of nine distinct tokens named in the callers table. |
| `LAYERX_IDENTITY_STORE_KEY_FILE` | Required CSRF seal-secret file. |
| `LAYERX_IDENTITY_SESSION_TTL_SECONDS` | Default session TTL, `86400`; valid range 1–2592000. |
| `LAYERX_IDENTITY_SIGNUP_PER_IP_PER_HOUR` | Default `5`. |
| `LAYERX_IDENTITY_SIGNUP_PER_EMAIL_PER_HOUR` | Default `3`. |
| `LAYERX_IDENTITY_SIGNUP_DAILY_CAP` | Default `1000`. |
| `LAYERX_IDENTITY_SIGNUP_TOKEN_TTL_SECONDS` | Default `86400`; valid range 1–2592000. |
| `LAYERX_IDENTITY_DISPOSABLE_DOMAINS_FILE` | Optional line-separated domain list. |
| `LAYERX_IDENTITY_SIGNUP_OUTBOX_DIR` | Default `signup-outbox` beneath the state directory. |
| `LAYERX_IDENTITY_SMTP_URL` | Optional SMTP configuration; enables the background outbox drain. |
| `LAYERX_IDENTITY_SMTP_FROM` | Required sender when SMTP URL is configured. |

Secret files trim trailing CR/LF, then reject empty values, more than 4096
bytes, or ASCII controls. Inbound TLS is rustls; configuring the client-CA
verifier requires client certificates. The checked-in Deployment does not
set a client CA. These TLS settings do not remove bearer authorization.

## Cluster apply and admitted edges

The [deployment](../../services/platform/hosted/identity/deployment.yaml) uses
image `ghcr.io/sidiora-labs/layerx-identity:0.1.0`, UID/GID 4020:4020, one
replica and `Recreate` strategy. The
[Dockerfile](../../docker/platform-identity/Dockerfile) also provides an
`env-init` image whose entrypoint runs `layerx-env-files` and `identity-env-init`.
The [initializer](../../services/platform/hosted/identity/env-init.sh) prepares
the volume as root, drops to UID/GID 4020:4020 and executes
`/usr/local/bin/layerx-identity`.

Service `layerx-identity` exposes TLS port 9443 in namespace `layerx-testnet`.
PVC `layerx-identity-state` is 5 GiB, ReadWriteOnce, mounted at
`/var/lib/layerx/identity`. Runtime Secrets are `layerx-identity-server-tls`,
`layerx-identity-service-tokens`, and `layerx-identity-store-key`.

The template's ingress policy admits TCP 9443 from testnet control, faucet and
gateway pods and from namespace `layerx-developer`; egress allows DNS on 53.
Network access and token permission are separate requirements: possessing a
`registrar` or `registry` token does not change that ingress policy.
Principal provisioning uses the tenant-bearing body shown above.

## Tests

[`tests/service.rs`](../../services/platform/hosted/identity/tests/service.rs)
exercises real TLS, all introspection shapes, unauthorized callers, registrar
principal creation without session minting, required distinct registrar tokens,
revocation, expiry, health, store failure, tenant binding and restart, retained
session capacity, and durable registry publication-key resolution.

Unit tests in `main.rs` cover exact token matching, capability separation,
strict session syntax, tenant validation and tenant-bearing response shapes.
Store tests cover snapshot/journal replay, tenant-index validation, torn tails,
idempotent revocation, failed writes and replaced inodes. `seal.rs` tests cover
key binding, fresh nonces, tamper refusal, HMAC and hex encoding.

## Make targets

The [platform Makefile](../../services/platform/Makefile.inc) defines
`platform-test-identity` for the identity crate and
`platform-test-trusted-boundary`, which depends on it.

[Hosted authority](HostedAuthority.md) · [Hosted registry](HostedRegistry.md) · [Home](Home.md)
