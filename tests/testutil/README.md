# Test utilities

This directory contains shared Go helpers for testing the Paxeer node and its
chain modules. The helpers create application and keeper fixtures, prepare
accounts and transactions, execute and commit blocks, and check resulting chain
state. They support tests of the integrated network, including EVM precompiles
and CosmWasm contracts.

## Layout

- [keeper/](keeper/): application, epoch keeper and EVM keeper fixtures, address helpers, receipt helpers and fixture isolation tests.
- [nullify/](nullify/): reflection helpers that normalize nil slices for assertions while retaining nonempty slice contents, with normalization tests.
- [processblock/](processblock/): application setup, genesis overrides, account funding, validators, staking, minting, contract instantiation, transaction signing and block execution; [msgs/](processblock/msgs/) constructs bank and EVM transactions, and [verify/](processblock/verify/) checks balances, fees, rewards, epochs and mint releases.
- [sample/](sample/): generation of sample account addresses.

## Build and test

These packages belong to the root [Go module](../../go.mod) and have no
standalone build target. Run the chain targets from the repository root, as
defined in the root [Makefile](../../Makefile) and [chain.mk](../../chain.mk):

```sh
make paxeer-build
make paxeer-test
```

The build target produces `build/paxd` and prepares the native libraries used by
the node. The test target includes the utility packages with tests in the chain
test groups, using the race detector and coverage. Both targets use `GOPROXY`
to require locally available Go dependencies.

## Configuration

The helpers define no configuration files. `NewTestAppWithGenesis` accepts
module genesis sections from its caller, and `NewContract` reads the Wasm file
at the caller-supplied `source` path.

`PROCESSBLOCK_TEST_UNKNOWN_GENESIS_MODULE` is read by the genesis constructor
test to select its subprocess branch; the test sets it when starting that
subprocess.

## Documentation

- [Paxeer network](../../docs/paxeer-network.md): node components and chain build and test targets.
- [Chain documentation](../../docs/paxeer-chain-docs.md): network and module documentation index.
- [Precompiles guide](../../docs/site/paxeer-docs/public/markdown/precompiles.md): documentation site reference for the native EVM interfaces used by the helpers.
