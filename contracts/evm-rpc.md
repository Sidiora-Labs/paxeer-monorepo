# EVM JSON-RPC contract

Paxeer mainnet-beta is one BFT network with EVM chain ID **125 (`0x7d`)**.
Its kernel is part of the network and proposes blocks; a receipt is final
once its block is committed. The node API is implemented under [rpc](../rpc/).

## HTTP endpoint

Public JSON-RPC uses HTTPS at the **root path** of
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`. For example:

```sh
curl --fail-with-body https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain ID result is `"0x7d"`. This is an integration example, not
a claim that a live request was executed. Compare block heights when choosing
among nodes; one node's available history and head can differ from another's.
`node.hyperpaxeer.com` is a library mirror, not an RPC endpoint.
The pre-merge router and its /rpc, /rpc/ws and /rpc/schema paths are retired.
There is no separate LayerX interface or separate sequencer service to configure.

[rpcstack.go](../rpc/rpcstack.go) checks the root path when the configured
prefix is empty. It also resolves `/chain/<domain-id>` through a domain
resolver and attaches the resulting domain scope before serving the request.
These sub-chains are tenant domains backed by database namespaces, not new
infrastructure. Unknown or retired domains are refused; this route does not
change the public mainnet root endpoint above.

## Served methods

The public node namespaces recorded for mainnet-beta are `debug`, `echo`,
`eth`, `net`, `pax`, `test`, and `txpool`. Availability of individual methods
depends on the binary, transport, activation and configuration. The source
[server.go](../rpc/server.go) additionally contains registrations for `web3`,
legacy `pax2`, and a compatibility receipt-key API under `px`; their presence
in source is not a guarantee that every public node exposes them.

Use the actual server's `rpc_modules` and method responses to establish its
surface. `ServedMethods`, `HTTPMethods`, and `WSMethods` in
[served_methods.go](../rpc/served_methods.go) enumerate registered callbacks;
HTTP enumeration excludes subscriptions and methods blocked by the legacy
allowlist. A Go helper or a registry row does not alone establish a callable
JSON-RPC method.

| Family | Owning implementation / behavior |
| --- | --- |
| Blocks and receipts | [block.go](../rpc/block.go), [tx.go](../rpc/tx.go); standard `eth_*` views and separate compatibility views |
| Account state | [state.go](../rpc/state.go); balance, code, storage and nonce reads at available heights |
| Calls and gas estimates | [simulate.go](../rpc/simulate.go); gas/time/concurrency limits apply |
| Raw signed submission | [send.go](../rpc/send.go), [raw_tx.go](../rpc/raw_tx.go) |
| Synchronous submission | [send_sync.go](../rpc/send_sync.go); bounded wait for inclusion |
| Logs and filters | [filter.go](../rpc/filter.go); bounded log ranges and filter expiry |
| Traces | [tracers.go](../rpc/tracers.go), [trace_profile.go](../rpc/trace_profile.go); requires available execution state |
| Address association helpers | [association.go](../rpc/association.go) |
| Client and peer metadata | [web3.go](../rpc/web3.go), [net.go](../rpc/net.go), [net_peers.go](../rpc/net_peers.go) |
| Transaction pool | [txpool.go](../rpc/txpool.go) |

The hosted gateway's `px_*` product methods are a different API contract:
[services/platform/hosted/gateway/openrpc.json](../services/platform/hosted/gateway/openrpc.json).
Do not send that method catalog to a public node merely because it supports
Ethereum JSON-RPC or a compatibility `px_getReceiptKeys` registration.

### Legacy Pax methods

`pax_*` and `pax2_*` compatibility methods are deprecated. The HTTP gate in
[pax_legacy_http.go](../rpc/pax_legacy_http.go) enforces
`[evm].enabled_legacy_pax_apis` after the deny list. The default enables only
`pax_getPaxAddress`, `pax_getEVMAddress`, and `pax_getCosmosTx`.
Allowed responses retain their shape and carry the
`Pax-Legacy-RPC-Deprecation` header. Disabled methods return JSON-RPC `-32601`
with `data: "legacy_pax_deprecated"` under HTTP 200. Batch notifications do
not generate response entries.

The `pax2` source has block views, block receipts, counts and trace-failure
variants with bank transfers included; it has no independent transaction or
filter service. Use `eth_*` and `debug_*` for new Ethereum integrations.

