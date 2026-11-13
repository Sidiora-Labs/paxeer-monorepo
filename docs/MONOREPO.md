# Monorepo layout

This repository contains the Paxeer network node, its embedded kernel, contracts,
services, SDKs and applications. `paxd` runs BFT consensus and calls the kernel
for execution and block proposal. Receipts are final when their blocks commit.
Paxeer mainnet-beta uses EVM chain ID `125` (`0x7d`); sub-chains are tenant
domains in the same network. See the [network overview](paxeer-network.md).

## Root layout

| Path | Contents |
| --- | --- |
| [src/](../src/), [include/](../include/) | C17 kernel implementation and public headers |
| [daemon/paxd/](../daemon/paxd/) | Production Go node entry point |
| [node/](../node/), [consensus/](../consensus/) | Node lifecycle, ABCI integration and BFT consensus |
| [modules/](../modules/), [precompiles/](../precompiles/) | Chain modules and EVM precompiles |
| [modules/lxphost/](../modules/lxphost/) | Go-to-native kernel and programs host integration |
| [rpc/](../rpc/), [api/](../api/) | JSON-RPC implementation and protobuf APIs |
| [storage/](../storage/), [sdk/](../sdk/) | Chain storage and the Go chain SDK |
| [core/deps/](../core/deps/) | Pinned upstream dependencies |
| [programs/](../programs/) | Rust programs runtime, sandbox and guests |
| [contracts/](../contracts/) | Solidity contracts and project-specific tests |
| [services/](../services/) | Agent, platform, launchpad, wallet and other services |
| [apps/](../apps/) | Wallet, explorer, inference peer and other applications |
| [sdk/integrations/](../sdk/integrations/), [services/platform/examples/](../services/platform/examples/) | Integration packages and example consumers |
| [docker/](../docker/), [deploy/](../deploy/) | Image definitions and deployment configuration |
| [tools/](../tools/) | Build, release, deployment and development utilities |
| [tests/](../tests/) | Native, integration and conformance tests |
| [docs/](./), [spec/](../spec/) | Documentation and protocol specifications |

The root [Makefile](../Makefile) builds the kernel and coordinates language
workspaces. [chain.mk](../chain.mk) owns Go node targets.
[go.mod](../go.mod) declares the root Go module and local dependency replacements.
[package.json](../package.json) declares the root npm workspaces.
Applications with their own package manifests use their declared package manager
and lockfile.

## Build boundaries

Run these commands from the repository root with the required toolchains installed:

| Target | Result |
| --- | --- |
| `make build` | Build the C kernel library and tools |
| `make paxd-native-libs` | Build the kernel and release programs sandbox archives and the evmone shared library needed by the node |
| `make paxeer-build` | Build the native prerequisites and linked Go node |
| `make paxd-build` | Build the node with release version metadata |
| `make programs-build` | Build the programs workspace |
| `make agent-build` | Build the agent workspace |
| `make workspace-build` | Build the repository workspaces |
| `make workspace-test` | Run workspace tests |
| `make workspace-lint` | Run workspace lint checks |
| `make monorepo-ci` | Run source audit, inventory, workspace builds, tests and lint |

The kernel uses C17, the root Go module declares Go 1.25.8, and
[rust-toolchain.toml](../rust-toolchain.toml) pins Rust 1.91.1.
The root Foundry configurations pin Solidity 0.8.27.
`foundry.toml` defines the kernel-contract project; `make test-contracts`
selects its `names` and `governance` profiles. `foundry.paxeer.toml` defines
the separate `contracts/src` and `contracts/test` project, selected with
`FOUNDRY_CONFIG=foundry.paxeer.toml`.
The node's native integration is declared in
[modules/lxphost/host.go](../modules/lxphost/host.go).
See [Qualification](QUALIFICATION.md) for the test suites and their coverage.

## Trust boundaries

The embedded kernel and Go node share consensus and committed-state authority.
Hosted services, agents and applications consume that network; local HTTP
success and off-chain inference receipts do not establish committed finality.
Public node JSON-RPC uses the node root URL. The
[hosted gateway contract](../services/platform/hosted/gateway/openrpc.json)
defines hosted identity and faucet operations.

Inference hosts and peers run off-chain. The
[inference market guides](agent/inference-market/README.md) describe program
operations, metered streams, client routes and development attestation limits.
The [inference peer guide](../apps/inference-peer/README.md) describes the
browser and native peer interfaces.

## Further reading

- [Network overview](paxeer-network.md)
- [Documentation index](README.md)
- [Docker local cluster](../docker/README.md)
- [Launchpad deployment](../deploy/launchpad/README.md)
- [Agent compatibility](agent/COMPATIBILITY.md)
