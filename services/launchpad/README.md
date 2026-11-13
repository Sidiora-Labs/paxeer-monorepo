# Launchpad services

This directory holds the Go backend services for Launchpad: market-event indexing,
portfolio accounting, statistics, rankings, charts, realtime data delivery, media,
community features and accounts. They support the launchpad markets on Paxeer's
single BFT network by reading committed chain state and maintaining application
views in PostgreSQL and Redis, with NATS used for event delivery. Market execution
belongs to the network's [launchpad module](../../modules/launchpad/) and
[precompile](../../precompiles/launchpad/); these services provide the application
backend around it.

## Layout

| Path | Contents |
| --- | --- |
| [api/](api/) | REST snapshots, WebSocket and SSE market-data delivery, with discovery integration. |
| [discovery/](discovery/) | Discovery feeds, callouts, squads, invitations, leaderboards and price alerts. |
| [gateway/](gateway/) | Authentication, private control integration, media delivery and forwarding to account, social and metadata services. |
| [indexer/](indexer/) | Chain log ingestion, receipt validation, backfill, database projections and event publication. |
| [licenses/](licenses/) | Launchpad license information and third-party notice guidance. |
| [livestream/](livestream/) | Persistent stream lifecycle, viewer sessions and configured media-provider workers. |
| [metadata/](metadata/) | Token metadata, image uploads, scanning, moderation and object storage. |
| [notifications/](notifications/) | Notification preferences, durable delivery and APNS, FCM and Web Push adapters. |
| [pnl-tracker/](pnl-tracker/) | Portfolio and profit-and-loss accounting, reconciliation, cards and referrals. |
| [ranking-algo/](ranking-algo/) | Scheduled market rankings and their read API. |
| [security/](security/) | Authorization policies, private operator controls and supply-chain admission tools. |
| [social/](social/) | Pool chat, direct messages, moderation and realtime social delivery. |
| [stats-workers/](stats-workers/) | Event-driven holder, trader and platform statistics with background aggregation. |
| [trading-charts/](trading-charts/) | Candle aggregation, chart history, TradingView UDF responses, realtime updates and archive rebuilding. |
| [user/](user/) | Profiles, avatars, account preferences, exports and notification integration. |
| [Makefile](Makefile) | Build, test, coverage, lint and query-generation targets for the Go modules. |
| [go.work](go.work) | Local Go workspace joining service modules, shared SDK modules and supporting tools. |
| [go.work.sum](go.work.sum) | Go workspace dependency checksums. |
| [LICENSE](LICENSE) | Launchpad-local license summary; see [licenses/README.md](licenses/README.md) for applicability. |

## Build and test

Run these commands from this directory. The [Makefile](Makefile) selects the Go
toolchain with `GOTOOLCHAIN=go1.27.3` and builds each listed module independently
with `GOWORK=off`. Its module list also includes the shared and protocol packages
under [sdk/launchpad](../../sdk/launchpad/).

```sh
make build
make test
```

`make build` runs `go build ./...` in the listed modules. `make test` runs each
module through the repository's
[test runner](../../tests/ci/data/launchpad_go_tests.py) and
[test baseline](../../tests/ci/data/launchpad_go_test_baseline.json), so Python 3
is also required. Integration tests use PostgreSQL, Redis, NATS and chain clients;
service-specific tests may also require Anvil and S3-compatible object storage.

Additional targets in the same Makefile are `make vet`, `make fmt-check`,
`make lint`, `make race` and `make cover-check`. `make ci` combines the declared
CI targets. `make sqlc` regenerates query bindings for modules with `sqlc.yaml`
and requires the `sqlc` executable.

The Python supply-chain tests have a separate command, run from the repository root:

```sh
python3 -m unittest discover -s services/launchpad/security/supplychain -p 'test_*.py' -v
```

See the [supply-chain guide](security/supplychain/README.md) for its dependencies
and the [livestream guide](livestream/PROGRESS.md) for media integration prerequisites.

## Configuration

Service daemons load their configuration from the process environment.
The linked loaders below define which inputs apply to each service and validate
their combinations. Names are listed without deployment values.

### Shared inputs

These inputs occur in several service loaders; each loader declares its own subset.

