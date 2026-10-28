# Market-maker ramps

Independent market-maker ramps are **not** an `services/interop/crates`
adapter. They live under `services/platform/ramps/`: toolkit crate
`layerx-ramp-toolkit` (`services/platform/ramps/toolkit`) and reference
material under `services/platform/ramps/reference/`
(`services/platform/ramps/README.md`).

`spec/layerx-platform/spec.kvx` task 26.1 is **implemented**. The
spec `reality` field records that the reference ramp service is
incomplete. Task 26.2 (labelling) is **implemented** with
qualification still pending. This page does not claim a finished
hosted ramp.

---

## Role

Market makers act as ordinary LayerX principals. External custody is
labelled `EXTERNAL_CUSTODY_LABEL`. Orders are receipt-gated. The
new journal records carry version 2 and are hashed under `LXP/market-maker-ramp/journal/v2`; journals written under `LXP/market-maker-ramp/journal/v1` are replayed read-only.

---

## Toolkit surface

From `services/platform/ramps/toolkit/src/lib.rs`:

`RampDirection`, `QuoteTerms`, `RampOrder`,
`compile_payer_grant_draw`, `compile_operator_send`,
`verify_order_receipt`, `RampPresentation`, `RampError`,
`platform_ramp_toolkit()`, plus modules `clients`, `engine`,
`journal`.

---

## Qualification gates

Makefile targets named in the spec and `services/platform/ramps/README.md`:

- `make interop-test-ramps`
- `make interop-test-ramps-sandbox` (requires `LAYERX_RAMP_URL`)

Fiat provider callbacks at the interop edge are
[Fiat ramps](FiatRamps.md).

[Interop](Interop.md) · [Home](Home.md)
