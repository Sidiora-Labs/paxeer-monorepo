# Telemetry contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cosmos-telemetry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.met-tel-key-begin-blocker</code> | <code>metric</code> | <code>stable</code> | <code>sdk/telemetry/wrapper.go:11</code> | <code>MetricKeyBeginBlocker = "begin_blocker"</code> |
| <code>interface.15.met-tel-key-end-blocker</code> | <code>metric</code> | <code>stable</code> | <code>sdk/telemetry/wrapper.go:13</code> | <code>MetricKeyEndBlocker = "end_blocker"</code> |
| <code>interface.15.met-tel-key-message</code> | <code>metric</code> | <code>stable</code> | <code>sdk/telemetry/wrapper.go:15</code> | <code>MessageCount = "message"</code> |
| <code>interface.15.met-tel-key-mid-blocker</code> | <code>metric</code> | <code>stable</code> | <code>sdk/telemetry/wrapper.go:12</code> | <code>MetricKeyMidBlocker = "mid_blocker"</code> |
| <code>interface.15.met-tel-key-module</code> | <code>metric</code> | <code>stable</code> | <code>sdk/telemetry/wrapper.go:14</code> | <code>MetricLabelNameModule = "module"</code> |
| <code>interface.15.met-tel-key-transaction</code> | <code>metric</code> | <code>stable</code> | <code>sdk/telemetry/wrapper.go:16</code> | <code>TxCount = "transaction"</code> |
| <code>interface.15.met-tel-pax-cosmos-validator-slashed</code> | <code>metric</code> | <code>retire</code> | <code>sdk/telemetry/metrics.go:128</code> | <code>CounterDefinition{Name: ["pax_cosmos_validator_slashed"], Help: "Total number of validator was slashed"}; IncrValidatorSlashedCounter keys ["pax","cosmos","validator","slashed"] labels [type, validator]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.met-tel-key-begin-blocker</code> | notes | go-metrics metric key component used by ModuleMeasureSince and the module timing calls |
| <code>interface.15.met-tel-key-end-blocker</code> | notes | go-metrics metric key component used by ModuleMeasureSince and the module timing calls |
| <code>interface.15.met-tel-key-message</code> | notes | go-metrics metric key component used by ModuleMeasureSince and the module timing calls |
| <code>interface.15.met-tel-key-mid-blocker</code> | notes | go-metrics metric key component used by ModuleMeasureSince and the module timing calls |
| <code>interface.15.met-tel-key-module</code> | notes | go-metrics label name used by ModuleMeasureSince and the module timing calls |
| <code>interface.15.met-tel-key-transaction</code> | notes | go-metrics metric key component used by ModuleMeasureSince and the module timing calls |
| <code>interface.15.met-tel-pax-cosmos-validator-slashed</code> | notes | legacy go-metrics counter; each caller carries TODO(PLT-414) to remove it once the OpenTelemetry slashing validator_slashed counter of sdk/x/slashing/keeper/metrics.go:15 is verified; recorded by sdk/telemetry/wrapper.go:84 |

