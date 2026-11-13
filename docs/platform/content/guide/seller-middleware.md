# Seller middleware

`@sidiora/layerx-seller-middleware` verifies payment evidence before releasing an HTTP resource. [Express](../framework/express.md) and [Next.js](../framework/next.md) mount the TypeScript implementation. [FastAPI](../framework/fastapi.md) and [Spring Boot](../framework/spring.md) have separate language bindings and configuration. Use `SellerMiddleware` directly for a decision without framework routing.

## The shape

`SellerMiddleware` has two decision methods.

| Method | Returns |
|---|---|
| `paymentRequired()` | The `402` decision and its `PAYMENT-REQUIRED` header, for a request that carried no payment |
| `handle(...)` | `payment-required` when the payment header is absent; otherwise `pending`, `refused` or `released` |

Successful calls return one of these decisions. Invalid inputs or unverifiable evidence raise a typed error.

| Decision | HTTP | Meaning |
|---|---|---|
| `payment-required` | `402` | Pay this, exactly this |
| `pending` | `202` | The authority reports a pending outcome |
| `refused` | `402` or `409` | The payment was refused, and the settlement response says why |
| `released` | `200` | The receipt verified. Here is the resource, and here is the settlement header |

## What `handle` actually does

1. Decodes the `PAYMENT-SIGNATURE` header into a payment payload.
2. Matches the payload's accepted requirements against your declared requirements. A mismatch raises `requirements-mismatch`.
3. Computes the request digest and looks for a stored fulfilment associated with the transaction. A matching stored request can reuse its fulfilment without another authority call. Otherwise, it asks the payment authority to settle. `ReceiptPayloadAuthority` verifies the signed receipt, raw transaction and receipt digest against independently pinned keys.
4. Checks the payment event against the declared requirements and verifies the requested commitment level. For a new fulfilment, it binds the verified transaction to a claim and supplies a `release` callback to the fulfilment repository. A settled payment already bound to a different request returns a `409` refusal. Stored receipt evidence is checked again before returning a resource.
5. Returns the released resource and the `PAYMENT-RESPONSE` header.

Your resource handler runs only after payment verification and a fulfilment claim.

## Configuration

**The payment requirements.** `paymentRequired` declares the resource and accepted payment terms using x402 version `2`. Requirements bind the scheme, network, asset, amount and recipient. The middleware does not take a `protocolVersion` setting.

**The receipt authority and pinned keys.** Supply `receiptKeys` and an `authority`. `ReceiptPayloadAuthority(receiptKeys)` verifies portable native receipt evidence against pinned key IDs, public keys and height ranges. Obtain those pins independently of the receipt supplier. Requirements select `receipted` by default; set `extra.layerx.commitment` to `committed` to require finality. For `committed` payment requirements, also supply a `commitments` resolver that authenticates the block commit, validator set and transaction inclusion.

**The fulfilment repository.** The repository binds each payment claim to its request digest:

- Same idempotency key, same request digest: return the stored fulfilment. Do not call `release` again.
- Same settled payment, different request digest: the middleware returns a `409` refusal; the durable repository also rejects conflicting direct calls with `fulfillment-conflict`.
- New key: call `release`, store the result durably, return it.

Use a durable repository in production. `DurableFulfillmentRepository` persists a claim before release; an uncertain release remains unresolved and raises `fulfillment-outcome-unknown` rather than running the callback again. The release operation must honour the supplied idempotency key.

## Errors

`MiddlewareError` carries one of a closed set of codes: `invalid-payment-required`, `invalid-payment-payload`, `requirements-mismatch`, `unsupported-payment`, `payment-pending`, `payment-refused`, `verification-failure`, `fulfillment-conflict`, `fulfillment-outcome-unknown`, `invalid-webhook`, `webhook-replay`. Map them to status codes once, centrally.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | Signed execution receipts are checked locally against pinned keys; committed evidence also requires a commitment resolver. |
| Atomic settlement | `protocol` | A verified receipt describes a payment that happened whole. |
| Receipt-gated resource release | `service` | Binds requests that reach your service through this middleware, and nothing else. |
| Exactly-once fulfilment | `service` | Requires a durable claim repository and a release operation that honours its idempotency key. |
