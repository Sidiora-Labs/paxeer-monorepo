# Explorer metrics

Explorer metrics describe indexing and supporting services for Paxeer mainnet-beta, one BFT network with EVM chain ID **125** (`0x7d`). The kernel proposes blocks as part of that network. Indexed height, HTTP success, and a live socket are operational observations; finality comes from the block's commit. See [HTTP health and status](explorer-http-api.md), [socket recovery](explorer-ws.md), and [service configuration](explorer-services-config.md).

## Exporters

The Phoenix [Endpoint](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex) plugs in two Prometheus exporters:

| GET path | Module | Registry |
| --- | --- | --- |
| `/metrics` | `BlockScoutWeb.Prometheus.Exporter` | Default Prometheus registry |
| `/public-metrics` | `BlockScoutWeb.Prometheus.PublicExporter` | `:public` |

The public exporter's [configuration](../services/explorer/backend/apps/block_scout_web/config/config.exs) explicitly sets `path: "/public-metrics"`, `format: :auto`, `registry: :public`, and `auth: false`. The private exporter uses `Prometheus.PlugExporter` defaults; inspect runtime configuration and edge access control before assuming it is protected. When the endpoint is compiled with `disable_api?`, only the default exporter and HealthRouter are mounted; the public exporter and sockets belong to the API-enabled branch. These are exporter paths, not REST API V2 resources. `GET /api/v2/config/public-metrics` instead returns public-metric configuration.

## Apps

These are application metric names before Prometheus exposition adds histogram bucket/count/sum series. Unless specified as public, they use the default registry. Labels in braces are exact declaration names; a row without labels has none declared by that instrumenter.

### JSON-RPC and throttle

[EthereumJSONRPC.Prometheus.Instrumenter](../services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/prometheus/instrumenter.ex) declares `json_rpc_requests_count{method}` and `json_rpc_requests_errors_count{method}` counters. They count upstream JSON-RPC calls and errors, not explorer user API calls.

[Throttle.Metrics](../services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/http/throttle/metrics.ex) declares:

| Name | Type | Labels/units |
| --- | --- | --- |
| `json_rpc_throttle_queue_depth` | gauge | `class`; waiting requests |
| `json_rpc_throttle_inflight` | gauge | `class`; active requests |
| `json_rpc_throttle_wait_time_milliseconds` | histogram | `class`; milliseconds |
| `json_rpc_throttle_coalesced_count` | counter | `class`; requests sharing an in-flight request |

`class` is `default`, `archive`, or `trace`. Endpoint URLs are deliberately excluded from labels because they can contain credentials. Wait histogram buckets are 1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000, and 60000 ms. Preserve that unit when calculating quantiles.

### Indexer

[Indexer.Prometheus.Instrumenter](../services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex) declares:

| Names | Type | Meaning |
| --- | --- | --- |
| `block_full_processing_duration_microseconds`, `block_import_duration_microseconds`, `block_batch_fetch_request_duration_microseconds` | histograms with `fetcher` | Processing, import, and batch-fetch durations in microseconds |
| `delay_from_last_node_block` | gauge | Seconds of delay from the last node block |
| `import_errors_count` | counter | Database import errors |
| `memory_consumed{fetcher}` | gauge | Fetcher memory in MB |
| `latest_block_number` | gauge | Indexed block height |
| `latest_block_timestamp` | gauge | Block timestamp in Unix seconds |
| `missing_blocks_count` | gauge | Missing blocks |
| `missing_internal_transactions_count` | gauge | Blocks awaiting internal transaction fetches |
| `missing_current_token_balances_count`, `missing_archival_token_balances_count` | gauges | Current/historical balance backlog |
| `unfetched_token_instances_count`, `failed_token_instances_metadata_count`, `token_instances_not_uploaded_to_cdn_count` | gauges | NFT metadata/media backlog |
| `multichain_search_db_main_export_queue_count`, `multichain_search_db_export_balances_queue_count`, `multichain_search_db_export_counters_queue_count`, `multichain_search_db_export_token_info_queue_count` | gauges | Search export queue sizes |

The duration histograms use buckets 1000, 5000, 10000, and 100000 microseconds. Height/timestamp setup can publish initial values before ingestion. Compare progress against the configured node rather than treating a nonzero series as proof of being caught up.

The shared indexer also declares `latest_batch_number` and `latest_batch_timestamp`; its update path is conditional on rollup chain types. Likewise, `Explorer.Prometheus.Instrumenter` declares `batch_average_time` for shared batch integrations. These names are not a second Paxeer settlement or finality stage.

### Explorer import and media

