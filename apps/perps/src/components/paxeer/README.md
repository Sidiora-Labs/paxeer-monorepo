# Paxeer Components — Visual Demonstrations

This directory is a collection of React visual demonstrations and experiments
exported by [`index.ts`](index.ts). The components contain sample data, local
state, timers and simulated interactions. They are not a production trading
SDK and are not mounted by the application's current page modules. A rendered
balance, network status, successful button animation or slider completion here
does not establish a wallet connection or submit a transaction.

For production UI work, start with the wired widgets listed below and the
[data API guide](../../../data_api.md). The [deployment guide](../../../deploy/README.md)
explains gateway configuration; the
[SettingsModal guide](../../app/modals/SettingsModal.md) distinguishes working
themes from local-only preference switches.

## Install

These files already live inside the Next.js application's `src` tree and use
its `@/` alias. No separate package install or folder-copy workflow is needed
for this checkout. They use React and local helpers in [`_shared.tsx`](_shared.tsx),
with inline styles, SVGs and CSS variables. The demonstration token sheet is
[`paxeer-tokens.css`](../../styles/paxeer-tokens.css); the current application
loads [`globals.css`](../../styles/globals.css) and `apple-ui.css` through
[`pages/_app.tsx`](../../pages/_app.tsx), rather than importing that demo sheet.

Copying a component into another project requires reviewing its real imports,
style/font assets, TypeScript types, timers and interactions. The collection
is not published with an independent compatibility or production-readiness
contract. Components using hooks are client components; that directive does
not certify server rendering or accessibility behavior.

## Usage

For a local visual preview, use the exported components with their actual
signatures:

```tsx
import { Overview, OrderBook, ConfirmSlider } from "@/components/paxeer";

export function ComponentPreview() {
  return (
    <div style={{ display: "grid", gap: 16 }}>
      <Overview />
      <OrderBook />
      <ConfirmSlider label="Preview interaction" confirmedLabel="Preview complete" />
    </div>
  );
}
```

This displays demonstration content. It is not an example of live market data
or order confirmation.

## Wiring live data

The application uses the Pages Router. Follow
[`page-modules/perps/index.tsx`](../../page-modules/perps/index.tsx), which
composes the following widgets and their domain hooks:

| Surface | Wired implementation |
| --- | --- |
| Pair header and market selection | [`PairHeader`](../../widgets/trading/PairHeader.tsx), [`MarketSelectorPanel`](../../widgets/trading/MarketSelectorPanel.tsx) |
| Orderbook and depth | [`Orderbook` / `DepthPanel`](../../widgets/trading/Orderbook.tsx) |
| Chart and candle datafeed | [`Chart`](../../widgets/trading/Chart.tsx) |
| Order entry | [`PerpsOrderForm`](../../widgets/trading/PerpsOrderForm.tsx) |
| Position management | [`PerpsPositionsPanel`](../../widgets/trading/PerpsPositionsPanel.tsx) |
| Markets table | [`MarketsTable`](../../widgets/markets/MarketsTable.tsx) and [`useMarketsRows`](../../widgets/markets/useMarketsRows.ts) |
| Wallet identity and collateral operations | [`WalletControlPanel`](../../app/modals/WalletControlPanel.tsx), [`DepositWithdrawModal`](../../app/modals/DepositWithdrawModal.tsx) |

Use each widget's declared props and its existing parent for provider/context
requirements. Market streams are in
[`hooks/perps/useMarketsStream.ts`](../../hooks/perps/useMarketsStream.ts) and
[`hooks/perps/useSidPaxMarketStream.ts`](../../hooks/perps/useSidPaxMarketStream.ts);
chain and gateway configuration live under `core`. The demo `OrderBook` is a
different export from the wired `Orderbook` widget (including capitalization).

Paxeer mainnet-beta is one network, chain ID 125. The integrated kernel
participates in proposal and receipts are final at block commit. A demo
network selector does not switch the real chain. Use the actual chain/wallet
integration, configured node root JSON-RPC endpoints and application gateway.

