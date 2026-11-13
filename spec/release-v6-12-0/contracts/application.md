# Application contract: perpetuals and spot

The [perps application](../../../apps/perps/) calls the native market precompiles and reads the data gateway on Paxeer mainnet-beta, chain ID 125 (`0x7d`). A receipt is final when its block commits. The [perpetuals](perps.md), [spot](spot.md), [data services](data-services.md) and [address table](address-table.md) contracts define the chain and service interfaces.

## Address table

[PRECOMPILES](../../../apps/perps/src/core/network/precompiles.ts) supplies typed viem addresses to the application.

| Role | Default address suffix | Optional override |
|---|---|---|
| Perpetuals | `0x1015` | `NEXT_PUBLIC_PERPS_PRECOMPILE` |
| Launchpad | `0x1017` | `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE` |
| Spot market | `0x1021` | `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE` |
| Spot limit orders | `0x1022` | `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE` |
| Spot stop orders | `0x1023` | `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE` |
| Spot quoter | `0x1024` | `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE` |
| Spot vault | `0x1025` | `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` |

Each suffix is left-padded to a 20-byte address. An absent or empty override uses the default; any other value must be `0x` followed by 40 hexadecimal digits. Invalid overrides throw with the variable name during module initialization. Slots `0x1026` and `0x1027` carry no code and have no application entry.

USDL is the existing ERC20 at `0x85FcD13735F4309833A503EE804ea32395851479`, with six decimals. [Collateral configuration](../../../apps/perps/src/core/perps/config.ts) and the [spot registry](../../../apps/perps/src/core/spot/addresses.ts) use that address. PAX and SID trade through native ERC20 pointers; the chain's [pointer view](../../../precompiles/pointerview/pointerview.go) resolves denoms to pointers.

The [chain definition](../../../apps/perps/src/core/chain.ts) supplies chain ID 125 to clients and the embedded wallet. Its EVM native-currency representation uses 18 decimals; bank `uhpx` and its pointer use six decimals.

## Spot tokens and markets

The [spot registry](../../../apps/perps/src/core/spot/addresses.ts) contains fixed USDL, PAX-pointer and SID-pointer addresses at six decimals. USDL is the only stable quote token. [SPOT_MARKETS](../../../apps/perps/src/core/spot/config.ts) contains PAX/USDL and SID/USDL and derives each base address and decimal count from that registry. [Stable selection](../../../apps/perps/src/core/spot/stableSelector.ts) selects USDL.

The PAX pointer balance and native wei balance represent the same coin with different scales: one `uhpx` base unit corresponds to `10^12` wei. A one-PAX spot amount is `1000000` pointer units.

## ABI modules

The [perpetuals ABI](../../../apps/perps/src/core/network/abi/PerpsPrecompile.ts) exports `PerpsPrecompileABI`, `OrderType` and `TimeInForce`. The [spot ABI barrel](../../../apps/perps/src/core/network/spot_abi/index.ts) exports the five served rails: market, quoter, limit orders, stop orders and vault. The modules use constant ABI tuples for typed call construction.

The [perpetuals call builders](../../../apps/perps/src/core/perps/calls.ts), [spot call builders](../../../apps/perps/src/core/spot/calls.ts) and [spot quote builders](../../../apps/perps/src/core/spot/quoterCalls.ts) bind method arguments to those ABIs. The error classifier uses the declared custom errors.

## Perpetual call sites

Perpetuals calls target `PRECOMPILES.perps`. The builders in [calls.ts](../../../apps/perps/src/core/perps/calls.ts) supply the following methods.

| Action | Precompile method |
|---|---|
| Open or increase a position | `placeOrder` with order type `MARKET` |
| Place a limit or stop-limit order | `placeOrder` with type `LIMIT` or `STOP_LIMIT` |
| Close all or part of a position | `closePositionWithParams` |
| Set position take-profit and stop-loss | `setPositionTpSl` |
| Add or remove collateral | `addCollateral`, `removeCollateral` |
| Top up a position | `topUpPosition` |
| Cancel an order or ladder rung | `cancelOrder`, `cancelByCloid` |
| Read positions and orders | `getPosition`, `getPositionDetails`, `getUserPositionIds`, `getOrder`, `getUserOrderIds` |
| Read market constraints | `getMarketConfig`, `getMarginTiers`, `getMarkPriceDetails` |

A full close sends `closeSizeUsd = 0`; partial close requires a positive 18-decimal USD size. Open and increase calls use the quoted size, leverage, acceptable price and deadline. Market orders use IOC by default or FOK when selected.

