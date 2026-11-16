# Paxeer Xtrades application variables

Paxeer Xtrades lives in `apps/perps`. The workspace root runs the Next.js
application in `apps/web`, while the shared client and native market adapters
are in `packages/`. This page inventories the settings those files read. Paths
in the reader column are relative to `apps/perps`.

`NEXT_PUBLIC_*` settings are included in the browser bundle. Server settings
are read by the API routes or the process that runs the application. Empty
optional values use the defaults in the named reader.

## Workspace and chain settings

| Name | What it sets | Default or requirement | Reader |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_PERPS_GATEWAY_URL` | Bare data-gateway origin used to derive REST, GraphQL and WebSocket paths. | Required; no path, credentials, query or fragment. | `packages/paxeer-backend/src/core/network/gateway.ts` |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Preferred node-root JSON-RPC origin. | Prepended when the plural override has no usable entries. | `packages/paxeer-backend/src/core/rpc.ts` |
| `NEXT_PUBLIC_PAXEER_RPC_URLS` | Comma-separated node-root JSON-RPC pool. | Empty uses the built-in chain-125 pool; entries are trimmed, deduplicated and validated. | `packages/paxeer-backend/src/core/rpc.ts` |
| `NEXT_PUBLIC_PERPS_PRECOMPILE` | Native perps precompile destination. | Release address `0x1015`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE` | Launchpad precompile destination. | Release address `0x1017`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE` | Native spot market destination. | Release address `0x1021`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE` | Native spot orders destination. | Release address `0x1022`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_STOP_PRECOMPILE` | Native spot stop-order destination. | Release address `0x1023`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE` | Native spot quote destination. | Release address `0x1024`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |
| `NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE` | Native spot vault destination. | Release address `0x1025`; override must be a 20-byte hex address. | `packages/paxeer-backend/src/core/network/precompiles.ts` |

## Wallet, feeds and application behavior

| Name | What it sets | Default or requirement | Reader |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_PAXEER_WALLET_API` | Embedded-wallet API origin. | Optional; needed when embedded sign-in is enabled. | `packages/paxeer-backend/src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_SUPABASE_URL` | Embedded-wallet identity-service URL. | The reader's default URL. | `packages/paxeer-backend/src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | Public identity-service key. | Empty disables embedded-wallet configuration. | `packages/paxeer-backend/src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL` | OAuth return URL. | Browser origin plus `/auth/callback`. | `packages/paxeer-backend/src/core/wallet/embedded/client.ts` |
| `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` | Trusted origin for the optional wallet postMessage bridge. | Unset leaves the bridge disabled. | `packages/paxeer-backend/src/core/wallet/postMessageEip1193.ts` |
| `NEXT_PUBLIC_PAXSCAN_API_URL` | Explorer HTTP API endpoint. | Empty string. | `packages/paxeer-backend/src/core/paxscan/config.ts` |
| `NEXT_PUBLIC_PAXSCAN_WS_URL` | Explorer socket endpoint. | Empty string. | `packages/paxeer-backend/src/core/paxscan/config.ts` |
| `NEXT_PUBLIC_PAXSCAN_BASE_URL` | Explorer origin used for external links. | Empty leaves those links unavailable. | `packages/paxeer-backend/src/core/paxscan/config.ts` |
| `NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS` | Comma-separated browser origins allowed to call the price feed directly. | Empty uses the feed proxy. | `packages/paxeer-backend/src/core/feeds.ts` |
| `PRICE_FEED_ORIGIN` | Upstream origin used by the server price-feed client. | The source default. | `apps/web/src/server/chainflow.ts` |
| `PRICE_FEED_REST_API_KEY` | Server credential for REST price-feed requests. | Required by the price-feed API route. | `apps/web/src/pages/api/price-feed/[...path].ts` |
| `PRICE_FEED_WS_API_KEY` | Server credential for price-feed socket token minting. | Required for socket token requests. | `apps/web/src/pages/api/price-feed/[...path].ts` |
| `NEXT_PUBLIC_RELEASE_SPOT_OPEN` | Spot route, navigation and market-mode switch. | Only `1` opens the switch. | `packages/paxeer-backend/src/core/nav.ts` |
| `NEXT_PUBLIC_LOG_LEVEL` | Browser logger minimum level. | `warn` in production and `info` otherwise. | `packages/paxeer-backend/src/core/log/logger.ts` |
| `NEXT_PUBLIC_RELEASE` | Release label included in error reports. | Unset. | `apps/web/src/legal/useCookieConsent.ts` |
| `NEXT_PUBLIC_SENTRY_DSN` | Client and fallback error-reporting destination. | Unset disables reporting. | `apps/web/src/legal/useCookieConsent.ts` |
| `NEXT_PUBLIC_SENTRY_ENV` | Client error-reporting environment label. | `NODE_ENV`, then `production`. | `apps/web/src/legal/useCookieConsent.ts` |

