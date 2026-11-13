# Perps application variables

The perps trading application lives in `apps/perps`. It serves the Next.js interface for Paxeer mainnet-beta, chain ID 125, and uses the configured data gateway for application data and node-root JSON-RPC for chain reads. This page inventories application, container and launcher inputs; all source paths below are relative to `apps/perps`.

`NEXT_PUBLIC_*` settings are client-visible build inputs. Rebuild the browser bundle to change them; recreate the server to apply server-only environment changes. The required table identifies the inputs needed by the Compose deployment; feature-specific requirements are stated in the optional table. Defaults that name hosts, precompiles or images are described by source name rather than reproduced here.

## Required

| Name | What it sets | Source default | Secret | Reader / configuration location |
| --- | --- | --- | --- | --- |
| `BUILD_COMMIT_SHA` | Compose image tag and both public and server build identity; required for Compose. | None | No | `deploy/docker-compose.yml: services.app.image, services.app.build.args.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA, services.app.environment.VERCEL_GIT_COMMIT_SHA` |
| `MARKET_PROXY_ORIGIN` | Bare HTTP(S) Origin header for candle upstream requests, with no trailing slash; required for Compose and candle serving. | None; candle route returns 503 when unset | No | `src/pages/api/candle/[...path].ts`; `src/instrumentation.ts`; `deploy/docker-compose.yml: services.app.environment.MARKET_PROXY_ORIGIN` |
| `NEXT_PUBLIC_PERPS_GATEWAY_URL` | Required bare HTTP(S) data-gateway origin, without credentials, path, query or fragment; derives REST, GraphQL and WebSocket URLs. | None; validated on import | No | `src/core/network/gateway.ts`; `deploy/docker-compose.yml: services.app.build.args.NEXT_PUBLIC_PERPS_GATEWAY_URL` |

## Optional

