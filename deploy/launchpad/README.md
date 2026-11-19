# KindleLaunch service deployment

This directory holds service image builds, Railway deployment definitions,
deployment values and production topology, bootstrap, observability and recovery
tooling for the launchpad applications. The applications use Paxeer mainnet-beta with EVM chain ID `125` (`0x7d`). The
LayerX kernel inside `paxd` executes and orders each transaction and builds each
block; validators check and commit it. A receipt is final when its block is
committed.
Sub-chains are tenant domains in the database. The deployment values select node
root JSON-RPC endpoints and public web origins; the image registry is supplied
through `REGISTRY`.

## Layout

- [Dockerfile.svc](Dockerfile.svc) builds admitted Go service/binary pairs from [services/launchpad/](../../services/launchpad/) with `SERVICE` and `BIN`.
- [Dockerfile.native](Dockerfile.native) builds native targets, including the metadata Railway service.
- [Dockerfile.web](Dockerfile.web) builds the launchpad pnpm workspace and Next.js standalone web server with public configuration supplied as build arguments.
- [Dockerfile.svc.dockerignore](Dockerfile.svc.dockerignore) excludes local dependencies, build output and private files from the Go service build context.
- [Dockerfile.native.dockerignore](Dockerfile.native.dockerignore) supplies the native image build context exclusions.
- [Dockerfile.web.dockerignore](Dockerfile.web.dockerignore) supplies the web image build context exclusions.
- [deploy.sh](deploy.sh) builds one Go image and pushes it when `REGISTRY` is set.
- [web-build-env.sh](web-build-env.sh) prepares the web build environment and unsets empty public and upstream build arguments so application defaults apply.
- [docker-compose.strangler.yml](docker-compose.strangler.yml) declares the Go service stack with immutable image inputs, environment files and read-only startup-document mounts.
- [railway-values.json](railway-values.json) supplies mainnet-beta RPC, public web origins and web upstream configuration.
- [railway/](railway/) contains service Dockerfile selections, source watch paths, readiness paths, restart policies and the [variable contracts](railway/variables.json).
- [production/](production/) contains [bootstrap fixtures](production/bootstrap/README.md), [topology declarations](production/topology/README.md), [observability tooling](production/observability/README.md) and [recovery tooling](production/recovery/README.md).
- [tests/](tests/) contains the Railway deployment values test.
- [documents/](documents/) contains the startup-document composer and reviewed values layout.

## Build and test

Run from the repository root:

```bash
docker build -f deploy/launchpad/Dockerfile.svc \
  --build-arg SERVICE=indexer \
  --build-arg BIN=indexerd \
  -t launchpad-indexer:dev .
```

The Go Dockerfiles validate the service/binary pair, use the pinned Go toolchain
and images, verify module checksums, and build with read-only module resolution.
Their runtime runs as UID/GID `65532:65532` with `/app/app` as its entry point.
The Go service deployment definitions use port `8080`.
The service mappings in `railway/variables.json` identify the configured pairs:
`charts` maps to `SERVICE=trading-charts`, `BIN=chartsd`. Build metadata and web
images with their respective Dockerfiles:

```bash
docker build -f deploy/launchpad/Dockerfile.native \
  --build-arg SERVICE=metadata --build-arg BIN=metadatad .
docker build -f deploy/launchpad/Dockerfile.web \
  --build-arg WEB_API_UPSTREAM --build-arg WEB_CHARTS_UPSTREAM \
  --build-arg WEB_GATEWAY_UPSTREAM --build-arg WEB_SOCKETS_UPSTREAM .
```

Export the four upstream variables before the web build command. Supply the
deployment's public configuration through additional `--build-arg` options using
the names listed in `Dockerfile.web` and `railway/variables.json`. The build
installs with pnpm's frozen lockfile, builds the SDK packages, then builds
`@sidiora/web`. Its runtime starts `web/server.js` on port `3000`.

The service build script provides the same Go image build and optional registry
publication:

```bash
deploy/launchpad/deploy.sh indexer indexerd
```