- **Storage and secrets**: `DATABASE_URL`, `REDIS_URL`, `SECRETS_PROVIDER`, `SECRETS_REFRESH_SECONDS`.
- **Runtime and HTTP**: `NODE_ENV`, `PORT`, `LOG_LEVEL`, `PUBLIC_URL`, `RUNTIME_PROFILE`, `BIND_ADDRESS`, `ZONE`,
  `METRICS_ENVIRONMENT`, `CORS_ALLOWED_ORIGINS`, `TRUSTED_PROXY_CIDRS`.
- **Database pool and schema documents**: `DB_BUDGET_FILE`, `DB_BUDGET_DOCUMENT`, `DB_BUDGET_SHA256`, `DB_POOL_ID`, `INSTANCE_ID`,
  `EXPECTED_SCHEMA_FILE`, `EXPECTED_SCHEMA_DOCUMENT`, `EXPECTED_SCHEMA_SHA256`.
- **Event delivery**: `DELIVERY_CONFIG_FILE`, `DELIVERY_CONFIG_DOCUMENT`, `DELIVERY_CONFIG_SHA256`, `NATS_URL`,
  `NATS_CREDENTIALS`, `NATS_CREDENTIALS_CREDS`, `NATS_USERNAME`, `NATS_PASSWORD`,
  `WEBHOOK_HMAC_SECRET`.
- **Identity**: `IDENTITY_SERVICE_URL`, `IDENTITY_SERVICE_TOKEN`, `IDENTITY_SIGNING_KEY`,
  `IDENTITY_PEER_CERT_SHA256`, `IDENTITY_MTLS_CA_FILE`, `IDENTITY_MTLS_CA_PEM`,
  `IDENTITY_MTLS_CLIENT_CERT_FILE`, `IDENTITY_MTLS_CLIENT_CERT_PEM`,
  `IDENTITY_MTLS_CLIENT_KEY_FILE`, `IDENTITY_MTLS_CLIENT_KEY_PEM`, `IDENTITY_MTLS_PEER_CERT_SHA256`,
  `IDENTITY_ASSERTION_SIGNING_KEY`.
- **Object storage**: `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION`.
- **Connections and transport trust**: `WS_MAX_CONNECTIONS`, `WS_MAX_PER_IP`, `DB_MAX_CONNS`, `GATEWAY_PEER_CERT_SHA256`.

The indexer, PnL tracker, ranking, statistics and chart loaders embed
[BaseEnv](../../sdk/launchpad/shared/config/env.go). In addition to the shared
inputs above, it reads:

`REDIS_BULL_URL`, `CHAIN_ID`, `RPC_URL`, `RPC_URL_FALLBACK`, `EVENT_EMITTER_ADDRESS`,
`POOL_REGISTRY_ADDRESS`, `ROUTER_ADDRESS`, `FACTORY_ADDRESS`, `QUOTER_ADDRESS`,
`PROTOCOL_CONFIG_ADDRESS`, `FEE_ACCUMULATOR_ADDRESS`, `SIDIORA_NFT_ADDRESS`,
`FEES_ROUTER_ADDRESS`, `POOL_BEACON_ADDRESS`, `DEPLOYMENT_REALM`, `PRIVATE_FIXTURE_MANIFEST_FILE`,
`PRIVATE_FIXTURE_MANIFEST_SHA256`.

### Service inputs

The following names supplement the shared inputs. Each linked source is the
service's configuration loader.

