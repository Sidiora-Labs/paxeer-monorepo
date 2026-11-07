# Hosted internal

`layerx-platform-internal` provides `layerx-kms` and `layerx-event-source`.
The KMS creates sealed signing keys and signs through opaque handles. Event
sources retain immutable observations for journeys, approvals, payments and
programs. These are hosted application services around the merged Paxeer
network; the kernel is part of that network and proposes blocks. A committed
block provides finality. Event delivery does not add a settlement or consensus
stage.

Source paths below are relative to `services/platform/hosted/internal/` unless
fully qualified. Human signing and owner handover use a separate custody
provider contract: see [Human custody](HumanCustody.md). The browser lifecycle
is [Human journeys](HumanJourneys.md).

---

## Namespace and apply

`deployment.yaml` declares namespace `layerx-internal`, its default-deny
Ingress/Egress NetworkPolicy, Redis, KMS and four event-source Deployments.
The KMS and each source use one replica, `Recreate`, UID/GID/fsGroup `4020`,
a read-only container root filesystem and a 1Gi `ReadWriteOnce` state PVC.
Their Services expose TLS port `443` to container port `9443`; Kubernetes
probes use HTTPS `/readyz` and `/livez`.

The image `ghcr.io/sidiora-labs/layerx-internal:0.1.0` contains both binaries
(`docker/platform-internal/Dockerfile`). Kubernetes sets the executable
explicitly. The image's environment-initializing entrypoint instead uses
`LAYERX_ROLE=kms|journeys|payments|approvals|programs` and
`services/platform/hosted/internal/env-init.sh`, stores state under `/data`,
and drops to UID/GID `4020`. Its entrypoint is not directly `layerx-kms`.

The beta installation path includes these workloads:

- `services/platform/hosted/tests/beta-images.sh` lists `layerx-internal`,
  `layerx-human` and `layerx-human-web` in `IMAGE_NAMES`.
- `beta-cluster.sh` in that directory renders `internal.yaml`, `human.yaml`
  and `human-web.yaml`. `internal_apply` applies the internal manifest;
  `manifests_apply` applies Human; `human_web_apply` applies its web manifest.
- `topology-check.sh` includes the internal, Human and webhooks manifests in
  its default set.

`services/platform/hosted/human/deployment.yaml` declares the `layerx-human`
Service in `layerx-testnet`. It selects **`app=layerx-node`**, forwarding Service
port `9443` to the pod's named `human-https` port. Its policy allows journey
and approval adapters to the Human listener in the node pod. Do not infer the
backend pod selector from the Service name.

The internal aliases `component` and `authority` resolve through
`internal-component` and `internal-authority` Services selecting
`app=layerx-node`, with target ports `agent-tls` and `authority-tls`.
`identity` resolves to `internal-identity`, selecting `app=layerx-identity`.
The authority alias is a retained transport name for the node boundary; the
standalone batch receipt authority is retired. It is not an additional
finality service.

## TLS and HTTP

Both binaries require their configured client CA. The rustls listener allows
connections without a client certificate for probes; protected routes check
whether a verified certificate was presented and require the appropriate
Bearer token. KMS reports missing certificates as `401
client_certificate_required` and token mismatches as `401 unauthorized`.
Event-source protected routes report `401 unauthorized`.

The HTTP parser accepts one HTTP/1.1 request per connection, with a Host header,
no query string or transfer encoding, no duplicate headers, and an exact
`Content-Length` body. Headers plus body are bounded to 96 KiB, socket I/O to
8 seconds, and concurrent connections to 128 (`src/http.rs`). Parse failures
return `400 invalid_request`. Responses are JSON with `Cache-Control:
no-store` and `Connection: close`.

Internal refusals have this envelope, distinct from the Human `/v1` envelope:

```json
{"error":{"code":"source_unavailable","retry":"after","retry_after_seconds":5}}
```

