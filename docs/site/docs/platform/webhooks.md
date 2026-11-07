# Hosted webhooks

`layerx-webhooks` delivers journey, payment, approval and program events to
subscriber HTTPS endpoints. The crate is `layerx-platform-webhooks` under
`services/platform/hosted/webhooks/`. Delivery is signed with Ed25519, ordered per
endpoint and subject, retried with bounded backoff, and recoverable through dead
letters and cursor redelivery. A delivery becomes `delivered` only after the
subscriber endpoint returns a successful HTTP status.

Paxeer is one BFT network. Its kernel executes operations and proposes blocks as
part of the network; a receipt is final when its block commits. Webhook delivery
is an off-chain notification of those facts. Receipt verification and subscriber
HTTP acceptance are separate states: accepting a webhook does not finalize an
operation on chain.

The public node JSON-RPC uses the root path and EVM chain id `125` (`0x7d`); see
[Public RPC](../reference/public-rpc.md). The hosted webhook HTTP API described
here uses its own developer and private ingress routes.

## Deployment

The binary requires `LAYERX_WEBHOOKS_ROLE=public` or `ingress`. Both roles use the
same durable delivery store, signing service and cursor key, and both run a
background dispatcher. Role-specific routes and credentials are defined in
`services/platform/hosted/webhooks/src/main.rs` and `src/trusted.rs`.

| Role | Routes | Inbound authentication |
| --- | --- | --- |
| `public` | Developer API under `/v1/webhooks/` | Developer session; public TLS does not request a client certificate |
| `ingress` | Producer and operator routes under `/internal/` | Internal-CA client certificate with a role URI SAN, plus the matching bearer token |

The public process returns `404 not_found` for internal paths, even when a source
token is supplied. The ingress process returns `404 not_found` for developer
routes, including the signature scheme route. `GET /healthz` is available on
either role; TLS ingress still requires a client certificate to reach it. An
optional separate plain HTTP health listener serves only `GET /healthz`.

### Railway packaging

`docker/platform-webhooks/Dockerfile` provides `env-init-prebuilt` and `env-init`
targets for `webhooks-public` and `webhooks-ingress`. The prebuilt target checks
the binary SHA-256. `layerx-env-files` decodes base64 file variables according to
`docker/platform-webhooks/files.tsv`, then invokes
`services/platform/hosted/webhooks/env-init.sh`. Set
`LAYERX_FILES_OWNER=65532:65532`; the init script starts the binary as uid `65532`
and assigns each replica an instance id from `RAILWAY_REPLICA_ID` or its hostname.

| Init role (`LAYERX_ROLE`) | Default TLS address | Default plain health address |
| --- | --- | --- |
| `public` | `[::]:9444` | `[::]:9442` |
| `ingress` | `[::]:9443` | `[::]:8080` |

`LAYERX_WEBHOOKS_LISTEN` overrides the TLS address. The script chooses the health
address from `LAYERX_HEALTH_ADDR`, then `[::]:$PORT`, then the role default.
`PORT` changes only that health listener. The script exports the binary's
`LAYERX_WEBHOOKS_ROLE`, `LAYERX_WEBHOOKS_LISTENER=tls` and
`LAYERX_WEBHOOKS_HEALTH_LISTEN` settings.

Supply operator-specific dependency values for
`services/platform/hosted/webhooks/railway.env.example`:

| Dependency | Example URL |
| --- | --- |
| Redis | `rediss://OPERATOR_HOST:6379` |
| KMS | `https://OPERATOR_HOST:9443` |
| Developer identity (public role) | `https://OPERATOR_HOST:9443` |
| Journey source | `https://OPERATOR_HOST:9443` |
| Payment source | `https://OPERATOR_HOST:9443` |
| Approval source | `https://OPERATOR_HOST:9443` |
| Program source | `https://OPERATOR_HOST:9443` |

Replace `OPERATOR_HOST` with the operator-supplied hostname of the dependency named in that row.

Provision the node receipt dependencies below for the ingress role as well.

### Node receipt trust and network identity

The live `TrustedSources` constructor uses `ReceiptVerifier`, which reads these
four required inputs:

