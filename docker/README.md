# Docker

This directory holds image Dockerfiles, Compose files and monitoring scripts.
Image builds use the repository root as their context and select the appropriate
Dockerfile, including those under `localnode/` and `rpcnode/`.
The node registry is described in [tools/hpx/README.md](../tools/hpx/README.md).
`monitornode/` and `docker_compose_monitoring/` contain Prometheus and Grafana
configuration.

The rest of this page describes the disposable local `paxd` runtime, the
services in [`docker-compose.yml`](docker-compose.yml) and monitoring.

## Prerequisite

Install Docker with the Docker Compose plugin (`docker compose`). On macOS, Docker Desktop provides both.

## Local node image

The image targets are defined in [`chain.mk`](../chain.mk). The root
[`Makefile`](../Makefile) forwards them, so run them from the repository root.
The Makefile also uses Git, jq and the Go toolchain on the host.

```sh
make build-docker-node
```

The Make callers select `local-<full git HEAD revision>` for both
`LOCALNODE_IMAGE_TAG` and `X_WEBSEARCH_IMAGE_TAG` by default.
`build-docker-node` builds `ghcr.io/sidiora-labs/localnode:<tag>` alongside the
`pax-chain/localnode` alias used for binary preparation. Compose builds
`ghcr.io/sidiora-labs/x-websearch:<tag>` from its declared build context.
No image-tag environment variables are needed for these local Make targets.

Override either tag through the environment or a Make command-line assignment,
for example `make build-docker-node LOCALNODE_IMAGE_TAG=local-experiment`.
Use the same overrides when building and selecting images. Integration CI selects
`ci-<workflow run id>` for `LOCALNODE_IMAGE_TAG` and retags the exact pulled integration package
image to that Compose alias; its registry package name stays unchanged.
`X_WEBSEARCH_IMAGE_TAG` still defaults to the source revision in CI. Direct
`docker compose` calls require both nonempty tags explicitly; to reuse images
after changing revisions, supply their original tags as overrides.

## Disposable node runtime

The local image starts
[`deploy.sh`](localnode/scripts/deploy.sh), which runs the
[`localnode_runtime.py`](localnode/scripts/localnode_runtime.py) coordinator
from the mounted repository root. Each container needs these environment inputs:

| Input | Meaning |
| --- | --- |
| `LOCALNODE_DISPOSABLE` | Must equal `1`; keys belong to this disposable local network |
| `LOCALNODE_RUN_ID` | Fresh shared run identifier, 16–128 ASCII letters, digits, underscores or hyphens, beginning with a letter or digit |
| `ID` | Node index, default `0` |
| `CLUSTER_SIZE` | Number of nodes, default `1`; between 1 and 100, with `ID` less than the size |
| `LOCALNODE_STARTUP_TIMEOUT` | Positive startup deadline in seconds, default `300`, maximum `3600` |
| `SKIP_BUILD` | `true` or `1` reuses a regular executable `build/paxd`; unset, `false` or `0` rebuilds it |

Supply runtime inputs to each container; all nodes in a cluster share the run
identifier, cluster size and writable repository mount. The coordinator creates
`build/localnode-runs/<run-id>` exclusively and uses consensus chain ID `pax`.
It snapshots the binary into `bin/paxd` there, writes `genesis.json`, and keeps
per-node logs in `logs/`. Node homes and keyrings use private temporary directories
that are removed on normal exit or a handled shutdown.

Node zero builds the binary and assembles genesis after all nodes initialize.
Each node publishes its launch marker only after its own consensus RPC reports
chain ID `pax`, a positive block height and `catching_up=false`. The supervisor
keeps the node attached and removes the marker when readiness is lost or it exits.

```sh
# Set LOCALNODE_RUN_ID to the identifier of the running local network.
tail -f "build/localnode-runs/$LOCALNODE_RUN_ID/logs/paxd-0.log"
```

## Compose services

