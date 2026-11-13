# Paxeer X Network Explorer frontend

This directory holds the Next.js and React frontend of the Paxeer X Network block
explorer, derived from Blockscout. It reads indexed chain data from the
[explorer backend](../../services/explorer/backend) and presents blocks,
transactions, accounts, contracts and the embedded LayerX kernel domain under
the `/explorer` base path. Paxeer mainnet-beta is one BFT network with EVM chain
id `125`; receipts are final when their block is committed. The presets identify
the network as Paxeer X Network, shortened to Paxeer X, and its native coin as
Paxeer (`PAX`) with 18 decimals.

## Layout

- `.cursor/` — editor rules and integration configuration.
- `.devcontainer/` — development container configuration.
- `.husky/` — Git hook scripts.
- `.scss/` — base and responsive stylesheets.
- `.vscode/` — editor settings, launch configurations and tasks.
- `configs/` — application settings, environment presets and chain configuration readers.
- `deploy/` — container entrypoint, asset preparation, environment validation and configuration generators.
- `icons/` — source SVG icons for the sprite.
- `lib/` — API utilities, hooks, context providers and socket support.
- `mocks/` — test response data.
- `nextjs/` — routing, server rendering, headers, redirects, security policies and global styles.
- `pages/` — Next.js Pages Router pages and API routes.
- `playwright/` — component-test entrypoint, fixtures and module replacements.
- `public/` — static assets, including the Paxeer X marks in [static/paxeer-x](public/static/paxeer-x).
- `stubs/` — data shapes used by loading states and tests.
- `toolkit/` — Chakra UI wrappers, theme tokens, components and reusable utilities.
- `tools/` — local build, development, test and preset-sync scripts.
- `types/` — API and client TypeScript types.
- `ui/` — page, feature and shared UI components.
- `vitest/` — unit-test setup and utilities.
- `package.json` — runtime requirements, dependencies and application commands.
- `yarn.lock` — pinned dependencies.
- `.nvmrc` — Node version requirement.
- `.env.example` — example integration variable names.
- `next.config.js` — the `/explorer` base path, standalone output and webpack configuration.
- `middleware.ts` — request preferences, account middleware and content-security headers.
- `instrumentation.ts` — server instrumentation and startup registration.
- `instrumentation.node.ts` — OpenTelemetry SDK configuration.
- `startup.node.ts` — server loading of enabled multichain and essential-dapp chain configurations.
- `vitest.config.ts` — unit-test discovery and setup.
- `playwright-ct.config.ts` — component-test projects and browser configuration.
- `tsconfig.json` and `tsconfig.jest.json` — TypeScript compiler configurations.
- `decs.d.ts`, `global.d.ts`, `next-env.d.ts` and `reset.d.ts` — module, browser and framework type declarations.
- `eslint.config.mjs` and `cspell.jsonc` — lint and spelling configuration.
- `svgo.config.js` and `svgo.config.format.js` — SVG optimization and formatting configuration.
- `.license-reportrc` — dependency license-report configuration.
- `prometheus.yml` — local Prometheus scrape configuration.
- [CLAUDE.md](CLAUDE.md) — frontend source guide.
- [LICENSE](LICENSE) — GNU General Public License v3.0.

## Build and test

Run application commands from `apps/explorer`. Use the Node requirement in
`.nvmrc` and the Yarn version declared by `package.json`:

```sh
corepack enable
yarn install --frozen-lockfile
yarn build
```

The following scripts are declared in [package.json](package.json):

| Command | Purpose |
| --- | --- |
| `yarn build` | Build the Next.js application. |
| `yarn build:next` | Download configuration assets, build the SVG sprite, generate browser environment configuration and build Next.js. |
| `yarn build:docker` | Build the local frontend image using the repository root as its Docker context. |
| `yarn dev` | Prepare assets and runtime configuration and start the development server. |
| `yarn start` | Start the production Next.js server. |
| `yarn lint:eslint` | Run ESLint. |
| `yarn lint:tsc` | Check application TypeScript types. |
| `yarn test:vitest` | Run Vitest in watch mode. |
| `yarn test:vitest:ci` | Run the unit suite once. |
| `yarn test:pw` | Run Playwright component tests. |
| `yarn lint:envs-validator:test` | Build and test the environment validator with its preset fixtures. |

The image uses the [frontend Dockerfile](../../docker/explorer-frontend/Dockerfile);
the [image workflow](../../.github/workflows/explorer-images.yml) builds and
publishes it. The [custom build guide](../../docs/explorer/CUSTOM_BUILD.md)
describes image configuration and startup. To validate a deployment preset
without building the frontend, run these commands from `apps/explorer`:

```sh
cd deploy/tools/envs-validator
yarn install --frozen-lockfile
NEXT_PUBLIC_GIT_COMMIT_SHA=$(git rev-parse --short HEAD) NEXT_PUBLIC_GIT_TAG=$(git describe --tags --always --abbrev=0) ../../scripts/collect_envs.sh ../../../../../docs/explorer/ENVS.md
yarn build
./node_modules/.bin/dotenv -e ../../../configs/envs/paxeer-x.env yarn run validate
```

## Configuration

