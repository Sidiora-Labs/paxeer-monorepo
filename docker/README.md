# Docker

This directory holds container image definitions, entrypoints, Compose files and monitoring configuration for Paxeer X nodes, hosted services and applications. The merged network runs the kernel inside `paxd`, with block commit providing finality. The local Compose stack provides a disposable development network and `x-websearch`; service images package the supporting hosted components.

## Layout

| Path | Contents |
| --- | --- |
| `bridge-relayer/` | Bridge relayer, cosigner and mirror signer image with role-specific startup checks. |
| `common/` | Shared `env-files.sh` entrypoint for materializing file inputs from environment variables. |
| `docker_compose_monitoring/` | Prometheus scrape and Grafana datasource configuration for Compose. |
| `explorer-backend/` | Blockscout backend Dockerfiles, including `oldUI.Dockerfile`. |
| `explorer-elixir-builder/` | Elixir build environment for the explorer backend. |
| `explorer-frontend/` | Explorer frontend image and Docker build context rules. |
| `explorer-sig-provider/` | Explorer signature provider image. |
| `explorer-smart-contract-verifier/` | Explorer contract verification service image. |
| `flyci-controller/` | Controller image for CI runners. |
| `flyci-runner/` | Runner image and its toolchain installation. |
| `gas-station/` | Gas sponsorship image and configuration-generating startup script. |
| `hpx-registry/` | hpx package registry and nginx image with volume initialization. |
| `human-service/` | Human service components, entrypoint and journal bootstrap helper. |
| `interop-gateway/` | Interoperability gateway image and file-input table. |
| `interop-mirror/` | Mirror publisher, verifier and signer image. |
| `kernel/` | Merged `paxd` kernel image, runtime supervisor, service unit and environment template. |
| `layerx/` | `paxd` image with native kernel libraries, program runtime, `paxctl` and `x-websearch`. |
| `localnode/` | Disposable local node image, node templates and coordinated initialization scripts. |
| `monitornode/` | Standalone Prometheus and Grafana configuration, dashboards and lifecycle scripts. |
| `paxeer/` | Paxeer boundary image and `paxd` node and initialization images. |
| `platform-agent-boundary/` | Agent authorization boundary image. |
| `platform-authority/` | Receipt authority image with the runtime clock. |
| `platform-core/` | Core boundary image. |
| `platform-dashboard/` | Dashboard API image and file-input table. |
| `platform-dashboard-web/` | Dashboard web application image. |
| `platform-faucet/` | Faucet service image. |
| `platform-gateway/` | Hosted gateway image and protected-file staging entrypoint. |
| `platform-identity/` | Identity service image and file-input table. |
| `platform-indexer/` | Indexer image, ABI packaging script and file-input table. |
| `platform-internal/` | KMS and event-source images and file-input table. |
| `platform-registry/` | Program registry image, chain trust bootstrap, service unit and environment template. |
| `platform-registry-builder/` | Isolated program build environment image. |
| `platform-testnet/` | Testnet control service image. |
| `platform-webhooks/` | Webhook service image and file-input table. |
| `ramps/` | Reference ramp image and file-input table. |
| `redis/` | TLS Redis image, file-input table, configuration renderer and test script. |
| `rpcnode/` | State-sync node image, node templates and initialization scripts. |
| `search-front/` | nginx search proxy image, resolver template, trusted-proxy renderer and tests. |
| `wallet-attestor/` | Wallet attestor image, policy files and file-input table. |
| `wallet-gateway/` | Wallet gateway image, identity binding entrypoint and file-input table. |
| `wallet-pwa/` | Wallet web application image with nginx. |
| `web/` | Web application image. |
| `x-websearch/` | Search service image and Fly startup entrypoint. |
| `docker-compose.yml` | Base stack declaring `node0` through `node3` and `x-websearch`. |
| `docker-compose.monitoring.yml` | Prometheus and Grafana overlay for the base stack. |
| `docker-compose.giga-mixed.yml` | Mixed execution overlay with Giga and OCC enabled on node zero. |
| `README.md` | Container layout, commands, configuration and documentation. |

## Build and test

Run the following Make commands from the repository root. Docker with the Compose plugin is required; Docker Desktop provides both on macOS. The node Make recipes also use Git, jq and the Go toolchain on the host. The root [Makefile](../Makefile) forwards these targets to [chain.mk](../chain.mk).

