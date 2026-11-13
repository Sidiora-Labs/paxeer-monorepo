# Telemetry contract

Node, EVM, RPC and SDK telemetry declarations are listed below. Metric names and registration belong to their defining packages. Metrics describe execution and service operation; transaction finality follows block commitment.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## cosmos-telemetry

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `MetricKeyBeginBlocker = "begin_blocker"` | [sdk/telemetry/wrapper.go](../../../sdk/telemetry/wrapper.go) |
| `metric` | `MetricKeyEndBlocker = "end_blocker"` | [sdk/telemetry/wrapper.go](../../../sdk/telemetry/wrapper.go) |
| `metric` | `MessageCount = "message"` | [sdk/telemetry/wrapper.go](../../../sdk/telemetry/wrapper.go) |
| `metric` | `MetricKeyMidBlocker = "mid_blocker"` | [sdk/telemetry/wrapper.go](../../../sdk/telemetry/wrapper.go) |
| `metric` | `MetricLabelNameModule = "module"` | [sdk/telemetry/wrapper.go](../../../sdk/telemetry/wrapper.go) |
| `metric` | `TxCount = "transaction"` | [sdk/telemetry/wrapper.go](../../../sdk/telemetry/wrapper.go) |
| source | — | [sdk/telemetry/metrics.go](../../../sdk/telemetry/metrics.go) |

## module-metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/keeper/metrics.go](../../../modules/epoch/keeper/metrics.go) |
| source | — | [modules/mint/types/metrics.go](../../../modules/mint/types/metrics.go) |
| source | — | [modules/oracle/metrics.go](../../../modules/oracle/metrics.go) |
| source | — | [modules/oracle/keeper/metrics.go](../../../modules/oracle/keeper/metrics.go) |
| source | — | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| source | — | [modules/evm/ante/metrics.go](../../../modules/evm/ante/metrics.go) |

## node-metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [node/metrics.go](../../../node/metrics.go) |

## rpc-metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [rpc/metrics.go](../../../rpc/metrics.go) |

## Instrument names

| Name | Instrument | Source |
|---|---|---|
| `epoch_begin_blocker_duration` | `Float64Histogram` | [modules/epoch/keeper/metrics.go](../../../modules/epoch/keeper/metrics.go) |
| `epoch_new` | `Int64Gauge` | [modules/epoch/keeper/metrics.go](../../../modules/epoch/keeper/metrics.go) |
| `mint_coins_minted` | `Int64Gauge` | [modules/mint/types/metrics.go](../../../modules/mint/types/metrics.go) |
| `oracle_mid_blocker_duration` | `Float64Histogram` | [modules/oracle/metrics.go](../../../modules/oracle/metrics.go) |
| `oracle_end_blocker_duration` | `Float64Histogram` | [modules/oracle/metrics.go](../../../modules/oracle/metrics.go) |
| `oracle_price_update` | `Int64Counter` | [modules/oracle/metrics.go](../../../modules/oracle/metrics.go) |
| `oracle_vote_penalty_count` | `Int64Gauge` | [modules/oracle/keeper/metrics.go](../../../modules/oracle/keeper/metrics.go) |
| `oracle_validator_slashed` | `Int64Counter` | [modules/oracle/keeper/metrics.go](../../../modules/oracle/keeper/metrics.go) |
| `evm_abci_begin_blocker_duration` | `Float64Histogram` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_abci_end_blocker_duration` | `Float64Histogram` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_block_base_fee` | `Float64Gauge` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_panics` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_errors` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_receipt_status` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_keeper_association_error` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_zero_storage_processed_keys` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_zero_storage_pruned_keys` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_zero_storage_pruned_bytes` | `Int64Counter` | [modules/evm/keeper/metrics.go](../../../modules/evm/keeper/metrics.go) |
| `evm_pending_nonce` | `Int64Counter` | [modules/evm/ante/metrics.go](../../../modules/evm/ante/metrics.go) |
| `evm_nonce_mismatch` | `Int64Counter` | [modules/evm/ante/metrics.go](../../../modules/evm/ante/metrics.go) |
| `evm_effective_gas_price` | `Float64Histogram` | [modules/evm/ante/metrics.go](../../../modules/evm/ante/metrics.go) |
| `evm_ante_association_error` | `Int64Counter` | [modules/evm/ante/metrics.go](../../../modules/evm/ante/metrics.go) |
| `app_abci_begin_block_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_abci_end_block_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_abci_module_end_block_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_abci_check_tx_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_abci_deliver_tx_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_abci_deliver_batch_tx_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_abci_commit_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_block_process_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_tx_count` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_tx_process_type` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_tx_gas` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_optimistic_processing` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_failed_total_gas_wanted_check` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_giga_fallback_to_v2` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_block_gas_wanted` | `Int64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_block_gas_wanted_ratio` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_lightinvariance_supply_duration` | `Float64Histogram` | [node/metrics.go](../../../node/metrics.go) |
| `app_lightinvariance_supply_invalid_key` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `app_lightinvariance_supply_unmarshal_failure` | `Int64Counter` | [node/metrics.go](../../../node/metrics.go) |
| `evmrpc_request_latency_seconds` | `Float64Histogram` | [rpc/metrics.go](../../../rpc/metrics.go) |
| `evmrpc_websocket_connect_attempts_total` | `Int64Counter` | [rpc/metrics.go](../../../rpc/metrics.go) |
| `evmrpc_websocket_connects_total` | `Int64Counter` | [rpc/metrics.go](../../../rpc/metrics.go) |
| `evmrpc_redirected_requests_total` | `Int64Counter` | [rpc/metrics.go](../../../rpc/metrics.go) |
| `evmrpc_historical_debug_trace_attempts_total` | `Int64Counter` | [rpc/metrics.go](../../../rpc/metrics.go) |
