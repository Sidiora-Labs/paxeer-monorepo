# Pricefeed

This directory contains the native pricefeed module for the Paxeer network.
Bonded validators submit signed index and mark price observations through their
registered feed accounts. The node aggregates fresh observations during
mid-block processing and exposes prices and freshness to market keepers, native
queries, and the pricefeed EVM precompile in
[`precompiles/pricefeed`](../../precompiles/pricefeed).

## Layout

- [`client/`](client/) — CLI queries for parameters, prices, feed keys, feed accounts, and observations.
- [`keeper/`](keeper/) — feed-key registration, observation admission, quorum and median aggregation, mark-price calculation, state, genesis, and message/query servers.
- [`testutil/`](testutil/) — application-backed keeper and store helpers for tests.
- [`types/`](types/) — markets, parameters, messages, state keys, events, errors, codecs, generated protobuf types, and their tests.
- [`module.go`](module.go) — module registration, genesis import/export, query routes, and the mid-block hook.
- [`abci.go`](abci.go) — invokes aggregation from `MidBlocker`.
- [`ante.go`](ante.go) — submission fee exemption, admission checks, and submission rate tracking.
- [`ante_test.go`](ante_test.go) — submission admission and fee-exemption tests.
- [`ante_locks_test.go`](ante_locks_test.go) — submission rate-lock tests.
- [`gateway_test.go`](gateway_test.go) — HTTP query gateway tests.
- [`gov_appended_market_test.go`](gov_appended_market_test.go) — governed market-list extension tests.
- [`gov_disabled_market_test.go`](gov_disabled_market_test.go) — disabled-market behavior tests.
- [`gov_mark_ema_test.go`](gov_mark_ema_test.go) — governed mark-price smoothing tests.
- [`gov_max_observations_test.go`](gov_max_observations_test.go) — governed submission observation-limit tests.
- [`gov_min_sources_test.go`](gov_min_sources_test.go) — governed aggregation source-count tests.
- [`gov_route_test.go`](gov_route_test.go) — governance proposal routing to parameter updates.
- [`module_genesis_test.go`](module_genesis_test.go) — module genesis validation tests.
- [`module_query_test.go`](module_query_test.go) — message and query service registration tests.
- [`module_test.go`](module_test.go) — module lifecycle and aggregation tests.
- [`proof_pricefeed_0x1020_test.go`](proof_pricefeed_0x1020_test.go) — transaction encoding, admission, genesis, and precompile integration tests.
- [`repair_m480_test.go`](repair_m480_test.go) — aggregation events, bonded-power quorum, and minimum-source tests.

## Build and test

The module belongs to the root [`go.mod`](../../go.mod) and has no separate
build target. Run these targets from the repository root; they are defined in
the root [`Makefile`](../../Makefile) and delegate to [`chain.mk`](../../chain.mk):

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds the node and its native library dependencies.
`paxeer-test` runs the chain package tests, including this module's packages.

## Configuration

The module reads its configuration from on-chain state and imported genesis;
it does not read module-specific environment variables or configuration files.
[`types/params.go`](types/params.go) defines the parameters, and
[`keeper/params.go`](keeper/params.go) applies authority-controlled updates
through `MsgSetParams`:

- `authority`, `markets`
- `staleness_window_ms`, `max_clock_skew_ms`, `max_price_age_ms`
- `min_sources`, `max_divergence_bps`, `max_deviation_bps`
- `max_observations_per_submission`, `max_submission_bytes`
- `mark_ema_tau_ms`, `max_mark_basis_bps`

Genesis import/export is implemented in
[`keeper/genesis.go`](keeper/genesis.go); parameter defaults and validation are
defined in [`types/params.go`](types/params.go).

## Documentation

- [Chain module parameters and messages](../../docs/system/chain-modules.md#pricefeed)
- [Pricefeed precompile ABI reference](../../docs/system/precompiles.md)
- [Documentation site: native modules](../../docs/site/paxeer-docs/src/app/modules/page.tsx)
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx)
- [Documentation site: perpetuals](../../docs/site/paxeer-docs/src/app/perpetuals/page.tsx)
