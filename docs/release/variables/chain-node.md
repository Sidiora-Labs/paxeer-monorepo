# Chain node variables

The chain node is `paxd`, with command utilities in `cmd/` and application and proposer execution in `node/`. Paxeer mainnet-beta is one BFT network with EVM chain ID 125; the kernel proposes blocks inside the node, and receipts become final when their block commits. Node JSON-RPC uses the root path. The settings below cover environment reads and configuration readers used by these directories.

Environment names are uppercase. TOML rows identify the file relative to the node home, section and key; `[root]` means a top-level key. JSON rows identify the file selected by `BENCHMARK_CONFIG`, its object and key. Required includes settings required only when the named feature is enabled. Empty means an empty string; none means no supplied default. Secret-file paths are not secrets themselves; their private contents must remain protected. Host addresses, account values and key values are omitted.

`node/.env.example` lists the environment inputs. The application does not load this file itself. `cmd/` reads command arguments and binary input files, and has no environment inputs or environment example. Neither directory contains container definitions, unit files or start scripts. Benchmark inputs apply only to benchmark builds on non-live chains. `UPGRADE_VERSION_LIST` is read by application startup even though its purpose is integration testing.

Storage rows distinguish generated `config/app.toml` defaults from absent-key conversions in `node/paxdb.go`. Preserve the generated storage settings: unguarded reads convert missing values to zero, false or empty. Node mode also sets defaults: full and archive nodes enable EVM HTTP and WebSocket service; validator and seed nodes disable them. Full nodes retain 100000 historical blocks, while archive nodes retain all.

## Required

| Name (file / section / key for configuration) | What it sets | Source default | Secret? | Reading source |
| --- | --- | --- | --- | --- |
| `HOME` | Process home used to derive the default node home, `.paxeer`; required on Linux. | None | No | `node/app.go` (`os.UserHomeDir`) |
| `config/app.toml` / `[eth_replay]` / `eth_rpc` | Upstream Ethereum RPC endpoint when replay is enabled. | Empty | No; protect any URL credentials | `node/app.go` → `modules/evm/replay/config.go` |
| `config/app.toml` / `[genesis]` / `genesis-stream-file` | Input file when streamed genesis import is enabled. | Empty | No | `node/genesis.go` |
| `config/app.toml` / `[sequencer]` / `lease-duration` | Positive execution lease duration when enabled. | Three times `replication-timeout`; initially `0s` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `microbatch-max-count` | Positive transaction count limit in microbatch mode. | `0` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `microbatch-window` | Positive group waiting interval in microbatch mode. | `0s` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `nonce-queue-max-per-sender` | Positive future-nonce queue bound per sender when enabled. | `0` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `nonce-queue-max-total` | Positive total queue bound, at least the per-sender bound, when enabled. | `0` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `receipt-key-file` | Receipt signing seed file when enabled; separate from the consensus key and restricted to mode `0600`. | Empty | No; file contents are secret | `node/sequencer/config.go`, `node/sequencer/receiptkey.go` |
| `config/app.toml` / `[sequencer]` / `replication-peer-name` | Authorized peer certificate DNS identity for replication. | Empty | No | `node/sequencer/config.go`, `node/sequencer/dial.go` |
| `config/app.toml` / `[sequencer]` / `replication-peer-spki` | Peer certificate SHA-256 public-key digest for replication. | Empty | No | `node/sequencer/config.go`, `node/sequencer/dial.go` |
| `config/app.toml` / `[sequencer]` / `replication-timeout` | Positive exchange timeout when a standby address is configured; otherwise nonnegative. | `0s` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `replication-tls-ca` | Trust-root file for mutual TLS replication. | Empty | No | `node/sequencer/config.go`, `node/sequencer/dial.go` |
| `config/app.toml` / `[sequencer]` / `replication-tls-cert` | Local certificate file for mutual TLS replication. | Empty | No | `node/sequencer/config.go`, `node/sequencer/dial.go` |
| `config/app.toml` / `[sequencer]` / `replication-tls-key` | Private-key file for mutual TLS replication. | Empty | No; file contents are secret | `node/sequencer/config.go`, `node/sequencer/dial.go` |
| `config/app.toml` / `[sequencer]` / `standby-address` | Replication listen address on a standby; optional destination on a primary. | Empty | No | `node/sequencer/config.go`, `node/sequencer/standby_role.go` |
| `config/app.toml` / `[sequencer]` / `wal-dir` | Durable execution write-ahead-log directory when enabled. | Empty | No | `node/sequencer/config.go` |
| `config/app.toml` / `[state-commit]` / `sc-enable` | Enables the required PaxDB state-commit backend. Must be true. | Generated: `true`; absent: `false` | No | `node/paxdb.go` |

