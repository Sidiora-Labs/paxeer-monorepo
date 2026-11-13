# Hosted webhooks

`layerx-webhooks` delivers protocol events to subscriber HTTPS endpoints. The
crate is `layerx-platform-webhooks`, under
`services/platform/hosted/webhooks/`. Deliveries use Ed25519 scheme
`LayerX/webhooks/v1`, durable queues, per-subject ordering, bounded retries,
dead letters and cursor-based replay. A delivery becomes `delivered` only after
the subscriber endpoint returns an accepting HTTP status.

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The
kernel is part of the network and proposes blocks; a receipt is final when its
block is committed. Webhooks transport observations of that network. Neither
an outgoing webhook signature nor a successful HTTP delivery creates finality.
See [Modules](Modules.md) for execution and [Hosted dashboard](HostedDashboard.md)
for developer views of delivery health, keys and test payments.

Public node JSON-RPC uses the root path; see [Public JSON-RPC](PublicRpc.md).
Hosted webhook REST routes use their configured application service.

---

## Deployment

The binary requires `LAYERX_WEBHOOKS_ROLE=public` or `ingress`. The roles serve
different routes and credentials (`src/main.rs` below refers to the webhook
crate). Share the Redis delivery store, cursor key and signing infrastructure
between replicas, and give each replica a distinct `LAYERX_WEBHOOKS_INSTANCE_ID`.

| Role | Application routes | Inbound authentication |
| --- | --- | --- |
| `public` | `/v1/webhooks/...` | Developer session, except the public signature-scheme document |
| `ingress` | `/internal/...` | TLS client certificate verified against the internal CA, an explicit webhook role URI SAN, and the route's bearer token |

`docker/platform-webhooks/Dockerfile` provides `env-init` and
`env-init-prebuilt` targets. Both run `layerx-env-files` and then
`services/platform/hosted/webhooks/env-init.sh`; the prebuilt target checks the
binary's SHA-256. The init script maps `LAYERX_ROLE` to the binary's role, selects
TLS and credential file paths, assigns the instance ID from
`RAILWAY_REPLICA_ID` or hostname, and runs as UID `65532`.

For Railway, public TLS defaults to `[::]:9444` and ingress TLS to `[::]:9443`.
`LAYERX_WEBHOOKS_LISTEN` overrides these defaults. The separate plain HTTP health
listener uses `LAYERX_HEALTH_ADDR`, then `[::]:$PORT`, then `[::]:9442` for public
or `[::]:8080` for ingress. `PORT` moves only the health listener. When launching
the binary directly, set `LAYERX_WEBHOOKS_HEALTH_LISTEN` explicitly if needed;
its application listener default is `0.0.0.0:9444` for either role.

Configure the Redis TLS origin, KMS, identity and all four event-source HTTPS
origins through the keys below.

The Kubernetes manifest `services/platform/hosted/webhooks/deployment.yaml`
exposes only the public role through the `/v1/webhooks` Ingress prefix. Its
private `layerx-webhooks` Service selects ingress replicas. Both application
listeners use port `9444` in that manifest; ingress has a separate plain health
listener on `9447`. NetworkPolicy isolation supplements certificate and bearer
authentication.

### Current receipt-verifier configuration

The active `TrustedSources::from_environment` constructs the node-backed
`ReceiptVerifier` in `src/trusted.rs`. Ingress requires all four settings:

| Key | Meaning |
| --- | --- |
| `LAYERX_WEBHOOKS_NODE_URL` | HTTPS node JSON-RPC origin; requests are POSTed to `/` |
| `LAYERX_WEBHOOKS_COMET_URL` | HTTPS consensus JSON-RPC origin; requests are POSTed to `/` |
| `LAYERX_WEBHOOKS_NODE_TOKEN_FILE` | Bearer credential used for both origins |
| `LAYERX_WEBHOOKS_CHAIN_POLICY_FILE` | JSON policy containing `chain_id` and pinned `validators` |

Each validator pin has `public_key` (a 32-byte array) and positive
`voting_power`. `src/chain.rs` validates the policy and verifies commit and
membership evidence. The policy's `chain_id` is the consensus network name
checked against consensus `status`; it is not the numeric EVM chain ID.
Provision the policy from the same network identity and validator authority as
the node. Tenant domains are database namespaces, not independent consensus
networks requiring their own infrastructure.

