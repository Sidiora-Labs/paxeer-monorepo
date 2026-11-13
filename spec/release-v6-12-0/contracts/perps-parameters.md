# Perpetuals parameter sheet

The native perpetuals venue is served at `0x0000000000000000000000000000000000001015` on Paxeer mainnet-beta, EVM chain id 125 (`0x7d`). The kernel is part of this BFT network and proposes blocks; a receipt is final when its block is committed. Trading enters through the perps precompile, administration through the governance Msg service, and periodic processing through the venue's EndBlock hook in `paxd`.

The figures below are activation defaults from [`DefaultParams()`](../../../modules/perps/types/params.go) and the market configurations written by [`SeedV6120`](../../../modules/perps/keeper/seed_v6120.go). Governance can replace them after activation. They are not a snapshot of current on-chain values. The fee structure follows the Hyperliquid venue, with the differences described below.

USD amounts, prices and base sizes use 18-decimal fixed point. Collateral uses 6-decimal USDL units: one USDL unit is `1e12` units of the USD representation. The default collateral ERC20 is `0x85FcD13735F4309833A503EE804ea32395851479`, held by `Keeper.ModuleEVMAddress()`, the EVM form of the `perps` module account. That custody address is distinct from the precompile address. Rates use parts per million (ppm; 1 ppm = 0.0001 percent = 0.01 bp), except maintenance rates and the spot fee section, which use basis points. Stored monetary words are 256 bits; arithmetic also uses `big.Int` and, for impact prices, rational intermediates. Conversion and settlement rounding residues are booked to `VenueBooks.ResidueUsd`; see [the venue contract](perps.md), sections 2, 8 and 24.

## Administration

The served service is `paxprotocol.paxchain.perps.Msg`, declared in [`api/perps/tx.proto`](../../../api/perps/tx.proto) and registered by [`AppModule.RegisterServices`](../../../modules/perps/module.go). Its messages can be carried in a `LayerXProposal` and executed through the application's SDK message service routing. The `authority` must equal `Keeper.Authority()`, the governance module's bech32 account; an ordinary EVM caller cannot administer these settings. The real proposal path is exercised in [`modules/perps/gov_route_test.go`](../../../modules/perps/gov_route_test.go).

| Message type URL | Payload besides `authority` | Effect |
|---|---|---|
| `/paxprotocol.paxchain.perps.MsgUpdateParams` | `params`: complete `Params` value | Replaces venue parameters after validation. |
| `/paxprotocol.paxchain.perps.MsgSetMarket` | `market`: complete `Market` value | Lists or updates a market after validation and pricefeed identity checks. |
| `/paxprotocol.paxchain.perps.MsgSetPaused` | `paused`: boolean | Sets or clears the venue pause flag. |

`MsgUpdateParams` is a replacement, not a patch: preserve fields that should stay unchanged. The protobuf `params` and `market` fields are bytes with Go custom types `Params` and `Market`; their stored JSON names are defined in [`params.go`](../../../modules/perps/types/params.go) and [`market.go`](../../../modules/perps/types/market.go). The served update refuses a different collateral token and refuses lowering `MaxOpenOrdersPerAccount` below a live account's open-order count. These checks are in [`msg_server.go`](../../../modules/perps/keeper/msg_server.go), beyond `Params.Validate()`.

`MsgSetMarket` preserves an existing market's `Symbol`, `OracleSymbol`, `SzDecimals` and `PxDecimals`. Its id and oracle symbol must agree with the pricefeed registry. A newly enabled or re-enabled market takes its listing time from the block; other updates preserve that time. Governance can change the tier class, impact notional, applicable OI cap, market funding cap and enabled flag subject to validation. `MaxLeverage` must equal the class's tier 0 leverage, currently 1000 for every class; it is not an independent leverage control. Class E uses the venue's listing-age table and must carry zero `OiCapUsd`. See [`msg_server_market.go`](../../../modules/perps/keeper/msg_server_market.go), [`Keeper.SetMarket`](../../../modules/perps/keeper/keeper.go) and [the market sheet](perps-markets.md).

