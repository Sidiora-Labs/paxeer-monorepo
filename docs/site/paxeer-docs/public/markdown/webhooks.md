<!-- Source: https://docs.paxeer.app/webhooks/ -->

# Webhooks

Receive signed Paxeer X events with durable delivery, per-subject ordering, bounded retries, and cursor-based replay.

Hosted webhooks deliver journey, payment, approval, and program events to your HTTPS receiver. Events and pending deliveries are persisted before transmission. Each endpoint has its own Ed25519 signing key, and retries preserve the event identifier and exact body bytes so your application can safely recognize repeats.

## Register an endpoint

Use an active developer session with `Authorization: Bearer`, or the `__Host-layerx-session` cookie. Cookie-authenticated POST and DELETE requests also carry `x-layerx-csrf` matching the session CSRF token. Registration requires an `Idempotency-Key`.

```
POST /v1/webhooks/endpoints
Authorization: Bearer <developer-session>
Idempotency-Key: <stable-registration-key>
Content-Type: application/json

{
  "url": "https://merchant.example.com/events/paxeer",
  "kinds": ["payment", "approval"],
  "minimum_verification": "unverified"
}
```

The URL uses a canonical DNS name and resolves exclusively to public addresses. Literal IP destinations are refused. Each developer principal can register up to 32 endpoints. Registration returns the endpoint identifier, signing key identifier, public key, and receiver obligation. Preserve the returned key in your receiver's trusted key configuration.

## Event families and evidence

| Kind | Resource and facts | Evidence |
| --- | --- | --- |
| `journey` | Journey identity, kind, state, updated time. | Source facts with optional activity receipt binding. |
| `payment` | Activity identity, amount, asset, settled state. | Successful verified receipt for settlement; source amount and asset retain their own evidence labels. |
| `approval` | Approval identity, agent, state, creation time. | Source facts with optional activity receipt binding. |
| `program` | Program identity, lifecycle, version, code hash, receipt digest. | Source facts with optional activity receipt binding. |

Webhook verification levels are `unverified`, `receipt-verified` and `block-committed`. Registering an endpoint with any other `minimum_verification` value is rejected as an invalid request; the legacy stored words `checkpoint-finalised` and `paxeer-finalised` are read back as `block-committed` but cannot be registered. Each fact retains its own level; the event header uses the weakest fact. A payment event can therefore have an unverified header while its settled-state fact carries receipt evidence. A minimum-verification filter applies to that aggregate header. Inspect and verify the facts your decision uses.

## Verify the raw delivery

```
layerx-webhook-id: <event-id>
layerx-webhook-timestamp: <unix-seconds>
layerx-webhook-key-id: <endpoint-key-id>
layerx-webhook-signature: v1=<padded-base64-ed25519-signature>

signed_message = UTF8(event_id + "." + timestamp + ".") || raw_body
```

1. Validate bounded identifiers and canonical integer timestamp syntax.
2. Reject signatures older than the accepted age or too far in the future. Defaults are 300 seconds of age and 30 seconds of future skew.
3. Verify Ed25519 against the trusted public key named by the key identifier, over the exact incoming body bytes.
4. Claim the event identifier and payload digest in a durable replay store before applying business effects.
5. For economic effects, resolve and verify the referenced receipt and compare its digest to the event evidence.

Read the body as raw bytes before JSON parsing. Re-serializing parsed JSON changes the signed message. Operational headers for delivery, attempt, kind, subject, sequence, and endpoint are outside the signature; use authenticated body fields for business decisions. `GET /v1/webhooks/scheme` publishes the receiver contract.

## Commit once, accept repeats

Delivery is at least once. Store the event identifier with its payload digest and business outcome in the same transactional database as your application effects. A completed repeat returns success without repeating fulfilment; an in-progress claim waits or retries; the same identifier with a different digest is a conflict. Release an unsuccessful processing lease so a legitimate retry can proceed.

The framework integrations provide verified webhook consumers and replay-store interfaces. An in-memory single-process store is useful for local execution; a durable shared store preserves deduplication across workers and restarts.

## Ordering, retries, and dead letters

Events are ordered per subject, rather than globally across every resource. Publish rejects a non-advancing subject sequence. Delivery progresses through pending, in-flight, retrying, delivered, and dead-lettered. Only an accepting 2xx status from your endpoint marks a delivery delivered.

| Default control | Value |
| --- | --- |
| Maximum attempts | 8 |
| Initial retry delay | 10 seconds |
| Maximum retry delay | 3,600 seconds |
| Deterministic backoff spread | 20% |
| In-flight lease | 120 seconds |
| Automatic suspension | 20 consecutive dead letters |

Backoff doubles up to its bound. Redirects are refused; HTTP 410 is a permanent failure. A permanent failure or exhausted attempts creates a dead letter. A suspended endpoint receives no dispatch until resumed. A lost worker's in-flight lease permits recovery without losing the persisted event.

## Manage delivery and replay

| Method and path | Purpose |
| --- | --- |
| `GET /v1/webhooks/endpoints` | List endpoint registrations. |
| `GET /v1/webhooks/endpoints/{id}/events` | Read endpoint event history and continuation cursor. |
| `GET /v1/webhooks/endpoints/{id}/keys` | Read current and overlapping verification keys. |
| `POST /v1/webhooks/endpoints/{id}/keys` | Rotate the signing key with an idempotency key. |
| `POST /v1/webhooks/endpoints/{id}/redeliveries` | Enqueue replay from a signed cursor with an idempotency key. |
| `POST /v1/webhooks/endpoints/{id}/suspensions` | Suspend an endpoint with a reason. |
| `POST /v1/webhooks/endpoints/{id}/resumptions` | Resume delivery and clear consecutive dead-letter accounting. |
| `GET /v1/webhooks/events`, `/deliveries`, `/dead-letters` | Inspect events and operational outcomes. |
| `POST /v1/webhooks/dead-letters/{id}/replay` | Retry a dead letter with an idempotency key. |

Pagination defaults to 50 items with a maximum of 200. Preserve cursors as opaque signed values. Expired retention returns `410 cursor_expired`; repair continuity using retained event and receipt history rather than silently assuming all events were delivered. Replay keeps the original event identifier and body digest.

## Rotate keys and monitor receivers

Key rotation announces a pending public key before activation; the default overlap is 86,400 seconds. Configure the new public key during the overlap while retaining the old key for eligible deliveries. Signing private keys stay with the signing service.

Monitor delivery latency, retry age, dead letters, suspension, and receiver signature failures. Return 2xx after durable acceptance. For long processing jobs, commit an inbox record and process it asynchronously. Use [agent subscriptions](https://docs.paxeer.app/subscriptions) for tenant-scoped streams with explicit acknowledgement and gap recovery.

**Reference:** `services/platform/hosted/webhooks/src/main.rs`, `hosted.rs`, `scheme.rs`, `events.rs`, `endpoints.rs` in the monorepo.