With `REGISTRY` set, the script builds `${REGISTRY}/launchpad-indexer:${TAG}` and
invokes `docker push`; otherwise it builds a local image. `TAG` defaults to `dev`.
Its optional `<group>/<svc>` form changes the image slug and passes the part
after the first slash as `SERVICE`. The script builds and optionally publishes
the image; Railway service deployment and runtime configuration use the Railway
definitions separately.

Run the deployment values test from the repository root:

```bash
bash deploy/launchpad/tests/railway-values.test.sh
```

This script checks the variable plan against a local replay, the unresolved
variable names and agreement between web values and Dockerfile build arguments.
The [bootstrap](production/bootstrap/README.md),
[observability](production/observability/README.md) and
[recovery](production/recovery/README.md) pages describe their separate commands
and prerequisites. The Compose service stack obtains PostgreSQL, Redis and
broker connections from its environment files; bootstrap provides isolated local
dependencies. Production capacity is evaluated from measurements by the
production tooling.

## Configuration

`deploy.sh` reads `REGISTRY` and `TAG` and takes the service and binary as
positional arguments. The Go Dockerfiles declare `SERVICE`, `BIN`, `BUILD_IMAGE`,
`RUNTIME_IMAGE` and `SOURCE_DATE_EPOCH`; the web Dockerfile declares `NODE_IMAGE`.

`railway/variables.json` lists each service's `buildArgs`, `fixed`, `required` and
`secrets` contracts, including the runtime configuration document and credential
variable names. `railway-values.json` supplies the shared `RPC_URL`,
`LIVEPEER_API_URL` and web deployment inputs. The RPC inputs `RPC_URL`,
`NEXT_PUBLIC_RPC_URL`, `NEXT_PUBLIC_PAXEER_RPC_URL` and
`NEXT_PUBLIC_RPC_URL_FALLBACK` select node root URLs.

### Livestream deployment forms

The provider form uses `railway/variables.json` and `railway-values.json`:
`RUNTIME_PROFILE=container`, `LIVEPEER_PROVIDER_MODE=available` and
`LIVESTREAM_CONNECTED_ENABLED=true`. It needs the Livepeer API and webhook
credentials, connected budget policy, identity mTLS and signing credentials,
vault transit, ingest, playback, viewer-token and audit settings listed in
`required`. The web build reads `NEXT_PUBLIC_LIVESTREAM_API_URL` as
`https://live.sidiora.fun`; its gateway URL stays `https://gateway.sidiora.fun`.
The private listener settings are empty in this form.

The form without a broadcast provider uses
[`railway/variables.platform.json`](railway/variables.platform.json) and
[`railway-values.platform.json`](railway-values.platform.json). This manifest
contains the livestream and web entries; other services use their regular
entries. It uses the same `fixed`, `required`, `secrets`, `optional`, `forms`
and `documentSources` fields as the regular manifest. Empty fixed settings
clear inputs absent from that form. Select it with the variable apply tool's
`--manifest` option and its matching `--values` file.

That form fixes `RUNTIME_PROFILE=platform`,
`LIVEPEER_PROVIDER_MODE=unavailable` and `LIVESTREAM_CONNECTED_ENABLED=false`.
It needs PostgreSQL, Redis, the shared startup documents,
`LIVESTREAM_VIEWER_TOKEN_KEY` of at least 32 bytes, web CORS origins and the
gateway client certificate's lowercase SHA-256 digest in
`GATEWAY_PEER_CERT_SHA256`. The contract selects PEM certificate, private key
and client CA settings (`LIVESTREAM_TLS_CERT_PEM`, `LIVESTREAM_TLS_KEY_PEM`,
`LIVESTREAM_TLS_CLIENT_CA_PEM`), with their `*_FILE` alternatives empty.
The runtime requires exactly one source per certificate, key and CA. It serves
the stream API on `LIVESTREAM_PRIVATE_LISTEN_ADDRESS=[::]:8443`, using mTLS and
the pinned gateway certificate; port `8080` serves health and metrics. The
gateway runtime's livestream upstream uses HTTPS on that private port.
The web build reads equal `NEXT_PUBLIC_LIVESTREAM_API_URL` and
`NEXT_PUBLIC_GATEWAY_URL` values, both `https://gateway.sidiora.fun`. Public
stream reads, viewer sessions and heartbeats use the lifecycle store;
provider-bound actions return `BROADCAST_NOT_PROVISIONED`.

