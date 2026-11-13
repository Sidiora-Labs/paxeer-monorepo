<!-- Source: https://docs.paxeer.app/perpetuals/ -->

# Perpetual Futures

Trade long and short exposure through the native order book, USDL cross margin, governed market parameters and committed transaction receipts.

## A market is a complete risk configuration

The perpetuals venue is implemented in `modules/perps` and exposed to EVM callers at `0x1015`. Its ABI is `precompiles/perps/abi/abi.json`; `precompiles/perps/Perps.sol` declares the Solidity interface. Collateral is the configured USDL ERC-20 with 6 decimal places. Prices, USD notionals, P&L and fees use 18-decimal fixed point. Every precompile method is nonpayable.

| Parameter group | What it controls |
| --- | --- |
| Tick size, lot size and order-value bounds | Valid price and base-size increments and permitted order notionals. |
| Leverage limits and margin tiers | Initial margin from leverage and maintenance requirements from the market tiers. |
| Liquidation fees, slices and cooldown | Fees and bounded closing orders used to resolve an undercollateralized cross account. |
| Premium sampling and funding cap | Sampling cadence, interest and premium adjustments, and the hourly rate cap. |
| Oracle symbol and price freshness | The pricefeed index source and whether current prices permit risk-taking operations. |
| Market enabled state and venue pause | Whether a market is active and which venue operations are permitted. |

Read `getMarketConfig`, `getMarginTiers`, `getMarkPriceDetails` and `getFeeTier` before sizing an order. `getMarketConfig` returns the market leverage cap as a plain integer; order leverage arguments use 18-decimal fixed point. Margin is shared across the account's positions.

## Open, increase, and close exposure

1. Use `getActiveMarketIds` and inspect the selected market configuration and price freshness.
2. Hold the configured collateral token, approve the precompile to spend it, and call `deposit` to fund the cross account.
3. Submit `placeOrder` with the complete order tuple, including side, size, leverage, time in force, price constraints and deadline.
4. Inspect fills, order status and `getPositionDetails`. Use `addCollateral` to increase collateral without changing position size.
5. Monitor `getAccount` and funding. Close through `closePosition`, `partialClose` or `closePositionWithParams`, then withdraw eligible collateral.

The `isLong` field selects long or short exposure. Cancel an order by `cancelOrder(orderId)` or `cancelByCloid(marketId, cloid)`. A successful order submission can leave a resting order or a partial fill; inspect the order and fill events to determine exposure.

### Leverage and initial margin

Leverage is notional exposure divided by posted margin. For example, 10,000 USD of notional against 1,000 USD of collateral represents 10× exposure; against 2,000 USD it represents 5×. These illustrative values do not define a market limit. Initial margin, maintenance tiers and existing cross-account exposure constrain an actual order.

### Profit and loss

For a long position, price P&L follows `(mark price − entry price) × sizeBase ÷ 10^18` with 18-decimal prices and base size; the sign reverses for a short. Use `getPositionDetails` for unrealized P&L and funding, and `getAccount` for account equity and withdrawable collateral. Keep USD fixed-point values separate from 6-decimal USDL amounts.

## Funding follows block time

The venue samples the order-book premium against the pricefeed index using block time. At each new whole hour, `SettleFunding` in `modules/perps/keeper/funding.go` updates the cumulative funding index for elapsed hours. Positions realize the index difference when they are settled.

The hourly rate combines the mean premium with the governed interest and premium clamp, divides the eight-hour rate into hourly units, and applies the governed hourly cap. Funding settlement converts USD values to USDL with payments rounded up and receipts rounded down. The rounding residue stays in the venue.

`getCurrentFundingRate`, `getFundingDetails` and `getPositionFunding` expose funding state. The position funding result is negative when the position pays and positive when it receives. Include funding separately from price P&L when displaying equity.

## Maintenance, liquidation, and insurance

Liquidation operates on the cross account when equity falls below its maintenance requirement. `checkLiquidatable` and `getAccount` expose the account risk state. `liquidate(positionId)` identifies an account through a position, settles its funding, cancels open orders and closes exposure against the order book. Large positions use governed slices and cooldowns.

1. Read fresh market prices and recompute cross-account equity and maintenance requirements.
2. Cancel open orders and attempt reduce-only market closes within the configured bounds.
3. Settle realized P&L, funding and fees against account collateral.
4. Handle deficits through insurance, backstop exposure and bounded auto-deleveraging.

Insurance and auto-deleveraging are handled in `modules/perps/keeper/insurance.go` and `modules/perps/keeper/adl.go`. `getInsuranceBalance` and `getADLRank` expose their state. End-block processing bounds scans, liquidations and closing slices. The caller recorded in a liquidation event receives no keeper payout.

## EVM method reference

| Method | Purpose |
| --- | --- |
| `getMarketConfig / getActiveMarketIds` | Read governed market parameters and active markets. |
| `getMarkPriceDetails` | Read mark, index, midpoint and price freshness. |
| `placeOrder / cancelOrder` | Submit a typed order or cancel an owned order. |
| `openPosition / addSize / closePosition` | Manage exposure through the order-book compatibility methods. |
| `getFundingDetails / getPositionFunding` | Read cumulative funding and the position amount owed or received. |
| `liquidate / getADLRank` | Liquidate an undercollateralized account or inspect position deleveraging rank. |

Use the exact tuple layouts and parameterless custom errors in `precompiles/perps/Perps.sol`. Methods reject attached native value, delegatecall and writes under staticcall. Check the error selector on a reverted call. Verify successful fills and the containing committed block before treating a trade as final.

## Continue building

- [Oracle acceptance and market risk controls](https://docs.paxeer.app/oracle-risk)

- [Fees and settlement](https://docs.paxeer.app/fees)

- [Paxeer X modules](https://docs.paxeer.app/modules)
