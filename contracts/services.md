# Launchpad service contracts

Launchpad services read and project the single Paxeer BFT network, EVM chain ID **125** (`0x7d`). The kernel belongs to `paxd` and block proposal, and a receipt is final on block commit. Services provide indexing, application state and authenticated HTTP/stream transports; they do not add a settlement network. Tenant domains are database namespaces, not new chain infrastructure.

Use public node JSON-RPC at the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`. For example, `https://api5.mainnet-beta.paxeer.network`. `node.hyperpaxeer.com` is the build library mirror, not an RPC endpoint. Hosted `px_` methods belong to the separate [platform gateway contract](../services/platform/hosted/gateway/openrpc.json). Launchpad REST and stream routes below are application services.

See [client contracts](clients.md) for origin selection and SDK decoders, and [execution contracts](launchpad.md) for the native module at `0x1017`, ABI surfaces, errors and events.

## Configuration and wire rules

Each service owns its startup configuration under its module. Shared [BaseEnv](../sdk/launchpad/shared/config/env.go) defines `DATABASE_URL`, `REDIS_URL`, `REDIS_BULL_URL`, `RPC_URL`, optional `RPC_URL_FALLBACK`, `CHAIN_ID`, logging, port and the role address book. Defaults include chain 125, `NODE_ENV=production`, `LOG_LEVEL=info` and `PORT=3000`. Validation checks the selected reviewed deployment, including role-address agreement. Private fixture configuration is a distinct validated realm with a digest-bound manifest and owned loopback RPC; it is not an alternate public deployment selector.

Services extend this with their own identity, delivery, schema, pool-budget and provider configuration. A document field and its digest bind a specific runtime input; setting a base database URL alone does not satisfy production startup. Keep private URLs, service credentials and raw operational evidence out of public network exports.

The generated Go [v1](../sdk/launchpad/shared/interfaces/v1) and [v2](../sdk/launchpad/shared/interfaces/v2) contracts preserve versioned field names and encodings. Compatibility routes can return unversioned snake-case DTOs. Match a decoder to the route actually called; do not infer its shape from a nearby SDK type or a `/v2` prefix alone. Money, revisions and watermarks retain the representation declared by that response schema.

## Launchpad API

[`services/launchpad/api`](../services/launchpad/api) is the public read and market-stream service. [app.go](../services/launchpad/api/internal/app/app.go) selects production roles using `API_ROLE=rest` or `API_ROLE=sockets`; shared configuration also allows `development` outside production. Production REST uses the accounting read model and public projections. The API does not run the source services' schema migrations.

| Surface | Producer and behavior |
| --- | --- |
| `GET /v2/pools`, `GET /v2/pools/{poolAddress}` | [Production pool reads](../services/launchpad/api/internal/rest/compatibility.go) |
| `GET /v2/pools/{poolAddress}/stats`, `POST /v2/pools/stats:batch` | Validated public stats projections |
| `GET /v2/rankings/{category}`, `GET /v2/platform/metrics` | Ranking and platform projections |
| `GET /v2/snapshots` | Snapshot plus replay position; private topics require authenticated scope |
| `GET /v2/stream`, `GET /v2/stream/sse` | Production sockets role; WebSocket or SSE transport |
| `GET /v2/network/authority`, `/v2/preflight/*` | [Reviewed deployment and pinned chain reads](../services/launchpad/api/internal/app/authority.go) |
| `GET /v2/search` | Production search proxy registered by the app, backed by stats-workers |

The [authority implementation](../services/launchpad/api/internal/app/authority.go) compares manifest and address sets, uses a committed head and supplies a projected activation attestation. Missing reviewed inputs leave authority unavailable. A recorded address or an off-chain quote does not authorize transaction dispatch.

[ResponseMetadata](../services/launchpad/api/internal/rest/compatibility.go) emits request/schema headers, validates query bounds and gives these wrapped reads a 400 ms context. Projection responses include `requestId`, `data`, `revision`, `watermark`, `quality`, `nextCursor` and `generatedAt`; the compatibility projection writer reports `quality: "partial"`. Never manufacture a complete snapshot from missing authoritative data.

### API errors and unavailable operations

[primitives.go](../services/launchpad/api/internal/rest/primitives.go) defines the API error object: `code`, `message`, `requestId`, `retryable`, nullable `retryAfterMs`, `fieldErrors` and `details`. It also sets `X-Request-ID` and `X-Schema-Version`. Representative v2 mappings are:

