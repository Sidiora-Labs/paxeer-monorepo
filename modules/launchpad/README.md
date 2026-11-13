# Launchpad module

This directory holds Paxeer's native launchpad module: market creation and trading, quote custody, fee rights, fee distribution, holder airdrops, and combined token and virtual-chain launches. Its keeper supplies the [launchpad precompile](../../precompiles/launchpad), message service, query service, and query CLI within the node, including home-chain trading rules and token-holder claims on chain revenue.

## Layout

- [client/](client/) — query CLI commands and tests for the gRPC gateway routes.
- [keeper/](keeper/) — market execution, custody, fees, permissions, chain launches, revenue epochs, genesis handling, queries, invariants, and migration data.
- [launchpadtest/](launchpadtest/) — keeper setup over the shared test application and tests for domain and query behavior.
- [legacy/](legacy/) — versioned `v610` keeper and types.
- [simulation/](simulation/) — module store decoder and its tests.
- [types/](types/) — parameters, market and genesis types, arithmetic, events, errors, messages, and generated query and transaction bindings.
- [module.go](module.go) — module registration, services, genesis import and export, invariants, CLI, and simulation wiring.
- [module_services_test.go](module_services_test.go) — query-service and gRPC gateway registration tests.
- [module_test.go](module_test.go) — module interface and consensus-version tests.

## Build and test

The directory belongs to the root [Go module](../../go.mod). Run the targets in [chain.mk](../../chain.mk) from the repository root:

| Command | Purpose |
| --- | --- |
| `make -f chain.mk build` | Build the node, including the launchpad module, into `build/paxd`. |
| `make -f chain.mk test` | Run the root Go package tests, including the launchpad packages. |

Both targets prepare the node's native libraries through `paxd-native-libs`.

## Configuration

Module parameters are loaded from genesis state and updated through the governance-authorized `MsgUpdateParams` message. [types/params.go](types/params.go) defines `virtual_quote_default`, `virtual_token_default`, `min_fee_bps`, `max_fee_bps`, `base_fee_bps`, `protocol_fee_bps`, `fee_decay_rate`, `volatility_weight`, `concentration_weight`, `creation_fee`, and `quote_token`.

The migration reader embeds [layout.json](keeper/testdata/v6120/layout.json), [pools.json](keeper/testdata/v6120/pools.json), [usdl.json](keeper/testdata/v6120/usdl.json), and [holders.json](keeper/testdata/v6120/holders.json). The module reads no environment variables.

## Documentation

- [Launch a market and chain](../../docs/launchpad/chain-launch.md) — combined launch, home-chain trading, revenue claims, and fee-right controls.
- [Virtual chains](../../docs/wiki/VirtualChains.md) — registry behavior, shared state, allowances, and fees.
- [Launchpad documentation site page](../../docs/site/paxeer-docs/src/app/launchpad/page.tsx) and its [Markdown page](../../docs/site/paxeer-docs/public/markdown/launchpad.md) — issuance, market trading, fee strategies, and protocol settings.
