# Network bringup tools

This directory holds the scripts, Go inspection library and command, deployment manifests, and tests used to initialize, inspect and maintain the Paxeer network and its hosted services. The tools manage native `paxd` identities and genesis, certificates, edge routing, RPC fleet recovery, software upgrades and service state. Paxeer mainnet-beta is one BFT chain with EVM chain ID 125; its kernel runs within the network, and receipts become final when their blocks commit. Node JSON-RPC uses the root path, while hosted gateway operations use their explicitly configured service endpoint.

## Layout

| Path | Contents |
| --- | --- |
| [cmd/](cmd/) | Entry point for the `bringup-inspect` command. |
| [tests/](tests/) | Shell and Python tests, fixtures, artifact checks and the Rust configuration loader for x-websearch tests. |
| [check-live.sh](check-live.sh) | Live checks of nodes, certificates, hosted services and committed transactions. |
| [check-live.test.sh](check-live.test.sh) | Shell tests for the live checker. |
| [ca.sh](ca.sh) | Certificate inventory, issuance, renewal and server certificate request handling. |
| [ca-services.tsv](ca-services.tsv) | Service identities, deployment targets, custody roles and certificate requirements. |
| [railway-names.env](railway-names.env) | Service names used to complete certificate subject alternative names. |
| [mint-secrets.sh](mint-secrets.sh) | Creates protected service environment files and delivery scripts from deployment inputs. |
| [kernel-genesis.sh](kernel-genesis.sh) | Native chain initialization, identity adoption and offline genesis artifact conversion. |
| [paxd-initialization.md](paxd-initialization.md) | Instructions for initializing a chain home and retaining node identities. |
| [build-foundation-artifacts.sh](build-foundation-artifacts.sh) | Builds the merged runtime artifact bundle through the foundation artifact script. |
| [runtime-bundle.json](runtime-bundle.json) | Executable, source directory and runtime library declarations for that bundle. |
| [runtime-dependencies.json](runtime-dependencies.json) | Go package dependencies and source inputs consumed by runtime bundle validation. |
| [sync-fleet.sh](sync-fleet.sh) | RPC fleet configuration, restart, resynchronization and binary rollout. |
| [sync-fleet.test.sh](sync-fleet.test.sh) | Shell tests for fleet commands. |
| [upgrade-fleet.sh](upgrade-fleet.sh) | Stages release binaries and swaps node units across an upgrade halt. |
| [install-archive.sh](install-archive.sh) | Installs an archive node and its nginx TLS site. |
| [archive-comet-location.conf](archive-comet-location.conf) | nginx location template for read-only, rate-limited Comet access. |
| [archive-comet-zone.conf](archive-comet-zone.conf) | nginx request limit zone for Comet access. |
| [edge.sh](edge.sh) | Registers, renders and maintains HTTP, TLS stream and RPC edge routes. |
| [edge-sync.sh](edge-sync.sh) | Obtains edge certificates and transfers acknowledged edge state to secondary edges. |
| [edge-apps.example](edge-apps.example) | Example map of public names to upstream modes and destinations. |
| [edge-contract.py](edge-contract.py) | Validates a route manifest against rendered nginx configuration. |
| [search-front.sh](search-front.sh) | Selects serving RPC nodes and renders, checks and deploys the search front. |
| [xweb-config.sh](xweb-config.sh) | Renders x-websearch configuration, validates asset prices and registers serving nodes. |
| [validator-firewall.sh](validator-firewall.sh) | Renders and applies validator access rules for node APIs and web attestors. |
| [human-state-preserve.sh](human-state-preserve.sh) | Exports, imports, verifies and restores retained human service state and cryptographic material. |
| [value-loop.sh](value-loop.sh) | Submits a retained signed transaction and checks committed inclusion, receipts and balances. |
| [registry-artifacts.py](registry-artifacts.py) | Builds and manages protected registry builder artifacts and their container lifecycle. |
| [docs-names.sh](docs-names.sh) | Checks documentation names against the public RPC inventory and network specification. |
| [inspect.go](inspect.go) | Shared inspector dispatch used by the command and library callers. |
| [foundation_cli.go](foundation_cli.go) | Inspector operations for foundation artifacts, native genesis, identities and fleet transport. |
| [live_cli.go](live_cli.go) | Inspector operations for committed transactions and live service checks. |
| [repair_cli.go](repair_cli.go) | Inspector operations for edge routes, deployment roles and event handoffs. |
| [repair_m795_cli.go](repair_m795_cli.go) | Inspector operations for archive state, retained material and recovery. |
| [repair_m798_cli.go](repair_m798_cli.go) | Inspector operations for upgrades and x-websearch. |
| [qualification_contract.go](qualification_contract.go) | Validates source-bound artifact manifests and protected artifact inputs. |
| [inventory.go](inventory.go) | Reads and validates the public endpoint inventory. |
| [foundation.go](foundation.go) | Native genesis, committed node observations and artifact validation. |
| [paxd_identity.go](paxd_identity.go) | Reads and validates retained native node identity. |
| [fleet.go](fleet.go) | SSH transport, bounded fleet operations and durable recovery. |
| [upgrade_state.go](upgrade_state.go) | Per-unit upgrade state and recovery checks. |
| [resync_state.go](resync_state.go) | Durable resynchronization state. |
| [archive_state.go](archive_state.go) | Archive certificate and nginx state reconciliation. |
| [edge_state.go](edge_state.go) | Durable nginx configuration transactions and reconciliation. |
| [edge_mutation.go](edge_mutation.go) | Serializes edge render, validation and reload operations. |
| [edge_routes.go](edge_routes.go) | Route validation and rendering. |
| [edge_transfer.go](edge_transfer.go) | Edge state export and import. |
| [bundle.go](bundle.go) | Protected identity bundle loading and certificate validation. |
| [pkcs12.go](pkcs12.go) | PKCS#12 identity decoding and validation. |
| [roster.go](roster.go) | Service identity roster parsing. |
| [renewal.go](renewal.go) | Selects reuse, issuance or renewal from retained identity state. |
| [secrets.go](secrets.go) | Generates service material and validates consumer inputs. |
| [secret_plan.go](secret_plan.go) | Maps retained service material to deployment consumers. |
| [runtime_bundle.go](runtime_bundle.go) | Embedded runtime declarations and source dependency validation. |
| [routes.go](routes.go) | Checks node and hosted gateway operations through their configured transports. |
| [live_rpc.go](live_rpc.go) | Node RPC and committed receipt validation. |
| [live_evidence.go](live_evidence.go) | Deployment, service manifest and readiness validation. |
| [live_journeys.go](live_journeys.go) | Payment and event delivery checks. |
| [live_repair.go](live_repair.go) | Edge readiness, event handoff and registry observation checks. |
| [sqlite_evidence.go](sqlite_evidence.go) | Reads exported SQLite cursor data. |
| [search_selection.go](search_selection.go) | Serving-node selection and search payment checks. |
| [xweb_transport.go](xweb_transport.go) | Asset inventory checks, x-websearch rendering and registration. |
| [value_loop.go](value_loop.go) | Binds committed transaction data to retained signed bytes and chain identity. |
| [registry_lifecycle.go](registry_lifecycle.go) | Owned registry build container lifecycle through the Docker API. |

