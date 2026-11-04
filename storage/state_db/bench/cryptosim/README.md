# Cryptosim

Cryptosim generates local database load shaped like cryptocurrency transfers.
It exercises the database wrappers and optionally the production receipt store;
it does not execute EVM bytecode, submit chain transactions or measure consensus
finality. See the [benchmark overview](../README.md) for the simpler write
microbenchmarks and the [migration overview](../../sc/migration/README.md) for
state-backend migration.

# Running Cryptosim

From this directory, with the repository's Go toolchain and `make` available:

```bash
./cryptosim.sh ./config/basic-config.json
```

[cryptosim.sh](cryptosim.sh) builds `bin/cryptosim` and `bin/configure-logger`
through the [Makefile](Makefile), configures `PAX_LOG_*` variables from the JSON
file, then passes that file to the benchmark binary. Paths in the config resolve
from the process working directory. The basic config creates `data` and `logs`
here and allocates a 1 GiB canned random buffer, then pre-populates millions of
accounts. Choose smaller setup counts in a separate config for a small machine.

`MaxRuntimeSeconds` bounds the transaction phase after setup; `0` runs until
interrupted. Ctrl-C cancels the workload. When `EnableSuspension` is true, Enter
toggles suspend/resume. Neither suspension nor the success banner proves that
all pending data and receipts were durably committed; see Data Persistence.

# Configuring Cryptosim

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
  controls; the accepted read mode is `cache`. Receipt retention uses
  `ReceiptKeepRecent` and `ReceiptPruneIntervalSeconds`.
- `MetricsAddr` serves `/metrics` (default `:9090`); an empty address disables
  the HTTP server. `LogLevel` accepts debug, info, warn or error.

Use separate data and log directories for each experiment. The deletion flags
can remove those directories; keep `DeleteDataDirOnStartup` and
`DeleteDataDirOnShutdown` false when retaining data.

# Data Persistence

Reopening a retained database can reuse pre-populated state, but **the current
runner does not provide a restart-safe continuation of the whole workload**.
Keep the backend, key/value layout settings, `Seed`, `CannedRandomSize` and
`Erc20InteractionsPerAccount` unchanged. Stable key generation is necessary;
it does not address the lifecycle limits below.

[database.go](database.go) includes account, contract and block counters in each
finalized batch as 8-byte big-endian values.
[DataGenerator](data_generator.go) reloads them from the database. Finalization
and durable commit are different events: state batches are applied each block,
while `Commit` is called only after `BlocksPerCommit` blocks or a forced
finalization with pending transactions.

Current continuation and shutdown limits are:

- The persisted block field is the number just written, before the in-memory
  increment. It is reloaded without adding one. Independently,
  [NewBlockBuilder](block_builder.go) starts its block number at zero on every
  process start. Receipt block identities therefore restart, even when state
  counters were reloaded; receipt hashes depend on those identities.
- If contract setup is skipped because enough contracts already exist,
  `CryptoSim.nextERC20ContractID` is not restored on that early-return path.
  Teardown uses that field, so a final batch can write an incorrect contract
  counter. Do not assume account/contract continuity from startup output alone.
- `FinalizeBlock` returns immediately when there are no transactions in the
  current block, even when `forceCommit` is true. Since ordinary block handling
  already finalized the block, shutdown or suspension can leave a partial group
  of fewer than `BlocksPerCommit` blocks without an explicit final commit.
- Cancellation lets executor flushes return before all queued work is drained.
  The receipt simulator is started without retaining a joinable handle;
  cancellation can leave queued receipts unwritten, and teardown does not wait
  for its store close. State and receipt durability have no common checkpoint.
- Errors during teardown are printed; `Close` still prints its success banner.
  That message is not a durability assertion.

For fresh-run comparisons, use a new directory and keep receipts from different
runs separate. `BlocksPerCommit=1` reduces the uncommitted group but does not fix
counter restoration, receipt identity or cancellation behavior. There is no
supported bounded reopen example that verifies all four of durable block,
account, contract and receipt continuity in this runner. Before treating a
retained run as continuation evidence, a lifecycle repair must demonstrate those
counters and receipt identities through stop/reopen on the real backend. Console
transaction totals and Prometheus counters describe the current process, not a
persisted transaction history.

# Transaction Model

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

# Setting Up Prometheus / Grafana

From the repository root, with Docker running:

```bash
docker/monitornode/scripts/start-prometheus.sh
docker/monitornode/scripts/start-grafana.sh
```

The supplied [Prometheus config](../../../../docker/monitornode/config/prometheus.yaml)
scrapes `host.docker.internal:9090`; keep this aligned with `MetricsAddr`.
Prometheus is exposed at `http://localhost:9091/`. Grafana is at
`http://localhost:3000/` with the script's initial local credentials `admin` /
`admin`. The scripts provision the Prometheus data source and dashboards from
[docker/monitornode](../../../../docker/monitornode), including
[cryptosim-dashboard.json](../../../../docker/monitornode/dashboards/cryptosim-dashboard.json).
These are benchmark metrics endpoints, not chain API endpoints.

Stop the containers from the repository root with:

```bash
docker/monitornode/scripts/stop-prometheus.sh
docker/monitornode/scripts/stop-grafana.sh
```

# Running in AWS

Use the same checkout and commands on a remote Ubuntu host with sufficient disk
and memory. The optional [setup script](tools/setup-ubuntu.sh) installs system
dependencies; review it for the host before using it. Run cryptosim from this
directory, optionally in tmux, and start Prometheus on the remote machine if
collecting metrics. Keep the revision and full config with the results.

To use the provisioned local Grafana with remote Prometheus, stop local
Prometheus and forward its host port through an existing SSH connection:

```bash
ssh -N -L 9091:localhost:9091 user@remote-host
```

The local Grafana data source uses `host.docker.internal:9091`, which reaches
that forwarded port when the host/container network permits it. The remote
Prometheus scrape target must reach the remote cryptosim metrics server.
