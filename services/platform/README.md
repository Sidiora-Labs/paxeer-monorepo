# Platform services

This directory holds hosted application services, payment middleware, reference applications, market-maker ramps and trading data services for Paxeer X Network. The Rust workspace includes the hosted gateway and supporting identity, authority, registry, indexing and runtime services. They connect applications to the network, whose kernel executes inside `paxd`; EVM chain ID is 125 and block commit establishes finality. The hosted gateway serves its declared `px_` API, while signed chain transactions use the node's JSON-RPC root. Tenant domains are namespaces within the same network.

## Layout

- [examples/](examples/) — reference buyer, seller, merchant and program applications, shared application support and language samples.
- [hosted/](hosted/) — hosted service implementations, deployment definitions, initialization scripts and integration tests.
- [middleware/](middleware/) — buyer, seller, merchant and agent payment middleware, examples and conformance suites.
- [perf-db/](perf-db/) — the independent PerfDB Rust library and CLI for ticks, candles, account state, relational data and events.
- [perps-data/](perps-data/) — Go indexer, gateway, explorer, portal, replay and load-test services, with SQL migrations and deployment files.
- [ramps/](ramps/) — market-maker ramp toolkit and reference HTTP service for customer orders, native payments and external provider settlement.
- [tests/](tests/) — cross-component tests for ramp dependencies, sandbox journeys and component boundaries.
- [tools/](tools/) — workspace validation, dependency policy, documentation checks, KVX parsing and benchmarking tools.
- [Cargo.toml](Cargo.toml) — Rust workspace members, shared dependencies, package metadata and lint settings.
- [Cargo.lock](Cargo.lock) — resolved dependencies for the Rust workspace.
- [Makefile.inc](Makefile.inc) — platform build, test, SDK, middleware and sample targets included by the root Makefile.
- [workspace.kvx](workspace.kvx) — platform directory and ecosystem declarations, including SDK, tooling and documentation roots outside this directory.
- [clippy.toml](clippy.toml) — Clippy configuration.
- [deny.toml](deny.toml) — dependency advisory, license, source and package policies.
- [.gitignore](.gitignore) — Cargo target-directory exclusions.

## Build and test

Run these targets from the repository root. They are defined in [Makefile.inc](Makefile.inc) and the [root Makefile](../../Makefile).

| Command | Purpose |
| --- | --- |
| `make platform-js-install` | Install the locked Node workspace dependencies. |
| `make platform-dependencies-install` | Prepare dependency policy tooling, language sample dependencies and local JVM artifacts. |
| `make platform-build` | Build the platform, platform tools and SDK generator Rust workspaces; check SDK generation and release registries. |
| `make platform-build-all` | Extend the build to Node packages, Go SDK, marketplace guest, JVM, .NET, Swift and Python sources. |
| `make platform-test` | Run the platform, platform tools and SDK generator Rust workspace tests. |
| `make platform-test-tooling` | Test CLI, emulator and testnet tooling, shell entry points and bootstrap behavior. |
| `make platform-test-middleware` | Run payment middleware and framework integration tests and conformance checks. |
| `make platform-test-reference-apps` | Build the buyer middleware and check the declared reference applications. |
| `make platform-test-docs` | Check developer documentation and build the language samples, with reference application prerequisites. |
| `make interop-test-ramps` | Test the ramp toolkit and build the reference ramp service. |

The Rust build targets use locked, offline dependencies. The sample targets require their language toolchains and installed dependencies. `platform-test` uses [run-with-clock.sh](../../tools/runtime/run-with-clock.sh) to supervise tests with the runtime clock and prepares its public TLS prerequisites through the root Makefile.

PerfDB has its own [Cargo manifest](perf-db/Cargo.toml) and lockfile, separate from the platform workspace; its [README](perf-db/README.md) describes its build, tests and benchmarks. The Go data services belong to the [root Go module](../../go.mod); their [Dockerfile](perps-data/deploy/Dockerfile) builds the binary selected by `SERVICE`, with `BUILD_IMAGE` and `RUNTIME_IMAGE` supplied as image build arguments.

## Configuration

Build configuration uses `PLATFORM_CARGO`, `PLATFORM_MAVEN_REPOSITORY` and `PLATFORM_MAVEN_OFFLINE` in [Makefile.inc](Makefile.inc). The runtime-clock wrapper reads `LAYERX_RUNTIME_CLOCK_BIN`, `LAYERX_RUNTIME_CLOCK_DIRECTORY`, `LAYERX_RUNTIME_CLOCK_SOCKET` and `CARGO_TARGET_DIR`.

