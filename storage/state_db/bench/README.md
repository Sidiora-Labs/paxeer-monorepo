# Benchmarks

This package measures local state commitment (SC) and state store (SS) database
workloads. These are storage microbenchmarks, not network transaction-throughput
or finality measurements. For the transfer-shaped workload, see
[cryptosim](cryptosim/README.md). For backend migration behavior, see the
[migration overview](../sc/migration/README.md) and
[operations roadmap](../sc/migration/OPERATIONS.md).

## Run benchmarks

From the repository root, select a benchmark whose backend can be constructed by
[the current helper](helper.go):

```bash
go test ./storage/state_db/bench -run '^$' \
  -bench '^BenchmarkMemIAVLWriteWithDifferentBlockSize$' -benchmem -benchtime=1x
```

Each Go benchmark iteration creates a new temporary database and executes all
`NumBlocks` generated blocks. `-benchtime=1x` requests one complete scenario per
selected sub-benchmark. Construction and snapshot import are outside the timer;
generation, commit work and the final close/flush are timed. The command above
is a usage example, not a recorded performance result.

### Backend configuration

`runBenchmark` uses `TestScenario.OpenDB` in [helper.go](helper.go), which supplies
concrete configurations to [the factory](wrappers/db_implementations.go).
Unsupported configuration types and typed nil pointers return explicit errors.

| `TestScenario.Backend` | Configuration supplied by the helper |
|---|---|
| `wrappers.MemIAVL` | Factory benchmark defaults, including asynchronous commits. |
| `wrappers.FlatKV` | `flatkv/config.DefaultConfig()` with the iteration's temporary data directory. |
| `wrappers.CompositeCosmos` | Composite `MemiavlOnly` mode. |
| `wrappers.CompositeSplit` | Composite `EVMMigrated` mode. |
| `wrappers.CompositeDual` | Composite `TestOnlyDualWrite` mode; for tests only. |
| `wrappers.SSComposite` | `DefaultBenchStateStoreConfig()`: default state store with the benchmark async write buffer and EVM split enabled. |
| `wrappers.CompositeDual_SSComposite` | The same state store configuration plus composite dual-write SC. |
| `wrappers.NoOp` | No-op wrapper; measures harness overhead without storage I/O. |
| `wrappers.SSHistoricalOffload` | Requires explicit `TestScenario.HistoricalOffloadConfig` with Kafka brokers and topic; no local default. |

`BenchmarkSSCompositeWrite` and
`BenchmarkCombinedCompositeDualSSCompositeWrite` use local stores and receive
the concrete defaults above. `BenchmarkSSHistoricalOffloadWrite` currently omits
its required destination and returns a configuration error: set that scenario's
`HistoricalOffloadConfig` and provision Kafka before selecting it. Do not use
`-bench .` for a local backend comparison; select the intended benchmark.

### Workload and result interpretation

Both MemIAVL and FlatKV distribution comparisons in
[bench_sc_test.go](bench_sc_test.go) use the shared `keyDistributionScenarios`
builder. The named workloads are explicitly wired as follows:

| Scenario | Distribution | Representative counts for 1,000 nominal keys / 10 blocks |
|---|---|---|
| `even_distribution` | `EvenDistribution` | 100 in every block. |
| `bursty_distribution` | `BurstyDistribution(1, 10, 5, 3)` | First block 497–503; other nine blocks 97–103. |
| `normal_distribution` | `NormalDistribution(1, 0.2)` | First block 75; seeded normal samples around mean 100, sigma 20. |
| `ramp_distribution` | `RampDistribution(0.5, 1.5)` | First block 50, fifth 94, last 150. |

The regression gate asserts these representative counts and commits each
workload through the real backends. `TotalKeys` is a nominal distribution input,
not a write quota: even counts use integer division, burst and normal counts
vary, and ramp counts are rounded down per block. Duplicate keys reduce unique
key growth. The reported `keys` metric counts committed key/value pairs,
including updates, and `blocks` counts completed commits. Both metrics sum all
`b.N` iterations. `seconds` is total timed work and `keys/sec` is committed pairs
divided by that same elapsed time. Progress reports use actual completed block
and pair counts. Every producer is canceled and joined when its consumer exits.

The bounded gate fixtures in [repair_m1194_test.go](repair_m1194_test.go) assert
these completed counts for **each** of MemIAVL and FlatKV, then close and reopen
the store and check every generated key and the recovered version:

