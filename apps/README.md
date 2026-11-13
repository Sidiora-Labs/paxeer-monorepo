# Paxeer applications

This directory holds the wallet, explorer, bridge, trading, launchpad, name-site,
inference-peer and Human interface applications for the Paxeer network. They use
the repository's SDKs, node APIs and application services. Paxeer mainnet-beta is
one BFT network running `paxd`, with EVM chain ID `125`; its kernel participates in
block proposal, and receipts are final when their blocks commit.

## Layout

- [bridge/](bridge/) — Next.js interface for deposits, withdrawals, attestations and transfer tracking across registered external chains.
- [explorer/](explorer/) — Blockscout-based Next.js explorer frontend backed by [services/explorer/backend](../services/explorer/backend/).
- [inference-peer/](inference-peer/) — Browser and native inference peer, with a vendored WebGPU engine, room transport and deal execution.
- [launchpad/](launchpad/) — Launchpad workspace containing the Next.js `web/` client and React Native `mobile/` client, linked to [SDK packages](../sdk/launchpad/packages/).
- [names/](names/) — Next.js shell that resolves `.pax` sites and verifies their storage proofs before serving content.
- [perps/](perps/) — Next.js trading interface with perpetual and spot market views, wallet integration and market data transport.
- [wallet/](wallet/) — Next.js wallet and installable PWA with account views, transfers and network surfaces.
- [web/](web/) — Human interface for accounts, agents, approvals, activity and receipt-backed journeys, including its UI library and copy resources.
- [README.md](README.md) — Directory overview and application entry points.

## Build and test

Each application has its own package manifest and lockfile. Run commands from the
directory in the first column, using the package manager declared by its manifest.
There is no package manifest or Makefile directly under `apps`.

| Directory and manifest | Build | Test |
| --- | --- | --- |
| [bridge/package.json](bridge/package.json) | `pnpm run build` | `pnpm run test` |
| [explorer/package.json](explorer/package.json) | `yarn build`; `yarn build:next` also prepares assets, the SVG sprite and runtime environment script | `yarn test:vitest:ci`; `yarn test:pw` for Playwright component tests |
| [inference-peer/package.json](inference-peer/package.json) | No build step; `npm run serve` serves the browser client | `npm test` |
| [launchpad/package.json](launchpad/package.json) | `pnpm run build:packages`, then `pnpm run build` for the web client | `pnpm --filter @sidiora/web test`; `pnpm --filter @sidiora/mobile test` |
| [launchpad/mobile/package.json](launchpad/mobile/package.json) | `pnpm run android` or `pnpm run ios` builds and runs the native client | `pnpm run test:vitest` for its Vitest suite |
| [names/package.json](names/package.json) | `pnpm run build` | No test script is declared; shell tests are in [names/e2e/shell.test.ts](names/e2e/shell.test.ts) |
| [perps/package.json](perps/package.json) | `pnpm run build` | `pnpm run test`; `pnpm run check:transport` checks transport usage |
| [wallet/package.json](wallet/package.json) | `pnpm run build` builds PWA assets and the Next.js app | `pnpm run test`; `pnpm run test:wallet-app` for Playwright |
| [web/package.json](web/package.json) | `npm run build` builds copy resources, the UI library and the Next.js app | `npm test`; `npm run test:e2e` for Playwright |

The launchpad's [pnpm-workspace.yaml](launchpad/pnpm-workspace.yaml) includes its
web and mobile clients and the SDK packages. Keep those relative paths when
installing. The wallet also links [sdk/wallet](../sdk/wallet/), and the names
package's `prepare` script compiles [sdk/names](../sdk/names/).

## Configuration

Configuration belongs to each application. Next.js embeds `NEXT_PUBLIC_*`
variables into browser builds. The following entry points describe service and
network inputs; variable names are listed without deployment values.