| Command | Purpose |
| --- | --- |
| `make build-docker-node` | Build the local node image from `localnode/Dockerfile`. |
| `make build-rpc-node` | Build the state-sync node image from `rpcnode/Dockerfile`. |
| `make docker-cluster-start` | Build the local node image and start the base Compose stack. |
| `make docker-cluster-start-skipbuild` | Rebuild the image and start the stack using an existing `build/paxd`. |
| `make docker-cluster-stop` | Stop the base Compose stack. |
| `make docker-cluster-start-monitoring` | Start the stack with the monitoring overlay. |
| `make docker-cluster-stop-monitoring` | Stop the stack and monitoring overlay. |
| `make giga-integration-test` | Run the EVM Giga integration script against a Giga-enabled cluster. |
| `make giga-mixed-integration-test` | Run the EVM Giga integration script against mixed execution nodes. |

The cluster start recipes stop the previous stack and remove `build/generated/`. Supply the disposable-run environment to every node container through the container deployment configuration. All nodes share a fresh run identifier, cluster size and writable repository mount. The local coordinator uses consensus chain ID `pax` and exclusively creates a directory beneath `build/localnode-runs/`. It snapshots the executable into that run's `bin/paxd`, writes `genesis.json` and keeps node logs in `logs/`. Node homes and keyrings live in private temporary directories removed on normal exit or handled shutdown.

The local image starts [deploy.sh](localnode/scripts/deploy.sh), which runs the coordinator from the mounted repository root. Node zero prepares the shared binary and assembles genesis after per-node initialization. A node publishes its launch marker after its own consensus RPC reports chain ID `pax`, a positive block height and completed catch-up. The supervisor keeps the node attached and removes the marker when readiness is lost or the process exits. The image build and the runtime binary build are separate: after changing node sources or the local replacements in [go.mod](../go.mod), start a fresh disposable run with binary rebuilding enabled.

The image recipes select source-revision tags for both the local node and search service, so local Make targets need no explicit image-tag environment variables. The local node build also produces the `pax-chain/localnode` alias used for binary preparation. Compose builds the search image from its declared context. Integration CI selects a run-specific local node tag and retags the exact pulled image to the Compose alias; the search tag still follows the source revision. Override tags through the environment or Make command-line assignments, using the same overrides when building and selecting images. Direct Compose calls require explicit nonempty tags; retaining an earlier image across source changes requires its original tag.

Most image builds use the repository root as their context and select a Dockerfile under this directory. Follow the selected Dockerfile's `COPY` paths and its build script: the explorer backend and registry builder also have component-specific contexts. For the explorer frontend, [apps/explorer/package.json](../apps/explorer/package.json) provides `yarn build:docker`, run from `apps/explorer`.

The directory also contains focused test scripts:

| Command from the repository root | Coverage and requirements |
| --- | --- |
| `bash docker/redis/tests/redis-image.test.sh` | File-input coverage and Redis configuration rendering; requires Docker, Python and the Git objects referenced by the script. |
| `bash docker/search-front/real-ip.test.sh` | Trusted-proxy rendering and input rejection; nginx and curl enable the served client-address cases. |

Standalone monitoring uses the scripts below, with Prometheus started before Grafana. These scripts and Compose use the same `pax-prometheus` and `pax-grafana` container names, so select one monitoring setup at a time. Stop scripts retain their containers for later starts.

```sh
./docker/monitornode/scripts/start-prometheus.sh
./docker/monitornode/scripts/start-grafana.sh
./docker/monitornode/scripts/stop-prometheus.sh
./docker/monitornode/scripts/stop-grafana.sh
```

Grafana publishes port `3000`. Compose publishes Prometheus on port `9099`, while the standalone script publishes it on port `9091`. Compose monitoring binds to loopback by default; the standalone Prometheus script publishes on host interfaces. Remote Grafana access uses the configured binding and password, with an authenticated TLS proxy or network restrictions for intended clients; the direct endpoint uses HTTP. Prometheus remains on loopback when the Compose Grafana binding changes. Changing the Compose secret file does not rotate a password already stored in Grafana's database. Existing installations rotate their password through Grafana before enabling remote access. Recreate containers to apply changed port bindings or secret mounts.

