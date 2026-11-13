# Fiat ramps

`layerx-fiat` is the interop-edge adapter for card, bank, and
real-time-payment **provider callbacks**
(`services/interop/crates/layerx-fiat/src/lib.rs`). The adapter id is `fiat`.

This is not the independent market-maker ramp toolkit. That lives
under `services/platform/ramps` and is documented on
[Market-maker ramps](MarketMakerRamps.md).

---

## Gateway routes

- `POST /v1/http/fiat/card/callbacks`
- `POST /v1/http/fiat/bank/callbacks`
- `POST /v1/http/fiat/rtp/callbacks`

---

## Public types

`FiatRail` (`Card`, `Bank`, `RealTimePayment`),
`EvidenceClass` (`Authorised`, `Clearing`, `Settled`, `Reversed`,
`Chargeback`), `ExternalId`, `TokenReference`, `ProviderEvidence`,
`VerifiedProviderFacts`, `ProviderVerifier`, `FiatIntentKind`,
`FiatIntent`, `FiatPlaneResult`, `FiatPlane`, `FiatJourneyState`,
`FiatAdapter` (`verify_evidence`, `apply`, `idempotency_key`),
`FiatError`, `fiat_adapter_descriptor(spec, conformance)`,
`interop_fiat_adapters()`.

The descriptor requires deployment-supplied `PinnedSpec` and `ConformanceSuite`
values from `layerx_interop_gateway::adapter`. The adapter verifies provider
facts and binds the resulting ledger payment to the signed receipt. Receipt
verification and committed-block finality are distinct checks on the same
Paxeer chain.

Callbacks carry token references only. PAN-shaped input is refused.
Credit, reversal, and chargeback apply only after
`ProviderVerifier` accepts the provider evidence and the LayerX
plane returns a receipt-gated result.

[Interop](Interop.md) · [Home](Home.md)
