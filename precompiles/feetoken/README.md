# Fee-token precompile

This directory contains the Go implementation, Solidity interface, ABI and tests for the `feeToken` precompile in Paxeer's EVM. It delegates account fee-denomination preferences to the EVM keeper: `setFeeDenom` selects an allowed denomination for the caller, `getFeeDenom` reads an account's preference, and `clearFeeDenom` removes the caller's preference so the network base denomination applies. Calls reject transferred value and delegatecall; static calls can read preferences but cannot change them.

## Layout

- [feetoken.go](feetoken.go) — precompile construction, keeper interface, method execution, gas calculation and revert reasons.
- [FeeToken.sol](FeeToken.sol) — Solidity `IFeeToken` interface and precompile constants.
- [abi.json](abi.json) — embedded ABI for the preference methods.
- [setup.go](setup.go) — generated versioned precompile registration.
- [versions](versions) — precompile version list.
- [feetoken_test.go](feetoken_test.go) — preference round trips, caller isolation, refusals, governance changes, gas, rollback and keeper requirements.
- [abi_freeze_test.go](abi_freeze_test.go) — ABI signatures, selectors and event inventory checks.
- [gas_golden_test.go](gas_golden_test.go) — required and charged gas checks across the merge activation height.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no local Makefile. Run these commands from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The [root build target](../../Makefile) builds the native libraries and `paxd`, including the precompile. The [chain test target](../../chain.mk) discovers Go packages with tests, including this directory, and runs them in test groups with race detection and coverage.

## Configuration

The precompile reads no environment variables or runtime configuration files. Its ABI is embedded from [abi.json](abi.json) at build time. The [EVM keeper](../../modules/evm/keeper/feetoken.go) supplies account preferences and enforces the `FeeTokenEnabled` and `AllowedFeeDenoms` module parameters when selecting a denomination. Clearing a preference remains available when selection is disabled or the denomination is removed from the allowed set.

## Documentation

- [Precompile ABI reference](../../docs/system/precompiles.md) — method signatures and destination.
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx) — native EVM interfaces and calling conventions.
- [Documentation site: Sidiora](../../docs/site/paxeer-docs/src/app/sidiora/page.tsx) — native fee preferences, charging and refunds.
