# Paxeer X wallet app

The Next.js wallet web app (`@paxeer/wallet-app`) of Paxeer X Network, installable as a PWA. It uses the wallet SDK in [`../../sdk/wallet`](../../sdk/wallet/README.md) (`@paxeer/wallet`) for embedded and injected wallets, and adds the unified account views under `/account` and the network surfaces under `/surfaces` (exchange, bridge, launchpad, fees, web data).

## Scripts

Run from this directory. Node 22.22.0 is pinned in [`.nvmrc`](.nvmrc), and
[package.json](package.json) pins pnpm 10.34.9.

- `pnpm install --frozen-lockfile` installs dependencies from `pnpm-lock.yaml`.
- `pnpm dev` starts the development server on port 3099.
- `pnpm build` builds the PWA assets (`scripts/build-pwa.mjs`) and then the Next.js app.
- `pnpm run type-check` runs the TypeScript check; `pnpm lint` runs ESLint.
- `pnpm test` (`vitest run`) runs the unit suite; `pnpm run test:wallet-app` runs the Playwright suite in `playwright.wallet-app.config.ts`.
- `pnpm run generate:swap-abis` regenerates the swap ABI index; `pnpm run check:swap-abis` fails on drift.
- `pnpm run release:check` runs `scripts/release-check.sh`.
- `scripts/scan-secrets.sh [dir]` scans the app tree, or `dir`, for committed secrets and local artefacts; it prints `path:line: class` for every hit without the matched value and exits 1 on any hit, 0 when clean. `scripts/scan-secrets.test.sh` exercises it against generated fixtures.

The app has its own [pnpm workspace configuration](pnpm-workspace.yaml),
including dependency build approvals. The wallet SDK is linked from
`../../sdk/wallet`; retain that relative layout when installing.

The production image is built from [`docker/wallet-pwa/Dockerfile`](../../docker/wallet-pwa/Dockerfile) at the repository root; it runs the standalone Next.js server behind nginx with `deployment/start-wallet.sh` and `deployment/nginx.conf`.

## Documentation

