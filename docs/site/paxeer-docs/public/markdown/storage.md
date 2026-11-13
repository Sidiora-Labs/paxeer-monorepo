<!-- Source: https://docs.paxeer.app/storage/ -->

# Storage

State commitment, historical queries, receipt retention, and backend configuration in Paxeer.

## Overview

Paxeer storage separates active state commitment from historical state queries and receipt storage. Each has its own configuration and retention behavior. The implementation is under `Paxeer-X-Network/storage/`.

## State Commitment

The state-commit layer under `storage/state_db/sc/` includes MemIAVL, FlatKV, and a composite store that routes state operations. It participates in application commitment, snapshot import/export, and supported proof queries.

The default write mode is `memiavl_only`. Other named modes support staged migration to FlatKV. Select routing modes according to the existing data layout and the chain's migration plan.

```
# app.toml
[state-commit]
sc-enable = true
sc-write-mode = "memiavl_only"
```

**Source:** `storage/config/sc_config.go`, `storage/config/write_mode.go`, `storage/state_db/sc/composite/store.go`.

## Historical State Store

The state-store layer maintains versioned key/value data for historical queries. Configure imported data and retention for the historical range you need. Commitment proofs also require the corresponding commitment-store history.

```
# app.toml
[state-store]
ss-enable = true
ss-backend = "pebbledb"
ss-keep-recent = 100000
```

The default backend is Pebble and the default retention is 100,000 versions. `ss-keep-recent = 0` retains all available versions. The config also supports a separate EVM historical store through `evm-ss-split`; its data routing and missing-key behavior differ from the unsplit store.

**Source:** `storage/config/ss_config.go`, `storage/config/toml.go`, `storage/state_db/ss/`.

## RocksDB Backend

The optional RocksDB backend uses user-defined timestamps and column families for versioned state. Build it from the Paxeer subtree:

```
make build-rocksdb
make install-rocksdb
```

After installing the compatible binary, set `ss-backend = "rocksdb"` under `[state-store]`. When changing backends, import or restore data in the target backend's format.

Compare backends with the same revision, hardware, retained history, and request workload. Measure historical-query latency, iteration time, disk usage, and write throughput for the operations your application uses.

**Source:** `chain.mk` — `build-rocksdb`, `install-rocksdb` (the root `Makefile` delegates to `chain.mk` through `PAXEER_MAKE`); `storage/db_engine/rocksdb/mvcc/opts.go`.

## Receipts and Blocks

```
# app.toml: root setting must precede section headers
min-retain-blocks = 100000

[receipt-store]
rs-backend = "pebbledb"
```

The receipt configuration accepts `pebbledb` or its `pebble` alias. Set receipt retention with the root `min-retain-blocks` setting and historical state retention with `ss-keep-recent`.

`--mode archive` configures block and SS retention to zero, preserving imported history and subsequent state. Start with a dataset covering the historical range you plan to serve.

**Source:** `storage/config/receipt_config.go`, `node/params/config.go` — `setArchiveTypeAppConfig`.

## Snapshots and Recovery

Application snapshots, commitment-store snapshots, and filesystem backups are distinct. Use the mechanism and directory layout matching your build. Stop a node before replacing its data, preserve validator last-sign state, and keep a compatible copy of the configuration and genesis.

Changelogs and write-ahead logs support storage replay and recovery. Follow the recovery procedure for the selected store, and preserve consensus signer state alongside the validator key.

**Source:** `storage/wal/`, `storage/state_db/sc/composite/`, `consensus/privval/file.go`.

## Inspection Tools

The separate PaxDB command provides storage inspection and benchmark operations. Build or run it from the Paxeer subtree, using its help to select the correct database path and height:

```
go run ./storage/tools/cmd/paxdb --help
go run ./storage/tools/cmd/paxdb dump-iavl --help
go run ./storage/tools/cmd/paxdb dump-db --help
```

Beyond inspection, `paxdb` provides `dump-flatkv` and `import-flatkv-from-memiavl` for FlatKV export and migration, `migrate-evm-status` for EVM status migration, `prune` for version pruning, and `replay-changelog` for changelog replay. Run these operations through `paxdb`. Use an offline copy when an operation needs exclusive database access.

**Source:** `storage/tools/cmd/paxdb/main.go`, `storage/tools/cmd/paxdb/operations/`.

## Next Steps

- [Configure node storage](https://docs.paxeer.app/configuration)

- [Operate and recover a node](https://docs.paxeer.app/operators)

- [Understand block commitment](https://docs.paxeer.app/consensus)