- **[api](api/internal/config/config.go)**: `DISCOVERY_RUNTIME_CONFIG_FILE`, `DISCOVERY_RUNTIME_CONFIG_DOCUMENT`,
  `DISCOVERY_RUNTIME_CONFIG_SHA256`, `NETWORK_AUTHORITY_CONFIG_FILE`,
  `NETWORK_AUTHORITY_CONFIG_DOCUMENT`, `NETWORK_AUTHORITY_CONFIG_SHA256`, `NETWORK_RPC_URL`,
  `IDENTITY_CA_FILE`, `IDENTITY_CLIENT_CERT_FILE`, `IDENTITY_CLIENT_KEY_FILE`, `IDENTITY_CA_PEM`,
  `IDENTITY_CLIENT_CERT_PEM`, `IDENTITY_CLIENT_KEY_PEM`, `IDENTITY_ASSERTION_ISSUER`,
  `IDENTITY_ASSERTION_AUDIENCE`, `STREAM_CONSUMER_CONFIG_FILE`, `STREAM_CONSUMER_CONFIG_DOCUMENT`,
  `STREAM_CONSUMER_CONFIG_SHA256`, `STREAM_SCHEMA`, `STREAM_RELAY_CONFIG_FILE`,
  `STREAM_RELAY_CONFIG_DOCUMENT`, `STREAM_RELAY_CONFIG_SHA256`, `API_ADMISSION_CAPACITY`,
  `API_ADMISSION_MAX_WAITING`, `NETWORK_AUTHORITY_CACHE_TTL_SEC`, `API_ROLE`, `STREAM_SIGNING_KEY`,
  `STATS_SERVICE_URL`, `ACCOUNTING_READ_CONFIG_SHA256`, `ACCOUNTING_READ_CONFIG_FILE`,
  `ACCOUNTING_READ_CONFIG_DOCUMENT`, `API_RATE_LIMIT_MAX`, `API_RATE_LIMIT_WINDOW_SEC`,
  `API_MAX_INFLIGHT_REQUESTS`, `SSE_MAX_CONNECTIONS`, `SSE_MAX_PER_IP`, `CLIENT_SEND_BUFFER`,
  `COALESCE_FLUSH_MS`, `ADMIN_API_KEY`.
- **[gateway](gateway/internal/config/config.go)**: `IDENTITY_CONFIG_SHA256`, `IDENTITY_CONFIG_DOCUMENT`, `IDENTITY_CONFIG_FILE`,
  `GATEWAY_RUNTIME_FILE`, `GATEWAY_RUNTIME_SHA256`, `GATEWAY_RUNTIME_DOCUMENT`,
  `IDENTITY_LIMITER_KEY`, `IDENTITY_SIGNING_SECRET`, `RPC_URL`, `GATEWAY_JWT_SECRET`,
  `GATEWAY_JWT_TTL_SECONDS`, `GATEWAY_NONCE_TTL_SECONDS`, `GATEWAY_APP_DOMAIN`, `SOCIAL_HTTP_URL`,
  `SOCIAL_WS_URL`, `METADATA_UPLOAD_URL`, `USER_HTTP_URL`, `UPSTREAM_TIMEOUT_SECONDS`,
  `METADATA_BUCKET`, `USER_BUCKET`, `OG_BUCKET`, `OBJECT_CACHE_MAX_BYTES`,
  `OBJECT_CACHE_TTL_SECONDS`, `MAX_UPLOAD_BYTES`, `RATE_LIMIT_MAX`, `RATE_LIMIT_WINDOW_SECONDS`.
- **[indexer](indexer/internal/config/config.go)**: `INDEXER_LIFECYCLE_CONFIG_FILE`, `INDEXER_LIFECYCLE_CONFIG_DOCUMENT`,
  `INDEXER_LIFECYCLE_CONFIG_SHA256`, `FIXTURE_PUBACK_BARRIER`, `INDEXER_RECEIPT_CONFIG_FILE`,
  `INDEXER_RECEIPT_CONFIG_DOCUMENT`, `INDEXER_RECEIPT_CONFIG_SHA256`, `INDEXER_START_BLOCK`,
  `INDEXER_BATCH_SIZE`, `INDEXER_POLL_INTERVAL_MS`, `INDEXER_CONCURRENCY`, `WEBHOOK_URLS`,
  `WEBHOOK_HMAC_SECRETS`, `INDEXER_MODE`, `BACKFILL_FROM_BLOCK`, `BACKFILL_BATCH_SIZE`,
  `BACKFILL_CONCURRENCY`, `BACKFILL_ONLY`, `PAXSCAN_DATABASE_URL`, `INDEXER_LOG_SOURCE`,
  `INDEXER_HEAD_SOURCE`, `INDEXER_HISTORICAL_RPC_URL`, `INDEXER_HISTORICAL_CONCURRENCY`,
  `INDEXER_HISTORICAL_READAHEAD`, `INDEXER_HISTORICAL_ATTEMPT_TIMEOUT_MS`, `RPC_URLS`,
  `RPC_GETLOGS_TIMEOUT_MS`, `RPC_GETLOGS_STALE_THRESHOLD`, `RPC_GETLOGS_HEALTH_INTERVAL_MS`,
  `RPC_GETLOGS_RECEIPT_CONCURRENCY`.
