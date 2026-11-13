# Sidiora launchpad applications

This directory holds Sidiora's Next.js web application and React Native mobile application for the Paxeer network. They provide market discovery, token creation and trading, portfolios, watchlists and account views; the web application also provides market-bound chain launch. The applications use the shared [launchpad SDK packages](../../sdk/launchpad/packages) and [launchpad services](../../services/launchpad), with wallet transactions submitted to the network. Paxeer mainnet-beta is one BFT network with EVM chain ID 125; sub-chains are tenant domains within that network.

## Layout

- [web/](web/) — Next.js routes, application components, wallet integration, static assets and web tests.
- [mobile/](mobile/) — React Native screens, native session and wallet integration, Android and iOS projects, and mobile tests.
- [patches/](patches/) — The Metro patch used to read image dimensions from file contents.
- [qualification/](qualification/) — Accessibility, responsive layout, browser and native application test harnesses.
- [package.json](package.json) — Workspace scripts, dependency overrides and the Metro patch declaration.
- [pnpm-workspace.yaml](pnpm-workspace.yaml) — Includes the web and mobile applications and shared packages under `sdk/launchpad/packages`.
- [pnpm-lock.yaml](pnpm-lock.yaml) — Workspace dependency lockfile.
- [.gitignore](.gitignore) — Excludes local environment files and application output.
- [.dockerignore](.dockerignore) — Selects files for container build contexts.

## Build and test

Use pnpm 9.15.0, as declared in [package.json](package.json), with the workspace dependencies installed. Run the following commands from `apps/launchpad`:

| Command | Purpose |
| --- | --- |
| `pnpm build:packages` | Build the shared launchpad SDK packages. |
| `pnpm build` | Build the Next.js web application. |
| `pnpm --filter @sidiora/web typecheck` | Check web TypeScript types. |
| `pnpm --filter @sidiora/web test` | Prepare the test services and chain inputs, then run the web Vitest suite. |
| `pnpm --filter @sidiora/mobile test` | Run the mobile Node.js tests. |
| `pnpm --filter @sidiora/mobile test:vitest` | Run the mobile Vitest suite. |
| `pnpm dev` | Start the web development server with Turbopack. |
| `pnpm --filter @sidiora/web start` | Serve a built web application. |
| `pnpm --filter @sidiora/mobile android` | Build and run the Android application. |
| `pnpm --filter @sidiora/mobile ios` | Build and run the iOS application. |

These commands come from the workspace, [web](web/package.json) and [mobile](mobile/package.json) manifests. Build the shared packages before building the applications. The web test script invokes [prepare-test-environment.py](web/scripts/prepare-test-environment.py) before Vitest; it prepares TLS, chain deployments and joined backend services and requires their toolchains and dependencies. The [native architecture guide](mobile/ARCHITECTURE.md) describes Android and iOS toolchains, Gradle and Xcode commands, and release signing inputs.

## Configuration

The web application reads environment variables through its [backend resolver](web/src/core/backend-origins.ts), [RPC resolver](web/src/core/rpc.ts), [SDK configuration](web/src/core/sdk-config.ts) and [Next.js configuration](web/next.config.ts).

| Area | Environment variables |
| --- | --- |
| Required web backends | `NEXT_PUBLIC_DATA_API_URL`, `NEXT_PUBLIC_GATEWAY_URL`, `NEXT_PUBLIC_METADATA_API_URL`, `NEXT_PUBLIC_PNL_API_URL` |
| Optional backends and streams | `NEXT_PUBLIC_STATS_API_URL`, `NEXT_PUBLIC_LIVESTREAM_API_URL`, `NEXT_PUBLIC_STREAM_ORIGIN`, `NEXT_PUBLIC_PRIVATE_API_ORIGIN`, `NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN` |
| Node RPC and explorer | `NEXT_PUBLIC_RPC_URL`, `NEXT_PUBLIC_RPC_URL_FALLBACK`, `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_EXPLORER_API` |
| Server-side build upstreams | `WEB_GATEWAY_UPSTREAM`, `WEB_API_UPSTREAM`, `WEB_SOCKETS_UPSTREAM`, `WEB_CHARTS_UPSTREAM` |
| Wallet and sign-in | `NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_AUTH_REDIRECT_URL` |
| Application identity and social links | `NEXT_PUBLIC_CANONICAL_ORIGIN`, `NEXT_PUBLIC_ACTION_ENVIRONMENT`, `NEXT_PUBLIC_SOCIAL_WRITE_PREFIX`, `NEXT_PUBLIC_SOCIAL_WS_PATH`, `NEXT_PUBLIC_MARKET_SOCIAL_ORIGINS` |
| Analytics and error reporting | `NEXT_PUBLIC_ANALYTICS_KEY`, `NEXT_PUBLIC_SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_ENV`, `NEXT_PUBLIC_SENTRY_RELEASE`, `NEXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE`, `NEXT_PUBLIC_SENTRY_REPLAY_ERROR_SAMPLE_RATE` |
| Build and runtime controls | `ANALYZE`, `NEXT_DIST_DIR`, `NEXT_DEV_ALLOWED_ORIGINS`, `NODE_ENV`, `NEXT_RUNTIME`, `VERCEL`, `CI`, `SENTRY_ORG`, `SENTRY_PROJECT`, `SENTRY_AUTH_TOKEN` |