| Key | Required value |
| --- | --- |
| `LAYERX_WEBHOOKS_NODE_URL` | HTTPS node JSON-RPC base URL, serving methods at `/` |
| `LAYERX_WEBHOOKS_COMET_URL` | HTTPS CometBFT JSON-RPC base URL for the same chain, serving methods at `/` |
| `LAYERX_WEBHOOKS_NODE_TOKEN_FILE` | File containing the bearer used for both node and Comet requests |
| `LAYERX_WEBHOOKS_CHAIN_POLICY_FILE` | Mounted JSON `ChainPolicy` with the trusted consensus chain id and validator pins |

Outbound node requests also use the configured internal CA and PKCS#12 client
identity. Supply authenticated HTTPS boundaries for these connections; the
adapter does not accept plain HTTP URLs.

`ChainPolicy` in `src/chain.rs` contains `chain_id` and `validators`. Each
validator contains `public_key` (an array of 32 byte values) and `voting_power`
(a positive integer). Provision the policy from the trusted consensus
configuration for the deployment. The policy requires a nonempty consensus
chain id of at most 50 bytes, 1–256 distinct valid Ed25519 validator keys, and
bounded total voting power. The consensus chain id must match CometBFT
`status.result.node_info.network` and the committed header. The EVM chain id
`125` is a different identifier and does not replace this string.

For an activity, the live verifier:

1. Calls node method `pax_sequencerReceipt` with the `0x`-prefixed activity id
   and requires `rung` to be `committed`.
2. Fetches CometBFT `commit` and `block`, node `eth_getBlockByNumber`, and an
   `abci_query` proof for `/store/sequencer/key`, key `0x03`, at the preceding
   height.
3. Verifies the commit against the pinned chain and validator set, requiring
   signatures representing more than two thirds of voting power. It checks the
   governed receipt key membership proof, receipt signature and activity,
   transaction/batch evidence, and equality with the served block hash.
4. Assigns `block-committed` only after those checks pass. Missing or rejected
   evidence refuses the event instead of upgrading its verification level.

The `sequencer` names in this node method and store key are current code
identifiers for the network's proposer/receipt machinery.

**Deployment examples need additional provisioning.** The checked-in Railway
example, init script/file table, and Kubernetes webhook manifest do not supply
the four live node inputs above. Mount the node token and policy through the
deployment's file provisioning and set their file paths explicitly; the current
Railway file table has no rows that create them. Without these inputs, ingress
startup fails. The examples' old component/authority URLs, static
`WEBHOOKS_SEQUENCER_*` files, `LAYERX_WEBHOOKS_NETWORK_ID` and
`LAYERX_WEBHOOKS_LXP_WIRE_VERSION` do not configure the live verifier. The router
network-name interpolation in the Railway example is retired and must be
replaced by the trusted chain policy provisioning described here.

`AuthorityReceiptVerifier` remains a library adapter for component/authority
documents; it is not selected by the running ingress constructor. The Kubernetes
ConfigMap now sets its legacy network label to `layerx-testnet`; the previous
`testnet` versus `layerx-testnet` mismatch is not the current manifest value.
Neither that label nor the legacy wire-version setting establishes live node
receipt trust.

### Kubernetes routing

`services/platform/hosted/webhooks/deployment.yaml` defines three replicas each
of `layerx-webhooks-public` and `layerx-webhooks` (the private ingress), with
role-specific selectors. Both TLS containers listen on `9444`; Services expose
port `443`. The `layerx-developer` Ingress routes `/v1/webhooks` on
`dev.paxeer.network` to `layerx-webhooks-public`. Producers address the private
`layerx-webhooks` Service. The ingress deployment uses a separate plain health
listener on `9447`. Keep that health port private. The manifest still needs the
live node URL, token and policy inputs described above.

## Subscriptions

A subscription is a `StoredEndpoint` owned by an authenticated developer
`Principal`. Public authentication accepts `Authorization: Bearer <session>` or
cookie `__Host-layerx-session`. The public process introspects the session with
`POST /v1/sessions/introspect` at `LAYERX_WEBHOOKS_IDENTITY_URL`, using its own
identity bearer and outbound mTLS identity. The session must be active and its
`sub` must be a valid principal. Cookie-authenticated `POST` and `DELETE`
requests also require `x-layerx-csrf` matching the introspected `csrf_token`.
Authentication failure returns `401 session_required`.

Register with `POST /v1/webhooks/endpoints` and an `Idempotency-Key` header.
The JSON body accepts `url`, optional `kinds`, and optional
`minimum_verification`; unknown fields are rejected. Missing or empty `kinds`
means all four event families, and the default minimum is `unverified`.

