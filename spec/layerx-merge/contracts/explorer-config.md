# Explorer config contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-bcrypt-elixir

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-bcrypt-elixir-log-rounds</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/test.exs:4</code> | <code>config :bcrypt_elixir, :log_rounds</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-bcrypt-elixir-log-rounds</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-ethereum-jsonrpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-geth</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:287</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.Geth [block_traceable?, allow_empty_traces?, debug_trace_timeout, tracer]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-http</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:273</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.HTTP</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-http-throttle</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:280</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.HTTP.Throttle [max_inflight, rps, trace_max_inflight, archive_max_inflight, coalesce?]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-paxeerx</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:293</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.PaxeerX [cosmos_transaction_type, native_coin_decimals]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-pendingtransaction</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:300</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.PendingTransaction [type]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-requestcoordinator</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:303</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.RequestCoordinator [wait_per_timeout, rolling_window_opts, max_jitter]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-tracer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs:19</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-utility-endpointavailabilitychecker</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:309</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.Utility.EndpointAvailabilityChecker [enabled]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-websocket-retryworker</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:306</code> | <code>config :ethereum_jsonrpc, EthereumJSONRPC.WebSocket.RetryWorker [retry_interval]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ipc-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:263</code> | <code>config :ethereum_jsonrpc, :ipc_path</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-rpc-transport</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:263</code> | <code>config :ethereum_jsonrpc, :rpc_transport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-geth</code> | notes | set in 1 config statement(s); 99 reader(s); 59 further callers omitted; 1 more listed callers dropped for line length |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-http</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-http-throttle</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-paxeerx</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-pendingtransaction</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-requestcoordinator</code> | notes | set in 4 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-tracer</code> | notes | set in 4 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-utility-endpointavailabilitychecker</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-websocket-retryworker</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-ipc-path</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-ethereum-jsonrpc-rpc-transport</code> | notes | set in 1 config statement(s); 1 reader(s) |

