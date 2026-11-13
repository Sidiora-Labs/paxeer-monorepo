# LayerX ledger module

This directory holds the native `layerxledger` module for Paxeer. Its keeper
handles registered assets, derived accounts, minting, burning, transfers and
exact payments through the shared kernel host in [`../lxphost`](../lxphost).
It mirrors ledger transfers into bank balances and binds asset issuers and
pause state to tokenfactory denominations. The EVM entry point is
[`layerxledger`](../../precompiles/layerxledger) at `0x101A`, served after merge
activation. The kernel runs inside `paxd`; receipts are final when their blocks
are committed.

## Layout

- [`keeper/`](keeper): Ledger operations, caller authority, kernel execution,
  bank mirroring, issuer bindings, events, views, genesis and keeper tests.
- [`types/`](types): Module keys, parameters, genesis and message types,
  generated protobuf bindings, payload codecs, errors, keeper interfaces and
  their tests.
- [`module.go`](module.go): Module registration, codecs, the parameter message
  service and genesis import and export.
- [`module_test.go`](module_test.go): Tests for module services, parameter
  authority, codecs and genesis round trips.

## Build and test

The module belongs to the root [`go.mod`](../../go.mod) and has no separate
Makefile or manifest. Run these commands from the repository root:

```sh
make paxeer-build
make paxeer-test
```

The root [`Makefile`](../../Makefile) forwards these targets to
[`chain.mk`](../../chain.mk). The build produces `build/paxd` and prepares
the native libraries used by the kernel host. The test target covers the
root Go module, including this directory's module, keeper and types tests.

## Configuration

The module reads no environment variables or configuration files directly.
Its genesis section is `layerxledger`, with the parameter `params.paused`.
The parameter is stored in the module's KV store and updated through
`MsgUpdateParams`, which checks the authority supplied when the keeper is
constructed. Ledger records live in the shared host store; genesis import
rebuilds issuer bindings from tokenfactory and host state.

## Documentation

- [Assets and tokens](../../docs/wiki/Assets.md): Asset registration,
  issuance, account derivation and transfers.
- [Payments developer path](../../docs/wiki/PaymentsQuickstart.md): Identity
  binding, account funding and exact payments.
- [Chain modules and parameters](../../docs/system/chain-modules.md#layerxledger):
  Ledger parameter and message declarations.
- [Precompile reference](../../docs/system/precompiles.md): Ledger ABI,
  events and activation behavior.
- [Documentation site module page](../../docs/site/paxeer-docs/public/markdown/modules.md):
  The ledger's role among the network's native modules.
