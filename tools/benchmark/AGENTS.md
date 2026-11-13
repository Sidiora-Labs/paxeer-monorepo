# Benchmark

## Single scenario

```bash
GIGA_EXECUTOR=true GIGA_OCC=true DEBUG=true tools/benchmark/benchmark.sh
```

TPS is logged every 5s as `tps=<value>` (with ANSI color codes). To capture output and extract TPS:

```bash
LOG_FILE=/tmp/bench.log DEBUG=true tools/benchmark/benchmark.sh

# Extract TPS values
sed 's/\x1b\[[0-9;]*m//g' /tmp/bench.log | sed -n 's/.*tps=\([0-9.]*\).*/\1/p'
```

Available scenarios in `tools/benchmark/scenarios/`: `evm.json` (default), `erc20.json`, `mixed.json`, `default.json`.

```bash
# Use a different scenario
BENCHMARK_CONFIG=tools/benchmark/scenarios/erc20.json tools/benchmark/benchmark.sh
```

## Environment variables

### benchmark.sh

| Var | Default | Purpose |
|-----|---------|---------|
| `BENCHMARK_PHASE` | `all` | `init` (build+init+configure), `start` (run node), `all` (both) |
| `PAX_HOME` | `$HOME/.pax-benchmark` | Absolute dedicated benchmark data directory, initialized in place |
| `BENCHMARK_RESET_HOME` | Empty | Exact benchmark path confirming a reset of an existing marked benchmark home |
| `PORT_OFFSET` | `0` | Added to all ports (RPC, P2P, pprof, gRPC, etc.) |
| `PAXD_BIN` | `""` | Pre-built binary path; initialization skips compilation and copies it to `~/go/bin/paxd`, while node startup uses the supplied path |
| `LOG_FILE` | `""` | Redirect paxd output to file |
| `BENCHMARK_CONFIG` | `$SCRIPT_DIR/scenarios/evm.json` | Default resolves from the script directory; an explicit relative path resolves from the working directory |
| `BENCHMARK_TXS_PER_BATCH` | `1000` | Transactions per batch |
| `GIGA_EXECUTOR` | `false` | Enable Giga EVM-optimized executor |
| `GIGA_OCC` | `false` | Enable OCC for Giga Executor |
| `DB_BACKEND` | `goleveldb` | Database backend (goleveldb, memdb, cleveldb, rocksdb) |
| `MOCK_BALANCES` | `true` | Displayed setting; `install-bench` always includes the `mock_balances` build tag |
| `DISABLE_INDEXER` | `true` | Disable indexer for benchmark (reduces I/O overhead) |
| `DEBUG` | `false` | Print all log output without filtering |

### benchmark-compare.sh

Delegates initialization and node startup to `benchmark.sh`, forwarding its
scenario, executor, database and batch settings. It assigns per-run homes,
ports, binaries and log paths. It builds binaries with the `mock_balances` tag
and initializes them with `DISABLE_INDEXER=true`. Additional settings:

| Var | Default | Purpose |
|-----|---------|---------|
| `DURATION` | `600` | How long (seconds) to run each node before stopping |
| `GIGA_EXECUTOR` | **`true`** | Overrides benchmark.sh default (false) |
| `GIGA_OCC` | **`true`** | Overrides benchmark.sh default (false) |
| `DB_BACKEND` | `goleveldb` | Forwarded to build and init phases |
| `BASE_DIR` | `/tmp/pax-bench` | Output directory; cleared before a new comparison |
| `RESULTS_ONLY` | `false` | Summarize and compare existing output without building or starting nodes |
| `MAX_REGRESSION_PCT` | `5` | Maximum allowed median TPS regression against the first argument |

**Note:** `GIGA_EXECUTOR` and `GIGA_OCC` default to `true` in the compare script but `false` in benchmark.sh. The compare script is designed for performance comparison where Giga Executor is typically enabled.

## Parallel multi-scenario comparison

Use `tools/benchmark/benchmark-compare.sh` to run multiple git commits side-by-side (minimum 2 scenarios required):

```bash
tools/benchmark/benchmark-compare.sh \
  baseline="${BASELINE_REF:?set the baseline git ref}" \
  candidate="${CANDIDATE_REF:?set the candidate git ref}"
```

Each scenario gets its own binary, home directory and port set (offset by 100).
Results include median/average/minimum/maximum TPS. Raw data is written to
`$BASE_DIR/<label>/tps.txt`. The script compares each candidate with the first argument and exits nonzero for missing readings or a
median TPS regression above `MAX_REGRESSION_PCT`.

## Benchmark directories

Initialization requires an empty dedicated home. Reusing a marked benchmark
home requires `BENCHMARK_RESET_HOME` to equal its absolute path. The home helper
refuses symlink components and overlap with protected directories; see
[`home.go`](home.go). The comparison script checks out each requested revision
and clears `BASE_DIR` before building, so run it from a separate checkout with
no work to retain there.

Compare results collected under the same workload and machine conditions.

## Comparing pprof profiles

`benchmark-compare.sh` captures CPU and heap profiles midway through the run.
Single-scenario runs enable the pprof HTTP endpoint; capture their profiles
with `curl` or `go tool pprof`. Use `pprof -diff_base` to compare profiles:

```bash
BENCHMARK_RESULTS="${BASE_DIR:-/tmp/pax-bench}"

# CPU diff (positive = regression, negative = improvement)
go tool pprof -top -diff_base "$BENCHMARK_RESULTS/baseline/pprof/cpu.pb.gz" "$BENCHMARK_RESULTS/candidate/pprof/cpu.pb.gz"

# Allocation diff
go tool pprof -alloc_space -top -diff_base "$BENCHMARK_RESULTS/baseline/pprof/heap.pb.gz" "$BENCHMARK_RESULTS/candidate/pprof/heap.pb.gz"
```