- [architecture](../../docs/wallet/app/architecture/README.md): entry point, routes, providers, server edge
- [storage forensics](../../docs/wallet/app/architecture/storage-forensics.md): browser storage registry
- [API](../../docs/wallet/app/api/README.md): server routes under `src/app/api`
- [development](../../docs/wallet/app/development/README.md) and [deployment](../../docs/wallet/app/deployment/README.md)
- [operations runbooks](../../docs/wallet/app/operations/runbooks.md): recovery runbooks

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
| `NEXT_PUBLIC_EXPLORER_BASE`                                                             | build      | Explorer base for transaction and address links; defaults to `https://paxscan.io`                                                      |
| `NEXT_PUBLIC_EXPLORER_STATUS_URL`                                                       | build      | Transfer status explorer endpoint, after `NEXT_PUBLIC_PAXEER_EXPLORER_URL` and before the same-origin Blockscout API in fallback order |
| `NEXT_PUBLIC_PAXEER_ATTESTOR_URL`                                                       | build      | Attestor base                                                                                                                          |
| `NEXT_PUBLIC_PNS_API_BASE`                                                              | build      | Name service indexer base; defaults to the same-origin `/wallet/api/pns` proxy when unset                                              |
| `PNS_API_ORIGIN`                                                                        | run        | Name indexer HTTP(S) origin without path, query or trailing slash; unset or invalid returns 503 `name_service_not_configured`          |
| `NEXT_PUBLIC_PNS_CONTROLLER`                                                            | build      | Deployed .pax controller address (0x-prefixed, 20-byte hex)                                                                            |
| `NEXT_PUBLIC_PNS_PUBLIC_RESOLVER`                                                       | build      | Deployed .pax public resolver address (0x-prefixed, 20-byte hex)                                                                       |
| `NEXT_PUBLIC_PNS_REVERSE_REGISTRAR`                                                     | build      | Deployed .pax reverse registrar address (0x-prefixed, 20-byte hex)                                                                     |
| `NEXT_PUBLIC_PNS_SHELL_DOMAIN`                                                          | build      | Optional shell hostname for website links at `https://<label>.pax.<shell-domain>/`; supply a hostname without a scheme or path         |
| `NEXT_PUBLIC_POINTS_API_BASE`                                                           | build      | Points indexer base                                                                                                                    |
| `NEXT_PUBLIC_MARKET_DATA_API`                                                           | build      | Market data base for PAX and listed token prices; price reads fail when unset                                                          |
| `NEXT_PUBLIC_FX_RATES_API`                                                              | build      | Fiat exchange rate base; amounts stay in USD when unset                                                                                |
| `NEXT_PUBLIC_MEDIA_STORAGE_ORIGIN`                                                      | build      | Storage origin allowed as an image source by the content security policy                                                               |
| `NEXT_PUBLIC_CHAIN_ID`                                                                  | build      | Chain ID, default 125                                                                                                                  |
| `NEXT_PUBLIC_PAXEER_GAS_SPONSOR`, `NEXT_PUBLIC_PAXEER_GAS_PAYMASTER`                    | build      | Gas sponsor and paymaster used by the fee surface                                                                                      |
| `NEXT_PUBLIC_PAXEER_USID_PER_PAX`                                                       | build      | Positive decimal micro-SID per PAX rate for SID fee estimates; estimates are unavailable when unset or invalid                         |
| `NEXT_PUBLIC_USDC`                                                                      | build      | USDC token address override; defaults to the address in `src/lib/constants.ts`                                                         |
| `NEXT_PUBLIC_USDT`                                                                      | build      | USDT token address override; defaults to the address in `src/lib/constants.ts`                                                         |
| `NEXT_PUBLIC_USDL`                                                                      | build      | USDL token address override; defaults to the address in `src/lib/constants.ts`                                                         |
| `NEXT_PUBLIC_WPAX9`                                                                     | build      | Wrapped PAX token address override; defaults to the address in `src/lib/constants.ts`                                                  |
| `NEXT_PUBLIC_SID`                                                                       | build      | SID token address override; defaults to the address in `src/lib/constants.ts`                                                          |
| `NEXT_PUBLIC_MTX`                                                                       | build      | MTX token address override; defaults to the address in `src/lib/constants.ts`                                                          |
| `NEXT_PUBLIC_PAXIE`                                                                     | build      | PAXIE token address override; defaults to the address in `src/lib/constants.ts`                                                        |
| `NEXT_PUBLIC_FLIP`                                                                      | build      | FLIP token address override; defaults to the address in `src/lib/constants.ts`                                                         |
| `NEXT_PUBLIC_MISFITZ`                                                                   | build      | MISFITZ token address override; defaults to the address in `src/lib/constants.ts`                                                      |
| `NEXT_PUBLIC_AGGIE`                                                                     | build      | AGGIE token address override; defaults to the address in `src/lib/constants.ts`                                                        |
| `BLOCKSCOUT_UPSTREAM_BASE`                                                              | run        | Explorer backend behind the same-origin `/api/wallet` route                                                                            |
| `SIDIORA_SDK_UPSTREAM`                                                                  | run        | Optional override of the `/api/sdk` upstream                                                                                           |
| `S3_ENDPOINT_URL`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION` | run        | Object store behind `/api/token-icon` and `/api/token-metadata`                                                                        |
| `OPENAI_API_KEY`                                                                        | run        | `/api/chat` credential                                                                                                                 |
| `VAPID_PRIVATE_KEY`, `NEXT_PUBLIC_VAPID_PUBLIC_KEY`, `VAPID_SUBJECT`                    | run, build | Web push keys; generate with `scripts/generate-vapid-keys.js`                                                                          |
| `PUSH_ADMIN_KEY`                                                                        | run        | Push administration routes                                                                                                             |
| `PUSH_DATA_DIR`, `RATE_LIMIT_STORE_PATH`                                                | run        | Push store directory and rate-limit store file                                                                                         |
| `TRUSTED_PROXY_SECRET`                                                                  | run        | Shared secret between the edge proxy and the app                                                                                       |
| `SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_DSN`                                                  | run, build | Error reporting; reporting is off when unset                                                                                           |
| `HEALTH_CAPTURE_DEGRADED`                                                               | run        | Set to `true` to report degraded readiness checks through error reporting; disabled otherwise                                          |
| `NODE_ENV`                                                                              | build, run | Next.js execution mode; the PWA provider registers the service worker only in `production`                                             |

Each configured `NEXT_PUBLIC_*` network base (`src/pwa/config.ts`) joins the content security policy connect list in `src/lib/security/csp.ts`.

The name widget shows `name service not configured` with no actions until the .pax name contracts are deployed and all three contract addresses are set and valid. Website records store a 32-byte manifest digest prefixed with the Paxeer contenthash tag `0x70617831`. Set website accepts a `0x`-prefixed 64-hex digest; configure `NEXT_PUBLIC_PNS_SHELL_DOMAIN` to enable the Website link. By default, name reads use `/wallet/api/pns` and the five indexer proxy routes under `src/app/api/pns` forward to `PNS_API_ORIGIN`. Without a valid origin they return 503 `name_service_not_configured` without contacting an upstream, and the wallet helpers return empty results.

`.env` and `.env.*` are ignored by git, together with build output, native intermediates, signing files and editor and OS artefacts; see `.gitignore`.
