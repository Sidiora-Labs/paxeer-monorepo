# EVM JSON-RPC contract

The [node RPC implementation](../../../rpc) serves EVM requests over HTTP and WebSocket. Paxeer mainnet-beta has EVM chain ID `125` (`0x7d`); a receipt is final when its block is committed. Node JSON-RPC uses the root path. The node also registers the `px_` reads described below.

## client-parity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getrawtransactionbyblockhashandindex-j38</code> | <code>jsonrpc_method</code> | [rpc/raw_tx_index.go](../../../rpc/raw_tx_index.go) | <code>eth_getRawTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (hexutil.Bytes, error)</code> |
| <code>interface.7.rpc-eth-getrawtransactionbyblocknumberandindex-j38</code> | <code>jsonrpc_method</code> | [rpc/raw_tx_index.go](../../../rpc/raw_tx_index.go) | <code>eth_getRawTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (hexutil.Bytes, error)</code> |
| <code>interface.7.rpc-eth-getrawtransactionbyhash-j38</code> | <code>jsonrpc_method</code> | [rpc/raw_tx.go](../../../rpc/raw_tx.go) | <code>eth_getRawTransactionByHash(hash common.Hash) (hexutil.Bytes, error)</code> |
| <code>interface.7.rpc-eth-getunclebyblockhashandindex-j38</code> | <code>jsonrpc_method</code> | [rpc/uncle.go](../../../rpc/uncle.go) | <code>eth_getUncleByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (map[string]interface{}, error)</code> |
| <code>interface.7.rpc-eth-getunclebyblocknumberandindex-j38</code> | <code>jsonrpc_method</code> | [rpc/uncle.go](../../../rpc/uncle.go) | <code>eth_getUncleByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (map[string]interface{}, error)</code> |
| <code>interface.7.rpc-eth-getunclecountbyblockhash-j38</code> | <code>jsonrpc_method</code> | [rpc/uncle_count.go](../../../rpc/uncle_count.go) | <code>eth_getUncleCountByBlockHash(blockHash common.Hash) (*hexutil.Uint, error)</code> |
| <code>interface.7.rpc-eth-getunclecountbyblocknumber-j38</code> | <code>jsonrpc_method</code> | [rpc/uncle_count.go](../../../rpc/uncle_count.go) | <code>eth_getUncleCountByBlockNumber(blockNr rpc.BlockNumber) (*hexutil.Uint, error)</code> |
| <code>interface.7.rpc-eth-sendrawtransactionsync-j38</code> | <code>jsonrpc_method</code> | [rpc/send_sync.go](../../../rpc/send_sync.go) | <code>eth_sendRawTransactionSync(input hexutil.Bytes, timeoutMs *hexutil.Uint64) (map[string]interface{}, error)</code> |
| <code>interface.7.rpc-net-listening-j38</code> | <code>jsonrpc_method</code> | [rpc/net.go](../../../rpc/net.go) | <code>net_listening() (bool, error)</code> |
| <code>interface.7.rpc-net-peercount-j38</code> | <code>jsonrpc_method</code> | [rpc/net_peers.go](../../../rpc/net_peers.go) | <code>net_peerCount() (hexutil.Uint, error)</code> |
| <code>interface.7.rpc-txpool-status-j38</code> | <code>jsonrpc_method</code> | [rpc/txpool.go](../../../rpc/txpool.go) | <code>txpool_status() (map[string]hexutil.Uint, error)</code> |
| <code>interface.7.rpc-web3-sha3-j38</code> | <code>jsonrpc_method</code> | [rpc/web3.go](../../../rpc/web3.go) | <code>web3_sha3(input hexutil.Bytes) hexutil.Bytes</code> |

| Field | Rule |
|---|---|
| interface | Methods called by in-repo clients and the go-ethereum client are served on the transport the client uses. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-batch-request-limit-j38</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>evm.batch_request_limit int (default 1000)</code> |
| <code>interface.7.cfg-evm-batch-response-max-size-j38</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>evm.batch_response_max_size int (default 25000000)</code> |
| <code>interface.7.cfg-evm-max-subscriptions-logs-j38</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>evm.max_subscriptions_logs uint64 (default 10000)</code> |
| <code>interface.7.cfg-evm-send-raw-tx-sync-timeout-j38</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>evm.send_raw_tx_sync_timeout duration (default 2s)</code> |
| <code>interface.7.go-evmrpcconfig-config-j38</code> | <code>go_exported</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>type Config struct</code> |
| <code>interface.7.go-evmrpcconfig-configtemplate-j38</code> | <code>go_exported</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>const ConfigTemplate</code> |
| <code>interface.7.go-evmrpcconfig-defaultconfig-j38</code> | <code>go_exported</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>var DefaultConfig Config</code> |
| <code>interface.7.go-evmrpcconfig-readconfig-j38</code> | <code>go_exported</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>func ReadConfig(opts servertypes.AppOptions) (Config, error)</code> |

| Field | Rule |
|---|---|
| interface | ReadConfig loads the supported `evm` keys from application options. ConfigTemplate renders their application configuration. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The rendered template decodes as TOML and round-trips DefaultConfig. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-cors-origins

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-cors-origins</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>CORSOrigins string mapstructure:"cors_origins"</code> |

## config-deny-list

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-deny-list</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>DenyList []string mapstructure:"deny_list"</code> |

| Field | Rule |
|---|---|
| interface | ReadConfig loads the supported `evm` keys from application options. ConfigTemplate renders their application configuration. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The rendered template decodes as TOML and round-trips DefaultConfig. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | No lx_* method is registered by the node; every lx_* name answers -32601 on HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No served name starts with lx_. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug methods and the legacy pax wrappers are HTTP-only; WebSocket also registers txpool methods and pax_sequencerReceipt. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses each denied member of a batch on both transports. |

## config-enable-parallelized-block-trace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-enable-parallelized-block-trace</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>EnableParallelizedBlockTrace bool mapstructure:"enable_parallelized_block_trace"</code> |

## config-enable-test-api

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-enable-test-api</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>EnableTestAPI bool mapstructure:"enable_test_api"</code> |

## config-enable-unsafe-keyring-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-enable-unsafe-keyring-rpc</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>EnableUnsafeKeyringRPC bool mapstructure:"enable_unsafe_keyring_rpc"</code> |

## config-enabled-legacy-pax-apis

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-enabled-legacy-pax-apis</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>EnabledLegacyPaxApis []string mapstructure:"enabled_legacy_pax_apis"</code> |

## config-filter-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-filter-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>FilterTimeout time.Duration mapstructure:"filter_timeout"</code> |

