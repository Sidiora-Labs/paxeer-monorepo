# Hosted identity

`layerx-identity` is the hosted principal and session service, built from crate
`layerx-platform-identity` in `services/platform/hosted/identity/`. It stores
principals and sessions on disk and serves TLS HTTP. Nine distinct service
credentials control its routes; a service credential and a user's session token
have different purposes.

Paxeer mainnet-beta is one BFT network, with EVM chain id `125` (`0x7d`).
Tenant domains select database namespaces within that network. Identity binds
a principal and its sessions to a tenant; it does not create another chain.
Network receipt commitment and native policy proofs are covered by
[Hosted authority](hosted-authority.md). This page covers hosted login and
provisioning, whose source contract is `src/main.rs` and `src/store.rs`.

---

## Callers

`LAYERX_IDENTITY_SERVICE_TOKENS_DIR` must contain every filename below.
`service_tokens()` requires each secret to be at least 16 bytes and distinct
from every other service secret. Missing files, duplicate secrets, and unknown
filenames refuse startup. Kubernetes projection entries starting with `..` are
ignored. The credential is named `testnet`, including when used by testnet
control.

| Token filename | Permitted operations |
| --- | --- |
| `gateway` | Session introspection and authenticated `GET /internal/readyz` |
| `webhooks` | Session introspection with CSRF response |
| `dashboard` | Session introspection with CSRF response |
| `faucet` | Session introspection with subject response |
| `testnet` | Session introspection with subject response |
| `ramp` | Session introspection with required audience |
| `provisioning` | Principal creation/update, session mint/revoke, publication-key binding, signup/verification and starter-credit routes |
| `registrar` | Principal creation/update only |
| `registry` | Publication principal resolution through `GET /internal/v1/principal` only |

Only the six introspecting service identities may call
`POST /v1/sessions/introspect` or its alias `POST /v1/introspect`.
`registrar` can create a principal but cannot mint or revoke its sessions.
`registry` cannot introspect or provision. Public health routes need no service
credential; other routes require an exact `Authorization: Bearer` match.

Bring-up in `services/platform/hosted/tests/beta-cluster.sh` creates the complete
nine-file directory: it copies gateway, registry, webhooks and faucet credentials
and generates dashboard, testnet, ramp, provisioning and registrar credentials.
Keep the mounted service token matched to its caller; a required but unused
credential is still required at startup.

## Credential and session model

A principal has `tenant`, `sub`, `allowed_signer_public_keys`, optional `account`,
and `audiences`:

| Field | Validation |
| --- | --- |
| `tenant` | Required, 1–128 bytes of lowercase ASCII letters, digits, `-`, `_`, `.` |
| `sub` | Required, 1–128 bytes using the tenant alphabet plus `:` |
| `allowed_signer_public_keys` | Required list of at most 128 unique lowercase 64-hex keys; an empty list is accepted |
| `account` | Optional nonempty identifier of at most 512 bytes |
| `audiences` | Defaults to an empty list; at most 32 unique nonempty identifiers, each at most 128 bytes |

Account and audience identifiers accept ASCII letters of either case, digits,
`-`, `_`, `.`, and `:`. Unknown request fields are rejected.

The store keys principals by tenant and subject, and keeps a durable
subject-to-tenant binding. A subject cannot be claimed by a second tenant:
`POST /v1/principals` returns `409 subject_tenant_conflict`. Updating an existing
principal within its tenant is allowed. Existing sessions read the current
principal's signer keys and grants during introspection.

A session token is `ses_{id}.{secret}`: the id is 16 random bytes represented as
32 lowercase hex characters, and the secret is 32 random bytes represented as
64 lowercase hex characters. The store records SHA-256 of the secret, not the
secret. The 32-byte random CSRF value is hex encoded, hashed and sealed for
storage; introspection verifies both the session digest and opened CSRF digest.

Sessions store their tenant explicitly. Minting accepts an optional `tenant`:
if supplied it must match the stored subject binding, otherwise the request
returns `404 principal_not_found`. If omitted, the service resolves the tenant
from that binding. Introspection retrieves the principal using the stored
session tenant and subject.

The default TTL is 86400 seconds. `ttl_seconds` may override it with a value in
`1..=2592000`. Each tenant/principal can have at most 4096 live unrevoked sessions
and 16384 retained session records, including expired and revoked history.
Expiry releases the live-session allowance, including after restart, but does
not remove retained history. Revocation is by session id; repeating it returns
the original `revoked_at`.