| Name | What it sets | Source default | Secret | Reader / configuration location |
| --- | --- | --- | --- | --- |
| `CANDLE_TRUSTED_PROXY_IPS` | Comma-separated IPs of socket peers allowed to supply X-Real-IP to candle rate limiting. | Empty trusted-peer set | No | `src/pages/api/candle/[...path].ts` |
| `CHARTING_IMAGE` | Docker build argument for the charting assets image; requires a digest pin. | Digest-pinned charting image declared in Dockerfile | No | `Dockerfile: global ARG and deps ARG` |
| `HOSTNAME` | Standalone Next.js server bind interface. | All interfaces in Docker and Compose | No | `Dockerfile: runner ENV`; `deploy/docker-compose.yml: services.app.environment.HOSTNAME` |
| `MARKET_PROXY_HOST` | Server candle-data upstream hostname. | Built-in public market-feed host | No | `src/pages/api/candle/[...path].ts`; `deploy/docker-compose.yml: services.app.environment.MARKET_PROXY_HOST` |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL` | Embedded-wallet OAuth callback URL. | Current browser origin plus /auth/callback | No | `src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE` | Launchpad precompile override; accepts a 20-byte hex address. | Release launchpad precompile in PRECOMPILES.launchpad | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_LOG_LEVEL` | Minimum logger level: debug, info, warn or error (case insensitive). | warn in production; info otherwise | No | `src/core/log/logger.ts` |
| `NEXT_PUBLIC_MARKET_PROXY_HOST` | Browser market and candle feed hostname or bare HTTP(S) origin. | Built-in public market-feed host | No | `src/core/feeds.ts` |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Preferred JSON-RPC node prepended to the built-in pool when no usable plural override is supplied. | Built-in chain-125 node pool | No | `src/core/rpc.ts` |
| `NEXT_PUBLIC_PAXEER_RPC_URLS` | Comma-separated JSON-RPC pool override; trimmed, deduplicated and filtered before use. | Singular override plus built-in pool, or built-in pool alone | No | `src/core/rpc.ts` |
| `NEXT_PUBLIC_PAXEER_WALLET_API` | Embedded-wallet API URL; required when the publishable key enables embedded sign-in. | None | No | `src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_PAXSCAN_API_URL` | Explorer HTTP API URL. | Empty string | No | `src/core/paxscan/config.ts` |
| `NEXT_PUBLIC_PAXSCAN_BASE_URL` | Bare HTTP(S) explorer origin for external links. | null; external explorer links unavailable | No | `src/core/paxscan/config.ts` |
| `NEXT_PUBLIC_PAXSCAN_WS_URL` | Explorer socket endpoint. | Empty string | No | `src/core/paxscan/config.ts` |
| `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE` | Spot market precompile override; accepts a 20-byte hex address. | Release spot market precompile in PRECOMPILES.spot.PECOR | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE` | Spot orders precompile override; accepts a 20-byte hex address. | Release spot orders precompile in PRECOMPILES.spot.PECOROrders | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE` | Spot quoter precompile override; accepts a 20-byte hex address. | Release spot quoter precompile in PRECOMPILES.spot.PECORQuoter | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE` | Spot stop-order precompile override; accepts a 20-byte hex address. | Release spot stop-order precompile in PRECOMPILES.spot.PECORStopOrders | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` | Spot vault precompile override; accepts a 20-byte hex address. | Release spot vault precompile in PRECOMPILES.spot.PECORVault | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PERPS_PRECOMPILE` | Perps precompile override; accepts a 20-byte hex address. | Release perps precompile in PRECOMPILES.perps | No | `src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_RELEASE` | Sentry release identifier for browser, server and edge events. | Unset | No | `sentry.client.config.ts`; `sentry.server.config.ts`; `sentry.edge.config.ts` |
| `NEXT_PUBLIC_RELEASE_SPOT_OPEN` | Opens the spot route, navigation and markets mode only when exactly 1. | Closed | No | `src/core/nav.ts` |
| `NEXT_PUBLIC_SENTRY_DSN` | Public Sentry event-ingestion DSN; also the fallback for server and edge capture. | Unset; capture disabled without a DSN | No | `sentry.client.config.ts`; `sentry.server.config.ts`; `sentry.edge.config.ts` |
| `NEXT_PUBLIC_SENTRY_ENV` | Browser Sentry environment label. | NODE_ENV, then production | No | `sentry.client.config.ts` |
| `NEXT_PUBLIC_SENTRY_TUNNEL` | Enables the /monitoring Sentry tunnel only when exactly 1. | Disabled | No | `next.config.js` |
| `NEXT_PUBLIC_STATUS_PAGE_URL` | Bare HTTPS status-page origin, without a trailing slash. | null; status unknown | No | `src/app/shell/Footer.tsx`; `src/instrumentation.ts` |
| `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | Public anonymous authentication key; a nonempty value enables embedded-wallet configuration. | Empty string; embedded wallet disabled | No (client-visible) | `src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_SUPABASE_URL` | Supabase project URL used for embedded-wallet authentication. | DEFAULT_SUPABASE_URL in the reader | No | `src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` | Build identifier in the HTML meta tag; Compose supplies BUILD_COMMIT_SHA. | development when undefined | No | `src/pages/_document.tsx`; `deploy/docker-compose.yml: services.app.build.args.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` |
| `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` | Trusted parent origin for the iframe postMessage wallet bridge. | Unset; bridge not installed | No | `src/core/wallet/postMessageEip1193.ts` |
| `NEXT_RUNTIME` | Framework-selected runtime; edge skips server instrumentation, nodejs or unset loads server Sentry. | Unset follows the Node.js instrumentation path | No | `src/instrumentation.ts` |
| `NEXT_TELEMETRY_DISABLED` | Disables Next.js telemetry in the container. | 1 in Docker | No | `Dockerfile: builder and runner ENV` |
| `NODE_ENV` | Runtime mode for logging, explorer warnings, secure presence cookies, PWA generation and Sentry environment fallback. | production in Docker, Compose and PM2 | No | `next.config.js`; `sentry.client.config.ts`; `sentry.server.config.ts`; `sentry.edge.config.ts`; `src/core/log/logger.ts`; `src/core/paxscan/config.ts`; `src/core/paxscan/socket/context.tsx`; `src/pages/api/presence.ts`; `ecosystem.config.js: apps[0].env.NODE_ENV`; `deploy/docker-compose.yml: services.app.environment.NODE_ENV` |
| `NODE_IMAGE` | Docker build argument for the Node base image; requires a digest pin. | Digest-pinned Node image declared in Dockerfile | No | `Dockerfile: global ARG and deps ARG` |
| `NODE_OPTIONS` | Node runtime options supplied by the PM2 launcher. | --max-http-header-size=32768 in PM2 | No | `ecosystem.config.js: apps[0].env.NODE_OPTIONS` |
| `PORT` | Standalone server listen port and Docker health-check port; PM2 also fixes its start argument. | 3000 in Docker, Compose and PM2 | No | `Dockerfile: runner ENV and HEALTHCHECK`; `deploy/docker-compose.yml: services.app.environment.PORT`; `ecosystem.config.js: apps[0].env.PORT and apps[0].args` |
| `PRESENCE_TRUSTED_PROXY_IPS` | Comma-separated IPs of socket peers allowed to supply X-Real-IP to presence limits. | Empty trusted-peer set | No | `src/pages/api/presence.ts` |
| `RAILWAY_GIT_COMMIT_SHA` | Highest-priority server /api/build-id identifier. | VERCEL_GIT_COMMIT_SHA, then development | No | `src/pages/api/build-id.ts` |
| `SENTRY_AUTH_TOKEN` | Build-time authentication for Sentry source-map uploads. | Unset | Yes | `next.config.js` |
| `SENTRY_DSN` | Server and edge Sentry event-ingestion DSN. | NEXT_PUBLIC_SENTRY_DSN; capture disabled if both unset | No (ingestion DSN) | `sentry.server.config.ts`; `sentry.edge.config.ts` |
| `SENTRY_ENV` | Server and edge Sentry environment label. | NODE_ENV, then production | No | `sentry.server.config.ts`; `sentry.edge.config.ts` |
| `SENTRY_ORG` | Sentry organization identifier for source-map uploads. | Unset | No | `next.config.js` |
| `SENTRY_PROJECT` | Sentry project identifier for source-map uploads. | Unset | No | `next.config.js` |
| `UV_THREADPOOL_SIZE` | libuv thread-pool size supplied by PM2. | 32 in PM2 | No | `ecosystem.config.js: apps[0].env.UV_THREADPOOL_SIZE` |
| `VERCEL_GIT_COMMIT_SHA` | Server /api/build-id identifier when the Railway identifier is absent; Compose supplies BUILD_COMMIT_SHA. | development if both server identifiers are empty | No | `src/pages/api/build-id.ts`; `deploy/docker-compose.yml: services.app.environment.VERCEL_GIT_COMMIT_SHA` |

