<!-- Source: https://docs.paxeer.app/installation/ -->

# Installation & Build

Build the Paxeer X node, deterministic LayerX kernel, hosted services, agent tools and Programs from the unified monorepo.

**Source:** `paxeer-monorepo/Makefile`, `chain.mk`, and `paxeer-monorepo/README.md`

## Build the complete network stack

The monorepo contains the Go chain node, the C17 kernel, Rust service workspaces, SDKs, and program tooling. The running network entry point is `paxd`. The kernel runs inside `paxd`. There is no standalone kernel daemon to build or run. That daemon is retired.

| Component | Build surface | Purpose |
| --- | --- | --- |
| Merged node | `make paxd-build` | `build/paxd` with linked native libraries and release version stamp |
| Kernel library and tools | `make build` | Kernel archive, genesis builder, and module registry, without rebuilding the node |
| Agent services | `make agent-build` | Agent runtime and its service boundaries |
| Human services | `make human-build` | Human API, providers, owner and security components |
| Platform services | `make platform-build-all` | Gateway, core boundary, registry, and hosted services |
| Programs and porting | `make programs-build` | Programs workspace and porting kits |

These commands run from the repository root. Use the Rust toolchain and dependency locks declared by each workspace, and the C compiler and native dependencies declared by the kernel build. Program guests target WebAssembly; install the target required by the selected guest before compiling it.

Installation provides binaries and images. Joining a network also requires its genesis, network identity, and role-specific credentials. Retain build provenance and artifact checksums with the deployment record. Continue with [node startup](https://docs.paxeer.app/run-node) and [operator roles](https://docs.paxeer.app/operators).

## Prerequisites

- **Go:** 1.25.8, as declared in `go.mod`, or the toolchain required by the selected revision

- **Rust:** 1.91.1, as declared in `rust-toolchain.toml`

- **C compiler:** C17 compiler for the kernel, via the root `Makefile`

- **Make:** Standard build tool

- **Git:** For cloning the repository

- **Solidity:** 0.8.27 with Foundry, for contract builds

- **Native toolchain:** GCC and the native libraries required by the selected build. Use `LEDGER_ENABLED=false` to disable the optional Ledger build tag.

## Repository Location

Clone the unified [Sidiora-Labs/paxeer-monorepo](https://github.com/Sidiora-Labs/paxeer-monorepo) repository. Chain sources, kernel sources, workspaces, and build configurations share this repository root. The name `Paxeer-X-Network` survives only as the Go module path.

```
git clone https://github.com/Sidiora-Labs/paxeer-monorepo.git
cd paxeer-monorepo
```

## Build the Chain Node

Build the merged node from the repository root. `make paxd-build` is the primary entry point. It builds `build/liblayerx.a` and `programs/target/release/liblayerx_programs_sandbox.a` before invoking `chain.mk`, and stamps the version from `tools/release/version.json`.

```
make paxd-build        # build/paxd, linked native libraries and release version stamp
make paxeer-build
make paxeer-lint
make paxeer-test
make paxeer-ci
```

`make paxeer-build` and `make -f chain.mk build` also build the linked native prerequisites and produce `build/paxd`, using the chain makefile's version defaults. Root `make build` builds only the kernel archive, genesis builder, and module registry. It does not produce a kernel daemon. Select the chain explicitly with `chain.mk`:

```
make -f chain.mk build
make -f chain.mk install
make -f chain.mk test
make -f chain.mk ci
```

### Build Output

- `build/paxd` from `make paxd-build`, `make paxeer-build`, or `make -f chain.mk build`.

- `$GOBIN/paxd` when GOBIN is set, otherwise `$(go env GOPATH)/bin/paxd`, from `make -f chain.mk install`.

- `make workspace-build` combines the kernel, services, node, app and tooling targets, and the root Foundry tree; `make monorepo-ci` delegates to the workspace CI target.

## Build Tags and Flags

### Ledger Support

Ledger hardware wallet support is enabled by default in `chain.mk`. To build without ledger:

```
make paxeer-build LEDGER_ENABLED=false
```

### Mock Balances (Testing)

For testing with mock balances:

```
make -f chain.mk install-mock-balances
```

### Benchmark Build

For benchmark runs:

```
make -f chain.mk install-bench
```

## Version Information

The Makefile resolves version from Git tags or branch names. The chain version resolver retains its historical namespaced tag pattern:

```
paxeer-network/vX.Y.Z
```

Version information is embedded in the binary at build time via linker flags:

- `Name`: paxeer

- `AppName`: paxd

- `Version`: Resolved from tag/branch

- `Commit`: Git commit SHA

Check version:

```
paxd version
```

## Static Linking

To build a statically-linked binary:

```
make paxeer-build LINK_STATICALLY=true
```

## Go Module

The repository root declares the chain Go module:

```
module github.com/Sidiora-Labs/Paxeer-X-Network

go 1.25.8
```

**Note:** The root `go.mod` and `go.sum` govern the chain Go dependencies. Rust components use their own workspace manifests and lockfiles.

## Foundry (Contracts)

Chain contracts under `contracts/src/` use `foundry.paxeer.toml`. Fetch the pinned dependencies and build from the repository root:

```
bash contracts/bootstrap-libs.sh
FOUNDRY_CONFIG=foundry.paxeer.toml forge build --offline --root .
```

Kernel contracts also live under `contracts/`, using repository-root `foundry.toml` and `tests/solidity/`. See `contracts/README.md` and the [contracts guide](https://docs.paxeer.app/contracts) for the two project configurations.

## Docker

Local Docker cluster targets are available. See [Docker documentation](https://docs.paxeer.app/docker) for:

- `make docker-cluster-start`

- `make run-local-node`

- Compose configurations in `docker/`

## Choose a Node Configuration

Use the local Docker configuration to try the built binary, or initialize a node with the genesis, peers, and mode for your chosen network. For an existing network, select the binary revision specified by its upgrade plan.

## Next Steps

- [Run a node](https://docs.paxeer.app/run-node)

- [Configure paxd](https://docs.paxeer.app/configuration)

- [Use Docker for local testing](https://docs.paxeer.app/docker)