Non-retriable refusals use `retry: "never"`. Bearer comparison is constant time.
Upstreams must be bare HTTPS DNS origins, with pinned CA trust, a 5-second
connect timeout, an 8-second I/O timeout and a 512 KiB response bound
(`src/tls.rs`). Client PKCS#12 identity is configurable for authenticated
upstream calls. Readiness/admission calls additionally impose a deadline.

## Who may call

KMS protected routes use `LAYERX_KMS_TOKEN_FILE` and a verified client
certificate. `/livez` and `/readyz` do not require either. Event sources separate
reader credentials (`LAYERX_EVENTS_TOKEN_FILE`) from producer credentials in
`LAYERX_EVENTS_PRODUCERS_FILE`; a producer token does not grant event reads.
Production observation and enrollment calls require producer mTLS and token.

NetworkPolicies admit webhooks and testnet control to the internal services.
Additional producer policies admit gateway pods to payments, program-registry
pods to programs, and node pods to journeys and approvals. Human adapter egress
to the node listener is declared in the Human manifest. KMS has no egress;
event adapters have DNS and upstream allowances (`deployment.yaml`,
`services/platform/hosted/human/deployment.yaml`). Policies are additive, so
inspect both producer and reader policies when diagnosing an edge.

Webhooks uses `<kms-origin>` and the four source Service origins, all supplied
by the operator over HTTPS. Its KMS token and per-source reader tokens come from
`LAYERX_WEBHOOKS_KMS_TOKEN_FILE` and
`LAYERX_WEBHOOKS_{JOURNEY|PAYMENT|APPROVAL|PROGRAM}_SOURCE_TOKEN_FILE`
(`services/platform/hosted/webhooks/deployment.yaml`).

---

## KMS request and response

Creation and signing accept exactly **`ed25519`** or **`secp256k1`**
(`src/kms.rs`). Create/sign request objects reject unknown fields.

| Method and path | Input | Success |
| --- | --- | --- |
| `GET /livez` | None | `200`, `{"alive":true}` |
| `GET /readyz` | None | `200` or `503`, `{"ready":true,"ed25519_non_exportable":true}` with the actual readiness boolean |
| `POST /v1/signing-keys` | Client certificate, Bearer, `Idempotency-Key`, JSON `algorithm` and `purpose` | `201` fresh or `200` replay; `key_id`, `handle`, `public_key` |
| `GET /v1/signing-keys/{key_id}` | Client certificate, Bearer | `200`, same public key body |
| `POST /v1/signatures` | Client certificate, Bearer, JSON `key_handle`, `algorithm`, `message` | `200`, base64 `signature`; secp256k1 also returns `recovery_id` |

| Algorithm | Public key encoding | Signing input and output |
| --- | --- | --- |
| `ed25519` | Standard base64 of 32 raw bytes | Canonical padded base64 message, at most 64 KiB decoded; base64 of the 64-byte signature; no `recovery_id` field |
| `secp256k1` | Standard base64 of the 33-byte compressed SEC1 key | Canonical padded base64 of an **exactly 32-byte prehash**; low-S 64-byte `r || s` signature in base64, plus numeric `recovery_id` 0 or 1 |

The secp256k1 endpoint signs the supplied digest without hashing it again.
The caller must construct the correct domain/transaction digest. The service
normalizes S, adjusts recovery parity and self-checks sender-key recovery;
non-EVM-recoverable results refuse. A key handle cannot be used with a different
algorithm: that returns `422 algorithm_mismatch`. An unsupported algorithm
returns `422 unsupported_algorithm`; a secp256k1 message of the wrong decoded
length returns `400 invalid_message`.

Create scope is `{purpose}:{Idempotency-Key}`. The idempotency token is at most
128 bytes; purpose is a token of at most 64 bytes. SHA-256 of the **raw request
body** is retained. The same scope/body/algorithm returns the original key;
a changed body or algorithm conflicts with `409 idempotency_conflict`.
Key IDs use `whk_` plus 12 random bytes in hex; handles use `kms-ed25519-` or
`kms-secp256k1-` plus 16 random bytes in hex. The store bounds keys at 100,000.

