# AP2 mandates

`layerx-ap2` verifies AP2 v1 checkout and payment mandate presentations
(SD-JWT / JOSE), evaluates their constraints, and authorizes a typed payment.
The adapter id is `ap2`; its descriptor declares version `1.0.0` and verifies
the vendored specification against `AP2_SPEC_SHA256` at `AP2_SPEC_COMMIT`.
See the [crate exports](../../services/interop/crates/layerx-ap2/src/lib.rs).

Payments execute on the merged Paxeer network, EVM chain id `125`. The kernel
is part of the network and proposes blocks. A payment receipt is final when
its block is committed. A verified mandate authorizes payment; mandate
verification alone does not establish payment or block commitment.

[Interop](Interop.md) indexes the service and its other adapters;
[x402 transport](X402Transport.md) describes the shared receipt evidence.
Portable mandate pairs are also inputs to the
[portable receipt verifier](PortableVerifier.md).

---

## Gateway routes

The route contract is in
[gateway server.rs](../../services/interop/crates/layerx-interop-gateway/src/server.rs).
The HTTP handlers are in
[service server.rs](../../services/interop/crates/layerx-interop-service/src/server.rs).

| Method | Path | Result |
| --- | --- | --- |
| POST | `/v1/http/ap2/mandates/verify` | Verify the mandate pair without executing payment |
| POST | `/v1/http/ap2/execute` | Bind the mandate to a payer-signed transaction and relay it |

The request uses `checkout_presentation`, `payment_presentation`, and `nonce`.
Execution additionally needs `activity`: despite that field's historical
name, the handler decodes it as a signed EIP-1559 transaction calling the
ledger's `payExact` entry point. Follow the authentication and idempotency
requirements on [Interop](Interop.md).

The service chooses asset and merchant bindings from its authenticated
principal's configured `ap2_assets`, resolves mandate keys from `ap2_keys`,
and uses server time. It refuses ambiguous bindings. The verify route returns
`state: "mandate-verified"` with mode, transaction and checkout identifiers,
currency, minor units, and execution time. This is a verification result.
Execution checks the payer, recipient, asset, exact amount, expiry and payment
reference before submission; pending submission does not become paid.

---

## Public types

Import the following from the `layerx_ap2` crate root. The implementation
modules `verify`, `jose`, `model`, `adapter`, `portable`, and `error` are private.

| Purpose | Public names |
| --- | --- |
| Mandate verification | `MandateMode`, `MandateUsage`, `VerificationContext`, `VerifiedMandates`, `MandateVerifier` |
| Trust resolution | `KeyUse`, `ProtectedHeader`, `KeyResolver` |
| Payment models | `Merchant`, `PaymentAmount`, `PaymentInstrument` |
| Authorization and execution | `LayerXAssetBinding`, `AuthorizedPayment`, `authorize_payment`, `LayerXIntentPlane`, `ExecutedPayment`, `PlaneOutcome`, `ReceiptSigner`, `SignedAp2Evidence`, `AdapterOutcome`, `Ap2Adapter` |
| Portable evidence | `PortableLayerXEvidence`, `AP2_MANDATE_PAIR_MEDIA_TYPE`, `Ap2MandatePair`, `Ap2ExternalMandateVerifier` |
| Errors | `Ap2Error` |

Construct a `MandateVerifier` with a `KeyResolver`, then call
`verify(checkout_presentation, payment_presentation, &context)`. The resolver
supplies a trusted P-256 key for the declared `KeyUse` after resolving `kid`
or validating the certificate chain. `VerificationContext` explicitly supplies
`now`, `clock_skew_seconds`, `expected_audience`, `expected_nonce`,
`currency_minor_exponent`, and optional `usage`. Recurrence and budget checks
need usage facts bound to the exact open mandate; the verifier does not read
an ambient ledger. The hosted handler currently passes `usage: None`.

`ap2_adapter_descriptor(conformance)` requires a caller-supplied
`layerx_interop_gateway::adapter::ConformanceSuite`. Likewise,
`Ap2ExternalMandateVerifier::new(resolver, conformance)` needs that declaration.
`interop_ap2()` is the crate's Codify anchor.

`PortableLayerXEvidence` aliases `layerx_portable::PortableReceiptV2`.
`Ap2MandatePair::new(checkout_mandate, payment_mandate).to_json()` preserves the
exact presentations in `checkoutMandate` and `paymentMandate` fields, with
media type `application/vnd.layerx.ap2-mandate-pair+json`.
`Ap2ExternalMandateVerifier` implements `ExternalEvidenceVerifier` using the
same explicit verification context.

---

## Receipt binding and qualification

The [adapter](../../services/interop/crates/layerx-ap2/src/adapter.rs) checks
a successful signed receipt under configured receipt keys and requires exactly
one matching ledger payment log. That log binds the payment reference, payer,
payee, asset and amount. `GatewayCore::settle_with_sequencer_evidence` verifies
the signed effects and records `ReceiptVerified` before signed AP2 payment and
checkout receipts can be returned. This status establishes receipt verification;
consumers requiring finality must also establish committed-block evidence.

The existing test files are
[mandates.rs](../../services/interop/crates/layerx-ap2/tests/mandates.rs),
[pinned_spec.rs](../../services/interop/crates/layerx-ap2/tests/pinned_spec.rs),
[portability_v2.rs](../../services/interop/crates/layerx-ap2/tests/portability_v2.rs),
and [sequencer_settlement.rs](../../services/interop/crates/layerx-ap2/tests/sequencer_settlement.rs).
They cover mandate constraints and bindings, specification integrity, portable
mandate inputs, and signed settlement evidence. These local sources do not
attest a deployed merchant or an independently operated AP2 service.

[Interop](Interop.md) · [x402 transport](X402Transport.md) · [Home](Home.md)
