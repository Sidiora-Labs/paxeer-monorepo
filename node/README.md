# Paxeer node application

This directory holds the Go application used by `paxd`: module and store wiring,
transaction admission and execution, block processing, receipts, and upgrades.
The LayerX kernel runs inside this application through the native host in
[`modules/lxphost`](../modules/lxphost). Paxeer mainnet-beta is one BFT network
with EVM chain ID 125; the proposer executes transactions and proposes them in
sequence order, and a receipt is final when its block is committed.

## Layout

- [`ante/`](ante/) — Cosmos and EVM transaction checks and delivery handlers.
- [`antedecorators/`](antedecorators/) — authorization, fee, gas, gasless transaction, and priority decorators.
- [`apptesting/`](apptesting/) — application, context, account, and transaction helpers for keeper tests.
- [`benchmark/`](benchmark/) — in-process transaction generation, scenario configuration, and timing metrics.
- [`genesis/`](genesis/) — genesis package and tests of the node's genesis file layout.
- [`legacyabci/`](legacyabci/) — begin-block, transaction, and end-block handlers used by the application.
- [`params/`](params/) — address and denomination settings, node profiles, encoding, and simulation parameters.
- [`sequencer/`](sequencer/) — integrated sequencing, signed receipts, consensus batches, WAL storage, replication, and recovery.
- [`testdata/`](testdata/) — JSON fixtures for allocation and upgrade tests, with an export height file.
- [`upgrades/`](upgrades/) — chain- and height-specific hard-fork handler registration and execution.
- [`wasmbinding/`](wasmbinding/) — CosmWasm custom queries and messages for native modules.
- [`app.go`](app.go) — application construction, keeper and module registration, genesis initialization, proposal validation, and block execution.
- [`abci.go`](abci.go) — ABCI transaction, block, and commit entry points.
- [`encoding.go`](encoding.go) — application codec and interface registration for test encoding configurations.
- [`const.go`](const.go) — application name, account prefix, and optimistic processing timeout.
- [`utils.go`](utils.go) — optimistic processing state and block request types.
- [`genesis.go`](genesis.go) — default module genesis state and streamed genesis import options.
- [`export.go`](export.go) — application state and validator export.
- [`paxdb.go`](paxdb.go) — state commitment, state storage, and snapshot configuration.
- [`receipt_store_config.go`](receipt_store_config.go) — receipt storage configuration and directory selection.
- [`receipt.go`](receipt.go) — translation of Cosmos and CosmWasm events into EVM receipts and logs.
- [`precompiles.go`](precompiles.go) — keeper access for native EVM precompiles.
- [`precompiles_v6120.go`](precompiles_v6120.go) — activation and version selection for native market precompiles.
- [`giga_v6120.go`](giga_v6120.go) — routing of market precompiles from the optimized executor to ordinary execution.
- [`prioritizer.go`](prioritizer.go) — transaction priority hints.
- [`process_proposal_order.go`](process_proposal_order.go) — validation of sequenced transaction order.
- [`process_proposal_proposer.go`](process_proposal_proposer.go) — validation of the scheduled proposer.
- [`leader_source.go`](leader_source.go) — committed leader schedule access for consensus.
- [`node_start.go`](node_start.go) — validation of the committed application hash.
- [`sequenced_halt.go`](sequenced_halt.go) — fatal sequenced-block delivery errors.
- [`sequenced_metrics.go`](sequenced_metrics.go) — sequenced proposal rejection and halt counters.
- [`invariance.go`](invariance.go) — supply invariance checks.
- [`metrics.go`](metrics.go) — application processing metrics.
- [`benchmark.go`](benchmark.go) — application integration for benchmark generation and receipt processing.
- [`eth_replay.go`](eth_replay.go) — Ethereum block replay and state-test integration.
- [`upgrades.go`](upgrades.go) — upgrade registration, merged module activation, migrations, and store changes.
- [`upgrades_v611.go`](upgrades_v611.go) — token balance migration using embedded holder data.
- [`upgrades_v6120.go`](upgrades_v6120.go) — market module migration and activation.
- [`upgrades_v6120_pointers.go`](upgrades_v6120_pointers.go) — pointer registration during the market upgrade.
- [`xweb_attestors.go`](xweb_attestors.go) — initial web-search attestor registration and validation.
- [`tags`](tags) — embedded upgrade version list.
- [`wasm_abi.json`](wasm_abi.json) — ABI for CosmWasm execution, instantiation, and queries used by receipt tests.
- [`test_helpers.go`](test_helpers.go) — application test wrapper and transaction helpers.

The Go test files beside these sources cover application wiring, proposal and
receipt handling, module execution, storage, and upgrades.

## Build and test

The package belongs to the root [Go module](../go.mod). Run these targets from
the repository root; the directory has no separate build manifest or Makefile.

```sh
make paxd-build
make test-paxd-domain
```

The root [Makefile](../Makefile) builds the kernel, program runtime, and EVM
executor native libraries before building `build/paxd` through
[`chain.mk`](../chain.mk). The test target prepares those libraries, runs the
kernel host tests, then runs the Go tests under `node`, `modules/evm/keeper`,
and `rpc` with cgo enabled. The build target uses offline Go dependency
resolution, so its dependencies must already be available locally. See the
[installation page](../docs/site/paxeer-docs/src/app/installation/page.tsx)
for build prerequisites.

## Configuration

The daemon loads `config.toml` and `app.toml` from the node home's configuration
directory and passes application options into this package. Node profiles are
defined in [`params/config.go`](params/config.go).

- `app.toml` supplies the state commitment and state storage options read by
  [`paxdb.go`](paxdb.go), receipt storage options read by
  [`receipt_store_config.go`](receipt_store_config.go), and
  `light_invariance.supply_enabled` read by [`invariance.go`](invariance.go).
- The `sequencer` section of `app.toml` configures the integrated role, execution
  mode, WAL directory, receipt signing key file, receipt forwarding, standby
  replication, queue bounds, and lease duration. The option names and template
  are in [`sequencer/config.go`](sequencer/config.go). Replication reads the
  certificate, private key, and CA files selected by
  `sequencer.replication-tls-cert`, `sequencer.replication-tls-key`, and
  `sequencer.replication-tls-ca`; the receipt key file is selected by
  `sequencer.receipt-key-file`.
- `config/genesis.json` supplies the chain genesis used by replay.
  `genesis.stream-import` and `genesis.genesis-stream-file` select streamed
  genesis import in [`genesis.go`](genesis.go).
- `upgrade-info.json` supplies the named upgrade plan read through the upgrade
  keeper. `UPGRADE_VERSION_LIST` overrides the embedded version list in
  [`tags`](tags).
- `BENCHMARK_CONFIG` selects a JSON scenario configuration read by
  [`benchmark/config.go`](benchmark/config.go). `BENCHMARK_TXS_PER_BATCH`
  controls generation batch size in [`benchmark/generator.go`](benchmark/generator.go).
  The benchmark manager refuses live EVM chain IDs.

## Documentation

- [Sequencing and block finality](../docs/wiki/Sequencing.md).
- [Validator runbook](../docs/release/7.0.0-validator-runbook.md).
- [Run a node](../docs/site/paxeer-docs/src/app/run-node/page.tsx) on the documentation site.
- [Node configuration](../docs/site/paxeer-docs/src/app/configuration/page.tsx) on the documentation site.
- [Consensus](../docs/site/paxeer-docs/src/app/consensus/page.tsx) on the documentation site.
- [Sequencing](../docs/site/paxeer-docs/src/app/sequencing/page.tsx) on the documentation site.
- [CosmWasm bindings](wasmbinding/README.md).
