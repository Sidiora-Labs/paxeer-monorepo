# Paxeer X services

This directory holds the agent and human interfaces, hosted platform services,
wallet custody and gateway, explorer backend, launchpad services, and bridge and
interoperability tooling for Paxeer X Network. Paxeer mainnet-beta is one BFT
network running `paxd`, with EVM chain ID `125` (`0x7d`). Its kernel is part of
the network and proposes blocks; a receipt is final once its block is committed.
These services provide clients, APIs, indexing and external integrations around
that network. Sub-chains are tenant domains in the database, launchable without
new infrastructure.

## Layout

- [`agent/`](agent/) — Rust agent runtime, policy, custody, proof verification, MCP transport, and Rust, TypeScript and Python SDKs.
- [`bridge/`](bridge/) — Bridge deployment scripts, chain configuration loaders, attestor manifests, application configuration generation and deployment checks.
- [`explorer/`](explorer/) — Blockscout backend, Rust contract verification and signature services, and deployment files; the frontend is in [`apps/explorer`](../apps/explorer/).
- [`human/`](human/) — Rust authentication, onboarding, intent compilation, custody, approvals, activity and identity and security providers.
- [`interop/`](interop/) — External protocol adapters, x402, mirror archives, bridge relaying, migration, gas sponsorship and web search services.
- [`launchpad/`](launchpad/) — Go API, gateway, indexing, discovery, metadata, notifications, social, user, ranking, statistics, profit and loss, livestream and chart services.
- [`platform/`](platform/) — Hosted gateway, authority, identity, indexer, registry, faucet, webhooks and supporting services, payment ramps, middleware, examples and workspace tools.
- [`wallet/`](wallet/) — Wallet HTTP gateway, demo, threshold attestor, custody ceremonies, API schema and deployment configuration.
- [`README.md`](README.md) — Directory overview and component entry points.

## Build and test

Run the following commands from the repository root. Builds and tests are owned
by the individual components; `services/` has no shared build manifest.

| Component | Build | Test |
| --- | --- | --- |
| Agent | `make agent-build` | `sh services/agent/tools/run-real-node-tests.sh test` |
| Human | `make human-build` | `make human-test` |
| Interoperability | `make interop-build` | `make interop-test` |
| Platform | `make platform-build` | `make platform-test` |
| Launchpad | `make -C services/launchpad build` | `make -C services/launchpad test` |
| Wallet JavaScript workspace | `pnpm --dir services/wallet build` | `pnpm --dir services/wallet test` |
| Explorer backend | `services/explorer/deploy/tools/mix-in-builder.sh compile` | `services/explorer/deploy/tools/mix-in-builder.sh test apps/explorer/test/explorer` |