## Build and test

Run commands from the repository root. The Go package belongs to the root [Go module](../../go.mod). The [foundation artifact builder](tests/foundation-artifacts.py) uses the Go version in that manifest and the Rust version in [rust-toolchain.toml](../../rust-toolchain.toml).

The runtime bundle build command is also used by [build-caps-codecs.sh](../qualification/paxeer-x/build-caps-codecs.sh). Set `CAPS_NATIVE_BIN_DIR` to an absent absolute output directory outside the checkout, under a parent owned by the caller with private permissions:

```sh
bash tools/bringup/build-foundation-artifacts.sh --output "$CAPS_NATIVE_BIN_DIR"
```

The builder requires a clean source checkout and builds `paxd`, `attestor` and `bringup-inspect`, with the declared runtime libraries and an artifact manifest. Its node build calls `make -f chain.mk build`.

For the inspector alone, the [tools test workflow](../../.github/workflows/ci.yml) uses the following commands. `RUNNER_TEMP` selects the output parent:

```sh
mkdir -p "$RUNNER_TEMP/tools-python-bin"
go build -o "$RUNNER_TEMP/tools-python-bin/bringup-inspect" ./tools/bringup/cmd/bringup-inspect
```

Put the built inspector on the command path, or select it with `BRINGUP_INSPECT_BIN`, for scripts that call it.

