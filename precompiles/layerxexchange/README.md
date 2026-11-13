# LayerX exchange precompile

This directory holds the `layerxExchange` EVM precompile implementation, exchange-intent ABI, Solidity interface, and tests. It provides compatibility code for the `layerxexchange` module, including custody-backed margin intents and witness-based state reads. In the merged Paxeer network, the node selects the native [`perps` precompile](../perps/) at `0x1015` when `v6.12.0` is active; exchange-intent creation is retired under that activation. The kernel executes within the network, and receipts are final when their blocks are committed.

## Layout

- [LayerXExchange.sol](LayerXExchange.sol): Solidity interface, tuple types, events, and gas formula for the exchange-intent surface.
- [abi.json](abi.json): Embedded ABI used by the Go executor, including both `placeOrder` overloads.
- [layerxexchange.go](layerxexchange.go): Constructor, gas calculation, calldata dispatch, keeper calls, and EVM event encoding.
- [errors.go](errors.go): Exchange and kernel error conversion to named EVM revert reasons.
- [setup.go](setup.go): Generated versioned precompile registration.
- [versions](versions): Version list used by precompile setup generation.
- [layerxexchange_test.go](layerxexchange_test.go): Tests of deposits, intents, events, state witnesses, gas, and time-in-force capabilities.
- [errors_test.go](errors_test.go): Tests of error-code mapping and ABI revert encoding.
- [repair_459_test.go](repair_459_test.go): Tests of account-bound order overloads, ownership checks, and TypeScript calldata.
- [testdata/](testdata/): TypeScript calldata fixture and its generator, `repair_459_generate.mjs`.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. Run these targets from the repository root using [chain.mk](../../chain.mk):

```sh
make -f chain.mk build
make -f chain.mk test
```

The build target compiles `paxd` with its native library prerequisites. The test target splits the root Go packages into groups and runs their tests, including this package, with race detection and coverage. The root [Makefile](../../Makefile) also exposes `paxd-build` for the node build with native library prerequisites; its plain `build` and `test` targets cover the kernel.

## Configuration

The executor reads the embedded `abi.json` and receives bank, EVM, and exchange keepers through its constructor. It reads no runtime environment variables or external configuration files of its own; module parameters and activation state come from the supplied keepers.

The tests read these environment variables:

- `PAXEER_X_TIF_NATIVE_GENESIS_FILE`: Path to native genesis input for the time-in-force capability test.
- `PAXEER_X_TIF_INGRESS_FILE`: Indicates that the cross-language test requires the native genesis input file.

Tests also read `testdata/repair_459_typescript.json` and the kernel result declarations in [lxp_result.h](../../include/layerx/lxp_result.h).

## Documentation

- [Precompile ABIs and availability](../../docs/system/precompiles.md).
- [Chain modules and parameters](../../docs/system/chain-modules.md).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: modules](../../docs/site/paxeer-docs/src/app/modules/page.tsx).