The backend resolver requires the web backend variables in development and production. Production backend URLs require secure schemes, and data, gateway and metadata SDK URLs must be origins without path prefixes. The [rewrite configuration](web/src/core/next-build-config.ts) requires all server-side upstreams for production builds. Public environment variables are included in the browser bundle.

Web tool configuration also lives in [tsconfig.json](web/tsconfig.json), [vitest.config.ts](web/vitest.config.ts), [eslint.config.mjs](web/eslint.config.mjs) and [postcss.config.mjs](web/postcss.config.mjs). Sentry initialization lives in [instrumentation-client.ts](web/instrumentation-client.ts), [sentry.server.config.ts](web/sentry.server.config.ts) and [sentry.edge.config.ts](web/sentry.edge.config.ts).

The mobile application loads public service configuration through its native bridge and validates it in [service-client.ts](mobile/src/runtime/service-client.ts). `SIDIORA_PUBLIC_CONFIGURATION` supplies the JSON embedded through [AndroidManifest.xml](mobile/android/app/src/main/AndroidManifest.xml) and [Info.plist](mobile/ios/Sidiora/Info.plist). It includes service origins, `canonicalOrigin`, `identityURI`, `manifestURL`, `manifestDigest`, `optionalFeatures`, `approvedPlaybackOrigins` and optional `walletConnect` and `firebasePush` settings.

| Native build inputs | Environment variables or Xcode settings |
| --- | --- |
| Application configuration and associated links | `SIDIORA_PUBLIC_CONFIGURATION`, `SIDIORA_ASSOCIATED_HOST` |
| Android release signing | `SIDIORA_ANDROID_KEYSTORE`, `SIDIORA_ANDROID_STORE_PASSWORD`, `SIDIORA_ANDROID_KEY_ALIAS`, `SIDIORA_ANDROID_KEY_PASSWORD`, `SIDIORA_SIGNING_OWNER` |
| iOS signing and push | `SIDIORA_IOS_TEAM`, `SIDIORA_IOS_SIGNING_IDENTITY`, `SIDIORA_IOS_PROVISIONING_PROFILE`, `SIDIORA_APNS_ENVIRONMENT` |

Native build configuration lives in [android/app/build.gradle](mobile/android/app/build.gradle), [ios/Podfile](mobile/ios/Podfile), the [Xcode project](mobile/ios/Sidiora.xcodeproj/project.pbxproj) and [Sidiora.entitlements](mobile/ios/Sidiora/Sidiora.entitlements). JavaScript tooling uses [metro.config.cjs](mobile/metro.config.cjs), [babel.config.cjs](mobile/babel.config.cjs), [tsconfig.json](mobile/tsconfig.json) and [vitest.config.ts](mobile/vitest.config.ts).

## Documentation

- [Launchpad web configuration checklist](../../docs/launchpad/production-checklist.md)
- [Launch a market and chain](../../docs/launchpad/chain-launch.md)
- [Launchpad documentation-site page](../../docs/site/paxeer-docs/src/app/launchpad/page.tsx)
- [Launchpad documentation in Markdown](../../docs/site/paxeer-docs/public/markdown/launchpad.md)
- [Native architecture and build prerequisites](mobile/ARCHITECTURE.md)
