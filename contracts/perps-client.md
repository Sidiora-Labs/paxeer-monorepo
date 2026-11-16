# Perps client contract

The [Paxeer Xtrades workspace](../apps/perps) serves its Next.js web application and trades the native [market
contracts](markets.md) on Paxeer mainnet-beta, EVM chain ID 125 (`0x7d`). The
LayerX kernel inside `paxd` executes and orders each transaction and builds each
block; validators check and commit it, and a transaction receipt is final when
its block commits. The [data services](perps-data.md) provide indexed views and
subscriptions; the client sends chain reads and transactions to node JSON-RPC.

## Env

[Gateway URL resolution](../apps/perps/packages/paxeer-backend/src/core/network/gateway.ts) requires
`NEXT_PUBLIC_PERPS_GATEWAY_URL` at module load. Supply a bare HTTP(S) origin
without credentials, a path, query or fragment. The resolver derives:

| Purpose                            | Path on the gateway origin |
| ---------------------------------- | -------------------------- |
| REST API                           | `/api`                     |
| GraphQL queries                    | `/graphql`                 |
| GraphQL subscriptions              | `/graphql`, using WS(S)    |
| Shared topic stream                | `/ws`, using WS(S)         |
| Spot account stream                | `/v1/ws`, using WS(S)      |
| Spot API and performance-data base | The origin itself          |

This origin does not supply node JSON-RPC. The client resolver in
[rpc.ts](../apps/perps/packages/paxeer-backend/src/core/rpc.ts) reads
`NEXT_PUBLIC_PAXEER_RPC_URLS` as a comma-separated override. If it has no usable
entries, `NEXT_PUBLIC_PAXEER_RPC_URL` prepends a single override to the defaults;
otherwise defaults apply. It trims and deduplicates entries and filters legacy
hosts. The default pool contains fourteen numbered node origins and one
aggregate origin, all using HTTPS at the root path. Configure an explicit
validated pool for the deployment.

[chain.ts](../apps/perps/packages/paxeer-backend/src/core/chain.ts) fixes chain ID 125 and PAX at
18 decimals. The transport uses viem fallback with endpoint ranking, ten-second
HTTP timeouts and a shared fallback retry budget of two retries, with a
200 ms retry delay. Endpoint ranking samples three observations every thirty
seconds, with a one-second ranking timeout.

| Variable                                                    | Reader and behavior                                                                                                                      |
| ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS`                     | `core/feeds.ts`: comma-separated browser origins allowed to call the public feed directly                                               |
| `PRICE_FEED_ORIGIN`                                         | Server Chainflow upstream origin; the default is declared in `core/feeds.ts`                                                             |
| `PRICE_FEED_REST_API_KEY`, `PRICE_FEED_WS_API_KEY`          | Server-only credentials used by the price-feed proxy                                                                                     |
| `NEXT_PUBLIC_PERPS_PRECOMPILE`                              | Optional address override for perps; default `0x1015`                                                                                    |
| `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE`                          | Optional launchpad address override; default `0x1017`                                                                                    |
| `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE`                       | Optional PECOR address override; default `0x1021`                                                                                        |
| `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE`                       | Optional limit-order address override; default `0x1022`                                                                                  |
| `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE`                         | Optional stop-order address override; default `0x1023`                                                                                   |
| `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE`                       | Optional quoter address override; default `0x1024`                                                                                       |
| `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE`                        | Optional vault address override; default `0x1025`                                                                                        |
| `NEXT_PUBLIC_PAXEER_WALLET_API`                             | Embedded wallet API override                                                                                                             |
| `NEXT_PUBLIC_SUPABASE_URL`                                  | Embedded wallet identity URL override                                                                                                    |
| `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`                      | Required to enable the embedded wallet; absent means that feature is unavailable                                                         |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL`                             | OAuth return URL override; otherwise browser origin plus `/auth/callback`                                                                |
| `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN`                          | Origin for the optional postMessage EIP-1193 bridge                                                                                      |
| `NEXT_PUBLIC_PAXSCAN_API_URL`, `NEXT_PUBLIC_PAXSCAN_WS_URL` | Explorer API and socket URLs; default to empty                                                                                           |
| `NEXT_PUBLIC_PAXSCAN_BASE_URL`                              | Optional bare HTTP(S) explorer origin; otherwise uses the explorer origin declared in `chain.ts`                                                    |
| `NEXT_PUBLIC_RELEASE_SPOT_OPEN`                             | Spot navigation release switch; only the value `1` enables it                                                                            |
| `NEXT_PUBLIC_LOG_LEVEL`                                     | `debug`, `info`, `warn` or `error`; default `warn` in production and `info` otherwise                                                    |

