# Paxeer X wallet app

The Next.js wallet web app (`@paxeer/wallet-app`) for Paxeer mainnet-beta, installable as a PWA and served under `/wallet`. It uses the wallet SDK in [`../../sdk/wallet`](../../sdk/wallet/README.md) (`@paxeer/wallet`) for embedded and injected wallets, and adds the unified account views under `/account` and the network surfaces under `/surfaces` (exchange, bridge, launchpad, fees, web data).

## Layout

- [`.github/`](.github/) holds application workflows and contribution templates.
- [`config/`](config/) holds environment schema and TypeScript configuration modules.
- [`deployment/`](deployment/) holds the nginx configuration, container startup script and edge trust renderer.
- [`e2e/`](e2e/) holds Playwright browser tests and their setup.
- [`public/`](public/) holds icons, splash screens and other PWA assets.
- [`scripts/`](scripts/) holds PWA builds, swap ABI generation, release checks and maintenance scripts.
- [`src/`](src/) holds Next.js routes, wallet and account providers, network surfaces, storage and server handlers.
- [`tests/`](tests/) holds wallet application wiring tests.
- [`tools/`](tools/) points to the application scripts.
- [`package.json`](package.json) declares dependencies and application commands.
- [`pnpm-lock.yaml`](pnpm-lock.yaml) records dependency resolutions.
- [`pnpm-workspace.yaml`](pnpm-workspace.yaml) configures dependency build approvals.
- [`.nvmrc`](.nvmrc) pins Node.js.
- [`.npmrc`](.npmrc) configures package installation.
- [`next.config.mjs`](next.config.mjs) configures the base path, standalone output, headers and SDK aliases.
- [`next-env.d.ts`](next-env.d.ts) supplies Next.js type references.
- [`tsconfig.json`](tsconfig.json) configures TypeScript and import aliases.
- [`vitest.config.ts`](vitest.config.ts) configures source tests and SDK aliases.
- [`playwright.wallet-app.config.ts`](playwright.wallet-app.config.ts) configures the browser suite and development server.
- [`eslint.config.mjs`](eslint.config.mjs) configures ESLint and import boundaries.
- [`.prettierrc`](.prettierrc) configures formatting.
- [`postcss.config.js`](postcss.config.js) configures CSS processing.
- [`tailwind.config.ts`](tailwind.config.ts) configures Tailwind styles.
- [`sentry.client.config.ts`](sentry.client.config.ts) configures browser error reporting.
- [`sentry.server.config.ts`](sentry.server.config.ts) configures server error reporting.
- [`sentry.edge.config.ts`](sentry.edge.config.ts) configures edge error reporting.
- [`railway.json`](railway.json) selects the container build and health check.
- [`nixpacks.toml`](nixpacks.toml) declares package installation, build and start commands.
- [`ecosystem.config.cjs`](ecosystem.config.cjs) configures the PM2 application process.
- [`.gitignore`](.gitignore) lists local files and build output excluded from version control.
- [`LICENSE`](LICENSE) contains the application license.

## Build and test

Run from this directory. Node 22.22.0 is pinned in [`.nvmrc`](.nvmrc), and
[package.json](package.json) pins pnpm 10.34.9.

- `pnpm install --frozen-lockfile` installs dependencies from `pnpm-lock.yaml`.
- `pnpm start` starts the built Next.js app.
- `pnpm dev` starts the development server on port 3099.
- `pnpm build` builds the PWA assets (`scripts/build-pwa.mjs`) and then the Next.js app.
- `pnpm run type-check` runs the TypeScript check; `pnpm lint` runs ESLint.
- `pnpm test` (`vitest run`) runs the unit suite; `pnpm run test:wallet-app` runs the Playwright suite in `playwright.wallet-app.config.ts`.
- `pnpm run generate:swap-abis` regenerates the swap ABI index; `pnpm run check:swap-abis` fails on drift.
- `pnpm run release:check` runs `scripts/release-check.sh`.
- `scripts/scan-secrets.sh [dir]` scans the app tree, or `dir`, for committed secrets and local artefacts; it prints `path:line: class` for content matches and `path: class` for file or directory matches, without the matched value. It exits 1 on a match, 0 when clean, or 2 for an invalid directory. `scripts/scan-secrets.test.sh` exercises it against generated fixtures.

The app has its own [pnpm workspace configuration](pnpm-workspace.yaml),
including dependency build approvals. The wallet SDK is linked from
`../../sdk/wallet`; retain that relative layout when installing.

The production image is built from [`docker/wallet-pwa/Dockerfile`](../../docker/wallet-pwa/Dockerfile) at the repository root; it runs the standalone Next.js server behind nginx with `deployment/start-wallet.sh` and `deployment/nginx.conf`.

## Configuration

