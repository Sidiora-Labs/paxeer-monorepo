<!-- Source: https://docs.paxeer.app/subscriptions/ -->

# Event subscriptions

Build durable agent consumers with scoped visibility, acknowledged cursors, explicit gap recovery, and verified receipt references.

Paxeer X subscriptions let an agent observe activity without repeatedly rebuilding its view from individual queries. The contract persists the subscription definition, acknowledgement position, continuity state, and lifecycle controls in the agent daemon. An event carries its identity, bytes, deduplication identifier, cursor, and an optional verified receipt reference.

The Agent API routes `subscription.create`, `subscription.list` and `subscription.health` to the daemon's durable subscription store. Creation uses an *idempotent mutation*; list and health reads require the authenticated session's operation permit.

## Choose the event interface

| Interface | Consumer model | Recovery mechanism |
| --- | --- | --- |
| Agent subscriptions | Authenticated tenant, agent, and capability; explicit filter and delivery target. | Durable acknowledged cursor, backfill barrier, truncation notice. |
| [Hosted webhooks](https://docs.paxeer.app/webhooks) | Developer HTTPS endpoints for journey, payment, approval, and program families. | Signed delivery, retries, dead letters, and signed replay cursors. |

Choose the agent subscription interface when an autonomous runtime needs visibility constrained by its session capability. Choose webhooks when an external application receives hosted event families over HTTPS. Their cursor and verification representations belong to their respective interfaces.

## Create a scoped subscription

`subscription.create` requires `scope`, `filter`, `start`, and `delivery_target`, inside an idempotent mutation. The scope names tenant, agent, and capability and must equal the authenticated owner coordinates. The same key and request return the original record; a changed request under that key is refused.

| Filter field | Restriction |
| --- | --- |
| agents | Selected agent identities. |
| accounts | Selected accounts visible to the owner. |
| activity_types | Selected native operation types. |
| modules | Selected module activity. |
| assets | Selected assets. |
| counterparties | Selected interacting principals. |
| result_classes | Selected outcome classes. |

Evaluation applies tenant restriction first, capability scope second, and filter third. A filter narrows visibility and cannot expand a capability or disclose another tenant's records. Administrative list and health operations also require the authenticated session's operation permit.

## Consume and acknowledge

1. Restore the persisted subscription and last acknowledged cursor.
2. Receive an Event, Gap, or Truncated delivery.
3. For an Event, claim its `deduplication_id` in your durable consumer store.
4. Verify any receipt evidence required for the business decision.
5. Commit the application effect and deduplication record together.
6. Acknowledge the delivered cursor after that commit.

```
EventDelivery
  event_identity
  event_bytes
  deduplication_id
  cursor
  receipt_reference: None | Verified { receipt_ref, rung }

cursor = { block_height: "125", tx_index: "0", log_index: "0" }

subscription.acknowledge(scope, subscription_id, cursor)
```

Delivery is at least once. An acknowledgement records progress; it does not make a business handler transactional. If the process crashes after applying an effect but before acknowledging, the repeated delivery must find the completed deduplication record. Each cursor is an object with block_height, tx_index, and log_index fields encoded as canonical decimal strings. Preserve all three coordinates when saving or acknowledging progress.

The daemon refuses acknowledgements for never-delivered cursors and rejects regressing positions. Keep processing and acknowledgement ordered for a subscription instead of advancing past unfinished work.

## Resolve gaps before advancing

A Gap contains `missing_first`, `missing_last`, `backfill_cursor`, and `backfill_attempted`. It forms a continuity barrier: later events remain blocked until successful backfill or an explicit truncation notice. Persist that blocked state so a restart cannot accidentally cross the missing interval.

If retention no longer contains the requested interval, Truncated reports `requested_first`, `oldest_available`, and `resume_cursor`. Rebuild the application view from authoritative state or retained history, record the discontinuity, and continue from the declared resume point. Treat truncation as an explicit loss of continuity.

## Pause, resume, and inspect health

| Operation | Required fields | Use |
| --- | --- | --- |
| `subscription.list` | scope | Find subscriptions visible to the authenticated owner. |
| `subscription.health` | scope, subscription_id | Inspect acknowledged progress and pending backfill. |
| `subscription.acknowledge` | scope, subscription_id, cursor | Persist completed delivery progress. |
| `subscription.pause` | scope, subscription_id | Stop delivery temporarily while preserving position. |
| `subscription.resume` | scope, subscription_id | Continue under current authorization and continuity controls. |
| `subscription.delete` | scope, subscription_id | Terminate delivery while retaining its audit record. |

Health returns the target, `last_acknowledged`, `last_delivery_at`, and `pending_backfill`. The current daemon returns null for the delivery timestamp. Pending backfill reports the missing sequence range, including truncated continuity over the subscription's own coordinates.

## Revocation and restart safety

Session, capability, or tenant revocation stops the corresponding subscription. Durable termination distinguishes Deleted, SessionRevoked, CapabilityRevoked, and TenantRevoked. Session-bound access continues to require an authorized operation rather than relying on possession of a subscription identifier.

Restart restores definition, acknowledged position, unacknowledged deliveries, and continuity. Preserve the daemon store together with its tenant and session context. A corrupt or unavailable store refuses progress instead of silently resetting the cursor.

## Separate notification from proof

An event may carry no receipt reference. A verified reference names its receipt and rung with Paxeer as the settlement domain. Event arrival alone does not establish an economic outcome. The Agent API reports `submitted`, `receipted` and `committed`; a verified receipt reference requires at least receipted. For settlement-sensitive automation, retrieve the evidence and require committed block evidence.

Keep local approval and policy notifications distinguishable from protocol receipts. See [Agents on Paxeer X](https://docs.paxeer.app/agents) for the write lifecycle and [Agent runtime](https://docs.paxeer.app/agent-runtime) for capability, readiness, and recovery controls.

**Reference:** `services/agent/crates/layerx-agent-api/src/stream.rs` and `services/agent/crates/layerx-agentd/src/agent_rpc_adapters.rs`; `services/agent/crates/layerx-agentd/src/events/subscription.rs` in the monorepo.
