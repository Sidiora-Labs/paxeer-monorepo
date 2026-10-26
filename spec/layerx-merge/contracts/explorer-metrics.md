# Explorer metrics contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## apps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.met-explorer-active-addresses-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:63</code> | <code>gauge active_addresses_number registry=public</code> |
| <code>interface.14.met-explorer-batch-average-time</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:90</code> | <code>gauge batch_average_time registry=default</code> |
| <code>interface.14.met-explorer-block-batch-fetch-request-duration-microseconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:29</code> | <code>histogram block_batch_fetch_request_duration_microseconds{fetcher} registry=default</code> |
| <code>interface.14.met-explorer-block-full-processing-duration-microseconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:13</code> | <code>histogram block_full_processing_duration_microseconds{fetcher} registry=default</code> |
| <code>interface.14.met-explorer-block-import-duration-microseconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:21</code> | <code>histogram block_import_duration_microseconds{fetcher} registry=default</code> |
| <code>interface.14.met-explorer-block-import-stage-runner-duration-microseconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:8</code> | <code>histogram block_import_stage_runner_duration_microseconds{stage,runner,step} registry=default</code> |
| <code>interface.14.met-explorer-delay-from-last-node-block</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:37</code> | <code>gauge delay_from_last_node_block registry=default</code> |
| <code>interface.14.met-explorer-deployed-smart-contracts-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:32</code> | <code>gauge deployed_smart_contracts_number registry=public</code> |
| <code>interface.14.met-explorer-event-handler-queue-length</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/prometheus/instrumenter.ex:8</code> | <code>gauge event_handler_queue_length{handler} registry=default</code> |
| <code>interface.14.met-explorer-failed-token-instances-metadata-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:56</code> | <code>gauge failed_token_instances_metadata_count registry=default</code> |
| <code>interface.14.met-explorer-failed-uploading-media-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:84</code> | <code>counter failed_uploading_media_number registry=public</code> |
| <code>interface.14.met-explorer-import-errors-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:39</code> | <code>counter import_errors_count registry=default</code> |
| <code>interface.14.met-explorer-json-rpc-requests-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/prometheus/instrumenter.ex:8</code> | <code>counter json_rpc_requests_count{method} registry=default</code> |
| <code>interface.14.met-explorer-json-rpc-requests-errors-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/prometheus/instrumenter.ex:9</code> | <code>counter json_rpc_requests_errors_count{method} registry=default</code> |
| <code>interface.14.met-explorer-json-rpc-throttle-coalesced-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/http/throttle/metrics.ex:35</code> | <code>counter json_rpc_throttle_coalesced_count{class} registry=default</code> |
| <code>interface.14.met-explorer-json-rpc-throttle-inflight</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/http/throttle/metrics.ex:21</code> | <code>gauge json_rpc_throttle_inflight{class} registry=default</code> |
| <code>interface.14.met-explorer-json-rpc-throttle-queue-depth</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/http/throttle/metrics.ex:15</code> | <code>gauge json_rpc_throttle_queue_depth{class} registry=default</code> |
| <code>interface.14.met-explorer-json-rpc-throttle-wait-time-milliseconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/http/throttle/metrics.ex:27</code> | <code>histogram json_rpc_throttle_wait_time_milliseconds{class} registry=default</code> |
| <code>interface.14.met-explorer-latest-batch-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:153</code> | <code>gauge latest_batch_number registry=default</code> |
| <code>interface.14.met-explorer-latest-batch-timestamp</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:155</code> | <code>gauge latest_batch_timestamp registry=default</code> |
| <code>interface.14.met-explorer-latest-block-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:43</code> | <code>gauge latest_block_number registry=default</code> |
| <code>interface.14.met-explorer-latest-block-timestamp</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:45</code> | <code>gauge latest_block_timestamp registry=default</code> |
| <code>interface.14.met-explorer-media-processing-time</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:71</code> | <code>histogram media_processing_time registry=default</code> |
| <code>interface.14.met-explorer-memory-consumed</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:41</code> | <code>gauge memory_consumed{fetcher} registry=default</code> |
| <code>interface.14.met-explorer-missing-archival-token-balances-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:54</code> | <code>gauge missing_archival_token_balances_count registry=default</code> |
| <code>interface.14.met-explorer-missing-blocks-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:48</code> | <code>gauge missing_blocks_count registry=default</code> |
| <code>interface.14.met-explorer-missing-current-token-balances-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:53</code> | <code>gauge missing_current_token_balances_count registry=default</code> |
| <code>interface.14.met-explorer-missing-internal-transactions-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:49</code> | <code>gauge missing_internal_transactions_count registry=default</code> |
| <code>interface.14.met-explorer-multichain-search-db-export-balances-queue-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:59</code> | <code>gauge multichain_search_db_export_balances_queue_count registry=default</code> |
| <code>interface.14.met-explorer-multichain-search-db-export-counters-queue-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:60</code> | <code>gauge multichain_search_db_export_counters_queue_count registry=default</code> |
| <code>interface.14.met-explorer-multichain-search-db-export-token-info-queue-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:61</code> | <code>gauge multichain_search_db_export_token_info_queue_count registry=default</code> |
| <code>interface.14.met-explorer-multichain-search-db-main-export-queue-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:58</code> | <code>gauge multichain_search_db_main_export_queue_count registry=default</code> |
| <code>interface.14.met-explorer-new-addresses-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:45</code> | <code>gauge new_addresses_number registry=public</code> |
| <code>interface.14.met-explorer-new-token-transfers-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:57</code> | <code>gauge new_token_transfers_number registry=public</code> |
| <code>interface.14.met-explorer-new-tokens-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:51</code> | <code>gauge new_tokens_number registry=public</code> |
| <code>interface.14.met-explorer-stats-import-stage-runner-duration-microseconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:16</code> | <code>histogram stats_import_stage_runner_duration_microseconds{stats_type} registry=default</code> |
| <code>interface.14.met-explorer-success-transactions-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:26</code> | <code>gauge success_transactions_number registry=public</code> |
| <code>interface.14.met-explorer-successfully-uploaded-media-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:78</code> | <code>counter successfully_uploaded_media_number registry=public</code> |
| <code>interface.14.met-explorer-token-instances-not-uploaded-to-cdn-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:57</code> | <code>gauge token_instances_not_uploaded_to_cdn_count registry=default</code> |
| <code>interface.14.met-explorer-unfetched-token-instances-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/instrumenter.ex:55</code> | <code>gauge unfetched_token_instances_count registry=default</code> |
| <code>interface.14.met-explorer-verified-smart-contracts-number</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:39</code> | <code>gauge verified_smart_contracts_number registry=public</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.met-explorer-active-addresses-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of active EOA addresses (participated in transactions in to/from) in the period (default is 1 day) |
| <code>interface.14.met-explorer-batch-average-time</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: L2 average batch time; set only by rollup batch fetchers, not by the paxeer_x chain type |
| <code>interface.14.met-explorer-block-batch-fetch-request-duration-microseconds</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Block fetch batch request processing time |
| <code>interface.14.met-explorer-block-full-processing-duration-microseconds</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Block whole processing time including fetch and import |
| <code>interface.14.met-explorer-block-import-duration-microseconds</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Block import time |
| <code>interface.14.met-explorer-block-import-stage-runner-duration-microseconds</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Block import stage, runner and step in runner processing time |
| <code>interface.14.met-explorer-delay-from-last-node-block</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Delay from the last block on the node in seconds |
| <code>interface.14.met-explorer-deployed-smart-contracts-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of deployed smart-contracts (smart-contracts from internal transactions are not accounted) in the period (default is 1 day) |
| <code>interface.14.met-explorer-event-handler-queue-length</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of events in event handlers queue |
| <code>interface.14.met-explorer-failed-token-instances-metadata-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of failed token instances metadata; no in-repo reader or setter found outside the declaration |
| <code>interface.14.met-explorer-failed-uploading-media-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of failed uploading media to CDN |
| <code>interface.14.met-explorer-import-errors-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of database import errors |
| <code>interface.14.met-explorer-json-rpc-requests-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of JSON RPC requests |
| <code>interface.14.met-explorer-json-rpc-requests-errors-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of JSON RPC requests errors |
| <code>interface.14.met-explorer-json-rpc-throttle-coalesced-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of JSON RPC requests served by an already in-flight identical request |
| <code>interface.14.met-explorer-json-rpc-throttle-inflight</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of JSON RPC requests currently in flight against the node |
| <code>interface.14.met-explorer-json-rpc-throttle-queue-depth</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of JSON RPC requests waiting for a slot on the node guard |
| <code>interface.14.met-explorer-json-rpc-throttle-wait-time-milliseconds</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Time a JSON RPC request spent waiting for a slot on the node guard |
| <code>interface.14.met-explorer-latest-batch-number</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: L2 latest batch number; set only by rollup batch fetchers, not by the paxeer_x chain type |
| <code>interface.14.met-explorer-latest-batch-timestamp</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: L2 latest batch timestamp; set only by rollup batch fetchers, not by the paxeer_x chain type |
| <code>interface.14.met-explorer-latest-block-number</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Latest block number |
| <code>interface.14.met-explorer-latest-block-timestamp</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Latest block timestamp |
| <code>interface.14.met-explorer-media-processing-time</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Time in seconds taken for media resizing and uploading |
| <code>interface.14.met-explorer-memory-consumed</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Amount of memory consumed by fetchers (MB) |
| <code>interface.14.met-explorer-missing-archival-token-balances-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of missing token balances in history |
| <code>interface.14.met-explorer-missing-blocks-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of blocks missing in the chain |
| <code>interface.14.met-explorer-missing-current-token-balances-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of missing current token balances; no in-repo reader or setter found outside the declaration |
| <code>interface.14.met-explorer-missing-internal-transactions-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of blocks with not yet fetched internal transactions |
| <code>interface.14.met-explorer-multichain-search-db-export-balances-queue-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Size of the balances export queue |
| <code>interface.14.met-explorer-multichain-search-db-export-counters-queue-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Size of the counters export queue |
| <code>interface.14.met-explorer-multichain-search-db-export-token-info-queue-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Size of the token info export queue |
| <code>interface.14.met-explorer-multichain-search-db-main-export-queue-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Size of the main multichain export queue |
| <code>interface.14.met-explorer-new-addresses-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of new wallet addresses in the period (default is 1 day) |
| <code>interface.14.met-explorer-new-token-transfers-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of new token transfers detected in the period (default is 1 day) |
| <code>interface.14.met-explorer-new-tokens-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of new tokens detected in the period (default is 1 day) |
| <code>interface.14.met-explorer-stats-import-stage-runner-duration-microseconds</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Stats import stage runner duration microseconds |
| <code>interface.14.met-explorer-success-transactions-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of successful transactions in the period (default is 1 day) |
| <code>interface.14.met-explorer-successfully-uploaded-media-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of successfully uploaded media to CDN |
| <code>interface.14.met-explorer-token-instances-not-uploaded-to-cdn-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Token instances not uploaded to CDN; no in-repo reader or setter found outside the declaration |
| <code>interface.14.met-explorer-unfetched-token-instances-count</code> | notes | exported at /metrics by BlockScoutWeb.Prometheus.Exporter; help: Number of unfetched token instances; no in-repo reader or setter found outside the declaration |
| <code>interface.14.met-explorer-verified-smart-contracts-number</code> | notes | exported at /public-metrics by BlockScoutWeb.Prometheus.PublicExporter; help: Number of verified smart-contracts in the period (default is 1 day) |