## config-http-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-http-address</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>HTTPAddress string mapstructure:"http_address"</code> |

| Field | Rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug methods and the legacy pax wrappers are HTTP-only; WebSocket also registers txpool methods and pax_sequencerReceipt. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-http-enabled

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-http-enabled</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>HTTPEnabled bool   mapstructure:"http_enabled"</code> |

## config-http-port

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-http-port</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>HTTPPort    int    mapstructure:"http_port"</code> |

## config-idle-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-idle-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>IdleTimeout time.Duration mapstructure:"idle_timeout"</code> |

## config-ip-rate-limit-burst

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-ip-rate-limit-burst</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>IPRateLimitBurst int mapstructure:"ip_rate_limit_burst"</code> |

| Field | Rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-ip-rate-limit-rps

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-ip-rate-limit-rps</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>IPRateLimitRPS float64 mapstructure:"ip_rate_limit_rps"</code> |

| Field | Rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-max-blocks-for-log

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-blocks-for-log</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxBlocksForLog int64 mapstructure:"max_blocks_for_log"</code> |

## config-max-concurrent-simulation-calls

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-concurrent-simulation-calls</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxConcurrentSimulationCalls int mapstructure:"max_concurrent_simulation_calls"</code> |

## config-max-concurrent-trace-calls

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-concurrent-trace-calls</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxConcurrentTraceCalls uint64 mapstructure:"max_concurrent_trace_calls"</code> |

## config-max-log-no-block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-log-no-block</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxLogNoBlock int64 mapstructure:"max_log_no_block"</code> |

## config-max-subscriptions-new-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-subscriptions-new-head</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxSubscriptionsNewHead uint64 mapstructure:"max_subscriptions_new_head"</code> |

## config-max-trace-lookback-blocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-trace-lookback-blocks</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxTraceLookbackBlocks int64 mapstructure:"max_trace_lookback_blocks"</code> |

## config-max-tx-pool-txs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-max-tx-pool-txs</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>MaxTxPoolTxs uint64 mapstructure:"max_tx_pool_txs"</code> |

## config-read-header-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-read-header-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>ReadHeaderTimeout time.Duration mapstructure:"read_header_timeout"</code> |

## config-read-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-read-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>ReadTimeout time.Duration mapstructure:"read_timeout"</code> |

## config-rpc-stats-interval

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-rpc-stats-interval</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>RPCStatsInterval time.Duration mapstructure:"rpc_stats_interval"</code> |

## config-simulation-evm-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-simulation-evm-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>SimulationEVMTimeout time.Duration mapstructure:"simulation_evm_timeout"</code> |

## config-simulation-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-simulation-gas-limit</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>SimulationGasLimit uint64 mapstructure:"simulation_gas_limit"</code> |

## config-slow

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-slow</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>Slow bool mapstructure:"slow"</code> |

## config-trace-bake-enabled

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-enabled</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeEnabled      bool     mapstructure:"trace_bake_enabled"</code> |

## config-trace-bake-queue-size

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-queue-size</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeQueueSize    int      mapstructure:"trace_bake_queue_size"    // in-flight height queue (default 4096)</code> |

## config-trace-bake-snapshot-window

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-snapshot-window</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeSnapshotWindow int64 mapstructure:"trace_bake_snapshot_window" // recent snapshots to keep (default 64)</code> |

## config-trace-bake-tracers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-tracers</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeTracers      []string mapstructure:"trace_bake_tracers"       // tracers to bake (default ["callTracer"])</code> |

## config-trace-bake-use-snapshot

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-use-snapshot</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeUseSnapshot    bool  mapstructure:"trace_bake_use_snapshot"</code> |

## config-trace-bake-window-blocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-window-blocks</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeWindowBlocks int64    mapstructure:"trace_bake_window_blocks" // rolling prune window; 0 disables</code> |

## config-trace-bake-workers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-workers</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceBakeWorkers      int      mapstructure:"trace_bake_workers"       // re-execution goroutines (default 1)</code> |

## config-trace-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-trace-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>TraceTimeout time.Duration mapstructure:"trace_timeout"</code> |

## config-worker-pool-size

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-worker-pool-size</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>WorkerPoolSize int mapstructure:"worker_pool_size"</code> |

## config-worker-queue-size

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-worker-queue-size</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>WorkerQueueSize int mapstructure:"worker_queue_size"</code> |

## config-write-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-write-timeout</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>WriteTimeout time.Duration mapstructure:"write_timeout"</code> |

## config-ws-enabled

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-ws-enabled</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>WSEnabled bool mapstructure:"ws_enabled"</code> |

## config-ws-origins

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-ws-origins</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>WSOrigins string mapstructure:"ws_origins"</code> |

## config-ws-port

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.cfg-evm-ws-port</code> | <code>config_key</code> | [rpc/config/config.go](../../../rpc/config/config.go) | <code>WSPort    int  mapstructure:"ws_port"</code> |

## debug-getrawblock

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-getrawblock</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) GetRawBlock(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)</code> |

## debug-getrawheader

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-getrawheader</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) GetRawHeader(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)</code> |

## debug-getrawreceipts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-getrawreceipts</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) GetRawReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ []hexutil.Bytes, returnErr error)</code> |

## debug-getrawtransaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-getrawtransaction</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) GetRawTransaction(ctx context.Context, hash common.Hash) (_ hexutil.Bytes, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | debug_getRawTransaction returns the envelope; debug_getRawHeader, debug_getRawBlock and debug_getRawReceipts stay refused; tracer outputs follow geth shapes. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | debug_traceBlockByNumber returns one entry per EVM transaction in block order. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## debug-traceblockbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-traceblockbyhash</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

## debug-traceblockbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-traceblockbynumber</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

## debug-tracecall

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-tracecall</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) TraceCall(ctx context.Context, args export.TransactionArgs, blockNrOrHash rpc.BlockNumberOrHash, config *tracers.TraceCallConfig) (result interface{}, returnErr error)</code> |

## debug-tracestateaccess

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-tracestateaccess</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) TraceStateAccess(ctx context.Context, hash common.Hash) (result interface{}, returnErr error)</code> |

## debug-tracetransaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-tracetransaction</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *DebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

## debug-tracetransactionprofile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-debug-tracetransactionprofile</code> | <code>jsonrpc_method</code> | [rpc/trace_profile.go](../../../rpc/trace_profile.go) | <code>func (api *DebugAPI) TraceTransactionProfile(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

## echo-echo

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-echo-echo</code> | <code>jsonrpc_method</code> | [rpc/echo.go](../../../rpc/echo.go) | <code>func (a *EchoAPI) Echo(data string) string</code> |

