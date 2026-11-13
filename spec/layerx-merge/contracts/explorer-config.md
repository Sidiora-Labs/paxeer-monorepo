# Explorer config contract

Explorer backend configuration is loaded from the [configuration tree](../../../services/explorer/backend/config) and application configuration files. Environment inputs and application keys are listed below. Configuration for other chain types applies only when that chain type is selected.

## cfg-bcrypt-elixir

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-bcrypt-elixir-log-rounds</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/test.exs](../../../services/explorer/backend/apps/explorer/config/test.exs) | <code>config :bcrypt_elixir, :log_rounds</code> |

## cfg-ethereum-jsonrpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-geth</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.Geth [block_traceable?, allow_empty_traces?, debug_trace_timeout, tracer]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-http</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.HTTP</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-http-throttle</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.HTTP.Throttle [max_inflight, rps, trace_max_inflight, archive_max_inflight, coalesce?]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-paxeerx</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.PaxeerX [cosmos_transaction_type, native_coin_decimals]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-pendingtransaction</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.PendingTransaction [type]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-requestcoordinator</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.RequestCoordinator [wait_per_timeout, rolling_window_opts, max_jitter]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-tracer</code> | <code>config_key</code> | [services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs](../../../services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-utility-endpointavailabilitychecker</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.Utility.EndpointAvailabilityChecker [enabled]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ethereumjsonrpc-websocket-retryworker</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, EthereumJSONRPC.WebSocket.RetryWorker [retry_interval]</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-ipc-path</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, :ipc_path</code> |
| <code>interface.14.cfg-ethereum-jsonrpc-rpc-transport</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ethereum_jsonrpc, :rpc_transport</code> |

## cfg-ex-aws

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-ex-aws-access-key-id</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ex_aws, :access_key_id</code> |
| <code>interface.14.cfg-ex-aws-json-codec</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ex_aws, :json_codec</code> |
| <code>interface.14.cfg-ex-aws-s3</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ex_aws, :s3 [scheme, host, port, public_r2_url, bucket_name]</code> |
| <code>interface.14.cfg-ex-aws-secret-access-key</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ex_aws, :secret_access_key</code> |

## cfg-ex-cldr

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-ex-cldr-default-backend</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :ex_cldr, :default_backend</code> |
| <code>interface.14.cfg-ex-cldr-default-locale</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :ex_cldr, :default_locale</code> |

