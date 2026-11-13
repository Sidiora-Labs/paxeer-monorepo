# Launchpad client contracts

This page documents the launchpad web and mobile clients and their SDK boundaries. See [service contracts](services.md) for HTTP producers and [native launchpad contracts](launchpad.md) for execution, events and governance.

## Network and transaction lifecycle

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). Its kernel runs inside `paxd` and participates in block proposal. A transaction receipt is final when its block is committed. Submission and a signed pre-commit receipt must not be displayed as committed execution. Sub-chains are tenant domains in the database, not separately deployed chains.

Node JSON-RPC uses HTTPS at the configured node’s root path. See [node transport and served methods](evm-rpc.md). Node-state `px_*` methods are distinct from the hosted identity and faucet operations in [the gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).

## Launchpad web app

The Next.js app lives in [apps/launchpad/web](../apps/launchpad/web). Its [workspace package](../apps/launchpad/package.json) uses pnpm; `dev` starts the web app on port 3050, `build` builds it, and `build:packages` builds the launchpad SDK packages. The web [package scripts](../apps/launchpad/web/package.json) also expose `start`, `test` and `typecheck`. These are development commands, not evidence that a deployment is active.

### Browser service configuration

[backend-origins.ts](../apps/launchpad/web/src/core/backend-origins.ts) is the shared resolver for browser clients, CSP and remote images. The following variables are required and have no fallback host:

| Variable | Destination |
| --- | --- |
| `NEXT_PUBLIC_DATA_API_URL` | Launchpad API reads |
| `NEXT_PUBLIC_GATEWAY_URL` | Identity, account and social gateway |
| `NEXT_PUBLIC_METADATA_API_URL` | Token metadata and media |
| `NEXT_PUBLIC_PNL_API_URL` | Portfolio and PnL |

`NEXT_PUBLIC_STATS_API_URL` and `NEXT_PUBLIC_LIVESTREAM_API_URL` are optional. `NEXT_PUBLIC_STREAM_ORIGIN` selects the public WebSocket origin; absent an override it is the WebSocket form of the data API origin. Private API and stream overrides use `NEXT_PUBLIC_PRIVATE_API_ORIGIN` and `NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN`; absent overrides, those clients use the page origin. Missing or malformed required values produce a configuration error state. Production backend URLs require HTTPS/WSS and reject credentials, queries and fragments.

[rpc.ts](../apps/launchpad/web/src/core/rpc.ts) builds the deduplicated node RPC list in this order: `NEXT_PUBLIC_RPC_URL`, `NEXT_PUBLIC_RPC_URL_FALLBACK`, `NEXT_PUBLIC_PAXEER_RPC_URL`. When all three variables are unset it selects the built-in mainnet-beta root endpoint. A configured malformed value raises `RpcConfigurationError`; production requires HTTPS and a root URL without credentials, query or fragment. This RPC list is separate from the launchpad REST origins.

### Same-origin routes

[next-build-config.ts](../apps/launchpad/web/src/core/next-build-config.ts) requires four server-only upstreams in production, read when the build writes its rewrites:

| Variable | Paths forwarded by the web server |
| --- | --- |
| `WEB_GATEWAY_UPSTREAM` | `/v2/auth/*`, `/v2/stream/tickets`, `/v2/control/*` |
| `WEB_API_UPSTREAM` | `/v2/network/authority`, `/v2/preflight/*`, `/v2/snapshots`, `/v2/pools/:pool/candles` |
| `WEB_SOCKETS_UPSTREAM` | `/v2/stream`, `/v2/stream/sse` |
| `WEB_CHARTS_UPSTREAM` | `/v2/udf/config`, `/v2/udf/symbols`, `/v2/udf/history`, `/v2/udf/time` |

The public market stream connects directly to the configured public stream origin. The social chat socket uses the gateway host; it is not the API market stream. [next.config.ts](../apps/launchpad/web/next.config.ts) chooses `NEXT_DIST_DIR` or `.next`, uses standalone output outside Vercel, and derives remote image hosts from the same backend resolver. `NEXT_DEV_ALLOWED_ORIGINS` accepts a comma-separated list of hostnames without schemes, ports, paths or wildcards.

### Deployment and wallet authority

[deployment.ts](../apps/launchpad/web/src/core/network/deployment.ts) contains the generated chain-125 role addresses and expected identities; [contracts.ts](../apps/launchpad/web/src/core/network/contracts.ts) consumes them. An address in that file is recorded configuration, not a live bytecode attestation. The API's [network authority](services.md#launchpad-api) supplies the committed-head attestation and preflight contract. ERC-20 approval helpers allow only the configured active launchpad trade spender. Optional `metaAg` and `pecorOrders` capabilities are disabled in the generated configuration.

