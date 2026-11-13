# LayerX exchange module

`layerxexchange` contains the Go module registration, exchange state types and keeper for exchange-intent compatibility in Paxeer X. It retains genesis import and export, intent and owner-nonce storage, proof verification helpers, and oracle observation replay through the native kernel host. Intent submission and administrative messages return `ErrRetired` once the retirement upgrade is active at the block height being executed. The merged network executes trading through the native [`perps`](../perps/) module.

## Layout

- [`module.go`](module.go): module registration, message service wiring, genesis validation, import and export.
- [`keeper/`](keeper/): parameter and intent storage, retirement checks, account-bound orders, state proofs, oracle replay and keeper tests.
- [`types/`](types/): parameters, genesis and intent types, keeper interfaces, errors, codecs, generated protobuf messages and type tests.

## Build and test

This directory belongs to the repository's root [`go.mod`](../../go.mod) and has no separate build manifest or Makefile. Run the covering targets from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The [`Makefile`](../../Makefile) defines `paxd-build`, which builds the native libraries and the chain binary. The [`chain.mk`](../../chain.mk) test target builds the native libraries and runs the root Go packages with tests, including this module's keeper and types packages, with race detection and coverage.

## Configuration

Runtime configuration comes from the module's genesis state and stored parameters: `authority`, `markets`, `paused` and `time_in_force`. Market entries contain `market_id`, `margin_asset_id`, `enabled`, `native_genesis`, `capability_batch` and `capability_witness`. Parameter validation is in [`types/params.go`](types/params.go); genesis handling is in [`keeper/genesis.go`](keeper/genesis.go). The runtime sources read no environment variables or standalone configuration files.

The keeper tests read these environment variables for native genesis input, intent output and database reopen checks:

- `PAXEER_X_TIF_NATIVE_GENESIS_FILE`
- `PAXEER_X_TIF_INGRESS_FILE`
- `PAXEER_X_TIF_KEEPER_PHASE`
- `PAXEER_X_TIF_KEEPER_DB`

## Documentation

- [Chain module parameters and messages](../../docs/system/chain-modules.md#layerxexchange)
- [Exchange ABI and precompile selection](../../docs/system/precompiles.md)
- [Documentation site module overview and settlement compatibility](../../docs/site/paxeer-docs/public/markdown/modules.md)