The destination must use HTTPS with a canonical DNS name, rather than an IP
literal, and resolve only to public addresses. The public outbound client
checks destination addresses when connecting and refuses redirects. There are
at most 32 endpoints per principal. Reusing an idempotency scope returns the
existing registration; registering an already registered URL under a new scope
returns `409 conflict`.

Registration returns `201` with the endpoint id, signing key id, public key and
`receiver_obligation`. Endpoint ids start with `whep_`. KMS creates
non-exportable Ed25519 signing keys through `POST /v1/signing-keys`, with purpose
`layerx-webhook-v1`; their ids start with `whk_`. Rotation announces a pending
key that activates after `LAYERX_WEBHOOKS_KEY_OVERLAP_SECONDS` (default `86400`).
Fetch keys through the endpoint's `keys` route and retain keys needed to verify
in-flight deliveries during rotation.

Suspension accepts a reason of 1–256 bytes without CR, LF or NUL. Dispatch
skips suspended endpoints. Resumption clears the suspension and consecutive
dead-letter count; it does not manufacture delivery success.

Implementation: `src/hosted.rs` (`register`, `rotate_key`, `suspend`, `resume`),
`src/boundary.rs` (destination checks), and `src/trusted.rs` (sessions).

## Event contracts

| Kind | Wire word | Source and receipt requirements |
| --- | --- | --- |
| Journey | `journey` | Canonical source facts; an optional `activity_id` causes node receipt verification |
| Payment | `payment` | Requires `activity_id`, digit-only `amount`, `asset`, and a verified successful operation |
| Approval | `approval` | Canonical source facts; optional verified activity |
| Program | `program` | Canonical source facts; optional verified activity; the optional market outbox also emits program events |

An authenticated producer calls
`POST /internal/v1/events/{kind}/{source_event}` on ingress. Webhooks fetches the
immutable record at `GET /internal/v1/events/{id}` from the configured source for
that kind, using the source reader bearer and outbound mTLS. The producer
supplies an identifier, not trusted event facts in the trigger body. Source
observation/journaling is owned by
`services/platform/hosted/internal/src/events.rs`; ingress does not call its
observe route.

Source records contain `id`, `principal`, `subject`, `subject_sequence`,
`occurred_at`, `facts` (name/value pairs), and optional `activity_id`, `amount`
and `asset`. The fetched id must match the requested id, facts are bounded at
32, and unknown JSON fields are refused. Tenant ownership is the record's
principal, and subscriptions, delivery state and cursors are scoped to that
principal.

`ProtocolEvent` exposes its id, kind, principal, subject, subject sequence,
occurrence time, verification, optional receipt digest, and facts. Displayed
verification levels are `unverified`, `receipt-verified` and `block-committed`,
in that order. A verified fact carries a 64-hex receipt digest. The event header
uses the weakest fact's verification; it does not promote unverified fields.

For the live source path, successful node verification establishes
`block-committed` for activity-backed facts. Non-payment source facts remain
`unverified`; verified activity and result code facts are added when available.
A payment requires nonempty receipt bytes and `result_code == 0`. Its `state`
fact is `settled` at the verified operation level, alongside verified
`activity_id` and `receipt_bytes` facts. Its source `amount` and `asset` remain
`unverified`, so the overall payment event remains `unverified`. A subscription
with a higher minimum therefore filters it out. These rules are implemented in
`src/events.rs` and `TrustedSources::event` in `src/trusted.rs`.

### Optional AI market outbox

Setting `LAYERX_WEBHOOKS_MARKET_OUTBOX_DIR` enables `MarketHub`; it also requires
`LAYERX_WEBHOOKS_MARKET_POLICY_FILE` containing its `CapturePolicy` and the shared
cursor key file. The outbox directory must persist across restarts. With the
feature absent, market routes return `503 market_outbox_unavailable`.

Ingress accepts `POST /internal/v1/ai-market/captures/{principal}` with producer
mTLS and the source bearer. Public endpoint routes under
`/v1/webhooks/endpoints/{id}/markets/{market}` support `POST grants`,
`POST revocations` and `GET events`; the market id is 32 bytes as hex.
Market event pages default to 32 items and require `limit` in `1..=32`.
The background loop drains the durable outbox into the normal delivery store.
See `src/ai_market.rs`, `src/chain.rs` and the market handlers in `src/main.rs`
for capture evidence, grants, quarantine and resume contracts.

