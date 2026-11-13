# LayerX bridge precompile

This directory holds the Go implementation, Solidity interface and ABI for the
`layerxBridge` EVM precompile in the Paxeer network. It connects EVM callers to
the native `layerxbridge` module: `bridgeIn` mints bridged denominations against
attestor signatures, `bridgeOut` burns the caller's bridged balance for an
external withdrawal, and view methods expose chain registrations, attestors,
caps, pause state and consumed source events. State changes emit EVM logs and
Cosmos events.

## Layout

- [layerxbridge.go](layerxbridge.go) — keeper interface, ABI dispatch, gas accounting, bridge events and revert codes.
- [setup.go](setup.go) — generated registration of the precompile by upgrade version.
- [versions](versions) — version input for precompile setup generation.
- [LayerXBridge.sol](LayerXBridge.sol) — Solidity interface, precompile address, events and attestation requirements.
- [abi.json](abi.json) — embedded ABI for bridge calls, views and events.
- [layerxbridge_test.go](layerxbridge_test.go) — keeper-backed EVM tests for minting, burning, refusals, caps, nonces and gas.
- [abi_test.go](abi_test.go) — Solidity interface and ABI agreement tests.
- [method_parity_test.go](method_parity_test.go) — method constant, gas and keeper dispatch coverage for the ABI.
- [revert_registry_test.go](revert_registry_test.go) — bridge revert code agreement with the shared error registry.
- [README.md](README.md) — component overview and build references.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no local
Makefile. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) defines `paxd-build`, which builds native
libraries and the `paxd` node containing the precompile. Its `paxeer-test` target
delegates to [chain.mk](../../chain.mk), whose Go test package discovery includes
this directory. That target runs the wider chain test suite with native library
prerequisites, race detection and coverage.

## Configuration

The precompile reads bridge state through the module keeper: registered chains,
vaults, finality depths, enabled flags, attestors and their threshold, asset
denominations, caps, pause state and nullifiers. Governance configures these in
the [bridge module](../../modules/layerxbridge). The application supplies the
bridge keeper and the upgrade keeper used to select registered revert codes.

The runtime embeds [abi.json](abi.json) at build time. This package reads no
environment variables or runtime configuration files.

## Documentation

- [Bridge module guide](../../docs/wiki/Bridge.md)
- [Precompile ABI reference](../../docs/system/precompiles.md)
- [Bridging and balances on the documentation site](../../docs/site/paxeer-docs/public/markdown/bridge.md)
- [Bridge attestation specification](../../modules/layerxbridge/ATTESTATION.md)
