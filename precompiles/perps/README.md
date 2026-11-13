# Perps precompile

This directory contains the Go EVM precompile for the perpetuals venue, its Solidity interface, embedded ABI, and tests. It exposes the native [`modules/perps`](../../modules/perps) keeper to EVM callers at `0x1015` for collateral, orders, positions, liquidation, and state queries. Collateral uses USDL with 6 decimals; prices, USD notionals, and leverage arguments use 18-decimal fixed point. Every method is nonpayable. The venue runs within the Paxeer BFT network, where a receipt is final when its block is committed.

## Layout

- `abi/` — embedded `abi.json`, ABI parsing, event packing, and ABI tests.
- `testdata/` — ABI golden text, USDL runtime bytecode, version vectors, and view golden JSON fixtures.
- `Perps.sol` — Solidity interface, structs, events, methods, and parameterless custom errors for the `0x1015` venue.
- `abi_test.go` — ABI declaration and golden-file tests.
- `account.go` — vault, collateral, account summary, margin mode, and USDL token-leg handlers.
- `account_test.go` — account and collateral handler tests.
- `address_test.go` — precompile address test.
- `errors.go` — keeper-error to EVM revert-selector mapping.
- `errors_test.go` — revert and nested token-call error tests.
- `events_test.go` — event and log tests.
- `governed_view_fee_tier_test.go` — governed fee-tier view test.
- `governed_view_funding_test.go` — governed funding-details view test.
- `governed_view_market_active_test.go` — disabled-market view test.
- `governed_view_market_config_test.go` — governed market-configuration view test.
- `governed_view_open_interest_test.go` — governed open-interest view test.
- `helpers_test.go` — fixed-point and order-test helpers.
- `integration_test.go` — EVM integration and settlement parity tests.
- `logs.go` — keeper event to EVM log conversion.
- `lxp_registry_test.go` — perps error registry test.
- `orders.go` — order placement, cancellation, trigger, and compatibility handlers.
- `orders_test.go` — order handler tests.
- `perps.go` — precompile construction, gas table, keeper wiring, dispatch, and call guards.
- `perps_test.go` — dispatch, call-kind, and wiring tests.
- `positions.go` — position opening, sizing, collateral, close, and liquidation handlers.
- `positions_test.go` — position handler and settlement tests.
- `quote_market_test.go` — market quote handler test.
- `quotes.go` — market, open-position, close-position, and partial-close quote handlers.
- `quotes_test.go` — quote handler tests.
- `switch_test.go` — ABI and activation compatibility tests.
- `views.go` — account, order, position, market, funding, liquidation, insurance, and ADL read handlers.
- `views_golden_account_test.go` — account and position golden view test.
- `views_golden_funding_test.go` — funding and fee golden view test.
- `views_golden_insurance_test.go` — insurance and ADL golden view test.
- `views_golden_market_test.go` — order and market golden view test.
- `views_golden_quotes_test.go` — quote golden view test.
- `views_test.go` — view handler tests.

## Build and test

This package belongs to the root [`go.mod`](../../go.mod) and has no local
Makefile or build manifest. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [`Makefile`](../../Makefile) target `paxd-build` builds the native
libraries and the node, including this precompile. Its `paxeer-test` target
runs the Go package test groups defined in [`chain.mk`](../../chain.mk),
covering this package and `abi/`. These targets use cached Go dependencies.
The collateral settlement parity test in `positions_test.go` also runs Node
with `tsx` from [`apps/perps`](../../apps/perps).

## Configuration

The precompile reads venue parameters and market configuration from the
injected perps keeper, including `CollateralToken` and `Paused`; their types
are declared in [`modules/perps/types`](../../modules/perps/types). It reads
no environment variables or runtime configuration files. The ABI is embedded
from [`abi/abi.json`](abi/abi.json) at build time.

The settlement parity test supplies `PERPS_SETTLEMENT_RPC` to its Node test
process for communication with the local test server.

## Documentation

- [System precompile catalogue](../../docs/system/precompiles.md)
- [Perps ABI and client encoding](../../docs/perps-abi-change-history.md)
- [Perps data API](../../docs/perps/data-api.md)
- [Perpetuals guide](../../docs/site/paxeer-docs/src/app/perpetuals/page.tsx)
- [Precompiles guide](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx)