For the document composer, use `documents/values/livestream.example.json` for
the provider form or copy `documents/values/livestream.platform.example.json`
to the values directory as `livestream.json` for the form without a provider.
For that form also copy `documents/values/gateway.platform.example.json` as
`gateway.json`; it selects the private livestream upstream.
The `environment.RUNTIME_PROFILE` selects the contract; `web` supplies the two
web build inputs. Compose `--service livestream --service gateway --service web`
to obtain all three outputs. The composer refuses a platform stream URL that
differs from its gateway URL, a provider form without connected configuration,
or a platform gateway upstream using another port. Supply deployment-issued
certificates, certificate digests and credentials in place of example material.

### Object-store account rights

[object-store/accounts.json](object-store/accounts.json) records the object
actions and key prefixes used by the gateway, user and metadata service
accounts. Its source-call table is checked against the production service
trees, so a newly added object call requires a corresponding right.

Render a provider policy with the contract bucket names supplied in order:

```bash
python3 deploy/launchpad/object-store/policy.py gateway media-approved media-user media-og
```

The command reads only the reviewed account document, prints an allow-only JSON
policy, and rejects unknown services, missing or surplus bucket arguments, and
bucket names outside lowercase letters, digits and hyphens.

### API network authority document

The API composer emits `ACCOUNTING_READ_CONFIG_DOCUMENT`,
`STREAM_RELAY_CONFIG_DOCUMENT` and `DISCOVERY_RUNTIME_CONFIG_DOCUMENT` from
the API values. The authority command below writes
`network-authority.json`; set `network_authority_rendered_file` and
`network_authority_runtime_path` in the API values so the composer emits
`NETWORK_AUTHORITY_CONFIG_FILE` and its `NETWORK_AUTHORITY_CONFIG_SHA256`.

Generate `network-authority.json` from the repository root with:

```bash
(cd services/launchpad/api && go run ./cmd/api-runtime-docs \
  -repo ../../.. \
  -source-schema "$INDEXER_SOURCE_SCHEMA" \
  -endpoints "$PUBLIC_ENDPOINTS_FILE" \
  -pool-runtime "$REVIEWED_POOL_RUNTIME" \
  -out "$AUTHORITY_OUTPUT_DIR")
```

Use absolute paths for `PUBLIC_ENDPOINTS_FILE` and `AUTHORITY_OUTPUT_DIR`.
`-repo` selects the repository root. `-source-schema` names the indexer's
committed-head schema: a lowercase letter followed by up to 62 lowercase
letters, digits or underscores. `-pool-runtime` is the reviewed, nonzero
`0x`-prefixed 32-byte keccak256 hash of the deployed pool runtime.
`-endpoints` reads one JSON object containing the reviewed public endpoints
and their exact allowlists, for example:

```json
{
  "RPC": "https://api1.mainnet-beta.paxeer.network",
  "Explorer": "https://paxscan.paxeer.app",
  "AllowedRPCs": ["https://api1.mainnet-beta.paxeer.network"],
  "AllowedExplorers": ["https://paxscan.paxeer.app"]
}
```

Endpoints must use HTTPS without credentials, queries or fragments and must
match their respective allowlists. Each allowlist contains 1 to 16 entries.
The command uses `deployment.Current()` for the manifest and both address maps,
reads the serial interface digest from task `1.99`'s `interface_bundle.expected_sha256`
in `tools/launchpad/production/bootstrap/check-registry.json`, and derives the
expected revision through the preflight engine's revision calculation. Generation
needs no network connection or database. Identical inputs produce identical bytes.
The output directory is created with mode `0750` and the document with mode `0600`.
The command prints `<sha256> network-authority.json <byte-count>`.

Set `NETWORK_AUTHORITY_CONFIG_FILE` to the mounted document path, or set
`NETWORK_AUTHORITY_CONFIG_DOCUMENT` to standard base64 encoding of its exact
bytes. Set only one source. In either case, set
`NETWORK_AUTHORITY_CONFIG_SHA256` to the printed lowercase SHA-256 digest.
`NETWORK_RPC_URL` supplies the API's HTTPS chain reader independently of the
public endpoints in the document. The API validates the document's schema,
manifest, reviewed runtime, revision digest and both address maps before use.

