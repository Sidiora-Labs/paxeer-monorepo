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

The checked-in [profiles](profiles/) target operator-owned loopback nodes on chain
125. A different chain ID requires an isolated node and a matching profile.
Before any deployment or traffic, the executable calls `eth_chainId` on **every**
configured endpoint, including entries excluded by `--nodes`, and refuses a
mismatch. The CLI has no chain-ID override flag. All endpoints must serve the
configured signing chain.

## Quick Start

### 1. Build

From the repository root, enter this directory. Use the pinned Go **1.25.8** toolchain; the nested module keeps its
`go 1.25.0` language directive in [go.mod](go.mod). Then build:

```bash
cd tools/loadtest
make build
```

The [Makefile](Makefile) writes **`build/paxload`** using the Go contract
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

For an isolated local run, copy `profiles/local.json` and use a node whose
`eth_chainId` matches its configured chain ID 125.

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
| `--track-user-latency` | `false` | `trackUserLatency` | Start a receipt-observation loop on the first endpoint |
| `--prewarm` | `false` | `prewarm` | Queue account self-transactions before the main workload |
| `--ramp-up` | `false` | `rampUp` | Adjust the shared limiter using the built-in ramp curve and block observations |
| `--metricsListenAddr` | `0.0.0.0:9090` | — | Prometheus HTTP listener; serves `/metrics` |
| `--report-path` | Empty | `reportPath` | Write final statistics as JSON |
| `--min-tps` | `0` | `minTps` | Minimum overall average submission TPS; 0 disables |
| `--max-p99-latency` | `0s` | `maxP99Latency` | Maximum send p99 per reported endpoint; 0 disables |
| `--min-total-txs` | `0` | `minTotalTxs` | Minimum accepted submissions; 0 disables |
| `--txs-dir` | Empty | `txsDir` | Dispatch transactions to files instead of the sender |
| `--target-gas` | `10000000` | `targetGas` | Gas budget per transaction-file batch |
| `--num-blocks-to-write` | `100` | `numBlocksToWrite` | Transaction-file batch limit |

## Examples

### Basic Load Test

On an operator-owned chain-125 node, this invocation evaluates
the existing EVM transfer profile's bounds (minimum 5 TPS, maximum p99 5000 ms),
adds a minimum total, and writes a JSON report after graceful shutdown:

```bash
./build/paxload --config profiles/evm_transfer.json \
  --workers 5 --tps 100 --min-total-txs 100 \
  --report-path fixture-report.json
```

It requires the fixture's funding behavior described above. Thresholds are
examples, not network guarantees.

For a simulation, provide the profile's local chain-125 RPC node and run:

```bash
./build/paxload --config profiles/evm_transfer.json --dry-run \
  --workers 2 --buffer-size 8 --stats-interval 20ms --tps 100 \
  --min-tps 0 --max-p99-latency 0s --min-total-txs 1 \
  --report-path fixture-report.json --metricsListenAddr 127.0.0.1:0
```

After a statistics interval, Ctrl+C requests shutdown and bound evaluation.
Dry-run statistics describe simulated sends; the executable still checks chain
identity before generation.

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

The [tracker](stats/user_latency_tracker.go) measures elapsed time from a
registered send to observation of its committed receipt, checking the receipt's
block and transaction index. Only hashes registered with `TrackTransaction`
contribute samples; transaction values do not supply timing data.

## Configuration

### Basic Structure

The complete quick-start JSON shows the structure. The CLI admits 1–256 HTTP(S) `endpoints` and 1–256 `scenarios`, a positive
`chainId`, and known scenario names before constructing generators or services.
Weights must be nonnegative with a positive total no greater than 2147483647.
Provide account settings either at the top level or within each scenario. `settings` is optional; omitted settings use
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
use a finite value from 0 to 1. Fixed pools require a positive count; a fresh-only
rate of exactly 1 permits count 0. Combined configured pools are capped at 100000
accounts. New accounts are also unfunded. Prewarming sends
self-transactions and does not fund them.

### Settings

