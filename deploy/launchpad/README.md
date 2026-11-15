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

The web build arguments are:

- Backend origins: `NEXT_PUBLIC_DATA_API_URL`, `NEXT_PUBLIC_GATEWAY_URL`, `NEXT_PUBLIC_METADATA_API_URL`, `NEXT_PUBLIC_PNL_API_URL`, `NEXT_PUBLIC_STATS_API_URL`, `NEXT_PUBLIC_LIVESTREAM_API_URL`, `NEXT_PUBLIC_SOCIAL_API_URL`, `NEXT_PUBLIC_USER_API_URL`.
- RPC and wallet inputs: `NEXT_PUBLIC_RPC_URL`, `NEXT_PUBLIC_RPC_URL_FALLBACK`, `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_EXPLORER_API`, `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID`, `NEXT_PUBLIC_WALLET_IFRAME_ORIGIN`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`.
- Application and stream inputs: `NEXT_PUBLIC_ACTION_ENVIRONMENT`, `NEXT_PUBLIC_AUTH_REDIRECT_URL`, `NEXT_PUBLIC_CANONICAL_ORIGIN`, `NEXT_PUBLIC_STREAM_ORIGIN`, `NEXT_PUBLIC_PRIVATE_API_ORIGIN`, `NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN`, `NEXT_PUBLIC_SOCIAL_WRITE_PREFIX`, `NEXT_PUBLIC_SOCIAL_WS_PATH`, `NEXT_PUBLIC_MARKET_SOCIAL_ORIGINS`, `NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE`.
- Telemetry inputs: `NEXT_PUBLIC_ANALYTICS_KEY`, `NEXT_PUBLIC_SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_ENV`, `NEXT_PUBLIC_SENTRY_RELEASE`, `NEXT_PUBLIC_SENTRY_REPLAY_ERROR_SAMPLE_RATE`, `NEXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE`.
- Server upstream origins: `WEB_API_UPSTREAM`, `WEB_CHARTS_UPSTREAM`, `WEB_GATEWAY_UPSTREAM`, `WEB_SOCKETS_UPSTREAM`.

`web-build-env.sh` sets `NODE_OPTIONS` and unsets empty `NEXT_PUBLIC_*` and `WEB_*`
variables. The web runtime sets `NODE_ENV`, `NEXT_TELEMETRY_DISABLED`, `HOSTNAME`
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