### Web build arguments

The web build arguments are:

- Backend origins: `NEXT_PUBLIC_DATA_API_URL`, `NEXT_PUBLIC_GATEWAY_URL`, `NEXT_PUBLIC_METADATA_API_URL`, `NEXT_PUBLIC_PNL_API_URL`, `NEXT_PUBLIC_STATS_API_URL`, `NEXT_PUBLIC_LIVESTREAM_API_URL`.
- RPC and wallet inputs: `NEXT_PUBLIC_RPC_URL`, `NEXT_PUBLIC_RPC_URL_FALLBACK`, `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_EXPLORER_API`, `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`.
- Application and stream inputs: `NEXT_PUBLIC_ACTION_ENVIRONMENT`, `NEXT_PUBLIC_AUTH_REDIRECT_URL`, `NEXT_PUBLIC_CANONICAL_ORIGIN`, `NEXT_PUBLIC_STREAM_ORIGIN`, `NEXT_PUBLIC_PRIVATE_API_ORIGIN`, `NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN`, `NEXT_PUBLIC_SOCIAL_WRITE_PREFIX`, `NEXT_PUBLIC_SOCIAL_WS_PATH`, `NEXT_PUBLIC_MARKET_SOCIAL_ORIGINS`, `NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE`.
- Telemetry inputs: `NEXT_PUBLIC_ANALYTICS_KEY`, `NEXT_PUBLIC_SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_ENV`, `NEXT_PUBLIC_SENTRY_RELEASE`, `NEXT_PUBLIC_SENTRY_REPLAY_ERROR_SAMPLE_RATE`, `NEXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE`.
- Server upstream origins: `WEB_API_UPSTREAM`, `WEB_CHARTS_UPSTREAM`, `WEB_GATEWAY_UPSTREAM`, `WEB_SOCKETS_UPSTREAM`.

`web-build-env.sh` unsets empty `NEXT_PUBLIC_*` and `WEB_*` variables so application
defaults apply. The build stage of `Dockerfile.web` sets `NODE_OPTIONS` to
`--max-old-space-size=4096`, a heap bound of 4096 megabytes.
The web runtime sets `NODE_ENV`, `NEXT_TELEMETRY_DISABLED`, `HOSTNAME`
and `PORT`.

`docker-compose.strangler.yml` reads these service-specific image, environment
file and startup-document directory variables:

| Service | Image | Environment file | Startup documents |
| --- | --- | --- | --- |
| api | `API_IMAGE` | `API_ENV_FILE` | `API_EVIDENCE_DIR` |
| sockets | `SOCKETS_IMAGE` | `SOCKETS_ENV_FILE` | `SOCKETS_EVIDENCE_DIR` |
| indexer | `INDEXER_IMAGE` | `INDEXER_ENV_FILE` | `INDEXER_EVIDENCE_DIR` |
| charts | `CHARTS_IMAGE` | `CHARTS_ENV_FILE` | `CHARTS_EVIDENCE_DIR` |
| stats | `STATS_IMAGE` | `STATS_ENV_FILE` | `STATS_EVIDENCE_DIR` |
| pnl | `PNL_IMAGE` | `PNL_ENV_FILE` | `PNL_EVIDENCE_DIR` |
| ranking | `RANKING_IMAGE` | `RANKING_ENV_FILE` | `RANKING_EVIDENCE_DIR` |
| gateway | `GATEWAY_IMAGE` | `GATEWAY_ENV_FILE` | `GATEWAY_EVIDENCE_DIR` |
| user | `USER_IMAGE` | `USER_ENV_FILE` | `USER_EVIDENCE_DIR` |
| social | `SOCIAL_IMAGE` | `SOCIAL_ENV_FILE` | `SOCIAL_EVIDENCE_DIR` |
| metadata | `METADATA_IMAGE` | `METADATA_ENV_FILE` | `METADATA_EVIDENCE_DIR` |
| livestream | `LIVESTREAM_IMAGE` | `LIVESTREAM_ENV_FILE` | `LIVESTREAM_EVIDENCE_DIR` |