Ingress receipt verification uses the configured consensus chain name, pinned
validators, governance proof and committed block. The consensus chain ID is
`hyperpax_125-1`; the EVM chain ID is `125`.

### Credential files

For env-file images, `docker/platform-webhooks/files.tsv` maps base64 file
variables to paths under `/run/layerx`; set
`LAYERX_FILES_OWNER=65532:65532`. The example's common files include server DER
certificate and PKCS#8 key (`tls/server.der`, `tls/server-key.der`), outbound
PKCS#12 identity and password (`client/identity.p12`, `client/password`), internal
and public CAs (`ca/internal.der`, `ca/public.der`), Redis username/password,
KMS bearer (`tokens/kms`) and the cursor HMAC key (`keys/cursor`). Secret
provisioning and certificate issuance helpers live in
`tools/bringup/mint-secrets.sh` and `tools/bringup/ca.sh`.

Public additionally needs its identity-introspection bearer. Ingress needs four
source-reader bearers, distinct source-trigger and operator bearers, and the
node verifier's token and policy. Mount the verifier token and policy at the paths configured by their file keys.

---

## Subscriptions

A subscription is a `StoredEndpoint` owned by an authenticated `Principal`.
Public authentication introspects `Authorization: Bearer <session>` or cookie
`__Host-layerx-session` at identity `POST /v1/sessions/introspect`, using the
service's identity bearer. The response must be `active` and its `sub` must be a
valid principal. Cookie-authenticated POST and DELETE requests require
`x-layerx-csrf` equal to the introspected `csrf_token`. Failure maps to
`401 session_required`.

`POST /v1/webhooks/endpoints` requires `Idempotency-Key` and JSON fields `url`,
optional `kinds`, and optional `minimum_verification`; unknown fields are
refused. Empty kinds accepts every family; minimum verification defaults to
`unverified`. Destination URLs must use HTTPS, a canonical DNS hostname rather
than an IP literal, and resolve only to public addresses. The outbound boundary
also enforces the destination restrictions when delivering.

At most 32 endpoints belong to one principal. Reusing the same idempotency scope
returns the existing registration; registering the same URL again in a new
scope is a conflict. Success is `201` with endpoint ID, signing key ID, public
key and the receiver obligation. Endpoint IDs use `whep_` plus 16 random bytes in
hex. KMS creates Ed25519 keys for purpose `layerx-webhook-v1`; generated key IDs
must start with `whk_` (`src/hosted.rs`).

Key rotation announces a pending key that activates after the configured
overlap, default `86400` seconds. Suspension requires a reason of 1–256 bytes
without CR, LF or NUL. Resumption clears suspension and consecutive dead-letter
count; dispatch skips suspended endpoints.

---

## Event contracts

The generic source adapter fetches immutable source records, not arbitrary
caller-supplied event bodies. Source records carry ID, principal, subject,
subject sequence, occurrence time, facts and optional activity information.

| Webhook kind | Canonical source facts | Receipt requirement |
| --- | --- | --- |
| `journey` | Journey identity and lifecycle state | Optional activity receipt |
| `payment` | Activity ID, amount and asset | Required successful verified operation and nonempty receipt |
| `approval` | Approval identity, agent and lifecycle state | Optional activity receipt |
| `program` | Program identity, lifecycle, version, code hash and receipt digest | Optional activity receipt; committed market events also use this kind |

The internal event-source crate uses plural kind names (`journeys`, `payments`,
`approvals`, `programs`), while webhook trigger routes use the singular words
above. Event-source observation journals a record before its
`GET /internal/v1/events/{id}` reader returns it
(`services/platform/hosted/internal/src/events.rs`). Webhook producers trigger
`POST /internal/v1/events/{kind}/{source_event}`; ingress fetches that exact ID
with the corresponding source-reader bearer and validates its record.