## module-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.met-otel-epoch-begin-blocker-duration</code> | <code>metric</code> | <code>stable</code> | <code>modules/epoch/keeper/metrics.go:21</code> | <code>Float64Histogram("epoch_begin_blocker_duration", unit "s")</code> |
| <code>interface.3.met-otel-epoch-new</code> | <code>metric</code> | <code>stable</code> | <code>modules/epoch/keeper/metrics.go:27</code> | <code>Int64Gauge("epoch_new", unit "{epoch}")</code> |
| <code>interface.3.met-otel-mint-coins-minted</code> | <code>metric</code> | <code>stable</code> | <code>modules/mint/types/metrics.go:15</code> | <code>Int64Gauge("mint_coins_minted", unit "{uhpx}") attrs [denom]</code> |
| <code>interface.3.met-otel-oracle-end-blocker-duration</code> | <code>metric</code> | <code>stable</code> | <code>modules/oracle/metrics.go:28</code> | <code>Float64Histogram("oracle_end_blocker_duration", unit "s")</code> |
| <code>interface.3.met-otel-oracle-mid-blocker-duration</code> | <code>metric</code> | <code>stable</code> | <code>modules/oracle/metrics.go:22</code> | <code>Float64Histogram("oracle_mid_blocker_duration", unit "s")</code> |
| <code>interface.3.met-otel-oracle-price-update</code> | <code>metric</code> | <code>stable</code> | <code>modules/oracle/metrics.go:34</code> | <code>Int64Counter("oracle_price_update", unit "{count}") attrs [denom]</code> |
| <code>interface.3.met-otel-oracle-validator-slashed</code> | <code>metric</code> | <code>stable</code> | <code>modules/oracle/keeper/metrics.go:26</code> | <code>Int64Counter("oracle_validator_slashed", unit "{count}") attrs [validator, type]</code> |
| <code>interface.3.met-otel-oracle-vote-penalty-count</code> | <code>metric</code> | <code>stable</code> | <code>modules/oracle/keeper/metrics.go:21</code> | <code>Int64Gauge("oracle_vote_penalty_count", unit "{count}")</code> |
| <code>interface.6.met-otel-evm-abci-begin-blocker-duration</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:40</code> | <code>Float64Histogram("evm_abci_begin_blocker_duration", unit "s")</code> |
| <code>interface.6.met-otel-evm-abci-end-blocker-duration</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:47</code> | <code>Float64Histogram("evm_abci_end_blocker_duration", unit "s")</code> |
| <code>interface.6.met-otel-evm-ante-association-error</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/ante/metrics.go:49</code> | <code>Int64Counter("evm_ante_association_error", unit "{count}") attrs [scenario, type]</code> |
| <code>interface.6.met-otel-evm-block-base-fee</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:54</code> | <code>Float64Gauge("evm_block_base_fee")</code> |
| <code>interface.6.met-otel-evm-effective-gas-price</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/ante/metrics.go:42</code> | <code>Float64Histogram("evm_effective_gas_price", unit "{wei}/{gas}")</code> |
| <code>interface.6.met-otel-evm-errors</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:65</code> | <code>Int64Counter("evm_errors", unit "{count}") attrs [type, status]</code> |
| <code>interface.6.met-otel-evm-keeper-association-error</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:77</code> | <code>Int64Counter("evm_keeper_association_error", unit "{count}") attrs [scenario, type]</code> |
| <code>interface.6.met-otel-evm-nonce-mismatch</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/ante/metrics.go:36</code> | <code>Int64Counter("evm_nonce_mismatch", unit "{count}") attrs [too_high]</code> |
| <code>interface.6.met-otel-evm-panics</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:59</code> | <code>Int64Counter("evm_panics", unit "{count}")</code> |
| <code>interface.6.met-otel-evm-pending-nonce</code> | <code>metric</code> | <code>change</code> | <code>modules/evm/ante/metrics.go:30</code> | <code>Int64Counter("evm_pending_nonce", unit "{count}")</code> |
| <code>interface.6.met-otel-evm-receipt-status</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:71</code> | <code>Int64Counter("evm_receipt_status", unit "{count}") attrs [status]</code> |
| <code>interface.6.met-otel-evm-zero-storage-processed-keys</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:83</code> | <code>Int64Counter("evm_zero_storage_processed_keys", unit "{count}")</code> |
| <code>interface.6.met-otel-evm-zero-storage-pruned-bytes</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:95</code> | <code>Int64Counter("evm_zero_storage_pruned_bytes", unit "By")</code> |
| <code>interface.6.met-otel-evm-zero-storage-pruned-keys</code> | <code>metric</code> | <code>stable</code> | <code>modules/evm/keeper/metrics.go:89</code> | <code>Int64Counter("evm_zero_storage_pruned_keys", unit "{count}")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.met-otel-epoch-begin-blocker-duration</code> | notes | meter "epoch_keeper"; description: Duration of epoch begin-blocker execution in seconds |
| <code>interface.3.met-otel-epoch-new</code> | notes | meter "epoch_keeper"; description: Current epoch number |
| <code>interface.3.met-otel-mint-coins-minted</code> | notes | meter "mint"; description: Amount of coins minted by denom |
| <code>interface.3.met-otel-oracle-end-blocker-duration</code> | notes | meter "oracle"; description: Duration of oracle end-blocker execution in seconds |
| <code>interface.3.met-otel-oracle-mid-blocker-duration</code> | notes | meter "oracle"; description: Duration of oracle mid-blocker execution in seconds |
| <code>interface.3.met-otel-oracle-price-update</code> | notes | meter "oracle"; description: Number of oracle price updates by denom |
| <code>interface.3.met-otel-oracle-validator-slashed</code> | notes | meter "oracle_keeper"; description: Number of validators slashed by oracle |
| <code>interface.3.met-otel-oracle-vote-penalty-count</code> | notes | meter "oracle_keeper"; description: Oracle vote penalty counts by validator and type (miss, abstain, success) |
| <code>interface.6.met-otel-evm-abci-begin-blocker-duration</code> | notes | meter "evm_keeper"; description: Duration of EVM module BeginBlock |
| <code>interface.6.met-otel-evm-abci-end-blocker-duration</code> | notes | meter "evm_keeper"; description: Duration of EVM module EndBlock |
| <code>interface.6.met-otel-evm-ante-association-error</code> | notes | meter "evm_ante"; description: EVM address association errors by scenario and address type |
| <code>interface.6.met-otel-evm-block-base-fee</code> | notes | meter "evm_keeper"; description: Current EVM block base fee per gas |
| <code>interface.6.met-otel-evm-effective-gas-price</code> | notes | meter "evm_ante"; description: Effective gas price (wei per gas) for EVM transactions |
| <code>interface.6.met-otel-evm-errors</code> | notes | meter "evm_keeper"; description: EVM processing errors by type (state_transition, stateDB_finalize, write_receipt, apply_message, vm_execution) |
| <code>interface.6.met-otel-evm-keeper-association-error</code> | notes | meter "evm_keeper"; description: EVM address association errors by scenario and address type |
| <code>interface.6.met-otel-evm-nonce-mismatch</code> | notes | meter "evm_ante"; description: EVM nonce mismatches by cause (too_high, too_low) |
| <code>interface.6.met-otel-evm-panics</code> | notes | meter "evm_keeper"; description: Number of panics recovered during EVM transaction processing |
| <code>interface.6.met-otel-evm-pending-nonce</code> | notes | meter "evm_ante"; description: EVM pending nonce events by type (added, expired, rejected, accepted); instrument is created but never recorded |
| <code>interface.6.met-otel-evm-receipt-status</code> | notes | meter "evm_keeper"; description: EVM transaction receipt outcomes by status (success, failed) |
| <code>interface.6.met-otel-evm-zero-storage-processed-keys</code> | notes | meter "evm_keeper"; description: Storage slots scanned during zero-value cleanup |
| <code>interface.6.met-otel-evm-zero-storage-pruned-bytes</code> | notes | meter "evm_keeper"; description: Bytes reclaimed by zero-value storage slot cleanup |
| <code>interface.6.met-otel-evm-zero-storage-pruned-keys</code> | notes | meter "evm_keeper"; description: Zero-value storage slots deleted during cleanup |

