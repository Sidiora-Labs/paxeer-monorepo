# Pointerview precompile

This directory holds the Go implementation, Solidity interface, ABI and tests for the pointer lookup precompile in Paxeer's `paxd` node. EVM callers use `getNativePointer`, `getCW20Pointer`, `getCW721Pointer` and `getCW1155Pointer` at `0x000000000000000000000000000000000000100A` to look up a native denomination or CosmWasm token contract in the EVM keeper. Each view returns the pointer address, its `uint16` version and an `exists` flag.

## Layout

- [pointerview.go](pointerview.go): precompile construction, ABI loading and lookup handlers.
- [Pointerview.sol](Pointerview.sol): `IPointerview` interface and precompile constants for Solidity callers.
- [abi.json](abi.json): embedded ABI for the four view methods.
- [setup.go](setup.go): generated constructors selected by upgrade version.
- [versions](versions): version list used by [scripts/bump_version](../../scripts/bump_version).
- [legacy/](legacy/): version-specific Go implementations and their embedded ABIs.
- [pointerview_test.go](pointerview_test.go): pointer lookup results for registered and absent tokens.
- [abi_freeze_test.go](abi_freeze_test.go): precompile address, ABI, method selectors and event checks.
- [gas_golden_test.go](gas_golden_test.go): gas and lookup checks across merge activation.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. Run these commands from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The [root Makefile](../../Makefile) target `paxd-build` builds the native libraries and the node that includes this precompile. The [chain.mk](../../chain.mk) target `test` splits the Go test packages into groups and runs them with race detection and coverage, including this directory's lookup, ABI and gas tests.

## Configuration

The package reads no environment variables or runtime configuration files. It embeds [abi.json](abi.json) at build time and receives the EVM keeper through `utils.Keepers`. `GetVersioned` receives `latestUpgrade` from the caller to select the current implementation alongside the version-specific constructors.

## Documentation

- [Assets and token pointers](../../docs/wiki/Assets.md).
- [Precompile ABI reference](../../docs/system/precompiles.md#pointerview-abijson).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/public/markdown/precompiles.md).
- [Documentation site: assets](../../docs/site/paxeer-docs/public/markdown/assets.md).