## Trusted-component credential

Public TLS uses a DER server certificate and PKCS#8 DER key without client
certificate authentication. Ingress TLS additionally requires
`LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER`; plain ingress mode is refused. After
TLS validates the client chain, `IngressRole::from_certificate` requires
`clientAuth` extended key usage and exactly one recognized role URI SAN:

| Client role | URI SAN | Allowed action | Additional credential |
| --- | --- | --- | --- |
| Producer | `urn:layerx:webhooks:role:producer` | Source triggers, trigger readiness and market captures | Source-trigger bearer |
| Operator | `urn:layerx:webhooks:role:operator` | Manual dispatch | Operator bearer |

CA membership alone grants neither role. Missing, duplicate, unknown or
contradictory role markers are refused. A valid operator leaf cannot publish,
and a valid producer leaf cannot dispatch. Source and operator bearer secrets
must differ; startup rejects shared secrets. Public configuration rejects the
ingress client CA and trigger token settings, while ingress rejects the public
identity token setting.

Use `tools/bringup/ca.sh` for certificate provisioning. Its `developer` row
issues the webhook server certificate, `developer-client` supplies the outbound
identity, and `webhook-operator-client` is the local operator identity. Producer
identities include `human-event-client` and `registry-event-client`. An outbound
`developer-client` certificate has no ingress role marker and cannot substitute
for a producer certificate. When connecting through a Railway TCP proxy, include
that DNS name in the server certificate through `LAYERX_EXTRA_SANS_<ROW>`.

| Credential | Connection |
| --- | --- |
| Internal CA + PKCS#12 client identity | Outbound HTTPS to KMS, identity, sources and node/Comet boundaries |
| Identity bearer (public only) | Session introspection |
| Per-kind source bearer (ingress only) | Matching source event fetch and readiness |
| Node bearer (ingress only) | Node and Comet JSON-RPC requests |
| Source-trigger bearer | Inbound producer calls on ingress |
| Operator bearer | Inbound dispatch on ingress |
| Redis username/password | Redis `AUTH` over `rediss://` with the internal CA |
| KMS bearer | Signing key creation, signatures and KMS readiness |
| Public CA, without client identity | Subscriber HTTPS delivery |

The Railway file table maps the shared `WEBHOOKS_INGRESS_TLS_CERT_DER` and
`WEBHOOKS_INGRESS_TLS_KEY_DER` variables to `tls/server.der` and
`tls/server-key.der` under `/run/layerx`. It maps `WEBHOOKS_CLIENT_P12`,
`WEBHOOKS_CLIENT_PASSWORD`, `WEBHOOKS_CLIENT_CA_DER` and `WEBHOOKS_PUBLIC_CA` to
`client/identity.p12`, `client/password`, `ca/internal.der` and `ca/public.der`.
Shared Redis, KMS and cursor values are `WEBHOOKS_REDIS_USERNAME`,
`WEBHOOKS_REDIS_PASSWORD`, `WEBHOOKS_KMS_TOKEN` and `WEBHOOKS_CURSOR_KEY`.
Public receives `WEBHOOKS_IDENTITY_TOKEN`; ingress receives the four
`WEBHOOKS_{JOURNEY,PAYMENT,APPROVAL,PROGRAM}_SOURCE_TOKEN` values plus
`WEBHOOKS_SOURCE_TRIGGER_TOKEN` and `WEBHOOKS_OPERATOR_TOKEN`. These are base64
file payloads; the runtime `LAYERX_WEBHOOKS_*_FILE` settings name decoded files.

## Delivery semantics

`HostedService::publish` atomically persists the event and pending deliveries in
a principal's Redis shard before any subscriber transmission. Repeating an id
with identical content returns `duplicate: true`; changed content returns
`409 conflict`. A new event must advance its subject sequence or it returns
`409 order_violation`. Only endpoints accepting the kind and minimum evidence
level receive a queued delivery.

Dispatch persists an in-flight lease before sending. Queues are scoped to an
endpoint and subject, and later entries wait behind their nonterminal head.
Replicas coordinate through Redis compare-and-set operations. After a crash,
an in-flight row becomes due again after the lease timeout; a stale worker
cannot settle a replacement worker's lease. If the destination accepted a POST
but the sender died before recording it, the retry can repeat that event.
Receivers must therefore make side effects idempotent and retain deduplication
state across receiver restarts. Redis persistence and availability are required
for recovery; process-local memory is not the delivery journal.