For each setting, an explicitly supplied CLI flag overrides the nested `settings`
value, which overrides the default. Explicit flags can set `--tps 0` or
`--dry-run=false`; merely omitting a flag does not override the profile. Zero-valued
JSON settings are omitted when loaded into Viper and generally fall back to
defaults. Duration settings use strings such as `"10s"` or `"250ms"`. The resolved statistics
interval must be positive. Workers must be in 1–1024 with at most 4096 total
participants; buffers must be in 1–100000 with at most 1000000 queued transactions
across endpoints. TPS must be finite and nonnegative. Transaction-file mode
requires positive `targetGas` and `numBlocksToWrite`. Gas ranges must be positive
and ordered, and minimum fee caps must cover maximum tip caps before sampling.

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

Reports use **`--report-path` > nonempty `settings.reportPath` > top-level
`reportPath` > no report**. An empty nested value inherits the top-level path.
An explicit `--report-path=` suppresses the configured report. The parent directory
must exist; a report overwrites the named file. Report write errors propagate as
command failures.

On SIGINT or SIGTERM, the CLI stops and joins generation, closes sender queues,
drains accepted submissions (with a 30-second deadline), and finalizes the writer
tail atomically. It then prints final statistics, writes the report when
requested, and checks the bounds. Equality passes. A violation returns exit **1**
with `performance bounds violated` and the limit and observed value for each
failed bound (`min_average_tps`, `min_total_txs`, or
`max_p99_latency:<endpoint>`). The report is written before bound enforcement, so a
failing performance run can still leave its statistics for inspection. Successful
shutdown with satisfied bounds and no run/report error returns exit **0**.

`load_test_statistics.total_txs` and scenario counts include failed submission
attempts. `overall_performance.total_transactions` and average TPS count
**accepted submissions**. The p99 check uses successful send-call latency samples
retained per endpoint, rather than receipt or commit latency. Enabled success
bounds require at least one accepted sample; the p99 bound also requires samples
for every endpoint named in `required_endpoints` or `endpoint_performance`.
An incomplete report fails any enabled bound. Receipt failures are logged;
submission failures survive draining and cause
exit status 1, as do metrics startup/serve failures, drain timeouts, writer
finalization failures, and report write failures. Inspect errors and sample counts
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
- **Disperse**: Calls the bundled disperse contract with up to four fresh
  recipients, limited by the transaction gas budget.

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

The observation log reports the scope, sample count, missing receipts and
per-endpoint outcomes. The scope is
`full_run_send_to_committed_receipt_observation`; an empty sample set is reported
as `insufficient_samples=true`.

### Block Stats (with --track-blocks)

The final block statistics report committed-header timestamp intervals, gas
usage and delivery lag separately. The collector backfills missing heights and
checks parent hashes. Its scope is `full_run_committed_headers`.

### JSON Report

Reports contain `required_endpoints`, `outcomes`, `throughput_basis`,
`commit_evidence`, `incomplete`, `load_test_statistics`, `scenario_distribution`,
`endpoint_performance`, `overall_tps`, and `overall_performance`, plus optional
`user_latency`, `evidence_errors`, `block_statistics` and `gas_statistics`.
`throughput_basis` is `node_accepted_submissions`. Go duration fields, including
`endpoint_performance.<endpoint>.latency_p99` and
`overall_performance.total_runtime`, are JSON integers in **nanoseconds**.
`load_test_statistics.start_time` is a timestamp string. The report is final statistics, not a list of
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
`abigen` tools. `make build` uses `-mod=readonly` and never installs binding tools
or tidies dependencies; `make tidy` is a separate maintenance operation.

For deliberate regeneration, `make install-tools` installs abigen v1.17.0 and
SHA-256-verified Solidity 0.8.19 release bytes into private `.tools/`. The
`setup-node` target installs only native solc; no Node or nvm download is needed.
`make generate` refuses missing or changed tools and records compiler/generator
versions, tool hashes, and ABI/bytecode/binding hashes in per-binding `.provenance`
files. It renders scenario constructors with `config.Scenario` and updates the
factory using the compiled `build/paxload scaffold` commands. Replace every
constructor and transaction placeholder that the scaffold emits before building a
new scenario.
The image uses digest-pinned stages, disables automatic toolchain downloads and
checks the compiler identity before building.

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
