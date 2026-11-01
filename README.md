<div align="center">

<img alt="Paxeer X Network" src="https://shieldcn.dev/header/transparent.svg?title=Paxeer+X+Network&subtitle=Distributed+HyperState+Machine+for+payments%2C+code+execution+and+intent+routing&logo=https%3A%2F%2Fwww.paxeer.network%2Fpaxeer.x_darkmode_white.png&mode=dark&image=https%3A%2F%2Fwww.sidioralabs.com%2Fmedia%2Floops%2Fsphere-field.jpg">

  <h3><a href="https://paxeer.network/">Paxeer X Network</a> brings payments, code execution and intent routing into one network.</h3>
  <p>
    
[![badge](https://shieldcn.dev/github/Sidiora-Labs/paxeer-monorepo/license.svg?variant=outline&size=lg&font=geist&logo=github&logoColor=dadee6&height=56&padX=15&iconSize=21&gap=4&labelGap=12)](https://github.com/Sidiora-Labs/paxeer-monorepo)
[![badge](https://shieldcn.dev/badge/GOLANG.svg?variant=outline&size=lg&font=geist&logo=go&logoColor=1363e6&labelTextColor=1c3e76&height=56&padX=15&iconSize=28&gap=4&labelGap=12)](#)
[![badge](https://shieldcn.dev/badge/RUST.svg?variant=outline&size=lg&font=geist&logo=rust&logoColor=ffffff&labelTextColor=1c3e76&height=56&padX=15&iconSize=28&gap=4&labelGap=12)](#)
[![badge](https://shieldcn.dev/badge/LANGUAGE%20C117.svg?variant=outline&size=lg&font=geist&logo=lu%3ACpu&logoColor=f4f4f2&labelTextColor=1c3e76&height=56&padX=15&iconSize=28&gap=4&labelGap=12)](#)
[![badge](https://shieldcn.dev/badge/SIDIORA%20LABS.svg?variant=outline&size=lg&font=geist&logo=lu%3AGlobe&logoColor=0043e6&labelTextColor=1c3e76&height=56&padX=15&iconSize=21&gap=4&labelGap=12)](https://github.com/org/Sidiora-Labs)
[![badge](https://shieldcn.dev/github/Sidiora-Labs/paxeer-monorepo/ci.svg?variant=outline&size=lg&font=geist&logo=github&logoColor=dadee6&height=56&padX=15&iconSize=21&gap=4&labelGap=12)](https://github.com/Sidiora-Labs/paxeer-monorepo/actions)



**Table of Contents**

<!--TOC--><p align="center">
  <a href="#what-is-paxeer-x-network"><img alt="About Paxeer X" src="https://shieldcn.dev/badge/01-About%20Paxeer%20X-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#documentation"><img alt="Documentation" src="https://shieldcn.dev/badge/02-Documentation-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#specification"><img alt="Specification" src="https://shieldcn.dev/badge/03-Specification-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#community"><img alt="Community" src="https://shieldcn.dev/badge/04-Community-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#contributing"><img alt="Contributing" src="https://shieldcn.dev/badge/05-Contributing-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#security-policy-and-vulnerability-reporting"><img alt="Security" src="https://shieldcn.dev/badge/06-Security-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#directory-structure"><img alt="Directory Structure" src="https://shieldcn.dev/badge/07-Directory%20Structure-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#development-and-release-process"><img alt="Development" src="https://shieldcn.dev/badge/08-Development-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#overview"><img alt="Overview" src="https://shieldcn.dev/badge/09-Overview-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#build"><img alt="Build" src="https://shieldcn.dev/badge/10-Build-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#test"><img alt="Test" src="https://shieldcn.dev/badge/11-Test-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#releases-and-network-upgrades"><img alt="Releases &amp; Upgrades" src="https://shieldcn.dev/badge/12-Releases%20%26%20Upgrades-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#downloading-and-shallow-cloning-the-monorepo"><img alt="Clone Repository" src="https://shieldcn.dev/badge/13-Clone%20Repository-blue.svg?variant=outline&amp;size=sm"></a>
  <a href="#license"><img alt="License" src="https://shieldcn.dev/badge/14-License-blue.svg?variant=outline&amp;size=sm"></a>
</p>
</div>
<!--TOC-->

## What is Paxeer X Network?

[Paxeer X Network](https://paxeer.network/) is a distributed HyperState Machine for payments, code execution and intent routing, developed by Sidiora Labs. It connects an EVM-compatible chain with a deterministic execution and accounting domain for autonomous agents.

In this repository you’ll find the core components of the network: the Paxeer X chain, the LayerX kernel, on-chain contracts, off-chain services, client SDKs and user-facing applications. Together, they provide the infrastructure for developers to build applications and agents on a shared network.

The network has two execution domains:

- **Paxeer X chain.** `paxd` is the Go node, with BFT consensus and an EVM execution layer using chain ID **125**. The main package is `daemon/paxd`, with state machine modules in `modules/` and RPC servers in `rpc/`.
- **LayerX kernel.** `layerxd` is the C17 daemon that executes and accounts for agent activity deterministically, returning a signed receipt for each activity. `layerxctl` provides the operator CLI, and `programs/` contains the kernel programs runtime. The kernel runs as a separate daemon built from `cmd/layerxd`; its checkpoints settle on the chain through the `layerxAnchor` precompile.

The monorepo also includes the agent daemon and MCP server, interop gateway and bridge relayer, hosted developer platform, human control plane, embedded wallet service, explorer and launchpad services. Applications include the Hyperpax perps frontend, wallet, web app, explorer and launchpad web and mobile apps.

## Documentation

- To build on Paxeer X Network, start with the [Paxeer documentation](https://docs.paxeer.app/) and the [developer wiki](docs/wiki/Home.md).
- To connect to the chain, see the [public JSON-RPC endpoints](docs/site/docs/reference/public-rpc.md) and [beta endpoint checklist](docs/wiki/Getting-Started-Beta.md).
- For wallets, funding and payments, follow the [Payments Quickstart](docs/wiki/PaymentsQuickstart.md).
- To run a local kernel cluster, follow the [Kernel Quickstart](docs/wiki/Quickstart.md).
- For protocol references, see [assets](docs/wiki/Assets.md), [RPC methods](docs/wiki/PublicRpc.md), [commitment levels](docs/wiki/CommitmentLevels.md) and the [public payment API](docs/wiki/PublicAPI.md).

The limited beta has not opened yet. The gateway API becomes available when it does. The beta runs on mainnet with real value; there is no faucet for general use. Approved developers receive test allocations from the team.

### Client SDKs

| Language | Path |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

The agent daemon is `services/agent/crates/layerx-agentd` and the MCP server is `services/agent/crates/layerx-mcp`. The developer CLI in `tools/platform/cli` installs those transports with `layerx install mcp` and `layerx install a2a`.

## Specification

Detailed feature specifications live in [`spec/`](spec/). Each feature’s `spec.kvx` defines its requirements, acceptance criteria and dependency-ordered task list. The [specification workflow](spec/workflow.kvx) describes how work is planned and tracked. `spec/specgen` runs `tools/specgen` to generate agent instruction files, including `CLAUDE.md` and `AGENTS.md`.

The proposed integration of the LayerX kernel into the chain is described in the [LayerX merge specification](spec/layerx-merge/spec.kvx), which remains a draft. The [monorepo layout specification](spec/monorepo-layout/spec.kvx) describes the target repository structure.

### EVM and Native Precompiles

Contracts reach the native modules through stateful precompiles under `precompiles/`. `precompiles/setup.go` registers `bank`, `wasmd`, `json`, `addr`, `staking`, `gov`, `distribution`, `oracle`, `ibc`, `pointer`, `pointerview`, `p256Verify`, `solo`, `layerxVerify` (`0x1012`), `layerxCustody` (`0x1013`), `layerxAnchor` (`0x1014`), `layerxExchange` (`0x1015`), `layerxBridge` (`0x1016`), `launchpad` (`0x1017`), `feeToken` (`0x1018`) and `xweb` (`0x1019`). The `MergeAddressTable` set (`precompiles/setup.go:66-79`) activates at the `v7.0` `MergeUpgrade` plan (`precompiles/common/precompiles.go:29`): `layerxledger` (`0x101A`), `layerxpay` (`0x101B`), `layerxauth` (`0x101C`), `layerxprograms` (`0x101D`), `orob` (`0x0901`), `clearing` (`0x0902`), `oracle aggregator` (`0x0903`), `pofq` (`0x0904`), `scheduler` (`0x0905`), `streams` (`0x0906`), `tee attestor` (`0x0907`) and `eip712` (`0x0908`). The v6.12.0 set built in `node/app.go` and served once that upgrade activates adds `perps` at `0x1015` ahead of `layerxExchange`, the second-generation `launchpad` at `0x1017`, `pricefeed` at `0x1020`, and the spot set `spot_pecor`, `spot_orders`, `spot_stoporders`, `spot_quoter` and `spot_vault` at `0x1021`-`0x1025`.

## Community

Paxeer X Network is developed by [Sidiora Labs](https://www.sidioralabs.com/). Visit the [Paxeer website](https://paxeer.network/) for project information and [SUPPORT.md](SUPPORT.md) for support channels.

[GOVERNANCE.md](GOVERNANCE.md) explains how project decisions are made. Community participation follows the [Code of Conduct](CODE_OF_CONDUCT.md).

## Contributing

Paxeer X Network is an open-source project spanning protocol engineering, agent infrastructure, developer tooling and applications. Contributions across these components help improve the network and the experience of building on it.

[CONTRIBUTING.md](CONTRIBUTING.md) explains how to propose and land changes. Before starting, read the [monorepo guide](docs/MONOREPO.md), the relevant feature specification and the [development process](#development-and-release-process) below.

Every change must pass the public repository audit. Run the checks for the components you modify, and include the relevant validation with your contribution.

## Security Policy and Vulnerability Reporting

Please follow [SECURITY.md](SECURITY.md) to report vulnerabilities privately. **Do not disclose security vulnerabilities in public issues.**

## Directory Structure

| Path | Contents |
| --- | --- |
| `apps/` | User-facing frontends: `apps/perps` (the Hyperpax perps frontend), `apps/wallet` (wallet app), `apps/web` (web app), `apps/explorer` (block explorer frontend), `apps/launchpad` (launchpad web and mobile apps) |
| `services/` | Off-chain programs: `services/agent` (agent daemon, MCP server, Rust, TypeScript and Python SDKs), `services/interop` (interop gateway, bridge relayer), `services/platform` (hosted developer platform), `services/explorer` (block explorer backend), `services/launchpad` (launchpad services), `services/human` (human control plane Rust workspace), `services/wallet` (embedded wallet service, whose pnpm workspace also lists `sdk/wallet`), `services/bridge` (bridge deployment) |
| `sdk/` | Client SDKs: `sdk/platform` (Go, JVM, .NET, Swift), `sdk/integrations`, `sdk/launchpad` (launchpad protocol, shared libraries and client packages), `sdk/chainflow` (Chainflow client libraries for Go, Python, Rust and TypeScript), `sdk/wallet` (wallet SDK), `sdk/js`; it also holds the application framework fork the node builds from |
| `contracts/` | On-chain sources: the root Foundry tree, `contracts/bridge` (EVM vaults, the Solana program and test vectors) and `contracts/launchpad` |
| `core/deps/` | Pinned snapshots of the upstream libraries the node builds from, each with a `VENDOR.kvx` pin record |
| `tests/` | Kernel C tests, fuzz targets (`tests/fuzz`), integration tests (`tests/integration`), OCC tests (`tests/occ`), `tests/testutil`, `tests/fixtures`, Solidity and vector suites |
| `tools/` | Developer and operator tools, including `tools/hpx`, `tools/platform` (CLI, emulator, release), `tools/launchpad`, `tools/loadtest`, `tools/paxctl`, `tools/uci` and `tools/wasm-tools` |
| `migrations/` | SQL for kernel genesis import sections, rebuildable projections and the history index |
| `api/` | Protobuf definitions (buf module) |
| `deploy/`, `docker/` | Deployment descriptors (including `deploy/launchpad`) and container builds |
| `docs/` | Wiki, documentation site sources, translations and qualification notes |
| `spec/` | Feature specifications and the kvx workflow |
| `scripts/` | CI, qualification, release and workspace scripts |

The chain and kernel trees stay at the root in this release and move under `core/` in a later one: `src`, `include`, `cmd` and `migrations` (the kernel), `modules`, `precompiles`, `engine`, `consensus`, `node`, `rpc`, `storage`, `wasm`, `wasm-runtime`, `daemon`, `admin`, `custodyproof`, `layerxproof`, `utils`, `interchain`, the framework fork in `sdk`, and `programs` (whose guest SDK moves to `sdk/programs`). [`docs/MONOREPO.md`](docs/MONOREPO.md) describes the layout and the destinations of the trees that move; the target layout is specified in [`spec/monorepo-layout/spec.kvx`](spec/monorepo-layout/spec.kvx).

## Development and Release Process

### Overview

The chain, kernel, services, contracts and applications share this repository but use different toolchains and build targets. Read the relevant component documentation before making changes across their interfaces.

Toolchains: Go 1.25.8 (`go.mod`), a C17 compiler for the kernel (root `Makefile`), Rust 1.91.1 (`rust-toolchain.toml`), Solidity 0.8.27 with Foundry (`foundry.toml`), and Node.js for the npm workspaces in `package.json`. Replay qualification also needs GCC 13, Clang 18, Docker, an amd64 musl runner, and an AArch64 cross-compiler with QEMU; see [`docs/QUALIFICATION.md`](docs/QUALIFICATION.md).

### Build

The node, from the repository root:

```sh
make paxeer-build      # build/paxd through chain.mk
make paxd-build        # also builds the kernel library and programs runtime the node links, and stamps the release version
make -f chain.mk build # the chain.mk target directly
```

The LayerX kernel and its tools:

```sh
make build
make layerxd
make layerxctl
```

The services and workspaces:

```sh
make agent-build
make interop-build
make platform-build
make human-build
make programs-build
make workspace-build   # every workspace above plus the node and the root Foundry tree
```

### Test

```sh
make test                  # kernel test suite
scripts/ci/run-tests.sh    # the kernel suite at -O0 and -O2
make test-contracts
make paxeer-test           # node Go tests through chain.mk
make paxeer-lint           # node lint
make agent-test interop-test platform-test human-test programs-test
make bridge-test-evm bridge-test-solana
make ci                    # public audit, kernel tests, reproducible build, consensus symbol scan, sanitizers
make paxeer-ci             # node lint and tests
make monorepo-ci           # every workspace: build, test and lint
```

`make public-audit` runs `scripts/ci/public-repo-audit.sh`, which every change must pass. `scripts/ci/layout-check.sh` and `scripts/ci/vendor-verify.sh` check the layout and the vendored snapshots. A local pass is not authorization to deploy contracts, move custody, or handle real assets.

### Releases and Network Upgrades

See [CHANGELOG.md](CHANGELOG.md) for release notes. Network upgrades determine which native interfaces are active; refer to the precompile registrations and upgrade definitions when working with version-dependent functionality.

The LayerX merge remains a draft specification. The current kernel runs separately and settles checkpoints on the chain. Likewise, the directory layout above describes the current tree; planned moves under `core/` are documented in the monorepo guide and layout specification.

## Downloading and Shallow-Cloning the Monorepo

To fetch the default branch without downloading the full Git history:

```sh
git clone --depth 1 https://github.com/Sidiora-Labs/Paxeer-X-Network.git
cd Paxeer-X-Network
```

For a specific branch or tag, replace `<ref>` with the desired reference:

```sh
git clone --depth 1 --branch <ref> https://github.com/Sidiora-Labs/Paxeer-X-Network.git
```

## License

Licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE) for attribution notices.

Paxeer X Network is developed by Sidiora Labs.