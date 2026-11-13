# Operations contract

Build and operational commands are defined by the repository Makefiles, scripts, deployment files and command parsers. Command flags and environment names below belong to those individual tools. Node execution, consensus and finality belong to `paxd`; a receipt is final when its block commits.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## bench-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if __name__ == "__main__":` | [tools/benchmark/loadtest/contracts/parser.py](../../../tools/benchmark/loadtest/contracts/parser.py) |
| `cli_command` | `func main() {` | [tools/benchmark/loadtest/main.go](../../../tools/benchmark/loadtest/main.go) |
| `cli_command` | `configFilePath := flag.String("config-file", GetDefaultConfigFilePath(), "Path to the config.json file to use for this run")` | [tools/benchmark/loadtest/main.go](../../../tools/benchmark/loadtest/main.go) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/benchmark/loadtest/scripts/metrics.py](../../../tools/benchmark/loadtest/scripts/metrics.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/benchmark/loadtest/scripts/populate_genesis_accounts.py](../../../tools/benchmark/loadtest/scripts/populate_genesis_accounts.py) |
| `cli_command` | `parser = argparse.ArgumentParser(` | [tools/benchmark/loadtest/scripts/run_tests.py](../../../tools/benchmark/loadtest/scripts/run_tests.py) |
| `cli_command` | `parser = argparse.ArgumentParser(` | [tools/benchmark/loadtest/scripts/validator_failures.py](../../../tools/benchmark/loadtest/scripts/validator_failures.py) |
| source | — | [tools/benchmark/benchmark.sh](../../../tools/benchmark/benchmark.sh) |
| source | — | [tools/benchmark/benchmark-compare.sh](../../../tools/benchmark/benchmark-compare.sh) |
| source | — | [tools/benchmark/loadtest/contracts/deploy.sh](../../../tools/benchmark/loadtest/contracts/deploy.sh) |
| source | — | [tools/benchmark/loadtest/contracts/deploy_erc20.sh](../../../tools/benchmark/loadtest/contracts/deploy_erc20.sh) |
| source | — | [tools/benchmark/loadtest/contracts/deploy_erc721.sh](../../../tools/benchmark/loadtest/contracts/deploy_erc721.sh) |
| source | — | [tools/benchmark/loadtest/contracts/deploy_fifty_contracts.sh](../../../tools/benchmark/loadtest/contracts/deploy_fifty_contracts.sh) |
| source | — | [tools/benchmark/loadtest/contracts/deploy_ten_contracts.sh](../../../tools/benchmark/loadtest/contracts/deploy_ten_contracts.sh) |
| source | — | [tools/benchmark/loadtest/contracts/evm/setup.sh](../../../tools/benchmark/loadtest/contracts/evm/setup.sh) |
| source | — | [tools/benchmark/loadtest/scripts/generate_bindings.sh](../../../tools/benchmark/loadtest/scripts/generate_bindings.sh) |

## bench-compare

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/benchmark/tps_stats.py](../../../tools/benchmark/tps_stats.py) |
| source | — | [tools/benchmark/benchmark-compare.sh](../../../tools/benchmark/benchmark-compare.sh) |

## bench-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | ``BlockchainEndpoint string                `json:"blockchain_endpoint"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"chain_id": "pax-loadtest-testnet",` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``ChainID            string                `json:"chain_id"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"contract_distribution": [` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``ContractDistr      ContractDistributions `json:"contract_distribution"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``EvmRpcEndpoints    string                `json:"evm_rpc_endpoints"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"evm_use_eip1559_txs": true,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``EvmUseEip1559Txs   bool                  `json:"evm_use_eip1559_txs"` // setting this to true could make gas go up to infinity`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``GrpcEndpoints      string                `json:"grpc_endpoints"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"max_accounts": 500,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``MaxAccounts        uint64                `json:"max_accounts"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"message_configs": {` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``PerMessageConfigs  MessageConfigs        `json:"message_configs"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"message_type_distribution": {` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``MsgTypeDistr       MsgTypeDistribution   `json:"message_type_distribution"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"message_types": [` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``MessageTypes       []string              `json:"message_types"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"metrics_port": 9695,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``MetricsPort        uint64                `json:"metrics_port"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"msgs_per_tx": 1,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``MsgsPerTx          uint64                `json:"msgs_per_tx"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``NodeURI            string                `json:"node_uri"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"post_tx_evm_queries": {` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``PostTxEvmQueries   PostTxEvmQueries      `json:"post_tx_evm_queries"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"price_distribution": {` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``PriceDistr         NumericDistribution   `json:"price_distribution"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"quantity_distribution": {` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``QuantityDistr      NumericDistribution   `json:"quantity_distribution"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"run_oracle": false,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | `"target_tps": 500,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``TargetTps          uint64                `json:"target_tps"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"ticks": 0` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``Ticks              uint64                `json:"ticks"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"tls": false,` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``TLS                bool                  `json:"tls"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `"wasm_msg_types": {` | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| `config_key` | ``WasmMsgTypes       WasmMessageTypes      `json:"wasm_msg_types"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``ContractAddr string  `json:"contract_address"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Percentage   sdk.Dec `json:"percentage"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``LimitOrderPct  sdk.Dec `json:"limit_order_percentage"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``MarketOrderPct sdk.Dec `json:"market_order_percentage"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | `[profile.default]` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `auto_detect_solc = false` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `fuzz_runs = 1_000` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `libs = ["lib"]` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `optimizer = true` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `optimizer_runs = 1_000` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `out = "out"` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `remappings = [` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `solc_version = '0.8.20'` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | `src = "src"` | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| `config_key` | ``Fee uint64 `json:"fee"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Gas uint64 `json:"gas"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Dex     DexMsgTypeDistribution     `json:"dex"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Staking StakingMsgTypeDistribution `json:"staking"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Max         sdk.Dec `json:"max"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Min         sdk.Dec `json:"min"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``NumDistinct int64   `json:"number_of_distinct_values"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``BlockByNumber int `json:"block_by_number"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Filters       int `json:"filters"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Receipt       int `json:"receipt"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``BeginRedelegatePct sdk.Dec `json:"begin_redelegate_percentage"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``DelegatePct        sdk.Dec `json:"delegate_percentage"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``UndelegatePct      sdk.Dec `json:"undelegate_percentage"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``ContractAddr   string `json:"contract_address"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``NumOrdersPerTx int64  `json:"num_orders_per_tx"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``CodeID  uint64 `json:"code_id"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Payload string `json:"payload"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Instantiate WasmInstantiateType `json:"instantiate"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``Vortex      VortexContract      `json:"vortex"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``MintNftType WasmMintNftType     `json:"wasm_mint_nft"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| `config_key` | ``ContractAddr string `json:"contract_address"`` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |

## bench-contracts

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (c Config) ValidateMessageTypes() error` | [tools/benchmark/loadtest/types.go](../../../tools/benchmark/loadtest/types.go) |
| source | — | [tools/benchmark/loadtest/contracts/deploy_erc20.sh](../../../tools/benchmark/loadtest/contracts/deploy_erc20.sh) |
| source | — | [tools/benchmark/loadtest/contracts/evm/src/ERC721.sol](../../../tools/benchmark/loadtest/contracts/evm/src/ERC721.sol) |
| source | — | [tools/benchmark/loadtest/contracts/evm/src/NoopToken.sol](../../../tools/benchmark/loadtest/contracts/evm/src/NoopToken.sol) |

## bench-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PAX_HOME` | [tools/benchmark/loadtest/scripts/populate_genesis_accounts.py](../../../tools/benchmark/loadtest/scripts/populate_genesis_accounts.py) |
| `env_var` | `PAXD_BIN` | [tools/benchmark/loadtest/scripts/populate_genesis_accounts.py](../../../tools/benchmark/loadtest/scripts/populate_genesis_accounts.py) |
| source | — | [tools/benchmark/benchmark.sh](../../../tools/benchmark/benchmark.sh) |
| source | — | [tools/benchmark/benchmark-compare.sh](../../../tools/benchmark/benchmark-compare.sh) |

## bench-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/benchmark/loadtest/config.json](../../../tools/benchmark/loadtest/config.json) |
| source | — | [tools/benchmark/loadtest/contracts/evm/bindings/erc20/abi.json](../../../tools/benchmark/loadtest/contracts/evm/bindings/erc20/abi.json) |
| source | — | [tools/benchmark/loadtest/contracts/evm/bindings/erc721/abi.json](../../../tools/benchmark/loadtest/contracts/evm/bindings/erc721/abi.json) |
| source | — | [tools/benchmark/loadtest/contracts/evm/foundry.toml](../../../tools/benchmark/loadtest/contracts/evm/foundry.toml) |
| source | — | [tools/benchmark/loadtest/contracts/evm/remappings.txt](../../../tools/benchmark/loadtest/contracts/evm/remappings.txt) |
| source | — | [tools/benchmark/loadtest/contracts/jupiter/Cargo.toml](../../../tools/benchmark/loadtest/contracts/jupiter/Cargo.toml) |
| source | — | [tools/benchmark/loadtest/contracts/mars/Cargo.toml](../../../tools/benchmark/loadtest/contracts/mars/Cargo.toml) |
| source | — | [tools/benchmark/loadtest/contracts/saturn/Cargo.toml](../../../tools/benchmark/loadtest/contracts/saturn/Cargo.toml) |
| source | — | [tools/benchmark/loadtest/contracts/venus/Cargo.toml](../../../tools/benchmark/loadtest/contracts/venus/Cargo.toml) |
| source | — | [tools/benchmark/scenarios/default.json](../../../tools/benchmark/scenarios/default.json) |
| source | — | [tools/benchmark/scenarios/erc20.json](../../../tools/benchmark/scenarios/erc20.json) |
| source | — | [tools/benchmark/scenarios/evm.json](../../../tools/benchmark/scenarios/evm.json) |
| source | — | [tools/benchmark/scenarios/mixed.json](../../../tools/benchmark/scenarios/mixed.json) |

## bench-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `metric.WithDescription("Number of transactions produced by message type"),` | [tools/benchmark/loadtest/metrics.go](../../../tools/benchmark/loadtest/metrics.go) |

## bench-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `http.HandleFunc("/healthz", s.healthzHandler)` | [tools/benchmark/loadtest/metrics.go](../../../tools/benchmark/loadtest/metrics.go) |
| `http_route` | `http.HandleFunc("/metrics", s.metricsHandler)` | [tools/benchmark/loadtest/metrics.go](../../../tools/benchmark/loadtest/metrics.go) |

<a id="bench-scenarios"></a>

## bringup-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `usage: tools/bringup/ca.sh init \| inventory [<service>] \| issue <service> \| services` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `init \| services \| local-services)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `install-server)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `inventory)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `issue)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `issue-local)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `request-server)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `sign-server)` | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| `cli_command` | `usage: tools/bringup/check-live.sh hosts\|rpc-nodes\|archive-node\|ca\|hpx\|explorer` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `agent-public) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `hosts \| rpc-nodes \| archive-node \| ca \| hpx \| explorer) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `bridge) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `ci) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `developers) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `edge) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `events-upstream) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `gas) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `human) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `human-session) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `identity) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `indexer) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `internal) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `interop) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `interop-adapters) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `kernel-app) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `kernel-boundaries) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `kernel-node) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `kernel-value-loop) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `mirrors) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `paxeer-boundary) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `ramp) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `registry) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `registry-bootstrap) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `registry-plan) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `relay) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `router) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `rpc-placement) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `search) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `search-front) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `validators) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `wallet) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `xweb-attestors) ;;` | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| `cli_command` | `usage: tools/bringup/edge.sh add <name>[:<port>] <railway\|box\|stream> <host:port> <sni\|-> \| register <name> <host>[:<port>]` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `-h:* \| --help:*)` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `add) edge_add "$@" ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `list) [ ! -r "$manifest" ] \|\| cat "$manifest" ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `register)` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `remove) edge_remove "$1" ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `render) cmd_render "$1" ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `rpc) render_rpc "$name" tls ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `rpc-health) cmd_rpc_health ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `set) cmd_set "$@" ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `stream) render_stream "$port" ;;` | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| `cli_command` | `usage: tools/bringup/edge-sync.sh [--dry-run] <env-file>` | [tools/bringup/edge-sync.sh](../../../tools/bringup/edge-sync.sh) |
| `cli_command` | `--dry-run)` | [tools/bringup/edge-sync.sh](../../../tools/bringup/edge-sync.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/edge-sync.sh](../../../tools/bringup/edge-sync.sh) |
| `cli_command` | `dns-auth \| dns-cleanup)` | [tools/bringup/edge-sync.sh](../../../tools/bringup/edge-sync.sh) |
| `cli_command` | `usage: tools/bringup/human-state-preserve.sh export <dir> \| import <dir> \| verify <dir>` | [tools/bringup/human-state-preserve.sh](../../../tools/bringup/human-state-preserve.sh) |
| `cli_command` | `usage: tools/bringup/install-archive.sh <public-rpc-name> <acme-email>` | [tools/bringup/install-archive.sh](../../../tools/bringup/install-archive.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/install-archive.sh](../../../tools/bringup/install-archive.sh) |
| `cli_command` | `render) render_step "${@:2}" ;;` | [tools/bringup/kernel-genesis.sh](../../../tools/bringup/kernel-genesis.sh) |
| `cli_command` | `parser = argparse.ArgumentParser(description='Source-bound registry artifacts and owned isolation lifecycle; no registry readiness claim.')` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `cli_command` | `command.add_argument('--directory', required=True)` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `cli_command` | `command.add_argument('--manifest', required=True)` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `cli_command` | `build_parser.add_argument('--output', required=True)` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `cli_command` | `build_parser.add_argument('--repo', required=True)` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `cli_command` | `build_parser = sub.add_parser('build')` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `cli_command` | `-h \| --help)` | [tools/bringup/search-front.sh](../../../tools/bringup/search-front.sh) |
| `cli_command` | `deploy)` | [tools/bringup/search-front.sh](../../../tools/bringup/search-front.sh) |
| `cli_command` | `usage: tools/bringup/sync-fleet.sh [--dry-run] config\|restart\|resync <paxd>\|rollout <paxd> <sha256>\|release <version>` | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| `cli_command` | `config \| restart)` | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| `cli_command` | `release)` | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| `cli_command` | `resync)` | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| `cli_command` | `rollout)` | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| `cli_command` | `usage: tools/bringup/upgrade-fleet.sh [--stage-only\|--watch-only] <height>` | [tools/bringup/upgrade-fleet.sh](../../../tools/bringup/upgrade-fleet.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/upgrade-fleet.sh](../../../tools/bringup/upgrade-fleet.sh) |
| `cli_command` | `--stage-only)` | [tools/bringup/upgrade-fleet.sh](../../../tools/bringup/upgrade-fleet.sh) |
| `cli_command` | `--watch-only)` | [tools/bringup/upgrade-fleet.sh](../../../tools/bringup/upgrade-fleet.sh) |
| `cli_command` | `usage: tools/bringup/xweb-config.sh prices \| render serve\|attestor <directory> \| inventory \| register` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| `cli_command` | `-h \| --help)` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| `cli_command` | `inventory)` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| `cli_command` | `prices)` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| `cli_command` | `register)` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| `cli_command` | `render)` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| source | — | [tools/bringup/build-foundation-artifacts.sh](../../../tools/bringup/build-foundation-artifacts.sh) |
| source | — | [tools/bringup/docs-names.sh](../../../tools/bringup/docs-names.sh) |
| source | — | [tools/bringup/mint-secrets.sh](../../../tools/bringup/mint-secrets.sh) |
| source | — | [tools/bringup/validator-firewall.sh](../../../tools/bringup/validator-firewall.sh) |
| source | — | [tools/bringup/value-loop.sh](../../../tools/bringup/value-loop.sh) |

## bringup-config

The certificate-name bindings select subject alternative names for each service or client role. Their values depend on the deployment.

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `AGENTD_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `DASHBOARD_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `DEVELOPER` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `DEVELOPER_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `GATEWAY_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `HUMAN_ATTESTOR_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `HUMAN_EVENT_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `HUMAN_KMS_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `HUMAN_KMS_EXECUTOR=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `IDENTITY` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INDEXER` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTERNAL_APPROVALS` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTERNAL_JOURNEYS` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTERNAL_KMS` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTERNAL_PAYMENTS` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTERNAL_PROGRAMS` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTERNAL_REDIS` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `INTEROP_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `RAMP_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |
| `config_key` | `REGISTRY_EVENT_CLIENT=` | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |

## bringup-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PATH, LANG` | [tools/bringup/registry-artifacts.py](../../../tools/bringup/registry-artifacts.py) |
| `env_var` | `XWEB_TIMEOUT` | [tools/bringup/xweb-config.sh](../../../tools/bringup/xweb-config.sh) |
| source | — | [tools/bringup/ca.sh](../../../tools/bringup/ca.sh) |
| source | — | [tools/bringup/check-live.sh](../../../tools/bringup/check-live.sh) |
| source | — | [tools/bringup/edge.sh](../../../tools/bringup/edge.sh) |
| source | — | [tools/bringup/edge-sync.sh](../../../tools/bringup/edge-sync.sh) |
| source | — | [tools/bringup/human-state-preserve.sh](../../../tools/bringup/human-state-preserve.sh) |
| source | — | [tools/bringup/install-archive.sh](../../../tools/bringup/install-archive.sh) |
| source | — | [tools/bringup/kernel-genesis.sh](../../../tools/bringup/kernel-genesis.sh) |
| source | — | [tools/bringup/mint-secrets.sh](../../../tools/bringup/mint-secrets.sh) |
| source | — | [tools/bringup/search-front.sh](../../../tools/bringup/search-front.sh) |
| source | — | [tools/bringup/sync-fleet.sh](../../../tools/bringup/sync-fleet.sh) |
| source | — | [tools/bringup/upgrade-fleet.sh](../../../tools/bringup/upgrade-fleet.sh) |
| source | — | [tools/bringup/validator-firewall.sh](../../../tools/bringup/validator-firewall.sh) |
| source | — | [tools/bringup/value-loop.sh](../../../tools/bringup/value-loop.sh) |

## bringup-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/bringup/archive-comet-location.conf](../../../tools/bringup/archive-comet-location.conf) |
| source | — | [tools/bringup/archive-comet-zone.conf](../../../tools/bringup/archive-comet-zone.conf) |
| source | — | [tools/bringup/edge-apps.example](../../../tools/bringup/edge-apps.example) |
| source | — | [tools/bringup/railway-names.env](../../../tools/bringup/railway-names.env) |

## ci-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `- any-glob-to-any-file:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:agent:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:contracts:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:docs:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:human:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:paxeer:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:platform:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:programs:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:protocol:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `area:spec:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| `config_key` | `- changed-files:` | [.github/labeler.yml](../../../.github/labeler.yml) |
| source | — | [.github/dependabot.yml](../../../.github/dependabot.yml) |

## ci-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [.github/workflows/agent.yml](../../../.github/workflows/agent.yml) |
| source | — | [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) |
| source | — | [.github/workflows/explorer-build.yml](../../../.github/workflows/explorer-build.yml) |
| source | — | [.github/workflows/explorer-images.yml](../../../.github/workflows/explorer-images.yml) |
| source | — | [.github/workflows/human.yml](../../../.github/workflows/human.yml) |
| source | — | [.github/workflows/interop-migration-testnets.yml](../../../.github/workflows/interop-migration-testnets.yml) |
| source | — | [.github/workflows/interop-x402.yml](../../../.github/workflows/interop-x402.yml) |
| source | — | [.github/workflows/jev-pr-advisory.yml](../../../.github/workflows/jev-pr-advisory.yml) |
| source | — | [.github/workflows/mirror-live.yml](../../../.github/workflows/mirror-live.yml) |
| source | — | [.github/workflows/mirror-verification-live.yml](../../../.github/workflows/mirror-verification-live.yml) |
| source | — | [.github/workflows/node-build.yml](../../../.github/workflows/node-build.yml) |
| source | — | [.github/workflows/offchain-images.yml](../../../.github/workflows/offchain-images.yml) |
| source | — | [.github/workflows/paxeer-cross-arch-build.yml](../../../.github/workflows/paxeer-cross-arch-build.yml) |
| source | — | [.github/workflows/paxeer-dapp-tests.yml](../../../.github/workflows/paxeer-dapp-tests.yml) |
| source | — | [.github/workflows/paxeer-db-tests.yml](../../../.github/workflows/paxeer-db-tests.yml) |
| source | — | [.github/workflows/paxeer-docker-build.yml](../../../.github/workflows/paxeer-docker-build.yml) |
| source | — | [.github/workflows/paxeer-docker-publish.yml](../../../.github/workflows/paxeer-docker-publish.yml) |
| source | — | [.github/workflows/paxeer-eth-blocktests.yml](../../../.github/workflows/paxeer-eth-blocktests.yml) |
| source | — | [.github/workflows/paxeer-forge-test.yml](../../../.github/workflows/paxeer-forge-test.yml) |
| source | — | [.github/workflows/paxeer-ghcr-integration-test-cleanup.yml](../../../.github/workflows/paxeer-ghcr-integration-test-cleanup.yml) |
| source | — | [.github/workflows/paxeer-go-test.yml](../../../.github/workflows/paxeer-go-test.yml) |
| source | — | [.github/workflows/paxeer-hpx-registry.yml](../../../.github/workflows/paxeer-hpx-registry.yml) |
| source | — | [.github/workflows/paxeer-integration-test.yml](../../../.github/workflows/paxeer-integration-test.yml) |
| source | — | [.github/workflows/paxeer-libwasmvm.yml](../../../.github/workflows/paxeer-libwasmvm.yml) |
| source | — | [.github/workflows/paxeer-pr-to-slack-codex.yml](../../../.github/workflows/paxeer-pr-to-slack-codex.yml) |
| source | — | [.github/workflows/paxeer-proto-registry.yml](../../../.github/workflows/paxeer-proto-registry.yml) |
| source | — | [.github/workflows/paxeer-release-check.yml](../../../.github/workflows/paxeer-release-check.yml) |
| source | — | [.github/workflows/paxeer-release-publish.yml](../../../.github/workflows/paxeer-release-publish.yml) |
| source | — | [.github/workflows/paxeer-x-qualification.yml](../../../.github/workflows/paxeer-x-qualification.yml) |
| source | — | [.github/workflows/platform.yml](../../../.github/workflows/platform.yml) |
| source | — | [.github/workflows/programs-conformance.yml](../../../.github/workflows/programs-conformance.yml) |
| source | — | [.github/workflows/publish-images.yml](../../../.github/workflows/publish-images.yml) |
| source | — | [.github/workflows/ramp-sandbox.yml](../../../.github/workflows/ramp-sandbox.yml) |
| source | — | [.github/workflows/paxeer-ecr.yml](../../../.github/workflows/paxeer-ecr.yml) |
| source | — | [.github/workflows/xweb-test.yml](../../../.github/workflows/xweb-test.yml) |

## ci-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [.github/actionlint.yaml](../../../.github/actionlint.yaml) |
| source | — | [.github/cliff.toml](../../../.github/cliff.toml) |
| source | — | [.github/codecov.yml](../../../.github/codecov.yml) |
| source | — | [.github/CODEOWNERS](../../../.github/CODEOWNERS) |
| source | — | [.github/codeql/codeql-config.yml](../../../.github/codeql/codeql-config.yml) |
| source | — | [.github/dependabot.yml](../../../.github/dependabot.yml) |
| source | — | [.github/ISSUE_TEMPLATE/bug.yml](../../../.github/ISSUE_TEMPLATE/bug.yml) |
| source | — | [.github/ISSUE_TEMPLATE/config.yml](../../../.github/ISSUE_TEMPLATE/config.yml) |
| source | — | [.github/ISSUE_TEMPLATE/feature.yml](../../../.github/ISSUE_TEMPLATE/feature.yml) |
| source | — | [.github/ISSUE_TEMPLATE/question.yml](../../../.github/ISSUE_TEMPLATE/question.yml) |
| source | — | [.github/ISSUE_TEMPLATE/security.md](../../../.github/ISSUE_TEMPLATE/security.md) |
| source | — | [.github/labeler.yml](../../../.github/labeler.yml) |
| source | — | [.github/pull_request_template.md](../../../.github/pull_request_template.md) |
| source | — | [.github/release.yml](../../../.github/release.yml) |

## ci-workflows

| Kind | Declaration | Source |
|---|---|---|
| source | — | [.github/workflows/agent.yml](../../../.github/workflows/agent.yml) |
| source | — | [.github/workflows/bridge-test.yml](../../../.github/workflows/bridge-test.yml) |
| source | — | [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) |
| source | — | [.github/workflows/codeql.yml](../../../.github/workflows/codeql.yml) |
| source | — | [.github/workflows/docs-links.yml](../../../.github/workflows/docs-links.yml) |
| source | — | [.github/workflows/docs-site.yml](../../../.github/workflows/docs-site.yml) |
| source | — | [.github/workflows/explorer-build.yml](../../../.github/workflows/explorer-build.yml) |
| source | — | [.github/workflows/explorer-images.yml](../../../.github/workflows/explorer-images.yml) |
| source | — | [.github/workflows/hosted-developer.yml](../../../.github/workflows/hosted-developer.yml) |
| source | — | [.github/workflows/human.yml](../../../.github/workflows/human.yml) |
| source | — | [.github/workflows/interop-migration-testnets.yml](../../../.github/workflows/interop-migration-testnets.yml) |
| source | — | [.github/workflows/interop-x402.yml](../../../.github/workflows/interop-x402.yml) |
| source | — | [.github/workflows/jev-pr-advisory.yml](../../../.github/workflows/jev-pr-advisory.yml) |
| source | — | [.github/workflows/labeler.yml](../../../.github/workflows/labeler.yml) |
| source | — | [.github/workflows/mirror-live.yml](../../../.github/workflows/mirror-live.yml) |
| source | — | [.github/workflows/mirror-verification-live.yml](../../../.github/workflows/mirror-verification-live.yml) |
| source | — | [.github/workflows/node-build.yml](../../../.github/workflows/node-build.yml) |
| source | — | [.github/workflows/offchain-images.yml](../../../.github/workflows/offchain-images.yml) |
| source | — | [.github/workflows/paxeer-buf-check.yml](../../../.github/workflows/paxeer-buf-check.yml) |
| source | — | [.github/workflows/paxeer-cross-arch-build.yml](../../../.github/workflows/paxeer-cross-arch-build.yml) |
| source | — | [.github/workflows/paxeer-dapp-tests.yml](../../../.github/workflows/paxeer-dapp-tests.yml) |
| source | — | [.github/workflows/paxeer-db-tests.yml](../../../.github/workflows/paxeer-db-tests.yml) |
| source | — | [.github/workflows/paxeer-docker-build.yml](../../../.github/workflows/paxeer-docker-build.yml) |
| source | — | [.github/workflows/paxeer-docker-publish.yml](../../../.github/workflows/paxeer-docker-publish.yml) |
| source | — | [.github/workflows/paxeer-ecr.yml](../../../.github/workflows/paxeer-ecr.yml) |
| source | — | [.github/workflows/paxeer-enforce-labels.yml](../../../.github/workflows/paxeer-enforce-labels.yml) |
| source | — | [.github/workflows/paxeer-eth-blocktests.yml](../../../.github/workflows/paxeer-eth-blocktests.yml) |
| source | — | [.github/workflows/paxeer-forge-test.yml](../../../.github/workflows/paxeer-forge-test.yml) |
| source | — | [.github/workflows/paxeer-ghcr-integration-test-cleanup.yml](../../../.github/workflows/paxeer-ghcr-integration-test-cleanup.yml) |
| source | — | [.github/workflows/paxeer-go-test.yml](../../../.github/workflows/paxeer-go-test.yml) |
| source | — | [.github/workflows/paxeer-golangci.yml](../../../.github/workflows/paxeer-golangci.yml) |
| source | — | [.github/workflows/paxeer-hpx-registry.yml](../../../.github/workflows/paxeer-hpx-registry.yml) |
| source | — | [.github/workflows/paxeer-integration-test.yml](../../../.github/workflows/paxeer-integration-test.yml) |
| source | — | [.github/workflows/paxeer-libwasmvm.yml](../../../.github/workflows/paxeer-libwasmvm.yml) |
| source | — | [.github/workflows/paxeer-mock-balances-build.yml](../../../.github/workflows/paxeer-mock-balances-build.yml) |
| source | — | [.github/workflows/paxeer-nightly-ecr.yml](../../../.github/workflows/paxeer-nightly-ecr.yml) |
| source | — | [.github/workflows/paxeer-pr-to-slack-codex.yml](../../../.github/workflows/paxeer-pr-to-slack-codex.yml) |
| source | — | [.github/workflows/paxeer-proto-registry.yml](../../../.github/workflows/paxeer-proto-registry.yml) |
| source | — | [.github/workflows/paxeer-release-check.yml](../../../.github/workflows/paxeer-release-check.yml) |
| source | — | [.github/workflows/paxeer-release-publish.yml](../../../.github/workflows/paxeer-release-publish.yml) |
| source | — | [.github/workflows/paxeer-rocksdb-unit-tests.yml](../../../.github/workflows/paxeer-rocksdb-unit-tests.yml) |
| source | — | [.github/workflows/paxeer-uci-backport.yml](../../../.github/workflows/paxeer-uci-backport.yml) |
| source | — | [.github/workflows/paxeer-uci-go-lint.yml](../../../.github/workflows/paxeer-uci-go-lint.yml) |
| source | — | [.github/workflows/paxeer-x-qualification.yml](../../../.github/workflows/paxeer-x-qualification.yml) |
| source | — | [.github/workflows/platform.yml](../../../.github/workflows/platform.yml) |
| source | — | [.github/workflows/programs-conformance.yml](../../../.github/workflows/programs-conformance.yml) |
| source | — | [.github/workflows/publish-images.yml](../../../.github/workflows/publish-images.yml) |
| source | — | [.github/workflows/ramp-sandbox.yml](../../../.github/workflows/ramp-sandbox.yml) |
| source | — | [.github/workflows/runner-canary.yml](../../../.github/workflows/runner-canary.yml) |
| source | — | [.github/workflows/scorecard.yml](../../../.github/workflows/scorecard.yml) |
| source | — | [.github/workflows/xweb-test.yml](../../../.github/workflows/xweb-test.yml) |

## ctool-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `int main(int argc, char **argv)` | [tools/log_inspect.c](../../../tools/log_inspect.c) |
| `cli_command` | `int main(int argc, char **argv)` | [tools/lxp_corpus_gen.c](../../../tools/lxp_corpus_gen.c) |
| `cli_command` | `int main(int argc, char **argv)` | [tools/lxp_reserve_report.c](../../../tools/lxp_reserve_report.c) |
| `cli_command` | `if (argc == 2 && strcmp(argv[1], "--classes") == 0) {` | [tools/lxp_reserve_report.c](../../../tools/lxp_reserve_report.c) |
| source | — | [tools/lxp_legacy_reader.c](../../../tools/lxp_legacy_reader.c) |
| source | — | [tools/lxp_shadow_compare.c](../../../tools/lxp_shadow_compare.c) |

## deploy-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if __name__ == "__main__":` | [deploy/launchpad/production/bootstrap/smoke.py](../../../deploy/launchpad/production/bootstrap/smoke.py) |
| `cli_command` | `if __name__ == "__main__":` | [deploy/launchpad/production/bootstrap/start.py](../../../deploy/launchpad/production/bootstrap/start.py) |
| `cli_command` | `if __name__ == "__main__":` | [deploy/launchpad/production/bootstrap/validate.py](../../../deploy/launchpad/production/bootstrap/validate.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('--promtool',required=True);p.add_argument('--output',required=True);a=p.parse_args()` | [deploy/launchpad/production/observability/check.py](../../../deploy/launchpad/production/observability/check.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('config');a=p.parse_args()` | [deploy/launchpad/production/observability/contracts.py](../../../deploy/launchpad/production/observability/contracts.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('config');a=p.parse_args()` | [deploy/launchpad/production/observability/contracts_v2.py](../../../deploy/launchpad/production/observability/contracts_v2.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('--binding',required=True);p.add_argument('--config',required=True);p.add_argument('--output',required=True);p.add_argument('--collector',required=True);p.add_argument('--collector-sha256',required=True);p.add_argument('--promtool',required=True);p.add_argument('--promtool-sha256',required=True);a=p.parse_args()` | [deploy/launchpad/production/observability/phase2/build.py](../../../deploy/launchpad/production/observability/phase2/build.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('action',choices=('propose','validate','activate','reject','current'));p.add_argument('--state',required=True);p.add_argument('--binding');p.add_argument('--config');p.add_argument('--actor');p.add_argument('--expected-sequence',type=int);p.add_argument('--evidence');p.add_argument('--attestation-key-file');p.add_argument('--output');p.add_argument('--phase',type=int,choices=(2,3),default=2);p.add_argument('--operator-authority-file');a=p.parse_args()` | [deploy/launchpad/production/observability/phase2/control.py](../../../deploy/launchpad/production/observability/phase2/control.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');pa` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence-sha256');pars` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-eviden` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence-sha256');p` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence-sha256');pa` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence-sha256` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence-` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-ev` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evidence-sha` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `parser=argparse.ArgumentParser();parser.add_argument('action',choices=('collect','evaluate'));parser.add_argument('--plan',required=True);parser.add_argument('--binding',required=True);parser.add_argument('--config',required=True);parser.add_argument('--output',required=True);parser.add_argument('--root',default=str(ROOT));parser.add_argument('--measurement');parser.add_argument('--measurement-sha256');parser.add_argument('--shared-capture');parser.add_argument('--shared-capture-sha256');parser.add_argument('--scenario-evidence');parser.add_argument('--scenario-evide` | [deploy/launchpad/production/observability/phase2/phase4.py](../../../deploy/launchpad/production/observability/phase2/phase4.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('--binding',required=True);p.add_argument('--config',required=True);p.add_argument('--root',required=True);p.add_argument('--output',required=True);p.add_argument('--cycles',type=int,default=1);a=p.parse_args()` | [deploy/launchpad/production/observability/phase2/probes.py](../../../deploy/launchpad/production/observability/phase2/probes.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('--binding',required=True);p.add_argument('--config',required=True);p.add_argument('--evidence',required=True);p.add_argument('--attestation-key-file',required=True);p.add_argument('--output',required=True);p.add_argument('--phase',type=int,choices=(2,3),default=2);a=p.parse_args()` | [deploy/launchpad/production/observability/phase2/qualify.py](../../../deploy/launchpad/production/observability/phase2/qualify.py) |
| `cli_command` | `p=argparse.ArgumentParser();p.add_argument('--binding', required=True);p.add_argument('--config', required=True);p.add_argument('--output', required=True);a=p.parse_args()` | [deploy/launchpad/production/observability/phase2/render.py](../../../deploy/launchpad/production/observability/phase2/render.py) |
| `cli_command` | `func main() {` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `configPath := flags.String("configuration", "", "private configuration file")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `configSHA := flags.String("configuration-sha256", "", "trusted configuration digest")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `deployment := flags.String("deployment-sha256", "", "trusted deployment digest")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `interf := flags.String("interface-sha256", "", "trusted prior gate interface digest")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `inventory := flags.String("inventory-sha256", "", "trusted resource inventory digest")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `proposal := flags.String("proposal", "", "exact recovery proposal")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `realm := flags.String("realm", "", "explicit admitted resource realm")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `source := flags.String("source-sha256", "", "trusted source digest")` | [deploy/launchpad/production/recovery/phase2/broker/main.go](../../../deploy/launchpad/production/recovery/phase2/broker/main.go) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [deploy/launchpad/production/recovery/phase2/check.py](../../../deploy/launchpad/production/recovery/phase2/check.py) |
| `cli_command` | `parser.add_argument('--phase', type=int, choices=(2, 3), default=2)` | [deploy/launchpad/production/recovery/phase2/check.py](../../../deploy/launchpad/production/recovery/phase2/check.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--configuration', required=True)` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--configuration-sha256', required=True)` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--observations')` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--operation', choices=('propose', 'validate', 'activate', 'reject', 'status'), required=True)` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--proposal', required=True)` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--scenarios')` | [deploy/launchpad/production/recovery/phase2/operator.py](../../../deploy/launchpad/production/recovery/phase2/operator.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--configuration', required=True)` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--configuration-sha256', required=True)` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('destination', nargs='?')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--identity')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--identity-sha256')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--inventory')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--key')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--kind', choices=('wal', 'daily-base', 'monthly-base', 'raw-chain', 'metadata', 'media'))` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('operation', choices=('wal-restore', 'archive-file', 'verify-objects', 'base-backup', 'rehearse'))` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--requirement')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--retain-until')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--source')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('--source-sha256', required=True)` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser.add_argument('wal', nargs='?')` | [deploy/launchpad/production/recovery/phase2/providers.py](../../../deploy/launchpad/production/recovery/phase2/providers.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [deploy/launchpad/production/recovery/phase4/release.py](../../../deploy/launchpad/production/recovery/phase4/release.py) |
| `cli_command` | `parser.add_argument('--configuration', required=True)` | [deploy/launchpad/production/recovery/phase4/release.py](../../../deploy/launchpad/production/recovery/phase4/release.py) |
| `cli_command` | `parser.add_argument('--configuration-sha256', required=True)` | [deploy/launchpad/production/recovery/phase4/release.py](../../../deploy/launchpad/production/recovery/phase4/release.py) |
| `cli_command` | `parser.add_argument('--mode', choices=('collect', 'evaluate'), required=True)` | [deploy/launchpad/production/recovery/phase4/release.py](../../../deploy/launchpad/production/recovery/phase4/release.py) |
| `cli_command` | `parser.add_argument('--output-directory', required=True)` | [deploy/launchpad/production/recovery/phase4/release.py](../../../deploy/launchpad/production/recovery/phase4/release.py) |
| `cli_command` | `parser=argparse.ArgumentParser()` | [deploy/launchpad/production/topology/phase2/capacity.py](../../../deploy/launchpad/production/topology/phase2/capacity.py) |
| `cli_command` | `parser.add_argument('action',choices=('capture','check'))` | [deploy/launchpad/production/topology/phase2/capacity.py](../../../deploy/launchpad/production/topology/phase2/capacity.py) |
| `cli_command` | `parser.add_argument('--inputs',required=True);parser.add_argument('--source-root',required=True)` | [deploy/launchpad/production/topology/phase2/capacity.py](../../../deploy/launchpad/production/topology/phase2/capacity.py) |
| `cli_command` | `parser.add_argument('--measurements');parser.add_argument('--output',required=True)` | [deploy/launchpad/production/topology/phase2/capacity.py](../../../deploy/launchpad/production/topology/phase2/capacity.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('action', choices=('render', 'propose', 'validate', 'activate', 'reject', 'diagnostics', 'rehearse', 'recover-rehearsal'))` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--actor', required=True)` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--authorization')` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--evidence')` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--inputs', required=True)` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--journal', required=True)` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--phase', type=int, choices=(2,3), default=2)` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--rehearsal')` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser.add_argument('--source-root', required=True)` | [deploy/launchpad/production/topology/phase2/operator.py](../../../deploy/launchpad/production/topology/phase2/operator.py) |
| `cli_command` | `parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('command',choices=['validate','render','verify-plan','check-evidence']);parser.add_argument('--config',required=True);parser.add_argument('--plan');parser.add_argument('--evidence');parser.add_argument('--output')` | [deploy/launchpad/production/topology/topology.py](../../../deploy/launchpad/production/topology/topology.py) |
| source | — | [deploy/launchpad/deploy.sh](../../../deploy/launchpad/deploy.sh) |
| source | — | [deploy/launchpad/production/topology/native_nats_check.py](../../../deploy/launchpad/production/topology/native_nats_check.py) |
| source | — | [deploy/launchpad/web-build-env.sh](../../../deploy/launchpad/web-build-env.sh) |

## deploy-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"build": {` | [deploy/launchpad/railway/api.railway.json](../../../deploy/launchpad/railway/api.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/api.railway.json](../../../deploy/launchpad/railway/api.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/api-sockets.railway.json](../../../deploy/launchpad/railway/api-sockets.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/api-sockets.railway.json](../../../deploy/launchpad/railway/api-sockets.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/charts.railway.json](../../../deploy/launchpad/railway/charts.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/charts.railway.json](../../../deploy/launchpad/railway/charts.railway.json) |
| `config_key` | `"alerts": [` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"capacity": {` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"environment": "qualification",` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"application_principal": "application-runtime",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"archive_bucket": "kindle-backups-primary",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"archive_days": 365,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"backup_principal": "backup-writer",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"base_backup_seconds": 86400,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"daily_points": 35,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"environment": "qualification",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"hot_hours": 72` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"monthly_points": 12,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"postgres_major": 17,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"primary_region": "region-primary",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"recovery_region": "region-recovery",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"replica_bucket": "kindle-backups-recovery",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"replica_lag_seconds": 300,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"restore_principal": "recovery-reader",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"retention_principal": "retention-operator",` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"rpo_seconds": 300,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"rto_seconds": 1800,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"version": 1,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"wal_age_seconds": 60,` | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| `config_key` | `"exporter": {` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"failure_queue_mib": 512,` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"probes": [` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"retention_days": 14,` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"revision": 1,` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"series_per_instance": 20000,` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"services": [` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"success_sample_percent": 1,` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"alerts": [` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"capacity": {` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"environment": "qualification",` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"exporter": {` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"failure_queue_mib": 512,` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"probes": [` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"retention_days": 14,` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"revision": 2,` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"series_per_instance": 20000,` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"services": [` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"success_sample_percent": 1,` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"version": "2.0",` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"zones": [` | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| `config_key` | `"version": "1.0",` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `"zones": [` | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| `config_key` | `CREATE FUNCTION __SCHEMA__.activate(proposal text, authenticated_actor text) RETURNS bigint` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE FUNCTION __SCHEMA__.activate_fenced_v2(proposal text, authenticated_actor text) RETURNS bigint` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE FUNCTION __SCHEMA__.activate_fenced_v3(proposal text, observation_digest text, scenarios_digest text)` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE FUNCTION __SCHEMA__.assert_epoch(wanted bigint, wanted_region text) RETURNS void` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE FUNCTION __SCHEMA__.current_epoch() RETURNS TABLE(epoch bigint,region text)` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE TABLE __SCHEMA__.recovery_audit (` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE TABLE __SCHEMA__.recovery_broker_fences (` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE TABLE __SCHEMA__.recovery_control (` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE TABLE __SCHEMA__.recovery_proposals (` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CREATE TABLE __SCHEMA__.recovery_validations_v3 (` | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| `config_key` | `CHAIN_ID=125` | [deploy/launchpad/production/bootstrap/fixture.env.example](../../../deploy/launchpad/production/bootstrap/fixture.env.example) |
| `config_key` | `S3_ACCESS_KEY_ID=kindle_fixture` | [deploy/launchpad/production/bootstrap/fixture.env.example](../../../deploy/launchpad/production/bootstrap/fixture.env.example) |
| `config_key` | `S3_BUCKET=kindle-fixture` | [deploy/launchpad/production/bootstrap/fixture.env.example](../../../deploy/launchpad/production/bootstrap/fixture.env.example) |
| `config_key` | `S3_REGION=us-east-1` | [deploy/launchpad/production/bootstrap/fixture.env.example](../../../deploy/launchpad/production/bootstrap/fixture.env.example) |
| `config_key` | `S3_SECRET_ACCESS_KEY=kindle_fixture_password` | [deploy/launchpad/production/bootstrap/fixture.env.example](../../../deploy/launchpad/production/bootstrap/fixture.env.example) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/gateway.railway.json](../../../deploy/launchpad/railway/gateway.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/gateway.railway.json](../../../deploy/launchpad/railway/gateway.railway.json) |
| `config_key` | `"images": [` | [deploy/launchpad/production/bootstrap/images.json](../../../deploy/launchpad/production/bootstrap/images.json) |
| `config_key` | `"limitations": [` | [deploy/launchpad/production/bootstrap/images.json](../../../deploy/launchpad/production/bootstrap/images.json) |
| `config_key` | `"schema_version": 1,` | [deploy/launchpad/production/bootstrap/images.json](../../../deploy/launchpad/production/bootstrap/images.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/indexer.railway.json](../../../deploy/launchpad/railway/indexer.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/indexer.railway.json](../../../deploy/launchpad/railway/indexer.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/livestream.railway.json](../../../deploy/launchpad/railway/livestream.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/livestream.railway.json](../../../deploy/launchpad/railway/livestream.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/metadata.railway.json](../../../deploy/launchpad/railway/metadata.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/metadata.railway.json](../../../deploy/launchpad/railway/metadata.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/pnl.railway.json](../../../deploy/launchpad/railway/pnl.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/pnl.railway.json](../../../deploy/launchpad/railway/pnl.railway.json) |
| `config_key` | `[meta]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.explorer]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.gateway]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.indexer]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.nats]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.nats-2]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.nats-3]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.portal]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.postgres]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[service.web]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.explorer]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.gateway]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.indexer]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.nats]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.nats-2]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.nats-3]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.portal]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.postgres]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `[vars.web]` | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| `config_key` | `"livestream": {` | [deploy/launchpad/railway-values.json](../../../deploy/launchpad/railway-values.json) |
| `config_key` | `"web": {` | [deploy/launchpad/railway-values.json](../../../deploy/launchpad/railway-values.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/ranking.railway.json](../../../deploy/launchpad/railway/ranking.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/ranking.railway.json](../../../deploy/launchpad/railway/ranking.railway.json) |
| `config_key` | `"activation": "disabled-until-coordinator-review",` | [deploy/launchpad/production/recovery/resources.json](../../../deploy/launchpad/production/recovery/resources.json) |
| `config_key` | `"command_contracts": {` | [deploy/launchpad/production/recovery/resources.json](../../../deploy/launchpad/production/recovery/resources.json) |
| `config_key` | `"iam": {` | [deploy/launchpad/production/recovery/resources.json](../../../deploy/launchpad/production/recovery/resources.json) |
| `config_key` | `"resources": {` | [deploy/launchpad/production/recovery/resources.json](../../../deploy/launchpad/production/recovery/resources.json) |
| `config_key` | `"version": 1,` | [deploy/launchpad/production/recovery/resources.json](../../../deploy/launchpad/production/recovery/resources.json) |
| `config_key` | `[meta]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.api]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.api-sockets]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.charts]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.gateway]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.indexer]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.livestream]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.metadata]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.pnl]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.ranking]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.social]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.stats]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.user]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[service.web]` | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| `config_key` | `[meta]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[meta]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.explorer]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.gateway]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.indexer]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.nats]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.nats-2]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.nats-3]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.portal]` | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |
| `config_key` | `[service.archive]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.dashboard]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.dashboard-web]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.gas-station]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.identity]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.indexer]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.internal-approvals]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.internal-journeys]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.internal-kms]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.internal-payments]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.internal-programs]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.interop]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.program-registry]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.ramp]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.redis-internal]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.redis-router]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.router]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.search-front]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.sequencer-primary]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.sequencer-standby]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.wallet-gateway]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.wallet-postgres]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.webhooks-ingress]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `[service.webhooks-public]` | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/social.railway.json](../../../deploy/launchpad/railway/social.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/social.railway.json](../../../deploy/launchpad/railway/social.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/stats.railway.json](../../../deploy/launchpad/railway/stats.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/stats.railway.json](../../../deploy/launchpad/railway/stats.railway.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/user.railway.json](../../../deploy/launchpad/railway/user.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/user.railway.json](../../../deploy/launchpad/railway/user.railway.json) |
| `config_key` | `"api": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"api-sockets": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"charts": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"gateway": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"indexer": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"livestream": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"metadata": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"pnl": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"ranking": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"social": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"stats": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"user": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"web": {` | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| `config_key` | `"build": {` | [deploy/launchpad/railway/web.railway.json](../../../deploy/launchpad/railway/web.railway.json) |
| `config_key` | `"deploy": {` | [deploy/launchpad/railway/web.railway.json](../../../deploy/launchpad/railway/web.railway.json) |
| source | — | [deploy/launchpad/docker-compose.strangler.yml](../../../deploy/launchpad/docker-compose.strangler.yml) |
| source | — | [deploy/launchpad/production/bootstrap/compose.yml](../../../deploy/launchpad/production/bootstrap/compose.yml) |

## deploy-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `RECOVERY_OPERATOR_QUALIFICATION_CONFIG` | [deploy/launchpad/production/recovery/phase2/check.py](../../../deploy/launchpad/production/recovery/phase2/check.py) |
| `env_var` | `RECOVERY_OPERATOR_QUALIFICATION_CONFIG_SHA256` | [deploy/launchpad/production/recovery/phase2/check.py](../../../deploy/launchpad/production/recovery/phase2/check.py) |
| `env_var` | `RECOVERY_OPERATOR_SCENARIO_OBSERVATIONS` | [deploy/launchpad/production/recovery/phase2/check.py](../../../deploy/launchpad/production/recovery/phase2/check.py) |
| `env_var` | `RECOVERY_OPERATOR_SCENARIO_REHEARSALS` | [deploy/launchpad/production/recovery/phase2/check.py](../../../deploy/launchpad/production/recovery/phase2/check.py) |
| source | — | [deploy/launchpad/deploy.sh](../../../deploy/launchpad/deploy.sh) |
| source | — | [deploy/launchpad/Dockerfile.native](../../../deploy/launchpad/Dockerfile.native) |
| source | — | [deploy/launchpad/Dockerfile.svc](../../../deploy/launchpad/Dockerfile.svc) |
| source | — | [deploy/launchpad/Dockerfile.web](../../../deploy/launchpad/Dockerfile.web) |

## deploy-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [deploy/launchpad/docker-compose.strangler.yml](../../../deploy/launchpad/docker-compose.strangler.yml) |
| source | — | [deploy/launchpad/Dockerfile.native](../../../deploy/launchpad/Dockerfile.native) |
| source | — | [deploy/launchpad/Dockerfile.svc](../../../deploy/launchpad/Dockerfile.svc) |
| source | — | [deploy/launchpad/Dockerfile.web](../../../deploy/launchpad/Dockerfile.web) |
| source | — | [deploy/launchpad/production/bootstrap/compose.yml](../../../deploy/launchpad/production/bootstrap/compose.yml) |
| source | — | [deploy/launchpad/production/bootstrap/fixture.env.example](../../../deploy/launchpad/production/bootstrap/fixture.env.example) |
| source | — | [deploy/launchpad/production/bootstrap/images.json](../../../deploy/launchpad/production/bootstrap/images.json) |
| source | — | [deploy/launchpad/production/bootstrap/nats.conf](../../../deploy/launchpad/production/bootstrap/nats.conf) |
| source | — | [deploy/launchpad/production/observability/config.json](../../../deploy/launchpad/production/observability/config.json) |
| source | — | [deploy/launchpad/production/observability/config-v2.json](../../../deploy/launchpad/production/observability/config-v2.json) |
| source | — | [deploy/launchpad/production/observability/config-v4.schema.json](../../../deploy/launchpad/production/observability/config-v4.schema.json) |
| source | — | [deploy/launchpad/production/observability/grafana-dashboard.json](../../../deploy/launchpad/production/observability/grafana-dashboard.json) |
| source | — | [deploy/launchpad/production/observability/otel-collector.yaml](../../../deploy/launchpad/production/observability/otel-collector.yaml) |
| source | — | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| source | — | [deploy/launchpad/production/recovery/config.example.json](../../../deploy/launchpad/production/recovery/config.example.json) |
| source | — | [deploy/launchpad/production/recovery/epoch.sql](../../../deploy/launchpad/production/recovery/epoch.sql) |
| source | — | [deploy/launchpad/production/recovery/phase2/broker/go.mod](../../../deploy/launchpad/production/recovery/phase2/broker/go.mod) |
| source | — | [deploy/launchpad/production/recovery/phase2/reconciliation.sql](../../../deploy/launchpad/production/recovery/phase2/reconciliation.sql) |
| source | — | [deploy/launchpad/production/recovery/phase4/config.schema.json](../../../deploy/launchpad/production/recovery/phase4/config.schema.json) |
| source | — | [deploy/launchpad/production/recovery/resources.json](../../../deploy/launchpad/production/recovery/resources.json) |
| source | — | [deploy/launchpad/production/topology/fixtures/cnpg-clusters-v1.29.0.yaml](../../../deploy/launchpad/production/topology/fixtures/cnpg-clusters-v1.29.0.yaml) |
| source | — | [deploy/launchpad/production/topology/fixtures/cnpg-poolers-v1.29.0.yaml](../../../deploy/launchpad/production/topology/fixtures/cnpg-poolers-v1.29.0.yaml) |
| source | — | [deploy/launchpad/production/topology/fixtures/crd-provenance.json](../../../deploy/launchpad/production/topology/fixtures/crd-provenance.json) |
| source | — | [deploy/launchpad/production/topology/fixtures/synthetic-config.json](../../../deploy/launchpad/production/topology/fixtures/synthetic-config.json) |
| source | — | [deploy/launchpad/production/topology/production-input.template.json](../../../deploy/launchpad/production/topology/production-input.template.json) |
| source | — | [deploy/launchpad/railway/api.railway.json](../../../deploy/launchpad/railway/api.railway.json) |
| source | — | [deploy/launchpad/railway/api-sockets.railway.json](../../../deploy/launchpad/railway/api-sockets.railway.json) |
| source | — | [deploy/launchpad/railway/charts.railway.json](../../../deploy/launchpad/railway/charts.railway.json) |
| source | — | [deploy/launchpad/railway/gateway.railway.json](../../../deploy/launchpad/railway/gateway.railway.json) |
| source | — | [deploy/launchpad/railway/indexer.railway.json](../../../deploy/launchpad/railway/indexer.railway.json) |
| source | — | [deploy/launchpad/railway/livestream.railway.json](../../../deploy/launchpad/railway/livestream.railway.json) |
| source | — | [deploy/launchpad/railway/metadata.railway.json](../../../deploy/launchpad/railway/metadata.railway.json) |
| source | — | [deploy/launchpad/railway/pnl.railway.json](../../../deploy/launchpad/railway/pnl.railway.json) |
| source | — | [deploy/launchpad/railway/ranking.railway.json](../../../deploy/launchpad/railway/ranking.railway.json) |
| source | — | [deploy/launchpad/railway/social.railway.json](../../../deploy/launchpad/railway/social.railway.json) |
| source | — | [deploy/launchpad/railway/stats.railway.json](../../../deploy/launchpad/railway/stats.railway.json) |
| source | — | [deploy/launchpad/railway/user.railway.json](../../../deploy/launchpad/railway/user.railway.json) |
| source | — | [deploy/launchpad/railway-values.json](../../../deploy/launchpad/railway-values.json) |
| source | — | [deploy/launchpad/railway/variables.json](../../../deploy/launchpad/railway/variables.json) |
| source | — | [deploy/launchpad/railway/web.railway.json](../../../deploy/launchpad/railway/web.railway.json) |
| source | — | [deploy/railway-perps-project.kvx](../../../deploy/railway-perps-project.kvx) |
| source | — | [deploy/rollout.kvx](../../../deploy/rollout.kvx) |
| source | — | [deploy/rollout-launchpad.kvx](../../../deploy/rollout-launchpad.kvx) |
| source | — | [deploy/rollout-perps.kvx](../../../deploy/rollout-perps.kvx) |

## deploy-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `"alert": "backlog_warning",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "delivery_latency",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "ingest_latency",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "mutation_burn_critical",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "mutation_burn_warning",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "mutation_traffic_insufficient",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "quote_latency",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "read_burn_critical",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "read_burn_warning",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "read_traffic_insufficient",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "rpc_integrity",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "storage_critical",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "storage_warning",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "synthetic_failed",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"alert": "telemetry_cardinality",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:burn_rate1h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:burn_rate30m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:burn_rate5m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:burn_rate6h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:requests1h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:requests30m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:requests5m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:mutation:requests6h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:burn_rate1h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:burn_rate30m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:burn_rate5m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:burn_rate6h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:requests1h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:requests30m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:requests5m",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |
| `metric` | `"record": "slo:read:requests6h",` | [deploy/launchpad/production/observability/prometheus-rules.json](../../../deploy/launchpad/production/observability/prometheus-rules.json) |

## docker-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `--prepare-material) [ "$#" = 1 ] \|\| exit 64; mode=material ;;` | [docker/platform-registry/init.sh](../../../docker/platform-registry/init.sh) |
| source | — | [docker/bridge-relayer/entrypoint.sh](../../../docker/bridge-relayer/entrypoint.sh) |
| source | — | [docker/common/env-files.sh](../../../docker/common/env-files.sh) |
| source | — | [docker/gas-station/init.sh](../../../docker/gas-station/init.sh) |
| source | — | [docker/hpx-registry/entrypoint.sh](../../../docker/hpx-registry/entrypoint.sh) |
| source | — | [docker/human-service/entrypoint.sh](../../../docker/human-service/entrypoint.sh) |
| source | — | [docker/kernel/init.sh](../../../docker/kernel/init.sh) |
| source | — | [docker/localnode/scripts/deploy.sh](../../../docker/localnode/scripts/deploy.sh) |
| source | — | [docker/localnode/scripts/step0_build.sh](../../../docker/localnode/scripts/step0_build.sh) |
| source | — | [docker/localnode/scripts/step1_configure_init.sh](../../../docker/localnode/scripts/step1_configure_init.sh) |
| source | — | [docker/localnode/scripts/step2_genesis.sh](../../../docker/localnode/scripts/step2_genesis.sh) |
| source | — | [docker/localnode/scripts/step3_add_validator_to_genesis.sh](../../../docker/localnode/scripts/step3_add_validator_to_genesis.sh) |
| source | — | [docker/localnode/scripts/step4_config_override.sh](../../../docker/localnode/scripts/step4_config_override.sh) |
| source | — | [docker/localnode/scripts/step5_start_pax.sh](../../../docker/localnode/scripts/step5_start_pax.sh) |
| source | — | [docker/monitornode/scripts/start-grafana.sh](../../../docker/monitornode/scripts/start-grafana.sh) |
| source | — | [docker/monitornode/scripts/start-prometheus.sh](../../../docker/monitornode/scripts/start-prometheus.sh) |
| source | — | [docker/monitornode/scripts/stop-grafana.sh](../../../docker/monitornode/scripts/stop-grafana.sh) |
| source | — | [docker/monitornode/scripts/stop-prometheus.sh](../../../docker/monitornode/scripts/stop-prometheus.sh) |
| source | — | [docker/platform-gateway/entrypoint.sh](../../../docker/platform-gateway/entrypoint.sh) |
| source | — | [docker/redis/entrypoint.sh](../../../docker/redis/entrypoint.sh) |
| source | — | [docker/rpcnode/scripts/deploy.sh](../../../docker/rpcnode/scripts/deploy.sh) |
| source | — | [docker/rpcnode/scripts/step0_build.sh](../../../docker/rpcnode/scripts/step0_build.sh) |
| source | — | [docker/rpcnode/scripts/step1_configure_init.sh](../../../docker/rpcnode/scripts/step1_configure_init.sh) |
| source | — | [docker/rpcnode/scripts/step2_start_pax.sh](../../../docker/rpcnode/scripts/step2_start_pax.sh) |
| source | — | [docker/search-front/real-ip.sh](../../../docker/search-front/real-ip.sh) |
| source | — | [docker/wallet-gateway/entrypoint.sh](../../../docker/wallet-gateway/entrypoint.sh) |
| source | — | [docker/x-websearch/entrypoint.sh](../../../docker/x-websearch/entrypoint.sh) |

## docker-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `[admin_server]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `admin_enabled = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[api]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[api]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enable = true` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enabled-unsafe-cors = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `max-open-connections = 1000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `max-open-connections = 1000` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `rpc-max-body-bytes = 1000000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `rpc-max-body-bytes = 1000000` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `rpc-read-timeout = 10` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `rpc-read-timeout = 10` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `rpc-write-timeout = 0` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `rpc-write-timeout = 0` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `swagger = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `swagger = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `concurrency-workers = 4` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[evm]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[evm]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable_test_api = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enable_test_api = true` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enabled_legacy_pax_apis = [` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enabled_legacy_pax_apis = [` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `max_concurrent_simulation_calls = 0` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[grpc]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[grpc]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[grpc-web]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[grpc-web]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable-unsafe-cors = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `halt-height = 0` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `halt-height = 0` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `halt-time = 0` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `halt-time = 0` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `index-events = []` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `index-events = []` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `inter-block-cache = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `inter-block-cache = true` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `min-retain-blocks = 4000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `min-retain-blocks = 0` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `minimum-gas-prices = "0.01uhpx"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `minimum-gas-prices = "0.01uhpx"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `occ-enabled = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `pruning = "nothing"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `pruning = "nothing"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `pruning-interval = "0"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `pruning-interval = "0"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `pruning-keep-every = "0"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `pruning-keep-every = "0"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `pruning-keep-recent = "0"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `pruning-keep-recent = "0"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[rosetta]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[rosetta]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `address = ":8080"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `address = ":8080"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `blockchain = "app"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `blockchain = "app"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enable = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `network = "network"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `network = "network"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `offline = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `offline = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `retries = 3` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `retries = 3` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[state-commit]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[state-commit]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `sc-async-commit-buffer = 100` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-async-commit-buffer = 100` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `sc-cache-size = 1000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-enable = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-enable = true` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `sc-keep-recent = 1` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-keep-recent = 1` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `sc-keys-to-migrate-per-block = 1024` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-snapshot-interval = 1000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-snapshot-interval = 1000` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `sc-snapshot-writer-limit = 2` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `sc-snapshot-writer-limit = 2` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `sc-zero-copy = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[state-store]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[state-store]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `evm-ss-db-directory = ""` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `evm-ss-db-directory = ""` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `evm-ss-separate-dbs = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `evm-ss-separate-dbs = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `evm-ss-split = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `evm-ss-split = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `ss-async-write-buffer = 100` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `ss-async-write-buffer = 100` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `ss-backend = "pebbledb"` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `ss-backend = "pebbledb"` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `ss-enable = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `ss-enable = true` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `ss-import-num-workers = 1` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `ss-import-num-workers = 1` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `ss-keep-recent = 10000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `ss-keep-recent = 1000` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `ss-prune-interval = 60` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `ss-prune-interval = 60` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[state-sync]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[state-sync]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `snapshot-directory = ""` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `snapshot-interval = 100` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `snapshot-interval = 0` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `snapshot-keep-recent = 3` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `snapshot-keep-recent = 2` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[telemetry]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[telemetry]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable-hostname = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enable-hostname = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable-hostname-label = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enable-hostname-label = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enable-service-label = false` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enable-service-label = false` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `enabled = true` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `enabled = true` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `global-labels = []` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `global-labels = []` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `prometheus-retention-time = 5` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `prometheus-retention-time = 7200` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `service-name = ""` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `service-name = ""` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[wasm]` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `[wasm]` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `lru_size = 0` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `lru_size = 0` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `query_gas_limit = 300000` | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| `config_key` | `query_gas_limit = 300000` | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| `config_key` | `[consensus]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[consensus]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `create-empty-blocks = true` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `create-empty-blocks = true` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `create-empty-blocks-interval = "0s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `create-empty-blocks-interval = "0s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `double-sign-check-height = 0` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `double-sign-check-height = 0` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `gossip-tx-key-only=true` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `gossip-tx-key-only=true` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `peer-gossip-sleep-duration = "100ms"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `peer-gossip-sleep-duration = "100ms"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `peer-query-maj23-sleep-duration = "2s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `peer-query-maj23-sleep-duration = "2s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe-bypass-commit-timeout-override = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe-bypass-commit-timeout-override = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe-commit-timeout-override = "50ms"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe-commit-timeout-override = "50ms"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe-propose-timeout-delta-override = "500ms"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe-propose-timeout-delta-override = "500ms"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe-propose-timeout-override = "3s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe-propose-timeout-override = "3s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe-vote-timeout-delta-override = "500ms"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe-vote-timeout-delta-override = "500ms"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe-vote-timeout-override = "50ms"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe-vote-timeout-override = "50ms"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `wal-file = "data/cs.wal/wal"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `wal-file = "data/cs.wal/wal"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `db-backend = "goleveldb"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `db-backend = "goleveldb"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `db-dir = "data"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `db-dir = "data"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `genesis-file = "config/genesis.json"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `genesis-file = "config/genesis.json"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[instrumentation]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[instrumentation]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-open-connections = 3` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-open-connections = 3` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `namespace = "tendermint"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `namespace = "tendermint"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `prometheus = true` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `prometheus = true` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `prometheus-listen-addr = ":26660"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `prometheus-listen-addr = ":26660"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `log-format = "plain"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `log-format = "plain"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `log-level = "info"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `log-level = "info"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[mempool]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[mempool]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `broadcast = true` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `broadcast = true` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `cache-size = 50000` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `cache-size = 50000` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `keep-invalid-txs-in-cache = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `keep-invalid-txs-in-cache = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-batch-bytes = 0` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-batch-bytes = 0` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-tx-bytes = 20485760` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-tx-bytes = 20485760` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-txs-bytes = 107374182400` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-txs-bytes = 107374182400` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `size = 5000` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `size = 5000` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `ttl-duration = "0s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `ttl-duration = "0s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `ttl-num-blocks = 0` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `ttl-num-blocks = 0` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `mode = "validator"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `mode = "full"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `moniker = "pax-node-0"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `moniker = "pax-rpc-node"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `node-key-file = "config/node_key.json"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `node-key-file = "config/node_key.json"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[p2p]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[p2p]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `allow-duplicate-ip = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `allow-duplicate-ip = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `bootstrap-peers = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `bootstrap-peers = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `dial-timeout = "3s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `dial-timeout = "3s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `external-address = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `external-address = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `flush-throttle-timeout = "10ms"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `flush-throttle-timeout = "10ms"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `handshake-timeout = "20s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `handshake-timeout = "20s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-connections = 200` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-connections = 200` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-incoming-connection-attempts = 100` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-incoming-connection-attempts = 100` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-packet-msg-payload-size = 102400` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-packet-msg-payload-size = 102400` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `persistent-peers = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `persistent-peers = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `pex = true` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `pex = true` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `private-peer-ids = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `private-peer-ids = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `queue-type = "priority"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `queue-type = "priority"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `recv-rate = 204800000` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `recv-rate = 204800000` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `send-rate = 204800000` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `send-rate = 204800000` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `upnp = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `upnp = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[priv-validator]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[priv-validator]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `client-certificate-file = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `client-certificate-file = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `client-key-file = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `client-key-file = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `key-file = "config/priv_validator_key.json"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `key-file = "config/priv_validator_key.json"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `laddr = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `laddr = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `root-ca-file = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `root-ca-file = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `state-file = "data/priv_validator_state.json"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `state-file = "data/priv_validator_state.json"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[rpc]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[rpc]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `cors-allowed-headers = ["Origin", "Accept", "Content-Type", "X-Requested-With", "X-Server-Time", ]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `cors-allowed-headers = ["Origin", "Accept", "Content-Type", "X-Requested-With", "X-Server-Time", ]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `cors-allowed-methods = ["HEAD", "GET", "POST", ]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `cors-allowed-methods = ["HEAD", "GET", "POST", ]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `cors-allowed-origins = []` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `cors-allowed-origins = []` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `event-log-max-items = 0` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `event-log-max-items = 0` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `event-log-window-size = "30s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `event-log-window-size = "30s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `experimental-disable-websocket = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `experimental-disable-websocket = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-body-bytes = 1000000` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-body-bytes = 1000000` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-header-bytes = 1048576` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-header-bytes = 1048576` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-open-connections = 900` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-open-connections = 900` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-subscription-clients = 100` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-subscription-clients = 100` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `max-subscriptions-per-client = 5` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `max-subscriptions-per-client = 5` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `pprof-laddr` (loopback profiling listener, port 6060) | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `pprof-laddr` (loopback profiling listener, port 6060) | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `timeout-broadcast-tx-commit = "10s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `timeout-broadcast-tx-commit = "10s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `tls-cert-file = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `tls-cert-file = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `tls-key-file = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `tls-key-file = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `unsafe = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `unsafe = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[statesync]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[statesync]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `chunk-request-timeout = "15s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `chunk-request-timeout = "15s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `discovery-time = "15s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `discovery-time = "15s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `enable = false` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `enable = false` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `fetchers = "4"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `fetchers = "4"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `rpc-servers = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `rpc-servers = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `temp-dir = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `temp-dir = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `trust-hash = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `trust-hash = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `trust-height = 0` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `trust-height = 0` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `trust-period = "168h0m0s"` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `trust-period = "168h0m0s"` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `[tx-index]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `[tx-index]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `indexer = ["kv"]` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `indexer = ["kv"]` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `psql-conn = ""` | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| `config_key` | `psql-conn = ""` | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| `config_key` | `LAYERX_KERNEL_PROFILE=native` | [docker/kernel/kernel.env.example](../../../docker/kernel/kernel.env.example) |
| `config_key` | `LAYERX_NODE_NETWORK_ID=125` | [docker/kernel/kernel.env.example](../../../docker/kernel/kernel.env.example) |
| `config_key` | `EnvironmentFile=/etc/layerx/kernel.env` | [docker/kernel/layerx-kernel.service](../../../docker/kernel/layerx-kernel.service) |
| `config_key` | `EnvironmentFile=/etc/layerx/registry.env` | [docker/platform-registry/layerx-registry.service](../../../docker/platform-registry/layerx-registry.service) |
| `config_key` | `LAYERX_EVENTS_PROGRAM_UPSTREAM_CA_DER=/data/tls/registry/ca.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_PROGRAM_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE=/data/tls/registry-event-client/password` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_PROGRAM_UPSTREAM_CLIENT_IDENTITY_PKCS12=/data/tls/registry-event-client/identity.p12` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_PROGRAM_UPSTREAM_TOKEN_FILE=/run/layerx/secrets/program-events-token` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CA_DER=/data/tls/registry/ca.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE=/data/tls/registry-event-client/password` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CLIENT_IDENTITY_PKCS12=/data/tls/registry-event-client/identity.p12` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_EVENTS_WEBHOOKS_UPSTREAM_TOKEN_FILE=/run/layerx/secrets/webhooks-events-token` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_FILE_SIZE_BYTES=67108864` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_MEMORY_BYTES=2147483648` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_PROCESS_LIMIT=64` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_QUOTA_BYTES=5368709120` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_QUOTA_INODES=65536` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_ROOT=/data/builds` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILD_TIMEOUT_SECONDS=1800` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILDER_ENTRYPOINT=/bin/layerx-build` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILDER_ENVIRONMENT_ROOT=/data/builder/rootfs` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME=/usr/bin/bwrap` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR=/usr/bin/layerx-cgroup-exec` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_CLIENT_CA_DER=/data/tls/registry/ca.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_HEALTH_LISTEN=[::]:9421` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_HOST_CGROUP_MOUNT=/run/layerx/host-cgroup` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_IDENTITY_CA_DER=/data/tls/registry/ca.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_IDENTITY_CLIENT_IDENTITY_PASSWORD_FILE=/data/tls/registry-event-client/password` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_IDENTITY_CLIENT_IDENTITY_PKCS12=/data/tls/registry-event-client/identity.p12` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_IDENTITY_TOKEN_FILE=/run/layerx/secrets/identity-token` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_JOURNAL=/data/journal` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_LISTEN=[::]:9420` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_MAX_BUILDS=4` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_MAX_CONNECTIONS=128` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_NODE_AUTHORIZATION=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_OUTBOUND_CA_DER=/data/tls/registry/ca.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_PUBLICATION_TOKEN_FILE=/data/tokens/publication` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_RECEIPT_AUTHORITY_AUTHORIZATION=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_REQUEST_TIMEOUT_SECONDS=1800` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_REQUEST_TOKEN_FILE=/data/tokens/request` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_STATE=/data/state` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_TLS_CERT_DER=/data/tls/registry/cert.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `LAYERX_REGISTRY_TLS_KEY_DER=/data/tls/registry/key.der` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `REGISTRY_IDENTITY_TOKEN=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `REGISTRY_PROGRAM_EVENTS_TOKEN=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `REGISTRY_PUBLICATION_TOKEN=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `REGISTRY_REQUEST_TOKEN=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| `config_key` | `REGISTRY_WEBHOOKS_EVENTS_TOKEN=` | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| source | — | [docker/docker-compose.giga-mixed.yml](../../../docker/docker-compose.giga-mixed.yml) |
| source | — | [docker/docker-compose.monitoring.yml](../../../docker/docker-compose.monitoring.yml) |
| source | — | [docker/docker-compose.yml](../../../docker/docker-compose.yml) |

## docker-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [docker/bridge-relayer/Dockerfile](../../../docker/bridge-relayer/Dockerfile) |
| source | — | [docker/bridge-relayer/entrypoint.sh](../../../docker/bridge-relayer/entrypoint.sh) |
| source | — | [docker/bridge-relayer/files.tsv](../../../docker/bridge-relayer/files.tsv) |
| source | — | [docker/common/env-files.sh](../../../docker/common/env-files.sh) |
| source | — | [docker/explorer-backend/Dockerfile](../../../docker/explorer-backend/Dockerfile) |
| source | — | [docker/explorer-backend/oldUI.Dockerfile](../../../docker/explorer-backend/oldUI.Dockerfile) |
| source | — | [docker/explorer-elixir-builder/Dockerfile](../../../docker/explorer-elixir-builder/Dockerfile) |
| source | — | [docker/explorer-frontend/Dockerfile](../../../docker/explorer-frontend/Dockerfile) |
| source | — | [docker/explorer-sig-provider/Dockerfile](../../../docker/explorer-sig-provider/Dockerfile) |
| source | — | [docker/explorer-smart-contract-verifier/Dockerfile](../../../docker/explorer-smart-contract-verifier/Dockerfile) |
| source | — | [docker/flyci-controller/Dockerfile](../../../docker/flyci-controller/Dockerfile) |
| source | — | [docker/flyci-runner/Dockerfile](../../../docker/flyci-runner/Dockerfile) |
| source | — | [docker/gas-station/Dockerfile](../../../docker/gas-station/Dockerfile) |
| source | — | [docker/gas-station/init.sh](../../../docker/gas-station/init.sh) |
| source | — | [docker/hpx-registry/Dockerfile](../../../docker/hpx-registry/Dockerfile) |
| source | — | [docker/hpx-registry/entrypoint.sh](../../../docker/hpx-registry/entrypoint.sh) |
| source | — | [docker/human-service/Dockerfile](../../../docker/human-service/Dockerfile) |
| source | — | [docker/human-service/entrypoint.sh](../../../docker/human-service/entrypoint.sh) |
| source | — | [docker/interop-gateway/files.tsv](../../../docker/interop-gateway/files.tsv) |
| source | — | [docker/interop-mirror/Dockerfile](../../../docker/interop-mirror/Dockerfile) |
| source | — | [docker/kernel/Dockerfile](../../../docker/kernel/Dockerfile) |
| source | — | [docker/kernel/files.tsv](../../../docker/kernel/files.tsv) |
| source | — | [docker/kernel/init.sh](../../../docker/kernel/init.sh) |
| source | — | [docker/layerx/Dockerfile](../../../docker/layerx/Dockerfile) |
| source | — | [docker/localnode/Dockerfile](../../../docker/localnode/Dockerfile) |
| source | — | [docker/localnode/scripts/deploy.sh](../../../docker/localnode/scripts/deploy.sh) |
| source | — | [docker/localnode/scripts/step0_build.sh](../../../docker/localnode/scripts/step0_build.sh) |
| source | — | [docker/localnode/scripts/step1_configure_init.sh](../../../docker/localnode/scripts/step1_configure_init.sh) |
| source | — | [docker/localnode/scripts/step2_genesis.sh](../../../docker/localnode/scripts/step2_genesis.sh) |
| source | — | [docker/localnode/scripts/step4_config_override.sh](../../../docker/localnode/scripts/step4_config_override.sh) |
| source | — | [docker/localnode/scripts/step5_start_pax.sh](../../../docker/localnode/scripts/step5_start_pax.sh) |
| source | — | [docker/monitornode/scripts/start-grafana.sh](../../../docker/monitornode/scripts/start-grafana.sh) |
| source | — | [docker/monitornode/scripts/start-prometheus.sh](../../../docker/monitornode/scripts/start-prometheus.sh) |
| source | — | [docker/paxeer/Dockerfile.paxd](../../../docker/paxeer/Dockerfile.paxd) |
| source | — | [docker/paxeer/Dockerfile.paxd-node](../../../docker/paxeer/Dockerfile.paxd-node) |
| source | — | [docker/platform-authority/Dockerfile](../../../docker/platform-authority/Dockerfile) |
| source | — | [docker/platform-dashboard/files.tsv](../../../docker/platform-dashboard/files.tsv) |
| source | — | [docker/platform-dashboard-web/Dockerfile](../../../docker/platform-dashboard-web/Dockerfile) |
| source | — | [docker/platform-gateway/files.tsv](../../../docker/platform-gateway/files.tsv) |
| source | — | [docker/platform-identity/files.tsv](../../../docker/platform-identity/files.tsv) |
| source | — | [docker/platform-indexer/files.tsv](../../../docker/platform-indexer/files.tsv) |
| source | — | [docker/platform-internal/Dockerfile](../../../docker/platform-internal/Dockerfile) |
| source | — | [docker/platform-internal/files.tsv](../../../docker/platform-internal/files.tsv) |
| source | — | [docker/platform-registry-builder/Dockerfile](../../../docker/platform-registry-builder/Dockerfile) |
| source | — | [docker/platform-registry/Dockerfile](../../../docker/platform-registry/Dockerfile) |
| source | — | [docker/platform-registry/init.sh](../../../docker/platform-registry/init.sh) |
| source | — | [docker/platform-testnet/Dockerfile](../../../docker/platform-testnet/Dockerfile) |
| source | — | [docker/platform-webhooks/Dockerfile](../../../docker/platform-webhooks/Dockerfile) |
| source | — | [docker/platform-webhooks/files.tsv](../../../docker/platform-webhooks/files.tsv) |
| source | — | [docker/ramps/files.tsv](../../../docker/ramps/files.tsv) |
| source | — | [docker/redis/Dockerfile](../../../docker/redis/Dockerfile) |
| source | — | [docker/redis/entrypoint.sh](../../../docker/redis/entrypoint.sh) |
| source | — | [docker/redis/files.tsv](../../../docker/redis/files.tsv) |
| source | — | [docker/rpcnode/Dockerfile](../../../docker/rpcnode/Dockerfile) |
| source | — | [docker/rpcnode/scripts/deploy.sh](../../../docker/rpcnode/scripts/deploy.sh) |
| source | — | [docker/rpcnode/scripts/step1_configure_init.sh](../../../docker/rpcnode/scripts/step1_configure_init.sh) |
| source | — | [docker/search-front/Dockerfile](../../../docker/search-front/Dockerfile) |
| source | — | [docker/search-front/files.tsv](../../../docker/search-front/files.tsv) |
| source | — | [docker/search-front/real-ip.sh](../../../docker/search-front/real-ip.sh) |
| source | — | [docker/wallet-attestor/files.tsv](../../../docker/wallet-attestor/files.tsv) |
| source | — | [docker/wallet-gateway/Dockerfile](../../../docker/wallet-gateway/Dockerfile) |
| source | — | [docker/wallet-gateway/entrypoint.sh](../../../docker/wallet-gateway/entrypoint.sh) |
| source | — | [docker/wallet-gateway/files.tsv](../../../docker/wallet-gateway/files.tsv) |
| source | — | [docker/wallet-pwa/Dockerfile](../../../docker/wallet-pwa/Dockerfile) |
| source | — | [docker/web/Dockerfile](../../../docker/web/Dockerfile) |
| source | — | [docker/x-websearch/Dockerfile](../../../docker/x-websearch/Dockerfile) |
| source | — | [docker/x-websearch/entrypoint.sh](../../../docker/x-websearch/entrypoint.sh) |

## docker-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [docker/bridge-relayer/Dockerfile](../../../docker/bridge-relayer/Dockerfile) |
| source | — | [docker/bridge-relayer/files.tsv](../../../docker/bridge-relayer/files.tsv) |
| source | — | [docker/docker-compose.giga-mixed.yml](../../../docker/docker-compose.giga-mixed.yml) |
| source | — | [docker/docker_compose_monitoring/grafana-datasource.yaml](../../../docker/docker_compose_monitoring/grafana-datasource.yaml) |
| source | — | [docker/docker_compose_monitoring/prometheus.yaml](../../../docker/docker_compose_monitoring/prometheus.yaml) |
| source | — | [docker/docker-compose.monitoring.yml](../../../docker/docker-compose.monitoring.yml) |
| source | — | [docker/docker-compose.yml](../../../docker/docker-compose.yml) |
| source | — | [docker/explorer-backend/Dockerfile](../../../docker/explorer-backend/Dockerfile) |
| source | — | [docker/explorer-backend/oldUI.Dockerfile](../../../docker/explorer-backend/oldUI.Dockerfile) |
| source | — | [docker/explorer-elixir-builder/Dockerfile](../../../docker/explorer-elixir-builder/Dockerfile) |
| source | — | [docker/explorer-frontend/Dockerfile](../../../docker/explorer-frontend/Dockerfile) |
| source | — | [docker/explorer-sig-provider/Dockerfile](../../../docker/explorer-sig-provider/Dockerfile) |
| source | — | [docker/explorer-smart-contract-verifier/Dockerfile](../../../docker/explorer-smart-contract-verifier/Dockerfile) |
| source | — | [docker/flyci-controller/Dockerfile](../../../docker/flyci-controller/Dockerfile) |
| source | — | [docker/flyci-runner/Dockerfile](../../../docker/flyci-runner/Dockerfile) |
| source | — | [docker/gas-station/Dockerfile](../../../docker/gas-station/Dockerfile) |
| source | — | [docker/hpx-registry/Dockerfile](../../../docker/hpx-registry/Dockerfile) |
| source | — | [docker/human-service/Dockerfile](../../../docker/human-service/Dockerfile) |
| source | — | [docker/interop-gateway/Dockerfile](../../../docker/interop-gateway/Dockerfile) |
| source | — | [docker/interop-gateway/files.tsv](../../../docker/interop-gateway/files.tsv) |
| source | — | [docker/interop-mirror/Dockerfile](../../../docker/interop-mirror/Dockerfile) |
| source | — | [docker/kernel/Dockerfile](../../../docker/kernel/Dockerfile) |
| source | — | [docker/kernel/files.tsv](../../../docker/kernel/files.tsv) |
| source | — | [docker/kernel/kernel.env.example](../../../docker/kernel/kernel.env.example) |
| source | — | [docker/kernel/layerx-kernel.service](../../../docker/kernel/layerx-kernel.service) |
| source | — | [docker/layerx/Dockerfile](../../../docker/layerx/Dockerfile) |
| source | — | [docker/localnode/config/app.toml](../../../docker/localnode/config/app.toml) |
| source | — | [docker/localnode/config/config.toml](../../../docker/localnode/config/config.toml) |
| source | — | [docker/localnode/Dockerfile](../../../docker/localnode/Dockerfile) |
| source | — | [docker/monitornode/config/grafana-dashboards.yaml](../../../docker/monitornode/config/grafana-dashboards.yaml) |
| source | — | [docker/monitornode/config/grafana.yaml](../../../docker/monitornode/config/grafana.yaml) |
| source | — | [docker/monitornode/config/prometheus.yaml](../../../docker/monitornode/config/prometheus.yaml) |
| source | — | [docker/monitornode/dashboards/cryptosim-dashboard.json](../../../docker/monitornode/dashboards/cryptosim-dashboard.json) |
| source | — | [docker/paxeer/Dockerfile](../../../docker/paxeer/Dockerfile) |
| source | — | [docker/paxeer/Dockerfile.paxd](../../../docker/paxeer/Dockerfile.paxd) |
| source | — | [docker/paxeer/Dockerfile.paxd-node](../../../docker/paxeer/Dockerfile.paxd-node) |
| source | — | [docker/platform-agent-boundary/Dockerfile](../../../docker/platform-agent-boundary/Dockerfile) |
| source | — | [docker/platform-authority/Dockerfile](../../../docker/platform-authority/Dockerfile) |
| source | — | [docker/platform-core/Dockerfile](../../../docker/platform-core/Dockerfile) |
| source | — | [docker/platform-dashboard/Dockerfile](../../../docker/platform-dashboard/Dockerfile) |
| source | — | [docker/platform-dashboard/files.tsv](../../../docker/platform-dashboard/files.tsv) |
| source | — | [docker/platform-dashboard-web/Dockerfile](../../../docker/platform-dashboard-web/Dockerfile) |
| source | — | [docker/platform-faucet/Dockerfile](../../../docker/platform-faucet/Dockerfile) |
| source | — | [docker/platform-gateway/Dockerfile](../../../docker/platform-gateway/Dockerfile) |
| source | — | [docker/platform-gateway/files.tsv](../../../docker/platform-gateway/files.tsv) |
| source | — | [docker/platform-identity/Dockerfile](../../../docker/platform-identity/Dockerfile) |
| source | — | [docker/platform-identity/files.tsv](../../../docker/platform-identity/files.tsv) |
| source | — | [docker/platform-indexer/Dockerfile](../../../docker/platform-indexer/Dockerfile) |
| source | — | [docker/platform-indexer/files.tsv](../../../docker/platform-indexer/files.tsv) |
| source | — | [docker/platform-internal/Dockerfile](../../../docker/platform-internal/Dockerfile) |
| source | — | [docker/platform-internal/files.tsv](../../../docker/platform-internal/files.tsv) |
| source | — | [docker/platform-registry-builder/Dockerfile](../../../docker/platform-registry-builder/Dockerfile) |
| source | — | [docker/platform-registry/Dockerfile](../../../docker/platform-registry/Dockerfile) |
| source | — | [docker/platform-registry/layerx-registry.service](../../../docker/platform-registry/layerx-registry.service) |
| source | — | [docker/platform-registry/registry.env.example](../../../docker/platform-registry/registry.env.example) |
| source | — | [docker/platform-testnet/Dockerfile](../../../docker/platform-testnet/Dockerfile) |
| source | — | [docker/platform-webhooks/Dockerfile](../../../docker/platform-webhooks/Dockerfile) |
| source | — | [docker/platform-webhooks/files.tsv](../../../docker/platform-webhooks/files.tsv) |
| source | — | [docker/ramps/Dockerfile](../../../docker/ramps/Dockerfile) |
| source | — | [docker/ramps/files.tsv](../../../docker/ramps/files.tsv) |
| source | — | [docker/redis/Dockerfile](../../../docker/redis/Dockerfile) |
| source | — | [docker/redis/files.tsv](../../../docker/redis/files.tsv) |
| source | — | [docker/rpcnode/config/app.toml](../../../docker/rpcnode/config/app.toml) |
| source | — | [docker/rpcnode/config/config.toml](../../../docker/rpcnode/config/config.toml) |
| source | — | [docker/rpcnode/Dockerfile](../../../docker/rpcnode/Dockerfile) |
| source | — | [docker/search-front/Dockerfile](../../../docker/search-front/Dockerfile) |
| source | — | [docker/search-front/files.tsv](../../../docker/search-front/files.tsv) |
| source | — | [docker/search-front/resolver.conf.template](../../../docker/search-front/resolver.conf.template) |
| source | — | [docker/wallet-attestor/Dockerfile](../../../docker/wallet-attestor/Dockerfile) |
| source | — | [docker/wallet-attestor/files.tsv](../../../docker/wallet-attestor/files.tsv) |
| source | — | [docker/wallet-gateway/Dockerfile](../../../docker/wallet-gateway/Dockerfile) |
| source | — | [docker/wallet-gateway/files.tsv](../../../docker/wallet-gateway/files.tsv) |
| source | — | [docker/wallet-pwa/Dockerfile](../../../docker/wallet-pwa/Dockerfile) |
| source | — | [docker/web/Dockerfile](../../../docker/web/Dockerfile) |
| source | — | [docker/x-websearch/Dockerfile](../../../docker/x-websearch/Dockerfile) |

## flyci-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `func main() {` | [tools/flyci/controller/main.go](../../../tools/flyci/controller/main.go) |
| `cli_command` | `--selfcheck)` | [tools/flyci/runner/entrypoint.sh](../../../tools/flyci/runner/entrypoint.sh) |
| `cli_command` | `case "qualification-client":` | [tools/flyci/controller/qualification_control.go](../../../tools/flyci/controller/qualification_control.go) |
| `cli_command` | `case "qualification-seal":` | [tools/flyci/controller/qualification_control.go](../../../tools/flyci/controller/qualification_control.go) |
| `cli_command` | `case "qualification-verify":` | [tools/flyci/controller/qualification_control.go](../../../tools/flyci/controller/qualification_control.go) |
| `cli_command` | `case "status", "export":` | [tools/flyci/controller/qualification_control.go](../../../tools/flyci/controller/qualification_control.go) |
| `cli_command` | `case "submit":` | [tools/flyci/controller/qualification_control.go](../../../tools/flyci/controller/qualification_control.go) |

## flyci-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `app = "paxeer-ci-controller"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `[build]` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `dockerfile = "docker/flyci-controller/Dockerfile"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `[env]` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `FLY_REGION = "iad"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `FLY_RUNNER_APP = "paxeer-ci-runners"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `GITHUB_REPOSITORY = "Sidiora-Labs/Paxeer-X-Network"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `QUALIFICATION_STATE_DIR = "/data/flyci"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `RUNNER_LABELS = "fly-linux"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `[[mounts]]` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `destination = "/data"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `source = "paxeer_ci_qualification"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `primary_region = "iad"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `[processes]` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `app = "/usr/local/bin/flyci-controller"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `[[vm]]` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `memory = "512mb"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `processes = ["app"]` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `size = "shared-cpu-1x"` | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| `config_key` | `app = "paxeer-ci-runners"` | [tools/flyci/runner/fly.toml](../../../tools/flyci/runner/fly.toml) |
| `config_key` | `[build]` | [tools/flyci/runner/fly.toml](../../../tools/flyci/runner/fly.toml) |
| `config_key` | `dockerfile = "docker/flyci-runner/Dockerfile"` | [tools/flyci/runner/fly.toml](../../../tools/flyci/runner/fly.toml) |
| `config_key` | `primary_region = "iad"` | [tools/flyci/runner/fly.toml](../../../tools/flyci/runner/fly.toml) |

## flyci-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/flyci/runner/entrypoint.sh](../../../tools/flyci/runner/entrypoint.sh) |

## flyci-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/flyci/controller/fly.toml](../../../tools/flyci/controller/fly.toml) |
| source | — | [tools/flyci/runner/fly.toml](../../../tools/flyci/runner/fly.toml) |

## hpx-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `help\|--help\|-h)       usage ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `version\|--version\|-v) prn "hpx v${VERSION}" ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `stake\|create)   cmd_validator_stake ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `keygen\|gen\|key) cmd_validator_keygen ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `status\|info)    cmd_validator_status ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `setup\|install)        flow_setup ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `logs)                 shift; cmd_logs "${1:-200}" ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `peers)                shift; cmd_peers "${1:-show}" "${2:-}" ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `refresh)` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `register)             cmd_register ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `remove\|uninstall)     cmd_remove ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `restart)              cmd_restart ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `show)` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `start)                cmd_start ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `statesync)            cmd_statesync ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `stop)                 cmd_stop ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `update\|upgrade)       cmd_update ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `validator\|val)        shift; cmd_validator "${1:-status}" ;;` | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| `cli_command` | `func main() {` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `cli_command` | `""\|help\|-h\|--help) usage ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| `cli_command` | `fund)         phase_fund ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| `cli_command` | `fund-wallet)  phase_fund_wallet ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| `cli_command` | `keygen)       phase_keygen ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| `cli_command` | `stake)        phase_stake ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| `cli_command` | `stake-cmds)   phase_stake_cmds ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| `cli_command` | `status)       phase_status ;;` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| source | — | [tools/hpx/fleet-keygen.sh](../../../tools/hpx/fleet-keygen.sh) |
| source | — | [tools/hpx/get-hpx.sh](../../../tools/hpx/get-hpx.sh) |
| source | — | [tools/hpx/hosting/deploy.sh](../../../tools/hpx/hosting/deploy.sh) |
| source | — | [tools/hpx/install.sh](../../../tools/hpx/install.sh) |
| source | — | [tools/hpx/publish.sh](../../../tools/hpx/publish.sh) |
| source | — | [tools/hpx/uninstall.sh](../../../tools/hpx/uninstall.sh) |

## hpx-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `app = "paxeer-hpx-registry"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[build]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `dockerfile = "../../../docker/hpx-registry/Dockerfile"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[deploy]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `strategy = "immediate"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[env]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `HPX_ARTIFACTS_DIR = "/srv/hpx/artifacts/current"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `HPX_CHAIN_ID = "hyperpax_125-1"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `HPX_DATA_DIR = "/srv/hpx/data"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[http_service]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `auto_start_machines = false` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `auto_stop_machines = false` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[[http_service.checks]]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `grace_period = "30s"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `interval = "15s"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `method = "GET"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `path = "/healthz"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `timeout = "10s"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[http_service.concurrency]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `hard_limit = 250` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `soft_limit = 200` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `type = "requests"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `force_https = true` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `internal_port = 8080` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `min_machines_running = 1` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `kill_signal = "SIGTERM"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `kill_timeout = "30s"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[[mounts]]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `destination = "/srv/hpx"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `initial_size = "5gb"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `snapshot_retention = 14` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `source = "hpx_data"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `primary_region = "ams"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `[[vm]]` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `cpu_kind = "shared"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `cpus = 2` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `memory = "1gb"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| `config_key` | `size = "shared-cpu-2x"` | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |

## hpx-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `HPX_REGISTER_TOKEN` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `env_var` | `HPX_SEED_PEERS` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `env_var` | `YES` | [tools/hpx/stake-fleet.sh](../../../tools/hpx/stake-fleet.sh) |
| source | — | [tools/hpx/fleet-keygen.sh](../../../tools/hpx/fleet-keygen.sh) |
| source | — | [tools/hpx/get-hpx.sh](../../../tools/hpx/get-hpx.sh) |
| source | — | [tools/hpx/hosting/deploy.sh](../../../tools/hpx/hosting/deploy.sh) |
| source | — | [tools/hpx/hpx](../../../tools/hpx/hpx) |
| source | — | [tools/hpx/install.sh](../../../tools/hpx/install.sh) |
| source | — | [tools/hpx/publish.sh](../../../tools/hpx/publish.sh) |
| source | — | [tools/hpx/uninstall.sh](../../../tools/hpx/uninstall.sh) |

## hpx-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/hpx/.ascii](../../../tools/hpx/.ascii) |
| source | — | [tools/hpx/hosting/fly.toml](../../../tools/hpx/hosting/fly.toml) |
| source | — | [tools/hpx/hosting/fonts.sha256](../../../tools/hpx/hosting/fonts.sha256) |
| source | — | [tools/hpx/hosting/index.html](../../../tools/hpx/hosting/index.html) |
| source | — | [tools/hpx/hosting/nginx.conf](../../../tools/hpx/hosting/nginx.conf) |
| source | — | [tools/hpx/hosting/nginx-http.conf](../../../tools/hpx/hosting/nginx-http.conf) |
| source | — | [tools/hpx/hosting/nginx-registry.conf.template](../../../tools/hpx/hosting/nginx-registry.conf.template) |
| source | — | [tools/hpx/registry/go.mod](../../../tools/hpx/registry/go.mod) |

## hpx-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `mux.HandleFunc("/api/myip", s.handleMyIP)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.HandleFunc("/api/nodes", s.handleNodes)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.HandleFunc("/api/peers", s.handlePeers)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.HandleFunc("/api/peers.txt", s.handlePeersTxt)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.HandleFunc("/api/register", s.handleRegister)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.HandleFunc("/api/statesync", s.handleStateSync)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.HandleFunc("/healthz", s.handleHealth)` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |
| `http_route` | `mux.Handle("/", artifactHandler(cfg.ArtifactsDir))` | [tools/hpx/registry/main.go](../../../tools/hpx/registry/main.go) |

## jev-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `from __future__ import annotations` | [tools/jev/checks/deps.py](../../../tools/jev/checks/deps.py) |
| `cli_command` | `parser.add_argument("--repo-root", type=Path, default=REPO_ROOT)` | [tools/jev/checks/deps.py](../../../tools/jev/checks/deps.py) |
| `cli_command` | `parser.add_argument("--run", action="store_true")` | [tools/jev/checks/deps.py](../../../tools/jev/checks/deps.py) |
| `cli_command` | `parser.add_argument("--workspace", action="append", choices=WORKSPACES)` | [tools/jev/checks/deps.py](../../../tools/jev/checks/deps.py) |
| `cli_command` | `from __future__ import annotations` | [tools/jev/checks/docs.py](../../../tools/jev/checks/docs.py) |
| `cli_command` | `parser.add_argument("--source", default=DEFAULT_SOURCE)` | [tools/jev/checks/docs.py](../../../tools/jev/checks/docs.py) |
| `cli_command` | `from __future__ import annotations` | [tools/jev/checks/failures.py](../../../tools/jev/checks/failures.py) |
| `cli_command` | `parser.add_argument("--format", choices=FORMAT_CHOICES, default="auto")` | [tools/jev/checks/failures.py](../../../tools/jev/checks/failures.py) |
| `cli_command` | `parser.add_argument("--log-file", action="append", required=True, type=Path)` | [tools/jev/checks/failures.py](../../../tools/jev/checks/failures.py) |
| `cli_command` | `from __future__ import annotations` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `parser.add_argument("--dedup-only", action="store_true")` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `parser.add_argument("--kvx", action="append", default=[], type=Path)` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `parser.add_argument("--max-pairs", type=int, default=DEFAULT_MAX_PAIRS)` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `parser.add_argument("--sample", type=int, default=0)` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `parser.add_argument("--seed", type=int, default=0)` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `parser.add_argument("--severity-only", action="store_true")` | [tools/jev/checks/ledger.py](../../../tools/jev/checks/ledger.py) |
| `cli_command` | `from __future__ import annotations` | [tools/jev/checks/pr.py](../../../tools/jev/checks/pr.py) |
| `cli_command` | `parser.add_argument("--base", required=True)` | [tools/jev/checks/pr.py](../../../tools/jev/checks/pr.py) |
| `cli_command` | `parser.add_argument("--body", type=Path)` | [tools/jev/checks/pr.py](../../../tools/jev/checks/pr.py) |
| `cli_command` | `parser.add_argument("--comment", type=Path)` | [tools/jev/checks/pr.py](../../../tools/jev/checks/pr.py) |
| `cli_command` | `parser.add_argument("--commits", action="store_true")` | [tools/jev/checks/pr.py](../../../tools/jev/checks/pr.py) |
| `cli_command` | `parser.add_argument("--head", default="HEAD")` | [tools/jev/checks/pr.py](../../../tools/jev/checks/pr.py) |
| `cli_command` | `parser = argparse.ArgumentParser(prog=f"python3 -m tools.jev {name}")` | [tools/jev/cli.py](../../../tools/jev/cli.py) |
| `cli_command` | `parser.add_argument("--dry-run", action="store_true")` | [tools/jev/cli.py](../../../tools/jev/cli.py) |
| `cli_command` | `parser.add_argument("--log", type=Path)` | [tools/jev/cli.py](../../../tools/jev/cli.py) |
| `cli_command` | `parser.add_argument("--model")` | [tools/jev/cli.py](../../../tools/jev/cli.py) |
| `cli_command` | `parser.add_argument("--out", required=True, type=Path)` | [tools/jev/cli.py](../../../tools/jev/cli.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/jev/__main__.py](../../../tools/jev/__main__.py) |
| `cli_command` | `raise SystemExit(main(sys.argv[1:]))` | [tools/jev/__main__.py](../../../tools/jev/__main__.py) |

## jev-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/jev/tests/fixtures/decision_response.json](../../../tools/jev/tests/fixtures/decision_response.json) |

## makefile-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [Makefile](../../../Makefile) |
| source | — | [chain.mk](../../../chain.mk) |
| source | — | [tools/paxeer-x/build/104.31.2.mk](../../../tools/paxeer-x/build/104.31.2.mk) |
| source | — | [tools/paxeer-x/build/104.32.1.mk](../../../tools/paxeer-x/build/104.32.1.mk) |
| source | — | [tools/paxeer-x/build/104.35.11.mk](../../../tools/paxeer-x/build/104.35.11.mk) |
| source | — | [tools/paxeer-x/build/104.35.7.mk](../../../tools/paxeer-x/build/104.35.7.mk) |
| source | — | [tools/paxeer-x/build/6.6.mk](../../../tools/paxeer-x/build/6.6.mk) |

## makefile-targets

| Kind | Declaration | Source |
|---|---|---|
| `makefile_target` | `agent-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-check: agent-check-boundary agent-check-secrets agent-test-boundary` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-check-boundary:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-check-secrets:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-fuzz: agent-fuzz-all` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-fuzz-wire agent-fuzz-interface agent-fuzz-all:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-fuzz-long agent-fuzz-wire-long:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-fuzz-minimize agent-fuzz-wire-minimize:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-lint:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-qualify-fabrication:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-qualify-fuzz:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-sdk-generate:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test agent-test-sanitize: agent-test-native-prerequisites` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-admission:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-approval:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-attenuation:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-audit-chain:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-audit-coverage:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-audit-export:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-authority:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-availability:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-balance:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-budget-create:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-budget-divergence:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-budget-reconcile:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-budget-reserve:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-budget-unknown:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-cache:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-capability:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-ceiling:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-checkpoint:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-config:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-deadlines:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-degraded:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-delivery:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-disclosure:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-enforcement-report:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-export:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-finality:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-gaps:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-handshake-gate:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-history:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-idempotency:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-identity:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-ingest:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-migration:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-narrowing:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-observability:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-operator:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-outbox:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-policy:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-policy-dryrun:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-policy-version:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-prepare:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-prepare-expiry:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-quota:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-ratelimit:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-receipts:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-recovery:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-redaction:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-revocation:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-session:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-signature-binding:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-signing:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-store:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-subscription:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-tenant-isolation:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-tenant-leakage:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-tenant-resolve:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-tenant-store:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-unknown:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-agentd-webhook:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-approvals:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-capability-report: agent-test-boundary` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-availability:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-connection:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-reads:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-receipt:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-stream:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-submit:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-client-submit-focused:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-contract-errors:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-contract-identity:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-contract-read:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-contract-schema:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-contract-stream:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-contract-write:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-crypto-disclosure:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-crypto-keystore: test-grants` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-crypto-remote:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-crypto-signer:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-crypto-verify: test-crypto-ed25519 test-crypto-secp256k1` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-errors:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-isolation: agent-test-policy-adversarial agent-test-mcp-untrusted-input` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-limits-exactly-once:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-approval:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-injection:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-read:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-readonly:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-scope:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-untrusted-input:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-mcp-write:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-native-prerequisites:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-policy-adversarial:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-proof-availability:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-proof-checkpoint:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-proof-inclusion:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-proof-levels:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-proof-merkle:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-proof-receipt:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-sdk-compat:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-sdk-generate:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-sdk-py:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-sdk-rust:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-sdk-ts:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-types-activity:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-types-ids:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-types-receipt:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-types-verification:` | [Makefile](../../../Makefile) |
| `makefile_target` | `agent-test-vectors:` | [Makefile](../../../Makefile) |
| `makefile_target` | `all: build` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(PAXEER_FORWARDED_TARGETS):` | [Makefile](../../../Makefile) |
| `makefile_target` | `beta-contract-check:` | [Makefile](../../../Makefile) |
| `makefile_target` | `beta-driver-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `beta-ledger-check:` | [Makefile](../../../Makefile) |
| `makefile_target` | `beta-qualify:` | [Makefile](../../../Makefile) |
| `makefile_target` | `beta-qualify-focused:` | [Makefile](../../../Makefile) |
| `makefile_target` | `beta-report:` | [Makefile](../../../Makefile) |
| `makefile_target` | `bridge-test-evm:` | [Makefile](../../../Makefile) |
| `makefile_target` | `bridge-test-solana:` | [Makefile](../../../Makefile) |
| `makefile_target` | `build: $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/obj/fuzz/fuzz_gateway_json.o: tests/fuzz/fuzz_gateway_json.c` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/obj/%.o: %.c` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/test-obj/%.o: %.c` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/explorer_fixture: tests/explorer_fixture.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_arena: tests/protocol/lxp_test_arena.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_arith_property: $(ARITH_PROPERTY_SOURCES) $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_ct: tests/crypto/lxp_test_ct.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_grants: tests/state/lxp_test_grants.c $(LIBRARY) $(PROGRAMS_RUNTIME_LIB) \| programs-build` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_hash: tests/crypto/lxp_test_hash.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_identity: tests/state/lxp_test_identity.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_journal: tests/state/lxp_test_journal.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_log: tests/storage/lxp_test_log.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_merkle: tests/crypto/lxp_test_merkle.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_merkle_proof: tests/crypto/lxp_test_merkle_proof.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_merkle_vector: tests/crypto/lxp_test_merkle_vector.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_nofloat: tests/arith/lxp_test_nofloat.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_owner_rotation_unit: tests/protocol/lxp_test_owner_rotation.c $(LIBRARY) $(PROGRAMS_RUNTIME_LIB) \| programs-build` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_protocol: tests/protocol/lxp_test_protocol.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_result: tests/protocol/lxp_test_result.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_rotation: tests/state/lxp_test_rotation.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_rounding: tests/arith/lxp_test_rounding.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_secp256k1: tests/crypto/lxp_test_secp256k1.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_u128: tests/arith/lxp_test_u128.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/lxp_test_u256: tests/arith/lxp_test_u256.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/test_account_id: tests/ledger/test_account_id.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/test_batch_header: tests/test_batch_header.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/test_call_builders: tests/programs/test_call_builders.c src/modules/programs/call.c $(LIBRARY) $(PROGRAMS_RUNTIME_LIB)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tests/test_qual_arith: $(QUAL_ARITH_SOURCES)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tools/log_inspect: tools/log_inspect.c $(LIBRARY)` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BUILD_DIR)/tools/lxp-reserve-report: tools/lxp_reserve_report.c` | [Makefile](../../../Makefile) |
| `makefile_target` | `all: lint install` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `autobahn-integration-test:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build: paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-docker-node:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-integration-ci-artifacts: build-rpc-node build-paxd-in-localnode` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-linux:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-loadtest:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-paxd-in-localnode: build-docker-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-paxd-in-localnode-ci: ensure-integration-ci-images` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-rocksdb: rocksdb-source-check` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-rpc-node:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `build-verbose: paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `$(BUILDDIR):` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `$(BUILDDIR)/packages.txt:$(GO_TEST_FILES) $(BUILDDIR)` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `ci: lint test` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `clean:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `dblint:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-start: docker-cluster-stop build-docker-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-start-ci: docker-cluster-stop ensure-integration-ci-images` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-start-giga-mixed: docker-cluster-stop build-docker-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-start-monitoring: docker-cluster-stop-monitoring build-docker-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-start-skipbuild: docker-cluster-stop build-docker-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-stop:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `docker-cluster-stop-monitoring:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `ensure-integration-ci-images:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `giga-integration-test:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `giga-mixed-integration-test:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `go.sum: go.mod` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `install: go.sum paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `install-bench: go.sum paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `install-mock-balances: go.sum paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `install-rocksdb: go.sum paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `install-with-race-detector: go.sum paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `kill-pax-node:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `kill-rpc-node:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `lint: paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `loadtest: go.sum` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `paxd-native-digest-check:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `paxd-native-libs:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `rocksdb-source-check:` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `run-local-node: kill-pax-node build-docker-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `run-rpc-node: build-rpc-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `run-rpc-node-integration-ci: kill-rpc-node ensure-integration-ci-images` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `run-rpc-node-skipbuild: build-rpc-node` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `split-test-packages:$(BUILDDIR)/packages.txt` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `test: test-group-0 test-group-1 test-group-2 test-group-3` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `test-group-%:split-test-packages paxd-native-libs` | [chain.mk](../../../chain.mk) |
| `makefile_target` | `check: test` | [Makefile](../../../Makefile) |
| `makefile_target` | `ci: public-audit test reproducible scan-consensus test-sanitizers` | [Makefile](../../../Makefile) |
| `makefile_target` | `clean:` | [Makefile](../../../Makefile) |
| `makefile_target` | `developer-dashboard-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `developer-dashboard-install:` | [Makefile](../../../Makefile) |
| `makefile_target` | `developer-dashboard-static-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `fuzz-codec-smoke: $(BUILD_DIR)/tests/lxp_fuzz_codec` | [Makefile](../../../Makefile) |
| `makefile_target` | `hpx-public-check:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-check:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-check-bundle:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-check-ui:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-e2e: human-test-e2e` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-e2e-explorer:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-e2e-foundation:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-e2e-journeys:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-e2e-perf:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-e2e-settings:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-fuzz-intents:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-gen-api:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-js-install:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-lint: human-lint-copy` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-lint-copy:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-qualify:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-qualify-fabrication:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-qualify-faults:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-qualify-journeys:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-qualify-perf:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-activity:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-agents: test-rotation` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-approvals:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-component:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-custody-prerequisites:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-e2e:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-e2e-long:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-fault:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-hosted-provisioning:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test human-test-integration: human-test-custody-prerequisites` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-intents:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-journey:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-journeys:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-notify:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-property:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-provider-probe:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test human-test-unit human-test-integration human-test-service: human-test-webhooks` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-visual:` | [Makefile](../../../Makefile) |
| `makefile_target` | `human-test-webhooks:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-build-ramps:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-lint:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-mandates:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-migration:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-migration-testnets:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-mirrors:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-portable:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-ramps:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-ramps-sandbox:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-ucp:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-visa-tap:` | [Makefile](../../../Makefile) |
| `makefile_target` | `interop-test-x402:` | [Makefile](../../../Makefile) |
| `makefile_target` | `jev-deps:` | [Makefile](../../../Makefile) |
| `makefile_target` | `jev-docs:` | [Makefile](../../../Makefile) |
| `makefile_target` | `jev-failures:` | [Makefile](../../../Makefile) |
| `makefile_target` | `jev-ledger:` | [Makefile](../../../Makefile) |
| `makefile_target` | `jev-pr:` | [Makefile](../../../Makefile) |
| `makefile_target` | `jev-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `latency-wave-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `layerx-genesis-build: $(BUILD_DIR)/bin/layerx-genesis-build` | [Makefile](../../../Makefile) |
| `makefile_target` | `layerx-module-registry: $(BUILD_DIR)/bin/layerx-module-registry` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(LIB_OBJECTS) $(TEST_LIB_OBJECTS): Makefile` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(LIBRARY): $(LIB_OBJECTS)` | [Makefile](../../../Makefile) |
| `makefile_target` | `list-tests: $(BUILD_DIR)/tests/lxp_test_harness` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(BINDINGS_DIR)/%.go: $(BUILD_DIR)/%.abi $(BUILD_DIR)/%.bin \| $(BINDINGS_DIR)` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(BUILD_DIR):` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `build-cli: \| $(BUILD_DIR)` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `check-tools:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `clean:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(BINDINGS_DIR):` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(SCENARIOS_DIR):` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `help:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `install: build-cli` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `install-tools: setup-node` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `lint:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(SCENARIOS_DIR)/%.go: \| $(SCENARIOS_DIR)` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `setup-node:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `test:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `mirror-live:` | [Makefile](../../../Makefile) |
| `makefile_target` | `mirror-verify-live:` | [Makefile](../../../Makefile) |
| `makefile_target` | `monorepo-ci: workspace-ci` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxai-host-boundary-build: $(BUILD_DIR)/tests/paxai_host_boundary` | [Makefile](../../../Makefile) |
| `makefile_target` | `build:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `clean:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `fmt:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `generate:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `install:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `lint:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `run:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `test:` | [tools/paxctl/Makefile](../../../tools/paxctl/Makefile) |
| `makefile_target` | `paxd-build: paxd-native-libs` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxd-release:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxd-version-check:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-ci:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-hardhat-compilers-ready: paxeer-npm-dependencies-ready` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-lint:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-manifest-build: workspace-inventory-check paxeer-npm-build` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-manifest-install: workspace-inventory-check paxeer-npm-install paxeer-tools-install` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-manifest-lint: workspace-inventory-check paxeer-npm-static-test` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-manifest-test: workspace-inventory-check paxeer-npm-static-test` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-node-preflight:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-npm-build: paxeer-hardhat-compilers-ready` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-npm-dependencies-ready:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-npm-install: paxeer-node-preflight` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-npm-static-test: paxeer-npm-dependencies-ready` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-tools-install:` | [Makefile](../../../Makefile) |
| `makefile_target` | `platform-qualify:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-abi-drift:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-adversarial:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-bench: programs-interpreter-bench` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-check-capability-fixture:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-check-executed-v4-fixture: programs-build` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-check-native-lifecycle-fixtures: $(BUILD_DIR)/tests/programs_call_activity` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-conservation:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-differential: $(BUILD_DIR)/tests/programs_parallel_differential` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-executed-v4-fixture: programs-build` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-fuzz-smoke:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-generate-capability-fixture:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-interpreter-bench:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-interpreter-conformance:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-js-install:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-lint: programs-module-boundaries` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-module-boundaries:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-native-lifecycle-fixtures: $(BUILD_DIR)/tests/programs_call_activity` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-porting-v2-references: $(BUILD_DIR)/tests/programs_call_activity` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-qualify:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-quickstart:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-reference-escrow:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-reference-fixtures:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-reference-web-reader:` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(PROGRAMS_RELEASE_RUNTIME_LIB) programs-release-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-sdk-assemblyscript:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-sdk-c:` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-sdk-rust:` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(PROGRAMS_RUNTIME_LIB):` | [Makefile](../../../Makefile) |
| `makefile_target` | `programs-test: programs-module-boundaries programs-abi-drift programs-core-test programs-protocol-regression programs-adversarial programs-fuzz-smoke programs-differential programs-interpreter-conformance programs-sdk-rust programs-sdk-c programs-sdk-assemblyscript programs-porting-v2-references` | [Makefile](../../../Makefile) |
| `makefile_target` | `public-audit:` | [Makefile](../../../Makefile) |
| `makefile_target` | `public-tls-test-prerequisites:` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-x-build-104.30.2: $(PX_104_30_2_MANIFEST)` | [tools/paxeer-x/build/104.30.2.mk](../../../tools/paxeer-x/build/104.30.2.mk) |
| `makefile_target` | `$(PX_104_30_2_MANIFEST): $(PX_104_30_2_MONETARY) $(PX_104_30_2_SANDBOX_LIB) $(PX_104_30_2_RUST_EXES) $(PX_104_30_2_INPUTS)` | [tools/paxeer-x/build/104.30.2.mk](../../../tools/paxeer-x/build/104.30.2.mk) |
| `makefile_target` | `$(PX_104_30_2_MONETARY): $(PX_104_30_2_NATIVE_INPUTS) $(TEST_LIBRARY) $(PX_104_30_2_SANDBOX_LIB)` | [tools/paxeer-x/build/104.30.2.mk](../../../tools/paxeer-x/build/104.30.2.mk) |
| `makefile_target` | `$(PX_104_30_2_RUST_EXES): $(PX_104_30_2_RUST_INPUTS)` | [tools/paxeer-x/build/104.30.2.mk](../../../tools/paxeer-x/build/104.30.2.mk) |
| `makefile_target` | `$(PX_104_30_2_SANDBOX_LIB): $(PX_104_30_2_RUST_INPUTS)` | [tools/paxeer-x/build/104.30.2.mk](../../../tools/paxeer-x/build/104.30.2.mk) |
| `makefile_target` | `paxeer-x-build-104.31.2:` | [tools/paxeer-x/build/104.31.2.mk](../../../tools/paxeer-x/build/104.31.2.mk) |
| `makefile_target` | `paxeer-x-build-104.31.6:` | [tools/paxeer-x/build/104.31.6.mk](../../../tools/paxeer-x/build/104.31.6.mk) |
| `makefile_target` | `paxeer-x-build-104.32.1:` | [tools/paxeer-x/build/104.32.1.mk](../../../tools/paxeer-x/build/104.32.1.mk) |
| `makefile_target` | `paxeer-x-build-104.32.4:` | [tools/paxeer-x/build/104.32.4.mk](../../../tools/paxeer-x/build/104.32.4.mk) |
| `makefile_target` | `paxeer-x-native-104.32.4: $(LIBRARY) $(CHECKPOINT_SETTLEMENT_HEADER)` | [tools/paxeer-x/build/104.32.4.mk](../../../tools/paxeer-x/build/104.32.4.mk) |
| `makefile_target` | `paxeer-x-arbiter-prestate-build:` | [tools/paxeer-x/build/104.35.11.mk](../../../tools/paxeer-x/build/104.35.11.mk) |
| `makefile_target` | `paxeer-x-single-step-conformance-build:` | [tools/paxeer-x/build/104.35.2.mk](../../../tools/paxeer-x/build/104.35.2.mk) |
| `makefile_target` | `paxeer-x-arbiter-native-authority-runtime:` | [tools/paxeer-x/build/104.35.35.mk](../../../tools/paxeer-x/build/104.35.35.mk) |
| `makefile_target` | `paxeer-x-attestation-begin:` | [tools/paxeer-x/build/104.35.7.mk](../../../tools/paxeer-x/build/104.35.7.mk) |
| `makefile_target` | `paxeer-x-build-104.35.7: paxeer-x-attestation-begin $(LIBRARY)` | [tools/paxeer-x/build/104.35.7.mk](../../../tools/paxeer-x/build/104.35.7.mk) |
| `makefile_target` | `paxeer-x-build-104.38.9:` | [tools/paxeer-x/build/104.38.9.mk](../../../tools/paxeer-x/build/104.38.9.mk) |
| `makefile_target` | `paxeer-x-build-108.1.10:` | [tools/paxeer-x/build/108.1.10.mk](../../../tools/paxeer-x/build/108.1.10.mk) |
| `makefile_target` | `paxeer-x-profile2-accounts-build:` | [tools/paxeer-x/build/6.6.mk](../../../tools/paxeer-x/build/6.6.mk) |
| `makefile_target` | `paxeer-x-profile2-native: $(LAYERXD_OBJECTS) $(LIBRARY)` | [tools/paxeer-x/build/6.6.mk](../../../tools/paxeer-x/build/6.6.mk) |
| `makefile_target` | `$(BUILD_DIR)/tests/programs_execution_context: tests/programs/test_execution_context.c tests/programs/test_call_activity.c $(LIBRARY) $(CHECKPOINT_SETTLEMENT_HEADER)` | [tools/qualification/paxeer-x/program_execution_context.mk](../../../tools/qualification/paxeer-x/program_execution_context.mk) |
| `makefile_target` | `paxeer-x-build-104.31.1:` | [tools/qualification/paxeer-x/program_execution_context.mk](../../../tools/qualification/paxeer-x/program_execution_context.mk) |
| `makefile_target` | `paxeer-x-native-104.31.1: $(BUILD_DIR)/tests/programs_execution_context` | [tools/qualification/paxeer-x/program_execution_context.mk](../../../tools/qualification/paxeer-x/program_execution_context.mk) |
| `makefile_target` | `$(BUILD_DIR)/tests/programs_spend_capability_composition: tests/programs/test_spend_capability_composition.c tests/programs/test_call_activity.c $(LIBRARY) $(CHECKPOINT_SETTLEMENT_HEADER)` | [tools/qualification/paxeer-x/program_spend_composition.mk](../../../tools/qualification/paxeer-x/program_spend_composition.mk) |
| `makefile_target` | `paxeer-x-build-104.30.4:` | [tools/qualification/paxeer-x/program_spend_composition.mk](../../../tools/qualification/paxeer-x/program_spend_composition.mk) |
| `makefile_target` | `paxeer-x-native-104.30.4: $(BUILD_DIR)/tests/programs_spend_capability_composition` | [tools/qualification/paxeer-x/program_spend_composition.mk](../../../tools/qualification/paxeer-x/program_spend_composition.mk) |
| `makefile_target` | `qualification-runner-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `qualify-faults: $(BUILD_DIR)/tests/test_qual_recovery` | [Makefile](../../../Makefile) |
| `makefile_target` | `qualify-fuzz: qualify-fuzz-run` | [Makefile](../../../Makefile) |
| `makefile_target` | `qualify-fuzz-run: $(BUILD_DIR)/tests/test_fuzz_smoke` | [Makefile](../../../Makefile) |
| `makefile_target` | `qualify-replay:` | [Makefile](../../../Makefile) |
| `makefile_target` | `reproducible:` | [Makefile](../../../Makefile) |
| `makefile_target` | `build-asan:` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `build-tsan:` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `build-ubsan:` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `test-asan:` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `test-sanitizers: test-asan test-ubsan test-tsan` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `test-tsan:` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `test-ubsan:` | [tools/build/sanitizers.mk](../../../tools/build/sanitizers.mk) |
| `makefile_target` | `scan-consensus: build` | [Makefile](../../../Makefile) |
| `makefile_target` | `paxeer-x-build-104.38.10:` | [scripts/build/paxeer-x/104.38.10.mk](../../../scripts/build/paxeer-x/104.38.10.mk) |
| `makefile_target` | `specgen-build:` | [Makefile](../../../Makefile) |
| `makefile_target` | `specgen-lint:` | [Makefile](../../../Makefile) |
| `makefile_target` | `specgen-test:` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-activity-codec: $(BUILD_DIR)/tests/lxp_test_activity_codec` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-admission: $(BUILD_DIR)/tests/lxp_test_admission` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-allowance: $(BUILD_DIR)/tests/lxp_test_allowance` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-arith-nofloat: build $(BUILD_DIR)/tests/lxp_test_nofloat` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-arith-rounding: $(BUILD_DIR)/tests/lxp_test_rounding` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-account-grant: $(BUILD_DIR)/tests/test_asset_account_grant` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-balance: $(BUILD_DIR)/tests/test_asset_state` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-deposit: $(BUILD_DIR)/tests/test_asset_deposit` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-registry: $(BUILD_DIR)/tests/test_asset_registry` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-reserve: $(BUILD_DIR)/tests/test_asset_reserve` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-transfer: $(BUILD_DIR)/tests/test_asset_transfer` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-withdraw: $(BUILD_DIR)/tests/test_asset_withdraw` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-asset-withdraw-activity: $(BUILD_DIR)/tests/test_asset_withdraw_activity` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-authority-multisig: $(BUILD_DIR)/tests/test_authority_multisig` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-authority-resolve: $(BUILD_DIR)/tests/lxp_test_authority_resolve` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-batch: $(BUILD_DIR)/tests/test_batch_header` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-bridge-deposit: $(BUILD_DIR)/tests/test_bridge_deposit test-contracts` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-bridge-withdraw: $(BUILD_DIR)/tests/test_bridge_withdraw test-contracts` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-budget-create: $(BUILD_DIR)/tests/test_budget_create` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-budget-delegate: $(BUILD_DIR)/tests/test_budget_delegate` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-budget-revoke: $(BUILD_DIR)/tests/test_budget_close` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-budget-spend: $(BUILD_DIR)/tests/test_budget_spend` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-codec: $(BUILD_DIR)/tests/lxp_test_codec_primitives` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-codec-limits: $(BUILD_DIR)/tests/lxp_test_codec_composite` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-codec-vectors: $(BUILD_DIR)/tests/lxp_test_codec_vectors` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-codec-version: $(BUILD_DIR)/tests/lxp_test_codec_strict` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-contract-state-surface:` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-contracts: test-contract-state-surface` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-crypto-ct: $(BUILD_DIR)/tests/lxp_test_ct` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-crypto-ed25519: $(BUILD_DIR)/tests/lxp_test_ed25519` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-crypto-hash: $(BUILD_DIR)/tests/lxp_test_hash` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-crypto-sanitizers:` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-crypto-secp256k1: $(BUILD_DIR)/tests/lxp_test_secp256k1` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-dispatch: $(BUILD_DIR)/tests/lxp_test_dispatch` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-envelope: $(BUILD_DIR)/tests/lxp_test_envelope` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-escrow-capture: $(BUILD_DIR)/tests/test_escrow_capture` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-escrow-dispute: $(BUILD_DIR)/tests/test_escrow_dispute` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-escrow-invariants: $(BUILD_DIR)/tests/test_escrow_invariants` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-escrow-open: $(BUILD_DIR)/tests/test_escrow_open` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-fee-gate: $(BUILD_DIR)/tests/lxp_test_fee_gate` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-fee-replay: $(BUILD_DIR)/tests/test_fee_replay test-dispatch` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-fees: $(BUILD_DIR)/tests/test_fees $(BUILD_DIR)/tests/test_fees_v3 $(BUILD_DIR)/tests/test_fees_v4` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-gateway-kv: $(BUILD_DIR)/tests/test_gateway_kv` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-gateway-send: $(BUILD_DIR)/tests/test_gateway_send` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-genesis: test-genesis-bootstrap` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-genesis-import: $(BUILD_DIR)/tests/test_genesis_import` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-genesis-module-table: $(BUILD_DIR)/tests/test_genesis_module_table` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-genesis-reconcile: $(BUILD_DIR)/tests/test_genesis_reconcile` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-governance: $(BUILD_DIR)/tests/test_governance_params` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-governance-activation: $(BUILD_DIR)/tests/test_governance_activation` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-governance-emergency: $(BUILD_DIR)/tests/test_governance_emergency` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-grants: $(BUILD_DIR)/tests/lxp_test_grants` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-group-%:` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-harness: $(BUILD_DIR)/tests/lxp_test_harness` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-identity: $(BUILD_DIR)/tests/lxp_test_identity` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-kernel-symbol-isolation: $(BUILD_DIR)/obj/src/protocol/lxp_kernel.o` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-accounts: $(BUILD_DIR)/tests/test_account_id` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-receipt: $(BUILD_DIR)/tests/test_receipt` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-receive: $(BUILD_DIR)/tests/test_receive` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-send: $(BUILD_DIR)/tests/test_send` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-send-allowance: $(BUILD_DIR)/tests/test_send_allowance` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-set: $(BUILD_DIR)/tests/test_transfer_set` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-ledger-transfer: $(BUILD_DIR)/tests/test_apply_transfer` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-legacy-readonly: $(BUILD_DIR)/tests/test_legacy_readonly` | [Makefile](../../../Makefile) |
| `makefile_target` | `$(TEST_LIBRARY): $(TEST_LIB_OBJECTS)` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-log: $(BUILD_DIR)/tests/lxp_test_log` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-log-durability: $(BUILD_DIR)/tests/lxp_test_log_durability` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-lxphost: paxd-native-libs` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-merkle: $(BUILD_DIR)/tests/lxp_test_merkle` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-merkle-proof: $(BUILD_DIR)/tests/lxp_test_merkle_proof` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-merkle-vector: $(BUILD_DIR)/tests/lxp_test_merkle_vector` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-metering: $(BUILD_DIR)/tests/test_metering` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-module-ctx: $(BUILD_DIR)/tests/lxp_test_module_ctx` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-module-registry: layerx-module-registry` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-native-withdrawal-proof: $(BUILD_DIR)/tests/test_asset_withdraw_activity` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-oracle-bounds: $(BUILD_DIR)/tests/test_oracle_checks` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-oracle-failclosed: $(BUILD_DIR)/tests/test_oracle_halt` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-oracle-intake: $(BUILD_DIR)/tests/test_oracle_intake` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-owner-rotation: $(BUILD_DIR)/tests/lxp_test_owner_rotation_unit` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-paxai-blob-lifecycle: $(BUILD_DIR)/tests/lxp_test_blob_lifecycle` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-paxai-native-gaps: $(BUILD_DIR)/tests/lxp_test_paxai_native_gaps` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-paxd-domain: paxd-native-libs` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-perps-book: $(BUILD_DIR)/tests/test_perps_book` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-perps-funding: $(BUILD_DIR)/tests/test_perps_funding` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-perps-insurance: $(BUILD_DIR)/tests/test_perps_insurance` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-perps-liquidation: $(BUILD_DIR)/tests/test_perps_liquidate` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-perps-margin: $(BUILD_DIR)/tests/test_perps_position` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-perps-market: $(BUILD_DIR)/tests/test_perps_market` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-program-call-builders: $(BUILD_DIR)/tests/test_call_builders` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-programs-abi5-admission: $(BUILD_DIR)/tests/test_programs_abi5_admission` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-programs-metered-call: $(BUILD_DIR)/tests/programs_metered_call` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-programs-oracle-read: $(BUILD_DIR)/tests/test_programs_oracle_read` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-programs-web-read: $(BUILD_DIR)/tests/test_programs_web_read` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-projection: $(BUILD_DIR)/tests/lxp_test_projection build` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-protocol: $(BUILD_DIR)/tests/lxp_test_protocol` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-rebuild: $(BUILD_DIR)/tests/lxp_test_rebuild $(BUILD_DIR)/tools/log_inspect` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-receipts: $(BUILD_DIR)/tests/lxp_test_receipts` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-recovery: $(BUILD_DIR)/tests/lxp_test_recovery` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-result: $(BUILD_DIR)/tests/lxp_test_result` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-revocation: $(BUILD_DIR)/tests/lxp_test_revocation` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-rotation: $(BUILD_DIR)/tests/lxp_test_rotation` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-sanitizer-smoke: $(BUILD_DIR)/tests/lxp_test_sanitizer_smoke` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-service-attest: $(BUILD_DIR)/tests/test_service_attest` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-service-commit: $(BUILD_DIR)/tests/test_service_commit` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-service-deliver: $(BUILD_DIR)/tests/test_service_deliver` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-service-dispute: $(BUILD_DIR)/tests/test_service_dispute` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-service-offer: $(BUILD_DIR)/tests/test_service_offer` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-shadow: $(BUILD_DIR)/tests/test_shadow_replay` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-snapshot: $(BUILD_DIR)/tests/test_snapshot` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-spot-book: $(BUILD_DIR)/tests/test_spot_book` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-state-commitment-transition: $(BUILD_DIR)/tests/test_state_commitment_transition` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-state-proof: $(BUILD_DIR)/tests/lxp_test_state_proof` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-state-root: $(BUILD_DIR)/tests/lxp_test_state_root` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-stream-accrual: $(BUILD_DIR)/tests/test_stream_accrue` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-stream-lifecycle: $(BUILD_DIR)/tests/test_stream_lifecycle` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-stream-meter: $(BUILD_DIR)/tests/test_stream_meter` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-stream-open: $(BUILD_DIR)/tests/test_stream_open` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-stream-settle: $(BUILD_DIR)/tests/test_stream_settle` | [Makefile](../../../Makefile) |
| `makefile_target` | `test-tests-python:` | [Makefile](../../../Makefile) |
| `makefile_target` | `workspace-ci: public-audit workspace-inventory-check workspace-build workspace-test workspace-lint` | [Makefile](../../../Makefile) |
| `makefile_target` | `workspace-install:` | [Makefile](../../../Makefile) |
| `makefile_target` | `workspace-inventory-check:` | [Makefile](../../../Makefile) |
| `makefile_target` | `workspace-node-preflight:` | [Makefile](../../../Makefile) |
| source | — | [tools/paxeer-x/build/104.35.29.mk](../../../tools/paxeer-x/build/104.35.29.mk) |

## ops-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `echo "usage: install.sh server [hosts.env] \| install.sh node" >&2` | [tools/ops/monitoring/install.sh](../../../tools/ops/monitoring/install.sh) |
| `cli_command` | `node)` | [tools/ops/monitoring/install.sh](../../../tools/ops/monitoring/install.sh) |
| `cli_command` | `server)` | [tools/ops/monitoring/install.sh](../../../tools/ops/monitoring/install.sh) |
| source | — | [tools/ops/backup/backup.sh](../../../tools/ops/backup/backup.sh) |
| source | — | [tools/ops/backup/restore.sh](../../../tools/ops/backup/restore.sh) |
| source | — | [tools/ops/keyring-migrate.sh](../../../tools/ops/keyring-migrate.sh) |
| source | — | [tools/ops/monitoring/render.sh](../../../tools/ops/monitoring/render.sh) |

## ops-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `BACKUP_DB_DUMP_CMD="pg_dump --format=custom --dbname=wallet"` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `BACKUP_DIR=/srv/backups/paxeer` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `BACKUP_HOST=backup@backup-host` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `BACKUP_IDENTITY=/root/backup-identity.txt` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `BACKUP_PREFIX=validator-a` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `BACKUP_RECIPIENTS_FILE=/etc/paxeer/backup-recipients.txt` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `BACKUP_RETENTION=14` | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| `config_key` | `ALERT_EMAIL_FROM=""` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `ALERT_EMAIL_TO=""` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `ALERT_SMTP_PASSWORD=""` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `ALERT_SMTP_SMARTHOST=""` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `ALERT_SMTP_USERNAME=""` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `COMET_METRICS_PORT=26660` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `HOSTS` (space-separated label-to-host assignments) | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `NODE_EXPORTER_PORT=9100` | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| `config_key` | `Environment=BACKUP_ENV=/etc/paxeer/backup.env` | [tools/ops/backup/paxeer-backup.service](../../../tools/ops/backup/paxeer-backup.service) |
| `config_key` | `ExecStart=/opt/paxeer/tools/ops/backup/backup.sh` | [tools/ops/backup/paxeer-backup.service](../../../tools/ops/backup/paxeer-backup.service) |
| `config_key` | `- job_name: ai-evaluator` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: cometbft` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: edge-cert` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: feeder` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: keeper` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: monitor-node` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: node` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: paxai-worker` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: receipt-authority` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: service-probe` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: wallet-attestors` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| `config_key` | `- job_name: x-websearch` | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| source | — | [tools/ops/monitoring/docker-compose.yml](../../../tools/ops/monitoring/docker-compose.yml) |

## ops-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/ops/backup/backup.sh](../../../tools/ops/backup/backup.sh) |
| source | — | [tools/ops/backup/lib.sh](../../../tools/ops/backup/lib.sh) |
| source | — | [tools/ops/backup/restore.sh](../../../tools/ops/backup/restore.sh) |
| source | — | [tools/ops/keyring-migrate.sh](../../../tools/ops/keyring-migrate.sh) |
| source | — | [tools/ops/monitoring/render.sh](../../../tools/ops/monitoring/render.sh) |

## ops-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/ops/backup/backup.env.example](../../../tools/ops/backup/backup.env.example) |
| source | — | [tools/ops/backup/paxeer-backup.service](../../../tools/ops/backup/paxeer-backup.service) |
| source | — | [tools/ops/backup/paxeer-backup.timer](../../../tools/ops/backup/paxeer-backup.timer) |
| source | — | [tools/ops/monitoring/blackbox.yml](../../../tools/ops/monitoring/blackbox.yml) |
| source | — | [tools/ops/monitoring/docker-compose.yml](../../../tools/ops/monitoring/docker-compose.yml) |
| source | — | [tools/ops/monitoring/hosts.env.example](../../../tools/ops/monitoring/hosts.env.example) |
| source | — | [tools/ops/monitoring/prometheus.yml](../../../tools/ops/monitoring/prometheus.yml) |
| source | — | [tools/ops/monitoring/rules_test.yml](../../../tools/ops/monitoring/rules_test.yml) |
| source | — | [tools/ops/monitoring/rules.yml](../../../tools/ops/monitoring/rules.yml) |

## ops-metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/ops/monitoring/rules.yml](../../../tools/ops/monitoring/rules.yml) |

## paxctl-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `Name:  "await",` | [tools/paxctl/await.go](../../../tools/paxctl/await.go) |
| `cli_command` | `Name:        "timeout",` | [tools/paxctl/await.go](../../../tools/paxctl/await.go) |
| `cli_command` | `Name:                   "validator",` | [tools/paxctl/await.go](../../../tools/paxctl/await.go) |
| `cli_command` | `Name:        "address",` | [tools/paxctl/await.go](../../../tools/paxctl/await.go) |
| `cli_command` | `Name:        "api",` | [tools/paxctl/await.go](../../../tools/paxctl/await.go) |
| `cli_command` | `Name:  "config",` | [tools/paxctl/config.go](../../../tools/paxctl/config.go) |
| `cli_command` | `Name:        "target",` | [tools/paxctl/config.go](../../../tools/paxctl/config.go) |
| `cli_command` | `Name:                   "patch",` | [tools/paxctl/config.go](../../../tools/paxctl/config.go) |
| `cli_command` | `Name:        "file",` | [tools/paxctl/config.go](../../../tools/paxctl/config.go) |
| `cli_command` | `Name:  "genesis",` | [tools/paxctl/genesis.go](../../../tools/paxctl/genesis.go) |
| `cli_command` | `Name:                   "patch",` | [tools/paxctl/genesis.go](../../../tools/paxctl/genesis.go) |
| `cli_command` | `Name:        "file",` | [tools/paxctl/genesis.go](../../../tools/paxctl/genesis.go) |
| `cli_command` | `Name:  "network",` | [tools/paxctl/seinetwork/cmd.go](../../../tools/paxctl/seinetwork/cmd.go) |
| `cli_command` | `Name: "apply",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNetwork"},` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "chain-id",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "dry-run",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "genesis-account",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "genesis-override",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "image",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:     "preset",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "replicas",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:  "set",` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `cli_command` | `Name:      "delete",` | [tools/paxctl/seinetwork/delete.go](../../../tools/paxctl/seinetwork/delete.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNetwork"},` | [tools/paxctl/seinetwork/delete.go](../../../tools/paxctl/seinetwork/delete.go) |
| `cli_command` | `Name:  "cascade",` | [tools/paxctl/seinetwork/delete.go](../../../tools/paxctl/seinetwork/delete.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinetwork/delete.go](../../../tools/paxctl/seinetwork/delete.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinetwork/delete.go](../../../tools/paxctl/seinetwork/delete.go) |
| `cli_command` | `Name:      "get",` | [tools/paxctl/seinetwork/get.go](../../../tools/paxctl/seinetwork/get.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNetwork"},` | [tools/paxctl/seinetwork/get.go](../../../tools/paxctl/seinetwork/get.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinetwork/get.go](../../../tools/paxctl/seinetwork/get.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinetwork/get.go](../../../tools/paxctl/seinetwork/get.go) |
| `cli_command` | `Name:    "output",` | [tools/paxctl/seinetwork/get.go](../../../tools/paxctl/seinetwork/get.go) |
| `cli_command` | `Name:  "list",` | [tools/paxctl/seinetwork/list.go](../../../tools/paxctl/seinetwork/list.go) |
| `cli_command` | `Name:    "all-namespaces",` | [tools/paxctl/seinetwork/list.go](../../../tools/paxctl/seinetwork/list.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinetwork/list.go](../../../tools/paxctl/seinetwork/list.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinetwork/list.go](../../../tools/paxctl/seinetwork/list.go) |
| `cli_command` | `Name:    "output",` | [tools/paxctl/seinetwork/list.go](../../../tools/paxctl/seinetwork/list.go) |
| `cli_command` | `Name:    "selector",` | [tools/paxctl/seinetwork/list.go](../../../tools/paxctl/seinetwork/list.go) |
| `cli_command` | `Name:      "watch",` | [tools/paxctl/seinetwork/watch.go](../../../tools/paxctl/seinetwork/watch.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNetwork"},` | [tools/paxctl/seinetwork/watch.go](../../../tools/paxctl/seinetwork/watch.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinetwork/watch.go](../../../tools/paxctl/seinetwork/watch.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinetwork/watch.go](../../../tools/paxctl/seinetwork/watch.go) |
| `cli_command` | `Name:  "timeout",` | [tools/paxctl/seinetwork/watch.go](../../../tools/paxctl/seinetwork/watch.go) |
| `cli_command` | `Name:     "until",` | [tools/paxctl/seinetwork/watch.go](../../../tools/paxctl/seinetwork/watch.go) |
| `cli_command` | `Name:  "node",` | [tools/paxctl/seinode/cmd.go](../../../tools/paxctl/seinode/cmd.go) |
| `cli_command` | `Name: "apply",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNode"},` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "chain-id",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "dry-run",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "external-address",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "image",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "network",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "override",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:     "preset",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:  "set",` | [tools/paxctl/seinode/apply.go](../../../tools/paxctl/seinode/apply.go) |
| `cli_command` | `Name:      "delete",` | [tools/paxctl/seinode/delete.go](../../../tools/paxctl/seinode/delete.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNode"},` | [tools/paxctl/seinode/delete.go](../../../tools/paxctl/seinode/delete.go) |
| `cli_command` | `Name:  "cascade",` | [tools/paxctl/seinode/delete.go](../../../tools/paxctl/seinode/delete.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinode/delete.go](../../../tools/paxctl/seinode/delete.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinode/delete.go](../../../tools/paxctl/seinode/delete.go) |
| `cli_command` | `Name:      "get",` | [tools/paxctl/seinode/get.go](../../../tools/paxctl/seinode/get.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNode"},` | [tools/paxctl/seinode/get.go](../../../tools/paxctl/seinode/get.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinode/get.go](../../../tools/paxctl/seinode/get.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinode/get.go](../../../tools/paxctl/seinode/get.go) |
| `cli_command` | `Name:    "output",` | [tools/paxctl/seinode/get.go](../../../tools/paxctl/seinode/get.go) |
| `cli_command` | `Name:  "list",` | [tools/paxctl/seinode/list.go](../../../tools/paxctl/seinode/list.go) |
| `cli_command` | `Name:    "all-namespaces",` | [tools/paxctl/seinode/list.go](../../../tools/paxctl/seinode/list.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinode/list.go](../../../tools/paxctl/seinode/list.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinode/list.go](../../../tools/paxctl/seinode/list.go) |
| `cli_command` | `Name:    "output",` | [tools/paxctl/seinode/list.go](../../../tools/paxctl/seinode/list.go) |
| `cli_command` | `Name:    "selector",` | [tools/paxctl/seinode/list.go](../../../tools/paxctl/seinode/list.go) |
| `cli_command` | `Name:      "watch",` | [tools/paxctl/seinode/watch.go](../../../tools/paxctl/seinode/watch.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNode"},` | [tools/paxctl/seinode/watch.go](../../../tools/paxctl/seinode/watch.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/seinode/watch.go](../../../tools/paxctl/seinode/watch.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/seinode/watch.go](../../../tools/paxctl/seinode/watch.go) |
| `cli_command` | `Name:  "timeout",` | [tools/paxctl/seinode/watch.go](../../../tools/paxctl/seinode/watch.go) |
| `cli_command` | `Name:     "until",` | [tools/paxctl/seinode/watch.go](../../../tools/paxctl/seinode/watch.go) |
| `cli_command` | `Name:                   "patch",` | [tools/paxctl/patch.go](../../../tools/paxctl/patch.go) |
| `cli_command` | `Name:        "file",` | [tools/paxctl/patch.go](../../../tools/paxctl/patch.go) |
| `cli_command` | `Name:        "target",` | [tools/paxctl/patch.go](../../../tools/paxctl/patch.go) |
| `cli_command` | `Name:  "report",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:  "divergence",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:    "bucket",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:  "env",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:  "height",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:  "json",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:  "key",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:    "prefix",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:    "region",` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `cli_command` | `Name:  "list",` | [tools/paxctl/report_list.go](../../../tools/paxctl/report_list.go) |
| `cli_command` | `Name:    "bucket",` | [tools/paxctl/report_list.go](../../../tools/paxctl/report_list.go) |
| `cli_command` | `Name:  "env",` | [tools/paxctl/report_list.go](../../../tools/paxctl/report_list.go) |
| `cli_command` | `Name:  "json",` | [tools/paxctl/report_list.go](../../../tools/paxctl/report_list.go) |
| `cli_command` | `Name:    "prefix",` | [tools/paxctl/report_list.go](../../../tools/paxctl/report_list.go) |
| `cli_command` | `Name:    "region",` | [tools/paxctl/report_list.go](../../../tools/paxctl/report_list.go) |
| `cli_command` | `Name: "seictl",` | [tools/paxctl/main.go](../../../tools/paxctl/main.go) |
| `cli_command` | `Name:        "home",` | [tools/paxctl/main.go](../../../tools/paxctl/main.go) |
| `cli_command` | `Name:  "serve",` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `cli_command` | `Name:    "port",` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/task/output.go](../../../tools/paxctl/task/output.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/task/output.go](../../../tools/paxctl/task/output.go) |
| `cli_command` | `Name:  "port",` | [tools/paxctl/task/output.go](../../../tools/paxctl/task/output.go) |
| `cli_command` | `Name:     "node",` | [tools/paxctl/task/output.go](../../../tools/paxctl/task/output.go) |
| `cli_command` | `Name:        "in-place-rewrite",` | [tools/paxctl/outputter.go](../../../tools/paxctl/outputter.go) |
| `cli_command` | `Name:        "output",` | [tools/paxctl/outputter.go](../../../tools/paxctl/outputter.go) |
| `cli_command` | `Name:  "task",` | [tools/paxctl/task/cmd.go](../../../tools/paxctl/task/cmd.go) |
| `cli_command` | `Name:      "delete",` | [tools/paxctl/task/delete.go](../../../tools/paxctl/task/delete.go) |
| `cli_command` | `&cli.StringArg{Name: "id", UsageText: "task UUID"},` | [tools/paxctl/task/delete.go](../../../tools/paxctl/task/delete.go) |
| `cli_command` | `Name:      "get",` | [tools/paxctl/task/get.go](../../../tools/paxctl/task/get.go) |
| `cli_command` | `&cli.StringArg{Name: "id", UsageText: "task UUID"},` | [tools/paxctl/task/get.go](../../../tools/paxctl/task/get.go) |
| `cli_command` | `Name:  "list",` | [tools/paxctl/task/list.go](../../../tools/paxctl/task/list.go) |
| `cli_command` | `Name:  "snapshot-upload",` | [tools/paxctl/task/snapshot_upload.go](../../../tools/paxctl/task/snapshot_upload.go) |
| `cli_command` | `Name:  "chain",` | [tools/paxctl/task/snapshot_upload.go](../../../tools/paxctl/task/snapshot_upload.go) |
| `cli_command` | `Name:  "poll-interval",` | [tools/paxctl/task/snapshot_upload.go](../../../tools/paxctl/task/snapshot_upload.go) |
| `cli_command` | `Name:  "timeout",` | [tools/paxctl/task/snapshot_upload.go](../../../tools/paxctl/task/snapshot_upload.go) |
| `cli_command` | `Name:      "submit",` | [tools/paxctl/task/submit.go](../../../tools/paxctl/task/submit.go) |
| `cli_command` | `&cli.StringArg{Name: "type", UsageText: "task type identifier (e.g. config-validate)"},` | [tools/paxctl/task/submit.go](../../../tools/paxctl/task/submit.go) |
| `cli_command` | `Name:  "params",` | [tools/paxctl/task/submit.go](../../../tools/paxctl/task/submit.go) |
| `cli_command` | `Name:  "workflow",` | [tools/paxctl/workflow/cmd.go](../../../tools/paxctl/workflow/cmd.go) |
| `cli_command` | `Name: "apply",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNodeTaskWorkflow"},` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "backend",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "config-patch",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "dry-run",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "migration",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "preset",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "require-phase",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "rpc-servers",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:  "set",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:     "target",` | [tools/paxctl/workflow/apply.go](../../../tools/paxctl/workflow/apply.go) |
| `cli_command` | `Name:      "delete",` | [tools/paxctl/workflow/delete.go](../../../tools/paxctl/workflow/delete.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNodeTaskWorkflow"},` | [tools/paxctl/workflow/delete.go](../../../tools/paxctl/workflow/delete.go) |
| `cli_command` | `Name:  "cascade",` | [tools/paxctl/workflow/delete.go](../../../tools/paxctl/workflow/delete.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/workflow/delete.go](../../../tools/paxctl/workflow/delete.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/workflow/delete.go](../../../tools/paxctl/workflow/delete.go) |
| `cli_command` | `Name:      "get",` | [tools/paxctl/workflow/get.go](../../../tools/paxctl/workflow/get.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNodeTaskWorkflow"},` | [tools/paxctl/workflow/get.go](../../../tools/paxctl/workflow/get.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/workflow/get.go](../../../tools/paxctl/workflow/get.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/workflow/get.go](../../../tools/paxctl/workflow/get.go) |
| `cli_command` | `Name:    "output",` | [tools/paxctl/workflow/get.go](../../../tools/paxctl/workflow/get.go) |
| `cli_command` | `Name:  "list",` | [tools/paxctl/workflow/list.go](../../../tools/paxctl/workflow/list.go) |
| `cli_command` | `Name:    "all-namespaces",` | [tools/paxctl/workflow/list.go](../../../tools/paxctl/workflow/list.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/workflow/list.go](../../../tools/paxctl/workflow/list.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/workflow/list.go](../../../tools/paxctl/workflow/list.go) |
| `cli_command` | `Name:    "output",` | [tools/paxctl/workflow/list.go](../../../tools/paxctl/workflow/list.go) |
| `cli_command` | `Name:    "selector",` | [tools/paxctl/workflow/list.go](../../../tools/paxctl/workflow/list.go) |
| `cli_command` | `Name: "state-sync",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `&cli.StringArg{Name: "node", UsageText: "metadata.name of the target SeiNode"},` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "backend",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "config-patch",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "dry-run",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "migration",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "name",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "no-watch",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "require-phase",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "rpc-servers",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "set",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:  "timeout",` | [tools/paxctl/workflow/state_sync.go](../../../tools/paxctl/workflow/state_sync.go) |
| `cli_command` | `Name:      "watch",` | [tools/paxctl/workflow/watch.go](../../../tools/paxctl/workflow/watch.go) |
| `cli_command` | `&cli.StringArg{Name: "name", UsageText: "metadata.name of the SeiNodeTaskWorkflow"},` | [tools/paxctl/workflow/watch.go](../../../tools/paxctl/workflow/watch.go) |
| `cli_command` | `Name:    "kubeconfig",` | [tools/paxctl/workflow/watch.go](../../../tools/paxctl/workflow/watch.go) |
| `cli_command` | `Name:    "namespace",` | [tools/paxctl/workflow/watch.go](../../../tools/paxctl/workflow/watch.go) |
| `cli_command` | `Name:  "timeout",` | [tools/paxctl/workflow/watch.go](../../../tools/paxctl/workflow/watch.go) |
| `cli_command` | `Name:     "until",` | [tools/paxctl/workflow/watch.go](../../../tools/paxctl/workflow/watch.go) |

## paxctl-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `[base] mode = "full"` | [tools/paxctl/sidecar/tasks/defaults/config.toml](../../../tools/paxctl/sidecar/tasks/defaults/config.toml) |
| `config_key` | `[consensus] timeout-commit = "5s"` | [tools/paxctl/sidecar/tasks/defaults/config.toml](../../../tools/paxctl/sidecar/tasks/defaults/config.toml) |
| `config_key` | `[mempool] size = 5000` | [tools/paxctl/sidecar/tasks/defaults/config.toml](../../../tools/paxctl/sidecar/tasks/defaults/config.toml) |
| `config_key` | `[p2p] persistent-peers = ""` | [tools/paxctl/sidecar/tasks/defaults/config.toml](../../../tools/paxctl/sidecar/tasks/defaults/config.toml) |
| `config_key` | `[statesync] enable = false` | [tools/paxctl/sidecar/tasks/defaults/config.toml](../../../tools/paxctl/sidecar/tasks/defaults/config.toml) |
| `config_key` | `TaskAssembleAndUploadGenesis TaskType = "assemble-and-upload-genesis"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskAwaitCondition           TaskType = "await-condition"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskConfigApply              TaskType = "config-apply"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskConfigPatch              TaskType = "config-patch"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskConfigReload             TaskType = "config-reload"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskConfigValidate           TaskType = "config-validate"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskConfigureGenesis         TaskType = "configure-genesis"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskConfigureStateSync       TaskType = "configure-state-sync"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskEvmLogicalDigest         TaskType = "evm-logical-digest"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskGenerateGentx            TaskType = "generate-gentx"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskGenerateIdentity         TaskType = "generate-identity"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskGovParamChange           TaskType = "gov-param-change"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskGovSoftwareUpgrade       TaskType = "gov-software-upgrade"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskGovVote                  TaskType = "gov-vote"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskMarkNotReady TaskType = "mark-not-ready"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskMarkReady                TaskType = "mark-ready"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskResetData    TaskType = "reset-data"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskRestartSeid              TaskType = "restart-seid"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskResultExport             TaskType = "result-export"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskSetGenesisPeers          TaskType = "set-genesis-peers"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskSnapshotRestore          TaskType = "snapshot-restore"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskSnapshotUpload           TaskType = "snapshot-upload"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskSnapshotUploadOnce       TaskType = "snapshot-upload-once"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskStopSeid     TaskType = "stop-seid"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | `TaskUploadGenesisArtifacts   TaskType = "upload-genesis-artifacts"` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``AccountBalance string                     `json:"accountBalance"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``Accounts       []GenesisAccountEntry      `json:"accounts,omitempty"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``Namespace      string                     `json:"namespace"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``Nodes          []AssembleNodeEntry        `json:"nodes"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``Overrides      map[string]json.RawMessage `json:"overrides,omitempty"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``GenesisHash string `json:"genesisHash"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``Name string `json:"name"`` | [tools/paxctl/sidecar/tasks/assemble_genesis.go](../../../tools/paxctl/sidecar/tasks/assemble_genesis.go) |
| `config_key` | ``Action       string `json:"action"`` | [tools/paxctl/sidecar/tasks/await_condition.go](../../../tools/paxctl/sidecar/tasks/await_condition.go) |
| `config_key` | ``Condition    string `json:"condition"`` | [tools/paxctl/sidecar/tasks/await_condition.go](../../../tools/paxctl/sidecar/tasks/await_condition.go) |
| `config_key` | ``TargetHeight int64  `json:"targetHeight"`` | [tools/paxctl/sidecar/tasks/await_condition.go](../../../tools/paxctl/sidecar/tasks/await_condition.go) |
| `config_key` | ``Files map[string]map[string]any `json:"files"`` | [tools/paxctl/sidecar/tasks/config.go](../../../tools/paxctl/sidecar/tasks/config.go) |
| `config_key` | ``Fields map[string]string `json:"fields"`` | [tools/paxctl/sidecar/tasks/config_reload.go](../../../tools/paxctl/sidecar/tasks/config_reload.go) |
| `config_key` | ``FlatKV  string `json:"flatkv"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Match   bool   `json:"match"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``MemIAVL string `json:"memiavl"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Account string `json:"account"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Code    string `json:"code"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Final   string `json:"final"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Legacy  string `json:"legacy"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Storage string `json:"storage"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Version int64  `json:"version"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``AxesProved    []string          `json:"axes_proved"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``FlatKVDigest  string            `json:"flatkv_digest"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``GeneratedAt   string            `json:"generated_at"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Height        int64             `json:"height"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Match         bool              `json:"match"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``MemIAVLDigest string            `json:"memiavl_digest"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Normalization string            `json:"normalization"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``PerBucket     map[string]bucket `json:"per_bucket"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Bucket     string `json:"bucket"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``FlatKVDir  string `json:"flatkvDir"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Height     int64  `json:"height"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``MemIAVLDir string `json:"memiavlDir"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Normalizations []string `json:"normalizations"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Prefix     string `json:"prefix"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``Region     string `json:"region"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``SeidbPath string `json:"seidbPath"`` | [tools/paxctl/sidecar/tasks/evm_logical_digest.go](../../../tools/paxctl/sidecar/tasks/evm_logical_digest.go) |
| `config_key` | ``AccountBalance string `json:"accountBalance"`` | [tools/paxctl/sidecar/tasks/generate_gentx.go](../../../tools/paxctl/sidecar/tasks/generate_gentx.go) |
| `config_key` | ``ChainID        string `json:"chainId"`` | [tools/paxctl/sidecar/tasks/generate_gentx.go](../../../tools/paxctl/sidecar/tasks/generate_gentx.go) |
| `config_key` | ``StakingAmount  string `json:"stakingAmount"`` | [tools/paxctl/sidecar/tasks/generate_gentx.go](../../../tools/paxctl/sidecar/tasks/generate_gentx.go) |
| `config_key` | ``ChainID string `json:"chainId"`` | [tools/paxctl/sidecar/tasks/generate_identity.go](../../../tools/paxctl/sidecar/tasks/generate_identity.go) |
| `config_key` | ``Moniker string `json:"moniker"`` | [tools/paxctl/sidecar/tasks/generate_identity.go](../../../tools/paxctl/sidecar/tasks/generate_identity.go) |
| `config_key` | ``ExpectedGenesisHash string `json:"expectedGenesisHash,omitempty"`` | [tools/paxctl/sidecar/tasks/genesis.go](../../../tools/paxctl/sidecar/tasks/genesis.go) |
| `config_key` | ``ChainID string `json:"chainId"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Changes []paramChange `json:"changes"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Description string `json:"description"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Fees string `json:"fees"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Gas  uint64 `json:"gas"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``InitialDeposit string `json:"initialDeposit"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``KeyName string `json:"keyName"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Memo string `json:"memo,omitempty"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Title       string `json:"title"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Key      string          `json:"key"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Subspace string          `json:"subspace"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``Value    json.RawMessage `json:"value"`` | [tools/paxctl/sidecar/tasks/gov_param_change.go](../../../tools/paxctl/sidecar/tasks/gov_param_change.go) |
| `config_key` | ``ChainID string `json:"chainId"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``Description string `json:"description"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``Fees string `json:"fees"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``Gas  uint64 `json:"gas"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``InitialDeposit string `json:"initialDeposit"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``KeyName string `json:"keyName"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``Memo string `json:"memo,omitempty"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``Title       string `json:"title"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``UpgradeHeight int64  `json:"upgradeHeight"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``UpgradeInfo   string `json:"upgradeInfo,omitempty"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``UpgradeName   string `json:"upgradeName"`` | [tools/paxctl/sidecar/tasks/gov_software_upgrade.go](../../../tools/paxctl/sidecar/tasks/gov_software_upgrade.go) |
| `config_key` | ``ChainID    string `json:"chainId"`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``Fees       string `json:"fees"`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``Gas        uint64 `json:"gas"`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``KeyName    string `json:"keyName"`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``Memo       string `json:"memo,omitempty"`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``Option     string `json:"option"` // yes \| no \| abstain \| no_with_veto`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``ProposalID uint64 `json:"proposalId"`` | [tools/paxctl/sidecar/tasks/gov_vote.go](../../../tools/paxctl/sidecar/tasks/gov_vote.go) |
| `config_key` | ``WipedBytes int64 `json:"wipedBytes"`` | [tools/paxctl/sidecar/tasks/reset_data.go](../../../tools/paxctl/sidecar/tasks/reset_data.go) |
| `config_key` | ``LastExportedHeight int64 `json:"lastExportedHeight"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``Bucket      string `json:"bucket"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``CanonicalEVMRPC string `json:"canonicalEvmRpc,omitempty"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``CanonicalRPC string `json:"canonicalRpc"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``ContinueOnDivergence bool `json:"continueOnDivergence,omitempty"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``MigrationMode bool `json:"migrationMode,omitempty"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``Prefix      string `json:"prefix"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``Region      string `json:"region"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``RPCEndpoint string `json:"rpcEndpoint"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``ShadowEVMRPC    string `json:"shadowEvmRpc,omitempty"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``TraceRPC string `json:"traceRpc,omitempty"`` | [tools/paxctl/sidecar/tasks/result_export.go](../../../tools/paxctl/sidecar/tasks/result_export.go) |
| `config_key` | ``AccountNumber uint64    `json:"accountNumber"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``BroadcastedAt time.Time `json:"broadcastedAt"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``ChainID       string    `json:"chainId"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``Code          uint32    `json:"code"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``Codespace     string    `json:"codespace,omitempty"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``GasUsed       int64     `json:"gasUsed"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``GasWanted     int64     `json:"gasWanted"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``Height        int64     `json:"height"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``IncludedAt *time.Time `json:"includedAt,omitempty"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``ProposalID uint64 `json:"proposalId,omitempty"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``RawLog        string    `json:"rawLog,omitempty"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``Sequence      uint64    `json:"sequence"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``TxHash        string    `json:"txHash"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``Unverifiable bool `json:"unverifiable,omitempty"`` | [tools/paxctl/sidecar/tasks/sign_and_broadcast.go](../../../tools/paxctl/sidecar/tasks/sign_and_broadcast.go) |
| `config_key` | ``TargetHeight int64 `json:"targetHeight,omitempty"`` | [tools/paxctl/sidecar/tasks/snapshot_restore.go](../../../tools/paxctl/sidecar/tasks/snapshot_restore.go) |
| `config_key` | ``Height     int64         `json:"height,omitempty"`` | [tools/paxctl/sidecar/tasks/snapshot_upload.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload.go) |
| `config_key` | ``Key        string        `json:"key,omitempty"`` | [tools/paxctl/sidecar/tasks/snapshot_upload.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload.go) |
| `config_key` | ``NoopReason NoopReason    `json:"noopReason,omitempty"`` | [tools/paxctl/sidecar/tasks/snapshot_upload.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload.go) |
| `config_key` | ``Outcome    UploadOutcome `json:"outcome"`` | [tools/paxctl/sidecar/tasks/snapshot_upload.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload.go) |
| `config_key` | ``LastUploadedAt     int64 `json:"lastUploadedAt,omitempty"`` | [tools/paxctl/sidecar/tasks/snapshot_upload.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload.go) |
| `config_key` | ``LastUploadedHeight int64 `json:"lastUploadedHeight"`` | [tools/paxctl/sidecar/tasks/snapshot_upload.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload.go) |
| `config_key` | ``BackfillBlocks   int64  `json:"backfillBlocks"`` | [tools/paxctl/sidecar/tasks/statesync.go](../../../tools/paxctl/sidecar/tasks/statesync.go) |
| `config_key` | ``RpcServers []string `json:"rpcServers"`` | [tools/paxctl/sidecar/tasks/statesync.go](../../../tools/paxctl/sidecar/tasks/statesync.go) |
| `config_key` | ``TrustPeriod      string `json:"trustPeriod"`` | [tools/paxctl/sidecar/tasks/statesync.go](../../../tools/paxctl/sidecar/tasks/statesync.go) |
| `config_key` | ``UseLocalSnapshot bool   `json:"useLocalSnapshot"`` | [tools/paxctl/sidecar/tasks/statesync.go](../../../tools/paxctl/sidecar/tasks/statesync.go) |
| `config_key` | ``NodeName string `json:"nodeName"`` | [tools/paxctl/sidecar/tasks/upload_genesis_artifacts.go](../../../tools/paxctl/sidecar/tasks/upload_genesis_artifacts.go) |
| `config_key` | ``Address string `json:"address"`` | [tools/paxctl/sidecar/wire/genesis_accounts.go](../../../tools/paxctl/sidecar/wire/genesis_accounts.go) |
| `config_key` | ``Balance string `json:"balance"`` | [tools/paxctl/sidecar/wire/genesis_accounts.go](../../../tools/paxctl/sidecar/wire/genesis_accounts.go) |
| `config_key` | ``Vesting *GenesisAccountVesting `json:"vesting,omitempty"`` | [tools/paxctl/sidecar/wire/genesis_accounts.go](../../../tools/paxctl/sidecar/wire/genesis_accounts.go) |
| `config_key` | ``Amount  string `json:"amount"`` | [tools/paxctl/sidecar/wire/genesis_accounts.go](../../../tools/paxctl/sidecar/wire/genesis_accounts.go) |
| `config_key` | ``Delayed bool   `json:"delayed,omitempty"`` | [tools/paxctl/sidecar/wire/genesis_accounts.go](../../../tools/paxctl/sidecar/wire/genesis_accounts.go) |
| `config_key` | ``EndTime int64  `json:"endTime"`` | [tools/paxctl/sidecar/wire/genesis_accounts.go](../../../tools/paxctl/sidecar/wire/genesis_accounts.go) |
| `config_key` | ``Code            uint32 `json:"code,omitempty"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``Codespace       string `json:"codespace,omitempty"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``Height          int64  `json:"height,omitempty"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``InclusionStatus string `json:"inclusionStatus"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``ProposalID      uint64 `json:"proposalId,omitempty"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``RawLog          string `json:"rawLog,omitempty"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |
| `config_key` | ``TxHash          string `json:"txHash"`` | [tools/paxctl/sidecar/wire/wire.go](../../../tools/paxctl/sidecar/wire/wire.go) |

## paxctl-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `HOSTNAME` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `KUBECONFIG` | [tools/paxctl/seinetwork/apply.go](../../../tools/paxctl/seinetwork/apply.go) |
| `env_var` | `SEI_CHAIN_ID` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_GENESIS_BUCKET` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_GENESIS_REGION` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_HOME` | [tools/paxctl/main.go](../../../tools/paxctl/main.go) |
| `env_var` | `SEI_KEYRING_BACKEND` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_KEYRING_DIR` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_KEYRING_PASSPHRASE` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_RESULT_EXPORT_BUCKET` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `env_var` | `SEI_RESULT_EXPORT_PREFIX` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `env_var` | `SEI_RESULT_EXPORT_REGION` | [tools/paxctl/report.go](../../../tools/paxctl/report.go) |
| `env_var` | `SEI_SIDECAR_AUTHN_MODE` | [tools/paxctl/sidecar/server/auth.go](../../../tools/paxctl/sidecar/server/auth.go) |
| `env_var` | `SEI_SIDECAR_PORT` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_SNAPSHOT_BUCKET` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_SNAPSHOT_REGION` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_SNAPSHOT_UPLOAD_INTERVAL` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |
| `env_var` | `SEI_SNAPSHOT_UPLOAD_TIMEOUT` | [tools/paxctl/serve.go](../../../tools/paxctl/serve.go) |

## paxctl-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `name: Containerize` | [tools/paxctl/.github/workflows/ghcr.yml](../../../tools/paxctl/.github/workflows/ghcr.yml) |
| `file_format` | `name: Go Linter` | [tools/paxctl/.github/workflows/uci-lint.yml](../../../tools/paxctl/.github/workflows/uci-lint.yml) |
| `file_format` | `name: Release Checker` | [tools/paxctl/.github/workflows/uci-release-check.yml](../../../tools/paxctl/.github/workflows/uci-release-check.yml) |
| `file_format` | `name: Releaser` | [tools/paxctl/.github/workflows/uci-release-publish.yml](../../../tools/paxctl/.github/workflows/uci-release-publish.yml) |
| `file_format` | `name: Stale Checker` | [tools/paxctl/.github/workflows/uci-stale-check.yml](../../../tools/paxctl/.github/workflows/uci-stale-check.yml) |
| source | — | [tools/paxctl/Dockerfile](../../../tools/paxctl/Dockerfile) |
| source | — | [tools/paxctl/go.mod](../../../tools/paxctl/go.mod) |
| source | — | [tools/paxctl/seinetwork/presets/genesis-chain.yaml](../../../tools/paxctl/seinetwork/presets/genesis-chain.yaml) |
| source | — | [tools/paxctl/seinode/presets/rpc.yaml](../../../tools/paxctl/seinode/presets/rpc.yaml) |
| source | — | [tools/paxctl/sidecar/api/codegen.yaml](../../../tools/paxctl/sidecar/api/codegen.yaml) |
| source | — | [tools/paxctl/sidecar/api/openapi.yaml](../../../tools/paxctl/sidecar/api/openapi.yaml) |
| source | — | [tools/paxctl/sidecar/tasks/defaults/config.toml](../../../tools/paxctl/sidecar/tasks/defaults/config.toml) |
| source | — | [tools/paxctl/VENDOR.kvx](../../../tools/paxctl/VENDOR.kvx) |
| source | — | [tools/paxctl/version.json](../../../tools/paxctl/version.json) |
| source | — | [tools/paxctl/workflow/presets/state-sync.yaml](../../../tools/paxctl/workflow/presets/state-sync.yaml) |

## paxctl-metrics

| Kind | Declaration | Source |
|---|---|---|
| `metric` | `Name: "seictl_shadow_blocks_compared_total",` | [tools/paxctl/sidecar/shadow/metrics.go](../../../tools/paxctl/sidecar/shadow/metrics.go) |
| `metric` | `Name: "seictl_shadow_divergences_total",` | [tools/paxctl/sidecar/shadow/metrics.go](../../../tools/paxctl/sidecar/shadow/metrics.go) |
| `metric` | `Name: "seictl_sidecar_authn_rejections_total",` | [tools/paxctl/sidecar/server/auth.go](../../../tools/paxctl/sidecar/server/auth.go) |
| `metric` | `Name: "seictl_snapshot_upload_last_run_success_timestamp_seconds",` | [tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go) |
| `metric` | `Name: "seictl_snapshot_upload_last_uploaded_height",` | [tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go) |
| `metric` | `Name: "seictl_snapshot_upload_last_uploaded_timestamp_seconds",` | [tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go) |
| `metric` | `Name: "seictl_snapshot_upload_outcome_total",` | [tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go](../../../tools/paxctl/sidecar/tasks/snapshot_upload_metrics.go) |
| `metric` | `Name:    "seictl_task_execution_duration_seconds",` | [tools/paxctl/sidecar/engine/metrics.go](../../../tools/paxctl/sidecar/engine/metrics.go) |
| `metric` | `Name: "seictl_task_failures_total",` | [tools/paxctl/sidecar/engine/metrics.go](../../../tools/paxctl/sidecar/engine/metrics.go) |
| `metric` | `Name: "seictl_task_panics_total",` | [tools/paxctl/sidecar/engine/metrics.go](../../../tools/paxctl/sidecar/engine/metrics.go) |
| `metric` | `Name: "seictl_task_submissions_total",` | [tools/paxctl/sidecar/engine/metrics.go](../../../tools/paxctl/sidecar/engine/metrics.go) |
| `metric` | `Name: "seictl_tx_broadcast_total",` | [tools/paxctl/sidecar/tasks/gov_result.go](../../../tools/paxctl/sidecar/tasks/gov_result.go) |

## paxctl-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `s.mux.HandleFunc("DELETE /v0/tasks/{id}", s.handleDeleteTask)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/healthz", s.handleHealthz)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/livez", s.handleLivez)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.Handle("GET /v0/metrics", promhttp.Handler())` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/node-id", s.handleNodeID)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/startupz", s.handleHealthz)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/status", s.handleStatus)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/tasks", s.handleListTasks)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("GET /v0/tasks/{id}", s.handleGetTask)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |
| `http_route` | `s.mux.HandleFunc("POST /v0/tasks", s.handlePostTask)` | [tools/paxctl/sidecar/server/server.go](../../../tools/paxctl/sidecar/server/server.go) |

## paxload-bounds

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | ``Bounds *BoundsConfig `json:"bounds,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``MaxP99Latency Duration `json:"maxP99Latency,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``MinTotalTxs uint64 `json:"minTotalTxs,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``MinTPS float64 `json:"minTps,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `go_exported` | `func (b Bounds) Check(fs *FinalStats) []Violation` | [tools/loadtest/stats/bounds.go](../../../tools/loadtest/stats/bounds.go) |
| `go_exported` | `type Bounds struct {` | [tools/loadtest/stats/bounds.go](../../../tools/loadtest/stats/bounds.go) |
| `go_exported` | `type Violation struct {` | [tools/loadtest/stats/bounds.go](../../../tools/loadtest/stats/bounds.go) |
| source | — | [tools/loadtest/main.go](../../../tools/loadtest/main.go) |

## paxload-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `func main() {` | [tools/loadtest/main.go](../../../tools/loadtest/main.go) |
| source | — | [tools/loadtest/scripts/generate_scenario_template.sh](../../../tools/loadtest/scripts/generate_scenario_template.sh) |
| source | — | [tools/loadtest/scripts/update_factory.sh](../../../tools/loadtest/scripts/update_factory.sh) |

## paxload-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | ``Accounts       int     `json:"count,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``NewAccountRate float64 `json:"newAccountRate,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Gas uint64 `json:"Gas"`` | [tools/loadtest/config/gas.go](../../../tools/loadtest/config/gas.go) |
| `config_key` | ``Name string `json:"Name"`` | [tools/loadtest/config/gas.go](../../../tools/loadtest/config/gas.go) |
| `config_key` | ``Accounts   *AccountConfig `json:"accounts,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``ChainID int64 `json:"chainId,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Endpoints  []string       `json:"endpoints"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``MockDeploy bool           `json:"mockDeploy,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``PaxChainID string         `json:"paxChainID,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``ReportPath string `json:"reportPath,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Scenarios  []Scenario     `json:"scenarios,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Settings   *Settings      `json:"settings,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Max uint64 `json:"Max"`` | [tools/loadtest/config/gas.go](../../../tools/loadtest/config/gas.go) |
| `config_key` | ``Min uint64 `json:"Min"`` | [tools/loadtest/config/gas.go](../../../tools/loadtest/config/gas.go) |
| `config_key` | ``Accounts        *AccountConfig `json:"accounts,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``GasFeeCapPicker *GasPicker     `json:"gasFeeCapPicker,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``GasPicker       *GasPicker     `json:"gasPicker,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``GasTipCapPicker *GasPicker     `json:"gasTipCapPicker,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Name            string         `json:"name,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``Weight          int            `json:"weight,omitempty"`` | [tools/loadtest/config/config.go](../../../tools/loadtest/config/config.go) |
| `config_key` | ``BufferSize       int      `json:"bufferSize,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``Debug            bool     `json:"debug,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``DryRun           bool     `json:"dryRun,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``NumBlocksToWrite int      `json:"numBlocksToWrite,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``Prewarm          bool     `json:"prewarm,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``RampUp           bool     `json:"rampUp,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``ReportPath       string   `json:"reportPath,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``StatsInterval    Duration `json:"statsInterval,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``TargetGas        uint64   `json:"targetGas,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``TPS              float64  `json:"tps,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``TrackBlocks      bool     `json:"trackBlocks,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``TrackReceipts    bool     `json:"trackReceipts,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``TrackUserLatency bool     `json:"trackUserLatency,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``TxsDir           string   `json:"txsDir,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |
| `config_key` | ``Workers          int      `json:"workers,omitempty"`` | [tools/loadtest/config/settings.go](../../../tools/loadtest/config/settings.go) |

## paxload-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/loadtest/Dockerfile](../../../tools/loadtest/Dockerfile) |
| source | — | [tools/loadtest/go.mod](../../../tools/loadtest/go.mod) |
| source | — | [tools/loadtest/profiles/autobake_evm_transfer.json](../../../tools/loadtest/profiles/autobake_evm_transfer.json) |
| source | — | [tools/loadtest/profiles/conflict.json](../../../tools/loadtest/profiles/conflict.json) |
| source | — | [tools/loadtest/profiles/evm_transfer.json](../../../tools/loadtest/profiles/evm_transfer.json) |
| source | — | [tools/loadtest/profiles/evm_transfer_write_file.json](../../../tools/loadtest/profiles/evm_transfer_write_file.json) |
| source | — | [tools/loadtest/profiles/loadtest_default.json](../../../tools/loadtest/profiles/loadtest_default.json) |
| source | — | [tools/loadtest/profiles/local_docker.json](../../../tools/loadtest/profiles/local_docker.json) |
| source | — | [tools/loadtest/profiles/local.json](../../../tools/loadtest/profiles/local.json) |
| source | — | [tools/loadtest/VENDOR.kvx](../../../tools/loadtest/VENDOR.kvx) |

## paxload-make

| Kind | Declaration | Source |
|---|---|---|
| `makefile_target` | `$(BINDINGS_DIR):` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(BINDINGS_DIR)/%.go: $(BUILD_DIR)/%.abi $(BUILD_DIR)/%.bin \| $(BINDINGS_DIR)` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `build: build-cli` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `build-cli: \| $(BUILD_DIR)` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(BUILD_DIR):` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `check-tools:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `clean:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `help:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `install: build-cli` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `install-tools: setup-node` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `lint:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(SCENARIOS_DIR):` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `$(SCENARIOS_DIR)/%.go: \| $(SCENARIOS_DIR)` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `setup-node:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |
| `makefile_target` | `test:` | [tools/loadtest/Makefile](../../../tools/loadtest/Makefile) |

## paxload-metrics

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/loadtest/sender/metrics.go](../../../tools/loadtest/sender/metrics.go) |
| source | — | [tools/loadtest/stats/metrics.go](../../../tools/loadtest/stats/metrics.go) |

## paxload-report

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/loadtest/stats/logger.go](../../../tools/loadtest/stats/logger.go) |

## paxload-routes

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/loadtest/main.go](../../../tools/loadtest/main.go) |

## pricesc-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"cadence_ms": 1000,` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"chain_id": "",` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"chainflow": {` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"feed_account": "",` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"gas_limit": 200000,` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"markets": [` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"note": "Fill chain_id, validator, feed_account and chainflow.base_url in access-controlled storage and delete this note; the loader refuses it. The unit passes PRICE_SIDECAR_CONFIG, PRICE_SIDECAR_FEED_KEY_FILE, PRICE_FEED_REST_API_KEY_FILE and PRICE_SIDECAR_STATE_DIR; the operator's endpoints.env s` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"retry_backoff_ms": 200,` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"retry_max_attempts": 3,` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `"validator": "",` | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| `config_key` | `Environment=PRICE_SIDECAR_FEED_KEY_FILE=%d/feed.key` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |
| `config_key` | `Environment=PRICE_FEED_REST_API_KEY_FILE=%d/chainflow.key` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |
| `config_key` | `Environment=PRICE_SIDECAR_CONFIG=/etc/price-sidecar/config.json` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |
| `config_key` | `Environment=PRICE_SIDECAR_STATE_DIR=%S/price-sidecar` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |
| `config_key` | `EnvironmentFile=/etc/price-sidecar/endpoints.env` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |
| `config_key` | `ExecStart=/usr/local/bin/price-sidecar` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |
| `config_key` | `User=price-sidecar` | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |

## pricesc-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/price-sidecar/config.template.json](../../../tools/price-sidecar/config.template.json) |
| source | — | [tools/price-sidecar/price-sidecar.service](../../../tools/price-sidecar/price-sidecar.service) |

## pxx-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if __name__ == '__main__':` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `cli_command` | `parser.add_argument('--selector', required=True, choices=SELECTORS)` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/paxeer-x/builders/104.13.1.py](../../../tools/paxeer-x/builders/104.13.1.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description='Read-only candidate inventory. Inputs must be sanitized, owned 0600 JSON; never supply environment files or credential values. Unknown evidence blocks action; inventories grant no release or mutation authority.')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--branch-notes', help='Private exact branch ancestry and touched-path inventory; retained by secure reference without integration credit')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `inventory.add_argument('--input', action='append', required=True)` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--inventory', action='append', default=[], help='Repeat for sanitized Fly, Docker, or runtime-selection JSON')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--mainline', default='refs/heads/main', help='Local mainline ref; no fetch is performed')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `check.add_argument('manifest')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--output', required=True, help='New private JSON file; existing files are never overwritten')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--repo', type=Path, required=True)` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `check.add_argument('--require-ready', action='store_true', help='Refuse dirty, branch-only or unresolved candidates; does not certify release')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `inventory.add_argument('--service', action='append', help='Exact catalogue ID; default all services')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--source-notes', help='Private source contract inventory containing branch_candidate_leads')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make.add_argument('--spec', type=Path, required=True)` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `make = sub.add_parser('create', help='Bind the complete service catalogue to Git identity and sanitized collector observations')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `inventory = sub.add_parser('inventory', help='Select actual observed runtime metadata from sanitized read-only collector records without repeating network probes')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `check = sub.add_parser('validate', help='Validate complete schema, provenance, immutable bindings and fail-closed decisions')` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `cli_command` | `if mode not in ("--build", "--verify"):` | [tools/paxeer-x/gates/103.6.10.sh](../../../tools/paxeer-x/gates/103.6.10.sh) |
| `cli_command` | `args in ([], ["--build"]),` | [tools/paxeer-x/gates/103.6.11.sh](../../../tools/paxeer-x/gates/103.6.11.sh) |
| `cli_command` | `args in ([], ["--build"]),` | [tools/paxeer-x/gates/103.6.13.sh](../../../tools/paxeer-x/gates/103.6.13.sh) |
| `cli_command` | `require(args in ([], ["--build"]), TASK + " accepts only --build or no arguments")` | [tools/paxeer-x/gates/103.6.3.sh](../../../tools/paxeer-x/gates/103.6.3.sh) |
| `cli_command` | `require(args in ([], ["--build"]), TASK + " accepts only --build or no arguments")` | [tools/paxeer-x/gates/103.6.4.sh](../../../tools/paxeer-x/gates/103.6.4.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/103.6.5.sh](../../../tools/paxeer-x/gates/103.6.5.sh) |
| `cli_command` | `--code-only)` | [tools/paxeer-x/gates/103.6.8.sh](../../../tools/paxeer-x/gates/103.6.8.sh) |
| `cli_command` | `require(args in ([], ["--build"]), TASK + " accepts only --build or no arguments")` | [tools/paxeer-x/gates/103.6.9.sh](../../../tools/paxeer-x/gates/103.6.9.sh) |
| `cli_command` | `for command in (["node", "--test", "e2e/agents.spec.ts"],` | [tools/paxeer-x/gates/104.12.4.sh](../../../tools/paxeer-x/gates/104.12.4.sh) |
| `cli_command` | `for command in (["node", "--test", "e2e/settings.spec.ts"],` | [tools/paxeer-x/gates/104.13.1.sh](../../../tools/paxeer-x/gates/104.13.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.13.6.sh](../../../tools/paxeer-x/gates/104.13.6.sh) |
| `cli_command` | `printf 'usage: %s (no arguments)\n' "$0" >&2` | [tools/paxeer-x/gates/104.15.4.sh](../../../tools/paxeer-x/gates/104.15.4.sh) |
| `cli_command` | `printf 'usage: %s (no arguments)\n' "$0" >&2` | [tools/paxeer-x/gates/104.16.2.sh](../../../tools/paxeer-x/gates/104.16.2.sh) |
| `cli_command` | `printf 'usage: %s [build]\n' "$0" >&2` | [tools/paxeer-x/gates/104.16.4.sh](../../../tools/paxeer-x/gates/104.16.4.sh) |
| `cli_command` | `printf 'usage: %s [--build-artifacts]\n' "$0" >&2` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `cli_command` | `require(subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--', *SOURCES]).returncode == 0,` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `cli_command` | `require(sys.argv[1:] in ([],['--build']),'unknown selector argument')` | [tools/paxeer-x/gates/104.17.5.sh](../../../tools/paxeer-x/gates/104.17.5.sh) |
| `cli_command` | `require(sys.argv[1:] in ([], ['--build']), 'unknown gate argument')` | [tools/paxeer-x/gates/104.17.6.sh](../../../tools/paxeer-x/gates/104.17.6.sh) |
| `cli_command` | `printf 'usage: %s (no arguments)\n' "$0" >&2` | [tools/paxeer-x/gates/104.18.2.sh](../../../tools/paxeer-x/gates/104.18.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.19.2.sh](../../../tools/paxeer-x/gates/104.19.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.19.3.sh](../../../tools/paxeer-x/gates/104.19.3.sh) |
| `cli_command` | `require(sys.argv[1:] in ([], ["--build"]), "usage: 104.19.4.sh [--build]")` | [tools/paxeer-x/gates/104.19.4.sh](../../../tools/paxeer-x/gates/104.19.4.sh) |
| `cli_command` | `require(len(sys.argv) == 1 or sys.argv[1:] == ['--build'], 'usage:104.21.1.sh [--build]')` | [tools/paxeer-x/gates/104.21.1.sh](../../../tools/paxeer-x/gates/104.21.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.24.1.sh](../../../tools/paxeer-x/gates/104.24.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.24.2.sh](../../../tools/paxeer-x/gates/104.24.2.sh) |
| `cli_command` | `raise SystemExit('usage: 104.25.1.sh [--build]')` | [tools/paxeer-x/gates/104.25.1.sh](../../../tools/paxeer-x/gates/104.25.1.sh) |
| `cli_command` | `BUILD = sys.argv[2:] == ['--build']` | [tools/paxeer-x/gates/104.25.1.sh](../../../tools/paxeer-x/gates/104.25.1.sh) |
| `cli_command` | `raise SystemExit('usage: 104.25.2.sh [--build]')` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `cli_command` | `BUILD = sys.argv[2:] == ['--build']` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.28.1.sh](../../../tools/paxeer-x/gates/104.28.1.sh) |
| `cli_command` | `output = run(base + [arg for suite in selected for arg in ('--test', suite)], 'existing integration suites', env)` | [tools/paxeer-x/gates/104.28.1.sh](../../../tools/paxeer-x/gates/104.28.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.28.2.sh](../../../tools/paxeer-x/gates/104.28.2.sh) |
| `cli_command` | `output = run(argv + [arg for suite in SUITES for arg in ('--test', suite)], 'activity calldata and composition suites', env)` | [tools/paxeer-x/gates/104.28.2.sh](../../../tools/paxeer-x/gates/104.28.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.28.3.sh](../../../tools/paxeer-x/gates/104.28.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.28.4.sh](../../../tools/paxeer-x/gates/104.28.4.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.28.5.sh](../../../tools/paxeer-x/gates/104.28.5.sh) |
| `cli_command` | `output = run(base + [arg for suite in SUITES for arg in ('--test', suite)], 'declared budget integration suites', env)` | [tools/paxeer-x/gates/104.28.5.sh](../../../tools/paxeer-x/gates/104.28.5.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.29.1.sh](../../../tools/paxeer-x/gates/104.29.1.sh) |
| `cli_command` | `output = run(base + [arg for suite in SUITES for arg in ('--test', suite)], 'namespace integration suites', env)` | [tools/paxeer-x/gates/104.29.1.sh](../../../tools/paxeer-x/gates/104.29.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.29.2.sh](../../../tools/paxeer-x/gates/104.29.2.sh) |
| `cli_command` | `output = run(base + [arg for suite in SUITES for arg in ('--test', suite)], 'shared-access integration suites', env)` | [tools/paxeer-x/gates/104.29.2.sh](../../../tools/paxeer-x/gates/104.29.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.29.3.sh](../../../tools/paxeer-x/gates/104.29.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.29.4.sh](../../../tools/paxeer-x/gates/104.29.4.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.29.5.sh](../../../tools/paxeer-x/gates/104.29.5.sh) |
| `cli_command` | `output = run(base + [arg for suite in SUITES for arg in ('--test', suite)], 'occupancy integration suites', env)` | [tools/paxeer-x/gates/104.29.5.sh](../../../tools/paxeer-x/gates/104.29.5.sh) |
| `cli_command` | `require(sys.argv[1:] in ([], ["--build"]), "usage: 104.30.3.sh [--build]")` | [tools/paxeer-x/gates/104.30.3.sh](../../../tools/paxeer-x/gates/104.30.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.31.2.sh](../../../tools/paxeer-x/gates/104.31.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.31.6.sh](../../../tools/paxeer-x/gates/104.31.6.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.32.1.sh](../../../tools/paxeer-x/gates/104.32.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.32.2.sh](../../../tools/paxeer-x/gates/104.32.2.sh) |
| `cli_command` | `output = run(base + ['--no-fail-fast'] + [arg for suite in selected for arg in ('--test', suite)],` | [tools/paxeer-x/gates/104.32.2.sh](../../../tools/paxeer-x/gates/104.32.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.32.3.sh](../../../tools/paxeer-x/gates/104.32.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.32.4.sh](../../../tools/paxeer-x/gates/104.32.4.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.32.5.sh](../../../tools/paxeer-x/gates/104.32.5.sh) |
| `cli_command` | `output = run(base + [arg for suite in SUITES for arg in ('--test', suite)], 'declared access guest suite', env)` | [tools/paxeer-x/gates/104.32.5.sh](../../../tools/paxeer-x/gates/104.32.5.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.32.6.sh](../../../tools/paxeer-x/gates/104.32.6.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.33.1.sh](../../../tools/paxeer-x/gates/104.33.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.33.3.sh](../../../tools/paxeer-x/gates/104.33.3.sh) |
| `cli_command` | `require(args in ([], ['--build']), 'only --build or the declared verify selector is admitted')` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `cli_command` | `require(args in ([], ['--build']), 'only --build or the declared verify selector is admitted')` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.34.1.sh](../../../tools/paxeer-x/gates/104.34.1.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.34.2.sh](../../../tools/paxeer-x/gates/104.34.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.34.3.sh](../../../tools/paxeer-x/gates/104.34.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.34.4.sh](../../../tools/paxeer-x/gates/104.34.4.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.34.5.sh](../../../tools/paxeer-x/gates/104.34.5.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.34.6.sh](../../../tools/paxeer-x/gates/104.34.6.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.35.2.sh](../../../tools/paxeer-x/gates/104.35.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.35.3.sh](../../../tools/paxeer-x/gates/104.35.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.35.5.sh](../../../tools/paxeer-x/gates/104.35.5.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.35.6.sh](../../../tools/paxeer-x/gates/104.35.6.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.35.7.sh](../../../tools/paxeer-x/gates/104.35.7.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.37.2.sh](../../../tools/paxeer-x/gates/104.37.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.38.1.sh](../../../tools/paxeer-x/gates/104.38.1.sh) |
| `cli_command` | `require(args in ([], ["--build"]), "104.38.2 accepts only --build or no arguments")` | [tools/paxeer-x/gates/104.38.2.sh](../../../tools/paxeer-x/gates/104.38.2.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.38.3.sh](../../../tools/paxeer-x/gates/104.38.3.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.38.4.sh](../../../tools/paxeer-x/gates/104.38.4.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.38.7.sh](../../../tools/paxeer-x/gates/104.38.7.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/104.38.9.sh](../../../tools/paxeer-x/gates/104.38.9.sh) |
| `cli_command` | `echo "usage: $0 (no arguments)" >&2` | [tools/paxeer-x/gates/110.8.2.sh](../../../tools/paxeer-x/gates/110.8.2.sh) |
| `cli_command` | `*) echo 'usage: 24.16.sh build\|verify' >&2; exit 2 ;;` | [tools/paxeer-x/gates/24.16.sh](../../../tools/paxeer-x/gates/24.16.sh) |
| `cli_command` | `build)` | [tools/paxeer-x/gates/24.16.sh](../../../tools/paxeer-x/gates/24.16.sh) |
| `cli_command` | `verify)` | [tools/paxeer-x/gates/24.16.sh](../../../tools/paxeer-x/gates/24.16.sh) |
| `cli_command` | `build)` | [tools/paxeer-x/gates/24.18.sh](../../../tools/paxeer-x/gates/24.18.sh) |
| `cli_command` | `verify)` | [tools/paxeer-x/gates/24.18.sh](../../../tools/paxeer-x/gates/24.18.sh) |
| `cli_command` | `assert value['native_build_source_binding']==hashlib.sha256(subprocess.check_output(['git','ls-tree','-r','-z','--full-tree','HEAD'])).hexdigest()` | [tools/paxeer-x/gates/24.20.sh](../../../tools/paxeer-x/gates/24.20.sh) |
| `cli_command` | `build)` | [tools/paxeer-x/gates/24.20.sh](../../../tools/paxeer-x/gates/24.20.sh) |
| `cli_command` | `verify)` | [tools/paxeer-x/gates/24.20.sh](../../../tools/paxeer-x/gates/24.20.sh) |
| `cli_command` | `assert not git('status','--porcelain','--untracked-files=all') and git('rev-parse','HEAD')==revision` | [tools/paxeer-x/gates/5.1.sh](../../../tools/paxeer-x/gates/5.1.sh) |
| `cli_command` | `build)` | [tools/paxeer-x/gates/5.1.sh](../../../tools/paxeer-x/gates/5.1.sh) |
| `cli_command` | `usage: render.sh --out DIR --height H --release-url URL --release-sha256 HEX` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--bridge-config) BRIDGE_CONFIGS+=("$2") ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--bridge-manifest) BRIDGE_MANIFEST=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--chain-id) CHAIN_ID=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--deposit) DEPOSIT=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--from) FROM=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--height) HEIGHT=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--kernel-network-id) KERNEL_NETWORK_ID=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--out) OUT=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--pax-usd) PAX_USD=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--paxd) PAXD=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--rate-update-height) RATE_UPDATE_HEIGHT=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--release-sha256) RELEASE_SHA256=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `--release-url) RELEASE_URL=$2 ;;` | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| `cli_command` | `if len(args) >= 2 and args[0] == '--manifest':` | [tools/paxeer-x/release.sh](../../../tools/paxeer-x/release.sh) |
| `cli_command` | `ap = argparse.ArgumentParser(description=__doc__)` | [tools/paxeer-x/sid-snapshot/snapshot.py](../../../tools/paxeer-x/sid-snapshot/snapshot.py) |
| `cli_command` | `ap.add_argument("--source", action="append", required=True,` | [tools/paxeer-x/sid-snapshot/snapshot.py](../../../tools/paxeer-x/sid-snapshot/snapshot.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/paxeer-x/tests/ai_evidence_test.py](../../../tools/paxeer-x/tests/ai_evidence_test.py) |
| `cli_command` | `parser.add_argument('--selector', required=True, choices=evidence.SELECTORS)` | [tools/paxeer-x/tests/ai_evidence_test.py](../../../tools/paxeer-x/tests/ai_evidence_test.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/paxeer-x/tests/candidate_test.py](../../../tools/paxeer-x/tests/candidate_test.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/paxeer-x/tests/durable_release_evidence.py](../../../tools/paxeer-x/tests/durable_release_evidence.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/paxeer-x/tests/gate_dispatch_test.py](../../../tools/paxeer-x/tests/gate_dispatch_test.py) |
| `cli_command` | `parser = argparse.ArgumentParser(` | [tools/paxeer-x/tests/retired_callers_test.py](../../../tools/paxeer-x/tests/retired_callers_test.py) |
| `cli_command` | `parser.add_argument("--component", action="append", default=[])` | [tools/paxeer-x/tests/retired_callers_test.py](../../../tools/paxeer-x/tests/retired_callers_test.py) |
| `cli_command` | `parser.add_argument("--path", action="append", default=[])` | [tools/paxeer-x/tests/retired_callers_test.py](../../../tools/paxeer-x/tests/retired_callers_test.py) |
| `cli_command` | `parser.add_argument("--self-test", action="store_true")` | [tools/paxeer-x/tests/retired_callers_test.py](../../../tools/paxeer-x/tests/retired_callers_test.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/paxeer-x/tests/routes_test.py](../../../tools/paxeer-x/tests/routes_test.py) |
| `cli_command` | `mode = args[0] if args and args[0] in ('--identity', '--check') else 'run'` | [tools/paxeer-x/verify-task.sh](../../../tools/paxeer-x/verify-task.sh) |
| `cli_command` | `if len(rest) == 3 and rest[1] == '--manifest':` | [tools/paxeer-x/verify-task.sh](../../../tools/paxeer-x/verify-task.sh) |
| source | — | [tools/paxeer-x/gates/0.2.sh](../../../tools/paxeer-x/gates/0.2.sh) |
| source | — | [tools/paxeer-x/gates/0.3.sh](../../../tools/paxeer-x/gates/0.3.sh) |
| source | — | [tools/paxeer-x/gates/0.4.sh](../../../tools/paxeer-x/gates/0.4.sh) |
| source | — | [tools/paxeer-x/gates/0.6.sh](../../../tools/paxeer-x/gates/0.6.sh) |
| source | — | [tools/paxeer-x/gates/104.11.5.sh](../../../tools/paxeer-x/gates/104.11.5.sh) |
| source | — | [tools/paxeer-x/gates/104.17.3.sh](../../../tools/paxeer-x/gates/104.17.3.sh) |
| source | — | [tools/paxeer-x/gates/104.17.4.sh](../../../tools/paxeer-x/gates/104.17.4.sh) |
| source | — | [tools/paxeer-x/gates/104.26.1.sh](../../../tools/paxeer-x/gates/104.26.1.sh) |
| source | — | [tools/paxeer-x/gates/104.26.2.sh](../../../tools/paxeer-x/gates/104.26.2.sh) |
| source | — | [tools/paxeer-x/gates/104.30.2.sh](../../../tools/paxeer-x/gates/104.30.2.sh) |
| source | — | [tools/paxeer-x/gates/104.30.4.sh](../../../tools/paxeer-x/gates/104.30.4.sh) |
| source | — | [tools/paxeer-x/gates/104.30.5.sh](../../../tools/paxeer-x/gates/104.30.5.sh) |
| source | — | [tools/paxeer-x/gates/104.30.7.sh](../../../tools/paxeer-x/gates/104.30.7.sh) |
| source | — | [tools/paxeer-x/gates/104.31.1.sh](../../../tools/paxeer-x/gates/104.31.1.sh) |
| source | — | [tools/paxeer-x/gates/104.33.7.sh](../../../tools/paxeer-x/gates/104.33.7.sh) |
| source | — | [tools/paxeer-x/gates/104.35.4.sh](../../../tools/paxeer-x/gates/104.35.4.sh) |
| source | — | [tools/paxeer-x/gates/104.38.10.sh](../../../tools/paxeer-x/gates/104.38.10.sh) |
| source | — | [tools/paxeer-x/gates/104.38.6.sh](../../../tools/paxeer-x/gates/104.38.6.sh) |
| source | — | [tools/paxeer-x/gates/104.5.8.sh](../../../tools/paxeer-x/gates/104.5.8.sh) |
| source | — | [tools/paxeer-x/gates/106.4.2.sh](../../../tools/paxeer-x/gates/106.4.2.sh) |
| source | — | [tools/paxeer-x/gates/106.4.3.sh](../../../tools/paxeer-x/gates/106.4.3.sh) |
| source | — | [tools/paxeer-x/gates/106.4.5.sh](../../../tools/paxeer-x/gates/106.4.5.sh) |
| source | — | [tools/paxeer-x/gates/106.4.6.sh](../../../tools/paxeer-x/gates/106.4.6.sh) |
| source | — | [tools/paxeer-x/gates/106.4.9.sh](../../../tools/paxeer-x/gates/106.4.9.sh) |
| source | — | [tools/paxeer-x/gates/108.0.7.sh](../../../tools/paxeer-x/gates/108.0.7.sh) |
| source | — | [tools/paxeer-x/gates/108.1.10.sh](../../../tools/paxeer-x/gates/108.1.10.sh) |
| source | — | [tools/paxeer-x/gates/11.2.sh](../../../tools/paxeer-x/gates/11.2.sh) |
| source | — | [tools/paxeer-x/gates/11.3.sh](../../../tools/paxeer-x/gates/11.3.sh) |
| source | — | [tools/paxeer-x/gates/110.8.4.sh](../../../tools/paxeer-x/gates/110.8.4.sh) |
| source | — | [tools/paxeer-x/gates/111.6.11.sh](../../../tools/paxeer-x/gates/111.6.11.sh) |
| source | — | [tools/paxeer-x/gates/111.6.12.sh](../../../tools/paxeer-x/gates/111.6.12.sh) |
| source | — | [tools/paxeer-x/gates/12.1.sh](../../../tools/paxeer-x/gates/12.1.sh) |
| source | — | [tools/paxeer-x/gates/13.2.sh](../../../tools/paxeer-x/gates/13.2.sh) |
| source | — | [tools/paxeer-x/gates/14.10.sh](../../../tools/paxeer-x/gates/14.10.sh) |
| source | — | [tools/paxeer-x/gates/14.2.sh](../../../tools/paxeer-x/gates/14.2.sh) |
| source | — | [tools/paxeer-x/gates/14.4.sh](../../../tools/paxeer-x/gates/14.4.sh) |
| source | — | [tools/paxeer-x/gates/14.8.sh](../../../tools/paxeer-x/gates/14.8.sh) |
| source | — | [tools/paxeer-x/gates/14.9.sh](../../../tools/paxeer-x/gates/14.9.sh) |
| source | — | [tools/paxeer-x/gates/15.1.sh](../../../tools/paxeer-x/gates/15.1.sh) |
| source | — | [tools/paxeer-x/gates/15.2.sh](../../../tools/paxeer-x/gates/15.2.sh) |
| source | — | [tools/paxeer-x/gates/16.2.sh](../../../tools/paxeer-x/gates/16.2.sh) |
| source | — | [tools/paxeer-x/gates/16.6.sh](../../../tools/paxeer-x/gates/16.6.sh) |
| source | — | [tools/paxeer-x/gates/17.1.sh](../../../tools/paxeer-x/gates/17.1.sh) |
| source | — | [tools/paxeer-x/gates/18.3.sh](../../../tools/paxeer-x/gates/18.3.sh) |
| source | — | [tools/paxeer-x/gates/20.2.sh](../../../tools/paxeer-x/gates/20.2.sh) |
| source | — | [tools/paxeer-x/gates/21.1.sh](../../../tools/paxeer-x/gates/21.1.sh) |
| source | — | [tools/paxeer-x/gates/21.2.sh](../../../tools/paxeer-x/gates/21.2.sh) |
| source | — | [tools/paxeer-x/gates/21.3.sh](../../../tools/paxeer-x/gates/21.3.sh) |
| source | — | [tools/paxeer-x/gates/22.2.sh](../../../tools/paxeer-x/gates/22.2.sh) |
| source | — | [tools/paxeer-x/gates/22.7.sh](../../../tools/paxeer-x/gates/22.7.sh) |
| source | — | [tools/paxeer-x/gates/22.8.sh](../../../tools/paxeer-x/gates/22.8.sh) |
| source | — | [tools/paxeer-x/gates/24.1.sh](../../../tools/paxeer-x/gates/24.1.sh) |
| source | — | [tools/paxeer-x/gates/24.10.sh](../../../tools/paxeer-x/gates/24.10.sh) |
| source | — | [tools/paxeer-x/gates/24.11.sh](../../../tools/paxeer-x/gates/24.11.sh) |
| source | — | [tools/paxeer-x/gates/24.12.sh](../../../tools/paxeer-x/gates/24.12.sh) |
| source | — | [tools/paxeer-x/gates/24.13.sh](../../../tools/paxeer-x/gates/24.13.sh) |
| source | — | [tools/paxeer-x/gates/24.14.sh](../../../tools/paxeer-x/gates/24.14.sh) |
| source | — | [tools/paxeer-x/gates/24.15.sh](../../../tools/paxeer-x/gates/24.15.sh) |
| source | — | [tools/paxeer-x/gates/24.9.sh](../../../tools/paxeer-x/gates/24.9.sh) |
| source | — | [tools/paxeer-x/gates/25.1.sh](../../../tools/paxeer-x/gates/25.1.sh) |
| source | — | [tools/paxeer-x/gates/25.2.sh](../../../tools/paxeer-x/gates/25.2.sh) |
| source | — | [tools/paxeer-x/gates/25.3.sh](../../../tools/paxeer-x/gates/25.3.sh) |
| source | — | [tools/paxeer-x/gates/25.4.sh](../../../tools/paxeer-x/gates/25.4.sh) |
| source | — | [tools/paxeer-x/gates/3.2.sh](../../../tools/paxeer-x/gates/3.2.sh) |
| source | — | [tools/paxeer-x/gates/4.1.1.sh](../../../tools/paxeer-x/gates/4.1.1.sh) |
| source | — | [tools/paxeer-x/gates/6.1.sh](../../../tools/paxeer-x/gates/6.1.sh) |
| source | — | [tools/paxeer-x/gates/6.3.sh](../../../tools/paxeer-x/gates/6.3.sh) |
| source | — | [tools/paxeer-x/gates/8.3.sh](../../../tools/paxeer-x/gates/8.3.sh) |
| source | — | [tools/paxeer-x/gov/test.sh](../../../tools/paxeer-x/gov/test.sh) |
| source | — | [tools/paxeer-x/runtime-inventory.sh](../../../tools/paxeer-x/runtime-inventory.sh) |
| source | — | [tools/paxeer-x/sid-snapshot/test.sh](../../../tools/paxeer-x/sid-snapshot/test.sh) |

## pxx-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"account-naming": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"anchor": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"authorized-batch": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"availability-fetch": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"batch-seal": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"checkpoint-evidence": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"checkpoint-finality": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"custody-claims": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"custody-credit": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"custody-deposit": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"custody-disclosure": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"da": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"dual-market-ids": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"envelope-signing": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"eth-relay": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"exchange-intents": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"explorer-kernel-views": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"fee-relay": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"feed-store": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"finality-authority": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"guarantor-runtime": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"handover-client": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"human-two-domain": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"idempotency": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"indexer-kernel-half": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"intent-ingester": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"intent-vocabulary": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"kernel-web": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"layerxctl-port": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"lni": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"mirror": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"oracle-web-roots": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"paxeer-verifier": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"program-events": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"programs-ingress": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"protocol-adapter": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"receive-grant": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"replay-golden": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"replica": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"router-joins": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"settlement-domain": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"storage-keeper": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"verify": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"wallet-caps": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"wallet-two-domain": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"websearch-kernel-relay": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"wire-codec": {` | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| `config_key` | `"chain_id": 125` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"denied_surfaces": [` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"node_signing_methods": [` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"routes": [` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"rpc_paths": [` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"rpc_records": [` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"schema": "paxeer-x.routes.v1"` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"services": [` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"wallet_authentication": "Supabase"` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `config_key` | `"wallet_placement": "existing-docker-host"` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |

## pxx-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PAXAI_ADMISSION_RECORD` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `env_var` | `PAXAI_APPROVAL_RECORD` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `env_var` | `LAYERX_FINALITY_QUALIFICATION_DIR` | [tools/paxeer-x/gates/103.6.10.sh](../../../tools/paxeer-x/gates/103.6.10.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/103.6.11.sh](../../../tools/paxeer-x/gates/103.6.11.sh) |
| `env_var` | `PAXEER_X_PROGRAMS_TARGET_DIR` | [tools/paxeer-x/gates/103.6.11.sh](../../../tools/paxeer-x/gates/103.6.11.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/103.6.11.sh](../../../tools/paxeer-x/gates/103.6.11.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/103.6.13.sh](../../../tools/paxeer-x/gates/103.6.13.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/103.6.13.sh](../../../tools/paxeer-x/gates/103.6.13.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/103.6.13.sh](../../../tools/paxeer-x/gates/103.6.13.sh) |
| `env_var` | `HUMAN_E2E_SETTINGS_ARTIFACTS` | [tools/paxeer-x/gates/104.11.5.sh](../../../tools/paxeer-x/gates/104.11.5.sh) |
| `env_var` | `HUMAN_E2E_SETTINGS_ARTIFACTS` | [tools/paxeer-x/gates/104.12.4.sh](../../../tools/paxeer-x/gates/104.12.4.sh) |
| `env_var` | `HUMAN_E2E_SETTINGS_ARTIFACTS` | [tools/paxeer-x/gates/104.13.1.sh](../../../tools/paxeer-x/gates/104.13.1.sh) |
| `env_var` | `LAYERX_MOBILE_AGENT_INTEGRATION_FIXTURE` | [tools/paxeer-x/gates/104.16.4.sh](../../../tools/paxeer-x/gates/104.16.4.sh) |
| `env_var` | `LAYERX_SAMPLE_WEBHOOK_DELIVERY_PATH` | [tools/paxeer-x/gates/104.16.4.sh](../../../tools/paxeer-x/gates/104.16.4.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.16.4.sh](../../../tools/paxeer-x/gates/104.16.4.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `env_var` | `LAYERX_TEN_LINE_REGISTRY_SNAPSHOT` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `env_var` | `LAYERX_TEN_LINE_REGISTRY_URL` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `env_var` | `PAXEER_X_TEN_LINE_ARTIFACTS` | [tools/paxeer-x/gates/104.16.5.sh](../../../tools/paxeer-x/gates/104.16.5.sh) |
| `env_var` | `LAYERX_BIN` | [tools/paxeer-x/gates/104.17.3.sh](../../../tools/paxeer-x/gates/104.17.3.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.17.3.sh](../../../tools/paxeer-x/gates/104.17.3.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.17.4.sh](../../../tools/paxeer-x/gates/104.17.4.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.17.5.sh](../../../tools/paxeer-x/gates/104.17.5.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/104.17.5.sh](../../../tools/paxeer-x/gates/104.17.5.sh) |
| `env_var` | `PAXEER_X_WEBHOOK_DASHBOARD_ARTIFACTS` | [tools/paxeer-x/gates/104.17.5.sh](../../../tools/paxeer-x/gates/104.17.5.sh) |
| `env_var` | `PAXEER_X_WEBHOOK_DASHBOARD_FIXTURE` | [tools/paxeer-x/gates/104.17.5.sh](../../../tools/paxeer-x/gates/104.17.5.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.17.6.sh](../../../tools/paxeer-x/gates/104.17.6.sh) |
| `env_var` | `LAYERX_GATEWAY_URL` | [tools/paxeer-x/gates/104.17.6.sh](../../../tools/paxeer-x/gates/104.17.6.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/104.17.6.sh](../../../tools/paxeer-x/gates/104.17.6.sh) |
| `env_var` | `PAXEER_X_CARGO` | [tools/paxeer-x/gates/104.17.6.sh](../../../tools/paxeer-x/gates/104.17.6.sh) |
| `env_var` | `PAXEER_X_INSTALL_ARTIFACTS` | [tools/paxeer-x/gates/104.17.6.sh](../../../tools/paxeer-x/gates/104.17.6.sh) |
| `env_var` | `PAXEER_X_MODULE_REGISTRATION_RUST_TARGET` | [tools/paxeer-x/gates/104.19.4.sh](../../../tools/paxeer-x/gates/104.19.4.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.21.1.sh](../../../tools/paxeer-x/gates/104.21.1.sh) |
| `env_var` | `PAXEER_X_MONETARY_ARTIFACTS` | [tools/paxeer-x/gates/104.21.1.sh](../../../tools/paxeer-x/gates/104.21.1.sh) |
| `env_var` | `PAXEER_X_MONETARY_BUILD_DIR` | [tools/paxeer-x/gates/104.21.1.sh](../../../tools/paxeer-x/gates/104.21.1.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.21.1.sh](../../../tools/paxeer-x/gates/104.21.1.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.24.1.sh](../../../tools/paxeer-x/gates/104.24.1.sh) |
| `env_var` | `LAYERX_MIGRATION_HISTORY_FIXTURE` | [tools/paxeer-x/gates/104.24.1.sh](../../../tools/paxeer-x/gates/104.24.1.sh) |
| `env_var` | `LAYERX_MIGRATION_SECRET_DIR` | [tools/paxeer-x/gates/104.24.1.sh](../../../tools/paxeer-x/gates/104.24.1.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.24.1.sh](../../../tools/paxeer-x/gates/104.24.1.sh) |
| `env_var` | `PAXEER_X_MIRROR_EVIDENCE_ROOT` | [tools/paxeer-x/gates/104.25.1.sh](../../../tools/paxeer-x/gates/104.25.1.sh) |
| `env_var` | `PAXEER_X_MIRROR_LIVE_EVIDENCE` | [tools/paxeer-x/gates/104.25.1.sh](../../../tools/paxeer-x/gates/104.25.1.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/104.25.1.sh](../../../tools/paxeer-x/gates/104.25.1.sh) |
| `env_var` | `LAYERX_MIRROR_VERIFY_BIN` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `env_var` | `PAXEER_X_MIRROR_EVIDENCE_ROOT` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `env_var` | `PAXEER_X_MIRROR_WEB_TSC` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `env_var` | `LAYERX_DEPLOYMENT_PROFILE` | [tools/paxeer-x/gates/104.26.1.sh](../../../tools/paxeer-x/gates/104.26.1.sh) |
| `env_var` | `LAYERX_RAMP_PUBLIC_PATH_PREFIX` | [tools/paxeer-x/gates/104.26.1.sh](../../../tools/paxeer-x/gates/104.26.1.sh) |
| `env_var` | `PAXEER_X_RAMP_BUILD_MANIFEST` | [tools/paxeer-x/gates/104.26.1.sh](../../../tools/paxeer-x/gates/104.26.1.sh) |
| `env_var` | `PAXEER_X_RAMP_PRODUCER_PHASE` | [tools/paxeer-x/gates/104.26.1.sh](../../../tools/paxeer-x/gates/104.26.1.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.28.1.sh](../../../tools/paxeer-x/gates/104.28.1.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.28.1.sh](../../../tools/paxeer-x/gates/104.28.1.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.28.2.sh](../../../tools/paxeer-x/gates/104.28.2.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.28.2.sh](../../../tools/paxeer-x/gates/104.28.2.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.28.3.sh](../../../tools/paxeer-x/gates/104.28.3.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.28.3.sh](../../../tools/paxeer-x/gates/104.28.3.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.28.4.sh](../../../tools/paxeer-x/gates/104.28.4.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.28.4.sh](../../../tools/paxeer-x/gates/104.28.4.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.28.5.sh](../../../tools/paxeer-x/gates/104.28.5.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.28.5.sh](../../../tools/paxeer-x/gates/104.28.5.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.29.1.sh](../../../tools/paxeer-x/gates/104.29.1.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.29.1.sh](../../../tools/paxeer-x/gates/104.29.1.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.29.2.sh](../../../tools/paxeer-x/gates/104.29.2.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.29.2.sh](../../../tools/paxeer-x/gates/104.29.2.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.29.3.sh](../../../tools/paxeer-x/gates/104.29.3.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.29.3.sh](../../../tools/paxeer-x/gates/104.29.3.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.29.4.sh](../../../tools/paxeer-x/gates/104.29.4.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.29.4.sh](../../../tools/paxeer-x/gates/104.29.4.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.29.5.sh](../../../tools/paxeer-x/gates/104.29.5.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.29.5.sh](../../../tools/paxeer-x/gates/104.29.5.sh) |
| `env_var` | `PAXEER_X_PROGRAM_ACCOUNTS_RUST_TARGET` | [tools/paxeer-x/gates/104.30.3.sh](../../../tools/paxeer-x/gates/104.30.3.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.32.2.sh](../../../tools/paxeer-x/gates/104.32.2.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.32.2.sh](../../../tools/paxeer-x/gates/104.32.2.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.32.3.sh](../../../tools/paxeer-x/gates/104.32.3.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.32.5.sh](../../../tools/paxeer-x/gates/104.32.5.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.32.5.sh](../../../tools/paxeer-x/gates/104.32.5.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.32.6.sh](../../../tools/paxeer-x/gates/104.32.6.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/104.32.6.sh](../../../tools/paxeer-x/gates/104.32.6.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.32.6.sh](../../../tools/paxeer-x/gates/104.32.6.sh) |
| `env_var` | `CARGO` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `env_var` | `CARGO_BUILD_JOBS` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `env_var` | `PAXEER_X_INTERPRETER_EVIDENCE` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `env_var` | `CARGO` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `env_var` | `CARGO_BUILD_JOBS` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `env_var` | `PATH` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `env_var` | `PAXEER_X_HOSTED_PROGRAM_ARTIFACTS` | [tools/paxeer-x/gates/104.33.7.sh](../../../tools/paxeer-x/gates/104.33.7.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.35.3.sh](../../../tools/paxeer-x/gates/104.35.3.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.35.3.sh](../../../tools/paxeer-x/gates/104.35.3.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.35.4.sh](../../../tools/paxeer-x/gates/104.35.4.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.35.5.sh](../../../tools/paxeer-x/gates/104.35.5.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.35.5.sh](../../../tools/paxeer-x/gates/104.35.5.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.35.6.sh](../../../tools/paxeer-x/gates/104.35.6.sh) |
| `env_var` | `PROGRAMS_CARGO` | [tools/paxeer-x/gates/104.35.6.sh](../../../tools/paxeer-x/gates/104.35.6.sh) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/gates/104.38.2.sh](../../../tools/paxeer-x/gates/104.38.2.sh) |
| `env_var` | `PAXEER_X_RPC_TRANSPORT_EVIDENCE` | [tools/paxeer-x/gates/104.38.2.sh](../../../tools/paxeer-x/gates/104.38.2.sh) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/paxeer-x/gates/104.38.2.sh](../../../tools/paxeer-x/gates/104.38.2.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/104.38.3.sh](../../../tools/paxeer-x/gates/104.38.3.sh) |
| `env_var` | `PAXEER_X_HUMAN_API_MANIFEST` | [tools/paxeer-x/gates/104.38.3.sh](../../../tools/paxeer-x/gates/104.38.3.sh) |
| `env_var` | `PAXEER_X_HUMAN_COMPONENT_ARTIFACT_MANIFEST` | [tools/paxeer-x/gates/104.38.3.sh](../../../tools/paxeer-x/gates/104.38.3.sh) |
| `env_var` | `PAXEER_X_106_4_2_BUNDLE` | [tools/paxeer-x/gates/106.4.2.sh](../../../tools/paxeer-x/gates/106.4.2.sh) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/gates/106.4.2.sh](../../../tools/paxeer-x/gates/106.4.2.sh) |
| `env_var` | `PAXEER_X_106_4_3_BUNDLE` | [tools/paxeer-x/gates/106.4.3.sh](../../../tools/paxeer-x/gates/106.4.3.sh) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/gates/106.4.3.sh](../../../tools/paxeer-x/gates/106.4.3.sh) |
| `env_var` | `PAXEER_X_106_4_6_BUNDLE` | [tools/paxeer-x/gates/106.4.6.sh](../../../tools/paxeer-x/gates/106.4.6.sh) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/gates/106.4.6.sh](../../../tools/paxeer-x/gates/106.4.6.sh) |
| `env_var` | `PAXEER_X_106_4_9_BUNDLE` | [tools/paxeer-x/gates/106.4.9.sh](../../../tools/paxeer-x/gates/106.4.9.sh) |
| `env_var` | `PAXEER_X_COMMITTED_RPC` | [tools/paxeer-x/gates/106.4.9.sh](../../../tools/paxeer-x/gates/106.4.9.sh) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/gates/106.4.9.sh](../../../tools/paxeer-x/gates/106.4.9.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/gates/110.8.2.sh](../../../tools/paxeer-x/gates/110.8.2.sh) |
| `env_var` | `PAXEER_X_GATEWAY_MAINTENANCE_ARTIFACTS` | [tools/paxeer-x/gates/16.6.sh](../../../tools/paxeer-x/gates/16.6.sh) |
| `env_var` | `LAYERX_MULTI_ASSET_BUILD_REVISION` | [tools/paxeer-x/gates/5.1.sh](../../../tools/paxeer-x/gates/5.1.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/release.sh](../../../tools/paxeer-x/release.sh) |
| `env_var` | `PAXEER_X_QUALIFICATION_BUILD_RECORD` | [tools/paxeer-x/tests/durable_release_evidence.py](../../../tools/paxeer-x/tests/durable_release_evidence.py) |
| `env_var` | `PAXEER_X_CANDIDATE_MANIFEST` | [tools/paxeer-x/tests/routes_test.py](../../../tools/paxeer-x/tests/routes_test.py) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/tests/routes_test.py](../../../tools/paxeer-x/tests/routes_test.py) |
| `env_var` | `PAXEER_X_ROUTE_CASES` | [tools/paxeer-x/tests/routes_test.py](../../../tools/paxeer-x/tests/routes_test.py) |
| `env_var` | `PAXEER_X_CANDIDATE_MANIFEST` | [tools/paxeer-x/verify-task.sh](../../../tools/paxeer-x/verify-task.sh) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/paxeer-x/verify-task.sh](../../../tools/paxeer-x/verify-task.sh) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/paxeer-x/verify-task.sh](../../../tools/paxeer-x/verify-task.sh) |
| source | — | [tools/paxeer-x/gates/103.6.3.sh](../../../tools/paxeer-x/gates/103.6.3.sh) |
| source | — | [tools/paxeer-x/gates/103.6.4.sh](../../../tools/paxeer-x/gates/103.6.4.sh) |
| source | — | [tools/paxeer-x/gates/103.6.5.sh](../../../tools/paxeer-x/gates/103.6.5.sh) |
| source | — | [tools/paxeer-x/gates/103.6.9.sh](../../../tools/paxeer-x/gates/103.6.9.sh) |
| source | — | [tools/paxeer-x/gates/104.13.6.sh](../../../tools/paxeer-x/gates/104.13.6.sh) |
| source | — | [tools/paxeer-x/gates/104.15.4.sh](../../../tools/paxeer-x/gates/104.15.4.sh) |
| source | — | [tools/paxeer-x/gates/104.16.2.sh](../../../tools/paxeer-x/gates/104.16.2.sh) |
| source | — | [tools/paxeer-x/gates/104.18.2.sh](../../../tools/paxeer-x/gates/104.18.2.sh) |
| source | — | [tools/paxeer-x/gates/104.19.2.sh](../../../tools/paxeer-x/gates/104.19.2.sh) |
| source | — | [tools/paxeer-x/gates/104.19.3.sh](../../../tools/paxeer-x/gates/104.19.3.sh) |
| source | — | [tools/paxeer-x/gates/104.24.2.sh](../../../tools/paxeer-x/gates/104.24.2.sh) |
| source | — | [tools/paxeer-x/gates/104.30.5.sh](../../../tools/paxeer-x/gates/104.30.5.sh) |
| source | — | [tools/paxeer-x/gates/104.31.6.sh](../../../tools/paxeer-x/gates/104.31.6.sh) |
| source | — | [tools/paxeer-x/gates/104.33.1.sh](../../../tools/paxeer-x/gates/104.33.1.sh) |
| source | — | [tools/paxeer-x/gates/104.33.3.sh](../../../tools/paxeer-x/gates/104.33.3.sh) |
| source | — | [tools/paxeer-x/gates/104.34.1.sh](../../../tools/paxeer-x/gates/104.34.1.sh) |
| source | — | [tools/paxeer-x/gates/104.34.2.sh](../../../tools/paxeer-x/gates/104.34.2.sh) |
| source | — | [tools/paxeer-x/gates/104.34.3.sh](../../../tools/paxeer-x/gates/104.34.3.sh) |
| source | — | [tools/paxeer-x/gates/104.34.4.sh](../../../tools/paxeer-x/gates/104.34.4.sh) |
| source | — | [tools/paxeer-x/gates/104.34.5.sh](../../../tools/paxeer-x/gates/104.34.5.sh) |
| source | — | [tools/paxeer-x/gates/104.34.6.sh](../../../tools/paxeer-x/gates/104.34.6.sh) |
| source | — | [tools/paxeer-x/gates/104.37.2.sh](../../../tools/paxeer-x/gates/104.37.2.sh) |
| source | — | [tools/paxeer-x/gates/104.38.1.sh](../../../tools/paxeer-x/gates/104.38.1.sh) |
| source | — | [tools/paxeer-x/gates/104.38.10.sh](../../../tools/paxeer-x/gates/104.38.10.sh) |
| source | — | [tools/paxeer-x/gates/104.38.4.sh](../../../tools/paxeer-x/gates/104.38.4.sh) |
| source | — | [tools/paxeer-x/gates/104.38.9.sh](../../../tools/paxeer-x/gates/104.38.9.sh) |
| source | — | [tools/paxeer-x/gates/108.1.10.sh](../../../tools/paxeer-x/gates/108.1.10.sh) |
| source | — | [tools/paxeer-x/gates/111.6.11.sh](../../../tools/paxeer-x/gates/111.6.11.sh) |
| source | — | [tools/paxeer-x/gates/111.6.12.sh](../../../tools/paxeer-x/gates/111.6.12.sh) |
| source | — | [tools/paxeer-x/gates/16.2.sh](../../../tools/paxeer-x/gates/16.2.sh) |
| source | — | [tools/paxeer-x/gates/17.1.sh](../../../tools/paxeer-x/gates/17.1.sh) |
| source | — | [tools/paxeer-x/gates/18.3.sh](../../../tools/paxeer-x/gates/18.3.sh) |
| source | — | [tools/paxeer-x/gates/20.2.sh](../../../tools/paxeer-x/gates/20.2.sh) |
| source | — | [tools/paxeer-x/gates/21.2.sh](../../../tools/paxeer-x/gates/21.2.sh) |
| source | — | [tools/paxeer-x/gates/24.14.sh](../../../tools/paxeer-x/gates/24.14.sh) |
| source | — | [tools/paxeer-x/gates/24.15.sh](../../../tools/paxeer-x/gates/24.15.sh) |
| source | — | [tools/paxeer-x/gates/24.16.sh](../../../tools/paxeer-x/gates/24.16.sh) |
| source | — | [tools/paxeer-x/gates/24.18.sh](../../../tools/paxeer-x/gates/24.18.sh) |
| source | — | [tools/paxeer-x/gates/24.20.sh](../../../tools/paxeer-x/gates/24.20.sh) |
| source | — | [tools/paxeer-x/gates/25.2.sh](../../../tools/paxeer-x/gates/25.2.sh) |
| source | — | [tools/paxeer-x/gov/render.sh](../../../tools/paxeer-x/gov/render.sh) |
| source | — | [tools/paxeer-x/gov/test.sh](../../../tools/paxeer-x/gov/test.sh) |
| source | — | [tools/paxeer-x/tests/ai_evidence_test.py](../../../tools/paxeer-x/tests/ai_evidence_test.py) |

## pxx-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/paxeer-x/gov/templates/01-software-upgrade.json](../../../tools/paxeer-x/gov/templates/01-software-upgrade.json) |
| source | — | [tools/paxeer-x/gov/templates/02-fee-token-params.json](../../../tools/paxeer-x/gov/templates/02-fee-token-params.json) |
| source | — | [tools/paxeer-x/gov/templates/03-xweb-fee.json](../../../tools/paxeer-x/gov/templates/03-xweb-fee.json) |
| source | — | [tools/paxeer-x/retired_callers.json](../../../tools/paxeer-x/retired_callers.json) |
| source | — | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |

## qual-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `parser = argparse.ArgumentParser(prog="beta_driver")` | [tools/qualification/beta_driver.py](../../../tools/qualification/beta_driver.py) |
| `cli_command` | `run.add_argument("--gate", required=True, choices=DRIVER_GATES)` | [tools/qualification/beta_driver.py](../../../tools/qualification/beta_driver.py) |
| `cli_command` | `run.add_argument("--output", required=True)` | [tools/qualification/beta_driver.py](../../../tools/qualification/beta_driver.py) |
| `cli_command` | `run.add_argument("--source-identity", required=True)` | [tools/qualification/beta_driver.py](../../../tools/qualification/beta_driver.py) |
| `cli_command` | `run = commands.add_parser("run")` | [tools/qualification/beta_driver.py](../../../tools/qualification/beta_driver.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/agent_http_bounds.py](../../../tools/qualification/paxeer-x/agent_http_bounds.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/agent_operation_envelope.py](../../../tools/qualification/paxeer-x/agent_operation_envelope.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/agent_tenant_readiness.py](../../../tools/qualification/paxeer-x/agent_tenant_readiness.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/arbiter_prestate.py](../../../tools/qualification/paxeer-x/arbiter_prestate.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/arbiter_prestate.py](../../../tools/qualification/paxeer-x/arbiter_prestate.py) |
| `cli_command` | `parser.add_argument("--build-dir", default="/root/lx-target/arbiter-prestate")` | [tools/qualification/paxeer-x/arbiter_prestate.py](../../../tools/qualification/paxeer-x/arbiter_prestate.py) |
| `cli_command` | `parser.add_argument("--cargo", default="cargo")` | [tools/qualification/paxeer-x/arbiter_prestate.py](../../../tools/qualification/paxeer-x/arbiter_prestate.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/asset-execution-prestate.py](../../../tools/qualification/paxeer-x/asset-execution-prestate.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/asset-execution-prestate.py](../../../tools/qualification/paxeer-x/asset-execution-prestate.py) |
| `cli_command` | `parser.add_argument("--cargo", default="cargo")` | [tools/qualification/paxeer-x/asset-execution-prestate.py](../../../tools/qualification/paxeer-x/asset-execution-prestate.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/asset-inventory.py](../../../tools/qualification/paxeer-x/asset-inventory.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/asset-inventory.py](../../../tools/qualification/paxeer-x/asset-inventory.py) |
| `cli_command` | `parser.add_argument("--cargo", default="cargo")` | [tools/qualification/paxeer-x/asset-inventory.py](../../../tools/qualification/paxeer-x/asset-inventory.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/authenticated-replay-step.py](../../../tools/qualification/paxeer-x/authenticated-replay-step.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/authenticated-replay-step.py](../../../tools/qualification/paxeer-x/authenticated-replay-step.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/budget-proof-exports.py](../../../tools/qualification/paxeer-x/budget-proof-exports.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/budget-proof-intake.py](../../../tools/qualification/paxeer-x/budget-proof-intake.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/caps-codecs.py](../../../tools/qualification/paxeer-x/caps-codecs.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `cli_command` | `compiler, *args = sys.argv[1:]` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `cli_command` | `if len(sys.argv) > 1 and not sys.argv[1].startswith('--'):` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser.add_argument('--manifest')` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser.add_argument('--matrix-key', choices=tuple(MATRIX))` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser.add_argument('--phase', choices=('build', 'run', 'collect', 'gate'), default='build')` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser.add_argument('--repository')` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser.add_argument('--run-id')` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `cli_command` | `parser.add_argument('--candidate-manifest', required=True)` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `cli_command` | `parser.add_argument('--case', choices=tuple(CASES), required=True)` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/explorer-availability.py](../../../tools/qualification/paxeer-x/explorer-availability.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/explorer-receipt-reader.py](../../../tools/qualification/paxeer-x/explorer-receipt-reader.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/external_custody.py](../../../tools/qualification/paxeer-x/external_custody.py) |
| `cli_command` | `require(sys.argv[1:] in ([], ['--build']), 'only one bounded build or focused verification supported')` | [tools/qualification/paxeer-x/external_custody.py](../../../tools/qualification/paxeer-x/external_custody.py) |
| `cli_command` | `if __name__ == '__main__' and len(sys.argv) == 7 and sys.argv[1] == 'serve':` | [tools/qualification/paxeer-x/fixtures/event_receiver_fixture.py](../../../tools/qualification/paxeer-x/fixtures/event_receiver_fixture.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__)` | [tools/qualification/paxeer-x/fixtures/xweb_attestation.py](../../../tools/qualification/paxeer-x/fixtures/xweb_attestation.py) |
| `cli_command` | `parser.add_argument("--candidate-manifest", required=True)` | [tools/qualification/paxeer-x/fixtures/xweb_attestation.py](../../../tools/qualification/paxeer-x/fixtures/xweb_attestation.py) |
| `cli_command` | `parser.add_argument("--case", required=True, choices=sorted(CASES))` | [tools/qualification/paxeer-x/fixtures/xweb_attestation.py](../../../tools/qualification/paxeer-x/fixtures/xweb_attestation.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/fixtures/xweb_fixture.py](../../../tools/qualification/paxeer-x/fixtures/xweb_fixture.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `cli_command` | `parser.add_argument('--candidate-manifest', required=True)` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `cli_command` | `parser.add_argument('--case', required=True, choices=['station-autonomous-recovery', 'first-use-sponsorship', 'quote-liability-lifecycle', 'rate-publication-recovery', 'browser-sponsorship-origin'])` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/hpx-ingress.py](../../../tools/qualification/paxeer-x/hpx-ingress.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/hpx-ingress.py](../../../tools/qualification/paxeer-x/hpx-ingress.py) |
| `cli_command` | `parser.add_argument('--repair-fixture-build', action='store_true')` | [tools/qualification/paxeer-x/hpx-ingress.py](../../../tools/qualification/paxeer-x/hpx-ingress.py) |
| `cli_command` | `parser.add_argument('--repair-hpx-build', action='store_true')` | [tools/qualification/paxeer-x/hpx-ingress.py](../../../tools/qualification/paxeer-x/hpx-ingress.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/human-api-boundary.py](../../../tools/qualification/paxeer-x/human-api-boundary.py) |
| `cli_command` | `parser.add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/human-api-boundary.py](../../../tools/qualification/paxeer-x/human-api-boundary.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human-approval-admission.py](../../../tools/qualification/paxeer-x/human-approval-admission.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human_attestor_intents.py](../../../tools/qualification/paxeer-x/human_attestor_intents.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/human-canonical-projections.py](../../../tools/qualification/paxeer-x/human-canonical-projections.py) |
| `cli_command` | `parser.add_argument('--candidate-manifest', required=True)` | [tools/qualification/paxeer-x/human-canonical-projections.py](../../../tools/qualification/paxeer-x/human-canonical-projections.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human_deposit_initial_response.py](../../../tools/qualification/paxeer-x/human_deposit_initial_response.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human_journey_projections.py](../../../tools/qualification/paxeer-x/human_journey_projections.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/human_native_custody.py](../../../tools/qualification/paxeer-x/human_native_custody.py) |
| `cli_command` | `mode.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/human_native_custody.py](../../../tools/qualification/paxeer-x/human_native_custody.py) |
| `cli_command` | `mode.add_argument('--candidate-manifest')` | [tools/qualification/paxeer-x/human_native_custody.py](../../../tools/qualification/paxeer-x/human_native_custody.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/human-native-send-producer.py](../../../tools/qualification/paxeer-x/human-native-send-producer.py) |
| `cli_command` | `parser.add_argument('--candidate-manifest', required=True)` | [tools/qualification/paxeer-x/human-native-send-producer.py](../../../tools/qualification/paxeer-x/human-native-send-producer.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/human-owner-evidence.py](../../../tools/qualification/paxeer-x/human-owner-evidence.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human-program-approvals.py](../../../tools/qualification/paxeer-x/human-program-approvals.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/human_program_approvals.py](../../../tools/qualification/paxeer-x/human_program_approvals.py) |
| `cli_command` | `parser.add_argument('--candidate-manifest', required=True)` | [tools/qualification/paxeer-x/human_program_approvals.py](../../../tools/qualification/paxeer-x/human_program_approvals.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human-projection-facts.py](../../../tools/qualification/paxeer-x/human-projection-facts.py) |
| `cli_command` | `modes.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/human-service-contract.py](../../../tools/qualification/paxeer-x/human-service-contract.py) |
| `cli_command` | `modes.add_argument('--build-clock', action='store_true')` | [tools/qualification/paxeer-x/human-service-contract.py](../../../tools/qualification/paxeer-x/human-service-contract.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human_stream_push.py](../../../tools/qualification/paxeer-x/human_stream_push.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/human_unified_custody_shapes.py](../../../tools/qualification/paxeer-x/human_unified_custody_shapes.py) |
| `cli_command` | `if sys.argv[1:] == ['--build']:` | [tools/qualification/paxeer-x/human_unified_custody_shapes.py](../../../tools/qualification/paxeer-x/human_unified_custody_shapes.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/identity-assertion-binding.py](../../../tools/qualification/paxeer-x/identity-assertion-binding.py) |
| `cli_command` | `parser = argparse.ArgumentParser(prog="interop-archive-contract")` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `cli_command` | `parser.add_argument("--candidate-manifest", required=True)` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `cli_command` | `parser.add_argument("--case", required=True, choices=CASES)` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/market-sandbox-capture.py](../../../tools/qualification/paxeer-x/market-sandbox-capture.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/market-sandbox-capture.py](../../../tools/qualification/paxeer-x/market-sandbox-capture.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/market-sandbox-profile.py](../../../tools/qualification/paxeer-x/market-sandbox-profile.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/market-sandbox-profile.py](../../../tools/qualification/paxeer-x/market-sandbox-profile.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/mcp_binding_listener.py](../../../tools/qualification/paxeer-x/mcp_binding_listener.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/mcp_daemon_revocation.py](../../../tools/qualification/paxeer-x/mcp_daemon_revocation.py) |
| `cli_command` | `if sys.argv[1:] == ["--worker"]:` | [tools/qualification/paxeer-x/mcp_daemon_revocation.py](../../../tools/qualification/paxeer-x/mcp_daemon_revocation.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/mcp_served_writes.py](../../../tools/qualification/paxeer-x/mcp_served_writes.py) |
| `cli_command` | `if sys.argv[1:] == ["--worker"]:` | [tools/qualification/paxeer-x/mcp_served_writes.py](../../../tools/qualification/paxeer-x/mcp_served_writes.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `cli_command` | `parser.add_argument('--build-artifacts', action='store_true')` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `cli_command` | `parser.add_argument('--worker', action='store_true', help=argparse.SUPPRESS)` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `cli_command` | `sys.exit(corpus() if sys.argv[1:] == ["--corpus"] else gate())` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/migration-v2.py](../../../tools/qualification/paxeer-x/migration-v2.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/movement-custody-profile.py](../../../tools/qualification/paxeer-x/movement-custody-profile.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/movement-executor-readiness.py](../../../tools/qualification/paxeer-x/movement-executor-readiness.py) |
| `cli_command` | `if sys.argv[1:] == ['--build']:` | [tools/qualification/paxeer-x/movement-executor-readiness.py](../../../tools/qualification/paxeer-x/movement-executor-readiness.py) |
| `cli_command` | `arguments = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/native-effect-sdk.py](../../../tools/qualification/paxeer-x/native-effect-sdk.py) |
| `cli_command` | `arguments.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/native-effect-sdk.py](../../../tools/qualification/paxeer-x/native-effect-sdk.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/native-journey-approval.py](../../../tools/qualification/paxeer-x/native-journey-approval.py) |
| `cli_command` | `require(not sys.argv[1:], "unsupported gate arguments")` | [tools/qualification/paxeer-x/native-journey-approval.py](../../../tools/qualification/paxeer-x/native-journey-approval.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/native-program-human-peer.py](../../../tools/qualification/paxeer-x/native-program-human-peer.py) |
| `cli_command` | `if sys.argv[1:] == ['--build']:` | [tools/qualification/paxeer-x/native-program-human-peer.py](../../../tools/qualification/paxeer-x/native-program-human-peer.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/native-program-presentation.py](../../../tools/qualification/paxeer-x/native-program-presentation.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/native-programs-link.py](../../../tools/qualification/paxeer-x/native-programs-link.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/native-programs-link.py](../../../tools/qualification/paxeer-x/native-programs-link.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/native_send_profile.py](../../../tools/qualification/paxeer-x/native_send_profile.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `cli_command` | `mode.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `cli_command` | `mode.add_argument('--candidate-manifest')` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/native-terminal-outcome.py](../../../tools/qualification/paxeer-x/native-terminal-outcome.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `cli_command` | `parser.add_argument("--candidate-manifest", required=True)` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/private-human-custody.py](../../../tools/qualification/paxeer-x/private-human-custody.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/private-human-custody.py](../../../tools/qualification/paxeer-x/private-human-custody.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/private-human-sponsored.py](../../../tools/qualification/paxeer-x/private-human-sponsored.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/private-human-sponsored.py](../../../tools/qualification/paxeer-x/private-human-sponsored.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-abi-freeze.py](../../../tools/qualification/paxeer-x/program-abi-freeze.py) |
| `cli_command` | `sub.add_parser('verify').add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/program-abi-freeze.py](../../../tools/qualification/paxeer-x/program-abi-freeze.py) |
| `cli_command` | `sub.add_parser('build').add_argument('--output', required=True)` | [tools/qualification/paxeer-x/program-abi-freeze.py](../../../tools/qualification/paxeer-x/program-abi-freeze.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-abi-policy.py](../../../tools/qualification/paxeer-x/program-abi-policy.py) |
| `cli_command` | `sub.add_parser('verify').add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/program-abi-policy.py](../../../tools/qualification/paxeer-x/program-abi-policy.py) |
| `cli_command` | `sub.add_parser('build').add_argument('--output', required=True)` | [tools/qualification/paxeer-x/program-abi-policy.py](../../../tools/qualification/paxeer-x/program-abi-policy.py) |
| `cli_command` | `before.add_argument('--cargo', required=True)` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `cli_command` | `before.add_argument('--cflags', required=True)` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `cli_command` | `before.add_argument('--compiler', required=True)` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `cli_command` | `before.add_argument('--cppflags', required=True)` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `cli_command` | `before.add_argument('--ldflags', required=True)` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `cli_command` | `producer.add_argument('--' + name, required=True)` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-bindings.py](../../../tools/qualification/paxeer-x/program-bindings.py) |
| `cli_command` | `parser.add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/program-bindings.py](../../../tools/qualification/paxeer-x/program-bindings.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-bindings-producer.py](../../../tools/qualification/paxeer-x/program-bindings-producer.py) |
| `cli_command` | `parser.add_argument('--deadline-epoch', type=float)` | [tools/qualification/paxeer-x/program-bindings-producer.py](../../../tools/qualification/paxeer-x/program-bindings-producer.py) |
| `cli_command` | `parser.add_argument('--interface-result', required=True)` | [tools/qualification/paxeer-x/program-bindings-producer.py](../../../tools/qualification/paxeer-x/program-bindings-producer.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [tools/qualification/paxeer-x/program-bindings-producer.py](../../../tools/qualification/paxeer-x/program-bindings-producer.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `before.add_argument('--cargo', required=True)` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `before.add_argument('--cflags', required=True)` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `before.add_argument('--compiler', required=True)` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `before.add_argument('--cppflags', required=True)` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `before.add_argument('--ldflags', required=True)` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `producer.add_argument('--' + name, required=True)` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `producer = sub.add_parser('record')` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `before = sub.add_parser('snapshot')` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `gate = sub.add_parser('verify')` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__)` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `cli_command` | `parser.add_argument("--build-dir", default="build")` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `cli_command` | `parser.add_argument("--cc", default=os.environ.get("CC", "cc"))` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `cli_command` | `parser.add_argument("--build-dir", default="build")` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `cli_command` | `parser.add_argument("--cc", default=os.environ.get("CC", "cc"))` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-interface.py](../../../tools/qualification/paxeer-x/program-interface.py) |
| `cli_command` | `parser.add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/program-interface.py](../../../tools/qualification/paxeer-x/program-interface.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-interface-producer.py](../../../tools/qualification/paxeer-x/program-interface-producer.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [tools/qualification/paxeer-x/program-interface-producer.py](../../../tools/qualification/paxeer-x/program-interface-producer.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `mode.add_argument("--begin-build", action="store_true")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `mode.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--cargo", default="cargo")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--cc", default="cc")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--cflags", default="")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--ldflags", default="")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--native-library", default="build/liblayerx.a")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--native-output", default="build/tests/programs_market_attestation")` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument("--target-dir", default=str(ROOT / "programs/target"))` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `cli_command` | `parser.add_argument('--build-dir', default='build')` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `cli_command` | `parser.add_argument('--cc', default='cc')` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-storage.py](../../../tools/qualification/paxeer-x/program-storage.py) |
| `cli_command` | `parser.add_argument('--balance-evidence', required=True)` | [tools/qualification/paxeer-x/program-storage.py](../../../tools/qualification/paxeer-x/program-storage.py) |
| `cli_command` | `parser.add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/program-storage.py](../../../tools/qualification/paxeer-x/program-storage.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/program-storage-producer.py](../../../tools/qualification/paxeer-x/program-storage-producer.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [tools/qualification/paxeer-x/program-storage-producer.py](../../../tools/qualification/paxeer-x/program-storage-producer.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/program_transfer_authority.py](../../../tools/qualification/paxeer-x/program_transfer_authority.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| `cli_command` | `parser.add_argument('--manifest')` | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| `cli_command` | `parser.add_argument('--task', choices=('6.4', '6.6'), default='6.4')` | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__)` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `cli_command` | `parser.add_argument("--build-output", help="explicitly build the focused native candidate into a new private directory")` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `cli_command` | `parser.add_argument("--manifest", default=os.environ.get("PAXEER_X_PROGRAM_VALIDATION_MANIFEST"))` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/programs_market_resolution.py](../../../tools/qualification/paxeer-x/programs_market_resolution.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/programs_native_interfaces.py](../../../tools/qualification/paxeer-x/programs_native_interfaces.py) |
| `cli_command` | `parser.add_argument('--manifest', default=os.environ.get('PAXEER_X_NATIVE_INTERFACES_MANIFEST'))` | [tools/qualification/paxeer-x/programs_native_interfaces.py](../../../tools/qualification/paxeer-x/programs_native_interfaces.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/programs_native_migration.py](../../../tools/qualification/paxeer-x/programs_native_migration.py) |
| `cli_command` | `parser.add_argument('--build-artifacts', action='store_true')` | [tools/qualification/paxeer-x/programs_native_migration.py](../../../tools/qualification/paxeer-x/programs_native_migration.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__)` | [tools/qualification/paxeer-x/programs_sandbox_bounds.py](../../../tools/qualification/paxeer-x/programs_sandbox_bounds.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true", help="build only the declared native ASan target and its production dependencies")` | [tools/qualification/paxeer-x/programs_sandbox_bounds.py](../../../tools/qualification/paxeer-x/programs_sandbox_bounds.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__)` | [tools/qualification/paxeer-x/programs_source_abi.py](../../../tools/qualification/paxeer-x/programs_source_abi.py) |
| `cli_command` | `group.add_argument('--record-artifacts', metavar='CARGO_JSON')` | [tools/qualification/paxeer-x/programs_source_abi.py](../../../tools/qualification/paxeer-x/programs_source_abi.py) |
| `cli_command` | `group.add_argument('--snapshot-build', action='store_true')` | [tools/qualification/paxeer-x/programs_source_abi.py](../../../tools/qualification/paxeer-x/programs_source_abi.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/programs_typed_interfaces.py](../../../tools/qualification/paxeer-x/programs_typed_interfaces.py) |
| `cli_command` | `parser.add_argument('--manifest', default=os.environ.get('PAXEER_X_TYPED_INTERFACES_MANIFEST'))` | [tools/qualification/paxeer-x/programs_typed_interfaces.py](../../../tools/qualification/paxeer-x/programs_typed_interfaces.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/ramp-source-settlement.py](../../../tools/qualification/paxeer-x/ramp-source-settlement.py) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `cli_command` | `parser.add_argument('--candidate-manifest', required=True)` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `cli_command` | `parser.add_argument('--case', required=True, choices=sorted(CASES))` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `cli_command` | `parser.add_argument('--corpus')` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `cli_command` | `parser.add_argument('--emulator-url')` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `cli_command` | `parser.add_argument('--hosted-url')` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `cli_command` | `parser = argparse.ArgumentParser(` | [tools/qualification/paxeer-x/registry-router-bootstrap.py](../../../tools/qualification/paxeer-x/registry-router-bootstrap.py) |
| `cli_command` | `worker.add_argument('--cell', required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `worker.add_argument('--controller-binary', required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `submit.add_argument('--controller-socket', required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `status.add_argument('--logical-id', required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `worker.add_argument('--output', required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `submit.add_argument('--request', required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `plan.add_argument('--' + key, required=True)` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/replay-vendor-closure.py](../../../tools/qualification/paxeer-x/replay-vendor-closure.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/replay-vendor-closure.py](../../../tools/qualification/paxeer-x/replay-vendor-closure.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/router-authority-schema.py](../../../tools/qualification/paxeer-x/router-authority-schema.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/runtime-boundary-witness.py](../../../tools/qualification/paxeer-x/runtime-boundary-witness.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/single-step-conformance.py](../../../tools/qualification/paxeer-x/single-step-conformance.py) |
| `cli_command` | `parser.add_argument("--build", action="store_true")` | [tools/qualification/paxeer-x/single-step-conformance.py](../../../tools/qualification/paxeer-x/single-step-conformance.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/storage-recovery.py](../../../tools/qualification/paxeer-x/storage-recovery.py) |
| `cli_command` | `parser.add_argument('--manifest', required=True)` | [tools/qualification/paxeer-x/storage-recovery.py](../../../tools/qualification/paxeer-x/storage-recovery.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/storage-recovery-producer.py](../../../tools/qualification/paxeer-x/storage-recovery-producer.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [tools/qualification/paxeer-x/storage-recovery-producer.py](../../../tools/qualification/paxeer-x/storage-recovery-producer.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/unified-account-identity.py](../../../tools/qualification/paxeer-x/unified-account-identity.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/paxeer-x/wallet-attestor-disclosure.py](../../../tools/qualification/paxeer-x/wallet-attestor-disclosure.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-ceremony-resume.py](../../../tools/qualification/paxeer-x/wallet-ceremony-resume.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-custody-recovery.py](../../../tools/qualification/paxeer-x/wallet-custody-recovery.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-custody-routing.py](../../../tools/qualification/paxeer-x/wallet-custody-routing.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `cli_command` | `import React, { useCallback, useEffect, useRef, useState } from 'react';` | [tools/qualification/paxeer-x/wallet-injected-entry.tsx](../../../tools/qualification/paxeer-x/wallet-injected-entry.tsx) |
| `cli_command` | `import fs from 'node:fs/promises';` | [tools/qualification/paxeer-x/wallet-injected-fixture.mjs](../../../tools/qualification/paxeer-x/wallet-injected-fixture.mjs) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `cli_command` | `group.add_argument('--build', action='store_true')` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `cli_command` | `if (process.argv.length !== 4 \|\| process.argv[2] !== '--manifest') fail('use --manifest ABS_PRIVATE_FILE');` | [tools/qualification/paxeer-x/wallet-injected-fixture.mjs](../../../tools/qualification/paxeer-x/wallet-injected-fixture.mjs) |
| `cli_command` | `group.add_argument('--provision', action='store_true')` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `cli_command` | `group.add_argument('--verify-baseline', action='store_true')` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-signing-adapters.py](../../../tools/qualification/paxeer-x/wallet-signing-adapters.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-sponsored-fees.py](../../../tools/qualification/paxeer-x/wallet-sponsored-fees.py) |
| `cli_command` | `require(sys.argv[1:] in ([], ['--build']), 'only --build or focused verification is supported')` | [tools/qualification/paxeer-x/wallet-sponsored-fees.py](../../../tools/qualification/paxeer-x/wallet-sponsored-fees.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-supported-demo.py](../../../tools/qualification/paxeer-x/wallet-supported-demo.py) |
| `cli_command` | `if sys.argv[1:] == ['--build']:` | [tools/qualification/paxeer-x/wallet-supported-demo.py](../../../tools/qualification/paxeer-x/wallet-supported-demo.py) |
| `cli_command` | `if __name__ == '__main__':` | [tools/qualification/paxeer-x/wallet-transfer-truth.py](../../../tools/qualification/paxeer-x/wallet-transfer-truth.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/release_runner.py](../../../tools/qualification/release_runner.py) |
| `cli_command` | `raise SystemExit(main(sys.argv[1:]))` | [tools/qualification/release_runner.py](../../../tools/qualification/release_runner.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/test_beta_driver.py](../../../tools/qualification/test_beta_driver.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/test_registry_router_bootstrap.py](../../../tools/qualification/test_registry_router_bootstrap.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/qualification/test_release_runner.py](../../../tools/qualification/test_release_runner.py) |
| source | — | [tools/qualification/paxeer-x/authority-evidence-read.py](../../../tools/qualification/paxeer-x/authority-evidence-read.py) |
| source | — | [tools/qualification/paxeer-x/build-caps-codecs.sh](../../../tools/qualification/paxeer-x/build-caps-codecs.sh) |
| source | — | [tools/qualification/paxeer-x/build-caps-discovery.sh](../../../tools/qualification/paxeer-x/build-caps-discovery.sh) |
| source | — | [tools/qualification/paxeer-x/caps-discovery.py](../../../tools/qualification/paxeer-x/caps-discovery.py) |
| source | — | [tools/qualification/paxeer-x/custody_governance.py](../../../tools/qualification/paxeer-x/custody_governance.py) |
| source | — | [tools/qualification/paxeer-x/fixtures/identity_fixture.py](../../../tools/qualification/paxeer-x/fixtures/identity_fixture.py) |
| source | — | [tools/qualification/paxeer-x/human-security-prerequisites.py](../../../tools/qualification/paxeer-x/human-security-prerequisites.py) |
| source | — | [tools/qualification/paxeer-x/migration-unified-transport.py](../../../tools/qualification/paxeer-x/migration-unified-transport.py) |
| source | — | [tools/qualification/paxeer-x/replay-authority.py](../../../tools/qualification/paxeer-x/replay-authority.py) |
| source | — | [tools/qualification/paxeer-x/router-complete-readiness.py](../../../tools/qualification/paxeer-x/router-complete-readiness.py) |
| source | — | [tools/qualification/paxeer-x/wallet-402-caps.py](../../../tools/qualification/paxeer-x/wallet-402-caps.py) |

## qual-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PAXEER_X_AGENT_ENVELOPE_ARTIFACT_MANIFEST` | [tools/qualification/paxeer-x/agent_operation_envelope.py](../../../tools/qualification/paxeer-x/agent_operation_envelope.py) |
| `env_var` | `PAXEER_X_AGENT_ENVELOPE_CONFIG` | [tools/qualification/paxeer-x/agent_operation_envelope.py](../../../tools/qualification/paxeer-x/agent_operation_envelope.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/agent_operation_envelope.py](../../../tools/qualification/paxeer-x/agent_operation_envelope.py) |
| `env_var` | `LAYERX_AUTHENTICATED_REPLAY_EVIDENCE` | [tools/qualification/paxeer-x/authenticated-replay-step.py](../../../tools/qualification/paxeer-x/authenticated-replay-step.py) |
| `env_var` | `HEAD, RUSTC` | [tools/qualification/paxeer-x/build-caps-codecs.sh](../../../tools/qualification/paxeer-x/build-caps-codecs.sh) |
| `env_var` | `CAPS_BUILD_DIR` | [tools/qualification/paxeer-x/caps-codecs.py](../../../tools/qualification/paxeer-x/caps-codecs.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/caps-codecs.py](../../../tools/qualification/paxeer-x/caps-codecs.py) |
| `env_var` | `PAXEER_X_RUNTIME_ARTIFACTS` | [tools/qualification/paxeer-x/caps-codecs.py](../../../tools/qualification/paxeer-x/caps-codecs.py) |
| `env_var` | `PAXEER_X_RUNTIME_CLIENT_MANIFEST` | [tools/qualification/paxeer-x/caps-codecs.py](../../../tools/qualification/paxeer-x/caps-codecs.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `env_var` | `PAXEER_X_CUSTODY_PHASE` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `env_var` | `PAXEER_X_CUSTODY_PROVIDER_MANIFEST` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `env_var` | `RUSTC_BOOTSTRAP, RUSTDOCFLAGS, RUSTFLAGS` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `env_var` | `GITHUB_ACTIONS` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `env_var` | `PAXEER_X_DETERMINISM_BUILD_JOBS` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `env_var` | `LAYERX_EVENTS_KIND, LAYERX_EVENTS_LISTEN` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `LAYERX_KMS_LISTEN` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `LAYERX_WEBHOOKS_LISTEN` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `LAYERX_WEBHOOKS_LISTENER` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `LAYERX_WEBHOOKS_ROLE` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_EVENT_BINARY_ATTESTATION` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_EVENT_PROVISIONING` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_EVENT_PROVISIONING_WORKER` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_EVENT_UPSTREAMS` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_HOSTED_BIN_DIR` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_HUMAN_FAIRNESS` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_REGISTRY_CONFIGURATION` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_WEBHOOK_ARTIFACTS` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/explorer-availability.py](../../../tools/qualification/paxeer-x/explorer-availability.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/external_custody.py](../../../tools/qualification/paxeer-x/external_custody.py) |
| `env_var` | `PAXEER_X_EXTERNAL_CUSTODY_BUILD_MANIFEST` | [tools/qualification/paxeer-x/external_custody.py](../../../tools/qualification/paxeer-x/external_custody.py) |
| `env_var` | `PAXEER_X_RAMP_BUILD_MANIFEST` | [tools/qualification/paxeer-x/external_custody.py](../../../tools/qualification/paxeer-x/external_custody.py) |
| `env_var` | `PAXEER_X_HOSTED_BIN_DIR` | [tools/qualification/paxeer-x/fixtures/identity_fixture.py](../../../tools/qualification/paxeer-x/fixtures/identity_fixture.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/fixtures/xweb_attestation.py](../../../tools/qualification/paxeer-x/fixtures/xweb_attestation.py) |
| `env_var` | `PAXEER_X_RUNTIME_ARTIFACTS` | [tools/qualification/paxeer-x/fixtures/xweb_fixture.py](../../../tools/qualification/paxeer-x/fixtures/xweb_fixture.py) |
| `env_var` | `PAXEER_X_XWEB_ARTIFACT` | [tools/qualification/paxeer-x/fixtures/xweb_fixture.py](../../../tools/qualification/paxeer-x/fixtures/xweb_fixture.py) |
| `env_var` | `PAXEER_X_XWEB_BINARY` | [tools/qualification/paxeer-x/fixtures/xweb_fixture.py](../../../tools/qualification/paxeer-x/fixtures/xweb_fixture.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `env_var` | `PAXEER_X_RATE_RECOVERY_MATERIAL` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `env_var` | `PAXEER_X_STATION_BROWSER_MATERIAL` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `env_var` | `PAXEER_X_STATION_FIRST_USE_MATERIAL` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `env_var` | `PAXEER_X_STATION_LIABILITY_MATERIAL` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `env_var` | `PAXEER_X_STATION_RECOVERY_MATERIAL` | [tools/qualification/paxeer-x/gas-station-contract.py](../../../tools/qualification/paxeer-x/gas-station-contract.py) |
| `env_var` | `PAXEER_X_HPX_GATE_DIRECTORY` | [tools/qualification/paxeer-x/hpx-ingress.py](../../../tools/qualification/paxeer-x/hpx-ingress.py) |
| `env_var` | `PAXEER_X_CANDIDATE_MANIFEST` | [tools/qualification/paxeer-x/human_attestor_intents.py](../../../tools/qualification/paxeer-x/human_attestor_intents.py) |
| `env_var` | `PAXEER_X_HUMAN_ATTESTOR_INTENTS_MANIFEST` | [tools/qualification/paxeer-x/human_attestor_intents.py](../../../tools/qualification/paxeer-x/human_attestor_intents.py) |
| `env_var` | `PAXEER_X_HUMAN_CANONICAL_MANIFEST` | [tools/qualification/paxeer-x/human-canonical-projections.py](../../../tools/qualification/paxeer-x/human-canonical-projections.py) |
| `env_var` | `PAXEER_X_NATIVE_CUSTODY_ARTIFACT_DIRECTORY` | [tools/qualification/paxeer-x/human_native_custody.py](../../../tools/qualification/paxeer-x/human_native_custody.py) |
| `env_var` | `PAXEER_X_NATIVE_CUSTODY_BUILD_MANIFEST` | [tools/qualification/paxeer-x/human_native_custody.py](../../../tools/qualification/paxeer-x/human_native_custody.py) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_UNIX` | [tools/qualification/paxeer-x/human_native_custody.py](../../../tools/qualification/paxeer-x/human_native_custody.py) |
| `env_var` | `PAXEER_X_HUMAN_NATIVE_SEND_PRODUCER_ARTIFACTS` | [tools/qualification/paxeer-x/human-native-send-producer.py](../../../tools/qualification/paxeer-x/human-native-send-producer.py) |
| `env_var` | `PAXEER_X_HUMAN_NATIVE_SEND_PRODUCER_FIXTURE` | [tools/qualification/paxeer-x/human-native-send-producer.py](../../../tools/qualification/paxeer-x/human-native-send-producer.py) |
| `env_var` | `PAXEER_X_HUMAN_PROGRAM_APPROVAL_FIXTURE` | [tools/qualification/paxeer-x/human-program-approvals.py](../../../tools/qualification/paxeer-x/human-program-approvals.py) |
| `env_var` | `PAXEER_X_HUMAN_PROGRAM_APPROVALS_MANIFEST` | [tools/qualification/paxeer-x/human_program_approvals.py](../../../tools/qualification/paxeer-x/human_program_approvals.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/human-projection-facts.py](../../../tools/qualification/paxeer-x/human-projection-facts.py) |
| `env_var` | `PAXEER_X_HUMAN_PROJECTION_FACTS_FIXTURE` | [tools/qualification/paxeer-x/human-projection-facts.py](../../../tools/qualification/paxeer-x/human-projection-facts.py) |
| `env_var` | `CARGO` | [tools/qualification/paxeer-x/human-service-contract.py](../../../tools/qualification/paxeer-x/human-service-contract.py) |
| `env_var` | `PAXEER_X_HUMAN_STREAM_MANIFEST` | [tools/qualification/paxeer-x/human_stream_push.py](../../../tools/qualification/paxeer-x/human_stream_push.py) |
| `env_var` | `PAXEER_X_UNIFIED_SHAPES_EVIDENCE` | [tools/qualification/paxeer-x/human_unified_custody_shapes.py](../../../tools/qualification/paxeer-x/human_unified_custody_shapes.py) |
| `env_var` | `PAXEER_X_UNIFIED_SHAPES_MANIFEST` | [tools/qualification/paxeer-x/human_unified_custody_shapes.py](../../../tools/qualification/paxeer-x/human_unified_custody_shapes.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/identity-assertion-binding.py](../../../tools/qualification/paxeer-x/identity-assertion-binding.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `env_var` | `PAXEER_X_INTEROP_READINESS_INPUTS` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `env_var` | `PAXEER_X_MIRROR_CHECKPOINT_INPUTS` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `env_var` | `PAXEER_X_RAMP_READINESS_INPUTS` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `env_var` | `LAYERX_MARKET_CAPTURE_TARGET` | [tools/qualification/paxeer-x/market-sandbox-capture.py](../../../tools/qualification/paxeer-x/market-sandbox-capture.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/market-sandbox-capture.py](../../../tools/qualification/paxeer-x/market-sandbox-capture.py) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/qualification/paxeer-x/market-sandbox-capture.py](../../../tools/qualification/paxeer-x/market-sandbox-capture.py) |
| `env_var` | `LAYERX_MARKET_SANDBOX_EVIDENCE` | [tools/qualification/paxeer-x/market-sandbox-profile.py](../../../tools/qualification/paxeer-x/market-sandbox-profile.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/mcp_binding_listener.py](../../../tools/qualification/paxeer-x/mcp_binding_listener.py) |
| `env_var` | `PAXEER_X_MCP_BOOT_SECONDS` | [tools/qualification/paxeer-x/mcp_binding_listener.py](../../../tools/qualification/paxeer-x/mcp_binding_listener.py) |
| `env_var` | `PAXEER_X_MCP_BASELINE_ARTIFACTS` | [tools/qualification/paxeer-x/mcp_daemon_revocation.py](../../../tools/qualification/paxeer-x/mcp_daemon_revocation.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `env_var` | `PAXEER_X_AGENTD_UPSTREAM_ARTIFACTS` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `env_var` | `PAXEER_X_HTTP_PARENT_` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `env_var` | `LAYERX_MIGRATE_BIN` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `env_var` | `LAYERX_MIGRATION_FUNDED_OPERATOR_FIXTURE` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `env_var` | `LAYERX_MIGRATION_FUNDED_OPERATOR_RUST_LAUNCH` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `env_var` | `LAYERX_MIGRATION_FUNDED_OPERATOR_TEST_BIN` | [tools/qualification/paxeer-x/migration-funded-operator.py](../../../tools/qualification/paxeer-x/migration-funded-operator.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/migration-unified-transport.py](../../../tools/qualification/paxeer-x/migration-unified-transport.py) |
| `env_var` | `LAYERX_MIGRATION_V2_FIXTURE` | [tools/qualification/paxeer-x/migration-v2.py](../../../tools/qualification/paxeer-x/migration-v2.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/movement-executor-readiness.py](../../../tools/qualification/paxeer-x/movement-executor-readiness.py) |
| `env_var` | `PAXEER_X_MOVEMENT_READINESS_MANIFEST` | [tools/qualification/paxeer-x/movement-executor-readiness.py](../../../tools/qualification/paxeer-x/movement-executor-readiness.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/native-journey-approval.py](../../../tools/qualification/paxeer-x/native-journey-approval.py) |
| `env_var` | `PAXEER_X_NATIVE_JOURNEY_APPROVAL_FIXTURE` | [tools/qualification/paxeer-x/native-journey-approval.py](../../../tools/qualification/paxeer-x/native-journey-approval.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/native-program-human-peer.py](../../../tools/qualification/paxeer-x/native-program-human-peer.py) |
| `env_var` | `PAXEER_X_NATIVE_PROGRAM_HUMAN_PEER_FIXTURE` | [tools/qualification/paxeer-x/native-program-human-peer.py](../../../tools/qualification/paxeer-x/native-program-human-peer.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/native-program-presentation.py](../../../tools/qualification/paxeer-x/native-program-presentation.py) |
| `env_var` | `PAXEER_X_NATIVE_PROGRAM_PRESENTATION_FIXTURE` | [tools/qualification/paxeer-x/native-program-presentation.py](../../../tools/qualification/paxeer-x/native-program-presentation.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/native-programs-link.py](../../../tools/qualification/paxeer-x/native-programs-link.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/native_send_profile.py](../../../tools/qualification/paxeer-x/native_send_profile.py) |
| `env_var` | `PAXEER_X_NATIVE_SEND_PROFILE_FIXTURE` | [tools/qualification/paxeer-x/native_send_profile.py](../../../tools/qualification/paxeer-x/native_send_profile.py) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_UNIX` | [tools/qualification/paxeer-x/native_send_profile.py](../../../tools/qualification/paxeer-x/native_send_profile.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `env_var` | `PAXEER_X_CANDIDATE_MANIFEST` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `env_var` | `PAXEER_X_NATIVE_SEND_ARTIFACT_DIRECTORY` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `env_var` | `PAXEER_X_NATIVE_SEND_BUILD_MANIFEST` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_UNIX` | [tools/qualification/paxeer-x/native_send_purpose.py](../../../tools/qualification/paxeer-x/native_send_purpose.py) |
| `env_var` | `PRIVATE_HUMAN_CUSTODY_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/private-human-custody.py](../../../tools/qualification/paxeer-x/private-human-custody.py) |
| `env_var` | `PRIVATE_HUMAN_CUSTODY_RUNTIME` | [tools/qualification/paxeer-x/private-human-custody.py](../../../tools/qualification/paxeer-x/private-human-custody.py) |
| `env_var` | `PRIVATE_HUMAN_CUSTODY_SDK_MANIFEST` | [tools/qualification/paxeer-x/private-human-custody.py](../../../tools/qualification/paxeer-x/private-human-custody.py) |
| `env_var` | `PRIVATE_HUMAN_SPONSORED_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/private-human-sponsored.py](../../../tools/qualification/paxeer-x/private-human-sponsored.py) |
| `env_var` | `PRIVATE_HUMAN_SPONSORED_RUNTIME` | [tools/qualification/paxeer-x/private-human-sponsored.py](../../../tools/qualification/paxeer-x/private-human-sponsored.py) |
| `env_var` | `PRIVATE_HUMAN_SPONSORED_SDK_MANIFEST` | [tools/qualification/paxeer-x/private-human-sponsored.py](../../../tools/qualification/paxeer-x/private-human-sponsored.py) |
| `env_var` | `PRIVATE_HUMAN_SPONSORED_SHARED_BUILD` | [tools/qualification/paxeer-x/private-human-sponsored.py](../../../tools/qualification/paxeer-x/private-human-sponsored.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/program-abi-policy.py](../../../tools/qualification/paxeer-x/program-abi-policy.py) |
| `env_var` | `PAXEER_X_BALANCE_ARTIFACT_MANIFEST` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `env_var` | `PAXEER_X_BINDINGS_BUILD_JOBS` | [tools/qualification/paxeer-x/program-bindings-producer.py](../../../tools/qualification/paxeer-x/program-bindings-producer.py) |
| `env_var` | `PAXEER_X_CACHE_ARTIFACT_MANIFEST` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/program-interface-producer.py](../../../tools/qualification/paxeer-x/program-interface-producer.py) |
| `env_var` | `PAXEER_X_INTERFACE_BUILD_JOBS` | [tools/qualification/paxeer-x/program-interface-producer.py](../../../tools/qualification/paxeer-x/program-interface-producer.py) |
| `env_var` | `PAXEER_X_ATTEST_ARTIFACT_DIR` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `env_var` | `PAXEER_X_BUILDER_ID` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `env_var` | `CARGO_TARGET_DIR` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `env_var` | `PAXEER_X_STORAGE_BUILD_JOBS` | [tools/qualification/paxeer-x/program-storage-producer.py](../../../tools/qualification/paxeer-x/program-storage-producer.py) |
| `env_var` | `BUILD_DIR` | [tools/qualification/paxeer-x/program_transfer_authority.py](../../../tools/qualification/paxeer-x/program_transfer_authority.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR, TASK_LOG_DIR` | [tools/qualification/paxeer-x/program_transfer_authority.py](../../../tools/qualification/paxeer-x/program_transfer_authority.py) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/qualification/paxeer-x/program_transfer_authority.py](../../../tools/qualification/paxeer-x/program_transfer_authority.py) |
| `env_var` | `PAXEER_X_ACCOUNTS_WINDDOWN_EVIDENCE` | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| `env_var` | `PAXEER_X_ACCOUNTS_WINDDOWN_MANIFEST` | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `env_var` | `PAXEER_X_PROGRAM_VALIDATION_BUILD_JOBS` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `env_var` | `PAXEER_X_PROGRAM_VALIDATION_MANIFEST` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `env_var` | `PAXEER_X_PROGRAMS_ASC` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `env_var` | `PAXEER_X_PROGRAMS_LANGUAGE_EVIDENCE` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `env_var` | `PAXEER_X_PROGRAMS_LANGUAGE_TARGET` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/programs_market_resolution.py](../../../tools/qualification/paxeer-x/programs_market_resolution.py) |
| `env_var` | `PROGRAMS_CARGO` | [tools/qualification/paxeer-x/programs_market_resolution.py](../../../tools/qualification/paxeer-x/programs_market_resolution.py) |
| `env_var` | `PAXEER_X_NATIVE_INTERFACES_MANIFEST` | [tools/qualification/paxeer-x/programs_native_interfaces.py](../../../tools/qualification/paxeer-x/programs_native_interfaces.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/programs_sandbox_bounds.py](../../../tools/qualification/paxeer-x/programs_sandbox_bounds.py) |
| `env_var` | `PAXEER_X_TYPED_INTERFACES_MANIFEST` | [tools/qualification/paxeer-x/programs_typed_interfaces.py](../../../tools/qualification/paxeer-x/programs_typed_interfaces.py) |
| `env_var` | `LAYERX_INTEROP_LISTEN` | [tools/qualification/paxeer-x/ramp-source-settlement.py](../../../tools/qualification/paxeer-x/ramp-source-settlement.py) |
| `env_var` | `LAYERX_RAMP_SOURCE_SETTLEMENT_FIXTURE` | [tools/qualification/paxeer-x/ramp-source-settlement.py](../../../tools/qualification/paxeer-x/ramp-source-settlement.py) |
| `env_var` | `PAXEER_X_EMULATOR_CONFORMANCE_ARTIFACTS` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `env_var` | `PAXEER_X_REGISTRY_ARTIFACT_MANIFEST` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `env_var` | `GITHUB_RUN_ATTEMPT` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `GITHUB_RUN_ID` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `GITHUB_SHA` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `PAXEER_X_MAINLINE` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `PAXEER_X_REQUEST` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `PAXEER_X_REQUEST_ID` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `RUNNER_TEMP` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `env_var` | `LAYERX_REPLAY_AUTHORITY_EVIDENCE` | [tools/qualification/paxeer-x/replay-authority.py](../../../tools/qualification/paxeer-x/replay-authority.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/router-complete-readiness.py](../../../tools/qualification/paxeer-x/router-complete-readiness.py) |
| `env_var` | `LAYERX_BOUNDARY_EVIDENCE` | [tools/qualification/paxeer-x/runtime-boundary-witness.py](../../../tools/qualification/paxeer-x/runtime-boundary-witness.py) |
| `env_var` | `LAYERX_SINGLE_STEP_EVIDENCE` | [tools/qualification/paxeer-x/single-step-conformance.py](../../../tools/qualification/paxeer-x/single-step-conformance.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/storage-recovery.py](../../../tools/qualification/paxeer-x/storage-recovery.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [tools/qualification/paxeer-x/unified-account-identity.py](../../../tools/qualification/paxeer-x/unified-account-identity.py) |
| `env_var` | `PAXEER_X_IDENTITY_BUNDLE` | [tools/qualification/paxeer-x/unified-account-identity.py](../../../tools/qualification/paxeer-x/unified-account-identity.py) |
| `env_var` | `PAXEER_X_IDENTITY_FIXTURE` | [tools/qualification/paxeer-x/unified-account-identity.py](../../../tools/qualification/paxeer-x/unified-account-identity.py) |
| `env_var` | `ATTESTOR_QUORUM` | [tools/qualification/paxeer-x/wallet-402-caps.py](../../../tools/qualification/paxeer-x/wallet-402-caps.py) |
| `env_var` | `WALLET_CAPS_FIXTURE_BUNDLE` | [tools/qualification/paxeer-x/wallet-402-caps.py](../../../tools/qualification/paxeer-x/wallet-402-caps.py) |
| `env_var` | `WALLET_CEREMONY_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/wallet-ceremony-resume.py](../../../tools/qualification/paxeer-x/wallet-ceremony-resume.py) |
| `env_var` | `WALLET_CEREMONY_FIXTURE_BUNDLE` | [tools/qualification/paxeer-x/wallet-ceremony-resume.py](../../../tools/qualification/paxeer-x/wallet-ceremony-resume.py) |
| `env_var` | `ATTESTOR_QUORUM` | [tools/qualification/paxeer-x/wallet-custody-routing.py](../../../tools/qualification/paxeer-x/wallet-custody-routing.py) |
| `env_var` | `DATABASE_URL` | [tools/qualification/paxeer-x/wallet-custody-routing.py](../../../tools/qualification/paxeer-x/wallet-custody-routing.py) |
| `env_var` | `WALLET_CUSTODY_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/wallet-custody-routing.py](../../../tools/qualification/paxeer-x/wallet-custody-routing.py) |
| `env_var` | `WALLET_CUSTODY_FIXTURE_BUNDLE` | [tools/qualification/paxeer-x/wallet-custody-routing.py](../../../tools/qualification/paxeer-x/wallet-custody-routing.py) |
| `env_var` | `WALLET_INJECTED_BASELINE_RECORD` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `env_var` | `WALLET_INJECTED_CHAIN_ROOT` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `env_var` | `WALLET_INJECTED_NODE` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `env_var` | `WALLET_INJECTED_NODE_MODULES` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `env_var` | `WALLET_INJECTED_FIXTURE_ROOT` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `env_var` | `WALLET_INJECTED_NODE` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `env_var` | `WALLET_INJECTED_NODE_MODULES` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `env_var` | `ATTESTOR_QUORUM` | [tools/qualification/paxeer-x/wallet-signing-adapters.py](../../../tools/qualification/paxeer-x/wallet-signing-adapters.py) |
| `env_var` | `WALLET_SIGNING_ADAPTERS_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/wallet-signing-adapters.py](../../../tools/qualification/paxeer-x/wallet-signing-adapters.py) |
| `env_var` | `WALLET_SIGNING_ADAPTERS_FIXTURE_BUNDLE` | [tools/qualification/paxeer-x/wallet-signing-adapters.py](../../../tools/qualification/paxeer-x/wallet-signing-adapters.py) |
| `env_var` | `PATH` | [tools/qualification/paxeer-x/wallet-sponsored-fees.py](../../../tools/qualification/paxeer-x/wallet-sponsored-fees.py) |
| `env_var` | `WALLET_SPONSORED_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/wallet-sponsored-fees.py](../../../tools/qualification/paxeer-x/wallet-sponsored-fees.py) |
| `env_var` | `WALLET_SPONSORED_FIXTURE_BUNDLE` | [tools/qualification/paxeer-x/wallet-sponsored-fees.py](../../../tools/qualification/paxeer-x/wallet-sponsored-fees.py) |
| `env_var` | `WALLET_SUPPORTED_DEMO_EVIDENCE_ROOT` | [tools/qualification/paxeer-x/wallet-supported-demo.py](../../../tools/qualification/paxeer-x/wallet-supported-demo.py) |
| `env_var` | `WALLET_SUPPORTED_DEMO_FIXTURE` | [tools/qualification/paxeer-x/wallet-supported-demo.py](../../../tools/qualification/paxeer-x/wallet-supported-demo.py) |
| source | — | [tools/qualification/paxeer-x/budget-proof-intake.py](../../../tools/qualification/paxeer-x/budget-proof-intake.py) |
| source | — | [tools/qualification/paxeer-x/build-caps-discovery.sh](../../../tools/qualification/paxeer-x/build-caps-discovery.sh) |
| source | — | [tools/qualification/paxeer-x/caps-discovery.py](../../../tools/qualification/paxeer-x/caps-discovery.py) |
| source | — | [tools/qualification/paxeer-x/custody_governance.py](../../../tools/qualification/paxeer-x/custody_governance.py) |
| source | — | [tools/qualification/paxeer-x/explorer-receipt-reader.py](../../../tools/qualification/paxeer-x/explorer-receipt-reader.py) |
| source | — | [tools/qualification/paxeer-x/human-approval-admission.py](../../../tools/qualification/paxeer-x/human-approval-admission.py) |
| source | — | [tools/qualification/paxeer-x/human_deposit_initial_response.py](../../../tools/qualification/paxeer-x/human_deposit_initial_response.py) |
| source | — | [tools/qualification/paxeer-x/human_journey_projections.py](../../../tools/qualification/paxeer-x/human_journey_projections.py) |
| source | — | [tools/qualification/paxeer-x/human-security-prerequisites.py](../../../tools/qualification/paxeer-x/human-security-prerequisites.py) |
| source | — | [tools/qualification/paxeer-x/movement-custody-profile.py](../../../tools/qualification/paxeer-x/movement-custody-profile.py) |
| source | — | [tools/qualification/paxeer-x/native-terminal-outcome.py](../../../tools/qualification/paxeer-x/native-terminal-outcome.py) |
| source | — | [tools/qualification/paxeer-x/registry-router-bootstrap.py](../../../tools/qualification/paxeer-x/registry-router-bootstrap.py) |
| source | — | [tools/qualification/paxeer-x/wallet-custody-recovery.py](../../../tools/qualification/paxeer-x/wallet-custody-recovery.py) |
| source | — | [tools/qualification/test_beta_driver.py](../../../tools/qualification/test_beta_driver.py) |

## railway-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if __name__ == "__main__":` | [tools/railway/fly-files-to-table.py](../../../tools/railway/fly-files-to-table.py) |

## release-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `-Wl,-rpath,/*) rpath=1 ;;` | [tools/release/extld.sh](../../../tools/release/extld.sh) |
| `cli_command` | `parser = argparse.ArgumentParser(description=__doc__)` | [tools/release/feed_key.py](../../../tools/release/feed_key.py) |
| `cli_command` | `parser = argparse.ArgumentParser(` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser.add_argument("--check", action="store_true")` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser.add_argument("--env-file", required=True, type=Path)` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser.add_argument("--no-rehearsal", action="store_true")` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser.add_argument("--phase", choices=PHASES, default="post")` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser.add_argument("--receipt", type=Path)` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser.add_argument("--rehearsal", type=Path)` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| `cli_command` | `parser = argparse.ArgumentParser(prog="pool_parity.py")` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `rec.add_argument("--block", type=int, required=True)` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `rec.add_argument("--out", required=True)` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `chk.add_argument("--record", required=True)` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `rec.add_argument("--registry", action="append", type=_address)` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `rec.add_argument("--rpc", required=True)` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `chk = sub.add_parser("check")` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `rec = sub.add_parser("record")` | [tools/release/rehearsal/pool_parity.py](../../../tools/release/rehearsal/pool_parity.py) |
| `cli_command` | `parser = argparse.ArgumentParser(prog="programme.py")` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("check", choices=sorted(CHECKS))` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--comet", action="append", default=[])` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--evm", action="append", default=[])` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--feed-key")` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--halt-height", type=int, required=True)` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--maker-key")` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--sidecar-pid", action="append", type=int)` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser.add_argument("--trader-key")` | [tools/release/rehearsal/programme.py](../../../tools/release/rehearsal/programme.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [tools/release/rehearsal/rewrite_genesis.py](../../../tools/release/rehearsal/rewrite_genesis.py) |
| `cli_command` | `parser.add_argument("exported", nargs="?", default="-")` | [tools/release/rehearsal/rewrite_genesis.py](../../../tools/release/rehearsal/rewrite_genesis.py) |
| `cli_command` | `parser.add_argument("--output", default="-")` | [tools/release/rehearsal/rewrite_genesis.py](../../../tools/release/rehearsal/rewrite_genesis.py) |
| `cli_command` | `usage: rehearse-v6120.sh --env-file FILE [flags] [--receipt PATH]` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--chain-id) need_value "$@"; chain_id=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--chainflow-key-file) need_value "$@"; chainflow_key_file=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--check) check=1; shift ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--env-file) need_value "$@"; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--export-height) need_value "$@"; export_height=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `-h \| --help) usage; exit 0 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--halt-height) need_value "$@"; halt_height=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--keyring-backend) need_value "$@"; keyring_backend=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--keyring-dir) need_value "$@"; keyring_dir=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--new-bin) need_value "$@"; new_bin=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--old-bin) need_value "$@"; old_bin=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--operator-keys) need_value "$@"; operator_keys=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--port-base) need_value "$@"; port_base=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--receipt) need_value "$@"; receipt=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--sidecar-config) need_value "$@"; sidecar_config=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--source-data) need_value "$@"; source_data=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--source-genesis) need_value "$@"; source_genesis=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--stop-after) need_value "$@"; stop_after=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--wait-seconds) need_value "$@"; wait_seconds=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--work) need_value "$@"; work=$2; shift 2 ;;` | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| `cli_command` | `--env-file)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--execute \| --readiness \| --verify)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--expected-halt-height)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--expected-release-id)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--local)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--phase)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--receipt)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `--service-manager)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `execute)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `readiness)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `verify)` | [tools/release/rollback.sh](../../../tools/release/rollback.sh) |
| `cli_command` | `if arg == "--home" and i + 1 < len(args):` | [tools/release/snapshot.sh](../../../tools/release/snapshot.sh) |
| `cli_command` | `--verify \| --check \| --automation-status)` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `--env-file \| --receipt)` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `--local)` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `automation-status)` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `check) printf 'stage: inputs valid\n' ;;` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `install) stage_install ;;` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `verify)` | [tools/release/stage.sh](../../../tools/release/stage.sh) |
| `cli_command` | `--verify) verify=true; shift ;;` | [tools/release/swap.sh](../../../tools/release/swap.sh) |
| `cli_command` | `class ArgumentParser(argparse.ArgumentParser):` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `node_parser.add_argument("--halt-height", type=int)` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `node_parser.add_argument("--height", type=int)` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `node_parser.add_argument("--home", type=Path, required=True)` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `release_parser.add_argument("--name")` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `release_parser.add_argument("--out", type=Path)` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `node_parser.add_argument("--release-doc", type=Path)` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `node_parser.add_argument("--sha256")` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `release_parser.add_argument("--sums", type=Path, required=True)` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `release_parser.add_argument("--tag")` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `node_parser = commands.add_parser("node")` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `release_parser = commands.add_parser("release")` | [tools/release/upgrade_info.py](../../../tools/release/upgrade_info.py) |
| `cli_command` | `--dist) [ "$#" -ge 2 ] \|\| usage; dist=$2; shift 2 ;;` | [tools/release/verify-release.sh](../../../tools/release/verify-release.sh) |
| `cli_command` | `--tag) [ "$#" -ge 2 ] \|\| usage; tag=$2; shift 2 ;;` | [tools/release/verify-release.sh](../../../tools/release/verify-release.sh) |
| source | — | [tools/release/build-paxd.sh](../../../tools/release/build-paxd.sh) |
| source | — | [tools/release/reproducible-check.sh](../../../tools/release/reproducible-check.sh) |

## release-config

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"no_code": [` | [tools/release/fleet-views.json](../../../tools/release/fleet-views.json) |
| `config_key` | `"pointers": [` | [tools/release/fleet-views.json](../../../tools/release/fleet-views.json) |
| `config_key` | `"served": [` | [tools/release/fleet-views.json](../../../tools/release/fleet-views.json) |
| `config_key` | `"usdl": {` | [tools/release/fleet-views.json](../../../tools/release/fleet-views.json) |
| `config_key` | `CHAINFLOW_KEY_FILE=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `COSMOS_CHAIN_ID=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `EXPORT_HEIGHT=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `NEW_BIN=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `OLD_BIN=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PORT_BASE=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_FEED_REST_API_KEY_FILE=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_SIDECAR_CONFIG=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_SIDECAR_CONSENSUS_RPC_URL=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_SIDECAR_EVM_RPC_URL=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_SIDECAR_FEED_KEY_FILE=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_SIDECAR_HEALTH_ADDR=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `PRICE_SIDECAR_STATE_DIR=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `REHEARSAL_HALT_HEIGHT=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `SOURCE_DATA=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `SOURCE_GENESIS=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `WORK=` | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| `config_key` | `APP_BUILD_ID=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `APP_URL=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `ARCHIVE_HOST=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `AUTOMATION_UNITS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `BIN_DIR=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `BUILD_HOST=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `CHAIN_ID=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `CHAINFLOW_KEY_FILE=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `CONSENSUS_RPC=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `CREDENTIAL_DIR=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `DATA_SERVICE_URLS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `EDGE_HOST=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `EVM_RPC=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `FULLNODE_HOMES=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `FULLNODE_HOSTS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `FULLNODE_UNITS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `HALT_HEIGHT=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `INVENTORY_FILE=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `KEYRING_BACKEND=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `OLD_FEEDER_UNITS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `OPERATOR_KEY_NAMES=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `OPS_HOST=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `POOL_COUNT=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `RECEIPT_DIR=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `RELEASE_ID=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `RELEASE_SHA256=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `RELEASE_TAG=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `SIDECAR_SHA256=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `SIDECAR_UNITS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `SNAPSHOT_DIR=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `VALIDATOR_HOMES=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `VALIDATOR_HOSTS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `VALIDATOR_UNITS=` | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| `config_key` | `CC_VERSION=13.3.0` | [tools/release/toolchain.env](../../../tools/release/toolchain.env) |
| source | — | [tools/release/version.json](../../../tools/release/version.json) |

## release-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `POOL_COUNT` | [tools/release/fleet_check.py](../../../tools/release/fleet_check.py) |
| source | — | [tools/release/build-paxd.sh](../../../tools/release/build-paxd.sh) |
| source | — | [tools/release/extld.sh](../../../tools/release/extld.sh) |
| source | — | [tools/release/rehearse-v6120.sh](../../../tools/release/rehearse-v6120.sh) |
| source | — | [tools/release/swap.sh](../../../tools/release/swap.sh) |

## release-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/release/env/rehearsal.env.example](../../../tools/release/env/rehearsal.env.example) |
| source | — | [tools/release/env/rollout.env.example](../../../tools/release/env/rollout.env.example) |
| source | — | [tools/release/fleet-views.json](../../../tools/release/fleet-views.json) |
| source | — | [tools/release/toolchain.env](../../../tools/release/toolchain.env) |
| source | — | [tools/release/version.json](../../../tools/release/version.json) |

## rollout-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `echo "usage: box-apply.sh <role> <image>:<sha12> <unit> <health> <registry-token-file> <registry-user>" >&2` | [tools/rollout/box-apply.sh](../../../tools/rollout/box-apply.sh) |
| `cli_command` | `echo "usage: ledger.sh <sha40> <service> <railway\|box> <started_at> <pass\|fail\|blocked> <log path>" >&2` | [tools/rollout/ledger.sh](../../../tools/rollout/ledger.sh) |
| `cli_command` | `if __name__ == "__main__":` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `cli_command` | `sys.exit(main(sys.argv[1:]))` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| source | — | [tools/rollout/rollout.sh](../../../tools/rollout/rollout.sh) |

## rollout-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `GHCR_ACTOR` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `RAILWAY_API_TOKEN` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `RAILWAY_PROJECT_ID` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `REGISTRY_TOKEN_FILE` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `ROLLOUT_DRY_RUN` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `ROLLOUT_HOSTS_FILE` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `ROLLOUT_LOG_DIR` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `ROLLOUT_MANIFEST` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `ROLLOUT_POLL` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| `env_var` | `ROLLOUT_TIMEOUT` | [tools/rollout/rollout.py](../../../tools/rollout/rollout.py) |
| source | — | [tools/rollout/box-apply.sh](../../../tools/rollout/box-apply.sh) |
| source | — | [tools/rollout/ledger.sh](../../../tools/rollout/ledger.sh) |

## rollout-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/rollout/tests/replay.json](../../../tools/rollout/tests/replay.json) |

## schemas

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `layerx.arbiter-prestate-build.v2` | [tools/qualification/paxeer-x/arbiter_prestate.py](../../../tools/qualification/paxeer-x/arbiter_prestate.py) |
| `file_format` | `layerx.asset-execution-prestate.v1` | [tools/qualification/paxeer-x/asset-execution-prestate.py](../../../tools/qualification/paxeer-x/asset-execution-prestate.py) |
| `file_format` | `layerx.asset-inventory.v1` | [tools/qualification/paxeer-x/asset-inventory.py](../../../tools/qualification/paxeer-x/asset-inventory.py) |
| `file_format` | `layerx.chain-proof-task-build.v2` | [tools/paxeer-x/gates/103.6.3.sh](../../../tools/paxeer-x/gates/103.6.3.sh) |
| `file_format` | `layerx.chain-rpc-task-build.v2` | [tools/paxeer-x/gates/103.6.4.sh](../../../tools/paxeer-x/gates/103.6.4.sh) |
| `file_format` | `layerx.chain-simulation-task-build.v2` | [tools/paxeer-x/gates/103.6.9.sh](../../../tools/paxeer-x/gates/103.6.9.sh) |
| `file_format` | `layerx.composite-state-transition-build.v2` | [tools/paxeer-x/gates/103.6.11.sh](../../../tools/paxeer-x/gates/103.6.11.sh) |
| `file_format` | `layerx.emulator-conformance.corpus.v1` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `file_format` | `layerx.emulator-conformance.observations.v1` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `file_format` | `layerx.event-admission.v1` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `file_format` | `layerx.gateway.maintenance-artifacts.v1` | [tools/paxeer-x/gates/16.6.sh](../../../tools/paxeer-x/gates/16.6.sh) |
| `file_format` | `layerx.human.budget-proof.v1` | [tools/qualification/paxeer-x/budget-proof-exports.py](../../../tools/qualification/paxeer-x/budget-proof-exports.py) |
| `file_format` | `layerx.human.owner-bundle.v2` | [tools/qualification/paxeer-x/human-owner-evidence.py](../../../tools/qualification/paxeer-x/human-owner-evidence.py) |
| `file_format` | `layerx.human.owner-producers.v1` | [tools/qualification/paxeer-x/human-owner-evidence.py](../../../tools/qualification/paxeer-x/human-owner-evidence.py) |
| `file_format` | `layerx.interpreter-pricing-build.v1` | [tools/paxeer-x/gates/104.33.5.sh](../../../tools/paxeer-x/gates/104.33.5.sh) |
| `file_format` | `layerx.interpreter-task-build.v1` | [tools/paxeer-x/gates/104.33.4.sh](../../../tools/paxeer-x/gates/104.33.4.sh) |
| `file_format` | `layerx.mcp.invocation.v1` | [tools/qualification/paxeer-x/mcp_daemon_revocation.py](../../../tools/qualification/paxeer-x/mcp_daemon_revocation.py) |
| `file_format` | `layerx.mirror-build.v1` | [tools/paxeer-x/gates/104.25.1.sh](../../../tools/paxeer-x/gates/104.25.1.sh) |
| `file_format` | `layerx.mirror-live-evidence.v1` | [scripts/qualify-mirror-live.sh](../../../scripts/qualify-mirror-live.sh) |
| `file_format` | `layerx.mirror-verification-build.v1` | [tools/paxeer-x/gates/104.25.2.sh](../../../tools/paxeer-x/gates/104.25.2.sh) |
| `file_format` | `layerx.native-artifacts.v1` | [docker/kernel/Dockerfile](../../../docker/kernel/Dockerfile) |
| `file_format` | `layerx.native-export-corpus-build.v2` | [tools/paxeer-x/gates/103.6.13.sh](../../../tools/paxeer-x/gates/103.6.13.sh) |
| `file_format` | `layerx.native-programs-link.v1` | [tools/qualification/paxeer-x/native-programs-link.py](../../../tools/qualification/paxeer-x/native-programs-link.py) |
| `file_format` | `layerx.program-language-abi.v1` | [tools/qualification/paxeer-x/programs_language_abi.py](../../../tools/qualification/paxeer-x/programs_language_abi.py) |
| `file_format` | `layerx.scan-load-build.v1` | [scripts/paxeer-x/qualify-scan-load.py](../../../scripts/paxeer-x/qualify-scan-load.py) |
| `file_format` | `paxai.admission.v1` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `file_format` | `paxai.approval.v1` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `file_format` | `paxai.selector-admission.v1` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `file_format` | `paxai.selector-evidence.v1` | [tools/paxeer-x/ai_evidence.py](../../../tools/paxeer-x/ai_evidence.py) |
| `file_format` | `paxeer-x.agent-envelope-artifacts.v1` | [tools/qualification/paxeer-x/agent_operation_envelope.py](../../../tools/qualification/paxeer-x/agent_operation_envelope.py) |
| `file_format` | `paxeer-x.agent-envelope-qualification.v1` | [tools/qualification/paxeer-x/agent_operation_envelope.py](../../../tools/qualification/paxeer-x/agent_operation_envelope.py) |
| `file_format` | `paxeer-x.agentd-http-fixture.v1` | [tools/qualification/paxeer-x/fixtures/agentd_fixture.py](../../../tools/qualification/paxeer-x/fixtures/agentd_fixture.py) |
| `file_format` | `paxeer-x.agentd-tenant-recovery.v1` | [tools/qualification/paxeer-x/fixtures/agentd_fixture.py](../../../tools/qualification/paxeer-x/fixtures/agentd_fixture.py) |
| `file_format` | `paxeer-x.budget-proof-fixture.v1` | [tools/qualification/paxeer-x/budget-proof-exports.py](../../../tools/qualification/paxeer-x/budget-proof-exports.py) |
| `file_format` | `paxeer-x.candidate.v1` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `file_format` | `paxeer-x.completed-evidence-validation.v1` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `file_format` | `paxeer-x.completed-qualification.v1` | [tools/flyci/controller/github.go](../../../tools/flyci/controller/github.go) |
| `file_format` | `paxeer-x.custody-provider-build.v1` | [tools/qualification/paxeer-x/custody-provider-producer.py](../../../tools/qualification/paxeer-x/custody-provider-producer.py) |
| `file_format` | `paxeer-x.deterministic-execution.v1` | [tools/qualification/paxeer-x/deterministic-execution-producer.py](../../../tools/qualification/paxeer-x/deterministic-execution-producer.py) |
| `file_format` | `paxeer-x.emulator-conformance-artifacts.v1` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `file_format` | `paxeer-x.encrypted-artifact.v1` | [tools/flyci/controller/qualification_evidence.go](../../../tools/flyci/controller/qualification_evidence.go) |
| `file_format` | `paxeer-x.event-source-binary.v1` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `file_format` | `paxeer-x.fly-inventory.v1` | [tools/paxeer-x/tests/candidate_test.py](../../../tools/paxeer-x/tests/candidate_test.py) |
| `file_format` | `paxeer-x.gate-evidence.v1` | [tools/paxeer-x/verify-task.sh](../../../tools/paxeer-x/verify-task.sh) |
| `file_format` | `paxeer-x.human-event-fairness.v1` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| `file_format` | `paxeer-x.human-native-send-producer.artifacts.v1` | [tools/qualification/paxeer-x/human-native-send-producer.py](../../../tools/qualification/paxeer-x/human-native-send-producer.py) |
| `file_format` | `paxeer-x.human-native-send-producer.v1` | [tools/qualification/paxeer-x/human-native-send-producer.py](../../../tools/qualification/paxeer-x/human-native-send-producer.py) |
| `file_format` | `paxeer-x.human-program-approvals.v1` | [tools/qualification/paxeer-x/human-program-approvals.py](../../../tools/qualification/paxeer-x/human-program-approvals.py) |
| `file_format` | `paxeer-x.human-projection-facts.v1` | [tools/qualification/paxeer-x/human-projection-facts.py](../../../tools/qualification/paxeer-x/human-projection-facts.py) |
| `file_format` | `paxeer-x.human-service-contract.v1` | [tools/qualification/paxeer-x/human-service-contract.py](../../../tools/qualification/paxeer-x/human-service-contract.py) |
| `file_format` | `paxeer-x.human-service-contract.v2` | [tools/qualification/paxeer-x/human-service-contract.py](../../../tools/qualification/paxeer-x/human-service-contract.py) |
| `file_format` | `paxeer-x.human-unified-custody-shapes.v1` | [tools/qualification/paxeer-x/human_unified_custody_shapes.py](../../../tools/qualification/paxeer-x/human_unified_custody_shapes.py) |
| `file_format` | `paxeer-x.identity-assertion-binding.v1` | [tools/qualification/paxeer-x/identity-assertion-binding.py](../../../tools/qualification/paxeer-x/identity-assertion-binding.py) |
| `file_format` | `paxeer-x.interop-readiness-inputs.v1` | [tools/qualification/paxeer-x/interop-archive-contract.py](../../../tools/qualification/paxeer-x/interop-archive-contract.py) |
| `file_format` | `paxeer-x.kernel-payout-readback.v1` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `file_format` | `paxeer-x.kernel-web-accounts.v1` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `file_format` | `paxeer-x.kernel-web-recovery.v1` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `file_format` | `paxeer-x.market-attestation-artifacts.v1` | [tools/qualification/paxeer-x/program_market_attestation.py](../../../tools/qualification/paxeer-x/program_market_attestation.py) |
| `file_format` | `paxeer-x.mcp-served-writes.v1` | [tools/qualification/paxeer-x/fixtures/agentd_fixture.py](../../../tools/qualification/paxeer-x/fixtures/agentd_fixture.py) |
| `file_format` | `paxeer-x.mcp-web-artifacts.v1` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `file_format` | `paxeer-x.mcp-web-owner-build.v1` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `file_format` | `paxeer-x.mcp-web-owner-fixture.v1` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `file_format` | `paxeer-x.mcp-web-produced-fixture.v1` | [tools/qualification/paxeer-x/mcp_web_payer.py](../../../tools/qualification/paxeer-x/mcp_web_payer.py) |
| `file_format` | `paxeer-x.movement-executor-readiness.v1` | [tools/qualification/paxeer-x/movement-executor-readiness.py](../../../tools/qualification/paxeer-x/movement-executor-readiness.py) |
| `file_format` | `paxeer-x.native-interface-result.v1` | [tools/qualification/paxeer-x/programs_native_interfaces.py](../../../tools/qualification/paxeer-x/programs_native_interfaces.py) |
| `file_format` | `paxeer-x.native-journey-approval.v2` | [tools/qualification/paxeer-x/native-journey-approval.py](../../../tools/qualification/paxeer-x/native-journey-approval.py) |
| `file_format` | `paxeer-x.native-migration-artifacts.v1` | [tools/qualification/paxeer-x/programs_native_migration.py](../../../tools/qualification/paxeer-x/programs_native_migration.py) |
| `file_format` | `paxeer-x.native-migration-legacy-provenance.v1` | [tools/qualification/paxeer-x/programs_native_migration.py](../../../tools/qualification/paxeer-x/programs_native_migration.py) |
| `file_format` | `paxeer-x.native-migration-refusal.v1` | [tools/qualification/paxeer-x/programs_native_migration.py](../../../tools/qualification/paxeer-x/programs_native_migration.py) |
| `file_format` | `paxeer-x.native-migration-result.v1` | [tools/qualification/paxeer-x/programs_native_migration.py](../../../tools/qualification/paxeer-x/programs_native_migration.py) |
| `file_format` | `paxeer-x.native-program-human-peer.v1` | [tools/qualification/paxeer-x/native-program-human-peer.py](../../../tools/qualification/paxeer-x/native-program-human-peer.py) |
| `file_format` | `paxeer-x.native-program-presentation.v1` | [tools/qualification/paxeer-x/native-program-presentation.py](../../../tools/qualification/paxeer-x/native-program-presentation.py) |
| `file_format` | `paxeer-x.native-send-profile.v1` | [tools/qualification/paxeer-x/native_send_profile.py](../../../tools/qualification/paxeer-x/native_send_profile.py) |
| `file_format` | `paxeer-x.paid-web-operational.v1` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `file_format` | `paxeer-x.paid-web-readiness.v1` | [tools/qualification/paxeer-x/paid-web-contract.py](../../../tools/qualification/paxeer-x/paid-web-contract.py) |
| `file_format` | `paxeer-x.program-abi-freeze-artifacts.v1` | [tools/qualification/paxeer-x/program-abi-freeze.py](../../../tools/qualification/paxeer-x/program-abi-freeze.py) |
| `file_format` | `paxeer-x.program-abi-freeze-result.v1` | [tools/qualification/paxeer-x/program-abi-freeze.py](../../../tools/qualification/paxeer-x/program-abi-freeze.py) |
| `file_format` | `paxeer-x.program-abi-policy-artifacts.v1` | [tools/qualification/paxeer-x/program-abi-policy.py](../../../tools/qualification/paxeer-x/program-abi-policy.py) |
| `file_format` | `paxeer-x.program-abi-policy-result.v1` | [tools/qualification/paxeer-x/program-abi-policy.py](../../../tools/qualification/paxeer-x/program-abi-policy.py) |
| `file_format` | `paxeer-x.program-account-integration.v1` | [tools/paxeer-x/gates/104.30.3.sh](../../../tools/paxeer-x/gates/104.30.3.sh) |
| `file_format` | `paxeer-x.program-balance-artifacts.v1` | [tools/qualification/paxeer-x/program_balance_read.py](../../../tools/qualification/paxeer-x/program_balance_read.py) |
| `file_format` | `paxeer-x.program-bindings-artifacts.v1` | [tools/qualification/paxeer-x/program-bindings-producer.py](../../../tools/qualification/paxeer-x/program-bindings-producer.py) |
| `file_format` | `paxeer-x.program-bindings-result.v1` | [tools/qualification/paxeer-x/program-bindings.py](../../../tools/qualification/paxeer-x/program-bindings.py) |
| `file_format` | `paxeer-x.program-cache-artifacts.v1` | [tools/qualification/paxeer-x/program_cache_native_equivalence.py](../../../tools/qualification/paxeer-x/program_cache_native_equivalence.py) |
| `file_format` | `paxeer-x.program-event-bounds-artifacts.v1` | [scripts/qualification/paxeer-x/program-event-bounds.py](../../../scripts/qualification/paxeer-x/program-event-bounds.py) |
| `file_format` | `paxeer-x.program-execution-context-build.v1` | [tools/qualification/paxeer-x/program_execution_context.py](../../../tools/qualification/paxeer-x/program_execution_context.py) |
| `file_format` | `paxeer-x.program-fee-governance-build.v1` | [tools/qualification/paxeer-x/program_fee_governance.py](../../../tools/qualification/paxeer-x/program_fee_governance.py) |
| `file_format` | `paxeer-x.program-interface-artifacts.v1` | [tools/qualification/paxeer-x/program-interface-producer.py](../../../tools/qualification/paxeer-x/program-interface-producer.py) |
| `file_format` | `paxeer-x.program-interface-result.v1` | [tools/qualification/paxeer-x/program-interface.py](../../../tools/qualification/paxeer-x/program-interface.py) |
| `file_format` | `paxeer-x.program-module-registration.v1` | [tools/paxeer-x/gates/104.19.4.sh](../../../tools/paxeer-x/gates/104.19.4.sh) |
| `file_format` | `paxeer-x.program-monetary-law-build.v1` | [tools/paxeer-x/gates/104.21.1.sh](../../../tools/paxeer-x/gates/104.21.1.sh) |
| `file_format` | `paxeer-x.program-spend-composition-build.v1` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `file_format` | `paxeer-x.program-spend-composition-result.v1` | [tools/qualification/paxeer-x/program_spend_composition.py](../../../tools/qualification/paxeer-x/program_spend_composition.py) |
| `file_format` | `paxeer-x.program-storage-artifacts.v1` | [tools/qualification/paxeer-x/program-storage-producer.py](../../../tools/qualification/paxeer-x/program-storage-producer.py) |
| `file_format` | `paxeer-x.program-storage-result.v1` | [tools/qualification/paxeer-x/program-storage.py](../../../tools/qualification/paxeer-x/program-storage.py) |
| `file_format` | `paxeer-x.program-terminal-v5-artifacts.v1` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `file_format` | `paxeer-x.programs-all-form-artifacts.v1` | [tools/qualification/paxeer-x/programs_all_form_validation.py](../../../tools/qualification/paxeer-x/programs_all_form_validation.py) |
| `file_format` | `paxeer-x.programs-market-resolution.v1` | [tools/qualification/paxeer-x/programs_market_resolution.py](../../../tools/qualification/paxeer-x/programs_market_resolution.py) |
| `file_format` | `paxeer-x.qualification-contract.v1` | [tools/qualification/paxeer_x_release.py](../../../tools/qualification/paxeer_x_release.py) |
| `file_format` | `paxeer-x.qualification-manifest.v1` | [tools/flyci/controller/qualification.go](../../../tools/flyci/controller/qualification.go) |
| `file_format` | `paxeer-x.reference-custody-build.v1` | [scripts/qualification/paxeer-x/reference-custody.py](../../../scripts/qualification/paxeer-x/reference-custody.py) |
| `file_format` | `paxeer-x.reference-custody-native.v1` | [scripts/qualification/paxeer-x/reference-custody.py](../../../scripts/qualification/paxeer-x/reference-custody.py) |
| `file_format` | `paxeer-x.reference-custody-result.v1` | [scripts/qualification/paxeer-x/reference-custody.py](../../../scripts/qualification/paxeer-x/reference-custody.py) |
| `file_format` | `paxeer-x.registry-artifacts.v1` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `file_format` | `paxeer-x.registry-readiness-artifacts.v1` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `file_format` | `paxeer-x.registry-readiness-operation.v1` | [tools/qualification/paxeer-x/registry-contract.py](../../../tools/qualification/paxeer-x/registry-contract.py) |
| `file_format` | `paxeer-x.release-evidence.v1` | [tools/paxeer-x/release.sh](../../../tools/paxeer-x/release.sh) |
| `file_format` | `paxeer-x.route-contract.v1` | [tools/paxeer-x/tests/routes_test.py](../../../tools/paxeer-x/tests/routes_test.py) |
| `file_format` | `paxeer-x.routes.v1` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `file_format` | `paxeer-x.rpc-transport-build.v1` | [tools/paxeer-x/gates/104.38.2.sh](../../../tools/paxeer-x/gates/104.38.2.sh) |
| `file_format` | `paxeer-x.runtime-selection.v1` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| `file_format` | `paxeer-x.storage-recovery-artifacts.v1` | [tools/qualification/paxeer-x/storage-recovery-producer.py](../../../tools/qualification/paxeer-x/storage-recovery-producer.py) |
| `file_format` | `paxeer-x.storage-recovery-result.v1` | [tools/qualification/paxeer-x/storage-recovery.py](../../../tools/qualification/paxeer-x/storage-recovery.py) |
| `file_format` | `paxeer-x.ui-bindings.v2` | [tools/paxeer-x/route-catalogue.json](../../../tools/paxeer-x/route-catalogue.json) |
| `file_format` | `paxeer-x.unified-identity-fixture.v1` | [tools/qualification/paxeer-x/unified-account-identity.py](../../../tools/qualification/paxeer-x/unified-account-identity.py) |
| `file_format` | `paxeer-x.wallet-injected-baseline.v1` | [tools/qualification/paxeer-x/wallet-injected-fixture.mjs](../../../tools/qualification/paxeer-x/wallet-injected-fixture.mjs) |
| `file_format` | `paxeer-x.wallet-injected-build.v1` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `file_format` | `paxeer-x.wallet-injected-chain-result.v1` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `file_format` | `paxeer-x.wallet-injected-chain-runtime.v1` | [tools/qualification/paxeer-x/wallet-injected-chain.py](../../../tools/qualification/paxeer-x/wallet-injected-chain.py) |
| `file_format` | `paxeer-x.wallet-injected-readiness.v1` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `file_format` | `paxeer-x.wallet-injected-runtime.v1` | [tools/qualification/paxeer-x/wallet-injected-fixture.py](../../../tools/qualification/paxeer-x/wallet-injected-fixture.py) |
| `file_format` | `paxeer-x.webhook-dashboard-fixture.v1` | [tools/paxeer-x/gates/104.17.5.sh](../../../tools/paxeer-x/gates/104.17.5.sh) |
| `file_format` | `paxeer-x.webhook-process-artifacts.v1` | [tools/qualification/paxeer-x/event-delivery-contract.py](../../../tools/qualification/paxeer-x/event-delivery-contract.py) |
| source | — | [tools/paxeer-x/gates/104.33.7.sh](../../../tools/paxeer-x/gates/104.33.7.sh) |
| source | — | [docker/kernel/init.sh](../../../docker/kernel/init.sh) |
| source | — | [tools/qualification/paxeer-x/human-security-prerequisites.py](../../../tools/qualification/paxeer-x/human-security-prerequisites.py) |
| source | — | [tools/qualification/paxeer-x/programs_accounts_winddown.py](../../../tools/qualification/paxeer-x/programs_accounts_winddown.py) |
| source | — | [tools/qualification/paxeer-x/budget-proof-intake.py](../../../tools/qualification/paxeer-x/budget-proof-intake.py) |
| source | — | [tools/qualification/paxeer-x/custody_governance.py](../../../tools/qualification/paxeer-x/custody_governance.py) |
| source | — | [tools/qualification/paxeer-x/human-approval-admission.py](../../../tools/qualification/paxeer-x/human-approval-admission.py) |
| source | — | [scripts/qualification/paxeer-x/program-http-producer.py](../../../scripts/qualification/paxeer-x/program-http-producer.py) |
| source | — | [tools/paxeer-x/gates/104.30.5.sh](../../../tools/paxeer-x/gates/104.30.5.sh) |
| source | — | [tools/qualification/paxeer-x/programs_typed_interfaces.py](../../../tools/qualification/paxeer-x/programs_typed_interfaces.py) |

## scripts-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `func main() {` | [scripts/bump_version/main.go](../../../scripts/bump_version/main.go) |
| `cli_command` | `*) echo 'usage: beta-ci-inputs.sh --evidence-root PRIVATE_DIR --exports-file PATH [--timeout SECONDS]' >&2; exit 2 ;;` | [scripts/ci/beta-ci-inputs.sh](../../../scripts/ci/beta-ci-inputs.sh) |
| `cli_command` | `--evidence-root\|--exports-file\|--timeout)` | [scripts/ci/beta-ci-inputs.sh](../../../scripts/ci/beta-ci-inputs.sh) |
| `cli_command` | `usage: scripts/ci/beta-ledger-check.sh [--ledger PATH] [--spec PATH] [--revisions]` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `-C \| -f \| -I \| -o \| -W) echo "make option $w is not supported; only the top-level Makefile and its includes are recognised"; return 0 ;;` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `--evidence-root\|--candidate)` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `--candidate "$candidate" --record "$record_json"); then` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `-h \| --help)` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `--ledger)` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `--revisions)` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `--spec)` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `cli_command` | `echo 'usage: beta-qualify.sh --spec PATH --task ID --ledger PRIVATE_PATH --evidence-root PRIVATE_DIR [--timeout SECONDS] -- "make TARGET"'` | [scripts/ci/beta-qualify.sh](../../../scripts/ci/beta-qualify.sh) |
| `cli_command` | `--spec\|--ledger\|--evidence-root\|--task\|--timeout\|--evidence-kind)` | [scripts/ci/beta-qualify.sh](../../../scripts/ci/beta-qualify.sh) |
| `cli_command` | `-h\|--help)` | [scripts/ci/beta-qualify.sh](../../../scripts/ci/beta-qualify.sh) |
| `cli_command` | `--) shift; break ;;` | [scripts/ci/beta-qualify.sh](../../../scripts/ci/beta-qualify.sh) |
| `cli_command` | `import assert from "node:assert/strict";` | [scripts/ci/developer-dashboard-lock.mjs](../../../scripts/ci/developer-dashboard-lock.mjs) |
| `cli_command` | `usage: scripts/ci/human-web-image-build.sh [--tag REF] [--work-dir DIR] [--keep-image]` | [scripts/ci/human-web-image-build.sh](../../../scripts/ci/human-web-image-build.sh) |
| `cli_command` | `-h\|--help) usage; exit 0 ;;` | [scripts/ci/human-web-image-build.sh](../../../scripts/ci/human-web-image-build.sh) |
| `cli_command` | `--keep-image) KEEP_IMAGE=1; shift ;;` | [scripts/ci/human-web-image-build.sh](../../../scripts/ci/human-web-image-build.sh) |
| `cli_command` | `--tag) [ "$#" -ge 2 ] \|\| { usage >&2; exit 2; }; IMAGE_REF=$2; shift 2 ;;` | [scripts/ci/human-web-image-build.sh](../../../scripts/ci/human-web-image-build.sh) |
| `cli_command` | `--work-dir) [ "$#" -ge 2 ] \|\| { usage >&2; exit 2; }; WORK_DIR=$2; shift 2 ;;` | [scripts/ci/human-web-image-build.sh](../../../scripts/ci/human-web-image-build.sh) |
| `cli_command` | `echo "usage: scripts/ci/layout-check.sh <old-path> [<old-path> ...]" >&2` | [scripts/ci/layout-check.sh](../../../scripts/ci/layout-check.sh) |
| `cli_command` | `usage: scripts/ci/release-artifact-record.sh <name> <file> <digest_of> <signature> <sbom> <attestation> <location>` | [scripts/ci/release-artifact-record.sh](../../../scripts/ci/release-artifact-record.sh) |
| `cli_command` | `--job)` | [scripts/ci/release-gate.sh](../../../scripts/ci/release-gate.sh) |
| `cli_command` | `--task)` | [scripts/ci/release-gate.sh](../../../scripts/ci/release-gate.sh) |
| `cli_command` | `--)` | [scripts/ci/release-gate.sh](../../../scripts/ci/release-gate.sh) |
| `cli_command` | `if __name__ == "__main__":` | [scripts/ci/release-version.py](../../../scripts/ci/release-version.py) |
| `cli_command` | `sys.exit(main(sys.argv[1:]))` | [scripts/ci/release-version.py](../../../scripts/ci/release-version.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [scripts/ci/restore-build-ownership.py](../../../scripts/ci/restore-build-ownership.py) |
| `cli_command` | `parser.add_argument("--capture-owner", action="store_true")` | [scripts/ci/restore-build-ownership.py](../../../scripts/ci/restore-build-ownership.py) |
| `cli_command` | `parser.add_argument("--output", action="append", default=[])` | [scripts/ci/restore-build-ownership.py](../../../scripts/ci/restore-build-ownership.py) |
| `cli_command` | `parser.add_argument("--owner")` | [scripts/ci/restore-build-ownership.py](../../../scripts/ci/restore-build-ownership.py) |
| `cli_command` | `parser.add_argument("--repo", required=True)` | [scripts/ci/restore-build-ownership.py](../../../scripts/ci/restore-build-ownership.py) |
| `cli_command` | `parser.add_argument("--validate-only", action="store_true")` | [scripts/ci/restore-build-ownership.py](../../../scripts/ci/restore-build-ownership.py) |
| `cli_command` | `if __name__ == "__main__":` | [scripts/ci/result-codes-gen.py](../../../scripts/ci/result-codes-gen.py) |
| `cli_command` | `sys.exit(main(sys.argv[1:]))` | [scripts/ci/result-codes-gen.py](../../../scripts/ci/result-codes-gen.py) |
| `cli_command` | `--self-test) self_test ;;` | [scripts/ci/secret-scan.sh](../../../scripts/ci/secret-scan.sh) |
| `cli_command` | `echo "usage: scripts/ci/vendor-verify.sh <dir>" >&2` | [scripts/ci/vendor-verify.sh](../../../scripts/ci/vendor-verify.sh) |
| `cli_command` | `if token in ("--manifest-path", "--match-path") and following:` | [scripts/ci/xweb-workflow-check.sh](../../../scripts/ci/xweb-workflow-check.sh) |
| `cli_command` | `func main() {` | [scripts/evm_stress/main.go](../../../scripts/evm_stress/main.go) |
| `cli_command` | `dumpPaxAddrs := flag.Bool("dump-pax-addrs", false, "print sender pax bech32 addresses for genesis funding and exit")` | [scripts/evm_stress/main.go](../../../scripts/evm_stress/main.go) |
| `cli_command` | `mode := flag.String("mode", workloadModeTransfer,` | [scripts/evm_stress/main.go](../../../scripts/evm_stress/main.go) |
| `cli_command` | `Usage: scripts/explorer/gate-lint.sh [--check]` | [scripts/explorer/gate-lint.sh](../../../scripts/explorer/gate-lint.sh) |
| `cli_command` | `--check)` | [scripts/explorer/gate-lint.sh](../../../scripts/explorer/gate-lint.sh) |
| `cli_command` | `-h \| --help)` | [scripts/explorer/gate-lint.sh](../../../scripts/explorer/gate-lint.sh) |
| `cli_command` | `--)` | [scripts/explorer/gate-lint.sh](../../../scripts/explorer/gate-lint.sh) |
| `cli_command` | `Usage: scripts/explorer/gate-test.sh [--check]` | [scripts/explorer/gate-test.sh](../../../scripts/explorer/gate-test.sh) |
| `cli_command` | `--check)` | [scripts/explorer/gate-test.sh](../../../scripts/explorer/gate-test.sh) |
| `cli_command` | `-h \| --help)` | [scripts/explorer/gate-test.sh](../../../scripts/explorer/gate-test.sh) |
| `cli_command` | `--)` | [scripts/explorer/gate-test.sh](../../../scripts/explorer/gate-test.sh) |
| `cli_command` | `Usage: scripts/explorer/test-ratio.sh BASE..HEAD` | [scripts/explorer/test-ratio.sh](../../../scripts/explorer/test-ratio.sh) |
| `cli_command` | `-h \| --help)` | [scripts/explorer/test-ratio.sh](../../../scripts/explorer/test-ratio.sh) |
| `cli_command` | `--)` | [scripts/explorer/test-ratio.sh](../../../scripts/explorer/test-ratio.sh) |
| `cli_command` | `if __name__ == "__main__":` | [scripts/paxd_problems_detector.py](../../../scripts/paxd_problems_detector.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [scripts/paxeer-x/qualify-scan-load.py](../../../scripts/paxeer-x/qualify-scan-load.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [scripts/paxeer-x/qualify-scan-load.py](../../../scripts/paxeer-x/qualify-scan-load.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [scripts/programs/generate-abi-vectors.py](../../../scripts/programs/generate-abi-vectors.py) |
| `cli_command` | `parser.add_argument("--check", action="store_true")` | [scripts/programs/generate-abi-vectors.py](../../../scripts/programs/generate-abi-vectors.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [scripts/qualification/paxeer-x/program-event-bounds.py](../../../scripts/qualification/paxeer-x/program-event-bounds.py) |
| `cli_command` | `sub.add_parser('verify').add_argument('--manifest', required=True)` | [scripts/qualification/paxeer-x/program-event-bounds.py](../../../scripts/qualification/paxeer-x/program-event-bounds.py) |
| `cli_command` | `sub.add_parser('build').add_argument('--output', required=True)` | [scripts/qualification/paxeer-x/program-event-bounds.py](../../../scripts/qualification/paxeer-x/program-event-bounds.py) |
| `cli_command` | `parser.add_argument('--output', required=True)` | [scripts/qualification/paxeer-x/program-http-producer.py](../../../scripts/qualification/paxeer-x/program-http-producer.py) |
| `cli_command` | `parser = argparse.ArgumentParser()` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `cli_command` | `parser.add_argument('--manifest', default=os.environ.get('PAXEER_X_PROGRAM_TERMINAL_V5_ARTIFACTS'))` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `cli_command` | `parser.add_argument('--output')` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `cli_command` | `parser.add_argument('--build', action='store_true')` | [scripts/qualification/paxeer-x/reference-custody.py](../../../scripts/qualification/paxeer-x/reference-custody.py) |
| `cli_command` | `parser.add_argument('--cc', default='cc')` | [scripts/qualification/paxeer-x/reference-custody.py](../../../scripts/qualification/paxeer-x/reference-custody.py) |
| `cli_command` | `if __name__ == "__main__":` | [scripts/replica/lxp_replay_fixture.py](../../../scripts/replica/lxp_replay_fixture.py) |
| `cli_command` | `echo "Usage: $0 INITIAL_HALT_HEIGHT SNAPSHOT_INTERVAL CHAIN_ID"` | [scripts/run_snapshot.sh](../../../scripts/run_snapshot.sh) |
| `cli_command` | `function deploy(address deployer, Parameters calldata parameters) external returns (SidioraProxyTimelock timelock) {` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function handover(address currentOwner, SidioraProxyTimelock timelock, Parameters calldata parameters) external {` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function initialize() external;` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function owner() external view returns (address);` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function startBroadcast(address sender) external;` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function stopBroadcast() external;` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function toString(address value) external pure returns (string memory);` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function transferOwnership(address newOwner) external;` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `function upgradeCall(address implementation) public pure returns (bytes memory) {` | [scripts/SidioraProxyGovernance.s.sol](../../../scripts/SidioraProxyGovernance.s.sol) |
| `cli_command` | `usage: scripts/wallet/check-deploy.sh [dir]` | [scripts/wallet/check-deploy.sh](../../../scripts/wallet/check-deploy.sh) |
| `cli_command` | `-h \| --help)` | [scripts/wallet/check-deploy.sh](../../../scripts/wallet/check-deploy.sh) |
| `cli_command` | `usage: scripts/wallet/check-live.sh endpoint \| attestors \| gateway \| cutover \| human \| human-session` | [scripts/wallet/check-live.sh](../../../scripts/wallet/check-live.sh) |
| `cli_command` | `-h \| --help)` | [scripts/wallet/check-live.sh](../../../scripts/wallet/check-live.sh) |
| `cli_command` | `Usage: scripts/wallet/gate-lint.sh [--check \| --help]` | [scripts/wallet/gate-lint.sh](../../../scripts/wallet/gate-lint.sh) |
| `cli_command` | `--check)` | [scripts/wallet/gate-lint.sh](../../../scripts/wallet/gate-lint.sh) |
| `cli_command` | `--help \| -h)` | [scripts/wallet/gate-lint.sh](../../../scripts/wallet/gate-lint.sh) |
| `cli_command` | `Usage: scripts/wallet/gate-test.sh [--check \| --help]` | [scripts/wallet/gate-test.sh](../../../scripts/wallet/gate-test.sh) |
| `cli_command` | `--check)` | [scripts/wallet/gate-test.sh](../../../scripts/wallet/gate-test.sh) |
| `cli_command` | `--help \| -h)` | [scripts/wallet/gate-test.sh](../../../scripts/wallet/gate-test.sh) |
| `cli_command` | `echo "usage: scan-secrets.sh <path>..." >&2` | [scripts/wallet/scan-secrets.sh](../../../scripts/wallet/scan-secrets.sh) |
| `cli_command` | `import { createRequire } from "node:module";` | [scripts/workspace/check-hardhat-compilers.mjs](../../../scripts/workspace/check-hardhat-compilers.mjs) |
| source | — | [scripts/check-paxd-native-digest.sh](../../../scripts/check-paxd-native-digest.sh) |
| source | — | [scripts/ci/codeql-build-mode-check.sh](../../../scripts/ci/codeql-build-mode-check.sh) |
| source | — | [scripts/ci/no-float-scan.sh](../../../scripts/ci/no-float-scan.sh) |
| source | — | [scripts/ci/path-length-check.sh](../../../scripts/ci/path-length-check.sh) |
| source | — | [scripts/ci/public-repo-audit.sh](../../../scripts/ci/public-repo-audit.sh) |
| source | — | [scripts/ci/run-tests.sh](../../../scripts/ci/run-tests.sh) |
| source | — | [scripts/ci/runner-routing-check.sh](../../../scripts/ci/runner-routing-check.sh) |
| source | — | [scripts/ci/solidity-state-surface.sh](../../../scripts/ci/solidity-state-surface.sh) |
| source | — | [scripts/ci/symbol-allowlist.sh](../../../scripts/ci/symbol-allowlist.sh) |
| source | — | [scripts/ci/xweb-forge-libs.sh](../../../scripts/ci/xweb-forge-libs.sh) |
| source | — | [scripts/codify.sh](../../../scripts/codify.sh) |
| source | — | [scripts/dump_app_state_for_height.sh](../../../scripts/dump_app_state_for_height.sh) |
| source | — | [scripts/evm_stress.sh](../../../scripts/evm_stress.sh) |
| source | — | [scripts/explorer/lint-backend.sh](../../../scripts/explorer/lint-backend.sh) |
| source | — | [scripts/explorer/lint-frontend.sh](../../../scripts/explorer/lint-frontend.sh) |
| source | — | [scripts/explorer/lint-services.sh](../../../scripts/explorer/lint-services.sh) |
| source | — | [scripts/generate-changelog.sh](../../../scripts/generate-changelog.sh) |
| source | — | [scripts/initialize_local_chain.sh](../../../scripts/initialize_local_chain.sh) |
| source | — | [scripts/interop/dependency-policy.sh](../../../scripts/interop/dependency-policy.sh) |
| source | — | [scripts/package-static-wasm.py](../../../scripts/package-static-wasm.py) |
| source | — | [scripts/paxeer-x/verify-sdk-program-dotnet.sh](../../../scripts/paxeer-x/verify-sdk-program-dotnet.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-go.sh](../../../scripts/paxeer-x/verify-sdk-program-go.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-jvm.sh](../../../scripts/paxeer-x/verify-sdk-program-jvm.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-python.sh](../../../scripts/paxeer-x/verify-sdk-program-python.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-rust.sh](../../../scripts/paxeer-x/verify-sdk-program-rust.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-schema.sh](../../../scripts/paxeer-x/verify-sdk-program-schema.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-swift.sh](../../../scripts/paxeer-x/verify-sdk-program-swift.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-typescript.sh](../../../scripts/paxeer-x/verify-sdk-program-typescript.sh) |
| source | — | [scripts/programs/dependency-policy.sh](../../../scripts/programs/dependency-policy.sh) |
| source | — | [scripts/programs/program-fixtures.sh](../../../scripts/programs/program-fixtures.sh) |
| source | — | [scripts/protoc.sh](../../../scripts/protoc.sh) |
| source | — | [scripts/qualify-mirror-live.sh](../../../scripts/qualify-mirror-live.sh) |
| source | — | [scripts/qualify-mirror-verification-live.sh](../../../scripts/qualify-mirror-verification-live.sh) |
| source | — | [scripts/run_blocktests.sh](../../../scripts/run_blocktests.sh) |
| source | — | [scripts/run-sha256sum.sh](../../../scripts/run-sha256sum.sh) |
| source | — | [scripts/snapshot.sh](../../../scripts/snapshot.sh) |
| source | — | [scripts/specgen.sh](../../../scripts/specgen.sh) |
| source | — | [scripts/start_faucet.sh](../../../scripts/start_faucet.sh) |
| source | — | [scripts/state_sync.sh](../../../scripts/state_sync.sh) |
| source | — | [scripts/update-swagger-ui-statik.sh](../../../scripts/update-swagger-ui-statik.sh) |
| source | — | [scripts/workspace/check-core-gates.sh](../../../scripts/workspace/check-core-gates.sh) |
| source | — | [scripts/workspace/check-core-gates-test.sh](../../../scripts/workspace/check-core-gates-test.sh) |
| source | — | [scripts/workspace/check-paxeer-manifests.sh](../../../scripts/workspace/check-paxeer-manifests.sh) |
| source | — | [scripts/workspace/check-paxeer-manifests-test.sh](../../../scripts/workspace/check-paxeer-manifests-test.sh) |
| source | — | [scripts/workspace/check-platform-dependencies.sh](../../../scripts/workspace/check-platform-dependencies.sh) |
| source | — | [scripts/workspace/check-platform-dependencies-test.sh](../../../scripts/workspace/check-platform-dependencies-test.sh) |

## scripts-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_QUALIFICATION_ARTIFACT_DIR` | [scripts/ci/beta-ledger-check.sh](../../../scripts/ci/beta-ledger-check.sh) |
| `env_var` | `GITHUB_OUTPUT` | [scripts/ci/release-version.py](../../../scripts/ci/release-version.py) |
| `env_var` | `LLVM_AR` | [scripts/package-static-wasm.py](../../../scripts/package-static-wasm.py) |
| `env_var` | `KUBERNETES_SERVICE_HOST` | [scripts/paxd_problems_detector.py](../../../scripts/paxd_problems_detector.py) |
| `env_var` | `PAXEER_X_EVIDENCE_DIR` | [scripts/paxeer-x/qualify-scan-load.py](../../../scripts/paxeer-x/qualify-scan-load.py) |
| `env_var` | `PAXEER_X_SCAN_NODE` | [scripts/paxeer-x/qualify-scan-load.py](../../../scripts/paxeer-x/qualify-scan-load.py) |
| `env_var` | `PAXEER_X_TASK_DEADLINE_EPOCH` | [scripts/paxeer-x/qualify-scan-load.py](../../../scripts/paxeer-x/qualify-scan-load.py) |
| `env_var` | `LAYERX_GO_PROGRAMS_ROOT` | [scripts/paxeer-x/verify-sdk-program-go.sh](../../../scripts/paxeer-x/verify-sdk-program-go.sh) |
| `env_var` | `LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE, PAXEER_X_PROGRAM_TERMINAL_V5_CORPUS` | [scripts/paxeer-x/verify-sdk-program-python.sh](../../../scripts/paxeer-x/verify-sdk-program-python.sh) |
| `env_var` | `CARGO_TARGET_DIR` | [scripts/qualification/paxeer-x/program-event-bounds.py](../../../scripts/qualification/paxeer-x/program-event-bounds.py) |
| `env_var` | `CARGO_TARGET_DIR` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `env_var` | `PAXEER_X_PROGRAM_TERMINAL_V5_ARTIFACTS` | [scripts/qualification/paxeer-x/program-terminal-v5.py](../../../scripts/qualification/paxeer-x/program-terminal-v5.py) |
| `env_var` | `CARGO_TARGET_DIR` | [scripts/qualification/paxeer-x/reference-custody.py](../../../scripts/qualification/paxeer-x/reference-custody.py) |
| `env_var` | `ATTESTOR_LISTEN_ADDR` | [scripts/wallet/check-deploy.sh](../../../scripts/wallet/check-deploy.sh) |
| `env_var` | `ATTESTOR_TLS_CA_FILE` | [scripts/wallet/check-deploy.sh](../../../scripts/wallet/check-deploy.sh) |
| source | — | [scripts/ci/beta-ci-inputs.sh](../../../scripts/ci/beta-ci-inputs.sh) |
| source | — | [scripts/ci/beta-qualify.sh](../../../scripts/ci/beta-qualify.sh) |
| source | — | [scripts/ci/release-artifact-record.sh](../../../scripts/ci/release-artifact-record.sh) |
| source | — | [scripts/ci/release-gate.sh](../../../scripts/ci/release-gate.sh) |
| source | — | [scripts/ci/symbol-allowlist.sh](../../../scripts/ci/symbol-allowlist.sh) |
| source | — | [scripts/codify.sh](../../../scripts/codify.sh) |
| source | — | [scripts/evm_stress.sh](../../../scripts/evm_stress.sh) |
| source | — | [scripts/explorer/gate-lint.sh](../../../scripts/explorer/gate-lint.sh) |
| source | — | [scripts/explorer/gate-test.sh](../../../scripts/explorer/gate-test.sh) |
| source | — | [scripts/explorer/lint-backend.sh](../../../scripts/explorer/lint-backend.sh) |
| source | — | [scripts/explorer/test-ratio.sh](../../../scripts/explorer/test-ratio.sh) |
| source | — | [scripts/initialize_local_chain.sh](../../../scripts/initialize_local_chain.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-dotnet.sh](../../../scripts/paxeer-x/verify-sdk-program-dotnet.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-jvm.sh](../../../scripts/paxeer-x/verify-sdk-program-jvm.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-rust.sh](../../../scripts/paxeer-x/verify-sdk-program-rust.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-schema.sh](../../../scripts/paxeer-x/verify-sdk-program-schema.sh) |
| source | — | [scripts/paxeer-x/verify-sdk-program-swift.sh](../../../scripts/paxeer-x/verify-sdk-program-swift.sh) |
| source | — | [scripts/programs/program-fixtures.sh](../../../scripts/programs/program-fixtures.sh) |
| source | — | [scripts/qualification/paxeer-x/program-http-producer.py](../../../scripts/qualification/paxeer-x/program-http-producer.py) |
| source | — | [scripts/qualify-mirror-live.sh](../../../scripts/qualify-mirror-live.sh) |
| source | — | [scripts/qualify-mirror-verification-live.sh](../../../scripts/qualify-mirror-verification-live.sh) |
| source | — | [scripts/run_blocktests.sh](../../../scripts/run_blocktests.sh) |
| source | — | [scripts/wallet/check-live.sh](../../../scripts/wallet/check-live.sh) |
| source | — | [scripts/wallet/gate-lint.sh](../../../scripts/wallet/gate-lint.sh) |
| source | — | [scripts/wallet/gate-test.sh](../../../scripts/wallet/gate-test.sh) |
| source | — | [scripts/wallet/scan-secrets.sh](../../../scripts/wallet/scan-secrets.sh) |

## scripts-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [scripts/bump_version/setup.go.tmpl](../../../scripts/bump_version/setup.go.tmpl) |

## tchain-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `package tools` | [tools/chain/cmd.go](../../../tools/chain/cmd.go) |
| `cli_command` | `Use:   "tools",` | [tools/chain/cmd.go](../../../tools/chain/cmd.go) |
| `cli_command` | `func main() {` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `blocks := flag.Int("blocks", 5, "blocks to commit after the genesis of a fixture (fixture mode)")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `chainID := flag.String("chain-id", defaultChainID, "chain id of the replayed block header and of a generated fixture")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `dataDir := flag.String("data", "", "chain data directory to replay against; it is written to, so hand over a disposable copy")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `home := flag.String("home", "", "throwaway application home whose data directory is the one replayed (default: a directory beside it)")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `keepStores := flag.Bool("keep-stores", false, "leave the fork modules' stores in a generated fixture, the shape of a chain whose binary carried them while its version map never did (fixture mode)")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `plan := flag.String("plan", app.V610Upgrade, "name of the upgrade plan to replay")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `preHeight := flag.Int64("pre-height", 0, "committed height the state carried before the plan (apply mode)")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| `cli_command` | `preMissing := flag.String("pre-missing", "", "comma separated stores the state did not carry before the plan (apply mode)")` | [tools/chain/upgrade-replay/main.go](../../../tools/chain/upgrade-replay/main.go) |
| source | — | [tools/chain/upgrade-replay/run.sh](../../../tools/chain/upgrade-replay/run.sh) |

## tchain-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/chain/upgrade-replay/run.sh](../../../tools/chain/upgrade-replay/run.sh) |

## tci-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `usage: tools/ci/beta-report.sh [--ledger PATH] [--check]` | [tools/ci/beta-report.sh](../../../tools/ci/beta-report.sh) |
| `cli_command` | `--check)` | [tools/ci/beta-report.sh](../../../tools/ci/beta-report.sh) |
| `cli_command` | `-h \| --help)` | [tools/ci/beta-report.sh](../../../tools/ci/beta-report.sh) |
| `cli_command` | `--ledger)` | [tools/ci/beta-report.sh](../../../tools/ci/beta-report.sh) |

## tdeploy-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `*) echo "usage: $0 [--source DIR] [--image NAME] [--push]" >&2; exit 2 ;;` | [tools/deploy/charting-library-image.sh](../../../tools/deploy/charting-library-image.sh) |
| `cli_command` | `--image) image="$2"; shift 2 ;;` | [tools/deploy/charting-library-image.sh](../../../tools/deploy/charting-library-image.sh) |
| `cli_command` | `--push) push=1; shift ;;` | [tools/deploy/charting-library-image.sh](../../../tools/deploy/charting-library-image.sh) |
| `cli_command` | `--source) source_dir="$2"; shift 2 ;;` | [tools/deploy/charting-library-image.sh](../../../tools/deploy/charting-library-image.sh) |
| `cli_command` | `--check)` | [tools/deploy/offchain-images.sh](../../../tools/deploy/offchain-images.sh) |
| `cli_command` | `--matrix)` | [tools/deploy/offchain-images.sh](../../../tools/deploy/offchain-images.sh) |
| `cli_command` | `--plan)` | [tools/deploy/offchain-images.sh](../../../tools/deploy/offchain-images.sh) |
| `cli_command` | `if __name__ == "__main__":` | [tools/deploy/railway-apply-variables.py](../../../tools/deploy/railway-apply-variables.py) |
| `cli_command` | `sys.exit(main(sys.argv[1:]))` | [tools/deploy/railway-apply-variables.py](../../../tools/deploy/railway-apply-variables.py) |
| `cli_command` | `if __name__ == "__main__":` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |
| `cli_command` | `sys.exit(main(sys.argv[1:]))` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |

## tdeploy-env

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `GHCR_USERNAME, GHCR_TOKEN` | [tools/deploy/railway-apply-variables.py](../../../tools/deploy/railway-apply-variables.py) |
| `env_var` | `RAILWAY_API_TOKEN` | [tools/deploy/railway-apply-variables.py](../../../tools/deploy/railway-apply-variables.py) |
| `env_var` | `GHCR_USERNAME, GHCR_TOKEN` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |
| `env_var` | `RAILWAY_API_TOKEN` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |
| `env_var` | `RAILWAY_WORKSPACE_ID` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |
| `env_var` | `ROLLOUT_POLL` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |
| `env_var` | `ROLLOUT_TIMEOUT` | [tools/deploy/railway-perps-project.py](../../../tools/deploy/railway-perps-project.py) |
| source | — | [tools/deploy/charting-library-image.sh](../../../tools/deploy/charting-library-image.sh) |
| source | — | [tools/deploy/offchain-images.sh](../../../tools/deploy/offchain-images.sh) |

## tdeploy-formats

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/deploy/charting-library.Dockerfile](../../../tools/deploy/charting-library.Dockerfile) |
| source | — | [tools/deploy/tests/fixtures/railway-apply-variables-replay.json](../../../tools/deploy/tests/fixtures/railway-apply-variables-replay.json) |

## tsh-cli

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/lxp_arith_proof.sh](../../../tools/lxp_arith_proof.sh) |
| source | — | [tools/lxp_qual_replay_matrix.sh](../../../tools/lxp_qual_replay_matrix.sh) |

## tsh-env

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tools/lxp_arith_proof.sh](../../../tools/lxp_arith_proof.sh) |

## txscan-cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `package cmd` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
| `cli_command` | `cmd.PersistentFlags().Int("batch-size", 100, "Batch size to query")` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
| `cli_command` | `cmd.PersistentFlags().Int("bps-limit", 400, "Blocks per second limit")` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
| `cli_command` | `cmd.PersistentFlags().Int("port", 9090, "GRPC server port")` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
| `cli_command` | `cmd.PersistentFlags().Int64("start-height", 0, "Start height")` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
| `cli_command` | `cmd.PersistentFlags().String("state-dir", "", "State file directory, the scanner will record the last scanned offset and scan results")` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
| `cli_command` | `Use:   "scan-tx",` | [tools/tx-scanner/cmd/scan.go](../../../tools/tx-scanner/cmd/scan.go) |
