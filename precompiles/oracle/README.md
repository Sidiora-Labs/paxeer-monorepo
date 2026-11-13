# Oracle precompile

This directory holds the Go executor, Solidity interface, ABI and versioned implementations of the Oracle EVM precompile in the Paxeer network. The current executor retains `getExchangeRates` and `getOracleTwaps(uint64)` for ABI compatibility; valid calls revert with `ErrOraclePrecompileRetired` and the reason `oracle precompile is retired; oracle data queries are disabled`.

## Layout

- [Oracle.sol](Oracle.sol): `IOracle`, the precompile address constant and the exchange-rate and TWAP result structs.
- [abi.json](abi.json): the query ABI embedded in the Go executor.
- [oracle.go](oracle.go): construction, gas calculation, input validation and retirement reverts.
- [oracle_test.go](oracle_test.go): tests that call both methods through the EVM precompile wrapper and decode their revert reasons.
- [setup.go](setup.go): generated `GetVersioned` registration of the current and versioned executors.
- [versions](versions): version names used by the setup generator.
- [legacy/](legacy/): versioned Go executors and their embedded ABIs.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. Run these covering targets from the repository root:

```sh
make paxeer-build
make paxeer-test
```

The root [Makefile](../../Makefile) forwards these targets to [chain.mk](../../chain.mk). The build compiles the node with its precompiles and native libraries; the test target runs the Go package tests, including `oracle_test.go`.

## Configuration

The executor reads no environment variables or runtime configuration files. It embeds `abi.json` at build time and receives its EVM keeper through `utils.Keepers`. `GetVersioned` receives the upgrade name from its caller.

## Documentation

- [Precompile ABI reference and availability](../../docs/system/precompiles.md).
- [Documentation site: Oracle module](../../docs/site/paxeer-docs/src/app/modules/oracle/page.tsx).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Oracle module Markdown reference](../../docs/site/paxeer-docs/public/markdown/modules/oracle.md).
