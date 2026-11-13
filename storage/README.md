# PaxDB

PaxDB is the storage layer of the Paxeer X chain (`paxd`). This directory holds the Go packages for active state commitment, historical state queries, ledger data, write-ahead logging, and database tools. The node connects these packages through [`node/app.go`](../node/app.go) and [`node/paxdb.go`](../node/paxdb.go). Paxeer is one BFT network with the kernel integrated into the node; a receipt is final once its block is committed.

## Layout

| Path | Contents |
| ---- | -------- |
| [`common`](common) | Shared errors, iterators, store keys, metrics, threading, utilities, and test helpers |
| [`config`](config) | SC, SS, receipt-store, and write-mode configuration and TOML templates |
| [`db_engine`](db_engine) | Key/value engines: PebbleDB, RocksDB, the [`litt`](db_engine/litt) store, and a sharded cache (`dbcache`) |
| [`ledger_db`](ledger_db) | Block, transaction, receipt, and event storage |
| [`proto`](proto) | Protobuf messages and Go bindings for changelogs, changesets, commit information, and memiavl |
| [`state_db`](state_db) | State commitment in [`sc`](state_db/sc), versioned historical state in [`ss`](state_db/ss), and workloads in [`bench`](state_db/bench), including [`cryptosim`](state_db/bench/cryptosim) |
| [`tools`](tools) | The `paxdb` operator CLI, database benchmarks, and RPC benchmarks; see [`tools/README.md`](tools/README.md) |
| [`wal`](wal) | Write-ahead log and changelog used by the SC layer |
| [`README.md`](README.md) | Storage overview, build commands, configuration, and documentation links |

### Architecture

The design follows the [Cosmos store v2 ADR](https://github.com/cosmos/cosmos-sdk/blob/main/docs/architecture/adr-065-store-v2.md). Separate SC and SS backends provide active state commitment and versioned historical queries in place of the single [IAVL](https://github.com/cosmos/iavl) database of a stock Cosmos SDK chain. The backends are swappable, and SS keeps raw key/value pairs to reduce disk use and write amplification. Historical proofs depend on retained commitment data and backend support; the raw SS data does not independently validate a historical state commitment.

### State Commitment (SC)

The SC layer provides:

- The root app hash for each new block
- The data access layer for transaction execution
- An API to import/export chain state for state sync
- Proofs for retained heights where the selected backend supports them

The memiavl implementation is a fork of MemIAVL from [Cronos](https://github.com/crypto-org-chain/cronos). To stay compatible with Cosmos SDK chains, it uses the same data structure (a Merkleized AVL tree), but represents the tree as memory-mapped flat files instead of persisting it as key/values in a database engine. See [`state_db/sc/memiavl/README.md`](state_db/sc/memiavl/README.md).

The [`composite`](state_db/sc/composite) store routes module state between memiavl and [`flatkv`](state_db/sc/flatkv), whose integrity checks use LtHash. The `sc-write-mode` setting (see [`config/write_mode.go`](config/write_mode.go)) selects routing and migration stages: EVM state first, then other modules except bank, then bank. The endpoint modes are `memiavl_only` and `flatkv_only`. The [`hashvault`](state_db/sc/hashvault) package durably records a block hash at each height and refuses a different hash for the same retained height.

### State Store (SS)

The SS layer provides a modular storage backend for versioned raw key/value pairs in an embedded database:

- Queries for versioned raw key/value pairs
- Versioned CRUD operations
- Versioned batching
- Versioned iteration
- Pruning

The [`ss`](state_db/ss) packages also provide Cosmos and EVM routing, history offload, and backend selection.

PebbleDB is the default SS backend (`pebbledb`). RocksDB (`rocksdb`) is available when the binary is built with `-tags=rocksdbBackend`.

## Build and test

Storage belongs to the root Go module, declared in [`go.mod`](../go.mod). The root [`Makefile`](../Makefile) delegates the following targets to [`chain.mk`](../chain.mk). Run them from the repository root:

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds the node and its native dependencies. `paxeer-test` runs the Go package test groups, including storage packages. These targets use the local Go dependency cache.

The storage tools and benchmarks have their own Makefiles. From this directory:

```sh
# Install the paxdb operator CLI.
make -C tools install

# Build the LittDB CLI and run its tests.
make -C db_engine/litt build
make -C db_engine/litt test

# Build the state and block benchmark binaries and their logging helpers.
make -C state_db/bench/cryptosim build
make -C ledger_db/block/blocksim build
```

[`tools/Makefile`](tools/Makefile) also provides `install-all` for the optional RocksDB and SQLite build tags; it requires the native libraries and headers listed there. The other targets are declared in [`db_engine/litt/Makefile`](db_engine/litt/Makefile), [`state_db/bench/cryptosim/Makefile`](state_db/bench/cryptosim/Makefile), and [`ledger_db/block/blocksim/Makefile`](ledger_db/block/blocksim/Makefile).

## Configuration

The node supplies storage settings from `app.toml` through application options. [`config/toml.go`](config/toml.go) defines the storage templates, and [`node/paxdb.go`](../node/paxdb.go) connects them to the stores:

- `[state-commit]`: `sc-enable`, `sc-directory`, `sc-write-mode`, snapshot settings, historical proof limits, and migration settings.
- `[state-commit.flatkv]`: `fsync`, `async-write-buffer`, `snapshot-interval`, and `snapshot-keep-recent`.
- `[state-store]`: `ss-enable`, `ss-db-directory`, `ss-backend`, `ss-async-write-buffer`, `ss-keep-recent`, `ss-prune-interval`, `ss-import-num-workers`, `evm-ss-db-directory`, `evm-ss-split`, and `evm-ss-separate-dbs`.
- `[receipt-store]`: `rs-backend`, `db-directory`, `async-write-buffer`, and `prune-interval-seconds`. Receipt retention comes from the root `min-retain-blocks` setting.

The benchmark commands read JSON configuration files supplied as arguments. Examples are [`cryptosim/config/basic-config.json`](state_db/bench/cryptosim/config/basic-config.json), [`blocksim/config/debug.json`](ledger_db/block/blocksim/config/debug.json), and [`litt/benchmark/config/basic-config.json`](db_engine/litt/benchmark/config/basic-config.json). The cryptosim and blocksim launchers set the logging environment variables `PAX_LOG_OUTPUT` and `PAX_LOG_LEVEL` from their JSON configuration using their `configure-logger` helpers.

## Documentation

- [Storage documentation](../docs/site/paxeer-docs/public/markdown/storage.md) and its [documentation site page](../docs/site/paxeer-docs/src/app/storage/page.tsx)
- [Node configuration](../docs/site/paxeer-docs/public/markdown/configuration.md)
- [Node operation and recovery](../docs/site/paxeer-docs/public/markdown/operators.md)
- [Data availability and receipt retention](../docs/wiki/DataAvailability.md)
- [MemIAVL design and snapshots](state_db/sc/memiavl/README.md)
- [LittDB API and design](db_engine/litt/README.md)
- [PaxDB tools](tools/README.md)