Unknown, malformed, digest-mismatched, revoked or expired nonempty tokens
introspect as inactive. An empty or over-4096-byte token is `400 invalid_argument`.
Storage or CSRF integrity failure returns `503 store_unavailable`.

---

## Request and response contracts

TLS uses `LAYERX_IDENTITY_TLS_CERT_DER` and
`LAYERX_IDENTITY_TLS_KEY_DER`. Setting `LAYERX_IDENTITY_CLIENT_CA_DER` requires
client certificates trusted by that CA. Without it, the TLS server does not
require a client certificate; the checked-in Deployment leaves it unset.
Service-bearer authorization still applies.

The parser requires HTTP/1.1 and `Host`, rejects query strings, duplicate headers
and transfer encodings, and bounds the whole request to 16 KiB. I/O timeout is
8 seconds; both listeners share a 128-connection limit, with excess connections
dropped. JSON routes require exactly `Content-Type: application/json`.
Responses use `application/json`, `Cache-Control: no-store`, and
`Connection: close`. Successful operations return HTTP 200.

Business routes reject `forwarded`, `x-forwarded-for`, `x-real-ip`,
`x-layerx-client-ip`, and `x-layerx-principal` before service authentication.
TLS `/livez` and `/readyz` are handled before that header check. Request headers
and bodies are zeroized on drop.

| Method and path | Request | Success response fields |
| --- | --- | --- |
| `GET /livez` | No authentication | `status: "live"`, `service: "identity"` |
| `GET /readyz` | No authentication | `status: "ready"`, `service: "identity"`, after a writable-store probe |
| `GET /internal/readyz` | Gateway bearer, empty body | Same readiness response |
| `POST /v1/principals` | Provisioning or registrar bearer; principal JSON | `tenant`, `sub`, `allowed_signer_public_keys`, `account`, `audiences` |
| `POST /v1/sessions` | Provisioning bearer; `sub`, optional `tenant`, optional `ttl_seconds` | `session_id`, `tenant`, `sub`, `token`, `csrf_token`, `expires_at` |
| `DELETE /v1/sessions/{id}` | Provisioning bearer; 32 lowercase hex id | `session_id`, `revoked: true`, `revoked_at` |
| `POST /v1/sessions/introspect` | Introspecting service bearer; `token`, plus `audience` for ramp | Caller-specific shape below |
| `POST /v1/introspect` | Same as above | Same as above |

Timestamps and TTLs use seconds, with timestamps measured from the Unix epoch.

### Tenant-bearing provisioning

With a provisioning or registrar bearer, post this body to `/v1/principals`:

```json
{
  "tenant": "example",
  "sub": "did:example:principal",
  "allowed_signer_public_keys": [],
  "account": "customer-example",
  "audiences": ["payments"]
}
```

The empty signer list is valid but grants no gateway signing keys. For signed
activity, supply the actual signer's 32-byte lowercase hex public key.
The response echoes all five principal fields, including `tenant`. Then use a
provisioning bearer on `/v1/sessions`:

```json
{"tenant":"example","sub":"did:example:principal","ttl_seconds":3600}
```

Retain the returned `session_id`, `tenant`, and `sub` alongside the session token
in the caller's secure session storage. The token and CSRF value are returned
at mint time. To revoke that session after a service restart, use the same
provisioning credential on `DELETE /v1/sessions/{session_id}`; do not create a
new principal under another tenant to recover it.

### Introspection shapes

The service bearer determines the response shape. Send the user's session token
in the JSON body, not as the service bearer.

| Caller | Response fields |
| --- | --- |
| Gateway | `active`, `sub`, `allowed_signer_public_keys` |
| Webhooks, dashboard | `active`, `sub`, `csrf_token` |
| Faucet, testnet | `active`, `sub` |
| Ramp | `active`, `principal_id`, `account`, `audience`, `expires_at` |

These introspection shapes currently omit `tenant`; principal and session mint
responses include it. The ramp field is `principal_id` in the actual producer
`RampShape`, with the stored subject as its value. Ramp requires a valid
`audience` in the request, plus a matching stored audience grant and an account.
A missing or invalid audience is `400 audience_required`. Supplying `audience`
for another service is `400 invalid_argument`.

Inactive responses keep the same shape with empty identity strings and key
lists. Ramp echoes the requested audience and sets `expires_at` to zero.

### Registry publication identity

`POST /v1/publication-keys` is provisioning-only and takes `sub` and `revoked`
with a printable `LayerX-Key` header of 1–4096 bytes. The subject must already
exist. The store persists the key's digest and subject binding; refusal to bind
is `409 publication_key_binding_refused`.