For an activity-bearing record, the active verifier calls the node method
`pax_sequencerReceipt` and requires `rung = "committed"`. This retained method
name is a node API. It then fetches consensus `commit` and `block`, the Ethereum
block's transaction IDs via `eth_getBlockByNumber`, and a proved
`abci_query` for `/store/sequencer/key` at the preceding height. Verification
binds the receipt, transactions, governed keys, policy, commit and served block
hash. Missing, malformed or inconsistent evidence is refused. A successful
operation receives `Verification::BlockCommitted` (`src/trusted.rs`,
`src/chain.rs`).

Verification labels are `unverified`, `receipt-verified` and `block-committed`.
A verified fact carries its receipt digest. An event's overall label is the
weakest of its facts, not the strongest receipt attached to it.

For a payment, `settled_payment_at` requires a zero result code, a receipt and
at least `receipt-verified`. It marks `state = settled`, activity ID and receipt
byte count at the operation's verified level. Amount and asset remain
`unverified` source facts. Therefore a generic payment event's overall label is
`unverified`, even when its operation is block committed. An endpoint filtering
above `unverified` excludes that event. Non-payment source facts likewise remain
unverified; verified activity ID and result code do not promote unrelated facts
(`src/events.rs`, `src/trusted.rs`).

Publication stores the event and pending deliveries in the principal's Redis
shard before transmission. Identical republishing returns `duplicate = true`;
reusing an ID with different content yields `EventConflict`. A new event must
advance its subject sequence or publication yields `OrderViolation`.

### Committed AI market events

`src/ai_market.rs` adds a durable per-principal market outbox. Enable it with
`LAYERX_WEBHOOKS_MARKET_OUTBOX_DIR` and supply
`LAYERX_WEBHOOKS_MARKET_POLICY_FILE`; it also reads the shared cursor key.
Capture is producer-authenticated at
`POST /internal/v1/ai-market/captures/{principal}`. The outbox verifies the capture
policy and committed evidence before releasing events to normal delivery;
conflicting evidence quarantines a stream. Persist the outbox directory across
process restarts; it is not interchangeable with the Redis delivery shard.

Developer control routes under
`/v1/webhooks/endpoints/{endpoint}/markets/{market}` provide POST `grants`, POST
`revocations`, and GET `events`. The endpoint must belong to the session
principal and the market ID is 32 bytes in hex. Market event pages accept
`limit` from 1 to 32, default 32, and authenticated resume cursors. Market access
is checked separately from cursor authenticity; a cursor grants no access.
Expired market history produces an explicit resynchronization response. When
the outbox is disabled these routes return `503 market_outbox_unavailable`.

---

## Trusted-component credential

Public TLS does not require a client certificate; developer routes require the
session checks above. Ingress always requires TLS and
`LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER`. After certificate-chain verification,
the leaf must have `clientAuth` extended key usage and exactly one recognized
role URI SAN:

| Certificate URI SAN | Allowed operation | Additional bearer |
| --- | --- | --- |
| `urn:layerx:webhooks:role:producer` | Event triggers, source admission readiness and market capture | `LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE` |
| `urn:layerx:webhooks:role:operator` | Explicit dispatch | `LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE` |

CA membership alone does not grant a role. Missing, duplicate, contradictory or
unknown role markers are refused. A wrong route role yields
`producer_role_required` or `operator_role_required` (`403`); an invalid peer
role yields `403 peer_role_refused`. A missing or wrong bearer yields the
route's `401 source_authentication_required` or
`operator_authentication_required`. Source and operator secrets must be
distinct or startup fails (`src/main.rs`, `src/trusted.rs`).

Public startup refuses ingress CA/trigger/operator settings; ingress startup
refuses the public identity-token setting. Set credentials per role rather than
injecting every secret into both processes. Ingress refuses plain application
listeners. The binary supports a public plain listener explicitly configured
without TLS certificate variables; the Railway init selects TLS for both roles.

Outbound trusted HTTPS uses the internal CA and PKCS#12 client identity plus
the destination's bearer: identity introspection, KMS, canonical source readers,
and the active node/consensus verifier. Outbound subscriber deliveries instead
use the public CA and no client identity. Redis requires `rediss://` and Redis
AUTH username/password (`src/hosted.rs`, `src/boundary.rs`).

---

## Delivery semantics