The [configuration labels](../../../apps/perps/src/core/perps/config.ts) cover order types LIMIT 0, STOP_LIMIT 1, TAKE_PROFIT 2, STOP_LOSS 3, MARKET 4 and STOP_MARKET 5. Trade types include OPEN 0, CLOSE 1, LIQUIDATION 2, REDUCE 3, INCREASE 4, ADL 5 and BACKSTOP 6.

## Account and collateral

Collateral is USDL and margin is cross margin. `deposit(USDL, amount)` and `withdraw(USDL, amount)` target the perpetuals precompile. The first deposit opens the account. [Account builders](../../../apps/perps/src/core/perps/calls.ts) pass the account explicitly to `getAccount`, `getAccountSummary`, `getBalance` and `getLockedBalance`.

`getAccount(account)` returns collateral, equity, initial margin, maintenance margin and withdrawable funds. Withdraw-all uses `withdrawable`. Deposits, withdrawals, top-ups and collateral adjustments use six-decimal USDL base units. [Collateral input conversion](../../../apps/perps/src/core/perps/collateralInputs.ts) supplies the modal conversions.

## Spot call sites

The [spot order hooks](../../../apps/perps/src/hooks/spot/useSpotOrders.ts) submit calls built by the shared spot module.

| Rail | Methods | Address suffix |
|---|---|---|
| Market | `marketBuy`, `marketSell`, `swapExactInNative` | `0x1021` |
| Limit | `placeLimitBuy`, `placeLimitSell`, `cancelLimitOrder`, `modifyLimitOrder`, `getUserLimitOrders`, `getLimitOrder` | `0x1022` |
| Stop | `placeStopLimitBuy`, `placeStopLimitSell`, `cancelStopLimitOrder`, `placeStopLoss`, `cancelStopLossOrder`, `getUserStopLimitOrders`, `getUserStopLossOrders`, `getStopLimitOrder`, `getStopLossOrder` | `0x1023` |
| Quoter | `quoteMarketBuy`, `quoteMarketSell`, `quoteExactIn`, `quoteExactOut`, `batchQuote`, `getTokenPrice`, `getLiquidityInfo`, `getAllLiquidityInfo`, `getTWAP` | `0x1024` |

A PAX market sell reads the `uhpx` pointer through the market rail. The native-value path sends `swapExactInNative` with `value = amount × 10^12` wei; the ordinary path sends `marketSell` with the pointer. [Call builders](../../../apps/perps/src/core/spot/calls.ts) validate the native conversion before signing. A quote must report positive output and a tradable result.

The [spot page](../../../apps/perps/src/pages/spot.tsx) opens when `NEXT_PUBLIC_RELEASE_SPOT_OPEN=1`. Otherwise server-side routing returns `notFound`. An unknown or malformed market also returns `notFound`; the default market is SID.

## Amounts

| Quantity | Wire units | Conversion |
|---|---|---|
| USDL collateral, deposit, withdrawal and top-up | Six-decimal base units | `safeParseUnits(value, 6)` |
| Prices, USD notionals and partial-close size | 18-decimal USD | `safeParseUnits(value, 18)` |
| Leverage | 18-decimal multiple, at least `10^18` | `safeParseUnits(value, 18)` |
| Fees returned by `getFees` | Parts per million | Divide by `10^6` for a ratio |
| Spot amounts | Token-native decimals | `parseTokenAmount` |
| Native PAX value | Wei, 18 decimals | `uhpx` base units multiplied by `10^12` |

[Amount helpers](../../../apps/perps/src/core/perps/amounts.ts) parse decimal strings without floating-point multiplication. Unknown collateral tokens are rejected rather than assigned a decimal count.

## Approvals

The [approval planner](../../../apps/perps/src/core/network/approvals.ts) checks allowance only for ERC20 inputs outside the supplied native-token set. When allowance is insufficient it approves the exact input amount. Native value and native pointers require no approval.

| Rail | ERC20 spender |
|---|---|
| Perpetuals | `PERPS_SPENDER`, the perpetuals precompile |
| Launchpad | `LAUNCHPAD_SPENDER`, the launchpad precompile |
| Spot market | `SPOT_SPENDERS.market` |
| Spot limit orders | `SPOT_SPENDERS.limit` |
| Spot stop orders | `SPOT_SPENDERS.stop` |

`needsApproval` and `planApproval` receive the native pointer set explicitly. Amounts must be nonnegative and less than the maximum uint256 value. [Spot approval hooks](../../../apps/perps/src/hooks/spot/useSpotApprovals.ts) and [position writes](../../../apps/perps/src/core/perps/positionWrites.ts) apply the corresponding allowance plan before submitting the venue call.

## Time in force

[Order planning](../../../apps/perps/src/core/perps/orderPlan.ts) maps GTC to 0, IOC to 1, FOK to 2 and POST_ONLY to 3. Selecting post-only, IOC or FOK clears the other toggles. With no option selected, resting orders use GTC and market orders use IOC. A market order can use FOK and cannot use post-only.

