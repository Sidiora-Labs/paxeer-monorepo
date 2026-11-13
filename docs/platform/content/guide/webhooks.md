# Webhooks

A webhook delivery must be authenticated before its payload is used. The TypeScript `VerifiedWebhookConsumer` is exported by `@sidiora/layerx-seller-middleware` and mounted by Express and Next.js. FastAPI and Spring Boot provide their own language implementations.

## The four checks, in order

1. **Shape.** The delivery id must be bounded text, the key id an identifier, the timestamp a canonical integer. Anything else is `invalid-webhook` before a byte of the body is parsed.
2. **Freshness.** More than 30 seconds in the future, or older than the maximum age (5 minutes by default), is refused. This is what bounds the window in which a captured delivery is worth anything.
3. **Signature.** Ed25519 over `"{delivery-id}.{timestamp}." || rawBody`, against the public key selected by `WebhookRequestHeaders.keyId`. The Express and Next.js HTTP header is `layerx-webhook-key-id`; the other delivery headers are `layerx-webhook-id`, `layerx-webhook-timestamp` and `layerx-webhook-signature`. Multiple keys can be declared at once. Keep the raw body bytes for signature verification.
4. **Replay.** The delivery store is asked to claim `(deliveryId, payloadDigest)` under a lease.

Only after all four does your handler run, and it runs with the parsed event and the delivery id.

## What the claim returns

| Claim | Result | Why |
|---|---|---|
| `claimed` | handler runs | The store granted a new lease, including after a processing lease expired |
| `completed` | `duplicate` | Already handled - a `200`, not a second effect |
| `processing` | `processing` | Another worker holds the lease |
| `conflict` | `webhook-replay` | Same delivery id, different payload digest |

`conflict` means the store already holds that delivery id with a different payload digest. The consumer raises `webhook-replay`.

If your handler throws, the lease is released for a retry. After successful handling, the delivery is marked completed; subsequent claims return `duplicate` while that record is retained. Make handler effects idempotent because a crash between the effect and completion can lead to another delivery.

## The store is yours, and it must be durable

The Express and Next.js packages each export `SingleProcessWebhookDeliveryStore`. These stores hold claims in memory for one process and lose them on restart. For durable processing, implement `WebhookDeliveryStore.claim`, `complete` and `release` against persistent storage and make handler effects idempotent. The consumer invokes the handler and completion separately, so storing claims durably alone does not make the business effect atomic with completion.

## Verifying rather than trusting the payload

A signed webhook proves the message came from the holder of the signing key. It does not prove a payment happened. Where the event asserts money - the merchant settlement event, for example - the merchant handler resolves the payment evidence, verifies the signed native receipt and transaction against pinned receipt keys, and compares the recomputed receipt digest to the event digest in constant time. Signature checking authenticates the messenger; receipt verification establishes the fact.

## Declaring keys

Express, Next.js and FastAPI read `LAYERX_WEBHOOK_PUBLIC_KEYS_JSON` and `LAYERX_WEBHOOK_PATH`. Spring Boot uses `layerx.webhook.public-keys` and `layerx.webhook.path`; its example maps individual environment values into those properties. When using the TypeScript `VerifiedWebhookConsumer` directly, pass the decoded key map as `publicKeys` and the delivery repository as `deliveries`. Verification keys are public; signing keys remain private.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | Money-bearing events are settled against receipts, not headers. |
| Verified, replay-protected webhooks | `service` | Shape, freshness, Ed25519 signature and replay claim, in that order. |
| Exactly-once fulfilment | `service` | Delivery claims suppress repeats while retained; handler effects must be idempotent across interrupted processing. |
| Refusal to publish a secret | `service` | Consumers receive public verification keys; signing keys stay with the publisher. |
