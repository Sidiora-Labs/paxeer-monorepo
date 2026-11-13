<!-- Source: https://docs.paxeer.app/docker/ -->

# Docker Development

Local Docker clusters, state-sync RPC nodes, and monitoring for Paxeer X development and testing.

**Source:** `docker/`, `Makefile`, `chain.mk` (monorepo root)

## Containers for the complete Paxeer X stack

Chain development containers cover EVM and Cosmos execution. The monorepo also provides images for the kernel, hosted gateway and core, receipt authority, registry, agent boundary, Human services, bridge relayer, and application surfaces. See [archive access](https://docs.paxeer.app/relay-archive) for retained evidence and mirrors.

| Image source | Role |
| --- | --- |
| `docker/localnode/` | Local chain node, genesis tooling and process supervision |
| `docker/platform-gateway/` and `docker/platform-core/` | Public receipt-verifying ingress and internal core boundary |
| `docker/platform-authority/` and `docker/platform-agent-boundary/` | Receipt authority and agent authorization boundary |
| `docker/platform-registry/` and `docker/human-service/` | Discovery registry and Human service components |
| `docker/bridge-relayer/` | Bridge delivery |

Hosted manifests under `services/platform/hosted/` define the service topology, persistent storage, probes, and infrastructure boundaries. The chain Docker Compose files below run their declared development topology; hosted services have their own configuration and dependencies.

Mount persistent state separately from runtime sockets and credentials. Pin image revisions and matching genesis artifacts. Publish the gateway through TLS ingress and keep core administration, signer channels, and node-local listeners private. Check `/readyz` and `/livez` where the selected service defines them, then verify an application receipt and its block-commit evidence. See [operator roles](https://docs.paxeer.app/operators) and [settlement](https://docs.paxeer.app/settlement).

## Prerequisites

### macOS

Install Docker Desktop from [docs.docker.com/desktop/install/mac-install/](https://docs.docker.com/desktop/install/mac-install/)

### Ubuntu

Install Docker Engine and Docker Compose:

- Docker: [docs.docker.com/engine/install/ubuntu/](https://docs.docker.com/engine/install/ubuntu/)

- Docker Compose: [docs.docker.com/compose/install/other/](https://docs.docker.com/compose/install/other/)

## Four-Node Local Cluster

The base Compose file declares four validators on the `localnet` bridge. Each local node process requires `LOCALNODE_DISPOSABLE=1` and the same fresh `LOCALNODE_RUN_ID`, a 16–128 character identifier starting with a letter or digit and containing only letters, digits, hyphens and underscores. Supply these through the container environment in your local deployment configuration.

### Cluster Recipes

```
# Cluster recipe: rebuild the localnode image and initialize node state
make docker-cluster-start

# Rebuild the image but reuse an existing paxd binary
make docker-cluster-start-skipbuild
```

The recipes invoke `docker/docker-compose.yml`, which declares four `pax-node-*` containers and `x-websearch` (`pax-x-websearch`). Set `X_WEBSEARCH_HOME` to its configured data directory. Image selection uses `LOCALNODE_IMAGE_TAG` and `X_WEBSEARCH_IMAGE_TAG`. The node coordinator stores genesis, node homes and logs under `build/localnode-runs/<run-id>/`. These recipes stop the cluster and remove `build/generated/` before starting.

### Published Host Ports

| Node | P2P | RPC | gRPC / gRPC-web | EVM RPC | Additional TCP port |
| --- | --- | --- | --- | --- | --- |
| node0 | 26656 | 26657 | 9090-9091 | 8545-8546 | 26658 |
| node1 | 26659 | 26660 | 9092-9093 | 8547-8548 | 26661 |
| node2 | 26662 | 26663 | 9094-9095 | 8549-8550 | 26664 |
| node3 | 26665 | 26666 | 9096-9097 | 8551-8552 | 26667 |

### Monitor Logs

```
# Tail logs for node 0
tail -f "build/localnode-runs/$LOCALNODE_RUN_ID/logs/paxd-0.log"

# View all logs
ls -l "build/localnode-runs/$LOCALNODE_RUN_ID/logs/"
```

### Open a Container Shell

```
# List containers
docker ps -a

# Open a shell in a node
docker exec -it pax-node-0 /bin/bash
```

### Environment Variables

The Compose file passes the following settings to the nodes. The Make cluster recipes set `NUM_ACCOUNTS=10`; the initialization script defaults to 5 when it is unset.

| Variable | Purpose |
| --- | --- |
| `NUM_ACCOUNTS` | Number of development accounts to create |
| `SKIP_BUILD` | Skip binary rebuild |
| `INVARIANT_CHECK_INTERVAL` | State invariant check frequency |
| `UPGRADE_VERSION_LIST` | Upgrade version list passed through the node environment |
| `MOCK_BALANCES` | Use mock balance data |
| `GIGA_EXECUTOR` | Enable Giga executor backend |
| `GIGA_OCC` | Enable optimistic concurrency control |
| `RECEIPT_BACKEND` | Receipt storage backend |
| `AUTOBAHN` | Enable Autobahn optimizations |
| `GIGA_STORAGE` | Use test_only_dual_write with the EVM state-store split |
| `GIGA_MIGRATE_FROM_MEMIAVL` | Start in memiavl_only mode for a migration scenario |
| `GIGA_FLATKV_ONLY` | Start in flatkv_only mode; cannot combine with GIGA_MIGRATE_FROM_MEMIAVL |

## Single Node (Not Recommended)

The single-node recipe builds its image and launches one container. Its node process requires the same explicit disposable-run environment described above:

```
make run-local-node
```

Use multiple validators when examining consensus behavior.

## Compose Files

| File | Purpose |
| --- | --- |
| `docker/docker-compose.yml` | Four-node cluster (base configuration) |
| `docker/docker-compose.monitoring.yml` | Prometheus + Grafana monitoring overlay |
| `docker/docker-compose.giga-mixed.yml` | Giga executor with OCC on node0 and standard execution on nodes1–3 |

## Monitoring with Prometheus & Grafana

The monitoring overlay requires `GRAFANA_ADMIN_PASSWORD_FILE` to name a private password file. These recipes start and stop the cluster with the monitoring overlay:

```
# Start cluster + monitoring together
make docker-cluster-start-monitoring

# Stop cluster + monitoring together
make docker-cluster-stop-monitoring
```

The standalone monitoring scripts use separate configuration under `docker/monitornode/`. Their Prometheus listener uses port 9091; select them separately from the Compose overlay, whose Prometheus host port is 9099:

```
# Start Prometheus
./docker/monitornode/scripts/start-prometheus.sh

# Start Grafana
./docker/monitornode/scripts/start-grafana.sh

# Stop
./docker/monitornode/scripts/stop-prometheus.sh
./docker/monitornode/scripts/stop-grafana.sh
```

### Access UIs

- **Grafana:** `3000`, using the configured password

- **Prometheus:** `9099` for the Compose overlay; `9091` for the standalone script

## State Sync RPC Node

RPC initialization requires `STATE_SYNC_RPC_SERVERS`, `STATE_SYNC_TRUST_HEIGHT` and `STATE_SYNC_TRUST_HASH` in the RPC container environment, plus a matching genesis at `build/generated/genesis.json`. It validates the selected checkpoint against the configured providers:

```
# Build the RPC-node image
make build-rpc-node

# RPC-node recipe, after supplying its container configuration
make run-rpc-node
```

State sync restores a trusted snapshot and then replays subsequent blocks. Select a retained snapshot height and matching 32-byte hexadecimal trust hash; there is no fixed 500-block startup threshold.

**Scripts:** `docker/rpcnode/scripts/`

## Fast Iteration & Local Development

The node containers mount the source tree and Go caches. The declared image and cluster recipes support this workflow:

1. Edit code at the monorepo root (modules, consensus, SDK, storage, WASM)
2. Rebuild the node image: `make build-docker-node`
3. Restart the cluster: `make docker-cluster-start`

The root `go.mod` declares module dependencies and replacements. External Go modules, base images and system libraries must also be available to the build.

### Volumes

The compose files mount:

- `$PROJECT_HOME` → `/pax-protocol/pax-chain` (source tree)

- `$GO_PKG_PATH/mod` → `/root/go/pkg/mod` (Go module cache)

- `$GOCACHE` → `/root/.cache/go-build` (Go build cache)

## Deployment Scripts

Each node type has a multi-step initialization flow:

### Local Node

- `step0_build.sh` — Build `paxd` binary

- `step1_configure_init.sh` — Initialize node configuration

- `step2_genesis.sh` — Generate genesis file

- `step3_add_validator_to_genesis.sh` — Add validator to genesis

- `step4_config_override.sh` — Override config files (ports, peers)

- `step5_start_pax.sh` — Start `paxd`

- `deploy.sh` — Orchestrates all steps

### RPC Node

- `step0_build.sh` — Build `paxd`

- `step1_configure_init.sh` — Initialize for state-sync

- `step2_start_pax.sh` — Start with state-sync enabled

- `deploy.sh` — Orchestrates RPC node setup

**Scripts:** `docker/localnode/scripts/`, `docker/rpcnode/scripts/`

## Docker Image

The `build-docker-node` target builds `docker/localnode/Dockerfile` and tags the image for local use and the Compose reference. `chain.mk` selects `linux/arm64` when `uname -m` reports arm64, otherwise `linux/amd64`. `DOCKER_PLATFORM` overrides that selection.

## Network Configuration

The base Compose file declares a `localnet` bridge with fixed per-node addresses for peer configuration. Its published host port ranges map to the same internal node ports; application clients select the appropriate published RPC port.
