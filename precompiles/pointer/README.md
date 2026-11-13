# Pointer precompile

This directory contains the Go implementation, Solidity interface and embedded ABI for Paxeer's pointer registration precompile at `0x000000000000000000000000000000000000100b`. It exposes native bank denominations and CosmWasm tokens to EVM applications through ERC-20, ERC-721 and ERC-1155 pointer contracts on the same network. The registration methods are `addNativePointer`, `addCW20Pointer`, `addCW721Pointer` and `addCW1155Pointer`; each returns the pointer address, accepts zero value and rejects static calls and delegate calls. Native registration reads bank denomination metadata, while CosmWasm registration queries the underlying contract's metadata before deploying or updating its pointer.

## Layout

- `pointer.go`: registration handlers, metadata validation and precompile construction.
- `Pointer.sol`: Solidity interface and precompile address constants.
- `abi.json`: registration ABI embedded in the Go binary.
- `setup.go`: generated selection of precompile implementations by upgrade version.
- `versions`: version list used for versioned implementations.
- `legacy/`: versioned Go implementations and their embedded ABIs.
- `pointer_test.go`: native registration, metadata, events and pointer upgrade tests.
- `abi_freeze_test.go`: ABI, Solidity mutability, selectors and event declarations tests.
- `gas_golden_test.go`: registration gas tests across merge activation.
- `repair_m1283_test.go`: registration, value refusal, versioned ABI and CosmWasm metadata validation tests.
- `repair_m1283.kvx`: ABI and metadata validation notes.

## Build and test

This package belongs to the root Go module in [go.mod](../../go.mod) and has no separate Makefile. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) defines `paxd-build` to build the native libraries and the `paxd` binary, which includes this precompile. Its `paxeer-test` target invokes the chain test target in [chain.mk](../../chain.mk); that target discovers Go test packages, including this directory, and runs them in groups with race detection and coverage. The pointer tests use the application keepers and CosmWasm binaries under [contracts/wasm](../../contracts/wasm/).

## Configuration

The implementation reads no environment variables or runtime configuration files directly. It receives the EVM, bank and CosmWasm keepers from the node. `abi.json` is embedded at build time; `setup.go` selects the implementation using the supplied upgrade version.

## Documentation

- [Precompile ABI catalogue](../../docs/system/precompiles.md).
- [Precompiles documentation site page](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Contracts and token pointers documentation site page](../../docs/site/paxeer-docs/src/app/contracts/page.tsx).
- [Precompiles guide](../../docs/site/paxeer-docs/public/markdown/precompiles.md).
- [Contracts and token pointers guide](../../docs/site/paxeer-docs/public/markdown/contracts.md).