Registry uses its own bearer and the same `LayerX-Key` on
`GET /internal/v1/principal` with an empty body. Success is
`{"ok":true,"result":{"principal_digest":...}}`, where the digest is SHA-256
of the bound subject. Missing/revoked bindings return `404 principal_not_found`.
This is the hosted identity resolver; the human service's similarly named route
is a separate contract.

### Signup and starter credits

Provisioning also owns `POST /v1/signup`, `/v1/signup/verify`,
`/v1/starter-credits`, and `/v1/starter-credits/settle`.
Signup takes `email`, `client_ip`, `tenant`, `sub`, and `signer_public_key`;
verification takes `token` and `sub` and issues a session. Starter-credit
requests take `sub`; settlement additionally requires `grant_id`.
The handlers, rate limits, verification rules and typed refusals are in
`src/main.rs` and `src/signup.rs`.

---

## Storage and durability

`LAYERX_IDENTITY_STATE_DIR` contains `snapshot.json`, `journal.log`, and
`ready.marker`; signup audit events use `audit.log`. `Store::open` validates
snapshot tenant/subject and session bindings, replays the journal, advances its
generation, durably writes a new snapshot, then truncates and syncs the journal.
Generation-aware replay avoids applying records already included in a snapshot.
A torn trailing record without a newline is discarded; malformed complete
records or inconsistent state refuse startup.

Mutations append bounded JSON journal records and call `sync_all` before
updating memory. A failed journal write makes the store unavailable until
restart. Replacing the state directory or journal inode also refuses subsequent
operations. Readiness checks availability and writes `ready.marker` using a
temporary file, file sync, rename and directory sync.

`src/seal.rs` derives the CSRF encryption/authentication keys from
`LAYERX_IDENTITY_STORE_KEY_FILE`. Preserve this secret and the complete state
directory across restarts. Replacing the key prevents opening existing CSRF
values. Session secrets and CSRF plaintext are absent from the session snapshot
and journal. Signup verification messages have a separate outbox containing
verification tokens; protect that directory as well.

Recovery should reopen the same durable store with the same seal key and token
credentials, then check readiness, introspect the saved session token, or revoke
the saved session id.
Tenant bindings and revocation timestamps survive journal replay and snapshot
compaction; restart does not release a subject for reassignment to another tenant.

## Typed refusals

Refusals are `{"error":{"code":...,"retry":"never"}}`, or `retry: "after"`
with `retry_after_seconds` and a `Retry-After` header.

| HTTP | Code | Condition |
| --- | --- | --- |
| 400 | `invalid_request` | HTTP framing/parser refusal |
| 400 | `untrusted_identity_header` | Forbidden forwarded identity header on a business route |
| 400 | `content_type_required` | JSON content type missing or different |
| 400 | `invalid_argument` | Unknown field, invalid tenant/subject/key/TTL or other request bound |
| 400 | `audience_required` | Ramp introspection lacks a valid audience |
| 400 | `publication_key_required` | Publication request lacks a valid `LayerX-Key` |
| 401 | `service_token_required` | Missing, empty, oversized or unmatched service bearer |
| 403 | `service_not_permitted` | Authenticated service lacks the route capability |
| 404 | `not_found` | Unknown method/path |
| 404 | `principal_not_found` | Unknown subject, session tenant mismatch, or unresolved publication key |
| 404 | `session_not_found` | Revoke id malformed or absent |
| 409 | `subject_tenant_conflict` | Subject already belongs to another tenant |
| 409 | `publication_key_binding_refused` | Publication-key binding rejected |
| 429 | `session_bound_reached` | 4096 live sessions; retry after 60 seconds |
| 429 | `session_history_capacity_reached` | 16384 retained sessions; no automatic retry interval |
| 503 | `store_unavailable` | Store integrity, persistence, CSRF opening or readiness failure; retry after 5 seconds |
| 503 | `clock_unavailable` | Clock precedes Unix epoch; retry after 5 seconds |
| 503 | `encoding_failed` | Response serialization failure; retry after 5 seconds |
| 503 | `entropy_unavailable` | Randomness or CSRF sealing failed; retry after 5 seconds |

Signup adds email-domain, rate-limit, duplicate-registration, verification and
starter-credit refusal codes defined by `signup::Refused::status`.

## Readiness

TLS `GET /livez` does not access the store. TLS `GET /readyz` and the
unauthenticated plain HTTP `GET /healthz` on the health listener perform the
writable-store probe. `GET /internal/readyz` performs it with a gateway bearer.
Startup opens and probes the store before binding both listeners.