| State | Wire word | Meaning |
| --- | --- | --- |
| Pending | `pending` | Queued, never attempted |
| In flight | `in-flight` | Attempt outstanding with a lease |
| Retrying | `retrying` | Failed attempt with another scheduled |
| Delivered | `delivered` | Destination returned HTTP `200..299` |
| Dead letter | `dead-lettered` | Permanent failure or attempt budget exhausted |

Defaults are eight attempts, base delay `10s`, maximum delay `3600s`,
deterministic spread `20%`, suspension after 20 consecutive dead letters, and
in-flight timeout `120s`. Backoff doubles from the base and applies a spread
from the delivery digest. HTTP `410` is a permanent `gone` failure. Redirects
are refused; other unsuccessful statuses are failures subject to the retry
budget. Successful delivery resets consecutive dead letters.

### Signature and receiver obligation

The delivered JSON envelope contains `scheme`, `endpoint_id`,
`receiver_obligation` and `event`. The scheme is `LayerX/webhooks/v1`. KMS signs
this exact message with the endpoint's Ed25519 key:

```text
<event-id>.<unix-timestamp>. followed by the exact request body bytes
```

The sender verifies the returned KMS signature against the public key before
transmitting. Headers are:

```text
layerx-webhook-id:        <event-id>
layerx-webhook-timestamp: <unix-seconds>
layerx-webhook-key-id:    <endpoint-signing-key-id>
layerx-webhook-signature: v1=<standard padded base64 of a 64-byte Ed25519 signature>
```

Verify the signature against the selected trusted public key before parsing the
body, enforce timestamp age/future skew, then deduplicate on
`layerx-webhook-id` before applying side effects. The library defaults are a
`300s` age tolerance and `30s` future skew. `ReplayGuard` rejects a repeated id
within its window, but application deduplication must also cover later retries
and intentional redeliveries.

Retries and redeliveries to the same endpoint preserve the event id and body
bytes; their timestamp, signature, key and delivery-attempt metadata may
change. The `layerx-webhook-delivery`, `layerx-webhook-attempt`,
`layerx-webhook-kind`, `layerx-webhook-subject`, `layerx-webhook-sequence` and
`layerx-webhook-endpoint` headers are outside the signature. Use the signed
body's event facts and identity for business decisions.

The receiver obligation is returned by public `GET /v1/webhooks/scheme` and by
registration. Signature code is in `src/scheme.rs`; transmission and durable
state transitions are in `src/hosted.rs`.

### Redelivery, replay and retention

`GET /v1/webhooks/endpoints/{id}/events` returns a page and signed `next_cursor`.
`POST /v1/webhooks/endpoints/{id}/redeliveries` uses the `cursor` and `limit`
query parameters plus `Idempotency-Key` to enqueue that retained page again.
Its `202` result includes `queued`, `next_cursor` and `has_more`; it reports
queue admission, not subscriber acceptance. The same scope derives stable
replacement delivery ids.

`POST /v1/webhooks/dead-letters/{id}/replay` also requires `Idempotency-Key`.
It creates a replacement for a dead-lettered delivery and records the old
reference. A non-dead-lettered original returns `409 not_dead_lettered`.
Resume a suspended endpoint before expecting these queued deliveries to run.

Cursors are authenticated with the shared cursor key and scoped to principal
and endpoint. Retention defaults to 10,000 events per principal and is clamped
to `1..=20000`. Pruning removes oldest events only once all their deliveries
are terminal, and removes associated delivery/dead-letter records. A cursor
behind the retained floor returns `410 cursor_expired`; replay is limited to
retained data. Keep the cursor key stable across replicas and restarts.

## HTTP surface

Developer routes below are served only by the public role. Every mutation using
cookie authentication needs CSRF, including rotation, suspension, resumption,
redelivery and dead-letter replay. All owned routes require a session.