The root [Makefile](../../Makefile) includes [tests/python-tests.mk](tests/python-tests.mk), so its tools test target includes this directory's Python suite:

```sh
make test-tools-python
```

The root [chain.mk](../../chain.mk) discovers Go test packages, including this package, and runs them through its split test target:

```sh
make -f chain.mk test
```

The x-websearch loader prerequisite is built with the command declared in the [tools test workflow](../../.github/workflows/ci.yml):

```sh
cargo build --locked --manifest-path tools/bringup/tests/xweb-loader/Cargo.toml
```

The shell test entry points can also be run directly:

```sh
bash tools/bringup/check-live.test.sh
bash tools/bringup/sync-fleet.test.sh
bash tools/bringup/tests/edge.test.sh
bash tools/bringup/tests/kernel-genesis.test.sh
bash tools/bringup/tests/mint-secrets.test.sh
bash tools/bringup/tests/search-front.test.sh
bash tools/bringup/tests/validator-firewall.test.sh
```

Their prerequisites depend on the selected suite: the live checker harness requires the inspector, `cast`, Python, curl and OpenSSL; fleet tests also use rsync and tar. The Go fleet integration tests use SSH and disposable local services. Runtime image and registry artifact tests take explicit image and artifact inputs from their test scripts.

## Configuration

Configuration is supplied by the invoking environment, the private host map named by `BRINGUP_HOSTS_FILE`, and the deployment files read by each operation. The following names select those inputs; values belong in the deployment environment.