[configs/app](configs/app) reads process environment variables on the server
and generated `public/assets/envs.js` variables in the browser. The
[environment reference](../../docs/explorer/ENVS.md) supplies the full public
variable inventory. Core names include:

| Purpose | Environment variables |
| --- | --- |
| Application | `NEXT_PUBLIC_APP_PROTOCOL`, `NEXT_PUBLIC_APP_HOST`, `NEXT_PUBLIC_APP_PORT`, `NEXT_PUBLIC_APP_ENV`, `NEXT_PUBLIC_APP_INSTANCE` |
| Backend API | `NEXT_PUBLIC_API_PROTOCOL`, `NEXT_PUBLIC_API_HOST`, `NEXT_PUBLIC_API_PORT`, `NEXT_PUBLIC_API_BASE_PATH`, `NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL` |
| Network | `NEXT_PUBLIC_NETWORK_NAME`, `NEXT_PUBLIC_NETWORK_SHORT_NAME`, `NEXT_PUBLIC_NETWORK_ID`, `NEXT_PUBLIC_NETWORK_RPC_URL`, `NEXT_PUBLIC_NETWORK_VERIFICATION_TYPE`, `NEXT_PUBLIC_IS_TESTNET` |
| Currency | `NEXT_PUBLIC_NETWORK_CURRENCY_NAME`, `NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL`, `NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS`, `NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL` |
| Branding | `NEXT_PUBLIC_NETWORK_LOGO`, `NEXT_PUBLIC_NETWORK_LOGO_DARK`, `NEXT_PUBLIC_NETWORK_ICON`, `NEXT_PUBLIC_NETWORK_ICON_DARK`, `NEXT_PUBLIC_COLOR_THEME_OVERRIDES`, `NEXT_PUBLIC_COLOR_THEME_DEFAULT` |
| Fonts and navigation | `NEXT_PUBLIC_FONT_FAMILY_HEADING`, `NEXT_PUBLIC_FONT_FAMILY_BODY`, `NEXT_PUBLIC_NAVIGATION_LAYOUT`, `NEXT_PUBLIC_OTHER_LINKS`, `NEXT_PUBLIC_FOOTER_LINKS` |
| Feature surfaces | `NEXT_PUBLIC_PAXEER_X_ENABLED`, `NEXT_PUBLIC_ASSISTANT_ENABLED`, `NEXT_PUBLIC_MARKETPLACE_ENABLED`, `NEXT_PUBLIC_AD_BANNER_PROVIDER`, `NEXT_PUBLIC_AD_TEXT_PROVIDER` |
| Build metadata | `NEXT_PUBLIC_GIT_COMMIT_SHA`, `NEXT_PUBLIC_GIT_TAG`, `NEXT_PUBLIC_ICON_SPRITE_HASH`, `BUNDLE_ANALYZER` |
| Instrumentation | `NEXT_RUNTIME`, `NEXT_OPEN_TELEMETRY_ENABLED`, `OTEL_SDK_ENABLED`, `NODE_ENV` |
| Assistant server | `EXPLORER_ASSISTANT_BASE_URL`, `EXPLORER_ASSISTANT_MODEL`, `EXPLORER_ASSISTANT_API_KEY` |
| Container preset | `ENVS_PRESET` |

The tracked presets are [paxeer-x.env](configs/envs/paxeer-x.env) for a public
deployment and [paxeer-x-dev.env](configs/envs/paxeer-x-dev.env) for development.
They declare network branding, navigation links and font resources; marketplace
and ad providers are disabled in both. The local footer data is
[configs/paxeer-x/footer.json](configs/paxeer-x/footer.json), which the public
preset references remotely.

The container entrypoint reads a preset using the
`configs/envs/.env.$ENVS_PRESET` filename. Copy a tracked preset to that dotfile
name before selecting it. The development preset's API and application host
placeholders must be replaced for the deployment. Preset loading excludes
`NEXT_PUBLIC_APP_PROTOCOL`, `NEXT_PUBLIC_APP_HOST`, `NEXT_PUBLIC_APP_PORT`,
`NEXT_PUBLIC_APP_ENV` and `NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL`; supply these
through the container environment.

[tools/scripts/dev.sh](tools/scripts/dev.sh) reads `.env.development.local`,
`.env.local`, `.env.development`, `.env` and `.env.secrets`.
The preset development script reads `configs/envs/.env.secrets` alongside its
selected preset. The container uses `.env.registry` for environment validation
and generates browser configuration from its packaged `.env` and runtime
environment.
The server configuration loaders read generated
`public/assets/multichain/config.json` and
`public/assets/essential-dapps/chains.json` when their features are enabled.
Build and test tools use `next.config.js`, `tsconfig.json`,
`eslint.config.mjs`, `vitest.config.ts` and `playwright-ct.config.ts`.

## Documentation

- [Environment variable reference](../../docs/explorer/ENVS.md)
- [Build-time environment variables](../../docs/explorer/BUILD-TIME_ENVS.md)
- [Custom image builds](../../docs/explorer/CUSTOM_BUILD.md)
- [Upstream attribution](../../docs/explorer/UPSTREAM.md)
- [Contribution guide](../../docs/explorer/CONTRIBUTING.md)
- [Code of conduct](../../docs/explorer/CODE_OF_CONDUCT.md)