IOC and FOK resting-order calls carry a market deadline. Other resting calls send deadline zero. Limit and stop-limit calls carry explicit `reduceOnly`, take-profit, stop-loss and client-order IDs.

## Take-profit and stop-loss

Open-position builders validate prices around the reference: a long requires stop-loss below and take-profit above; a short uses the reverse. Zero leaves a side unset. Position updates call `setPositionTpSl`; zero clears that side. When both prices are nonzero their ordering must match the position side.

## Price bound and deadline

`acceptablePrice` computes the bound in bigint arithmetic. Long opens and short closes allow the reference price increased by slippage; short opens and long closes allow the reference decreased by slippage. The default slippage is 50 basis points. Basis points must be an integer in `[0, 10000)`.

[Deadline helpers](../../../apps/perps/src/core/spot/slippage.ts) use a default market deadline of 60 seconds. Market-priced open, increase and close builders supply the price bound and deadline.

## Order plan

[buildOrderPlan](../../../apps/perps/src/core/perps/orderPlan.ts) returns typed calls or a user-facing validation error. It checks market tick size, lot size and maximum leverage. Market quotes older than six seconds are refreshed before submission.

Scaled orders contain 2–20 rungs. Flat, ascending and descending distributions use integer weights, with the remainder assigned to the last rung. Custom distribution is rejected. `ladderCloid` encodes a nonzero 96-bit ladder ID in the high 12 bytes and a zero-based 32-bit rung in the low four bytes of a bytes16 ID. Each rung is submitted as a separate order.

## Gas and confirmation

[writeContract](../../../apps/perps/src/hooks/perps/useContract.ts) simulates with the connected account, estimates gas and applies the [gas margin](../../../apps/perps/src/core/network/gas.ts). It estimates EIP-1559 fees and clamps both fee fields to at least 10 wei, with maximum fee covering the priority fee.

`waitForTx` verifies chain ID and polls transaction receipts every 400 ms for up to 30 seconds. A matching successful receipt completes the wait; a reverted receipt raises an error. Broadcast hashes are retained for pending-transaction reconciliation. Sender lookup runs independently of receipt polling. The committed receipt supplies finality on this BFT network.

## Data surfaces

