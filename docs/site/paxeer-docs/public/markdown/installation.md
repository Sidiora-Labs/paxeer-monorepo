<!-- Source: https://docs.paxeer.app/installation/ -->

# Installation & Build

Build the Paxeer X node, deterministic LayerX kernel, hosted services, agent tools and Programs from the unified monorepo.

**Source:** `Makefile`, `chain.mk`, and `README.md` at the repository root

## Build the complete network stack

The monorepo contains the Go chain node, the C17 kernel, Rust service workspaces, SDKs, and program tooling. The network entry point is `paxd`, which embeds the kernel and proposes blocks. A receipt is final when its block commits.

| Component | Build surface | Purpose |
| --- | --- | --- |
| Merged node | `make paxd-build` | `build/paxd` with linked native libraries and release version stamp |
| Kernel library and tools | `make build` | Kernel archive, genesis builder, and module registry, without rebuilding the node |
| Agent services | `make agent-build` | Agent runtime and its service boundaries |
| Human services | `make human-build` | Human API, providers, owner and security components |
| Platform services | `make platform-build-all` | Hosted services, SDKs, integrations, and samples |
| Programs and porting | `make programs-build` | Programs workspace and porting kits |

These commands run from the repository root. Use the Rust toolchain and dependency locks declared by each workspace, and the C compiler and native dependencies declared by the kernel build. Program guests target WebAssembly; install the target required by the selected guest before compiling it. The platform target also requires Node.js, Python, Maven/JVM, .NET and Swift, with dependencies prepared as declared in `services/platform/Makefile.inc`.

Installation provides binaries and images. Joining a network also requires its genesis, network identity, and role-specific credentials. Retain build provenance and artifact checksums with the deployment record. Continue with [node startup](https://docs.paxeer.app/run-node) and [operator roles](https://docs.paxeer.app/operators).

## Prerequisites

- **Go:** 1.25.8, as declared in `go.mod`, or the toolchain required by the selected revision

- **Rust:** 1.91.1, as declared in `rust-toolchain.toml`

- **C compiler:** C17 compiler for the kernel, via the root `Makefile`

- **Build tools:** Make and jq; the node also builds the native EVM library through `make evmone-lib` in `chain.mk`

- **Git:** For cloning the repository

- **Solidity:** 0.8.27 with Foundry, for contract builds

- **Native toolchain:** GCC, a C++ compiler and CMake for the native EVM library. The `evmone-lib` target builds that library on Linux/amd64 and fetches its declared source dependencies. Use `LEDGER_ENABLED=false` to disable the optional Ledger build tag.

## Repository Location

Clone the [Sidiora-Labs/paxeer-monorepo](https://github.com/Sidiora-Labs/paxeer-monorepo) repository. Chain sources, kernel sources, workspaces, and build configurations share this repository root. The root Go module is `github.com/Sidiora-Labs/Paxeer-X-Network`.

```
git clone https://github.com/Sidiora-Labs/paxeer-monorepo.git
cd paxeer-monorepo
```

## Build the Chain Node

Build the node from the repository root. `make paxd-build` builds `build/liblayerx.a`, `programs/target/release/liblayerx_programs_sandbox.a`, and the native EVM library before invoking `chain.mk`, and stamps the version from `tools/release/version.json`. The root node build disables Go module proxy downloads, so populate the declared dependency cache before using it.

```
make paxd-build        # build/paxd, linked native libraries and release version stamp
make paxeer-build
make paxeer-lint
make paxeer-test
make paxeer-ci
```

`make paxeer-build` and `make -f chain.mk build` also build the linked native prerequisites and produce `build/paxd`, using the chain makefile's version resolver. Root `make build` builds the kernel archive, genesis builder, and module registry. Select the chain explicitly with `chain.mk`:

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

The chain version resolver compares matching Git tags and versioned branch names, and uses `tools/release/version.json` when no matching tag exists. Its tag pattern is:

```
paxeer-network/vX.Y.Z
```

Version information is embedded in the binary at build time via linker flags:

- `Name`: paxeer

- `AppName`: paxd

- `Version`: The selected version; `make paxd-build` selects it from `tools/release/version.json`

- `Commit`: Git commit SHA

Check version:

```
paxd version
```

## Static Linking

Request static external linking for the Go binary with the flag below. The native EVM loader in `engine/executor/lib/evmlib.go` still loads a platform-specific shared library:

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