## error-codes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.err-evm-not-supported-32000-j38</code> | <code>error_code</code> | [rpc/rpc_unsupported.go](../../../rpc/rpc_unsupported.go) | <code>const ErrCodeEVMNotSupported = -32000</code> |
| <code>interface.7.err-pax-legacy-not-enabled-32601-j38</code> | <code>error_code</code> | [rpc/pax_legacy_http.go](../../../rpc/pax_legacy_http.go) | <code>paxLegacyNotEnabled = -32601</code> |
| <code>interface.7.err-rate-limited-32005-j38</code> | <code>error_code</code> | [rpc/ip_rate_limit.go](../../../rpc/ip_rate_limit.go) | <code>-32005 rate limit exceeded (HTTP 429)</code> |
| <code>interface.7.err-revert-3-j38</code> | <code>error_code</code> | [rpc/simulate.go](../../../rpc/simulate.go) | <code>func (e *RevertError) ErrorCode() int  // 3</code> |
| <code>interface.7.err-send-sync-timeout-4-j38</code> | <code>error_code</code> | [rpc/send_sync.go](../../../rpc/send_sync.go) | <code>4 transaction not yet included (data: transaction hash)</code> |
| <code>interface.7.err-sequencer-not-served-32000-j38</code> | <code>error_code</code> | [rpc/sequencer_receipt.go](../../../rpc/sequencer_receipt.go) | <code>sequencerNotServedCode = -32000</code> |

| Field | Rule |
|---|---|
| interface | Error codes: -32000 unsupported or not served, -32601 method not found or pax not enabled, 3 revert, -32005 rate limited, 4 sync send timeout. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | Each code is produced by exactly one documented path. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-accounts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-accounts</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) Accounts(ctx context.Context) (result []common.Address, returnErr error)</code> |

## eth-blobbasefee

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-blobbasefee</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) BlobBaseFee(ctx context.Context) (result *hexutil.Big, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | Request latency is recorded under the served method name on the evmrpc meter. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-blocknumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-blocknumber</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) BlockNumber(ctx context.Context) (result hexutil.Uint64, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-call</code> | <code>jsonrpc_method</code> | [rpc/simulate.go](../../../rpc/simulate.go) | <code>func (s *SimulationAPI) Call(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride, blockOverrides *export.BlockOverrides) (result hexutil.Bytes, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-chainid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-chainid</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) ChainId(ctx context.Context) *hexutil.Big</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-coinbase

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-coinbase</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) Coinbase(ctx context.Context) (addr common.Address, err error)</code> |

## eth-createaccesslist

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-createaccesslist</code> | <code>jsonrpc_method</code> | [rpc/simulate.go](../../../rpc/simulate.go) | <code>func (s *SimulationAPI) CreateAccessList(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash) (result *AccessListResult, returnErr error)</code> |

## eth-estimategas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-estimategas</code> | <code>jsonrpc_method</code> | [rpc/simulate.go](../../../rpc/simulate.go) | <code>func (s *SimulationAPI) EstimateGas(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride) (result hexutil.Uint64, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-estimategasaftercalls

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-estimategasaftercalls</code> | <code>jsonrpc_method</code> | [rpc/simulate.go](../../../rpc/simulate.go) | <code>func (s *SimulationAPI) EstimateGasAfterCalls(ctx context.Context, args export.TransactionArgs, calls []export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride) (result hexutil.Uint64, returnErr error)</code> |

## eth-feehistory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-feehistory</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) FeeHistory(ctx context.Context, blockCount gmath.HexOrDecimal64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (result *FeeHistoryResult, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gasprice

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gasprice</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) GasPrice(ctx context.Context) (result *hexutil.Big, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gaspricehelper

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gaspricehelper</code> | <code>go_exported</code> | [rpc/info.go](../../../rpc/info.go) | <code>func GasPriceHelper(i *InfoAPI, ctx context.Context, baseFee *big.Int, totalGasUsedPrevBlock uint64, medianRewardPrevBlock *big.Int) (*hexutil.Big, error)</code> |

| Field | Rule |
|---|---|
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The registered method set follows the concrete services in [server.go](../../../rpc/server.go); `pax_sequencerReceipt` uses the configured receipt source. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getbalance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getbalance</code> | <code>jsonrpc_method</code> | [rpc/state.go](../../../rpc/state.go) | <code>func (a *StateAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Big, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getblockbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getblockbyhash</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getblockbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getblockbynumber</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | When a pending source is configured, the pending tag reads its header fields, transaction count and transactions by index; otherwise it reads the latest block. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getblockreceipts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getblockreceipts</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)</code> |

## eth-getblocktransactioncountbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getblocktransactioncountbyhash</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)</code> |

## eth-getblocktransactioncountbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getblocktransactioncountbynumber</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | When a pending source is configured, the pending tag reads its header fields, transaction count and transactions by index; otherwise it reads the latest block. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getcode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getcode</code> | <code>jsonrpc_method</code> | [rpc/state.go](../../../rpc/state.go) | <code>func (a *StateAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getfilterchanges

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getfilterchanges</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) GetFilterChanges(ctx context.Context, filterID ethrpc.ID) (res interface{}, err error)</code> |

## eth-getfilterlogs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getfilterlogs</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) GetFilterLogs(ctx context.Context, filterID ethrpc.ID) (res []*FilterLog, err error)</code> |

## eth-getlogs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getlogs</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | eth_getLogs fails when the earliest height cannot be read and answers unknown block for an unknown or zero blockHash. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A log query never scans from height 0 because of a store read failure. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getnonce

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getnonce</code> | <code>jsonrpc_method</code> | [rpc/state.go](../../../rpc/state.go) | <code>func (a *StateAPI) GetNonce(ctx context.Context, address common.Address) uint64</code> |

## eth-getproof

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getproof</code> | <code>jsonrpc_method</code> | [rpc/state.go](../../../rpc/state.go) | <code>func (a *StateAPI) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockNrOrHash rpc.BlockNumberOrHash) (result *ProofResult, returnErr error)</code> |