The durable shard records an event, endpoint deliveries and queue positions
before any subscriber POST. Dispatch claims a due head row with an in-flight
lease before sending. The per-endpoint, per-subject queue preserves ordering;
an outstanding head blocks later rows of that queue. Lease expiry allows a
replica to retry after a crash. A receiver can see the same event more than
once if acceptance occurred before the sender persisted the outcome.

| State | Meaning |
| --- | --- |
| `pending` | Queued, no attempt yet |
| `in-flight` | An attempt owns a lease |
| `retrying` | Failed attempt with a scheduled retry |
| `delivered` | Subscriber returned HTTP `200`–`299` |
| `dead-lettered` | Permanent failure or attempt limit reached |

The default policy is 8 attempts, a 10-second base delay, 3600-second delay
cap, 20% deterministic spread, suspension after 20 consecutive dead letters,
and a 120-second in-flight lease. Backoff doubles from the base and spreads by
the delivery digest. HTTP `410` is permanent `Gone`; redirects are refused.
Success clears the endpoint's consecutive dead-letter count. Suspended
endpoints remain queued and are skipped rather than reported as delivered
(`src/endpoints.rs`, `src/hosted.rs`).

### Signatures and receiver obligations

The service signs the UTF-8 prefix `<event-id>.<timestamp>.` followed by the
**exact body bytes**, using the endpoint's Ed25519 signing key. It verifies the
KMS signature locally before sending. Headers are:

```text
layerx-webhook-id:        <event-id>
layerx-webhook-timestamp: <unix-seconds>
layerx-webhook-key-id:    <endpoint-signing-key-id>
layerx-webhook-signature: v1=<standard padded base64 of a 64-byte Ed25519 signature>
```

Verify the signature before parsing or applying the body. The event ID and body
remain the same across attempts and redelivery. Reject stale timestamps and
deduplicate by event ID before side effects; the default acceptance window is
300 seconds, with at most 30 seconds of future skew. `ReplayGuard` rejects a
repeat within its window. Applications need durable deduplication when side
effects must survive receiver restarts.

Operational `layerx-webhook-delivery`, `layerx-webhook-attempt`,
`layerx-webhook-kind`, `layerx-webhook-subject`, `layerx-webhook-sequence` and
`layerx-webhook-endpoint` headers are outside the signature. Read authoritative
values from the signed body. `GET /v1/webhooks/scheme` and registration responses
publish the receiver obligation (`src/scheme.rs`).

### Replay and retention

GET endpoint events returns a signed cursor scoped to principal and endpoint.
POST endpoint `redeliveries` queues retained events from that cursor, with an
idempotency key. POST `/v1/webhooks/dead-letters/{id}/replay` requires the original
delivery to be dead-lettered. Replay enqueues delivery work; it does not alter
the event's committed history or make a failed HTTP attempt successful.

Retention is bounded. A generic cursor below the retained floor yields
`410 cursor_expired`; forged or foreign cursors yield `400 invalid_cursor`.
Persist shared cursor keys and Redis data across restarts. Cursor continuity
depends on both authentication keys and retained history, not just a running
process (`src/hosted.rs`).

---

## HTTP surface

The public role returns `404` for internal paths, and ingress returns `404` for
developer paths. `/healthz` is available on the application listener and on the
optional dedicated health listener; the health listener serves no other routes.
Ingress application TLS still requires a client certificate even for health.

