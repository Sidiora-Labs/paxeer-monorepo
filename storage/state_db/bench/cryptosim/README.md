# Cryptosim

Cryptosim generates local database load shaped like cryptocurrency transfers.
It exercises the database wrappers and optionally the production receipt store;
it does not execute EVM bytecode, submit chain transactions or measure consensus
finality. See the [benchmark overview](../README.md) for the simpler write
microbenchmarks and the [migration overview](../../sc/migration/README.md) for
state-backend migration.

## Running Cryptosim

From this directory, with the repository's Go toolchain and `make` available:

```bash
./cryptosim.sh ./config/basic-config.json
```

[cryptosim.sh](cryptosim.sh) builds `bin/cryptosim` and `bin/configure-logger`
through the [Makefile](Makefile), stops with the build exit status on failure,
prints the executable's Go VCS settings and SHA-256 digest as run metadata, configures `PAX_LOG_*` variables from the JSON
file, then passes that file to the benchmark binary. Paths in the config resolve
from the process working directory. The basic config creates `data` and `logs`
here and allocates a 1 GiB canned random buffer, then pre-populates millions of
accounts. Choose smaller setup counts in a separate config for a small machine.

`MaxRuntimeSeconds` bounds the transaction phase after setup; `0` runs until
interrupted. Ctrl-C cancels the workload. When `EnableSuspension` is true, Enter
toggles suspend/resume. Suspension forces applied state blocks to commit. A
successful shutdown drains accepted transactions and receipts and joins their
workers before reporting completion; storage errors make `Close` return an error.

## Configuring Cryptosim

The authoritative fields, defaults and validation are in
[cryptosim_config.go](cryptosim_config.go). The loader overlays JSON onto
`DefaultCryptoSimConfig`, rejects unknown fields and validates the result. Use
the struct field names exactly, including the existing spelling
`RecieptChannelCapacity`. `DataDir` and `LogDir` are required. The example
[basic-config.json](config/basic-config.json) overrides some defaults: notably,
`BlocksPerCommit` is 32 in that file while the Go default is 1.

Key controls include:

- `Backend`, `FlatKVConfig`, `StateStoreConfig` and `HistoricalOffload` select
  and configure the database. [NewCryptoSim](cryptosim.go) supplies the factory's
  typed configuration for FlatKV, SSComposite, SSHistoricalOffload and
  CompositeDual+SSComposite. The historical-offload example is
  [historical-offload-kafka.json](config/historical-offload-kafka.json).
- Account counts, contract counts, hot-set probabilities and
  `NewAccountProbability` control the workload. Validation requires at least
  `HotErc20ContractSetSize + 1` contracts; setup ensures at least twice
  `TransactionsPerBlock` dormant accounts.
- `TransactionsPerBlock` groups simulated transactions;
  `BlocksPerCommit` controls database commits. Both must be positive.
- `ThreadsPerCore`, `ConstantThreadCount`, `ExecutorQueueSize` and
  `BlockChannelCapacity` control worker/queue sizes. The implementation uses
  `int(ThreadsPerCore) * runtime.NumCPU() + ConstantThreadCount`, with a minimum
  of one executor; fractional `ThreadsPerCore` is truncated before multiplying.
- `DisableTransactionReads` retains writes while suppressing transaction-time
  reads. `DisableTransactionExecution` skips executor work, but the main loop
  still finalizes blocks and updates counters. Start a fresh database before
  turning execution back on after such a run.
- `GenerateReceipts` enables synthetic receipts under `DataDir/receipts`.
  Receipt readers and log-filter readers have separate concurrency/rate
  controls; the accepted read mode is `recent`. Obsolete `cache` modes are
  rejected. Receipt retention uses `ReceiptKeepRecent` and
  `ReceiptPruneIntervalSeconds`. The active backend is PebbleDB. Its
  `FilterLogs` API refuses range queries with `ErrRangeQueryNotSupported`;
  enabling log readers measures these refusals, not successful log scans.
- `MetricsAddr` serves `/metrics` (default `:9090`); an empty address disables
  the HTTP server. `LogLevel` accepts debug, info, warn or error.

Use separate data and log directories for each experiment. The deletion flags
can remove those directories; keep `DeleteDataDirOnStartup` and
`DeleteDataDirOnShutdown` false when retaining data.

## Data persistence

A retained database supports continuation after a successful normal shutdown.
Keep the backend, key/value layout settings, `Seed`, `CannedRandomSize` and
`Erc20InteractionsPerAccount` unchanged, and keep deletion flags false. Preserve
state and its `receipts` subdirectory together. Continuation requires the
next-unused counter layout described below.