1. [gatewayUrls](../../../apps/perps/src/core/network/gateway.ts) requires `NEXT_PUBLIC_PERPS_GATEWAY_URL` as a bare HTTP(S) origin with no credentials, query or fragment. It derives REST, GraphQL, venue socket and spot-user socket URLs.
2. [feeds.ts](../../../apps/perps/src/core/feeds.ts) supplies Chainflow reference data through `NEXT_PUBLIC_MARKET_PROXY_HOST`, accepting a host or bare HTTP(S) origin. Candle symbols are validated against the registered catalogue. Venue books and fills come from the gateway.
3. Market ticker and open-interest reads in [useQuoter](../../../apps/perps/src/hooks/perps/useQuoter.ts) use `MARKET_DATA_URL`, derived from `FEED_ORIGIN`. The gateway also serves venue REST aliases for API clients; see [data services section 24](data-services.md#24-venue-rest-routes).

| Surface | Derived path |
|---|---|
| Perpetuals REST | Gateway origin plus `/api` |
| GraphQL and subscriptions | Gateway `/graphql`, HTTP(S) and WS(S) respectively |
| Venue topic socket | Gateway `/ws` |
| Spot account REST | Gateway `/v1/users/...` |
| Spot account socket | Gateway `/v1/ws` |
| Account sign-in and writes | Gateway `/v2/...` |
| Candle and reference market data | Configured feed origin |
| Chain reads and transaction submission | Node RPC configuration through `getPaxeerRpcUrls` |

## Data hooks REST

[Spot API helpers](../../../apps/perps/src/core/spot/api.ts) use the gateway's bare origin for holdings, positions, orders, trades, closed legs, PnL and identity. [PerfDB](../../../apps/perps/src/core/perfdb/client.ts) derives GraphQL and authenticated REST paths from the same gateway configuration. [Data services](data-services.md#25-spot-user-routes-and-socket) defines response fields, raw amount scaling and pagination.

## Venue streams

[venueStream.ts](../../../apps/perps/src/core/net/venueStream.ts) shares gateway topic subscriptions and validates frames. Perpetuals topics are `<SYMBOL>@perp_orderbook` and `<SYMBOL>@perp_trade`; spot topics are `<SYMBOL>@orderbook` and `<SYMBOL>@trade`. Subscribe and unsubscribe frames use `{event, topic}`. Subscription count is bounded at 200, fill retention at 100 and resynchronization depth at 100.

Spot account updates use `/v1/ws` with streams `trade`, `position`, `pnl`, `order` and `closed-leg`. Perpetuals account updates use GraphQL subscriptions.

## WebSocket resume

[sequenceGuard](../../../apps/perps/src/core/net/realtime.ts) tracks sequence and snapshot identity. A snapshot resets the baseline. A sequence gap or changed snapshot invalidates incremental state; venue streams close with code 4000 and restore state from REST before continuing. Reconnection uses the active-page scheduler and socket retry policy.

## GraphQL operations

The gateway's [executable schema](../../../services/platform/perps-data/internal/graphql/schema.go) defines field types and nullability. [Application queries](../../../apps/perps/src/hooks/perps/useSubgraph.ts) declare 44 venue query operations, including positions, orders, fills, balances, fees, funding, market snapshots and supported spot records. [Data services section 14](data-services.md#14-graphql-operation-set) lists the 43-operation venue conformance set; the application also declares `GetMarket`, selecting the supported `market` root.

[Subscriptions](../../../apps/perps/src/hooks/perps/useIndexerSubscriptions.ts) consume indexer status, latest prices, price updates, position changes, order changes, trades and liquidations. The shared GraphQL client uses `graphql-transport-ws`, `connection_init` and `subscribe` frames. Missing values retain their schema-defined nullability.

## PerfDB operations

The account operations are `User`, `UserBalances`, `UserDailyPnl`, `UserStats`, `UserAlerts`, `UserReferrals`, `ReferralTiers`, `Competitions` and `Competition`. They use the same gateway GraphQL endpoint. Account writes use the authenticated REST routes in [data services section 26](data-services.md#26-wallet-sign-in-and-account-routes).

## Embedded wallet

The [embedded-wallet resolver](../../../apps/perps/src/core/wallet/embedded/client.ts) returns null during server-side rendering or when `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` is absent. When enabled it requires `NEXT_PUBLIC_PAXEER_WALLET_API`. It reads optional `NEXT_PUBLIC_SUPABASE_URL`, obtains chain ID from `paxeerChain` and selects RPC through `getPaxeerRpcUrls`.

The wallet client is reused for the same API and authentication configuration. The authentication callback derives from the browser origin unless `NEXT_PUBLIC_AUTH_REDIRECT_URL` overrides it. Injected wallets use the same chain and call interfaces.

## Deploy configuration

| Variable | Purpose |
|---|---|
| `NEXT_PUBLIC_PERPS_GATEWAY_URL` | Required gateway origin |
| `NEXT_PUBLIC_MARKET_PROXY_HOST` | Reference feed host or bare origin |
| `NEXT_PUBLIC_PAXEER_RPC_URLS` | Comma-separated RPC override list; duplicates, empty entries and excluded legacy endpoints are removed |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Preferred RPC URL followed by defaults when the override list has no usable entries |
| Precompile overrides in the address table | Optional address selection |
| `NEXT_PUBLIC_RELEASE_SPOT_OPEN` | Set to `1` to open the spot route |
| Embedded-wallet variables above | Optional wallet configuration |

The [RPC resolver](../../../apps/perps/src/core/rpc.ts) uses the default list when neither override supplies a usable endpoint. The [application package](../../../apps/perps/package.json) pins pnpm 10.34.9. Run its `dev`, `build`, `start`, `typecheck` and `test` scripts from `apps/perps/`. Deployment supplies public browser configuration at build time.

## Errors

[classifyError](../../../apps/perps/src/core/errors/classify.ts) decodes custom errors from the perps and spot ABIs using `PRECOMPILE_ERROR_CODES`, then applies transport and message classification. Errors retain typed user-facing codes. Unknown reverts follow the generic revert path. [Error tests](../../../apps/perps/tests/classify-precompile.test.ts) exercise declared error mappings.

## End-to-end test

The [local-node test](../../../apps/perps/tests/e2e-local-node.test.ts) uses `LOCAL_NODE_RPC`, `LOCAL_NODE_KEY` and `NEXT_PUBLIC_PERPS_GATEWAY_URL`. It requires each variable and a valid funded test key. It exercises approval, deposit, market open with take-profit and stop-loss, readback, limit placement and cancellation, collateral addition, partial and full close, indexed funding, withdrawal and spot quote/pointer checks through the real application interfaces.

## Invariants

Native source amounts use explicit decimal scales. Perpetuals calls use typed precompile builders; native pointers and ERC20 allowance inputs retain separate custody semantics. Market-priced writes carry price and time bounds. Data gateway URLs derive from one origin, while node RPC and the reference feed have their own configuration. Successful transaction state comes from a committed receipt.