| Area | Environment variables and host-map keys |
| --- | --- |
| Shared command and host selection | `BRINGUP_INSPECT_BIN`, `BRINGUP_HOSTS_FILE`, `CHECK_LIVE_TIMEOUT`, `EDGE_HOST`, `ARCHIVE_HOST`, `KERNEL_HOST`, `RPC_HOSTS`, `VALIDATOR_HOSTS`, `OLD_WALLET_HOST`, `RETAINED_ON_VALIDATOR`, `SSH_AUTH_SOCK` |
| Certificates | `LAYERX_CA_DIR`, `LAYERX_ATTESTOR_CA_DIR`, `LAYERX_TLS_DIR`, `LAYERX_CA_NAMES_FILE`, `LAYERX_CA_RENEW_DAYS`, `LAYERX_EXTRA_SANS_<ROW>`, `LAYERX_RAILWAY_BIN`, `LAYERX_RAILWAY_ENVIRONMENT` |
| Material and initialization | `ATTESTOR_CERT_DIR`, `MODULE_REGISTRY_FILE`, `LAYERX_GENESIS_BUILD`, `PAXD` |
| Archive installation | `BRINGUP_RPC_INVENTORY`, `HPX_MIRROR`, `HPX_HOME` |
| Fleet recovery | `PAXD_RELEASE_URL`, `PAXD_RELEASE_DIR`, `BRINGUP_NEVER_PLACE`, `SYNC_FLEET_STATE`, `SYNC_FLEET_JOURNAL`, `SYNC_FLEET_REPORT`, `SYNC_FLEET_SETTLE`, `SYNC_FLEET_CONNECT_TIMEOUT`, `SYNC_FLEET_TRANSFER_TIMEOUT`, `SYNC_FLEET_RECOVERY_TIMEOUT` |
| Upgrade | `UPGRADE_UNITS`, `PAXD_VERSION`, `UPGRADE_NAME`, `UPGRADE_JOURNAL`, `UPGRADE_JOURNAL_LINES`, `UPGRADE_POLL`, `UPGRADE_WATCH_TIMEOUT`, `UPGRADE_RECOVERY_TIMEOUT` |
| Edge routing | `EDGE_LOCAL`, `EDGE_APP`, `EDGE_NGINX_DIR`, `EDGE_WEBROOT`, `EDGE_CERT_DIR`, `EDGE_RESOLVER`, `EDGE_CRON_DIR`, `EDGE_RPC_NAME`, `EDGE_RPC_POOL`, `EDGE_RPC_MAX_LAG` |
| Edge certificate synchronization | `EDGE_SECONDARIES`, `EDGE_ACME_EMAIL`, `EDGE_DNS_API`, `EDGE_DNS_API_KEY`, `EDGE_DNS_WAIT`, `CERTBOT_DOMAIN`, `CERTBOT_VALIDATION` |
| Search selection | `SEARCH_FRONT_PRIMARY`, `SEARCH_FRONT_BACKUP`, `SEARCH_FRONT_READINESS_TIMEOUT` |
| x-websearch | `XWEB_SERVE_NAMES`, `XWEB_GATEWAY_ENDPOINT`, `XWEB_EVM_ENDPOINT`, `XWEB_RECEIPT_KEYS_ENDPOINT`, `XWEB_SID_PER_PAX`, `XWEB_PAX_USD_PRICE`, `XWEB_SEEDS`, `XWEB_REGISTER_KEY_FILE`; `XWEB_ASSET_ID_` and `XWEB_DECIMALS_` variables for `SID`, `PAX`, `USDC` and `USDL` |
| Native node checks | `CHECK_LIVE_PAXD_HOME`, `CHECK_LIVE_KERNEL_RPC`, `CHECK_LIVE_KERNEL_COMET_RPC`, `CHECK_LIVE_KERNEL_LOCAL` |
| Value loop | `LAYERX_KERNEL_DATA`, `LAYERX_KERNEL_RUN`, `LAYERX_KERNEL_TLS`, `LAYERX_NODE_RPC`, `LAYERX_NODE_TRUST_ANCHOR_DER`, `LAYERX_NODE_NETWORK_ID`, `SSL_CERT_FILE`, `CHECK_LIVE_VALUE_LOOP_ASSET`, `CHECK_LIVE_VALUE_LOOP_TIMEOUT` |
| Gateway and indexer checks | `CHECK_LIVE_GATEWAY_RPC`, `CHECK_LIVE_GATEWAY_TOKEN`, `CHECK_LIVE_ROUTER_URL`, `CHECK_LIVE_ROUTER_ACCOUNT`, `CHECK_LIVE_ROUTER_CONNECT`, `CHECK_LIVE_INDEXER_RPC`, `CHECK_LIVE_INDEXER_ACCOUNT`, `CHECK_LIVE_INDEXER_CONNECT` |
| Service readiness and state | `CHECK_LIVE_STAGE_DIR`, `CHECK_LIVE_EDGE_NGINX_DIR`, `CHECK_LIVE_EDGE_BOX_CA`, `CHECK_LIVE_EDGE_READINESS_PATH`, `CHECK_LIVE_HUMAN_BASE`, `CHECK_LIVE_HUMAN_ORIGIN`, `CHECK_LIVE_HUMAN_PROBE_EMAIL`, `LAYERX_HUMAN_BIND`, `XWEB_ATTESTORS_ON_FLY` |
| Distribution and explorer checks | `CHECK_LIVE_HPX_ORIGIN`, `CHECK_LIVE_HPX_APP_ORIGIN`, `CHECK_LIVE_EXPLORER_ORIGIN`, `EXPLORER_DATABASE_URL`, `EXPLORER_LEGACY_DATABASE_URL`, `PAXSCAN_DATABASE_PUBLIC_URL` |
| Sponsored transaction checks | `CHECK_LIVE_GAS_ORIGIN`, `CHECK_LIVE_GAS_RPC`, `CHECK_LIVE_GAS_ACCOUNT_KEYSTORE`, `CHECK_LIVE_GAS_ACCOUNT_PASSWORD_FILE`, `CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT`, `CHECK_LIVE_GAS_RECEIPT_ATTEMPTS` |
| Archive receipt checks | `CHECK_LIVE_RELAY_ORIGIN`, `CHECK_LIVE_RELAY_TRANSACTION`, `CHECK_LIVE_RELAY_CA`, `CHECK_LIVE_RELAY_DELIVERY_TIMEOUT` |
| Interop checks | `CHECK_LIVE_INTEROP_ORIGIN`, `CHECK_LIVE_INTEROP_RPC`, `CHECK_LIVE_INTEROP_NODE_RPC`, `CHECK_LIVE_INTEROP_TRANSACTION`, `CHECK_LIVE_INTEROP_API_KEY_FILE`, `CHECK_LIVE_INTEROP_DELIVERY_TIMEOUT`, `CHECK_LIVE_INTEROP_ADAPTERS_CUSTOMER_TOKEN_FILE`, `CHECK_LIVE_INTEROP_ADAPTERS_OPERATOR_TOKEN_FILE` |
| Event delivery checks | `CHECK_LIVE_DEVELOPERS_RECEIVER`, `CHECK_LIVE_DEVELOPERS_TOKEN_FILE`, `CHECK_LIVE_DEVELOPERS_DELIVERY_ATTEMPTS`, `CHECK_LIVE_DEVELOPERS_SOURCE_URL`, `CHECK_LIVE_DEVELOPERS_SOURCE_REQUEST`, `CHECK_LIVE_DEVELOPERS_SOURCE_TOKEN_FILE` |
| Registry, bridge and mirror checks | `CHECK_LIVE_REGISTRY_CURSOR_DIR`, `CHECK_LIVE_REGISTRY_PROGRAM_ID`, `PAXEER_BRIDGE_RECORDS_DIR`, `CHECK_LIVE_MIRRORS_REQUEST`, `CHECK_LIVE_MIRRORS_STATUS`, `CHECK_LIVE_MIRRORS_VERIFY_BIN`, `CHECK_LIVE_MIRRORS_VERIFY_CONFIG`, `CHECK_LIVE_MIRRORS_VERIFY_TIMEOUT` |
| Search payment checks | `CHECK_LIVE_SEARCH_PAYER_ADDRESS`, `CHECK_LIVE_SEARCH_GRANT_ID` |
| Artifact tools and test inputs | `PAXEER_GO`, `PAXEER_RUSTUP`, `CC`, `CAPS_NATIVE_BIN_DIR`, `RUNNER_TEMP`, `BRINGUP_DOCKER_SOCKET`, `PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST`, `PAXEER_X_EVIDENCE_DIR`, `PAXEER_X_RUNTIME_IMAGE`, `PAXEER_X_RUNTIME_FIXTURE_DIR`, `PAXEER_X_REGISTRY_ARTIFACT_MANIFEST`, `XWEB_CONFIG_LOADER` |

