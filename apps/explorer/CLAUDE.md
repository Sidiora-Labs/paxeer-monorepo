# Explorer source guide

The explorer uses Next.js 15 Pages Router, React 19, Chakra UI 3, React Query 5,
Wagmi 2, Viem 2 and Valibot. [package.json](package.json) declares Node
`>=22.14.0`, npm `>=10.9.0` and Yarn 1.22.22.

## Commands

Run these scripts from `apps/explorer`:

| Command | Behavior |
| --- | --- |
| `yarn dev` | Starts the development server through `tools/scripts/dev.sh` |
| `yarn build` | Runs `next build` |
| `yarn build:next` | Downloads assets, builds the SVG sprite, creates the environment script, then runs `next build` |
| `yarn lint:eslint` | Runs ESLint |
| `yarn lint:eslint:fix` | Runs ESLint with automatic fixes |
| `yarn lint:tsc` | Checks the application TypeScript configuration |
| `yarn lint:cspell` | Checks spelling |
| `yarn lint:envs-validator:test` | Runs the environment validator tests |
| `yarn test:vitest` | Starts Vitest in watch mode |
| `yarn test:vitest:ci` | Runs the unit suite once |
| `yarn test:pw` | Runs the Playwright wrapper |
| `yarn test:pw:docker` | Runs Playwright in its declared container image |
| `yarn svg:build-sprite` | Builds the SVG sprite |
| `yarn chakra:typegen` | Generates Chakra types from `toolkit/theme/theme.ts` |

## Application structure

- [pages](pages/) contains Next.js page components.
- [ui](ui/) contains feature UI components.
- [lib](lib/) contains API utilities, hooks and context providers.
- [toolkit](toolkit/) contains Chakra wrappers, theme tokens and reusable hooks.
- [configs/app](configs/app/) supplies features, API settings and UI configuration.
- [nextjs](nextjs/) contains configuration helpers and route utilities.
- [mocks](mocks/) contains test data.
- [deploy/tools/envs-validator](deploy/tools/envs-validator/) contains environment validation.

[pages/_app.tsx](pages/_app.tsx) mounts `AppContextProvider`,
`SettingsContextProvider` and `SocketProvider`. Pages use React Query for server
state; the socket provider supplies real-time updates. The route declarations
are in [nextjs/nextjs-routes.d.ts](nextjs/nextjs-routes.d.ts).

Theme colors are defined in
[semanticTokens.ts](toolkit/theme/foundations/semanticTokens.ts) and
[colors.ts](toolkit/theme/foundations/colors.ts). Component wrappers live in
[toolkit/chakra](toolkit/chakra/); lint configuration is in
[eslint.config.mjs](eslint.config.mjs). Browser globals are declared in
[global.d.ts](global.d.ts), and module declarations in [decs.d.ts](decs.d.ts).

## Configuration and test discovery

The [environment reference](../../docs/explorer/ENVS.md) documents public
configuration. [configs/envs](configs/envs/) holds deployment presets;
[download_assets.sh](deploy/scripts/download_assets.sh) lists asset inputs.
CSP policies live in [nextjs/csp/policies](nextjs/csp/policies/).

[Vitest](vitest.config.ts) discovers `apps/explorer/**/*.spec.ts` and
`apps/explorer/**/*.spec.tsx` from the repository root. From this application,
`yarn test:vitest path/to/file.spec.ts` selects a file in watch mode.

[Playwright](playwright-ct.config.ts) discovers `*.pw.tsx`. Its `default`
project uses desktop Chrome at 1200×750; `mobile` uses iPhone 13 Pro and names
containing `+@mobile`; `dark-color-mode` selects `+@dark-mode`. Names containing
`-@default` are excluded from the default project.