Contract context <code>contract_context.6.evm-ante-metrics</code>:

| Field | Frozen rule |
|---|---|
| interface | modules/evm/ante/metrics.go and modules/evm/ante/sig.go |
| storage | none |
| errors | none |
| events | none |
| gas | none |
| activation | none |
| invariants | evm_pending_nonce counts CheckTx nonces with type added or rejected; evm_nonce_mismatch unchanged |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.30.1</code> | req | 6030.1 |
| <code>acceptance.6.30.1</code> | scenario | Given a ManualReader on the global MeterProvider and an account at nonce 5, when CheckTx runs the sig decorator with nonce 7 and then nonce 3, then the counter reads added=1 and rejected=1 |
| <code>acceptance.6.30.1</code> | test | modules/evm/ante/sig_metrics_test.go::TestPendingNonceMetricRecorded (planned) |
| <code>acceptance.6.30.1</code> | qualification | UNRUN |
| <code>req.6030</code> | ac_1 | In CheckTx a future nonce adds evm_pending_nonce{type="added"} and a past nonce adds evm_pending_nonce{type="rejected"} (modules/evm/ante/sig.go:70-74, modules/evm/ante/metrics.go:29). |
| <code>capability.6.8.2</code> | title | Pending-nonce metric |
| <code>capability.6.8.2</code> | reqs | 6030.1 |
| <code>resolution.17.712</code> | title | decision.609 binds: evm_pending_nonce counts added and rejected CheckTx nonces |
| <code>resolution.17.712</code> | applies_to | decision.609<br>task.6.8.2 |
| <code>resolution.17.712</code> | source | decision.609 in fragments/J34/decision.kvx; tasks from affects |

