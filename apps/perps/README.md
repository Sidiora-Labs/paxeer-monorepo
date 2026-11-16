# Paxeer Xtrades

Paxeer Xtrades is the trading workspace for Paxeer's native perpetual markets.
It contains the Next.js web application, the shared trading packages, and the
Paxeer client that reads market data and prepares signed chain calls.

## Workspace layout

| Path | Contents |
| --- | --- |
| [`apps/web/`](apps/web/) | Next.js pages, API routes, application providers, native layout bindings and themes. |
| [`apps/web/public/`](apps/web/public/) | Browser assets, fonts, icons and the charting-library mount point. |
| [`packages/paxeer-backend/`](packages/paxeer-backend/) | Paxeer RPC, gateway, wallet, market, order, position and receipt clients. |
| [`packages/`](packages/) | Shared UI, layout, market, portfolio, wallet and SDK packages used by the web app. |
| [`tests/`](tests/) | Node and browser checks for the workspace. |
| [`package.json`](package.json) | Workspace scripts and the pnpm package-manager pin. |
| [`pnpm-workspace.yaml`](pnpm-workspace.yaml) | The `apps/*` and `packages/*` workspace members. |
| [`pnpm-lock.yaml`](pnpm-lock.yaml) | Locked dependency resolutions. |
| [`.npmrc`](.npmrc) | Package-manager settings. |

## Install, check, build and run

Run commands from `apps/perps`. The workspace pins pnpm in `package.json` and
uses Corepack to select it.

```sh
corepack pnpm install --frozen-lockfile
pnpm typecheck
pnpm check:ui
pnpm test
pnpm test:browser
pnpm build
pnpm start
```

`pnpm dev` starts the web application at `/trade`; `/trade/grid`, `/portfolio`
and `/markets` select the grid, account and market views. `pnpm test:browser`
expects the browser test dependencies and a running preview. Development and
production builds use separate `.next-dev` and `.next-production` directories.

## Settings

The web package reads public settings from `apps/web/.env.example`. Values
prefixed with `NEXT_PUBLIC_` are part of the browser bundle; server settings are
read only by the API routes.

| Area | Settings |
| --- | --- |
| Data gateway and node | `NEXT_PUBLIC_PERPS_GATEWAY_URL`, `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_PAXEER_RPC_URLS` |
| Native destinations | `NEXT_PUBLIC_PERPS_PRECOMPILE`, `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE`, `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE`, `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE`, `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE`, `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE`, `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` |
| Wallet and sign-in | `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, `NEXT_PUBLIC_AUTH_REDIRECT_URL`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` |
| Explorer | `NEXT_PUBLIC_PAXSCAN_API_URL`, `NEXT_PUBLIC_PAXSCAN_WS_URL`, `NEXT_PUBLIC_PAXSCAN_BASE_URL` |
| Feeds and API routes | `NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS`, `PRICE_FEED_ORIGIN`, `PRICE_FEED_REST_API_KEY`, `PRICE_FEED_WS_API_KEY`, `MARKET_PROXY_ORIGIN`, `CANDLE_TRUSTED_PROXY_IPS`, `PRESENCE_TRUSTED_PROXY_IPS` |
| Features and diagnostics | `NEXT_PUBLIC_RELEASE_SPOT_OPEN`, `NEXT_PUBLIC_LOG_LEVEL`, `NEXT_PUBLIC_RELEASE`, `NEXT_PUBLIC_SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_ENV`, `SENTRY_DSN`, `SENTRY_ENV`, `NODE_ENV` |

The readers for these settings are in
[`packages/paxeer-backend/src/core`](packages/paxeer-backend/src/core),
[`apps/web/src/pages/api`](apps/web/src/pages/api), and
[`apps/web/src/server`](apps/web/src/server). The complete variable inventory
is in [`docs/release/variables/app-perps.md`](../../docs/release/variables/app-perps.md).

## Client contract and order path

The client contract is documented in
[`contracts/perps-client.md`](../../contracts/perps-client.md). ABI types are
defined in [`packages/paxeer-backend/src/core/network/abi/PerpsPrecompile.ts`](packages/paxeer-backend/src/core/network/abi/PerpsPrecompile.ts),
and typed call descriptors are built by
[`packages/paxeer-backend/src/core/perps/calls.ts`](packages/paxeer-backend/src/core/perps/calls.ts).
The destination is the native perps precompile at `0x1015`; its handlers and
ABI are described in [`precompiles/perps/README.md`](../../precompiles/perps/README.md).

An order follows the code path from the wallet to the chain: the wallet signs a
call to the perps precompile, the LayerX kernel executes it when it arrives at
`paxd`, assigns its sequence position, orders it and issues a signed execution
receipt. The kernel builds the block from that ordered sequence. Validators
check the block and commit it, so the receipt is final when its block commits.

Market and account reads use the configured gateway and node-root JSON-RPC;
order, position and collateral writes use the typed precompile calls and wait
for the resulting chain receipt. The gateway reference is
[`docs/perps/data-api.md`](../../docs/perps/data-api.md).