The base Compose file defines `node0` through `node3` with `CLUSTER_SIZE=4`,
plus `x-websearch`. Its node environment lists and repository/cache mounts are
declared per service. `x-websearch` reads its configuration, key files and data
from `X_WEBSEARCH_HOME`. Its application configuration is described in the
[`x-websearch` guide](../services/interop/crates/x-websearch/README.md).
The monitoring overlay adds Prometheus and Grafana to the same network.

### Open a container shell

Set `CONTAINER_NAME` to the running container selected from `docker ps -a`.

```sh
# List all containers
docker ps -a
# Open a shell in a running container
docker exec -it "$CONTAINER_NAME" /bin/bash
```

## Prometheus / Grafana (monitornode)

The monitoring overlay requires a file-backed Grafana admin password for both
local and remote access. Create a private file outside the checkout containing a
unique password, then export its absolute path. For example,
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

By default Grafana publishes on loopback port `3000` and Prometheus on
loopback port `9099`. Log in to Grafana as `admin` with the password from your file.
For deliberate remote Grafana access, keep the same secret file configured and
set `GRAFANA_BIND_ADDRESS` in the environment to the intended host interface
address before starting the stack.

Use an interface reachable only by intended clients, or put Grafana behind an
authenticated TLS proxy; the direct Grafana endpoint is HTTP. Prometheus stays
on loopback even when Grafana is exposed remotely. An unset or empty
`GRAFANA_BIND_ADDRESS` selects loopback. The file seeds the initial admin password;
changing it does not rotate a password already stored in Grafana's database.
Existing Grafana installations must rotate their admin password through Grafana
before enabling remote access. Recreate existing containers to apply the new
port bindings and secret mount; restarting an old container retains its settings.

**Scripts only:** the standalone scripts use `GRAFANA_ADMIN_PASSWORD` directly.
Read the password from the private file before starting Grafana:

```sh
IFS= read -r GRAFANA_ADMIN_PASSWORD < "$GRAFANA_ADMIN_PASSWORD_FILE"
export GRAFANA_ADMIN_PASSWORD
./docker/monitornode/scripts/start-prometheus.sh
./docker/monitornode/scripts/start-grafana.sh
```

Grafana defaults to loopback port `3000`. The standalone Prometheus script
publishes port 9091 on host interfaces; restrict access through the host network
configuration. The scripts use the same `pax-grafana` and `pax-prometheus`
container names as the Compose overlay; choose one monitoring setup at a time.
Stop containers started by the scripts with:

```sh
./docker/monitornode/scripts/stop-prometheus.sh
./docker/monitornode/scripts/stop-grafana.sh
```

## State Sync RPC Node

The [`rpcnode` image](rpcnode/Dockerfile) initializes a node with consensus chain
ID `pax`. Its [initializer](rpcnode/scripts/step1_configure_init.sh) reads
`build/generated/genesis.json` and the templates under `rpcnode/config/`.
Its [state-sync configuration](rpcnode/scripts/state_sync.py) requires:

- `STATE_SYNC_RPC_SERVERS`: exactly two HTTP(S) origins with distinct hosts,
  without credentials, query strings or fragments.
- `STATE_SYNC_TRUST_HEIGHT`: a positive int64 checkpoint height.
- `STATE_SYNC_TRUST_HASH`: the checkpoint's 32-byte hexadecimal block hash.

Both providers must return the pinned block on chain `pax`; peer discovery
uses their `/net_info` results. Snapshot availability is also required.
The local node template sets `snapshot-interval = 100` in
[`localnode/config/app.toml`](localnode/config/app.toml).

## Local source changes

The Go module uses local replacements declared in [`go.mod`](../go.mod).
Node containers mount the checkout and build from it. After editing the node or
its local dependencies, use a fresh disposable run with `SKIP_BUILD` unset to
build a new binary. `make build-docker-node` builds the container image;
the coordinator builds `paxd` when the container starts.