## eth-getstorageat

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getstorageat</code> | <code>jsonrpc_method</code> | [rpc/state.go](../../../rpc/state.go) | <code>func (a *StateAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactionbyblockhashandindex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyblockhashandindex</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

## eth-gettransactionbyblocknumberandindex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyblocknumberandindex</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

| Field | Rule |
|---|---|
| interface | When a pending source is configured, the pending tag reads its header fields, transaction count and transactions by index; otherwise it reads the latest block. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactionbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyhash</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactioncount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactioncount</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactionerrorbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionerrorbyhash</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

## eth-gettransactionreceipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionreceipt</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getvmerror

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-getvmerror</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

## eth-maxpriorityfeepergas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-maxpriorityfeepergas</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) MaxPriorityFeePerGas(ctx context.Context) (fee *hexutil.Big, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-newblockfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-newblockfilter</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) NewBlockFilter(ctx context.Context) (id ethrpc.ID, err error)</code> |

## eth-newfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-newfilter</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) NewFilter(ctx context.Context, crit filters.FilterCriteria) (id ethrpc.ID, err error)</code> |

## eth-newpendingtransactionfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-newpendingtransactionfilter</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) NewPendingTransactionFilter(ctx context.Context, fullTx *bool) (id ethrpc.ID, err error)</code> |

## eth-sendrawtransaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-sendrawtransaction</code> | <code>jsonrpc_method</code> | [rpc/send.go](../../../rpc/send.go) | <code>func (s *SendAPI) SendRawTransaction(ctx context.Context, input hexutil.Bytes) (hash common.Hash, err error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-sendtransaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-sendtransaction</code> | <code>jsonrpc_method</code> | [rpc/send.go](../../../rpc/send.go) | <code>func (s *SendAPI) SendTransaction(ctx context.Context, args export.TransactionArgs) (result common.Hash, returnErr error)</code> |

## eth-sign

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-sign</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error)</code> |

## eth-signtransaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-signtransaction</code> | <code>jsonrpc_method</code> | [rpc/send.go](../../../rpc/send.go) | <code>func (s *SendAPI) SignTransaction(ctx context.Context, args apitypes.SendTxArgs, _ *string) (result *export.SignTransactionResult, returnErr error)</code> |

## eth-subscribe

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-subscribe</code> | <code>jsonrpc_method</code> | [core/deps/geth/rpc/json.go](../../../core/deps/geth/rpc/json.go) | <code>subscribeMethodSuffix    = "_subscribe"</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-syncing

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-syncing</code> | <code>jsonrpc_method</code> | [rpc/info.go](../../../rpc/info.go) | <code>func (i *InfoAPI) Syncing(ctx context.Context) (result any, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-uninstallfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-uninstallfilter</code> | <code>jsonrpc_method</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func (a *FilterAPI) UninstallFilter(ctx context.Context, filterID ethrpc.ID) (res bool)</code> |

## eth-unsubscribe

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-eth-unsubscribe</code> | <code>jsonrpc_method</code> | [core/deps/geth/rpc/json.go](../../../core/deps/geth/rpc/json.go) | <code>unsubscribeMethodSuffix  = "_unsubscribe"</code> |

## height-availability

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## http-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.http-jsonrpc-root</code> | <code>http_route</code> | [rpc/rpcstack.go](../../../rpc/rpcstack.go) | <code>func CheckPath(r *http.Request, path string) bool</code> |

| Field | Rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## metrics

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.env-evm-debug-metrics-j38</code> | <code>env_var</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>EVM_DEBUG_METRICS</code> |
| <code>interface.7.met-evmrpc-trace-bake-baked-total-j38</code> | <code>metric</code> | [rpc/metrics.go](../../../rpc/metrics.go) | <code>evmrpc_trace_bake_baked_total Int64ObservableCounter</code> |
| <code>interface.7.met-evmrpc-trace-bake-dropped-total-j38</code> | <code>metric</code> | [rpc/metrics.go](../../../rpc/metrics.go) | <code>evmrpc_trace_bake_dropped_total Int64ObservableCounter</code> |
| <code>interface.7.met-evmrpc-trace-bake-failed-total-j38</code> | <code>metric</code> | [rpc/metrics.go](../../../rpc/metrics.go) | <code>evmrpc_trace_bake_failed_total Int64ObservableCounter</code> |

| Field | Rule |
|---|---|
| interface | Request latency is recorded under the served method name on the evmrpc meter. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## metrics-legacy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.met-pax-evm-subscriptions-errors</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>telemetry.IncrCounter(1, "pax", "evm", "subscriptions", "errors")</code> |
| <code>interface.7.met-pax-rpc-request-counter</code> | <code>metric</code> | [utils/metrics/metrics_util.go](../../../utils/metrics/metrics_util.go) | <code>func IncrementRpcRequestCounter(endpoint string, connectionType string, success bool)</code> |
| <code>interface.7.met-pax-rpc-request-latency-ms</code> | <code>metric</code> | [utils/metrics/metrics_util.go](../../../utils/metrics/metrics_util.go) | <code>func MeasureRpcRequestLatency(endpoint string, connectionType string, startTime time.Time)</code> |
| <code>interface.7.met-pax-websocket-connect</code> | <code>metric</code> | [utils/metrics/metrics_util.go](../../../utils/metrics/metrics_util.go) | <code>func IncWebsocketConnects()</code> |

| Field | Rule |
|---|---|
| interface | Request latency is recorded under the served method name on the evmrpc meter. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## metrics-workerpool

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.met-evmrpc-db-semaphore-available</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_db_semaphore_available")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-avg-wait-ms</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_db_semaphore_avg_wait_ms")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-capacity</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_db_semaphore_capacity")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-inuse</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_db_semaphore_inuse")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-wait-count</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_db_semaphore_wait_count")</code> |
| <code>interface.7.met-evmrpc-getlogs-avg-blockrange</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_getlogs_avg_blockrange")</code> |
| <code>interface.7.met-evmrpc-getlogs-avg-latency-ms</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_getlogs_avg_latency_ms")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-backpressure</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_errors_backpressure")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-range-too-large</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_errors_range_too_large")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-rate-limited</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_errors_rate_limited")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_errors_total")</code> |
| <code>interface.7.met-evmrpc-getlogs-max-latency-ms</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_getlogs_max_latency_ms")</code> |
| <code>interface.7.met-evmrpc-getlogs-peak-blockrange</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_peak_blockrange")</code> |
| <code>interface.7.met-evmrpc-getlogs-requests-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_requests_total")</code> |
| <code>interface.7.met-evmrpc-getlogs-success-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_getlogs_success_total")</code> |
| <code>interface.7.met-evmrpc-getlogs-tps</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_getlogs_tps")</code> |
| <code>interface.7.met-evmrpc-subscriptions-active</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_subscriptions_active")</code> |
| <code>interface.7.met-evmrpc-workerpool-avg-exec-time-ms</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_workerpool_avg_exec_time_ms")</code> |
| <code>interface.7.met-evmrpc-workerpool-avg-queue-wait-ms</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_workerpool_avg_queue_wait_ms")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-capacity</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_queue_capacity")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-depth</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_queue_depth")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-peak</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_queue_peak")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-utilization-pct</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Float64Gauge("evmrpc_workerpool_queue_utilization_pct")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-completed-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_tasks_completed_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-panicked-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_tasks_panicked_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-rejected-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_tasks_rejected_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-submitted-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_tasks_submitted_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-workers-active</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_workers_active")</code> |
| <code>interface.7.met-evmrpc-workerpool-workers-idle</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_workers_idle")</code> |
| <code>interface.7.met-evmrpc-workerpool-workers-total</code> | <code>metric</code> | [rpc/worker_pool_metrics.go](../../../rpc/worker_pool_metrics.go) | <code>Int64Gauge("evmrpc_workerpool_workers_total")</code> |