- **[livestream](livestream/internal/config/config.go)**: `LIVEPEER_PLAYBACK_WEBHOOK_ID`, `LIVEPEER_PLAYBACK_WEBHOOK_URL`,
  `LIVEPEER_PLAYBACK_WEBHOOK_SECRET`, `LIVESTREAM_BUDGET_ALERT_URL`, `LIVESTREAM_BUDGET_ALERT_KEY`,
  `LIVESTREAM_BUDGET_POLICY_FILE`, `LIVESTREAM_BUDGET_POLICY_SHA256`,
  `LIVESTREAM_BUDGET_POLICY_DOCUMENT`, `LIVEPEER_WEBHOOK_TIMESTAMP_UNIT`,
  `LIVESTREAM_CONNECTED_ENABLED`, `MAX_GLOBAL_STREAMS`, `ACTION_ENVIRONMENT`, `ACTION_AUDIT_KEY`,
  `LIVESTREAM_VIEWER_TOKEN_KEY`, `LIVESTREAM_VAULT_TRANSIT_URL`, `LIVESTREAM_VAULT_TRANSIT_TOKEN`,
  `LIVESTREAM_VAULT_TRANSIT_KEY`, `LIVESTREAM_INGEST_ENDPOINT`, `LIVEPEER_PROVIDER_MODE`,
  `RPC_URL`, `DEPLOYMENT_REALM`, `PRIVATE_FIXTURE_MANIFEST_FILE`,
  `PRIVATE_FIXTURE_MANIFEST_SHA256`, `LIVEPEER_API_KEY`, `LIVEPEER_API_URL`,
  `MAX_STREAMS_PER_WALLET`, `LIVEPEER_WEBHOOK_SECRET`,
  `LIVESTREAM_PRIVATE_LISTEN_ADDRESS`, `LIVESTREAM_TLS_CERT_FILE`, `LIVESTREAM_TLS_CERT_PEM`,
  `LIVESTREAM_TLS_KEY_FILE`, `LIVESTREAM_TLS_KEY_PEM`, `LIVESTREAM_TLS_CLIENT_CA_FILE`,
  `LIVESTREAM_TLS_CLIENT_CA_PEM`.
- **[metadata](metadata/internal/config/config.go)**: `METADATA_PUBLICATION_CONFIG_FILE`, `METADATA_PUBLICATION_CONFIG_SHA256`,
  `METADATA_PUBLICATION_AUDIT_KEY`, `MODERATION_PEER_CERT_SHA256`,
  `METADATA_ASSERTION_PRIVATE_KEY_HEX`, `USER_PEER_CERT_SHA256`, `USER_ASSERTION_PUBLIC_KEY_HEX`,
  `METADATA_TLS_CERT_FILE`, `METADATA_TLS_KEY_FILE`, `METADATA_TLS_CLIENT_CA_FILE`,
  `GATEWAY_ASSERTION_PUBLIC_KEY_HEX`, `METADATA_SCHEMA`, `CANONICAL_SCHEMA`, `S3_QUARANTINE_BUCKET`,
  `S3_APPROVED_BUCKET`, `RASTER_EXECUTABLE`, `RASTER_CGROUP_PARENT`, `CLAMAV_HOST`, `CLAMAV_PORT`,
  `IDENTITY_ENDPOINT`, `MODERATION_ENDPOINT`, `MODERATION_BEARER_KEY`,
  `MEDIA_SCAN_INTERVAL_SECONDS`, `MEDIA_EXPIRY_INTERVAL_SECONDS`, `METADATA_INGRESS`,
  `RASTER_ISOLATION`, `MODERATION_SOURCE`, `MODERATION_SCHEMA`,
  `METADATA_PUBLICATION_CONFIG_DOCUMENT`, `IDENTITY_AUDIENCE`, `CANONICAL_STREAM_ID`, `RPC_URL`,
  `MAX_LOGO_SIZE_BYTES`, `MAX_BANNER_SIZE_BYTES`.
- **[pnl-tracker](pnl-tracker/internal/config/config.go)**: `PNL_JOURNEY_CONFIG_FILE`, `PNL_JOURNEY_CONFIG_DOCUMENT`, `PNL_JOURNEY_CONFIG_SHA256`,
  `PNL_CURSOR_KEY`, `PNL_CDN_TOKEN`, `FIXTURE_COMMIT_ACK_BARRIER`, `RUNTIME_ACTION`,
  `BASELINE_WALLET`, `BASELINE_TOKEN`, `PNL_RECONCILE_INTERVAL_MS`, `PNL_RECONCILE_BATCH_SIZE`,
  `PNL_PUBLIC_BASE_URL`, `PNL_OG_BASE_URL`, `PNL_FONT_DIR`, `PNL_REWARD_PER_CONVERSION`.