`Paused` defaults to false. Pausing stops opening orders while preserving reduce-only exits; EndBlock continues pricing, funding and protective trigger processing. Disabling one market excludes it from the enabled-market EndBlock pass and blocks its opening orders, while reduce-only closure remains available subject to the usual freshness and execution checks. Successful messages emit `perps_params_updated`, `perps_market_set` or `perps_pause_set`. The handlers use cached contexts; a refused proposal does not partially apply its earlier messages.

`Params.Validate()` enforces the following venue bounds; `Market.Validate()` also checks the per-market funding cap:

- Fee thresholds start at zero and strictly increase; taker rates are below 1,000,000 ppm, ordinary maker rates are between zero and the taker rate, and neither rate increases with the tier. Rebate shares strictly increase within (0, 1,000,000) ppm; rebate rates are negative, become more negative by tier, and their magnitude cannot exceed the lowest taker fee. `InsuranceSharePpm` is at most 1,000,000.
- Funding interest and premium clamp are at most 1,000,000 ppm; venue and market hourly caps are positive and at most 1,000,000 ppm. Premium interval and EMA time constant are positive.
- Class E cap rows start at age zero with positive caps, and both ages and caps strictly increase. The withdrawal floor is at most 1,000,000 ppm. Liquidation threshold and cooldown are positive, slice ppm is in (0, 1,000,000], and the positive backstop ratio is below one.
- All seven EndBlock budget fields, the open-order and fill limits, the minimum order value and the limit-order multiple are positive. Market-order bounds start at leverage zero, exceed the minimum order value and strictly increase with their leverage thresholds. Default slippage is in (0, 1,000,000) ppm. Positivity of a budget is validation, not a guarantee that every underlying operation has a constant execution cost.

## 1. Trading fees

Fee tier of an account = the highest tier whose threshold its 14-day perpetuals volume strictly exceeds, with tier 0 as the base. [`fees.go`](../../../modules/perps/keeper/fees.go) keeps daily buckets keyed by UTC day `d = floor(blockTime / 86400)`. The window includes days `max(0, d-13)` through d; each bucket series deletes older entries on its first write of day d. Both accounts count a fill's notional; the venue bucket counts that notional once. Rates are read when fees are booked, and matching records volume after booking a fill, so the new volume affects subsequent fee calculations.

Taker fee = fill notional x taker rate, rounded up first to 1e-18 USD, then to a USDL unit. Positive maker fees round the same way. A negative maker rate pays a rebate from `perps_fees`: the maker's credit rounds down, and the fee account's debit rounds up. Trading-fee credits to the fee and insurance accounts round down; conversion differences are booked to `ResidueUsd`.

| tier | 14-day volume (USD) | taker (bp) | taker (ppm) | maker (bp) | maker (ppm) |
|---|---|---|---|---|---|
| 0 | <= 5,000,000 | 4.5 | 450 | 1.5 | 150 |
| 1 | > 5,000,000 | 4.0 | 400 | 1.2 | 120 |
| 2 | > 25,000,000 | 3.5 | 350 | 0.8 | 80 |
| 3 | > 100,000,000 | 3.0 | 300 | 0.4 | 40 |
| 4 | > 500,000,000 | 2.8 | 280 | 0.0 | 0 |
| 5 | > 2,000,000,000 | 2.6 | 260 | 0.0 | 0 |
| 6 | > 7,000,000,000 | 2.4 | 240 | 0.0 | 0 |

`FeeTiers` contains the table above. `RebateTiers` replaces the maker rate when the account's 14-day maker volume is strictly above a share of the venue's 14-day volume:

| rebate tier | maker share of venue volume | maker rate (bp) | maker rate (ppm) |
|---|---|---|---|
| 1 | > 0.5% | -0.1 | -10 |
| 2 | > 1.5% | -0.2 | -20 |
| 3 | > 3.0% | -0.3 | -30 |