| HTTP | Code |
| --- | --- |
| 400 | `QUERY_INVALID`, `CURSOR_INVALID` |
| 401 / 403 | `AUTH_EXPIRED` / `SCOPE_DENIED` |
| 404 | `NOT_FOUND` |
| 409 | `REVISION_CONFLICT`, `IDEMPOTENCY_CONFLICT`, `RESNAPSHOT_REQUIRED` |
| 422 | `FIELD_INVALID`, `FIELD_UNKNOWN` |
| 429 | `RATE_LIMITED` |
| 501 | `FEATURE_UNAVAILABLE` |
| 503 | `DEPENDENCY_UNAVAILABLE`, `INTEGRITY_QUARANTINED` |

Ordinary 429/503 responses from this writer are retryable with a one-second delay. `WriteV2NotConfiguredError` instead returns **503 `DEPENDENCY_UNAVAILABLE`, `retryable: false`, `retryAfterMs: null` and no `Retry-After`**. [production_unserved.go](../services/launchpad/api/internal/app/production_unserved.go) uses this for operations without configured providers, including quotes, funding, automation and several operator routes. Schema publication does not enable them. Private operation responses remain private and non-cacheable.

The [production search proxy](../services/launchpad/api/internal/rest/rest.go) strips credentials before forwarding `/v2/search`. It does not register `/v2/search/expanded`; that path has a development API handler and a stats-workers handler. The production API also removes the unversioned `/stats/*`, `/rankings/*` and other legacy read families. Point compatibility SDKs only at a service that actually registers their routes.

The frozen [v2 schemas](../sdk/launchpad/shared/interfaces/v2) define `RankingsResponse`, `PoolListResponse`, `PoolDetailResponse`, `SearchResponse` and `Error`; REST and generated SDK consumers use the same encodings. Public market snapshots and compact search are anonymous reads. Rankings are replacement snapshots: `limit` is 1–100 (default 50), `totalEligible` describes the complete eligible set, and `truncated` remains true when the response or its upstream projection omits entries. `nextCursor` is null. A nonempty rankings cursor returns HTTP 400 `CURSOR_INVALID` in both profiles; an empty cursor returns 422 `FIELD_INVALID`.

For search errors, `/v2/search` preserves a reachable upstream's frozen error response. Connection failures, the two-second response-header timeout and the three-second complete-response timeout return HTTP 503 `DEPENDENCY_UNAVAILABLE` in the seven-field `Error` envelope, with `retryable: true`, `retryAfterMs: 1000`, `Retry-After: 1`, `Cache-Control: no-store`, `X-Request-ID` and `X-Schema-Version: 2`. The proxy removes `Authorization`, `Cookie` and actor-wallet headers before forwarding; public search does not convey a caller's identity to its upstream.

Development market startup checks `eth_chainId=0x7d` before accepting a configured RPC endpoint. Reads and the exposed chain-readiness callback refresh that binding, including when block or creation positions are cached. A mismatched or unreachable node makes development reads unavailable; missing RPC configuration never creates a verified watermark. Metadata applies the same chain admission before cached or published v2 reads, and its `media_rpc` readiness check refreshes the binding.

Development routes are mounted separately. [Development discovery](../services/launchpad/api/internal/rest/dev_discovery.go) serves anonymous reads over local projections. Production discovery depends on its configured runtime authorities, as described below.

## Launchpad indexer

[`indexerd`](../services/launchpad/indexer/cmd/indexerd/main.go) wires committed-chain ingestion, decode, persistence and publication in [app.go](../services/launchpad/indexer/internal/app/app.go). Production requires receipt qualification and lifecycle/delivery configuration; committed logs must agree with reviewed emitter and chain identities. [The decoder registry](../services/launchpad/indexer/internal/decode/registry.go) refuses `removed_log`, `invalid_identity`, `ambiguous_emitter`, `unauthorized_signature` and `untrusted_emitter`. Public trade data is served through the API, not the indexer's status server.

[config.go](../services/launchpad/indexer/internal/config/config.go) defines `INDEXER_MODE` (`live` or `backfill`), `INDEXER_START_BLOCK`, batch/concurrency/poll settings, source selection and optional `INDEXER_HISTORICAL_RPC_URL`. Source values are `paxscan`, `rpc` and `rpc-getlogs`; `INDEXER_HEAD_SOURCE` overrides the head source. Archive RPC is for history unavailable on the primary node, not a second execution authority. Production receipt and lifecycle documents impose additional requirements beyond these source switches.

