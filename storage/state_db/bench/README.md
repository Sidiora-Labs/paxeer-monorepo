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

Each Go benchmark iteration creates a new temporary database and executes a whole
scenario. `-benchtime=1x` requests one iteration per selected sub-benchmark; it does
not mean one block. The command above is a usage example, not a recorded
performance result. Do not use `-bench .` as a supported backend comparison: some
registered cases fail during construction in the current tree.

### Backend configuration limits

`runBenchmark` calls `wrappers.NewDBImpl` with an untyped `nil` configuration.
[The factory](wrappers/db_implementations.go) handles that argument as follows:

| `TestScenario.Backend` | Behavior with the helper's `nil` argument |
|---|---|
| `wrappers.MemIAVL` | Constructs memiavl using the factory's benchmark configuration. |
| `wrappers.CompositeCosmos` | Constructs a composite store in `MemiavlOnly` mode. |
| `wrappers.CompositeSplit` | Constructs a composite store in `EVMMigrated` mode. |
| `wrappers.CompositeDual` | Constructs a composite store in `TestOnlyDualWrite` mode; this mode is for tests only. |
| `wrappers.NoOp` | Constructs the no-op wrapper; this does not measure storage I/O. |
| `wrappers.FlatKV` | Panics at the `*flatkv/config.Config` type assertion before the constructor's default handling. |
| `wrappers.SSComposite` | Panics at the `*config.StateStoreConfig` type assertion. |
| `wrappers.SSHistoricalOffload` | Panics at the `*wrappers.HistoricalOffloadConfig` type assertion. |
| `wrappers.CompositeDual_SSComposite` | Panics at the `*config.StateStoreConfig` type assertion. |

`TestScenario` has no backend-configuration field. Selecting a backend enum alone
cannot supply these configurations. The SS cases registered in
[bench_ss_test.go](bench_ss_test.go) are `BenchmarkSSCompositeWrite`,
`BenchmarkSSHistoricalOffloadWrite`, and
`BenchmarkCombinedCompositeDualSSCompositeWrite`; their presence does not make
them runnable through the current helper. Cryptosim has a separate configuration
path that supplies the appropriate configuration object.

### Workload and result interpretation

[bench_sc_test.go](bench_sc_test.go) registers block-size and distribution cases.
The MemIAVL and FlatKV cases named `even_distribution`, `bursty_distribution`,
`normal_distribution`, and `ramp_distribution` all leave `Distribution` unset, so
all four use `EvenDistribution`. Only the composite distribution cases explicitly
wire the bursty, normal, and ramp functions. A sub-benchmark name alone is not
evidence of a distinct workload.

The generator produces `NumBlocks` change sets, but the consumer loops from block
`1` while `block < NumBlocks`: one iteration commits **`NumBlocks - 1` blocks**
for a positive block count. With the even distribution it attempts
`(NumBlocks - 1) * floor(TotalKeys / NumBlocks)` key/value writes. For example,
`100_keys_per_block` with `TotalKeys=100000` and `NumBlocks=1000` commits 999
blocks and attempts 99,900 writes. Duplicate keys reduce unique-key growth.
Other distributions need their consumed per-block counts summed; `TotalKeys` is
an input to the distribution, not an enforced write quota.

The custom `keys/sec` metric divides configured `TotalKeys` by the accumulated
benchmark elapsed time, rather than counting writes actually consumed. Progress
block counts are inferred from the even rate, and the final progress line counts
attempted pairs, not distinct keys. Record the revision, exact invocation,
backend, explicit distribution, completed block count, attempted writes, duplicate
ratio and elapsed time before comparing results. No benchmark execution is
claimed by this documentation rewrite.

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
measurement. The FlatKV long-running variants have the configuration failure
listed above.

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
`BenchmarkFlatKVLongRunningWriteWithInitialState` is also registered but cannot
reach import through the current helper's configuration path.

## Define new scenarios

[helper.go](helper.go) defines `TestScenario`:

- `Name`: sub-benchmark label.
- `TotalKeys`: nominal input for the distribution.
- `NumBlocks`: generator block count; see the consumer limit above.
- `DuplicateRatio`: requested update fraction, clamped to `[0, 1]`; updates
  select previously generated keys when any exist.
- `Backend`: factory selector; see the configuration limits above.
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