- **[ranking-algo](ranking-algo/internal/config/config.go)**: `RANKING_RUNTIME_FILE`, `RANKING_RUNTIME_DOCUMENT`, `RANKING_RUNTIME_SHA256`,
  `RANKING_TICK_INTERVAL_MS`, `RANKING_NEW_TICK_INTERVAL_MS`, `RANKING_MAX_ENTRIES`.
- **[social](social/internal/config/config.go)**: `SOCIAL_RUNTIME_FILE`, `SOCIAL_RUNTIME_SHA256`, `SOCIAL_RUNTIME_DOCUMENT`,
  `SOCIAL_PRIVATE_LISTEN_ADDRESS`, `SOCIAL_TLS_CERT_FILE`, `SOCIAL_TLS_CERT_PEM`,
  `SOCIAL_TLS_KEY_FILE`, `SOCIAL_TLS_KEY_PEM`, `SOCIAL_TLS_CLIENT_CA_FILE`,
  `SOCIAL_TLS_CLIENT_CA_PEM`, `SOCIAL_AUTHORITY_PEERS`, `MAX_MESSAGE_LENGTH`, `RATE_WINDOW_SECONDS`,
  `MAX_POOL_MSGS_PER_WINDOW`, `MAX_DM_MSGS_PER_WINDOW`, `WS_SEND_BUFFER`,
  `WS_WRITE_TIMEOUT_SECONDS`, `WS_READ_LIMIT_BYTES`, `CHAT_ADMIN_API_KEY`, `SOCIAL_CURSOR_KEY`.
- **[stats-workers](stats-workers/internal/config/config.go)**: `STATS_CURSOR_KEY`, `STATS_AUDIT_KEY`, `STATS_HOLDER_REFRESH_INTERVAL_MS`,
  `STATS_ACTIVE_POOL_WINDOW_HOURS`, `STATS_PUBLIC_STATS_INTERVAL_MS`, `STATS_PUBLIC_STATS_BATCH`,
  `STATS_PUBLIC_STREAM_SCHEMA`, `STATS_LEGACY_BRIDGE_INTERVAL_MS`, `STATS_LEGACY_BRIDGE_BATCH`.
- **[trading-charts](trading-charts/internal/config/config.go)**: `CHART_FEATURES_CONFIG_FILE`, `CHART_FEATURES_CONFIG_DOCUMENT`, `CHART_FEATURES_CONFIG_SHA256`,
  `CHART_OPERATOR_TOKEN`.
- **[user](user/internal/config/config.go)**: `ACCOUNT_SERVICE_SIGNING_SEED`, `IDENTITY_CERT_SHA256`, `METADATA_SERVICE_URL`,
  `METADATA_CERT_SHA256`, `ACCOUNT_ENABLE_EXTENDED_V3`, `ACCOUNT_EXPORT_KEY`,
  `ACCOUNT_ENDPOINT_KEY`, `ACCOUNT_ENDPOINT_FINGERPRINT_KEY`, `ACCOUNT_AUDIT_KEY`,
  `ACCOUNT_ENDPOINT_PROOF_PUBLIC_KEY`, `ACCOUNT_GATEWAY_PUBLIC_KEY`, `ACCOUNT_GATEWAY_CERT_SHA256`,
  `ACCOUNT_ASSERTION_HEADER`, `ACCOUNT_ASSERTION_SIGNATURE_HEADER`, `ACCOUNT_ASSERTION_ISSUER`,
  `ACCOUNT_ASSERTION_AUDIENCE`, `ACCOUNT_OPERATION_ENVIRONMENT`, `ACCOUNT_CHAIN_RPC_URL`,
  `ACCOUNT_DOWNLOAD_BASE_URL`, `ACCOUNT_ALLOW_METADATA_AVATARS`, `ACCOUNT_USE_READ_SWITCH`,
  `ACCOUNT_DELIVERY_SCHEMA`, `ACCOUNT_SUBJECTS_FILE`, `ACCOUNT_SUBJECTS_SHA256`,
  `ACCOUNT_SUBJECTS_DOCUMENT`, `ACCOUNT_EXPORT_BUCKET`, `ACCOUNT_NATS_URL`,
  `ACCOUNT_NATS_CREDENTIALS_FILE`, `ACCOUNT_NATS_STREAM`, `ACCOUNT_NATS_CA_FILE`,
  `ACCOUNT_NATS_CERT_FILE`, `ACCOUNT_NATS_KEY_FILE`, `ACCOUNT_TLS_CERT_FILE`,
  `ACCOUNT_TLS_KEY_FILE`, `ACCOUNT_TLS_CA_FILE`, `ACCOUNT_NATS_CREDS`, `ACCOUNT_NATS_CA_PEM`,
  `ACCOUNT_NATS_CERT_PEM`, `ACCOUNT_NATS_KEY_PEM`, `ACCOUNT_TLS_CERT_PEM`, `ACCOUNT_TLS_KEY_PEM`,
  `ACCOUNT_TLS_CA_PEM`, `NOTIFICATION_SERVICE_URL`, `NOTIFICATION_SERVICE_TOKEN`,
  `NOTIFICATION_CERT_SHA256`, `NOTIFICATION_D11_TOKEN`, `ACCOUNT_CURSOR_KEY`,
  `MAX_AVATAR_SIZE_BYTES`, `MAX_BANNER_SIZE_BYTES`.