## net-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-net-version</code> | <code>jsonrpc_method</code> | [rpc/net.go](../../../rpc/net.go) | <code>func (i *NetAPI) Version(ctx context.Context) string</code> |

| Field | Rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-associate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-associate</code> | <code>jsonrpc_method</code> | [rpc/association.go](../../../rpc/association.go) | <code>func (t *AssociationAPI) Associate(ctx context.Context, req *AssociateRequest) (returnErr error)</code> |

## pax-getblockbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbyhash</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getblockbyhashexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbyhashexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *PaxBlockAPI) GetBlockByHashExcludeTraceFail(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

## pax-getblockbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbynumber</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getblockbynumberexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbynumberexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *PaxBlockAPI) GetBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

## pax-getblockreceipts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblockreceipts</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)</code> |

## pax-getblocktransactioncountbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblocktransactioncountbyhash</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)</code> |

## pax-getblocktransactioncountbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getblocktransactioncountbynumber</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | When a pending source is configured, the pending tag reads its header fields, transaction count and transactions by index; otherwise it reads the latest block. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getcosmostx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getcosmostx</code> | <code>jsonrpc_method</code> | [rpc/association.go](../../../rpc/association.go) | <code>func (t *AssociationAPI) GetCosmosTx(ctx context.Context, ethHash common.Hash) (result string, returnErr error)</code> |

## pax-getevmaddress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getevmaddress</code> | <code>jsonrpc_method</code> | [rpc/association.go](../../../rpc/association.go) | <code>func (t *AssociationAPI) GetEVMAddress(ctx context.Context, paxAddress string) (result string, returnErr error)</code> |

## pax-getevmtx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getevmtx</code> | <code>jsonrpc_method</code> | [rpc/association.go](../../../rpc/association.go) | <code>func (t *AssociationAPI) GetEvmTx(ctx context.Context, cosmosHash string) (result string, returnErr error)</code> |

## pax-getfilterchanges

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getfilterchanges</code> | <code>jsonrpc_method</code> | [rpc/pax_filter.go](../../../rpc/pax_filter.go) | <code>func (a *PaxFilterAPI) GetFilterChanges(ctx context.Context, filterID ethrpc.ID) (res interface{}, err error)</code> |

## pax-getfilterlogs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getfilterlogs</code> | <code>jsonrpc_method</code> | [rpc/pax_filter.go](../../../rpc/pax_filter.go) | <code>func (a *PaxFilterAPI) GetFilterLogs(ctx context.Context, filterID ethrpc.ID) (res []*FilterLog, err error)</code> |

## pax-getlogs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getlogs</code> | <code>jsonrpc_method</code> | [rpc/pax_filter.go](../../../rpc/pax_filter.go) | <code>func (a *PaxFilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error)</code> |

| Field | Rule |
|---|---|
| interface | eth_getLogs fails when the earliest height cannot be read and answers unknown block for an unknown or zero blockHash. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A log query never scans from height 0 because of a store read failure. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getpaxaddress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getpaxaddress</code> | <code>jsonrpc_method</code> | [rpc/association.go](../../../rpc/association.go) | <code>func (t *AssociationAPI) GetPaxAddress(ctx context.Context, ethAddress common.Address) (result string, returnErr error)</code> |

## pax-gettransactionbyblockhashandindex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyblockhashandindex</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

## pax-gettransactionbyblocknumberandindex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyblocknumberandindex</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

| Field | Rule |
|---|---|
| interface | When a pending source is configured, the pending tag reads its header fields, transaction count and transactions by index; otherwise it reads the latest block. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-gettransactionbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyhash</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error)</code> |

## pax-gettransactioncount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactioncount</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error)</code> |

## pax-gettransactionerrorbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionerrorbyhash</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

## pax-gettransactionreceipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionreceipt</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)</code> |

## pax-gettransactionreceiptexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionreceiptexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *PaxTransactionAPI) GetTransactionReceiptExcludeTraceFail(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)</code> |

## pax-getvmerror

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-getvmerror</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

## pax-newblockfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-newblockfilter</code> | <code>jsonrpc_method</code> | [rpc/pax_filter.go](../../../rpc/pax_filter.go) | <code>func (a *PaxFilterAPI) NewBlockFilter(ctx context.Context) (id ethrpc.ID, err error)</code> |

## pax-newfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-newfilter</code> | <code>jsonrpc_method</code> | [rpc/pax_filter.go](../../../rpc/pax_filter.go) | <code>func (a *PaxFilterAPI) NewFilter(ctx context.Context, crit filters.FilterCriteria) (id ethrpc.ID, err error)</code> |

## pax-sequencerreceipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-sequencerreceipt</code> | <code>jsonrpc_method</code> | [rpc/sequencer_receipt.go](../../../rpc/sequencer_receipt.go) | <code>func (a *SequencerReceiptAPI) SequencerReceipt(ctx context.Context, txHash common.Hash) (result *SequencerReceiptResult, returnErr error)</code> |

## pax-sign

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-sign</code> | <code>jsonrpc_method</code> | [rpc/tx.go](../../../rpc/tx.go) | <code>func (t *TransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error)</code> |

## pax-traceblockbyhashexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-traceblockbyhashexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *PaxDebugAPI) TraceBlockByHashExcludeTraceFail(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

## pax-traceblockbynumberexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-traceblockbynumberexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/tracers.go](../../../rpc/tracers.go) | <code>func (api *PaxDebugAPI) TraceBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

## pax-uninstallfilter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax-uninstallfilter</code> | <code>jsonrpc_method</code> | [rpc/pax_filter.go](../../../rpc/pax_filter.go) | <code>func (a *PaxFilterAPI) UninstallFilter(ctx context.Context, filterID ethrpc.ID) (res bool)</code> |

## pax2-getblockbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbyhash</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax2-getblockbyhashexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbyhashexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *PaxBlockAPI) GetBlockByHashExcludeTraceFail(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

## pax2-getblockbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbynumber</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax2-getblockbynumberexcludetracefail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbynumberexcludetracefail</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *PaxBlockAPI) GetBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