## Configuration

### Local nodes and Compose

The Compose files and node scripts consume these environment variables:

| Area | Names |
| --- | --- |
| Image selection and platform | `LOCALNODE_IMAGE_TAG`, `X_WEBSEARCH_IMAGE_TAG`, `DOCKER_PLATFORM` |
| Repository, caches and user mapping | `PROJECT_HOME`, `GO_PKG_PATH`, `GOCACHE`, `USERID`, `GROUPID` |
| Make cluster attachment | `DOCKER_DETACH` |
| Disposable node coordination | `LOCALNODE_DISPOSABLE`, `LOCALNODE_RUN_ID`, `ID`, `CLUSTER_SIZE`, `LOCALNODE_STARTUP_TIMEOUT` |
| Binary build and initialization | `SKIP_BUILD`, `MOCK_BALANCES`, `NUM_ACCOUNTS`, `INVARIANT_CHECK_INTERVAL`, `UPGRADE_VERSION_LIST` |
| Execution and storage | `GIGA_EXECUTOR`, `GIGA_OCC`, `RECEIPT_BACKEND`, `AUTOBAHN`, `GIGA_STORAGE`, `GIGA_MIGRATE_FROM_MEMIAVL`, `GIGA_FLATKV_ONLY` |
| Search mounts and key files | `X_WEBSEARCH_HOME`, `X_WEBSEARCH_ATTESTOR_KEY_FILE`, `X_WEBSEARCH_SUBMITTER_KEY_FILE`, `X_WEBSEARCH_RECEIVER_KEY_FILE` |
| State-sync checkpoint | `STATE_SYNC_RPC_SERVERS`, `STATE_SYNC_TRUST_HEIGHT`, `STATE_SYNC_TRUST_HASH` |

The local coordinator requires explicit disposable-network consent and a fresh shared run identifier. It validates node indices, cluster size and a bounded startup deadline. Its internal script environment includes `PAXD_BIN`, `PAX_HOME`, `PAX_PRIVATE_DIR`, `PAX_RUN_DIR` and `PAX_STARTUP_DEADLINE`. See [localnode_runtime.py](localnode/scripts/localnode_runtime.py) and [localnode_common.sh](localnode/scripts/localnode_common.sh) for their use.

Local node configuration comes from [localnode/config/app.toml](localnode/config/app.toml) and [localnode/config/config.toml](localnode/config/config.toml). The local application template enables periodic state snapshots. The base Compose file declares per-node environment lists and repository and Go cache mounts. Search reads `config.json`, key files and persistent data from its configured home.

The RPC node initializer reads [rpcnode/config/app.toml](rpcnode/config/app.toml), [rpcnode/config/config.toml](rpcnode/config/config.toml) and `build/generated/genesis.json`. Its [state_sync.py](rpcnode/scripts/state_sync.py) requires two HTTP(S) origins with distinct hosts and no credentials, query strings or fragments, plus a positive int64 checkpoint height and its 32-byte hexadecimal block hash. Both providers must return the pinned block on chain `pax`; peer discovery uses their `/net_info` responses. State sync also requires an available snapshot.

### Monitoring

Compose reads `GRAFANA_ADMIN_PASSWORD_FILE` and `GRAFANA_BIND_ADDRESS`. The password file is required and mounted as the `grafana_admin_password` secret, consumed through `GF_SECURITY_ADMIN_PASSWORD__FILE`. The standalone Grafana script reads `GRAFANA_ADMIN_PASSWORD` directly and exports `GF_SECURITY_ADMIN_PASSWORD`. Set its password before starting it for remote access. Both configurations also set `GF_SECURITY_ADMIN_USER` and `GF_USERS_ALLOW_SIGN_UP`.

Compose loads [prometheus.yaml](docker_compose_monitoring/prometheus.yaml) and [grafana-datasource.yaml](docker_compose_monitoring/grafana-datasource.yaml). Standalone scripts load [prometheus.yaml](monitornode/config/prometheus.yaml), [grafana.yaml](monitornode/config/grafana.yaml), [grafana-dashboards.yaml](monitornode/config/grafana-dashboards.yaml) and the [dashboards directory](monitornode/dashboards/).

### Service images