## collectors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.met-explorer-active-db-connections</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/prometheus/collector/active_db_connections.ex:11</code> | <code>gauge active_db_connections (collector active_db_connections)</code> |
| <code>interface.14.met-explorer-ecto-query-duration</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/repo/prometheus_logger.ex:8</code> | <code>use Prometheus.EctoInstrumenter (ecto query, queue, decode and total duration histograms)</code> |
| <code>interface.14.met-explorer-filecoin-pending-address-operations</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/collector/filecoin_pending_address_operations_collector.ex:19</code> | <code>gauge filecoin_pending_address_operations (collector filecoin_pending_address_operations_collector)</code> |
| <code>interface.14.met-explorer-pending-block-operations-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/collector/pending_block_operations_collector.ex:15</code> | <code>gauge pending_block_operations_count (collector pending_block_operations_collector)</code> |
| <code>interface.14.met-explorer-pending-transaction-operations-count</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/prometheus/collector/pending_transaction_operations_collector.ex:15</code> | <code>gauge pending_transaction_operations_count (collector pending_transaction_operations_collector)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.met-explorer-active-db-connections</code> | notes | Prometheus.Collector gauge computed at scrape time and exported at /metrics; the collector is registered by behaviour discovery at prometheus start, with no explicit in-repo registration |
| <code>interface.14.met-explorer-ecto-query-duration</code> | notes | metric names come from the prometheus_ecto library defaults |
| <code>interface.14.met-explorer-filecoin-pending-address-operations</code> | notes | Prometheus.Collector gauge computed at scrape time and exported at /metrics; the collector is registered by behaviour discovery at prometheus start, with no explicit in-repo registration; compiled only for CHAIN_TYPE filecoin |
| <code>interface.14.met-explorer-pending-block-operations-count</code> | notes | Prometheus.Collector gauge computed at scrape time and exported at /metrics; the collector is registered by behaviour discovery at prometheus start, with no explicit in-repo registration |
| <code>interface.14.met-explorer-pending-transaction-operations-count</code> | notes | Prometheus.Collector gauge computed at scrape time and exported at /metrics; the collector is registered by behaviour discovery at prometheus start, with no explicit in-repo registration |