Environment variables configure service origins and deployment overrides; source
defaults are defined in [src/lib/constants.ts](src/lib/constants.ts).
`NEXT_PUBLIC_*` values are inlined at build time. The Dockerfile exposes its
supported values through explicit `ARG` declarations; the .pax contract-address
and shell-domain inputs below are consumed by the source build.

| Variable                                                                                | Scope      | Purpose                                                                                                                                |
| --------------------------------------------------------------------------------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_PAXEER_WALLET_API`                                                         | build      | Wallet gateway base                                                                                                                    |
| `NEXT_PUBLIC_PAXEER_RPC_URL`                                                            | build      | Chain RPC base                                                                                                                         |
| `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`                      | build      | Supabase project for sign-in                                                                                                           |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL`                                                         | build      | Sign-in redirect target                                                                                                                |
| `NEXT_PUBLIC_PAXEER_HUMAN_API`                                                          | build      | Human service base used by the account views                                                                                           |
| `NEXT_PUBLIC_PAXEER_EXPLORER_URL`                                                       | build      | Explorer base used by the account views and transfer status                                                                            |
| `NEXT_PUBLIC_EXPLORER_BASE`                                                             | build      | Explorer base for transaction and address links; default in `src/lib/constants.ts`                                                     |
| `NEXT_PUBLIC_EXPLORER_STATUS_URL`                                                       | build      | Transfer status explorer endpoint, after `NEXT_PUBLIC_PAXEER_EXPLORER_URL` and before the same-origin Blockscout API in fallback order |
| `NEXT_PUBLIC_PAXEER_HOSTED_URL`                                                         | build      | Optional hosted gateway for unified account reads                                                                                      |
| `NEXT_PUBLIC_PERPS_GATEWAY_URL`                                                         | build      | Perpetuals collateral reads used by the exchange surface                                                                               |
| `NEXT_PUBLIC_PAXAI_VIEWS_URL`, `NEXT_PUBLIC_PAXAI_MARKET_ID`                            | build      | Inference reward source                                                                                                                |
| `NEXT_PUBLIC_PAXEER_ATTESTOR_URL`                                                       | build      | Attestor base                                                                                                                          |
| `NEXT_PUBLIC_PNS_API_BASE`                                                              | build      | Name service indexer base; defaults to the same-origin `/wallet/api/pns` proxy when unset                                              |
| `PNS_API_ORIGIN`                                                                        | run        | Name indexer HTTP(S) origin without path, query or trailing slash; unset or invalid returns 503 `name_service_not_configured`          |
| `NEXT_PUBLIC_PNS_CONTROLLER`                                                            | build      | Deployed .pax controller address (0x-prefixed, 20-byte hex)                                                                            |
| `NEXT_PUBLIC_PNS_PUBLIC_RESOLVER`                                                       | build      | Deployed .pax public resolver address (0x-prefixed, 20-byte hex)                                                                       |
| `NEXT_PUBLIC_PNS_REVERSE_REGISTRAR`                                                     | build      | Deployed .pax reverse registrar address (0x-prefixed, 20-byte hex)                                                                     |
| `NEXT_PUBLIC_PNS_SHELL_DOMAIN`                                                          | build      | Optional shell domain for name website links; supply a hostname without a scheme or path                                               |
| `NEXT_PUBLIC_POINTS_API_BASE`                                                           | build      | Points indexer base                                                                                                                    |
| `NEXT_PUBLIC_MARKET_DATA_API`                                                           | build      | Market data base for PAX and listed token prices; price reads fail when unset                                                          |
| `NEXT_PUBLIC_FX_RATES_API`                                                              | build      | Fiat exchange rate base; amounts stay in USD when unset                                                                                |
| `NEXT_PUBLIC_MEDIA_STORAGE_ORIGIN`                                                      | build      | Storage origin allowed as an image source by the content security policy                                                               |
| `NEXT_PUBLIC_CHAIN_ID`                                                                  | build      | EVM chain ID                                                                                                                           |
| `NEXT_PUBLIC_PAXEER_GAS_SPONSOR`, `NEXT_PUBLIC_PAXEER_GAS_PAYMASTER`                    | build      | Gas sponsor and paymaster used by the fee surface                                                                                      |
| `NEXT_PUBLIC_PAXEER_USID_PER_PAX`                                                       | build      | Positive decimal micro-SID per PAX rate for SID fee estimates; estimates are unavailable when unset or invalid                         |
| `NEXT_PUBLIC_MTX`                                                                       | build      | MTX token address override; defaults to the address in `src/lib/constants.ts`                                                          |
| `NEXT_PUBLIC_PAXIE`                                                                     | build      | PAXIE token address override; defaults to the address in `src/lib/constants.ts`                                                        |
| `NEXT_PUBLIC_FLIP`                                                                      | build      | FLIP token address override; defaults to the address in `src/lib/constants.ts`                                                         |
| `NEXT_PUBLIC_MISFITZ`                                                                   | build      | MISFITZ token address override; defaults to the address in `src/lib/constants.ts`                                                      |
| `NEXT_PUBLIC_AGGIE`                                                                     | build      | AGGIE token address override; defaults to the address in `src/lib/constants.ts`                                                        |
| `BLOCKSCOUT_UPSTREAM_BASE`                                                              | run        | Read API upstream behind the same-origin `/wallet/api/wallet` proxy                                                                    |
| `PORTFOLIO_UPSTREAM_BASE`                                                               | run        | Portfolio upstream behind the same-origin wallet proxy                                                                                 |
| `TRUSTED_EDGE_CIDRS`                                                                    | run        | Edge networks accepted for forwarded client addresses by nginx                                                                         |
| `SIDIORA_SDK_UPSTREAM`                                                                  | run        | Optional override of the `/api/sdk` upstream                                                                                           |
| `S3_ENDPOINT_URL`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION` | run        | Object store behind `/api/token-icon` and `/api/token-metadata`                                                                        |
| `OPENAI_API_KEY`                                                                        | run        | `/api/chat` credential                                                                                                                 |
| `VAPID_PRIVATE_KEY`, `NEXT_PUBLIC_VAPID_PUBLIC_KEY`, `VAPID_SUBJECT`                    | run, build | Web push keys; generate with `scripts/generate-vapid-keys.js`                                                                          |
| `PUSH_ADMIN_KEY`                                                                        | run        | Push administration routes                                                                                                             |
| `PUSH_DATA_DIR`, `RATE_LIMIT_STORE_PATH`                                                | run        | Push store directory and rate-limit store file                                                                                         |
| `TRUSTED_PROXY_SECRET`                                                                  | run        | Shared secret between the edge proxy and the app                                                                                       |
| `SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_DSN`                                                  | run, build | Error reporting; reporting is off when unset                                                                                           |
| `HEALTH_CAPTURE_DEGRADED`                                                               | run        | Controls reporting of degraded readiness checks through error reporting                                                                |
| `NODE_ENV`                                                                              | build, run | Next.js execution mode and production service worker registration                                                                      |

USDC, USDT, USDL, WPAX9 and SID addresses come from the generated swap SDK
registry in [src/lib/swap/sdk/addresses.ts](src/lib/swap/sdk/addresses.ts),
which is consumed by `src/lib/constants.ts`.

Configured service bases in `src/pwa/config.ts` must be absolute HTTP(S) URLs
without credentials. Their origins join the content security policy connect
list in `src/lib/security/csp.ts`; Supabase also adds its WebSocket origin.

The name widget shows `name service not configured` with no actions until the .pax name contracts are deployed and all three contract addresses are set and valid. Website records store a 32-byte manifest digest prefixed with the Paxeer contenthash tag `0x70617831`. Set website accepts a `0x`-prefixed 64-hex digest; configure `NEXT_PUBLIC_PNS_SHELL_DOMAIN` to enable the Website link. By default, name reads use `/wallet/api/pns` and the five indexer proxy routes under `src/app/api/pns` forward to `PNS_API_ORIGIN`. Without a valid origin they return 503 `name_service_not_configured` without contacting an upstream, and the wallet helpers return empty results.

`.env` and `.env.*` are ignored by git, together with build output, native intermediates, signing files and editor and OS artefacts; see `.gitignore`.

The application reads configuration through [src/wallet/config.ts](src/wallet/config.ts),
[src/account/config.ts](src/account/config.ts), [src/pwa/config.ts](src/pwa/config.ts),
[src/surfaces/config.ts](src/surfaces/config.ts), [src/lib/pns.ts](src/lib/pns.ts)
and [src/lib/constants.ts](src/lib/constants.ts). Next.js loads `next.config.mjs`
and environment files; TypeScript, Vitest and Playwright use the configuration files
listed in Layout. The container startup script reads `deployment/nginx.conf` and
uses `deployment/render-edge-trust.sh` to configure trusted edge networks.

`RELEASE_CHECK_BROWSERS` controls browser execution in `scripts/release-check.sh`.
`NEXT_MANUAL_SIG_HANDLE` is set by the build script and PM2 configuration.

## Documentation

- [architecture](../../docs/wallet/app/architecture/README.md): entry point, routes, providers, server edge
- [storage forensics](../../docs/wallet/app/architecture/storage-forensics.md): browser storage registry
- [API](../../docs/wallet/app/api/README.md): server routes under `src/app/api`
- [development](../../docs/wallet/app/development/README.md) and [deployment](../../docs/wallet/app/deployment/README.md)
- [operations runbooks](../../docs/wallet/app/operations/runbooks.md): recovery runbooks

- [Wallet app documentation](../../docs/wallet/app/README.md): documentation index
- [Accounts on the documentation site](../../docs/site/paxeer-docs/src/app/accounts/page.tsx): key derivation and unified accounts
- [Building a frontend on the documentation site](../../docs/site/paxeer-docs/src/app/building-a-frontend/page.tsx): browser wallet integration
