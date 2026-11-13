# Hosted internal

The internal services provide signing and durable event delivery for the
hosted platform. They operate alongside the single merged Paxeer network;
their acknowledgements and journals do not establish chain finality. See
[Activities](../concepts/activities.md#status) for signed execution receipts
and committed inclusion, and [Payments](../overview/payments.md) for the
merchant settlement boundary.

`layerx-kms` serves the operator-supplied `<kms-origin>` and supports Ed25519 and
secp256k1 signing. It generates keys from operating-system entropy, seals
their private material at rest and returns only opaque handles, public keys
and signatures. `layerx-event-source` serves the `journeys`, `payments`,
`approvals`, and `programs` adapters. Both binaries belong
to `services/platform/hosted/internal` (`layerx-platform-internal`).

## Namespace and apply

`services/platform/hosted/internal/deployment.yaml` declares the
`layerx-internal` namespace, default-deny ingress/egress, TLS Redis, KMS,
four event-source Deployments, their Services and PVCs, and aliases for
internal identity, component and authority access. The component and
authority aliases resolve to Services selecting the node pods. They are
internal adapters, not a second network or settlement authority.

KMS and each event source use one replica with strategy `Recreate`, user
and group `4020`, a read-only root filesystem and a 1Gi `ReadWriteOnce` PVC
mounted at `/var/lib/layerx`. Their Service port `443` targets the TLS
listener on `9443`. The image
`ghcr.io/sidiora-labs/layerx-internal:0.1.0` contains both binaries; the
Docker entrypoint is KMS, while event-source Deployments override the command.

The image inventory in `services/platform/hosted/tests/beta-images.sh`
includes internal and Human images. Deployment manifests separately define
the KMS, event-source and Human application workloads. The native cluster
helper's `up` and `render` paths apply or render the native node manifest;
image inventory membership does not install the application workloads.

`services/platform/hosted/human/deployment.yaml` declares Service
`layerx-human` on `9443`, selecting node pods and targeting `human-https`.
The hosting deployment must supply that named port and its Human process.
The web workload is declared in `human/web-deployment.yaml`.

Consult the node, Human, internal, gateway, registry and webhooks manifests
for each Service selector, target port and NetworkPolicy edge. Both ingress
and caller egress must permit the connection, with the corresponding TLS
identity and credentials. The source's Human upstream is an application
service, not the native node's JSON-RPC listener.

Retain each PVC, KMS seal secret and event enrollment key across restart.
Installing an image or passing liveness does not enroll a principal, create
upstream credentials, or prove producer admission. Use
[Beta cluster](beta-cluster.md) for the cluster lifecycle.

## TLS and HTTP

Both binaries require their prefixed `CLIENT_CA_DER` at startup. The rustls
listener permits a connection without a client certificate so kubelet can
probe unauthenticated health routes; protected routes still require a verified
peer certificate and the appropriate bearer credential.

The HTTP parser accepts one HTTP/1.1 request per connection, requires a Host
header and an exact Content-Length body, rejects duplicate headers, query
strings and transfer encoding, and bounds headers plus body at 96 KiB.
I/O timeout is eight seconds and concurrency is bounded at 128 connections.
Responses are JSON with `Cache-Control: no-store` and `Connection: close`.
Parser failures return `400 invalid_request`.

Refusals have the form
`{"error":{"code":"…","retry":"never"}}` or `retry: "after"` with
`retry_after_seconds`. Token comparisons use constant-time equality.
The implementation is `internal/src/http.rs`.

Upstream clients use bare HTTPS DNS origins, a pinned CA, TLS 1.2 or newer,
a five-second connect timeout, eight-second I/O timeout and a 512 KiB
response bound (`internal/src/tls.rs`). Literal IP origins are refused.
A client identity uses PKCS#12 plus its password file when configured.

Both binaries also serve plain HTTP `GET /healthz` on `LAYERX_HEALTH_ADDR`
(default `[::]:8080`). That route reports process health, not dependency
readiness. The manifest's TLS probes use `/livez` and `/readyz`.

## Who may call

KMS key operations require both a verified client certificate and
`Authorization: Bearer` matching `LAYERX_KMS_TOKEN_FILE`. The token is at
least 16 bytes. A missing certificate gives `401 client_certificate_required`;
a bearer mismatch gives `401 unauthorized`. `GET /livez` and `GET /readyz`
do not authenticate.

Event sources distinguish the reader token (`LAYERX_EVENTS_TOKEN_FILE`)
from producer credentials in `LAYERX_EVENTS_PRODUCERS_FILE`. A protected
request requires a verified client certificate. Producer credentials authorize
observations and producer admission; the reader token authorizes event reads
and the retained pull adapter. Webhooks reads the source and separately
accepts an authenticated event trigger. A producer must not reuse the reader
token. At most three distinct producer credentials are accepted per source;
only payment and program sources can permit principal-digest observations.

The source always binds a principal to its enrolled upstream credential.
Journeys/approvals use `Cookie: __Host-layerx_access=…` and require an active
Human session whose `sub` equals the principal. Payments/programs use
`Authorization: LayerX-Key …` and require the upstream `principal_digest`
to equal the SHA-256 hex digest of that principal. Issued Human enrollment
also checks its stored tenant binding. Principal strings and credentials
are not interchangeable across tenants or users.

## KMS request and response

The implementation and exact JSON contracts are in `internal/src/kms.rs`.
Create/sign bodies reject unknown fields; message and public-key encodings
use standard base64.

| Method and path | Input | Success |
| --- | --- | --- |
| `GET /livez` | None | `200 {"alive":true}` |
| `GET /readyz` | None | `200` or `503` with `ready` and `ed25519_non_exportable` |
| `POST /v1/signing-keys` | Certificate, bearer, `Idempotency-Key`, JSON `{"algorithm":"ed25519","purpose":"…"}` or the `secp256k1` algorithm | `201` fresh or `200` existing; `key_id`, `handle`, `public_key` |
| `GET /v1/signing-keys/{key_id}` | Certificate, bearer, valid key id | `200` with the same public key body |
| `POST /v1/signatures` | Certificate, bearer, JSON `{"key_handle":"…","algorithm":"…","message":"…"}` | `200` with base64 `signature`, and secp256k1 `recovery_id` |

The readiness field `ed25519_non_exportable: true` is retained in the body;
it is not an exhaustive supported-algorithm list. Readiness requires the
store lock and a successful durable-write probe.

Create scope is `{purpose}:{Idempotency-Key}`. The request digest is SHA-256
of the raw body, so retry with exactly the same bytes. The same scope/digest
returns the existing key; a different digest gives `409 idempotency_conflict`.
The idempotency token is at most 128 characters and purpose at most 64.
Key ids start `whk_`; handles start `kms-ed25519-` or `kms-secp256k1-`.
The store is bounded at 100,000 keys.

| Algorithm | Public key | Signed input | Signature |
| --- | --- | --- | --- |
| `ed25519` | 32 bytes | Message, at most 64 KiB decoded | 64 bytes |
| `secp256k1` | 33-byte compressed SEC1 point | Exactly a 32-byte prehash; the KMS does not hash the message again | 64-byte low-S `r || s`, plus separate `recovery_id` 0 or 1 |

For an EVM transaction, submit its signing digest under a secp256k1 handle
and encode the returned scalars and recovery parity in the transaction. Do
not substitute the transaction hash after signing for the signing digest.
Request algorithm and key algorithm must agree. The service self-verifies
signatures and refuses an invalid digest width or non-EVM-recoverable result.
Webhooks uses Ed25519 with purpose `layerx-webhook-v1` and verifies returned
signatures under the returned public key.

## Event source request and response

`LAYERX_EVENTS_KIND` selects `journeys`, `approvals`, `payments`, or
`programs`. The live producer path is implemented by
`internal/src/producer.rs` and `internal/src/events.rs`:

| Producer | Source | Durable origin |
| --- | --- | --- |
| Human | Journeys and approvals | `services/human/crates/layerx-human-service/src/event_producer.rs`; `HumanOutbox` starts the producer client and replays pending observations |
| Gateway | Payments | `services/platform/hosted/gateway/src/store.rs` and `src/main.rs`; verified payment completion records its observation and delivery state |
| Registry | Programs | `services/platform/hosted/registry/src/event_producer.rs`; `ProgramOutbox` journals verified publication transitions |

These producers issue `POST /internal/v1/observe`. Delivery to the source
and the later webhook trigger are separate steps. An event does not replace
the consumer's verification of the underlying chain result.

| Method and path | Authentication and body | Outcome |
| --- | --- | --- |
| `GET /livez` | None | `200 {"alive":true}` |
| `GET /readyz` | None | Readiness, generation, principal count and last enrollment refusal |
| `POST /internal/v1/enrollments` | Authorized producer, certificate, issued Human enrollment | Authenticated journey/approval enrollment; binds tenant, principal, session, revision, credential and expiry |
| `GET /internal/v1/producer-readiness` | Producer bearer and certificate | Role-specific admission evidence |
| `GET /internal/v1/reader-readiness` | Reader bearer and certificate | Role-specific admission evidence |
| `POST /internal/v1/observe` | Producer bearer, certificate, JSON `Observation` | `200` immutable `Record`, or typed refusal |
| `GET /internal/v1/events/{id}` | Reader bearer, certificate, 32-byte lowercase hex id | `200` stored `Record` after renewed principal binding; unknown/un-enrolled record gives `404 event_not_found` |

A producer observation contains `kind`, deterministic `id`, exactly one of
`principal` or permitted `principal_digest`, `resource`, `sequence`,
`source_sequence`, `occurred_at`, and bounded `facts`. Payment observations
also bind `activity_id`, `amount` and `asset`. The id is derived by
`producer::event_id` from kind, resource and sequence. Bodies are bounded
at 16 KiB. A duplicate id succeeds only for the identical body; conflicting
bytes or sequence return `409 observation_refused`. Malformed observations
return `400 observation_refused`; invalid principal/kind authority returns
`403 observation_refused`; durable-store failures return
`503 observation_refused` with a five-second retry hint.

The stored record has a per-principal/subject `subject_sequence` and includes
`source_sequence` as a fact. Program facts are `version`, `code_hash`,
`receipt_digest`, and `lifecycle`: the producer takes the latest member of
the verified publication's `versions` array, using its
`deployment_receipt_digest`. Registry chain deployment verification lives
in `registry/src/programs_chain.rs`. A registry event is an observation of
that verified publication, not independent proof of program execution or
payment commitment.

The reader-token pull adapter retains JSON `{"principal":"…","resource":"…"}`
for journeys, approvals and payment snapshots. Its Human upstream paths are
`/v1/journeys/{resource}` and `/v1/approvals/{resource}`; the payment adapter
retains `/v1/receipts/{resource}` and validates the decoded protocol receipt.
This compatibility adapter does not establish merged transaction commitment;
use the producer and committed-receipt boundaries for current integrations.
Program polling is retired: a reader-token observe request for programs
returns `410 program_pull_retired`. Do not configure registry polling as a
replacement for the program producer.

## Durability and ordering

The append-only `journal.log` is replayed in order on open. Each record is
written and `sync_all` completes before success is returned. The directory
is mode `0700`; new journal files are mode `0600` and exclusively locked.
A failed append marks the journal unavailable. Bounds are 64 KiB per record
and one million records. Readiness writes and syncs `ready.marker` in the
same directory (`internal/src/journal.rs`).

Event ordering starts at 1 per `(principal, subject)`. Replay rejects invalid
ids/principals/subjects, duplicate ids, excessive facts and sequence gaps.
Producer records also retain the exact observation bytes for retry comparison.
Producer outboxes retain pending delivery and acknowledgement state across
restart; preserve those stores as well as the source journal.

KMS uses AES-256-GCM with a fresh 12-byte nonce. Ed25519 material uses seal
label `layerx-kms-ed25519-seed`; secp256k1 uses
`layerx-kms-secp256k1-scalar`. The journal stores `sealed_seed` and public
metadata, not a plaintext private seed. Reopen authenticates every sealed
key and checks its public key; the wrong seal secret refuses startup.
Retaining the journal without the matching seal secret does not recover keys.

## Readiness and enrollment

`LAYERX_EVENTS_CREDENTIALS_FILE` names an authenticated enrollment
snapshot. Its version-1 JSON fields
are `version`, `generation`, `principals` (entries with `principal` and
`credential_file`), and `mac`. The HMAC-SHA256 covers the source kind,
generation and ordered principal/credential digests. Use the canonical
`enrollment_snapshot_mac` implementation; JSON serialization is not the
MAC input. Snapshot size is bounded at 1 MiB and enrollment at 10,000
principals. Protected credential/key reads reject unsafe files.

`LAYERX_EVENTS_ENROLLMENT_KEY_FILE` is required. The source polls every
three seconds, verifies new snapshots and binds new or rotated credentials
upstream before atomically adopting a generation. Bad MACs, stale or
conflicting generations, credential reuse across principals and binding
failures retain the last valid generation. Durable `enrollment.json` tracks
adopted authority; restart revalidates persisted principals upstream before
serving them. Removing a principal revokes its source access.

An empty source starts live but unready. `GET /readyz` returns `503` with
`state: "waiting-principals"`, the generation and principal count. Ordinary
operations return `503 waiting_principals` with a three-second retry hint;
issued-enrollment and role admission routes have their own checks. The
`--empty-enrollment-mac` mode generates the empty generation-zero MAC from
the configured kind and enrollment key without printing that key.

For a populated source, readiness requires upstream `/readyz`, principal
binding for every enrolled credential and a writable journal, within its
bounded deadline. Role admission routes require the corresponding bearer
and certificate and report schema `layerx.event-admission.v1`, role, kind,
generation, count, digest capability and `fresh_for_ms: 10000`. Readiness is
short-lived dependency evidence, not a durable permission or chain receipt.

## Configuration

Paths below are supplied by the manifest and entrypoint; secret values stay
in mounted files.

| Variable or family | Role |
| --- | --- |
| `LAYERX_KMS_LISTEN`, `LAYERX_KMS_STATE_DIR` | TLS address and durable key store |
| `LAYERX_KMS_TOKEN_FILE`, `LAYERX_KMS_SEAL_SECRET_FILE` | API bearer and at-rest sealing secret |
| `LAYERX_KMS_TLS_CERT_DER`, `LAYERX_KMS_TLS_KEY_DER`, `LAYERX_KMS_CLIENT_CA_DER` | Server DER certificate, PKCS#8 DER key and required client CA |
| `LAYERX_EVENTS_LISTEN`, `LAYERX_EVENTS_STATE_DIR`, `LAYERX_EVENTS_KIND` | Source address, journal and one of four kinds |
| `LAYERX_EVENTS_TOKEN_FILE` | Reader bearer |
| `LAYERX_EVENTS_TLS_CERT_DER`, `LAYERX_EVENTS_TLS_KEY_DER`, `LAYERX_EVENTS_CLIENT_CA_DER` | Source TLS identity and required client CA |
| `LAYERX_EVENTS_UPSTREAM_URL`, `LAYERX_EVENTS_UPSTREAM_CA_DER` | Bare HTTPS upstream origin and pinned CA |
| `LAYERX_EVENTS_CREDENTIALS_FILE` | Authenticated enrollment snapshot |
| `LAYERX_EVENTS_ENROLLMENT_KEY_FILE` | Required protected HMAC key file, at least 16 bytes |
| `LAYERX_EVENTS_BOOTSTRAP_CREDENTIALS_FILE` | Optional authenticated bootstrap snapshot, provisioned by the cluster |
| `LAYERX_EVENTS_PRODUCERS_FILE` | JSON array of `token_file` and `allow_principal_digest` producer entries |
| `LAYERX_EVENTS_UPSTREAM_TOKEN_FILE`, `LAYERX_EVENTS_UPSTREAM_COOKIE_FILE` | Optional default upstream credentials; principal-bound fetches use enrolled credentials |
| `LAYERX_EVENTS_UPSTREAM_CLIENT_IDENTITY_PKCS12`, `LAYERX_EVENTS_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE` | Optional upstream client identity; password file required with the identity |
| `LAYERX_HEALTH_ADDR` | Separate plain HTTP health bind |

Manifest listeners use `0.0.0.0:9443` and state `/var/lib/layerx/state`.
Source enrollment is under `/var/lib/layerx/enrollment`, with bootstrap
material copied to `/var/lib/layerx/bootstrap`. The operator supplies
`<human-service-origin>` for Human sources and `<gateway-origin>` for payment
and program sources' principal binding. Both must be bare HTTPS origins.

Producer clients have separate `LAYERX_EVENTS_{JOURNEY|APPROVAL|PAYMENT|PROGRAM}`
upstream configuration and `LAYERX_EVENTS_WEBHOOKS` notification configuration.
For each configured producer family the client requires `UPSTREAM_URL`,
`UPSTREAM_CA_DER`, `UPSTREAM_TOKEN_FILE`,
`UPSTREAM_CLIENT_IDENTITY_PKCS12`, and
`UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE`. These are not the source's reader
credentials. See `internal/src/producer.rs` and the calling workload manifest.

Redis in the internal namespace uses TLS `6379`, `tls-auth-clients no`,
`appendonly yes`, and `appendfsync always`. Applications still need their
configured Redis authentication. Redis persistence and source PVCs cover
different state and must both survive restart.
The operator supplies `<redis-url>`, using the `rediss` scheme for TLS.

## Refusals

| Code | HTTP status | Meaning |
| --- | --- | --- |
| `client_certificate_required`, `unauthorized` | 401 | Missing required KMS certificate or wrong credentials; event protected routes use `unauthorized` |
| `idempotency_key_required`, `invalid_request` | 400 | Missing/invalid KMS request or malformed HTTP |
| `invalid_message` | 400 | Noncanonical base64 or wrong secp256k1 digest width |
| `unsupported_algorithm`, `algorithm_mismatch`, `invalid_purpose` | 422 | Unsupported algorithm, handle/request mismatch, or bad purpose |
| `idempotency_conflict` | 409 | KMS create scope reused for a different raw-body digest |
| `unknown_key`, `event_not_found`, `not_found` | 404 | Unknown key/event or unmatched route |
| `message_too_large` | 413 | Decoded KMS input exceeds 64 KiB |
| `method_not_allowed` | 405 | Wrong method on a known KMS route |
| `dependency_unavailable`, `source_unavailable` | 503 | KMS persistence/signing failure or source binding/pull failure; five-second retry hint |
| `waiting_principals` | 503 | No enrolled principals; three-second retry hint |
| `observation_refused` | 400/403/409/503 | Producer validation, authority, conflict or durable-store failure |
| `program_pull_retired` | 410 | Retired Programs pull adapter |
| `serialization_failed` | 500 | JSON response serialization failure |

Enrollment and role-admission checks return their specific refusal codes;
inspect `last_refusal` without logging credential contents. A refusal is never
permission to bypass principal binding or to invent replacement observations.

## Tests

Existing test sources document the guarantees; this page does not assert
that they were run for a deployment:

- `internal/tests/kms_tls.rs` exercises real TLS authentication, restart
  identity preservation and secp256k1 transaction-recoverable signatures.
- `internal/src/events.rs` covers empty enrollment, credential rotation,
  rejected snapshots, restart revalidation, principal isolation and identical
  producer retries after restart.
- `internal/src/journal.rs`, `internal/src/seal.rs`, `internal/src/http.rs`,
  and `internal/src/tls.rs` cover durable replay, authenticated sealing,
  bounded parsing and origins.
- Human and registry `event_producer.rs` and gateway `store.rs` own the
  producer/outbox behavior; successful installation alone does not prove
  observation delivery.

All abbreviated `internal/` paths above are relative to
`services/platform/hosted/`. Follow [Monorepo layout](../overview/monorepo.md)
for the service and SDK workspaces.

[Home](../index.md)