| Method | Path | Additional requirement | Success |
| --- | --- | --- | --- |
| GET | `/v1/webhooks/scheme` | No session | `200` |
| GET | `/v1/webhooks/endpoints` | Session | `200` |
| POST | `/v1/webhooks/endpoints` | `Idempotency-Key` | `201` |
| GET | `/v1/webhooks/endpoints/{id}/events` | Optional `cursor`, `limit` | `200` |
| GET | `/v1/webhooks/endpoints/{id}/keys` | Session | `200` |
| POST | `/v1/webhooks/endpoints/{id}/keys` | `Idempotency-Key` | `201` |
| POST | `/v1/webhooks/endpoints/{id}/redeliveries` | `Idempotency-Key`, optional `cursor`, `limit` | `202` |
| POST | `/v1/webhooks/endpoints/{id}/suspensions` | JSON `reason` | `200` |
| POST | `/v1/webhooks/endpoints/{id}/resumptions` | Session | `200` |
| GET | `/v1/webhooks/events` | Session | `200` |
| GET | `/v1/webhooks/deliveries` | Session | `200` |
| GET | `/v1/webhooks/dead-letters` | Session | `200` |
| POST | `/v1/webhooks/dead-letters/{id}/replay` | `Idempotency-Key` | `202` |

Private ingress routes require both the certificate role and bearer shown:

| Method | Path | Authorization | Success |
| --- | --- | --- | --- |
| GET | `/internal/v1/readiness/{kind}` | Producer + source-trigger bearer | `200` |
| POST | `/internal/v1/events/{kind}/{source_event}` | Producer + source-trigger bearer | `202` |
| POST | `/internal/v1/dispatch` | Operator + operator bearer | `200` |

Trigger readiness requires an empty body and supports absolute Unix milliseconds
in `x-layerx-admission-deadline-ms`, capped at a five-second budget. It checks
source reader admission, node receipt readiness and delivery state, and returns
schema `layerx.event-admission.v1`, role `webhook-trigger`, generation and
principal count, with `fresh_for_ms: 10000`. See the optional market routes
above for capture and tenant market access.

Ordinary page `limit` defaults to 50 and is clamped to `1..=200`. The service
allows at most 256 concurrent connections across API and health listeners.
The HTTP/1.1 request cap is `128 KiB`, including headers and body.

## Refusals

Refusals use `{"error":{"code":"...","retry":"never"}}`, or `retry: "after"`
with `retry_after_seconds` and an HTTP `Retry-After` header. The mapping in
`src/main.rs` includes:

| Status | Codes | Retry-After |
| --- | --- | --- |
| 400 | `invalid_request`, `invalid_cursor`, `stale_timestamp`, `idempotency_key_required`, `body_not_allowed`, `invalid_admission_deadline` | None |
| 401 | `session_required`, `source_authentication_required`, `operator_authentication_required`, `signature_rejected` | None |
| 403 | `peer_role_refused`, `producer_role_required`, `operator_role_required` | None |
| 404 | `not_found`, `unknown_endpoint`, `unknown_delivery` | None |
| 409 | `not_dead_lettered`, `endpoint_suspended`, `conflict`, `order_violation`, `replay_rejected` | None |
| 410 | `cursor_expired` | None |
| 422 | `verification_required`, `verification_refused` | None |
| 503 | `replay_capacity` | 10 seconds |
| 503 | `entropy_unavailable`, `dependency_unavailable`, `encoding_failed` | 5 seconds |
| 503 | `admission_deadline_exceeded`, `delivery_state_unavailable`, `market_outbox_unavailable` | None |

Invalid or missing ingress certificates can fail the TLS handshake before an
HTTP refusal exists. The optional market handlers additionally return
`invalid_market_capture` (400), `market_access_refused` (403),
`market_quarantined` (409), `commit_rejected` (422), and
`market_capture_stopped` (503, retry after 10 seconds).

## Readiness

`GET /healthz` returns `200` when the role's dependencies are ready, otherwise
`503`, using a shared five-second deadline. Public checks Redis and KMS and
reports component `delivery_state_and_signer`. Ingress also checks canonical
sources and node receipts under `canonical_sources_and_node_receipts`.

Redis readiness is `PING`/`PONG`. KMS `/readyz` must report `ready` and
`ed25519_non_exportable`. Each canonical source must return `200` from
`/readyz`. Node readiness calls `eth_chainId`; Comet readiness calls `status`
and matches the configured consensus chain id. These readiness probes do not
replace the commit and receipt evidence checks on each activity.

The Kubernetes public readiness probe uses HTTPS `/healthz` on the API port;
ingress uses HTTP `/healthz` on `9447`. Both use TCP liveness checks on their TLS
port. Railway uses the init script's separate health listener described above.

## Config keys

All names in this table have the prefix `LAYERX_WEBHOOKS_`. Values ending in
`_FILE`, certificate paths and identity paths name files, not inline secrets.