| Method | Path | Role / authentication | Success |
| --- | --- | --- | --- |
| GET | `/healthz` | Either role; health listener needs no application bearer | `200` or `503` |
| GET | `/v1/webhooks/scheme` | Public; no session | `200` |
| GET / POST | `/v1/webhooks/endpoints` | Public; session; POST needs idempotency key | `200` / `201` |
| GET | `/v1/webhooks/endpoints/{id}/events` | Public; session | `200` |
| GET / POST | `/v1/webhooks/endpoints/{id}/keys` | Public; session; POST needs idempotency key | `200` / `201` |
| POST | `/v1/webhooks/endpoints/{id}/redeliveries` | Public; session and idempotency key | `202` |
| POST | `/v1/webhooks/endpoints/{id}/suspensions` | Public; session | `200` |
| POST | `/v1/webhooks/endpoints/{id}/resumptions` | Public; session | `200` |
| GET | `/v1/webhooks/events` | Public; session | `200` |
| GET | `/v1/webhooks/deliveries` | Public; session | `200` |
| GET | `/v1/webhooks/dead-letters` | Public; session | `200` |
| POST | `/v1/webhooks/dead-letters/{id}/replay` | Public; session and idempotency key | `202` |
| GET | `/internal/v1/readiness/{kind}` | Ingress; producer certificate and source-trigger bearer | `200` |
| POST | `/internal/v1/events/{kind}/{source_event}` | Ingress; producer certificate and source-trigger bearer | `202` |
| POST | `/internal/v1/dispatch` | Ingress; operator certificate and operator bearer | `200` |
| POST | `/internal/v1/ai-market/captures/{principal}` | Ingress; producer certificate and source-trigger bearer | `202` |