[common/env-files.sh](common/env-files.sh) reads `LAYERX_FILES_TABLE`, `LAYERX_ROLE` and `LAYERX_FILES_OWNER`, with per-variable ownership overrides. Each selected row in a service's `files.tsv` names an environment variable containing a base64-encoded file, its destination, permissions and applicable roles. The entrypoint writes those files and removes the table's input variables from the environment before launching the service. The tables list the file-input names for each image:

| Image | File-input configuration |
| --- | --- |
| Bridge relayer | [bridge-relayer/files.tsv](bridge-relayer/files.tsv) |
| Interoperability gateway | [interop-gateway/files.tsv](interop-gateway/files.tsv) |
| Dashboard | [platform-dashboard/files.tsv](platform-dashboard/files.tsv) |
| Hosted gateway | [platform-gateway/files.tsv](platform-gateway/files.tsv) |
| Identity | [platform-identity/files.tsv](platform-identity/files.tsv) |
| Indexer | [platform-indexer/files.tsv](platform-indexer/files.tsv) |
| Internal services | [platform-internal/files.tsv](platform-internal/files.tsv) |
| Webhooks | [platform-webhooks/files.tsv](platform-webhooks/files.tsv) |
| Ramps | [ramps/files.tsv](ramps/files.tsv) |
| Redis | [redis/files.tsv](redis/files.tsv) |
| Search proxy | [search-front/files.tsv](search-front/files.tsv) |
| Wallet attestor | [wallet-attestor/files.tsv](wallet-attestor/files.tsv) |
| Wallet gateway | [wallet-gateway/files.tsv](wallet-gateway/files.tsv) |

Additional entrypoint inputs and configuration sources include:

| Component | Names and files |
| --- | --- |
| Merged kernel | `LAYERX_KERNEL_PROFILE`, `LAYERX_KERNEL_NODE_HOME`, `LAYERX_NODE_NETWORK_ID`, `LAYERX_NODE_EVM_CHAIN_ID`, `LAYERX_NODE_PAXEER_CHAIN_ID`; [kernel/kernel.env.example](kernel/kernel.env.example) and [kernel/layerx-kernel.service](kernel/layerx-kernel.service). The unit reads `kernel.env`; the runtime reads the provisioned node home's `config` and `data` files. |
| Program registry | `LAYERX_REGISTRY_IMAGE`, `LAYERX_REGISTRY_CHAIN_ID`, `LAYERX_REGISTRY_DEPLOYMENT_TRUST_ROOT`, `LAYERX_REGISTRY_DEPLOYMENT_TRUST_SHA256`, `LAYERX_REGISTRY_STATE`, `LAYERX_REGISTRY_JOURNAL`; [platform-registry/registry.env.example](platform-registry/registry.env.example) lists listener, TLS, authentication, builder and event inputs. [platform-registry/layerx-registry.service](platform-registry/layerx-registry.service) reads `registry.env`. |
| hpx registry | `PORT`, `HPX_ADDR`, `LAYERX_TRUSTED_PROXIES`; [nginx-registry.conf.template](../tools/hpx/hosting/nginx-registry.conf.template). |
| Search proxy | `LAYERX_TRUSTED_PROXIES`, `SEARCH_FRONT_UPSTREAMS`, `NGINX_ENTRYPOINT_LOCAL_RESOLVERS`, `NGINX_ENVSUBST_OUTPUT_DIR`; [resolver.conf.template](search-front/resolver.conf.template) and [nginx.conf](../services/interop/deploy/search-front/nginx.conf). |
| Redis | `LAYERX_ROLE`, `REDIS_MAXMEMORY`, `REDIS_CONF`, `REDIS_TLS_CERT`, `REDIS_TLS_KEY`, `REDIS_TLS_CA`, `REDIS_ACL`; [redis/entrypoint.sh](redis/entrypoint.sh) renders the server configuration. |
| Human services | `LAYERX_HUMAN_SERVICE_PRIVATE_DIR`, `LAYERX_HUMAN_ATTESTOR_NODES`, `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`; [human-service/entrypoint.sh](human-service/entrypoint.sh). |
| Bridge relayer | `LAYERX_BRIDGE_RELAYER_CONFIG`, `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG`, `LAYERX_BRIDGE_OPERATOR_INVENTORY`, `LAYERX_BRIDGE_APPROVED_MEMBERSHIP`, `LAYERX_BRIDGE_ATTESTOR_INDEX`, `LAYERX_BRIDGE_ATTESTOR_SET_SIZE`, `LAYERX_BRIDGE_ATTESTOR_THRESHOLD`, `LAYERX_BRIDGE_ATTESTOR_KEY_FILE`; [bridge-relayer/entrypoint.sh](bridge-relayer/entrypoint.sh). |
| Wallet gateway | `WALLET_IDENTITY_BINDING_TENANT`, `WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE`; [wallet-gateway/entrypoint.sh](wallet-gateway/entrypoint.sh). |
| Search Fly entrypoint | `XWEB_HOPS` and the search key-file variables; [x-websearch/entrypoint.sh](x-websearch/entrypoint.sh). |
| Dashboard web | `DASHBOARD_API_ORIGIN`; [platform-dashboard-web/Dockerfile](platform-dashboard-web/Dockerfile). |
| Web application | `LAYERX_HUMAN_SERVICE_URL`; [web/Dockerfile](web/Dockerfile). |
| Explorer frontend | `GIT_COMMIT_SHA`, `GIT_TAG`, `NEXT_OPEN_TELEMETRY_ENABLED`, `ENVS_PRESET`; [environment reference](../docs/explorer/ENVS.md) and [build-time environment reference](../docs/explorer/BUILD-TIME_ENVS.md). |
| Wallet web application | Public build inputs are declared individually in [wallet-pwa/Dockerfile](wallet-pwa/Dockerfile), including `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_PAXEER_RPC_URL`, `NEXT_PUBLIC_CHAIN_ID`; nginx reads [nginx.conf](../apps/wallet/deployment/nginx.conf). |

