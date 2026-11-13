# PECOR spot module

This directory holds the Go implementation of the native PECOR spot venue in
`paxd`. It provides swaps, limit and stop orders, quotes, liquidity vaults and
vault shares through the spot precompiles in
[`precompiles/spot`](../../precompiles/spot). The keeper uses the bank, EVM and
pricefeed keepers, holds custody in the spot module account and executes resting
orders at the end of each block on the shared Paxeer network.

## Layout

- [`keeper/`](keeper) — custody, pricing, fees, swaps, orders, vaults, shares,
  genesis import and export, migrations, invariants and their tests.
- [`types/`](types) — parameters, genesis and order records, storage keys, events,
  errors, governance messages, generated message bindings and type tests.
- [`module.go`](module.go) — application module registration, genesis handling,
  message services, invariants and migration registration.
- [`abci.go`](abci.go) — end-of-block resting-order execution and result events.
- [`module_test.go`](module_test.go) — module registration, genesis round-trip
  and end-of-block integration tests.

## Build and test

The module belongs to the repository's root [`go.mod`](../../go.mod) and builds
with the node. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [`Makefile`](../../Makefile) defines both targets. `paxd-build` prepares
the native libraries and builds `paxd` through [`chain.mk`](../../chain.mk).
`paxeer-test` runs the chain's Go test groups, which include this module and its
keeper and types packages.

## Configuration

Runtime configuration comes from the spot `GenesisState.Params` and the
parameters stored by the keeper. [`types/params.go`](types/params.go) defines the
settings for swap and order fees, price impact, fee collection, pause controls,
keeper permissions, TWAP checks, order limits, per-block execution bounds and
registered tokens. [`types/genesis.go`](types/genesis.go) defines the genesis
state imported by the application module. The governance module account changes
parameters through `MsgUpdateParams`; `MsgSyncReserves` reconciles token reserves
under the same authority. The module reads no environment variables or separate
configuration files.

## Documentation

- [Chain module parameters and messages](../../docs/system/chain-modules.md#spot)
- [Spot precompiles and ABIs](../../docs/system/precompiles.md)
- [Spot Exchange on the documentation site](../../docs/site/paxeer-docs/public/markdown/spot-exchange.md)
