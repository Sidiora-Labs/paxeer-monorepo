# Wasmd precompile

This directory contains the Go implementation, Solidity interface, ABI, and tests for the wasmd precompile. It connects EVM callers to CosmWasm contracts within the Paxeer network at `0x0000000000000000000000000000000000001002`, exposing `instantiate`, `execute`, `execute_batch`, and `query`. The implementation uses the node's Wasm, EVM, and bank keepers to handle contract calls, native payments, caller associations, and dynamic gas accounting.

## Layout

- [wasmd.go](wasmd.go): current executor, ABI loading, argument validation, payments, and contract queries.
- [Wasmd.sol](Wasmd.sol): Solidity interface and precompile address constants.
- [abi.json](abi.json): embedded ABI for the contract methods and return types.
- [setup.go](setup.go): generated registration of current and versioned implementations.
- [versions](versions): version names used by the precompile setup generator.
- [legacy/](legacy/): versioned Go implementations and their ABIs.
- [wasmd_test.go](wasmd_test.go): contract lifecycle, query, payment, and batch execution tests.
- [abi_freeze_test.go](abi_freeze_test.go): ABI signatures, selectors, and address checks.
- [gas_golden_test.go](gas_golden_test.go): dynamic gas and result checks across merge activation.
- [repair_m1275_test.go](repair_m1275_test.go): pointer delegatecall, artifact, payment, query, and batch rollback tests, including state recovery after restart.
- [repair_m1275.kvx](repair_m1275.kvx): contract-call behavior and pointer artifact references.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate build target. Run these commands from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) defines `paxd-build`, which builds the native libraries and the node containing this precompile. Its `paxeer-test` target delegates to [chain.mk](../../chain.mk), whose test groups include the Go packages under `precompiles/` and run with race detection and coverage.

The contract tests load the checked-in [echo contract](../../tests/fixtures/example/cosmwasm/echo/artifacts/echo.wasm), [token contracts](../../contracts/wasm/), and [pointer artifact manifest](../common/pointer_artifacts/manifest.json).

## Configuration

The executor reads no environment variables or runtime configuration files directly. The node supplies its keepers through `utils.Keepers` and execution settings through `sdk.Context`. The ABI in [abi.json](abi.json) is embedded at compile time; [versions](versions) supplies version names to [the setup generator](../../scripts/bump_version/README.md).

## Documentation

- [Precompile ABI catalogue](../../docs/system/precompiles.md).
- [Documentation site: CosmWasm lifecycle and EVM calls](../../docs/site/paxeer-docs/src/app/wasm/page.tsx).
- [Documentation site: precompile interfaces](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [CosmWasm guide in Markdown](../../docs/site/paxeer-docs/public/markdown/wasm.md).
