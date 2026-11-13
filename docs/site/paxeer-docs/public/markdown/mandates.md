<!-- Source: https://docs.paxeer.app/mandates/ -->

# Payment mandates and trusted agents

Authorize direct and autonomous purchases with signed AP2 mandates, exact asset bindings, and trusted-agent request verification.

## Separate the agent, the authorization, and the payment

Paxeer X verifies who presents a commerce request, what the payer authorizes, and what payment executes. Visa Trusted Agent Protocol supplies signed agent credentials and intent tags. AP2 supplies signed mandate constraints and checkout bindings. The interoperability service binds verified authorization to a payer-signed EVM payment and returns verified settlement evidence. Each step retains its own verification boundary.

## AP2 v1 mandate verification

The Rust AP2 verifier in services/interop/crates/layerx-ap2 verifies SD-JWT and JOSE mandate material and evaluates its constraints before preparing payment. Direct mode handles direct authorization; autonomous mode evaluates delegated purchasing authorization, including the applicable line-item constraints. Verification binds payment and checkout identities, merchant details, amount, currency, payment instrument, audience, nonce, and validity conditions.

| Verification input | Purpose |
| --- | --- |
| `now` and `clock_skew_seconds` | Evaluate validity windows with an explicit clock and permitted skew. |
| `expected_audience` | Require the mandate to target the expected audience. |
| `expected_nonce` | Bind verification to the expected authorization context. |
| `currency_minor_exponent` | Interpret the payment currency at its correct minor-unit precision. |
| `usage` | Evaluate an open mandate against caller-supplied prior-use facts in the Rust verifier. |

### Bounded autonomous authorization

For an open payment mandate, retain a usage record with its exact mandate reference, frequency, previous occurrence count, amount already spent in minor units, and next eligible execution time. The reference is the AP2 `sd_hash` of that specific mandate, preventing usage facts from being applied to a different authorization. Supply these facts explicitly to the Rust verifier for the next purchase. The hosted HTTP handler supplies no usage record and therefore cannot verify constraints that require prior-use facts.

The verification context requires a positive timestamp, nonempty audience and nonce, a currency minor exponent no greater than 18, and clock skew no greater than 3,600 seconds. These are verifier input bounds enforced before constraint evaluation; the mandate still has to pass its signed constraints. The adapter profile is AP2 v1 (`ap2-v1-signature-constraint-receipt-verified`).

## Bind authorization to actual settlement

A verified mandate becomes an authorized payment only after its merchant and currency match the configured LayerX asset binding. The binding identifies the asset, payer and payee receipt accounts, minor-unit exponent, and atomic units per currency minor unit. Amount conversion uses checked multiplication and rejects overflow or a zero result.

```
asset atomic amount = mandate minor units × atomic units per minor unit
idempotency identity = principal + stable payment reference
request binding = mandate material + currency + asset
                + payer account + payee account + amount
```

The idempotency identity remains stable across retries of the same principal and payment reference. The request digest commits to the signed authorization material and settlement binding. An explicit execution date is already bound by the signed presentations; a locally resolved execution time does not change the digest on retry.

## Interoperability service routes

| HTTP route | Operation |
| --- | --- |
| `POST /v1/http/ap2/mandates/verify` | Verify signed mandate material and its constraint context. |
| `POST /v1/http/ap2/execute` | Validate and relay the payer-signed payment, then export signed receipt evidence. |
| `POST /v1/http/visa-tap/intents/verify` | Verify a trusted-agent request. |
| `POST /v1/http/visa-tap/intents/execute` | Verify the trusted-agent request and relay its authorized payment. |

1. Send checkout_presentation, payment_presentation, and nonce. The HTTP handler uses its server clock, configured audience and currency precision, and zero clock skew to verify the mandate pair.
2. Evaluate the signed merchant, amount, checkout, and applicable autonomous constraints.
3. Apply the asset and receipt-account binding configured for the authenticated principal.
4. For execution, include activity as hex-encoded, payer-signed EIP-1559 transaction bytes. The service checks the payment accounts, asset, amount, reference and expiry before relay.
5. A pending relay returns HTTP 202. Once execution evidence verifies, the configured receipt signers export payment_receipt_jwt and checkout_receipt_jwt.

## Visa Trusted Agent Protocol

The TAP adapter verifies RFC 9421-style `sig2` request signatures against a trusted-agent registry. The `agent-browser-auth` tag identifies browsing intent and `agent-payer-auth` identifies payment intent. Verification checks the validity window, registered key status, signature algorithm, signed request target, cryptographic signature, and nonce uniqueness.

### Replay protection and credential binding

A nonce window consumes the nonce together with its key identifier and expiry. A verified credential can be associated with a LayerX agent through the credential-binding store. That association records identity context; payment authority still requires the matching payer-signed transaction. A valid agent signature does not by itself supply the payer's mandate or authorize an arbitrary transfer.

Maintain the registry's key status and validity information, use canonical authority and path representations for signature verification, and preserve nonce state across the merchant request lifecycle. Browse and pay operations retain distinct intent tags so an operation is evaluated in its declared context.

## Portable purchase evidence

Completed AP2 execution returns signed evidence containing portable LayerX receipt evidence. Payment and checkout receipt references preserve the exact verified authorization bindings. The portable mandate-pair format carries the external evidence needed by `Ap2ExternalMandateVerifier`, allowing verification to travel with the purchase evidence.

Retain the signed mandate pair, signed payment transaction, canonical execution receipt, independent receipt-key trust configuration, merchant checkout identity, and applicable mandate usage facts. Keep raw payment credentials out of operational logs. Use [agent commerce](https://docs.paxeer.app/commerce) to connect this authorization flow to merchant checkout, order reads, and paid-resource fulfillment.
