# Perps module

This directory implements the native perpetual futures venue in the Paxeer network. It holds the Go module wiring, persistent trading state and risk logic for order matching, USDL cross margin, funding, liquidation, insurance and auto-deleveraging. Trading calls and state views reach the keeper through the [perps precompile](../../precompiles/perps), governance messages control venue parameters and markets, and end-block processing advances prices, funding and bounded maintenance work.

## Layout

- [keeper/](keeper/) — state storage, collateral accounting, order books, matching, positions, fees, pricefeed integration, funding, liquidation, insurance, governance handlers and end-block processing, with tests and fixtures.
- [types/](types/) — market and account types, parameters, genesis state, store keys, codecs, governance messages, events, registered errors and their tests.
- [module.go](module.go) — module registration, governance message service, invariants, genesis validation, import and export.
- [abci.go](abci.go) — end-block entry point that delegates to the keeper.
- [module_test.go](module_test.go) — module shape and node-to-precompile integration tests.
- [module_genesis_test.go](module_genesis_test.go) — module genesis validation tests.
- [module_services_test.go](module_services_test.go) — governance message registration and routing tests.
- [gov_route_test.go](gov_route_test.go) — governance proposal execution and atomicity tests.
- [migration_test.go](migration_test.go) — consensus-version migration and state preservation tests.

## Build and test

The packages belong to the repository's [Go module](../../go.mod). Build and test them through the root targets in [Makefile](../../Makefile) and [chain.mk](../../chain.mk), from the repository root:

```sh
make paxd-build
make paxeer-test
```

`make paxd-build` builds the native libraries and the `paxd` node, including this module. `make paxeer-test` runs the root Go module's tests in package groups, including the perps module, keeper and types tests. Both targets use cached Go dependencies with `GOPROXY` set to offline mode.

## Configuration

The module receives its configuration through genesis state and governance messages. [GenesisState](types/genesis.go) supplies `Params`, `Markets` and the venue's stored state; [params.go](types/params.go) defines the venue parameter fields and defaults, and [market.go](types/market.go) defines market configuration.

`MsgUpdateParams`, `MsgSetMarket` and `MsgSetPaused` apply changes authorized by the governance module account. These settings control collateral, fees and rebates, funding, risk limits, maintenance budgets and the venue pause state. The module reads no environment variables or local configuration files directly.

## Documentation

- [Perpetual futures](../../docs/site/paxeer-docs/public/markdown/perpetuals.md) — documentation-site guide to the venue, margin, funding, liquidation and EVM methods.
- [Perps precompile ABI](../../docs/perps-abi-change-history.md) — argument layouts, collateral units and SDK encoding.
- [Perps data API](../../docs/perps/data-api.md) — REST and WebSocket reference for the accompanying market-data service.
