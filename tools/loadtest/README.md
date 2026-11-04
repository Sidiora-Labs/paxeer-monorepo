# pax-load

A Go load generator that signs EVM transactions, distributes them across configured
node endpoints, and collects submission, receipt, and block metrics.

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). Its kernel
participates in block proposal and execution; receipts are final when their block
is committed. Public JSON-RPC uses the root URL of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, or
`https://api-mainnet-beta.paxeer.network`. See the
[public RPC reference](../../docs/site/docs/reference/public-rpc.md).

The checked-in [profiles](profiles/) use chain ID `713714` for isolated fixtures.
That value is not the mainnet chain ID. Match `chainId` to the endpoint's
`eth_chainId` before sending; the CLI has no chain-ID override flag or automatic
identity check. Multiple endpoints must serve the same chain.

## Quick Start

### 1. Build

From the repository root, enter this directory. Install Go compatible with the
`go 1.25.0` directive in [go.mod](go.mod), then build:

```bash
cd tools/loadtest
make build
```

The [Makefile](Makefile) writes **`build/paxload`** using the committed Go contract
bindings. This build does not require regenerating Solidity bindings. All commands
below assume the working directory is `tools/loadtest`.

### 2. Create Config

Save this as `my-config.json`. It describes a local chain-125 node; replace the
endpoint for your environment. The initial run below simulates sending.

```json
{
  "endpoints": ["http://127.0.0.1:8545"],
  "chainId": 125,
  "paxChainID": "local-chain-125",
  "scenarios": [
    {"name": "EVMTransfer", "weight": 100}
  ],
  "accounts": {
    "count": 100,
    "newAccountRate": 0
  },
  "bounds": {
    "minAverageTps": 5,
    "maxP99LatencyMs": 5000,
    "minTotalTxs": 100
  },
  "settings": {
    "workers": 5,
    "tps": 100,
    "statsInterval": "10s",
    "bufferSize": 1000,
    "reportPath": "loadtest-report.json"
  }
}
```

For the isolated fixture instead, copy `profiles/local.json` and retain its
fixture chain ID only when the local node uses that ID.

### 3. Run

```bash
./build/paxload --config my-config.json --dry-run
```

Let several statistics intervals elapse, then press Ctrl+C. SIGINT or SIGTERM
requests graceful shutdown, final reporting, and evaluation of the configured
bounds. A minimum transaction count is an assertion at shutdown, not a stop limit;
there is no duration flag.

Dry-run skips real contract deployment and transaction submission. Its throughput
measures the local simulation. Block tracking, ramp-up, user-latency tracking, and
transaction-file mode can still contact nodes, so dry-run is not an offline
connectivity check.

For a live run, omit `--dry-run` in an environment that can fund the generated
accounts and deployer. The generator creates fresh random keys on every run and
does not import wallet keys or fund accounts. An ordinary fee-paying node will
reject unfunded transactions; changing the endpoint to mainnet does not provide
funding.

## Command Line Options

Defaults below are effective settings from
[config/settings.go](config/settings.go), before profile values or explicit flags.
The `settings` column gives the nested JSON key.

| Flag | Default | `settings` key | Description |
| --- | --- | --- | --- |
| `--config`, `-c` | Required | — | JSON profile path |
| `--workers`, `-w` | `1` | `workers` | Sending goroutines per endpoint |
| `--nodes`, `-n` | `0` | — | Use the first N endpoints; 0 uses all |
| `--tps`, `-t` | `0` | `tps` | Shared sending rate across all workers; 0 is unlimited |
| `--stats-interval`, `-s` | `10s` | `statsInterval` | Periodic log interval; use a positive duration |
| `--buffer-size`, `-b` | `1000` | `bufferSize` | Queue capacity for each endpoint's worker group |
| `--dry-run` | `false` | `dryRun` | Simulate deployments and sends |
| `--debug` | `false` | `debug` | Enable detailed transaction and endpoint logging |
| `--track-receipts` | `false` | `trackReceipts` | Poll submitted transaction receipts |
| `--track-blocks` | `false` | `trackBlocks` | Subscribe to new heads using the first endpoint |
| `--track-user-latency` | `false` | `trackUserLatency` | Read timestamp-valued transactions from the first endpoint's latest block |
| `--prewarm` | `false` | `prewarm` | Queue account self-transactions before the main workload |
| `--ramp-up` | `false` | `rampUp` | Adjust the shared limiter using the built-in ramp curve and block observations |
| `--metricsListenAddr` | `0.0.0.0:9090` | — | Prometheus HTTP listener; serves `/metrics` |
| `--report-path` | Empty | `reportPath` | Write final statistics as JSON |
| `--min-tps` | `0` | `minTps` | Minimum overall average submission TPS; 0 disables |
| `--max-p99-latency` | `0s` | `maxP99Latency` | Maximum send p99 per reported endpoint; 0 disables |
| `--min-total-txs` | `0` | `minTotalTxs` | Minimum total submission attempts; 0 disables |
| `--txs-dir` | Empty | `txsDir` | Dispatch transactions to files instead of the sender |
| `--target-gas` | `10000000` | `targetGas` | Gas budget per transaction-file batch |
| `--num-blocks-to-write` | `100` | `numBlocksToWrite` | Transaction-file batch limit |

