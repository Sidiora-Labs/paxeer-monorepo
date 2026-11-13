# Spot precompiles

This directory holds the Go EVM precompiles for the native spot module in
[`modules/spot`](../../modules/spot/). PECOR swaps, limit orders, stop orders,
quotes and liquidity vault operations execute through the spot keeper on the
Paxeer chain. The precompiles meter gas, enforce call restrictions, encode
reverts and emit EVM logs within the transaction. Receipts are final when their
blocks are committed.

## Layout

- [`abi/`](abi/) — embedded JSON ABIs and a Go package that exports parsed ABIs for clients and indexers, including vault share pointers.
- [`testdata/`](testdata/) — golden ABI snapshots for selectors, events and errors.
- [`spot.go`](spot.go) — shared gas table, atomic execution wrapper, ABI discovery and construction of the served precompile set.
- [`setup.go`](setup.go) — dynamic gas wrappers and versioned registration under `v6.12.0` and the supplied latest upgrade name.
- [`pecor.go`](pecor.go) — market buys and sells, swaps, fee collection, quote views and multicall dispatch.
- [`orders.go`](orders.go) — limit order placement, modification, cancellation, execution and queries.
- [`stoporders.go`](stoporders.go) — stop-limit and stop-loss order operations, activation and batch execution.
- [`quoter.go`](quoter.go) — swap quotes, token prices, TWAP and liquidity queries.
- [`vault.go`](vault.go) — liquidity deposits and withdrawals, reserve views, token registry views and share pointer lookup.
- [`errors.go`](errors.go) — ABI revert encoding and fee collector authorization.
- [`events.go`](events.go) — ABI event validation, EVM log emission and spot end-block receipt logs.
- [`spot_test.go`](spot_test.go) — shared execution, gas accounting and rollback tests.
- [`setup_test.go`](setup_test.go) — served set construction and keeper requirements.
- [`spot_abis_test.go`](spot_abis_test.go) — ABI discovery without application keepers.
- [`errors_test.go`](errors_test.go) — revert payloads and parameter refusals.
- [`events_test.go`](events_test.go) — event encoding and receipt log behavior.
- [`integration_test.go`](integration_test.go) — spot operations through the application and EVM with token and balance checks.
- [`listing_test.go`](listing_test.go) — governance token listing and subsequent PECOR trading.
- [`production_test.go`](production_test.go) — dynamic gas dispatch through the node's precompile registration.
- [`production_rails_test.go`](production_rails_test.go) — registered precompile receive handling and gas charges.

The other Go tests cover individual executors, ABI snapshots and vault share
interfaces.

## Build and test

The packages belong to the root [`go.mod`](../../go.mod) and have no separate
build manifest or Makefile. Run these targets from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The [`Makefile`](../../Makefile) target `paxd-build` builds the native libraries
and the node binary containing the spot precompiles. The
[`chain.mk`](../../chain.mk) target `test` prepares native libraries and runs
the root Go test packages in groups with race detection and coverage, including
this package and `precompiles/spot/abi`. The root `make test` target runs the
kernel test suite; use the explicit `chain.mk` target for these Go packages.

## Configuration

The precompiles read spot parameters through the injected keeper, including
token registrations, fees, pause flags and keeper permissions. Administrative
changes use the spot module's governance messages. This package reads no
environment variables or runtime configuration files directly. The JSON files
in [`abi/`](abi/) are embedded contract interfaces.

## Documentation

- [Spot module and native EVM routes](../../docs/wiki/Spot.md)
- [Precompile ABI reference](../../docs/system/precompiles.md)
- [Unified network architecture](../../docs/wiki/UnifiedNetwork.md)
- [Documentation site: Spot Exchange](../../docs/site/paxeer-docs/src/app/spot-exchange/page.tsx)
- [Spot Exchange Markdown](../../docs/site/paxeer-docs/public/markdown/spot-exchange.md)
- [Documentation site: Precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx)
