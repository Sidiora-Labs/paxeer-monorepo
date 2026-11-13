# Staking precompile

This directory contains the Go implementation, embedded ABI, Solidity interface, and tests for the EVM staking precompile at `0x0000000000000000000000000000000000001005`. It connects EVM contracts to the staking module on the single Paxeer BFT network, supporting delegation, redelegation, undelegation, validator creation and editing, and queries for validators, delegations, unbonding, redelegations, historical information, the pool, and staking parameters. Versioned executors support execution at the corresponding upgrade heights.

## Layout

- `legacy/` — versioned staking executors, embedded ABIs, and upgrade-boundary tests.
- `Staking.sol` — Solidity interface, address constants, events, transactions, queries, and response structs.
- `abi.json` — embedded ABI served by the current precompile.
- `abi_freeze_test.go` — checks the served ABI, method selectors, event topics, and address.
- `gas_golden_test.go` — gas usage and ABI result checks across merge activation.
- `repair_126.kvx` — redelegation-share projection and activation contract.
- `repair_126_test.go` — redelegation-share precision and upgrade replay tests.
- `repair_m1201.kvx` — validator-key validation and staking query contracts.
- `repair_m1201_test.go` — validator-key, error, share precision, static query, and upgrade-boundary tests.
- `setup.go` — generated version registry that selects the current or historical executor.
- `staking.go` — current staking precompile executor and its keeper-backed transaction and query handlers.
- `staking_events_test.go` — EVM event emission tests.
- `staking_test.go` — current executor transaction, query, error, and response tests.
- `versions` — list of staking upgrade versions used by the generated registry.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no separate Makefile. Run these targets from the repository root:

```sh
make paxeer-build
make paxeer-test
```

The root [Makefile](../../Makefile) forwards these targets to `build` and `test` in [chain.mk](../../chain.mk). The build compiles `build/paxd`, including the staking precompile, after building its native libraries. The test target discovers Go packages with tests and runs them in groups, including this package and its versioned subpackages. Both targets use the locally available Go dependencies.

## Configuration

The package reads no environment variables or configuration files directly. Runtime dependencies are supplied through the staking, bank, distribution, EVM, and query keepers; ABI data is embedded from `abi.json`.

## Documentation

- [Precompile catalogue](../../docs/system/precompiles.md) — network availability, lifecycle, ABI, and staking precompile reference.
- [Precompiles documentation site page](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx) — user-facing precompile addresses and staking behavior.
- [Precompiles documentation site markdown](../../docs/site/paxeer-docs/public/markdown/precompiles.md) — published markdown copy of the precompile page.