`LAYERX_EXTRA_SANS_<ROW>` uses a service roster row name. Internal readiness checks also select private connections through the `CHECK_LIVE_INTERNAL_CONNECT_<GROUP>` variables read by [check-live.sh](check-live.sh).

The maintained files read by these tools include:

- [ca-services.tsv](ca-services.tsv) and [railway-names.env](railway-names.env) for certificate identities and service names.
- The supplied edge environment file and its `EDGE_APP` map, whose format is shown in [edge-apps.example](edge-apps.example).
- [runtime-bundle.json](runtime-bundle.json) and [runtime-dependencies.json](runtime-dependencies.json) for the artifact bundle.
- [deploy/rollout.kvx](../../deploy/rollout.kvx) for service roles and deployment consumers.
- [services/wallet/deploy/human.toml](../../services/wallet/deploy/human.toml) and the `attestor-*.env.example` files under [services/wallet/deploy/](../../services/wallet/deploy/) for app names and material consumers.
- [services/platform/hosted/interop/fly.toml](../../services/platform/hosted/interop/fly.toml) for hosted interop checks.
- [services/interop/deploy/search-front/railway.env.example](../../services/interop/deploy/search-front/railway.env.example) for the search front configuration.
- [tools/release/version.json](../release/version.json) for the release version and upgrade plan name.
- The installed public RPC inventory selected by `BRINGUP_RPC_INVENTORY` for archive installation, and [spec/layerx-merge/spec.kvx](../../spec/layerx-merge/spec.kvx) for declared public names.

Node operations also read the selected home's `config/config.toml`, `config/app.toml` and `config/genesis.json`, along with its identity and signing state. Certificate operations read protected PEM, DER and PKCS#12 bundles from the configured custody directories. The per-operation scripts describe their additional deployment inputs.

## Documentation

- [Native chain initialization](paxd-initialization.md).
- [Network routes and transports](../../docs/system/routes.md).
- [Node RPC](../../docs/system/rpc.md).
- [Hosted webhook service material](../../docs/wiki/HostedWebhooks.md).
- [Node installation and build on the documentation site](../../docs/site/paxeer-docs/src/app/installation/page.tsx).
- [Node configuration on the documentation site](../../docs/site/paxeer-docs/src/app/configuration/page.tsx).
- [HPX node operations on the documentation site](../../docs/site/paxeer-docs/src/app/admin-hpx/page.tsx).
