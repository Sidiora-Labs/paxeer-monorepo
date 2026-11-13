# Retries and unknown outcomes

A payment request can time out after submission. Keep its idempotency key and transaction reference so the outcome can be reconciled without creating a second payment.

## Idempotency keys

Every mutation on the human plane that could move money requires an `Idempotency-Key` header. You choose the key. Repeating the request with the same key returns the original journey rather than producing a second effect.

The SDKs enforce this before the request leaves your process. Calling `move.commit` without a key fails locally with `idempotency-required`; you do not discover the problem from a duplicate payment. Sending the same key with a different body is a conflict, not a silent overwrite.

Choose a key that is stable for the payment, not for the attempt. A good key is derived from the thing being paid for - an order identifier, a cart digest - so that a retry after a crash reuses it naturally. Persist a generated key before submission and reuse it for every attempt at that payment.

## What a retry is allowed to do

Human API refusals carry a retriability class in their error record:

| Class | What it means |
|---|---|
| `retriable` | Retry the same request as-is |
| `retriable-after` | Retry once the carried `retry_after_ms` has elapsed. Rate-limit refusals always classify here and always carry the timing |
| `structural` | Retrying cannot help until something around it changes - re-authenticate, reload |
| `final` | The refusal is final for this request; do not retry it unchanged |

The TypeScript production SDK uses `safe`, `after`, `never` and `unknown-outcome` in `PlatformSdkError.retry`. Its delay field is `retryAfterMs`; the human API error field is `retry_after_ms`. Keep those vocabularies separate when handling errors.

## Unknown

Sometimes a submission genuinely cannot be classified: the request left, the response did not come back, and nothing yet proves which side of the transition it landed on. LayerX answers `Unknown` on the agent plane and `still-checking` on the human plane.

An unknown outcome requires reconciliation, including when an SDK surfaces it as an `unknown-outcome` error:

1. Do not retry the payment. The idempotency key already protects you, but a retry does not answer the question either.
2. Resolve it using the stored payment identity. The human plane reconciles by idempotency key; agent `track` and `wait` use the submission reference for the signed transaction.
3. Keep every duplicate-capable control locked while it lasts. The human plane does this for you; do it in your own UI too.

Display the outcome as unresolved until evidence establishes whether the payment applied.

## Protocol-level replay

At the EVM boundary, the sender nonce prevents an applied signed transaction from applying again. Kernel activities additionally carry their protocol sequence. Reusing an application idempotency key resolves the original request; constructing a new transaction with a new nonce can create a separate payment.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Replay refusal | `protocol` | An already-applied activity is refused by the transition function. |
| Atomic settlement | `protocol` | A timed-out call left either a whole payment or none of one behind. |
| Unknown is a real outcome | `agent-layer` | An unclassifiable submission is reported as `Unknown` and resolved by tracking. |
| Idempotent money moves | `service` | The human service requires the key and returns the original journey on repeat. |
| Exactly-once fulfilment | `service` | Middleware can retain fulfilment under the payment identity; the application must coordinate its side effect with durable storage. |
| Hosted rate limits | `hosted-surface` | The gateway refuses excess traffic with honest retry timing. |