## Examples

### Basic Load Test

On an isolated fixture configured for chain `713714`, this invocation evaluates
the existing EVM transfer profile's bounds (minimum 5 TPS, maximum p99 5000 ms),
adds a minimum total, and writes a JSON report after graceful shutdown:

```bash
./build/paxload --config profiles/evm_transfer.json \
  --workers 5 --tps 100 --min-total-txs 100 \
  --report-path fixture-report.json
```

It requires the fixture's funding behavior described above. Add `--dry-run` to
exercise generation and reporting without broadcasting; that result describes
simulation performance only. Thresholds are examples, not network guarantees.

### High Throughput Test

For the same isolated fixture, explicitly request unlimited sending:

```bash
./build/paxload --config profiles/evm_transfer.json \
  --workers 20 --buffer-size 2000 --tps 0
```

### Debug Mode

```bash
./build/paxload --config my-config.json --debug --dry-run
```

### With Receipt Tracking

For a live fixture run, enable receipt polling and block subscriptions:

```bash
./build/paxload --config profiles/evm_transfer.json --tps 100 \
  --track-receipts --track-blocks
```

Block tracking and ramp-up derive a WebSocket URL from the first endpoint:
`http` becomes `ws`, `https` becomes `wss`, and port `8545` becomes `8546`.
The helper also replaces `evm-rpc` with `evm-ws` in fixture hostnames. Public nodes
use the root WebSocket URL. See [utils/endpoint.go](utils/endpoint.go).

### User Latency Monitoring

```bash
./build/paxload --config profiles/evm_transfer.json --tps 100 \
  --track-user-latency --stats-interval 5s
```

This tracker interprets positive transaction values as Unix timestamps, matching
the `EVMTransfer` workload. It scans the latest block, not just this process's
transactions, and is meaningful only for a controlled workload with that encoding.

## Configuration

### Basic Structure

The complete quick-start JSON shows the structure. The CLI requires non-empty
`endpoints` and `scenarios`. Provide a positive account count either at the top
level or within each scenario. `settings` is optional; omitted settings use
defaults. [config/config.go](config/config.go) declares the profile fields.

`chainId` controls transaction signing. `paxChainID` is a string label for metrics;
it does not select a chain or tenant. Sub-chains are tenant domains in the
database, and this CLI has no tenant-domain selector. `mockDeploy` only replaces
deployment with attachment to generated addresses; it does not disable sending.
Use `--dry-run` for a simulation.

### Scenarios

Names are case-insensitive and weights select the workload mix:

```json
{
  "scenarios": [
    {"name": "EVMTransfer", "weight": 50},
    {"name": "ERC20", "weight": 30},
    {"name": "ERC721", "weight": 20}
  ]
}
```

Each entry creates a separate scenario instance. Use positive weights for active
entries; zero-weight entries still go through setup before selection excludes them.

### Account Management

```json
{
  "accounts": {
    "count": 100,
    "newAccountRate": 0.1
  }
}
```

Scenarios share the top-level pool unless they provide their own `accounts` object.
`newAccountRate` is the probability of generating a new account for a pool request;
use a value from 0 to 1. New accounts are also unfunded. Prewarming sends
self-transactions and does not fund them.

### Settings

For each setting, an explicitly supplied CLI flag overrides the nested `settings`
value, which overrides the default. Explicit flags can set `--tps 0` or
`--dry-run=false`; merely omitting a flag does not override the profile. Zero-valued
JSON settings are omitted when loaded into Viper and generally fall back to
defaults. Duration settings use strings such as `"10s"` or `"250ms"`.

### Performance Bounds and Reports

Bounds may be placed in the top-level profile object:

```json
{
  "bounds": {
    "minAverageTps": 5,
    "maxP99LatencyMs": 5000,
    "minTotalTxs": 100
  },
  "settings": {
    "reportPath": "report.json"
  }
}
```