- **Bridge:** [src/lib/bridge/config.ts](bridge/src/lib/bridge/config.ts) reads
  `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_PAXEER_EXPLORER_URL`,
  `NEXT_PUBLIC_BRIDGE_API_URL`, `NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` and
  `NEXT_PUBLIC_BRIDGE_RPC_SOLANA`. It also reads `NEXT_PUBLIC_BRIDGE_VAULT_*` and
  `NEXT_PUBLIC_BRIDGE_RPC_*` for `ETHEREUM`, `BASE`, `ARBITRUM`, `OPTIMISM`, `BNB`,
  `POLYGON`, `AVALANCHE` and `HYPEREVM`. Framework and test configuration live in
  [next.config.ts](bridge/next.config.ts) and [vitest.config.ts](bridge/vitest.config.ts).
- **Explorer:** [configs/app/](explorer/configs/app/) reads the API, application,
  network and feature environment settings, including `NEXT_PUBLIC_API_HOST`,
  `NEXT_PUBLIC_APP_HOST` and `NEXT_PUBLIC_NETWORK_ID`. Container preset selection
  uses `ENVS_PRESET` and files under [configs/envs/](explorer/configs/envs/).
  See [.env.example](explorer/.env.example), [next.config.js](explorer/next.config.js),
  the [runtime environment reference](../docs/explorer/ENVS.md) and the
  [build environment reference](../docs/explorer/BUILD-TIME_ENVS.md).
- **Inference peer:** [scripts/serve.mjs](inference-peer/scripts/serve.mjs) reads
  `PORT`; [src/cli.mjs](inference-peer/src/cli.mjs) reads `PAXAI_ASK_BEARER`.
  CLI inputs include `--manifest`, `--deal`, `--host-key` and
  `--model-manifest-url`; the [peer README](inference-peer/README.md) describes
  manifest and enrolment documents.
- **Launchpad:** [backend-origins.ts](launchpad/web/src/core/backend-origins.ts)
  reads `NEXT_PUBLIC_DATA_API_URL`, `NEXT_PUBLIC_GATEWAY_URL`,
  `NEXT_PUBLIC_METADATA_API_URL`, `NEXT_PUBLIC_PNL_API_URL`,
  `NEXT_PUBLIC_STATS_API_URL`, `NEXT_PUBLIC_LIVESTREAM_API_URL`,
  `NEXT_PUBLIC_STREAM_ORIGIN`, `NEXT_PUBLIC_PRIVATE_API_ORIGIN`,
  `NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN`, `NEXT_PUBLIC_EXPLORER_API`,
  `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`,
  `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID` and
  `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN`. [rpc.ts](launchpad/web/src/core/rpc.ts) reads
  `NEXT_PUBLIC_RPC_URL`, `NEXT_PUBLIC_RPC_URL_FALLBACK` and
  `NEXT_PUBLIC_PAXEER_RPC_URL`. [next.config.ts](launchpad/web/next.config.ts) and
  [next-build-config.ts](launchpad/web/src/core/next-build-config.ts) configure
  rewrites with `WEB_GATEWAY_UPSTREAM`, `WEB_API_UPSTREAM`, `WEB_SOCKETS_UPSTREAM`
  and `WEB_CHARTS_UPSTREAM`.
- **Names:** [the site route](names/src/app/_pns/[label]/[[...path]]/route.ts)
  reads `NAMES_RPC_URL`, `NAMES_RESOLVER`, `NAMES_CONSENSUS_RPC_URL`,
  `NAMES_BUCKET_BASE_URL` and `NAMES_SHELL_DOMAIN`.
  [next.config.ts](names/next.config.ts) configures the shell framework.