Market control/resume routes are described under [Event contracts](#event-contracts).
All public cookie mutations require CSRF, including those whose table rows list
an idempotency key. Generic page `limit` defaults to 50 and clamps to 1–200;
market pages use their separate bound. The process caps concurrent connections
at 256 and HTTP requests at 128 KiB (`src/main.rs`, `src/http.rs`).

---

## Refusals

Generic errors map as follows (`src/main.rs`, `src/error.rs`):

| HTTP | Error code(s) |
| --- | --- |
| `400` | `invalid_request`, `invalid_cursor`, `stale_timestamp`, `idempotency_key_required` |
| `401` | `session_required`, `source_authentication_required`, `operator_authentication_required`, `signature_rejected` |
| `403` | `peer_role_refused`, `producer_role_required`, `operator_role_required` |
| `404` | `unknown_endpoint`, `unknown_delivery`, `not_found` |
| `409` | `not_dead_lettered`, `endpoint_suspended`, `conflict`, `order_violation`, `replay_rejected` |
| `410` | `cursor_expired` |
| `422` | `verification_required`, `verification_refused` |
| `503` | `replay_capacity` (retry 10 seconds), `entropy_unavailable`, `dependency_unavailable`, `encoding_failed` (retry 5 seconds for the latter three) |

The JSON refusal body is `{"error":{"code":"...","retry":...}}`. When present,
retry seconds also appear in `Retry-After`. Market routes additionally use
`invalid_market_capture` (`400`), `market_access_refused` (`403`),
`commit_rejected` (`422`), `market_quarantined` (`409`), and
`market_capture_stopped` (`503`, retry 10 seconds). Producer admission readiness
also refuses invalid bodies/deadlines and unavailable dependencies; it does not
turn a failed evidence fetch into a verified event.

---

## Readiness

Public `/healthz` checks durable delivery state and signer readiness. Ingress
also checks canonical sources and the node receipt verifier. Responses name
`delivery_state_and_signer` and, for ingress,
`canonical_sources_and_node_receipts`. Overall readiness is `200` only when the
role's dependencies are ready, otherwise `503`, within a shared five-second
readiness budget (`src/main.rs`).

Delivery readiness checks Redis `PING`/`PONG` and KMS `/readyz`, including its
`ready` and `ed25519_non_exportable` response fields. Canonical source readiness
checks each source `/readyz`; the node verifier checks root JSON-RPC
`eth_chainId` and consensus `status`, including the consensus policy chain-name
match. Readiness does not replace per-event receipt and commit verification.

The Kubernetes public probe uses HTTPS application `/healthz`; ingress probes
its dedicated HTTP health port `9447`. Both use TCP application-listener
liveness. The separate producer route `/internal/v1/readiness/{kind}` checks the
source reader admission document and delivery readiness, and accepts an optional
`x-layerx-admission-deadline-ms` bounded to five seconds.

---

## Config keys

All binary environment names below begin with `LAYERX_WEBHOOKS_`. File settings
contain paths, not inline secrets. Shared settings are read by both roles.

| Suffix | Use / default |
| --- | --- |
| `ROLE` | Required `public` or `ingress` |
| `LISTEN` | Application bind; direct binary default `0.0.0.0:9444` |
| `LISTENER` | Default `tls`; ingress requires TLS |
| `HEALTH_LISTEN` | Optional separate plain health bind |
| `TLS_CERT_DER`, `TLS_KEY_DER` | TLS certificate and PKCS#8 server key |
| `INGRESS_CLIENT_CA_DER` | Required for ingress client authentication; excluded from public role |
| `INTERNAL_CA_DER`, `PUBLIC_CA_DER` | Trusted dependency/Redis CA and subscriber-delivery CA |
| `CLIENT_IDENTITY_PKCS12`, `CLIENT_IDENTITY_PASSWORD_FILE` | Outbound trusted client identity |
| `REDIS_URL`, `REDIS_USERNAME_FILE`, `REDIS_PASSWORD_FILE` | Durable store; URL must use `rediss://` |
| `KMS_URL`, `KMS_TOKEN_FILE` | Signing key creation, signing and readiness |
| `CURSOR_KEY_FILE` | 32-byte hex HMAC key |
| `INSTANCE_ID` | Required replica-specific lease prefix |
| `IDENTITY_URL`, `IDENTITY_TOKEN_FILE` | Public session introspection |
| `{JOURNEY,PAYMENT,APPROVAL,PROGRAM}_SOURCE_URL` | Ingress canonical source origins |
| `{JOURNEY,PAYMENT,APPROVAL,PROGRAM}_SOURCE_TOKEN_FILE` | Ingress canonical source-reader bearers |
| `SOURCE_TRIGGER_TOKEN_FILE`, `OPERATOR_TOKEN_FILE` | Ingress trigger and dispatch bearers; must differ |
| `NODE_URL`, `COMET_URL`, `NODE_TOKEN_FILE`, `CHAIN_POLICY_FILE` | Required active ingress receipt verifier |
| `MARKET_OUTBOX_DIR`, `MARKET_POLICY_FILE` | Optional durable market outbox and required policy when enabled |
| `DISPATCH_INTERVAL_SECONDS`, `DISPATCH_BUDGET` | Positive interval, default `1`; attempts per tick, default `64` |
| `RETENTION_EVENTS` | Default `10000`, clamped to 1–20000 |
| `KEY_OVERLAP_SECONDS` | Default `86400` |
| `BASE_DELAY_SECONDS`, `MAXIMUM_DELAY_SECONDS` | Default `10`, `3600` |
| `MAXIMUM_ATTEMPTS`, `SPREAD_PERCENT` | Default `8`, `20` |
| `SUSPEND_AFTER_DEAD_LETTERS`, `LEASE_SECONDS` | Default `20`, `120` |

`TrustedSources::from_environment` and `ReceiptVerifier::from_environment`
read the active source and node-verification configuration.

---

## Tests

Existing coverage lives under `services/platform/hosted/webhooks/` and
`services/platform/hosted/internal/`. The webhook boundary tests cover public
address restrictions, including translation and tunnel ranges. Signature and
verification-level tests cover the published scheme and committed labels.

`tests/fault-injection.sh` is a live-service exercise requiring a registered
subscriber, developer session/CSRF, a producer-role client certificate and
source-trigger bearer, canonical source event IDs, Kubernetes access and a real
receiver observation API. It asserts public rejection of internal routes,
durable delivery across pod deletion, duplicate publication, stale-sequence
refusal, order, byte-identical bodies and cursor redelivery. It uses separate
public and ingress URLs; a public session never authorizes a producer call.

Event-source tests in `services/platform/hosted/internal/src/events.rs` cover
principal binding and journal immutability/order. Market outbox and chain proof
coverage exercise committed event admission and recovery. These are coverage
locations, not a claim that a particular deployment has passed them.

---

## Make targets

`services/platform/Makefile.inc` defines workspace `platform-test`,
`platform-hosted-topology-check`, `platform-beta-cluster-up`,
`platform-real-agent-integration`, `platform-real-ios-integration` and
`platform-real-android-integration`. The real application integrations require
delivery evidence paths. There is no dedicated `platform-test-webhooks` target;
the live fault-injection script has its own required deployment inputs.

[Modules](Modules.md) · [Hosted dashboard](HostedDashboard.md) · [Home](Home.md)
