<!-- Source: https://docs.paxeer.app/mandates/ -->

# Payment mandates and trusted agents

Authorize direct and autonomous purchases with signed AP2 mandates, exact asset bindings, and trusted-agent request verification.

## Separate the agent, the authorization, and the payment

Paxeer X verifies who presents a commerce request, what the payer authorizes, and what payment executes. Visa Trusted Agent Protocol supplies signed agent credentials and intent tags. AP2 supplies signed mandate constraints and checkout bindings. The LayerX payment plane executes the resulting typed intent and returns settlement evidence. Each step retains its own verification boundary.

## AP2 v1 mandate verification

The AP2 adapter verifies SD-JWT and JOSE mandate material and evaluates its constraints before preparing payment. Direct mode handles direct authorization; autonomous mode evaluates delegated purchasing authorization, including the applicable line-item constraints. Verification binds payment and checkout identities, merchant details, amount, currency, payment instrument, audience, nonce, and validity conditions.

| Verification input | Purpose |
| --- | --- |
| `now` and `clock_skew_seconds` | Evaluate validity windows with an explicit clock and permitted skew. |
| `expected_audience` | Require the mandate to target the expected audience. |
| `expected_nonce` | Bind verification to the expected authorization context. |
| `currency_minor_exponent` | Interpret the payment currency at its correct minor-unit precision. |
| `usage` | Evaluate an open mandate against durable prior-use facts. |

### Bounded autonomous authorization

For an open payment mandate, retain a usage record with its exact mandate reference, frequency, previous occurrence count, amount already spent in minor units, and next eligible execution time. The reference is the AP2 `sd_hash` of that specific mandate, preventing usage facts from being applied to a different authorization. Supply these facts explicitly when verifying the next purchase.

The verification context requires a positive timestamp, nonempty audience and nonce, a currency minor exponent no greater than 18, and clock skew no greater than 3,600 seconds. These are verifier input bounds enforced before constraint evaluation; the mandate still has to pass its signed constraints. The adapter profile is AP2 v1 (`ap2-v1-signature-constraint-receipt-verified`).

## Bind authorization to actual settlement

A verified mandate becomes an authorized payment only after its merchant and currency match the configured LayerX asset binding. The binding identifies the asset, payer and payee receipt accounts, minor-unit exponent, and atomic units per currency minor unit. Amount conversion uses checked multiplication and rejects overflow or a zero result.

```
asset atomic amount = mandate minor units × atomic units per minor unit
idempotency identity = principal + stable payment reference
request binding = mandate material + currency + asset
                + payer account + payee account + amount + execution time
```

The idempotency identity remains stable across retries of the same principal and payment reference. The request digest commits to the authorization material and settlement binding, preserving the relationship between the requested purchase and the executed payment.

## Verify and execute

| HTTP route | Operation |
| --- | --- |
| `POST /v1/http/ap2/mandates/verify` | Verify signed mandate material and its constraint context. |
| `POST /v1/http/ap2/execute` | Map the authorized purchase to typed payment execution and signed evidence. |
| `POST /v1/http/visa-tap/intents/verify` | Verify a trusted-agent request. |
| `POST /v1/http/visa-tap/intents/execute` | Hand the trusted commerce intent to the LayerX authority path. |

1. Resolve the signing keys and verify the signed AP2 mandate pair against the explicit verification context.
2. Evaluate merchant, amount, checkout, and autonomous-use constraints.
3. Apply the configured asset and receipt-account binding.
4. Execute through the LayerX intent plane using the derived idempotency identity and request digest.
5. Verify execution evidence and sign the AP2 payment and checkout receipt evidence.

## Visa Trusted Agent Protocol

The TAP adapter verifies RFC 9421-style `sig2` request signatures against a trusted-agent registry. The `agent-browser-auth` tag identifies browsing intent and `agent-payer-auth` identifies payment intent. Verification checks the validity window, registered key status, signature algorithm, signed request target, cryptographic signature, and nonce uniqueness.

### Replay protection and credential binding

A nonce window consumes the nonce together with its key identifier and expiry. A verified credential can be associated with a LayerX agent through the credential-binding store. That association records identity context; payment authority still passes through the typed intent authority path. A valid agent signature does not by itself supply the payer's mandate or authorize an arbitrary transfer.

Maintain the registry's key status and validity information, use canonical authority and path representations for signature verification, and preserve nonce state across the merchant request lifecycle. Browse and pay operations retain distinct intent tags so an operation is evaluated in its declared context.

## Portable purchase evidence

AP2 execution returns signed evidence containing portable LayerX receipt evidence. Payment and checkout receipt references preserve the exact verified authorization bindings. The portable mandate-pair format carries the external evidence needed by `Ap2ExternalMandateVerifier`, allowing verification to travel with the purchase evidence.

Retain the signed mandate pair, canonical execution receipt, authorized batch, merchant checkout identity, and mandate usage facts as the appropriate records for the transaction. Keep raw payment credentials out of operational logs. Use [agent commerce](https://docs.paxeer.app/commerce) to connect this authorization flow to merchant checkout, order reads, and paid-resource fulfillment.
