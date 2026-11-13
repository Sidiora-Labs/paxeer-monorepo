# deploy/launchpad/ — KindleLaunch service deployment

This directory contains service image builds, Railway deployment definitions,
and production qualification tooling for the launchpad applications. Railway
host wiring is checked in: [railway-values.json](railway-values.json) supplies
mainnet-beta RPC and public web origins, and [railway/](railway/) contains the
service deployment definitions. The image registry remains an input to
[deploy.sh](deploy.sh) through `REGISTRY`.

The applications use one Paxeer network: `paxd` includes the kernel in block
execution and proposal, EVM chain ID is `125` (`0x7d`), and a receipt is final
when its block is committed. Sub-chains are tenant domains in the database.
See the [public node endpoint reference](../../docs/site/docs/reference/public-rpc.md)
for the JSON-RPC endpoints at the node root and their method contracts.

## Layout

- [Dockerfile.svc](Dockerfile.svc) builds admitted Go service/binary pairs from
  `services/launchpad/` with `SERVICE` and `BIN`. The runtime runs as UID/GID
  `65532:65532` with `/app/app` as its entry point.
- [Dockerfile.native](Dockerfile.native) builds the separately admitted native
  targets; the metadata Railway service uses this Dockerfile.
- [Dockerfile.web](Dockerfile.web) builds the launchpad pnpm workspace and Next.js
  standalone web server. Public configuration is supplied as build arguments;
  [web-build-env.sh](web-build-env.sh) prepares the build environment.
- [deploy.sh](deploy.sh) builds one Go image and pushes it when `REGISTRY` is set.
  `TAG` defaults to `dev`; an unset registry produces a local image only.
- [docker-compose.strangler.yml](docker-compose.strangler.yml) declares the Go
  service stack. It requires service-specific immutable image references,
  reviewed environment files and read-only startup-document mounts. PostgreSQL,
  Redis and broker connections come from those environments; the file does not
  provision those dependencies.
- [railway/variables.json](railway/variables.json) lists each service's
  `buildArgs`, `fixed`, `required` and `secrets` contracts.
- [production/bootstrap/](production/bootstrap/README.md) supplies isolated local
  dependencies. [Observability](production/observability/README.md) and
  [recovery](production/recovery/README.md) describe their checks, operator
  prerequisites and evidence requirements.

## Build one service image

Run from the repository root:

```bash
docker build -f deploy/launchpad/Dockerfile.svc \
  --build-arg SERVICE=indexer \
  --build-arg BIN=indexerd \
  -t launchpad-indexer:dev .
```

The build validates the service/binary pair, uses the pinned Go toolchain and
images, verifies module checksums, and builds with read-only module resolution.
The service mappings in `railway/variables.json` identify the configured pairs;
for example, `charts` maps to `SERVICE=trading-charts`, `BIN=chartsd`.

## Registry and Railway configuration

For an authorized image publication, supply the registry and release tag:

```bash
REGISTRY=registry.example/team TAG=release-tag \
  deploy/launchpad/deploy.sh indexer indexerd
```

The script builds `${REGISTRY}/launchpad-indexer:${TAG}` and invokes
`docker push`. Its optional `<group>/<svc>` form changes the image slug and
passes the part after the first slash as `SERVICE`. It does not deploy Railway
services or configure their runtime variables.

Each `railway/*.railway.json` selects a Dockerfile, source watch paths, readiness
path and restart policy. For example, [api.railway.json](railway/api.railway.json)
uses `/health/ready`; [web.railway.json](railway/web.railway.json) uses
`/api/health`. Supply the matching build arguments and all required runtime
variables from `railway/variables.json`, including the referenced private
configuration documents and credentials.

The checked-in `RPC_URL`, `NEXT_PUBLIC_RPC_URL` and
`NEXT_PUBLIC_PAXEER_RPC_URL` value is
`https://api5.mainnet-beta.paxeer.network`; the web fallback is
`https://api2.mainnet-beta.paxeer.network`. These are node root URLs. Public web
origins include `https://sidiora.fun`, `https://api.sidiora.fun` and
`https://gateway.sidiora.fun`. Configure each `WEB_*_UPSTREAM` with the
HTTP origin of its API, charts, gateway or sockets service; the service
containers listen on port 8080.

Review the JSON contracts when changing these values. Checked-in
configuration establishes neither deployed reachability nor production capacity.
