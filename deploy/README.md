# Deployment configuration

This directory holds service rollout manifests, the perps Railway project
definition, and launchpad image builds and deployment tooling. These services
support the single Paxeer mainnet-beta network: `paxd` embeds the kernel in
block proposal and execution, and receipts become final when their blocks
commit. Deployment manifests describe service configuration and rollout order.

## Layout

- [launchpad/](launchpad/README.md) contains Dockerfiles, Railway service
  definitions, configuration tests, and production bootstrap, topology,
  observability and recovery tooling.
- [rollout.kvx](rollout.kvx) is the default service rollout manifest read by
  [tools/rollout/rollout.sh](../tools/rollout/rollout.sh).
- [rollout-launchpad.kvx](rollout-launchpad.kvx) declares launchpad image
  rollouts, service readiness paths and deployment waves.
- [rollout-perps.kvx](rollout-perps.kvx) declares perps image rollouts, service
  readiness paths and deployment waves.
- [railway-perps-project.kvx](railway-perps-project.kvx) declares the perps
  Railway project services, variables, volumes and deployment order, consumed
  by [railway-perps-project.py](../tools/deploy/railway-perps-project.py).
- [.gitignore](.gitignore) excludes private host configuration and rollout
  ledger files.

## Build and test

The files at this directory's root are deployment data files with no build of
their own. Run the following launchpad commands from the repository root.

Build a service image with [deploy.sh](launchpad/deploy.sh), leaving `REGISTRY`
unset for a local build:

```sh
deploy/launchpad/deploy.sh indexer indexerd
```

With `REGISTRY` set, the same script also pushes the image. `TAG` selects its
tag. The script uses [Dockerfile.svc](launchpad/Dockerfile.svc); the separate
[Dockerfile.native](launchpad/Dockerfile.native) and
[Dockerfile.web](launchpad/Dockerfile.web) define native-service and web builds.

Run the Railway values and web build-argument checks with the checked-in test
script:

```sh
bash deploy/launchpad/tests/railway-values.test.sh
```

The [bootstrap guide](launchpad/production/bootstrap/README.md) describes the
local dependencies used by integration checks. The
[observability guide](launchpad/production/observability/README.md) and
[recovery guide](launchpad/production/recovery/README.md) give their check
commands and prerequisites.

## Configuration

- The rollout driver reads `ROLLOUT_MANIFEST`, `RAILWAY_API_TOKEN`,
  `RAILWAY_PROJECT_ID`, `ROLLOUT_HOSTS_FILE`, `REGISTRY_TOKEN_FILE`,
  `GHCR_ACTOR`, `ROLLOUT_TIMEOUT`, `ROLLOUT_POLL`, `ROLLOUT_DRY_RUN` and
  `ROLLOUT_LOG_DIR`. Its ledger writer reads `ROLLOUT_LEDGER_DIR` and
  `ROLLOUT_EVIDENCE_ROOT`. The default manifest is `rollout.kvx`;
  `ROLLOUT_MANIFEST` selects either application rollout manifest.
- The perps project tool reads [railway-perps-project.kvx](railway-perps-project.kvx)
  and the service configuration files under
  [services/platform/perps-data/deploy/railway/](../services/platform/perps-data/deploy/railway/).
  Its environment inputs include `RAILWAY_API_TOKEN`, `RAILWAY_WORKSPACE_ID`,
  `GHCR_USERNAME`, `GHCR_TOKEN`, `PERPS_POSTGRES_PASSWORD`,
  `PERPS_NATS_CLUSTER_PASSWORD`, `PERPS_NATS_INDEXER_PASSWORD`,
  `PERPS_NATS_GATEWAY_PASSWORD`, `PERPS_AUTH_SECRET`, `PERPS_COMET_URLS` and
  `PERPS_RPC_URLS`.
- Launchpad image publication reads `REGISTRY` and `TAG`; service builds take
  `SERVICE` and `BIN` arguments. [railway/variables.json](launchpad/railway/variables.json)
  lists each service's build arguments, fixed variables, required variables and
  secret variable names. [railway-values.json](launchpad/railway-values.json)
  supplies application values. [railway/](launchpad/railway/) holds service
  deployment definitions, and
  [docker-compose.strangler.yml](launchpad/docker-compose.strangler.yml)
  reads service image references and environment-file and configuration-mount
  inputs. See the [launchpad deployment page](launchpad/README.md) for runtime
  configuration details.

## Documentation

- [Monorepo layout](../docs/MONOREPO.md)
- [Network overview](../docs/paxeer-network.md)
- [Launchpad deployment](launchpad/README.md)
- [Production topology](launchpad/production/topology/README.md)
- [Launchpad documentation site page](../docs/site/paxeer-docs/src/app/launchpad/page.tsx)
- [Public RPC documentation site page](../docs/site/paxeer-docs/src/app/public-rpc/page.tsx)