[precompiles.ts](../apps/perps/packages/paxeer-backend/src/core/network/precompiles.ts) requires overrides
to be `0x` plus 40 hex digits. Short addresses in this page are zero-padded to
20 bytes in the code. An address override changes the client's destination;
it does not deploy or register a chain precompile.

The [workspace environment example](../apps/perps/apps/web/.env.example) lists
the public settings above and the server-only price-feed settings.
`NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS` selects browser origins that may call the
public feed directly; other origins use the web app's `/api/price-feed` proxy,
which reads `PRICE_FEED_ORIGIN`, `PRICE_FEED_REST_API_KEY` and
`PRICE_FEED_WS_API_KEY` on the server. Public-prefixed values are shipped to
browsers, while feed credentials remain server-only.

## Core

[core/perps](../apps/perps/packages/paxeer-backend/src/core/perps) owns typed calls, order planning,
quotes, market configuration, account views and position actions.
[The ABI](../apps/perps/packages/paxeer-backend/src/core/network/abi/PerpsPrecompile.ts) supplies tuple
components and return types; [calls.ts](../apps/perps/packages/paxeer-backend/src/core/perps/calls.ts)
returns `{address, abi, functionName, args}` descriptors for the configured
precompile. Keep amounts as `bigint` through construction and submission.

USDL collateral is `0x85FcD13735F4309833A503EE804ea32395851479`, six decimals.
Prices, USD notionals and leverage multiples use 18 decimals. A 2x leverage
argument is `2000000000000000000`, not `2`. PAX gas currency uses 18 decimals;
that does not change the registered spot token's decimals.

[Order planning](../apps/perps/packages/paxeer-backend/src/core/perps/orderPlan.ts) reads chain tick,
lot and leverage limits. Market orders use `placeOrder` with MARKET and IOC
(or FOK), a quote-derived acceptable price and a deadline. Limit and stop-limit
orders also use `placeOrder`; GTC, POST_ONLY, IOC and FOK are explicit choices.
Resting orders carry nonzero `bytes16` client IDs. `ladderCloid` places the
ladder ID in the high 12 bytes and rung number in the low four bytes.
Scaled plans accept 2–20 rungs and enforce tick/lot alignment.

A market quote older than six seconds is fetched again before submission.
[Position planners](../apps/perps/packages/paxeer-backend/src/core/perps/positionWrites.ts) make market
opens, size additions and orders against deposited collateral. They do not add
wallet approvals to those `placeOrder` plans. Deposits and `addCollateral`
use the [approval planner](../apps/perps/packages/paxeer-backend/src/core/network/approvals.ts), requesting
the exact amount when allowance is short and respecting native-token custody.
Withdrawals, removals and closes do not require an input-token approval.

Full closes use `closePositionWithParams` with `closeSizeUsd=0`; partial closes
require a positive size. Both carry an acceptable price and deadline.
TP/SL prices use 18 decimals and zero clears a side. `buildWithdrawAll` uses
`getAccount().withdrawable`, not the idle balance.