The root [`Makefile`](../Makefile), platform
[`Makefile.inc`](platform/Makefile.inc), launchpad
[`Makefile`](launchpad/Makefile), and wallet
[`package.json`](wallet/package.json) define these targets and scripts. The
wallet workspace includes packages outside this directory, listed in
[`pnpm-workspace.yaml`](wallet/pnpm-workspace.yaml). Platform targets use offline
Cargo resolution and require dependencies to be available locally. Agent tests
require the native toolchains and root test environment described in the
[agent guide](agent/README.md#tests). Rust test targets use the repository's
[runtime clock wrapper](../tools/runtime/run-with-clock.sh). Human targets also
build and test the web application.

The explorer's [builder script](explorer/deploy/tools/mix-in-builder.sh) uses
Docker with its Elixir builder image and PostgreSQL and Redis sidecars. Its
Rust services have separate manifests under
[`smart-contract-verifier`](explorer/services/smart-contract-verifier/Cargo.toml)
and [`sig-provider`](explorer/services/sig-provider/Cargo.toml); the
[explorer guide](explorer/README.md) covers their build commands.

Bridge deployment tooling is compiled by its deployment and dry-run scripts.
The [deployment script checks](bridge/deploy/tests/deploy-scripts-check.sh)
run with `bash services/bridge/deploy/tests/deploy-scripts-check.sh`. Contract
and relayer build and test instructions are in the
[bridge guide](../contracts/bridge/README.md).

## Configuration

Configuration belongs to each service. The following entry points identify
environment variables and configuration files; the linked sources and guides
describe the remaining service-specific settings.

- Agent: `LAYERX_AGENT_NODE_ENDPOINT`, `LAYERX_AGENT_NODE_RPC_URL`,
  `LAYERX_AGENT_NODE_CHAIN_ID` and `LAYERX_AGENT_HUMAN_PEERS` are read by the
  [daemon entry point](agent/crates/layerx-agentd/src/main.rs). See the
  [daemon guide](agent/crates/layerx-agentd/README.md) for policy, storage,
  custody and listener settings.
- Bridge: `BRIDGE_DEPLOY_OVERLAY`, `PAXEER_BRIDGE_DEPLOYMENT_RECORD` and
  `PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR` select deployment inputs and records.
  The [overlay script](bridge/deploy/overlay.sh) reads the selected overlay;
  the [chain configuration loader](bridge/deploy/chainconfig/config.go) reads
  chain JSON configuration. [`attestors.json`](bridge/deploy/attestors.json)
  describes the attestor manifest.
- Explorer: [`backend/config/`](explorer/backend/config/) contains Elixir
  configuration. The builder reads `MIX_ENV`, `MIX_IN_BUILDER_IMAGE`,
  `CHAIN_TYPE`, `ETHEREUM_JSONRPC_VARIANT`, `PGUSER` and `PGPASSWORD`.
  [Runtime environment variables](../docs/explorer/ENVS.md) and
  [build-time variables](../docs/explorer/BUILD-TIME_ENVS.md) are documented
  separately.
- Human: the [HTTP service entry point](human/crates/layerx-human-service/src/main.rs)
  reads `LAYERX_HUMAN_BIND`, `LAYERX_HUMAN_LISTENER`,
  `LAYERX_HUMAN_TLS_CERT_DER`, `LAYERX_HUMAN_TLS_KEY_DER` and
  `LAYERX_HUMAN_COMPONENT_SOCKET`. Provider configuration is documented in
  the [Human workspace guide](human/README.md).
- Interoperability: `LAYERX_INTEROP_CONFIG` selects the runtime JSON manifest;
  `LAYERX_INTEROP_MODULE_REGISTRY_FILE` selects the module registry.
  `LAYERX_INTEROP_NETWORK_ID`, `LAYERX_INTEROP_PROTOCOL_NETWORK_ID`,
  `LAYERX_INTEROP_LISTEN` and `LAYERX_INTEROP_REDIS_URL` are read by the
  [service configuration loader](interop/crates/layerx-interop-service/src/config.rs).
  The [gateway deployment guide](interop/deploy/gateway/README.md) describes
  [`config.example.json`](interop/deploy/gateway/config.example.json) and
  [`module-registry.example.json`](interop/deploy/gateway/module-registry.example.json).
- Launchpad: each Go service has its own configuration loader. The
  [API loader](launchpad/api/internal/config/config.go) reads `DATABASE_URL`,
  `REDIS_URL`, `PORT`, `LOG_LEVEL`, `NODE_ENV`, `CORS_ALLOWED_ORIGINS`,
  `DISCOVERY_RUNTIME_CONFIG_FILE`, `NETWORK_AUTHORITY_CONFIG_FILE` and
  `STREAM_CONSUMER_CONFIG_FILE`.
- Platform: the [hosted gateway entry point](platform/hosted/gateway/src/main.rs)
  reads `LAYERX_GATEWAY_LISTEN`, `LAYERX_GATEWAY_LISTENER`,
  `LAYERX_GATEWAY_NETWORK_ID`, `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID`,
  `LAYERX_GATEWAY_COMPONENT_URL`, `LAYERX_GATEWAY_AUTHORITY_URL`,
  `LAYERX_GATEWAY_REDIS_URL`, `LAYERX_GATEWAY_MODULE_REGISTRY_FILE` and
  `LAYERX_GATEWAY_RECEIPT_KEYS_FILE`. Its
  [`railway.env.example`](platform/hosted/gateway/railway.env.example)
  lists deployment variable names. The hosted `px_` method contract is
  [`openrpc.json`](platform/hosted/gateway/openrpc.json).
- Wallet: the [gateway environment schema](wallet/gateway/src/env.ts) reads
  `DATABASE_URL`, `SUPABASE_URL`, `RPC_URLS`, `HYPERPAXEER_CHAIN_ID`,
  `HYPERPAXEER_RPC_URL` and `ATTESTOR_ENDPOINTS`, together with custody,
  authentication and policy settings. [`deploy/env`](wallet/deploy/env)
  and the [wallet guide](wallet/README.md#configuration) list variables
  by service.

## Documentation

- [Platform documentation](../docs/platform/README.md) and [platform API site page](../docs/site/paxeer-docs/public/markdown/platform-api.md).
- [Agent SDK compatibility](../docs/agent/COMPATIBILITY.md) and [agent runtime site page](../docs/site/paxeer-docs/public/markdown/agent-runtime.md).
- [Human API reference](../docs/platform/content/reference/human-api.md) and [Human interface site page](../docs/site/paxeer-docs/public/markdown/human-interface.md).
- [Portable receipt verification](../docs/interop/PORTABILITY.md) and [interoperability site page](../docs/site/paxeer-docs/public/markdown/interop.md).
- [Bridge contracts and deployment](../contracts/bridge/README.md) and [bridge site page](../docs/site/paxeer-docs/public/markdown/bridge.md).
- [Explorer environment reference](../docs/explorer/ENVS.md) and [custom build guide](../docs/explorer/CUSTOM_BUILD.md).
- [Launchpad chain integration](../docs/launchpad/chain-launch.md), [production checklist](../docs/launchpad/production-checklist.md) and [launchpad site page](../docs/site/paxeer-docs/public/markdown/launchpad.md).
- [Wallet service runbooks](../docs/wallet/service/runbooks/) and [security review](../docs/wallet/service/security-review.md).