[Explorer.Prometheus.Instrumenter](../services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex) declares `block_import_stage_runner_duration_microseconds{stage,runner,step}` and `stats_import_stage_runner_duration_microseconds{stats_type}` histograms. Both use 1000, 5000, 10000, and 100000 microsecond buckets. `media_processing_time` is a seconds histogram in the default registry.

The public registry contains these gauges: `success_transactions_number`, `deployed_smart_contracts_number`, `verified_smart_contracts_number`, `new_addresses_number`, `new_tokens_number`, `new_token_transfers_number`, and `active_addresses_number`. They report a configured reporting period (default one day), not monotonically increasing totals. Deployed-contract counts exclude contracts found only in internal transactions. `successfully_uploaded_media_number` and `failed_uploading_media_number` are public counters.

### Web event handlers

[BlockScoutWeb.Prometheus.Instrumenter](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/prometheus/instrumenter.ex) exposes the gauge `event_handler_queue_length{handler}`. Queue depth measures work awaiting notification handling, not the number of subscribers or committed transactions.

## Collectors

Scrape-time collectors can query runtime/database state:

| Collector metric | Source and meaning |
| --- | --- |
| `active_db_connections` | [ActiveDbConnections](../services/explorer/backend/apps/explorer/lib/explorer/prometheus/collector/active_db_connections.ex): sum of pool size minus ready connections for running explorer repos |
| `pending_block_operations_count` | [PendingBlockOperations](../services/explorer/backend/apps/indexer/lib/indexer/prometheus/collector/pending_block_operations_collector.ex): pending-block table row count |
| `pending_transaction_operations_count` | [PendingTransactionOperations](../services/explorer/backend/apps/indexer/lib/indexer/prometheus/collector/pending_transaction_operations_collector.ex): pending-transaction table row count |
| `filecoin_pending_address_operations` | [Filecoin collector](../services/explorer/backend/apps/indexer/lib/indexer/prometheus/collector/filecoin_pending_address_operations_collector.ex): shared Filecoin integration backlog |

[Explorer.Repo.PrometheusLogger](../services/explorer/backend/apps/explorer/lib/explorer/repo/prometheus_logger.ex) uses `Prometheus.EctoInstrumenter` for query, queue, decode, and total durations. Collector registration and application configuration determine which series are exposed. Filecoin and rollup-specific declarations in shared code do not imply those subsystems run on Paxeer.

## Launcher

Rust explorer services using [launcher::Metrics](../services/explorer/services/libs/blockscout-service-launcher/src/launcher/metrics.rs) expose a separate listener only when `<SERVICE>__METRICS__ENABLED` is true. Defaults are disabled, `0.0.0.0:6060`, and `/metrics`. Configure `METRICS__ADDR` and `METRICS__ROUTE` under the service's prefix as described in [shared settings](explorer-services-config.md#shared-server-settings).

The metrics middleware uses namespace `rust_microservices` with constant label `service_name`. Application HTTP middleware uses the service-name namespace; actix-web-prom supplies HTTP request counts and duration histograms (`http_requests_total` and `http_requests_duration_seconds` within that namespace). Both use the default Prometheus registry. Service-specific registered metrics retain their declared names.

The metrics route is independent of `server.http.base_path` and of the application HTTP port. Enabling application HTTP alone does not start metrics. The launcher does not attach user authentication to its metrics listener; expose it according to deployment access policy.

## Verifier

The [verifier library metrics](../services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs) are:

| Metric | Type | Units |
| --- | --- | --- |
| `smart_contract_verifier_download_cache_total` | integer counter | Cache lookup calls |
| `smart_contract_verifier_download_cache_hits` | integer counter | Cache hits |
| `smart_contract_verifier_compiler_fetch_time_seconds` | histogram | Seconds |
| `smart_contract_verifier_compile_time_seconds` | histogram | Seconds |
| `smart_contract_verifier_compilation_queue_time_seconds` | histogram | Seconds |
| `smart_contract_verifier_compiles_in_flight` | gauge | Running compilations |
| `smart_contract_verifier_compiles_in_queue` | gauge | Waiting compilations |

The [server metric](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/metrics.rs) `smart_contract_verifier_verify_contract` is an integer counter vector with labels `chain_id`, `language`, `endpoint`, and `status`, in that order. The counting helper supplies its `method` argument as the `endpoint` label. These metrics measure source verification, not consensus validation.

## Restart and interpretation

Application counters and histogram observations live in process memory and reset on restart. Database collectors re-read persisted state; gauges are refreshed by their producers. Use reset-aware counter rates and retain scrape history externally. Missing series, zero-initialized heights, a successful liveness probe, and unavailable capability flags have different meanings. Check [HTTP health](explorer-http-api.md#api-health) and indexing state together before diagnosing lag.