The Deployment probes HTTPS `/readyz` every 5 seconds with failure threshold 3,
and HTTPS `/livez` every 15 seconds. Healthy session storage is separate from
network receipt commitment; see [Hosted authority](hosted-authority.md).

## Config keys

| Key | Behavior |
| --- | --- |
| `LAYERX_IDENTITY_LISTEN` | TLS address; default `[::]:9443`. A configured `0.0.0.0` is converted to the IPv6 unspecified address |
| `LAYERX_IDENTITY_HEALTH_ADDR` | Plain health address; default `[::]:8080` |
| `LAYERX_IDENTITY_TLS_CERT_DER` | Required server certificate file, DER |
| `LAYERX_IDENTITY_TLS_KEY_DER` | Required server private key file, PKCS#8 DER |
| `LAYERX_IDENTITY_CLIENT_CA_DER` | Optional CA file enabling required TLS client authentication |
| `LAYERX_IDENTITY_STATE_DIR` | Required durable state directory |
| `LAYERX_IDENTITY_SERVICE_TOKENS_DIR` | Required complete directory of nine distinct service credentials |
| `LAYERX_IDENTITY_STORE_KEY_FILE` | Required CSRF seal secret file |
| `LAYERX_IDENTITY_SESSION_TTL_SECONDS` | Default `86400`; allowed `1..=2592000` |
| `LAYERX_IDENTITY_SIGNUP_PER_IP_PER_HOUR` | Default `5` |
| `LAYERX_IDENTITY_SIGNUP_PER_EMAIL_PER_HOUR` | Default `3` |
| `LAYERX_IDENTITY_SIGNUP_DAILY_CAP` | Default `1000` |
| `LAYERX_IDENTITY_SIGNUP_TOKEN_TTL_SECONDS` | Default `86400`; allowed `1..=2592000` |
| `LAYERX_IDENTITY_DISPOSABLE_DOMAINS_FILE` | Optional line-separated disposable email domain list |
| `LAYERX_IDENTITY_SIGNUP_OUTBOX_DIR` | Defaults to `signup-outbox` under the state directory |
| `LAYERX_IDENTITY_SMTP_URL` | Optional mail transport; enables periodic outbox draining |
| `LAYERX_IDENTITY_SMTP_FROM` | Required sender when SMTP URL is set |

Secret-file reads strip trailing CR/LF and reject empty values, values above
4096 bytes, or ASCII control characters. Service files additionally require
16 bytes minimum. `Service::ALL` and `config()` are the authoritative list and
configuration loader in `src/main.rs`.

## Cluster apply and admitted edges

`services/platform/hosted/identity/deployment.yaml` runs one replica with strategy
`Recreate`, user/group `4020`, image
`ghcr.io/sidiora-labs/layerx-identity:0.1.0`, and Service port `9443`.
`docker/platform-identity/Dockerfile` installs `/usr/local/bin/layerx-identity`.
The `layerx-identity-state` PVC is `ReadWriteOnce`, 5Gi, mounted at
`/var/lib/layerx/identity`. TLS, service-token and store-key material comes from
Secrets `layerx-identity-server-tls`, `layerx-identity-service-tokens`, and
`layerx-identity-store-key`.

The checked-in NetworkPolicy admits TCP 9443 from testnet control, faucet,
gateway, and the developer namespace. Egress admits DNS on port 53 and TCP
465 to pods labelled `app: layerx-identity-smtp`. SMTP configuration comes from
Secret `layerx-identity-smtp` key `url` and ConfigMap `layerx-identity-mail` key
`from`; the SMTP Service uses port 465. The identity health listener's port is
not exposed by the identity Service.

Principal and session creation return HTTP 200. Provisioning supplies the
tenant and uses the provisioning credential.

## Tests

Maintained TLS integration tests are in
`services/platform/hosted/identity/tests/service.rs`. They cover service-token
permissions, registrar-only principal creation, a distinct registrar credential
at boot, caller-specific introspection, expiry/revocation, durable restart,
tenant validation/conflicts, tenant-bound sessions, registry resolution across
restart, health and unavailable storage. Store tests cover journal replay,
compaction, replaced storage and failed writes; seal tests cover fresh nonces,
authentication and tampering.

`make platform-test-identity` runs the crate through
`tools/runtime/run-with-clock.sh`. `platform-test-trusted-boundary` includes it;
`platform-hosted-topology-check` checks hosted deployment topology.

[Hosted authority](hosted-authority.md) · [Home](../index.md)