## cfg-explorer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-explorer-air-table-audit-reports</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :air_table_audit_reports [table_url, api_key, enabled]</code> |
| <code>interface.14.cfg-explorer-celo</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :celo [l2_migration_block, epoch_manager_contract_address, unreleased_treasury_contract_address, validators_contract_address, locked_gold_contract_address, accounts_contract_address]</code> |
| <code>interface.14.cfg-explorer-chain-identity</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :chain_identity</code> |
| <code>interface.14.cfg-explorer-chain-type</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :chain_type</code> |
| <code>interface.14.cfg-explorer-coin</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :coin</code> |
| <code>interface.14.cfg-explorer-coin-name</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :coin_name</code> |
| <code>interface.14.cfg-explorer-datadog</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :datadog [port]</code> |
| <code>interface.14.cfg-explorer-ecto-repos</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :ecto_repos</code> |
| <code>interface.14.cfg-explorer-explorer-account</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Account [enabled, sendgrid]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-blackfort-validator</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Blackfort.Validator [api_url]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-block-reward</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Block.Reward [validators_contract_address, keys_manager_contract_address]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-bridgedtoken</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.BridgedToken [eth_omni_bridge_mediator, bsc_omni_bridge_mediator, poa_omni_bridge_mediator, amb_bridge_mediators, foreign_json_rpc, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-accounts</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Accounts [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-blocknumber</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.BlockNumber [ttl_check_interval, global_ttl, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-blocks</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Blocks [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-celocorecontracts</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.CeloCoreContracts [contracts]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-celoepochs</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.CeloEpochs [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescoinbalancesum</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressesCoinBalanceSum [global_ttl, enabled, ttl_check_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescoinbalancesumminusburnt</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressesCoinBalanceSumMinusBurnt [global_ttl, enabled, ttl_check_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addressescount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressesCount [update_interval_in_milliseconds, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstabselementscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTabsElementsCount [ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstokensusdsum</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTokensUsdSum [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstokentransferscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTokenTransfersCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstransactionscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTransactionsCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-addresstransactionsgasusagesum</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AddressTransactionsGasUsageSum [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-averageblocktime</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.AverageBlockTime [enabled, period, cache_period, num_of_blocks]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blackfort-validatorscount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.Blackfort.ValidatorsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockburntfeecount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.BlockBurntFeeCount [enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockpriorityfeecount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.BlockPriorityFeeCount [enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-blockscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.BlocksCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-contractscount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.ContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-gasusagesum</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.GasUsageSum [global_ttl, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newcontractscount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.NewContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newpendingtransactionscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.NewPendingTransactionsCount [enabled, cache_period, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-newverifiedcontractscount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.NewVerifiedContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-optimism-lastoutputrootsizecount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.Optimism.LastOutputRootSizeCount [enabled, enable_consolidation, cache_period]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-pendingblockoperationcount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.PendingBlockOperationCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-pendingtransactionoperationcount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.PendingTransactionOperationCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-rootstock-lockedbtccount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.Rootstock.LockedBTCCount [enabled, global_ttl, locking_cap]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-stability-validatorscount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.Stability.ValidatorsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-tokenholderscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.TokenHoldersCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-tokentransferscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.TokenTransfersCount [cache_period, enabled, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-transactions24hcount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.Transactions24hCount [enabled, cache_period, enable_consolidation]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-transactionscount</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.TransactionsCount [global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-verifiedcontractscount</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.VerifiedContractsCount [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-counters-withdrawalssum</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.Counters.WithdrawalsSum [enabled, enable_consolidation, update_interval_in_milliseconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-gaspriceoracle</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.GasPriceOracle [global_ttl, simple_transaction_gas, num_of_blocks, safelow_percentile, average_percentile, fast_percentile, safelow_time_coefficient, average_time_coefficient, fast_time_coefficient]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-minmissingblocknumber</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.MinMissingBlockNumber [enabled, batch_size]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-optimismfinalizationperiod</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.OptimismFinalizationPeriod [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactionactiontokensdata</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.TransactionActionTokensData [max_cache_size, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactionactionuniswappools</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Cache.TransactionActionUniswapPools [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-transactions</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Transactions [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-cache-uncles</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Cache.Uncles [ttl_check_interval, global_ttl]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-events-listener</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Events.Listener [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-addressesblacklist</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Fetcher.AddressesBlacklist [url, enabled, update_interval, retry_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-checkbytecodematchingondemand</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Fetcher.CheckBytecodeMatchingOnDemand [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-fetchvalidatorinfoondemand</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Chain.Fetcher.FetchValidatorInfoOnDemand [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-fetcher-lookupsmartcontractsourcesondemand</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Fetcher.LookUpSmartContractSourcesOnDemand</code> |
| <code>interface.14.cfg-explorer-explorer-chain-filecoin-nativeaddress</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Filecoin.NativeAddress [network_prefix]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-health-monitor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Health.Monitor [check_interval, healthy_blocks_period, healthy_batches_period]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-metrics-publicmetrics</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Metrics.PublicMetrics [enabled, update_period_hours]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-mud</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Mud [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-capabilities</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.PaxeerX.Capabilities [enabled, refresh_interval_seconds]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-paxeerx-sequencerreceipt</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.PaxeerX.SequencerReceipt [router_url]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-scroll-l1feeparam</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Scroll.L1FeeParam [curie_upgrade_block, scalar_init, overhead_init, commit_scalar_init, blob_scalar_init, l1_base_fee_init, l1_blob_base_fee_init]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-tokentransfer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.TokenTransfer [whitelisted_weth_contracts, weth_token_transfers_filtering_enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-transaction</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Transaction [block_miner_gets_burnt_fees?, suave_bid_contracts, rootstock_remasc_address, rootstock_bridge_address]</code> |
| <code>interface.14.cfg-explorer-explorer-chain-transaction-history-historian</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Chain.Transaction.History.Historian [enabled, init_lag_milliseconds, days_to_compile_at_init]</code> |
| <code>interface.14.cfg-explorer-explorer-chainspec-genesisdata</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ChainSpec.GenesisData [chain_spec_path, genesis_processing_delay, emission_format, rewards_contract_address, precompiled_config_path, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-integrations-ectologger</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Integrations.EctoLogger [query_time_ms_threshold]</code> |
| <code>interface.14.cfg-explorer-explorer-mailer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Mailer [adapter, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-market</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-coin</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Fetcher.Coin [store, enabled, fetch_btc_value]</code> |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-history</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Fetcher.History [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-fetcher-token</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Fetcher.Token [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-history-cataloger</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/runtime/test.exs](../../../services/explorer/backend/apps/explorer/config/runtime/test.exs) | <code>config :explorer, Explorer.Market.History.Cataloger [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-market-markethistorycache</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.MarketHistoryCache [cache_period]</code> |
| <code>interface.14.cfg-explorer-explorer-market-source</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-coingecko</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.CoinGecko</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-coinmarketcap</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.CoinMarketCap</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-cryptocompare</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.CryptoCompare</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-cryptorank</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.CryptoRank</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-defillama</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.DefiLlama [coin_id, base_url]</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-dia</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.DIA [blockchain, base_url, coin_address_hash, secondary_coin_address_hash]</code> |
| <code>interface.14.cfg-explorer-explorer-market-source-mobula</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Market.Source.Mobula</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-accountabstraction</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.MicroserviceInterfaces.AccountAbstraction [service_url, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-bens</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.MicroserviceInterfaces.BENS [service_url, enabled, protocols]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-metadata</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.MicroserviceInterfaces.Metadata [service_url, enabled, requests_timeout, proxy_requests_timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-multichainsearch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.MicroserviceInterfaces.MultichainSearch [api_key, service_url]</code> |
| <code>interface.14.cfg-explorer-explorer-microserviceinterfaces-tacoperationlifecycle</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.MicroserviceInterfaces.TACOperationLifecycle [enabled, service_url]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-arbitrumdarecordsnormalization</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.ArbitrumDaRecordsNormalization [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-backfillmetadataurl</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.BackfillMetadataURL [enabled, batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-backfillmultichainsearchdb</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.BackfillMultichainSearchDB [concurrency, batch_size]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-celoaccounts</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.CeloAccounts [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-celoaggregatedelectionrewards</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.CeloAggregatedElectionRewards [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-celol2epochs</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.CeloL2Epochs</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-deletezerovalueinternaltransactions</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.DeleteZeroValueInternalTransactions [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-emptyinternaltransactionsdata</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.EmptyInternalTransactionsData</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-filecoinpendingaddressoperations</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.FilecoinPendingAddressOperations [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.HeavyDbIndexOperation [check_interval]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation-createarbitrumbatchl2blocksunconfirmedblocksindex</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.HeavyDbIndexOperation.CreateArbitrumBatchL2BlocksUnconfirmedBlocksIndex [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-heavydbindexoperation-createtransactionsoperatorfeeconstantindex</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.HeavyDbIndexOperation.CreateTransactionsOperatorFeeConstantIndex [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-mergeadjacentmissingblockranges</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.MergeAdjacentMissingBlockRanges</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-refetchcontractcodes</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.RefetchContractCodes [concurrency, batch_size]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexblockswithmissingtransactions</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.ReindexBlocksWithMissingTransactions</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexduplicatedinternaltransactions</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.ReindexDuplicatedInternalTransactions</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-reindexinternaltransactionswithincompatiblestatus</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.ReindexInternalTransactionsWithIncompatibleStatus</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-restoreomittedwethtransfers</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.RestoreOmittedWETHTransfers</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeduplicatedlogindexlogs</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.SanitizeDuplicatedLogIndexLogs</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeemptycontractcodeaddresses</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.SanitizeEmptyContractCodeAddresses</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeincorrectnfttokentransfers</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.SanitizeIncorrectNFTTokenTransfers [batch_size, concurrency, timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeincorrectwethtokentransfers</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.SanitizeIncorrectWETHTokenTransfers [batch_size, concurrency, timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-sanitizeverifiedaddresses</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.SanitizeVerifiedAddresses [enabled, batch_size, concurrency, timeout]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-shrinkinternaltransactions</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.ShrinkInternalTransactions [enabled, batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-smartcontractlanguage</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.SmartContractLanguage [enabled, batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-tokentransfertokentype</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.TokenTransferTokenType [batch_size, concurrency]</code> |
| <code>interface.14.cfg-explorer-explorer-migrator-transactionsdenormalization</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Migrator.TransactionsDenormalization</code> |
| <code>interface.14.cfg-explorer-explorer-repo</code> | <code>config_key</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>config :explorer, Explorer.Repo [database, hostname, url, pool_size, queue_target, ssl, migration_timestamps, disconnect_on_error_codes, timeout, migration_lock, prepare, ssl_opts, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-account</code> | <code>config_key</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>config :explorer, Explorer.Repo.Account [database, hostname, url, pool_size, queue_target, ssl, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-eventnotifications</code> | <code>config_key</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>config :explorer, Explorer.Repo.EventNotifications [database, hostname, url, pool_size, queue_target, ssl, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-mud</code> | <code>config_key</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>config :explorer, Explorer.Repo.Mud [database, hostname, url, pool_size, queue_target, ssl]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-polygonzkevm</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/test.exs](../../../services/explorer/backend/apps/explorer/config/test.exs) | <code>config :explorer, Explorer.Repo.PolygonZkevm [database, hostname, url, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-replica1</code> | <code>config_key</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>config :explorer, Explorer.Repo.Replica1 [database, hostname, url, pool_size, queue_target, ssl, pool]</code> |
| <code>interface.14.cfg-explorer-explorer-repo-suave</code> | <code>config_key</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>config :explorer, Explorer.Repo.Suave [database, hostname, url, pool_size, ssl]</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-certifiedsmartcontractcataloger</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.SmartContract.CertifiedSmartContractCataloger [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-rustverifierinterfacebehaviour</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-sigproviderinterface</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.SmartContract.SigProviderInterface [service_url, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-smartcontract-stylusverifierinterface</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.SmartContract.StylusVerifierInterface [service_url]</code> |
| <code>interface.14.cfg-explorer-explorer-tags-addresstag-cataloger</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Tags.AddressTag.Cataloger [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-dynamic</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.Dynamic [enabled, env_id, url]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-dynamic-strategy</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.Dynamic.Strategy [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-keycloak</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.Keycloak [domain, realm, client_id, client_secret, email_webhook_url]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-novesfi</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.NovesFi [service_url, chain_name, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-solidityscan</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.SolidityScan [platform_id, chain_id, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-sourcify</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.Sourcify [server_url, enabled, chain_id, repo_url]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-universalproxy</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.UniversalProxy [config_url, config_json]</code> |
| <code>interface.14.cfg-explorer-explorer-thirdpartyintegrations-xname</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.ThirdPartyIntegrations.Xname [service_url, api_key]</code> |
| <code>interface.14.cfg-explorer-explorer-tokeninstanceowneraddressmigration</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.TokenInstanceOwnerAddressMigration [concurrency, batch_size, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-tokeninstanceowneraddressmigration-supervisor</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.TokenInstanceOwnerAddressMigration.Supervisor [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-tracer</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-hammer-ets</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Utility.Hammer.ETS [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-hammer-redis</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Utility.Hammer.Redis [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-missingbalanceoftoken</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Utility.MissingBalanceOfToken [window_size]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-ratelimiter</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Utility.RateLimiter [storage, redis_url, on_demand, enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-utility-versionconstantsupdater</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, Explorer.Utility.VersionConstantsUpdater [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-validator-metadataprocessor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Validator.MetadataProcessor [enabled]</code> |
| <code>interface.14.cfg-explorer-explorer-validator-metadataretriever</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Validator.MetadataRetriever [metadata_contract_address, validators_contract_address]</code> |
| <code>interface.14.cfg-explorer-explorer-visualize-sol2uml</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, Explorer.Visualize.Sol2uml [service_url, enabled]</code> |
| <code>interface.14.cfg-explorer-http-client</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, :http_client</code> |
| <code>interface.14.cfg-explorer-json-rpc-named-arguments</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/dev/anvil.exs](../../../services/explorer/backend/apps/explorer/config/dev/anvil.exs) | <code>config :explorer, :json_rpc_named_arguments</code> |
| <code>interface.14.cfg-explorer-mode</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :mode</code> |
| <code>interface.14.cfg-explorer-proxy</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :proxy [caching_implementation_data_enabled, implementation_data_ttl_via_avg_block_time, fallback_cached_implementation_data_ttl, implementation_data_fetching_timeout]</code> |
| <code>interface.14.cfg-explorer-realtime-events-sender</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/runtime/test.exs](../../../services/explorer/backend/apps/explorer/config/runtime/test.exs) | <code>config :explorer, :realtime_events_sender</code> |
| <code>interface.14.cfg-explorer-solc-bin-api-url</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, :solc_bin_api_url</code> |
| <code>interface.14.cfg-explorer-spandex</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :spandex [batch_size, sync_threshold]</code> |
| <code>interface.14.cfg-explorer-subscribe-named-arguments</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/dev/anvil.exs](../../../services/explorer/backend/apps/explorer/config/dev/anvil.exs) | <code>config :explorer, :subscribe_named_arguments</code> |
| <code>interface.14.cfg-explorer-supply</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :explorer, :supply</code> |
| <code>interface.14.cfg-explorer-token-functions-reader-max-retries</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :explorer, :token_functions_reader_max_retries</code> |
| <code>interface.14.cfg-explorer-transport</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/test/besu.exs](../../../services/explorer/backend/apps/explorer/config/test/besu.exs) | <code>config :explorer, :transport</code> |

## cfg-indexer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-indexer-arc</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :arc [arc_native_token_decimals]</code> |
| <code>interface.14.cfg-indexer-block-interval</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev/anvil.exs](../../../services/explorer/backend/apps/indexer/config/dev/anvil.exs) | <code>config :indexer, :block_interval</code> |
| <code>interface.14.cfg-indexer-block-ranges</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :block_ranges</code> |
| <code>interface.14.cfg-indexer-block-transformer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :block_transformer</code> |
| <code>interface.14.cfg-indexer-blocks-concurrency</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev/rsk.exs](../../../services/explorer/backend/apps/indexer/config/dev/rsk.exs) | <code>config :indexer, :blocks_concurrency</code> |
| <code>interface.14.cfg-indexer-chain-id</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :chain_id</code> |
| <code>interface.14.cfg-indexer-ecto-repos</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/config.exs](../../../services/explorer/backend/apps/indexer/config/config.exs) | <code>config :indexer, :ecto_repos</code> |
| <code>interface.14.cfg-indexer-fetch-rewards-way</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :fetch_rewards_way</code> |
| <code>interface.14.cfg-indexer-fetcher-init-delay</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :fetcher_init_delay</code> |
| <code>interface.14.cfg-indexer-fetcher-init-limit</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :fetcher_init_limit</code> |
| <code>interface.14.cfg-indexer-first-block</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :first_block</code> |
| <code>interface.14.cfg-indexer-hide-indexing-progress-alert</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :hide_indexing_progress_alert</code> |
| <code>interface.14.cfg-indexer-indexer-block-catchup-fetcher</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Block.Catchup.Fetcher [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-block-catchup-missingrangescollector</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Block.Catchup.MissingRangesCollector [batch_size, future_check_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-block-catchup-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Block.Catchup.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-block-realtime-fetcher</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Block.Realtime.Fetcher [max_gap, polling_period, batch_size, max_lag]</code> |
| <code>interface.14.cfg-indexer-indexer-block-realtime-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Block.Realtime.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum [l1_rpc, l1_rpc_chunk_size, l1_rpc_block_range, l1_rollup_address, l1_rollup_init_block, l1_start_block, l1_finalization_threshold, rollup_chunk_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-databackfill</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.DataBackfill</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-databackfill-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.DataBackfill.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-messagestol2matcher-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.MessagesToL2Matcher.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-messaging</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.Messaging</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-rollupmessagescatchup</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.RollupMessagesCatchup [recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-rollupmessagescatchup-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.RollupMessagesCatchup.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingbatchesstatuses</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingBatchesStatuses [recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingbatchesstatuses-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingBatchesStatuses.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingmessagesonl1</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingMessagesOnL1</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-arbitrum-trackingmessagesonl1-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Arbitrum.TrackingMessagesOnL1.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon [beacon_rpc]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-blob</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon.Blob [slot_duration, reference_slot, start_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-blob-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon.Blob.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit [interval, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-status</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit.Status</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-status-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit.Status.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-beacon-deposit-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon.Deposit.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-blockreward</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.BlockReward [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-blockreward-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.BlockReward.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-epochblockoperations-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Celo.EpochBlockOperations.Supervisor [enabled, disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-legacy-account</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Celo.Legacy.Account [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-legacy-account-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Celo.Legacy.Account.Supervisor [enabled, disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-validatorgroupvotes</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Celo.ValidatorGroupVotes [batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-celo-validatorgroupvotes-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Celo.ValidatorGroupVotes.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-catchup</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.CoinBalance.Catchup [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-catchup-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.CoinBalance.Catchup.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-realtime</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.CoinBalance.Realtime [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-coinbalance-realtime-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.CoinBalance.Realtime.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-emptyblockssanitizer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.EmptyBlocksSanitizer [batch_size, interval, head_offset]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-emptyblockssanitizer-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.EmptyBlocksSanitizer.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-addressinfo</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Filecoin.AddressInfo [concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-addressinfo-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Filecoin.AddressInfo.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-beryxapi</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Filecoin.BeryxAPI</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-filecoin-filfoxapi</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Filecoin.FilfoxAPI [base_url]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.InternalTransaction [batch_size, concurrency, lookback_blocks]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction-deletequeue</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.InternalTransaction.DeleteQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-internaltransaction-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.InternalTransaction.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-balancesexportqueue</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-balancesexportqueue-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersexportqueue</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.CountersExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersexportqueue-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.CountersExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-countersfetcher-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.CountersFetcher.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-mainexportqueue</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.MainExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-mainexportqueue-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.MainExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-tokeninfoexportqueue</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-multichainsearchdb-tokeninfoexportqueue-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-coinbalance</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.OnDemand.CoinBalance [threshold, fallback_threshold_in_blocks]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-coinbalance-supervisor</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/runtime/test.exs](../../../services/explorer/backend/apps/indexer/config/runtime/test.exs) | <code>config :indexer, Indexer.Fetcher.OnDemand.CoinBalance.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-contractcode</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.OnDemand.ContractCode [threshold]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokenbalance</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.OnDemand.TokenBalance [threshold, fallback_threshold_in_blocks]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokenbalance-supervisor</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/runtime/test.exs](../../../services/explorer/backend/apps/indexer/config/runtime/test.exs) | <code>config :indexer, Indexer.Fetcher.OnDemand.TokenBalance.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-ondemand-tokeninstancemetadatarefetch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.OnDemand.TokenInstanceMetadataRefetch</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism [optimism_l1_rpc, optimism_l1_system_config, l1_eth_get_logs_range_size, l2_eth_get_logs_range_size, block_duration, start_block_l1, portal, isthmus_timestamp_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-deposit</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Deposit [transaction_type]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-deposit-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Deposit.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-disputegame-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.DisputeGame.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-eip1559configupdate</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.EIP1559ConfigUpdate [chunk_size, holocene_timestamp_l2, jovian_timestamp_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-eip1559configupdate-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.EIP1559ConfigUpdate.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-message</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.Message [start_block, blocks_chunk_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-message-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.Message.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagefailed-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MessageFailed.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagequeue</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MessageQueue [chainscout_api_url, chainscout_fallback_map, private_key, connect_timeout, recv_timeout, export_expiration]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-messagequeue-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MessageQueue.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-multichainexport</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MultichainExport [batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-interop-multichainexport-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Interop.MultichainExport.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-operatorfee</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.OperatorFee [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-operatorfee-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.OperatorFee.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-outputroot</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.OutputRoot [output_oracle]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-outputroot-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.OutputRoot.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-transactionbatch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.TransactionBatch [blocks_chunk_size, eip4844_blobs_api_url, celestia_blobs_api_url, eigenda_blobs_api_url, eigenda_proxy_base_url, alt_da_server_url, genesis_block_l2, inbox, submitter]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-transactionbatch-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.TransactionBatch.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawal</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Withdrawal [start_block_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawal-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.Withdrawal.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-optimism-withdrawalevent-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Optimism.WithdrawalEvent.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-pricefeedmaintenance</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PaxeerX.PriceFeedMaintenance [enabled, retention_days]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-priceupdates</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PaxeerX.PriceUpdates [cosmos_rest_urls, cometbft_rpc_urls, start_block, concurrency, markets_refresh_seconds]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-paxeerx-priceupdates-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-pendingtransaction-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PendingTransaction.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL1 [rpc, start_block, bridge_contract, native_symbol, native_decimals, rollup_network_id_l1, rollup_index_l1]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL1.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel1tokens-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL1Tokens.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel2</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL2 [start_block, bridge_contract, rollup_network_id_l2, rollup_index_l2]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-bridgel2-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.BridgeL2.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-transactionbatch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.TransactionBatch [chunk_size, ignore_numbers, recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-polygonzkevm-transactionbatch-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.PolygonZkevm.TransactionBatch.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-replacedtransaction-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.ReplacedTransaction.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-rootstockdata</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.RootstockData [interval, batch_size, max_concurrency, db_batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-rootstockdata-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.RootstockData.Supervisor</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll [l1_eth_get_logs_range_size, l2_eth_get_logs_range_size, rpc]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-batch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.Batch [scroll_chain_contract, start_block, eip4844_blobs_api_url]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-batch-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.Batch.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel1</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL1 [messenger_contract, start_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel1-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL1.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel2</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL2 [messenger_contract, start_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-bridgel2-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.BridgeL2.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-l1feeparam</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.L1FeeParam [gas_oracle]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-scroll-l1feeparam-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Scroll.L1FeeParam.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l1</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Shibarium.L1 [rpc, start_block, deposit_manager_proxy, ether_predicate_proxy, erc20_predicate_proxy, erc721_predicate_proxy, erc1155_predicate_proxy, withdraw_manager_proxy]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l1-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Shibarium.L1.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l2</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Shibarium.L2 [start_block, child_chain, weth, bone_withdraw]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-shibarium-l2-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Shibarium.L2.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-signedauthorizationstatus</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.SignedAuthorizationStatus [batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-stats-hotsmartcontracts-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Stats.HotSmartContracts.Supervisor [disabled?, enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-token</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Token [concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-current</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenBalance.Current [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-historical</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenBalance.Historical [batch_size, concurrency]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenbalance-historical-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenBalance.Historical.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokencountersupdater</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenCountersUpdater [milliseconds_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokencountersupdater-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenCountersUpdater.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-helper</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Helper [base_uri_retry?, cidr_blacklist, host_filtering_enabled?, allowed_uri_protocols]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-realtime</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Realtime [concurrency, batch_size, retry_with_cooldown?, retry_timeout]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-realtime-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Realtime.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-refetch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Refetch [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-refetch-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Refetch.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-retry</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Retry [concurrency, batch_size, max_refetch_interval, exp_timeout_base]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-retry-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Retry.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitize</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Sanitize [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitize-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.Sanitize.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitizeerc1155</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.SanitizeERC1155</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokeninstance-sanitizeerc721</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenInstance.SanitizeERC721 [concurrency, batch_size]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokentotalsupplyupdater-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenTotalSupplyUpdater.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-tokenupdater-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TokenUpdater.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-transactionaction</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TransactionAction [reindex_first_block, reindex_last_block, reindex_protocols, aave_v3_pool]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-transactionaction-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.TransactionAction.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-withdrawal</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Withdrawal [first_block]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-withdrawal-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Withdrawal.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zilliqa-scillasmartcontracts-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Zilliqa.ScillaSmartContracts.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zilliqa-zrc2tokens-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Zilliqa.Zrc2Tokens.Supervisor [disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-batchesstatustracker</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.ZkSync.BatchesStatusTracker [zksync_l1_rpc, recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-batchesstatustracker-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.ZkSync.BatchesStatusTracker.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-transactionbatch</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.ZkSync.TransactionBatch [chunk_size, batches_max_range, recheck_interval]</code> |
| <code>interface.14.cfg-indexer-indexer-fetcher-zksync-transactionbatch-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.ZkSync.TransactionBatch.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-migrator-recoverywethtokentransfers</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Migrator.RecoveryWETHTokenTransfers [concurrency, batch_size, timeout, enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-pendingtransactionssanitizer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.PendingTransactionsSanitizer [interval]</code> |
| <code>interface.14.cfg-indexer-indexer-prometheus-metrics</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Prometheus.Metrics</code> |
| <code>interface.14.cfg-indexer-indexer-supervisor</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Supervisor [enabled]</code> |
| <code>interface.14.cfg-indexer-indexer-tokentransferblockconsensussanitizer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.TokenTransferBlockConsensusSanitizer</code> |
| <code>interface.14.cfg-indexer-indexer-tracer</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/config.exs](../../../services/explorer/backend/apps/indexer/config/config.exs) | <code>config :indexer, Indexer.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-indexer-indexer-utils-eventnotificationscleaner</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Utils.EventNotificationsCleaner [interval]</code> |
| <code>interface.14.cfg-indexer-ipfs</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :ipfs [gateway_url, gateway_url_param_key, gateway_url_param_value]</code> |
| <code>interface.14.cfg-indexer-json-rpc-named-arguments</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev/anvil.exs](../../../services/explorer/backend/apps/indexer/config/dev/anvil.exs) | <code>config :indexer, :json_rpc_named_arguments</code> |
| <code>interface.14.cfg-indexer-last-block</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :last_block</code> |
| <code>interface.14.cfg-indexer-memory-limit</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :memory_limit</code> |
| <code>interface.14.cfg-indexer-metadata-updater-milliseconds-interval</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :metadata_updater_milliseconds_interval</code> |
| <code>interface.14.cfg-indexer-receipts-batch-size</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :receipts_batch_size</code> |
| <code>interface.14.cfg-indexer-receipts-concurrency</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :receipts_concurrency</code> |
| <code>interface.14.cfg-indexer-subscribe-named-arguments</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev/anvil.exs](../../../services/explorer/backend/apps/indexer/config/dev/anvil.exs) | <code>config :indexer, :subscribe_named_arguments</code> |
| <code>interface.14.cfg-indexer-system-memory-percentage</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :system_memory_percentage</code> |
| <code>interface.14.cfg-indexer-trace-block-ranges</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :trace_block_ranges</code> |
| <code>interface.14.cfg-indexer-trace-first-block</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :trace_first_block</code> |
| <code>interface.14.cfg-indexer-trace-last-block</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, :trace_last_block</code> |

## cfg-libcluster

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-libcluster-topologies</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :libcluster, :topologies</code> |

## cfg-logger

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-logger-account</code> | <code>config_key</code> | [services/explorer/backend/config/dev.exs](../../../services/explorer/backend/config/dev.exs) | <code>config :logger, :account [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-addresses-without-code</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :addresses_without_code [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-api</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :logger, :api [format, metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-api-v2</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :logger, :api_v2 [format, metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-backends</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :logger, :backends</code> |
| <code>interface.14.cfg-logger-block-import-timings</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :block_import_timings [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-console</code> | <code>config_key</code> | [services/explorer/backend/config/config.exs](../../../services/explorer/backend/config/config.exs) | <code>config :logger, :console [metadata]</code> |
| <code>interface.14.cfg-logger-default-formatter</code> | <code>config_key</code> | [services/explorer/backend/config/config.exs](../../../services/explorer/backend/config/config.exs) | <code>config :logger, :default_formatter [format]</code> |
| <code>interface.14.cfg-logger-default-handler</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :logger, :default_handler</code> |
| <code>interface.14.cfg-logger-empty-blocks-to-refetch</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :empty_blocks_to_refetch [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-error</code> | <code>config_key</code> | [services/explorer/backend/config/config.exs](../../../services/explorer/backend/config/config.exs) | <code>config :logger, :error [metadata, path, level, rotate]</code> |
| <code>interface.14.cfg-logger-ethereum-jsonrpc</code> | <code>config_key</code> | [services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs](../../../services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs) | <code>config :logger, :ethereum_jsonrpc [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-explorer</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :logger, :explorer [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-failed-contract-creations</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :failed_contract_creations [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-indexer</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/config.exs](../../../services/explorer/backend/apps/indexer/config/config.exs) | <code>config :logger, :indexer [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-indexer-token-balances</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :indexer_token_balances [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-level</code> | <code>config_key</code> | [services/explorer/backend/config/dev.exs](../../../services/explorer/backend/config/dev.exs) | <code>config :logger, :level</code> |
| <code>interface.14.cfg-logger-pending-transactions-to-refetch</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :pending_transactions_to_refetch [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-reading-token-functions</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/dev.exs](../../../services/explorer/backend/apps/explorer/config/dev.exs) | <code>config :logger, :reading_token_functions [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-token-instances</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/dev.exs](../../../services/explorer/backend/apps/explorer/config/dev.exs) | <code>config :logger, :token_instances [path, metadata_filter, rotate]</code> |
| <code>interface.14.cfg-logger-web</code> | <code>config_key</code> | [services/explorer/backend/config/config.exs](../../../services/explorer/backend/config/config.exs) | <code>config :logger, :block_scout_web [metadata, metadata_filter, path, rotate]</code> |
| <code>interface.14.cfg-logger-withdrawal</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/dev.exs](../../../services/explorer/backend/apps/indexer/config/dev.exs) | <code>config :logger, :withdrawal [path, metadata_filter, rotate]</code> |

## cfg-nft-media-handler

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-nft-media-handler-cache-uniqueness-max-size</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :cache_uniqueness_max_size</code> |
| <code>interface.14.cfg-nft-media-handler-cache-uniqueness-name</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :cache_uniqueness_name</code> |
| <code>interface.14.cfg-nft-media-handler-enabled</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :enabled?</code> |
| <code>interface.14.cfg-nft-media-handler-indexer-nftmediahandler-backfiller</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, Indexer.NFTMediaHandler.Backfiller [enabled?, queue_size]</code> |
| <code>interface.14.cfg-nft-media-handler-r2-folder</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :r2_folder</code> |
| <code>interface.14.cfg-nft-media-handler-remote</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :remote?</code> |
| <code>interface.14.cfg-nft-media-handler-standalone-media-worker</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :standalone_media_worker?</code> |
| <code>interface.14.cfg-nft-media-handler-tmp-dir</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :tmp_dir</code> |
| <code>interface.14.cfg-nft-media-handler-worker</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :worker?</code> |
| <code>interface.14.cfg-nft-media-handler-worker-batch-size</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :worker_batch_size</code> |
| <code>interface.14.cfg-nft-media-handler-worker-concurrency</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :worker_concurrency</code> |
| <code>interface.14.cfg-nft-media-handler-worker-spawn-tasks-timeout</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :nft_media_handler, :worker_spawn_tasks_timeout</code> |

## cfg-oauth2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-oauth2-adapter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :oauth2, :adapter</code> |

## cfg-os-mon

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-os-mon-start-cpu-sup</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/config.exs](../../../services/explorer/backend/apps/indexer/config/config.exs) | <code>config :os_mon, :start_cpu_sup</code> |
| <code>interface.14.cfg-os-mon-start-disksup</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/config.exs](../../../services/explorer/backend/apps/indexer/config/config.exs) | <code>config :os_mon, :start_disksup</code> |
| <code>interface.14.cfg-os-mon-start-memsup</code> | <code>config_key</code> | [services/explorer/backend/apps/indexer/config/config.exs](../../../services/explorer/backend/apps/indexer/config/config.exs) | <code>config :os_mon, :start_memsup</code> |

## cfg-phoenix

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-phoenix-json-library</code> | <code>config_key</code> | [services/explorer/backend/config/config.exs](../../../services/explorer/backend/config/config.exs) | <code>config :phoenix, :json_library</code> |
| <code>interface.14.cfg-phoenix-stacktrace-depth</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/dev.exs](../../../services/explorer/backend/apps/block_scout_web/config/dev.exs) | <code>config :phoenix, :stacktrace_depth</code> |

## cfg-prometheus

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-prometheus-web-prometheus-publicexporter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :prometheus, BlockScoutWeb.Prometheus.PublicExporter [path, format, registry, auth]</code> |

## cfg-spandex-ecto

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-spandex-ecto-spandexecto-ectologger</code> | <code>config_key</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>config :spandex_ecto, SpandexEcto.EctoLogger [service, tracer, otp_app]</code> |

## cfg-spandex-phoenix

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-spandex-phoenix-tracer</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :spandex_phoenix, :tracer</code> |

## cfg-tesla

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-tesla-adapter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :tesla, :adapter</code> |
| <code>interface.14.cfg-tesla-disable-deprecated-builder-warning</code> | <code>config_key</code> | [services/explorer/backend/config/config.exs](../../../services/explorer/backend/config/config.exs) | <code>config :tesla, :disable_deprecated_builder_warning</code> |

## cfg-ueberauth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-ueberauth-ueberauth</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ueberauth, Ueberauth [logout_url, providers]</code> |
| <code>interface.14.cfg-ueberauth-ueberauth-strategy-auth0-oauth</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :ueberauth, Ueberauth.Strategy.Auth0.OAuth [domain, client_id, client_secret, auth0_application_id]</code> |

## cfg-wallaby

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-wallaby-driver</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/test.exs](../../../services/explorer/backend/apps/block_scout_web/config/test.exs) | <code>config :wallaby, :driver</code> |
| <code>interface.14.cfg-wallaby-js-errors</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/test.exs](../../../services/explorer/backend/apps/block_scout_web/config/test.exs) | <code>config :wallaby, :js_errors</code> |
| <code>interface.14.cfg-wallaby-screenshot-on-failure</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/test.exs](../../../services/explorer/backend/apps/block_scout_web/config/test.exs) | <code>config :wallaby, :screenshot_on_failure</code> |

## cfg-web

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-web-admin-panel-enabled</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, :admin_panel_enabled</code> |
| <code>interface.14.cfg-web-api-graphql</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, Api.GraphQL [enabled, token_limit, max_complexity]</code> |
| <code>interface.14.cfg-web-api-rate-limit</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :api_rate_limit [disabled, static_api_key_value, static_api_key, whitelisted_ip, ip, temporary_token, account_api_key, no_rate_limit_api_key_value, whitelisted_ips, api_v2_token_ttl, eth_json_rpc_max_batch_size, redis_url]</code> |
| <code>interface.14.cfg-web-api-url</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :api_url</code> |
| <code>interface.14.cfg-web-apps</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :apps</code> |
| <code>interface.14.cfg-web-apps-menu</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :apps_menu</code> |
| <code>interface.14.cfg-web-chain-id</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :chain_id</code> |
| <code>interface.14.cfg-web-chart</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :chart [chart_config, price_chart_legend_enabled?]</code> |
| <code>interface.14.cfg-web-checksum-address-hashes</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :checksum_address_hashes</code> |
| <code>interface.14.cfg-web-circles-addresses</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :circles_addresses</code> |
| <code>interface.14.cfg-web-contract</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :contract [verification_max_libraries, max_length_to_show_string_without_trimming, disable_interaction, certified_list, partial_reverification_disabled]</code> |
| <code>interface.14.cfg-web-cookie-domain</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, :cookie_domain</code> |
| <code>interface.14.cfg-web-dark-forest-addresses</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :dark_forest_addresses</code> |
| <code>interface.14.cfg-web-dark-forest-addresses-v-0-5</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :dark_forest_addresses_v_0_5</code> |
| <code>interface.14.cfg-web-disable-add-to-mm-button</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :disable_add_to_mm_button</code> |
| <code>interface.14.cfg-web-disable-api</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :disable_api?</code> |
| <code>interface.14.cfg-web-display-token-icons</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :display_token_icons</code> |
| <code>interface.14.cfg-web-ecto-repos</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, :ecto_repos</code> |
| <code>interface.14.cfg-web-footer</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :footer [logo, chat_link, github_link, forum_link_enabled, forum_link, telegram_link_enabled, telegram_link, link_to_other_explorers, other_explorers]</code> |
| <code>interface.14.cfg-web-gas-price</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :gas_price</code> |
| <code>interface.14.cfg-web-hide-block-miner</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :hide_block_miner</code> |
| <code>interface.14.cfg-web-hide-scam-addresses</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :hide_scam_addresses</code> |
| <code>interface.14.cfg-web-json-rpc</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :json_rpc</code> |
| <code>interface.14.cfg-web-namespace</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, :namespace</code> |
| <code>interface.14.cfg-web-new-tags</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :new_tags</code> |
| <code>interface.14.cfg-web-other-networks</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :other_networks</code> |
| <code>interface.14.cfg-web-permanent-dark-mode-enabled</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :permanent_dark_mode_enabled</code> |
| <code>interface.14.cfg-web-permanent-light-mode-enabled</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :permanent_light_mode_enabled</code> |
| <code>interface.14.cfg-web-recaptcha</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :recaptcha [v2_client_key, v2_secret_key, v3_client_key, v3_secret_key, is_disabled, check_hostname?, score_threshold, bypass_token, scoped_bypass_tokens]</code> |
| <code>interface.14.cfg-web-release-link</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :release_link</code> |
| <code>interface.14.cfg-web-remote-ip</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :remote_ip [is_blockscout_behind_proxy, headers, proxies]</code> |
| <code>interface.14.cfg-web-sensitive-endpoints-api-key</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :sensitive_endpoints_api_key</code> |
| <code>interface.14.cfg-web-show-percentage</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :show_percentage</code> |
| <code>interface.14.cfg-web-show-tenderly-link</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :show_tenderly_link</code> |
| <code>interface.14.cfg-web-sql-sandbox</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/test.exs](../../../services/explorer/backend/apps/block_scout_web/config/test.exs) | <code>config :block_scout_web, :sql_sandbox</code> |
| <code>interface.14.cfg-web-version</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :version</code> |
| <code>interface.14.cfg-web-web-api-v2</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, BlockScoutWeb.API.V2 [enabled]</code> |
| <code>interface.14.cfg-web-web-chain</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, BlockScoutWeb.Chain [network, subnetwork, network_icon, logo, logo_text, has_emission_funds, show_maintenance_alert, enable_testnet_label, testnet_label_text]</code> |
| <code>interface.14.cfg-web-web-chain-address-coinbalance</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, BlockScoutWeb.Chain.Address.CoinBalance [coin_balance_history_days]</code> |
| <code>interface.14.cfg-web-web-chain-transactionhistorychartcontroller</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Chain.TransactionHistoryChartController</code> |
| <code>interface.14.cfg-web-web-counters-blocksindexedcounter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Counters.BlocksIndexedCounter [enabled]</code> |
| <code>interface.14.cfg-web-web-counters-internaltransactionsindexedcounter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Counters.InternalTransactionsIndexedCounter [enabled]</code> |
| <code>interface.14.cfg-web-web-cspheader</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.CSPHeader [mixpanel_url, amplitude_url]</code> |
| <code>interface.14.cfg-web-web-endpoint</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, BlockScoutWeb.Endpoint [server, url, render_errors, pubsub_server, secret_key_base, check_origin, http, debug_errors, code_reloader, watchers, live_reload, cache_static_manifest, force_ssl, checksum_address_hashes]</code> |
| <code>interface.14.cfg-web-web-gettext</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Gettext [locales, default_locale]</code> |
| <code>interface.14.cfg-web-web-healthendpoint</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, BlockScoutWeb.HealthEndpoint [server, url, render_errors, pubsub_server, secret_key_base, check_origin, http]</code> |
| <code>interface.14.cfg-web-web-microserviceinterfaces-transactioninterpretation</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, BlockScoutWeb.MicroserviceInterfaces.TransactionInterpretation [service_url, enabled]</code> |
| <code>interface.14.cfg-web-web-routers-apirouter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Routers.ApiRouter [writing_enabled, reading_enabled]</code> |
| <code>interface.14.cfg-web-web-routers-webrouter</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Routers.WebRouter [enabled]</code> |
| <code>interface.14.cfg-web-web-socialmedia</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.SocialMedia [twitter, telegram, facebook, instagram]</code> |
| <code>interface.14.cfg-web-web-tracer</code> | <code>config_key</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>config :block_scout_web, BlockScoutWeb.Tracer [service, adapter, trace_key, env, disabled?]</code> |
| <code>interface.14.cfg-web-webapp-url</code> | <code>config_key</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :block_scout_web, :webapp_url</code> |

## env-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-account-auth0-application-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_AUTH0_APPLICATION_ID</code> |
| <code>interface.14.env-account-auth0-client-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_AUTH0_CLIENT_ID</code> |
| <code>interface.14.env-account-auth0-client-secret</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_AUTH0_CLIENT_SECRET</code> |
| <code>interface.14.env-account-auth0-domain</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_AUTH0_DOMAIN</code> |
| <code>interface.14.env-account-cloak-key</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/vault.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/vault.ex) | <code>ACCOUNT_CLOAK_KEY</code> |
| <code>interface.14.env-account-database-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>ACCOUNT_DATABASE_URL</code> |
| <code>interface.14.env-account-dynamic-env-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_DYNAMIC_ENV_ID</code> |
| <code>interface.14.env-account-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_ENABLED</code> |
| <code>interface.14.env-account-keycloak-client-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_KEYCLOAK_CLIENT_ID</code> |
| <code>interface.14.env-account-keycloak-client-secret</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_KEYCLOAK_CLIENT_SECRET</code> |
| <code>interface.14.env-account-keycloak-domain</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_KEYCLOAK_DOMAIN</code> |
| <code>interface.14.env-account-keycloak-email-webhook-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_KEYCLOAK_EMAIL_WEBHOOK_URL</code> |
| <code>interface.14.env-account-keycloak-realm</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_KEYCLOAK_REALM</code> |
| <code>interface.14.env-account-otp-resend-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_OTP_RESEND_INTERVAL</code> |
| <code>interface.14.env-account-pool-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>ACCOUNT_POOL_SIZE</code> |
| <code>interface.14.env-account-private-tags-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_PRIVATE_TAGS_LIMIT</code> |
| <code>interface.14.env-account-redis-url</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/application.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/application.ex) | <code>{ConfigHelper.parse_url_env_var("ACCOUNT_REDIS_URL", "&lt;url&gt;"), [name: :redix]}</code> |
| <code>interface.14.env-account-sendgrid-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_SENDGRID_API_KEY</code> |
| <code>interface.14.env-account-sendgrid-otp-template</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_SENDGRID_OTP_TEMPLATE</code> |
| <code>interface.14.env-account-sendgrid-sender</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_SENDGRID_SENDER</code> |
| <code>interface.14.env-account-sendgrid-template</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_SENDGRID_TEMPLATE</code> |
| <code>interface.14.env-account-siwe-message</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_SIWE_MESSAGE</code> |
| <code>interface.14.env-account-verification-email-resend-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_VERIFICATION_EMAIL_RESEND_INTERVAL</code> |
| <code>interface.14.env-account-watchlist-addresses-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_WATCHLIST_ADDRESSES_LIMIT</code> |
| <code>interface.14.env-account-watchlist-notifications-limit-for-30-days</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ACCOUNT_WATCHLIST_NOTIFICATIONS_LIMIT_FOR_30_DAYS</code> |

## env-addresses

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-addresses-blacklist</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_BLACKLIST</code> |
| <code>interface.14.env-addresses-blacklist-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_BLACKLIST_KEY</code> |
| <code>interface.14.env-addresses-blacklist-provider</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_BLACKLIST_PROVIDER</code> |
| <code>interface.14.env-addresses-blacklist-retry-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_BLACKLIST_RETRY_INTERVAL</code> |
| <code>interface.14.env-addresses-blacklist-update-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_BLACKLIST_UPDATE_INTERVAL</code> |
| <code>interface.14.env-addresses-blacklist-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_BLACKLIST_URL</code> |
| <code>interface.14.env-addresses-tabs-counters-ttl</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ADDRESSES_TABS_COUNTERS_TTL</code> |

## env-admin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-admin-panel-enabled</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>ADMIN_PANEL_ENABLED</code> |

## env-amb

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-amb-bridge-mediators</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex) | <code>AMB_BRIDGE_MEDIATORS</code> |

## env-amplitude

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-amplitude-url</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>amplitude_url: ConfigHelper.parse_url_env_var("AMPLITUDE_URL", "&lt;url&gt;")</code> |

## env-api

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-api-disable-contract-creation-internal-transaction-association</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_DISABLE_CONTRACT_CREATION_INTERNAL_TRANSACTION_ASSOCIATION</code> |
| <code>interface.14.env-api-graphql-default-transaction-hash</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_DEFAULT_TRANSACTION_HASH</code> |
| <code>interface.14.env-api-graphql-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_ENABLED</code> |
| <code>interface.14.env-api-graphql-max-complexity</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>API_GRAPHQL_MAX_COMPLEXITY</code> |
| <code>interface.14.env-api-graphql-rate-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT</code> |
| <code>interface.14.env-api-graphql-rate-limit-by-ip</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT_BY_IP</code> |
| <code>interface.14.env-api-graphql-rate-limit-by-ip-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT_BY_IP_TIME_INTERVAL</code> |
| <code>interface.14.env-api-graphql-rate-limit-by-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT_BY_KEY</code> |
| <code>interface.14.env-api-graphql-rate-limit-disabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT_DISABLED</code> |
| <code>interface.14.env-api-graphql-rate-limit-static-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT_STATIC_API_KEY</code> |
| <code>interface.14.env-api-graphql-rate-limit-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_GRAPHQL_RATE_LIMIT_TIME_INTERVAL</code> |
| <code>interface.14.env-api-graphql-token-limit</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>API_GRAPHQL_TOKEN_LIMIT</code> |
| <code>interface.14.env-api-internal-transactions-indexing-finished-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_INTERNAL_TRANSACTIONS_INDEXING_FINISHED_THRESHOLD</code> |
| <code>interface.14.env-api-no-rate-limit-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_NO_RATE_LIMIT_API_KEY</code> |
| <code>interface.14.env-api-rate-limit-by-account-api-key-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_ACCOUNT_API_KEY_TIME_INTERVAL</code> |
| <code>interface.14.env-api-rate-limit-by-ip</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_IP</code> |
| <code>interface.14.env-api-rate-limit-by-ip-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_IP_TIME_INTERVAL</code> |
| <code>interface.14.env-api-rate-limit-by-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_KEY</code> |
| <code>interface.14.env-api-rate-limit-by-key-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_KEY_TIME_INTERVAL</code> |
| <code>interface.14.env-api-rate-limit-by-whitelisted-ip</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_WHITELISTED_IP</code> |
| <code>interface.14.env-api-rate-limit-by-whitelisted-ip-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_BY_WHITELISTED_IP_TIME_INTERVAL</code> |
| <code>interface.14.env-api-rate-limit-config-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_CONFIG_URL</code> |
| <code>interface.14.env-api-rate-limit-disabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_DISABLED</code> |
| <code>interface.14.env-api-rate-limit-hammer-redis-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_HAMMER_REDIS_URL</code> |
| <code>interface.14.env-api-rate-limit-is-blockscout-behind-proxy</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_IS_BLOCKSCOUT_BEHIND_PROXY</code> |
| <code>interface.14.env-api-rate-limit-remote-ip-headers</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_REMOTE_IP_HEADERS</code> |
| <code>interface.14.env-api-rate-limit-remote-ip-known-proxies</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_REMOTE_IP_KNOWN_PROXIES</code> |
| <code>interface.14.env-api-rate-limit-static-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_STATIC_API_KEY</code> |
| <code>interface.14.env-api-rate-limit-ui-v2-token-ttl</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_UI_V2_TOKEN_TTL</code> |
| <code>interface.14.env-api-rate-limit-ui-v2-with-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_UI_V2_WITH_TOKEN</code> |
| <code>interface.14.env-api-rate-limit-ui-v2-with-token-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_UI_V2_WITH_TOKEN_TIME_INTERVAL</code> |
| <code>interface.14.env-api-rate-limit-whitelisted-ips</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_RATE_LIMIT_WHITELISTED_IPS</code> |
| <code>interface.14.env-api-sensitive-endpoints-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_SENSITIVE_ENDPOINTS_KEY</code> |
| <code>interface.14.env-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_URL</code> |
| <code>interface.14.env-api-v1-read-methods-disabled</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>API_V1_READ_METHODS_DISABLED</code> |
| <code>interface.14.env-api-v1-write-methods-disabled</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>API_V1_WRITE_METHODS_DISABLED</code> |
| <code>interface.14.env-api-v2-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>API_V2_ENABLED</code> |

## env-application

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-application-mode</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>APPLICATION_MODE</code> |

## env-apps

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-apps</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXTERNAL_APPS</code> |
| <code>interface.14.env-apps-menu</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>APPS_MENU</code> |

## env-beryx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-beryx-api-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BERYX_API_BASE_URL</code> |
| <code>interface.14.env-beryx-api-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BERYX_API_TOKEN</code> |

## env-blackfort

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-blackfort-validator-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BLACKFORT_VALIDATOR_API_URL</code> |

## env-block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-block-miner-gets-burnt-fees</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BLOCK_MINER_GETS_BURNT_FEES</code> |
| <code>interface.14.env-block-ranges</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BLOCK_RANGES</code> |
| <code>interface.14.env-block-transformer</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>BLOCK_TRANSFORMER</code> |

## env-blockscout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-blockscout-host</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>host: System.get_env("BLOCKSCOUT_HOST") &#124;&#124; "&lt;host&gt;"</code> |
| <code>interface.14.env-blockscout-protocol</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BLOCKSCOUT_PROTOCOL</code> |
| <code>interface.14.env-blockscout-version</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BLOCKSCOUT_VERSION</code> |

## env-bridged

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-bridged-tokens-amb-bridge-mediators</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BRIDGED_TOKENS_AMB_BRIDGE_MEDIATORS</code> |
| <code>interface.14.env-bridged-tokens-bsc-omni-bridge-mediator</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BRIDGED_TOKENS_BSC_OMNI_BRIDGE_MEDIATOR</code> |
| <code>interface.14.env-bridged-tokens-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>BRIDGED_TOKENS_ENABLED</code> |
| <code>interface.14.env-bridged-tokens-eth-omni-bridge-mediator</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BRIDGED_TOKENS_ETH_OMNI_BRIDGE_MEDIATOR</code> |
| <code>interface.14.env-bridged-tokens-foreign-json-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BRIDGED_TOKENS_FOREIGN_JSON_RPC</code> |
| <code>interface.14.env-bridged-tokens-poa-omni-bridge-mediator</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>BRIDGED_TOKENS_POA_OMNI_BRIDGE_MEDIATOR</code> |

## env-cache

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-cache-address-count-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_ADDRESS_COUNT_PERIOD</code> |
| <code>interface.14.env-cache-address-sum-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_ADDRESS_SUM_PERIOD</code> |
| <code>interface.14.env-cache-address-token-transfers-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_ADDRESS_TOKEN_TRANSFERS_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-address-tokens-usd-sum-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_ADDRESS_TOKENS_USD_SUM_PERIOD</code> |
| <code>interface.14.env-cache-address-transactions-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_ADDRESS_TRANSACTIONS_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-address-transactions-gas-usage-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_ADDRESS_TRANSACTIONS_GAS_USAGE_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-average-block-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_AVERAGE_BLOCK_PERIOD</code> |
| <code>interface.14.env-cache-average-block-time-window</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_AVERAGE_BLOCK_TIME_WINDOW</code> |
| <code>interface.14.env-cache-block-count-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_BLOCK_COUNT_PERIOD</code> |
| <code>interface.14.env-cache-fresh-pending-transactions-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_FRESH_PENDING_TRANSACTIONS_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-market-history-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_MARKET_HISTORY_PERIOD</code> |
| <code>interface.14.env-cache-optimism-last-output-root-size-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_OPTIMISM_LAST_OUTPUT_ROOT_SIZE_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-pending-operations-count-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_PENDING_OPERATIONS_COUNT_PERIOD</code> |
| <code>interface.14.env-cache-token-holders-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_TOKEN_HOLDERS_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-token-transfers-counter-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_TOKEN_TRANSFERS_COUNTER_PERIOD</code> |
| <code>interface.14.env-cache-total-gas-usage-counter-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_TOTAL_GAS_USAGE_COUNTER_ENABLED</code> |
| <code>interface.14.env-cache-total-gas-usage-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_TOTAL_GAS_USAGE_PERIOD</code> |
| <code>interface.14.env-cache-transactions-24h-stats-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_TRANSACTIONS_24H_STATS_PERIOD</code> |
| <code>interface.14.env-cache-txs-count-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CACHE_TXS_COUNT_PERIOD</code> |

## env-celo

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-celo-accounts-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_ACCOUNTS_CONTRACT</code> |
| <code>interface.14.env-celo-core-contracts</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_CORE_CONTRACTS</code> |
| <code>interface.14.env-celo-epoch-manager-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_EPOCH_MANAGER_CONTRACT</code> |
| <code>interface.14.env-celo-l2-migration-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_L2_MIGRATION_BLOCK</code> |
| <code>interface.14.env-celo-locked-gold-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_LOCKED_GOLD_CONTRACT</code> |
| <code>interface.14.env-celo-unreleased-treasury-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_UNRELEASED_TREASURY_CONTRACT</code> |
| <code>interface.14.env-celo-validators-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CELO_VALIDATORS_CONTRACT</code> |

## env-certfile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-certfile</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>certfile: System.get_env("CERTFILE") &#124;&#124; "priv/cert/selfsigned.pem",</code> |

## env-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-chain-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CHAIN_ID</code> |
| <code>interface.14.env-chain-spec-path</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CHAIN_SPEC_PATH</code> |
| <code>interface.14.env-chain-spec-processing-delay</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CHAIN_SPEC_PROCESSING_DELAY</code> |
| <code>interface.14.env-chain-type</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>CHAIN_TYPE</code> |

## env-check

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-check-origin</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/prod.exs](../../../services/explorer/backend/config/runtime/prod.exs) | <code>CHECK_ORIGIN</code> |

## env-checksum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-checksum-address-hashes</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CHECKSUM_ADDRESS_HASHES</code> |

## env-coin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-coin</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>coin = System.get_env("COIN") &#124;&#124; "ETH"</code> |
| <code>interface.14.env-coin-balance-history-days</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>COIN_BALANCE_HISTORY_DAYS</code> |
| <code>interface.14.env-coin-balance-on-demand-fetcher-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>COIN_BALANCE_ON_DEMAND_FETCHER_THRESHOLD</code> |
| <code>interface.14.env-coin-name</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>COIN_NAME</code> |

## env-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-contract-audit-reports-airtable-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_AUDIT_REPORTS_AIRTABLE_API_KEY</code> |
| <code>interface.14.env-contract-audit-reports-airtable-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_AUDIT_REPORTS_AIRTABLE_URL</code> |
| <code>interface.14.env-contract-certified-list</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_CERTIFIED_LIST</code> |
| <code>interface.14.env-contract-code-on-demand-fetcher-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_CODE_ON_DEMAND_FETCHER_THRESHOLD</code> |
| <code>interface.14.env-contract-disable-interaction</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_DISABLE_INTERACTION</code> |
| <code>interface.14.env-contract-enable-partial-reverification</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_ENABLE_PARTIAL_REVERIFICATION</code> |
| <code>interface.14.env-contract-max-string-length-without-trimming</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_MAX_STRING_LENGTH_WITHOUT_TRIMMING</code> |
| <code>interface.14.env-contract-proxy-implementation-ttl-via-avg-block-time</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_PROXY_IMPLEMENTATION_TTL_VIA_AVG_BLOCK_TIME</code> |
| <code>interface.14.env-contract-verification-allowed-solidity-evm-versions</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_VERIFICATION_ALLOWED_SOLIDITY_EVM_VERSIONS</code> |
| <code>interface.14.env-contract-verification-allowed-vyper-evm-versions</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_VERIFICATION_ALLOWED_VYPER_EVM_VERSIONS</code> |
| <code>interface.14.env-contract-verification-max-libraries</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CONTRACT_VERIFICATION_MAX_LIBRARIES</code> |

## env-csv

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-csv-export-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CSV_EXPORT_LIMIT</code> |

## env-custom

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-custom-contract-addresses-amb-bridge-mediators</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex) | <code>AMB_BRIDGE_MEDIATORS, CUSTOM_CONTRACT_ADDRESSES_AMB_BRIDGE_MEDIATORS</code> |
| <code>interface.14.env-custom-contract-addresses-aox</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex) | <code>CUSTOM_CONTRACT_ADDRESSES_AOX</code> |
| <code>interface.14.env-custom-contract-addresses-circles</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CUSTOM_CONTRACT_ADDRESSES_CIRCLES</code> |
| <code>interface.14.env-custom-contract-addresses-dark-forest</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CUSTOM_CONTRACT_ADDRESSES_DARK_FOREST</code> |
| <code>interface.14.env-custom-contract-addresses-dark-forest-v-0-5</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>CUSTOM_CONTRACT_ADDRESSES_DARK_FOREST_V_0_5</code> |

## env-database

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-database-event-pool-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>DATABASE_EVENT_POOL_SIZE</code> |
| <code>interface.14.env-database-event-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>DATABASE_EVENT_URL</code> |
| <code>interface.14.env-database-queue-target</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>DATABASE_QUEUE_TARGET</code> |
| <code>interface.14.env-database-read-only-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>DATABASE_READ_ONLY_API_URL</code> |
| <code>interface.14.env-database-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>DATABASE_URL</code> |

## env-datadog

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-datadog-host</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/application.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/application.ex) | <code>host: System.get_env("DATADOG_HOST") &#124;&#124; "&lt;host&gt;",</code> |
| <code>interface.14.env-datadog-port</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DATADOG_PORT</code> |

## env-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-decode-not-a-contract-calls</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/config/config.exs](../../../services/explorer/backend/apps/explorer/config/config.exs) | <code>DECODE_NOT_A_CONTRACT_CALLS</code> |

## env-disable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-disable-add-to-mm-button</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_ADD_TO_MM_BUTTON</code> |
| <code>interface.14.env-disable-api</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_API</code> |
| <code>interface.14.env-disable-catchup-indexer</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_CATCHUP_INDEXER</code> |
| <code>interface.14.env-disable-exchange-rates</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_EXCHANGE_RATES</code> |
| <code>interface.14.env-disable-file-logging</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>DISABLE_FILE_LOGGING</code> |
| <code>interface.14.env-disable-indexer</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_INDEXER</code> |
| <code>interface.14.env-disable-market</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_MARKET</code> |
| <code>interface.14.env-disable-realtime-indexer</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_REALTIME_INDEXER</code> |
| <code>interface.14.env-disable-token-exchange-rate</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISABLE_TOKEN_EXCHANGE_RATE</code> |
| <code>interface.14.env-disable-webapp</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>DISABLE_WEBAPP</code> |

## env-display

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-display-token-icons</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>DISPLAY_TOKEN_ICONS</code> |

## env-ecto

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-ecto-use-ssl</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex) | <code>ECTO_USE_SSL</code> |

## env-eip

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-eip-1559-base-fee-lower-bound-wei</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EIP_1559_BASE_FEE_LOWER_BOUND_WEI</code> |
| <code>interface.14.env-eip-1559-base-fee-max-change-denominator</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EIP_1559_BASE_FEE_MAX_CHANGE_DENOMINATOR</code> |
| <code>interface.14.env-eip-1559-elasticity-multiplier</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EIP_1559_ELASTICITY_MULTIPLIER</code> |

## env-emission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-emission-format</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EMISSION_FORMAT</code> |

## env-eth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-eth-json-rpc-max-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETH_JSON_RPC_MAX_BATCH_SIZE</code> |

## env-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-ethereum-jsonrpc-archive-balances-window</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_ARCHIVE_BALANCES_WINDOW</code> |
| <code>interface.14.env-ethereum-jsonrpc-debug-trace-transaction-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_DEBUG_TRACE_TRANSACTION_TIMEOUT</code> |
| <code>interface.14.env-ethereum-jsonrpc-disable-archive-balances</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_DISABLE_ARCHIVE_BALANCES</code> |
| <code>interface.14.env-ethereum-jsonrpc-eth-call-url</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_ETH_CALL_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-eth-call-urls</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_ETH_CALL_URLS</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-eth-call-url</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-eth-call-urls</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_ETH_CALL_URLS</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-http-url</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_HTTP_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-http-urls</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_HTTP_URLS</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-trace-url</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_TRACE_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-trace-urls</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_TRACE_URLS</code> |
| <code>interface.14.env-ethereum-jsonrpc-fallback-ws-url</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/config/dev/anvil.exs](../../../services/explorer/backend/apps/explorer/config/dev/anvil.exs) | <code>ETHEREUM_JSONRPC_FALLBACK_WS_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-geth-allow-empty-traces</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_GETH_ALLOW_EMPTY_TRACES</code> |
| <code>interface.14.env-ethereum-jsonrpc-geth-trace-by-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_GETH_TRACE_BY_BLOCK</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-gzip-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_HTTP_GZIP_ENABLED</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-headers</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_HTTP_HEADERS</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-insecure</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_HTTP_INSECURE</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_HTTP_TIMEOUT</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-url</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_HTTP_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-http-urls</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_HTTP_URLS</code> |
| <code>interface.14.env-ethereum-jsonrpc-max-receipts-by-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_MAX_RECEIPTS_BY_BLOCK</code> |
| <code>interface.14.env-ethereum-jsonrpc-password</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_PASSWORD</code> |
| <code>interface.14.env-ethereum-jsonrpc-pending-transactions-type</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_PENDING_TRANSACTIONS_TYPE</code> |
| <code>interface.14.env-ethereum-jsonrpc-receipts-by-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_RECEIPTS_BY_BLOCK</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-archive-max-inflight</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_THROTTLE_ARCHIVE_MAX_INFLIGHT</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-coalesce-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_THROTTLE_COALESCE_ENABLED</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-max-inflight</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_THROTTLE_MAX_INFLIGHT</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-rps</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_THROTTLE_RPS</code> |
| <code>interface.14.env-ethereum-jsonrpc-throttle-trace-max-inflight</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_THROTTLE_TRACE_MAX_INFLIGHT</code> |
| <code>interface.14.env-ethereum-jsonrpc-trace-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_TRACE_URL</code> |
| <code>interface.14.env-ethereum-jsonrpc-trace-urls</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_TRACE_URLS</code> |
| <code>interface.14.env-ethereum-jsonrpc-transport</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_TRANSPORT</code> |
| <code>interface.14.env-ethereum-jsonrpc-user</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>ETHEREUM_JSONRPC_USER</code> |
| <code>interface.14.env-ethereum-jsonrpc-variant</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_VARIANT</code> |
| <code>interface.14.env-ethereum-jsonrpc-wait-per-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_WAIT_PER_TIMEOUT</code> |
| <code>interface.14.env-ethereum-jsonrpc-ws-retry-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ETHEREUM_JSONRPC_WS_RETRY_INTERVAL</code> |
| <code>interface.14.env-ethereum-jsonrpc-ws-url</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/config/dev/anvil.exs](../../../services/explorer/backend/apps/explorer/config/dev/anvil.exs) | <code>ETHEREUM_JSONRPC_WS_URL</code> |

## env-exchange

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-exchange-rates-coingecko-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINGECKO_API_KEY</code> |
| <code>interface.14.env-exchange-rates-coingecko-base-pro-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINGECKO_BASE_PRO_URL</code> |
| <code>interface.14.env-exchange-rates-coingecko-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINGECKO_BASE_URL</code> |
| <code>interface.14.env-exchange-rates-coingecko-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINGECKO_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-coingecko-platform-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINGECKO_PLATFORM_ID</code> |
| <code>interface.14.env-exchange-rates-coingecko-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINGECKO_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINMARKETCAP_API_KEY</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINMARKETCAP_BASE_URL</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINMARKETCAP_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-coinmarketcap-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_COINMARKETCAP_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-cryptocompare-secondary-coin-symbol</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_CRYPTOCOMPARE_SECONDARY_COIN_SYMBOL</code> |
| <code>interface.14.env-exchange-rates-cryptorank-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_CRYPTORANK_API_KEY</code> |
| <code>interface.14.env-exchange-rates-cryptorank-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_CRYPTORANK_BASE_URL</code> |
| <code>interface.14.env-exchange-rates-cryptorank-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_CRYPTORANK_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-cryptorank-platform-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_CRYPTORANK_PLATFORM_ID</code> |
| <code>interface.14.env-exchange-rates-cryptorank-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_CRYPTORANK_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-history-first-fetch-day-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_HISTORY_FIRST_FETCH_DAY_COUNT</code> |
| <code>interface.14.env-exchange-rates-market-cap-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_MARKET_CAP_SOURCE</code> |
| <code>interface.14.env-exchange-rates-mobula-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_MOBULA_API_KEY</code> |
| <code>interface.14.env-exchange-rates-mobula-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_MOBULA_BASE_URL</code> |
| <code>interface.14.env-exchange-rates-mobula-chain-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_MOBULA_CHAIN_ID</code> |
| <code>interface.14.env-exchange-rates-mobula-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_MOBULA_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-mobula-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_MOBULA_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-exchange-rates-price-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_PRICE_SOURCE</code> |
| <code>interface.14.env-exchange-rates-secondary-coin-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_SECONDARY_COIN_SOURCE</code> |
| <code>interface.14.env-exchange-rates-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_SOURCE</code> |
| <code>interface.14.env-exchange-rates-tvl-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXCHANGE_RATES_TVL_SOURCE</code> |

## env-external

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-external-apps</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>EXTERNAL_APPS</code> |

## env-fetch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fetch-rewards-way</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FETCH_REWARDS_WAY</code> |

## env-filecoin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-filecoin-network-prefix</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FILECOIN_NETWORK_PREFIX</code> |

## env-filfox

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-filfox-api-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>base_url: ConfigHelper.parse_url_env_var("FILFOX_API_BASE_URL", "&lt;url&gt;")</code> |

## env-first

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-first-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FIRST_BLOCK</code> |

## env-footer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-footer-chat-link</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>chat_link: System.get_env("FOOTER_CHAT_LINK", "&lt;url&gt;"),</code> |
| <code>interface.14.env-footer-forum-link</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>forum_link: System.get_env("FOOTER_FORUM_LINK", "&lt;url&gt;"),</code> |
| <code>interface.14.env-footer-forum-link-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FOOTER_FORUM_LINK_ENABLED</code> |
| <code>interface.14.env-footer-github-link</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>github_link: System.get_env("FOOTER_GITHUB_LINK", "&lt;url&gt;"),</code> |
| <code>interface.14.env-footer-link-to-other-explorers</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FOOTER_LINK_TO_OTHER_EXPLORERS</code> |
| <code>interface.14.env-footer-logo</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FOOTER_LOGO</code> |
| <code>interface.14.env-footer-other-explorers</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FOOTER_OTHER_EXPLORERS</code> |
| <code>interface.14.env-footer-telegram-link</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FOOTER_TELEGRAM_LINK</code> |
| <code>interface.14.env-footer-telegram-link-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>FOOTER_TELEGRAM_LINK_ENABLED</code> |

## env-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-gas-price</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE</code> |
| <code>interface.14.env-gas-price-oracle-average-percentile</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_AVERAGE_PERCENTILE</code> |
| <code>interface.14.env-gas-price-oracle-average-time-coefficient</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_AVERAGE_TIME_COEFFICIENT</code> |
| <code>interface.14.env-gas-price-oracle-cache-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_CACHE_PERIOD</code> |
| <code>interface.14.env-gas-price-oracle-fast-percentile</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_FAST_PERCENTILE</code> |
| <code>interface.14.env-gas-price-oracle-fast-time-coefficient</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_FAST_TIME_COEFFICIENT</code> |
| <code>interface.14.env-gas-price-oracle-num-of-blocks</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_NUM_OF_BLOCKS</code> |
| <code>interface.14.env-gas-price-oracle-safelow-percentile</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_SAFELOW_PERCENTILE</code> |
| <code>interface.14.env-gas-price-oracle-safelow-time-coefficient</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_SAFELOW_TIME_COEFFICIENT</code> |
| <code>interface.14.env-gas-price-oracle-simple-transaction-gas</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>GAS_PRICE_ORACLE_SIMPLE_TRANSACTION_GAS</code> |

## env-hackney

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-hackney-default-pool-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>HACKNEY_DEFAULT_POOL_SIZE</code> |

## env-health

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-health-monitor-batches-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>HEALTH_MONITOR_BATCHES_PERIOD</code> |
| <code>interface.14.env-health-monitor-blocks-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>HEALTH_MONITOR_BLOCKS_PERIOD</code> |
| <code>interface.14.env-health-monitor-check-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>HEALTH_MONITOR_CHECK_INTERVAL</code> |

## env-hide

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-hide-block-miner</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>HIDE_BLOCK_MINER</code> |
| <code>interface.14.env-hide-scam-addresses</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>HIDE_SCAM_ADDRESSES</code> |

## env-indexer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-indexer-arbitrum-arbsys-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_ARBSYS_CONTRACT</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_BATCHES_TRACKING_ENABLED</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-failure-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_BATCHES_TRACKING_FAILURE_THRESHOLD</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-l1-finalization-check-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_BATCHES_TRACKING_L1_FINALIZATION_CHECK_ENABLED</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-messages-to-blocks-shift</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_BATCHES_TRACKING_MESSAGES_TO_BLOCKS_SHIFT</code> |
| <code>interface.14.env-indexer-arbitrum-batches-tracking-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_BATCHES_TRACKING_RECHECK_INTERVAL</code> |
| <code>interface.14.env-indexer-arbitrum-bridge-messages-tracking-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_BRIDGE_MESSAGES_TRACKING_ENABLED</code> |
| <code>interface.14.env-indexer-arbitrum-confirmations-tracking-finalized</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_CONFIRMATIONS_TRACKING_FINALIZED</code> |
| <code>interface.14.env-indexer-arbitrum-data-backfill-blocks-depth</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_DATA_BACKFILL_BLOCKS_DEPTH</code> |
| <code>interface.14.env-indexer-arbitrum-data-backfill-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_DATA_BACKFILL_ENABLED</code> |
| <code>interface.14.env-indexer-arbitrum-data-backfill-unindexed-blocks-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_DATA_BACKFILL_UNINDEXED_BLOCKS_RECHECK_INTERVAL</code> |
| <code>interface.14.env-indexer-arbitrum-l1-common-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_COMMON_START_BLOCK</code> |
| <code>interface.14.env-indexer-arbitrum-l1-finalization-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_FINALIZATION_THRESHOLD</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rollup-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_ROLLUP_CONTRACT</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rollup-init-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_ROLLUP_INIT_BLOCK</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_RPC</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rpc-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_RPC_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-arbitrum-l1-rpc-historical-blocks-range</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_L1_RPC_HISTORICAL_BLOCKS_RANGE</code> |
| <code>interface.14.env-indexer-arbitrum-messages-tracking-failure-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_MESSAGES_TRACKING_FAILURE_THRESHOLD</code> |
| <code>interface.14.env-indexer-arbitrum-missed-message-ids-range</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_MISSED_MESSAGE_IDS_RANGE</code> |
| <code>interface.14.env-indexer-arbitrum-missed-messages-blocks-depth</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_MISSED_MESSAGES_BLOCKS_DEPTH</code> |
| <code>interface.14.env-indexer-arbitrum-missed-messages-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_MISSED_MESSAGES_RECHECK_INTERVAL</code> |
| <code>interface.14.env-indexer-arbitrum-missing-batches-range</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_MISSING_BATCHES_RANGE</code> |
| <code>interface.14.env-indexer-arbitrum-new-batches-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_NEW_BATCHES_LIMIT</code> |
| <code>interface.14.env-indexer-arbitrum-node-interface-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_NODE_INTERFACE_CONTRACT</code> |
| <code>interface.14.env-indexer-arbitrum-rollup-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_ROLLUP_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-arbitrum-tracking-messages-on-l1-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARBITRUM_TRACKING_MESSAGES_ON_L1_RECHECK_INTERVAL</code> |
| <code>interface.14.env-indexer-arc-native-token-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARC_NATIVE_TOKEN_CONTRACT</code> |
| <code>interface.14.env-indexer-arc-native-token-decimals</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARC_NATIVE_TOKEN_DECIMALS</code> |
| <code>interface.14.env-indexer-arc-native-token-system-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARC_NATIVE_TOKEN_SYSTEM_CONTRACT</code> |
| <code>interface.14.env-indexer-archival-token-balances-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARCHIVAL_TOKEN_BALANCES_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-archival-token-balances-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARCHIVAL_TOKEN_BALANCES_CONCURRENCY</code> |
| <code>interface.14.env-indexer-archival-token-balances-exponential-timeout-coeff</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARCHIVAL_TOKEN_BALANCES_EXPONENTIAL_TIMEOUT_COEFF</code> |
| <code>interface.14.env-indexer-archival-token-balances-max-refetch-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ARCHIVAL_TOKEN_BALANCES_MAX_REFETCH_INTERVAL</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-end-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_BLOB_FETCHER_END_BLOCK</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-reference-slot</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_BLOB_FETCHER_REFERENCE_SLOT</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-reference-timestamp</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_BLOB_FETCHER_REFERENCE_TIMESTAMP</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-slot-duration</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_BLOB_FETCHER_SLOT_DURATION</code> |
| <code>interface.14.env-indexer-beacon-blob-fetcher-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_BLOB_FETCHER_START_BLOCK</code> |
| <code>interface.14.env-indexer-beacon-deposit-fetcher-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_DEPOSIT_FETCHER_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-beacon-deposit-fetcher-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_DEPOSIT_FETCHER_INTERVAL</code> |
| <code>interface.14.env-indexer-beacon-deposit-status-fetcher-epoch-duration</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_DEPOSIT_STATUS_FETCHER_EPOCH_DURATION</code> |
| <code>interface.14.env-indexer-beacon-deposit-status-fetcher-reference-timestamp</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BEACON_DEPOSIT_STATUS_FETCHER_REFERENCE_TIMESTAMP</code> |
| <code>interface.14.env-indexer-beacon-rpc-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>config :indexer, Indexer.Fetcher.Beacon, beacon_rpc: System.get_env("INDEXER_BEACON_RPC_URL") &#124;&#124; "&lt;url&gt;"</code> |
| <code>interface.14.env-indexer-block-reward-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BLOCK_REWARD_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-block-reward-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_BLOCK_REWARD_CONCURRENCY</code> |
| <code>interface.14.env-indexer-catchup-block-interval</code> | <code>env_var</code> | [services/explorer/backend/apps/indexer/config/dev/anvil.exs](../../../services/explorer/backend/apps/indexer/config/dev/anvil.exs) | <code>INDEXER_CATCHUP_BLOCK_INTERVAL</code> |
| <code>interface.14.env-indexer-catchup-blocks-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CATCHUP_BLOCKS_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-catchup-blocks-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CATCHUP_BLOCKS_CONCURRENCY</code> |
| <code>interface.14.env-indexer-catchup-missing-ranges-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CATCHUP_MISSING_RANGES_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-celo-accounts-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CELO_ACCOUNTS_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-celo-accounts-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CELO_ACCOUNTS_CONCURRENCY</code> |
| <code>interface.14.env-indexer-celo-validator-group-votes-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CELO_VALIDATOR_GROUP_VOTES_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-coin-balances-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_COIN_BALANCES_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-coin-balances-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_COIN_BALANCES_CONCURRENCY</code> |
| <code>interface.14.env-indexer-coin-balances-fetcher-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_COIN_BALANCES_FETCHER_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-current-token-balances-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CURRENT_TOKEN_BALANCES_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-current-token-balances-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_CURRENT_TOKEN_BALANCES_CONCURRENCY</code> |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DB_EVENT_NOTIFICATIONS_CLEANUP_ENABLED</code> |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DB_EVENT_NOTIFICATIONS_CLEANUP_INTERVAL</code> |
| <code>interface.14.env-indexer-db-event-notifications-cleanup-max-age</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DB_EVENT_NOTIFICATIONS_CLEANUP_MAX_AGE</code> |
| <code>interface.14.env-indexer-disable-address-coin-balance-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_ADDRESS_COIN_BALANCE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-archival-token-balances-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_ARCHIVAL_TOKEN_BALANCES_FETCHER</code> |
| <code>interface.14.env-indexer-disable-beacon-blob-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_BEACON_BLOB_FETCHER</code> |
| <code>interface.14.env-indexer-disable-beacon-deposit-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_BEACON_DEPOSIT_FETCHER</code> |
| <code>interface.14.env-indexer-disable-beacon-deposit-status-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_BEACON_DEPOSIT_STATUS_FETCHER</code> |
| <code>interface.14.env-indexer-disable-block-reward-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_BLOCK_REWARD_FETCHER</code> |
| <code>interface.14.env-indexer-disable-cataloged-token-updater-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_CATALOGED_TOKEN_UPDATER_FETCHER</code> |
| <code>interface.14.env-indexer-disable-celo-epoch-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_CELO_EPOCH_FETCHER</code> |
| <code>interface.14.env-indexer-disable-celo-validator-group-votes-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_CELO_VALIDATOR_GROUP_VOTES_FETCHER</code> |
| <code>interface.14.env-indexer-disable-empty-blocks-sanitizer</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_EMPTY_BLOCKS_SANITIZER</code> |
| <code>interface.14.env-indexer-disable-filecoin-address-info-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_FILECOIN_ADDRESS_INFO_FETCHER</code> |
| <code>interface.14.env-indexer-disable-hot-smart-contracts-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_HOT_SMART_CONTRACTS_FETCHER</code> |
| <code>interface.14.env-indexer-disable-internal-transactions-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_INTERNAL_TRANSACTIONS_FETCHER</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-balances-queue-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-counters-queue-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-main-queue-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-multichain-search-db-export-token-info-queue-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-optimism-interop-multichain-export</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_OPTIMISM_INTEROP_MULTICHAIN_EXPORT</code> |
| <code>interface.14.env-indexer-disable-pending-transactions-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER</code> |
| <code>interface.14.env-indexer-disable-replaced-transaction-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_REPLACED_TRANSACTION_FETCHER</code> |
| <code>interface.14.env-indexer-disable-rootstock-data-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_ROOTSTOCK_DATA_FETCHER</code> |
| <code>interface.14.env-indexer-disable-token-instance-erc-1155-sanitize-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_TOKEN_INSTANCE_ERC_1155_SANITIZE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-token-instance-erc-721-sanitize-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_TOKEN_INSTANCE_ERC_721_SANITIZE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-token-instance-realtime-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_TOKEN_INSTANCE_REALTIME_FETCHER</code> |
| <code>interface.14.env-indexer-disable-token-instance-refetch-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_TOKEN_INSTANCE_REFETCH_FETCHER</code> |
| <code>interface.14.env-indexer-disable-token-instance-retry-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_TOKEN_INSTANCE_RETRY_FETCHER</code> |
| <code>interface.14.env-indexer-disable-token-instance-sanitize-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_TOKEN_INSTANCE_SANITIZE_FETCHER</code> |
| <code>interface.14.env-indexer-disable-withdrawals-fetcher</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_DISABLE_WITHDRAWALS_FETCHER</code> |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_EMPTY_BLOCKS_SANITIZER_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-head-offset</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_EMPTY_BLOCKS_SANITIZER_HEAD_OFFSET</code> |
| <code>interface.14.env-indexer-empty-blocks-sanitizer-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_EMPTY_BLOCKS_SANITIZER_INTERVAL</code> |
| <code>interface.14.env-indexer-fetcher-init-delay</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_FETCHER_INIT_DELAY</code> |
| <code>interface.14.env-indexer-fetcher-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_FETCHER_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-filecoin-address-info-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_FILECOIN_ADDRESS_INFO_CONCURRENCY</code> |
| <code>interface.14.env-indexer-graceful-shutdown-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_GRACEFUL_SHUTDOWN_PERIOD</code> |
| <code>interface.14.env-indexer-hide-indexing-progress-alert</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_HIDE_INDEXING_PROGRESS_ALERT</code> |
| <code>interface.14.env-indexer-internal-transactions-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-internal-transactions-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_CONCURRENCY</code> |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_DELETE_QUEUE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_DELETE_QUEUE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-internal-transactions-delete-queue-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_DELETE_QUEUE_THRESHOLD</code> |
| <code>interface.14.env-indexer-internal-transactions-fetch-order</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_FETCH_ORDER</code> |
| <code>interface.14.env-indexer-internal-transactions-lookback-blocks</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_LOOKBACK_BLOCKS</code> |
| <code>interface.14.env-indexer-internal-transactions-tracer-type</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_INTERNAL_TRANSACTIONS_TRACER_TYPE</code> |
| <code>interface.14.env-indexer-memory-limit</code> | <code>env_var</code> | [services/explorer/backend/config/config_helper.exs](../../../services/explorer/backend/config/config_helper.exs) | <code>INDEXER_MEMORY_LIMIT</code> |
| <code>interface.14.env-indexer-metrics-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_METRICS_ENABLED</code> |
| <code>interface.14.env-indexer-metrics-enabled-failed-token-instances-metadata-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_METRICS_ENABLED_FAILED_TOKEN_INSTANCES_METADATA_COUNT</code> |
| <code>interface.14.env-indexer-metrics-enabled-missing-archival-token-balances-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_METRICS_ENABLED_MISSING_ARCHIVAL_TOKEN_BALANCES_COUNT</code> |
| <code>interface.14.env-indexer-metrics-enabled-missing-current-token-balances-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_METRICS_ENABLED_MISSING_CURRENT_TOKEN_BALANCES_COUNT</code> |
| <code>interface.14.env-indexer-metrics-enabled-token-instances-not-uploaded-to-cdn-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_METRICS_ENABLED_TOKEN_INSTANCES_NOT_UPLOADED_TO_CDN_COUNT</code> |
| <code>interface.14.env-indexer-metrics-enabled-unfetched-token-instances-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_METRICS_ENABLED_UNFETCHED_TOKEN_INSTANCES_COUNT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-balances-queue-max-queue-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_BALANCES_QUEUE_MAX_QUEUE_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-counters-queue-max-queue-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_COUNTERS_QUEUE_MAX_QUEUE_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-main-queue-max-queue-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_MAIN_QUEUE_MAX_QUEUE_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-multichain-search-db-export-token-info-queue-max-queue-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_MULTICHAIN_SEARCH_DB_EXPORT_TOKEN_INFO_QUEUE_MAX_QUEUE_SIZE</code> |
| <code>interface.14.env-indexer-optimism-block-duration</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_BLOCK_DURATION</code> |
| <code>interface.14.env-indexer-optimism-chainscout-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_CHAINSCOUT_API_URL</code> |
| <code>interface.14.env-indexer-optimism-chainscout-fallback-map</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_CHAINSCOUT_FALLBACK_MAP</code> |
| <code>interface.14.env-indexer-optimism-interop-connect-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_INTEROP_CONNECT_TIMEOUT</code> |
| <code>interface.14.env-indexer-optimism-interop-export-expiration-days</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_INTEROP_EXPORT_EXPIRATION_DAYS</code> |
| <code>interface.14.env-indexer-optimism-interop-private-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_INTEROP_PRIVATE_KEY</code> |
| <code>interface.14.env-indexer-optimism-interop-recv-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_INTEROP_RECV_TIMEOUT</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-alt-da-server-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_ALT_DA_SERVER_URL</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-blocks-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_BLOCKS_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-blockscout-blobs-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_BLOCKSCOUT_BLOBS_API_URL</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-celestia-blobs-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_CELESTIA_BLOBS_API_URL</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-eigenda-blobs-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_EIGENDA_BLOBS_API_URL</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-eigenda-proxy-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_EIGENDA_PROXY_BASE_URL</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-inbox</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_INBOX</code> |
| <code>interface.14.env-indexer-optimism-l1-batch-submitter</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_BATCH_SUBMITTER</code> |
| <code>interface.14.env-indexer-optimism-l1-deposits-transaction-type</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_DEPOSITS_TRANSACTION_TYPE</code> |
| <code>interface.14.env-indexer-optimism-l1-eth-get-logs-range-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_ETH_GET_LOGS_RANGE_SIZE</code> |
| <code>interface.14.env-indexer-optimism-l1-output-oracle-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_OUTPUT_ORACLE_CONTRACT</code> |
| <code>interface.14.env-indexer-optimism-l1-portal-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_PORTAL_CONTRACT</code> |
| <code>interface.14.env-indexer-optimism-l1-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_RPC</code> |
| <code>interface.14.env-indexer-optimism-l1-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_START_BLOCK</code> |
| <code>interface.14.env-indexer-optimism-l1-system-config-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L1_SYSTEM_CONFIG_CONTRACT</code> |
| <code>interface.14.env-indexer-optimism-l2-batch-genesis-block-number</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_BATCH_GENESIS_BLOCK_NUMBER</code> |
| <code>interface.14.env-indexer-optimism-l2-eth-get-logs-range-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_ETH_GET_LOGS_RANGE_SIZE</code> |
| <code>interface.14.env-indexer-optimism-l2-holocene-blocks-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_HOLOCENE_BLOCKS_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-optimism-l2-holocene-timestamp</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_HOLOCENE_TIMESTAMP</code> |
| <code>interface.14.env-indexer-optimism-l2-interop-blocks-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_INTEROP_BLOCKS_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-optimism-l2-interop-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_INTEROP_START_BLOCK</code> |
| <code>interface.14.env-indexer-optimism-l2-isthmus-timestamp</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_ISTHMUS_TIMESTAMP</code> |
| <code>interface.14.env-indexer-optimism-l2-jovian-timestamp</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_JOVIAN_TIMESTAMP</code> |
| <code>interface.14.env-indexer-optimism-l2-message-passer-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_MESSAGE_PASSER_CONTRACT</code> |
| <code>interface.14.env-indexer-optimism-l2-withdrawals-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_L2_WITHDRAWALS_START_BLOCK</code> |
| <code>interface.14.env-indexer-optimism-multichain-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_MULTICHAIN_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_ENQUEUE_BUSY_WAITING_TIMEOUT</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-optimism-operator-fee-queue-max-queue-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_OPTIMISM_OPERATOR_FEE_QUEUE_MAX_QUEUE_SIZE</code> |
| <code>interface.14.env-indexer-pending-transactions-sanitizer-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_PENDING_TRANSACTIONS_SANITIZER_INTERVAL</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_BATCHES_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_BATCHES_ENABLED</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-ignore</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_BATCHES_IGNORE</code> |
| <code>interface.14.env-indexer-polygon-zkevm-batches-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_BATCHES_RECHECK_INTERVAL</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_BRIDGE_CONTRACT</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-native-decimals</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_BRIDGE_NATIVE_DECIMALS</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-native-symbol</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_BRIDGE_NATIVE_SYMBOL</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-network-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_BRIDGE_NETWORK_ID</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-rollup-index</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_BRIDGE_ROLLUP_INDEX</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-bridge-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_BRIDGE_START_BLOCK</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l1-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L1_RPC</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L2_BRIDGE_CONTRACT</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-network-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L2_BRIDGE_NETWORK_ID</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-rollup-index</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L2_BRIDGE_ROLLUP_INDEX</code> |
| <code>interface.14.env-indexer-polygon-zkevm-l2-bridge-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_POLYGON_ZKEVM_L2_BRIDGE_START_BLOCK</code> |
| <code>interface.14.env-indexer-realtime-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_REALTIME_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-realtime-fetcher-max-gap</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_REALTIME_FETCHER_MAX_GAP</code> |
| <code>interface.14.env-indexer-realtime-fetcher-polling-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_REALTIME_FETCHER_POLLING_PERIOD</code> |
| <code>interface.14.env-indexer-realtime-max-lag</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_REALTIME_MAX_LAG</code> |
| <code>interface.14.env-indexer-receipts-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_RECEIPTS_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-receipts-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_RECEIPTS_CONCURRENCY</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ROOTSTOCK_DATA_FETCHER_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ROOTSTOCK_DATA_FETCHER_CONCURRENCY</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-db-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ROOTSTOCK_DATA_FETCHER_DB_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-rootstock-data-fetcher-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ROOTSTOCK_DATA_FETCHER_INTERVAL</code> |
| <code>interface.14.env-indexer-scroll-l1-batch-blockscout-blobs-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_BATCH_BLOCKSCOUT_BLOBS_API_URL</code> |
| <code>interface.14.env-indexer-scroll-l1-batch-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_BATCH_START_BLOCK</code> |
| <code>interface.14.env-indexer-scroll-l1-chain-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_CHAIN_CONTRACT</code> |
| <code>interface.14.env-indexer-scroll-l1-eth-get-logs-range-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_ETH_GET_LOGS_RANGE_SIZE</code> |
| <code>interface.14.env-indexer-scroll-l1-messenger-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_MESSENGER_CONTRACT</code> |
| <code>interface.14.env-indexer-scroll-l1-messenger-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_MESSENGER_START_BLOCK</code> |
| <code>interface.14.env-indexer-scroll-l1-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L1_RPC</code> |
| <code>interface.14.env-indexer-scroll-l2-eth-get-logs-range-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L2_ETH_GET_LOGS_RANGE_SIZE</code> |
| <code>interface.14.env-indexer-scroll-l2-gas-oracle-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L2_GAS_ORACLE_CONTRACT</code> |
| <code>interface.14.env-indexer-scroll-l2-messenger-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L2_MESSENGER_CONTRACT</code> |
| <code>interface.14.env-indexer-scroll-l2-messenger-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SCROLL_L2_MESSENGER_START_BLOCK</code> |
| <code>interface.14.env-indexer-shibarium-l1-deposit-manager-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_DEPOSIT_MANAGER_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l1-erc1155-predicate-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_ERC1155_PREDICATE_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l1-erc20-predicate-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_ERC20_PREDICATE_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l1-erc721-predicate-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_ERC721_PREDICATE_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l1-ether-predicate-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_ETHER_PREDICATE_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l1-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_RPC</code> |
| <code>interface.14.env-indexer-shibarium-l1-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_START_BLOCK</code> |
| <code>interface.14.env-indexer-shibarium-l1-withdraw-manager-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L1_WITHDRAW_MANAGER_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l2-bone-withdraw-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L2_BONE_WITHDRAW_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l2-child-chain-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L2_CHILD_CHAIN_CONTRACT</code> |
| <code>interface.14.env-indexer-shibarium-l2-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L2_START_BLOCK</code> |
| <code>interface.14.env-indexer-shibarium-l2-weth-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SHIBARIUM_L2_WETH_CONTRACT</code> |
| <code>interface.14.env-indexer-signed-authorization-status-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SIGNED_AUTHORIZATION_STATUS_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-system-memory-percentage</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_SYSTEM_MEMORY_PERCENTAGE</code> |
| <code>interface.14.env-indexer-token-balances-fetcher-init-query-limit</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_BALANCES_FETCHER_INIT_QUERY_LIMIT</code> |
| <code>interface.14.env-indexer-token-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_CONCURRENCY</code> |
| <code>interface.14.env-indexer-token-instance-allowed-uri-protocols</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_ALLOWED_URI_PROTOCOLS</code> |
| <code>interface.14.env-indexer-token-instance-cidr-blacklist</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_CIDR_BLACKLIST</code> |
| <code>interface.14.env-indexer-token-instance-host-filtering-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_HOST_FILTERING_ENABLED</code> |
| <code>interface.14.env-indexer-token-instance-realtime-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_REALTIME_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-token-instance-realtime-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_REALTIME_CONCURRENCY</code> |
| <code>interface.14.env-indexer-token-instance-realtime-retry-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_REALTIME_RETRY_ENABLED</code> |
| <code>interface.14.env-indexer-token-instance-realtime-retry-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_REALTIME_RETRY_TIMEOUT</code> |
| <code>interface.14.env-indexer-token-instance-refetch-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_REFETCH_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-token-instance-refetch-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_REFETCH_CONCURRENCY</code> |
| <code>interface.14.env-indexer-token-instance-retry-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_RETRY_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-token-instance-retry-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_RETRY_CONCURRENCY</code> |
| <code>interface.14.env-indexer-token-instance-retry-exponential-timeout-base</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_RETRY_EXPONENTIAL_TIMEOUT_BASE</code> |
| <code>interface.14.env-indexer-token-instance-retry-exponential-timeout-coeff</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_RETRY_EXPONENTIAL_TIMEOUT_COEFF</code> |
| <code>interface.14.env-indexer-token-instance-retry-max-refetch-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_RETRY_MAX_REFETCH_INTERVAL</code> |
| <code>interface.14.env-indexer-token-instance-sanitize-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_SANITIZE_BATCH_SIZE</code> |
| <code>interface.14.env-indexer-token-instance-sanitize-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_SANITIZE_CONCURRENCY</code> |
| <code>interface.14.env-indexer-token-instance-use-base-uri-retry</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_INSTANCE_USE_BASE_URI_RETRY</code> |
| <code>interface.14.env-indexer-token-transfer-block-consensus-sanitizer-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TOKEN_TRANSFER_BLOCK_CONSENSUS_SANITIZER_INTERVAL</code> |
| <code>interface.14.env-indexer-tx-actions-aave-v3-pool-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_AAVE_V3_POOL_CONTRACT</code> |
| <code>interface.14.env-indexer-tx-actions-enable</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_ENABLE</code> |
| <code>interface.14.env-indexer-tx-actions-max-token-cache-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_MAX_TOKEN_CACHE_SIZE</code> |
| <code>interface.14.env-indexer-tx-actions-reindex-first-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_REINDEX_FIRST_BLOCK</code> |
| <code>interface.14.env-indexer-tx-actions-reindex-last-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_REINDEX_LAST_BLOCK</code> |
| <code>interface.14.env-indexer-tx-actions-reindex-protocols</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_REINDEX_PROTOCOLS</code> |
| <code>interface.14.env-indexer-tx-actions-uniswap-v3-factory-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_UNISWAP_V3_FACTORY_CONTRACT</code> |
| <code>interface.14.env-indexer-tx-actions-uniswap-v3-nft-position-manager-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_TX_ACTIONS_UNISWAP_V3_NFT_POSITION_MANAGER_CONTRACT</code> |
| <code>interface.14.env-indexer-zksync-batches-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ZKSYNC_BATCHES_CHUNK_SIZE</code> |
| <code>interface.14.env-indexer-zksync-batches-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ZKSYNC_BATCHES_ENABLED</code> |
| <code>interface.14.env-indexer-zksync-batches-status-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ZKSYNC_BATCHES_STATUS_RECHECK_INTERVAL</code> |
| <code>interface.14.env-indexer-zksync-l1-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ZKSYNC_L1_RPC</code> |
| <code>interface.14.env-indexer-zksync-new-batches-max-range</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ZKSYNC_NEW_BATCHES_MAX_RANGE</code> |
| <code>interface.14.env-indexer-zksync-new-batches-recheck-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>INDEXER_ZKSYNC_NEW_BATCHES_RECHECK_INTERVAL</code> |

## env-ipc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-ipc-path</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>IPC_PATH</code> |

## env-ipfs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-ipfs-gateway-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>gateway_url: ConfigHelper.parse_url_env_var("IPFS_GATEWAY_URL", "&lt;url&gt;"),</code> |
| <code>interface.14.env-ipfs-gateway-url-param-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>IPFS_GATEWAY_URL_PARAM_KEY</code> |
| <code>interface.14.env-ipfs-gateway-url-param-location</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>IPFS_GATEWAY_URL_PARAM_LOCATION</code> |
| <code>interface.14.env-ipfs-gateway-url-param-value</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>IPFS_GATEWAY_URL_PARAM_VALUE</code> |
| <code>interface.14.env-ipfs-public-gateway-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>public_gateway_url: ConfigHelper.parse_url_env_var("IPFS_PUBLIC_GATEWAY_URL", "&lt;url&gt;")</code> |

## env-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-json-rpc</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>JSON_RPC</code> |

## env-k8s

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-k8s-service</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>K8S_SERVICE</code> |

## env-keyfile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-keyfile</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>keyfile: System.get_env("KEYFILE") &#124;&#124; "priv/cert/selfsigned_key.pem"</code> |

## env-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-keys-manager-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>KEYS_MANAGER_CONTRACT</code> |

## env-last

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-last-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>LAST_BLOCK</code> |

## env-logo

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-logo</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>logo: System.get_env("LOGO"),</code> |
| <code>interface.14.env-logo-text</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>LOGO_TEXT</code> |

## env-market

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-market-cap-history-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CAP_HISTORY_SOURCE</code> |
| <code>interface.14.env-market-coin-cache-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COIN_CACHE_PERIOD</code> |
| <code>interface.14.env-market-coin-fetcher-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COIN_FETCHER_ENABLED</code> |
| <code>interface.14.env-market-coingecko-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINGECKO_API_KEY</code> |
| <code>interface.14.env-market-coingecko-base-pro-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINGECKO_BASE_PRO_URL</code> |
| <code>interface.14.env-market-coingecko-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINGECKO_BASE_URL</code> |
| <code>interface.14.env-market-coingecko-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINGECKO_COIN_ID</code> |
| <code>interface.14.env-market-coingecko-platform-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINGECKO_PLATFORM_ID</code> |
| <code>interface.14.env-market-coingecko-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINGECKO_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-market-coinmarketcap-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINMARKETCAP_API_KEY</code> |
| <code>interface.14.env-market-coinmarketcap-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINMARKETCAP_BASE_URL</code> |
| <code>interface.14.env-market-coinmarketcap-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINMARKETCAP_COIN_ID</code> |
| <code>interface.14.env-market-coinmarketcap-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_COINMARKETCAP_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-market-cryptocompare-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTOCOMPARE_BASE_URL</code> |
| <code>interface.14.env-market-cryptocompare-coin-symbol</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTOCOMPARE_COIN_SYMBOL</code> |
| <code>interface.14.env-market-cryptocompare-secondary-coin-symbol</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTOCOMPARE_SECONDARY_COIN_SYMBOL</code> |
| <code>interface.14.env-market-cryptorank-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTORANK_API_KEY</code> |
| <code>interface.14.env-market-cryptorank-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTORANK_BASE_URL</code> |
| <code>interface.14.env-market-cryptorank-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTORANK_COIN_ID</code> |
| <code>interface.14.env-market-cryptorank-platform-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTORANK_PLATFORM_ID</code> |
| <code>interface.14.env-market-cryptorank-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_CRYPTORANK_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-market-defillama-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_DEFILLAMA_COIN_ID</code> |
| <code>interface.14.env-market-dia-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>base_url: ConfigHelper.parse_url_env_var("MARKET_DIA_BASE_URL", "&lt;url&gt;"),</code> |
| <code>interface.14.env-market-dia-blockchain</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_DIA_BLOCKCHAIN</code> |
| <code>interface.14.env-market-dia-coin-address-hash</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_DIA_COIN_ADDRESS_HASH</code> |
| <code>interface.14.env-market-dia-secondary-coin-address-hash</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_DIA_SECONDARY_COIN_ADDRESS_HASH</code> |
| <code>interface.14.env-market-history-fetch-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_HISTORY_FETCH_INTERVAL</code> |
| <code>interface.14.env-market-history-fetcher-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_HISTORY_FETCHER_ENABLED</code> |
| <code>interface.14.env-market-history-first-fetch-day-count</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_HISTORY_FIRST_FETCH_DAY_COUNT</code> |
| <code>interface.14.env-market-mobula-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_MOBULA_API_KEY</code> |
| <code>interface.14.env-market-mobula-base-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_MOBULA_BASE_URL</code> |
| <code>interface.14.env-market-mobula-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_MOBULA_COIN_ID</code> |
| <code>interface.14.env-market-mobula-platform-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_MOBULA_PLATFORM_ID</code> |
| <code>interface.14.env-market-mobula-secondary-coin-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_MOBULA_SECONDARY_COIN_ID</code> |
| <code>interface.14.env-market-native-coin-history-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_NATIVE_COIN_HISTORY_SOURCE</code> |
| <code>interface.14.env-market-native-coin-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_NATIVE_COIN_SOURCE</code> |
| <code>interface.14.env-market-secondary-coin-history-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_SECONDARY_COIN_HISTORY_SOURCE</code> |
| <code>interface.14.env-market-secondary-coin-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_SECONDARY_COIN_SOURCE</code> |
| <code>interface.14.env-market-tokens-fetcher-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_TOKENS_FETCHER_ENABLED</code> |
| <code>interface.14.env-market-tokens-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_TOKENS_INTERVAL</code> |
| <code>interface.14.env-market-tokens-max-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_TOKENS_MAX_BATCH_SIZE</code> |
| <code>interface.14.env-market-tokens-refetch-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_TOKENS_REFETCH_INTERVAL</code> |
| <code>interface.14.env-market-tokens-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_TOKENS_SOURCE</code> |
| <code>interface.14.env-market-tvl-history-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MARKET_TVL_HISTORY_SOURCE</code> |

## env-metadata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-metadata-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>METADATA_CONTRACT</code> |

## env-microservice

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-microservice-account-abstraction-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_ACCOUNT_ABSTRACTION_ENABLED</code> |
| <code>interface.14.env-microservice-account-abstraction-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_ACCOUNT_ABSTRACTION_URL</code> |
| <code>interface.14.env-microservice-bens-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_BENS_ENABLED</code> |
| <code>interface.14.env-microservice-bens-protocols</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_BENS_PROTOCOLS</code> |
| <code>interface.14.env-microservice-bens-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_BENS_URL</code> |
| <code>interface.14.env-microservice-eth-bytecode-db-interval-between-lookups</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_ETH_BYTECODE_DB_INTERVAL_BETWEEN_LOOKUPS</code> |
| <code>interface.14.env-microservice-eth-bytecode-db-max-lookups-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_ETH_BYTECODE_DB_MAX_LOOKUPS_CONCURRENCY</code> |
| <code>interface.14.env-microservice-metadata-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_METADATA_ENABLED</code> |
| <code>interface.14.env-microservice-metadata-proxy-requests-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_METADATA_PROXY_REQUESTS_TIMEOUT</code> |
| <code>interface.14.env-microservice-metadata-requests-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_METADATA_REQUESTS_TIMEOUT</code> |
| <code>interface.14.env-microservice-metadata-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_METADATA_URL</code> |
| <code>interface.14.env-microservice-multichain-search-addresses-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_MULTICHAIN_SEARCH_ADDRESSES_CHUNK_SIZE</code> |
| <code>interface.14.env-microservice-multichain-search-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_MULTICHAIN_SEARCH_API_KEY</code> |
| <code>interface.14.env-microservice-multichain-search-counters-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_MULTICHAIN_SEARCH_COUNTERS_CHUNK_SIZE</code> |
| <code>interface.14.env-microservice-multichain-search-token-info-chunk-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_MULTICHAIN_SEARCH_TOKEN_INFO_CHUNK_SIZE</code> |
| <code>interface.14.env-microservice-multichain-search-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_MULTICHAIN_SEARCH_URL</code> |
| <code>interface.14.env-microservice-sc-verifier-api-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_SC_VERIFIER_API_KEY</code> |
| <code>interface.14.env-microservice-sc-verifier-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_SC_VERIFIER_ENABLED</code> |
| <code>interface.14.env-microservice-sc-verifier-type</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_SC_VERIFIER_TYPE</code> |
| <code>interface.14.env-microservice-sc-verifier-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_SC_VERIFIER_URL</code> |
| <code>interface.14.env-microservice-sig-provider-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_SIG_PROVIDER_ENABLED</code> |
| <code>interface.14.env-microservice-sig-provider-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_SIG_PROVIDER_URL</code> |
| <code>interface.14.env-microservice-stylus-verifier-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_STYLUS_VERIFIER_URL</code> |
| <code>interface.14.env-microservice-tac-operation-lifecycle-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_TAC_OPERATION_LIFECYCLE_ENABLED</code> |
| <code>interface.14.env-microservice-tac-operation-lifecycle-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_TAC_OPERATION_LIFECYCLE_URL</code> |
| <code>interface.14.env-microservice-transaction-interpretation-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_TRANSACTION_INTERPRETATION_ENABLED</code> |
| <code>interface.14.env-microservice-transaction-interpretation-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_TRANSACTION_INTERPRETATION_URL</code> |
| <code>interface.14.env-microservice-visualize-sol2uml-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_VISUALIZE_SOL2UML_ENABLED</code> |
| <code>interface.14.env-microservice-visualize-sol2uml-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MICROSERVICE_VISUALIZE_SOL2UML_URL</code> |

## env-migration

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-migration-arbitrum-da-records-normalization-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_ARBITRUM_DA_RECORDS_NORMALIZATION_BATCH_SIZE</code> |
| <code>interface.14.env-migration-arbitrum-da-records-normalization-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_ARBITRUM_DA_RECORDS_NORMALIZATION_CONCURRENCY</code> |
| <code>interface.14.env-migration-backfill-metadata-url-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_BACKFILL_METADATA_URL_BATCH_SIZE</code> |
| <code>interface.14.env-migration-backfill-metadata-url-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_BACKFILL_METADATA_URL_CONCURRENCY</code> |
| <code>interface.14.env-migration-backfill-metadata-url-disabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_BACKFILL_METADATA_URL_DISABLED</code> |
| <code>interface.14.env-migration-backfill-multichain-search-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_BACKFILL_MULTICHAIN_SEARCH_BATCH_SIZE</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-check-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_CHECK_INTERVAL</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_ENABLED</code> |
| <code>interface.14.env-migration-delete-zero-value-internal-transactions-storage-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_DELETE_ZERO_VALUE_INTERNAL_TRANSACTIONS_STORAGE_PERIOD</code> |
| <code>interface.14.env-migration-empty-internal-transactions-data-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_EMPTY_INTERNAL_TRANSACTIONS_DATA_BATCH_SIZE</code> |
| <code>interface.14.env-migration-empty-internal-transactions-data-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_EMPTY_INTERNAL_TRANSACTIONS_DATA_CONCURRENCY</code> |
| <code>interface.14.env-migration-empty-internal-transactions-data-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_EMPTY_INTERNAL_TRANSACTIONS_DATA_TIMEOUT</code> |
| <code>interface.14.env-migration-filecoin-pending-address-operations-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_FILECOIN_PENDING_ADDRESS_OPERATIONS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-filecoin-pending-address-operations-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_FILECOIN_PENDING_ADDRESS_OPERATIONS_CONCURRENCY</code> |
| <code>interface.14.env-migration-heavy-index-operations-check-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_HEAVY_INDEX_OPERATIONS_CHECK_INTERVAL</code> |
| <code>interface.14.env-migration-merge-adjacent-missing-block-ranges-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_MERGE_ADJACENT_MISSING_BLOCK_RANGES_BATCH_SIZE</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-blocks-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_BLOCKS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_CONCURRENCY</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-high-verbosity</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_HIGH_VERBOSITY</code> |
| <code>interface.14.env-migration-recovery-weth-token-transfers-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RECOVERY_WETH_TOKEN_TRANSFERS_TIMEOUT</code> |
| <code>interface.14.env-migration-refetch-contract-codes-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REFETCH_CONTRACT_CODES_BATCH_SIZE</code> |
| <code>interface.14.env-migration-refetch-contract-codes-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REFETCH_CONTRACT_CODES_CONCURRENCY</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_CONCURRENCY</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_ENABLED</code> |
| <code>interface.14.env-migration-reindex-blocks-with-missing-transactions-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_BLOCKS_WITH_MISSING_TRANSACTIONS_TIMEOUT</code> |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_DUPLICATED_INTERNAL_TRANSACTIONS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_DUPLICATED_INTERNAL_TRANSACTIONS_CONCURRENCY</code> |
| <code>interface.14.env-migration-reindex-duplicated-internal-transactions-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_DUPLICATED_INTERNAL_TRANSACTIONS_TIMEOUT</code> |
| <code>interface.14.env-migration-reindex-internal-transactions-status-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_INTERNAL_TRANSACTIONS_STATUS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-reindex-internal-transactions-status-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_INTERNAL_TRANSACTIONS_STATUS_CONCURRENCY</code> |
| <code>interface.14.env-migration-reindex-internal-transactions-status-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_REINDEX_INTERNAL_TRANSACTIONS_STATUS_TIMEOUT</code> |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RESTORE_OMITTED_WETH_TOKEN_TRANSFERS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RESTORE_OMITTED_WETH_TOKEN_TRANSFERS_CONCURRENCY</code> |
| <code>interface.14.env-migration-restore-omitted-weth-token-transfers-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_RESTORE_OMITTED_WETH_TOKEN_TRANSFERS_TIMEOUT</code> |
| <code>interface.14.env-migration-sanitize-duplicated-log-index-logs-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_DUPLICATED_LOG_INDEX_LOGS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-sanitize-duplicated-log-index-logs-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_DUPLICATED_LOG_INDEX_LOGS_CONCURRENCY</code> |
| <code>interface.14.env-migration-sanitize-empty-contract-code-addresses-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_EMPTY_CONTRACT_CODE_ADDRESSES_BATCH_SIZE</code> |
| <code>interface.14.env-migration-sanitize-empty-contract-code-addresses-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_EMPTY_CONTRACT_CODE_ADDRESSES_CONCURRENCY</code> |
| <code>interface.14.env-migration-sanitize-incorrect-nft-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_INCORRECT_NFT_BATCH_SIZE</code> |
| <code>interface.14.env-migration-sanitize-incorrect-nft-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_INCORRECT_NFT_CONCURRENCY</code> |
| <code>interface.14.env-migration-sanitize-incorrect-nft-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_INCORRECT_NFT_TIMEOUT</code> |
| <code>interface.14.env-migration-sanitize-incorrect-weth-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_INCORRECT_WETH_BATCH_SIZE</code> |
| <code>interface.14.env-migration-sanitize-incorrect-weth-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_INCORRECT_WETH_CONCURRENCY</code> |
| <code>interface.14.env-migration-sanitize-incorrect-weth-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_INCORRECT_WETH_TIMEOUT</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_VERIFIED_ADDRESSES_BATCH_SIZE</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_VERIFIED_ADDRESSES_CONCURRENCY</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-disabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_VERIFIED_ADDRESSES_DISABLED</code> |
| <code>interface.14.env-migration-sanitize-verified-addresses-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SANITIZE_VERIFIED_ADDRESSES_TIMEOUT</code> |
| <code>interface.14.env-migration-shrink-internal-transactions-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SHRINK_INTERNAL_TRANSACTIONS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-shrink-internal-transactions-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SHRINK_INTERNAL_TRANSACTIONS_CONCURRENCY</code> |
| <code>interface.14.env-migration-smart-contract-language-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SMART_CONTRACT_LANGUAGE_BATCH_SIZE</code> |
| <code>interface.14.env-migration-smart-contract-language-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SMART_CONTRACT_LANGUAGE_CONCURRENCY</code> |
| <code>interface.14.env-migration-smart-contract-language-disabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_SMART_CONTRACT_LANGUAGE_DISABLED</code> |
| <code>interface.14.env-migration-token-instance-erc-1155-sanitize-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_ERC_1155_SANITIZE_BATCH_SIZE</code> |
| <code>interface.14.env-migration-token-instance-erc-1155-sanitize-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_ERC_1155_SANITIZE_CONCURRENCY</code> |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_ERC_721_SANITIZE_BATCH_SIZE</code> |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_ERC_721_SANITIZE_CONCURRENCY</code> |
| <code>interface.14.env-migration-token-instance-erc-721-sanitize-tokens-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_ERC_721_SANITIZE_TOKENS_BATCH_SIZE</code> |
| <code>interface.14.env-migration-token-instance-owner-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_OWNER_BATCH_SIZE</code> |
| <code>interface.14.env-migration-token-instance-owner-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_OWNER_CONCURRENCY</code> |
| <code>interface.14.env-migration-token-instance-owner-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_INSTANCE_OWNER_ENABLED</code> |
| <code>interface.14.env-migration-token-transfer-token-type-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_TRANSFER_TOKEN_TYPE_BATCH_SIZE</code> |
| <code>interface.14.env-migration-token-transfer-token-type-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TOKEN_TRANSFER_TOKEN_TYPE_CONCURRENCY</code> |
| <code>interface.14.env-migration-transactions-table-denormalization-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TRANSACTIONS_TABLE_DENORMALIZATION_BATCH_SIZE</code> |
| <code>interface.14.env-migration-transactions-table-denormalization-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIGRATION_TRANSACTIONS_TABLE_DENORMALIZATION_CONCURRENCY</code> |

## env-min

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-min-missing-block-number-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MIN_MISSING_BLOCK_NUMBER_BATCH_SIZE</code> |

## env-missing

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-missing-balance-of-tokens-window-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MISSING_BALANCE_OF_TOKENS_WINDOW_SIZE</code> |

## env-mixpanel

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-mixpanel-url</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>mixpanel_url: ConfigHelper.parse_url_env_var("MIXPANEL_URL", "&lt;url&gt;"),</code> |

## env-mud

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-mud-database-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>MUD_DATABASE_URL</code> |
| <code>interface.14.env-mud-indexer-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>MUD_INDEXER_ENABLED</code> |
| <code>interface.14.env-mud-pool-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>MUD_POOL_SIZE</code> |

## env-network

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-network</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>network: System.get_env("NETWORK"),</code> |
| <code>interface.14.env-network-icon</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NETWORK_ICON</code> |
| <code>interface.14.env-network-path</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NETWORK_PATH</code> |

## env-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-new-tags</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NEW_TAGS</code> |

## env-nft

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-nft-media-handler-aws-access-key-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_AWS_ACCESS_KEY_ID</code> |
| <code>interface.14.env-nft-media-handler-aws-bucket-host</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_AWS_BUCKET_HOST</code> |
| <code>interface.14.env-nft-media-handler-aws-bucket-name</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_AWS_BUCKET_NAME</code> |
| <code>interface.14.env-nft-media-handler-aws-public-bucket-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_AWS_PUBLIC_BUCKET_URL</code> |
| <code>interface.14.env-nft-media-handler-aws-secret-access-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_AWS_SECRET_ACCESS_KEY</code> |
| <code>interface.14.env-nft-media-handler-backfill-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_BACKFILL_ENABLED</code> |
| <code>interface.14.env-nft-media-handler-backfill-enqueue-busy-waiting-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_BACKFILL_ENQUEUE_BUSY_WAITING_TIMEOUT</code> |
| <code>interface.14.env-nft-media-handler-backfill-queue-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_BACKFILL_QUEUE_SIZE</code> |
| <code>interface.14.env-nft-media-handler-bucket-folder</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_BUCKET_FOLDER</code> |
| <code>interface.14.env-nft-media-handler-cache-uniqueness-max-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_CACHE_UNIQUENESS_MAX_SIZE</code> |
| <code>interface.14.env-nft-media-handler-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_ENABLED</code> |
| <code>interface.14.env-nft-media-handler-is-worker</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_IS_WORKER</code> |
| <code>interface.14.env-nft-media-handler-remote-dispatcher-node-mode-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_REMOTE_DISPATCHER_NODE_MODE_ENABLED</code> |
| <code>interface.14.env-nft-media-handler-worker-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_WORKER_BATCH_SIZE</code> |
| <code>interface.14.env-nft-media-handler-worker-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_WORKER_CONCURRENCY</code> |
| <code>interface.14.env-nft-media-handler-worker-spawn-tasks-timeout</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NFT_MEDIA_HANDLER_WORKER_SPAWN_TASKS_TIMEOUT</code> |

## env-noves

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-noves-fi-api-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NOVES_FI_API_TOKEN</code> |
| <code>interface.14.env-noves-fi-base-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>service_url: ConfigHelper.parse_url_env_var("NOVES_FI_BASE_API_URL", "&lt;url&gt;"),</code> |
| <code>interface.14.env-noves-fi-chain-name</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>NOVES_FI_CHAIN_NAME</code> |

## env-paxeer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-paxeer-x-capabilities-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_CAPABILITIES_ENABLED</code> |
| <code>interface.14.env-paxeer-x-capabilities-refresh-seconds</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_CAPABILITIES_REFRESH_SECONDS</code> |
| <code>interface.14.env-paxeer-x-cometbft-rpc-urls</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_COMETBFT_RPC_URLS</code> |
| <code>interface.14.env-paxeer-x-cosmos-rest-urls</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_COSMOS_REST_URLS</code> |
| <code>interface.14.env-paxeer-x-cosmos-transaction-type</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_COSMOS_TRANSACTION_TYPE</code> |
| <code>interface.14.env-paxeer-x-native-coin-decimals</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_NATIVE_COIN_DECIMALS</code> |
| <code>interface.14.env-paxeer-x-price-feed-concurrency</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_PRICE_FEED_CONCURRENCY</code> |
| <code>interface.14.env-paxeer-x-price-feed-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_PRICE_FEED_ENABLED</code> |
| <code>interface.14.env-paxeer-x-price-feed-markets-refresh-seconds</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_PRICE_FEED_MARKETS_REFRESH_SECONDS</code> |
| <code>interface.14.env-paxeer-x-price-feed-precompile</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_PRICE_FEED_PRECOMPILE</code> |
| <code>interface.14.env-paxeer-x-price-feed-retention-days</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_PRICE_FEED_RETENTION_DAYS</code> |
| <code>interface.14.env-paxeer-x-price-feed-start-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PAXEER_X_PRICE_FEED_START_BLOCK</code> |

## env-permanent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-permanent-dark-mode-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PERMANENT_DARK_MODE_ENABLED</code> |
| <code>interface.14.env-permanent-light-mode-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PERMANENT_LIGHT_MODE_ENABLED</code> |

## env-pool

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-pool-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>POOL_SIZE</code> |
| <code>interface.14.env-pool-size-api</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>POOL_SIZE_API</code> |

## env-port

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-port</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex) | <code>port = System.get_env("PORT") &#124;&#124; raise "expected the PORT environment variable to be set"</code> |

## env-precompiled

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-precompiled-contracts-config-path</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PRECOMPILED_CONTRACTS_CONFIG_PATH</code> |

## env-public

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-public-metrics-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PUBLIC_METRICS_ENABLED</code> |
| <code>interface.14.env-public-metrics-update-period-hours</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>PUBLIC_METRICS_UPDATE_PERIOD_HOURS</code> |

## env-rate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-rate-limiter-on-demand-exponential-timeout-coeff</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RATE_LIMITER_ON_DEMAND_EXPONENTIAL_TIMEOUT_COEFF</code> |
| <code>interface.14.env-rate-limiter-on-demand-limit-by-ip</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RATE_LIMITER_ON_DEMAND_LIMIT_BY_IP</code> |
| <code>interface.14.env-rate-limiter-on-demand-limitation-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RATE_LIMITER_ON_DEMAND_LIMITATION_PERIOD</code> |
| <code>interface.14.env-rate-limiter-on-demand-max-ban-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RATE_LIMITER_ON_DEMAND_MAX_BAN_INTERVAL</code> |
| <code>interface.14.env-rate-limiter-on-demand-time-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RATE_LIMITER_ON_DEMAND_TIME_INTERVAL</code> |
| <code>interface.14.env-rate-limiter-redis-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RATE_LIMITER_REDIS_URL</code> |

## env-re

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-re-captcha-bypass-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_BYPASS_TOKEN</code> |
| <code>interface.14.env-re-captcha-check-hostname</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_CHECK_HOSTNAME</code> |
| <code>interface.14.env-re-captcha-client-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_CLIENT_KEY</code> |
| <code>interface.14.env-re-captcha-disabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_DISABLED</code> |
| <code>interface.14.env-re-captcha-score-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_SCORE_THRESHOLD</code> |
| <code>interface.14.env-re-captcha-secret-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_SECRET_KEY</code> |
| <code>interface.14.env-re-captcha-token-instance-refetch-metadata-scoped-bypass-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_TOKEN_INSTANCE_REFETCH_METADATA_SCOPED_BYPASS_TOKEN</code> |
| <code>interface.14.env-re-captcha-v3-client-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_V3_CLIENT_KEY</code> |
| <code>interface.14.env-re-captcha-v3-secret-key</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RE_CAPTCHA_V3_SECRET_KEY</code> |

## env-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-release-link</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>RELEASE_LINK</code> |

## env-replica

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-replica-max-lag</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>REPLICA_MAX_LAG</code> |

## env-rewards

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-rewards-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>REWARDS_CONTRACT</code> |

## env-rootstock

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-rootstock-bridge-address</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ROOTSTOCK_BRIDGE_ADDRESS</code> |
| <code>interface.14.env-rootstock-locked-btc-cache-period</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ROOTSTOCK_LOCKED_BTC_CACHE_PERIOD</code> |
| <code>interface.14.env-rootstock-locking-cap</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ROOTSTOCK_LOCKING_CAP</code> |
| <code>interface.14.env-rootstock-remasc-address</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>ROOTSTOCK_REMASC_ADDRESS</code> |

## env-scroll

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-scroll-l1-base-fee-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L1_BASE_FEE_INIT</code> |
| <code>interface.14.env-scroll-l1-blob-base-fee-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L1_BLOB_BASE_FEE_INIT</code> |
| <code>interface.14.env-scroll-l1-blob-scalar-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L1_BLOB_SCALAR_INIT</code> |
| <code>interface.14.env-scroll-l1-commit-scalar-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L1_COMMIT_SCALAR_INIT</code> |
| <code>interface.14.env-scroll-l1-overhead-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L1_OVERHEAD_INIT</code> |
| <code>interface.14.env-scroll-l1-scalar-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L1_SCALAR_INIT</code> |
| <code>interface.14.env-scroll-l2-curie-upgrade-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SCROLL_L2_CURIE_UPGRADE_BLOCK</code> |

## env-secret

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-secret-key-base</code> | <code>env_var</code> | [services/explorer/backend/config/runtime/dev.exs](../../../services/explorer/backend/config/runtime/dev.exs) | <code>SECRET_KEY_BASE</code> |

## env-session

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-session-cookie-domain</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/config/config.exs](../../../services/explorer/backend/apps/block_scout_web/config/config.exs) | <code>SESSION_COOKIE_DOMAIN</code> |

## env-show

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-show-address-marketcap-percentage</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_ADDRESS_MARKETCAP_PERCENTAGE</code> |
| <code>interface.14.env-show-maintenance-alert</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_MAINTENANCE_ALERT</code> |
| <code>interface.14.env-show-price-chart</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_PRICE_CHART</code> |
| <code>interface.14.env-show-price-chart-legend</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_PRICE_CHART_LEGEND</code> |
| <code>interface.14.env-show-tenderly-link</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_TENDERLY_LINK</code> |
| <code>interface.14.env-show-testnet-label</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_TESTNET_LABEL</code> |
| <code>interface.14.env-show-txs-chart</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHOW_TXS_CHART</code> |

## env-shrink

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-shrink-internal-transactions-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SHRINK_INTERNAL_TRANSACTIONS_ENABLED</code> |

## env-solidityscan

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-solidityscan-api-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SOLIDITYSCAN_API_TOKEN</code> |
| <code>interface.14.env-solidityscan-chain-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SOLIDITYSCAN_CHAIN_ID</code> |
| <code>interface.14.env-solidityscan-platform-id</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SOLIDITYSCAN_PLATFORM_ID</code> |

## env-sourcify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-sourcify-integration-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SOURCIFY_INTEGRATION_ENABLED</code> |
| <code>interface.14.env-sourcify-repo-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>repo_url: ConfigHelper.parse_url_env_var("SOURCIFY_REPO_URL", "&lt;url&gt;")</code> |
| <code>interface.14.env-sourcify-server-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>server_url: ConfigHelper.parse_url_env_var("SOURCIFY_SERVER_URL", "&lt;url&gt;"),</code> |

## env-spandex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-spandex-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SPANDEX_BATCH_SIZE</code> |
| <code>interface.14.env-spandex-sync-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SPANDEX_SYNC_THRESHOLD</code> |

## env-suave

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-suave-bid-contracts</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SUAVE_BID_CONTRACTS</code> |
| <code>interface.14.env-suave-database-url</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex) | <code>SUAVE_DATABASE_URL, DATABASE_URL</code> |

## env-subnetwork

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-subnetwork</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>subnetwork: System.get_env("SUBNETWORK"),</code> |

## env-supply

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-supply-module</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SUPPLY_MODULE</code> |

## env-supported

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-supported-chains</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>SUPPORTED_CHAINS</code> |

## env-tenderly

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-tenderly-chain-path</code> | <code>env_var</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/transaction_view.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/transaction_view.ex) | <code>TENDERLY_CHAIN_PATH</code> |

## env-test

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-test-database-read-only-api-url</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/config/test.exs](../../../services/explorer/backend/apps/explorer/config/test.exs) | <code>TEST_DATABASE_READ_ONLY_API_URL</code> |
| <code>interface.14.env-test-database-url</code> | <code>env_var</code> | [services/explorer/backend/apps/explorer/config/test.exs](../../../services/explorer/backend/apps/explorer/config/test.exs) | <code>TEST_DATABASE_URL</code> |

## env-testnet

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-testnet-label-text</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TESTNET_LABEL_TEXT</code> |

## env-tests

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-test-layerx-gateway-bin</code> | <code>env_var</code> | [tests/explorer/paxeer-x-data-boundary.py](../../../tests/explorer/paxeer-x-data-boundary.py) | <code>LAYERX_GATEWAY_BIN</code> |
| <code>interface.14.env-test-layerx-indexer-bin</code> | <code>env_var</code> | [tests/explorer/paxeer-x-data-boundary.py](../../../tests/explorer/paxeer-x-data-boundary.py) | <code>LAYERX_INDEXER_BIN</code> |
| <code>interface.14.env-test-paxeer-x-gate-secret-key-base</code> | <code>env_var</code> | [tests/explorer/paxeer-x-data-boundary.py](../../../tests/explorer/paxeer-x-data-boundary.py) | <code>PAXEER_X_GATE_SECRET_KEY_BASE</code> |

## env-token

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-token-balance-on-demand-fetcher-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_BALANCE_ON_DEMAND_FETCHER_THRESHOLD</code> |
| <code>interface.14.env-token-counters-update-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_COUNTERS_UPDATE_INTERVAL</code> |
| <code>interface.14.env-token-exchange-rate-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_EXCHANGE_RATE_INTERVAL</code> |
| <code>interface.14.env-token-exchange-rate-max-batch-size</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_EXCHANGE_RATE_MAX_BATCH_SIZE</code> |
| <code>interface.14.env-token-exchange-rate-refetch-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_EXCHANGE_RATE_REFETCH_INTERVAL</code> |
| <code>interface.14.env-token-exchange-rates-source</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_EXCHANGE_RATES_SOURCE</code> |
| <code>interface.14.env-token-instance-metadata-refetch-on-demand-fetcher-threshold</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_INSTANCE_METADATA_REFETCH_ON_DEMAND_FETCHER_THRESHOLD</code> |
| <code>interface.14.env-token-metadata-update-interval</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TOKEN_METADATA_UPDATE_INTERVAL</code> |

## env-trace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-trace-block-ranges</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TRACE_BLOCK_RANGES</code> |
| <code>interface.14.env-trace-first-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TRACE_FIRST_BLOCK</code> |
| <code>interface.14.env-trace-last-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TRACE_LAST_BLOCK</code> |

## env-txs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-txs-historian-init-lag</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TXS_HISTORIAN_INIT_LAG</code> |
| <code>interface.14.env-txs-stats-days-to-compile-at-init</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TXS_STATS_DAYS_TO_COMPILE_AT_INIT</code> |
| <code>interface.14.env-txs-stats-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>TXS_STATS_ENABLED</code> |

## env-uncles

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-uncles-in-average-block-time</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>UNCLES_IN_AVERAGE_BLOCK_TIME</code> |

## env-universal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-universal-proxy-config</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>UNIVERSAL_PROXY_CONFIG</code> |
| <code>interface.14.env-universal-proxy-config-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>UNIVERSAL_PROXY_CONFIG_URL</code> |

## env-validators

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-validators-contract</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>VALIDATORS_CONTRACT</code> |

## env-webapp

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-webapp-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>WEBAPP_URL</code> |

## env-weth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-weth-token-transfers-filtering-enabled</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>WETH_TOKEN_TRANSFERS_FILTERING_ENABLED</code> |

## env-whitelisted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-whitelisted-weth-contracts</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>WHITELISTED_WETH_CONTRACTS</code> |

## env-withdrawals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-withdrawals-first-block</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>WITHDRAWALS_FIRST_BLOCK</code> |

## env-xname

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-xname-api-token</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>XNAME_API_TOKEN</code> |
| <code>interface.14.env-xname-base-api-url</code> | <code>env_var</code> | [services/explorer/backend/config/runtime.exs](../../../services/explorer/backend/config/runtime.exs) | <code>service_url: ConfigHelper.parse_url_env_var("XNAME_BASE_API_URL", "&lt;url&gt;"),</code> |
