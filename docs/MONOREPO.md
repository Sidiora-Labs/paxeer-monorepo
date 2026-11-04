# Monorepo layout

This repository contains one Paxeer X Network: the `paxd` node with the LayerX kernel embedded in it, contracts, hosted services, SDKs and applications. The kernel participates in block proposal and execution inside the network; a receipt is final when its block is committed. Source directories and language-specific builds are development boundaries within that network.

See [Paxeer Network](paxeer-network.md) for chain identity, public endpoints and tenant domains. The target directory layout is specified in [`spec/monorepo-layout/spec.kvx`](../spec/monorepo-layout/spec.kvx); the tables below distinguish existing paths from planned moves.

## Target groups

| Group | Holds |
| --- | --- |
| `apps/` | User-facing frontends |
| `services/` | Off-chain programs |
| `sdk/` | Client and guest SDKs |
| `core/` | The chain: kernel, node packages, framework fork, programs runtime and pinned upstream libraries |
| `tests/` | Shared test trees and fixtures; packages also retain local tests |
| `tools/` | Developer and operator tools |
| `contracts/` | On-chain sources |

`api/`, `deploy/`, `docker/`, `docs/`, `scripts/`, `spec/` and the root build and community files stay at the root.

## Current tree

| Path | Contents |
| --- | --- |
| `apps/perps` | The Hyperpax perps frontend |
| `apps/wallet`, `apps/web` | Wallet app and web app |
| `apps/explorer` | Block explorer frontend |
| `apps/bridge` | Bridge frontend |
| `apps/launchpad` | Launchpad web and mobile apps and the client workspace files |
| `services/agent` | Agent daemon, MCP server, shared Rust crates, TypeScript and Python SDKs |
| `services/interop` | Interop gateway, bridge relayer and agent-commerce services |
| `services/platform` | Hosted developer platform, middleware, perps data and ramps |
| `services/explorer` | Block explorer backend, services and deployment |
| `services/launchpad` | Launchpad services |
| `services/human` | Human control plane Rust workspace (crates, schema, tools) |
| `services/wallet` | Embedded wallet service: attestor, ceremony, gateway, demo, schema, deployment and runbooks |
| `services/bridge` | Bridge deployment |
| `sdk/platform`, `sdk/integrations` | Platform SDKs (Go, JVM, .NET, Swift) and framework integrations |
| `sdk/launchpad` | Launchpad protocol, shared libraries and client packages |
| `sdk/chainflow` | Chainflow client libraries for Go, Python, Rust and TypeScript |
| `sdk/js` | First-party `@pax-js/cosmos` package, edited in place and aligned to the protos of this repository |
| `sdk/wallet` | Wallet SDK (`@paxeer/wallet`), a member of the `services/wallet` pnpm workspace |
| `daemon/paxd`, `node`, `rpc` | Node entrypoint, application wiring and node JSON-RPC |
| `src`, `include`, `modules/lxphost` | C kernel implementation and headers, and its Go/cgo host inside `paxd` |
| `modules`, `precompiles`, `programs` | Chain modules, EVM precompiles and programs runtime |
| `core/deps` | Pinned snapshots of the upstream libraries the node builds from, each with a `VENDOR.kvx` pin record |
| `tests/` | Kernel C tests, `tests/fuzz`, `tests/integration`, `tests/occ`, `tests/testutil`, `tests/fixtures`, Solidity and vector suites |
| `tools/hpx`, `tools/platform`, `tools/launchpad` | Node installer and registry, platform CLI, emulator and release tooling, launchpad tools |
| `tools/loadtest`, `tools/paxctl`, `tools/uci`, `tools/wasm-tools` | Pinned tool snapshots with `VENDOR.kvx` pin records |
| `contracts/` | Root Foundry sources; `contracts/src` and `contracts/test` use `foundry.paxeer.toml` |
| `contracts/bridge` | Bridge vault contracts for EVM chains, the Solana program and test vectors |
| `contracts/launchpad` | Launchpad contracts |
| `deploy/` | Deployment descriptors, including `deploy/launchpad` |

## Trees that move under core in a later release

These are planned directory moves, not additional deployments. In particular, the current kernel sources still live at the repository root.

