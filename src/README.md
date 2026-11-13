# LayerX kernel

This directory holds the C17 execution kernel of the Paxeer network and Go
helpers for account and history reads. The kernel validates activities, resolves
authority, executes native modules, applies ledger transfers, and produces
receipts and state commitments. It runs inside `paxd` through
[`modules/lxphost/`](../modules/lxphost/); the kernel participates in block
proposal, and receipts become final when their blocks commit.

## Layout

| Path | Contents |
| --- | --- |
| [codec/](codec/) | Canonical binary encoding and decoding primitives. |
| [crypto/](crypto/) | Hashing, Ed25519 and secp256k1 signatures, Merkle proofs, and constant-time helpers. |
| [ledger/](ledger/) | Account identifiers and registry, transfer validation and application, sends, receipts, and ledger journals. |
| [modules/](modules/) | Native asset, escrow, budget, stream, service, trading, governance, bridge, and programs implementations, plus account documentation and Go history helpers. |
| [network/](network/) | Payment requirement and invoice codecs, send and receive helpers, oracle encoding, and a verification worker pool. |
| [protocol/](protocol/) | Kernel execution and host integration, activities, receipts, fees, metering, genesis, deterministic arithmetic, arenas, and batch time. |
| [sequencer/](sequencer/) | Activity admission, batch header encoding and signatures, and batch Merkle roots used by node execution. |
| [state/](state/) | Identity and delegated authority, transactional journals, snapshots, state roots, and state proofs. |
| [storage/](storage/) | Durable logs, recovery, snapshot files, SQLite projections, and storage fault hooks. |

Public C interfaces live in [`include/layerx/`](../include/layerx/). The
programs module calls the Rust runtime in [`programs/`](../programs/).

## Build and test

Run these targets from the repository root; they are declared in the root
[`Makefile`](../Makefile):

| Command | Purpose |
| --- | --- |
| `make build` | Build the C kernel archive and the genesis and module registry tools, including the programs runtime prerequisite. |
| `make test` | Run the native suite covering protocol, codec, cryptography, arithmetic, storage, admission, authority, and economic modules. |
| `make test-kernel` | Run kernel execution tests. |
| `make test-storage` | Run log, durability, recovery, SQLite projection, and projection rebuild tests. |
| `make test-lxphost` | Build native prerequisites and test the Go host integration. |
| `make paxeer-test` | Run the root Go test groups through [`chain.mk`](../chain.mk), including consumers of the history helpers. |

The kernel archive is `build/liblayerx.a`. The SQLite projection in
[`storage/lxp_projection.c`](storage/lxp_projection.c) is compiled separately
by its test targets and links SQLite. Kernel tools and execution tests link the
programs sandbox archive. The build uses a C17 compiler, Make, Python, Rust/Cargo,
OpenSSL, and pthreads; the Go targets also require Go and the node's native
dependencies. See [`programs/README.md`](../programs/README.md) for the runtime.

## Configuration

The kernel receives network identity, module registrations, fee parameters,
authority, and batch execution context through the host APIs declared in
[`include/layerx/`](../include/layerx/). Its sources read no environment variables.
The Go history client receives its endpoint through `Client.Endpoint` in
[`modules/history/indexer.go`](modules/history/indexer.go).

Storage APIs receive log, snapshot, database, and migration paths from callers.
The projection loads a SQL migration supplied to `lxp_projection_open`; its
tests use [`migrations/0001_projection.sql`](../migrations/0001_projection.sql).

Build overrides in the root Makefile include `CC`, `AR`, `BUILD_DIR`,
`OPT_LEVEL`, `LXP_REVISION`, `EXTRA_CFLAGS`, `EXTRA_LDFLAGS`, and `RUN_PREFIX`.

## Documentation

- [Network overview](../docs/paxeer-network.md)
- [Monorepo layout and build boundaries](../docs/MONOREPO.md)
- [Kernel module interfaces and activity types](../docs/system/kernel-modules.md)
- [Kernel and node test coverage](../docs/QUALIFICATION.md)
- [LayerX kernel documentation site page](../docs/site/paxeer-docs/src/app/layerx-kernel/page.tsx)
- [LayerX kernel Markdown page](../docs/site/paxeer-docs/public/markdown/layerx-kernel.md)
- [Sequencing and finality](../docs/site/paxeer-docs/public/markdown/sequencing.md)