[Operational routes](../services/launchpad/indexer/internal/httpapi/status.go) always mount `GET /status`, `GET /status/dead-letter` and `POST /status/dead-letter/drain`, independently of the publisher. Status contains `chainId`, `mode`, `lastProcessedBlock`, indexed pool/swap counts and uptime. Canonical mode reads the committed stream watermark and durable delivery health. Legacy mode reads the cursor and may include publisher webhook counters. Missing configuration or failed reads return HTTP 503 unavailable with unknown values left null and unavailableComponents identifying failures. Quarantined deliveries or delivery lag exceeding MaxDeliveryLag return HTTP 503 degraded. Unavailable components take precedence over degraded delivery health; healthy reads return HTTP 200 with `status: "ok"`.

Both dead-letter routes are always mounted, independently of the publisher, and return HTTP 503 when the policy is unavailable. Configured dead-letter inspection requires an authenticated operator session, scoped authorization and durable allow/deny audit, and returns bounded quarantine metadata. The [audited handler](../services/launchpad/indexer/internal/httpapi/deadletter.go) requires the `inspect` scope and reads durable quarantine with cursor pagination and a limit of 1–100 entries (default 100). Metadata includes consumer, generation, envelope hash, failure class and creation time, without delivery payloads or credentials. Authorization or audit dependency failures return HTTP 503; authentication and scope denials return HTTP 401 and 403 respectively. Drain is refused even with retention-delete scope and deletes neither durable quarantine nor the compatibility publisher queue. An authorized drain request returns HTTP 403 because separate retention approval is required.

These are configured handler capabilities. The current production [RegisterStatus caller](../services/launchpad/indexer/internal/app/app.go) supplies the store, chain ID, publisher and start time, but does not supply the status mode, canonical stream ID, durable delivery health, maximum delivery lag or dead-letter policy dependencies. Mounting these routes alone does not configure healthy status reads or enable policy-protected inspection.

## Launchpad gateway

[`services/launchpad/gateway`](../services/launchpad/gateway) is the launchpad identity/media application gateway. [app.go](../services/launchpad/gateway/internal/app/app.go) composes identity, uploads, connected service proxies and control routes. [connected.go](../services/launchpad/gateway/internal/proxy/connected.go) defines the forwarded route inventory; use that inventory to determine the actual owner rather than assuming every SDK operation is routed.

Identity HTTP behavior lives in [internal/auth](../services/launchpad/gateway/internal/auth). The web forwards `/v2/auth/*` and `/v2/stream/tickets` here. Connected account/social routes apply their required session or assertion guards. The gateway's `/v2/stream` is the social socket, distinct from the API's market stream. Server-to-server identity assertions and trusted transport metadata are not browser-supplied account authorization.

## Launchpad discovery

[`discovery.HTTP`](../services/launchpad/discovery/http.go) exposes feed and feed-detail reads, callout writes, alerts, squads, invitations and discovery leaderboards. It is a library mounted by the API runtime, not a separate chain node. Feed requests accept `order`, `cursor` and bounded `limit`; authenticated mutations resolve an actor through the identity authority. Squad changes bind expected revisions and permissions.

Visibility, source and interval authorities are explicit dependencies. [registerDiscovery](../services/launchpad/api/internal/app/app.go) loads `DISCOVERY_RUNTIME_CONFIG_FILE` or `DISCOVERY_RUNTIME_CONFIG_DOCUMENT`, bound by `DISCOVERY_RUNTIME_CONFIG_SHA256`. With that configuration it constructs pool and interval authorities and the notification/discovery runtime, including readiness checks. Without it, the fallback mounts selected account/squad handlers but returns non-retryable 503 for feed, callout and discovery-leaderboard routes, and reports missing source/delivery authorities in readiness. A missing provider must remain unavailable; do not represent it as an empty successful feed.

## Launchpad notifications

[`HTTP`](../services/launchpad/notifications/http.go) serves `GET /v2/me/notifications` and `PUT /v2/me/notifications/{notificationId}/read`, plus discovery compatibility routes. [HTTPRuntime](../services/launchpad/notifications/runtime.go) mounts notification and discovery handlers and owns their runtime dependencies. Account identity scopes private notification reads and changes.

[consumers.go](../services/launchpad/notifications/consumers.go) and the provider adapters implement delivery. Credentials come from the configured secret provider and refresh under the runtime; missing or stale credentials make that authority unavailable. A durable notification and its push-provider delivery are different states; provider acceptance does not commit a chain transaction.