### Environment examples and deployment inputs

`.env.example` covers the application, Sentry and launcher environment. `src/env/.env.template` covers settings read by application code and Next configuration. `deploy/.env.production.example` covers the variables interpolated by `deploy/docker-compose.yml`. Empty values preserve the source fallback or await a deployment-specific value; the application templates leave framework-selected mode and runtime overrides commented, while the production example selects production logging. The authentication token stays empty in both application examples.

Docker build arguments `NODE_IMAGE` and `CHARTING_IMAGE` use the digest-pinned defaults in `Dockerfile`; an environment file does not supply Docker build arguments automatically. `deploy/build-args.production.env` holds public build inputs. Compose maps its public arguments from `services.app.build.args`, except that it supplies `NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` from `BUILD_COMMIT_SHA`. The Docker build removes empty public arguments before running Next.js so source fallbacks apply.

Compose sets `NODE_ENV`, `PORT` and `HOSTNAME` directly and forwards only the runtime keys listed in `services.app.environment`. Additional server settings require the chosen launcher to pass them to the process. `NEXT_RUNTIME` and `NODE_ENV` are selected by Next.js; their overrides remain commented in the application examples. PM2 sets its own `env` values and fixes the Next start port in `args`.

### Deployment configuration keys

The application reads its deployment settings from the environment. Railway and PM2 additionally read the following configuration-file keys. File sections and keys identify the settings without reproducing deployment paths or account identifiers.

| File / section / key | What it sets | Checked-in value or default | Secret | Reader |
| --- | --- | --- | --- | --- |
| `ecosystem.config.js` / `apps[0]` / `args` | Next start arguments | start -p 3000 | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `autorestart` | Automatic worker restart | true | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `cwd` | Application working directory | Deployment-specific application directory | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `env.NODE_ENV` | Runtime mode | production | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `env.NODE_OPTIONS` | Node options | --max-http-header-size=32768 | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `env.PORT` | Listen port | 3000 | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `env.UV_THREADPOOL_SIZE` | libuv pool size | 32 | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `error_file` | Worker error-log destination | Configured PM2 error-log path | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `exec_mode` | Worker execution mode | cluster | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `exp_backoff_restart_delay` | Initial exponential restart backoff | 200 ms | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `instances` | Worker count | max | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `kill_timeout` | Graceful shutdown timeout | 8000 ms | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `listen_timeout` | Ready-signal timeout | 10000 ms | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `log_date_format` | Log timestamp format | YYYY-MM-DD HH:mm:ss.SSS Z | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `max_memory_restart` | Worker memory restart threshold | 3500M | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `max_restarts` | Consecutive unstable restart limit | 10 | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `merge_logs` | Shared worker logs | true | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `min_uptime` | Minimum stable uptime | 30s | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `name` | PM2 application name | frontend | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `node_args` | Node worker heap option | --max-old-space-size=3072 | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `out_file` | Worker output-log destination | Configured PM2 output-log path | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `restart_delay` | Restart delay | 4000 ms | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `script` | Next CLI entry point | node_modules/next/dist/bin/next | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `shutdown_with_message` | Message-based shutdown | true | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `source_map_support` | PM2 source-map support | false | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `time` | Log timestamps | true | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `treekill` | Worker process-tree shutdown | true | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `wait_ready` | Ready-signal startup | true | No | PM2 |
| `ecosystem.config.js` / `apps[0]` / `watch` | File-watching restart | false | No | PM2 |
| `railway.json` / `build` / `builder` | Image builder | DOCKERFILE | No | Railway |
| `railway.json` / `build` / `dockerfilePath` | Docker build definition | Dockerfile | No | Railway |
| `railway.json` / `deploy` / `healthcheckPath` | HTTP health route | /api/build-id | No | Railway |
| `railway.json` / `deploy` / `healthcheckTimeout` | Health-check startup timeout | 300 seconds | No | Railway |
| `railway.json` / `deploy` / `restartPolicyMaxRetries` | Failure restart limit | 3 | No | Railway |
| `railway.json` / `deploy` / `restartPolicyType` | Restart policy | ON_FAILURE | No | Railway |
