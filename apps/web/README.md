# Human web application

`@layerx/human-web` is the Next.js and React application for the Paxeer network's Human interface. It provides passkey onboarding, account balances, fund movement, agents, approvals, activity, settings and support, alongside explorer, exchange, launchpad and bridge pages. The application uses the Human API, wallet and agent SDKs, and network state and receipts; a receipt is final when its block commits.

## Layout

- `copy/` — copy catalogue, formatting helpers and generated runtime messages.
- `e2e/` — Node test suites, browser journeys, fixtures and production browser helpers.
- `node_modules/` — installed application dependencies.
- `packages/` — the shared `@layerx/ui` React component library.
- `policy/` — runtime dependency policy and its checker.
- `src/` — application routes, API clients, authentication, journeys, explorer, settings, UI and server modules.
- `test/` — additional Node tests and fixtures.
- `tools/` — dependency preparation script for the wallet, agent SDK and UI library.
- `.gitignore` — local ignore rules for the UI package build output.
- `package.json` — application dependencies, runtime requirement and commands.
- `package-lock.json` — npm dependency lockfile.
- `pnpm-lock.yaml` — pnpm dependency lockfile.
- `next.config.mjs` — standalone output, asset prefix, webpack resolution, rewrites and response headers.
- `next-env.d.ts` — Next.js TypeScript declarations.
- `tsconfig.json` — TypeScript options, source inclusion and wallet source aliases.
- `eslint.config.mjs` — TypeScript ESLint configuration.
- `postcss.config.mjs` — Tailwind PostCSS configuration.
- `playwright.config.ts` — browser suites, shell profiles and test server configuration.
- `playwright.perf.config.ts` — browser performance suite configuration.

## Build and test

Run the following commands from `apps/web`, with its dependencies installed. The package requires Node.js `>=24.0.0`. Commands are declared in [package.json](package.json).

| Command | Purpose |
| --- | --- |
| `npm run prepare:dependencies` | Install the wallet's locked dependencies and build the agent SDK, wallet and shared UI library; requires pnpm. |
| `npm run build` | Generate runtime copy, build the UI library and run the Next.js production build with webpack. |
| `npm run build:copy` | Generate runtime messages from the copy catalogue. |
| `npm run build:ui` | Build the shared UI library with tsup. |
| `npm run start` | Serve the production application with Next.js. |
| `npm test` | Run the declared Node test suites. |
| `npm run test:component` | Run the component and application Node suites. |
| `npm run test:foundation` | Run shell, accessibility and application state Node tests. |
| `npm run test:explorer` | Run the explorer Node suites. |
| `npm run typecheck` | Check TypeScript without emitting output. |
| `npm run lint` | Check generated copy, TypeScript, ESLint, dependency policy and the npm security audit. |
| `npm run test:e2e` | Run the Playwright browser suites. |
| `npm run test:journey` | Run Playwright tests tagged for journeys. |
| `npm run test:settings` | Run settings Node tests and tagged Playwright tests. |
| `npm run test:visual` | Run Playwright tests tagged for visual checks. |
| `npm run test:perf` | Run the desktop shell Playwright performance suite. |

The `test`, `typecheck` and `lint` commands run dependency preparation through their npm lifecycle hooks. Browser suites use a configured real application stack through [e2e/harness.ts](e2e/harness.ts). [e2e/run-production-browser.sh](e2e/run-production-browser.sh) prepares and builds the local production stack for its supported suites; [e2e/run-production-perf.sh](e2e/run-production-perf.sh) selects the performance suite.

## Configuration

Runtime environment variable names read by the application:

| Area | Variables |
| --- | --- |
| Human service and application origin | `LAYERX_HUMAN_SERVICE_URL`, `LAYERX_HUMAN_WEB_ORIGIN` |
| Network reads and hosted operations | `LAYERX_NODE_ENDPOINT`, `LAYERX_GATEWAY_URL` |
| Explorer service and browser links | `PAXEER_X_EXPLORER_BASE_URL`, `NEXT_PUBLIC_PAXEER_X_EXPLORER_BASE_URL` |
| Gas station | `PAXEER_GAS_STATION_QUOTE_URL`, `PAXEER_GAS_STATION_SUBMIT_URL`, `PAXEER_GAS_STATION_CHAIN_ID`, `PAXEER_GAS_STATION_SPONSOR`, `PAXEER_GAS_STATION_PAYMASTER` |
| Performance storage | `LAYERX_RUM_STORAGE_DIRECTORY`, `LAYERX_RUM_DEVELOPMENT_FALLBACK` |
| Support report storage and access | `LAYERX_SUPPORT_REPORT_DIR`, `LAYERX_SUPPORT_REPORT_RETENTION`, `LAYERX_SUPPORT_REPORT_BEARER_TOKEN` |
| Runtime mode | `NODE_ENV` |

Browser configuration reads `HUMAN_E2E_REAL_STACK`, `HUMAN_E2E_BASE_URL`, `HUMAN_E2E_LOCAL_PRODUCTION`, `HUMAN_E2E_TLS_CONFIG`, `HUMAN_E2E_BROWSER_HOME`, `HUMAN_E2E_TRACE_DIRECTORY`, `HUMAN_VISUAL_BASELINE_REVIEWED` and `CI`. The local production helper also configures Node's trust store through `NODE_EXTRA_CA_CERTS`.

Application and tool configuration lives in [next.config.mjs](next.config.mjs), [tsconfig.json](tsconfig.json), [eslint.config.mjs](eslint.config.mjs), [postcss.config.mjs](postcss.config.mjs), [playwright.config.ts](playwright.config.ts) and [playwright.perf.config.ts](playwright.perf.config.ts). The dependency checker reads [package.json](package.json), [package-lock.json](package-lock.json) and [policy/runtime-dependencies.json](policy/runtime-dependencies.json). Copy generation reads [copy/catalog.ts](copy/catalog.ts).

## Documentation

- [Human journeys and API contract](../../docs/wiki/HumanJourneys.md)
- [Human custody and receipt verification](../../docs/wiki/HumanCustody.md)
- [Hosted Human material](../../docs/wiki/HostedHuman.md)
- [Human interface documentation site page](../../docs/site/paxeer-docs/public/markdown/human-interface.md)
- [Shared UI library](packages/layerx-ui/README.md)
