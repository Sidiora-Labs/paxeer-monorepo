# Launchpad web configuration checklist

The launchpad web app is in [`apps/launchpad/web`](../../apps/launchpad/web).
Its [package manifest](../../apps/launchpad/web/package.json) pins Next.js
15.5.28 and declares React 19. The launchpad workspace uses pnpm 9.15.0.
The checks below cover configuration and behavior present in this application.

## Backend and wallet configuration

- [ ] Set `NEXT_PUBLIC_DATA_API_URL`, `NEXT_PUBLIC_GATEWAY_URL`,
  `NEXT_PUBLIC_METADATA_API_URL` and `NEXT_PUBLIC_PNL_API_URL` for the selected
  deployment. The [backend resolver](../../apps/launchpad/web/src/core/backend-origins.ts)
  validates HTTP(S) URLs and requires HTTPS in production. The data, gateway
  and metadata SDK values must be origins without a path prefix.
- [ ] Select the public market stream with `NEXT_PUBLIC_STREAM_ORIGIN`, or use
  the resolver's WebSocket form of the data origin. Optional
  `NEXT_PUBLIC_PRIVATE_API_ORIGIN` and `NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN`
  select private transports; absent values use the page origin.
- [ ] Set the server-side build inputs `WEB_GATEWAY_UPSTREAM`,
  `WEB_API_UPSTREAM`, `WEB_SOCKETS_UPSTREAM` and `WEB_CHARTS_UPSTREAM`.
  [`backendRewrites`](../../apps/launchpad/web/src/core/next-build-config.ts)
  requires all four in production and maps same-origin authentication,
  control, network, snapshot, stream and chart requests to their backends.
- [ ] Configure node-root RPC URLs through `NEXT_PUBLIC_RPC_URL`,
  `NEXT_PUBLIC_RPC_URL_FALLBACK` or `NEXT_PUBLIC_PAXEER_RPC_URL` when overriding
  the app's default. The [RPC resolver](../../apps/launchpad/web/src/core/rpc.ts)
  rejects credentials, query strings, fragments and path prefixes. Production
  RPC URLs require HTTPS. Verify chain ID 125 before a wallet submission.
- [ ] Set `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` only for wallet parents that may
  embed the app. It accepts comma-separated HTTP(S) origins; production values
  require HTTPS. The same values configure the wallet bridge and frame policy.
- [ ] Supply `NEXT_PUBLIC_SUPABASE_URL` and
  `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` together when enabling the embedded
  wallet. Without both, the sign-in dialog offers browser wallets. Keep private
  signing and service credentials out of `NEXT_PUBLIC_*` inputs.
- [ ] Check the root layout with the deployment configuration. Missing or
  unusable backend configuration renders the
  [configuration message](../../apps/launchpad/web/src/ui/shared/backend-configuration-error.tsx)
  instead of mounting the application providers.

## Security and assets

- [ ] Inspect the headers produced by
  [`src/middleware.ts`](../../apps/launchpad/web/src/middleware.ts).
  Matched routes receive HSTS, `X-Content-Type-Options` and `Referrer-Policy`.
  The main content security policy derives connection and frame origins from
  the configured backends and wallets; its script and style directives permit
  inline content. `/charting_library` uses its own policy, including
  `unsafe-eval`.
- [ ] Check frame embedding with the selected wallet configuration. The main
  middleware response sets `X-Frame-Options: SAMEORIGIN` when no wallet parent
  origin is configured and uses CSP `frame-ancestors` for configured parents.
- [ ] Check remote images against
  [`next.config.ts`](../../apps/launchpad/web/next.config.ts). Image patterns
  include resolved backend image origins and the configured external image
  services. Remote SVG optimization is disabled; the image response limit is
  8,388,608 bytes.
- [ ] Configure `SENTRY_ORG`, `SENTRY_PROJECT` and `SENTRY_AUTH_TOKEN` when
  uploading source maps. The Next.js Sentry wrapper disables source-map upload
  without the token and uses `/monitoring` as its ingestion tunnel.

## Market and chain launch

- [ ] Open `/create/chain` on chain ID 125 and review token details, chain name,
  size tier and fee share. The [chain adapter](../../apps/launchpad/web/src/widgets/create-chain/chain-adapters.ts)
  reads registry parameters and name availability, estimates transaction gas,
  and rechecks the wallet account and chain before sending `launchChain`.
- [ ] Retain the submitted transaction hash through confirmation. A pending
  submission is confirmed again before another launch is sent. The adapter
  checks the receipt's `ChainLaunched` event and reads the launch, registry and
  token details at the receipt's block.
- [ ] Use the returned registry number for the domain RPC path and its derived
  EVM chain ID for signing. See [Launch a market and chain](chain-launch.md)
  for the ABI, home-chain trading rules and revenue claims.

## Build and runtime commands

Run these commands from the repository root when working on this application:

```sh
pnpm --dir apps/launchpad --filter @sidiora/web typecheck
pnpm --dir apps/launchpad --filter @sidiora/web test
pnpm --dir apps/launchpad --filter @sidiora/web build
```

The test script prepares its environment before running Vitest. The development
script uses Next.js with Turbopack on port 3050:

```sh
pnpm --dir apps/launchpad --filter @sidiora/web dev
```

The build uses standalone output unless `VERCEL` is set. `NEXT_DIST_DIR`
selects an alternate build directory. Set `ANALYZE=true` for the configured
bundle analyzer when inspecting the production bundle:

```sh
ANALYZE=true pnpm --dir apps/launchpad --filter @sidiora/web build
```
