# lxphost

`lxphost` is the Go/cgo host for the native LayerX kernel and Rust programs
runtime inside `paxd`. It registers kernel modules, validates and executes
activities, stages account and module state in the transaction's SDK store,
and provides typed reads for chain keepers and native precompiles. The kernel
runs within the Paxeer BFT network; a receipt is final when its block commits.

## Layout

- [`host.go`](host.go) — native module registration, host worker pool, activity and receipt types, execution, and blob storage.
- [`ctx.go`](ctx.go) — transaction context, sequence checks, account restoration, and SDK store persistence.
- [`execute.go`](execute.go) — activity construction for authenticated native callers and result error conversion.
- [`simulate.go`](simulate.go) — hypothetical execution in a cached context without publishing state.
- [`accountmap.go`](accountmap.go) — canonical account derivation, ownership lookup, and bank address mapping.
- [`authority_hash.go`](authority_hash.go) — kernel authority kind and authority hash binding.
- [`read.go`](read.go) — typed asset, account, authority, identity, stream, and schedule reads.
- [`read_program.go`](read_program.go) — program records, accounts, storage heads, wind-down, and sandbox lifecycle reads.
- [`sandbox_encoding.go`](sandbox_encoding.go) — canonical sandbox funding payload encoding.
- [`programs_schedules.go`](programs_schedules.go) — installation of program fee and metering schedules from native genesis producers.
- [`genesis.go`](genesis.go) — host store genesis validation, import, and export.
- [`module.go`](module.go) — SDK genesis module integration.
- [`code.go`](code.go) — Go error type for kernel result codes.
- [`code_names.go`](code_names.go) — generated kernel result code names.
- [`host_test.go`](host_test.go) — host execution tests and shared chain fixtures.
- [`store_keys_vector_test.go`](store_keys_vector_test.go) — assertions for canonical SDK store key vectors.
- [`internal/`](internal/) — native bootstrap record helpers in `m909fixture` for host tests.
- [`testdata/`](testdata/) — canonical store key vectors in `store_keys_v1.json`.

Additional Go tests cover authority, receipts, concurrency, simulation, genesis,
program lifecycle operations, restart persistence, and native codec agreement.

## Build and test

This package uses the root [Go module](../../go.mod) and
[Makefile](../../Makefile). Run these targets from the repository root:

| Command | Purpose |
| --- | --- |
| `make paxd-native-libs` | Build the C kernel archive, release programs sandbox archive, and evmone library required by the node. |
| `make paxd-build` | Build the node, including this host, with its native dependencies and release version metadata. |
| `make test-lxphost` | Build native dependencies and run `CGO_ENABLED=1 go test ./modules/lxphost/...`. |

The native bindings require cgo, C headers from [`include/`](../../include/),
generated headers under `build/generated`, the kernel archive under `build`,
and the programs sandbox archive under `programs/target/release` or
`programs/target/debug`. The linker also uses OpenSSL and system libraries
declared in `host.go`.

## Configuration

The embedding node supplies `Config` to `NewHost` in `host.go`: `Workers`,
`StoreKey`, `NetworkID`, `Epoch`, `ParameterVersion`, `ProgramsGuestABI`,
`TimestampWindowMs`, and `GasLimit`. Execution takes its block time and height
from the SDK context and its global sequence from `WithSequence` in `ctx.go`.
Genesis state is supplied as `StoreGenesis`, with `last_sequence` and `records`.
The host reads runtime settings from these Go inputs and chain state rather
than a package configuration file.

`CGO_ENABLED` controls cgo compilation in the test target. The store key vector
test reads `LXP_WRITE_VECTORS` to select regeneration of
[`testdata/store_keys_v1.json`](testdata/store_keys_v1.json).

## Documentation

- [Monorepo layout and build boundaries](../../docs/MONOREPO.md)
- [Kernel module registration and state binding](../../docs/wiki/Modules.md)
- [Programs and host integration](../../docs/wiki/Programs.md)
- [Chain modules](../../docs/system/chain-modules.md)
- [LayerX kernel on the documentation site](../../docs/site/paxeer-docs/public/markdown/layerx-kernel.md)
- [Modules on the documentation site](../../docs/site/paxeer-docs/public/markdown/modules.md)
