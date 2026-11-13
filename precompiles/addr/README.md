# Address precompile

This directory holds the Go implementation, Solidity interface, embedded ABI and tests for the address precompile at `0x0000000000000000000000000000000000001004`. It lets EVM callers associate Paxeer and EVM addresses, bind an EVM address to a LayerX Ed25519 identity, and query the resulting unified account within the Paxeer network. `bindLayerX` requires the caller's transaction and the DID key's signature over the chain identifier, EVM address and current binding nonce; `unbindLayerX` removes the caller's binding and consumes a nonce.

## Layout

- `addr.go` — address association, identity binding, queries, gas accounting and revert reasons.
- `Addr.sol` — the `IAddr` Solidity interface and precompile address constant.
- `abi.json` — method and event definitions embedded in the Go implementation.
- `setup.go` — generated selection of implementations by upgrade version.
- `versions` — version names used for the precompile implementations.
- `legacy/` — versioned implementations and their ABIs.
- `addr_test.go` — association, address lookup and revert tests.
- `layerx_binding_test.go` — binding, signature, nonce, unbinding and signer-vector tests through the precompile.
- `get_evm_addr_by_layerx_keep_test.go` — reverse lookup of a bound EVM address.
- `abi_freeze_test.go` — method selector, event topic and embedded ABI assertions.
- `gas_golden_test.go` — gas charges, outputs and state changes through constructed and application-served precompiles.
- `resolution_reason_test.go` — binding revert encoding at the merge activation height.
- `repair_m1266_test.go` — execution of the gas assertions in `TestAddrGasGolden`.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate build target. Run these commands from the repository root, using the targets in [chain.mk](../../chain.mk):

```sh
make -f chain.mk build
make -f chain.mk test
```

The build target prepares the native libraries and builds `daemon/paxd` into `build/paxd`, including the registered address precompile. The test target runs the repository's Go tests in package groups, including this directory and its versioned implementations, with race detection and coverage.

## Configuration

The implementation reads no environment variables or runtime configuration files directly. `abi.json` is embedded at compile time. EVM, bank and account keepers are supplied by the node; `GetVersioned` in `setup.go` selects implementations by upgrade version. Binding signatures use the chain identifier and nonce supplied by the EVM keeper.

## Documentation

- [Precompile ABI reference](../../docs/system/precompiles.md#addr-abijson).
- [Unified network and account binding](../../docs/wiki/UnifiedNetwork.md#account-binding).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: accounts and unified identity](../../docs/site/paxeer-docs/src/app/accounts/page.tsx).
- [Documentation site: identity](../../docs/site/paxeer-docs/src/app/identity/page.tsx).
