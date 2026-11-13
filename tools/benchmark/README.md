# Paxeer benchmark tools

This directory holds scripts, workload configurations, a load-test client and TPS analysis tools for exercising the merged Paxeer network locally. The benchmark launcher builds and starts `paxd` with in-process load generation; the separate load-test client submits transactions and measures committed blocks. The node benchmark manager refuses to start on a live EVM chain ID.

## Layout

- [home/](home/) — command entry point for preparing a dedicated benchmark home.
- [loadtest/](loadtest/) — Go load-test client, benchmark operations, contract workloads and supporting scripts.
- [scenarios/](scenarios/) — JSON workloads for native EVM transfers, ERC20 transfers and mixed traffic.
- [testdata/](testdata/) — shell fixtures for launcher output, exit-status and signal tests.
- [benchmark.sh](benchmark.sh) — build, initialize, configure and start a local benchmark node.
- [benchmark-compare.sh](benchmark-compare.sh) — build selected Git revisions, run nodes in parallel and compare their TPS statistics.
- [home.go](home.go) — validate and prepare benchmark homes, including explicitly confirmed resets.
- [tps_stats.py](tps_stats.py) — summarize TPS readings and compare median throughput against a regression limit.
- [python-tests.mk](python-tests.mk) — register this directory with the root Python tools test target.
- [scenarios_test.go](scenarios_test.go) — check scenario loading and running-chain identity overrides.
- [repair_430_test.go](repair_430_test.go) — test benchmark home ownership, path boundaries and reset behavior.
- [repair_m825_test.go](repair_m825_test.go) — test scenario documentation commands and deployment descriptions.
- [repair_m858_test.go](repair_m858_test.go) — test home propagation, node exit status, output and signal delivery.
- [test_benchmark_compare.py](test_benchmark_compare.py) — test the comparison script with collected TPS output.
- [test_tps_stats.py](test_tps_stats.py) — test TPS filtering, summaries and regression comparisons.

## Build and test

Run these commands from the repository root. The directory uses the root
[Go module](../../go.mod), [Makefile](../../Makefile) and
[chain.mk](../../chain.mk).

```sh
make install-bench
make build-loadtest
```

`install-bench` installs `paxd` with the `benchmark` and `mock_balances` build
tags. `build-loadtest` produces the load-test executable used by the account
population and contract-operation scripts. Both targets build the declared
native prerequisites. The launcher requires Bash, Go, Python, jq and the node
build dependencies described in the installation documentation.

The root test targets cover the directory:

```sh
make test-tools-python
make -f chain.mk test
```

The Python target includes this directory through `python-tests.mk`. The chain
test target discovers Go packages with tests, including the benchmark packages.

To initialize and run a local benchmark using the default EVM transfer scenario:

```sh
./tools/benchmark/benchmark.sh
```

Initialization uses an empty dedicated home. Reusing an existing marked home
requires `BENCHMARK_RESET_HOME` to confirm its exact absolute path. The home
helper rejects symlink components and overlap with protected directories.

`benchmark-compare.sh` accepts at least two arguments, each pairing a label with
a Git revision. It assigns separate binaries, homes, ports and output files,
captures CPU and heap profiles midway through execution, and compares each
candidate's median TPS with the first argument. It checks out the requested
revisions and clears `BASE_DIR` before building; use a separate checkout and
dedicated output directory. `RESULTS_ONLY` selects analysis of existing output.
Compare measurements collected under the same workload and machine conditions.

`tps_stats.py` strips ANSI color codes, sorts readings, drops the three smallest
and retains readings above 1000 TPS. It reports median, average, minimum and
maximum throughput. Comparison returns a nonzero status for missing retained
readings or a median regression exceeding `MAX_REGRESSION_PCT`.

## Configuration

The launcher reads these environment variables; the names below omit values:

- `BENCHMARK_PHASE` — select initialization, node startup or both.
- `PAX_HOME`, `BENCHMARK_RESET_HOME` — dedicated home and reset confirmation.
- `PAXD_BIN`, `PORT_OFFSET`, `LOG_FILE` — executable, port offsets and node output.
- `BENCHMARK_CONFIG`, `BENCHMARK_TXS_PER_BATCH` — scenario file and batch size.
- `GIGA_EXECUTOR`, `GIGA_OCC` — executor and optimistic concurrency settings.
- `DB_BACKEND`, `DISABLE_INDEXER` — database selection and indexer configuration.
- `MOCK_BALANCES` — displayed setting; the benchmark installation target includes the `mock_balances` tag.
- `DEBUG` — unfiltered terminal output.
- `HOME`, `OSTYPE` — user paths and platform-specific script behavior.

The comparison script additionally reads `DURATION`, `MAX_REGRESSION_PCT`,
`RESULTS_ONLY` and `BASE_DIR`. It forwards scenario, batch, executor and database
settings to the launcher. The launcher and comparison script pass database
build options through `COSMOS_BUILD_OPTIONS`.

The load-test operation scripts read `PAX_LOADTEST_OPS`, `PAXD_BIN`, `PAX_HOME`,
`ABIGEN_BIN` and `BENCH_DEPLOYER_KEY` for executable selection, account
population, binding generation and contract deployment.

`BENCHMARK_CONFIG` selects a scenario JSON file. The supplied files are
[evm.json](scenarios/evm.json), [default.json](scenarios/default.json),
[erc20.json](scenarios/erc20.json) and [mixed.json](scenarios/mixed.json).
The launcher configures `config.toml`, `app.toml` and `genesis.json` in the
benchmark home's configuration directory and reads `priv_validator_key.json`
during initialization. Its first and second positional arguments select
alternate `config.toml` and `app.toml` files.

The standalone client reads [loadtest/config.json](loadtest/config.json), or
the file selected with `--config-file`. Its configuration includes chain
identity, endpoints, account counts, transaction mix, target throughput,
metrics, TLS and post-transaction queries.

## Documentation

- [Scenario guide](scenarios/README.md) — workloads, scenario format and in-process deployment.
- [Installation and build](../../docs/site/paxeer-docs/public/markdown/installation.md) — toolchain prerequisites and benchmark installation.
- [Documentation site installation page](../../docs/site/paxeer-docs/src/app/installation/page.tsx) — source of the site's benchmark build instructions.