## pax2-getblockreceipts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockreceipts</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)</code> |

## pax2-getblocktransactioncountbyhash

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblocktransactioncountbyhash</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)</code> |

## pax2-getblocktransactioncountbynumber

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-pax2-getblocktransactioncountbynumber</code> | <code>jsonrpc_method</code> | [rpc/block.go](../../../rpc/block.go) | <code>func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | When a pending source is configured, the pending tag reads its header fields, transaction count and transactions by index; otherwise it reads the latest block. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## rpc-modules

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-rpc-modules</code> | <code>jsonrpc_method</code> | [core/deps/geth/rpc/server.go](../../../core/deps/geth/rpc/server.go) | <code>func (s *RPCService) Modules() map[string]string</code> |

## sequencer-options

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.go-newreceiptlogfeed-j38</code> | <code>go_exported</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func NewReceiptLogFeed(committedHeight int64) *ReceiptLogFeed</code> |
| <code>interface.7.go-pendingoption-j38</code> | <code>go_exported</code> | [rpc/server_options.go](../../../rpc/server_options.go) | <code>type PendingOption; func NewPendingOption(source PendingSnapshot) (PendingOption, error)</code> |
| <code>interface.7.go-receiptlogfeed-j38</code> | <code>go_exported</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>type ReceiptLogFeed struct</code> |
| <code>interface.7.go-sequencerforwardoption-j38</code> | <code>go_exported</code> | [rpc/server_options.go](../../../rpc/server_options.go) | <code>type SequencerForwardOption; func NewSequencerForwardOption(raw string) (SequencerForwardOption, error)</code> |

