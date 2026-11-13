# Market-maker ramps

Independent market-maker ramps live under `services/platform/ramps/`, separately
from the interoperability adapters: toolkit crate
`layerx-ramp-toolkit` (`services/platform/ramps/toolkit`) and reference
material under `services/platform/ramps/reference/`
(`services/platform/ramps/README.md`).

The reference HTTP worker does not expose the toolkit's customer-signed
off-ramp submission path. A complete off-ramp journey requires an authenticated
integration with that path (`services/platform/ramps/toolkit/src/engine.rs`,
`services/platform/ramps/README.md`).

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

Makefile targets for the ramp toolkit and reference service:

- `make interop-test-ramps`
- `make interop-test-ramps-sandbox` (requires `LAYERX_RAMP_URL`)

Fiat provider callbacks at the interop edge are
[Fiat ramps](FiatRamps.md).

[Interop](Interop.md) · [Home](Home.md)