## node-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.met-otel-app-abci-begin-block-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:79</code> | <code>Float64Histogram("app_abci_begin_block_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-abci-check-tx-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:100</code> | <code>Float64Histogram("app_abci_check_tx_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-abci-commit-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:121</code> | <code>Float64Histogram("app_abci_commit_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-abci-deliver-batch-tx-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:114</code> | <code>Float64Histogram("app_abci_deliver_batch_tx_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-abci-deliver-tx-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:107</code> | <code>Float64Histogram("app_abci_deliver_tx_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-abci-end-block-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:86</code> | <code>Float64Histogram("app_abci_end_block_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-abci-module-end-block-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:93</code> | <code>Float64Histogram("app_abci_module_end_block_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-block-gas-wanted</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:171</code> | <code>Int64Histogram("app_block_gas_wanted", unit "{gas}")</code> |
| <code>interface.5.met-otel-app-block-gas-wanted-ratio</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:178</code> | <code>Float64Histogram("app_block_gas_wanted_ratio", unit "1")</code> |
| <code>interface.5.met-otel-app-block-process-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:128</code> | <code>Float64Histogram("app_block_process_duration", unit "s") attrs [type]</code> |
| <code>interface.5.met-otel-app-build-info</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:206</code> | <code>Int64ObservableGauge("app_build_info") attrs [paxd_version, commit]</code> |
| <code>interface.5.met-otel-app-failed-total-gas-wanted-check</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:159</code> | <code>Int64Counter("app_failed_total_gas_wanted_check", unit "{count}") attrs [proposer]</code> |
| <code>interface.5.met-otel-app-giga-fallback-to-v2</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:165</code> | <code>Int64Counter("app_giga_fallback_to_v2", unit "{count}")</code> |
| <code>interface.5.met-otel-app-lightinvariance-supply-duration</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:185</code> | <code>Float64Histogram("app_lightinvariance_supply_duration", unit "s")</code> |
| <code>interface.5.met-otel-app-lightinvariance-supply-invalid-key</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:192</code> | <code>Int64Counter("app_lightinvariance_supply_invalid_key", unit "{count}") attrs [type, step]</code> |
| <code>interface.5.met-otel-app-lightinvariance-supply-unmarshal-failure</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:198</code> | <code>Int64Counter("app_lightinvariance_supply_unmarshal_failure", unit "{count}") attrs [type, step]</code> |
| <code>interface.5.met-otel-app-optimistic-processing</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:153</code> | <code>Int64Counter("app_optimistic_processing", unit "{count}") attrs [enabled]</code> |
| <code>interface.5.met-otel-app-tx-count</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:135</code> | <code>Int64Counter("app_tx_count", unit "{count}") attrs [result]</code> |
| <code>interface.5.met-otel-app-tx-gas</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:147</code> | <code>Int64Counter("app_tx_gas", unit "{count}") attrs [type]</code> |
| <code>interface.5.met-otel-app-tx-process-type</code> | <code>metric</code> | <code>stable</code> | <code>node/metrics.go:141</code> | <code>Int64Counter("app_tx_process_type", unit "{count}") attrs [type]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.met-otel-app-abci-begin-block-duration</code> | notes | meter "app"; description: Duration of ABCI BeginBlock |
| <code>interface.5.met-otel-app-abci-check-tx-duration</code> | notes | meter "app"; description: Duration of ABCI CheckTx |
| <code>interface.5.met-otel-app-abci-commit-duration</code> | notes | meter "app"; description: Duration of ABCI Commit (state write to disk) |
| <code>interface.5.met-otel-app-abci-deliver-batch-tx-duration</code> | notes | meter "app"; description: Duration of ABCI DeliverTxBatch |
| <code>interface.5.met-otel-app-abci-deliver-tx-duration</code> | notes | meter "app"; description: Duration of ABCI DeliverTx |
| <code>interface.5.met-otel-app-abci-end-block-duration</code> | notes | meter "app"; description: Duration of ABCI EndBlock |
| <code>interface.5.met-otel-app-abci-module-end-block-duration</code> | notes | meter "app"; description: Duration of module EndBlock calls within ABCI EndBlock |
| <code>interface.5.met-otel-app-block-gas-wanted</code> | notes | meter "app"; description: Per-block total gas wanted across all transactions |
| <code>interface.5.met-otel-app-block-gas-wanted-ratio</code> | notes | meter "app"; description: Per-block ratio of total gas wanted to MaxGasWanted consensus parameter |
| <code>interface.5.met-otel-app-block-process-duration</code> | notes | meter "app"; description: Duration of block tx processing by execution type |
| <code>interface.5.met-otel-app-build-info</code> | notes | meter "app"; description: Running binary build info; value is always 1; observable gauge, reported on every scrape; the legacy go-metrics version gauge it replaces is still called from node/abci.go (TODO PLT-327) |
| <code>interface.5.met-otel-app-failed-total-gas-wanted-check</code> | notes | meter "app"; description: Proposals rejected because total block gas wanted exceeded max |
| <code>interface.5.met-otel-app-giga-fallback-to-v2</code> | notes | meter "app"; description: Times giga executor fell back to V2 processing |
| <code>interface.5.met-otel-app-lightinvariance-supply-duration</code> | notes | meter "app"; description: Duration of light invariance total supply check |
| <code>interface.5.met-otel-app-lightinvariance-supply-invalid-key</code> | notes | meter "app"; description: Invalid changed-pair keys detected during invariance check |
| <code>interface.5.met-otel-app-lightinvariance-supply-unmarshal-failure</code> | notes | meter "app"; description: Unmarshal failures during invariance supply check |
| <code>interface.5.met-otel-app-optimistic-processing</code> | notes | meter "app"; description: Optimistic processing attempts; enabled:true means cache hit, false means miss |
| <code>interface.5.met-otel-app-tx-count</code> | notes | meter "app"; description: Number of transactions delivered |
| <code>interface.5.met-otel-app-tx-gas</code> | notes | meter "app"; description: Cumulative transaction gas by type (gas_used, gas_wanted) |
| <code>interface.5.met-otel-app-tx-process-type</code> | notes | meter "app"; description: Transactions processed by execution type |

