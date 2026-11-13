<!-- Source: https://docs.paxeer.app/docker/ -->

# Docker Development

Local Docker clusters, state-sync RPC nodes, and monitoring for Paxeer X development and testing.

**Source:** `docker/`, `Makefile`, `chain.mk` (monorepo root)

## Containers for the complete Paxeer X stack

Chain development containers cover EVM and Cosmos execution. The monorepo also provides images for the kernel, hosted gateway and core, receipt authority, registry, agent boundary, Human services, bridge relayer, and application surfaces. There is no `docker/relay-archive/` image; relay/archive is retired (see [relay/archive status](https://docs.paxeer.app/relay-archive)).

| Image source | Role |
| --- | --- |
| `docker/platform-node/` | Hosted node, genesis tooling, bootstrap, and supervisor |
| `docker/platform-gateway/` and `docker/platform-core/` | Public receipt-verifying ingress and internal core boundary |
| `docker/platform-authority/` and `docker/platform-agent-boundary/` | Receipt authority and agent authorization boundary |
| `docker/platform-registry/` and `docker/human-service/` | Discovery registry and Human service components |
| `docker/bridge-relayer/` | Bridge delivery |

Hosted manifests under `services/platform/hosted/` define the service topology, persistent storage, probes, and infrastructure boundaries. The chain Docker Compose files below run their declared development topology; assembling the full product also requires the kernel, gateway, registry, and user-service dependencies.

Mount persistent state separately from runtime sockets and credentials. Pin image revisions and matching genesis artifacts. Publish the gateway through TLS ingress and keep core administration, signer channels, and node-local listeners private. Check `/readyz` and `/livez` where the selected service defines them, then verify an application receipt and its settlement lifecycle. See [operator roles](https://docs.paxeer.app/operators) and [settlement](https://docs.paxeer.app/settlement).

## Prerequisites

### macOS

Install Docker Desktop from [docs.docker.com/desktop/install/mac-install/](https://docs.docker.com/desktop/install/mac-install/)

### Ubuntu

Install Docker Engine and Docker Compose:

- Docker: [docs.docker.com/engine/install/ubuntu/](https://docs.docker.com/engine/install/ubuntu/)

- Docker Compose: [docs.docker.com/compose/install/other/](https://docs.docker.com/compose/install/other/)

## Four-Node Local Cluster

The standard development setup runs a four-validator Paxeer cluster on a private network (`192.168.10.0/24`).

### Start Cluster

```
# Build and start (first time or after code changes)
make docker-cluster-start

# Quick start (skip build if paxd binary exists)
make docker-cluster-start-skipbuild
```

This launches four `pax-node-*` containers plus a fifth compose service, `x-websearch` (`pax-x-websearch`, image `pax-chain/x-websearch`). Genesis files and logs are generated under `build/generated/`.

### Node Ports

| Node | P2P | RPC | gRPC | EVM RPC | IP |
| --- | --- | --- | --- | --- | --- |
| node0 | 26656-26658 | 26657 | 9090-9091 | 8545-8546 | 192.168.10.10 |
| node1 | 26659-26661 | 26660 | 9092-9093 | 8547-8548 | 192.168.10.11 |
| node2 | 26662-26664 | 26663 | 9094-9095 | 8549-8550 | 192.168.10.12 |
| node3 | 26665-26667 | 26666 | 9096-9097 | 8551-8552 | 192.168.10.13 |

### Monitor Logs

```
# Tail logs for node 0
tail -f build/generated/logs/paxd-0.log

# View all logs
ls -l build/generated/logs/
```

### SSH Into Container

```
# List containers
docker ps -a

# SSH into a node
docker exec -it pax-node-0 /bin/bash
```

### Environment Variables

The cluster supports environment variable configuration:

| Variable | Purpose |
| --- | --- |
| `NUM_ACCOUNTS` | Number of test accounts to create |
| `SKIP_BUILD` | Skip binary rebuild |
| `INVARIANT_CHECK_INTERVAL` | State invariant check frequency |
| `UPGRADE_VERSION_LIST` | Chain upgrade version schedule |
| `MOCK_BALANCES` | Use mock balance data |
| `GIGA_EXECUTOR` | Enable Giga executor backend |
| `GIGA_OCC` | Enable optimistic concurrency control |
| `RECEIPT_BACKEND` | Receipt storage backend |
| `AUTOBAHN` | Enable Autobahn optimizations |
| `GIGA_STORAGE` | Enable Giga storage engine |
| `GIGA_MIGRATE_FROM_MEMIAVL` | Migrate from MEMIAVL to Giga |
| `GIGA_FLATKV_ONLY` | Use flat key-value storage only |

## Single Node (Not Recommended)

For minimal testing only:

```
make build-docker-node && make run-local-node
```

The four-node cluster is preferred for realistic consensus behavior.

## Compose Files

| File | Purpose |
| --- | --- |
| `docker-compose.yml` | Four-node cluster (base configuration) |
| `docker-compose.monitoring.yml` | Prometheus + Grafana monitoring overlay |
| `docker-compose.giga-mixed.yml` | Mixed Giga/legacy storage testing overlay |

## Monitoring with Prometheus & Grafana

Start the cluster with monitoring containers:

```
# Start cluster + monitoring together
make docker-cluster-start-monitoring

# Stop cluster + monitoring together
make docker-cluster-stop-monitoring
```

Or run monitoring scripts independently:

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

- **Grafana:** `http://localhost:3000` (login: `admin` / `admin`)

- **Prometheus:** `http://localhost:9090`

## State Sync RPC Node

Test state-sync by starting an additional RPC node that syncs from the four-node cluster:

```
# Prerequisite: Start a 4-node cluster
make docker-cluster-start

# Wait until block height exceeds 500 (configurable via app.toml)
paxd status | jq

# Start state-sync RPC node
make run-rpc-node
```

The RPC node bootstraps from a recent snapshot instead of replaying full history.

**Scripts:** `docker/rpcnode/scripts/`

## Fast Iteration & Local Development

Docker mounts local source directories, so you can edit code and rebuild without re-pulling dependencies:

1. Edit code at the monorepo root (modules, consensus, SDK, storage, WASM)
2. Rebuild the node image: `make build-docker-node`
3. Restart the cluster: `make docker-cluster-start`

No `go.mod` replacements or sibling repositories required. The monorepo includes all dependencies.

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

The cluster uses the `pax-chain/localnode` image built from `docker/localnode/Dockerfile` (there is no root `Dockerfile`). Platform defaults to `linux/amd64` but can be overridden with `DOCKER_PLATFORM`.

## Network Configuration

The cluster creates a `localnet` bridge network with subnet `192.168.10.0/24`. Each node has a static IP for deterministic peer configuration.