## exporters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/prometheus/exporter.ex:8</code> | <code>GET /metrics -&gt; BlockScoutWeb.Prometheus.Exporter (Prometheus.PlugExporter default path)</code> |
| <code>interface.14.http-get-public-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/prometheus/public_exporter.ex:8</code> | <code>GET /public-metrics -&gt; BlockScoutWeb.Prometheus.PublicExporter (registry :public, auth false)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-metrics</code> | notes | plugged in endpoint.ex ahead of the router; default registry |
| <code>interface.14.http-get-public-metrics</code> | notes | path set by config :prometheus at services/explorer/backend/apps/block_scout_web/config/config.exs:60-64; services/explorer/backend/apps/explorer/lib/explorer/prometheus/instrumenter.ex:24 comment says /public_metrics |

## launcher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-launcher-get-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:177</code> | <code>GET /metrics on &lt;SERVICE&gt;__METRICS__ADDR (default port 6060) when &lt;SERVICE&gt;__METRICS__ENABLED</code> |
| <code>interface.14.met-launcher-rust-microservices-http</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/libs/blockscout-service-launcher/src/launcher/metrics.rs:19</code> | <code>actix-web-prom namespace rust_microservices with const label service_name, plus per-service namespace http middleware (http_requests_total, http_requests_duration_seconds)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-launcher-get-metrics</code> | notes | served by the launcher metrics server (services/explorer/services/libs/blockscout-service-launcher/src/launcher/metrics.rs:40) for sig-provider-server and smart-contract-verifier-server |
| <code>interface.14.met-launcher-rust-microservices-http</code> | notes | shared by sig-provider-server and smart-contract-verifier-server; sig-provider defines no service metrics of its own |

## verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.met-verifier-smart-contract-verifier-compilation-queue-time-seconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:33</code> | <code>Histogram smart_contract_verifier_compilation_queue_time_seconds</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-compile-time-seconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:23</code> | <code>Histogram smart_contract_verifier_compile_time_seconds</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-compiler-fetch-time-seconds</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:17</code> | <code>Histogram smart_contract_verifier_compiler_fetch_time_seconds</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-compiles-in-flight</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:28</code> | <code>Gauge smart_contract_verifier_compiles_in_flight</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-compiles-in-queue</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:38</code> | <code>Gauge smart_contract_verifier_compiles_in_queue</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-download-cache-hits</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:12</code> | <code>IntCounter smart_contract_verifier_download_cache_hits</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-download-cache-total</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier/src/metrics.rs:7</code> | <code>IntCounter smart_contract_verifier_download_cache_total</code> |
| <code>interface.14.met-verifier-smart-contract-verifier-verify-contract</code> | <code>metric</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/metrics.rs:5</code> | <code>IntCounterVec smart_contract_verifier_verify_contract{chain_id,language,endpoint,status}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.met-verifier-smart-contract-verifier-compilation-queue-time-seconds</code> | notes | help: waiting for the compilation queue in seconds; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-compile-time-seconds</code> | notes | help: contract compilation time in seconds; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-compiler-fetch-time-seconds</code> | notes | help: download time for compilers in seconds; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-compiles-in-flight</code> | notes | help: number of compilations currently running; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-compiles-in-queue</code> | notes | help: number of compilations in queue; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-download-cache-hits</code> | notes | help: number of cache hits in DownloadCache; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-download-cache-total</code> | notes | help: total number of get calls in DownloadCache; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
| <code>interface.14.met-verifier-smart-contract-verifier-verify-contract</code> | notes | help: number of contract verifications; exported on the launcher metrics server (MetricsSettings route default /metrics, default port 6060) |