The gas station [init.sh](gas-station/init.sh) generates sponsorship and rate configuration from these variables:

- `GAS_STATION_CHAIN_ID`, `GAS_STATION_ENDPOINTS`, `GAS_STATION_LISTEN`, `GAS_STATION_PAYMASTER`, `GAS_STATION_TOKEN`, `GAS_STATION_DECIMALS`.
- `GAS_STATION_MAX_PRIORITY_FEE_PER_GAS`, `GAS_STATION_GAS_LIMIT`, `GAS_STATION_MAX_RATE_AGE`, `GAS_STATION_SPREAD_BPS`, `GAS_STATION_MARGIN_BPS`.
- `GAS_STATION_PER_ACCOUNT_LIMIT`, `GAS_STATION_PER_INTERVAL_LIMIT`, `GAS_STATION_PER_QUOTE_LIMIT`, `GAS_STATION_INTERVAL_SECONDS`, `GAS_STATION_BALANCE_FLOOR`, `GAS_STATION_RELAYER_KEY_ENV`.
- `GAS_STATION_RATE_OWNER_KEY_ENV`, `GAS_STATION_RATE_CADENCE_SECONDS`, `GAS_STATION_RATE_GAS_BUDGET_PER_DAY`, `GAS_STATION_RATE_BALANCE_FLOOR`, `GAS_STATION_RATE_MAX_FEE_PER_GAS`, `GAS_STATION_RATE_CONFIRMATION_RETRY_SECONDS`.

Hosted service topology, storage and service-specific configuration are declared under [services/platform/hosted](../services/platform/hosted/), [services/interop/deploy](../services/interop/deploy/) and [services/wallet/deploy](../services/wallet/deploy/).

## Documentation

- [Docker development guide](../docs/site/paxeer-docs/public/markdown/docker.md) and its [documentation site page](../docs/site/paxeer-docs/src/app/docker/page.tsx).
- [Service operator runbook](../docs/release/7.0.0-service-operator-runbook.md) and [validator runbook](../docs/release/7.0.0-validator-runbook.md).
- [Explorer container builds](../docs/explorer/CUSTOM_BUILD.md), [runtime environment reference](../docs/explorer/ENVS.md) and [build-time environment reference](../docs/explorer/BUILD-TIME_ENVS.md).
- [Wallet deployment](../docs/wallet/app/deployment/README.md).
- [hpx node registry](../tools/hpx/README.md).
- [Search service guide](../services/interop/crates/x-websearch/README.md).