WalletConnect is enabled by `NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID`. The embedded wallet requires both `NEXT_PUBLIC_SUPABASE_URL` and `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`; `NEXT_PUBLIC_PAXEER_WALLET_API` can override its API origin. `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN` supplies the parent-origin allowlist shared with the iframe bridge. These values configure client integrations; they do not replace transaction signing or service authorization.

## Launchpad mobile app

[service-client.ts](../apps/launchpad/mobile/src/runtime/service-client.ts) validates a release `PublicConfiguration` before constructing its service client. API, identity and media origins must equal the approved HTTPS `canonicalOrigin`. The manifest URL must share that origin, and `manifestDigest` must be 64 lowercase hex characters. The configuration also carries an approved identity URI, stream origins, explicit chat/live feature flags and playback origins. Native account operations use [operation-authority.ts](../apps/launchpad/mobile/src/runtime/operation-authority.ts); a displayed account address alone does not authorize an operation.

## Launchpad SDK

The main package is [`@kindle/sdk`](../sdk/launchpad/packages/sdk/package.json). Its [transport](../sdk/launchpad/packages/sdk/src/transport.ts) exports `SdkClient`, `TransportConfig`, `HttpError`, `CapabilityUnavailableError` and `StreamTransportError`. Configure API, identity, media and approved stream origins explicitly. Production accepts HTTPS/WSS public origins; transport origins cannot include a path prefix, credentials, query or fragment. Cookie sessions require the application origin to equal the API origin; bearer sessions supply a token callback.

[`Operations.call`](../sdk/launchpad/packages/sdk/src/operations.ts) takes an operation name, typed path/query/body input and request options. It validates inputs and responses against the compiled contracts, applies [route corrections](../sdk/launchpad/packages/sdk/src/route-corrections.ts), and consults [served-route status](../sdk/launchpad/packages/sdk/src/served-routes.ts). Development-only operations are refused in production; unrouted operations and operations requiring a dedicated stream transport throw `CapabilityUnavailableError`. A generated DTO alone does not establish a served route.

Operation timeouts default to the greater of the generated budget and 8 seconds for GET or 10 seconds for writes. An explicit timeout overrides that calculation; transport bounds it to 1–60,000 ms. Only declared idempotent GET requests receive retries, at most two; mutations receive none. `HttpError` preserves status and the decoded error envelope, including its code and request ID. Public GET operations share concurrent in-flight requests; private requests do not.

[`browserStreamTransportV2`](../sdk/launchpad/packages/sdk/src/browser-stream.ts) reads `/v2/snapshots` and connects to `/v2/stream` with subprotocol `sidiora.v2`. Authenticated connections obtain a fresh ticket from `/v2/stream/tickets`, bound to origin, topics, schema major, filter digest and endpoint. Public streams refuse private topics. The client sends a ping every 4 seconds by default. [`SnapshotStreamV2`](../sdk/launchpad/packages/sdk/src/stream.ts) applies the snapshot and watermarks before replay. Keep monetary values and sequence values in their declared decimal-string/BigInt representations; PAX has 18 decimals and USDL has 6.

## Launchpad SDK v2

[`@kindle/sdk-v2`](../sdk/launchpad/packages/sdk-v2/package.json) publishes generated v2 [types](../sdk/launchpad/packages/sdk-v2/src/types.gen.ts) and [schemas](../sdk/launchpad/packages/sdk-v2/src/schemas.gen.json). The main SDK uses these through `decodeV2Contract` and generated operations. This package supplies wire definitions, not a separate network connection.

## Launchpad SDK production contracts

[`@kindle/production-contracts`](../sdk/launchpad/packages/production-contracts/src/index.ts) exports the generated types plus `ContractValidationError`, `validateContract` and `decodeContract`. Its [schemas](../sdk/launchpad/packages/production-contracts/src/schemas.gen.json) define the accepted v1 shapes.

## Launchpad SDK production contracts v2

[`@kindle/production-contracts-v2`](../sdk/launchpad/packages/production-contracts-v2/package.json) publishes generated [v2 types](../sdk/launchpad/packages/production-contracts-v2/src/types.gen.ts) and [schemas](../sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json). Match the producer's schema version and field set; a similarly named v1 DTO is not an interchangeable decoder.

## Launchpad candles SDK