## cfg-ex-aws

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-ex-aws-access-key-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2164</code> | <code>config :ex_aws, :access_key_id</code> |
| <code>interface.14.cfg-ex-aws-json-codec</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2164</code> | <code>config :ex_aws, :json_codec</code> |
| <code>interface.14.cfg-ex-aws-s3</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2169</code> | <code>config :ex_aws, :s3 [scheme, host, port, public_r2_url, bucket_name]</code> |
| <code>interface.14.cfg-ex-aws-secret-access-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2164</code> | <code>config :ex_aws, :secret_access_key</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-ex-aws-access-key-id</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-ex-aws-json-codec</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-ex-aws-s3</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-ex-aws-secret-access-key</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-ex-cldr

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-ex-cldr-default-backend</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:56</code> | <code>config :ex_cldr, :default_backend</code> |
| <code>interface.14.cfg-ex-cldr-default-locale</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:56</code> | <code>config :ex_cldr, :default_locale</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-ex-cldr-default-backend</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-ex-cldr-default-locale</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-explorer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-explorer-air-table-audit-reports</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:825</code> | <code>config :explorer, :air_table_audit_reports [table_url, api_key, enabled]</code> |
| <code>interface.14.cfg-explorer-celo</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:711</code> | <code>config :explorer, :celo [l2_migration_block, epoch_manager_contract_address, unreleased_treasury_contract_address, validators_contract_address, locked_gold_contract_address, accounts_contract_address]</code> |
| <code>interface.14.cfg-explorer-chain-identity</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:325</code> | <code>config :explorer, :chain_identity</code> |
| <code>interface.14.cfg-explorer-chain-type</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:325</code> | <code>config :explorer, :chain_type</code> |
| <code>interface.14.cfg-explorer-coin</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:325</code> | <code>config :explorer, :coin</code> |
| <code>interface.14.cfg-explorer-coin-name</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:325</code> | <code>config :explorer, :coin_name</code> |
| <code>interface.14.cfg-explorer-datadog</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:858</code> | <code>config :explorer, :datadog [port]</code> |
| <code>interface.14.cfg-explorer-ecto-repos</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:325</code> | <code>config :explorer, :ecto_repos</code> |
| <code>interface.14.cfg-explorer-explorer-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:834</code> | <code>config :explorer, Explorer.Account [enabled, sendgrid]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-blackfort-validator</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1144</code> | <code>config :explorer, Explorer.Chain.Blackfort.Validator [api_url]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-block-reward</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:676</code> | <code>config :explorer, Explorer.Chain.Block.Reward [validators_contract_address, keys_manager_contract_address]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-bridgedtoken</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1123</code> | <code>config :explorer, Explorer.Chain.BridgedToken [eth_omni_bridge_mediator, bsc_omni_bridge_mediator, poa_omni_bridge_mediator, amb_bridge_mediators, foreign_json_rpc, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-accounts</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:700</code> | <code>config :explorer, Explorer.Chain.Cache.Accounts [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-blocknumber</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:688</code> | <code>config :explorer, Explorer.Chain.Cache.BlockNumber [ttl_check_interval, global_ttl, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-blocks</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:692</code> | <code>config :explorer, Explorer.Chain.Cache.Blocks [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-celocorecontracts</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:719</code> | <code>config :explorer, Explorer.Chain.Cache.CeloCoreContracts [contracts]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-celoepochs</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1077</code> | <code>config :explorer, Explorer.Chain.Cache.CeloEpochs [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescoinbalancesum</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:398</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressesCoinBalanceSum [global_ttl, enabled, ttl_check_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescoinbalancesumminusburnt</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:400</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressesCoinBalanceSumMinusBurnt [global_ttl, enabled, ttl_check_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:409</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressesCount [update_interval_in_milliseconds, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstabselementscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:871</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTabsElementsCount [ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstokensusdsum</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:460</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTokensUsdSum [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstokentransferscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:463</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTokenTransfersCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstransactionscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:457</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTransactionsCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstransactionsgasusagesum</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:439</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTransactionsGasUsageSum [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-averageblocktime</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:448</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.AverageBlockTime [enabled, period, cache_period, num_of_blocks]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blackfort-validatorscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:77</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.Blackfort.ValidatorsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockburntfeecount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:104</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.BlockBurntFeeCount [enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockpriorityfeecount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:108</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.BlockPriorityFeeCount [enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:406</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.BlocksCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-contractscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:47</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.ContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-gasusagesum</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:402</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.GasUsageSum [global_ttl, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newcontractscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:52</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.NewContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newpendingtransactionscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:476</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.NewPendingTransactionsCount [enabled, cache_period, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newverifiedcontractscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:62</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.NewVerifiedContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-optimism-lastoutputrootsizecount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:466</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.Optimism.LastOutputRootSizeCount [enabled, enable_consolidation, cache_period]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-pendingblockoperationcount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:415</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.PendingBlockOperationCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-pendingtransactionoperationcount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:418</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.PendingTransactionOperationCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-rootstock-lockedbtccount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:432</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.Rootstock.LockedBTCCount [enabled, global_ttl, locking_cap]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-stability-validatorscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:72</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.Stability.ValidatorsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-tokenholderscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:442</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.TokenHoldersCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-tokentransferscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:445</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.TokenTransfersCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-transactions24hcount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:471</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.Transactions24hCount [enabled, cache_period, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-transactionscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:412</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.TransactionsCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-verifiedcontractscount</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:57</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.VerifiedContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-withdrawalssum</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:67</code> | <code>config :explorer, Explorer.Chain.Cache.Counters.WithdrawalsSum [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-gaspriceoracle</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:421</code> | <code>config :explorer, Explorer.Chain.Cache.GasPriceOracle [global_ttl, simple_transaction_gas, num_of_blocks, safelow_percentile, average_percentile, fast_percentile, safelow_time_coefficient, average_time_coefficient, fast_time_coefficient]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-minmissingblocknumber</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:850</code> | <code>config :explorer, Explorer.Chain.Cache.MinMissingBlockNumber [enabled, batch_size]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-optimismfinalizationperiod</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:437</code> | <code>config :explorer, Explorer.Chain.Cache.OptimismFinalizationPeriod [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactionactiontokensdata</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:860</code> | <code>config :explorer, Explorer.Chain.Cache.TransactionActionTokensData [max_cache_size, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactionactionuniswappools</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:84</code> | <code>config :explorer, Explorer.Chain.Cache.TransactionActionUniswapPools [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactions</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:696</code> | <code>config :explorer, Explorer.Chain.Cache.Transactions [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-uncles</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:704</code> | <code>config :explorer, Explorer.Chain.Cache.Uncles [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-events-listener</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:375</code> | <code>config :explorer, Explorer.Chain.Events.Listener [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-addressesblacklist</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1148</code> | <code>config :explorer, Explorer.Chain.Fetcher.AddressesBlacklist [url, enabled, update_interval, retry_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-checkbytecodematchingondemand</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:192</code> | <code>config :explorer, Explorer.Chain.Fetcher.CheckBytecodeMatchingOnDemand [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-fetchvalidatorinfoondemand</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:194</code> | <code>config :explorer, Explorer.Chain.Fetcher.FetchValidatorInfoOnDemand [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-lookupsmartcontractsourcesondemand</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:863</code> | <code>config :explorer, Explorer.Chain.Fetcher.LookUpSmartContractSourcesOnDemand</code> |
| <code>interface.14.cfg-explorer-explorer-chain-filecoin-nativeaddress</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1141</code> | <code>config :explorer, Explorer.Chain.Filecoin.NativeAddress [network_prefix]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-health-monitor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:360</code> | <code>config :explorer, Explorer.Chain.Health.Monitor [check_interval, healthy_blocks_period, healthy_batches_period]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-metrics-publicmetrics</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1137</code> | <code>config :explorer, Explorer.Chain.Metrics.PublicMetrics [enabled, update_period_hours]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-mud</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1195</code> | <code>config :explorer, Explorer.Chain.Mud [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-capabilities</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:356</code> | <code>config :explorer, Explorer.Chain.PaxeerX.Capabilities [enabled, refresh_interval_seconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-finality</code> | <code>config_key</code> | <code>change</code> | <code>services/explorer/backend/config/runtime.exs:656</code> | <code>config :explorer, Explorer.Chain.PaxeerX.Finality [cache_ms, probe_batches, final_confirmations]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-sequencerreceipt</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:297</code> | <code>config :explorer, Explorer.Chain.PaxeerX.SequencerReceipt [router_url]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-scroll-l1feeparam</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1197</code> | <code>config :explorer, Explorer.Chain.Scroll.L1FeeParam [curie_upgrade_block, scalar_init, overhead_init, commit_scalar_init, blob_scalar_init, l1_base_fee_init, l1_blob_base_fee_init]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-tokentransfer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1133</code> | <code>config :explorer, Explorer.Chain.TokenTransfer [whitelisted_weth_contracts, weth_token_transfers_filtering_enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-transaction</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:650</code> | <code>config :explorer, Explorer.Chain.Transaction [block_miner_gets_burnt_fees?, suave_bid_contracts, rootstock_remasc_address, rootstock_bridge_address]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-transaction-history-historian</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:661</code> | <code>config :explorer, Explorer.Chain.Transaction.History.Historian [enabled, init_lag_milliseconds, days_to_compile_at_init]</code> |
| <code>interface.14.cfg-explorer-explorer-chainspec-genesisdata</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:389</code> | <code>config :explorer, Explorer.ChainSpec.GenesisData [chain_spec_path, genesis_processing_delay, emission_format, rewards_contract_address, precompiled_config_path, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-integrations-ectologger</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:196</code> | <code>config :explorer, Explorer.Integrations.EctoLogger [query_time_ms_threshold]</code> |
| <code>interface.14.cfg-explorer-explorer-mailer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:830</code> | <code>config :explorer, Explorer.Mailer [adapter, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-market</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:481</code> | <code>config :explorer, Explorer.Market [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-coin</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:608</code> | <code>config :explorer, Explorer.Market.Fetcher.Coin [store, enabled, fetch_btc_value]</code> |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-history</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:639</code> | <code>config :explorer, Explorer.Market.Fetcher.History [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-token</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:621</code> | <code>config :explorer, Explorer.Market.Fetcher.Token [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-history-cataloger</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/runtime/test.exs:38</code> | <code>config :explorer, Explorer.Market.History.Cataloger [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-markethistorycache</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:454</code> | <code>config :explorer, Explorer.Market.MarketHistoryCache [cache_period]</code> |
| <code>interface.14.cfg-explorer-explorer-market-source</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:483</code> | <code>config :explorer, Explorer.Market.Source</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-coingecko</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:504</code> | <code>config :explorer, Explorer.Market.Source.CoinGecko</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-coinmarketcap</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:531</code> | <code>config :explorer, Explorer.Market.Source.CoinMarketCap</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-cryptocompare</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:549</code> | <code>config :explorer, Explorer.Market.Source.CryptoCompare</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-cryptorank</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:561</code> | <code>config :explorer, Explorer.Market.Source.CryptoRank</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-defillama</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:582</code> | <code>config :explorer, Explorer.Market.Source.DefiLlama [coin_id, base_url]</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-dia</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:602</code> | <code>config :explorer, Explorer.Market.Source.DIA [blockchain, base_url, coin_address_hash, secondary_coin_address_hash]</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-mobula</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:586</code> | <code>config :explorer, Explorer.Market.Source.Mobula</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-accountabstraction</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:786</code> | <code>config :explorer, Explorer.MicroserviceInterfaces.AccountAbstraction [service_url, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-bens</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:781</code> | <code>config :explorer, Explorer.MicroserviceInterfaces.BENS [service_url, enabled, protocols]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-metadata</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:790</code> | <code>config :explorer, Explorer.MicroserviceInterfaces.Metadata [service_url, enabled, requests_timeout, proxy_requests_timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-multichainsearch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:799</code> | <code>config :explorer, Explorer.MicroserviceInterfaces.MultichainSearch [api_key, service_url]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-tacoperationlifecycle</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:818</code> | <code>config :explorer, Explorer.MicroserviceInterfaces.TACOperationLifecycle [enabled, service_url]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-arbitrumdarecordsnormalization</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1019</code> | <code>config :explorer, Explorer.Migrator.ArbitrumDaRecordsNormalization [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-backfillmetadataurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1089</code> | <code>config :explorer, Explorer.Migrator.BackfillMetadataURL [enabled, batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-backfillmultichainsearchdb</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:994</code> | <code>config :explorer, Explorer.Migrator.BackfillMultichainSearchDB [concurrency, batch_size]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-celoaccounts</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1053</code> | <code>config :explorer, Explorer.Migrator.CeloAccounts [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-celoaggregatedelectionrewards</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1068</code> | <code>config :explorer, Explorer.Migrator.CeloAggregatedElectionRewards [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-celol2epochs</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1071</code> | <code>config :explorer, Explorer.Migrator.CeloL2Epochs</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-deletezerovalueinternaltransactions</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1101</code> | <code>config :explorer, Explorer.Migrator.DeleteZeroValueInternalTransactions [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-emptyinternaltransactionsdata</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1055</code> | <code>config :explorer, Explorer.Migrator.EmptyInternalTransactionsData</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-filecoinpendingaddressoperations</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1040</code> | <code>config :explorer, Explorer.Migrator.FilecoinPendingAddressOperations [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:998</code> | <code>config :explorer, Explorer.Migrator.HeavyDbIndexOperation [check_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation-createarbitrumbatchl2blocksunconfirmedblocksindex</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1032</code> | <code>config :explorer, Explorer.Migrator.HeavyDbIndexOperation.CreateArbitrumBatchL2BlocksUnconfirmedBlocksIndex [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation-createtransactionsoperatorfeeconstantindex</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1036</code> | <code>config :explorer, Explorer.Migrator.HeavyDbIndexOperation.CreateTransactionsOperatorFeeConstantIndex [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-mergeadjacentmissingblockranges</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1094</code> | <code>config :explorer, Explorer.Migrator.MergeAdjacentMissingBlockRanges</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-refetchcontractcodes</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:990</code> | <code>config :explorer, Explorer.Migrator.RefetchContractCodes [concurrency, batch_size]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexblockswithmissingtransactions</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:939</code> | <code>config :explorer, Explorer.Migrator.ReindexBlocksWithMissingTransactions</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexduplicatedinternaltransactions</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:922</code> | <code>config :explorer, Explorer.Migrator.ReindexDuplicatedInternalTransactions</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexinternaltransactionswithincompatiblestatus</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:905</code> | <code>config :explorer, Explorer.Migrator.ReindexInternalTransactionsWithIncompatibleStatus</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-restoreomittedwethtransfers</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:961</code> | <code>config :explorer, Explorer.Migrator.RestoreOmittedWETHTransfers</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeduplicatedlogindexlogs</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:978</code> | <code>config :explorer, Explorer.Migrator.SanitizeDuplicatedLogIndexLogs</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeemptycontractcodeaddresses</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1007</code> | <code>config :explorer, Explorer.Migrator.SanitizeEmptyContractCodeAddresses</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeincorrectnfttokentransfers</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:895</code> | <code>config :explorer, Explorer.Migrator.SanitizeIncorrectNFTTokenTransfers [batch_size, concurrency, timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeincorrectwethtokentransfers</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:900</code> | <code>config :explorer, Explorer.Migrator.SanitizeIncorrectWETHTokenTransfers [batch_size, concurrency, timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeverifiedaddresses</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1001</code> | <code>config :explorer, Explorer.Migrator.SanitizeVerifiedAddresses [enabled, batch_size, concurrency, timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-shrinkinternaltransactions</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1079</code> | <code>config :explorer, Explorer.Migrator.ShrinkInternalTransactions [enabled, batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-smartcontractlanguage</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1084</code> | <code>config :explorer, Explorer.Migrator.SmartContractLanguage [enabled, batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-tokentransfertokentype</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:891</code> | <code>config :explorer, Explorer.Migrator.TokenTransferTokenType [batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-transactionsdenormalization</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:879</code> | <code>config :explorer, Explorer.Migrator.TransactionsDenormalization</code> |
| <code>interface.14.cfg-explorer-explorer-repo</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:65</code> | <code>config :explorer, Explorer.Repo [database, hostname, url, pool_size, queue_target, ssl, migration_timestamps, disconnect_on_error_codes, timeout, migration_lock, prepare, ssl_opts, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:87</code> | <code>config :explorer, Explorer.Repo.Account [database, hostname, url, pool_size, queue_target, ssl, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-eventnotifications</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:116</code> | <code>config :explorer, Explorer.Repo.EventNotifications [database, hostname, url, pool_size, queue_target, ssl, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-mud</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:98</code> | <code>config :explorer, Explorer.Repo.Mud [database, hostname, url, pool_size, queue_target, ssl]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-polygonzkevm</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/test.exs:102</code> | <code>config :explorer, Explorer.Repo.PolygonZkevm [database, hostname, url, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-replica1</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:76</code> | <code>config :explorer, Explorer.Repo.Replica1 [database, hostname, url, pool_size, queue_target, ssl, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-suave</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:106</code> | <code>config :explorer, Explorer.Repo.Suave [database, hostname, url, pool_size, ssl]</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-certifiedsmartcontractcataloger</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:200</code> | <code>config :explorer, Explorer.SmartContract.CertifiedSmartContractCataloger [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-rustverifierinterfacebehaviour</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:762</code> | <code>config :explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-sigproviderinterface</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:777</code> | <code>config :explorer, Explorer.SmartContract.SigProviderInterface [service_url, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-stylusverifierinterface</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:796</code> | <code>config :explorer, Explorer.SmartContract.StylusVerifierInterface [service_url]</code> |
| <code>interface.14.cfg-explorer-explorer-tags-addresstag-cataloger</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:198</code> | <code>config :explorer, Explorer.Tags.AddressTag.Cataloger [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-dynamic</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:744</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.Dynamic [enabled, env_id, url]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-dynamic-strategy</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:749</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.Dynamic.Strategy [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-keycloak</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:751</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.Keycloak [domain, realm, client_id, client_secret, email_webhook_url]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-novesfi</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:733</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.NovesFi [service_url, chain_name, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-solidityscan</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:728</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.SolidityScan [platform_id, chain_id, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-sourcify</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:722</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.Sourcify [server_url, enabled, chain_id, repo_url]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-universalproxy</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1191</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.UniversalProxy [config_url, config_json]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-xname</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:738</code> | <code>config :explorer, Explorer.ThirdPartyIntegrations.Xname [service_url, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-tokeninstanceowneraddressmigration</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:874</code> | <code>config :explorer, Explorer.TokenInstanceOwnerAddressMigration [concurrency, batch_size, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-tokeninstanceowneraddressmigration-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:112</code> | <code>config :explorer, Explorer.TokenInstanceOwnerAddressMigration.Supervisor [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-tracer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:211</code> | <code>config :explorer, Explorer.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-hammer-ets</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:205</code> | <code>config :explorer, Explorer.Utility.Hammer.ETS [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-hammer-redis</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:204</code> | <code>config :explorer, Explorer.Utility.Hammer.Redis [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-missingbalanceoftoken</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1130</code> | <code>config :explorer, Explorer.Utility.MissingBalanceOfToken [window_size]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-ratelimiter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1163</code> | <code>config :explorer, Explorer.Utility.RateLimiter [storage, redis_url, on_demand, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-versionconstantsupdater</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:118</code> | <code>config :explorer, Explorer.Utility.VersionConstantsUpdater [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-validator-metadataprocessor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:671</code> | <code>config :explorer, Explorer.Validator.MetadataProcessor [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-validator-metadataretriever</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:667</code> | <code>config :explorer, Explorer.Validator.MetadataRetriever [metadata_contract_address, validators_contract_address]</code> |
| <code>interface.14.cfg-explorer-explorer-visualize-sol2uml</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:773</code> | <code>config :explorer, Explorer.Visualize.Sol2uml [service_url, enabled]</code> |
| <code>interface.14.cfg-explorer-http-client</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:219</code> | <code>config :explorer, :http_client</code> |
| <code>interface.14.cfg-explorer-json-rpc-named-arguments</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/dev/anvil.exs:7</code> | <code>config :explorer, :json_rpc_named_arguments</code> |
| <code>interface.14.cfg-explorer-mode</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:325</code> | <code>config :explorer, :mode</code> |
| <code>interface.14.cfg-explorer-proxy</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:365</code> | <code>config :explorer, :proxy [caching_implementation_data_enabled, implementation_data_ttl_via_avg_block_time, fallback_cached_implementation_data_ttl, implementation_data_fetching_timeout]</code> |
| <code>interface.14.cfg-explorer-realtime-events-sender</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/runtime/test.exs:120</code> | <code>config :explorer, :realtime_events_sender</code> |
| <code>interface.14.cfg-explorer-solc-bin-api-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:216</code> | <code>config :explorer, :solc_bin_api_url</code> |
| <code>interface.14.cfg-explorer-spandex</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:854</code> | <code>config :explorer, :spandex [batch_size, sync_threshold]</code> |
| <code>interface.14.cfg-explorer-subscribe-named-arguments</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/dev/anvil.exs:7</code> | <code>config :explorer, :subscribe_named_arguments</code> |
| <code>interface.14.cfg-explorer-supply</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:682</code> | <code>config :explorer, :supply</code> |
| <code>interface.14.cfg-explorer-token-functions-reader-max-retries</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:13</code> | <code>config :explorer, :token_functions_reader_max_retries</code> |
| <code>interface.14.cfg-explorer-transport</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/test/besu.exs:3</code> | <code>config :explorer, :transport</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-explorer-air-table-audit-reports</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-celo</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-explorer-chain-identity</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-chain-type</code> | notes | set in 2 config statement(s); 127 reader(s); 88 further callers omitted; 3 more listed callers dropped for line length |
| <code>interface.14.cfg-explorer-coin</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-explorer-coin-name</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-datadog</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-ecto-repos</code> | notes | set in 2 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-account</code> | notes | set in 1 config statement(s); 29 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-blackfort-validator</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-block-reward</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-bridgedtoken</code> | notes | set in 2 config statement(s); 8 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-accounts</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-blocknumber</code> | notes | set in 3 config statement(s); 12 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-blocks</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-celocorecontracts</code> | notes | set in 1 config statement(s); 27 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-celoepochs</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescoinbalancesum</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescoinbalancesumminusburnt</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescount</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstabselementscount</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstokensusdsum</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstokentransferscount</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstransactionscount</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstransactionsgasusagesum</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-averageblocktime</code> | notes | set in 2 config statement(s); 36 reader(s); 2 more listed callers dropped for line length |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blackfort-validatorscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockburntfeecount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockpriorityfeecount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-contractscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-gasusagesum</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newcontractscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newpendingtransactionscount</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newverifiedcontractscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-optimism-lastoutputrootsizecount</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-pendingblockoperationcount</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-pendingtransactionoperationcount</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-rootstock-lockedbtccount</code> | notes | set in 2 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-stability-validatorscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-tokenholderscount</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-tokentransferscount</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-transactions24hcount</code> | notes | set in 2 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-transactionscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-verifiedcontractscount</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-withdrawalssum</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-gaspriceoracle</code> | notes | set in 1 config statement(s); 15 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-minmissingblocknumber</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-optimismfinalizationperiod</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactionactiontokensdata</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactionactionuniswappools</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactions</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-cache-uncles</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-events-listener</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-addressesblacklist</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-checkbytecodematchingondemand</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-fetchvalidatorinfoondemand</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-lookupsmartcontractsourcesondemand</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-filecoin-nativeaddress</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-health-monitor</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-metrics-publicmetrics</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-mud</code> | notes | set in 2 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-capabilities</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-finality</code> | notes | set in 1 config statement(s); 3 reader(s); keys cache_ms, probe_batches and final_confirmations have no reader in Explorer.Chain.PaxeerX.Finality |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-sequencerreceipt</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-scroll-l1feeparam</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-tokentransfer</code> | notes | set in 1 config statement(s); 16 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-transaction</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chain-transaction-history-historian</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-chainspec-genesisdata</code> | notes | set in 2 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-explorer-explorer-integrations-ectologger</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper); module not defined in the backend (library or removed module) |
| <code>interface.14.cfg-explorer-explorer-mailer</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-coin</code> | notes | set in 3 config statement(s); 22 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-history</code> | notes | set in 2 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-token</code> | notes | set in 3 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-history-cataloger</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper); module not defined in the backend (library or removed module) |
| <code>interface.14.cfg-explorer-explorer-market-markethistorycache</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source</code> | notes | set in 1 config statement(s); 41 reader(s); 1 further callers omitted |
| <code>interface.14.cfg-explorer-explorer-market-source-coingecko</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source-coinmarketcap</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source-cryptocompare</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source-cryptorank</code> | notes | set in 1 config statement(s); 8 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source-defillama</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source-dia</code> | notes | set in 1 config statement(s); 8 reader(s) |
| <code>interface.14.cfg-explorer-explorer-market-source-mobula</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-accountabstraction</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-bens</code> | notes | set in 1 config statement(s); 19 reader(s) |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-metadata</code> | notes | set in 1 config statement(s); 24 reader(s) |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-multichainsearch</code> | notes | set in 1 config statement(s); 106 reader(s); 66 further callers omitted; 5 more listed callers dropped for line length |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-tacoperationlifecycle</code> | notes | set in 1 config statement(s); 39 reader(s); 8 more listed callers dropped for line length |
| <code>interface.14.cfg-explorer-explorer-migrator-arbitrumdarecordsnormalization</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-backfillmetadataurl</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-backfillmultichainsearchdb</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-celoaccounts</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-celoaggregatedelectionrewards</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-celol2epochs</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-deletezerovalueinternaltransactions</code> | notes | set in 2 config statement(s); 11 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-emptyinternaltransactionsdata</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-filecoinpendingaddressoperations</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation-createarbitrumbatchl2blocksunconfirmedblocksindex</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation-createtransactionsoperatorfeeconstantindex</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-migrator-mergeadjacentmissingblockranges</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-refetchcontractcodes</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexblockswithmissingtransactions</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexduplicatedinternaltransactions</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexinternaltransactionswithincompatiblestatus</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-restoreomittedwethtransfers</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeduplicatedlogindexlogs</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeemptycontractcodeaddresses</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeincorrectnfttokentransfers</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeincorrectwethtokentransfers</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeverifiedaddresses</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-shrinkinternaltransactions</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-explorer-explorer-migrator-smartcontractlanguage</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-tokentransfertokentype</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-migrator-transactionsdenormalization</code> | notes | set in 1 config statement(s); 4 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-explorer-explorer-repo</code> | notes | set in 6 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-repo-account</code> | notes | set in 3 config statement(s); 1 reader(s); repo module is generated by the defmodule loop in the explorer repo module |
| <code>interface.14.cfg-explorer-explorer-repo-eventnotifications</code> | notes | set in 3 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-repo-mud</code> | notes | set in 2 config statement(s); 1 reader(s); repo module is generated by the defmodule loop in the explorer repo module |
| <code>interface.14.cfg-explorer-explorer-repo-polygonzkevm</code> | notes | set in 1 config statement(s); 1 reader(s); repo module is generated by the defmodule loop in the explorer repo module |
| <code>interface.14.cfg-explorer-explorer-repo-replica1</code> | notes | set in 3 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-repo-suave</code> | notes | set in 2 config statement(s); 1 reader(s); repo module is generated by the defmodule loop in the explorer repo module |
| <code>interface.14.cfg-explorer-explorer-smartcontract-certifiedsmartcontractcataloger</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-smartcontract-rustverifierinterfacebehaviour</code> | notes | set in 1 config statement(s); 32 reader(s) |
| <code>interface.14.cfg-explorer-explorer-smartcontract-sigproviderinterface</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-smartcontract-stylusverifierinterface</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-tags-addresstag-cataloger</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-dynamic</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-dynamic-strategy</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-keycloak</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-novesfi</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-solidityscan</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-sourcify</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-universalproxy</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-xname</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-tokeninstanceowneraddressmigration</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-tokeninstanceowneraddressmigration-supervisor</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-tracer</code> | notes | set in 4 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-utility-hammer-ets</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-utility-hammer-redis</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-utility-missingbalanceoftoken</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-explorer-utility-ratelimiter</code> | notes | set in 3 config statement(s); 14 reader(s) |
| <code>interface.14.cfg-explorer-explorer-utility-versionconstantsupdater</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-validator-metadataprocessor</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-explorer-explorer-validator-metadataretriever</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-explorer-visualize-sol2uml</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-http-client</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-explorer-json-rpc-named-arguments</code> | notes | set in 24 config statement(s); 59 reader(s); 42 further callers omitted |
| <code>interface.14.cfg-explorer-mode</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-explorer-proxy</code> | notes | set in 2 config statement(s); 36 reader(s); 3 more listed callers dropped for line length |
| <code>interface.14.cfg-explorer-realtime-events-sender</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-solc-bin-api-url</code> | notes | set in 1 config statement(s); 15 reader(s) |
| <code>interface.14.cfg-explorer-spandex</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-explorer-subscribe-named-arguments</code> | notes | set in 24 config statement(s); 3 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-explorer-supply</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-explorer-token-functions-reader-max-retries</code> | notes | set in 1 config statement(s); 8 reader(s) |
| <code>interface.14.cfg-explorer-transport</code> | notes | set in 4 config statement(s); 10 reader(s); reader matched by the atom outside an Application env call |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.264.1</code> | req | 14264.1 |
| <code>acceptance.14.264.1</code> | scenario | Given the runtime config after the change, when Application.get_env(:explorer, Explorer.Chain.PaxeerX.Finality) is read, then it is nil |
| <code>acceptance.14.264.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/config/paxeer_x_finality_config_test.exs::finality_config_is_not_set (planned) |
| <code>acceptance.14.264.1</code> | qualification | UNRUN |
| <code>req.14264</code> | ac_1 | The Finality config statement at services/explorer/backend/config/runtime.exs:656-659 is gone and the app env holds no Finality key. |
| <code>acceptance.14.299.1</code> | req | 14299.1 |
| <code>acceptance.14.299.1</code> | scenario | Given frontend.example.env, when the comment above the lists flag is read, then it names the kernel archive and no line mentions custody |
| <code>acceptance.14.299.1</code> | test | services/explorer/tests/test_frontend_env_example.py::test_frontend_env_names_kernel_archive (planned) |
| <code>acceptance.14.299.1</code> | qualification | UNRUN |
| <code>req.14299</code> | ac_1 | The frontend env example (services/explorer/deploy/env/frontend.example.env:45) describes the lists flag as the kernel archive and does not mention custody. |
| <code>capability.14.26.2</code> | title | Remove the unread PAXEER_X_FINALITY_* and PAXEER_X_FINAL_CONFIRMATIONS settings |
| <code>capability.14.26.2</code> | reqs | 14264.1 |
| <code>capability.14.37.1</code> | title | Describe the paxeer-x list feature in the frontend env example as the kernel archive |
| <code>capability.14.37.1</code> | reqs | 14299.1 |
| <code>resolution.17.949</code> | title | decision.1424 binds: Remove the unread finality settings instead of implementing a cache |
| <code>resolution.17.949</code> | applies_to | decision.1424<br>task.14.26.2 |
| <code>resolution.17.949</code> | source | decision.1424 in fragments/J54/decision.kvx; tasks from affects |

## cfg-indexer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-indexer-arc</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1270</code> | <code>config :indexer, :arc [arc_native_token_decimals]</code> |
| <code>interface.14.cfg-indexer-block-interval</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev/anvil.exs:7</code> | <code>config :indexer, :block_interval</code> |
| <code>interface.14.cfg-indexer-block-ranges</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :block_ranges</code> |
| <code>interface.14.cfg-indexer-block-transformer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :block_transformer</code> |
| <code>interface.14.cfg-indexer-blocks-concurrency</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev/rsk.exs:7</code> | <code>config :indexer, :blocks_concurrency</code> |
| <code>interface.14.cfg-indexer-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :chain_id</code> |
| <code>interface.14.cfg-indexer-ecto-repos</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/config.exs:5</code> | <code>config :indexer, :ecto_repos</code> |
| <code>interface.14.cfg-indexer-fetch-rewards-way</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :fetch_rewards_way</code> |
| <code>interface.14.cfg-indexer-fetcher-init-delay</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :fetcher_init_delay</code> |
| <code>interface.14.cfg-indexer-fetcher-init-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :fetcher_init_limit</code> |
| <code>interface.14.cfg-indexer-first-block</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :first_block</code> |
| <code>interface.14.cfg-indexer-hide-indexing-progress-alert</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :hide_indexing_progress_alert</code> |
| <code>interface.14.cfg-indexer-indexer-block-catchup-fetcher</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1467</code> | <code>config :indexer, Indexer.Block.Catchup.Fetcher [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-block-catchup-missingrangescollector</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1464</code> | <code>config :indexer, Indexer.Block.Catchup.MissingRangesCollector [batch_size, future_check_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-block-catchup-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1391</code> | <code>config :indexer, Indexer.Block.Catchup.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-block-realtime-fetcher</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1458</code> | <code>config :indexer, Indexer.Block.Realtime.Fetcher [max_gap, polling_period, batch_size, max_lag]</code> |
| <code>interface.14.cfg-indexer-indexer-block-realtime-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1388</code> | <code>config :indexer, Indexer.Block.Realtime.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1825</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum [l1_rpc, l1_rpc_chunk_size, l1_rpc_block_range, l1_rollup_address, l1_rollup_init_block, l1_start_block, l1_finalization_threshold, rollup_chunk_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-databackfill</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1885</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.DataBackfill</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-databackfill-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1893</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.DataBackfill.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-messagestol2matcher-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1882</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.MessagesToL2Matcher.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-messaging</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1818</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.Messaging</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-rollupmessagescatchup</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1874</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.RollupMessagesCatchup [recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-rollupmessagescatchup-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1879</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.RollupMessagesCatchup.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingbatchesstatuses</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1848</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingBatchesStatuses [recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingbatchesstatuses-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1871</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingBatchesStatuses.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingmessagesonl1</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1835</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingMessagesOnL1</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingmessagesonl1-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1845</code> | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingMessagesOnL1.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1909</code> | <code>config :indexer, Indexer.Fetcher.Beacon [beacon_rpc]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-blob</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1965</code> | <code>config :indexer, Indexer.Fetcher.Beacon.Blob [slot_duration, reference_slot, start_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-blob-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1960</code> | <code>config :indexer, Indexer.Fetcher.Beacon.Blob.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1981</code> | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit [interval, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-status</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1990</code> | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit.Status</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-status-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1985</code> | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit.Status.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1976</code> | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-blockreward</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1471</code> | <code>config :indexer, Indexer.Fetcher.BlockReward [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-blockreward-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1361</code> | <code>config :indexer, Indexer.Fetcher.BlockReward.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-epochblockoperations-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2066</code> | <code>config :indexer, Indexer.Fetcher.Celo.EpochBlockOperations.Supervisor [enabled, disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-legacy-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2070</code> | <code>config :indexer, Indexer.Fetcher.Celo.Legacy.Account [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-legacy-account-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2074</code> | <code>config :indexer, Indexer.Fetcher.Celo.Legacy.Account.Supervisor [enabled, disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-validatorgroupvotes</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2054</code> | <code>config :indexer, Indexer.Fetcher.Celo.ValidatorGroupVotes [batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-validatorgroupvotes-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2057</code> | <code>config :indexer, Indexer.Fetcher.Celo.ValidatorGroupVotes.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-catchup</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1556</code> | <code>config :indexer, Indexer.Fetcher.CoinBalance.Catchup [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-catchup-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1372</code> | <code>config :indexer, Indexer.Fetcher.CoinBalance.Catchup.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-realtime</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1560</code> | <code>config :indexer, Indexer.Fetcher.CoinBalance.Realtime [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-realtime-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1374</code> | <code>config :indexer, Indexer.Fetcher.CoinBalance.Realtime.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-emptyblockssanitizer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1453</code> | <code>config :indexer, Indexer.Fetcher.EmptyBlocksSanitizer [batch_size, interval, head_offset]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-emptyblockssanitizer-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1385</code> | <code>config :indexer, Indexer.Fetcher.EmptyBlocksSanitizer.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-addressinfo</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2094</code> | <code>config :indexer, Indexer.Fetcher.Filecoin.AddressInfo [concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-addressinfo-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2089</code> | <code>config :indexer, Indexer.Fetcher.Filecoin.AddressInfo.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-beryxapi</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2078</code> | <code>config :indexer, Indexer.Fetcher.Filecoin.BeryxAPI</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-filfoxapi</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2086</code> | <code>config :indexer, Indexer.Fetcher.Filecoin.FilfoxAPI [base_url]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1527</code> | <code>config :indexer, Indexer.Fetcher.InternalTransaction [batch_size, concurrency, lookback_blocks]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction-deletequeue</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1537</code> | <code>config :indexer, Indexer.Fetcher.InternalTransaction.DeleteQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1364</code> | <code>config :indexer, Indexer.Fetcher.InternalTransaction.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-balancesexportqueue</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1606</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-balancesexportqueue-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1427</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersexportqueue</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1660</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.CountersExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersexportqueue-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1437</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.CountersExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersfetcher-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1443</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.CountersFetcher.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-mainexportqueue</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1579</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.MainExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-mainexportqueue-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1422</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.MainExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-tokeninfoexportqueue</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1633</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-tokeninfoexportqueue-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1432</code> | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-coinbalance</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1347</code> | <code>config :indexer, Indexer.Fetcher.OnDemand.CoinBalance [threshold, fallback_threshold_in_blocks]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-coinbalance-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/runtime/test.exs:7</code> | <code>config :indexer, Indexer.Fetcher.OnDemand.CoinBalance.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-contractcode</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1351</code> | <code>config :indexer, Indexer.Fetcher.OnDemand.ContractCode [threshold]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokenbalance</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1343</code> | <code>config :indexer, Indexer.Fetcher.OnDemand.TokenBalance [threshold, fallback_threshold_in_blocks]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokenbalance-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/runtime/test.exs:8</code> | <code>config :indexer, Indexer.Fetcher.OnDemand.TokenBalance.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokeninstancemetadatarefetch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1354</code> | <code>config :indexer, Indexer.Fetcher.OnDemand.TokenInstanceMetadataRefetch</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1721</code> | <code>config :indexer, Indexer.Fetcher.Optimism [optimism_l1_rpc, optimism_l1_system_config, l1_eth_get_logs_range_size, l2_eth_get_logs_range_size, block_duration, start_block_l1, portal, isthmus_timestamp_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-deposit</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1731</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Deposit [transaction_type]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-deposit-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1696</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Deposit.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-disputegame-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1694</code> | <code>config :indexer, Indexer.Fetcher.Optimism.DisputeGame.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-eip1559configupdate</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1756</code> | <code>config :indexer, Indexer.Fetcher.Optimism.EIP1559ConfigUpdate [chunk_size, holocene_timestamp_l2, jovian_timestamp_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-eip1559configupdate-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1702</code> | <code>config :indexer, Indexer.Fetcher.Optimism.EIP1559ConfigUpdate.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-message</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1761</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.Message [start_block, blocks_chunk_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-message-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1705</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.Message.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagefailed-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1707</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MessageFailed.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagequeue</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1765</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MessageQueue [chainscout_api_url, chainscout_fallback_map, private_key, connect_timeout, recv_timeout, export_expiration]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagequeue-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1710</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MessageQueue.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-multichainexport</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1773</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MultichainExport [batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-multichainexport-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1713</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MultichainExport.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-operatorfee</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1776</code> | <code>config :indexer, Indexer.Fetcher.Optimism.OperatorFee [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-operatorfee-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1795</code> | <code>config :indexer, Indexer.Fetcher.Optimism.OperatorFee.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-outputroot</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1734</code> | <code>config :indexer, Indexer.Fetcher.Optimism.OutputRoot [output_oracle]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-outputroot-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1692</code> | <code>config :indexer, Indexer.Fetcher.Optimism.OutputRoot.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-transactionbatch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1745</code> | <code>config :indexer, Indexer.Fetcher.Optimism.TransactionBatch [blocks_chunk_size, eip4844_blobs_api_url, celestia_blobs_api_url, eigenda_blobs_api_url, eigenda_proxy_base_url, alt_da_server_url, genesis_block_l2, inbox, submitter]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-transactionbatch-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1690</code> | <code>config :indexer, Indexer.Fetcher.Optimism.TransactionBatch.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawal</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1737</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Withdrawal [start_block_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawal-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1698</code> | <code>config :indexer, Indexer.Fetcher.Optimism.Withdrawal.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawalevent-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1700</code> | <code>config :indexer, Indexer.Fetcher.Optimism.WithdrawalEvent.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-pricefeedmaintenance</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1956</code> | <code>config :indexer, Indexer.Fetcher.PaxeerX.PriceFeedMaintenance [enabled, retention_days]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-priceupdates</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1939</code> | <code>config :indexer, Indexer.Fetcher.PaxeerX.PriceUpdates [cosmos_rest_urls, cometbft_rpc_urls, start_block, concurrency, markets_refresh_seconds]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-priceupdates-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1937</code> | <code>config :indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-pendingtransaction-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1314</code> | <code>config :indexer, Indexer.Fetcher.PendingTransaction.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2022</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL1 [rpc, start_block, bridge_contract, native_symbol, native_decimals, rollup_network_id_l1, rollup_index_l1]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2031</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL1.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1tokens-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2033</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL1Tokens.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel2</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2036</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL2 [start_block, bridge_contract, rollup_network_id_l2, rollup_index_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel2-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2042</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL2.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-transactionbatch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2044</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.TransactionBatch [chunk_size, ignore_numbers, recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-transactionbatch-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2049</code> | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.TransactionBatch.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-replacedtransaction-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1393</code> | <code>config :indexer, Indexer.Fetcher.ReplacedTransaction.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-rootstockdata</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1903</code> | <code>config :indexer, Indexer.Fetcher.RootstockData [interval, batch_size, max_concurrency, db_batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-rootstockdata-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1898</code> | <code>config :indexer, Indexer.Fetcher.RootstockData.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2097</code> | <code>config :indexer, Indexer.Fetcher.Scroll [l1_eth_get_logs_range_size, l2_eth_get_logs_range_size, rpc]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-batch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2114</code> | <code>config :indexer, Indexer.Fetcher.Scroll.Batch [scroll_chain_contract, start_block, eip4844_blobs_api_url]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-batch-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2123</code> | <code>config :indexer, Indexer.Fetcher.Scroll.Batch.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel1</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2106</code> | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL1 [messenger_contract, start_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel1-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2119</code> | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL1.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel2</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2110</code> | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL2 [messenger_contract, start_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel2-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2121</code> | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL2.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-l1feeparam</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2102</code> | <code>config :indexer, Indexer.Fetcher.Scroll.L1FeeParam [gas_oracle]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-l1feeparam-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2104</code> | <code>config :indexer, Indexer.Fetcher.Scroll.L1FeeParam.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l1</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2002</code> | <code>config :indexer, Indexer.Fetcher.Shibarium.L1 [rpc, start_block, deposit_manager_proxy, ether_predicate_proxy, erc20_predicate_proxy, erc721_predicate_proxy, erc1155_predicate_proxy, withdraw_manager_proxy]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l1-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2018</code> | <code>config :indexer, Indexer.Fetcher.Shibarium.L1.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l2</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2012</code> | <code>config :indexer, Indexer.Fetcher.Shibarium.L2 [start_block, child_chain, weth, bone_withdraw]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l2-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2020</code> | <code>config :indexer, Indexer.Fetcher.Shibarium.L2.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-signedauthorizationstatus</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1687</code> | <code>config :indexer, Indexer.Fetcher.SignedAuthorizationStatus [batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-stats-hotsmartcontracts-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1449</code> | <code>config :indexer, Indexer.Fetcher.Stats.HotSmartContracts.Supervisor [disabled?, enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-token</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1317</code> | <code>config :indexer, Indexer.Fetcher.Token [concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-current</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1336</code> | <code>config :indexer, Indexer.Fetcher.TokenBalance.Current [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-historical</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1319</code> | <code>config :indexer, Indexer.Fetcher.TokenBalance.Historical [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-historical-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1333</code> | <code>config :indexer, Indexer.Fetcher.TokenBalance.Historical.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokencountersupdater</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1340</code> | <code>config :indexer, Indexer.Fetcher.TokenCountersUpdater [milliseconds_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokencountersupdater-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1382</code> | <code>config :indexer, Indexer.Fetcher.TokenCountersUpdater.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-helper</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1475</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Helper [base_uri_retry?, cidr_blacklist, host_filtering_enabled?, allowed_uri_protocols]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-realtime</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1492</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Realtime [concurrency, batch_size, retry_with_cooldown?, retry_timeout]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-realtime-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1396</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Realtime.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-refetch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1502</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Refetch [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-refetch-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1405</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Refetch.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-retry</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1481</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Retry [concurrency, batch_size, max_refetch_interval, exp_timeout_base]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-retry-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1399</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Retry.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitize</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1498</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Sanitize [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitize-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1402</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.Sanitize.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitizeerc1155</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1408</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.SanitizeERC1155</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitizeerc721</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1415</code> | <code>config :indexer, Indexer.Fetcher.TokenInstance.SanitizeERC721 [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokentotalsupplyupdater-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1379</code> | <code>config :indexer, Indexer.Fetcher.TokenTotalSupplyUpdater.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenupdater-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1376</code> | <code>config :indexer, Indexer.Fetcher.TokenUpdater.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-transactionaction</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1288</code> | <code>config :indexer, Indexer.Fetcher.TransactionAction [reindex_first_block, reindex_last_block, reindex_protocols, aave_v3_pool]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-transactionaction-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1285</code> | <code>config :indexer, Indexer.Fetcher.TransactionAction.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-withdrawal</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1801</code> | <code>config :indexer, Indexer.Fetcher.Withdrawal [first_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-withdrawal-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1798</code> | <code>config :indexer, Indexer.Fetcher.Withdrawal.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zilliqa-scillasmartcontracts-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2205</code> | <code>config :indexer, Indexer.Fetcher.Zilliqa.ScillaSmartContracts.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zilliqa-zrc2tokens-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2208</code> | <code>config :indexer, Indexer.Fetcher.Zilliqa.Zrc2Tokens.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-batchesstatustracker</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1811</code> | <code>config :indexer, Indexer.Fetcher.ZkSync.BatchesStatusTracker [zksync_l1_rpc, recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-batchesstatustracker-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1815</code> | <code>config :indexer, Indexer.Fetcher.ZkSync.BatchesStatusTracker.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-transactionbatch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1803</code> | <code>config :indexer, Indexer.Fetcher.ZkSync.TransactionBatch [chunk_size, batches_max_range, recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-transactionbatch-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1808</code> | <code>config :indexer, Indexer.Fetcher.ZkSync.TransactionBatch.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-migrator-recoverywethtokentransfers</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1564</code> | <code>config :indexer, Indexer.Migrator.RecoveryWETHTokenTransfers [concurrency, batch_size, timeout, enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-pendingtransactionssanitizer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1304</code> | <code>config :indexer, Indexer.PendingTransactionsSanitizer [interval]</code> |
| <code>interface.14.cfg-indexer-indexer-prometheus-metrics</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2132</code> | <code>config :indexer, Indexer.Prometheus.Metrics</code> |
| <code>interface.14.cfg-indexer-indexer-supervisor</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1283</code> | <code>config :indexer, Indexer.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-tokentransferblockconsensussanitizer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1307</code> | <code>config :indexer, Indexer.TokenTransferBlockConsensusSanitizer</code> |
| <code>interface.14.cfg-indexer-indexer-tracer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/config.exs:8</code> | <code>config :indexer, Indexer.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-utils-eventnotificationscleaner</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2125</code> | <code>config :indexer, Indexer.Utils.EventNotificationsCleaner [interval]</code> |
| <code>interface.14.cfg-indexer-ipfs</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1262</code> | <code>config :indexer, :ipfs [gateway_url, gateway_url_param_key, gateway_url_param_value]</code> |
| <code>interface.14.cfg-indexer-json-rpc-named-arguments</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev/anvil.exs:7</code> | <code>config :indexer, :json_rpc_named_arguments</code> |
| <code>interface.14.cfg-indexer-last-block</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :last_block</code> |
| <code>interface.14.cfg-indexer-memory-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :memory_limit</code> |
| <code>interface.14.cfg-indexer-metadata-updater-milliseconds-interval</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :metadata_updater_milliseconds_interval</code> |
| <code>interface.14.cfg-indexer-receipts-batch-size</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :receipts_batch_size</code> |
| <code>interface.14.cfg-indexer-receipts-concurrency</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :receipts_concurrency</code> |
| <code>interface.14.cfg-indexer-subscribe-named-arguments</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev/anvil.exs:7</code> | <code>config :indexer, :subscribe_named_arguments</code> |
| <code>interface.14.cfg-indexer-system-memory-percentage</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :system_memory_percentage</code> |
| <code>interface.14.cfg-indexer-trace-block-ranges</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :trace_block_ranges</code> |
| <code>interface.14.cfg-indexer-trace-first-block</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :trace_first_block</code> |
| <code>interface.14.cfg-indexer-trace-last-block</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1231</code> | <code>config :indexer, :trace_last_block</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-indexer-arc</code> | notes | set in 1 config statement(s); 12 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-indexer-block-interval</code> | notes | set in 16 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-indexer-block-ranges</code> | notes | set in 1 config statement(s); 36 reader(s) |
| <code>interface.14.cfg-indexer-block-transformer</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-blocks-concurrency</code> | notes | set in 2 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-indexer-chain-id</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-ecto-repos</code> | notes | set in 1 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-indexer-fetch-rewards-way</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-fetcher-init-delay</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-fetcher-init-limit</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-first-block</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-hide-indexing-progress-alert</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-block-catchup-fetcher</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-block-catchup-missingrangescollector</code> | notes | set in 3 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-block-catchup-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-indexer-indexer-block-realtime-fetcher</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-block-realtime-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-databackfill</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-databackfill-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-messagestol2matcher-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-messaging</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-rollupmessagescatchup</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-rollupmessagescatchup-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingbatchesstatuses</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingbatchesstatuses-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingmessagesonl1</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingmessagesonl1-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-blob</code> | notes | set in 2 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-blob-supervisor</code> | notes | set in 2 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-status</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-status-supervisor</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-supervisor</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-blockreward</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-blockreward-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-epochblockoperations-supervisor</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-legacy-account</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-legacy-account-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-validatorgroupvotes</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-validatorgroupvotes-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-catchup</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-catchup-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-realtime</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-realtime-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-emptyblockssanitizer</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-emptyblockssanitizer-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-addressinfo</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-addressinfo-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-beryxapi</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-filfoxapi</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction-deletequeue</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction-supervisor</code> | notes | set in 1 config statement(s); 17 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-balancesexportqueue</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-balancesexportqueue-supervisor</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersexportqueue</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersexportqueue-supervisor</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersfetcher-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-mainexportqueue</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-mainexportqueue-supervisor</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-tokeninfoexportqueue</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-tokeninfoexportqueue-supervisor</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-coinbalance</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-coinbalance-supervisor</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-contractcode</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokenbalance</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokenbalance-supervisor</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokeninstancemetadatarefetch</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-deposit</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-deposit-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-disputegame-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-eip1559configupdate</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-eip1559configupdate-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-message</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-message-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagefailed-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagequeue</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagequeue-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-multichainexport</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-multichainexport-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-operatorfee</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-operatorfee-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-outputroot</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-outputroot-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-transactionbatch</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-transactionbatch-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawal</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawal-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawalevent-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-pricefeedmaintenance</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-priceupdates</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-priceupdates-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-pendingtransaction-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1</code> | notes | set in 1 config statement(s); 6 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1tokens-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel2</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel2-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-transactionbatch</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-transactionbatch-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-replacedtransaction-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-rootstockdata</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-rootstockdata-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-batch</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-batch-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel1</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel1-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel2</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel2-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-l1feeparam</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-l1feeparam-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l1</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l1-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l2</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l2-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-signedauthorizationstatus</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-stats-hotsmartcontracts-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-token</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-current</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-historical</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-historical-supervisor</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokencountersupdater</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokencountersupdater-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-helper</code> | notes | set in 2 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-realtime</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-realtime-supervisor</code> | notes | set in 1 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-refetch</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-refetch-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-retry</code> | notes | set in 1 config statement(s); 8 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-retry-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitize</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitize-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitizeerc1155</code> | notes | set in 2 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitizeerc721</code> | notes | set in 2 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokentotalsupplyupdater-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenupdater-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-transactionaction</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-transactionaction-supervisor</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-withdrawal</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-withdrawal-supervisor</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-zilliqa-scillasmartcontracts-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-zilliqa-zrc2tokens-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-batchesstatustracker</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-batchesstatustracker-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-transactionbatch</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-transactionbatch-supervisor</code> | notes | set in 1 config statement(s); 2 reader(s); read via __MODULE__ in a used macro module |
| <code>interface.14.cfg-indexer-indexer-migrator-recoverywethtokentransfers</code> | notes | set in 3 config statement(s); 7 reader(s) |
| <code>interface.14.cfg-indexer-indexer-pendingtransactionssanitizer</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-prometheus-metrics</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-indexer-supervisor</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-indexer-indexer-tokentransferblockconsensussanitizer</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-indexer-tracer</code> | notes | set in 4 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-indexer-indexer-utils-eventnotificationscleaner</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-indexer-ipfs</code> | notes | set in 1 config statement(s); 22 reader(s) |
| <code>interface.14.cfg-indexer-json-rpc-named-arguments</code> | notes | set in 24 config statement(s); 14 reader(s) |
| <code>interface.14.cfg-indexer-last-block</code> | notes | set in 1 config statement(s); 5 reader(s) |
| <code>interface.14.cfg-indexer-memory-limit</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-metadata-updater-milliseconds-interval</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-receipts-batch-size</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-indexer-receipts-concurrency</code> | notes | set in 3 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-subscribe-named-arguments</code> | notes | set in 16 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-system-memory-percentage</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-indexer-trace-block-ranges</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-indexer-trace-first-block</code> | notes | set in 1 config statement(s); 14 reader(s) |
| <code>interface.14.cfg-indexer-trace-last-block</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-libcluster

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-libcluster-topologies</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2210</code> | <code>config :libcluster, :topologies</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-libcluster-topologies</code> | notes | set in 1 config statement(s); 1 reader(s) |

## cfg-logger

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-logger-account</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/dev.exs:7</code> | <code>config :logger, :account [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-addresses-without-code</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:15</code> | <code>config :logger, :addresses_without_code [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-api</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:21</code> | <code>config :logger, :api [format, metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-api-v2</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:26</code> | <code>config :logger, :api_v2 [format, metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-backends</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:7</code> | <code>config :logger, :backends</code> |
| <code>interface.14.cfg-logger-block-import-timings</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:27</code> | <code>config :logger, :block_import_timings [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-console</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/config.exs:18</code> | <code>config :logger, :console [metadata]</code> |
| <code>interface.14.cfg-logger-default-formatter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/config.exs:16</code> | <code>config :logger, :default_formatter [format]</code> |
| <code>interface.14.cfg-logger-default-handler</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:10</code> | <code>config :logger, :default_handler</code> |
| <code>interface.14.cfg-logger-empty-blocks-to-refetch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:23</code> | <code>config :logger, :empty_blocks_to_refetch [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-error</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/config.exs:20</code> | <code>config :logger, :error [metadata, path, level, rotate]</code> |
| <code>interface.14.cfg-logger-ethereum-jsonrpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs:24</code> | <code>config :logger, :ethereum_jsonrpc [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-explorer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:223</code> | <code>config :logger, :explorer [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-failed-contract-creations</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:11</code> | <code>config :logger, :failed_contract_creations [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-indexer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/config.exs:17</code> | <code>config :logger, :indexer [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-indexer-token-balances</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:7</code> | <code>config :logger, :indexer_token_balances [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-level</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/dev.exs:3</code> | <code>config :logger, :level</code> |
| <code>interface.14.cfg-logger-pending-transactions-to-refetch</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:19</code> | <code>config :logger, :pending_transactions_to_refetch [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-reading-token-functions</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/dev.exs:44</code> | <code>config :logger, :reading_token_functions [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-token-instances</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/dev.exs:48</code> | <code>config :logger, :token_instances [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-web</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/config.exs:22</code> | <code>config :logger, :block_scout_web [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-withdrawal</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev.exs:31</code> | <code>config :logger, :withdrawal [path, metadata_filter, rotate]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-logger-account</code> | notes | set in 2 config statement(s); 20 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-addresses-without-code</code> | notes | set in 3 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-api</code> | notes | set in 3 config statement(s); 22 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-api-v2</code> | notes | set in 3 config statement(s); 34 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-backends</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-block-import-timings</code> | notes | set in 2 config statement(s); 1 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-console</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-default-formatter</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-default-handler</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-empty-blocks-to-refetch</code> | notes | set in 2 config statement(s); 7 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-error</code> | notes | set in 4 config statement(s); 40 reader(s); reader matched by the atom outside an Application env call; 3 further callers omitted |
| <code>interface.14.cfg-logger-ethereum-jsonrpc</code> | notes | set in 4 config statement(s); 40 reader(s); reader matched by the atom outside an Application env call; 3 further callers omitted |
| <code>interface.14.cfg-logger-explorer</code> | notes | set in 4 config statement(s); 40 reader(s); reader matched by the atom outside an Application env call; 3 further callers omitted |
| <code>interface.14.cfg-logger-failed-contract-creations</code> | notes | set in 3 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-indexer</code> | notes | set in 4 config statement(s); 40 reader(s); reader matched by the atom outside an Application env call; 3 further callers omitted |
| <code>interface.14.cfg-logger-indexer-token-balances</code> | notes | set in 3 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-level</code> | notes | set in 3 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-pending-transactions-to-refetch</code> | notes | set in 2 config statement(s); 10 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-reading-token-functions</code> | notes | set in 2 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-logger-token-instances</code> | notes | set in 2 config statement(s); 20 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-logger-web</code> | notes | set in 5 config statement(s); 40 reader(s); reader matched by the atom outside an Application env call; 4 further callers omitted |
| <code>interface.14.cfg-logger-withdrawal</code> | notes | set in 2 config statement(s); 36 reader(s); reader matched by the atom outside an Application env call |

## cfg-nft-media-handler

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-nft-media-handler-cache-uniqueness-max-size</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :cache_uniqueness_max_size</code> |
| <code>interface.14.cfg-nft-media-handler-cache-uniqueness-name</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :cache_uniqueness_name</code> |
| <code>interface.14.cfg-nft-media-handler-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :enabled?</code> |
| <code>interface.14.cfg-nft-media-handler-indexer-nftmediahandler-backfiller</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2196</code> | <code>config :nft_media_handler, Indexer.NFTMediaHandler.Backfiller [enabled?, queue_size]</code> |
| <code>interface.14.cfg-nft-media-handler-r2-folder</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :r2_folder</code> |
| <code>interface.14.cfg-nft-media-handler-remote</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :remote?</code> |
| <code>interface.14.cfg-nft-media-handler-standalone-media-worker</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :standalone_media_worker?</code> |
| <code>interface.14.cfg-nft-media-handler-tmp-dir</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :tmp_dir</code> |
| <code>interface.14.cfg-nft-media-handler-worker</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :worker?</code> |
| <code>interface.14.cfg-nft-media-handler-worker-batch-size</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :worker_batch_size</code> |
| <code>interface.14.cfg-nft-media-handler-worker-concurrency</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :worker_concurrency</code> |
| <code>interface.14.cfg-nft-media-handler-worker-spawn-tasks-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2183</code> | <code>config :nft_media_handler, :worker_spawn_tasks_timeout</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-nft-media-handler-cache-uniqueness-max-size</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-nft-media-handler-cache-uniqueness-name</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-nft-media-handler-enabled</code> | notes | set in 1 config statement(s); 5 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-nft-media-handler-indexer-nftmediahandler-backfiller</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-nft-media-handler-r2-folder</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-nft-media-handler-remote</code> | notes | set in 1 config statement(s); 4 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-nft-media-handler-standalone-media-worker</code> | notes | set in 1 config statement(s); 9 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-nft-media-handler-tmp-dir</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-nft-media-handler-worker</code> | notes | set in 1 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-nft-media-handler-worker-batch-size</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-nft-media-handler-worker-concurrency</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-nft-media-handler-worker-spawn-tasks-timeout</code> | notes | set in 1 config statement(s); 1 reader(s) |

## cfg-oauth2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-oauth2-adapter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:93</code> | <code>config :oauth2, :adapter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-oauth2-adapter</code> | notes | set in 2 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-os-mon

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-os-mon-start-cpu-sup</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/config.exs:21</code> | <code>config :os_mon, :start_cpu_sup</code> |
| <code>interface.14.cfg-os-mon-start-disksup</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/config.exs:21</code> | <code>config :os_mon, :start_disksup</code> |
| <code>interface.14.cfg-os-mon-start-memsup</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/config.exs:21</code> | <code>config :os_mon, :start_memsup</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-os-mon-start-cpu-sup</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-os-mon-start-disksup</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-os-mon-start-memsup</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-phoenix

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-phoenix-json-library</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/config.exs:14</code> | <code>config :phoenix, :json_library</code> |
| <code>interface.14.cfg-phoenix-stacktrace-depth</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/dev.exs:65</code> | <code>config :phoenix, :stacktrace_depth</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-phoenix-json-library</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-phoenix-stacktrace-depth</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-prometheus

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-prometheus-web-prometheus-publicexporter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:60</code> | <code>config :prometheus, BlockScoutWeb.Prometheus.PublicExporter [path, format, registry, auth]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-prometheus-web-prometheus-publicexporter</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |

## cfg-spandex-ecto

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-spandex-ecto-spandexecto-ectologger</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:227</code> | <code>config :spandex_ecto, SpandexEcto.EctoLogger [service, tracer, otp_app]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-spandex-ecto-spandexecto-ectologger</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper); module not defined in the backend (library or removed module) |

## cfg-spandex-phoenix

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-spandex-phoenix-tracer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:66</code> | <code>config :spandex_phoenix, :tracer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-spandex-phoenix-tracer</code> | notes | set in 1 config statement(s); 3 reader(s); reader matched by the atom outside an Application env call |

## cfg-tesla

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-tesla-adapter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:95</code> | <code>config :tesla, :adapter</code> |
| <code>interface.14.cfg-tesla-disable-deprecated-builder-warning</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/config.exs:25</code> | <code>config :tesla, :disable_deprecated_builder_warning</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-tesla-adapter</code> | notes | set in 6 config statement(s); 135 reader(s); 100 further callers omitted; 9 more listed callers dropped for line length |
| <code>interface.14.cfg-tesla-disable-deprecated-builder-warning</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-ueberauth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-ueberauth-ueberauth</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:253</code> | <code>config :ueberauth, Ueberauth [logout_url, providers]</code> |
| <code>interface.14.cfg-ueberauth-ueberauth-strategy-auth0-oauth</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:246</code> | <code>config :ueberauth, Ueberauth.Strategy.Auth0.OAuth [domain, client_id, client_secret, auth0_application_id]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-ueberauth-ueberauth</code> | notes | set in 4 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-ueberauth-ueberauth-strategy-auth0-oauth</code> | notes | set in 2 config statement(s); 12 reader(s) |

## cfg-wallaby

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-wallaby-driver</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/test.exs:19</code> | <code>config :wallaby, :driver</code> |
| <code>interface.14.cfg-wallaby-js-errors</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/test.exs:19</code> | <code>config :wallaby, :js_errors</code> |
| <code>interface.14.cfg-wallaby-screenshot-on-failure</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/test.exs:19</code> | <code>config :wallaby, :screenshot_on_failure</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-wallaby-driver</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-wallaby-js-errors</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |
| <code>interface.14.cfg-wallaby-screenshot-on-failure</code> | notes | set in 1 config statement(s); 0 reader(s); no direct Application env reader matched (read by the library or via helper) |

## cfg-web

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-web-admin-panel-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:24</code> | <code>config :block_scout_web, :admin_panel_enabled</code> |
| <code>interface.14.cfg-web-api-graphql</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:194</code> | <code>config :block_scout_web, Api.GraphQL [enabled, token_limit, max_complexity]</code> |
| <code>interface.14.cfg-web-api-rate-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:149</code> | <code>config :block_scout_web, :api_rate_limit [disabled, static_api_key_value, static_api_key, whitelisted_ip, ip, temporary_token, account_api_key, no_rate_limit_api_key_value, whitelisted_ips, api_v2_token_ttl, eth_json_rpc_max_batch_size, redis_url]</code> |
| <code>interface.14.cfg-web-api-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :api_url</code> |
| <code>interface.14.cfg-web-apps</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :apps</code> |
| <code>interface.14.cfg-web-apps-menu</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :apps_menu</code> |
| <code>interface.14.cfg-web-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :chain_id</code> |
| <code>interface.14.cfg-web-chart</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:232</code> | <code>config :block_scout_web, :chart [chart_config, price_chart_legend_enabled?]</code> |
| <code>interface.14.cfg-web-checksum-address-hashes</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :checksum_address_hashes</code> |
| <code>interface.14.cfg-web-circles-addresses</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :circles_addresses</code> |
| <code>interface.14.cfg-web-contract</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:138</code> | <code>config :block_scout_web, :contract [verification_max_libraries, max_length_to_show_string_without_trimming, disable_interaction, certified_list, partial_reverification_disabled]</code> |
| <code>interface.14.cfg-web-cookie-domain</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:13</code> | <code>config :block_scout_web, :cookie_domain</code> |
| <code>interface.14.cfg-web-dark-forest-addresses</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :dark_forest_addresses</code> |
| <code>interface.14.cfg-web-dark-forest-addresses-v-0-5</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :dark_forest_addresses_v_0_5</code> |
| <code>interface.14.cfg-web-disable-add-to-mm-button</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :disable_add_to_mm_button</code> |
| <code>interface.14.cfg-web-disable-api</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :disable_api?</code> |
| <code>interface.14.cfg-web-display-token-icons</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :display_token_icons</code> |
| <code>interface.14.cfg-web-ecto-repos</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:13</code> | <code>config :block_scout_web, :ecto_repos</code> |
| <code>interface.14.cfg-web-footer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:127</code> | <code>config :block_scout_web, :footer [logo, chat_link, github_link, forum_link_enabled, forum_link, telegram_link_enabled, telegram_link, link_to_other_explorers, other_explorers]</code> |
| <code>interface.14.cfg-web-gas-price</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :gas_price</code> |
| <code>interface.14.cfg-web-hide-block-miner</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :hide_block_miner</code> |
| <code>interface.14.cfg-web-hide-scam-addresses</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :hide_scam_addresses</code> |
| <code>interface.14.cfg-web-json-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :json_rpc</code> |
| <code>interface.14.cfg-web-namespace</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:13</code> | <code>config :block_scout_web, :namespace</code> |
| <code>interface.14.cfg-web-new-tags</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :new_tags</code> |
| <code>interface.14.cfg-web-other-networks</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :other_networks</code> |
| <code>interface.14.cfg-web-permanent-dark-mode-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :permanent_dark_mode_enabled</code> |
| <code>interface.14.cfg-web-permanent-light-mode-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :permanent_light_mode_enabled</code> |
| <code>interface.14.cfg-web-recaptcha</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:69</code> | <code>config :block_scout_web, :recaptcha [v2_client_key, v2_secret_key, v3_client_key, v3_secret_key, is_disabled, check_hostname?, score_threshold, bypass_token, scoped_bypass_tokens]</code> |
| <code>interface.14.cfg-web-release-link</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :release_link</code> |
| <code>interface.14.cfg-web-remote-ip</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:187</code> | <code>config :block_scout_web, :remote_ip [is_blockscout_behind_proxy, headers, proxies]</code> |
| <code>interface.14.cfg-web-sensitive-endpoints-api-key</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :sensitive_endpoints_api_key</code> |
| <code>interface.14.cfg-web-show-percentage</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :show_percentage</code> |
| <code>interface.14.cfg-web-show-tenderly-link</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :show_tenderly_link</code> |
| <code>interface.14.cfg-web-sql-sandbox</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/test.exs:3</code> | <code>config :block_scout_web, :sql_sandbox</code> |
| <code>interface.14.cfg-web-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :version</code> |
| <code>interface.14.cfg-web-web-api-v2</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:239</code> | <code>config :block_scout_web, BlockScoutWeb.API.V2 [enabled]</code> |
| <code>interface.14.cfg-web-web-chain</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:116</code> | <code>config :block_scout_web, BlockScoutWeb.Chain [network, subnetwork, network_icon, logo, logo_text, has_emission_funds, show_maintenance_alert, enable_testnet_label, testnet_label_text]</code> |
| <code>interface.14.cfg-web-web-chain-address-coinbalance</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:236</code> | <code>config :block_scout_web, BlockScoutWeb.Chain.Address.CoinBalance [coin_balance_history_days]</code> |
| <code>interface.14.cfg-web-web-chain-transactionhistorychartcontroller</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:48</code> | <code>config :block_scout_web, BlockScoutWeb.Chain.TransactionHistoryChartController</code> |
| <code>interface.14.cfg-web-web-counters-blocksindexedcounter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:30</code> | <code>config :block_scout_web, BlockScoutWeb.Counters.BlocksIndexedCounter [enabled]</code> |
| <code>interface.14.cfg-web-web-counters-internaltransactionsindexedcounter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:32</code> | <code>config :block_scout_web, BlockScoutWeb.Counters.InternalTransactionsIndexedCounter [enabled]</code> |
| <code>interface.14.cfg-web-web-cspheader</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:75</code> | <code>config :block_scout_web, BlockScoutWeb.CSPHeader [mixpanel_url, amplitude_url]</code> |
| <code>interface.14.cfg-web-web-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:96</code> | <code>config :block_scout_web, BlockScoutWeb.Endpoint [server, url, render_errors, pubsub_server, secret_key_base, check_origin, http, debug_errors, code_reloader, watchers, live_reload, cache_static_manifest, force_ssl, checksum_address_hashes]</code> |
| <code>interface.14.cfg-web-web-gettext</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:40</code> | <code>config :block_scout_web, BlockScoutWeb.Gettext [locales, default_locale]</code> |
| <code>interface.14.cfg-web-web-healthendpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:106</code> | <code>config :block_scout_web, BlockScoutWeb.HealthEndpoint [server, url, render_errors, pubsub_server, secret_key_base, check_origin, http]</code> |
| <code>interface.14.cfg-web-web-microserviceinterfaces-transactioninterpretation</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:241</code> | <code>config :block_scout_web, BlockScoutWeb.MicroserviceInterfaces.TransactionInterpretation [service_url, enabled]</code> |
| <code>interface.14.cfg-web-web-routers-apirouter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:68</code> | <code>config :block_scout_web, BlockScoutWeb.Routers.ApiRouter [writing_enabled, reading_enabled]</code> |
| <code>interface.14.cfg-web-web-routers-webrouter</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:72</code> | <code>config :block_scout_web, BlockScoutWeb.Routers.WebRouter [enabled]</code> |
| <code>interface.14.cfg-web-web-socialmedia</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:42</code> | <code>config :block_scout_web, BlockScoutWeb.SocialMedia [twitter, telegram, facebook, instagram]</code> |
| <code>interface.14.cfg-web-web-tracer</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:34</code> | <code>config :block_scout_web, BlockScoutWeb.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-web-webapp-url</code> | <code>config_key</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:42</code> | <code>config :block_scout_web, :webapp_url</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-web-admin-panel-enabled</code> | notes | set in 1 config statement(s); 1 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-api-graphql</code> | notes | set in 2 config statement(s); 16 reader(s) |
| <code>interface.14.cfg-web-api-rate-limit</code> | notes | set in 1 config statement(s); 42 reader(s); 2 further callers omitted; 3 more listed callers dropped for line length |
| <code>interface.14.cfg-web-api-url</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-apps</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-apps-menu</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-chain-id</code> | notes | set in 1 config statement(s); 55 reader(s); 15 further callers omitted; 5 more listed callers dropped for line length |
| <code>interface.14.cfg-web-chart</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-checksum-address-hashes</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-web-circles-addresses</code> | notes | set in 1 config statement(s); 3 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-contract</code> | notes | set in 1 config statement(s); 14 reader(s) |
| <code>interface.14.cfg-web-cookie-domain</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-dark-forest-addresses</code> | notes | set in 1 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-dark-forest-addresses-v-0-5</code> | notes | set in 1 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-disable-add-to-mm-button</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-disable-api</code> | notes | set in 2 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-display-token-icons</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-web-ecto-repos</code> | notes | set in 1 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-footer</code> | notes | set in 1 config statement(s); 12 reader(s) |
| <code>interface.14.cfg-web-gas-price</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-hide-block-miner</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-web-hide-scam-addresses</code> | notes | set in 1 config statement(s); 181 reader(s); 141 further callers omitted; 5 more listed callers dropped for line length |
| <code>interface.14.cfg-web-json-rpc</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-namespace</code> | notes | set in 1 config statement(s); 2 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-new-tags</code> | notes | set in 1 config statement(s); 1 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-other-networks</code> | notes | set in 1 config statement(s); 9 reader(s) |
| <code>interface.14.cfg-web-permanent-dark-mode-enabled</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-permanent-light-mode-enabled</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-recaptcha</code> | notes | set in 1 config statement(s); 36 reader(s); 1 more listed callers dropped for line length |
| <code>interface.14.cfg-web-release-link</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-web-remote-ip</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-sensitive-endpoints-api-key</code> | notes | set in 1 config statement(s); 13 reader(s) |
| <code>interface.14.cfg-web-show-percentage</code> | notes | set in 1 config statement(s); 8 reader(s) |
| <code>interface.14.cfg-web-show-tenderly-link</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-sql-sandbox</code> | notes | set in 1 config statement(s); 1 reader(s); reader matched by the atom outside an Application env call |
| <code>interface.14.cfg-web-version</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-web-web-api-v2</code> | notes | set in 2 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-web-chain</code> | notes | set in 1 config statement(s); 32 reader(s) |
| <code>interface.14.cfg-web-web-chain-address-coinbalance</code> | notes | set in 1 config statement(s); 4 reader(s) |
| <code>interface.14.cfg-web-web-chain-transactionhistorychartcontroller</code> | notes | set in 1 config statement(s); 3 reader(s) |
| <code>interface.14.cfg-web-web-counters-blocksindexedcounter</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-web-web-counters-internaltransactionsindexedcounter</code> | notes | set in 2 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-web-web-cspheader</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-web-endpoint</code> | notes | set in 7 config statement(s); 38 reader(s); 4 further callers omitted; 2 more listed callers dropped for line length |
| <code>interface.14.cfg-web-web-gettext</code> | notes | set in 1 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-web-web-healthendpoint</code> | notes | set in 3 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-web-web-microserviceinterfaces-transactioninterpretation</code> | notes | set in 1 config statement(s); 10 reader(s) |
| <code>interface.14.cfg-web-web-routers-apirouter</code> | notes | set in 1 config statement(s); 2 reader(s) |
| <code>interface.14.cfg-web-web-routers-webrouter</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-web-socialmedia</code> | notes | set in 1 config statement(s); 1 reader(s) |
| <code>interface.14.cfg-web-web-tracer</code> | notes | set in 4 config statement(s); 1 reader(s); consumed by the module at its defmodule line |
| <code>interface.14.cfg-web-webapp-url</code> | notes | set in 1 config statement(s); 1 reader(s) |

## env-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-account-auth0-application-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:250</code> | <code>auth0_application_id: ConfigHelper.safe_get_env("ACCOUNT_AUTH0_APPLICATION_ID", nil) &#124;&gt; String.replace(".", "")</code> |
| <code>interface.14.env-account-auth0-client-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:248</code> | <code>client_id: System.get_env("ACCOUNT_AUTH0_CLIENT_ID"),</code> |
| <code>interface.14.env-account-auth0-client-secret</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:249</code> | <code>client_secret: System.get_env("ACCOUNT_AUTH0_CLIENT_SECRET"),</code> |
| <code>interface.14.env-account-auth0-domain</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:247</code> | <code>domain: System.get_env("ACCOUNT_AUTH0_DOMAIN"),</code> |
| <code>interface.14.env-account-cloak-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/vault.ex:11</code> | <code>default: {Cloak.Ciphers.AES.GCM, tag: "AES.GCM.V1", key: decode_env!("ACCOUNT_CLOAK_KEY")}</code> |
| <code>interface.14.env-account-database-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:83</code> | <code>database_account = if System.get_env("ACCOUNT_DATABASE_URL"), do: nil, else: database</code> |
| <code>interface.14.env-account-dynamic-env-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:742</code> | <code>dynamic_env_id = System.get_env("ACCOUNT_DYNAMIC_ENV_ID")</code> |
| <code>interface.14.env-account-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:835</code> | <code>enabled: ConfigHelper.parse_bool_env_var("ACCOUNT_ENABLED"),</code> |
| <code>interface.14.env-account-keycloak-client-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:754</code> | <code>client_id: System.get_env("ACCOUNT_KEYCLOAK_CLIENT_ID"),</code> |
| <code>interface.14.env-account-keycloak-client-secret</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:755</code> | <code>client_secret: System.get_env("ACCOUNT_KEYCLOAK_CLIENT_SECRET"),</code> |
| <code>interface.14.env-account-keycloak-domain</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:752</code> | <code>domain: ConfigHelper.parse_url_env_var("ACCOUNT_KEYCLOAK_DOMAIN", nil, true),</code> |
| <code>interface.14.env-account-keycloak-email-webhook-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:756</code> | <code>email_webhook_url: ConfigHelper.parse_url_env_var("ACCOUNT_KEYCLOAK_EMAIL_WEBHOOK_URL")</code> |
| <code>interface.14.env-account-keycloak-realm</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:753</code> | <code>realm: System.get_env("ACCOUNT_KEYCLOAK_REALM"),</code> |
| <code>interface.14.env-account-otp-resend-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:843</code> | <code>otp_resend_interval: ConfigHelper.parse_time_env_var("ACCOUNT_OTP_RESEND_INTERVAL", "1m"),</code> |
| <code>interface.14.env-account-pool-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:91</code> | <code>pool_size: ConfigHelper.parse_integer_env_var("ACCOUNT_POOL_SIZE", 10),</code> |
| <code>interface.14.env-account-private-tags-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:844</code> | <code>private_tags_limit: ConfigHelper.parse_integer_env_var("ACCOUNT_PRIVATE_TAGS_LIMIT", 2_000),</code> |
| <code>interface.14.env-account-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/application.ex:524</code> | <code>{ConfigHelper.parse_url_env_var("ACCOUNT_REDIS_URL", "&lt;url&gt;"), [name: :redix]}</code> |
| <code>interface.14.env-account-sendgrid-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:832</code> | <code>api_key: System.get_env("ACCOUNT_SENDGRID_API_KEY")</code> |
| <code>interface.14.env-account-sendgrid-otp-template</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:839</code> | <code>otp_template: System.get_env("ACCOUNT_SENDGRID_OTP_TEMPLATE")</code> |
| <code>interface.14.env-account-sendgrid-sender</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:837</code> | <code>sender: System.get_env("ACCOUNT_SENDGRID_SENDER"),</code> |
| <code>interface.14.env-account-sendgrid-template</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:838</code> | <code>template: System.get_env("ACCOUNT_SENDGRID_TEMPLATE"),</code> |
| <code>interface.14.env-account-siwe-message</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:848</code> | <code>siwe_message: System.get_env("ACCOUNT_SIWE_MESSAGE", "Sign in to Blockscout Account V2")</code> |
| <code>interface.14.env-account-verification-email-resend-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:842</code> | <code>ConfigHelper.parse_time_env_var("ACCOUNT_VERIFICATION_EMAIL_RESEND_INTERVAL", "5m"),</code> |
| <code>interface.14.env-account-watchlist-addresses-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:845</code> | <code>watchlist_addresses_limit: ConfigHelper.parse_integer_env_var("ACCOUNT_WATCHLIST_ADDRESSES_LIMIT", 15),</code> |
| <code>interface.14.env-account-watchlist-notifications-limit-for-30-days</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:847</code> | <code>ConfigHelper.parse_integer_env_var("ACCOUNT_WATCHLIST_NOTIFICATIONS_LIMIT_FOR_30_DAYS", 1_000),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-account-auth0-application-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-auth0-client-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-auth0-client-secret</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-auth0-domain</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-cloak-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-database-url</code> | notes | read 3x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-dynamic-env-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-enabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-account-keycloak-client-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-keycloak-client-secret</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-keycloak-domain</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-keycloak-email-webhook-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-keycloak-realm</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-otp-resend-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-pool-size</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-private-tags-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-redis-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-sendgrid-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-sendgrid-otp-template</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-sendgrid-sender</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-sendgrid-template</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-siwe-message</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-verification-email-resend-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-watchlist-addresses-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-account-watchlist-notifications-limit-for-30-days</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-addresses

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-addresses-blacklist</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:346</code> | <code>addresses_blacklist: System.get_env("ADDRESSES_BLACKLIST"),</code> |
| <code>interface.14.env-addresses-blacklist-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:347</code> | <code>addresses_blacklist_key: System.get_env("ADDRESSES_BLACKLIST_KEY"),</code> |
| <code>interface.14.env-addresses-blacklist-provider</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1155</code> | <code>"ADDRESSES_BLACKLIST_PROVIDER",</code> |
| <code>interface.14.env-addresses-blacklist-retry-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1152</code> | <code>retry_interval: ConfigHelper.parse_time_env_var("ADDRESSES_BLACKLIST_RETRY_INTERVAL", "5s"),</code> |
| <code>interface.14.env-addresses-blacklist-update-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1151</code> | <code>update_interval: ConfigHelper.parse_time_env_var("ADDRESSES_BLACKLIST_UPDATE_INTERVAL", "15m"),</code> |
| <code>interface.14.env-addresses-blacklist-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1146</code> | <code>addresses_blacklist_url = ConfigHelper.parse_url_env_var("ADDRESSES_BLACKLIST_URL")</code> |
| <code>interface.14.env-addresses-tabs-counters-ttl</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:872</code> | <code>ttl: ConfigHelper.parse_time_env_var("ADDRESSES_TABS_COUNTERS_TTL", "10m")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-addresses-blacklist</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-addresses-blacklist-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-addresses-blacklist-provider</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-addresses-blacklist-retry-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-addresses-blacklist-update-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-addresses-blacklist-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-addresses-tabs-counters-ttl</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-admin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-admin-panel-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:25</code> | <code>admin_panel_enabled: ConfigHelper.parse_bool_env_var("ADMIN_PANEL_ENABLED")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-admin-panel-enabled</code> | notes | read 1x in backend; set in 9 non-backend sites |

## env-amb

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-amb-bridge-mediators</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex:142</code> | <code>["AMB_BRIDGE_MEDIATORS", "CUSTOM_CONTRACT_ADDRESSES_AMB_BRIDGE_MEDIATORS"],</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-amb-bridge-mediators</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-amplitude

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-amplitude-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:77</code> | <code>amplitude_url: ConfigHelper.parse_url_env_var("AMPLITUDE_URL", "&lt;url&gt;")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-amplitude-url</code> | notes | read 1x in backend; set in 3 non-backend sites |

## env-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-api-disable-contract-creation-internal-transaction-association</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:333</code> | <code>ConfigHelper.parse_bool_env_var("API_DISABLE_CONTRACT_CREATION_INTERNAL_TRANSACTION_ASSOCIATION"),</code> |
| <code>interface.14.env-api-graphql-default-transaction-hash</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:197</code> | <code>"API_GRAPHQL_DEFAULT_TRANSACTION_HASH",</code> |
| <code>interface.14.env-api-graphql-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:200</code> | <code>enabled: ConfigHelper.parse_bool_env_var("API_GRAPHQL_ENABLED", "true"),</code> |
| <code>interface.14.env-api-graphql-max-complexity</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:82</code> | <code>max_complexity: ConfigHelper.parse_integer_env_var("API_GRAPHQL_MAX_COMPLEXITY", 100)</code> |
| <code>interface.14.env-api-graphql-rate-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:202</code> | <code>global_limit: ConfigHelper.parse_integer_env_var("API_GRAPHQL_RATE_LIMIT", default_graphql_rate_limit),</code> |
| <code>interface.14.env-api-graphql-rate-limit-by-ip</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:209</code> | <code>limit_by_ip: ConfigHelper.parse_integer_env_var("API_GRAPHQL_RATE_LIMIT_BY_IP", 500),</code> |
| <code>interface.14.env-api-graphql-rate-limit-by-ip-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:210</code> | <code>time_interval_limit_by_ip: ConfigHelper.parse_time_env_var("API_GRAPHQL_RATE_LIMIT_BY_IP_TIME_INTERVAL", "1h"),</code> |
| <code>interface.14.env-api-graphql-rate-limit-by-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:205</code> | <code>"API_GRAPHQL_RATE_LIMIT_BY_KEY",</code> |
| <code>interface.14.env-api-graphql-rate-limit-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:201</code> | <code>rate_limit_disabled?: ConfigHelper.parse_bool_env_var("API_GRAPHQL_RATE_LIMIT_DISABLED"),</code> |
| <code>interface.14.env-api-graphql-rate-limit-static-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:211</code> | <code>static_api_key: System.get_env("API_GRAPHQL_RATE_LIMIT_STATIC_API_KEY")</code> |
| <code>interface.14.env-api-graphql-rate-limit-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:208</code> | <code>time_interval_limit: ConfigHelper.parse_time_env_var("API_GRAPHQL_RATE_LIMIT_TIME_INTERVAL", "1s"),</code> |
| <code>interface.14.env-api-graphql-token-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:81</code> | <code>token_limit: ConfigHelper.parse_integer_env_var("API_GRAPHQL_TOKEN_LIMIT", 1000),</code> |
| <code>interface.14.env-api-internal-transactions-indexing-finished-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1533</code> | <code>"API_INTERNAL_TRANSACTIONS_INDEXING_FINISHED_THRESHOLD",</code> |
| <code>interface.14.env-api-no-rate-limit-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:175</code> | <code>no_rate_limit_api_key_value: System.get_env("API_NO_RATE_LIMIT_API_KEY"),</code> |
| <code>interface.14.env-api-rate-limit-by-account-api-key-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:173</code> | <code>period: ConfigHelper.parse_time_env_var("API_RATE_LIMIT_BY_ACCOUNT_API_KEY_TIME_INTERVAL", "1s")</code> |
| <code>interface.14.env-api-rate-limit-by-ip</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:165</code> | <code>limit: ConfigHelper.parse_integer_env_var("API_RATE_LIMIT_BY_IP", 300),</code> |
| <code>interface.14.env-api-rate-limit-by-ip-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:166</code> | <code>period: ConfigHelper.parse_time_env_var("API_RATE_LIMIT_BY_IP_TIME_INTERVAL", "1m")</code> |
| <code>interface.14.env-api-rate-limit-by-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:153</code> | <code>limit: ConfigHelper.parse_integer_env_var("API_RATE_LIMIT_BY_KEY", default_api_rate_limit_by_key),</code> |
| <code>interface.14.env-api-rate-limit-by-key-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:154</code> | <code>period: ConfigHelper.parse_time_env_var("API_RATE_LIMIT_BY_KEY_TIME_INTERVAL", "1s")</code> |
| <code>interface.14.env-api-rate-limit-by-whitelisted-ip</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:159</code> | <code>"API_RATE_LIMIT_BY_WHITELISTED_IP",</code> |
| <code>interface.14.env-api-rate-limit-by-whitelisted-ip-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:162</code> | <code>period: ConfigHelper.parse_time_env_var("API_RATE_LIMIT_BY_WHITELISTED_IP_TIME_INTERVAL", "1s")</code> |
| <code>interface.14.env-api-rate-limit-config-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:185</code> | <code>config_url: ConfigHelper.parse_url_env_var("API_RATE_LIMIT_CONFIG_URL")</code> |
| <code>interface.14.env-api-rate-limit-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:150</code> | <code>disabled: ConfigHelper.parse_bool_env_var("API_RATE_LIMIT_DISABLED"),</code> |
| <code>interface.14.env-api-rate-limit-hammer-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:147</code> | <code>api_rate_limit_redis_url = ConfigHelper.safe_get_env("API_RATE_LIMIT_HAMMER_REDIS_URL", nil)</code> |
| <code>interface.14.env-api-rate-limit-is-blockscout-behind-proxy</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:188</code> | <code>is_blockscout_behind_proxy: ConfigHelper.parse_bool_env_var("API_RATE_LIMIT_IS_BLOCKSCOUT_BEHIND_PROXY"),</code> |
| <code>interface.14.env-api-rate-limit-remote-ip-headers</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:189</code> | <code>headers: ConfigHelper.parse_list_env_var("API_RATE_LIMIT_REMOTE_IP_HEADERS", "x-forwarded-for"),</code> |
| <code>interface.14.env-api-rate-limit-remote-ip-known-proxies</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:190</code> | <code>proxies: ConfigHelper.parse_list_env_var("API_RATE_LIMIT_REMOTE_IP_KNOWN_PROXIES", "")</code> |
| <code>interface.14.env-api-rate-limit-static-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:151</code> | <code>static_api_key_value: System.get_env("API_RATE_LIMIT_STATIC_API_KEY"),</code> |
| <code>interface.14.env-api-rate-limit-ui-v2-token-ttl</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:177</code> | <code>api_v2_token_ttl: ConfigHelper.parse_time_env_var("API_RATE_LIMIT_UI_V2_TOKEN_TTL", "30m"),</code> |
| <code>interface.14.env-api-rate-limit-ui-v2-with-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:169</code> | <code>limit: ConfigHelper.parse_integer_env_var("API_RATE_LIMIT_UI_V2_WITH_TOKEN", 5),</code> |
| <code>interface.14.env-api-rate-limit-ui-v2-with-token-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:170</code> | <code>period: ConfigHelper.parse_time_env_var("API_RATE_LIMIT_UI_V2_WITH_TOKEN_TIME_INTERVAL", "1s")</code> |
| <code>interface.14.env-api-rate-limit-whitelisted-ips</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:176</code> | <code>whitelisted_ips: System.get_env("API_RATE_LIMIT_WHITELISTED_IPS"),</code> |
| <code>interface.14.env-api-sensitive-endpoints-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:66</code> | <code>sensitive_endpoints_api_key: System.get_env("API_SENSITIVE_ENDPOINTS_KEY"),</code> |
| <code>interface.14.env-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:49</code> | <code>api_url: ConfigHelper.parse_url_env_var("API_URL"),</code> |
| <code>interface.14.env-api-v1-read-methods-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:70</code> | <code>reading_enabled: !ConfigHelper.parse_bool_env_var("API_V1_READ_METHODS_DISABLED")</code> |
| <code>interface.14.env-api-v1-write-methods-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:69</code> | <code>writing_enabled: !ConfigHelper.parse_bool_env_var("API_V1_WRITE_METHODS_DISABLED"),</code> |
| <code>interface.14.env-api-v2-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:239</code> | <code>config :block_scout_web, BlockScoutWeb.API.V2, enabled: ConfigHelper.parse_bool_env_var("API_V2_ENABLED", "true")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-api-disable-contract-creation-internal-transaction-association</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-default-transaction-hash</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-enabled</code> | notes | read 2x in backend; set in 2 non-backend sites |
| <code>interface.14.env-api-graphql-max-complexity</code> | notes | read 1x in backend; set in 10 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit-by-ip</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit-by-ip-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit-by-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit-disabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit-static-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-rate-limit-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-graphql-token-limit</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-api-internal-transactions-indexing-finished-threshold</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-api-no-rate-limit-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-account-api-key-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-ip</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-ip-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-key-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-whitelisted-ip</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-by-whitelisted-ip-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-config-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-disabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-api-rate-limit-hammer-redis-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-is-blockscout-behind-proxy</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-remote-ip-headers</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-remote-ip-known-proxies</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-static-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-ui-v2-token-ttl</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-api-rate-limit-ui-v2-with-token</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-ui-v2-with-token-time-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-rate-limit-whitelisted-ips</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-sensitive-endpoints-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-api-v1-read-methods-disabled</code> | notes | read 1x in backend; set in 12 non-backend sites |
| <code>interface.14.env-api-v1-write-methods-disabled</code> | notes | read 1x in backend; set in 12 non-backend sites |
| <code>interface.14.env-api-v2-enabled</code> | notes | read 1x in backend; set in 3 non-backend sites |

## env-application

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-application-mode</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:512</code> | <code>def mode, do: parse_catalog_value("APPLICATION_MODE", @supported_modes, true, "all")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-application-mode</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-apps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-apps</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:51</code> | <code>apps: System.get_env("APPS") &#124;&#124; System.get_env("EXTERNAL_APPS"),</code> |
| <code>interface.14.env-apps-menu</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:50</code> | <code>apps_menu: ConfigHelper.parse_bool_env_var("APPS_MENU"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-apps</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-apps-menu</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-beryx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-beryx-api-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2081</code> | <code>"BERYX_API_BASE_URL",</code> |
| <code>interface.14.env-beryx-api-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2084</code> | <code>api_token: System.get_env("BERYX_API_TOKEN")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-beryx-api-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-beryx-api-token</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-blackfort

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-blackfort-validator-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1144</code> | <code>config :explorer, Explorer.Chain.Blackfort.Validator, api_url: System.get_env("BLACKFORT_VALIDATOR_API_URL")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-blackfort-validator-api-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-block-miner-gets-burnt-fees</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:651</code> | <code>block_miner_gets_burnt_fees?: ConfigHelper.parse_bool_env_var("BLOCK_MINER_GETS_BURNT_FEES"),</code> |
| <code>interface.14.env-block-ranges</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1214</code> | <code>ConfigHelper.safe_get_env("BLOCK_RANGES", "#{first_block}..#{last_block &#124;&#124; "latest"}")</code> |
| <code>interface.14.env-block-transformer</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:322</code> | <code>configured_transformer = safe_get_env("BLOCK_TRANSFORMER", "base")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-block-miner-gets-burnt-fees</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-block-ranges</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-block-transformer</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-blockscout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-blockscout-host</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:101</code> | <code>host: System.get_env("BLOCKSCOUT_HOST") &#124;&#124; "&lt;host&gt;"</code> |
| <code>interface.14.env-blockscout-protocol</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:100</code> | <code>scheme: System.get_env("BLOCKSCOUT_PROTOCOL") &#124;&#124; "http",</code> |
| <code>interface.14.env-blockscout-version</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:43</code> | <code>version: System.get_env("BLOCKSCOUT_VERSION"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-blockscout-host</code> | notes | read 6x in backend; set in 1 non-backend sites |
| <code>interface.14.env-blockscout-protocol</code> | notes | read 4x in backend; set in 1 non-backend sites |
| <code>interface.14.env-blockscout-version</code> | notes | read 1x in backend; set in 7 non-backend sites |

## env-bridged

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-bridged-tokens-amb-bridge-mediators</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1127</code> | <code>amb_bridge_mediators: System.get_env("BRIDGED_TOKENS_AMB_BRIDGE_MEDIATORS"),</code> |
| <code>interface.14.env-bridged-tokens-bsc-omni-bridge-mediator</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1125</code> | <code>bsc_omni_bridge_mediator: System.get_env("BRIDGED_TOKENS_BSC_OMNI_BRIDGE_MEDIATOR"),</code> |
| <code>interface.14.env-bridged-tokens-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:36</code> | <code>{parse_bool_env_var("BRIDGED_TOKENS_ENABLED"), Explorer.Repo.BridgedTokens},</code> |
| <code>interface.14.env-bridged-tokens-eth-omni-bridge-mediator</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1124</code> | <code>eth_omni_bridge_mediator: System.get_env("BRIDGED_TOKENS_ETH_OMNI_BRIDGE_MEDIATOR"),</code> |
| <code>interface.14.env-bridged-tokens-foreign-json-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1128</code> | <code>foreign_json_rpc: System.get_env("BRIDGED_TOKENS_FOREIGN_JSON_RPC", "")</code> |
| <code>interface.14.env-bridged-tokens-poa-omni-bridge-mediator</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1126</code> | <code>poa_omni_bridge_mediator: System.get_env("BRIDGED_TOKENS_POA_OMNI_BRIDGE_MEDIATOR"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-bridged-tokens-amb-bridge-mediators</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-bridged-tokens-bsc-omni-bridge-mediator</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-bridged-tokens-enabled</code> | notes | read 2x in backend; set in 10 non-backend sites |
| <code>interface.14.env-bridged-tokens-eth-omni-bridge-mediator</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-bridged-tokens-foreign-json-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-bridged-tokens-poa-omni-bridge-mediator</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-cache

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-cache-address-count-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:410</code> | <code>update_interval_in_milliseconds: ConfigHelper.parse_time_env_var("CACHE_ADDRESS_COUNT_PERIOD", "30m")</code> |
| <code>interface.14.env-cache-address-sum-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:396</code> | <code>address_sum_global_ttl = ConfigHelper.parse_time_env_var("CACHE_ADDRESS_SUM_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-address-token-transfers-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:464</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_ADDRESS_TOKEN_TRANSFERS_COUNTER_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-address-tokens-usd-sum-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:461</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_ADDRESS_TOKENS_USD_SUM_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-address-transactions-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:458</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_ADDRESS_TRANSACTIONS_COUNTER_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-address-transactions-gas-usage-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:440</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_ADDRESS_TRANSACTIONS_GAS_USAGE_COUNTER_PERIOD", "30m")</code> |
| <code>interface.14.env-cache-average-block-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:451</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_AVERAGE_BLOCK_PERIOD", "30m"),</code> |
| <code>interface.14.env-cache-average-block-time-window</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:452</code> | <code>num_of_blocks: ConfigHelper.parse_integer_env_var("CACHE_AVERAGE_BLOCK_TIME_WINDOW", 100)</code> |
| <code>interface.14.env-cache-block-count-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:407</code> | <code>global_ttl: ConfigHelper.parse_time_env_var("CACHE_BLOCK_COUNT_PERIOD", "2h")</code> |
| <code>interface.14.env-cache-fresh-pending-transactions-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:478</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_FRESH_PENDING_TRANSACTIONS_COUNTER_PERIOD", "5m"),</code> |
| <code>interface.14.env-cache-market-history-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:455</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_MARKET_HISTORY_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-optimism-last-output-root-size-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:469</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_OPTIMISM_LAST_OUTPUT_ROOT_SIZE_COUNTER_PERIOD", "5m")</code> |
| <code>interface.14.env-cache-pending-operations-count-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:416</code> | <code>global_ttl: ConfigHelper.parse_time_env_var("CACHE_PENDING_OPERATIONS_COUNT_PERIOD", "5m")</code> |
| <code>interface.14.env-cache-token-holders-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:443</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_TOKEN_HOLDERS_COUNTER_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-token-transfers-counter-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:446</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_TOKEN_TRANSFERS_COUNTER_PERIOD", "1h")</code> |
| <code>interface.14.env-cache-total-gas-usage-counter-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:404</code> | <code>enabled: ConfigHelper.parse_bool_env_var("CACHE_TOTAL_GAS_USAGE_COUNTER_ENABLED")</code> |
| <code>interface.14.env-cache-total-gas-usage-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:403</code> | <code>global_ttl: ConfigHelper.parse_time_env_var("CACHE_TOTAL_GAS_USAGE_PERIOD", "2h"),</code> |
| <code>interface.14.env-cache-transactions-24h-stats-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:473</code> | <code>cache_period: ConfigHelper.parse_time_env_var("CACHE_TRANSACTIONS_24H_STATS_PERIOD", "1h"),</code> |
| <code>interface.14.env-cache-txs-count-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:413</code> | <code>global_ttl: ConfigHelper.parse_time_env_var("CACHE_TXS_COUNT_PERIOD", "2h")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-cache-address-count-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-address-sum-period</code> | notes | read 3x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-address-token-transfers-counter-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-address-tokens-usd-sum-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-address-transactions-counter-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-address-transactions-gas-usage-counter-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-average-block-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-average-block-time-window</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-cache-block-count-period</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-fresh-pending-transactions-counter-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-market-history-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-optimism-last-output-root-size-counter-period</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-cache-pending-operations-count-period</code> | notes | read 4x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-token-holders-counter-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-token-transfers-counter-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-total-gas-usage-counter-enabled</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-cache-total-gas-usage-period</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-transactions-24h-stats-period</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-cache-txs-count-period</code> | notes | read 2x in backend; set in 1 non-backend sites |

## env-celo

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-celo-accounts-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:717</code> | <code>accounts_contract_address: System.get_env("CELO_ACCOUNTS_CONTRACT")</code> |
| <code>interface.14.env-celo-core-contracts</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:720</code> | <code>contracts: ConfigHelper.parse_json_env_var("CELO_CORE_CONTRACTS")</code> |
| <code>interface.14.env-celo-epoch-manager-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:709</code> | <code>celo_epoch_manager_contract_address = System.get_env("CELO_EPOCH_MANAGER_CONTRACT")</code> |
| <code>interface.14.env-celo-l2-migration-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:708</code> | <code>celo_l2_migration_block = ConfigHelper.parse_integer_or_nil_env_var("CELO_L2_MIGRATION_BLOCK")</code> |
| <code>interface.14.env-celo-locked-gold-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:716</code> | <code>locked_gold_contract_address: System.get_env("CELO_LOCKED_GOLD_CONTRACT"),</code> |
| <code>interface.14.env-celo-unreleased-treasury-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:714</code> | <code>unreleased_treasury_contract_address: System.get_env("CELO_UNRELEASED_TREASURY_CONTRACT"),</code> |
| <code>interface.14.env-celo-validators-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:715</code> | <code>validators_contract_address: System.get_env("CELO_VALIDATORS_CONTRACT"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-celo-accounts-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-celo-core-contracts</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-celo-epoch-manager-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-celo-l2-migration-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-celo-locked-gold-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-celo-unreleased-treasury-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-celo-validators-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-certfile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-certfile</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:25</code> | <code>certfile: System.get_env("CERTFILE") &#124;&#124; "priv/cert/selfsigned.pem",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-certfile</code> | notes | read 2x in backend; set in 0 non-backend sites |

## env-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-chain-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:57</code> | <code>chain_id: System.get_env("CHAIN_ID"),</code> |
| <code>interface.14.env-chain-spec-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:390</code> | <code>chain_spec_path: System.get_env("CHAIN_SPEC_PATH"),</code> |
| <code>interface.14.env-chain-spec-processing-delay</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:391</code> | <code>genesis_processing_delay: ConfigHelper.parse_time_env_var("CHAIN_SPEC_PROCESSING_DELAY", "15s"),</code> |
| <code>interface.14.env-chain-type</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:501</code> | <code>"CHAIN_TYPE"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-chain-id</code> | notes | read 3x in backend; set in 119 non-backend sites; 81 further callers omitted |
| <code>interface.14.env-chain-spec-path</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-chain-spec-processing-delay</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-chain-type</code> | notes | read 1x in backend; set in 17 non-backend sites |

## env-check

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-check-origin</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/prod.exs:14</code> | <code>check_origin: System.get_env("CHECK_ORIGIN", "false") == "true" &#124;&#124; false,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-check-origin</code> | notes | read 2x in backend; set in 1 non-backend sites |

## env-checksum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-checksum-address-hashes</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:46</code> | <code>checksum_address_hashes: ConfigHelper.parse_bool_env_var("CHECKSUM_ADDRESS_HASHES", "true"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-checksum-address-hashes</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-coin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-coin</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:323</code> | <code>coin = System.get_env("COIN") &#124;&#124; "ETH"</code> |
| <code>interface.14.env-coin-balance-history-days</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:237</code> | <code>coin_balance_history_days: ConfigHelper.parse_integer_env_var("COIN_BALANCE_HISTORY_DAYS", 10)</code> |
| <code>interface.14.env-coin-balance-on-demand-fetcher-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1348</code> | <code>threshold: ConfigHelper.parse_time_env_var("COIN_BALANCE_ON_DEMAND_FETCHER_THRESHOLD", "1h"),</code> |
| <code>interface.14.env-coin-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:331</code> | <code>coin_name: System.get_env("COIN_NAME") &#124;&#124; "ETH",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-coin</code> | notes | read 1x in backend; set in 16 non-backend sites |
| <code>interface.14.env-coin-balance-history-days</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-coin-balance-on-demand-fetcher-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-coin-name</code> | notes | read 1x in backend; set in 3 non-backend sites |

## env-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-contract-audit-reports-airtable-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:823</code> | <code>audit_reports_api_key = System.get_env("CONTRACT_AUDIT_REPORTS_AIRTABLE_API_KEY")</code> |
| <code>interface.14.env-contract-audit-reports-airtable-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:822</code> | <code>audit_reports_table_url = System.get_env("CONTRACT_AUDIT_REPORTS_AIRTABLE_URL")</code> |
| <code>interface.14.env-contract-certified-list</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:142</code> | <code>certified_list: ConfigHelper.parse_list_env_var("CONTRACT_CERTIFIED_LIST", ""),</code> |
| <code>interface.14.env-contract-code-on-demand-fetcher-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1352</code> | <code>threshold: ConfigHelper.parse_time_env_var("CONTRACT_CODE_ON_DEMAND_FETCHER_THRESHOLD", "5s")</code> |
| <code>interface.14.env-contract-disable-interaction</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:141</code> | <code>disable_interaction: ConfigHelper.parse_bool_env_var("CONTRACT_DISABLE_INTERACTION"),</code> |
| <code>interface.14.env-contract-enable-partial-reverification</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:143</code> | <code>partial_reverification_disabled: !ConfigHelper.parse_bool_env_var("CONTRACT_ENABLE_PARTIAL_REVERIFICATION")</code> |
| <code>interface.14.env-contract-max-string-length-without-trimming</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:140</code> | <code>max_length_to_show_string_without_trimming: System.get_env("CONTRACT_MAX_STRING_LENGTH_WITHOUT_TRIMMING", "2040"),</code> |
| <code>interface.14.env-contract-proxy-implementation-ttl-via-avg-block-time</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:369</code> | <code>"CONTRACT_PROXY_IMPLEMENTATION_TTL_VIA_AVG_BLOCK_TIME",</code> |
| <code>interface.14.env-contract-verification-allowed-solidity-evm-versions</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:335</code> | <code>System.get_env("CONTRACT_VERIFICATION_ALLOWED_SOLIDITY_EVM_VERSIONS") &#124;&#124;</code> |
| <code>interface.14.env-contract-verification-allowed-vyper-evm-versions</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:338</code> | <code>System.get_env("CONTRACT_VERIFICATION_ALLOWED_VYPER_EVM_VERSIONS") &#124;&#124;</code> |
| <code>interface.14.env-contract-verification-max-libraries</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:139</code> | <code>verification_max_libraries: ConfigHelper.parse_integer_env_var("CONTRACT_VERIFICATION_MAX_LIBRARIES", 10),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-contract-audit-reports-airtable-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-audit-reports-airtable-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-certified-list</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-code-on-demand-fetcher-threshold</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-contract-disable-interaction</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-enable-partial-reverification</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-max-string-length-without-trimming</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-proxy-implementation-ttl-via-avg-block-time</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-contract-verification-allowed-solidity-evm-versions</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-verification-allowed-vyper-evm-versions</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-contract-verification-max-libraries</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-csv

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-csv-export-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:351</code> | <code>csv_export_limit: ConfigHelper.parse_integer_env_var("CSV_EXPORT_LIMIT", 10_000),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-csv-export-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-custom

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-custom-contract-addresses-amb-bridge-mediators</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex:142</code> | <code>["AMB_BRIDGE_MEDIATORS", "CUSTOM_CONTRACT_ADDRESSES_AMB_BRIDGE_MEDIATORS"],</code> |
| <code>interface.14.env-custom-contract-addresses-aox</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex:159</code> | <code>set_tag_for_multiple_env_var_addresses(["CUSTOM_CONTRACT_ADDRESSES_AOX"], "l2")</code> |
| <code>interface.14.env-custom-contract-addresses-circles</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:55</code> | <code>circles_addresses: System.get_env("CUSTOM_CONTRACT_ADDRESSES_CIRCLES"),</code> |
| <code>interface.14.env-custom-contract-addresses-dark-forest</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:53</code> | <code>dark_forest_addresses: System.get_env("CUSTOM_CONTRACT_ADDRESSES_DARK_FOREST"),</code> |
| <code>interface.14.env-custom-contract-addresses-dark-forest-v-0-5</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:54</code> | <code>dark_forest_addresses_v_0_5: System.get_env("CUSTOM_CONTRACT_ADDRESSES_DARK_FOREST_V_0_5"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-custom-contract-addresses-amb-bridge-mediators</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-custom-contract-addresses-aox</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-custom-contract-addresses-circles</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-custom-contract-addresses-dark-forest</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-custom-contract-addresses-dark-forest-v-0-5</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-database

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-database-event-pool-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:120</code> | <code>pool_size: ConfigHelper.parse_integer_env_var("DATABASE_EVENT_POOL_SIZE", 10),</code> |
| <code>interface.14.env-database-event-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:112</code> | <code>database_event_notification = if System.get_env("DATABASE_EVENT_URL"), do: nil, else: database</code> |
| <code>interface.14.env-database-queue-target</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:62</code> | <code>queue_target = ConfigHelper.parse_integer_env_var("DATABASE_QUEUE_TARGET", 50)</code> |
| <code>interface.14.env-database-read-only-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:58</code> | <code>if System.get_env("DATABASE_READ_ONLY_API_URL"),</code> |
| <code>interface.14.env-database-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:54</code> | <code>database = if System.get_env("DATABASE_URL"), do: nil, else: "explorer_dev"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-database-event-pool-size</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-database-event-url</code> | notes | read 3x in backend; set in 1 non-backend sites |
| <code>interface.14.env-database-queue-target</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-database-read-only-api-url</code> | notes | read 5x in backend; set in 1 non-backend sites |
| <code>interface.14.env-database-url</code> | notes | read 11x in backend; set in 82 non-backend sites; 52 further callers omitted |

## env-datadog

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-datadog-host</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/application.ex:503</code> | <code>host: System.get_env("DATADOG_HOST") &#124;&#124; "&lt;host&gt;",</code> |
| <code>interface.14.env-datadog-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:858</code> | <code>config :explorer, :datadog, port: ConfigHelper.parse_integer_env_var("DATADOG_PORT", 8126)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-datadog-host</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-datadog-port</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-decode-not-a-contract-calls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/config.exs:19</code> | <code>decode_not_a_contract_calls: ConfigHelper.parse_bool_env_var("DECODE_NOT_A_CONTRACT_CALLS")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-decode-not-a-contract-calls</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-disable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-disable-add-to-mm-button</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:59</code> | <code>disable_add_to_mm_button: ConfigHelper.parse_bool_env_var("DISABLE_ADD_TO_MM_BUTTON"),</code> |
| <code>interface.14.env-disable-api</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:40</code> | <code>disable_api? = ConfigHelper.parse_bool_env_var("DISABLE_API")</code> |
| <code>interface.14.env-disable-catchup-indexer</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1391</code> | <code>config :indexer, Indexer.Block.Catchup.Supervisor, enabled: !ConfigHelper.parse_bool_env_var("DISABLE_CATCHUP_INDEXER")</code> |
| <code>interface.14.env-disable-exchange-rates</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:321</code> | <code>else: ConfigHelper.parse_bool_env_var("DISABLE_EXCHANGE_RATES")</code> |
| <code>interface.14.env-disable-file-logging</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:60</code> | <code>if parse_bool_env_var("DISABLE_FILE_LOGGING") do</code> |
| <code>interface.14.env-disable-indexer</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:316</code> | <code>disable_indexer? = app_mode == :api &#124;&#124; ConfigHelper.parse_bool_env_var("DISABLE_INDEXER")</code> |
| <code>interface.14.env-disable-market</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:319</code> | <code>if System.get_env("DISABLE_MARKET"),</code> |
| <code>interface.14.env-disable-realtime-indexer</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1389</code> | <code>enabled: !ConfigHelper.parse_bool_env_var("DISABLE_REALTIME_INDEXER")</code> |
| <code>interface.14.env-disable-token-exchange-rate</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:616</code> | <code>disable_token_exchange_rates? = ConfigHelper.parse_bool_env_var("DISABLE_TOKEN_EXCHANGE_RATE")</code> |
| <code>interface.14.env-disable-webapp</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:73</code> | <code>enabled: !ConfigHelper.parse_bool_env_var("DISABLE_WEBAPP", "true")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-disable-add-to-mm-button</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-disable-api</code> | notes | read 2x in backend; set in 9 non-backend sites |
| <code>interface.14.env-disable-catchup-indexer</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-disable-exchange-rates</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-disable-file-logging</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-disable-indexer</code> | notes | read 2x in backend; set in 10 non-backend sites |
| <code>interface.14.env-disable-market</code> | notes | read 2x in backend; set in 2 non-backend sites |
| <code>interface.14.env-disable-realtime-indexer</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-disable-token-exchange-rate</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-disable-webapp</code> | notes | read 1x in backend; set in 8 non-backend sites |

## env-display

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-display-token-icons</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:62</code> | <code>display_token_icons: ConfigHelper.parse_bool_env_var("DISPLAY_TOKEN_ICONS"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-display-token-icons</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-ecto

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-ecto-use-ssl</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex:67</code> | <code>def ssl_enabled?, do: String.equivalent?(System.get_env("ECTO_USE_SSL") &#124;&#124; "true", "true")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-ecto-use-ssl</code> | notes | read 1x in backend; set in 4 non-backend sites |

## env-eip

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-eip-1559-base-fee-lower-bound-wei</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:350</code> | <code>base_fee_lower_bound: ConfigHelper.parse_integer_env_var("EIP_1559_BASE_FEE_LOWER_BOUND_WEI", 0),</code> |
| <code>interface.14.env-eip-1559-base-fee-max-change-denominator</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:349</code> | <code>base_fee_max_change_denominator: ConfigHelper.parse_integer_env_var("EIP_1559_BASE_FEE_MAX_CHANGE_DENOMINATOR", 8),</code> |
| <code>interface.14.env-eip-1559-elasticity-multiplier</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:348</code> | <code>elasticity_multiplier: ConfigHelper.parse_integer_env_var("EIP_1559_ELASTICITY_MULTIPLIER", 2),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-eip-1559-base-fee-lower-bound-wei</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-eip-1559-base-fee-max-change-denominator</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-eip-1559-elasticity-multiplier</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-emission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-emission-format</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:392</code> | <code>emission_format: System.get_env("EMISSION_FORMAT", "DEFAULT"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-emission-format</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-eth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-eth-json-rpc-max-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:178</code> | <code>eth_json_rpc_max_batch_size: ConfigHelper.parse_integer_env_var("ETH_JSON_RPC_MAX_BATCH_SIZE", 5),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-eth-json-rpc-max-batch-size</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-ethereum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-ethereum-jsonrpc-archive-balances-window</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:269</code> | <code>archive_balances_window: ConfigHelper.parse_integer_env_var("ETHEREUM_JSONRPC_ARCHIVE_BALANCES_WINDOW", 200),</code> |
| <code>interface.14.env-ethereum-jsonrpc-debug-trace-transaction-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:290</code> | <code>debug_trace_timeout: System.get_env("ETHEREUM_JSONRPC_DEBUG_TRACE_TRANSACTION_TIMEOUT", "5s"),</code> |
| <code>interface.14.env-ethereum-jsonrpc-disable-archive-balances</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:268</code> | <code>ConfigHelper.parse_bool_env_var("ETHEREUM_JSONRPC_DISABLE_ARCHIVE_BALANCES"),</code> |
| <code>interface.14.env-ethereum-jsonrpc-eth-call-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:611</code> | <code>defp define_urls_vars(:eth_call), do: {"ETHEREUM_JSONRPC_ETH_CALL_URLS", "ETHEREUM_JSONRPC_ETH_CALL_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-eth-call-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:611</code> | <code>defp define_urls_vars(:eth_call), do: {"ETHEREUM_JSONRPC_ETH_CALL_URLS", "ETHEREUM_JSONRPC_ETH_CALL_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-eth-call-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:620</code> | <code>do: {"ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URLS", "ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-eth-call-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:620</code> | <code>do: {"ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URLS", "ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-http-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:614</code> | <code>do: {"ETHEREUM_JSONRPC_FALLBACK_HTTP_URLS", "ETHEREUM_JSONRPC_FALLBACK_HTTP_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-http-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:614</code> | <code>do: {"ETHEREUM_JSONRPC_FALLBACK_HTTP_URLS", "ETHEREUM_JSONRPC_FALLBACK_HTTP_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-trace-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:617</code> | <code>do: {"ETHEREUM_JSONRPC_FALLBACK_TRACE_URLS", "ETHEREUM_JSONRPC_FALLBACK_TRACE_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-trace-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:617</code> | <code>do: {"ETHEREUM_JSONRPC_FALLBACK_TRACE_URLS", "ETHEREUM_JSONRPC_FALLBACK_TRACE_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-ws-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/dev/anvil.exs:28</code> | <code>fallback_url: ConfigHelper.parse_url_env_var("ETHEREUM_JSONRPC_FALLBACK_WS_URL")</code> |
| <code>interface.14.env-ethereum-jsonrpc-geth-allow-empty-traces</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:289</code> | <code>allow_empty_traces?: ConfigHelper.parse_bool_env_var("ETHEREUM_JSONRPC_GETH_ALLOW_EMPTY_TRACES"),</code> |
| <code>interface.14.env-ethereum-jsonrpc-geth-trace-by-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:288</code> | <code>block_traceable?: ConfigHelper.parse_bool_env_var("ETHEREUM_JSONRPC_GETH_TRACE_BY_BLOCK"),</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-gzip-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:278</code> | <code>gzip_enabled?: ConfigHelper.parse_bool_env_var("ETHEREUM_JSONRPC_HTTP_GZIP_ENABLED", "false")</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-headers</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:276</code> | <code>&#124;&gt; Map.merge(ConfigHelper.parse_json_env_var("ETHEREUM_JSONRPC_HTTP_HEADERS", "{}"))</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-insecure</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:113</code> | <code>&#124;&gt; (&amp;if(System.get_env("ETHEREUM_JSONRPC_HTTP_INSECURE", "") == "true", do: [insecure: true] ++ &amp;1, else: &amp;1)).()</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:122</code> | <code>case Integer.parse(safe_get_env("ETHEREUM_JSONRPC_HTTP_TIMEOUT", "#{default_minutes * 60}")) do</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:609</code> | <code>defp define_urls_vars(:http), do: {"ETHEREUM_JSONRPC_HTTP_URLS", "ETHEREUM_JSONRPC_HTTP_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:609</code> | <code>defp define_urls_vars(:http), do: {"ETHEREUM_JSONRPC_HTTP_URLS", "ETHEREUM_JSONRPC_HTTP_URL"}</code> |
| <code>interface.14.env-ethereum-jsonrpc-max-receipts-by-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:271</code> | <code>max_receipts_by_block: ConfigHelper.parse_integer_env_var("ETHEREUM_JSONRPC_MAX_RECEIPTS_BY_BLOCK", 1000)</code> |
| <code>interface.14.env-ethereum-jsonrpc-password</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:110</code> | <code>basic_auth_pass = System.get_env("ETHEREUM_JSONRPC_PASSWORD", nil)</code> |
| <code>interface.14.env-ethereum-jsonrpc-pending-transactions-type</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:301</code> | <code>type: System.get_env("ETHEREUM_JSONRPC_PENDING_TRANSACTIONS_TYPE", "default")</code> |
| <code>interface.14.env-ethereum-jsonrpc-receipts-by-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:270</code> | <code>receipts_by_block?: ConfigHelper.parse_bool_env_var("ETHEREUM_JSONRPC_RECEIPTS_BY_BLOCK", "false"),</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-archive-max-inflight</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:284</code> | <code>archive_max_inflight: ConfigHelper.parse_integer_env_var("ETHEREUM_JSONRPC_THROTTLE_ARCHIVE_MAX_INFLIGHT", 8),</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-coalesce-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:285</code> | <code>coalesce?: ConfigHelper.parse_bool_env_var("ETHEREUM_JSONRPC_THROTTLE_COALESCE_ENABLED", "true")</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-max-inflight</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:281</code> | <code>max_inflight: ConfigHelper.parse_integer_env_var("ETHEREUM_JSONRPC_THROTTLE_MAX_INFLIGHT", 32),</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-rps</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:282</code> | <code>rps: ConfigHelper.parse_integer_env_var("ETHEREUM_JSONRPC_THROTTLE_RPS", 200),</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-trace-max-inflight</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:283</code> | <code>trace_max_inflight: ConfigHelper.parse_integer_env_var("ETHEREUM_JSONRPC_THROTTLE_TRACE_MAX_INFLIGHT", 4),</code> |
| <code>interface.14.env-ethereum-jsonrpc-trace-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:260</code> | <code>System.get_env("ETHEREUM_JSONRPC_TRACE_URL") in ["", nil] and</code> |
| <code>interface.14.env-ethereum-jsonrpc-trace-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:261</code> | <code>System.get_env("ETHEREUM_JSONRPC_TRACE_URLS") in ["", nil]</code> |
| <code>interface.14.env-ethereum-jsonrpc-transport</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:566</code> | <code>case safe_get_env("ETHEREUM_JSONRPC_TRANSPORT", "http") do</code> |
| <code>interface.14.env-ethereum-jsonrpc-user</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:109</code> | <code>basic_auth_user = System.get_env("ETHEREUM_JSONRPC_USER", "")</code> |
| <code>interface.14.env-ethereum-jsonrpc-variant</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:433</code> | <code>enabled: System.get_env("ETHEREUM_JSONRPC_VARIANT") == "rsk",</code> |
| <code>interface.14.env-ethereum-jsonrpc-wait-per-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:304</code> | <code>wait_per_timeout: ConfigHelper.parse_time_env_var("ETHEREUM_JSONRPC_WAIT_PER_TIMEOUT", "20s")</code> |
| <code>interface.14.env-ethereum-jsonrpc-ws-retry-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:307</code> | <code>retry_interval: ConfigHelper.parse_time_env_var("ETHEREUM_JSONRPC_WS_RETRY_INTERVAL", "1m")</code> |
| <code>interface.14.env-ethereum-jsonrpc-ws-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/dev/anvil.exs:27</code> | <code>url: ConfigHelper.parse_url_env_var("ETHEREUM_JSONRPC_WS_URL"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-ethereum-jsonrpc-archive-balances-window</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-debug-trace-transaction-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-disable-archive-balances</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-eth-call-url</code> | notes | read 1x in backend; set in 5 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-eth-call-urls</code> | notes | read 1x in backend; set in 5 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-eth-call-url</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-eth-call-urls</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-http-url</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-http-urls</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-trace-url</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-trace-urls</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-fallback-ws-url</code> | notes | read 32x in backend; set in 33 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-geth-allow-empty-traces</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-geth-trace-by-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-http-gzip-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-http-headers</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-http-insecure</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-ethereum-jsonrpc-http-timeout</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-http-url</code> | notes | read 1x in backend; set in 16 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-http-urls</code> | notes | read 1x in backend; set in 4 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-max-receipts-by-block</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-ethereum-jsonrpc-password</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-ethereum-jsonrpc-pending-transactions-type</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-ethereum-jsonrpc-receipts-by-block</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-ethereum-jsonrpc-throttle-archive-max-inflight</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-throttle-coalesce-enabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-throttle-max-inflight</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-throttle-rps</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-throttle-trace-max-inflight</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-trace-url</code> | notes | read 2x in backend; set in 5 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-trace-urls</code> | notes | read 2x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-transport</code> | notes | read 17x in backend; set in 25 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-user</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-ethereum-jsonrpc-variant</code> | notes | read 2x in backend; set in 15 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-wait-per-timeout</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-ws-retry-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ethereum-jsonrpc-ws-url</code> | notes | read 48x in backend; set in 57 non-backend sites; 17 further callers omitted |

## env-exchange

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-exchange-rates-coingecko-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:522</code> | <code>System.get_env("EXCHANGE_RATES_COINGECKO_API_KEY"),</code> |
| <code>interface.14.env-exchange-rates-coingecko-base-pro-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:517</code> | <code>"EXCHANGE_RATES_COINGECKO_BASE_PRO_URL",</code> |
| <code>interface.14.env-exchange-rates-coingecko-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:511</code> | <code>"EXCHANGE_RATES_COINGECKO_BASE_URL",</code> |
| <code>interface.14.env-exchange-rates-coingecko-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:525</code> | <code>System.get_env("EXCHANGE_RATES_COINGECKO_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-coingecko-platform-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:507</code> | <code>System.get_env("EXCHANGE_RATES_COINGECKO_PLATFORM_ID"),</code> |
| <code>interface.14.env-exchange-rates-coingecko-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:528</code> | <code>System.get_env("EXCHANGE_RATES_COINGECKO_SECONDARY_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:540</code> | <code>System.get_env("EXCHANGE_RATES_COINMARKETCAP_API_KEY"),</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:535</code> | <code>"EXCHANGE_RATES_COINMARKETCAP_BASE_URL",</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:543</code> | <code>System.get_env("EXCHANGE_RATES_COINMARKETCAP_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:546</code> | <code>System.get_env("EXCHANGE_RATES_COINMARKETCAP_SECONDARY_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-cryptocompare-secondary-coin-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:558</code> | <code>System.get_env("EXCHANGE_RATES_CRYPTOCOMPARE_SECONDARY_COIN_SYMBOL"),</code> |
| <code>interface.14.env-exchange-rates-cryptorank-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:573</code> | <code>System.get_env("EXCHANGE_RATES_CRYPTORANK_API_KEY"),</code> |
| <code>interface.14.env-exchange-rates-cryptorank-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:568</code> | <code>"EXCHANGE_RATES_CRYPTORANK_BASE_URL",</code> |
| <code>interface.14.env-exchange-rates-cryptorank-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:576</code> | <code>ConfigHelper.parse_integer_or_nil_env_var("EXCHANGE_RATES_CRYPTORANK_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-cryptorank-platform-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:564</code> | <code>ConfigHelper.parse_integer_or_nil_env_var("EXCHANGE_RATES_CRYPTORANK_PLATFORM_ID"),</code> |
| <code>interface.14.env-exchange-rates-cryptorank-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:579</code> | <code>ConfigHelper.parse_integer_or_nil_env_var("EXCHANGE_RATES_CRYPTORANK_SECONDARY_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-history-first-fetch-day-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:647</code> | <code>ConfigHelper.parse_integer_env_var("EXCHANGE_RATES_HISTORY_FIRST_FETCH_DAY_COUNT", 365)</code> |
| <code>interface.14.env-exchange-rates-market-cap-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:499</code> | <code>ConfigHelper.market_source("EXCHANGE_RATES_MARKET_CAP_SOURCE"),</code> |
| <code>interface.14.env-exchange-rates-mobula-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:596</code> | <code>api_key: System.get_env("MARKET_MOBULA_API_KEY") &#124;&#124; System.get_env("EXCHANGE_RATES_MOBULA_API_KEY"),</code> |
| <code>interface.14.env-exchange-rates-mobula-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:593</code> | <code>"EXCHANGE_RATES_MOBULA_BASE_URL",</code> |
| <code>interface.14.env-exchange-rates-mobula-chain-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:589</code> | <code>System.get_env("EXCHANGE_RATES_MOBULA_CHAIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-mobula-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:597</code> | <code>coin_id: System.get_env("MARKET_MOBULA_COIN_ID") &#124;&#124; System.get_env("EXCHANGE_RATES_MOBULA_COIN_ID"),</code> |
| <code>interface.14.env-exchange-rates-mobula-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:600</code> | <code>System.get_env("EXCHANGE_RATES_MOBULA_SECONDARY_COIN_ID")</code> |
| <code>interface.14.env-exchange-rates-price-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:495</code> | <code>ConfigHelper.market_source("EXCHANGE_RATES_PRICE_SOURCE"),</code> |
| <code>interface.14.env-exchange-rates-secondary-coin-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:489</code> | <code>ConfigHelper.market_source("EXCHANGE_RATES_SECONDARY_COIN_SOURCE"),</code> |
| <code>interface.14.env-exchange-rates-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:486</code> | <code>ConfigHelper.market_source("EXCHANGE_RATES_SOURCE"),</code> |
| <code>interface.14.env-exchange-rates-tvl-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:502</code> | <code>ConfigHelper.market_source("EXCHANGE_RATES_TVL_SOURCE")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-exchange-rates-coingecko-api-key</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coingecko-base-pro-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coingecko-base-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coingecko-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coingecko-platform-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coingecko-secondary-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coinmarketcap-api-key</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coinmarketcap-base-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coinmarketcap-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-coinmarketcap-secondary-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-cryptocompare-secondary-coin-symbol</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-cryptorank-api-key</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-cryptorank-base-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-cryptorank-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-cryptorank-platform-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-cryptorank-secondary-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-history-first-fetch-day-count</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-market-cap-source</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-mobula-api-key</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-mobula-base-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-mobula-chain-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-mobula-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-mobula-secondary-coin-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-price-source</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-secondary-coin-source</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-source</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-exchange-rates-tvl-source</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-external

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-external-apps</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:51</code> | <code>apps: System.get_env("APPS") &#124;&#124; System.get_env("EXTERNAL_APPS"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-external-apps</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-fetch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fetch-rewards-way</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1241</code> | <code>fetch_rewards_way: System.get_env("FETCH_REWARDS_WAY", "trace_block"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fetch-rewards-way</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-filecoin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-filecoin-network-prefix</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1142</code> | <code>network_prefix: ConfigHelper.parse_catalog_value("FILECOIN_NETWORK_PREFIX", ["f", "t"], true, "f")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-filecoin-network-prefix</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-filfox

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-filfox-api-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2087</code> | <code>base_url: ConfigHelper.parse_url_env_var("FILFOX_API_BASE_URL", "&lt;url&gt;")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-filfox-api-base-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-first

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-first-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1210</code> | <code>first_block = ConfigHelper.parse_integer_env_var("FIRST_BLOCK", 0)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-first-block</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-footer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-footer-chat-link</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:129</code> | <code>chat_link: System.get_env("FOOTER_CHAT_LINK", "&lt;url&gt;"),</code> |
| <code>interface.14.env-footer-forum-link</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:132</code> | <code>forum_link: System.get_env("FOOTER_FORUM_LINK", "&lt;url&gt;"),</code> |
| <code>interface.14.env-footer-forum-link-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:131</code> | <code>forum_link_enabled: ConfigHelper.parse_bool_env_var("FOOTER_FORUM_LINK_ENABLED"),</code> |
| <code>interface.14.env-footer-github-link</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:130</code> | <code>github_link: System.get_env("FOOTER_GITHUB_LINK", "&lt;url&gt;"),</code> |
| <code>interface.14.env-footer-link-to-other-explorers</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:135</code> | <code>link_to_other_explorers: ConfigHelper.parse_bool_env_var("FOOTER_LINK_TO_OTHER_EXPLORERS"),</code> |
| <code>interface.14.env-footer-logo</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:128</code> | <code>logo: System.get_env("FOOTER_LOGO"),</code> |
| <code>interface.14.env-footer-other-explorers</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:136</code> | <code>other_explorers: System.get_env("FOOTER_OTHER_EXPLORERS", "")</code> |
| <code>interface.14.env-footer-telegram-link</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:134</code> | <code>telegram_link: System.get_env("FOOTER_TELEGRAM_LINK"),</code> |
| <code>interface.14.env-footer-telegram-link-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:133</code> | <code>telegram_link_enabled: ConfigHelper.parse_bool_env_var("FOOTER_TELEGRAM_LINK_ENABLED"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-footer-chat-link</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-forum-link</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-forum-link-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-github-link</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-link-to-other-explorers</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-logo</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-other-explorers</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-telegram-link</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-footer-telegram-link-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-gas-price</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:52</code> | <code>gas_price: System.get_env("GAS_PRICE"),</code> |
| <code>interface.14.env-gas-price-oracle-average-percentile</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:426</code> | <code>average_percentile: ConfigHelper.parse_integer_env_var("GAS_PRICE_ORACLE_AVERAGE_PERCENTILE", 60),</code> |
| <code>interface.14.env-gas-price-oracle-average-time-coefficient</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:429</code> | <code>average_time_coefficient: ConfigHelper.parse_float_env_var("GAS_PRICE_ORACLE_AVERAGE_TIME_COEFFICIENT", 3),</code> |
| <code>interface.14.env-gas-price-oracle-cache-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:422</code> | <code>global_ttl: ConfigHelper.parse_time_env_var("GAS_PRICE_ORACLE_CACHE_PERIOD", "30s"),</code> |
| <code>interface.14.env-gas-price-oracle-fast-percentile</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:427</code> | <code>fast_percentile: ConfigHelper.parse_integer_env_var("GAS_PRICE_ORACLE_FAST_PERCENTILE", 90),</code> |
| <code>interface.14.env-gas-price-oracle-fast-time-coefficient</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:430</code> | <code>fast_time_coefficient: ConfigHelper.parse_float_env_var("GAS_PRICE_ORACLE_FAST_TIME_COEFFICIENT", 1)</code> |
| <code>interface.14.env-gas-price-oracle-num-of-blocks</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:424</code> | <code>num_of_blocks: ConfigHelper.parse_integer_env_var("GAS_PRICE_ORACLE_NUM_OF_BLOCKS", 200),</code> |
| <code>interface.14.env-gas-price-oracle-safelow-percentile</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:425</code> | <code>safelow_percentile: ConfigHelper.parse_integer_env_var("GAS_PRICE_ORACLE_SAFELOW_PERCENTILE", 35),</code> |
| <code>interface.14.env-gas-price-oracle-safelow-time-coefficient</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:428</code> | <code>safelow_time_coefficient: ConfigHelper.parse_float_env_var("GAS_PRICE_ORACLE_SAFELOW_TIME_COEFFICIENT", 5),</code> |
| <code>interface.14.env-gas-price-oracle-simple-transaction-gas</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:423</code> | <code>simple_transaction_gas: ConfigHelper.parse_integer_env_var("GAS_PRICE_ORACLE_SIMPLE_TRANSACTION_GAS", 21_000),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-gas-price</code> | notes | read 1x in backend; set in 4 non-backend sites |
| <code>interface.14.env-gas-price-oracle-average-percentile</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-average-time-coefficient</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-cache-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-fast-percentile</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-fast-time-coefficient</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-num-of-blocks</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-safelow-percentile</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-safelow-time-coefficient</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-gas-price-oracle-simple-transaction-gas</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-hackney

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-hackney-default-pool-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:354</code> | <code>hackney_default_pool_size: ConfigHelper.parse_integer_env_var("HACKNEY_DEFAULT_POOL_SIZE", 1_000)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-hackney-default-pool-size</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-health-monitor-batches-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:363</code> | <code>healthy_batches_period: ConfigHelper.parse_time_env_var("HEALTH_MONITOR_BATCHES_PERIOD", "4h")</code> |
| <code>interface.14.env-health-monitor-blocks-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:362</code> | <code>healthy_blocks_period: ConfigHelper.parse_time_env_var("HEALTH_MONITOR_BLOCKS_PERIOD", "5m"),</code> |
| <code>interface.14.env-health-monitor-check-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:361</code> | <code>check_interval: ConfigHelper.parse_time_env_var("HEALTH_MONITOR_CHECK_INTERVAL", "1m"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-health-monitor-batches-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-health-monitor-blocks-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-health-monitor-check-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-hide

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-hide-block-miner</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:63</code> | <code>hide_block_miner: ConfigHelper.parse_bool_env_var("HIDE_BLOCK_MINER"),</code> |
| <code>interface.14.env-hide-scam-addresses</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:64</code> | <code>hide_scam_addresses: ConfigHelper.parse_bool_env_var("HIDE_SCAM_ADDRESSES"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-hide-block-miner</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-hide-scam-addresses</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-indexer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-indexer-arbitrum-arbsys-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1821</code> | <code>"INDEXER_ARBITRUM_ARBSYS_CONTRACT",</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1872</code> | <code>enabled: ConfigHelper.parse_bool_env_var("INDEXER_ARBITRUM_BATCHES_TRACKING_ENABLED")</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-failure-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1869</code> | <code>ConfigHelper.parse_time_env_var("INDEXER_ARBITRUM_BATCHES_TRACKING_FAILURE_THRESHOLD", "10m")</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-l1-finalization-check-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1852</code> | <code>"INDEXER_ARBITRUM_BATCHES_TRACKING_L1_FINALIZATION_CHECK_ENABLED",</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-messages-to-blocks-shift</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1857</code> | <code>"INDEXER_ARBITRUM_BATCHES_TRACKING_MESSAGES_TO_BLOCKS_SHIFT",</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1849</code> | <code>recheck_interval: ConfigHelper.parse_time_env_var("INDEXER_ARBITRUM_BATCHES_TRACKING_RECHECK_INTERVAL", "20s"),</code> |
| <code>interface.14.env-indexer-arbitrum-bridge-messages-tracking-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1846</code> | <code>enabled: ConfigHelper.parse_bool_env_var("INDEXER_ARBITRUM_BRIDGE_MESSAGES_TRACKING_ENABLED")</code> |
| <code>interface.14.env-indexer-arbitrum-confirmations-tracking-finalized</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1860</code> | <code>finalized_confirmations: ConfigHelper.parse_bool_env_var("INDEXER_ARBITRUM_CONFIRMATIONS_TRACKING_FINALIZED", "true"),</code> |
| <code>interface.14.env-indexer-arbitrum-data-backfill-blocks-depth</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1891</code> | <code>backfill_blocks_depth: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_DATA_BACKFILL_BLOCKS_DEPTH", 500)</code> |
| <code>interface.14.env-indexer-arbitrum-data-backfill-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1896</code> | <code>not ConfigHelper.parse_bool_env_var("INDEXER_ARBITRUM_DATA_BACKFILL_ENABLED")</code> |
| <code>interface.14.env-indexer-arbitrum-data-backfill-unindexed-blocks-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1888</code> | <code>"INDEXER_ARBITRUM_DATA_BACKFILL_UNINDEXED_BLOCKS_RECHECK_INTERVAL",</code> |
| <code>interface.14.env-indexer-arbitrum-l1-common-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1831</code> | <code>l1_start_block: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_L1_COMMON_START_BLOCK", 0),</code> |
| <code>interface.14.env-indexer-arbitrum-l1-finalization-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1832</code> | <code>l1_finalization_threshold: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_L1_FINALIZATION_THRESHOLD", 1_000),</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rollup-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1829</code> | <code>l1_rollup_address: System.get_env("INDEXER_ARBITRUM_L1_ROLLUP_CONTRACT"),</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rollup-init-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1830</code> | <code>l1_rollup_init_block: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_L1_ROLLUP_INIT_BLOCK", 1),</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1826</code> | <code>l1_rpc: System.get_env("INDEXER_ARBITRUM_L1_RPC"),</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rpc-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1827</code> | <code>l1_rpc_chunk_size: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_L1_RPC_CHUNK_SIZE", 20),</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rpc-historical-blocks-range</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1828</code> | <code>l1_rpc_block_range: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_L1_RPC_HISTORICAL_BLOCKS_RANGE", 1_000),</code> |
| <code>interface.14.env-indexer-arbitrum-messages-tracking-failure-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1842</code> | <code>ConfigHelper.parse_time_env_var("INDEXER_ARBITRUM_MESSAGES_TRACKING_FAILURE_THRESHOLD", "10m"),</code> |
| <code>interface.14.env-indexer-arbitrum-missed-message-ids-range</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1843</code> | <code>missed_message_ids_range: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_MISSED_MESSAGE_IDS_RANGE", 10_000)</code> |
| <code>interface.14.env-indexer-arbitrum-missed-messages-blocks-depth</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1877</code> | <code>ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_MISSED_MESSAGES_BLOCKS_DEPTH", 10_000)</code> |
| <code>interface.14.env-indexer-arbitrum-missed-messages-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1875</code> | <code>recheck_interval: ConfigHelper.parse_time_env_var("INDEXER_ARBITRUM_MISSED_MESSAGES_RECHECK_INTERVAL", "1h"),</code> |
| <code>interface.14.env-indexer-arbitrum-missing-batches-range</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1867</code> | <code>missing_batches_range: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_MISSING_BATCHES_RANGE", 10_000),</code> |
| <code>interface.14.env-indexer-arbitrum-new-batches-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1861</code> | <code>new_batches_limit: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_NEW_BATCHES_LIMIT", 10),</code> |
| <code>interface.14.env-indexer-arbitrum-node-interface-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1864</code> | <code>"INDEXER_ARBITRUM_NODE_INTERFACE_CONTRACT",</code> |
| <code>interface.14.env-indexer-arbitrum-rollup-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1833</code> | <code>rollup_chunk_size: ConfigHelper.parse_integer_env_var("INDEXER_ARBITRUM_ROLLUP_CHUNK_SIZE", 20)</code> |
| <code>interface.14.env-indexer-arbitrum-tracking-messages-on-l1-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1838</code> | <code>"INDEXER_ARBITRUM_TRACKING_MESSAGES_ON_L1_RECHECK_INTERVAL",</code> |
| <code>interface.14.env-indexer-arc-native-token-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1274</code> | <code>"INDEXER_ARC_NATIVE_TOKEN_CONTRACT",</code> |
| <code>interface.14.env-indexer-arc-native-token-decimals</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1271</code> | <code>arc_native_token_decimals: ConfigHelper.parse_integer_env_var("INDEXER_ARC_NATIVE_TOKEN_DECIMALS", 6),</code> |
| <code>interface.14.env-indexer-arc-native-token-system-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1279</code> | <code>"INDEXER_ARC_NATIVE_TOKEN_SYSTEM_CONTRACT",</code> |
| <code>interface.14.env-indexer-archival-token-balances-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1320</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_ARCHIVAL_TOKEN_BALANCES_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-indexer-archival-token-balances-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1321</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_ARCHIVAL_TOKEN_BALANCES_CONCURRENCY", 10),</code> |
| <code>interface.14.env-indexer-archival-token-balances-exponential-timeout-coeff</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1329</code> | <code>"INDEXER_ARCHIVAL_TOKEN_BALANCES_EXPONENTIAL_TIMEOUT_COEFF",</code> |
| <code>interface.14.env-indexer-archival-token-balances-max-refetch-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1324</code> | <code>"INDEXER_ARCHIVAL_TOKEN_BALANCES_MAX_REFETCH_INTERVAL",</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-end-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1974</code> | <code>end_block: ConfigHelper.parse_integer_env_var("INDEXER_BEACON_BLOB_FETCHER_END_BLOCK", 0)</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-reference-slot</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1967</code> | <code>reference_slot: ConfigHelper.parse_integer_env_var("INDEXER_BEACON_BLOB_FETCHER_REFERENCE_SLOT", 8_000_000),</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-reference-timestamp</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1970</code> | <code>"INDEXER_BEACON_BLOB_FETCHER_REFERENCE_TIMESTAMP",</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-slot-duration</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1966</code> | <code>slot_duration: ConfigHelper.parse_integer_env_var("INDEXER_BEACON_BLOB_FETCHER_SLOT_DURATION", 12),</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1973</code> | <code>start_block: ConfigHelper.parse_integer_env_var("INDEXER_BEACON_BLOB_FETCHER_START_BLOCK", 19_200_000),</code> |
| <code>interface.14.env-indexer-beacon-deposit-fetcher-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1983</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_BEACON_DEPOSIT_FETCHER_BATCH_SIZE", 1_000)</code> |
| <code>interface.14.env-indexer-beacon-deposit-fetcher-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1982</code> | <code>interval: ConfigHelper.parse_time_env_var("INDEXER_BEACON_DEPOSIT_FETCHER_INTERVAL", "6s"),</code> |
| <code>interface.14.env-indexer-beacon-deposit-status-fetcher-epoch-duration</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1993</code> | <code>"INDEXER_BEACON_DEPOSIT_STATUS_FETCHER_EPOCH_DURATION",</code> |
| <code>interface.14.env-indexer-beacon-deposit-status-fetcher-reference-timestamp</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1998</code> | <code>"INDEXER_BEACON_DEPOSIT_STATUS_FETCHER_REFERENCE_TIMESTAMP",</code> |
| <code>interface.14.env-indexer-beacon-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1909</code> | <code>config :indexer, Indexer.Fetcher.Beacon, beacon_rpc: System.get_env("INDEXER_BEACON_RPC_URL") &#124;&#124; "&lt;url&gt;"</code> |
| <code>interface.14.env-indexer-block-reward-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1472</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_BLOCK_REWARD_BATCH_SIZE", 10),</code> |
| <code>interface.14.env-indexer-block-reward-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1473</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_BLOCK_REWARD_CONCURRENCY", 4)</code> |
| <code>interface.14.env-indexer-catchup-block-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/indexer/config/dev/anvil.exs:8</code> | <code>block_interval: ConfigHelper.parse_time_env_var("INDEXER_CATCHUP_BLOCK_INTERVAL", "0s"),</code> |
| <code>interface.14.env-indexer-catchup-blocks-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1468</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_CATCHUP_BLOCKS_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-indexer-catchup-blocks-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1469</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_CATCHUP_BLOCKS_CONCURRENCY", 20)</code> |
| <code>interface.14.env-indexer-catchup-missing-ranges-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1465</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_CATCHUP_MISSING_RANGES_BATCH_SIZE", 100_000)</code> |
| <code>interface.14.env-indexer-celo-accounts-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2072</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_CELO_ACCOUNTS_BATCH_SIZE", 100)</code> |
| <code>interface.14.env-indexer-celo-accounts-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2071</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_CELO_ACCOUNTS_CONCURRENCY", 1),</code> |
| <code>interface.14.env-indexer-celo-validator-group-votes-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2055</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_CELO_VALIDATOR_GROUP_VOTES_BATCH_SIZE", 200_000)</code> |
| <code>interface.14.env-indexer-coin-balances-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1551</code> | <code>ConfigHelper.parse_integer_env_var("INDEXER_COIN_BALANCES_BATCH_SIZE", 100)</code> |
| <code>interface.14.env-indexer-coin-balances-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1554</code> | <code>ConfigHelper.parse_integer_env_var("INDEXER_COIN_BALANCES_CONCURRENCY", 4)</code> |
| <code>interface.14.env-indexer-coin-balances-fetcher-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1252</code> | <code>ConfigHelper.parse_integer_env_var("INDEXER_COIN_BALANCES_FETCHER_INIT_QUERY_LIMIT", 2_000),</code> |
| <code>interface.14.env-indexer-current-token-balances-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1337</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_CURRENT_TOKEN_BALANCES_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-indexer-current-token-balances-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1338</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_CURRENT_TOKEN_BALANCES_CONCURRENCY", 10)</code> |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2129</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DB_EVENT_NOTIFICATIONS_CLEANUP_ENABLED", "true"),</code> |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2126</code> | <code>interval: ConfigHelper.parse_time_env_var("INDEXER_DB_EVENT_NOTIFICATIONS_CLEANUP_INTERVAL", "2m"),</code> |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-max-age</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2130</code> | <code>max_age: ConfigHelper.parse_time_env_var("INDEXER_DB_EVENT_NOTIFICATIONS_CLEANUP_MAX_AGE", "5m")</code> |
| <code>interface.14.env-indexer-disable-address-coin-balance-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1370</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_ADDRESS_COIN_BALANCE_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-archival-token-balances-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1334</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_ARCHIVAL_TOKEN_BALANCES_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-beacon-blob-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1963</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_BEACON_BLOB_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-beacon-deposit-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1979</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_BEACON_DEPOSIT_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-beacon-deposit-status-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1988</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_BEACON_DEPOSIT_STATUS_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-block-reward-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1362</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_BLOCK_REWARD_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-cataloged-token-updater-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1377</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_CATALOGED_TOKEN_UPDATER_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-celo-epoch-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2064</code> | <code>not ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_CELO_EPOCH_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-celo-validator-group-votes-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2060</code> | <code>not ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_CELO_VALIDATOR_GROUP_VOTES_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-empty-blocks-sanitizer</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1386</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_EMPTY_BLOCKS_SANITIZER")</code> |
| <code>interface.14.env-indexer-disable-filecoin-address-info-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2092</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_FILECOIN_ADDRESS_INFO_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-hot-smart-contracts-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1450</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_HOT_SMART_CONTRACTS_FETCHER"),</code> |
| <code>interface.14.env-indexer-disable-internal-transactions-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1367</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_INTERNAL_TRANSACTIONS_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-balances-queue-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1429</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_FETCHER") &#124;&#124;</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-counters-queue-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1226</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-main-queue-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1424</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_FETCHER") &#124;&#124;</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-token-info-queue-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1434</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_FETCHER") &#124;&#124;</code> |
| <code>interface.14.env-indexer-disable-optimism-interop-multichain-export</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1717</code> | <code>"INDEXER_DISABLE_OPTIMISM_INTEROP_MULTICHAIN_EXPORT",</code> |
| <code>interface.14.env-indexer-disable-pending-transactions-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1315</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-replaced-transaction-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1394</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_REPLACED_TRANSACTION_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-rootstock-data-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1901</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_ROOTSTOCK_DATA_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-token-instance-erc-1155-sanitize-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1411</code> | <code>"INDEXER_DISABLE_TOKEN_INSTANCE_ERC_1155_SANITIZE_FETCHER",</code> |
| <code>interface.14.env-indexer-disable-token-instance-erc-721-sanitize-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1418</code> | <code>"INDEXER_DISABLE_TOKEN_INSTANCE_ERC_721_SANITIZE_FETCHER",</code> |
| <code>interface.14.env-indexer-disable-token-instance-realtime-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1397</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_TOKEN_INSTANCE_REALTIME_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-token-instance-refetch-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1406</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_TOKEN_INSTANCE_REFETCH_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-token-instance-retry-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1400</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_TOKEN_INSTANCE_RETRY_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-token-instance-sanitize-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1403</code> | <code>disabled?: ConfigHelper.parse_bool_env_var("INDEXER_DISABLE_TOKEN_INSTANCE_SANITIZE_FETCHER")</code> |
| <code>interface.14.env-indexer-disable-withdrawals-fetcher</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1799</code> | <code>disabled?: System.get_env("INDEXER_DISABLE_WITHDRAWALS_FETCHER", "true") == "true"</code> |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1454</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_EMPTY_BLOCKS_SANITIZER_BATCH_SIZE", 10),</code> |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-head-offset</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1456</code> | <code>head_offset: ConfigHelper.parse_integer_env_var("INDEXER_EMPTY_BLOCKS_SANITIZER_HEAD_OFFSET", 1000)</code> |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1455</code> | <code>interval: ConfigHelper.parse_time_env_var("INDEXER_EMPTY_BLOCKS_SANITIZER_INTERVAL", "10s"),</code> |
| <code>interface.14.env-indexer-fetcher-init-delay</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1248</code> | <code>fetcher_init_delay: ConfigHelper.parse_time_env_var("INDEXER_FETCHER_INIT_DELAY", "10m"),</code> |
| <code>interface.14.env-indexer-fetcher-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1247</code> | <code>fetcher_init_limit: ConfigHelper.parse_integer_env_var("INDEXER_FETCHER_INIT_QUERY_LIMIT", 100),</code> |
| <code>interface.14.env-indexer-filecoin-address-info-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2095</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_FILECOIN_ADDRESS_INFO_CONCURRENCY", 1)</code> |
| <code>interface.14.env-indexer-graceful-shutdown-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1253</code> | <code>graceful_shutdown_period: ConfigHelper.parse_time_env_var("INDEXER_GRACEFUL_SHUTDOWN_PERIOD", "5m"),</code> |
| <code>interface.14.env-indexer-hide-indexing-progress-alert</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1246</code> | <code>hide_indexing_progress_alert: ConfigHelper.parse_bool_env_var("INDEXER_HIDE_INDEXING_PROGRESS_ALERT"),</code> |
| <code>interface.14.env-indexer-internal-transactions-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1528</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_INTERNAL_TRANSACTIONS_BATCH_SIZE", 10),</code> |
| <code>interface.14.env-indexer-internal-transactions-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1529</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_INTERNAL_TRANSACTIONS_CONCURRENCY", 4),</code> |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1540</code> | <code>"INDEXER_INTERNAL_TRANSACTIONS_DELETE_QUEUE_BATCH_SIZE",</code> |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1545</code> | <code>"INDEXER_INTERNAL_TRANSACTIONS_DELETE_QUEUE_CONCURRENCY",</code> |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1548</code> | <code>threshold: ConfigHelper.parse_time_env_var("INDEXER_INTERNAL_TRANSACTIONS_DELETE_QUEUE_THRESHOLD", "0s")</code> |
| <code>interface.14.env-indexer-internal-transactions-fetch-order</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1256</code> | <code>"INDEXER_INTERNAL_TRANSACTIONS_FETCH_ORDER",</code> |
| <code>interface.14.env-indexer-internal-transactions-lookback-blocks</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1530</code> | <code>lookback_blocks: ConfigHelper.parse_integer_env_var("INDEXER_INTERNAL_TRANSACTIONS_LOOKBACK_BLOCKS", 9_000),</code> |
| <code>interface.14.env-indexer-internal-transactions-tracer-type</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:291</code> | <code>tracer: System.get_env("INDEXER_INTERNAL_TRANSACTIONS_TRACER_TYPE", "call_tracer")</code> |
| <code>interface.14.env-indexer-memory-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/config_helper.exs:264</code> | <code>"INDEXER_MEMORY_LIMIT"</code> |
| <code>interface.14.env-indexer-metrics-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2135</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_METRICS_ENABLED", "true"),</code> |
| <code>interface.14.env-indexer-metrics-enabled-failed-token-instances-metadata-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2144</code> | <code>"INDEXER_METRICS_ENABLED_FAILED_TOKEN_INSTANCES_METADATA_COUNT",</code> |
| <code>interface.14.env-indexer-metrics-enabled-missing-archival-token-balances-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2159</code> | <code>"INDEXER_METRICS_ENABLED_MISSING_ARCHIVAL_TOKEN_BALANCES_COUNT",</code> |
| <code>interface.14.env-indexer-metrics-enabled-missing-current-token-balances-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2154</code> | <code>"INDEXER_METRICS_ENABLED_MISSING_CURRENT_TOKEN_BALANCES_COUNT",</code> |
| <code>interface.14.env-indexer-metrics-enabled-token-instances-not-uploaded-to-cdn-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2139</code> | <code>"INDEXER_METRICS_ENABLED_TOKEN_INSTANCES_NOT_UPLOADED_TO_CDN_COUNT",</code> |
| <code>interface.14.env-indexer-metrics-enabled-unfetched-token-instances-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2149</code> | <code>"INDEXER_METRICS_ENABLED_UNFETCHED_TOKEN_INSTANCES_COUNT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1614</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_BATCH_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1609</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_CONCURRENCY",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1619</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1629</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_INIT_QUERY_LIMIT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-max-queue-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1624</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_MAX_QUEUE_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1668</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_BATCH_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1663</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_CONCURRENCY",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1673</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1683</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_INIT_QUERY_LIMIT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-max-queue-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1678</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_MAX_QUEUE_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1587</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_BATCH_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1582</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_CONCURRENCY",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1592</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1602</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_INIT_QUERY_LIMIT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-max-queue-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1597</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_MAX_QUEUE_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1641</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_BATCH_SIZE",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1636</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_CONCURRENCY",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1646</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1656</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_INIT_QUERY_LIMIT",</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-max-queue-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1651</code> | <code>"INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_MAX_QUEUE_SIZE",</code> |
| <code>interface.14.env-indexer-optimism-block-duration</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1726</code> | <code>block_duration: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_BLOCK_DURATION", 2),</code> |
| <code>interface.14.env-indexer-optimism-chainscout-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1766</code> | <code>chainscout_api_url: ConfigHelper.parse_url_env_var("INDEXER_OPTIMISM_CHAINSCOUT_API_URL", nil, true),</code> |
| <code>interface.14.env-indexer-optimism-chainscout-fallback-map</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1767</code> | <code>chainscout_fallback_map: ConfigHelper.parse_json_env_var("INDEXER_OPTIMISM_CHAINSCOUT_FALLBACK_MAP"),</code> |
| <code>interface.14.env-indexer-optimism-interop-connect-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1769</code> | <code>connect_timeout: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_INTEROP_CONNECT_TIMEOUT", 8),</code> |
| <code>interface.14.env-indexer-optimism-interop-export-expiration-days</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1771</code> | <code>export_expiration: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_INTEROP_EXPORT_EXPIRATION_DAYS", 10)</code> |
| <code>interface.14.env-indexer-optimism-interop-private-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1768</code> | <code>private_key: System.get_env("INDEXER_OPTIMISM_INTEROP_PRIVATE_KEY", ""),</code> |
| <code>interface.14.env-indexer-optimism-interop-recv-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1770</code> | <code>recv_timeout: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_INTEROP_RECV_TIMEOUT", 10),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-alt-da-server-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1751</code> | <code>alt_da_server_url: System.get_env("INDEXER_OPTIMISM_L1_BATCH_ALT_DA_SERVER_URL", ""),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-blocks-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1746</code> | <code>blocks_chunk_size: System.get_env("INDEXER_OPTIMISM_L1_BATCH_BLOCKS_CHUNK_SIZE", "4"),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-blockscout-blobs-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1747</code> | <code>eip4844_blobs_api_url: System.get_env("INDEXER_OPTIMISM_L1_BATCH_BLOCKSCOUT_BLOBS_API_URL", ""),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-celestia-blobs-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1748</code> | <code>celestia_blobs_api_url: System.get_env("INDEXER_OPTIMISM_L1_BATCH_CELESTIA_BLOBS_API_URL", ""),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-eigenda-blobs-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1749</code> | <code>eigenda_blobs_api_url: ConfigHelper.parse_url_env_var("INDEXER_OPTIMISM_L1_BATCH_EIGENDA_BLOBS_API_URL", ""),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-eigenda-proxy-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1750</code> | <code>eigenda_proxy_base_url: ConfigHelper.parse_url_env_var("INDEXER_OPTIMISM_L1_BATCH_EIGENDA_PROXY_BASE_URL"),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-inbox</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1753</code> | <code>inbox: System.get_env("INDEXER_OPTIMISM_L1_BATCH_INBOX"),</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-submitter</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1754</code> | <code>submitter: System.get_env("INDEXER_OPTIMISM_L1_BATCH_SUBMITTER")</code> |
| <code>interface.14.env-indexer-optimism-l1-deposits-transaction-type</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1732</code> | <code>transaction_type: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_L1_DEPOSITS_TRANSACTION_TYPE", 126)</code> |
| <code>interface.14.env-indexer-optimism-l1-eth-get-logs-range-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1724</code> | <code>l1_eth_get_logs_range_size: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_L1_ETH_GET_LOGS_RANGE_SIZE", 250),</code> |
| <code>interface.14.env-indexer-optimism-l1-output-oracle-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1735</code> | <code>output_oracle: System.get_env("INDEXER_OPTIMISM_L1_OUTPUT_ORACLE_CONTRACT")</code> |
| <code>interface.14.env-indexer-optimism-l1-portal-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1728</code> | <code>portal: System.get_env("INDEXER_OPTIMISM_L1_PORTAL_CONTRACT"),</code> |
| <code>interface.14.env-indexer-optimism-l1-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1722</code> | <code>optimism_l1_rpc: System.get_env("INDEXER_OPTIMISM_L1_RPC"),</code> |
| <code>interface.14.env-indexer-optimism-l1-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1727</code> | <code>start_block_l1: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_OPTIMISM_L1_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-optimism-l1-system-config-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1723</code> | <code>optimism_l1_system_config: System.get_env("INDEXER_OPTIMISM_L1_SYSTEM_CONFIG_CONTRACT"),</code> |
| <code>interface.14.env-indexer-optimism-l2-batch-genesis-block-number</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1752</code> | <code>genesis_block_l2: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_OPTIMISM_L2_BATCH_GENESIS_BLOCK_NUMBER"),</code> |
| <code>interface.14.env-indexer-optimism-l2-eth-get-logs-range-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1725</code> | <code>l2_eth_get_logs_range_size: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_L2_ETH_GET_LOGS_RANGE_SIZE", 250),</code> |
| <code>interface.14.env-indexer-optimism-l2-holocene-blocks-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1757</code> | <code>chunk_size: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_L2_HOLOCENE_BLOCKS_CHUNK_SIZE", 25),</code> |
| <code>interface.14.env-indexer-optimism-l2-holocene-timestamp</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1758</code> | <code>holocene_timestamp_l2: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_OPTIMISM_L2_HOLOCENE_TIMESTAMP"),</code> |
| <code>interface.14.env-indexer-optimism-l2-interop-blocks-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1763</code> | <code>blocks_chunk_size: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_L2_INTEROP_BLOCKS_CHUNK_SIZE", 4)</code> |
| <code>interface.14.env-indexer-optimism-l2-interop-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1762</code> | <code>start_block: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_OPTIMISM_L2_INTEROP_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-optimism-l2-isthmus-timestamp</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1229</code> | <code>ConfigHelper.parse_integer_or_nil_env_var("INDEXER_OPTIMISM_L2_ISTHMUS_TIMESTAMP")</code> |
| <code>interface.14.env-indexer-optimism-l2-jovian-timestamp</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1759</code> | <code>jovian_timestamp_l2: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_OPTIMISM_L2_JOVIAN_TIMESTAMP")</code> |
| <code>interface.14.env-indexer-optimism-l2-message-passer-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1741</code> | <code>"INDEXER_OPTIMISM_L2_MESSAGE_PASSER_CONTRACT",</code> |
| <code>interface.14.env-indexer-optimism-l2-withdrawals-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1738</code> | <code>start_block_l2: System.get_env("INDEXER_OPTIMISM_L2_WITHDRAWALS_START_BLOCK", "1"),</code> |
| <code>interface.14.env-indexer-optimism-multichain-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1774</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_MULTICHAIN_BATCH_SIZE", 100)</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1778</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1777</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_CONCURRENCY", 3),</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1781</code> | <code>"INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT",</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1791</code> | <code>"INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_INIT_QUERY_LIMIT",</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-max-queue-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1786</code> | <code>"INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_MAX_QUEUE_SIZE",</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-codec</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:71</code> | <code>codec: System.fetch_env!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_CODEC"),</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-first-batch</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:69</code> | <code>first_batch: env_integer!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_FIRST_BATCH", nil),</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-interval-ms</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:72</code> | <code>interval: env_integer!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_INTERVAL_MS", @default_interval)</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-last-batch</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:70</code> | <code>last_batch: env_integer!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_LAST_BATCH", nil),</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-network-id</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:66</code> | <code>network_id: env_integer!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_NETWORK_ID", nil),</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-relay-url</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:59</code> | <code>case System.get_env("INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL") do</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-sequencer-id</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:68</code> | <code>sequencer_id: env_hex!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_SEQUENCER_ID"),</code> |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-sequencer-public-key</code> | <code>env_var</code> | <code>retire</code> | <code>services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex:67</code> | <code>sequencer_public_key: env_hex!("INDEXER_PAXEER_X_KERNEL_RECEIPTS_SEQUENCER_PUBLIC_KEY"),</code> |
| <code>interface.14.env-indexer-pending-transactions-sanitizer-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1305</code> | <code>interval: ConfigHelper.parse_time_env_var("INDEXER_PENDING_TRANSACTIONS_SANITIZER_INTERVAL", "1h")</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2045</code> | <code>chunk_size: ConfigHelper.parse_integer_env_var("INDEXER_POLYGON_ZKEVM_BATCHES_CHUNK_SIZE", 20),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2052</code> | <code>ConfigHelper.parse_bool_env_var("INDEXER_POLYGON_ZKEVM_BATCHES_ENABLED")</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-ignore</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2046</code> | <code>ignore_numbers: System.get_env("INDEXER_POLYGON_ZKEVM_BATCHES_IGNORE", "0"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2047</code> | <code>recheck_interval: ConfigHelper.parse_integer_env_var("INDEXER_POLYGON_ZKEVM_BATCHES_RECHECK_INTERVAL", 60)</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2025</code> | <code>bridge_contract: System.get_env("INDEXER_POLYGON_ZKEVM_L1_BRIDGE_CONTRACT"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-native-decimals</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2027</code> | <code>native_decimals: ConfigHelper.parse_integer_env_var("INDEXER_POLYGON_ZKEVM_L1_BRIDGE_NATIVE_DECIMALS", 18),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-native-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2026</code> | <code>native_symbol: System.get_env("INDEXER_POLYGON_ZKEVM_L1_BRIDGE_NATIVE_SYMBOL", "ETH"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2028</code> | <code>rollup_network_id_l1: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_POLYGON_ZKEVM_L1_BRIDGE_NETWORK_ID"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-rollup-index</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2029</code> | <code>rollup_index_l1: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_POLYGON_ZKEVM_L1_BRIDGE_ROLLUP_INDEX")</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2024</code> | <code>start_block: System.get_env("INDEXER_POLYGON_ZKEVM_L1_BRIDGE_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2023</code> | <code>rpc: System.get_env("INDEXER_POLYGON_ZKEVM_L1_RPC"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2038</code> | <code>bridge_contract: System.get_env("INDEXER_POLYGON_ZKEVM_L2_BRIDGE_CONTRACT"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2039</code> | <code>rollup_network_id_l2: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_POLYGON_ZKEVM_L2_BRIDGE_NETWORK_ID"),</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-rollup-index</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2040</code> | <code>rollup_index_l2: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_POLYGON_ZKEVM_L2_BRIDGE_ROLLUP_INDEX")</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2037</code> | <code>start_block: System.get_env("INDEXER_POLYGON_ZKEVM_L2_BRIDGE_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-realtime-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1461</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_REALTIME_BATCH_SIZE", 64),</code> |
| <code>interface.14.env-indexer-realtime-fetcher-max-gap</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1459</code> | <code>max_gap: ConfigHelper.parse_integer_env_var("INDEXER_REALTIME_FETCHER_MAX_GAP", 1_000),</code> |
| <code>interface.14.env-indexer-realtime-fetcher-polling-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1460</code> | <code>polling_period: ConfigHelper.parse_time_env_var("INDEXER_REALTIME_FETCHER_POLLING_PERIOD"),</code> |
| <code>interface.14.env-indexer-realtime-max-lag</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1462</code> | <code>max_lag: ConfigHelper.parse_integer_env_var("INDEXER_REALTIME_MAX_LAG", 5_000)</code> |
| <code>interface.14.env-indexer-receipts-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1244</code> | <code>receipts_batch_size: ConfigHelper.parse_integer_env_var("INDEXER_RECEIPTS_BATCH_SIZE", 250),</code> |
| <code>interface.14.env-indexer-receipts-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1245</code> | <code>receipts_concurrency: ConfigHelper.parse_integer_env_var("INDEXER_RECEIPTS_CONCURRENCY", 10),</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1905</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_ROOTSTOCK_DATA_FETCHER_BATCH_SIZE", 10),</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1906</code> | <code>max_concurrency: ConfigHelper.parse_integer_env_var("INDEXER_ROOTSTOCK_DATA_FETCHER_CONCURRENCY", 5),</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-db-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1907</code> | <code>db_batch_size: ConfigHelper.parse_integer_env_var("INDEXER_ROOTSTOCK_DATA_FETCHER_DB_BATCH_SIZE", 300)</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1904</code> | <code>interval: ConfigHelper.parse_time_env_var("INDEXER_ROOTSTOCK_DATA_FETCHER_INTERVAL", "3s"),</code> |
| <code>interface.14.env-indexer-scroll-l1-batch-blockscout-blobs-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2117</code> | <code>eip4844_blobs_api_url: System.get_env("INDEXER_SCROLL_L1_BATCH_BLOCKSCOUT_BLOBS_API_URL", "")</code> |
| <code>interface.14.env-indexer-scroll-l1-batch-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2116</code> | <code>start_block: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_SCROLL_L1_BATCH_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-scroll-l1-chain-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2115</code> | <code>scroll_chain_contract: System.get_env("INDEXER_SCROLL_L1_CHAIN_CONTRACT"),</code> |
| <code>interface.14.env-indexer-scroll-l1-eth-get-logs-range-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2098</code> | <code>l1_eth_get_logs_range_size: ConfigHelper.parse_integer_env_var("INDEXER_SCROLL_L1_ETH_GET_LOGS_RANGE_SIZE", 250),</code> |
| <code>interface.14.env-indexer-scroll-l1-messenger-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2107</code> | <code>messenger_contract: System.get_env("INDEXER_SCROLL_L1_MESSENGER_CONTRACT"),</code> |
| <code>interface.14.env-indexer-scroll-l1-messenger-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2108</code> | <code>start_block: ConfigHelper.parse_integer_or_nil_env_var("INDEXER_SCROLL_L1_MESSENGER_START_BLOCK")</code> |
| <code>interface.14.env-indexer-scroll-l1-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2100</code> | <code>rpc: System.get_env("INDEXER_SCROLL_L1_RPC")</code> |
| <code>interface.14.env-indexer-scroll-l2-eth-get-logs-range-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2099</code> | <code>l2_eth_get_logs_range_size: ConfigHelper.parse_integer_env_var("INDEXER_SCROLL_L2_ETH_GET_LOGS_RANGE_SIZE", 1_000),</code> |
| <code>interface.14.env-indexer-scroll-l2-gas-oracle-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2102</code> | <code>config :indexer, Indexer.Fetcher.Scroll.L1FeeParam, gas_oracle: System.get_env("INDEXER_SCROLL_L2_GAS_ORACLE_CONTRACT")</code> |
| <code>interface.14.env-indexer-scroll-l2-messenger-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2111</code> | <code>messenger_contract: System.get_env("INDEXER_SCROLL_L2_MESSENGER_CONTRACT"),</code> |
| <code>interface.14.env-indexer-scroll-l2-messenger-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2112</code> | <code>start_block: ConfigHelper.parse_integer_env_var("INDEXER_SCROLL_L2_MESSENGER_START_BLOCK", first_block)</code> |
| <code>interface.14.env-indexer-shibarium-l1-deposit-manager-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2005</code> | <code>deposit_manager_proxy: System.get_env("INDEXER_SHIBARIUM_L1_DEPOSIT_MANAGER_CONTRACT"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-erc1155-predicate-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2009</code> | <code>erc1155_predicate_proxy: System.get_env("INDEXER_SHIBARIUM_L1_ERC1155_PREDICATE_CONTRACT"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-erc20-predicate-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2007</code> | <code>erc20_predicate_proxy: System.get_env("INDEXER_SHIBARIUM_L1_ERC20_PREDICATE_CONTRACT"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-erc721-predicate-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2008</code> | <code>erc721_predicate_proxy: System.get_env("INDEXER_SHIBARIUM_L1_ERC721_PREDICATE_CONTRACT"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-ether-predicate-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2006</code> | <code>ether_predicate_proxy: System.get_env("INDEXER_SHIBARIUM_L1_ETHER_PREDICATE_CONTRACT"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2003</code> | <code>rpc: System.get_env("INDEXER_SHIBARIUM_L1_RPC"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2004</code> | <code>start_block: System.get_env("INDEXER_SHIBARIUM_L1_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-shibarium-l1-withdraw-manager-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2010</code> | <code>withdraw_manager_proxy: System.get_env("INDEXER_SHIBARIUM_L1_WITHDRAW_MANAGER_CONTRACT")</code> |
| <code>interface.14.env-indexer-shibarium-l2-bone-withdraw-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2016</code> | <code>bone_withdraw: System.get_env("INDEXER_SHIBARIUM_L2_BONE_WITHDRAW_CONTRACT")</code> |
| <code>interface.14.env-indexer-shibarium-l2-child-chain-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2014</code> | <code>child_chain: System.get_env("INDEXER_SHIBARIUM_L2_CHILD_CHAIN_CONTRACT"),</code> |
| <code>interface.14.env-indexer-shibarium-l2-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2013</code> | <code>start_block: System.get_env("INDEXER_SHIBARIUM_L2_START_BLOCK"),</code> |
| <code>interface.14.env-indexer-shibarium-l2-weth-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2015</code> | <code>weth: System.get_env("INDEXER_SHIBARIUM_L2_WETH_CONTRACT"),</code> |
| <code>interface.14.env-indexer-signed-authorization-status-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1688</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_SIGNED_AUTHORIZATION_STATUS_BATCH_SIZE", 10)</code> |
| <code>interface.14.env-indexer-system-memory-percentage</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1243</code> | <code>system_memory_percentage: ConfigHelper.parse_integer_env_var("INDEXER_SYSTEM_MEMORY_PERCENTAGE", 60),</code> |
| <code>interface.14.env-indexer-token-balances-fetcher-init-query-limit</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1250</code> | <code>ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_BALANCES_FETCHER_INIT_QUERY_LIMIT", 100_000),</code> |
| <code>interface.14.env-indexer-token-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1317</code> | <code>config :indexer, Indexer.Fetcher.Token, concurrency: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_CONCURRENCY", 10)</code> |
| <code>interface.14.env-indexer-token-instance-allowed-uri-protocols</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1479</code> | <code>allowed_uri_protocols: ConfigHelper.parse_list_env_var("INDEXER_TOKEN_INSTANCE_ALLOWED_URI_PROTOCOLS", "http,https")</code> |
| <code>interface.14.env-indexer-token-instance-cidr-blacklist</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1477</code> | <code>cidr_blacklist: ConfigHelper.parse_list_env_var("INDEXER_TOKEN_INSTANCE_CIDR_BLACKLIST", ""),</code> |
| <code>interface.14.env-indexer-token-instance-host-filtering-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1478</code> | <code>host_filtering_enabled?: ConfigHelper.parse_bool_env_var("INDEXER_TOKEN_INSTANCE_HOST_FILTERING_ENABLED", "true"),</code> |
| <code>interface.14.env-indexer-token-instance-realtime-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1494</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_REALTIME_BATCH_SIZE", 1),</code> |
| <code>interface.14.env-indexer-token-instance-realtime-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1493</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_REALTIME_CONCURRENCY", 10),</code> |
| <code>interface.14.env-indexer-token-instance-realtime-retry-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1495</code> | <code>retry_with_cooldown?: ConfigHelper.parse_bool_env_var("INDEXER_TOKEN_INSTANCE_REALTIME_RETRY_ENABLED"),</code> |
| <code>interface.14.env-indexer-token-instance-realtime-retry-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1496</code> | <code>retry_timeout: ConfigHelper.parse_time_env_var("INDEXER_TOKEN_INSTANCE_REALTIME_RETRY_TIMEOUT", "5s")</code> |
| <code>interface.14.env-indexer-token-instance-refetch-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1504</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_REFETCH_BATCH_SIZE", 10)</code> |
| <code>interface.14.env-indexer-token-instance-refetch-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1503</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_REFETCH_CONCURRENCY", 10),</code> |
| <code>interface.14.env-indexer-token-instance-retry-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1483</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_RETRY_BATCH_SIZE", 10),</code> |
| <code>interface.14.env-indexer-token-instance-retry-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1482</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_RETRY_CONCURRENCY", 10),</code> |
| <code>interface.14.env-indexer-token-instance-retry-exponential-timeout-base</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1485</code> | <code>exp_timeout_base: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_RETRY_EXPONENTIAL_TIMEOUT_BASE", 2),</code> |
| <code>interface.14.env-indexer-token-instance-retry-exponential-timeout-coeff</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1488</code> | <code>"INDEXER_TOKEN_INSTANCE_RETRY_EXPONENTIAL_TIMEOUT_COEFF",</code> |
| <code>interface.14.env-indexer-token-instance-retry-max-refetch-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1484</code> | <code>max_refetch_interval: ConfigHelper.parse_time_env_var("INDEXER_TOKEN_INSTANCE_RETRY_MAX_REFETCH_INTERVAL", "168h"),</code> |
| <code>interface.14.env-indexer-token-instance-sanitize-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1500</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_SANITIZE_BATCH_SIZE", 10)</code> |
| <code>interface.14.env-indexer-token-instance-sanitize-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1499</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("INDEXER_TOKEN_INSTANCE_SANITIZE_CONCURRENCY", 10),</code> |
| <code>interface.14.env-indexer-token-instance-use-base-uri-retry</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1476</code> | <code>base_uri_retry?: ConfigHelper.parse_bool_env_var("INDEXER_TOKEN_INSTANCE_USE_BASE_URI_RETRY"),</code> |
| <code>interface.14.env-indexer-token-transfer-block-consensus-sanitizer-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1310</code> | <code>"INDEXER_TOKEN_TRANSFER_BLOCK_CONSENSUS_SANITIZER_INTERVAL",</code> |
| <code>interface.14.env-indexer-tx-actions-aave-v3-pool-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1292</code> | <code>aave_v3_pool: System.get_env("INDEXER_TX_ACTIONS_AAVE_V3_POOL_CONTRACT"),</code> |
| <code>interface.14.env-indexer-tx-actions-enable</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1286</code> | <code>enabled: ConfigHelper.parse_bool_env_var("INDEXER_TX_ACTIONS_ENABLE")</code> |
| <code>interface.14.env-indexer-tx-actions-max-token-cache-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:861</code> | <code>max_cache_size: ConfigHelper.parse_integer_env_var("INDEXER_TX_ACTIONS_MAX_TOKEN_CACHE_SIZE", 100_000)</code> |
| <code>interface.14.env-indexer-tx-actions-reindex-first-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1289</code> | <code>reindex_first_block: System.get_env("INDEXER_TX_ACTIONS_REINDEX_FIRST_BLOCK"),</code> |
| <code>interface.14.env-indexer-tx-actions-reindex-last-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1290</code> | <code>reindex_last_block: System.get_env("INDEXER_TX_ACTIONS_REINDEX_LAST_BLOCK"),</code> |
| <code>interface.14.env-indexer-tx-actions-reindex-protocols</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1291</code> | <code>reindex_protocols: System.get_env("INDEXER_TX_ACTIONS_REINDEX_PROTOCOLS", ""),</code> |
| <code>interface.14.env-indexer-tx-actions-uniswap-v3-factory-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1295</code> | <code>"INDEXER_TX_ACTIONS_UNISWAP_V3_FACTORY_CONTRACT",</code> |
| <code>interface.14.env-indexer-tx-actions-uniswap-v3-nft-position-manager-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1300</code> | <code>"INDEXER_TX_ACTIONS_UNISWAP_V3_NFT_POSITION_MANAGER_CONTRACT",</code> |
| <code>interface.14.env-indexer-zksync-batches-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1804</code> | <code>chunk_size: ConfigHelper.parse_integer_env_var("INDEXER_ZKSYNC_BATCHES_CHUNK_SIZE", 50),</code> |
| <code>interface.14.env-indexer-zksync-batches-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1809</code> | <code>enabled: ConfigHelper.parse_bool_env_var("INDEXER_ZKSYNC_BATCHES_ENABLED")</code> |
| <code>interface.14.env-indexer-zksync-batches-status-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1813</code> | <code>recheck_interval: ConfigHelper.parse_integer_env_var("INDEXER_ZKSYNC_BATCHES_STATUS_RECHECK_INTERVAL", 60)</code> |
| <code>interface.14.env-indexer-zksync-l1-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1812</code> | <code>zksync_l1_rpc: System.get_env("INDEXER_ZKSYNC_L1_RPC"),</code> |
| <code>interface.14.env-indexer-zksync-new-batches-max-range</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1805</code> | <code>batches_max_range: ConfigHelper.parse_integer_env_var("INDEXER_ZKSYNC_NEW_BATCHES_MAX_RANGE", 50),</code> |
| <code>interface.14.env-indexer-zksync-new-batches-recheck-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1806</code> | <code>recheck_interval: ConfigHelper.parse_integer_env_var("INDEXER_ZKSYNC_NEW_BATCHES_RECHECK_INTERVAL", 60)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-indexer-arbitrum-arbsys-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-failure-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-l1-finalization-check-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-messages-to-blocks-shift</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-bridge-messages-tracking-enabled</code> | notes | read 3x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-confirmations-tracking-finalized</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-data-backfill-blocks-depth</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-data-backfill-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-data-backfill-unindexed-blocks-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-common-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-finalization-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-rollup-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-rollup-init-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-rpc-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-l1-rpc-historical-blocks-range</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-messages-tracking-failure-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-missed-message-ids-range</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-missed-messages-blocks-depth</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-missed-messages-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-missing-batches-range</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-new-batches-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-node-interface-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-rollup-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arbitrum-tracking-messages-on-l1-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arc-native-token-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arc-native-token-decimals</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-arc-native-token-system-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-archival-token-balances-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-archival-token-balances-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-archival-token-balances-exponential-timeout-coeff</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-archival-token-balances-max-refetch-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-blob-fetcher-end-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-blob-fetcher-reference-slot</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-blob-fetcher-reference-timestamp</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-blob-fetcher-slot-duration</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-blob-fetcher-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-deposit-fetcher-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-deposit-fetcher-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-deposit-status-fetcher-epoch-duration</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-deposit-status-fetcher-reference-timestamp</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-beacon-rpc-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-block-reward-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-block-reward-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-catchup-block-interval</code> | notes | read 16x in backend; set in 17 non-backend sites |
| <code>interface.14.env-indexer-catchup-blocks-batch-size</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-catchup-blocks-concurrency</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-catchup-missing-ranges-batch-size</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-celo-accounts-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-celo-accounts-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-celo-validator-group-votes-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-coin-balances-batch-size</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-coin-balances-concurrency</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-coin-balances-fetcher-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-current-token-balances-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-current-token-balances-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-max-age</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-address-coin-balance-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-archival-token-balances-fetcher</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-disable-beacon-blob-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-beacon-deposit-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-beacon-deposit-status-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-block-reward-fetcher</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-disable-cataloged-token-updater-fetcher</code> | notes | read 3x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-celo-epoch-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-celo-validator-group-votes-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-empty-blocks-sanitizer</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-filecoin-address-info-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-hot-smart-contracts-fetcher</code> | notes | read 2x in backend; set in 0 non-backend sites |
| <code>interface.14.env-indexer-disable-internal-transactions-fetcher</code> | notes | read 1x in backend; set in 4 non-backend sites |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-balances-queue-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-counters-queue-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-main-queue-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-token-info-queue-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-optimism-interop-multichain-export</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-pending-transactions-fetcher</code> | notes | read 1x in backend; set in 4 non-backend sites |
| <code>interface.14.env-indexer-disable-replaced-transaction-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-rootstock-data-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-token-instance-erc-1155-sanitize-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-token-instance-erc-721-sanitize-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-token-instance-realtime-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-token-instance-refetch-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-token-instance-retry-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-token-instance-sanitize-fetcher</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-disable-withdrawals-fetcher</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-head-offset</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-fetcher-init-delay</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-fetcher-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-filecoin-address-info-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-graceful-shutdown-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-hide-indexing-progress-alert</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-internal-transactions-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-internal-transactions-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-batch-size</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-concurrency</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-threshold</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-internal-transactions-fetch-order</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-internal-transactions-lookback-blocks</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-internal-transactions-tracer-type</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-memory-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-metrics-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-metrics-enabled-failed-token-instances-metadata-count</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-metrics-enabled-missing-archival-token-balances-count</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-metrics-enabled-missing-current-token-balances-count</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-metrics-enabled-token-instances-not-uploaded-to-cdn-count</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-metrics-enabled-unfetched-token-instances-count</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-enqueue-busy-waiting-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-max-queue-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-enqueue-busy-waiting-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-max-queue-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-enqueue-busy-waiting-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-max-queue-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-enqueue-busy-waiting-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-max-queue-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-block-duration</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-chainscout-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-chainscout-fallback-map</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-interop-connect-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-interop-export-expiration-days</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-interop-private-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-interop-recv-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-alt-da-server-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-blocks-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-blockscout-blobs-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-celestia-blobs-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-eigenda-blobs-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-eigenda-proxy-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-inbox</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-batch-submitter</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-deposits-transaction-type</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-eth-get-logs-range-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-output-oracle-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-portal-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l1-system-config-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-batch-genesis-block-number</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-eth-get-logs-range-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-holocene-blocks-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-holocene-timestamp</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-interop-blocks-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-interop-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-isthmus-timestamp</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-jovian-timestamp</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-message-passer-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-l2-withdrawals-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-multichain-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-enqueue-busy-waiting-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-max-queue-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-codec</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-first-batch</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-interval-ms</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-last-batch</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-network-id</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-relay-url</code> | notes | read 1x in backend; set in 5 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-sequencer-id</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-paxeer-x-kernel-receipts-sequencer-public-key</code> | notes | read 1x in backend; set in 1 non-backend sites; kernel receipts fetcher removal in task 8.7.19 retires it |
| <code>interface.14.env-indexer-pending-transactions-sanitizer-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-batches-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-batches-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-batches-ignore</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-batches-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-native-decimals</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-native-symbol</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-network-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-rollup-index</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l1-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-network-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-rollup-index</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-realtime-batch-size</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-realtime-fetcher-max-gap</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-realtime-fetcher-polling-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-realtime-max-lag</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-receipts-batch-size</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-receipts-concurrency</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-indexer-rootstock-data-fetcher-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-rootstock-data-fetcher-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-rootstock-data-fetcher-db-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-rootstock-data-fetcher-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-batch-blockscout-blobs-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-batch-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-chain-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-eth-get-logs-range-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-messenger-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-messenger-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l1-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l2-eth-get-logs-range-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l2-gas-oracle-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l2-messenger-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-scroll-l2-messenger-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-shibarium-l1-deposit-manager-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-erc1155-predicate-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-erc20-predicate-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-erc721-predicate-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-ether-predicate-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-rpc</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-start-block</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l1-withdraw-manager-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l2-bone-withdraw-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l2-child-chain-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l2-start-block</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-shibarium-l2-weth-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-signed-authorization-status-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-system-memory-percentage</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-balances-fetcher-init-query-limit</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-allowed-uri-protocols</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-cidr-blacklist</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-host-filtering-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-realtime-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-realtime-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-realtime-retry-enabled</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-token-instance-realtime-retry-timeout</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-token-instance-refetch-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-refetch-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-retry-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-retry-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-retry-exponential-timeout-base</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-token-instance-retry-exponential-timeout-coeff</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-token-instance-retry-max-refetch-interval</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-token-instance-sanitize-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-sanitize-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-instance-use-base-uri-retry</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-token-transfer-block-consensus-sanitizer-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-aave-v3-pool-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-enable</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-max-token-cache-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-reindex-first-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-reindex-last-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-reindex-protocols</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-tx-actions-uniswap-v3-factory-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-tx-actions-uniswap-v3-nft-position-manager-contract</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-indexer-zksync-batches-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-zksync-batches-enabled</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-zksync-batches-status-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-zksync-l1-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-zksync-new-batches-max-range</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-indexer-zksync-new-batches-recheck-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-ipc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-ipc-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:265</code> | <code>ipc_path: System.get_env("IPC_PATH"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-ipc-path</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-ipfs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-ipfs-gateway-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1263</code> | <code>gateway_url: ConfigHelper.parse_url_env_var("IPFS_GATEWAY_URL", "&lt;url&gt;"),</code> |
| <code>interface.14.env-ipfs-gateway-url-param-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1264</code> | <code>gateway_url_param_key: System.get_env("IPFS_GATEWAY_URL_PARAM_KEY"),</code> |
| <code>interface.14.env-ipfs-gateway-url-param-location</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1267</code> | <code>ConfigHelper.parse_catalog_value("IPFS_GATEWAY_URL_PARAM_LOCATION", ["query", "header"], true),</code> |
| <code>interface.14.env-ipfs-gateway-url-param-value</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1265</code> | <code>gateway_url_param_value: System.get_env("IPFS_GATEWAY_URL_PARAM_VALUE"),</code> |
| <code>interface.14.env-ipfs-public-gateway-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1268</code> | <code>public_gateway_url: ConfigHelper.parse_url_env_var("IPFS_PUBLIC_GATEWAY_URL", "&lt;url&gt;")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-ipfs-gateway-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ipfs-gateway-url-param-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ipfs-gateway-url-param-location</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ipfs-gateway-url-param-value</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-ipfs-public-gateway-url</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-json-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:58</code> | <code>json_rpc: System.get_env("JSON_RPC"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-json-rpc</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-k8s

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-k8s-service</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2215</code> | <code>service: System.get_env("K8S_SERVICE"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-k8s-service</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-keyfile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-keyfile</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:26</code> | <code>keyfile: System.get_env("KEYFILE") &#124;&#124; "priv/cert/selfsigned_key.pem"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-keyfile</code> | notes | read 2x in backend; set in 0 non-backend sites |

## env-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-keys-manager-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:678</code> | <code>keys_manager_contract_address: System.get_env("KEYS_MANAGER_CONTRACT")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-keys-manager-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-last

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-last-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1211</code> | <code>last_block = ConfigHelper.parse_integer_or_nil_env_var("LAST_BLOCK")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-last-block</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-logo

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-logo</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:120</code> | <code>logo: System.get_env("LOGO"),</code> |
| <code>interface.14.env-logo-text</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:121</code> | <code>logo_text: System.get_env("LOGO_TEXT"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-logo</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-logo-text</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-market

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-market-cap-history-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:498</code> | <code>ConfigHelper.market_source("MARKET_CAP_HISTORY_SOURCE") &#124;&#124;</code> |
| <code>interface.14.env-market-coin-cache-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:614</code> | <code>cache_period: ConfigHelper.parse_time_env_var("MARKET_COIN_CACHE_PERIOD", "10m")</code> |
| <code>interface.14.env-market-coin-fetcher-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:612</code> | <code>ConfigHelper.parse_bool_env_var("MARKET_COIN_FETCHER_ENABLED", "true"),</code> |
| <code>interface.14.env-market-coingecko-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:521</code> | <code>System.get_env("MARKET_COINGECKO_API_KEY") &#124;&#124;</code> |
| <code>interface.14.env-market-coingecko-base-pro-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:515</code> | <code>ConfigHelper.parse_url_env_var("MARKET_COINGECKO_BASE_PRO_URL") &#124;&#124;</code> |
| <code>interface.14.env-market-coingecko-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:509</code> | <code>ConfigHelper.parse_url_env_var("MARKET_COINGECKO_BASE_URL") &#124;&#124;</code> |
| <code>interface.14.env-market-coingecko-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:524</code> | <code>System.get_env("MARKET_COINGECKO_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-coingecko-platform-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:506</code> | <code>System.get_env("MARKET_COINGECKO_PLATFORM_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-coingecko-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:527</code> | <code>System.get_env("MARKET_COINGECKO_SECONDARY_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-coinmarketcap-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:539</code> | <code>System.get_env("MARKET_COINMARKETCAP_API_KEY") &#124;&#124;</code> |
| <code>interface.14.env-market-coinmarketcap-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:533</code> | <code>ConfigHelper.parse_url_env_var("MARKET_COINMARKETCAP_BASE_URL") &#124;&#124;</code> |
| <code>interface.14.env-market-coinmarketcap-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:542</code> | <code>System.get_env("MARKET_COINMARKETCAP_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-coinmarketcap-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:545</code> | <code>System.get_env("MARKET_COINMARKETCAP_SECONDARY_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-cryptocompare-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:552</code> | <code>"MARKET_CRYPTOCOMPARE_BASE_URL",</code> |
| <code>interface.14.env-market-cryptocompare-coin-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:555</code> | <code>coin_symbol: System.get_env("MARKET_CRYPTOCOMPARE_COIN_SYMBOL", coin),</code> |
| <code>interface.14.env-market-cryptocompare-secondary-coin-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:557</code> | <code>System.get_env("MARKET_CRYPTOCOMPARE_SECONDARY_COIN_SYMBOL") &#124;&#124;</code> |
| <code>interface.14.env-market-cryptorank-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:572</code> | <code>System.get_env("MARKET_CRYPTORANK_API_KEY") &#124;&#124;</code> |
| <code>interface.14.env-market-cryptorank-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:566</code> | <code>ConfigHelper.parse_url_env_var("MARKET_CRYPTORANK_BASE_URL") &#124;&#124;</code> |
| <code>interface.14.env-market-cryptorank-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:575</code> | <code>System.get_env("MARKET_CRYPTORANK_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-cryptorank-platform-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:563</code> | <code>ConfigHelper.parse_integer_or_nil_env_var("MARKET_CRYPTORANK_PLATFORM_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-cryptorank-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:578</code> | <code>System.get_env("MARKET_CRYPTORANK_SECONDARY_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-defillama-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:583</code> | <code>coin_id: System.get_env("MARKET_DEFILLAMA_COIN_ID"),</code> |
| <code>interface.14.env-market-dia-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:604</code> | <code>base_url: ConfigHelper.parse_url_env_var("MARKET_DIA_BASE_URL", "&lt;url&gt;"),</code> |
| <code>interface.14.env-market-dia-blockchain</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:603</code> | <code>blockchain: System.get_env("MARKET_DIA_BLOCKCHAIN"),</code> |
| <code>interface.14.env-market-dia-coin-address-hash</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:605</code> | <code>coin_address_hash: System.get_env("MARKET_DIA_COIN_ADDRESS_HASH"),</code> |
| <code>interface.14.env-market-dia-secondary-coin-address-hash</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:606</code> | <code>secondary_coin_address_hash: System.get_env("MARKET_DIA_SECONDARY_COIN_ADDRESS_HASH")</code> |
| <code>interface.14.env-market-history-fetch-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:643</code> | <code>history_fetch_interval: ConfigHelper.parse_time_env_var("MARKET_HISTORY_FETCH_INTERVAL", "1h"),</code> |
| <code>interface.14.env-market-history-fetcher-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:642</code> | <code>ConfigHelper.parse_bool_env_var("MARKET_HISTORY_FETCHER_ENABLED", "true"),</code> |
| <code>interface.14.env-market-history-first-fetch-day-count</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:646</code> | <code>"MARKET_HISTORY_FIRST_FETCH_DAY_COUNT",</code> |
| <code>interface.14.env-market-mobula-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:596</code> | <code>api_key: System.get_env("MARKET_MOBULA_API_KEY") &#124;&#124; System.get_env("EXCHANGE_RATES_MOBULA_API_KEY"),</code> |
| <code>interface.14.env-market-mobula-base-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:591</code> | <code>ConfigHelper.parse_url_env_var("MARKET_MOBULA_BASE_URL") &#124;&#124;</code> |
| <code>interface.14.env-market-mobula-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:597</code> | <code>coin_id: System.get_env("MARKET_MOBULA_COIN_ID") &#124;&#124; System.get_env("EXCHANGE_RATES_MOBULA_COIN_ID"),</code> |
| <code>interface.14.env-market-mobula-platform-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:588</code> | <code>System.get_env("MARKET_MOBULA_PLATFORM_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-mobula-secondary-coin-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:599</code> | <code>System.get_env("MARKET_MOBULA_SECONDARY_COIN_ID") &#124;&#124;</code> |
| <code>interface.14.env-market-native-coin-history-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:494</code> | <code>ConfigHelper.market_source("MARKET_NATIVE_COIN_HISTORY_SOURCE") &#124;&#124;</code> |
| <code>interface.14.env-market-native-coin-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:485</code> | <code>ConfigHelper.market_source("MARKET_NATIVE_COIN_SOURCE") &#124;&#124;</code> |
| <code>interface.14.env-market-secondary-coin-history-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:496</code> | <code>secondary_coin_history_source: ConfigHelper.market_source("MARKET_SECONDARY_COIN_HISTORY_SOURCE"),</code> |
| <code>interface.14.env-market-secondary-coin-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:488</code> | <code>ConfigHelper.market_source("MARKET_SECONDARY_COIN_SOURCE") &#124;&#124;</code> |
| <code>interface.14.env-market-tokens-fetcher-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:619</code> | <code>ConfigHelper.parse_bool_env_var("MARKET_TOKENS_FETCHER_ENABLED", "true")</code> |
| <code>interface.14.env-market-tokens-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:625</code> | <code>"MARKET_TOKENS_INTERVAL",</code> |
| <code>interface.14.env-market-tokens-max-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:635</code> | <code>"MARKET_TOKENS_MAX_BATCH_SIZE",</code> |
| <code>interface.14.env-market-tokens-refetch-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:630</code> | <code>"MARKET_TOKENS_REFETCH_INTERVAL",</code> |
| <code>interface.14.env-market-tokens-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:491</code> | <code>ConfigHelper.market_source("MARKET_TOKENS_SOURCE") &#124;&#124;</code> |
| <code>interface.14.env-market-tvl-history-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:501</code> | <code>ConfigHelper.market_source("MARKET_TVL_HISTORY_SOURCE") &#124;&#124;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-market-cap-history-source</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coin-cache-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coin-fetcher-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coingecko-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coingecko-base-pro-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coingecko-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coingecko-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coingecko-platform-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coingecko-secondary-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coinmarketcap-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coinmarketcap-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coinmarketcap-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-coinmarketcap-secondary-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptocompare-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptocompare-coin-symbol</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptocompare-secondary-coin-symbol</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptorank-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptorank-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptorank-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptorank-platform-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-cryptorank-secondary-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-defillama-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-dia-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-dia-blockchain</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-dia-coin-address-hash</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-dia-secondary-coin-address-hash</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-history-fetch-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-history-fetcher-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-history-first-fetch-day-count</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-mobula-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-mobula-base-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-mobula-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-mobula-platform-id</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-market-mobula-secondary-coin-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-native-coin-history-source</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-native-coin-source</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-secondary-coin-history-source</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-secondary-coin-source</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-tokens-fetcher-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-tokens-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-tokens-max-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-tokens-refetch-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-tokens-source</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-market-tvl-history-source</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-metadata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-metadata-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:666</code> | <code>if System.get_env("METADATA_CONTRACT") &amp;&amp; System.get_env("VALIDATORS_CONTRACT") do</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-metadata-contract</code> | notes | read 2x in backend; set in 2 non-backend sites |

## env-microservice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-microservice-account-abstraction-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:788</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MICROSERVICE_ACCOUNT_ABSTRACTION_ENABLED")</code> |
| <code>interface.14.env-microservice-account-abstraction-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:787</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_ACCOUNT_ABSTRACTION_URL"),</code> |
| <code>interface.14.env-microservice-bens-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:783</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MICROSERVICE_BENS_ENABLED"),</code> |
| <code>interface.14.env-microservice-bens-protocols</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:784</code> | <code>protocols: ConfigHelper.parse_list_env_var("MICROSERVICE_BENS_PROTOCOLS")</code> |
| <code>interface.14.env-microservice-bens-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:782</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_BENS_URL"),</code> |
| <code>interface.14.env-microservice-eth-bytecode-db-interval-between-lookups</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:866</code> | <code>"MICROSERVICE_ETH_BYTECODE_DB_INTERVAL_BETWEEN_LOOKUPS",</code> |
| <code>interface.14.env-microservice-eth-bytecode-db-max-lookups-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:869</code> | <code>max_concurrency: ConfigHelper.parse_integer_env_var("MICROSERVICE_ETH_BYTECODE_DB_MAX_LOOKUPS_CONCURRENCY", 10)</code> |
| <code>interface.14.env-microservice-metadata-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:791</code> | <code>service_url: ConfigHelper.parse_microservice_url("MICROSERVICE_METADATA_URL", "MICROSERVICE_METADATA_ENABLED"),</code> |
| <code>interface.14.env-microservice-metadata-proxy-requests-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:794</code> | <code>proxy_requests_timeout: ConfigHelper.parse_time_env_var("MICROSERVICE_METADATA_PROXY_REQUESTS_TIMEOUT", "30s")</code> |
| <code>interface.14.env-microservice-metadata-requests-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:793</code> | <code>requests_timeout: ConfigHelper.parse_time_env_var("MICROSERVICE_METADATA_REQUESTS_TIMEOUT", "1s"),</code> |
| <code>interface.14.env-microservice-metadata-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:791</code> | <code>service_url: ConfigHelper.parse_microservice_url("MICROSERVICE_METADATA_URL", "MICROSERVICE_METADATA_ENABLED"),</code> |
| <code>interface.14.env-microservice-multichain-search-addresses-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:804</code> | <code>"MICROSERVICE_MULTICHAIN_SEARCH_ADDRESSES_CHUNK_SIZE",</code> |
| <code>interface.14.env-microservice-multichain-search-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:800</code> | <code>api_key: System.get_env("MICROSERVICE_MULTICHAIN_SEARCH_API_KEY"),</code> |
| <code>interface.14.env-microservice-multichain-search-counters-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:814</code> | <code>"MICROSERVICE_MULTICHAIN_SEARCH_COUNTERS_CHUNK_SIZE",</code> |
| <code>interface.14.env-microservice-multichain-search-token-info-chunk-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:809</code> | <code>"MICROSERVICE_MULTICHAIN_SEARCH_TOKEN_INFO_CHUNK_SIZE",</code> |
| <code>interface.14.env-microservice-multichain-search-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:32</code> | <code>ConfigHelper.parse_url_env_var("MICROSERVICE_MULTICHAIN_SEARCH_URL")</code> |
| <code>interface.14.env-microservice-sc-verifier-api-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:771</code> | <code>api_key: System.get_env("MICROSERVICE_SC_VERIFIER_API_KEY")</code> |
| <code>interface.14.env-microservice-sc-verifier-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:758</code> | <code>enabled? = ConfigHelper.parse_bool_env_var("MICROSERVICE_SC_VERIFIER_ENABLED", "true")</code> |
| <code>interface.14.env-microservice-sc-verifier-type</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:760</code> | <code>type = System.get_env("MICROSERVICE_SC_VERIFIER_TYPE", "sc_verifier")</code> |
| <code>interface.14.env-microservice-sc-verifier-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:765</code> | <code>"MICROSERVICE_SC_VERIFIER_URL",</code> |
| <code>interface.14.env-microservice-sig-provider-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:779</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MICROSERVICE_SIG_PROVIDER_ENABLED")</code> |
| <code>interface.14.env-microservice-sig-provider-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:778</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_SIG_PROVIDER_URL"),</code> |
| <code>interface.14.env-microservice-stylus-verifier-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:797</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_STYLUS_VERIFIER_URL")</code> |
| <code>interface.14.env-microservice-tac-operation-lifecycle-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:819</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MICROSERVICE_TAC_OPERATION_LIFECYCLE_ENABLED", "true"),</code> |
| <code>interface.14.env-microservice-tac-operation-lifecycle-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:820</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_TAC_OPERATION_LIFECYCLE_URL")</code> |
| <code>interface.14.env-microservice-transaction-interpretation-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:243</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MICROSERVICE_TRANSACTION_INTERPRETATION_ENABLED")</code> |
| <code>interface.14.env-microservice-transaction-interpretation-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:242</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_TRANSACTION_INTERPRETATION_URL"),</code> |
| <code>interface.14.env-microservice-visualize-sol2uml-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:775</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MICROSERVICE_VISUALIZE_SOL2UML_ENABLED")</code> |
| <code>interface.14.env-microservice-visualize-sol2uml-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:774</code> | <code>service_url: ConfigHelper.parse_url_env_var("MICROSERVICE_VISUALIZE_SOL2UML_URL"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-microservice-account-abstraction-enabled</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-microservice-account-abstraction-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-bens-enabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-microservice-bens-protocols</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-bens-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-eth-bytecode-db-interval-between-lookups</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-eth-bytecode-db-max-lookups-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-metadata-enabled</code> | notes | read 2x in backend; set in 9 non-backend sites |
| <code>interface.14.env-microservice-metadata-proxy-requests-timeout</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-microservice-metadata-requests-timeout</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-microservice-metadata-url</code> | notes | read 1x in backend; set in 10 non-backend sites |
| <code>interface.14.env-microservice-multichain-search-addresses-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-multichain-search-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-multichain-search-counters-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-multichain-search-token-info-chunk-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-multichain-search-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-sc-verifier-api-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-sc-verifier-enabled</code> | notes | read 1x in backend; set in 4 non-backend sites |
| <code>interface.14.env-microservice-sc-verifier-type</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-microservice-sc-verifier-url</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-microservice-sig-provider-enabled</code> | notes | read 1x in backend; set in 4 non-backend sites |
| <code>interface.14.env-microservice-sig-provider-url</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-microservice-stylus-verifier-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-tac-operation-lifecycle-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-tac-operation-lifecycle-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-microservice-transaction-interpretation-enabled</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-microservice-transaction-interpretation-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-microservice-visualize-sol2uml-enabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-microservice-visualize-sol2uml-url</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-migration

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-migration-arbitrum-da-records-normalization-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1023</code> | <code>"MIGRATION_ARBITRUM_DA_RECORDS_NORMALIZATION_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-arbitrum-da-records-normalization-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1028</code> | <code>"MIGRATION_ARBITRUM_DA_RECORDS_NORMALIZATION_CONCURRENCY",</code> |
| <code>interface.14.env-migration-backfill-metadata-url-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1091</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_BACKFILL_METADATA_URL_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-migration-backfill-metadata-url-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1092</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_BACKFILL_METADATA_URL_CONCURRENCY", 5)</code> |
| <code>interface.14.env-migration-backfill-metadata-url-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1090</code> | <code>enabled: !ConfigHelper.parse_bool_env_var("MIGRATION_BACKFILL_METADATA_URL_DISABLED"),</code> |
| <code>interface.14.env-migration-backfill-multichain-search-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:996</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_BACKFILL_MULTICHAIN_SEARCH_BATCH_SIZE", 10)</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1109</code> | <code>"MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-check-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1119</code> | <code>"MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_CHECK_INTERVAL",</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1104</code> | <code>"MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_ENABLED",</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-storage-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1114</code> | <code>"MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_STORAGE_PERIOD",</code> |
| <code>interface.14.env-migration-empty-internal-transactions-data-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1058</code> | <code>"MIGRATION_EMPTY_INTERNAL_TRANSACTIONS_DATA_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-empty-internal-transactions-data-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1063</code> | <code>"MIGRATION_EMPTY_INTERNAL_TRANSACTIONS_DATA_CONCURRENCY",</code> |
| <code>interface.14.env-migration-empty-internal-transactions-data-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1066</code> | <code>timeout: ConfigHelper.parse_time_env_var("MIGRATION_EMPTY_INTERNAL_TRANSACTIONS_DATA_TIMEOUT", "0s")</code> |
| <code>interface.14.env-migration-filecoin-pending-address-operations-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1044</code> | <code>"MIGRATION_FILECOIN_PENDING_ADDRESS_OPERATIONS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-filecoin-pending-address-operations-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1049</code> | <code>"MIGRATION_FILECOIN_PENDING_ADDRESS_OPERATIONS_CONCURRENCY",</code> |
| <code>interface.14.env-migration-heavy-index-operations-check-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:999</code> | <code>check_interval: ConfigHelper.parse_time_env_var("MIGRATION_HEAVY_INDEX_OPERATIONS_CHECK_INTERVAL", "10m")</code> |
| <code>interface.14.env-migration-merge-adjacent-missing-block-ranges-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1097</code> | <code>"MIGRATION_MERGE_ADJACENT_MISSING_BLOCK_RANGES_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1566</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_BATCH_SIZE", 50),</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-blocks-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1570</code> | <code>"MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_BLOCKS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1565</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_CONCURRENCY", 5),</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-high-verbosity</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1575</code> | <code>"MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_HIGH_VERBOSITY",</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1567</code> | <code>timeout: ConfigHelper.parse_time_env_var("MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_TIMEOUT", "0s"),</code> |
| <code>interface.14.env-migration-refetch-contract-codes-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:992</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_REFETCH_CONTRACT_CODES_BATCH_SIZE", 100)</code> |
| <code>interface.14.env-migration-refetch-contract-codes-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:991</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_REFETCH_CONTRACT_CODES_CONCURRENCY", 5),</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:942</code> | <code>"MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:947</code> | <code>"MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_CONCURRENCY",</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:957</code> | <code>"MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_ENABLED",</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:952</code> | <code>"MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_TIMEOUT",</code> |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:925</code> | <code>"MIGRATION_REINDEX_DUPLICATED_INTERNAL_TRANSACTIONS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:930</code> | <code>"MIGRATION_REINDEX_DUPLICATED_INTERNAL_TRANSACTIONS_CONCURRENCY",</code> |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:935</code> | <code>"MIGRATION_REINDEX_DUPLICATED_INTERNAL_TRANSACTIONS_TIMEOUT",</code> |
| <code>interface.14.env-migration-reindex-internal-transactions-status-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:908</code> | <code>"MIGRATION_REINDEX_INTERNAL_TRANSACTIONS_STATUS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-reindex-internal-transactions-status-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:913</code> | <code>"MIGRATION_REINDEX_INTERNAL_TRANSACTIONS_STATUS_CONCURRENCY",</code> |
| <code>interface.14.env-migration-reindex-internal-transactions-status-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:918</code> | <code>"MIGRATION_REINDEX_INTERNAL_TRANSACTIONS_STATUS_TIMEOUT",</code> |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:969</code> | <code>"MIGRATION_RESTORE_OMITTED_WETH_TOKEN_TRANSFERS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:964</code> | <code>"MIGRATION_RESTORE_OMITTED_WETH_TOKEN_TRANSFERS_CONCURRENCY",</code> |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:974</code> | <code>"MIGRATION_RESTORE_OMITTED_WETH_TOKEN_TRANSFERS_TIMEOUT",</code> |
| <code>interface.14.env-migration-sanitize-duplicated-log-index-logs-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:986</code> | <code>"MIGRATION_SANITIZE_DUPLICATED_LOG_INDEX_LOGS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-sanitize-duplicated-log-index-logs-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:981</code> | <code>"MIGRATION_SANITIZE_DUPLICATED_LOG_INDEX_LOGS_CONCURRENCY",</code> |
| <code>interface.14.env-migration-sanitize-empty-contract-code-addresses-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1010</code> | <code>"MIGRATION_SANITIZE_EMPTY_CONTRACT_CODE_ADDRESSES_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-sanitize-empty-contract-code-addresses-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1015</code> | <code>"MIGRATION_SANITIZE_EMPTY_CONTRACT_CODE_ADDRESSES_CONCURRENCY",</code> |
| <code>interface.14.env-migration-sanitize-incorrect-nft-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:896</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_SANITIZE_INCORRECT_NFT_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-migration-sanitize-incorrect-nft-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:897</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_SANITIZE_INCORRECT_NFT_CONCURRENCY", 1),</code> |
| <code>interface.14.env-migration-sanitize-incorrect-nft-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:898</code> | <code>timeout: ConfigHelper.parse_time_env_var("MIGRATION_SANITIZE_INCORRECT_NFT_TIMEOUT", "0s")</code> |
| <code>interface.14.env-migration-sanitize-incorrect-weth-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:901</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_SANITIZE_INCORRECT_WETH_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-migration-sanitize-incorrect-weth-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:902</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_SANITIZE_INCORRECT_WETH_CONCURRENCY", 1),</code> |
| <code>interface.14.env-migration-sanitize-incorrect-weth-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:903</code> | <code>timeout: ConfigHelper.parse_time_env_var("MIGRATION_SANITIZE_INCORRECT_WETH_TIMEOUT", "0s")</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1003</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_SANITIZE_VERIFIED_ADDRESSES_BATCH_SIZE", 500),</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1004</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_SANITIZE_VERIFIED_ADDRESSES_CONCURRENCY", 1),</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1002</code> | <code>enabled: !ConfigHelper.parse_bool_env_var("MIGRATION_SANITIZE_VERIFIED_ADDRESSES_DISABLED"),</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1005</code> | <code>timeout: ConfigHelper.parse_time_env_var("MIGRATION_SANITIZE_VERIFIED_ADDRESSES_TIMEOUT", "0s")</code> |
| <code>interface.14.env-migration-shrink-internal-transactions-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1081</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_SHRINK_INTERNAL_TRANSACTIONS_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-migration-shrink-internal-transactions-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1082</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_SHRINK_INTERNAL_TRANSACTIONS_CONCURRENCY", 10)</code> |
| <code>interface.14.env-migration-smart-contract-language-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1086</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_SMART_CONTRACT_LANGUAGE_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-migration-smart-contract-language-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1087</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_SMART_CONTRACT_LANGUAGE_CONCURRENCY", 1)</code> |
| <code>interface.14.env-migration-smart-contract-language-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1085</code> | <code>enabled: !ConfigHelper.parse_bool_env_var("MIGRATION_SMART_CONTRACT_LANGUAGE_DISABLED"),</code> |
| <code>interface.14.env-migration-token-instance-erc-1155-sanitize-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1514</code> | <code>"MIGRATION_TOKEN_INSTANCE_ERC_1155_SANITIZE_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-token-instance-erc-1155-sanitize-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1509</code> | <code>"MIGRATION_TOKEN_INSTANCE_ERC_1155_SANITIZE_CONCURRENCY",</code> |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1520</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_TOKEN_INSTANCE_ERC_721_SANITIZE_BATCH_SIZE", 50),</code> |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1519</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_TOKEN_INSTANCE_ERC_721_SANITIZE_CONCURRENCY", 2),</code> |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-tokens-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1523</code> | <code>"MIGRATION_TOKEN_INSTANCE_ERC_721_SANITIZE_TOKENS_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-token-instance-owner-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:876</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_TOKEN_INSTANCE_OWNER_BATCH_SIZE", 50),</code> |
| <code>interface.14.env-migration-token-instance-owner-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:875</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_TOKEN_INSTANCE_OWNER_CONCURRENCY", 5),</code> |
| <code>interface.14.env-migration-token-instance-owner-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:877</code> | <code>enabled: ConfigHelper.parse_bool_env_var("MIGRATION_TOKEN_INSTANCE_OWNER_ENABLED")</code> |
| <code>interface.14.env-migration-token-transfer-token-type-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:892</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIGRATION_TOKEN_TRANSFER_TOKEN_TYPE_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-migration-token-transfer-token-type-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:893</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("MIGRATION_TOKEN_TRANSFER_TOKEN_TYPE_CONCURRENCY", 1)</code> |
| <code>interface.14.env-migration-transactions-table-denormalization-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:882</code> | <code>"MIGRATION_TRANSACTIONS_TABLE_DENORMALIZATION_BATCH_SIZE",</code> |
| <code>interface.14.env-migration-transactions-table-denormalization-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:887</code> | <code>"MIGRATION_TRANSACTIONS_TABLE_DENORMALIZATION_CONCURRENCY",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-migration-arbitrum-da-records-normalization-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-arbitrum-da-records-normalization-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-backfill-metadata-url-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-backfill-metadata-url-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-backfill-metadata-url-disabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-backfill-multichain-search-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-check-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-storage-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-empty-internal-transactions-data-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-empty-internal-transactions-data-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-empty-internal-transactions-data-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-filecoin-pending-address-operations-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-filecoin-pending-address-operations-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-heavy-index-operations-check-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-merge-adjacent-missing-block-ranges-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-recovery-weth-token-transfers-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-recovery-weth-token-transfers-blocks-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-recovery-weth-token-transfers-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-recovery-weth-token-transfers-high-verbosity</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-recovery-weth-token-transfers-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-refetch-contract-codes-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-refetch-contract-codes-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-internal-transactions-status-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-internal-transactions-status-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-reindex-internal-transactions-status-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-duplicated-log-index-logs-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-duplicated-log-index-logs-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-empty-contract-code-addresses-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-empty-contract-code-addresses-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-incorrect-nft-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-incorrect-nft-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-incorrect-nft-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-incorrect-weth-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-incorrect-weth-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-incorrect-weth-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-verified-addresses-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-verified-addresses-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-verified-addresses-disabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-sanitize-verified-addresses-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-shrink-internal-transactions-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-shrink-internal-transactions-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-smart-contract-language-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-smart-contract-language-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-smart-contract-language-disabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-erc-1155-sanitize-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-erc-1155-sanitize-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-tokens-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-owner-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-owner-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-instance-owner-enabled</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-migration-token-transfer-token-type-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-token-transfer-token-type-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-transactions-table-denormalization-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-migration-transactions-table-denormalization-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-min

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-min-missing-block-number-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:852</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("MIN_MISSING_BLOCK_NUMBER_BATCH_SIZE", 100_000)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-min-missing-block-number-batch-size</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-missing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-missing-balance-of-tokens-window-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1131</code> | <code>window_size: ConfigHelper.parse_integer_env_var("MISSING_BALANCE_OF_TOKENS_WINDOW_SIZE", 100)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-missing-balance-of-tokens-window-size</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-mixpanel

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-mixpanel-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:76</code> | <code>mixpanel_url: ConfigHelper.parse_url_env_var("MIXPANEL_URL", "&lt;url&gt;"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-mixpanel-url</code> | notes | read 1x in backend; set in 3 non-backend sites |

## env-mud

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-mud-database-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:94</code> | <code>database_mud = if System.get_env("MUD_DATABASE_URL"), do: nil, else: database</code> |
| <code>interface.14.env-mud-indexer-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1195</code> | <code>config :explorer, Explorer.Chain.Mud, enabled: ConfigHelper.parse_bool_env_var("MUD_INDEXER_ENABLED")</code> |
| <code>interface.14.env-mud-pool-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:102</code> | <code>pool_size: ConfigHelper.parse_integer_env_var("MUD_POOL_SIZE", 10),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-mud-database-url</code> | notes | read 3x in backend; set in 1 non-backend sites |
| <code>interface.14.env-mud-indexer-enabled</code> | notes | read 3x in backend; set in 10 non-backend sites |
| <code>interface.14.env-mud-pool-size</code> | notes | read 2x in backend; set in 1 non-backend sites |

## env-network

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-network</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:117</code> | <code>network: System.get_env("NETWORK"),</code> |
| <code>interface.14.env-network-icon</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:119</code> | <code>network_icon: System.get_env("NETWORK_ICON"),</code> |
| <code>interface.14.env-network-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:87</code> | <code>"NETWORK_PATH"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-network</code> | notes | read 1x in backend; set in 58 non-backend sites; 18 further callers omitted |
| <code>interface.14.env-network-icon</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-network-path</code> | notes | read 2x in backend; set in 1 non-backend sites |

## env-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-new-tags</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:56</code> | <code>new_tags: System.get_env("NEW_TAGS"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-new-tags</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-nft

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-nft-media-handler-aws-access-key-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2166</code> | <code>access_key_id: System.get_env("NFT_MEDIA_HANDLER_AWS_ACCESS_KEY_ID"),</code> |
| <code>interface.14.env-nft-media-handler-aws-bucket-host</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2171</code> | <code>host: System.get_env("NFT_MEDIA_HANDLER_AWS_BUCKET_HOST"),</code> |
| <code>interface.14.env-nft-media-handler-aws-bucket-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2174</code> | <code>bucket_name: System.get_env("NFT_MEDIA_HANDLER_AWS_BUCKET_NAME")</code> |
| <code>interface.14.env-nft-media-handler-aws-public-bucket-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2173</code> | <code>public_r2_url: ConfigHelper.parse_url_env_var("NFT_MEDIA_HANDLER_AWS_PUBLIC_BUCKET_URL", nil, false),</code> |
| <code>interface.14.env-nft-media-handler-aws-secret-access-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2167</code> | <code>secret_access_key: System.get_env("NFT_MEDIA_HANDLER_AWS_SECRET_ACCESS_KEY")</code> |
| <code>interface.14.env-nft-media-handler-backfill-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2197</code> | <code>enabled?: ConfigHelper.parse_bool_env_var("NFT_MEDIA_HANDLER_BACKFILL_ENABLED"),</code> |
| <code>interface.14.env-nft-media-handler-backfill-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2201</code> | <code>"NFT_MEDIA_HANDLER_BACKFILL_ENQUEUE_BUSY_WAITING_TIMEOUT",</code> |
| <code>interface.14.env-nft-media-handler-backfill-queue-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2198</code> | <code>queue_size: ConfigHelper.parse_integer_env_var("NFT_MEDIA_HANDLER_BACKFILL_QUEUE_SIZE", 1_000),</code> |
| <code>interface.14.env-nft-media-handler-bucket-folder</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2188</code> | <code>r2_folder: ConfigHelper.parse_path_env_var("NFT_MEDIA_HANDLER_BUCKET_FOLDER"),</code> |
| <code>interface.14.env-nft-media-handler-cache-uniqueness-max-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2194</code> | <code>cache_uniqueness_max_size: ConfigHelper.parse_integer_env_var("NFT_MEDIA_HANDLER_CACHE_UNIQUENESS_MAX_SIZE", 100_000)</code> |
| <code>interface.14.env-nft-media-handler-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2176</code> | <code>nmh_enabled? = ConfigHelper.parse_bool_env_var("NFT_MEDIA_HANDLER_ENABLED")</code> |
| <code>interface.14.env-nft-media-handler-is-worker</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2181</code> | <code>nmh_worker? = ConfigHelper.parse_bool_env_var("NFT_MEDIA_HANDLER_IS_WORKER")</code> |
| <code>interface.14.env-nft-media-handler-remote-dispatcher-node-mode-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2179</code> | <code>ConfigHelper.parse_bool_env_var("NFT_MEDIA_HANDLER_REMOTE_DISPATCHER_NODE_MODE_ENABLED")</code> |
| <code>interface.14.env-nft-media-handler-worker-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2191</code> | <code>worker_batch_size: ConfigHelper.parse_integer_env_var("NFT_MEDIA_HANDLER_WORKER_BATCH_SIZE", 10),</code> |
| <code>interface.14.env-nft-media-handler-worker-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2190</code> | <code>worker_concurrency: ConfigHelper.parse_integer_env_var("NFT_MEDIA_HANDLER_WORKER_CONCURRENCY", 10),</code> |
| <code>interface.14.env-nft-media-handler-worker-spawn-tasks-timeout</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:2192</code> | <code>worker_spawn_tasks_timeout: ConfigHelper.parse_time_env_var("NFT_MEDIA_HANDLER_WORKER_SPAWN_TASKS_TIMEOUT", "100ms"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-nft-media-handler-aws-access-key-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-aws-bucket-host</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-aws-bucket-name</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-aws-public-bucket-url</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-nft-media-handler-aws-secret-access-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-backfill-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-backfill-enqueue-busy-waiting-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-backfill-queue-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-bucket-folder</code> | notes | read 1x in backend; set in 21 non-backend sites |
| <code>interface.14.env-nft-media-handler-cache-uniqueness-max-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-enabled</code> | notes | read 1x in backend; set in 3 non-backend sites |
| <code>interface.14.env-nft-media-handler-is-worker</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-remote-dispatcher-node-mode-enabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-nft-media-handler-worker-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-worker-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-nft-media-handler-worker-spawn-tasks-timeout</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-noves

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-noves-fi-api-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:736</code> | <code>api_key: System.get_env("NOVES_FI_API_TOKEN")</code> |
| <code>interface.14.env-noves-fi-base-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:734</code> | <code>service_url: ConfigHelper.parse_url_env_var("NOVES_FI_BASE_API_URL", "&lt;url&gt;"),</code> |
| <code>interface.14.env-noves-fi-chain-name</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:735</code> | <code>chain_name: System.get_env("NOVES_FI_CHAIN_NAME"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-noves-fi-api-token</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-noves-fi-base-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-noves-fi-chain-name</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-paxeer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-paxeer-x-capabilities-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:357</code> | <code>enabled: ConfigHelper.parse_bool_env_var("PAXEER_X_CAPABILITIES_ENABLED"),</code> |
| <code>interface.14.env-paxeer-x-capabilities-refresh-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:358</code> | <code>refresh_interval_seconds: ConfigHelper.parse_integer_env_var("PAXEER_X_CAPABILITIES_REFRESH_SECONDS", 30)</code> |
| <code>interface.14.env-paxeer-x-cometbft-rpc-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1941</code> | <code>cometbft_rpc_urls: paxeer_x_url_list.("PAXEER_X_COMETBFT_RPC_URLS"),</code> |
| <code>interface.14.env-paxeer-x-cosmos-rest-urls</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1940</code> | <code>cosmos_rest_urls: paxeer_x_url_list.("PAXEER_X_COSMOS_REST_URLS"),</code> |
| <code>interface.14.env-paxeer-x-cosmos-transaction-type</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:294</code> | <code>cosmos_transaction_type: ConfigHelper.parse_integer_env_var("PAXEER_X_COSMOS_TRANSACTION_TYPE", 127),</code> |
| <code>interface.14.env-paxeer-x-final-confirmations</code> | <code>env_var</code> | <code>change</code> | <code>services/explorer/backend/config/runtime.exs:659</code> | <code>final_confirmations: ConfigHelper.parse_integer_env_var("PAXEER_X_FINAL_CONFIRMATIONS", 0)</code> |
| <code>interface.14.env-paxeer-x-finality-cache-ms</code> | <code>env_var</code> | <code>change</code> | <code>services/explorer/backend/config/runtime.exs:657</code> | <code>cache_ms: ConfigHelper.parse_integer_env_var("PAXEER_X_FINALITY_CACHE_MS", 1000),</code> |
| <code>interface.14.env-paxeer-x-finality-probe-batches</code> | <code>env_var</code> | <code>change</code> | <code>services/explorer/backend/config/runtime.exs:658</code> | <code>probe_batches: ConfigHelper.parse_integer_env_var("PAXEER_X_FINALITY_PROBE_BATCHES", 16),</code> |
| <code>interface.14.env-paxeer-x-native-coin-decimals</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:295</code> | <code>native_coin_decimals: ConfigHelper.parse_integer_env_var("PAXEER_X_NATIVE_COIN_DECIMALS", 18)</code> |
| <code>interface.14.env-paxeer-x-price-feed-concurrency</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1943</code> | <code>concurrency: ConfigHelper.parse_integer_env_var("PAXEER_X_PRICE_FEED_CONCURRENCY", 4),</code> |
| <code>interface.14.env-paxeer-x-price-feed-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1911</code> | <code>paxeer_x_price_feed_enabled? = ConfigHelper.parse_bool_env_var("PAXEER_X_PRICE_FEED_ENABLED")</code> |
| <code>interface.14.env-paxeer-x-price-feed-markets-refresh-seconds</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1944</code> | <code>markets_refresh_seconds: ConfigHelper.parse_integer_env_var("PAXEER_X_PRICE_FEED_MARKETS_REFRESH_SECONDS", 300),</code> |
| <code>interface.14.env-paxeer-x-price-feed-precompile</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1946</code> | <code>System.get_env("PAXEER_X_PRICE_FEED_PRECOMPILE", "0x0000000000000000000000000000000000001020")</code> |
| <code>interface.14.env-paxeer-x-price-feed-retention-days</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1949</code> | <code>paxeer_x_price_feed_retention_days = ConfigHelper.parse_integer_or_nil_env_var("PAXEER_X_PRICE_FEED_RETENTION_DAYS")</code> |
| <code>interface.14.env-paxeer-x-price-feed-start-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1930</code> | <code>paxeer_x_price_feed_start_block = ConfigHelper.parse_integer_or_nil_env_var("PAXEER_X_PRICE_FEED_START_BLOCK")</code> |
| <code>interface.14.env-paxeer-x-router-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:298</code> | <code>router_url: ConfigHelper.parse_url_env_var("PAXEER_X_ROUTER_URL")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-paxeer-x-capabilities-enabled</code> | notes | read 1x in backend; set in 5 non-backend sites |
| <code>interface.14.env-paxeer-x-capabilities-refresh-seconds</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-paxeer-x-cometbft-rpc-urls</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-cosmos-rest-urls</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-cosmos-transaction-type</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-paxeer-x-final-confirmations</code> | notes | read 1x in backend; set in 3 non-backend sites; written to config at services/explorer/backend/config/runtime.exs:659 but no module reads the resulting keys |
| <code>interface.14.env-paxeer-x-finality-cache-ms</code> | notes | read 1x in backend; set in 2 non-backend sites; written to config at services/explorer/backend/config/runtime.exs:657 but no module reads the resulting keys |
| <code>interface.14.env-paxeer-x-finality-probe-batches</code> | notes | read 1x in backend; set in 2 non-backend sites; written to config at services/explorer/backend/config/runtime.exs:658 but no module reads the resulting keys |
| <code>interface.14.env-paxeer-x-native-coin-decimals</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-paxeer-x-price-feed-concurrency</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-price-feed-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-price-feed-markets-refresh-seconds</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-price-feed-precompile</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-price-feed-retention-days</code> | notes | read 2x in backend; set in 2 non-backend sites |
| <code>interface.14.env-paxeer-x-price-feed-start-block</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-paxeer-x-router-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-permanent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-permanent-dark-mode-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:60</code> | <code>permanent_dark_mode_enabled: ConfigHelper.parse_bool_env_var("PERMANENT_DARK_MODE_ENABLED"),</code> |
| <code>interface.14.env-permanent-light-mode-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:61</code> | <code>permanent_light_mode_enabled: ConfigHelper.parse_bool_env_var("PERMANENT_LIGHT_MODE_ENABLED"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-permanent-dark-mode-enabled</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-permanent-light-mode-enabled</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-pool

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-pool-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:59</code> | <code>do: ConfigHelper.parse_integer_env_var("POOL_SIZE", 30),</code> |
| <code>interface.14.env-pool-size-api</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:80</code> | <code>pool_size: ConfigHelper.parse_integer_env_var("POOL_SIZE_API", 10),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-pool-size</code> | notes | read 3x in backend; set in 7 non-backend sites |
| <code>interface.14.env-pool-size-api</code> | notes | read 2x in backend; set in 2 non-backend sites |

## env-port

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-port</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex:110</code> | <code>port = System.get_env("PORT") &#124;&#124; raise "expected the PORT environment variable to be set"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-port</code> | notes | read 4x in backend; set in 116 non-backend sites; 78 further callers omitted |

## env-precompiled

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-precompiled-contracts-config-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:394</code> | <code>precompiled_config_path: System.get_env("PRECOMPILED_CONTRACTS_CONFIG_PATH", precompiled_config_default_path)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-precompiled-contracts-config-path</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-public

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-public-metrics-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1138</code> | <code>enabled: ConfigHelper.parse_bool_env_var("PUBLIC_METRICS_ENABLED", "false"),</code> |
| <code>interface.14.env-public-metrics-update-period-hours</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1139</code> | <code>update_period_hours: ConfigHelper.parse_integer_env_var("PUBLIC_METRICS_UPDATE_PERIOD_HOURS", 24)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-public-metrics-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-public-metrics-update-period-hours</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-rate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-rate-limiter-on-demand-exponential-timeout-coeff</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1169</code> | <code>exp_timeout_coeff: ConfigHelper.parse_integer_env_var("RATE_LIMITER_ON_DEMAND_EXPONENTIAL_TIMEOUT_COEFF", 100),</code> |
| <code>interface.14.env-rate-limiter-on-demand-limit-by-ip</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1168</code> | <code>limit_by_ip: ConfigHelper.parse_integer_env_var("RATE_LIMITER_ON_DEMAND_LIMIT_BY_IP", 50),</code> |
| <code>interface.14.env-rate-limiter-on-demand-limitation-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1171</code> | <code>limitation_period: ConfigHelper.parse_time_env_var("RATE_LIMITER_ON_DEMAND_LIMITATION_PERIOD", "1h")</code> |
| <code>interface.14.env-rate-limiter-on-demand-max-ban-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1170</code> | <code>max_ban_interval: ConfigHelper.parse_time_env_var("RATE_LIMITER_ON_DEMAND_MAX_BAN_INTERVAL", "1h"),</code> |
| <code>interface.14.env-rate-limiter-on-demand-time-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1167</code> | <code>time_interval_limit: ConfigHelper.parse_time_env_var("RATE_LIMITER_ON_DEMAND_TIME_INTERVAL", "5s"),</code> |
| <code>interface.14.env-rate-limiter-redis-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1161</code> | <code>rate_limiter_redis_url = ConfigHelper.parse_url_env_var("RATE_LIMITER_REDIS_URL")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-rate-limiter-on-demand-exponential-timeout-coeff</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-rate-limiter-on-demand-limit-by-ip</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-rate-limiter-on-demand-limitation-period</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-rate-limiter-on-demand-max-ban-interval</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-rate-limiter-on-demand-time-interval</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-rate-limiter-redis-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-re

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-re-captcha-bypass-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:77</code> | <code>bypass_token: ConfigHelper.safe_get_env("RE_CAPTCHA_BYPASS_TOKEN", nil),</code> |
| <code>interface.14.env-re-captcha-check-hostname</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:75</code> | <code>check_hostname?: ConfigHelper.parse_bool_env_var("RE_CAPTCHA_CHECK_HOSTNAME", "true"),</code> |
| <code>interface.14.env-re-captcha-client-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:70</code> | <code>v2_client_key: System.get_env("RE_CAPTCHA_CLIENT_KEY"),</code> |
| <code>interface.14.env-re-captcha-disabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:74</code> | <code>is_disabled: ConfigHelper.parse_bool_env_var("RE_CAPTCHA_DISABLED"),</code> |
| <code>interface.14.env-re-captcha-score-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:76</code> | <code>score_threshold: ConfigHelper.parse_float_env_var("RE_CAPTCHA_SCORE_THRESHOLD", "0.5"),</code> |
| <code>interface.14.env-re-captcha-secret-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:71</code> | <code>v2_secret_key: System.get_env("RE_CAPTCHA_SECRET_KEY"),</code> |
| <code>interface.14.env-re-captcha-token-instance-refetch-metadata-scoped-bypass-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:81</code> | <code>"RE_CAPTCHA_TOKEN_INSTANCE_REFETCH_METADATA_SCOPED_BYPASS_TOKEN",</code> |
| <code>interface.14.env-re-captcha-v3-client-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:72</code> | <code>v3_client_key: System.get_env("RE_CAPTCHA_V3_CLIENT_KEY"),</code> |
| <code>interface.14.env-re-captcha-v3-secret-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:73</code> | <code>v3_secret_key: System.get_env("RE_CAPTCHA_V3_SECRET_KEY"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-re-captcha-bypass-token</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-check-hostname</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-client-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-disabled</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-re-captcha-score-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-secret-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-token-instance-refetch-metadata-scoped-bypass-token</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-v3-client-key</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-re-captcha-v3-secret-key</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-release-link</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:44</code> | <code>release_link: System.get_env("RELEASE_LINK"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-release-link</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-replica

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-replica-max-lag</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:353</code> | <code>replica_max_lag: ConfigHelper.parse_time_env_var("REPLICA_MAX_LAG", "5m"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-replica-max-lag</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-rewards

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-rewards-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:393</code> | <code>rewards_contract_address: System.get_env("REWARDS_CONTRACT", "0xeca443e8e1ab29971a45a9c57a6a9875701698a5"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-rewards-contract</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-rootstock

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-rootstock-bridge-address</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:654</code> | <code>rootstock_bridge_address: System.get_env("ROOTSTOCK_BRIDGE_ADDRESS")</code> |
| <code>interface.14.env-rootstock-locked-btc-cache-period</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:434</code> | <code>global_ttl: ConfigHelper.parse_time_env_var("ROOTSTOCK_LOCKED_BTC_CACHE_PERIOD", "10m"),</code> |
| <code>interface.14.env-rootstock-locking-cap</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:435</code> | <code>locking_cap: ConfigHelper.parse_integer_env_var("ROOTSTOCK_LOCKING_CAP", 21_000_000)</code> |
| <code>interface.14.env-rootstock-remasc-address</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:653</code> | <code>rootstock_remasc_address: System.get_env("ROOTSTOCK_REMASC_ADDRESS"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-rootstock-bridge-address</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-rootstock-locked-btc-cache-period</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-rootstock-locking-cap</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-rootstock-remasc-address</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-scroll

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-scroll-l1-base-fee-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1203</code> | <code>l1_base_fee_init: ConfigHelper.parse_integer_env_var("SCROLL_L1_BASE_FEE_INIT", 0),</code> |
| <code>interface.14.env-scroll-l1-blob-base-fee-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1204</code> | <code>l1_blob_base_fee_init: ConfigHelper.parse_integer_env_var("SCROLL_L1_BLOB_BASE_FEE_INIT", 0)</code> |
| <code>interface.14.env-scroll-l1-blob-scalar-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1202</code> | <code>blob_scalar_init: ConfigHelper.parse_integer_env_var("SCROLL_L1_BLOB_SCALAR_INIT", 0),</code> |
| <code>interface.14.env-scroll-l1-commit-scalar-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1201</code> | <code>commit_scalar_init: ConfigHelper.parse_integer_env_var("SCROLL_L1_COMMIT_SCALAR_INIT", 0),</code> |
| <code>interface.14.env-scroll-l1-overhead-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1200</code> | <code>overhead_init: ConfigHelper.parse_integer_env_var("SCROLL_L1_OVERHEAD_INIT", 0),</code> |
| <code>interface.14.env-scroll-l1-scalar-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1199</code> | <code>scalar_init: ConfigHelper.parse_integer_env_var("SCROLL_L1_SCALAR_INIT", 0),</code> |
| <code>interface.14.env-scroll-l2-curie-upgrade-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1198</code> | <code>curie_upgrade_block: ConfigHelper.parse_integer_env_var("SCROLL_L2_CURIE_UPGRADE_BLOCK", 0),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-scroll-l1-base-fee-init</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-scroll-l1-blob-base-fee-init</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-scroll-l1-blob-scalar-init</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-scroll-l1-commit-scalar-init</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-scroll-l1-overhead-init</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-scroll-l1-scalar-init</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-scroll-l2-curie-upgrade-block</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-secret

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-secret-key-base</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime/dev.exs:14</code> | <code>System.get_env("SECRET_KEY_BASE") &#124;&#124; "RMgI4C1HSkxsEjdhtGMfwAHfyT6CKWXOgzCboJflfSm4jeAlic52io05KB6mqzc5",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-secret-key-base</code> | notes | read 4x in backend; set in 6 non-backend sites |

## env-session

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-session-cookie-domain</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/config/config.exs:16</code> | <code>cookie_domain: System.get_env("SESSION_COOKIE_DOMAIN"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-session-cookie-domain</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-show

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-show-address-marketcap-percentage</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:45</code> | <code>show_percentage: ConfigHelper.parse_bool_env_var("SHOW_ADDRESS_MARKETCAP_PERCENTAGE", "true"),</code> |
| <code>interface.14.env-show-maintenance-alert</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:123</code> | <code>show_maintenance_alert: ConfigHelper.parse_bool_env_var("SHOW_MAINTENANCE_ALERT"),</code> |
| <code>interface.14.env-show-price-chart</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:215</code> | <code>if ConfigHelper.parse_bool_env_var("SHOW_PRICE_CHART") do</code> |
| <code>interface.14.env-show-price-chart-legend</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:223</code> | <code>ConfigHelper.parse_bool_env_var("SHOW_PRICE_CHART_LEGEND")</code> |
| <code>interface.14.env-show-tenderly-link</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:65</code> | <code>show_tenderly_link: ConfigHelper.parse_bool_env_var("SHOW_TENDERLY_LINK"),</code> |
| <code>interface.14.env-show-testnet-label</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:124</code> | <code>enable_testnet_label: ConfigHelper.parse_bool_env_var("SHOW_TESTNET_LABEL"),</code> |
| <code>interface.14.env-show-txs-chart</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:226</code> | <code>if ConfigHelper.parse_bool_env_var("SHOW_TXS_CHART", "true") do</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-show-address-marketcap-percentage</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-show-maintenance-alert</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-show-price-chart</code> | notes | read 2x in backend; set in 1 non-backend sites |
| <code>interface.14.env-show-price-chart-legend</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-show-tenderly-link</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-show-testnet-label</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-show-txs-chart</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-shrink

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-shrink-internal-transactions-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:352</code> | <code>shrink_internal_transactions_enabled: ConfigHelper.parse_bool_env_var("SHRINK_INTERNAL_TRANSACTIONS_ENABLED"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-shrink-internal-transactions-enabled</code> | notes | read 3x in backend; set in 1 non-backend sites |

## env-solidityscan

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-solidityscan-api-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:731</code> | <code>api_key: System.get_env("SOLIDITYSCAN_API_TOKEN")</code> |
| <code>interface.14.env-solidityscan-chain-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:730</code> | <code>chain_id: System.get_env("SOLIDITYSCAN_CHAIN_ID"),</code> |
| <code>interface.14.env-solidityscan-platform-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:729</code> | <code>platform_id: System.get_env("SOLIDITYSCAN_PLATFORM_ID", "16"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-solidityscan-api-token</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-solidityscan-chain-id</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-solidityscan-platform-id</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-sourcify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-sourcify-integration-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:724</code> | <code>enabled: ConfigHelper.parse_bool_env_var("SOURCIFY_INTEGRATION_ENABLED"),</code> |
| <code>interface.14.env-sourcify-repo-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:726</code> | <code>repo_url: ConfigHelper.parse_url_env_var("SOURCIFY_REPO_URL", "&lt;url&gt;")</code> |
| <code>interface.14.env-sourcify-server-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:723</code> | <code>server_url: ConfigHelper.parse_url_env_var("SOURCIFY_SERVER_URL", "&lt;url&gt;"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-sourcify-integration-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-sourcify-repo-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-sourcify-server-url</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-spandex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-spandex-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:855</code> | <code>batch_size: ConfigHelper.parse_integer_env_var("SPANDEX_BATCH_SIZE", 100),</code> |
| <code>interface.14.env-spandex-sync-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:856</code> | <code>sync_threshold: ConfigHelper.parse_integer_env_var("SPANDEX_SYNC_THRESHOLD", 100)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-spandex-batch-size</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-spandex-sync-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-suave

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-suave-bid-contracts</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:652</code> | <code>suave_bid_contracts: System.get_env("SUAVE_BID_CONTRACTS", ""),</code> |
| <code>interface.14.env-suave-database-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex:36</code> | <code>do: UtilsConfigHelper.parse_url_env_var("SUAVE_DATABASE_URL") &#124;&#124; UtilsConfigHelper.parse_url_env_var("DATABASE_URL")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-suave-bid-contracts</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-suave-database-url</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-subnetwork

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-subnetwork</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:118</code> | <code>subnetwork: System.get_env("SUBNETWORK"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-subnetwork</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-supply

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-supply-module</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:680</code> | <code>case System.get_env("SUPPLY_MODULE") do</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-supply-module</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-supported

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-supported-chains</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:47</code> | <code>other_networks: System.get_env("SUPPORTED_CHAINS"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-supported-chains</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-tenderly

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-tenderly-chain-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/transaction_view.ex:588</code> | <code>System.get_env("TENDERLY_CHAIN_PATH") &#124;&#124; "/"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-tenderly-chain-path</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-test

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-test-database-read-only-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/test.exs:53</code> | <code>account_database_url = System.get_env("TEST_DATABASE_READ_ONLY_API_URL") &#124;&#124; database_url</code> |
| <code>interface.14.env-test-database-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/config/test.exs:6</code> | <code>database_url = ConfigHelper.parse_url_env_var("TEST_DATABASE_URL")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-test-database-read-only-api-url</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-test-database-url</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-testnet

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-testnet-label-text</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:125</code> | <code>testnet_label_text: System.get_env("TESTNET_LABEL_TEXT", "Testnet")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-testnet-label-text</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |

## env-tests

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-test-cargo-target-dir</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:649</code> | <code>os.environ.get("CARGO_TARGET_DIR")</code> |
| <code>interface.14.env-test-layerx-build-dir</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:18</code> | <code>os.environ.get("LAYERX_BUILD_DIR")</code> |
| <code>interface.14.env-test-layerx-explorer-authority-bearer-token</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:590</code> | <code>"LAYERX_EXPLORER_AUTHORITY_BEARER_TOKEN"</code> |
| <code>interface.14.env-test-layerx-explorer-authority-ca-der</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:594</code> | <code>"LAYERX_EXPLORER_AUTHORITY_CA_DER"</code> |
| <code>interface.14.env-test-layerx-explorer-authority-endpoint</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:593</code> | <code>"LAYERX_EXPLORER_AUTHORITY_ENDPOINT"</code> |
| <code>interface.14.env-test-layerx-explorer-authority-replica-id</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:595</code> | <code>"LAYERX_EXPLORER_AUTHORITY_REPLICA_ID"</code> |
| <code>interface.14.env-test-layerx-explorer-deployment-journal</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:597</code> | <code>"LAYERX_EXPLORER_DEPLOYMENT_JOURNAL"</code> |
| <code>interface.14.env-test-layerx-explorer-finality-policy</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:601</code> | <code>"LAYERX_EXPLORER_FINALITY_POLICY"</code> |
| <code>interface.14.env-test-layerx-explorer-genesis-trust</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:600</code> | <code>"LAYERX_EXPLORER_GENESIS_TRUST"</code> |
| <code>interface.14.env-test-layerx-explorer-index-bin</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:646</code> | <code>os.environ.get("LAYERX_EXPLORER_INDEX_BIN")</code> |
| <code>interface.14.env-test-layerx-explorer-ingest-cursor</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:615</code> | <code>SR_CURSOR_ENV = "LAYERX_EXPLORER_INGEST_CURSOR"</code> |
| <code>interface.14.env-test-layerx-explorer-ingest-interval-ms</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:613</code> | <code>"LAYERX_EXPLORER_INGEST_INTERVAL_MS"</code> |
| <code>interface.14.env-test-layerx-explorer-lni-socket</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:610</code> | <code>"LAYERX_EXPLORER_LNI_SOCKET"</code> |
| <code>interface.14.env-test-layerx-explorer-naming-program</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:608</code> | <code>"LAYERX_EXPLORER_NAMING_PROGRAM"</code> |
| <code>interface.14.env-test-layerx-explorer-network-id</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:611</code> | <code>"LAYERX_EXPLORER_NETWORK_ID"</code> |
| <code>interface.14.env-test-layerx-explorer-node-bearer-token</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:589</code> | <code>"LAYERX_EXPLORER_NODE_BEARER_TOKEN"</code> |
| <code>interface.14.env-test-layerx-explorer-node-endpoint</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:592</code> | <code>"LAYERX_EXPLORER_NODE_ENDPOINT"</code> |
| <code>interface.14.env-test-layerx-explorer-program-bearer-token</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:588</code> | <code>"LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN"</code> |
| <code>interface.14.env-test-layerx-explorer-program-listen</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:587</code> | <code>"LAYERX_EXPLORER_PROGRAM_LISTEN"</code> |
| <code>interface.14.env-test-layerx-explorer-program-max-staleness-ms</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:591</code> | <code>"LAYERX_EXPLORER_PROGRAM_MAX_STALENESS_MS"</code> |
| <code>interface.14.env-test-layerx-explorer-program-probe-id</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:599</code> | <code>"LAYERX_EXPLORER_PROGRAM_PROBE_ID"</code> |
| <code>interface.14.env-test-layerx-explorer-protocol-version</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:612</code> | <code>"LAYERX_EXPLORER_PROTOCOL_VERSION"</code> |
| <code>interface.14.env-test-layerx-explorer-read-ca-der</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:604</code> | <code>"LAYERX_EXPLORER_READ_CA_DER"</code> |
| <code>interface.14.env-test-layerx-explorer-read-endpoint</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:603</code> | <code>"LAYERX_EXPLORER_READ_ENDPOINT"</code> |
| <code>interface.14.env-test-layerx-explorer-read-fee-limit</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:607</code> | <code>"LAYERX_EXPLORER_READ_FEE_LIMIT"</code> |
| <code>interface.14.env-test-layerx-explorer-read-key-file</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:602</code> | <code>"LAYERX_EXPLORER_READ_KEY_FILE"</code> |
| <code>interface.14.env-test-layerx-explorer-read-network-id</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:606</code> | <code>"LAYERX_EXPLORER_READ_NETWORK_ID"</code> |
| <code>interface.14.env-test-layerx-explorer-read-sequencer-public-key-file</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:605</code> | <code>"LAYERX_EXPLORER_READ_SEQUENCER_PUBLIC_KEY_FILE"</code> |
| <code>interface.14.env-test-layerx-explorer-sequencer-trust-history</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:596</code> | <code>"LAYERX_EXPLORER_SEQUENCER_TRUST_HISTORY"</code> |
| <code>interface.14.env-test-layerx-explorer-verified-source-store</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:598</code> | <code>"LAYERX_EXPLORER_VERIFIED_SOURCE_STORE"</code> |
| <code>interface.14.env-test-layerx-gateway-bin</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1044</code> | <code>"LAYERX_GATEWAY_BIN"</code> |
| <code>interface.14.env-test-layerx-history-relay-fixture-config</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1057</code> | <code>os.environ.get("LAYERX_HISTORY_RELAY_FIXTURE_CONFIG")</code> |
| <code>interface.14.env-test-layerx-indexer-bin</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1043</code> | <code>"LAYERX_INDEXER_BIN"</code> |
| <code>interface.14.env-test-layerx-indexer-relay-url</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1247</code> | <code>"LAYERX_INDEXER_RELAY_URL"</code> |
| <code>interface.14.env-test-layerx-kernel-receipt-relay-fixture</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:43</code> | <code>os.environ["LAYERX_KERNEL_RECEIPT_RELAY_FIXTURE"]</code> |
| <code>interface.14.env-test-layerx-kernel-receipt-submission-authorization</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:71</code> | <code>os.environ.get("LAYERX_KERNEL_RECEIPT_SUBMISSION_AUTHORIZATION")</code> |
| <code>interface.14.env-test-layerx-kernel-receipt-submissions-dir</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:76</code> | <code>os.environ["LAYERX_KERNEL_RECEIPT_SUBMISSIONS_DIR"]</code> |
| <code>interface.14.env-test-layerx-network-gateway-endpoint</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:609</code> | <code>"LAYERX_NETWORK_GATEWAY_ENDPOINT"</code> |
| <code>interface.14.env-test-layerx-receipt-refresh-fixture</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1464</code> | <code>os.environ.get("LAYERX_RECEIPT_REFRESH_FIXTURE")</code> |
| <code>interface.14.env-test-layerx-receipt-refresh-node</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1500</code> | <code>os.environ.get("LAYERX_RECEIPT_REFRESH_NODE")</code> |
| <code>interface.14.env-test-paxeer-x-evidence-dir</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:895</code> | <code>os.environ.get("PAXEER_X_EVIDENCE_DIR")</code> |
| <code>interface.14.env-test-paxeer-x-gate-secret-key-base</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:1385</code> | <code>"PAXEER_X_GATE_SECRET_KEY_BASE"</code> |
| <code>interface.14.env-test-paxeer-x-layerx-assets</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:621</code> | <code>SR_ASSETS_ENV = "PAXEER_X_LAYERX_ASSETS"</code> |
| <code>interface.14.env-test-paxeer-x-layerx-send-cmd</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:620</code> | <code>SR_SEND_ENV = "PAXEER_X_LAYERX_SEND_CMD"</code> |
| <code>interface.14.env-test-paxeer-x-node-private-dir</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:626</code> | <code>SR_NODE_PRIVATE_ENV = "PAXEER_X_NODE_PRIVATE_DIR"</code> |
| <code>interface.14.env-test-paxeer-x-standalone-receipts-account</code> | <code>env_var</code> | <code>retire</code> | <code>tests/explorer/paxeer-x-data-boundary.py:625</code> | <code>SR_ACCOUNT_ENV = "PAXEER_X_STANDALONE_RECEIPTS_ACCOUNT"</code> |
| <code>interface.14.env-test-task-log-dir</code> | <code>env_var</code> | <code>stable</code> | <code>tests/explorer/paxeer-x-data-boundary.py:243</code> | <code>os.environ.get("TASK_LOG_DIR")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-test-cargo-target-dir</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-build-dir</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-explorer-authority-bearer-token</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-authority-ca-der</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-authority-endpoint</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-authority-replica-id</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-deployment-journal</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-finality-policy</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-genesis-trust</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-index-bin</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes; input of the standalone receipts case that drives layerx-explorer-index, deleted by stale spec task 11.7.41 |
| <code>interface.14.env-test-layerx-explorer-ingest-cursor</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-ingest-interval-ms</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-lni-socket</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-naming-program</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-network-id</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-node-bearer-token</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-node-endpoint</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-program-bearer-token</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-program-listen</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-program-max-staleness-ms</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-program-probe-id</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-protocol-version</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-read-ca-der</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-read-endpoint</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-read-fee-limit</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-read-key-file</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-read-network-id</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-read-sequencer-public-key-file</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-sequencer-trust-history</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-explorer-verified-source-store</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-gateway-bin</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-history-relay-fixture-config</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-indexer-bin</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-indexer-relay-url</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-kernel-receipt-relay-fixture</code> | notes | read by the data-boundary harness; feeds the kernel receipt relay fixture whose fetcher and routes tasks 8.7.19 and 8.7.20 remove |
| <code>interface.14.env-test-layerx-kernel-receipt-submission-authorization</code> | notes | read by the data-boundary harness; feeds the kernel receipt relay fixture whose fetcher and routes tasks 8.7.19 and 8.7.20 remove |
| <code>interface.14.env-test-layerx-kernel-receipt-submissions-dir</code> | notes | read by the data-boundary harness; feeds the kernel receipt relay fixture whose fetcher and routes tasks 8.7.19 and 8.7.20 remove |
| <code>interface.14.env-test-layerx-network-gateway-endpoint</code> | notes | read by the data-boundary harness; forwarded to layerx-explorer-index (reader services/human/crates/layerx-explorer-index/src/main.rs), a crate the stale spec task 11.7.41 deletes |
| <code>interface.14.env-test-layerx-receipt-refresh-fixture</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-layerx-receipt-refresh-node</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-paxeer-x-evidence-dir</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-paxeer-x-gate-secret-key-base</code> | notes | read by the data-boundary harness |
| <code>interface.14.env-test-paxeer-x-layerx-assets</code> | notes | read by the data-boundary harness; input of the standalone receipts case that drives layerx-explorer-index, deleted by stale spec task 11.7.41 |
| <code>interface.14.env-test-paxeer-x-layerx-send-cmd</code> | notes | read by the data-boundary harness; input of the standalone receipts case that drives layerx-explorer-index, deleted by stale spec task 11.7.41 |
| <code>interface.14.env-test-paxeer-x-node-private-dir</code> | notes | read by the data-boundary harness; input of the standalone receipts case that drives layerx-explorer-index, deleted by stale spec task 11.7.41 |
| <code>interface.14.env-test-paxeer-x-standalone-receipts-account</code> | notes | read by the data-boundary harness; input of the standalone receipts case that drives layerx-explorer-index, deleted by stale spec task 11.7.41 |
| <code>interface.14.env-test-task-log-dir</code> | notes | read by the data-boundary harness |

## env-token

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-token-balance-on-demand-fetcher-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1344</code> | <code>threshold: ConfigHelper.parse_time_env_var("TOKEN_BALANCE_ON_DEMAND_FETCHER_THRESHOLD", "1h"),</code> |
| <code>interface.14.env-token-counters-update-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1341</code> | <code>milliseconds_interval: ConfigHelper.parse_time_env_var("TOKEN_COUNTERS_UPDATE_INTERVAL", "3h")</code> |
| <code>interface.14.env-token-exchange-rate-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:626</code> | <code>ConfigHelper.safe_get_env("TOKEN_EXCHANGE_RATE_INTERVAL", "10s")</code> |
| <code>interface.14.env-token-exchange-rate-max-batch-size</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:636</code> | <code>ConfigHelper.parse_integer_env_var("TOKEN_EXCHANGE_RATE_MAX_BATCH_SIZE", 500)</code> |
| <code>interface.14.env-token-exchange-rate-refetch-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:631</code> | <code>ConfigHelper.safe_get_env("TOKEN_EXCHANGE_RATE_REFETCH_INTERVAL", "1h")</code> |
| <code>interface.14.env-token-exchange-rates-source</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:492</code> | <code>ConfigHelper.market_source("TOKEN_EXCHANGE_RATES_SOURCE"),</code> |
| <code>interface.14.env-token-instance-metadata-refetch-on-demand-fetcher-threshold</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1357</code> | <code>"TOKEN_INSTANCE_METADATA_REFETCH_ON_DEMAND_FETCHER_THRESHOLD",</code> |
| <code>interface.14.env-token-metadata-update-interval</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1234</code> | <code>metadata_updater_milliseconds_interval: ConfigHelper.parse_time_env_var("TOKEN_METADATA_UPDATE_INTERVAL", "48h"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-token-balance-on-demand-fetcher-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-token-counters-update-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-token-exchange-rate-interval</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-token-exchange-rate-max-batch-size</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-token-exchange-rate-refetch-interval</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-token-exchange-rates-source</code> | notes | read 1x in backend; set in 0 non-backend sites; no in-repo setter (default applies) |
| <code>interface.14.env-token-instance-metadata-refetch-on-demand-fetcher-threshold</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-token-metadata-update-interval</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-trace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-trace-block-ranges</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1220</code> | <code>case ConfigHelper.safe_get_env("TRACE_BLOCK_RANGES", nil) do</code> |
| <code>interface.14.env-trace-first-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1216</code> | <code>trace_first_block = ConfigHelper.parse_integer_env_var("TRACE_FIRST_BLOCK", 0)</code> |
| <code>interface.14.env-trace-last-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1217</code> | <code>trace_last_block = ConfigHelper.parse_integer_or_nil_env_var("TRACE_LAST_BLOCK")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-trace-block-ranges</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-trace-first-block</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-trace-last-block</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-txs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-txs-historian-init-lag</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:663</code> | <code>init_lag_milliseconds: ConfigHelper.parse_time_env_var("TXS_HISTORIAN_INIT_LAG", "0"),</code> |
| <code>interface.14.env-txs-stats-days-to-compile-at-init</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:664</code> | <code>days_to_compile_at_init: ConfigHelper.parse_integer_env_var("TXS_STATS_DAYS_TO_COMPILE_AT_INIT", 40)</code> |
| <code>interface.14.env-txs-stats-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:34</code> | <code>transactions_stats_enabled = ConfigHelper.parse_bool_env_var("TXS_STATS_ENABLED", "true")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-txs-historian-init-lag</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-txs-stats-days-to-compile-at-init</code> | notes | read 1x in backend; set in 2 non-backend sites |
| <code>interface.14.env-txs-stats-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-uncles

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-uncles-in-average-block-time</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:340</code> | <code>include_uncles_in_average_block_time: ConfigHelper.parse_bool_env_var("UNCLES_IN_AVERAGE_BLOCK_TIME"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-uncles-in-average-block-time</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-universal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-universal-proxy-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1185</code> | <code>universal_proxy_config = System.get_env("UNIVERSAL_PROXY_CONFIG")</code> |
| <code>interface.14.env-universal-proxy-config-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1181</code> | <code>"UNIVERSAL_PROXY_CONFIG_URL",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-universal-proxy-config</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-universal-proxy-config-url</code> | notes | read 2x in backend; set in 1 non-backend sites |

## env-validators

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-validators-contract</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:666</code> | <code>if System.get_env("METADATA_CONTRACT") &amp;&amp; System.get_env("VALIDATORS_CONTRACT") do</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-validators-contract</code> | notes | read 3x in backend; set in 2 non-backend sites |

## env-webapp

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-webapp-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:48</code> | <code>webapp_url: ConfigHelper.parse_url_env_var("WEBAPP_URL"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-webapp-url</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-weth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-weth-token-transfers-filtering-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1135</code> | <code>weth_token_transfers_filtering_enabled: ConfigHelper.parse_bool_env_var("WETH_TOKEN_TRANSFERS_FILTERING_ENABLED")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-weth-token-transfers-filtering-enabled</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-whitelisted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-whitelisted-weth-contracts</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1134</code> | <code>whitelisted_weth_contracts: ConfigHelper.parse_list_env_var("WHITELISTED_WETH_CONTRACTS", ""),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-whitelisted-weth-contracts</code> | notes | read 1x in backend; set in 1 non-backend sites |

## env-withdrawals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-withdrawals-first-block</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:1801</code> | <code>config :indexer, Indexer.Fetcher.Withdrawal, first_block: System.get_env("WITHDRAWALS_FIRST_BLOCK")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-withdrawals-first-block</code> | notes | read 1x in backend; set in 2 non-backend sites |

## env-xname

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-xname-api-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:740</code> | <code>api_key: System.get_env("XNAME_API_TOKEN")</code> |
| <code>interface.14.env-xname-base-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/explorer/backend/config/runtime.exs:739</code> | <code>service_url: ConfigHelper.parse_url_env_var("XNAME_BASE_API_URL", "&lt;url&gt;"),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-xname-api-token</code> | notes | read 1x in backend; set in 1 non-backend sites |
| <code>interface.14.env-xname-base-api-url</code> | notes | read 1x in backend; set in 1 non-backend sites |