## Launchpad metadata

[`services/launchpad/metadata`](../services/launchpad/metadata) separates canonical token identity from creator-edited display content and moderated assets. [ConnectedMedia](../services/launchpad/metadata/internal/metadata/connected.go) registers:

| Method and path | Contract |
| --- | --- |
| `GET`, `PATCH /v2/tokens/{tokenAddress}/metadata` | Canonical identity plus revisioned display metadata |
| `GET /v2/tokens/{tokenAddress}/media` | Token media read |
| `PUT`, `GET /v2/tokens/{tokenAddress}/assets/{assetId}` | Creator asset upload/status |
| `PUT`, `GET /v2/accounts/{wallet}/assets/{assetId}` | Account-owned asset upload/read |

The [v2 patch parser](../services/launchpad/metadata/internal/metadata/v2.go) requires `expectedRevision`, bounds the JSON body to 32 KiB and rejects undeclared or duplicate fields. Display content cannot rewrite chain identity. Connected production readiness requires canonical identity, storage verification, raster worker, scanner, object storage and moderation dependencies.

[ProductionLegacyReads](../services/launchpad/metadata/internal/metadata/legacy_reads.go) retains `GET /metadata/{tokenAddress}` (also accepting a `.json` suffix), `GET /metadata/batch?addresses=...`, `GET /logo/{tokenAddress}.png` and the banner path. Reads combine canonical creation identity with allowed published metadata. Unknown tokens retain a 200 response with nullable identity/display fields. Production banners return 404. Logos are approved 512-pixel variants checked against content identity. Legacy multipart upload is not served in production; current writes use versioned asset and publication routes, including [publication.go](../services/launchpad/metadata/internal/metadata/publication.go).

The frozen v2 `MetadataGetResponse` and `ConnectedTokenMediaResponse` envelopes distinguish canonical token identity from creator display edits and approved media. Public GETs do not grant write permission. Asset upload/status and metadata mutations require the authenticated owner session and the route's declared revision checks; v3 publication also requires its signed operation envelope. Missing identity, moderation, scanner, chain or storage authorities fail closed.

For media visibility, upload acceptance does not make an object public: assets move through `UPLOADING`, `QUARANTINED`, `VALIDATING` and `APPROVED`, or into `REJECTED`/`DELETED`. Only approved, content-addressed variants may be exposed by connected reads. Removed content and images stay hidden after moderation refresh, including legacy single, batch and logo reads; canonical identity alone is not permission to show creator content. With no published approved image, connected media reports a nullable image rather than inventing a URL. Legacy errors retain their legacy shape; v2 metadata errors use the frozen metadata envelope and lowercase codes such as `unavailable`.

For publication recovery, `POST /v3/tokens/{tokenAddress}/metadata-publications` admits a durable intent and job tied to `draftId`, `operationId`, creation transaction, metadata content hash and upload references. Admission is not publication. `GET /v3/tokens/{tokenAddress}/metadata-publications/{operationId}` reports the persisted operation status. The worker leases durable work and, after a restart, retries eligible work while rechecking identity, canonical creator, revision, asset approval and moderation. A retry uses the same operation/content binding; changing the body under the same operation is a conflict. Reads expose the committed published revision, never a queued draft. `attemptCount`, `lastErrorCode` and nullable `revision` distinguish retryable work from published or terminal outcomes. Legacy multipart publication and cleanup have separate lifecycle paths; a durable v3 job does not certify recovery of an unrelated legacy write.

## Launchpad user

[`account.HTTP`](../services/launchpad/user/internal/account/http.go) owns versioned profile, watchlist and account routes. Public profile reads include `/v2/profiles/{wallet}` and `/v2/profiles:batch`; authenticated watchlist routes are `GET /v2/me/watchlist` and `PUT`/`DELETE /v2/me/watchlist/{poolAddress}`. The owner routes are registered by [v2_owner_http.go](../services/launchpad/user/internal/account/v2_owner_http.go); extended v3 profile/preferences routes require an extended runtime.

Private account operations consume an authenticated principal; mutations also enforce their declared revision and idempotency requirements. Owner preferences use the declared models in [models.go](../services/launchpad/user/internal/account/models.go) and [preferences_http.go](../services/launchpad/user/internal/account/preferences_http.go); do not drop notification preference categories while adapting the response. Compatibility `/users/*` routes and their SDK use a separate wire shape. The application wiring in [app.go](../services/launchpad/user/internal/app/app.go) determines which adapters and routes run in each environment.

