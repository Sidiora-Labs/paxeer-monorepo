# Development

Run app commands from `apps/wallet`. The SDK preparation commands below run
from `services/wallet`.

## Setup

- Node 22.22.0 (`.nvmrc`) and pnpm 10.34.9 (`package.json`).
- The app links the wallet SDK from `../../sdk/wallet` (`"@paxeer/wallet": "link:../../sdk/wallet"`); build it first from `services/wallet` with `pnpm install --frozen-lockfile --filter ../../sdk/wallet...` and `pnpm --filter ../../sdk/wallet... run build`.

```sh
pnpm install --frozen-lockfile
pnpm dev                       # port 3099, /wallet
```

Set the build-time variables listed in the [app README](../../../../apps/wallet/README.md#configuration) in the environment, or in a local `.env.local`, which git ignores.

## Checks

```sh
pnpm lint                      # eslint .
pnpm run type-check            # tsc --noEmit
pnpm test                      # vitest run
pnpm run test:wallet-app       # Playwright, playwright.wallet-app.config.ts
pnpm run check:swap-abis       # swap ABI index drift
scripts/scan-secrets.sh        # committed secrets and local artefacts
```

## Build

```sh
pnpm build                     # scripts/build-pwa.mjs, then next build --webpack
pnpm start                     # next start
```

Build output (`.next`, `public/sw.js`, `tsconfig.tsbuildinfo`) is ignored by git.
