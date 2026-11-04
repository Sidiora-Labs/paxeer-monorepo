# Installation

The LittDB CLI is built from the Paxeer monorepo. It operates on storage files; it does not connect
to the network JSON-RPC. See the [LittDB API](../README.md#api) and [architecture](architecture.md)
for the embedded library and its durability guarantees.

## Building from source

Use the Go toolchain required by the repository's [go.mod](../../../../go.mod) (currently Go 1.25.8).
From the monorepo root:

```bash
cd storage/db_engine/litt
make build
./bin/litt --help
```

The [Makefile](../Makefile) runs `go build -o ./bin/litt ./cli`. The resulting binary is
`storage/db_engine/litt/bin/litt`, relative to the repository root.

### Optional: Shortcuts

Use the absolute location of your checkout when defining an alias or extending `PATH`:

```bash
alias litt='/path/to/paxeer-monorepo/storage/db_engine/litt/bin/litt'
export PATH="$PATH:/path/to/paxeer-monorepo/storage/db_engine/litt/bin"
```

Alternatively, copy or symlink that binary into a directory already on your `PATH`. Examples below
assume `litt` resolves to this binary. Remote transfers also require SSH access, a trusted host-key
entry, and `rsync` on the participating machines. The receiver must support `rsync --fsync` and
GNU `sync -f`, which the transfer uses before acknowledging durable remote files.

# Sources and Destinations

`--src` (alias `-s`) names a local database or snapshot root. `--dst` (alias `-d`) names a destination
root: local for `rebase`, remote for `push` and `sync`. Repeat the flag for each root. These are roots
containing table directories, not the table or `segments` directories themselves; see the
[filesystem layout](filesystem_layout.md).

For a regular database, supply every root in the [`litt.Config.Paths`](../littdb_config.go) passed to
`littbuilder.NewDB`. For example, an application configured with:

```go
config, err := litt.DefaultConfig("/data0", "/data1", "/data2")
```

uses the following source selection:

```bash
litt ls --src /data0 --src /data1 --src /data2
```

The CLI reads explicit flags, not node environment variables or a node configuration file. This
package does not define a node flag-to-`Config.Paths` mapping; obtain the actual roots from the
application opening LittDB. Do not infer them from a validator example or assume a data-directory
suffix. Missing roots can cause incomplete results or missing-file errors; do not rely on every
command detecting an incomplete source list.

A rolling snapshot instead has a single source root: `Config.SnapshotDirectory`. Its symlinks refer
to files retained on the database volumes. Keep those volumes accessible while consuming the snapshot.

Stop the application before using ordinary database roots with the CLI. Commands take local locks;
removing locks to bypass a running writer can corrupt data. A published snapshot is the supported
source for backups while the application remains running. Transfers do not lock remote destination
roots, so keep the receiving database stopped and serialize destination maintenance.

# Subcommands

The registered commands and flags are defined in [litt_cli.go](../cli/litt_cli.go).

## `litt --help`

Print command help. Use `litt <command> --help` for that command's flags. Global diagnostic flags
precede the command; subcommand source/destination flags follow it.

## `litt ls`

List table names alphabetically from the provided roots:

```bash
litt ls --src /data0 --src /data1 --src /data2
```

The command logs the discovered names. It does not query the running node.

## `litt table-info`

Inspect one named table, including key count, byte size, snapshot status, seal times, segment indices
and keymap type:

```bash
litt table-info --src /data0 --src /data1 --src /data2 tableA
```

Supply exactly one table-name argument. Table names use ASCII letters, digits, underscores and dashes.
This is file-based inspection, not a retention-policy configuration command.

## `litt rebase`

Move a stopped database's files from the source roots to a new set of local roots. Roots can overlap:

```bash
litt rebase --src /data0 --src /data1 --src /data2 --dst /data2 --dst /data3 --dst /data4
```

The operation is resumable: after interruption, rerun the same source and destination lists. Missing
old roots can be expected after a completed transfer. `--preserve` retains original files; `--quiet`
reduces output. Rebasing a symlinked snapshot is unsupported.

When destinations include all source roots, there are no removed roots to move. Adding roots alone
requires no rebase, but still requires stopping the application, changing `Config.Paths` and reopening
it. There is no live root-membership setter. Changing the table's sharding factor is a separate
operation. After a rebase, restart with every destination root in `Config.Paths` and supply the
per-table settings again.

## `litt benchmark`

Run the benchmark with exactly one JSON configuration filename:

```bash
litt benchmark /path/to/benchmark.json
```

This example matches the fields in [basic-config.json](../benchmark/config/basic-config.json):

```json
{
  "LittConfig": {
    "Paths": ["~/benchmark/volume1", "~/benchmark/volume2", "~/benchmark/volume3"],
    "SnapshotDirectory": "~/snapshot"
  },
  "MaximumWriteThroughputMB": 1024,
  "MetricsLoggingPeriodSeconds": 1
}
```

The [loader](../benchmark/config/benchmark_config.go) starts from `DefaultBenchmarkConfig`, overlays
the JSON and rejects unknown fields. JSON does not allow trailing commas. Use dedicated benchmark
paths. Omitted fields retain defaults; `TimeLimitSeconds` defaults to zero, so the benchmark runs
until interrupted. A nonempty `SnapshotDirectory` retains snapshot files that must be consumed or
pruned. Keep the seed and random-pool size unchanged when resuming the same benchmark data.

## `litt prune`

Remove segments older than `--max-age`, in seconds, from a stopped database or snapshot:

```bash
litt prune --src /data0 --src /data1 --src /data2 --max-age 3600
```

Age is measured from segment seal time; this is segment-level retention, not an exact per-key
expiration cutoff. Repeat `--table tableA` to restrict pruning to selected tables; by default all
discovered tables are considered. Snapshot pruning respects the publication upper bound. Snapshots
and remote archives are not automatically pruned by the originating database's TTL.

Serialize pruning with transfers that use the same roots. In particular, do not run archive pruning
while another machine is pushing to that archive: remote destination locks are not acquired by push.

## `litt push`

Transfer segment files over SSH with `rsync`. For a running database, enable
`Config.SnapshotDirectory` and use that single snapshot root as the source:

```bash
litt push --src /snapshot --dst /backup1 --dst /backup2 --dst /backup3 user@backup-host
```

This consumes published sealed segments, not the active mutable segment. By default, after successful
transfer it releases the contiguous acknowledged snapshot prefix and updates the consumption boundary.
Unpublished segments remain owned by the snapshot publisher. Releasing snapshot links does not delete
live database entries. It does reclaim the snapshot's retained files, so the next backup transfers
newly published data. `rsync` can also reuse files already at the destination when retrying an
interrupted transfer.

Use `--no-gc` to retain the source files. Without it, an ordinary, non-snapshot source is rejected
with `--no-gc is required when pushing a non-snapshot table`. Remote destinations may span multiple
roots on the same host; list all of them on each transfer so existing file placement is preserved.

SSH flags include `--key` (`-i`, default `~/.ssh/id_rsa`), `--port` (`-p`, default 22), and
`--known-hosts` (`-k`, default `~/.ssh/known_hosts`). `--threads` defaults to 8 and must be positive;
`--throttle` limits aggregate transfer bandwidth and defaults to zero (unlimited). `--quiet` reduces
output. Verify host trust before transferring.

### Restoring from a Backup

Run this on the backup machine, with every archive root as a source and every target root as a
destination. Stop the target application and use dedicated restore roots so unrelated old segments
are not combined with the restored data:

```bash
litt push --no-gc --src /backup1 --src /backup2 --src /backup3 \
  --dst /restore0 --dst /restore1 user@target-host
```

`--no-gc` is required because a received archive contains ordinary segment files, not snapshot
symlinks. It also preserves the archive during restoration. The source database must likewise be
stopped if restoring from ordinary database roots rather than an archive.

After a successful transfer, configure the target application with `Config.Paths` containing
`/restore0` and `/restore1`, then open the restored tables with their intended `TableConfig` values.
TTL, cache sizes and other per-table runtime settings are not recovered from the archive. Reopening
rebuilds a missing keymap from the transferred segment key files. A snapshot backup contains only
published sealed segments; it does not include writes still in the mutable segment.

### Backup Garbage Collection

Archives grow until explicitly pruned. On the backup machine, while no transfer is writing the roots:

```bash
litt prune --src /backup1 --src /backup2 --src /backup3 --max-age 86400
```

Choose retention for the restore history you need. This command deletes old archive segments.

## `litt sync`

Periodically push to one remote host, optionally pruning the remote archive:

```bash
litt sync --src /snapshot --dst /backup1 --dst /backup2 --period 300 user@backup-host
```

It accepts the transfer flags described for `push`. `--period` is in seconds and defaults to 300.
A nonzero `--max-age` enables remote pruning in seconds; `--litt-binary` selects the remote CLI
(default `litt`), which must be installed there for pruning. Ordinary source roots still require
`--no-gc` and a stopped source application. Avoid other processes transferring to or pruning the
same destination roots concurrently.

## `litt unlock`

Remove stale local lock files only after confirming no process is using any of the roots:

```bash
litt unlock --src /data0 --src /data1 --src /data2
```

The command prompts before removal; `--force` bypasses that prompt. It does not stop another process
or make concurrent access safe. See [file locking](../util/file_lock.go) for the implementation.