| `config/app.toml` / `[state-store]` / `ss-enable` | Enables historical state; required when state-sync snapshot export is enabled. | Generated: `true`, or `false` for validator/seed; absent: `false` | No | `node/paxdb.go`, `node/params/config.go` |

## Optional

| Name (file / section / key for configuration) | What it sets | Source default | Secret? | Reading source |
| --- | --- | --- | --- | --- |
| `BENCHMARK_CONFIG` | JSON load configuration path; empty selects the built-in EVM transfer scenario with 5000 generated accounts and weight 1. | Empty | No | `node/benchmark/benchmark.go`, `node/benchmark/config.go` |
| `BENCHMARK_CONFIG` JSON / `[root]` / `chainId` | Decoded EVM chain ID, replaced with the running chain's ID. | Running chain | No | `node/benchmark/config.go` |
| `BENCHMARK_CONFIG` JSON / `[root]` / `mockDeploy` | Decoded deployment setting, always replaced with true for in-process deployment. | `true` | No | `node/benchmark/config.go` |
| `BENCHMARK_CONFIG` JSON / `[root]` / `paxChainID` | Decoded textual chain ID, replaced with the running chain's ID. | Running chain | No | `node/benchmark/config.go` |
| `BENCHMARK_CONFIG` JSON / `accounts` / `count` | Shared generated account-pool size for scenarios without their own pool. | Built-in configuration: `5000`; JSON omission: `0` | No | `node/benchmark/config.go`, `node/benchmark/generator.go` |
| `BENCHMARK_CONFIG` JSON / `accounts` / `newAccountRate` | Shared pool's rate of generating new accounts. | `0` | No | `node/benchmark/generator.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `accounts.count` | Per-scenario generated pool size; a scenario needs its own pool or the shared pool. | None; absent object uses shared pool | No | `node/benchmark/generator.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `accounts.newAccountRate` | Per-scenario pool's rate of generating new accounts. | `0` | No | `node/benchmark/generator.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasFeeCapPicker.Gas` | Fixed fee cap when the picker name is `fixed`. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasFeeCapPicker.Max` | Inclusive upper fee-cap bound for a `random` picker; must exceed Min. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasFeeCapPicker.Min` | Inclusive lower fee-cap bound for a `random` picker. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasFeeCapPicker.Name` | Fee-cap picker: `fixed` or `random` for transfer scenarios. | Absent picker: `20000000000` wei; explicit empty Name: `0` | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasPicker.Gas` | Fixed gas limit when the picker name is `fixed`. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasPicker.Max` | Inclusive upper gas bound for a `random` picker; must exceed Min. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasPicker.Min` | Inclusive lower gas bound for a `random` picker. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasPicker.Name` | Gas-limit picker: `fixed` or `random` for transfer scenarios. | Absent picker: `21000`; explicit empty Name: `0` | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasTipCapPicker.Gas` | Fixed priority fee when the picker name is `fixed`. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasTipCapPicker.Max` | Inclusive upper priority-fee bound for a `random` picker; must exceed Min. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasTipCapPicker.Min` | Inclusive lower priority-fee bound for a `random` picker. | `0` in an explicit picker | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `gasTipCapPicker.Name` | Priority-fee picker: `fixed` or `random` for transfer scenarios. | Absent picker: `2000000000` wei; explicit empty Name: `0` | No | `node/benchmark/config.go` → `pax-load/config/gas.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `name` | Scenario implementation to instantiate. | Built-in configuration: `evmtransfer`; JSON omission: empty | No | `node/benchmark/config.go`, `node/benchmark/generator.go` |
| `BENCHMARK_CONFIG` JSON / `scenarios[]` / `weight` | Scenario selection weight; zero excludes the scenario from load generation. | Built-in configuration: `1`; JSON omission: `0` | No | `node/benchmark/generator.go` |
| `BENCHMARK_TXS_PER_BATCH` | Positive benchmark transaction count per batch; empty, invalid and nonpositive values use the default. | `1000` | No | `node/benchmark/generator.go` |
| `UPGRADE_VERSION_LIST` | Comma- or newline-separated override of the embedded upgrade list, sorted by semantic version. | Empty: use `node/tags` | No | `node/upgrades.go` |
| `config/app.toml` / `[admin_server]` / `admin_address` | Admin gRPC listen address, restricted to loopback when enabled. | IPv4 loopback, port `9095` | No | `node/app.go` → `admin/config.go` |
| `config/app.toml` / `[admin_server]` / `admin_enabled` | Starts admin gRPC for runtime log-level control. | `false` | No | `node/app.go` → `admin/config.go` |
| `config/app.toml` / `[eth_blocktest]` / `eth_blocktest_enabled` | Enables Ethereum block-data execution mode. | `false` | No | `node/app.go` → `modules/evm/blocktest/config.go` |
| `config/app.toml` / `[eth_blocktest]` / `eth_blocktest_test_data_path` | Ethereum block-data input directory. | `~/testdata/` | No | `node/app.go` → `modules/evm/blocktest/config.go` |
| `config/app.toml` / `[eth_replay]` / `contract_state_checks` | Performs contract-state comparisons during replay. | `false` | No | `node/app.go` → `modules/evm/replay/config.go` |
| `config/app.toml` / `[eth_replay]` / `eth_data_dir` | Local Ethereum chain-data directory. | `/root/.ethereum/chaindata` | No | `node/app.go` → `modules/evm/replay/config.go` |
| `config/app.toml` / `[eth_replay]` / `eth_replay_enabled` | Enables Ethereum replay. | `false` | No | `node/app.go` → `modules/evm/replay/config.go` |
| `config/app.toml` / `[evm]` / `batch_request_limit` | Maximum requests in an RPC batch. | `1000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `batch_response_max_size` | Maximum RPC batch response size in bytes. | `25000000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `cors_origins` | Comma-separated HTTP CORS origins. | `*` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `deny_list` | RPC methods refused immediately. | Empty list | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `enable_parallelized_block_trace` | Enables parallel default block tracing. | `false` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `enable_test_api` | Enables test override APIs. | `false` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `enable_unsafe_keyring_rpc` | Enables local test-keyring signing RPC methods. | `false` | No; keyring contents are secret | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `enabled_legacy_pax_apis` | Allowlist for gated legacy methods. | `pax_getPaxAddress`, `pax_getEVMAddress`, `pax_getCosmosTx` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `filter_timeout` | Filter lifetime; minimum `2ns`. | `120s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `http_address` | Literal IP interface for EVM HTTP service. | All IPv4 interfaces | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `http_enabled` | Enables EVM HTTP service. | `true`; `false` for validator/seed initialization | No | `node/app.go`, `node/params/config.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `http_port` | EVM HTTP port. | `8545` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `idle_timeout` | Idle HTTP keep-alive timeout. | `120s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `ip_rate_limit_burst` | Per-IP request burst capacity. | `400` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `ip_rate_limit_rps` | Per-IP sustained requests per second; zero disables limiting. | `200` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_blocks_for_log` | Maximum block range for log queries. | `2000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_concurrent_simulation_calls` | Concurrent EVM simulations; zero means unlimited. | CPU count | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_concurrent_trace_calls` | Concurrent tracing calls; zero means unlimited. | `10` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_log_no_block` | Maximum logs for an open-ended block range. | `10000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_subscriptions_logs` | Concurrent log subscription limit. | `10000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_subscriptions_new_head` | Concurrent new-head subscription limit. | `10000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_trace_lookback_blocks` | Historical block lookback for tracing; negative one permits unlimited lookback. | `10000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `max_tx_pool_txs` | Maximum transactions pulled from the mempool. | `1000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `px_indexer_url` | Indexer for asset-list and history reads; empty leaves those reads unavailable. | Empty | No; protect any URL credentials | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `px_inference_program` | Inference program identifier. | Empty | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `px_inference_reader` | Inference reader contract identifier. | Empty | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `read_header_timeout` | HTTP header read timeout. | `30s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `read_timeout` | Entire HTTP request read timeout. | `30s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `rpc_stats_interval` | RPC metrics interval; minimum `1ns`. | `10s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `send_raw_tx_sync_timeout` | Synchronous raw-transaction submission timeout. | `2s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `simulation_evm_timeout` | EVM simulation execution timeout. | `60s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `simulation_gas_limit` | EVM simulation gas ceiling. | `10000000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `slow` | Serializes transaction handling. | `false` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_enabled` | Enables background trace caching. | `false` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_queue_size` | Pending trace-bake height queue capacity. | `4096` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_snapshot_window` | Recent in-memory snapshots retained for trace baking. | `64` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_tracers` | Named tracers cached per block. | `callTracer` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_use_snapshot` | Uses available in-memory snapshots for trace baking. | `false` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_window_blocks` | Trace cache retention window; zero disables pruning. | `0` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_bake_workers` | Background re-execution worker count. | `1` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `trace_timeout` | Per-trace execution timeout. | `30s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `worker_pool_size` | Log-query worker count; zero selects the default. | Minimum of `64` and twice CPU count | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `worker_queue_size` | Log-query task queue capacity; zero selects the default. | `1000` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `write_timeout` | HTTP response write timeout. | `30s` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `ws_enabled` | Enables EVM WebSocket service. | `true`; `false` for validator/seed initialization | No | `node/app.go`, `node/params/config.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `ws_origins` | Comma-separated allowed WebSocket origins. | `*` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm]` / `ws_port` | EVM WebSocket port. | `8546` | No | `node/app.go` → `rpc/config/config.go` |
| `config/app.toml` / `[evm_query]` / `evm_query_gas_limit` | Gas limit for EVM keeper queries. | `300000` | No | `node/app.go` → `modules/evm/querier/config.go` |
| `config/app.toml` / `[genesis]` / `stream-import` | Imports genesis from a stream file. | `false` | No | `node/genesis.go` |
| `config/app.toml` / `[giga_executor]` / `enabled` | Enables the Giga EVM executor. | `true` | No | `node/app.go`, `node/paxdb.go` → `engine/executor/config/config.go` |
| `config/app.toml` / `[giga_executor]` / `occ_enabled` | Enables optimistic parallel execution in Giga. | `true` | No | `node/app.go` → `engine/executor/config/config.go` |
| `config/app.toml` / `[light_invariance]` / `supply_enabled` | Checks native-denomination and wei supply invariants. | `true` | No | `node/invariance.go` |
| `config/app.toml` / `[receipt-store]` / `async-write-buffer` | Receipt-store asynchronous write queue; nonpositive values select synchronous writes. | `100` | No | `node/receipt_store_config.go` → `storage/config/receipt_config.go` |
| `config/app.toml` / `[receipt-store]` / `backend` | Rejected key name; use `rs-backend`. | Must be absent | No | `node/receipt_store_config.go` → `storage/config/receipt_config.go` |
| `config/app.toml` / `[receipt-store]` / `db-directory` | Receipt database directory. | Empty: node-home receipt path, preserving an existing legacy directory | No | `node/receipt_store_config.go` |
| `config/app.toml` / `[receipt-store]` / `prune-interval-seconds` | Receipt pruning interval in seconds. | `600` | No | `node/receipt_store_config.go` → `storage/config/receipt_config.go` |
| `config/app.toml` / `[receipt-store]` / `rs-backend` | Receipt database backend; accepts `pebbledb` or `pebble`. | `pebbledb` | No | `node/receipt_store_config.go` → `storage/config/receipt_config.go` |
| `config/app.toml` / `[root]` / `min-retain-blocks` | Consensus block and receipt-store retention; zero retains everything. | `0`; full-node initialization: `100000`; archive: `0` | No | `node/receipt_store_config.go`, `node/params/config.go` |
| `config/app.toml` / `[root]` / `trace` | Enables contract debug output through the global trace option. | `false` | No | `node/app.go` → `wasm/x/wasm/module.go` |
| `config/app.toml` / `[sequencer]` / `enable` | Enables proposer execution and receipt production inside the node. | `false` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `mode` | Executor selection: `sequential` or `microbatch`. | `sequential` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `receipt-forward-url` | Receipt-request forwarding destination when local execution is disabled; must be empty when enabled. | Empty | No; protect any URL credentials | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `role` | WAL replication role: `primary` or `standby`. | `primary` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[sequencer]` / `wal-retain-blocks` | Positive number of committed WAL blocks to retain. | `2` | No | `node/sequencer/config.go` |
| `config/app.toml` / `[state-commit.flatkv]` / `async-write-buffer` | FlatKV asynchronous write queue; nonpositive values select synchronous writes. | `0` | No | `node/paxdb.go` → `storage/config/flatkv_parse.go` |
| `config/app.toml` / `[state-commit.flatkv]` / `fsync` | Syncs FlatKV data and metadata database writes. | `false` | No | `node/paxdb.go` → `storage/config/flatkv_parse.go` |
| `config/app.toml` / `[state-commit.flatkv]` / `snapshot-interval` | FlatKV checkpoint interval in blocks; zero disables automatic checkpoints. | `10000` | No | `node/paxdb.go` → `storage/config/flatkv_parse.go` |
| `config/app.toml` / `[state-commit.flatkv]` / `snapshot-keep-recent` | Old FlatKV checkpoints retained in addition to the latest. | `2` | No | `node/paxdb.go` → `storage/config/flatkv_parse.go` |
| `config/app.toml` / `[state-commit]` / `sc-async-commit-buffer` | MemIAVL asynchronous commit queue. | Generated: `100`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-directory` | State-commit directory override. | Empty: resolve under node home | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-historical-proof-burst` | Historical proof request burst capacity. | `1` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-historical-proof-max-inflight` | Concurrent historical proof queries. | `1` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-historical-proof-rate-limit` | Historical proof requests per second; nonpositive disables limiting. | `1` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-keep-recent` | Old MemIAVL snapshots retained in addition to the latest. | `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-keys-to-migrate-per-block` | Positive per-block migration batch size. | `1024` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-snapshot-interval` | MemIAVL snapshot block interval. | Generated: `10000`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-snapshot-min-time-interval` | Minimum interval between MemIAVL snapshots in seconds. | Generated: `3600`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-snapshot-prefetch-threshold` | Page-cache residency fraction below which snapshot prefetch runs. | Generated: `0.8`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-snapshot-write-rate-mbps` | Global snapshot write rate in MB/s; zero is unlimited. | Generated: `100`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-snapshot-writer-limit` | Concurrent snapshot writers. | `4` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-commit]` / `sc-write-mode` | Write routing: `memiavl_only`, `migrate_evm`, `evm_migrated`, `migrate_all_but_bank`, `all_migrated_but_bank`, `migrate_bank`, `flatkv_only` or `test_only_dual_write`. | `memiavl_only` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `evm-ss-db-directory` | Dedicated EVM historical-state directory. | Empty: resolve under node home | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `evm-ss-separate-dbs` | Splits EVM history into databases by key family. | `false` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `evm-ss-split` | Uses a dedicated EVM historical-state backend. | `false` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `ss-async-write-buffer` | Historical-state write queue; nonpositive values select synchronous writes. | Generated: `100`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `ss-backend` | Historical-state backend: `pebbledb` or `rocksdb`. | Generated: `pebbledb`; absent: empty | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `ss-db-directory` | Cosmos historical-state directory. | Empty: resolve under node home | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `ss-import-num-workers` | Historical-state import worker count. | `1` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-store]` / `ss-keep-recent` | Historical-state retention in blocks; zero keeps all. | Generated: `100000`, or `0` for archive; absent: `0` | No | `node/paxdb.go`, `node/params/config.go` |
| `config/app.toml` / `[state-store]` / `ss-prune-interval` | Historical-state pruning interval in seconds. | Generated: `600`; absent: `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[state-sync]` / `snapshot-interval` | State-sync snapshot export interval in blocks; positive values require historical state. | `0` | No | `node/paxdb.go` |
| `config/app.toml` / `[wasm]` / `memory_cache_size` | Wasm module cache size in MiB; zero disables it. | `100` | No | `node/app.go` → `wasm/x/wasm/module.go` |
| `config/app.toml` / `[wasm]` / `query_gas_limit` | Smart-contract query gas ceiling. | `3000000` | No | `node/app.go` → `wasm/x/wasm/module.go` |
| `config/app.toml` / `[wasm]` / `simulation_gas_limit` | Quoted positive decimal simulation gas ceiling. | Empty: consensus block gas limit | No | `node/app.go` → `wasm/x/wasm/module.go` |
| `config/config.toml` / `[root]` / `autobahn-config-file` | Consensus mode input whose presence selects the node's committed-head notifier path. | Empty | No | `node/app.go` |
| `config/config.toml` / `[root]` / `mode` | Consensus role used to select validator, seed or full-node defaults; archive application initialization also sets archive retention. | `full` | No | `node/params/config.go` |
