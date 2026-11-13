# LayerXLedger precompile

This directory holds the Go implementation, embedded ABI and tests for the `layerxledger` EVM precompile at `0x101A`. It connects asset registration, supply operations, per-asset accounts, transfers, exact payments and ledger reads to the network's [ledger keeper](../../modules/layerxledger/). Writes resolve the caller's bound identity and emit EVM logs from kernel receipts; the precompile runs inside `paxd` on the same network as the kernel.

## Layout

- `layerxledger.go` — executor, keeper interface, merge constructor registration and call validation.
- `abi.json` — published method arguments, return values, mutability and events.
- `abi.go` — embeds and loads the ABI.
- `assets.go` — asset registration, pause, unpause, mint, burn and asset event encoding.
- `transfers.go` — account opening, SEND envelope handling and EVM log emission.
- `payments.go` — exact payments, payment logs and payer-grant entry points that return refusals.
- `views.go` — asset, balance, supply, account ownership and account sequence queries.
- `history.go` — access to the persisted owned-account registry for node history reads.
- `gas.go` — method gas table.
- `errors.go` — kernel and module error conversion to `LXP_` revert reasons.
- `harness_test.go` — shared application, keeper and EVM test setup.
- `layerxledger_test.go` — constructor registration and call authorization tests.
- `abi_test.go` — method and event ABI assertions.
- `abi_published_views_test.go` — published ledger and authorization ABI view and digest assertions.
- `activation_test.go` — availability tests at the merge activation boundary.
- `assets_test.go` — registration, pause, mint and burn tests.
- `transfers_test.go` — account opening, sends and persisted recovery tests.
- `payments_test.go` — exact payment and payer-grant refusal tests.
- `views_test.go` — ledger reads, reads while paused and sequence recovery tests.
- `gas_test.go` — gas table assertions.
- `gas_conformance_test.go` — EVM gas accounting across ledger methods.
- `errors_test.go` — revert reason and kernel result-code assertions.
- `client_test.go` — signed EVM transaction and receipt tests.
- `mirror_app_test.go` — bank and kernel balance agreement tests.
- `parity_test.go` — native and EVM ledger receipt parity tests.
- `pause_test.go` — governance pause enforcement tests.
- `surface_test.go` — method coverage, ABI output and gas assertions.
- `completion_gate_test.go` — related ledger, authorization, payment and agent source file checks.
- `proof_layerxledger_0x101a_test.go` — emitted log consumption by the gateway, indexer and wallet.
- `proof_layerxledger_errors_test.go` — error propagation and persisted recovery tests.
- `issue_1715_test.go` — ledger sequence queries through the web request path.
- `repair_144_test.go` — account sequences, payment expiry and transfer recovery tests.
- `repair_144.kvx` — account sequence, payment expiry and transfer contract notes.
- `repair_m1501_test.go` — ledger interface, receipt, balance and consumer conformance tests.
- `repair_m1501.kvx` — ABI units and ledger consumer contract notes.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no local Makefile. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) defines both targets. `paxd-build` prepares the native libraries and builds the node with its precompiles. `paxeer-test` delegates to [chain.mk](../../chain.mk), which prepares native libraries and runs the Go test packages, including this directory, in groups with race detection and coverage. These targets cover the chain beyond this package.

The log-consumer tests also invoke Cargo, Node.js, npm and pnpm for the gateway, indexer and wallet consumers.

## Configuration

The precompile reads no environment variables or runtime configuration files directly. The node supplies its keeper through `LedgerKeepers`; `abi.json` is embedded at compile time, and `gas.go` defines the method gas table. Availability follows the node's merge activation state.

## Documentation

- [Assets and ledger operations](../../docs/wiki/Assets.md).
- [Payments quickstart](../../docs/wiki/PaymentsQuickstart.md).
- [Unified network](../../docs/wiki/UnifiedNetwork.md).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: modules](../../docs/site/paxeer-docs/src/app/modules/page.tsx).