For each bound independently, precedence is **explicit CLI flag > nonzero nested
setting > top-level bound > disabled default**. The nested equivalents are
`settings.minTps`, `settings.maxP99Latency` (a duration string), and
`settings.minTotalTxs`. The top-level latency is an integer number of milliseconds.
A zero nested value inherits a nonzero top-level bound. Explicit flags
`--min-tps 0 --max-p99-latency 0s --min-total-txs 0` disable the respective bounds
even when the profile sets them.

Reports use **`--report-path` > `settings.reportPath` > no report**. Although
`LoadConfig` accepts a top-level `reportPath`, the CLI does not consume that field.
An explicit `--report-path=` suppresses the configured report. The parent directory
must exist; a report overwrites the named file. Report write errors propagate as
command failures.

After graceful shutdown, the CLI prints final statistics, writes the report when
requested, and checks the bounds. Equality passes. A violation returns exit **1**
with `performance bounds violated` and the limit and observed value for each
failed bound (`min_average_tps`, `min_total_txs`, or
`max_p99_latency:<endpoint>`). The report is written before bound enforcement, so a
failing performance run can still leave its statistics for inspection. Successful
shutdown with satisfied bounds and no run/report error returns exit **0**.

Total transactions and average TPS count completed **submission attempts, including
failed sends**. The p99 check uses successful send-call latency samples retained
per endpoint, not receipt or commit latency, and only checks endpoints present in
the final endpoint map. Receipt failures are logged; individual send failures are
also logged and do not alone force a nonzero exit. Inspect errors and sample counts
alongside bounds when judging a live run. Dry-run samples measure simulated sends;
file-writing mode does not collect sender throughput samples.

## Available Scenarios

The [scenario factory](generator/scenarios/factory.go) registers:

- **EVMTransfer**: Native-value transfers with a Unix timestamp encoded as value.
- **EVMTransferNoop**: Zero-value self-transfers.
- **ERC20**: Transfers using the bundled test token.
- **ERC20Noop**: Calls a test token whose transfer returns without changing balances.
- **ERC20Conflict**: Test-token transfers that also update a shared counter.
- **ERC721**: Mints using the bundled test NFT contract.
- **Disperse**: Calls the bundled disperse contract with 100 generated recipients.

Contract scenarios deploy their own test contracts unless deployment is simulated.
These are workload contracts, not production token or module addresses. Scenario
gas and fee selections are declared in [config/gas.go](config/gas.go).

## Output

### Standard Metrics

Periodic output includes submission TPS and latency, for example:

```text
throughput tps=133.00, txs=1330, latency(avg=8ms p50=5ms p99=27ms max=494ms)
```

The final console summary includes scenario counts, endpoint performance, and
overall performance. Prometheus metrics are served at `/metrics` on the listener
selected by `--metricsListenAddr`.

### User Latency (with --track-user-latency)

```text
user latency height=5191 txs=32 min=1s p50=2s max=5s
```

This is the timestamp-value estimate described above, separate from send latency.

### Block Stats (with --track-blocks)

```text
blocks height=5191 time(p50=2s p99=5s max=8s) gas(p50=21000 p99=50000 max=100000)
```

Block time here measures time between observed headers. It does not measure a
separate settlement stage.

### JSON Report

Reports contain `load_test_statistics`, `scenario_distribution`,
`endpoint_performance`, `overall_tps`, and `overall_performance`, plus optional
`block_statistics` and `gas_statistics`. Go duration fields, including
`endpoint_performance.<endpoint>.latency_p99` and
`overall_performance.total_runtime`, are JSON integers in **nanoseconds**.
`start_time` is a timestamp string. The report is final statistics, not a list of
bound verdicts. See [stats/logger.go](stats/logger.go) for the exact schema.

## Development

Run these from `tools/loadtest`:

```bash
make build
make test
make lint
```

`make test` runs the Go packages with race detection and coverage. `make lint`
requires `golangci-lint`. `make clean` removes both `build/` and
`generator/bindings/`; rebuilding after cleaning requires restoring the committed
bindings or regenerating them with `make generate` and the Makefile's Solidity and
`abigen` tools.

## Troubleshooting

### Connection Issues

- Check root endpoint URLs and confirm `eth_chainId` matches the signing ID.
- For block tracking or ramp-up, check the derived WebSocket endpoint and new-head subscription support.
- Check send errors, JSON-RPC responses, and receipt status; dry-run does not prove live admission or funding.

### Low Performance

- Inspect send failures and endpoint sample counts before tuning concurrency.
- Increase `--workers` or `--buffer-size` as needed, and check whether `--tps` limits the shared rate.
- Keep comparison runs on the same workload and duration; average TPS includes elapsed setup time.

### Memory Issues

- Reduce `--buffer-size` and worker count.
- Disable receipt tracking when receipt metrics are unnecessary.
- Account for the number of endpoints and generated account pools when sizing a run.