| Workload | Nominal keys | Completed blocks | Committed pairs |
|---|---:|---:|---:|
| Even, one block | 4 | 1 | 4 |
| Even | 12 | 3 | 12 |
| Even, integer remainder | 10 | 3 | 9 |
| Bursty `(1, 2, 3, 0)` | 8 | 4 | 16 |
| Empty blocks | 0 | 3 | 0 |

Run the storage correctness tests from the repository root:

```bash
go test -count=1 -v -run '^TestRepairM_1194_' ./storage/state_db/bench/
```

The multi-iteration fixture also asserts 16 committed pairs and four blocks per
iteration, and checks throughput against the aggregate elapsed time. These are
correctness fixtures, not qualified performance comparisons. For performance
evidence, record the revision, exact invocation, backend, explicit distribution,
completed blocks/pairs, duplicate ratio and elapsed time from the selected run.

## Long running benchmark

[bench_sc_long_running_test.go](bench_sc_long_running_test.go) is guarded by
`slow_bench`. The MemIAVL case prints progress about every five seconds:

```bash
go test ./storage/state_db/bench -run '^$' \
  -bench '^BenchmarkMemIAVLLongRunningWrite$' \
  -benchmem -benchtime=1x -tags=slow_bench
```

The configured scenario is enormous (one billion blocks, one trillion nominal
writes, duplicate ratio 0.5). `-benchtime` controls repetition/calibration; it
does not bound the duration of one scenario. An interrupted run is partial
measurement. The FlatKV long-running variants receive the local default configuration too.

### With snapshot pre-population

`BenchmarkMemIAVLLongRunningWriteWithInitialState` imports the EVM store from a
state-sync snapshot before timing writes. It skips when `SNAPSHOT_PATH` is empty.
The helper expects numbered chunk files (`0`, `1`, `2`, ...) in a directory with
layout `<node_home>/data/snapshots/<height>/<format>/`; it parses a positive
32-bit height from the parent directory. It uses the wrapper's native
`Importer`, closes the importer, and reloads the latest version.

```bash
SNAPSHOT_PATH=/path/to/node/data/snapshots/12345/1 \
  go test ./storage/state_db/bench -run '^$' \
    -bench '^BenchmarkMemIAVLLongRunningWriteWithInitialState$' \
    -benchmem -benchtime=1x -tags=slow_bench
```

The helper filters for the `evm` store and IAVL snapshot nodes. This is not a
full composite snapshot restore: other stores, including the physical `flatkv`
snapshot section, are skipped. Use a compatible EVM/IAVL snapshot.
`BenchmarkFlatKVLongRunningWriteWithInitialState` uses the same helper and
imports through the FlatKV wrapper's native importer.

## Define new scenarios

[helper.go](helper.go) defines `TestScenario`:

- `Name`: sub-benchmark label.
- `TotalKeys`: nominal input for the distribution.
- `NumBlocks`: generated and consumed block count.
- `DuplicateRatio`: requested update fraction, clamped to `[0, 1]`; updates
  select previously generated keys when any exist.
- `Backend`: factory selector; see the configurations above.
- `HistoricalOffloadConfig`: required explicit Kafka destination for historical offload.
- `Distribution`: per-block count function; `nil` means even.
- `SnapshotPath`: optional compatible snapshot chunk directory.

For example, a scenario that explicitly selects the bursty function is:

```go
scenario := TestScenario{
    Name:           "bursty_updates",
    TotalKeys:      100_000,
    NumBlocks:      10_000,
    DuplicateRatio: 0.25,
    Distribution:   BurstyDistribution(1, 10, 5, 3),
    Backend:        wrappers.MemIAVL,
}
```

Keys are 53-byte synthetic EVM storage keys with prefix `0x03`; values are 32
random bytes. Key selection uses a fixed pseudorandom seed, but values use
`crypto/rand`, so the data is not byte-for-byte reproducible.

## Add a new distribution

Define a `KeyDistribution` with the signature below and assign it explicitly to
`TestScenario.Distribution` in the benchmark source:

```go
func MyDistribution(numBlocks, totalKeys, block int64) int64 {
    if numBlocks <= 0 {
        return 0
    }
    return totalKeys / numBlocks
}
```

Negative returned counts are clamped to zero. Bursty, normal and ramp functions
are already available in [helper.go](helper.go); their output totals need not
equal `TotalKeys`.