- **Perps:** [gateway.ts](perps/src/core/network/gateway.ts) reads
  `NEXT_PUBLIC_PERPS_GATEWAY_URL`; [rpc.ts](perps/src/core/rpc.ts) reads
  `NEXT_PUBLIC_PAXEER_RPC_URLS` and `NEXT_PUBLIC_PAXEER_RPC_URL`.
  [The deployment guide](perps/deploy/README.md) documents the remaining inputs,
  including market proxy, wallet and error-reporting configuration. Tracked
  configuration includes [next.config.js](perps/next.config.js),
  [railway.json](perps/railway.json),
  [deploy/.env.production.example](perps/deploy/.env.production.example) and
  [deploy/build-args.production.env](perps/deploy/build-args.production.env).
- **Wallet:** browser and server inputs include `NEXT_PUBLIC_PAXEER_WALLET_API`,
  `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_SUPABASE_URL`,
  `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, `NEXT_PUBLIC_AUTH_REDIRECT_URL`,
  `PNS_API_ORIGIN`, `BLOCKSCOUT_UPSTREAM_BASE`, `PORTFOLIO_UPSTREAM_BASE`,
  `PUSH_DATA_DIR` and `RATE_LIMIT_STORE_PATH`. See the
  [configuration reference](wallet/README.md#configuration) and the
  [read proxy](wallet/src/app/api/wallet/[...path]/route.ts).
  [next.config.mjs](wallet/next.config.mjs) configures the application, and
  [deployment/nginx.conf](wallet/deployment/nginx.conf) configures its edge proxy.
- **Human web:** [src/api/](web/src/api/) reads `LAYERX_HUMAN_SERVICE_URL`,
  `LAYERX_HUMAN_WEB_ORIGIN`, `LAYERX_NODE_ENDPOINT` and `LAYERX_GATEWAY_URL`.
  Explorer reads use `PAXEER_X_EXPLORER_BASE_URL`; browser explorer links use
  `NEXT_PUBLIC_PAXEER_X_EXPLORER_BASE_URL`. Gas station inputs are
  `PAXEER_GAS_STATION_QUOTE_URL`, `PAXEER_GAS_STATION_CHAIN_ID`,
  `PAXEER_GAS_STATION_SPONSOR`, `PAXEER_GAS_STATION_PAYMASTER` and
  `PAXEER_GAS_STATION_SUBMIT_URL`. Performance and support storage use
  `LAYERX_RUM_STORAGE_DIRECTORY`, `LAYERX_RUM_DEVELOPMENT_FALLBACK`,
  `LAYERX_SUPPORT_REPORT_DIR`, `LAYERX_SUPPORT_REPORT_RETENTION` and
  `LAYERX_SUPPORT_REPORT_BEARER_TOKEN`. [next.config.mjs](web/next.config.mjs)
  configures assets and Human API rewrites.

## Documentation

- [Monorepo layout](../docs/MONOREPO.md) and [network overview](../docs/paxeer-network.md).
- [Wallet application](../docs/wallet/app/README.md), [architecture](../docs/wallet/app/architecture/README.md), [development](../docs/wallet/app/development/README.md) and [deployment](../docs/wallet/app/deployment/README.md).
- [Explorer configuration](../docs/explorer/ENVS.md) and [custom builds](../docs/explorer/CUSTOM_BUILD.md).
- [Inference market](../docs/agent/inference-market/README.md) and [peer client](../docs/agent/inference-market/client.md).
- [Launchpad chain launch](../docs/launchpad/chain-launch.md) and [production checklist](../docs/launchpad/production-checklist.md).
- [Perps data API](../docs/perps/data-api.md) and [Human API reference](../docs/platform/content/reference/human-api.md).
- Documentation site: [Human interface](../docs/site/paxeer-docs/public/markdown/human-interface.md), [bridge](../docs/site/paxeer-docs/public/markdown/bridge.md), [launchpad](../docs/site/paxeer-docs/public/markdown/launchpad.md), [perpetuals](../docs/site/paxeer-docs/public/markdown/perpetuals.md) and [public RPC](../docs/site/paxeer-docs/public/markdown/public-rpc.md).