The demo `Leaderboard` has no `rows` prop and the demo `OrderBook` has no `data`
prop. Wrapping either in a fetch hook cannot inject values through an absent
interface. Adapting a demonstration requires an explicit typed data/callback
contract and real consumer wiring. Follow the existing application transport
and unavailable/error handling; do not carry sample balances or simulated
success states into trading surfaces.

## Component map

### Primitives

These are the current interfaces of the components previously presented as
generic primitives:

| Component | Actual input and behavior |
| --- | --- |
| `AddressChip` | No props; sample address/name/balance and illustrative QR/actions |
| `SmartButton` | No props; clicks cycle local idle/loading/success states using timers |
| `DataInput` | No props; local amount and token selection using sample tokens |
| `NetworkPill` | No props; local selection among predefined sample networks/latencies |
| `ConfirmSlider` | `label`, `confirmedLabel`, `accent`, each with a default; pointer drag changes local state and resets after a timer |

`AddressChip` does not accept `address`/`ens`; `SmartButton` has no caller
`onClick`; `DataInput` has no controlled `value`/`onChange`; `NetworkPill` has
no `chainId`; `ConfirmSlider` has no `onConfirm` callback. Its sample swap
summary is fixed even when the label changes.

### Trading

`OrderEntry`, `OrderBook`, `RecentTrades`, `DepthChart`, `PositionCard` and
`MagneticSwap` illustrate trading layouts. Their sample figures and local
interactions are not market execution or portfolio accounting. Use the wired
trading widgets for application work.

`PositionCard` accepts a `data` object with `sym`, `side`, `size`, `entry`,
`mark`, `lev`, `liq`, `funding`, `pnl` and `pnlPct`. A falsy `data` uses its
sample position; its action buttons have no execution handlers. This display
shape is not the application's canonical position type.

### Analytics

`Overview`, `Performance`, `CumulativePnL`, `FeesBreakdown`, `ActiveOrders`,
`ClosedPositions`, `TradesTable` and `TransfersHistory` illustrate statistics,
charts and tables. They do not obtain the connected account's history.

### Social

`Leaderboard`, `LeaderboardStats`, `TraderCard` and `ReactionRail` demonstrate
ranking, profile and reaction layouts. Do not infer real copy-trading,
identity or leaderboard endpoints from their content.

### Novel / experimental

The barrel also exports `CommandPalette`, `LeverageDial`, `GestureUnlock`,
`TradePathSankey`, `OrderbookTopo`, `GhostHold`, `GasWeather`, `TxSimulator`,
`RiskDial`, `EpochCountdown`, `TransferModal`, `LiquidStat`, `OrbitalSelector`,
`PulseFeed`, `RadialAllocator` and `ShardStatus`. Read each source for its
individual interface and local behavior; names such as unlock, simulator or
status do not imply security, transaction simulation or network telemetry.
`TransferModal` accepts `initialMode` (default `deposit`, with `withdraw` used
for the alternate view). It is a demonstration distinct from the wired
collateral modal.

## Customization

Styles use shared CSS variables including `--bg`, `--fg`, `--border-soft`,
`--font-body` and `--accent-solid`. Review the actual token stylesheet and
component values before overriding them. Changing a visual token does not add
live data or a transaction handler, and adding a theme selector does not make
every demo support that theme.

## Types

[`_shared.tsx`](_shared.tsx) exports `Token`, the sample `TOKENS` array, `Icon`,
`cx`, `fmt`, `shortAddr`, `TokenBadge` and `useAnimatedNumber`. They are
re-exported through the barrel:

```tsx
import { TOKENS, fmt, shortAddr } from "@/components/paxeer";
import type { Token } from "@/components/paxeer";
```

These are demo helpers/data, not the canonical market catalog, token registry,
wallet balance source or amount parser for transaction construction.

## Notes / caveats

Review keyboard and screen-reader operation, hydration, timers, cleanup,
responsive behavior and types before adapting a component. The gallery's
illustrative QR code, latency values, balances and local success animations
must not be presented as operational results. For supported application flows,
trace a wired widget through its actual hooks and transports using the
[API contract](../../../data_api.md).