| Suffix | Role | Default or requirement |
| --- | --- | --- |
| `ROLE` | Both | Required: `public` or `ingress` |
| `LISTENER` | Both | `tls`; public also supports `plain` without TLS cert/key variables; ingress requires TLS |
| `LISTEN` | Both | Binary default `0.0.0.0:9444`; Railway init overrides per role |
| `HEALTH_LISTEN` | Both | Optional separate plain health address |
| `TLS_CERT_DER`, `TLS_KEY_DER` | TLS listeners | Required DER certificate and PKCS#8 DER private key |
| `INGRESS_CLIENT_CA_DER` | Ingress | Required inbound client CA |
| `INTERNAL_CA_DER` | Both | Required outbound internal/Redis CA |
| `PUBLIC_CA_DER` | Both | Required subscriber TLS CA |
| `CLIENT_IDENTITY_PKCS12`, `CLIENT_IDENTITY_PASSWORD_FILE` | Both | Required outbound mTLS identity/password |
| `REDIS_URL` | Both | Required `rediss://` URL |
| `REDIS_USERNAME_FILE`, `REDIS_PASSWORD_FILE` | Both | Required Redis credentials |
| `KMS_URL`, `KMS_TOKEN_FILE` | Both | Required HTTPS signer and bearer |
| `CURSOR_KEY_FILE` | Both | Required 32-byte hex HMAC key, shared across replicas |
| `IDENTITY_URL`, `IDENTITY_TOKEN_FILE` | Public | Required session introspection endpoint/bearer |
| `{JOURNEY,PAYMENT,APPROVAL,PROGRAM}_SOURCE_URL` | Ingress | Four required HTTPS source endpoints |
| `{JOURNEY,PAYMENT,APPROVAL,PROGRAM}_SOURCE_TOKEN_FILE` | Ingress | Four required source reader bearers |
| `SOURCE_TRIGGER_TOKEN_FILE`, `OPERATOR_TOKEN_FILE` | Ingress | Required, distinct inbound bearers |
| `NODE_URL`, `COMET_URL`, `NODE_TOKEN_FILE`, `CHAIN_POLICY_FILE` | Ingress | Required live node verification inputs; see deployment section |
| `INSTANCE_ID` | Both | Required unique instance token; Railway uses replica id, Kubernetes pod UID |
| `DISPATCH_INTERVAL_SECONDS` | Both | `1`, must be positive |
| `DISPATCH_BUDGET` | Both | `64` attempts per tick |
| `RETENTION_EVENTS` | Both | `10000`, clamped to `1..=20000` |
| `KEY_OVERLAP_SECONDS` | Both | `86400` |
| `BASE_DELAY_SECONDS`, `MAXIMUM_DELAY_SECONDS` | Both | `10`, `3600` |
| `MAXIMUM_ATTEMPTS`, `SPREAD_PERCENT` | Both | `8`, `20` |
| `SUSPEND_AFTER_DEAD_LETTERS`, `LEASE_SECONDS` | Both | `20`, `120` |
| `MARKET_OUTBOX_DIR`, `MARKET_POLICY_FILE` | Optional market feature | Durable directory and required capture policy when enabled |

## Tests

The crate contains destination IP guard tests in `src/boundary.rs`, verification
vocabulary tests in `src/events.rs`, and integration tests under `tests/`.
`plain_listener.rs` covers listener configuration. `repair_22_31_188.rs` includes
committed node evidence/foreign-chain refusal and authenticated capture restart
and durable handoff cases.

`tests/fault-injection.sh` exercises a live public service, private mTLS ingress,
canonical sources and subscriber observation API. It asserts that public
internal calls return 404, publishes two source events across webhook pod
deletion, checks duplicate acknowledgement and stale sequence refusal, observes
ordered delivery with stable body digests, then requests cursor redelivery and
checks the repeated body's digest. It requires real credentials and a receiver;
it is an operator qualification tool with cluster side effects.

## Make targets

`services/platform/Makefile.inc` exposes `platform-test` for the platform
workspaces and `platform-test-plain-listeners` for the listener integration
suite. Both use the repository runtime clock wrapper. The live fault-injection
script is separate from those targets. Running the documentation checker does
not qualify the service or exercise a deployed webhook receiver.

[Platform](index.md) · [Identity](identity.md) · [Home](../index.md)