| Current path | Destination |
| --- | --- |
| `src`, `include`, `cmd`, `migrations` | `core/kernel` |
| `sdk` framework fork entries | `core/cosmos-sdk` |
| `consensus`, `interchain`, `wasm`, `wasm-runtime`, `storage`, `modules`, `precompiles`, `node`, `rpc`, `engine`, `daemon`, `admin`, `custodyproof`, `layerxproof`, `utils` | `core/<name>` |
| `programs/sdk` | `sdk/programs` |
| `programs` | `core/programs` |

## Build boundaries

- **Kernel library**: C17 (`-std=c17` in the root `Makefile`); `make build` produces `build/liblayerx.a`. Native tests and replay, arithmetic, fault and fuzz qualification exercise this library. These checks alone do not qualify the integrated node; see [Qualification](QUALIFICATION.md).
- **Integrated node**: Go 1.25.8 (`go.mod`), built through `chain.mk`. `make paxeer-build` builds `build/paxd`; `make paxd-build` also stamps the release version from `tools/release/version.json`. The native prerequisite `make paxd-native-libs` builds the C library and release programs sandbox archive, linked by `modules/lxphost/host.go`. `make paxeer-lint`, `make paxeer-test` and `make paxeer-ci` cover the Go tree; `make test-paxd-domain` includes the kernel host, node, EVM keeper and RPC tests.
- **Workspaces and contracts**: Rust 1.91.1 (`rust-toolchain.toml`), Solidity 0.8.27 (`foundry.toml` and `foundry.paxeer.toml`), and the npm workspaces listed in `package.json`. `make test-contracts` selects `tests/solidity` for the root Foundry project. The separate Foundry configuration selects `contracts/src` and `contracts/test`; see [Build and test](paxeer-network.md#build-and-test).
- **Aggregate checks**: `make workspace-build` composes the declared kernel, node, service, tool and contract builds. `make monorepo-ci` depends on `workspace-ci`, which composes source audit, inventory, workspace build, test and lint targets. It does not replace the explicit replay, arithmetic, fault or fuzz qualification matrices. Run aggregate and sanitizer gates at wave or release qualification; an individual task runs only its declared build and verification.
- `make ci` runs `public-audit`, kernel tests, a two-build archive comparison, consensus symbol checks and sanitizer suites. `scripts/ci/layout-check.sh` and `scripts/ci/vendor-verify.sh` check the layout and the vendored snapshots.

## Trust boundaries

The network's consensus commit is the authority for finality. There is no separate execution-to-settlement hop between the kernel and `paxd`.

- Kernel execution is reached through the node's `modules/lxphost` integration and chain modules. Native balances, asset-ledger state and receipts must be interpreted in that committed chain state.
- Hosted services, explorers and bridge clients consume node APIs and committed evidence. Their databases, caches and request acknowledgements do not establish network finality. Hosted `px_` methods have their own contract in `services/platform/hosted/gateway/openrpc.json`.
- A sub-chain is a tenant domain in the database, with its own namespace and allowances under the same network security. Creating one needs no new node infrastructure.
- Retained legacy source or migration code is not a deployment inventory. The standalone `layerxd` daemon and checkpoint/guarantor settlement model are retired.
- Imported code keeps its license and provenance boundaries. In particular, the explorer backend and frontend run as separate processes; consult `services/explorer/LICENSE`, `apps/explorer/LICENSE` and the imported service licenses before redistribution.

## Workflows

Workflows live in `.github/workflows/`. `node-build.yml` builds native libraries and `paxd` and exercises the host integration; its pull-request paths include kernel, programs and node sources. Other package workflows include `agent.yml`, `human.yml`, `platform.yml`, `programs-conformance.yml`, `explorer-build.yml` and `bridge-test.yml`, alongside the `paxeer-*.yml` workflows. Consult each workflow's triggers and commands for its actual coverage.

## Further reading

- [`README.md`](../README.md): network overview, build and test commands
- [`CONTRIBUTING.md`](../CONTRIBUTING.md): contribution guidelines
- [`QUALIFICATION.md`](QUALIFICATION.md): evidence levels and qualification gates
- [`paxeer-network.md`](paxeer-network.md): merged runtime, root node endpoints and tenant domains