The Go service definitions set `NODE_ENV`, `RUNTIME_PROFILE`, `BIND_ADDRESS`,
`PORT`, `SECRETS_PROVIDER` and `SECRETS_REFRESH_SECONDS`; API definitions also set
`API_ROLE`. The Railway definitions select readiness checks: the API uses
`/health/ready` and the web uses `/api/health`.

The connected API, ranking, user, metadata and social services keep their health
listener on `PORT=8080` and serve their private mTLS routes on
`API_PRIVATE_LISTEN_ADDRESS`, `RANKING_PRIVATE_LISTEN_ADDRESS`,
`USER_PRIVATE_LISTEN_ADDRESS`, `METADATA_PRIVATE_LISTEN_ADDRESS` and
`SOCIAL_PRIVATE_LISTEN_ADDRESS`, each set to `[::]:8443` in the Railway
contract to accept both IPv4 and IPv6 connections. The Compose file keeps
`0.0.0.0:8443` for its IPv4 network. The user and social services reach the
API's internal notification routes at that private API address. Gateway upstreams for user, metadata and
social use their corresponding HTTPS `:8443` origins; the ranking runtime's
social visibility endpoint does the same.

Metadata uses `METADATA_INGRESS=mtls`. Its private listener requires TLS 1.3
and a certificate verified by `METADATA_TLS_CLIENT_CA_PEM`. Set
`METADATA_TLS_CERT_PEM`, `METADATA_TLS_KEY_PEM` and
`METADATA_TLS_CLIENT_CA_PEM` to the listener's PEM materials. Each also admits
its `*_FILE` alternative outside the inline platform contract; select exactly
one source for each material. Key files must be regular and accessible only
to their owner. The inline key and `METADATA_ASSERTION_PRIVATE_KEY_HEX` are
secrets resolved through the configured secrets provider.

The gateway channel uses `GATEWAY_ASSERTION_PUBLIC_KEY_HEX` and
`GATEWAY_PEER_CERT_SHA256`; the user account channel uses
`USER_ASSERTION_PUBLIC_KEY_HEX` and `USER_PEER_CERT_SHA256`. Metadata calls
`IDENTITY_ENDPOINT` over mTLS using its listener certificate and assertion key,
with `IDENTITY_PEER_CERT_SHA256` pinning the identity server. The separate
`edge` form uses `IDENTITY_SERVICE_URL` and `IDENTITY_SERVICE_TOKEN` for session
introspection and requires database moderation. The platform contract selects
the mTLS form and database moderation.

Production tooling has separate configuration described in the linked pages:
bootstrap uses `compose.yml`, `fixture.env.example`, `images.json` and
`nats.conf`; topology uses `production-input.template.json`; observability uses
`config.json`, `config-v2.json`, `config-v4.schema.json`, `otel-collector.yaml`,
`prometheus-rules.json` and `grafana-dashboard.json`; recovery uses
`config.example.json`, `resources.json` and `epoch.sql`.

## Documentation

- [Launchpad web configuration checklist](../../docs/launchpad/production-checklist.md).
- [Launch a market and chain](../../docs/launchpad/chain-launch.md).
- [Launchpad documentation site page](../../docs/site/paxeer-docs/public/markdown/launchpad.md).
- [Public node endpoint reference](../../docs/site/paxeer-docs/public/markdown/public-rpc.md).

### History below the indexer anchor

Set `history_from_height` in the indexer values to ingest historical markets
and events through the anchor before live ingestion. The height is inclusive
and must be between the receipt deployment manifest's activation height and
`anchor_height`, inclusive. Supply `history_operator` (the operator's issued
identity, using letters, digits, underscores or hyphens) and
`history_expires_at` (an RFC3339 timestamp with a timezone). For example:

```json
{
  "history_from_height": 37220320,
  "history_operator": "launchpad_operator",
  "history_expires_at": "2027-01-01T00:00:00Z",
  "backfill_batch_size": 32
}
```

