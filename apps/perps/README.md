# Paxeer Xtrades

Paxeer Xtrades is the trading workspace for the perpetual markets of Paxeer.

## Run locally

```sh
corepack pnpm install
pnpm dev
```

Open http://localhost:3000/trade. Use `/trade/grid` for the grid renderer, `/portfolio` for account pages, and `/markets` for market discovery. Split layout choices and panel sizes persist locally. The grid picker offers Exchange Style and Order Entry Left, each with its own saved arrangement. Mobile uses the duplicated SDK mobile components and bottom navigation.

```sh
pnpm typecheck
pnpm check:ui
pnpm test
pnpm test:browser          # requires the running preview and Playwright Chromium
pnpm build
pnpm start
```

Development and production both use Turbopack and use separate output directories (`.next-dev` and `.next-production`). Building does not overwrite the running development server's output.

## Packages

`apps/web` owns routing, theme, app configuration, and registration of native layout plugins. The duplicated SDK packages live under `packages/` with `@paxeer-network/*` ownership. Components retain the original JSX, styles, and responsive structure; controllers and services connect them to native functionality.

| Package | Responsibility |
| --- | --- |
| `ui`, `ui-scaffold`, `i18n` | Shared controls, navigation, dialogs, typography and translations |
| `layout-core`, `layout-split`, `layout-grid` | Original layout strategies, draggable/resizable panels and layout controls |
| `trading-next`, `ui-order-entry`, `ui-orderbook`, `ui-tradingview` | Desktop/mobile trading modules |
| `ui-positions`, `ui-orders`, `ui-tp-sl`, `ui-transfer` | Account actions and dialogs |
| `portfolio`, `markets` | Original SDK page modules |
| `hooks`, `core`, `net`, `wallet-connector` | Native state, request transport, wallet and transaction bindings |
| `paxeer-backend` | paxeer-network contracts, vaults, indexer queries, embedded wallet and original market/candle streams |

`pnpm check:ui` verifies the presentation files against `docs/ui-source-baseline.json`.

## Native services and current boundaries

The application uses Paxeer Mainnet (chain 125), the original paxeer-network Diamond and vault contracts, the original GraphQL indexer, and paxeer-network's per-symbol market streams and TradingView datafeed. Injected wallets and the original embedded wallet are available. No exchange API account, exchange trading key, sample prices or fake transaction receipts are used.

Market opening uses a fresh native contract quote bound to the wallet, network and exact order inputs. Native limit/stop-limit placement, position market closing, order cancellation, token allowance and vault deposit/withdrawal wait for actual transaction receipts. USDC is the order-entry collateral; the vault supports the original collateral tokens. Native account writes still require the user's wallet approval.

Some SDK protocol features have no matching configured paxeer-network service. Those operations return an explicit capability error, including exchange API keys, subaccounts, transfers/conversions, hidden and IOC/FOK/post-only instructions, TP/SL placement and editing through the conditional-order API, and SDK per-symbol margin-mode writes. These are not simulated. An SDK page being present does not imply its exchange-specific backend feature is supported.

On `https://app.hyperpax.xyz`, the browser calls `https://data.feed.chainflowtrading.com/api/*` and `wss://data.feed.chainflowtrading.com/ws/*` directly. The gateway authorizes this production origin, so REST requests carry no API key and sockets do not request or attach a token. Prices, markets, candle history, orderbooks and trades all use this path. `NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS` can override the comma-separated list of approved origins; it defaults to this production origin.

Localhost and other origins use `packages/chainflow` through same-origin `/api/price-feed/*` routes. Configure `PRICE_FEED_REST_API_KEY` and `PRICE_FEED_WS_API_KEY` in `apps/web/.env.local` or that preview's server environment. These credentials stay server-side. Preview sockets obtain a fresh opaque credential per handshake using the SDK's exact `{ symbol }` request. The gateway owns signing; `PRICE_FEED_WS_TOKEN_SECRET` is never sent to browsers. `PRICE_FEED_ORIGIN` optionally selects the authenticated upstream gateway.

For authenticated previews, the token issuer permits per-symbol socket credentials and rejects the aggregate `markets` scope. Market-list snapshots and totals therefore use the SDK REST operations on a shared five-second poll that pauses when the page is hidden or offline. Books, trades and live candles use authenticated per-symbol WebSockets. All data comes from the configured services.

`pnpm dev` and `pnpm build` build the local Chainflow client before running the web app. The API adapter translates ticker fields and funding durations into the native UI's data contracts. HTTP methods, feed paths, symbols and history resolutions are validated; upstream authentication failures return a generic error without forwarding credentials.

Trade-history requests return arrays, while live trade subscriptions deliver individual records with their source timestamps and contract quantities. History and live subscriptions are released together when the selected market changes. Native orderbook messages identify complete snapshots so the component controller replaces the book rather than merging them as incremental updates.

Funded wallet transactions have not been executed by the implementation agent. Unit tests cover transaction planning; browser checks cover component behavior and navigation.

## Branding and deployment

The public site name is **Paxeer Xtrades**. Internal package ownership remains `@paxeer-network/*`. The supplied [Paxeer mark](https://www.paxeer.network/paxeer.x_darkmode_white.png) is stored at `apps/web/public/brand/paxeer-x.png` and used for the site, favicon and PAX market. The supplied [Sidiora mark](https://www.sidioralabs.com/sidiora_labs_mark_lightning.png) is stored alongside it and used for SID. Site accent tokens are white, with neutral hover and selected surfaces. Trading profit/loss colors retain their meaning.

The workspace-root `vercel.json` selects the root package as the deployment entrypoint and sets the builder root to `apps/web`. This serves pages, API functions and static assets at the domain root, without an `/apps/web` URL prefix or a catch-all rewrite. Import `paxeer-network` as the Vercel project root. Its app build first compiles the local Chainflow workspace client, then writes `.next-production`. The root configuration uses Vercel's supported explicit builder format so the workspace root can remain separate from the Next.js app.

The approved production origin uses direct REST and WSS without deployment credentials. Other deployment origins need the preview server credentials described above, unless the gateway authorizes them and they are explicitly added to `NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS`. Secrets are absent from deployment JSON and committed files.
