# Merchant middleware

`@sidiora/layerx-merchant-middleware` is a checkout built on top of the seller middleware. It prices a cart, opens an order, gates the resource on payment, and reconciles late settlement webhooks against the order it already has.

## Pricing

`quote(checkoutKey, lines)` turns a cart into a `MerchantQuote` and the `PAYMENT-REQUIRED` offer that goes with it. It is strict on purpose:

| Condition | Code |
|---|---|
| Empty cart, more than 256 lines, duplicate SKU, quantity outside 1–1,000,000 or not a safe integer | `invalid-cart` |
| A SKU the catalog does not have | `catalog-item-missing` |
| Two lines whose asset, `payTo`, scheme, network or serialized `extra` facts differ | `mixed-payment-facts` |
| A total that would exceed the 128-bit amount range | `amount-overflow` |

Amounts are integer strings and are multiplied and summed as `bigint`. No line total is ever computed in floating point, and no rounding step exists to argue about.

A cart produces one payment offer with one asset, recipient, scheme, network and set of extension facts. Use separate checkouts for items with different payment facts. The offer uses the first item's `maxTimeoutSeconds`.

Offers use `eip155:<chain-id>` networks and carry `extra.layerx` payment facts, including commitment, account and currency. The seller validates that the advertised account derives the offered `payTo`; buyers verify the same binding before paying.

## Checking out

`checkout(principal, checkoutKey, lines, paymentHeader?)` returns one of four results.

| Result | What happened |
|---|---|
| `payment-required` | The order is open and unpaid. Send `decision.headers` and `402` |
| `pending` | Settlement is pending, or an already paid order was opened without a payment header |
| `refused` | The payment was refused; the order is marked `refused` |
| `paid` | The receipt verified, the order is `paid-verified`, and it carries the receipt digest and transaction |

The order store is asked to `open` the order before the seller decision. The returned order must match the checkout key, request digest and quote. The middleware also checks `markPaid` and `markRefused` responses and raises `order-conflict` on a mismatch. Resource release is delegated to the supplied store's `releaseResource` method.

The request digest covers the lines, total, asset, recipient, scheme, network, timeout and complete payment offer, including its resource URL and extension facts. Identical quotes produce the same digest. A price or resource-URL change changes the digest; the supplied order store decides how checkout keys and digests map to orders.

## Late settlement

`MerchantSettlementWebhooks.consume(rawBody, headers)` handles the case where the payment lands after the HTTP exchange ended. It runs inside the verified webhook consumer, so signature, freshness and replay are already handled, and then:

1. Looks the order up by `order_id` and requires the event's `request_digest` to match the stored one.
2. Resolves the receipt reference to `PortableReceiptV2` evidence and checks the raw transaction hash against the event's transaction.
3. Verifies the signed receipt and payment requirements against independently pinned `receiptKeys`, using a commitment resolver when required.
4. Compares the verified receipt digest and transaction to `receipt_digest` and `transaction`, and refuses a claimed verification level above the offer's requirement.
5. Marks the order paid and checks the resulting order's identity, digest, state, receipt digest and transaction.

`MerchantMiddleware` takes `catalog`, `orders`, `sellers` and `resourceUrl`. `MerchantSettlementWebhooks` takes `verifier`, `orders`, `receipts`, `receiptKeys` and an optional `commitments` resolver. Neither constructor selects a receipt protocol through a `protocolVersion` option.

Webhook verification levels are `receipted` and `committed`. Unknown fields, malformed digests and invalid verification levels are refused as `invalid-webhook`. A webhook success claim alone cannot mark an order paid.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | Both the HTTP path and the webhook path verify signed receipt bytes against pinned receipt keys. |
| Conserved supply | `protocol` | The amount in the receipt is the amount that moved. |
| Quote then commit | `service` | The order is bound to the digest of the quote the buyer accepted. |
| Exactly-once fulfilment | `service` | Order state transitions are digest-checked on every store response; application fulfilment must use the verified order identity. |
| Verified, replay-protected webhooks | `service` | Signature, freshness, replay and receipt digest, in that order. |