- The fee recipient is the store account `perps_fees`; it receives the trading fee remaining after `Params.InsuranceSharePpm` and pays maker rebates. The insurance share is 0 ppm at activation; the fund also receives backstop margin and backstop position profit (sections 6 and 7).
- Liquidation fee: 0. A market liquidation is an ordinary taker fill at the account's taker rate; the caller of `liquidate` receives nothing.
- The perpetuals fee window counts perpetuals volume only. The Hyperliquid 2 x spot-volume weight is not applied; [spot fees](spot.md#fees) use a separate per-trade schedule.

## 2. Margin

- Margin is cross: one USDL collateral balance per account backs every position and resting order of that account.
- Initial margin of a position = notional at mark / requested leverage, rounded down to 1e-18 USD. Requested leverage is bounded by the tier maximum for the resulting notional ([market sheet](perps-markets.md), section 4). Initial margin of a resting order uses its unfilled size at the limit price; untriggered orders do not reserve resting-order margin and are checked on activation.
- Maintenance margin = max(0, ceil(notional at mark x `MaintenanceBps` / 10,000) - `DeductionUsd`). [`TiersFor`](../../../modules/perps/types/tiers.go) sets MMR = 0.5 / tier max leverage and computes each deduction from the preceding tier plus the boundary notional times the change in MMR.
- Account equity = collateral x 1e12 - account deficit + the sum of unrealised PnL and unsettled funding received over all positions. PnL is side x (size x mark / 1e18 - cost basis), rounded toward the venue; funding is negative when paid. See [`margin.go`](../../../modules/perps/keeper/margin.go) and [position settlement](perps.md), section 8.
- An order or position increase is accepted only if equity - the taker fee on the order notional >= initial margin of all positions and resting orders after the change. The fee is reserved at the account's taker rate whether the order later fills as taker or maker.
- Compatibility opens `openPosition` and `addSize` size notional as N = c x L / (1 + L x f), rounded down, with c expressed in USD, L as leverage and f as the taker fraction. Base conversion also obeys lot and execution-bound checks. `placeLimitOrder` and `placeStopLimitOrder` use min(sizeUsd, c x L / (1 + L x f)) at the limit price. See [the venue contract](perps.md), sections 5 and 8; account-wide margin checks still apply.
- Withdrawable = max(0, equity - max(initial margin of positions and resting orders, ceil(total position notional x `WithdrawFloorPpm` / 1,000,000))), converted down to whole USDL units. `WithdrawFloorPpm` defaults to 100,000 (10 percent). Withdraw, removeCollateral and transferCollateral are bounded by this amount and the actual collateral balance; withdrawals and transfers settle the affected accounts first. See [`account.go`](../../../modules/perps/keeper/account.go).
- Leverage arguments use a 1e18 scale with a minimum of 1e18 (1x); `MaxLeverage` configuration fields are plain integers.

## 3. Funding

- Interest component I = `FundingInterestPpm8h`, initially 100 ppm (0.01 percent per 8 hours, or 0.00125 percent per hour).
- Premium sampling is due once the last sample is at least `PremiumIntervalSeconds` old, initially 5 seconds of block time; EndBlock takes at most one sample per market per pass. Premium = (max(impactBid - oracle, 0) - max(oracle - impactAsk, 0)) / oracle. Impact bid and ask are average execution prices of selling and buying `Market.ImpactNotionalUsd` against the book (20,000 USD for seeded BTC/ETH, 6,000 USD otherwise), rounded bid down and ask up. A side with insufficient notional depth uses the oracle. Oracle = pricefeed index price.
- 8-hour rate F = P + clamp(I - P, -C, C), where P is the arithmetic mean of samples accumulated since the last index reset (zero when there are none), and C is the configured premium clamp as a fraction.
- The clamp is `FundingPremiumClampPpm`, initially 500 ppm (0.0005 as a fraction). Hourly rate = F / 8, truncated toward zero, then clamped to the smaller of `Params.FundingCapPpmPerHour` and the market's `FundingCapPpmPerHour`; both default to 40,000 ppm (4 percent per hour).
- Index: when `floor(blockTime / 3600)` advances, the cumulative funding index per unit of base advances by oracle x hourly rate x elapsed hours. The first observed hour initializes the hour and clears samples without charging funding; subsequent advances clear the sample sums after calculating the rate. Longs pay shorts for a positive rate. No position is visited by this index update.
- Lazy settlement: the owed amount is side x (size x current index / 1e18 - stored funding basis), positive when owed by the position. Equity includes unsettled funding at every read. Settlement books it to collateral and resets the basis; the EndBlock cursor sweep also settles accounts as it reaches their positions. Failed or skipped settlements wait for a later pass. The interface uses the opposite sign: negative when the position pays.
- The funding basis rounds up for longs and down for shorts at 1e-18 USD precision. Payments in USDL round up, receipts round down, and conversion residue stays in custody and is booked to `ResidueUsd`. The exact integer formulas and initialization behavior are in [`funding.go`](../../../modules/perps/keeper/funding.go).

## 4. Open interest caps

- Each market has a cap on open interest per side in USD at mark ([market sheet](perps-markets.md), section 2). For class E, `ClassEOiCaps` defaults to 2,000,000 USD from listing day 0, 5,000,000 from day 30, 10,000,000 from day 90 and 25,000,000 from day 180. Age is floor((blockTime - `ListedAt`) / 86400); governance can replace this table.
- A fill that increases OI above the cap in force fails with `PerpsOpenInterestCap()` at the EVM boundary. The cap check admits unchanged or reduced OI, including reduce-only, liquidation, backstop and ADL fills; other admission and execution checks still apply. See [`openinterest.go`](../../../modules/perps/keeper/openinterest.go).
- Long and short open interest are always equal, because every fill adds or removes the same size on both sides.
- There is no borrowing fee and no borrowing index on this venue; positions pay funding only, as on the Hyperliquid venue.

## 5. Mark price

- Mark = median of (a) oracle + EMA(mid - oracle), (b) median(best bid, best ask, last trade price), and (c) pricefeed `Mark`. Oracle is pricefeed `Index`; see [pricefeed mark calculation](pricefeed.md#mark-price).
- `MarkEmaSeconds` defaults to 150. On a newer pricefeed timestamp, EMA basis B advances as `B + (mid - oracle - B) x dt / (dt + tau)`, truncated toward zero, with dt the change in `TimestampMs` and tau = `MarkEmaSeconds x 1000`. The first mark initializes the basis to mid - oracle. This is the timestamp-based rational update in [`mark.go`](../../../modules/perps/keeper/mark.go).
- A missing bid, ask, last trade or pricefeed mark uses the oracle for that component. If either book side is absent, mid uses the oracle.
- Opening orders, market closes and liquidation require pricefeed `GetPrice` to report `Fresh`; the EVM stale-price error is `PerpsPriceStale()`. A stale EndBlock price update stores the freshness flag while retaining mark and EMA; trigger evaluation skips the stale market, and the liquidation scan skips accounts with a stale held market. Cancels and collateral operations remain subject to their own checks, including any account valuation needed. See [pricefeed freshness](pricefeed.md#freshness-and-read-api).

## 6. Liquidation

- Trigger: account equity < account maintenance margin, evaluated at mark in EndBlock and on each `liquidate` call.
- Market liquidation settles the account, cancels its open orders, then submits reduce-only IOC closes against the book. `LiquidationSliceThresholdUsd` defaults to 100,000 USD. Above that threshold, `LiquidationSlicePpm` (200,000, or 20 percent) applies to the base size stored at the first slice, rounded down to the market lot with a one-lot minimum and capped at remaining size. The stored basis persists across slices. `LiquidationCooldownSeconds` defaults to 30 seconds of block time. At or below the threshold, one order attempts to close the remaining size. IOC liquidity and slippage can leave size unfilled.
- Liquidation price with other positions held constant = mark - side x marginAvailable / size / (1 - l x side), with side +1 for long and -1 for short, marginAvailable = equity - maintenance margin, and l = the current tier MMR. The code evaluates the expression as one floor division to 1e-18 USD and floors a negative result to zero.
- Backstop: when equity < `BackstopRatioNumerator` / `BackstopRatioDenominator` x maintenance margin (defaults 2/3), the insurance account takes over every position at mark, netting against any position it already holds, and receives remaining collateral. The user's positions and collateral become zero; any deficit enters deficit coverage. The insurance fund itself is not backstopped.
- No liquidation fee and no penalty is charged; `Liquidation.penalty` is 0.
- Candidates come from the stored margin-ratio index, walked from its lowest bucket. A bucket is min(65535, floor(100 x equity / maintenance margin)); nonpositive equity uses zero and positive equity without maintenance uses 65535. Fresh candidates are revalued and re-bucketed. The scan reads at most `LiquidationScanPerBlock` candidates (1,024), liquidates at most `LiquidationsPerBlock` accounts (64), and submits at most `SlicesPerBlock` closes (256) per scan. The next scan starts at the index's beginning; there is no liquidation-scan continuation cursor.
- The separate position sweep reads up to `SweepPositionsPerBlock` position entries (512) from a persisted id cursor and settles each distinct account encountered. Settling an account visits all its positions, so the entry limit is not a bound on the total positions settled. Skipped markets and failed settlements prevent a fixed refresh-time guarantee. Backstop closure has its own `SlicesPerBlock` allowance, separate from the liquidation scan. Sources: [`liquidation.go`](../../../modules/perps/keeper/liquidation.go), [`insurance.go`](../../../modules/perps/keeper/insurance.go) and [`endblock.go`](../../../modules/perps/keeper/endblock.go).

## 7. Insurance fund and auto-deleveraging

- The insurance fund is the store account `perps_insurance` backed by the USDL balance of the perps module EVM address. It holds backstop positions, receives backstop margin, backstop position profit and `InsuranceSharePpm` of trading fees, and covers account deficits ([venue contract](perps.md), section 8).
- The insurance fund closes its backstop positions against the book in EndBlock with the same slice rule as section 6.
- ADL trigger: an account (including the insurance fund) has a deficit that the insurance fund balance cannot cover.
- ADL rank of an opposite-side profitable position = (mark / entry for a long, entry / mark for a short) x (position notional / account value), with integer fixed-point rounding. The per-market index orders by descending stored rank and ascending position id for ties. Settlement, collateral changes and fills refresh affected account ranks after the final balance is stored; rank is not continuously recomputed on every read.
- [`PreviousCommittedMark`](../../../modules/perps/keeper/adl_recovery.go) supplies the durable mark saved before the block's price update. ADL uses it to check counterparty profitability and execute paired size reductions; a missing or zero previous mark is a stale-price refusal. This snapshot and the scan cursors are stored, so restart does not substitute the live mark or lose continuation.
- `AdlPositionsPerBlock` defaults to 64. [`ExecuteADL`](../../../modules/perps/keeper/adl.go) shares both a counterparty-match budget and an examined-index-entry budget across calls at the same block height. Ineligible entries consume the visit allowance. A per-deficit cursor resumes the rank walk; a deficit cursor rotates the starting deficit. Unfinished work carries to later blocks. A match reduces both positions by the same size and applies eligible counterparty profit toward the deficit; coverage is also bounded by collateral and the outstanding deficit.

## 8. Order limits

- Order types in [`state.go`](../../../modules/perps/types/state.go): LIMIT 0, STOP_LIMIT 1, TAKE_PROFIT 2, STOP_LOSS 3, MARKET 4, STOP_MARKET 5. Time in force: GTC 0, IOC 1, FOK 2, POST_ONLY 3. MARKET and triggered market orders execute as IOC.
- `MinOrderValueUsd`: 10 USD. `MarketOrderMaxValues`: 500,000 USD for market maximum leverage below 10; 2,000,000 for [10, 20); 5,000,000 for [20, 25); 30,000,000 for >= 25. `LimitOrderValueMultiple`: 10 times the applicable market-order maximum. All seeded markets have `MaxLeverage = 1000`, so their default market-order bound is 30,000,000 USD.
- `DefaultSlippagePpm`: 5,000 ppm (50 bps) from mark for a MARKET order without `acceptablePrice`, including compatibility opens and closes that supply no explicit bound. The price is rounded to the tick toward mark. Matching stops at the bound and cancels an IOC remainder; an empty opposite side yields `PerpsNoLiquidity()`, while no fill within the price bound yields `PerpsSlippageExceeded()` at the EVM boundary.
- `MaxOpenOrdersPerAccount`: 1,000. `MaxFillsPerOrder`: 32. IOC remainder is cancelled; GTC/POST_ONLY limit remainder can rest, and FOK must fill completely or fail atomically. See [`orders.go`](../../../modules/perps/keeper/orders.go) and [`matching.go`](../../../modules/perps/keeper/matching.go).
- Self-trade prevention: an incoming order that would match a resting order of the same account cancels that resting order and continues.
- `TriggersPerBlock`: 256 evaluated triggers across enabled markets, ascending market id, rising direction before falling direction, prices in crossing order and then placement sequence. Rising means mark >= trigger price and falling means mark <= trigger price, independently of order side. While paused, the walk uses a separate protective reduce-only index. Refused triggered executions are cancelled after the trigger event; remaining indexed orders wait for another pass. See [`triggers.go`](../../../modules/perps/keeper/triggers.go).
- `ExpiriesPerBlock`: 256 expiry entries per pass, in deadline order; a nonzero deadline expires when it is below block time. Remaining entries wait for another pass. Matching also checks deadlines before fills.

## 9. Consequences of the 1000x cap

- Every market's tier 0 maximum leverage is 1000x; at 1000x the initial margin rate is 10 bps and the maintenance margin rate is 5 bps, so the buffer between opening and maintenance is 5 bps of notional.
- The tier 0 taker fee is 4.5 bps; an order is accepted only if equity less the reserved taker fee still covers initial margin, so a 1000x position opened by a taker order needs collateral of at least 14.5 bps of notional.
- The round-trip tier 0 taker fee is 9 bps, which exceeds the 5 bps buffer: a position opened and closed by taker orders at 1000x returns less than its deposited collateral unless the price moves in its favour by more than 9 bps.
- An adverse price move of approximately 5 bps takes a 1000x position with only its initial margin remaining to maintenance; mark-based notional and rounding determine the exact line. Above 100,000 USD notional the 20 percent slice rule applies; at or below that value one IOC order attempts to close the position.
- The funding cap of 4 percent per hour is 40 times the 1000x initial margin rate: one capped hourly index step exceeds the margin of a 1000x position. Equity includes funding owed by the index at every read, so such a position is liquidatable from the block in which the index advances, without waiting for its funding to settle.
- These figures assume the default fee, funding and tier tables and an otherwise empty account; lot sizing, rounding, other positions and later governance changes affect actual admission and liquidation. Relevant coverage lives in [`margin_test.go`](../../../modules/perps/keeper/margin_test.go) and [`funding_test.go`](../../../modules/perps/keeper/funding_test.go); [the venue contract](perps.md), section 24, describes the accounting invariants.

## Spot fees

The [spot venue](spot.md#fees) uses a PECOR per-trade schedule. [`modules/spot/keeper/fees.go`](../../../modules/spot/keeper/fees.go) computes input USD volume for the single trade: effective bps = base + tier 1 at volume >= 10,000 USD + tier 2 on top at volume >= 100,000 USD. There is no rolling-volume input, and the spot keeper does not use the perpetuals volume buckets. Swap and order rails have separate base, tier and impact settings in [`modules/spot/types/params.go`](../../../modules/spot/types/params.go).

| Go parameter (stored JSON key) | initial value | rail | bound (`Params.Validate`) |
|---|---|---|---|
| `SwapFeeBps` (`swap_fee_bps`) | 7 | swap at 0x1021 | base plus both swap tiers at most 200 |
| `Tier1FeeBps` / `Tier2FeeBps` (`tier1_fee_bps` / `tier2_fee_bps`) | 0 / 0 | swap thresholds above | tier 2 >= tier 1; total at most 200 |
| `OrderFeeBps` (`order_fee_bps`) | 4 | limit/stop orders at 0x1022 and 0x1023 | base plus both order tiers at most 200 |
| `OrderTier1FeeBps` / `OrderTier2FeeBps` (`order_tier1_fee_bps` / `order_tier2_fee_bps`) | 0 / 0 | order thresholds above | tier 2 >= tier 1; total at most 200 |
| `PriceImpactScalarBps` (`price_impact_scalar_bps`) | 100 | swap impact; enabled by default | scalar and computed impact at most 500 bps |
| `OrderPriceImpactScalarBps` (`order_price_impact_scalar_bps`) | 100 | order impact; enabled by default | scalar and computed impact at most 500 bps |

- Fees are deducted from the gross output token amount, rounded down in output token units; impact applies afterward. The fee and impact deduction accrue by rail and output token in the vault, and `collectFees` pays the configured `FeeCollector` (the governance EVM address by default). There is no `spot_fees` store account or maker rebate. Invalid governance parameters return module errors, including `ScalarTooHigh()` for a scalar over 500; these validation errors are not EVM revert data.
- The [opening pairs](spot.md#opening-pairs) are PAX/USDL and SID/USDL, using the registered tokens of `InitV6120`.
- The perpetuals venue keeps counting perpetuals volume only (section 1); the Hyperliquid 2 x spot weight is not applied anywhere, because the spot venue has no rolling-volume tiers.
