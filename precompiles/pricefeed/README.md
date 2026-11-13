# Pricefeed precompile

This directory holds the Go implementation, Solidity interface and ABI for the
Paxeer X pricefeed precompile at `0x1020`. It exposes the native
[pricefeed module](../../modules/pricefeed)'s index and mark prices, markets,
observations, feed keys and parameters to EVM callers, and lets validator
operators register and revoke feed keys. Validators submit prices through
`MsgSubmitPrices` on the native module; the precompile reads the resulting state.

## Layout

- [PriceFeed.sol](PriceFeed.sol): Solidity interface, return structs, events and precompile address constants.
- [pricefeed.go](pricefeed.go): constructors, method dispatch, gas calculation and call guards.
- [views.go](views.go): ABI views for prices, markets, observations, feed keys and parameters.
- [register.go](register.go): feed-key registration, revocation and EVM event emission.
- [setup.go](setup.go): generated versioned precompile construction.
- [abi/](abi/): embedded `abi.json` and its standalone Go export in `abi.go`.
- [testdata/](testdata/): golden ABI return data and revert payloads.
- [abi_test.go](abi_test.go): ABI selectors, tuple layouts, events and Solidity interface checks.
- [dispatch_test.go](dispatch_test.go): method dispatch, gas accounting, call guards and refusal checks.
- [gov_views_test.go](gov_views_test.go): governed parameter changes exposed through EVM views.
- [harness_test.go](harness_test.go): shared keeper and EVM test setup.
- [lxp_registry_test.go](lxp_registry_test.go): pricefeed error-code registration checks.
- [register_test.go](register_test.go): feed-key ownership, replacement, revocation and event checks.
- [revert_golden_test.go](revert_golden_test.go): byte-for-byte revert payload checks.
- [views_golden_test.go](views_golden_test.go): byte-for-byte view return checks.
- [views_test.go](views_test.go): price freshness, batch limits, market lookup, observations and feed-key views.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate
build manifest or Makefile. From the repository root, build the node that
includes it with the `build` target in [chain.mk](../../chain.mk):

```sh
make -f chain.mk build
```

Run the module and precompile tests with the Go test command used by
[the price-sidecar domain script](../../tools/price-sidecar/tests/domain-check.sh):

```sh
go test ./modules/pricefeed/... ./precompiles/pricefeed/...
```

The tests cover ABI compatibility, EVM calls, gas, feed-key changes, governed
parameters and golden outputs. Prices use unsigned integers with 18 decimals;
`getPrices` accepts at most 64 market IDs and preserves their input order.

## Configuration

The precompile reads no runtime environment variables or configuration files.
The application supplies its pricefeed and EVM keepers, and the ABI is embedded
from [abi/abi.json](abi/abi.json). Market definitions and pricing parameters come
from the native module's governed state. `getParams` exposes
`stalenessWindowMs`, `maxClockSkewMs`, `minSources`, `maxDivergenceBps`,
`maxDeviationBps`, `maxObservationsPerSubmission`, `maxSubmissionBytes`,
`markEmaTauMs`, `maxMarkBasisBps` and `maxPriceAgeMs`; `getMarket` and
`getMarketCount` expose the configured markets.

## Documentation

- [Precompile reference](../../docs/system/precompiles.md): activation, ABI functions and events.
- [Chain module reference](../../docs/system/chain-modules.md#pricefeed): native pricefeed parameters and messages.
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: Markdown precompile reference](../../docs/site/paxeer-docs/public/markdown/precompiles.md).