The API returns handles, public keys and signatures, with no key-export route.
Webhooks requests Ed25519 keys for purpose `layerx-webhook-v1`, uses the
create/sign endpoints and verifies signatures against the returned public key
(`services/platform/hosted/webhooks/src/hosted.rs`). Human owner export is
[documented separately](HumanCustody.md#owner-export-and-handover).

## Event source request and response

`LAYERX_EVENTS_KIND` is `journeys`, `approvals`, `payments` or `programs`.
The operator supplies the bare HTTPS origins `<human-service-origin>` and
`<gateway-origin>` for the configured upstreams below.

| Kind | Configured upstream | Principal credential | Retained snapshot pull |
| --- | --- | --- | --- |
| `journeys` | `<human-service-origin>` | `__Host-layerx_access` cookie | `/v1/journeys/{resource}` |
| `approvals` | Same Human origin | Same session cookie | `/v1/approvals/{resource}` |
| `payments` | `<gateway-origin>` | `Authorization: LayerX-Key …` | `/v1/receipts/{resource}` |
| `programs` | Same gateway origin, for principal binding | `Authorization: LayerX-Key …` | Retired: reader-token observation returns `410 program_pull_retired` |

The normal producer path sends a full observation to
`POST /internal/v1/observe` with its producer token and client certificate.
`src/producer.rs::Observation` carries `kind` (singular), `id`, exactly one of
`principal` or permitted `principal_digest`, `resource`, `sequence`,
`source_sequence`, `occurred_at`, `facts`, and optional payment facts
`activity_id`, `amount`, `asset`. Observations are bounded to 16 KiB.

The ID is SHA-256 over length-prefixed kind, resource and big-endian sequence.
Sequences are positive; facts are bounded and cannot supply their own
`source_sequence` fact. Digest identity is allowed only for payments/programs
and only by a producer credential with `allow_principal_digest: true`.
Before append, the source resolves the enrolled principal and rebinds its
upstream credential. A valid append returns `200 Record`.

`Record` contains `id`, `principal`, `subject`, `subject_sequence`,
`occurred_at`, `facts`, and optional `activity_id`, `amount`, `asset`.
The source adds the observation's source sequence to the stored facts.
Exact byte retries return the same record, including after restart;
changed bytes for an existing ID or a sequence gap return `409
observation_refused`. Invalid observations return `400`, wrong kind/principal
returns `403`, and storage failure returns `503`, all with code
`observation_refused`; the `503` response permits retry after 5 seconds.

Reader-token `POST /internal/v1/observe` with `{"principal","resource"}`
retains snapshot observation for journeys, approvals and payments. It binds the
principal, fetches the named resource, checks its identity, and derives facts.
Journey facts are `kind`, `state`, `updated_at`; approval facts are `agent_id`,
`state`, `created_at`; payment amount, asset and time come from the decoded
protocol receipt. The snapshot-derived ID hashes `(principal, resource,
snapshot)`. This legacy pull path is distinct from producer-supplied IDs.

`GET /internal/v1/events/{id}` uses reader mTLS and token, requires a 32-byte
lowercase hex ID, verifies current enrollment and rebinds the stored principal
before returning the record. Unknown or removed-principal events return `404
event_not_found`; binding failure returns `503 source_unavailable`.

### Human producer and webhook delivery

`services/human/crates/layerx-human-service/src/event_producer.rs` constructs
`Client::from_environment(&["journey", "approval"])` and stores
observations in the principal-scoped `Table::EventOutbox`. Journey and approval
stream transitions enqueue delivery; the producer preserves the encoded bytes
and sequence across retries and restart. Session enrollment is also retained
and reconciled to both sources through `POST /internal/v1/enrollments`.

The shared `src/producer.rs` client first posts the observation and verifies
the returned `Record` exactly. After durable observation acknowledgment, it
posts to the webhook service's `POST /internal/v1/events/{kind}/{id}`. Webhooks
fetches the stored source record; a valid `202` notification acknowledgment
completes delivery. A lost acknowledgment leaves the same pending bytes for
retry. Observation and notification progress are distinct durable states.
The shared queue bounds pending entries at 1,024 and automatic delivery attempts
at five, retaining failed entries for explicit redelivery rather than dropping
them. Human scheduling prevents one failing principal from starving another.

## Enrollment, durability and readiness

`LAYERX_EVENTS_CREDENTIALS_FILE` is a versioned, MAC-authenticated enrollment
snapshot, not a plain principal-to-path map. It contains `version`,
`generation`, `principals` entries (`principal`, `credential_file`) and `mac`.
The enrollment key and credential files must satisfy protected-file checks.
A newer valid generation is adopted atomically after upstream identity checks;
a refused snapshot leaves the last valid generation intact. Enrollment state
survives restart and credentials are revalidated. The source polls every three
seconds (`src/events.rs`, `src/bin/layerx-event-source.rs`).

Principal binding calls upstream `GET /internal/v1/principal`. Human sources
require `active: true` and a matching `sub`; gateway sources require the SHA-256
principal digest. Issuer-managed enrollment also checks the tenant identity.
An upstream result must be HTTP 200 JSON with `ok: true` and `result`.

The journal is `journal.log` in the configured state directory. It locks
exclusively, appends JSON lines, syncs before success and replays in order.
Directory/file modes are `0700`/`0600`; records are bounded to 64 KiB and the
journal to 1,000,000 entries. Failed appends make it unavailable. Readiness
syncs a `ready.marker` (`src/journal.rs`). Event replay rejects invalid
identities, duplicate IDs and per-principal/subject sequence gaps.

KMS records retain sealed key material. The service derives its AES-256-GCM
seal key using `layerx-kms-ed25519-seed` and the configured seal secret.
Secp256k1 scalar plaintext carries the additional
`layerx-kms-secp256k1-scalar` domain prefix. Each seal uses a fresh 12-byte nonce.
Replay authenticates every key and checks its public key; a wrong seal secret
refuses startup (`src/kms.rs`, `src/seal.rs`). Retain the journal and matching
seal secret together.

KMS `/readyz` requires the store lock and a successful durable write probe;
its body retains `ed25519_non_exportable: true` even though secp256k1 is also
supported. Event `/livez` stays live while an empty enrollment produces
`503 /readyz`, `state: "waiting-principals"`. With principals enrolled,
readiness requires upstream readiness, all principal bindings and a writable
journal within its deadline. Protected `/internal/v1/producer-readiness` and
`/internal/v1/reader-readiness` return the role, kind, generation, principal
count and readiness under schema `layerx.event-admission.v1`.

## Configuration

The Kubernetes manifest binds `0.0.0.0:9443`, stores journals under
`/var/lib/layerx/state` and mounts runtime material under `/run/layerx`.
Environment-initialized deployments use the `/data` paths described above.

| Variable | Role |
| --- | --- |
| `LAYERX_KMS_LISTEN`, `LAYERX_KMS_STATE_DIR` | TLS bind and key journal directory |
| `LAYERX_KMS_TOKEN_FILE`, `LAYERX_KMS_SEAL_SECRET_FILE` | Bearer and seal secrets, each at least 16 bytes |
| `LAYERX_KMS_TLS_CERT_DER`, `LAYERX_KMS_TLS_KEY_DER`, `LAYERX_KMS_CLIENT_CA_DER` | Server certificate, PKCS#8 key and required client CA |
| `LAYERX_EVENTS_LISTEN`, `LAYERX_EVENTS_STATE_DIR` | Source TLS bind and journal directory |
| `LAYERX_EVENTS_TOKEN_FILE` | Reader/legacy observer Bearer secret |
| `LAYERX_EVENTS_TLS_CERT_DER`, `LAYERX_EVENTS_TLS_KEY_DER`, `LAYERX_EVENTS_CLIENT_CA_DER` | Source server identity and required client CA |
| `LAYERX_EVENTS_KIND` | Source family |
| `LAYERX_EVENTS_UPSTREAM_URL`, `LAYERX_EVENTS_UPSTREAM_CA_DER` | Bare HTTPS origin and pinned CA |
| `LAYERX_EVENTS_CREDENTIALS_FILE`, `LAYERX_EVENTS_ENROLLMENT_KEY_FILE` | Authenticated enrollment snapshot and its protected MAC key |
| `LAYERX_EVENTS_BOOTSTRAP_CREDENTIALS_FILE` | Optional bootstrap enrollment snapshot |
| `LAYERX_EVENTS_PRODUCERS_FILE` | JSON array of `token_file`, `allow_principal_digest` entries |
| `LAYERX_EVENTS_UPSTREAM_TOKEN_FILE`, `LAYERX_EVENTS_UPSTREAM_COOKIE_FILE` | Optional default upstream credentials; principal binding uses enrolled credentials |
| `LAYERX_EVENTS_UPSTREAM_CLIENT_IDENTITY_PKCS12`, `LAYERX_EVENTS_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE` | Optional upstream client identity and its required password file |
| `LAYERX_HEALTH_ADDR` | Optional plain HTTP health listener bind; `/healthz` is liveness, not dependency readiness |

Human producer configuration uses `LAYERX_EVENTS_JOURNEY_*`,
`LAYERX_EVENTS_APPROVAL_*` and `LAYERX_EVENTS_WEBHOOKS_*`, each with
`UPSTREAM_URL`, `UPSTREAM_CA_DER`, `UPSTREAM_TOKEN_FILE`,
`UPSTREAM_CLIENT_IDENTITY_PKCS12` and
`UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE`. Partial configuration refuses startup
(`src/producer.rs`); producer tokens are separate from reader tokens.

Redis serves TLS `6379`, with `appendonly yes`, `appendfsync always` and an ACL
file. Its readiness uses an authenticated probe user. Webhooks and dashboard
use the operator-supplied `<redis-url>` with the `rediss` scheme and their
configured credentials (`deployment.yaml`,
`services/platform/hosted/webhooks/deployment.yaml`).

## Refusals

| Code | Status | Condition |
| --- | --- | --- |
| `client_certificate_required` | 401 | KMS protected request without a verified peer certificate |
| `unauthorized` | 401 | Wrong token or missing required event-source certificate |
| `idempotency_key_required` | 400 | Missing/invalid KMS create token |
| `invalid_request`, `invalid_message` | 400 | Malformed KMS request or noncanonical/wrong-length signing message |
| `unsupported_algorithm`, `algorithm_mismatch`, `invalid_purpose` | 422 | Invalid KMS algorithm, handle algorithm mismatch or purpose |
| `idempotency_conflict` | 409 | Same KMS scope with different request bytes/algorithm |
| `unknown_key` | 404 | Unknown/invalid KMS identifier or handle |
| `message_too_large` | 413 | Decoded signing input exceeds 64 KiB |
| `method_not_allowed`, `not_found` | 405 / 404 | Unsupported method on a known KMS route / unmatched route |
| `dependency_unavailable`, `source_unavailable` | 503 | KMS dependency failure / snapshot observation or event rebind failure; retry after 5 seconds |
| `observation_refused` | 400 / 403 / 409 / 503 | Producer shape, identity, ordering or storage refusal |
| `program_pull_retired` | 410 | Retired program snapshot observation |
| `event_not_found` | 404 | No visible event under the enrolled principal |
| `serialization_failed` | 500 | Response encoding failure |

Enrollment and admission have additional typed refusal codes in `src/events.rs`.
An empty source reports waiting for principals; a live process alone does not
establish readiness.

## Tests

Source coverage includes sealed/idempotent KMS restart and real TLS signing in
`src/kms.rs` and `tests/kms_tls.rs`; producer-byte replay, enrollment rotation
and immutable event ordering in `src/events.rs`; and bounded durable delivery
in `src/producer.rs`. Human's real TLS observation and sink-loss recovery path
is in
`services/human/crates/layerx-human-service/src/event_producer_tls.rs`.
These are locations of implementation tests, not a claim that this documentation
rewrite ran those suites.

[Human custody](HumanCustody.md) · [Human journeys](HumanJourneys.md) ·
[Home](Home.md)