## API and process settings

| Name | What it sets | Default or requirement | Reader |
| --- | --- | --- | --- |
| `BUILD_COMMIT_SHA` | Image tag and server build identity used by the Compose configuration. | Required by the Compose input file. | `deploy/docker-compose.yml` |
| `MARKET_PROXY_ORIGIN` | Upstream origin for the candle API route. | Required for candle requests; must be a bare HTTP(S) origin. | `apps/web/src/pages/api/candle/[...path].ts` |
| `CANDLE_TRUSTED_PROXY_IPS` | Socket peers allowed to supply the candle route's forwarded client address. | Empty trusted-peer set. | `apps/web/src/pages/api/candle/[...path].ts` |
| `PRESENCE_TRUSTED_PROXY_IPS` | Socket peers allowed to supply the presence route's forwarded client address. | Empty trusted-peer set. | `apps/web/src/pages/api/presence.ts` |
| `RAILWAY_GIT_COMMIT_SHA` | Build identifier returned by `/api/build-id`. | Takes precedence over the Vercel value. | `apps/web/src/pages/api/build-id.ts` |
| `VERCEL_GIT_COMMIT_SHA` | Fallback build identifier returned by `/api/build-id`. | `development` when both identifiers are empty. | `apps/web/src/pages/api/build-id.ts` |
| `SENTRY_DSN` | Server and edge error-reporting destination. | Falls back to `NEXT_PUBLIC_SENTRY_DSN`; unset disables reporting. | `apps/web/src/legal/useCookieConsent.ts` |
| `SENTRY_ENV` | Server and edge error-reporting environment label. | Falls back to `NODE_ENV`, then `production`. | `apps/web/src/legal/useCookieConsent.ts` |
| `NODE_ENV` | Framework mode and logger, cookie and error-reporting behavior. | `production` for the production process. | `apps/web/next.config.js`, `packages/paxeer-backend/src/core/log/logger.ts` |
| `PORT` | Next.js listen and health-check port. | `3000` in the container. | `Dockerfile` |
| `HOSTNAME` | Next.js listen interface. | `0.0.0.0` in the container. | `Dockerfile` |
| `NEXT_TELEMETRY_DISABLED` | Next.js telemetry collection. | `1` in the container. | `Dockerfile` |
| `NODE_IMAGE` | Digest-pinned Node base image for the container build. | Digest-pinned default in `Dockerfile`. | `Dockerfile` |
| `CHARTING_IMAGE` | Digest-pinned charting asset image for the container build. | Digest-pinned default in `Dockerfile`. | `Dockerfile` |
| `PAXEER_NETWORK_BUILD_DIR` | Next.js output directory selector. | `.next-dev` in development and `.next-production` in production. | `apps/web/next.config.js` |

The checked-in examples are [`apps/web/.env.example`](../../../apps/perps/apps/web/.env.example),
[`deploy/.env.production.example`](../../../apps/perps/deploy/.env.production.example)
and [`deploy/build-args.production.env`](../../../apps/perps/deploy/build-args.production.env).
The client contract and native call path are documented in
[`contracts/perps-client.md`](../../../contracts/perps-client.md).