[`CandlesClient`](../sdk/launchpad/packages/candles-sdk/src/index.ts) implements TradingView UDF config, symbol, history and time reads. Configure its base URL to the UDF prefix, such as the same-origin `/v2/udf`; it appends the individual resource names. `HistoryResponse` distinguishes bars from `{s: 'no_data', nextTime?}`. `CandlesHttpError` represents HTTP failure and `UdfResponseError` a UDF error response. See [trading charts](services.md#launchpad-trading-charts).

## Launchpad indexer SDK

[`HttpClient`](../sdk/launchpad/packages/indexer-sdk/src/index.ts) requires `baseUrl` before use. `getRecentTrades(poolAddress)` requests `/stats/{poolAddress}/transactions`, validates `transactions`, then returns `{pool, trades}`. Amounts, prices and fees remain strings. This is a compatibility data API call, not an indexer operational-status call. The production API has removed the unversioned `/stats/*` family; use the versioned trade operation there. See [API](services.md#launchpad-api) and [indexer](services.md#launchpad-indexer).

## Launchpad metadata SDK

[`createMetadataClient`](../sdk/launchpad/packages/metadata-sdk/src/index.ts) exposes the compatibility metadata DTO with snake-case fields, nullable display fields and string `total_supply`. Its error types distinguish HTTP, malformed responses and upload failure. The retained upload method does not imply that production serves legacy multipart uploads; use the [metadata service's](services.md#launchpad-metadata) authenticated asset and publication routes for current writes.

## Launchpad ranking SDK

[`RankingHttpClient`](../sdk/launchpad/packages/ranking-algo-sdk/src/index.ts) reads `/rankings/{category}` from its configured base URL, preserving snapshot freshness and watermark information. Categories are `trending`, `breakout`, `volume`, `gainers`, `losers`, `movers`, `unusual` and `new`; query options include limit, cursor and cancellation. See [ranking](services.md#launchpad-ranking-algo).

## Launchpad stats SDK

[`StatsHttpClient`](../sdk/launchpad/packages/stats-sdk/src/index.ts) reads `/stats/{poolAddress}` and `/stats/batch?pools=...` from a configured compatibility service base. It validates stats and distinguishes `StatsHttpError` from `StatsResponseError`. Production API registration has removed these unversioned routes; use the declared v2 stats operations for that producer.

## Launchpad users SDK

[`UsersHttpClient`](../sdk/launchpad/packages/users-sdk/src/index.ts) serves the compatibility `/users/{wallet}` family, including signed profile/watchlist changes and image uploads. Responses retain their snake-case fields. This is a distinct contract from the current authenticated `/v2/me/*` account routes in the [user service](services.md#launchpad-user); do not substitute one response shape for the other.

## Launchpad SDK Sidiora launchpad v3

[`sidiora-launchpad-v3`](../sdk/launchpad/packages/sidiora-launchpad-v3/src/index.ts) exports ABI-backed factory, trade, quote and configuration clients. `validateDeployment` requires chain ID 125 and nonzero valid addresses; `createLaunchpadClient` also requires the viem public client to specify that chain. `deploymentFromManifest` reads `_meta.network`, `_meta.usdl` and the named proxy keys. These checks validate configuration; use deployment authority to attest code identity.

## Launchpad SDK Sidiora v3

The ABI package's [provenance.json](../sdk/launchpad/packages/sidiora-launchpad-v3/provenance.json) records its source provenance. Compare it with the selected deployment and [native ABI](launchpad.md#native-execution) before using an exported ABI for writes.

## Launchpad SDK protocol Go

[`sdk/launchpad/protocol`](../sdk/launchpad/protocol) owns the Go contract bindings and deployment address package used by services. Generated bindings preserve ABI argument types and event shapes. They describe the selected contract interface, not an independent execution network; native and retained Solidity roles are distinguished in [launchpad.md](launchpad.md).

## Launchpad SDK shared Go

[`sdk/launchpad/shared`](../sdk/launchpad/shared) supplies service configuration, chain access, authentication, delivery, database support and generated [v1](../sdk/launchpad/shared/interfaces/v1) / [v2](../sdk/launchpad/shared/interfaces/v2) contracts. [BaseEnv](../sdk/launchpad/shared/config/env.go) defaults `CHAIN_ID` to 125 and validates role addresses against the reviewed deployment. RPC, database and Redis URLs are private startup configuration. Only the allowlisted deployment export is intended for public clients; see [service configuration](services.md#configuration-and-wire-rules).