#<a id="sequencer-options"></a>

## Retired receipt options

`SequencerOption`, `SequencerForwardOption`, and `SequencerReceiptOption`
remain compatibility types in [server.go](../rpc/server.go),
[server_options.go](../rpc/server_options.go), and
[sequencer_receipt.go](../rpc/sequencer_receipt.go). They are not instructions
to deploy a separate sequencer service; that pre-merge topology is retired.
The compatibility `pax_sequencerReceipt` and `px_getReceiptKeys` handlers
return a typed `-32000` error without a configured source/forwarder.
Their historical receipt rungs do not replace committed-block finality.

<a id="ws-transport"></a>

## WS endpoint

The node's default local WebSocket listener is port **8546**, with root path
`/`; the default local HTTP listener is port **8545**. Public HTTPS node names
do not by themselves guarantee a public WSS listener. Obtain the deployed
WSS endpoint from its operator and configure it independently.
Subscriptions travel over WebSocket, not HTTP: see
[subscribe.go](../rpc/subscribe.go) for `newHeads` and `logs`, and the send
path for `newPendingTransactions`. A pending event is not a committed receipt.

## Height availability

[utils.go](../rpc/utils.go) and
[watermark_manager.go](../rpc/watermark_manager.go) select contexts and bound
available heights. Historical state, receipt and trace retention can differ.
Keep block tags and hash/number encodings appropriate to the requested method;
do not interpret an unavailable historical state as a zero balance or an empty
successful trace. Large log scans should use bounded ranges; the default
`max_blocks_for_log` is 2,000.

## Config

[rpc/config/config.go](../rpc/config/config.go) defines `Config`,
`DefaultConfig`, `ReadConfig`, and the generated TOML template. Keys below
belong to `[evm]` in `app.toml`; application option names add the `evm.` prefix.
They are node settings, separate from
[explorer environment variables](explorer-config.md).

| Keys | Source defaults / interpretation |
| --- | --- |
| `http_enabled`, `http_address`, `http_port` | `true`, `0.0.0.0`, 8545; address input must be a literal IP |
| `ws_enabled`, `ws_port` | `true`, 8546 |
| `cors_origins`, `ws_origins` | `"*"`, `"*"`; comma-separated origin strings |
| `batch_request_limit`, `batch_response_max_size` | 1,000 requests, 25,000,000 response bytes |
| `send_raw_tx_sync_timeout` | `2s` |
| `read_timeout`, `read_header_timeout`, `write_timeout`, `idle_timeout` | inherited from the linked Geth `rpc.DefaultHTTPTimeouts` |
| `simulation_gas_limit`, `simulation_evm_timeout` | 10,000,000 gas, `60s` |
| `max_concurrent_simulation_calls` | number of CPUs; 0 permits unlimited calls |
| `filter_timeout`, `max_tx_pool_txs` | `120s`, 1,000 |
| `max_log_no_block`, `max_blocks_for_log` | 10,000 logs for open-ended queries, 2,000 blocks |
| `max_subscriptions_new_head`, `max_subscriptions_logs` | 10,000 each |
| `deny_list`, `slow` | empty method list, `false` |
| `enable_test_api`, `enable_unsafe_keyring_rpc` | both `false`; test APIs are withheld on live chain IDs, and unsafe keyring RPC is rejected on live chains |
| `max_concurrent_trace_calls`, `max_trace_lookback_blocks`, `trace_timeout` | 10, 10,000, `30s`; trace lookback `-1` allows unlimited lookback, subject to retained state |
| `enable_parallelized_block_trace`, `rpc_stats_interval` | `false`, `10s` |
| `worker_pool_size`, `worker_queue_size` | `min(64, 2 * CPU count)`, 1,000 tasks; zero selects defaults |
| `ip_rate_limit_rps`, `ip_rate_limit_burst` | 200 requests/second per IP, burst 400; RPS zero disables the limiter |
| `enabled_legacy_pax_apis` | the three helper methods listed above |
| `trace_bake_enabled`, `trace_bake_workers`, `trace_bake_queue_size` | `false`, 1, 4,096 |
| `trace_bake_tracers`, `trace_bake_window_blocks` | `["callTracer"]`, 0; zero disables cache pruning |
| `trace_bake_use_snapshot`, `trace_bake_snapshot_window` | `false`, 64 |

