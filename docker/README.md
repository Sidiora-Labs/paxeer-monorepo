# Docker

This directory holds the Docker build contexts and compose files of the repository. Every subdirectory with a `Dockerfile` is the build context of one image: `localnode/` and `rpcnode/` for the local cluster below, `hpx-registry/` for the node registry described in [`tools/hpx/README.md`](../tools/hpx/README.md), and the other directories for the remaining node and service images (kernel, explorer, platform, wallet, interop, CI runner and others). `monitornode/` and `docker_compose_monitoring/` hold the Prometheus and Grafana setup. Images are built from the repository root.

The rest of this page covers the local `paxd` cluster defined by [`docker-compose.yml`](docker-compose.yml).

## Prerequisite

Install Docker with the Docker Compose plugin (`docker compose`). On macOS, Docker Desktop provides both.

## Local Cluster

The make targets below are defined in [`chain.mk`](../chain.mk). The root [`Makefile`](../Makefile) forwards them, so run them from the repository root.

**To start a single local node (Not Recommended)**

```sh
make build-docker-node && make run-local-node
```

**To start 4 node cluster**

The Make callers select `local-<full git HEAD revision>` for both
`LOCALNODE_IMAGE_TAG` and `X_WEBSEARCH_IMAGE_TAG` by default, including skipbuild,
monitoring, mixed-mode starts and stop targets (and their stop prerequisites).
`build-docker-node` builds `ghcr.io/sidiora-labs/localnode:<tag>` alongside the
`pax-chain/localnode` alias used for binary preparation. Compose builds
`ghcr.io/sidiora-labs/x-websearch:<tag>` from its declared build context.
No image-tag environment variables are needed for these local Make targets.

Override either tag through the environment or a Make command-line assignment,
for example `make docker-cluster-start LOCALNODE_IMAGE_TAG=local-experiment`.
Use the same overrides when building and starting; skipbuild skips the binary
build but still builds the node image. Integration CI selects `ci-<workflow run
id>` for `LOCALNODE_IMAGE_TAG` and retags the exact pulled integration package
image to that Compose alias; its registry package name stays unchanged.
`X_WEBSEARCH_IMAGE_TAG` still defaults to the source revision in CI. Direct
`docker compose` calls require both nonempty tags explicitly; to reuse images
after changing revisions, supply their original tags as overrides.

This starts a 4 node `paxd` cluster (chain ID `pax`).
```sh
# If this is the first time or you want to rebuild the binary:
make docker-cluster-start

# If you have run docker-cluster-start and build/paxd exist,
# you can skip the build process to quick start by:
make docker-cluster-start-skipbuild
```
All the logs and genesis files will be generated under the temporary build/generated folder.

```sh
# To monitor logs after cluster is started
tail -f build/generated/logs/paxd-0.log
```

**To ssh into a single node**
```sh
# List all containers
docker ps -a
# SSH into a running container
docker exec -it [container_name] /bin/bash
```

## Prometheus / Grafana (monitornode)

**Cluster and monitoring together:** from the repo root you can run:

The monitoring overlay requires a file-backed Grafana admin password for both
local and remote access. Create a private file outside the checkout containing a
unique password (never `admin`), then export its absolute path. For example,
generate the file once and retain it for later starts:

```sh
install -d -m 700 "$HOME/.config/paxeer-monitoring"
export GRAFANA_ADMIN_PASSWORD_FILE="$HOME/.config/paxeer-monitoring/grafana-admin-password"
(umask 077; openssl rand -base64 32 > "$GRAFANA_ADMIN_PASSWORD_FILE")
```

Compose rejects an unset or empty `GRAFANA_ADMIN_PASSWORD_FILE`; there is no
embedded password fallback. It mounts the file as the `grafana_admin_password`
secret at `/run/secrets/grafana_admin_password`, which Grafana reads through
`GF_SECURITY_ADMIN_PASSWORD__FILE`. Keep the file private and do not commit it.

```sh
make docker-cluster-start-monitoring
```

By default Grafana publishes on `127.0.0.1:3000` and Prometheus on
`127.0.0.1:9099`. Log in to Grafana as `admin` with the password from your file.
For deliberate remote Grafana access, keep the same secret file configured and
set `GRAFANA_BIND_ADDRESS` to the host interface address before starting the stack:

```sh
export GRAFANA_BIND_ADDRESS=192.0.2.10 # Replace with your host's private interface address.
make docker-cluster-start-monitoring
```

Use an interface reachable only by intended clients, or put Grafana behind an
authenticated TLS proxy; the direct Grafana endpoint is HTTP. Prometheus stays
on loopback even when Grafana is exposed remotely. An unset or empty
`GRAFANA_BIND_ADDRESS` selects loopback. The file seeds the initial admin password;
changing it does not rotate a password already stored in Grafana's database.
Existing Grafana installations must rotate their admin password through Grafana
before enabling remote access. Recreate existing containers to apply the new
port bindings and secret mount; restarting an old container retains its settings.

The make target stops any existing compose stack, rebuilds the node image, starts the four-node local cluster, and brings up the Prometheus and Grafana containers via the monitoring compose overlay, so you do not need to run the scripts below for that flow. To tear down the cluster and monitoring containers together:

```sh
make docker-cluster-stop-monitoring
```

**Scripts only:** to start Prometheus and Grafana by themselves (for example when you are not using the make target above):

```sh
./docker/monitornode/scripts/start-prometheus.sh
./docker/monitornode/scripts/start-grafana.sh
```

Grafana UI: http://localhost:3000 (login: admin / admin). To stop containers started via the scripts:

```sh
./docker/monitornode/scripts/stop-prometheus.sh
./docker/monitornode/scripts/stop-grafana.sh
```

## State Sync RPC Node

Requirement: Follow the above steps to start a 4 node docker cluster before starting any state sync node

```sh
# Be sure to start up a 4-node cluster before you start a state sync node
make docker-cluster-start
# Wait until the cluster has passed block 100, the first state sync snapshot
# (snapshot-interval in docker/localnode/config/app.toml)
# Start up a state sync node
make run-rpc-node
```

## Local Debugging & Testing
One of the benefits of using Docker is fast iteration. This setup supports:
- Being able to make changes locally and start up the chain to see the immediate impact
- Being able to change the vendored Cosmos SDK, Tendermint, database, and Wasm modules in this repository without publishing separate module versions

The dependencies are part of this monorepo, so no sibling repositories or `go.mod` replacements are required:
```sh
# Edit sdk/, consensus/, storage/, wasm/, or wasm-runtime/, then rebuild.
make build-docker-node
```