Use the actual anchor height and hash of the deployment. For an approved
historical deployment, supply its rendered receipt document with
`receipt_rendered_file` and `receipt_runtime_path`; its manifest determines
the activation bound. For current values without a rendered receipt, history
uses the repository's current deployment manifest. The optional member is the
operator's explicit selection: without it the composer emits the existing
documents without historical requests, authorizations or mode variables.

The composer writes ordered `backfillRequests` in the lifecycle document,
splitting the inclusive range into requests of at most 1,000,000 heights
(up to 16 requests per document). Each request pins the same anchor checkpoint.
Equal values produce equal job and authorization identities. The composer
sets `INDEXER_MODE=backfill`, `BACKFILL_ONLY=false`, `BACKFILL_FROM_BLOCK` and
`BACKFILL_BATCH_SIZE`. The optional `backfill_batch_size` is 1–64; when omitted
it uses `replay_batch_size`. These variables are optional indexer names in
`railway/variables.json`.

Before starting the indexer, the migration role applies the adjacent
`indexer-history-authorizations.sql` file:

```bash
psql "$MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f "$DOCUMENTS_DIR/indexer-history-authorizations.sql"
```

It contains one scoped authorization per request, binding the operator,
backfill action, indexer target, generation, projection revision, height range,
evidence digest and expiry. Reapplying the same file changes no rows. Use an
expiry that covers the full history run and any restart. The runtime reads
authorizations; only the migration role applies the statement file. An absent,
expired, revoked or mismatched authorization refuses the request. Requests of
another canonical stream and requests exceeding the height bound are refused.

For rendered lifecycle inputs, mount the composer's adjacent
`indexer-lifecycle.json` at `lifecycle_runtime_path`: it contains the historical
requests and its digest is the composed `INDEXER_LIFECYCLE_CONFIG_SHA256`.
The rendered receipt bytes retain their original digest.

Set the four optional service variables from the composed `indexer.json`
alongside its document variables. Start the service once with those variables
and documents after the
statement file is applied. It processes the historical requests in order,
persisting progress per request, then starts live ingestion at anchor + 1 in
the same process. Live source readiness polling starts with live ingestion, so
bootstrap history can finish before live-lag admission applies. A restart
with the same documents resumes unfinished
requests and preserves the live watermark. `BACKFILL_ONLY=true` instead exits
after history; a later start with `INDEXER_MODE=live` uses the same documents
and continues from the persisted live watermark.

An indexer backfill writes canonical creations, token transfers, accounting
fills, typed market/event read projections, block completeness and durable
canonical delivery intents in the same transaction as replay progress and
replay effects. Historical completion does not move the live stream watermark.
A regular `replay` request records immutable raw source and isolated replay
effects and deliveries; `reconstruct` uses the raw archive for an isolated
generation. Those modes do not populate the canonical creation or market
projections and do not move the live watermark.

There is no configured number of completed backfill heights per second.
One historical request runs batches sequentially: completed heights per second
are the batch size divided by qualified source-read plus transaction time.
Source reads independently verify committed blocks and receipt evidence;
RPC latency, receipts and pinned contract reads, response bounds, retries,
database work and historical admission pauses determine that time.
`BACKFILL_BATCH_SIZE` bounds a backfill batch to 64 heights; the lifecycle's
`ReplayBatchSize` bounds regular replay batches. `INDEXER_HISTORICAL_RPC_URL`
selects an archive for pruned history. `INDEXER_HISTORICAL_CONCURRENCY` (1–96,
default 48 per archive client), `INDEXER_HISTORICAL_READAHEAD` (0–1024, default
256) and `INDEXER_HISTORICAL_ATTEMPT_TIMEOUT_MS` (500–15000, default 8000)
control archive reads. Primary historical reads share 16 process permits and
four bulk permits per endpoint; archive endpoints have their own permits.
`INDEXER_CANONICAL_READ_TIMEOUT_MS` bounds a replay batch read; lifecycle
`BulkDeadline` and receipt `Timeout` also bound reads. Live lag can pause
historical admission. `BACKFILL_CONCURRENCY`, `WorkConcurrency`, `WorkBurst`,
`EventsPerSecond` and the live poll interval do not set a backfill heights-per-
second rate.