### Configuration files

`DB_BUDGET_FILE` and `EXPECTED_SCHEMA_FILE` select database budget and expected-schema
JSON documents. Event consumers also read the document selected by
`DELIVERY_CONFIG_FILE`. The corresponding `DB_BUDGET_DOCUMENT`,
`EXPECTED_SCHEMA_DOCUMENT` and `DELIVERY_CONFIG_DOCUMENT` inputs provide inline
forms, with the corresponding `SHA256` inputs binding document contents.

Service documents are selected by `DISCOVERY_RUNTIME_CONFIG_FILE`,
`NETWORK_AUTHORITY_CONFIG_FILE`, `STREAM_CONSUMER_CONFIG_FILE`,
`STREAM_RELAY_CONFIG_FILE`, `ACCOUNTING_READ_CONFIG_FILE`, `IDENTITY_CONFIG_FILE`,
`GATEWAY_RUNTIME_FILE`, `INDEXER_LIFECYCLE_CONFIG_FILE`,
`INDEXER_RECEIPT_CONFIG_FILE`, `PNL_JOURNEY_CONFIG_FILE`, `RANKING_RUNTIME_FILE`,
`CHART_FEATURES_CONFIG_FILE`, `LIVESTREAM_BUDGET_POLICY_FILE`,
`METADATA_PUBLICATION_CONFIG_FILE`, `SOCIAL_RUNTIME_FILE` and
`ACCOUNT_SUBJECTS_FILE`. Their loaders define the accepted document schemas,
inline forms and digest requirements. Certificate, key and NATS credential files
are selected by the corresponding file inputs in those loaders.

The shared deployment configuration embeds
[current.json](../../sdk/launchpad/shared/config/deployment/current.json) and
[history/](../../sdk/launchpad/shared/config/deployment/history/) through its
[manifest loader](../../sdk/launchpad/shared/config/deployment/manifest.go).
The [discovery runtime](discovery/runtime.go),
[notification runtime](notifications/runtime.go) and
[private control runtime](security/control/providers.go) accept configuration
through Go options supplied by their callers.

Query generation reads [indexer/sqlc.yaml](indexer/sqlc.yaml),
[livestream/sqlc.yaml](livestream/sqlc.yaml),
[metadata/sqlc.yaml](metadata/sqlc.yaml), [social/sqlc.yaml](social/sqlc.yaml) and
[user/sqlc.yaml](user/sqlc.yaml).

## Documentation

- [Market and chain launch](../../docs/launchpad/chain-launch.md).
- [Launchpad web configuration checklist](../../docs/launchpad/production-checklist.md).
- [Launchpad documentation site page](../../docs/site/paxeer-docs/public/markdown/launchpad.md).
- [Unified network guide](../../docs/wiki/UnifiedNetwork.md).
- [Deployment guide](../../deploy/launchpad/README.md).
- [Livestream service](livestream/PROGRESS.md).
- [Private control primitives](security/control/README.md).
- [Supply-chain tools](security/supplychain/README.md).
- [License notices](licenses/README.md).
