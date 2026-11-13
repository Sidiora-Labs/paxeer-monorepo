# Buyer middleware

`@sidiora/layerx-buyer-middleware` is the other half of a paid HTTP call: it reads a `402`, decides whether the price is acceptable, pays, and retries the request with proof.

## One call

`fetch(url, init, idempotencyKey)` does the whole loop:

1. Issues the request.
2. If the response is `402`, parses the `PAYMENT-REQUIRED` header into a typed offer.
3. Checks the offer against the kinds you declared as supported. An unsupported scheme or network is refused as `unsupported-payment`. Supported networks use `eip155:<chain-id>`, where the chain identifier is a positive 32-bit integer.
4. For an `exact` offer, quotes and commits the move through the SDK under a caller-supplied idempotency key, then waits for the journey to settle.
5. Retries the original request with a `PAYMENT-SIGNATURE` header carrying the receipt evidence.
6. Reads the `PAYMENT-RESPONSE` header and returns the response together with the captured settlement.

## Or the pieces

| Method | Use it when |
|---|---|
| `parseOffer(header)` | You want to inspect or price-check an offer without paying |
| `prepare(header, idempotencyKey)` | You want to pay now and retry later, or pay under your own key |
| `captureSettlement(...)` | You issued the retry yourself and want the settlement parsed and verified |
| `fetch(url, init, idempotencyKey)` | You want the whole loop |

## Keys and retries

The idempotency key you pass to `prepare` is the one that protects the payment. Derive it from what you are buying - the URL plus a request digest, an order identifier - not from the attempt. The middleware's retry policy governs how many times it will re-quote, re-commit and re-poll; the key governs whether any of that can cost you twice. Both matter, and only the key is a correctness property.

The retry policy is configurable and validated: quote and commit attempt counts must be positive safe integers; journey polls and the base delay may be zero. Configure `receiptKeys` with independently trusted, height-ranged receipt keys. A `committed` offer also requires a `commitments` resolver. Metered and subscription payments use `grantHeader` and `fetchGrant` with an already signed grant transaction rather than the exact-payment `prepare` flow.

## The transport

`LayerXPaymentHttpTransport` implements the SDK's `ProductionTransport` over the human plane. It refuses a non-loopback `http://` base URL outright, holds the bearer token in `SecretBytes`, sets `Idempotency-Key` when the call carries one, and maps HTTP failures onto the SDK error taxonomy - `429` to `rate-limit` with the carried retry timing, `409` to `idempotency-conflict`, `400` and `422` to `invalid-argument`. It only speaks the four operations a buyer needs: `move.quote`, `move.commit`, `journey.get` and `evidence.get`.

## Verification is not optional here either

A captured settlement is checked against `PortableReceiptV2` evidence: signed receipt bytes, the bound transaction and pinned receipt keys. Verification also matches the payment requirements. The offer's `extra.layerx` block identifies its commitment, account and currency; account derivation must match `payTo`. A `PAYMENT-RESPONSE` success claim alone does not produce a verified settlement.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The move behind the payment applied whole or not at all. |
| Replay refusal | `protocol` | The same signed transaction cannot apply twice. |
| Offline receipt verification | `protocol` | The captured settlement is checked against receipt evidence. |
| Idempotent money moves | `service` | The commit carries the caller's key for reconciliation with the original journey. |
| Quote then commit | `service` | The price you accepted from the offer is the price that is committed. |
