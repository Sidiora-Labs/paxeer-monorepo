# Market-maker ramps

Independent market-maker ramps live under `services/platform/ramps/`, separately
from the interoperability adapters: toolkit crate
`layerx-ramp-toolkit` (`services/platform/ramps/toolkit`) and reference
material under `services/platform/ramps/reference/`
(`services/platform/ramps/README.md`).

The reference HTTP worker exposes operator-authenticated `POST /internal/v1/work`.
The `submit_layerx` action submits the operator-funded on-ramp payment;
`submit_customer_payment` accepts the customer's signed off-ramp transaction.
The engine verifies the customer payment before broadcast
(`services/platform/ramps/reference/src/main.rs`,
`services/platform/ramps/toolkit/src/engine.rs`).

---

## Role

Market makers act as ordinary LayerX principals. External custody is
labelled `EXTERNAL_CUSTODY_LABEL`. Orders are receipt-gated. The
new journal records carry version 2 and are hashed under `LXP/market-maker-ramp/journal/v2`; journals written under `LXP/market-maker-ramp/journal/v1` are replayed read-only.

---

## Toolkit surface

From `services/platform/ramps/toolkit/src/lib.rs`:

`RampDirection`, `QuoteTerms`, `RampOrder`,
`compile_customer_payment`, `compile_payer_grant_draw`, `compile_operator_send`,
`verify_order_receipt`, `RampPresentation`, `RampError`,
`platform_ramp_toolkit()`, plus modules `clients`, `engine`,
`journal`.

`compile_customer_payment` encodes `budgetSpend` for the pay precompile at
`0x000000000000000000000000000000000000101b`, binding the customer's budget,
recipient and amount. `compile_payer_grant_draw` delegates to that function;
its sequence argument is unused. `compile_operator_send` encodes `payExact`
for the ledger precompile at `0x000000000000000000000000000000000000101a`.

---

## Qualification gates

Makefile targets for the ramp toolkit and reference service:

- `make interop-test-ramps`
- `make interop-test-ramps-sandbox`

The sandbox journey requires a receipt verifier and protected verification
configuration, customer and operator endpoints and credentials, a CA certificate,
and protected order inputs. The full-contract journey also requires service
control inputs. See the required environment in
[`sandbox-journey.sh`](../../services/platform/ramps/sandbox-journey.sh).

Fiat provider callbacks at the interop edge are
[Fiat ramps](FiatRamps.md).

[Interop](Interop.md) · [Home](Home.md)