Runtime configuration belongs to the individual services. Configuration entry points include:

| Component | Inputs and configuration sources |
| --- | --- |
| Hosted gateway | `LAYERX_GATEWAY_LISTEN`, `PORT`, `LAYERX_GATEWAY_LISTENER`, `LAYERX_GATEWAY_NETWORK_ID`, `LAYERX_GATEWAY_LXP_WIRE_VERSION`, `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID`, `LAYERX_GATEWAY_REDIS_URL`, `LAYERX_GATEWAY_REDIS_USERNAME_FILE`, `LAYERX_GATEWAY_REDIS_PASSWORD_FILE`, `LAYERX_GATEWAY_TLS_CERT_DER`, `LAYERX_GATEWAY_TLS_KEY_DER`, `LAYERX_GATEWAY_OUTBOUND_CA_DER` and `LAYERX_GATEWAY_RECEIPT_KEYS_FILE` are read by [main.rs](hosted/gateway/src/main.rs). Backend and authentication inputs are described in the [gateway documentation](../../docs/wiki/HostedGateway.md). |
| Reference applications | [runtime.mjs](examples/support/runtime.mjs) reads `LAYERX_EXAMPLE_CONFIG` and the selected application's `layerx.example.json`. The [examples README](examples/README.md) names the receipt-key and account inputs for each application. |
| Market-maker ramps | The reference service reads a JSON configuration passed on its command line; [config.example.json](ramps/config.example.json) describes the structure. See the [ramp README](ramps/README.md) and [operations guide](ramps/OPERATIONS.md) for configuration and referenced credential files. |
| Perps data services | [config.go](perps-data/internal/config/config.go) reads `PERPS_CHAIN_ID`, `PERPS_RPC_URLS`, `PERPS_COMET_URLS`, `DATABASE_URL`, `NATS_URL`, `PORT`, `PERPS_AUTH_SECRET`, `PERPS_EXPLORER_TX_BASE`, `PERPS_GATEWAY_URL`, `PERPS_START_HEIGHT` and `PERPS_TRUSTED_PROXY_CIDRS`, according to the selected service. [services.json](perps-data/deploy/services.json) and [railway/](perps-data/deploy/railway/) declare deployment inputs; [nats.conf](perps-data/deploy/nats/nats.conf) configures NATS. |
| PerfDB CLI | [main.rs](perf-db/src/main.rs) reads `PERFDB_DATA_DIR`; library users supply `PerfDbConfig`. |

Hosted services carry their own deployment and environment initialization files under [hosted/](hosted/). Their service-specific configuration is documented in the hosted pages below.

## Documentation

- [Developer documentation](../../docs/platform/README.md), [installation](../../docs/platform/content/install.md) and [reference applications](../../docs/platform/content/guide/reference-applications.md).
- [Unified network](../../docs/wiki/UnifiedNetwork.md) and [hosted gateway](../../docs/wiki/HostedGateway.md).
- [Core](../../docs/wiki/HostedCore.md), [authority](../../docs/wiki/HostedAuthority.md), [agent boundary](../../docs/wiki/HostedAgentBoundary.md), [identity](../../docs/wiki/HostedIdentity.md) and [internal services](../../docs/wiki/HostedInternal.md).
- [Human service](../../docs/wiki/HostedHuman.md), [dashboard](../../docs/wiki/HostedDashboard.md), [faucet](../../docs/wiki/HostedFaucet.md), [registry](../../docs/wiki/HostedRegistry.md) and [webhooks](../../docs/wiki/HostedWebhooks.md).
- [Seller](../../docs/platform/content/guide/seller-middleware.md), [buyer](../../docs/platform/content/guide/buyer-middleware.md), [merchant](../../docs/platform/content/guide/merchant-middleware.md) and [agent](../../docs/platform/content/guide/agent-middleware.md) middleware guides.
- [Market-maker ramps](../../docs/wiki/MarketMakerRamps.md).
- Documentation site: [Platform API page](../../docs/site/paxeer-docs/src/app/platform-api/page.tsx) and its [Markdown reference](../../docs/site/paxeer-docs/public/markdown/platform-api.md).
