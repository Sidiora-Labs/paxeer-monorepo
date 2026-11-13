<!-- Source: https://docs.paxeer.app/perpetuals/ -->

# Perpetual Futures

Trade long and short exposure with explicit margin accounts, deterministic funding, signed market prices, and accountable liquidation settlement.

## A market is a complete risk configuration

Paxeer X perpetual markets combine an order book with position, margin, funding, insurance, and liquidation state. Every market identifies its quote asset, administrator, liquidity account, long and short funding accounts, and insurance account. Losses, fees, funding, and margin releases settle through conserved transfer legs in the shared ledger.

| Parameter group | What it controls |
| --- | --- |
| Contract size, tick size, lot size, price scale | Contract units, valid price and quantity increments, and integer price interpretation. |
| Initial and maintenance margin ratios | Collateral required to open exposure and the threshold used by maintenance checks. |
| Liquidation fee and liquidator share | The fee charged during liquidation and its division between the liquidator and insurance. |
| Funding interval and maximum rate | Accrual cadence and the permitted funding-rate magnitude. |
| Oracle keys, freshness, bounds, deviation | Which signed observations are acceptable for the market. |
| Parameter version and halt state | The configuration revision and whether risk-taking operations are permitted. |

Initial margin must exceed maintenance margin. Market accounts are distinct, and the permitted oracle set contains up to eight canonical keys. Read the selected market configuration before sizing an order; ratios, intervals, and limits belong to the market rather than a universal exchange default.

## Open, increase, and close exposure

1. Select a market and inspect its quote asset, price scale, lot size, margin ratios, oracle observation, and halt state.
2. Fund your main account with the required quote asset and choose a dedicated margin account.
3. Place a buy or sell order with a price and quantity, or submit a position-open activity with side, size, entry notional, and margin amount.
4. Opening transfers posted margin from the owner main account into the position margin account. Increase an existing position with explicit size, notional, and margin deltas.
5. Monitor accepted prices, accrued funding, margin, and the market maintenance threshold. Close the position to settle exposure and release eligible remaining margin.

Buy positions express long exposure; sell positions express short exposure. Orders have explicit owner and order identifiers, and cancellations identify both the market and order. A rejected activity does not become an executed position: inspect its receipt before updating a trading interface.

### Leverage and initial margin

Leverage is notional exposure divided by posted margin. A hypothetical market with a 1,000-basis-point initial margin ratio requires at least 1,000 quote units for 10,000 quote units of opening notional, corresponding to 10× exposure at that minimum. Posting 2,000 quote units lowers that ratio to 5×. These figures illustrate the calculation; the selected market defines the actual requirements.

### Profit and loss

For a long position, price profit or loss follows `(mark price − entry price) × size ÷ price scale`; the sign reverses for a short. Integer settlement rounds positive profit down and loss magnitude up when division leaves a remainder. A long with one normalized unit entered at 100 and marked at 105 has five quote units of price profit before funding and fees.

## Funding follows sealed batch time

Funding ticks use the sealed batch timestamp. Only whole configured funding intervals accrue; a tick before the next complete interval is refused. The cumulative funding index advances by the rate multiplied by elapsed intervals, and a position records the index at entry so its accrued funding can be computed from the difference.

The funding-rate calculation measures the signed difference between oracle and reference prices relative to the reference price, then caps its magnitude at the market maximum. Positive funding transfers from the long funding account to the short funding account; negative funding reverses that direction. At the position level, positive index movement makes longs pay and shorts receive. Funding payments are rounded conservatively when integer units leave a remainder.

For example, 10,000 quote units of entry notional and a positive cumulative change of 20 basis points produce 20 quote units of funding owed by a long, before any rounding effects. The same index movement credits a short of equal entry notional.

## Maintenance, liquidation, and insurance

The maintenance check computes mark notional from current price and size, rounds required maintenance collateral upward, and compares it with the supplied margin balance. A balance strictly below that requirement is liquidatable; equality satisfies that check. Applications should keep price P&L, funding, fees, and collateral accounting visible rather than inferring a liquidation price from leverage alone.

1. Trading loss is paid from the position margin account to market liquidity, up to the available margin.
2. A remaining trading-loss deficit draws on the market insurance account.
3. The liquidation fee is limited by margin remaining after losses and split between the liquidator and insurance account.
4. Any residual margin returns to the owner main account.

Insurance is a funded account, not an unlimited guarantee. A liquidation whose deficit exceeds the available insurance balance is refused. Auto-deleveraging accepts an explicitly ordered list of position identifiers. The codec capacity is 128 (`LX_PERPS_ADL_CAPACITY`), but dispatch refuses more than `LX_PERPS_DISPATCH_MAX_ADL` (16) positions per activity; canonical sorted ordering makes processing deterministic.

## Activity reference

| Activity | Purpose |
| --- | --- |
| `MARKET_CREATE / MARKET_HALT` | Create a market configuration or change its halt state. |
| `ORACLE_PUSH` | Submit a signed market price observation. |
| `ORDER_PLACE / ORDER_CANCEL` | Manage explicit buy and sell orders. |
| `POSITION_OPEN / POSITION_INCREASE / POSITION_CLOSE` | Manage exposure and its margin lifecycle. |
| `FUNDING_TICK` | Accrue completed funding intervals. |
| `LIQUIDATE / ADL` | Resolve undercollateralized exposure and ordered deleveraging. |

Perpetual activities use module identifier `6` and the `0x0006xxxx` activity family: `0x00060001`–`0x0006000b` for the activities above, plus `LX_PERPS_MARGIN_REGISTER` at `0x0006000c`. A versioned 130-byte time-in-force order payload is also defined (`lx_perps_tif_module_iface`). Unknown ordinals, noncanonical payloads, zero amounts, insufficient margin, unauthorized debits, and halted markets produce explicit refusals.

## Continue building

- [Oracle acceptance and market risk controls](https://docs.paxeer.app/oracle-risk)

- [Fees and settlement](https://docs.paxeer.app/fees)

- [Paxeer X modules](https://docs.paxeer.app/modules)
