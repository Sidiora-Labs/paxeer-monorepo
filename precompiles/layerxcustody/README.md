# LayerX custody precompile

This directory holds the Go implementation, Solidity interface, embedded ABI and tests for the `layerxCustody` EVM precompile in Paxeer. The precompile remains registered on the merged network, but all its methods, including views, revert after `v7.0` activation with `LXP_MODULE_DISABLED: module disabled; layerxCustody retired`. Account funding uses native bank transfers; outward transfers use the governed bridge.

## Layout

- `LayerXCustody.sol` — Solidity interface, ABI tuples, events and precompile address constant.
- `abi.json` — ABI embedded by the Go implementation.
- `layerxcustody.go` — precompile construction, retirement check, gas accounting, method dispatch and event encoding.
- `layerxcustody_test.go` — application-keeper tests for deposits, withdrawal and forced-exit refusals, deposit-root calls and gas accounting.
- `retire_test.go` — tests that every ABI method refuses at merge activation, including static views and calls with value or delegatecall restrictions.
- `retire_registry_test.go` — EVM transaction test for continued registration, retirement revert data and failed transaction receipts.
- `setup.go` — generated versioned precompile registration.
- `versions` — version list used by precompile generation.

## Build and test

The package belongs to the root Go module and has no separate build manifest or Makefile. Run the covering targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) defines these targets. `paxd-build` builds the native libraries and the node through [chain.mk](../../chain.mk), including the registered custody precompile. `paxeer-test` runs the chain Go test groups, whose package inventory includes this directory's tests.

## Configuration

The package reads no environment variables or runtime configuration files directly. `abi.json` is embedded at compile time. The node supplies the custody, bank and EVM keepers; custody parameters and retirement state come from chain state through the custody keeper.

## Documentation

- [Precompile ABI reference and retirement behavior](../../docs/system/precompiles.md).
- [Chain modules and custody state](../../docs/system/chain-modules.md).
- [Documentation site: account funding and key custody](../../docs/site/paxeer-docs/public/markdown/custody.md).
