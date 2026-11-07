# AP2

`layerx-ap2` verifies AP2 Checkout and Payment Mandates and binds their
authorised meaning to a payment on Paxeer X Network. The adapter ID is
`ap2`. The pinned upstream revision is
`e1ea56db72a6385bce3e5c1112b3a56ce60acb43`, compiled from
`services/interop/specs/vendor/ap2/specification.md`.

## What the crate exposes

The library in `services/interop/crates/layerx-ap2` exports
`MandateVerifier`, `VerificationContext`, `VerifiedMandates`,
`Ap2Adapter`, `AuthorizedPayment` and `ExecutedPayment`.
`ap2_adapter_descriptor` declares schema version `1.0.0`, verifies the
pinned specification document and accepts a real caller-supplied
`ConformanceSuite`. It does not fabricate a vector digest.

Checkout and Payment Mandates are verified as a bound pair, including
signatures, selective disclosures, delegation and payment constraints.
`authorize_payment` binds the verified merchant and currency to deployment
asset policy and converts integer minor units by checked multiplication.
The adapter translates authorised meaning; native network execution updates
balances ([Interoperability](index.md)).

## Mounted HTTP routes

`services/interop/crates/layerx-interop-gateway/src/server.rs` declares
these routes, and `layerx-interop-service/src/server.rs` dispatches them:

| Method and path | Result |
| --- | --- |
| `POST /v1/http/ap2/mandates/verify` | Verifies the pair and returns `state: "mandate-verified"` with the mode, transaction and checkout IDs, currency, minor units and execution time. It does not execute payment. |
| `POST /v1/http/ap2/execute` | Verifies the pair, binds the supplied transaction to the authorised payment, relays it and checks the resulting receipt and payment log. |

Requests carry `checkout_presentation`, `payment_presentation` and `nonce`.
Execution also requires `activity`: the hex-encoded signed EVM transaction.
Unknown request members are refused. Authentication supplies the principal;
the service supplies the clock and selects audience, currency and account
bindings from `ap2_keys` and `ap2_assets` in its deployment manifest.
Clients cannot supply those trust decisions as request fields.

Execution checks the decoded `payExact` payer, payee, asset, amount and
reference against `AuthorizedPayment`. It verifies the signed receipt under
pinned receipt keys and requires a matching `PaymentExact` event from the
native ledger at `0x000000000000000000000000000000000000101a`.
An accepted transaction without a receipt returns HTTP 202 with a pending
status and `state: "submitted"`. A verified execution returns HTTP 200,
`state: "receipt-verified"`, its transaction and receipt digest, and the
actual `verificationLevel` (`receipted` or `committed`). HTTP 200 from mandate
verification alone is not payment evidence. See [Receipts](../concepts/receipts.md).

These are routes of the configured interop service. Use its deployment
address; the public node JSON-RPC remains at the node's root path.

## Portable evidence

The library's `Ap2Adapter` can produce ES256-signed Payment and Checkout
Receipt JWTs via caller-provided `ReceiptSigner` implementations. Their
shared network evidence is `PortableReceiptV2`, also exported as
`PortableLayerXEvidence`. If receipt signing fails after execution has
already been verified, `SettledEvidenceUnavailable` retains the receipt
digest instead of implying that payment did not happen. The mounted HTTP
handler returns the receipt-backed response described above; it does not
promise those library-generated JWTs.

For portable mandate verification, `Ap2MandatePair` uses
`AP2_MANDATE_PAIR_MEDIA_TYPE`:
`application/vnd.layerx.ap2-mandate-pair+json`. Its two members,
`checkoutMandate` and `paymentMandate`, contain the unchanged upstream
presentations. `Ap2ExternalMandateVerifier` verifies the pair through the
generic external-evidence binding. See [Portable receipts](portable-receipts.md)
for the distinction between external mandate evidence and network receipts.

## Status

The library, mounted routes, pinned specification and tests are in this
tree. Library vectors live under
`services/interop/crates/layerx-ap2/tests/`; shared AP2 conformance cases
live under `services/interop/specs/conformance/ap2/`. Their presence does
not establish that any particular hosted deployment is configured or qualified.
