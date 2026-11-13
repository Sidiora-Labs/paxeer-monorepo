# Core dependencies

`core` holds vendored dependencies for Paxeer's execution, storage, logging and utility code. The network runs through `paxd`, with the kernel executing inside the node and participating in block proposal; receipts become final when their blocks commit. The root [Go module](../go.mod) selects the local Geth fork in `core/deps/geth` for Ethereum execution and RPC types.

## Layout

- [deps/](deps/) — vendored source trees, with module manifests and `VENDOR.kvx` provenance records.
- [deps/cosmwasm/](deps/cosmwasm/) — CosmWasm Rust workspace, contract libraries, virtual machine and example contracts.
- [deps/db-backend/](deps/db-backend/) — common database interface, persistent and in-memory backends, prefix databases and remote database support.
- [deps/geth/](deps/geth/) — Go Ethereum fork with EVM execution, transaction types, RPC, storage and command-line tools.
- [deps/log/](deps/log/) — structured logging using `log/slog`, with per-logger runtime level control.
- [deps/tm-db/](deps/tm-db/) — custom database backends, including the Arweave read backend and its local index.
- [deps/utils/](deps/utils/) — generic helpers for in-place and immutable slice appends.

## Build and test

`core` has no directory-wide Makefile. Each dependency has its own manifest. The root [Makefile](../Makefile) and [chain.mk](../chain.mk) build and test the node that consumes the local Geth replacement:

| Working directory | Command | Purpose |
| --- | --- | --- |
| Repository root | `make paxd-build` | Build native prerequisites and the linked node with release version metadata. |
| Repository root | `make test-paxd-domain` | Test the native host, node, EVM keeper and RPC integration. |
| `core/deps/geth` | `make all` | Build the fork's packages and executables. |
| `core/deps/geth` | `make test` | Build and run the fork's tests. |
| `core/deps/db-backend` | `make test` | Test the default database backends. |
| `core/deps/db-backend` | `make test-all` | Test all supported backends with their native libraries installed. |
| `core/deps/cosmwasm` | `task build` | Build workspace crates using the declared Rust toolchain. |
| `core/deps/cosmwasm` | `task test` | Test workspace crates using the declared Rust toolchain. |

The dependency commands are declared in the [Geth Makefile](deps/geth/Makefile), [database makefile](deps/db-backend/makefile) and [CosmWasm Taskfile](deps/cosmwasm/Taskfile.yml). CosmWasm's [workspace manifest](deps/cosmwasm/Cargo.toml) includes the packages and excludes the example contracts. Node integration tests cover their selected consumers; dependency suites run separately in their own directories.

## Configuration

The logging library reads these environment variables during initialization in [seilog.go](deps/log/seilog.go):

- `PAX_LOG_LEVEL` — default logging level.
- `PAX_LOG_FORMAT` — output format.
- `PAX_LOG_OUTPUT` — output destination.
- `PAX_LOG_ADD_SOURCE` — source-location reporting.

The vendored Geth command reads a TOML file selected with `--config`; its filename is supplied by the caller, as defined in [config.go](deps/geth/cmd/geth/config.go). Its platform data-directory and terminal helpers read `HOME`, `LOCALAPPDATA`, `APPDATA` and `TERM`. These settings belong to the vendored tools and libraries. Node configuration is described in the [documentation site's configuration page](../docs/site/paxeer-docs/public/markdown/configuration.md).

## Documentation

- [Monorepo layout](../docs/MONOREPO.md) — dependency placement and node build boundaries.
- [Network overview](../docs/paxeer-network.md) — the node and embedded kernel.
- [Qualification](../docs/QUALIFICATION.md) — build and test coverage.
- [RPC implementation](../docs/system/rpc.md) — node RPC methods and their sources.
- [EVM module on the documentation site](../docs/site/paxeer-docs/public/markdown/evm.md) — execution and receipt integration.
- [Run a node on the documentation site](../docs/site/paxeer-docs/public/markdown/run-node.md) — node setup and operation.
