# Hyperpax trading frontend

This directory holds the Next.js and React trading application for Paxeer mainnet-beta, including perpetual orders and positions, markets, portfolio analytics, explorer views, embedded-wallet integration and spot trading screens. It sends contract calls to the network's native venues through node JSON-RPC and reads REST, WebSocket and GraphQL data from the perps data gateway. Paxeer is one BFT network with EVM chain ID 125; transaction receipts are final when their block commits.

## Layout

- `deploy/` — Compose configuration, environment templates and the deployment guide.
- `public/` — Static images, fonts, chart assets and service-worker assets.
- `scripts/` — The browser HTTP transport check.
- `src/` — Pages and API handlers, application shell, trading widgets, hooks, wallet integration, network clients, types and styles.
- `tests/` — Node test suites, browser scenarios and fixtures.
- `package.json` — Dependencies, package-manager version and application scripts.
- `pnpm-lock.yaml` — Locked dependency resolutions.
- `pnpm-workspace.yaml` — Workspace membership, including the wallet SDK and wallet gateway packages.
- `.npmrc` — Package-manager settings.
- `next.config.js` — Standalone output, asset caching, security headers and Sentry build integration.
- `next-env.d.ts` — Next.js TypeScript declarations.
- `tsconfig.json` — TypeScript options and source aliases.
- `tailwind.config.js` — Theme tokens and utility styling.
- `postcss.config.js` — Tailwind and Autoprefixer plugins.
- `components.json` — Component generator settings and aliases.
- `sentry.client.config.ts` — Browser error reporting configuration.
- `sentry.server.config.ts` — Server error reporting configuration.
- `sentry.edge.config.ts` — Edge error reporting configuration.
- `Dockerfile` — Dependency, charting, application build and standalone runtime stages.
- `.dockerignore` — Docker build-context exclusions.
- `railway.json` — Docker deployment and health-check configuration.
- `ecosystem.config.js` — PM2 process configuration.
- `swagger_spot.json` — Spot API schema.
- `renovate.json` — Dependency update configuration.
- `.gitignore` — Dependency, build and local configuration exclusions.

## Build and test

Run these commands from `apps/perps`, using the pnpm version declared in `package.json`. The local workspace includes `../../sdk/wallet` and `../../services/wallet/gateway`.

| Command | Purpose |
| --- | --- |
| `pnpm install --frozen-lockfile` | Install the locked dependencies, as in the Dockerfile. |
| `pnpm dev` | Start the Next.js development server. |
| `pnpm build` | Build the Next.js application with standalone output. |
| `pnpm start` | Serve the built application. |
| `pnpm typecheck` | Check TypeScript without emitting files. |
| `pnpm test` | Run the Node test suites selected by the package script. |
| `pnpm check:transport` | Check browser source for HTTP calls outside the shared transport. |

Configure `NEXT_PUBLIC_PERPS_GATEWAY_URL` before building or starting the application. The Dockerfile obtains the charting library and datafeeds from its charting image stage. See the [deployment guide](deploy/README.md) for container build inputs and Compose commands.

## Configuration

The application reads the following environment variables. Public variables are embedded in the browser bundle during the build.

| Area | Environment variables |
| --- | --- |
| Data gateway | `NEXT_PUBLIC_PERPS_GATEWAY_URL` |
| Node JSON-RPC | `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_PAXEER_RPC_URLS` |
| Market feeds and candle proxy | `NEXT_PUBLIC_MARKET_PROXY_HOST`, `MARKET_PROXY_HOST`, `MARKET_PROXY_ORIGIN` |
| Proxy client identification | `CANDLE_TRUSTED_PROXY_IPS`, `PRESENCE_TRUSTED_PROXY_IPS` |
| Embedded wallet and authentication | `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, `NEXT_PUBLIC_AUTH_REDIRECT_URL`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` |
| Explorer | `NEXT_PUBLIC_PAXSCAN_API_URL`, `NEXT_PUBLIC_PAXSCAN_WS_URL`, `NEXT_PUBLIC_PAXSCAN_BASE_URL` |
| Perpetuals and launchpad precompiles | `NEXT_PUBLIC_PERPS_PRECOMPILE`, `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE` |
| Spot precompiles | `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE`, `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE`, `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE`, `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE`, `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` |
| Spot availability and status link | `NEXT_PUBLIC_RELEASE_SPOT_OPEN`, `NEXT_PUBLIC_STATUS_PAGE_URL` |
| Logging | `NEXT_PUBLIC_LOG_LEVEL` |
| Error reporting | `NEXT_PUBLIC_SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_ENV`, `NEXT_PUBLIC_RELEASE`, `SENTRY_DSN`, `SENTRY_ENV` |
| Sentry build integration | `SENTRY_ORG`, `SENTRY_PROJECT`, `SENTRY_AUTH_TOKEN`, `NEXT_PUBLIC_SENTRY_TUNNEL` |
| Build identity | `NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA`, `RAILWAY_GIT_COMMIT_SHA`, `VERCEL_GIT_COMMIT_SHA` |
| Next.js runtime | `NODE_ENV`, `NEXT_RUNTIME` |

`NEXT_PUBLIC_PERPS_GATEWAY_URL` must be a bare HTTP(S) origin. The gateway client derives the REST, GraphQL and WebSocket paths from it. Node JSON-RPC connections use the node root path. `NEXT_PUBLIC_RELEASE_SPOT_OPEN` controls the spot route, navigation and markets mode.

Environment templates are in [`src/env/.env.template`](src/env/.env.template) and [`deploy/.env.production.example`](deploy/.env.production.example); [`deploy/build-args.production.env`](deploy/build-args.production.env) lists public container build inputs. [`deploy/docker-compose.yml`](deploy/docker-compose.yml) maps build and runtime variables and reads `BUILD_COMMIT_SHA` for image identity. The Dockerfile also uses `NODE_IMAGE` and `CHARTING_IMAGE` build arguments and sets `NEXT_TELEMETRY_DISABLED`, `PORT` and `HOSTNAME` for the standalone server. The PM2 configuration sets `PORT`, `UV_THREADPOOL_SIZE` and `NODE_OPTIONS`.

Build and tooling configuration lives in `next.config.js`, `tsconfig.json`, `tailwind.config.js`, `postcss.config.js`, `components.json`, `.npmrc` and `pnpm-workspace.yaml`. Error reporting uses the three `sentry.*.config.ts` files. Deployment configuration lives in `Dockerfile`, `railway.json`, `ecosystem.config.js` and `deploy/docker-compose.yml`.

## Documentation

- [Frontend data API](../../docs/perps/data-api.md)
- [Perpetual futures documentation site page](../../docs/site/paxeer-docs/src/app/perpetuals/page.tsx)
- [Perpetual futures reference](../../docs/site/paxeer-docs/public/markdown/perpetuals.md)
- [Spot exchange documentation site page](../../docs/site/paxeer-docs/src/app/spot-exchange/page.tsx)
- [Deployment and environment](deploy/README.md)
- [Settings behavior](src/app/modals/SettingsModal.md)
- [Trading component integration](src/components/paxeer/README.md)