[database.go](database.go) persists the **next unused** block sequence, account ID
and contract ID in every finalized batch as 8-byte big-endian values. Setup may
finalize additional blocks; the builder starts from the resulting sequence after
setup. Reopening restores account and contract counters even when setup can
return early. Shutdown and suspension commit every applied state block, including
a final group smaller than `BlocksPerCommit` with no current transactions.

Normal shutdown stops block admission, joins the builder, drains accepted
executor work, closes the receipt input, waits for the receipt writer to drain
and close its store, then closes state storage. `Close` returns storage errors
before completion can be reported as successful. Prefetched blocks that were
never accepted by the main loop are discarded; their generated account counters
are not persisted. Receipt heights and deterministic hashes resume from the last
accepted block rather than from speculative generator state.

This is a normal-stop boundary, not an atomic checkpoint shared by the two
stores. A process crash or storage failure between state and receipt writes can
leave different durable tips. Do not treat such a run as successful continuation
evidence. Suspension commits state but does not promise to have drained the
asynchronous receipt queue; successful shutdown does. Console transaction totals
and metric counters describe this process, not a persisted transaction history.

The reopen example is in
[repair_m1195_test.go](repair_m1195_test.go):

```bash
# From the repository root; small MemIAVL SC + Pebble receipt stores in temp dirs.
go test -count=1 -v -run '^TestRepairM_1195_PersistenceDoc$' ./storage/state_db/bench/cryptosim/
```

`TestRepairM_1195_PersistenceDoc` creates eight setup accounts and three contracts,
accepts a two-transaction block, closes all participants, reopens, accepts one
more block and closes/reopens again. It verifies the durable block, account and
contract counters against each accepted block, verifies the receipt latest-block
counter, and compares all four decoded receipts with the producer's originals.
It also checks that new receipt heights and hashes do not reuse prior IDs.
`BlocksPerCommit=8` deliberately leaves a partial commit group at each stop.

Receipt metrics observe actual Pebble write/read calls: written receipts,
receipt reads/found/not-found, operation durations, and
`cryptosim_receipt_log_filter_reads_total` plus
`cryptosim_receipt_log_filter_unsupported_total`. No cache hit rate is inferred
from a backend operation. Reader targets come from successfully submitted writes
within backend retention; an async write not yet visible counts as not found.

## Transaction model

[transaction.go](transaction.go) chooses a source account, destination account,
storage slot for each, and a simulated ERC20 contract. With reads enabled, each
transaction reads the contract, both account records, both storage slots and
the fee-collection account. It writes random replacement data for the source,
destination and fee account records and both storage slots.

Account records use the EVM code-hash key prefix as a proxy for account storage;
they are synthetic padded data, not valid balance/nonce/code-hash records.
Contracts and receipts are also synthetic. The workload approximates I/O access
patterns, including hot/cold accounts and optional new-account growth, rather
than transfer execution semantics.

## Prometheus and Grafana

From the repository root, with Docker running:

```bash
docker/monitornode/scripts/start-prometheus.sh
docker/monitornode/scripts/start-grafana.sh
```

The supplied [Prometheus config](../../../../docker/monitornode/config/prometheus.yaml)
scrapes `host.docker.internal:9090`; keep this aligned with `MetricsAddr`.
Prometheus is exposed at `http://localhost:9091/`. Grafana is at
`http://localhost:3000/`. Set `GRAFANA_ADMIN_PASSWORD` in the environment before
starting Grafana. The script binds Grafana to loopback by default; a different
`GRAFANA_BIND_ADDRESS` requires an explicit password. The scripts provision the
Prometheus data source and dashboards from
[docker/monitornode](../../../../docker/monitornode), including
[cryptosim-dashboard.json](../../../../docker/monitornode/dashboards/cryptosim-dashboard.json).
These are benchmark metrics endpoints, not chain API endpoints.

Stop the containers from the repository root with:

```bash
docker/monitornode/scripts/stop-prometheus.sh
docker/monitornode/scripts/stop-grafana.sh
```

## Remote execution

Use the same checkout and commands on a remote Ubuntu host with sufficient disk
and memory. The optional [setup script](tools/setup-ubuntu.sh) installs system
dependencies; review it for the host before using it. Run cryptosim from this
directory, optionally in tmux, and start Prometheus on the remote machine if
collecting metrics. Keep the full config with the measurements.

To use the provisioned local Grafana with remote Prometheus, stop local
Prometheus and forward its host port through an existing SSH connection:

```bash
ssh -N -L 9091:localhost:9091 user@remote-host
```

The local Grafana data source uses `host.docker.internal:9091`, which reaches
that forwarded port when the host/container network permits it. The remote
Prometheus scrape target must reach the remote cryptosim metrics server.