Trace baking caches re-execution results under `<home>/data/trace_db`.
Snapshot use requires a compatible state backend; it does not make pruned
history available. Public transaction submission should use client-signed
`eth_sendRawTransaction`. Local-keyring `eth_sign`, `eth_signTransaction`, and
`eth_sendTransaction` require the unsafe keyring setting.

## Error codes

| Code / transport | Meaning and owner |
| --- | --- |
| `-32601` | missing method or disabled legacy Pax method; inspect the message and legacy error data |
| `-32000` | explicitly unsupported EVM operation (`ErrEVMNotSupported` in [rpc_unsupported.go](../rpc/rpc_unsupported.go)); also used by unavailable compatibility receipt handlers |
| `3` | execution revert in [simulate.go](../rpc/simulate.go), with revert bytes in error data |
| `4` | synchronous submission not yet included; error data is the transaction hash in [send_sync.go](../rpc/send_sync.go) |
| HTTP 429, JSON-RPC `-32005` | per-IP limit exceeded in [rpcstack.go](../rpc/rpcstack.go) |

Paxeer has no PoW, uncle blocks or EIP-4844 blob transactions. For example,
`eth_blobBaseFee` returns `-32000`, `blobs not supported on this chain`.
Registration of an unsupported method provides a typed refusal, not a claim
of feature support. Precompile execution reasons are a separate
[`LXP_*` registry](address-inventory.md#error-registry).

## Metrics

[metrics.go](../rpc/metrics.go) and
[worker_pool_metrics.go](../rpc/worker_pool_metrics.go) own RPC and worker-pool
metrics. `rpc_stats_interval` controls request-stat reporting. Pool capacity
and queue occupancy describe local query capacity; they are not consensus
finality signals. Consumers should preserve typed failures and configure
timeouts rather than replacing unavailable data with successful empty values.

Bridge consumers use [deployment records](bridge-deploy.md) and
[attestation encodings](bridge-vectors.md); browser consumers use
[explorer frontend settings](explorer-frontend-config.md).

## Registry source index

These link targets preserve interface-registry references. Each row points
to the current source owner for its declaration, ABI, message, configuration
family or helper. It does not assert that a method is registered, a feature
is enabled, or a contract is deployed. Rows marked retired identify source
references absent from this tree and must not be used as deployment targets.

| Link targets | Source owner |
| --- | --- |
| <a id="client-parity"></a>`client-parity` | [rpc/net.go](../rpc/net.go); [rpc/net_peers.go](../rpc/net_peers.go); [rpc/raw_tx.go](../rpc/raw_tx.go); [rpc/raw_tx_index.go](../rpc/raw_tx_index.go); [rpc/send_sync.go](../rpc/send_sync.go); [rpc/txpool.go](../rpc/txpool.go); [rpc/uncle.go](../rpc/uncle.go); [rpc/uncle_count.go](../rpc/uncle_count.go); [rpc/web3.go](../rpc/web3.go) |
| <a id="config-checktx-timeout"></a>`config-checktx-timeout`, <a id="config-cors-origins"></a>`config-cors-origins`, <a id="config-deny-list"></a>`config-deny-list`, <a id="config-enable-parallelized-block-trace"></a>`config-enable-parallelized-block-trace`, <a id="config-enable-test-api"></a>`config-enable-test-api`, <a id="config-enable-unsafe-keyring-rpc"></a>`config-enable-unsafe-keyring-rpc`, <a id="config-enabled-legacy-pax-apis"></a>`config-enabled-legacy-pax-apis`, <a id="config-filter-timeout"></a>`config-filter-timeout`, <a id="config-http-address"></a>`config-http-address`, <a id="config-http-enabled"></a>`config-http-enabled`, <a id="config-http-port"></a>`config-http-port`, <a id="config-idle-timeout"></a>`config-idle-timeout`, <a id="config-ip-rate-limit-burst"></a>`config-ip-rate-limit-burst`, <a id="config-ip-rate-limit-rps"></a>`config-ip-rate-limit-rps`, <a id="config-max-blocks-for-log"></a>`config-max-blocks-for-log`, <a id="config-max-concurrent-simulation-calls"></a>`config-max-concurrent-simulation-calls`, <a id="config-max-concurrent-trace-calls"></a>`config-max-concurrent-trace-calls`, <a id="config-max-log-no-block"></a>`config-max-log-no-block`, <a id="config-max-subscriptions-new-head"></a>`config-max-subscriptions-new-head`, <a id="config-max-trace-lookback-blocks"></a>`config-max-trace-lookback-blocks`, <a id="config-max-tx-pool-txs"></a>`config-max-tx-pool-txs`, <a id="config-read-header-timeout"></a>`config-read-header-timeout`, <a id="config-read-timeout"></a>`config-read-timeout`, <a id="config-rpc-stats-interval"></a>`config-rpc-stats-interval`, <a id="config-simulation-evm-timeout"></a>`config-simulation-evm-timeout`, <a id="config-simulation-gas-limit"></a>`config-simulation-gas-limit`, <a id="config-slow"></a>`config-slow`, <a id="config-trace-bake-enabled"></a>`config-trace-bake-enabled`, <a id="config-trace-bake-queue-size"></a>`config-trace-bake-queue-size`, <a id="config-trace-bake-snapshot-window"></a>`config-trace-bake-snapshot-window`, <a id="config-trace-bake-tracers"></a>`config-trace-bake-tracers`, <a id="config-trace-bake-use-snapshot"></a>`config-trace-bake-use-snapshot`, <a id="config-trace-bake-window-blocks"></a>`config-trace-bake-window-blocks`, <a id="config-trace-bake-workers"></a>`config-trace-bake-workers`, <a id="config-trace-timeout"></a>`config-trace-timeout`, <a id="config-worker-pool-size"></a>`config-worker-pool-size`, <a id="config-worker-queue-size"></a>`config-worker-queue-size`, <a id="config-write-timeout"></a>`config-write-timeout`, <a id="config-ws-enabled"></a>`config-ws-enabled`, <a id="config-ws-origins"></a>`config-ws-origins`, <a id="config-ws-port"></a>`config-ws-port` | [rpc/config/config.go](../rpc/config/config.go) |
| <a id="debug-getrawblock"></a>`debug-getrawblock`, <a id="debug-getrawheader"></a>`debug-getrawheader`, <a id="debug-getrawreceipts"></a>`debug-getrawreceipts`, <a id="debug-getrawtransaction"></a>`debug-getrawtransaction`, <a id="debug-traceblockbyhash"></a>`debug-traceblockbyhash`, <a id="debug-traceblockbynumber"></a>`debug-traceblockbynumber`, <a id="debug-tracecall"></a>`debug-tracecall`, <a id="debug-tracestateaccess"></a>`debug-tracestateaccess`, <a id="debug-tracetransaction"></a>`debug-tracetransaction`, <a id="pax-getrawblock"></a>`pax-getrawblock`, <a id="pax-getrawheader"></a>`pax-getrawheader`, <a id="pax-getrawreceipts"></a>`pax-getrawreceipts`, <a id="pax-getrawtransaction"></a>`pax-getrawtransaction`, <a id="pax-traceblockbyhash"></a>`pax-traceblockbyhash`, <a id="pax-traceblockbyhashexcludetracefail"></a>`pax-traceblockbyhashexcludetracefail`, <a id="pax-traceblockbynumber"></a>`pax-traceblockbynumber`, <a id="pax-traceblockbynumberexcludetracefail"></a>`pax-traceblockbynumberexcludetracefail`, <a id="pax-tracecall"></a>`pax-tracecall`, <a id="pax-tracestateaccess"></a>`pax-tracestateaccess`, <a id="pax-tracetransaction"></a>`pax-tracetransaction` | [rpc/tracers.go](../rpc/tracers.go) |
| <a id="debug-tracetransactionprofile"></a>`debug-tracetransactionprofile`, <a id="pax-tracetransactionprofile"></a>`pax-tracetransactionprofile` | [rpc/trace_profile.go](../rpc/trace_profile.go) |
| <a id="echo-echo"></a>`echo-echo` | [rpc/echo.go](../rpc/echo.go) |
| <a id="eth-accounts"></a>`eth-accounts`, <a id="eth-blobbasefee"></a>`eth-blobbasefee`, <a id="eth-blocknumber"></a>`eth-blocknumber`, <a id="eth-calculategasusedratio"></a>`eth-calculategasusedratio`, <a id="eth-chainid"></a>`eth-chainid`, <a id="eth-coinbase"></a>`eth-coinbase`, <a id="eth-feehistory"></a>`eth-feehistory`, <a id="eth-gasprice"></a>`eth-gasprice`, <a id="eth-gaspricehelper"></a>`eth-gaspricehelper`, <a id="eth-maxpriorityfeepergas"></a>`eth-maxpriorityfeepergas`, <a id="eth-syncing"></a>`eth-syncing` | [rpc/info.go](../rpc/info.go) |
| <a id="eth-call"></a>`eth-call`, <a id="eth-createaccesslist"></a>`eth-createaccesslist`, <a id="eth-estimategas"></a>`eth-estimategas`, <a id="eth-estimategasaftercalls"></a>`eth-estimategasaftercalls` | [rpc/simulate.go](../rpc/simulate.go) |
| <a id="eth-cleanup"></a>`eth-cleanup`, <a id="eth-getfilterchanges"></a>`eth-getfilterchanges`, <a id="eth-getfilterlogs"></a>`eth-getfilterlogs`, <a id="eth-getlogs"></a>`eth-getlogs`, <a id="eth-newblockfilter"></a>`eth-newblockfilter`, <a id="eth-newfilter"></a>`eth-newfilter`, <a id="eth-newpendingtransactionfilter"></a>`eth-newpendingtransactionfilter`, <a id="eth-setreceiptlogfeed"></a>`eth-setreceiptlogfeed`, <a id="eth-uninstallfilter"></a>`eth-uninstallfilter`, <a id="pax-cleanup"></a>`pax-cleanup`, <a id="pax-getfilterchanges"></a>`pax-getfilterchanges`, <a id="pax-getfilterlogs"></a>`pax-getfilterlogs`, <a id="pax-getlogs"></a>`pax-getlogs`, <a id="pax-newblockfilter"></a>`pax-newblockfilter`, <a id="pax-newfilter"></a>`pax-newfilter`, <a id="pax-newpendingtransactionfilter"></a>`pax-newpendingtransactionfilter`, <a id="pax-setreceiptlogfeed"></a>`pax-setreceiptlogfeed`, <a id="pax-uninstallfilter"></a>`pax-uninstallfilter` | [rpc/filter.go](../rpc/filter.go) |
| <a id="eth-getbalance"></a>`eth-getbalance`, <a id="eth-getcode"></a>`eth-getcode`, <a id="eth-getnonce"></a>`eth-getnonce`, <a id="eth-getproof"></a>`eth-getproof`, <a id="eth-getstorageat"></a>`eth-getstorageat` | [rpc/state.go](../rpc/state.go) |
| <a id="eth-getblockbyhash"></a>`eth-getblockbyhash`, <a id="eth-getblockbynumber"></a>`eth-getblockbynumber`, <a id="eth-getblockreceipts"></a>`eth-getblockreceipts`, <a id="eth-getblocktransactioncountbyhash"></a>`eth-getblocktransactioncountbyhash`, <a id="eth-getblocktransactioncountbynumber"></a>`eth-getblocktransactioncountbynumber`, <a id="pax-getblockbyhash"></a>`pax-getblockbyhash`, <a id="pax-getblockbyhashexcludetracefail"></a>`pax-getblockbyhashexcludetracefail`, <a id="pax-getblockbynumber"></a>`pax-getblockbynumber`, <a id="pax-getblockbynumberexcludetracefail"></a>`pax-getblockbynumberexcludetracefail`, <a id="pax-getblockreceipts"></a>`pax-getblockreceipts`, <a id="pax-getblocktransactioncountbyhash"></a>`pax-getblocktransactioncountbyhash`, <a id="pax-getblocktransactioncountbynumber"></a>`pax-getblocktransactioncountbynumber`, <a id="pax2-getblockbyhash"></a>`pax2-getblockbyhash`, <a id="pax2-getblockbyhashexcludetracefail"></a>`pax2-getblockbyhashexcludetracefail`, <a id="pax2-getblockbynumber"></a>`pax2-getblockbynumber`, <a id="pax2-getblockbynumberexcludetracefail"></a>`pax2-getblockbynumberexcludetracefail`, <a id="pax2-getblockreceipts"></a>`pax2-getblockreceipts`, <a id="pax2-getblocktransactioncountbyhash"></a>`pax2-getblocktransactioncountbyhash`, <a id="pax2-getblocktransactioncountbynumber"></a>`pax2-getblocktransactioncountbynumber` | [rpc/block.go](../rpc/block.go) |
| <a id="eth-gettransactionbyblockhashandindex"></a>`eth-gettransactionbyblockhashandindex`, <a id="eth-gettransactionbyblocknumberandindex"></a>`eth-gettransactionbyblocknumberandindex`, <a id="eth-gettransactionbyhash"></a>`eth-gettransactionbyhash`, <a id="eth-gettransactioncount"></a>`eth-gettransactioncount`, <a id="eth-gettransactionerrorbyhash"></a>`eth-gettransactionerrorbyhash`, <a id="eth-gettransactionreceipt"></a>`eth-gettransactionreceipt`, <a id="eth-getvmerror"></a>`eth-getvmerror`, <a id="eth-sign"></a>`eth-sign`, <a id="pax-gettransactionbyblockhashandindex"></a>`pax-gettransactionbyblockhashandindex`, <a id="pax-gettransactionbyblocknumberandindex"></a>`pax-gettransactionbyblocknumberandindex`, <a id="pax-gettransactionbyhash"></a>`pax-gettransactionbyhash`, <a id="pax-gettransactioncount"></a>`pax-gettransactioncount`, <a id="pax-gettransactionerrorbyhash"></a>`pax-gettransactionerrorbyhash`, <a id="pax-gettransactionreceipt"></a>`pax-gettransactionreceipt`, <a id="pax-gettransactionreceiptexcludetracefail"></a>`pax-gettransactionreceiptexcludetracefail`, <a id="pax-getvmerror"></a>`pax-getvmerror`, <a id="pax-sign"></a>`pax-sign` | [rpc/tx.go](../rpc/tx.go) |
| <a id="eth-sendrawtransaction"></a>`eth-sendrawtransaction`, <a id="eth-sendtransaction"></a>`eth-sendtransaction`, <a id="eth-signtransaction"></a>`eth-signtransaction`, <a id="ws-new-pending-transactions"></a>`ws-new-pending-transactions` | [rpc/send.go](../rpc/send.go) |
| <a id="eth-subscribe"></a>`eth-subscribe`, <a id="eth-unsubscribe"></a>`eth-unsubscribe` | [core/deps/geth/rpc/json.go](../core/deps/geth/rpc/json.go) |
| <a id="metrics-legacy"></a>`metrics-legacy` | [rpc/worker_pool_metrics.go](../rpc/worker_pool_metrics.go); [utils/metrics/metrics_util.go](../utils/metrics/metrics_util.go) |
| <a id="metrics-workerpool"></a>`metrics-workerpool` | [rpc/worker_pool_metrics.go](../rpc/worker_pool_metrics.go) |
| <a id="net-version"></a>`net-version` | [rpc/net.go](../rpc/net.go) |
| <a id="pax-associate"></a>`pax-associate`, <a id="pax-getcosmostx"></a>`pax-getcosmostx`, <a id="pax-getevmaddress"></a>`pax-getevmaddress`, <a id="pax-getevmtx"></a>`pax-getevmtx`, <a id="pax-getpaxaddress"></a>`pax-getpaxaddress` | [rpc/association.go](../rpc/association.go) |
| <a id="pax-sequencerreceipt"></a>`pax-sequencerreceipt` | [rpc/sequencer_receipt.go](../rpc/sequencer_receipt.go) |
| <a id="rpc-modules"></a>`rpc-modules` | [core/deps/geth/rpc/server.go](../core/deps/geth/rpc/server.go) |
| <a id="test-incrementpointerversion"></a>`test-incrementpointerversion` | [rpc/test.go](../rpc/test.go) |
| <a id="txpool-content"></a>`txpool-content` | [rpc/txpool.go](../rpc/txpool.go) |
| <a id="web3-clientversion"></a>`web3-clientversion` | [rpc/web3.go](../rpc/web3.go) |
| <a id="ws-logs"></a>`ws-logs`, <a id="ws-new-heads"></a>`ws-new-heads` | [rpc/subscribe.go](../rpc/subscribe.go) |