[getPosition decoding](../apps/perps/packages/paxeer-backend/src/core/perps/positionWrites.ts) names
outputs from the ABI and refuses tuples with the wrong arity. Distinguish the
[perps custom errors](markets.md#perps-errors) from display errors in
[core/errors](../apps/perps/packages/paxeer-backend/src/core/errors). Error classification and reporting
must preserve actionable failures such as stale price, margin, precision,
slippage, deadline and authorization refusals.

## Hooks

[hooks/perps](../apps/perps/packages/paxeer-backend/src/hooks/perps) connect account and market reads,
quotes, writes and receipt handling to UI state. Refresh indexed views after
chain writes, while allowing for indexer lag after a committed receipt.
An empty or unavailable history response does not undo chain commitment.

[`usePerpsMarkets()`](../apps/perps/packages/paxeer-backend/src/hooks/perps/usePerpsMarkets.ts) takes
no arguments and returns `{ catalog, tradeable, isLoading, error, refetch }`.
`catalog: GatewayCatalogEntry[]` includes disabled markets and preserves nullable
precision, tick/lot, tier and leverage fields. Entries contain `marketId`,
`symbol`, `name`, `szDecimals`, `pxDecimals`, `maxLeverage`, `enabled`, `listedAt`,
`category`, `baseToken`, `oracleSymbol`, `tickSize`, `lotSize` and `tierClass`.
`baseToken` is the gateway symbol string. `tradeable` contains only enabled
entries, each with non-null numeric `maxLeverage`; it also satisfies
`TradeableMarket`. No catalog or risk limits are invented for missing data.

The hook loads the gateway `GetMarkets` GraphQL query through React Query with
key `["perps", "markets"]` and no automatic retry. Before the first response,
`catalog` and `tradeable` are empty, `isLoading` is true and `error` is null.
A valid empty response has empty arrays, false `isLoading` and null `error`.
A failed query
exposes `error` and empty arrays. Network, GraphQL and decoder errors include
failed refreshes with older data in React Query's cache. `isLoading` is
`query.isPending`, so a background refetch retains available arrays while
pending and does not restore the initial loading flag. `refetch` is React
Query's refetch function. Consumers use the existing `catalog`/`tradeable`
fields; unavailable data must not be treated as a supported trading market.
Successful catalog loads also register venue symbols. Default React Query
staleness, mount/focus/reconnect and cache policies apply; this hook adds no
polling interval.

`loadPerpsMarkets()` returns `{ catalog, tradeable }` after decoding the gateway
GraphQL `MARKETS_QUERY` through `decodeGatewayMarketsResult` and
`catalogFromGateway`. The loader rejects on query or decoding failure;
`usePerpsMarkets()` exposes that rejection through `error` and returns empty
arrays. Consumers check loading and error before treating an empty catalog as
a successful empty market set.

[`useFeeTier(address: string | undefined)`](../apps/perps/packages/paxeer-backend/src/hooks/perps/useFeeTier.ts)
returns `{ view, isLoading, error }`. `view` contains `tier`, `takerBps`,
`makerBps` (including negative maker rebates), and exact `volume14dUsd: bigint`.
A missing address disables the read: `view` is undefined, `isLoading` is false,
and `error` is null. A selected address loads with `isLoading: true`; invalid
addresses, RPC failures and malformed ABI results expose `error` and an
undefined `view`. Failures are not automatically retried. Consumers must retain
the unavailable state instead of inventing a fee tier.

[`useMarketConfig(marketId: bigint)`](../apps/perps/packages/paxeer-backend/src/hooks/perps/useMarketConfig.ts)
requires a market ID and returns the React Query result. Successful `data` is
`{ config, tiers }`, with ABI-derived bigint limits; there are no top-level
`config` or `tiers` fields. Before the first response `data` is undefined and
`isPending`/`isLoading` describe the pending read. RPC, ABI and invalid-limit
failures expose `error`/`isError` without an automatic retry. React Query retains
the last successful `data` during refresh and on a refresh failure; consumers
must check the error when deciding availability. An unknown market is
unavailable; a valid `config.enabled: false` disables order limits and trading.
Config and margin tiers refresh as one query, with 30-second freshness and
polling while active, stale reads on mount/focus/reconnect, stream-resync
invalidation, and five-minute unused-cache retention. Governance changes become
visible to mounted consumers through those reads without a reload.

`usePerfdbTraderStats(address)` and `usePerfdbUser(address)` normalize the
selected address and return `{ stats, isLoading, error }` and
`{ user, isLoading, error }`, respectively. Replacement clears prior-account
data and errors immediately; disconnect returns null data, false loading and
null error. Successes and failures from earlier request generations cannot
modify the current selection, including a reconnect to the same account.

`usePortfolioStats()` scopes snapshots to one account session. A full-list
`positionChanged` push supersedes all position snapshots already pending in
that session. Older overlapping snapshots and responses from stopped sessions
are discarded. Reopening the consumer starts a fresh snapshot/subscription;
reconnect resync follows the same ordering rule.

`useQuoter(marketDataUrl?)` uses the normalized HTTP(S) base URL, including its
path, for both ticker and open-interest requests. It shares open-interest
cache/in-flight work only within that base URL. A quote's `openInterest`
metadata is `{ origin, source, fetchedAt, ageMs }`: `origin` is the normalized
base URL, times are epoch milliseconds/elapsed milliseconds, and `source` is
`fresh`, `cache`, `stale` or `unavailable`. Cached rows are fresh for sixty
seconds; failed refreshes may use same-origin rows for at most five minutes
since their successful fetch. Unavailable OI has no rows; `fetchedAt`/`ageMs`
are null if no successful fetch exists. A whole-quote on-chain fallback uses
`source: "on-chain"`, null origin and null times. Consumers can distinguish
absent or stale OI from an observed zero.

`useRealtimePrices({ enabled? })` expires each price and ticker thirty seconds
after its `price_updated_at`. A cancellable deadline wakes the consumer even
when its shared market WebSocket remains open without sending frames. Expired
rows are omitted from both maps; a newer frame establishes a new deadline.

[venueStream.ts](../apps/perps/packages/paxeer-backend/src/core/net/venueStream.ts) multiplexes topics
on a shared socket per origin. Perps use `<SYMBOL>@perp_orderbook` and
`<SYMBOL>@perp_trade`; spot book/trade topics are available for PAX and SID.
Frames are validated before being exposed to widgets; malformed book levels
refuse the entire frame. A new `snapshotId` or a sequence gap requires a fresh
baseline. Reconnects use backoff, and unavailable data remains unavailable.
The server's exact frames and limits are in the
[data WebSocket contract](perps-data.md#websocket).

[useSpotTokenState](../apps/perps/packages/paxeer-backend/src/hooks/spot/useSpotTokenState.ts) has the
signature `useSpotTokenState(token: Address, native?: boolean): SpotTokenStateResult`.
The token address is required; `native` defaults to `false`. Native reads resolve
the PAX pointer before reading the vault. The return value is
`{ state: SpotTokenState | null, error: string | null }`.
`SpotTokenState` contains `tradeable`, `paused` and
`reason: "TokenPaused" | "TokenNotRegistered" | null`.
While the initial read is loading, both fields are null. Invalid addresses,
failed reads and invalid response shapes yield null state and a classified
error message, even if a previous read succeeded. Queries do not retry and
refresh every eight seconds; a same-token background refresh retains cached
state until a failed read makes it unavailable. `SpotOrderForm` consumes
`state` and `error`, blocks submission for unavailable state, and displays the
error or “Checking token trading status…” while unavailable.

## Routes

The [Pages Router](../apps/perps/apps/web/src/pages) exposes the workspace
routes `/trade`, `/trade/grid`, `/portfolio` and `/markets`, alongside the root
route. The web application keeps routing in the catch-all page and native layout
bindings in `apps/perps/apps/web/src/native`; spot navigation follows its release
switch.

The server API handler at `/api/price-feed/[...path]`
([source](../apps/perps/apps/web/src/pages/api/price-feed/%5B...path%5D.ts)) accepts
only GET and HEAD, validates feed paths and symbols, and forwards approved
market-data requests through the configured Chainflow client. Transaction
transport uses the configured chain nodes.

## Instrumentation

[The web entry point](../apps/perps/apps/web/src/pages/_app.tsx) mounts the
workspace providers and native layout. Server-only feed credentials are read by
the price-feed route; malformed requests are rejected at that boundary.

## App

[app](../apps/perps/packages/app/src) holds providers and shared shell. Chain and wallet
providers use the canonical chain definition. The
[embedded wallet client](../apps/perps/packages/paxeer-backend/src/core/wallet/embedded/client.ts) returns
null during SSR and when its publishable key is missing. Obtain the OAuth
redirect URL from browser/event-handler context.

## Pages

[pages](../apps/perps/apps/web/src/pages) contains Next.js route entry points and API
handlers. Route files compose their page modules and widgets; chain and
transport policy belongs in core readers and builders.

## Page modules

[page-modules](../apps/perps/packages/portfolio/src) composes page-level views. Keep
market identifiers from authoritative chain/catalog data when connecting these
views to perps reads and writes.

## Widgets

[widgets](../apps/perps/packages/paxeer-backend/src/widgets) contains trading, market, portfolio,
analytics, explorer and mobile panels. Book and tape widgets consume validated
venue streams; account actions consume the call planners. Indexed analytics
and external chart feeds are views of data, not execution receipts.

## Components

[components](../apps/perps/packages/ui/src) supplies reusable application
components. Preserve loading, empty, unavailable and rejected states when
presenting data or wallet actions.

## UI

[ui](../apps/perps/packages/paxeer-backend/src/ui) holds shared atoms, tokens, hooks and formatters.
Display formatting may convert fixed-point values for presentation; it must not
feed rounded display numbers back into transaction construction.

## Types

[types](../apps/perps/packages/paxeer-backend/src/types) and ABI-derived types carry application data
shapes. Match GraphQL scalar nullability and socket frame fields to the
[data service contract](perps-data.md), rather than inventing defaults for
missing source facts.

## Utils

[utils](../apps/perps/packages/utils/src) holds shared presentation helpers. Use domain
amount and order helpers in core for financial values and precompile calls.
