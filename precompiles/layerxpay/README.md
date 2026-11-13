# LayerX Pay precompile

This directory holds the Go implementation, Solidity interface, ABI and tests for the `layerxpay` EVM precompile. It connects contracts to escrow, budget, stream and service payments through the shared payment keeper in `paxd`, on the single Paxeer network. Calls validate authority and calldata, execute payment operations, return state views and emit EVM logs.

## Layout

- `LayerXPay.sol` — Solidity interface with payment methods, record types and events.
- `abi.json` — ABI embedded by the executor and gas calculation code.
- `layerxpay.go` — constructors, merge registration, call validation, dispatch and ABI encoding.
- `keepers.go` — interface supplying the application's shared payment keeper.
- `budget.go` — budget creation, funding, delegation, spending, closure and views.
- `escrow.go` — escrow lifecycle, disputes, remaining balances and result views.
- `stream.go` — stream lifecycle, ask bindings, metering, settlement and views.
- `service.go` — offers, agreements, commitments, delivery, disputes and default outcome events.
- `service_views.go` — service record views and delivery tuple conversion.
- `events.go` — payment event packing and EVM log emission.
- `errors.go` — conversion of payment, kernel and governance errors to revert data.
- `gas.go` — method gas table and calldata gas calculation.
- `abi_test.go` — method, event and Solidity interface checks.
- `layerxpay_test.go` — merge constructor registration and gas charging checks.
- `budget_test.go` — budget operations and view checks.
- `escrow_test.go` — escrow lifecycle and caller checks.
- `stream_test.go` — stream lifecycle, views and event checks.
- `stream_ask_test.go` — ask stream opening, metering, persistence and gas checks.
- `service_test.go` — service lifecycle and dispute checks.
- `service_views_test.go` — service view encoding and pause behavior checks.
- `events_test.go` — event encoding and emission checks.
- `errors_test.go` — revert encoding checks.
- `gas_test.go` — gas table checks.
- `charge_test.go` — gas charging against store work.
- `gas_bound_test.go` — charged gas bounds against host store work.
- `conformance_budget_test.go` — budget execution compared with host execution.
- `conformance_escrow_test.go` — escrow execution compared with host execution.
- `conformance_service_test.go` — service execution compared with host execution.
- `harness_test.go` — shared keeper and EVM test harness.
- `export_test.go` — helpers exposed to external package tests.
- `proof_layerxpay_0x101b_test.go` — payment calls through the EVM, SDK encoding, indexer consumption and reopened state.
- `served/` — application tests for precompile availability and escrow execution through the EVM.

## Build and test

This package belongs to the root [Go module](../../go.mod) and uses the chain build and test targets in the root [Makefile](../../Makefile) and [chain.mk](../../chain.mk). Run these commands from the repository root:

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds the node and its native dependencies. `paxeer-test` runs the chain's Go test suite, including this package and `served/`; its prerequisites build the native libraries. The directory has no separate build manifest or Makefile.

## Configuration

The package reads no environment variables or runtime configuration files. `abi.json` is embedded at compile time. The application supplies the shared payment keeper through `LayerXPayKeepers` and the EVM keeper through the merge constructor; payment parameters and authority checks come from the keeper.

## Documentation

- [Precompile ABI reference](../../docs/system/precompiles.md) and [documentation site precompiles page](../../docs/site/paxeer-docs/public/markdown/precompiles.md).
- [Payments developer path](../../docs/wiki/PaymentsQuickstart.md) and [documentation site payments page](../../docs/site/paxeer-docs/public/markdown/payments.md).
- [Escrow module](../../docs/wiki/Escrow.md) and [documentation site escrow page](../../docs/site/paxeer-docs/public/markdown/escrow.md).
- [Budget module](../../docs/wiki/Budget.md) and [documentation site agent budgets page](../../docs/site/paxeer-docs/public/markdown/agent-budgets.md).
- [Stream module](../../docs/wiki/Stream.md) and [documentation site streams page](../../docs/site/paxeer-docs/public/markdown/streams.md).
- [Service module](../../docs/wiki/Service.md) and [documentation site services page](../../docs/site/paxeer-docs/public/markdown/services.md).
