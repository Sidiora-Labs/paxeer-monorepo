# JSON precompile

This directory contains the JSON EVM precompile, its ABI and Solidity interface, tests, and versioned legacy implementations. The precompile is registered at `0x0000000000000000000000000000000000001003` on the Paxeer mainnet-beta EVM network (chain ID `125`) and provides read-only extraction of object fields, object field lists, unsigned integers, and array elements from JSON bytes.

## Layout

- `legacy/` — versioned implementations and ABI files for `v5.5.2`, `v5.5.5`, `v5.6.2`, `v6.0.3`, `v6.0.5`, `v6.0.6`, `v6.1.0`, `v6.1.4`, `v6.2.0`, `v6.3.0`, `v6.4.0`, and `v6.5`.
- `Json.sol` — Solidity constants and the `IJson` interface for the four read-only methods.
- `abi.json` — ABI for `extractAsBytes`, `extractAsBytesList`, `extractAsUint256`, and `extractAsBytesFromArray`.
- `abi_freeze_test.go` — checks the JSON ABI, address, selectors, and embedded ABI.
- `gas_golden_test.go` — checks gas values and results across the merge activation height.
- `json.go` — current precompile implementation, address, method selectors, JSON decoding, and gas calculation.
- `json_test.go` — behavior tests for field, list, integer, array, and nested extraction.
- `setup.go` — generated registration of the current and legacy versioned precompiles.
- `versions` — list of supported legacy version labels.

## Build and test

This directory uses the root [Go module](../../go.mod) and has no local build target. Run these targets from the repository root; both are defined in the root [Makefile](../../Makefile) and delegate the Go build or tests to [chain.mk](../../chain.mk):

```sh
make paxd-build
make paxeer-test
```

`paxd-build` builds the native libraries and `paxd`, which imports the JSON precompile through the precompile registry. `paxeer-test` runs the split Go test groups, including the JSON extraction, ABI, and gas tests.

## Configuration

The sources do not read environment variables or configuration files. The ABI is embedded from `abi.json` at build time.

## Documentation

- [System precompile reference](../../docs/system/precompiles.md)
- [Precompiles documentation site page](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx)