## rpc-metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.met-otel-evmrpc-historical-debug-trace-attempts-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/metrics.go:68</code> | <code>Int64Counter("evmrpc_historical_debug_trace_attempts_total", unit "{count}") attrs [endpoint, connection]</code> |
| <code>interface.7.met-otel-evmrpc-redirected-requests-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/metrics.go:63</code> | <code>Int64Counter("evmrpc_redirected_requests_total", unit "{count}") attrs [endpoint, connection]</code> |
| <code>interface.7.met-otel-evmrpc-request-latency-seconds</code> | <code>metric</code> | <code>stable</code> | <code>rpc/metrics.go:49</code> | <code>Float64Histogram("evmrpc_request_latency_seconds", unit "s") attrs [endpoint, connection, success, error_class, jsonrpc_code]</code> |
| <code>interface.7.met-otel-evmrpc-websocket-connects-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/metrics.go:58</code> | <code>Int64Counter("evmrpc_websocket_connects_total", unit "{count}")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.met-otel-evmrpc-historical-debug-trace-attempts-total</code> | notes | meter "evmrpc"; description: Number of debug_trace* requests targeting historical blocks |
| <code>interface.7.met-otel-evmrpc-redirected-requests-total</code> | notes | meter "evmrpc"; description: Number of EVM RPC requests forwarded to another validator |
| <code>interface.7.met-otel-evmrpc-request-latency-seconds</code> | notes | meter "evmrpc"; description: RPC request latency in seconds (labeled by success, error_class, and jsonrpc_code bucket) |
| <code>interface.7.met-otel-evmrpc-websocket-connects-total</code> | notes | meter "evmrpc"; description: Number of new websocket connections |