## Launchpad trading charts

[`RegisterProductionUDF`](../services/launchpad/trading-charts/internal/httpapi/udf.go) mounts `/v2/udf/config`, `/v2/udf/symbols`, `/v2/udf/history` and `/v2/udf/time`. UDF history follows TradingView's bars/no-data contract. The retained unversioned `/config`, `/symbols`, `/history` and `/time` routes are a different registration path. The web's same-origin UDF rewrites target this service; live market candles use API stream topics. See [CandlesClient](clients.md#launchpad-sdk-candles-sdk).

## Launchpad stats workers

[`stats-workers`](../services/launchpad/stats-workers) computes public stats and holdings/search projections. [search.go](../services/launchpad/stats-workers/internal/httpapi/search.go) registers canonical `/v2/search` and integrated `/v2/search/expanded` alongside compatibility search. The API's production app proxies only `/v2/search` against the stats origin; expanded search requires a separately registered path to its producer. Expanded search can require identity and visibility authorities; compatibility `/search` is not a substitute for the v2 response contract.

## Launchpad ranking algo

[`ranking-algo`](../services/launchpad/ranking-algo) produces ranked snapshots. [snapshots.go](../services/launchpad/ranking-algo/internal/httpapi/snapshots.go) owns snapshot HTTP behavior, and [app.go](../services/launchpad/ranking-algo/internal/app/app.go) chooses the runtime registration. Rankings retain category, revision/watermark and freshness information. Clients must preserve those fields and treat an unavailable or stale projection according to its declared quality, rather than synthesizing rankings from missing inputs.

## Launchpad PnL tracker

[`pnl-tracker`](../services/launchpad/pnl-tracker) owns portfolio accounting projections, history, share journeys and referral/reward adapters. [Production app wiring](../services/launchpad/pnl-tracker/internal/app/app.go) registers `/v3/me/portfolio`, `/v3/wallets/{wallet}/portfolio`, private history/live-enrollment routes and versioned referral operations; non-production reads are mounted separately. Private routes require their identity authorities. A portfolio projection or prepared reward claim is not proof that an on-chain transfer committed.

## Launchpad social

[`social`](../services/launchpad/social) owns interactions, chat, moderation and safety state. [app.go](../services/launchpad/social/internal/app/app.go) mounts the connected `/v2/stream` chat socket, `/v2/social` interactions and contract/safety routes; the gateway fronts selected operations with identity guards. The chat socket and the API market/private-data stream share a path spelling on different services, with different payload contracts. Preserve room authorization and moderation visibility when forwarding or caching content.

## Launchpad livestream

[`streams/handlers.go`](../services/launchpad/livestream/internal/streams/handlers.go) registers `/v2/streams` lifecycle, playback, ingest-key and operation-status routes, plus provider webhooks. Provider-backed mutations are wrapped in provisioning checks and creator authorization. Legacy `/streams/*` routes use the legacy handler; their presence does not establish current support. Provider credentials, playback policy and operation status remain separate from public stream metadata. A submitted provider operation is not the same as a live or ended stream.

## Launchpad security

[`security`](../services/launchpad/security) and [`security/control`](../services/launchpad/security/control) implement policy, provider and control-plane workflows consumed by the gateway. Web requests for `/v2/control/*` go to that gateway, including capability admission before token creation. Use the selected capability and actual runtime authority to decide whether a write is available; a catalog row or an exported type does not grant permission.

## Launchpad workspace

[go.work](../services/launchpad/go.work) declares Go 1.25.0 and the service modules together with shared/protocol SDK modules and recovery/load helpers. The [Makefile](../services/launchpad/Makefile) drives an explicit `MODULES` list including discovery, notifications, security and security/control. Its list and `go.work` membership serve different purposes; neither implies every helper runs in production.

`build`, `vet`, `fmt-check`, `lint`, `test`, `race`, `cover` and `cover-check` operate across that declared list; `all` and `ci` combine the release/development checks. `test-module` requires a listed `MODULE` and an anchored `RUN` regex. `cover-check` applies 85% generally and 90% to the declared money modules. `sqlc` regenerates modules that contain a configuration. These commands describe workspace tooling; qualification evidence must name the command and revision that actually ran. See [launchpad tooling](launchpad.md#launchpad-tools).
