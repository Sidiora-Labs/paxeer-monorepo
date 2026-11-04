# Benchmark Scenarios

This folder contains scenario configurations for benchmarking the merged Paxeer network locally. [`tools/benchmark/benchmark.sh`](../benchmark.sh) builds and starts a local `paxd` that generates its own load. The script passes the scenario file path through `BENCHMARK_CONFIG`, and [`LoadConfig`](../../../node/benchmark/config.go) reads it inside the node. The benchmark manager refuses to start on a [live EVM chain ID](../../../modules/evm/config/config.go), including mainnet chain ID 125 (`0x7d`).

## Usage

Run these commands from the repository root:

```bash
# Use the default scenario (evm.json, EVMTransfer)
./tools/benchmark/benchmark.sh

# Use ERC20 scenario
BENCHMARK_CONFIG=tools/benchmark/scenarios/erc20.json ./tools/benchmark/benchmark.sh

# Use mixed scenario (EVMTransfer + ERC20)
BENCHMARK_CONFIG=tools/benchmark/scenarios/mixed.json ./tools/benchmark/benchmark.sh
```

When `BENCHMARK_CONFIG` is unset or empty, the script resolves `evm.json` relative to its own directory. An explicitly supplied relative path is resolved from the working directory. The default benchmark data directory is `$HOME/.pax-benchmark`; see the [home-directory guard](../home.go) for initialization and reset rules.

## Available Scenarios

### `evm.json` (used when `BENCHMARK_CONFIG` is unset)

Simple EVM native token transfers. No contract deployment required.

- **Scenarios**: EVMTransfer (weight: 1)
- **Accounts**: 5000

### `default.json`

Same content as `evm.json`.

### `erc20.json`

ERC20 token transfers. The node deploys the contract during setup.

- **Scenarios**: ERC20 (weight: 1)
- **Accounts**: 5000

### `mixed.json`

Combination of native transfers and ERC20 transfers.

- **Scenarios**: EVMTransfer (weight: 3), ERC20 (weight: 1)
- **Accounts**: 5000

## Configuration Format

Configurations follow the `LoadConfig` format of the `github.com/paxeer-network/pax-load` module pinned in the [root Go module](../../../go.mod). For example, this is the content of [`evm.json`](evm.json):

```json
{
  "accounts": {
    "count": 5000,
    "newAccountRate": 0.0
  },
  "scenarios": [
    {
      "name": "EVMTransfer",
      "weight": 1
    }
  ]
}
```

`accounts.count` sets the shared account pool size; `newAccountRate: 0.0` keeps that pool fixed. Each scenario's `name` selects a generator and `weight` controls weighted load selection. A zero weight excludes the scenario from load generation.

`LoadConfig` always replaces `chainID` and `paxChainID` with the running chain's values and sets `mockDeploy` to `true`. Here, `mockDeploy` leaves deployment ownership with the in-process benchmark generator: contract scenarios still submit deployment transactions and wait for successful receipts before generating load. Set `BENCHMARK_TXS_PER_BATCH` to a positive integer to change the default batch size of 1000.

## Supported Scenario Names

The [benchmark generator](../../../node/benchmark/generator.go) handles these scenario names:

| Name | Description | Requires Deployment |
|------|-------------|---------------------|
| `EVMTransfer` | Native token transfers | No |
| `EVMTransferNoop` | No-op transfers | No |
| `ERC20` | ERC20 token transfers | Yes |
| `ERC721` | ERC721 NFT transfers | Yes |
| `ERC20Conflict` | ERC20 with conflict patterns | Yes |
| `ERC20Noop` | ERC20 no-op transfers | Yes |
| `Disperse` | Batch token dispersal | Yes |

## Two-Phase Execution

The generator starts with a warmup counter (`WarmupBlocks = 3`), then advances through setup and load generation inside the same node:

1. **Setup Phase**: Deployment transactions are created and processed. After each block,
   receipts are checked for successful deployment and contract addresses. Scenarios without
   contracts attach with a zero address. A failed deployment receipt stops the benchmark
   with a panic.

2. **Load Phase**: Once all contracts are deployed, the benchmark generates load
   transactions according to the configured scenario weights.

Typical log messages below show the configuration and phase transitions; deployment messages
apply to contract scenarios, and the final scenario count depends on the configuration:

```text
benchmark generator config txsPerBatch=1000
benchmark: Warmup complete, transitioning to setup phase
benchmark: Scenario doesn't need deployment, attaching with zero address scenario=...
benchmark: Created deployment transaction (will only deploy once) scenario=...
benchmark: Contract deployed successfully scenario=... address=0x...
benchmark: All scenarios deployed, transitioning to load phase
benchmark: Load generator initialized and ready scenarios=2
```
