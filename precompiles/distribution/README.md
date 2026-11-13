# Distribution precompile

This directory contains the Go implementation, Solidity interface, ABIs, and tests for the distribution precompile in Paxeer. It connects EVM contracts to the Cosmos SDK distribution keeper for staking reward queries, delegation reward withdrawals, withdrawal-address changes, and validator commission withdrawals.

## Layout

- `distribution.go` — precompile construction, method dispatch, reward queries, withdrawals, and event emission.
- `setup.go` — registration of distribution executors by upgrade version.
- `Distribution.sol` — the `IDistr` Solidity interface and precompile address constant.
- `abi.json` — the distribution ABI embedded in the Go implementation.
- `staking_abi.json` — staking ABI embedded by the distribution tests to set up delegations.
- `distribution_test.go` — withdrawal, query, input-validation, and gas-execution tests.
- `abi_freeze_test.go` — checks for the served ABI, method selectors, and event topics.
- `gas_golden_test.go` — gas and output checks across merge activation.
- `repair_183_test.go` — rewards call-value, query, and transaction rollback tests.
- `repair_183.kvx` — rewards call-value and activation semantics.
- `versions` — distribution version list.
- `legacy/` — versioned distribution executors, ABIs, and tests.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. Run these targets from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The root [Makefile](../../Makefile) builds the node and its native dependencies through `paxd-build`. The test target in [chain.mk](../../chain.mk) splits the Go test packages into groups and runs them with race detection and coverage, including this package and its versioned executors.

## Configuration

The precompile reads no environment variables or runtime configuration files. Its distribution and EVM keepers are supplied by the node. `abi.json` is embedded at build time; `staking_abi.json` is embedded by the tests.

## Documentation

- [Distribution ABI and events](../../docs/system/precompiles.md#distribution-abijson)
- [Precompiles documentation site page](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx)
- [Precompiles documentation Markdown](../../docs/site/paxeer-docs/public/markdown/precompiles.md)