| Field | Rule |
|---|---|
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | SequencerOption and SequencerForwardOption are mutually exclusive on one server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## served-methods

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.go-associaterequest-j38</code> | <code>go_exported</code> | [rpc/association.go](../../../rpc/association.go) | <code>type AssociateRequest struct</code> |
| <code>interface.7.go-blockheadernotifier-j38</code> | <code>go_exported</code> | [rpc/notifier.go](../../../rpc/notifier.go) | <code>type BlockHeaderNotifier struct</code> |
| <code>interface.7.go-connectiontypehttp-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>var ConnectionTypeHTTP ConnectionType = "http"</code> |
| <code>interface.7.go-evmserver-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>type EVMServer interface</code> |
| <code>interface.7.go-httpconfig-j38</code> | <code>go_exported</code> | [rpc/rpcstack.go](../../../rpc/rpcstack.go) | <code>type HTTPConfig struct</code> |
| <code>interface.7.go-httpserver-j38</code> | <code>go_exported</code> | [rpc/rpcstack.go](../../../rpc/rpcstack.go) | <code>type HTTPServer struct</code> |
| <code>interface.7.go-httpserver-servedmethods-j38</code> | <code>go_exported</code> | [rpc/served_methods.go](../../../rpc/served_methods.go) | <code>func (h *HTTPServer) HTTPMethods() []string; func (h *HTTPServer) WSMethods() []string</code> |
| <code>interface.7.go-latestctxheight-j38</code> | <code>go_exported</code> | [rpc/utils.go](../../../rpc/utils.go) | <code>const LatestCtxHeight int64 = -1</code> |
| <code>interface.7.go-localaddress-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>const LocalAddress = "0.0.0.0"</code> |
| <code>interface.7.go-newblockapi-j38</code> | <code>go_exported</code> | [rpc/block.go](../../../rpc/block.go) | <code>func NewBlockAPI(tmClient client.LocalClient, k *keeper.Keeper, ctxProvider func(int64) sdk.Context, txConfigProvider func(int64) client.TxConfig, connectionType ConnectionType, watermarks *WatermarkManager, globalBlockCache BlockCache, cacheCreationMutex *sync.Mutex) *BlockAPI</code> |
| <code>interface.7.go-newblockcache-j38</code> | <code>go_exported</code> | [rpc/filter.go](../../../rpc/filter.go) | <code>func NewBlockCache(maxSize int) BlockCache</code> |
| <code>interface.7.go-newblockheadernotifier-j38</code> | <code>go_exported</code> | [rpc/notifier.go](../../../rpc/notifier.go) | <code>func NewBlockHeaderNotifier(capacity int) *BlockHeaderNotifier</code> |
| <code>interface.7.go-newechoapi-j38</code> | <code>go_exported</code> | [rpc/echo.go](../../../rpc/echo.go) | <code>func NewEchoAPI() *EchoAPI</code> |
| <code>interface.7.go-newevmhttpserver-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>func NewEVMHTTPServer(</code> |
| <code>interface.7.go-newevmwebsocketserver-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>func NewEVMWebSocketServer(</code> |
| <code>interface.7.go-newhttpserver-j38</code> | <code>go_exported</code> | [rpc/rpcstack.go](../../../rpc/rpcstack.go) | <code>func NewHTTPServer(timeouts rpc.HTTPTimeouts) *HTTPServer</code> |
| <code>interface.7.go-newpaxblockapi-j38</code> | <code>go_exported</code> | [rpc/block.go](../../../rpc/block.go) | <code>func NewPaxBlockAPI(</code> |
| <code>interface.7.go-newsequenceroption-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>func NewSequencerOption(accepter SequencerAccepter, feed *ReceiptLogFeed) SequencerOption</code> |
| <code>interface.7.go-newwatermarkmanager-j38</code> | <code>go_exported</code> | [rpc/watermark_manager.go](../../../rpc/watermark_manager.go) | <code>func NewWatermarkManager(</code> |
| <code>interface.7.go-pendingsnapshot-j38</code> | <code>go_exported</code> | [rpc/utils.go](../../../rpc/utils.go) | <code>type PendingSnapshot interface</code> |
| <code>interface.7.go-pendingtx-j38</code> | <code>go_exported</code> | [rpc/utils.go](../../../rpc/utils.go) | <code>type PendingTx struct</code> |
| <code>interface.7.go-sequenceraccepter-j38</code> | <code>go_exported</code> | [rpc/send.go](../../../rpc/send.go) | <code>type SequencerAccepter interface</code> |
| <code>interface.7.go-sequenceroption-j38</code> | <code>go_exported</code> | [rpc/server.go](../../../rpc/server.go) | <code>type SequencerOption struct</code> |
| <code>interface.7.go-sequencerreceiptoption-j38</code> | <code>go_exported</code> | [rpc/sequencer_receipt.go](../../../rpc/sequencer_receipt.go) | <code>type SequencerReceiptOption struct</code> |
| <code>interface.7.go-servedmethods-j38</code> | <code>go_exported</code> | [rpc/served_methods.go](../../../rpc/served_methods.go) | <code>func ServedMethods(apis []rpc.API) []string</code> |
| <code>interface.7.go-tracecontextprovider-j38</code> | <code>go_exported</code> | [rpc/simulate.go](../../../rpc/simulate.go) | <code>type TraceContextProvider func(int64) (sdk.Context, func())</code> |

| Field | Rule |
|---|---|
| interface | No lx_* method is registered by the node; every lx_* name answers -32601 on HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | No served name starts with lx_. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | Method registration follows [geth service rules](../../../core/deps/geth/rpc/service.go); [ServedMethods](../../../rpc/served_methods.go) lists methods for each transport. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## test-incrementpointerversion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-test-incrementpointerversion</code> | <code>jsonrpc_method</code> | [rpc/test.go](../../../rpc/test.go) | <code>func (a *TestAPI) IncrementPointerVersion(pointerType string, offset int16) error</code> |

## txpool-content

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-txpool-content</code> | <code>jsonrpc_method</code> | [rpc/txpool.go](../../../rpc/txpool.go) | <code>func (t *TxPoolAPI) Content(ctx context.Context) (result map[string]map[string]map[string]*export.RPCTransaction, returnErr error)</code> |

| Field | Rule |
|---|---|
| interface | Methods called by in-repo clients and the go-ethereum client are served on the transport the client uses. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## web3-clientversion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.rpc-web3-clientversion</code> | <code>jsonrpc_method</code> | [rpc/web3.go](../../../rpc/web3.go) | <code>func (w *Web3API) ClientVersion() string</code> |

## ws-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.ws-endpoint-root</code> | <code>ws_route</code> | [rpc/rpcstack.go](../../../rpc/rpcstack.go) | <code>func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |

| Field | Rule |
|---|---|
| interface | Method registration follows [geth service rules](../../../core/deps/geth/rpc/service.go); [ServedMethods](../../../rpc/served_methods.go) lists methods for each transport. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Field | Rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug methods and the legacy pax wrappers are HTTP-only; WebSocket also registers txpool methods and pax_sequencerReceipt. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## ws-logs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.ws-eth-subscribe-logs</code> | <code>ws_route</code> | [rpc/subscribe.go](../../../rpc/subscribe.go) | <code>func (a *SubscriptionAPI) Logs(ctx context.Context, filter *filters.FilterCriteria) (s *rpc.Subscription, _err error)</code> |

| Field | Rule |
|---|---|
| interface | newHeads carries the header keys of eth_getBlockByNumber with stateRoot from the header AppHash and bloom and gasUsed from EVM receipts; logs subscriptions end instead of sending an error as a log. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | header.Hash() of a received head equals the eth_getBlockByNumber hash of the same height. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## ws-new-heads

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.ws-eth-subscribe-newheads</code> | <code>ws_route</code> | [rpc/subscribe.go](../../../rpc/subscribe.go) | <code>func (a *SubscriptionAPI) NewHeads(ctx context.Context) (s *rpc.Subscription, err error)</code> |

| Field | Rule |
|---|---|
| interface | newHeads carries the header keys of eth_getBlockByNumber with stateRoot from the header AppHash and bloom and gasUsed from EVM receipts; logs subscriptions end instead of sending an error as a log. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | header.Hash() of a received head equals the eth_getBlockByNumber hash of the same height. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## ws-new-pending-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.ws-eth-subscribe-newpendingtransactions</code> | <code>ws_route</code> | [rpc/send.go](../../../rpc/send.go) | <code>func (s *SendAPI) NewPendingTransactions(ctx context.Context) (sub *rpc.Subscription, err error)</code> |

## ws-transport

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.7.go-wsconfig-j38</code> | <code>go_exported</code> | [rpc/rpcstack.go](../../../rpc/rpcstack.go) | <code>type WsConfig struct</code> |

| Field | Rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug methods and the legacy pax wrappers are HTTP-only; WebSocket also registers txpool methods and pax_sequencerReceipt. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per [error codes](#error-codes); -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## px-node-root-account

The px namespace uses the same node root `/` as eth
and pax. Both HTTP and WebSocket register `px_resolveAccount`, `px_getAccount`,
`px_getBalances`, `px_listAssets`, `px_getNetwork`, `px_getCapabilities` and
`px_getRouteCatalogue`. Parameters and results follow
`services/platform/hosted/gateway/openrpc.json`. Existing px receipt-key
registration remains available independently.

Account resolution checks both addr binding directions and the derived native
main account. Account and coin reads share one committed watermark; balances
and nonces use the existing state/transaction paths and bank amounts retain
all 256 bits as decimal strings. A bare native account ID returns `-32001`,
`Paxeer read unavailable`, data.code `native_account_reverse_resolution_unsupported`;
conflicts return `conflicting_identity_binding`. An unbound DID has null paxeer
state; its coin-list read returns `identity_binding_unavailable`.

Asset enumeration uses `[evm].px_indexer_url` or `WithPxIndexerURL` on each server; no hosted gateway
or environment variable is required. A missing indexer returns `-32001`,
`Paxeer read unavailable`, data.code `indexer_not_configured`. Each call fetches exactly one indexer page with limit=16, preserving producer
registration order. An omitted cursor (absent params or an empty positional
array) starts enumeration; a present cursor whose value is JSON null is invalid.
The HTTP and WebSocket `pxParamsGuard` rewrites an explicit null first parameter
to its invalid-params method before positional decoding. `ListAssets` accepts a
string pointer, where nil starts the first page; omission still returns `-32001`
if the indexer is unavailable. A supplied cursor must decode to a canonical positive signed-64-bit
decimal string, passed unchanged to the indexer. Invalid arguments return
`-32602` before indexer access. The joined result includes `order: "registration"`,
`joined_limit: 16`, and a string `next_cursor` for continuation or null at the end.
Continuation remains valid across a node restart against the same indexer records.
Local metadata uses one committed height under a ten-second aggregate deadline;
malformed indexer envelopes or nonadvancing continuations fail closed.
Bank asset decimals use the committed metadata's display-unit exponent; supply
remains an integer string in base units.
Failed per-asset metadata reads retain the documented null fields. Successful
ledger reads alone carry authenticated snapshot verification and paused state.

Network identity comes from the node chain context and EVM keeper. An invalid
or unavailable local consensus Status leaves consensus null with reason
`consensus_unavailable`. Capability probes execute the seven documented views
at one committed height and require canonical ABI output; the result is cached
for fifteen seconds. Hosted capability write gating stays with the gateway.
The catalogue carries committed product route definitions, advertises only
`/`, reports actual node registrations and marks hosted dependencies unknown.
It publishes no private route bindings or upstream topology.

`px_register` and `px_requestFunds` validate positional arguments before
returning `-32004`, `Hosted gateway service operation`, with data
`{code:"hosted_gateway_service_required", method:<requested method>}`.
Malformed parameters return `-32602`. These are deliberate, dispatched
refusals on both transports, never method-not-found responses. Every px method
records the same per-method/error/transport metrics as the existing RPC APIs.
The node handlers perform no chain writes and require no hosted credentials.

## px-node-root-history

The node also registers `px_getHistory`, `px_getUnifiedHistory`
and `px_listHolderRecords` at node root `/` on HTTP and WebSocket. Their
positional parameters and result schemas follow
`services/platform/hosted/gateway/openrpc.json`. These methods share the
existing px indexer configuration (`evm.px_indexer_url` / `WithPxIndexerURL`),
local account resolution, committed watermark and RPC metrics. No hosted
credential or gateway upstream is needed. The handlers perform no chain writes.

History and holder enumeration come from `/v1/history/{account}` and
`/v1/holders/{address}` of the configured indexer. Amounts remain exact decimal
strings. Asset records are joined from `/v1/assets/{asset}` and registry
metadata; missing asset records yield null metadata. Indexer `indexedHeight`,
readiness from `/readyz`, and the node's pinned `chainHeight` remain distinct.
Indexer readiness never promotes a transfer's finality. HTTP and WebSocket
expose the same result and error contract, including per-method metrics.

Unified history resolves the binding locally and enumerates persisted owned
accounts using the kernel account codec and owner rules over the committed
`lxphost` registry. It includes the binding's canonical main account and every
owned persisted account, with at most sixteen keys including the EVM address.
Each side uses the same position cursor. Rows merge in indexer order and
identical indexer row keys are deduplicated. An unavailable, corrupt or oversized
native enumeration returns `-32001`, `History unavailable`, data.code
`native_account_enumeration_unavailable`; an EVM-only partial unified answer
must not be substituted.

The existing indexer and gateway store implementations order positions oldest
first, strictly after the supplied cursor. Node reads use that cursor direction, with 32 lowercase hex digits encoding a signed-range
height and two uint32 indexes. They keep every row at a page's last position
together so advancing a cursor cannot lose self-transfer legs. The result stays
within OpenRPC's 100-row bound; an indivisible position exceeding it fails with
`-32001`, `History unavailable`, data.code `history_position_page_too_large`.

Invalid selectors, unbound DID history selectors, legacy 112-hex cursors,
invalid kinds and limits outside 1–100 return `-32602`, `Invalid params`.
Missing indexer configuration returns `-32001`, `History unavailable`,
data.code `indexer_not_configured`. Unreachable or invalid indexer reads return
that same code/message with `indexer_unreachable`, `indexer_unavailable`,
`invalid_indexer_response`, `registry_unavailable` or
`invalid_indexer_readiness` as appropriate. An indexer parameter refusal
preserves its refusal document as `-32602` data. Local state/binding failures
retain the account API's `Paxeer read unavailable` errors. These registered
methods never use `-32601` to represent an unavailable backend. Existing
`px_register` and `px_requestFunds` hosted-service refusals remain dispatched.

## px-node-root-consensus-and-receipts

The node exposes this group at root `/` over both HTTP
and WebSocket. These are dispatched methods in `HTTPMethods` and `WSMethods`;
`pax_sequencerReceipt` is served under that method name and bypasses the legacy
pax allowlist. There is no `px_sequencerReceipt` alias.

| Method | Parameters | Local authority and result |
|---|---|---|
| `px_getCommit` | One positive decimal u64 height string | Local consensus Commit; decimal `height`, unmodified consensus `signedHeader.header` and `signedHeader.commit` |
| `px_getValidators` | One positive decimal u64 height string | Local consensus Validators at that height, every page; `height`, complete `validators` with consensus `address`/`pubKey` and decimal `votingPower`, decimal `total` |
| `px_getReceiptKeys` | Explicit empty positional array `[]` | Latest committed governed sequencer ReceiptKeys; `keys` with 0x-prefixed 32-byte `keyId`/`publicKey` and inclusive u64 JSON-integer `firstHeight`/`lastHeight` |
| `px_getSequenceBatch` | One positive decimal u64 height string | Local consensus Block transaction zero decoded with sequencer types; decimal `height`, 0x transaction `tx`, consensus Merkle `proof` with integer `total`/`index=0`, 0x `leafHash`/`aunts` |
| `pax_sequencerReceipt` | One 0x-prefixed 32-byte transaction hash | Local sequencer send record, governed receipt pins and committed LastSeq/LastSeqHash; null for unknown hashes, otherwise the submitted/receipted/committed rung and signed execution output, with covering block coordinates on committed answers |

Height strings outside the positive u64 domain return `-32602`. Valid u64
heights beyond the consensus int64 range are unavailable. Missing, pruned,
unreachable or inconsistent commit/validator state returns `-32003`,
`Chain read unavailable`, data.code `consensus_unavailable`. Pagination checks
height, count, stable total, public-key envelopes and duplicate validators.

A batch proof authenticates the consensus transaction hash under
`header.data_hash`; it is verified before serving and is separate from the
batch activity `TxRoot`. An absent batch returns `-32602`, `Invalid params`,
data.code `no_sequence_batch`; malformed batches or inconsistent block/proof
state return `-32003`, `Chain read unavailable`, data.code
`invalid_consensus_response`. Unavailable consensus returns data.code
`consensus_unavailable` with the same `-32003` error.

Receipt keys are read locally on ordinary nodes, without a send-record source
or forwarding URL. An explicitly configured empty list returns `{"keys":[]}`.
An absent or invalid governed list returns `-32003`, `Chain read unavailable`,
data.code `receipt_keys_unavailable`. Each request reads the latest committed
governed state before validating parameters, including after a server restart;
unavailable authority takes precedence over parameter errors. When the governed
list is available, omitted, null, object or nonempty params return `-32602`,
`Invalid params`, on either transport, including mixed batches.
Receipt keys beside a receipt never establish authority by themselves.

A node without a local sequencer send record uses only an explicitly configured
sequencer forwarding path for `pax_sequencerReceipt`; otherwise it returns
`-32000`, `pax_sequencerReceipt served by sequencer`. The method never constructs
send records from committed transactions. All seven methods record the existing
per-method, error and connection metrics. The hosted-operation decisions remain
`-32004`, `Hosted gateway service operation`, data
`{code:"hosted_gateway_service_required",method:<requested method>}` for valid
`px_register`/`px_requestFunds` parameters, and `-32602` for invalid parameters.
