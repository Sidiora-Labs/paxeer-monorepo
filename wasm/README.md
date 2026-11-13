# CosmWasm module

This directory holds the CosmWasm module embedded in `paxd`, its application helpers, protobuf definitions, benchmarks, and development tools. The module manages WebAssembly contract code, instantiation, execution, queries, migrations, and IBC messages within the Paxeer network. It connects contract execution to the SDK account, bank, storage, and message services and uses the execution libraries in [`wasm-runtime`](../wasm-runtime/).

## Layout

- [`app/`](app/): application wiring, transaction ante handlers, genesis and export helpers, and test support.
- [`benchmarks/`](benchmarks/): transaction and contract benchmarks with compiled contract fixtures in `testdata/`.
- [`contrib/`](contrib/): developer tools, protobuf tooling, Prometheus configuration, and local upstream `wasmd` examples.
- [`proto/`](proto/): CosmWasm transaction, query, genesis, governance, and IBC protobuf definitions.
- [`scripts/`](scripts/): entry points for the repository's protobuf and Swagger generators.
- [`staticarchive/`](staticarchive/): validation of native archive manifests, checksums, object members, and linker scripts.
- [`x/`](x/): the `wasm` SDK module, including its keeper, message handlers, CLI, types, simulations, tests, and versioned native VM artifacts.
- [`Makefile`](Makefile): help listing the root build targets and module test command.

## Build and test

The directory belongs to the root [`go.mod`](../go.mod). Build the node from the repository root using the target in the root [`Makefile`](../Makefile):

```sh
make paxd-build
```

The module [`Makefile`](Makefile) lists the following commands for preparing native libraries and running the module tests. Run them from the repository root, in this order:

```sh
make paxd-native-libs
CGO_ENABLED=1 go test -mod=readonly ./wasm/x/wasm/...
```

The build uses Go, a C/C++ toolchain, and the Rust toolchain for the native execution libraries. The root node build also depends on `paxd-native-libs`.

The scripts in [`scripts/`](scripts/) delegate protobuf and Wasm Swagger generation to the root generator. From the repository root:

```sh
bash scripts/protoc.sh
bash scripts/protoc.sh --wasm-swagger
```

Generation uses the root [`buf.yaml`](../buf.yaml) and [`buf.gen.yaml`](../buf.gen.yaml). See the [script guide](scripts/README.md) for the generator dependencies and outputs.

## Configuration

[`ReadWasmConfig`](x/wasm/module.go) reads the `[wasm]` section of the node's `config/app.toml`: `memory_cache_size`, `query_gas_limit`, and `simulation_gas_limit`. The corresponding start flags are `wasm.memory_cache_size`, `wasm.query_gas_limit`, and `wasm.simulation_gas_limit`. It also reads the global `trace` option for contract debugging. The node configuration template is defined in [`app_config.go`](../daemon/paxd/cmd/app_config.go).

The module's `uploadAccess` and `instantiateAccess` parameters control code upload and default instantiation permissions; their types are defined in [`params.go`](x/wasm/types/params.go).

The local upstream example's [`setup_wasmd.sh`](contrib/local/setup_wasmd.sh) reads the environment variables `PASSWORD`, `STAKE_TOKEN`, `FEE_TOKEN`, `CHAIN_ID`, `MONIKER`, and `HOME`, and updates `.wasmd/config/genesis.json` under the home directory. The [relayer examples](contrib/relayer-tests/README.md) use the files under [`configs/wasmd/chains/`](contrib/relayer-tests/configs/wasmd/chains/) and [`configs/wasmd/paths/`](contrib/relayer-tests/configs/wasmd/paths/), together with the local nodes' `config/config.toml` and `config/app.toml`. These examples use an external upstream `wasmd` executable.

The [Prometheus example](contrib/prometheus/README.md) uses [`prometheus.yaml`](contrib/prometheus/prometheus.yaml). Native archive validation reads `static-archives.json` in the versioned artifact directories [`v152/api/`](x/wasm/artifacts/v152/api/) and [`v155/api/`](x/wasm/artifacts/v155/api/).

## Documentation

- [WASM guide](../docs/site/paxeer-docs/public/markdown/wasm.md) and its [documentation site page](../docs/site/paxeer-docs/src/app/wasm/page.tsx).
- [WASM runtime guide](../docs/site/paxeer-docs/public/markdown/wasm-runtime.md).
- [WASM bindings guide](../docs/site/paxeer-docs/public/markdown/wasmbinding.md).
- [Module reference](x/wasm/README.md), [governance](x/wasm/Governance.md), and [IBC integration](x/wasm/IBC.md).
- [Local examples](contrib/local/README.md), [relayer tests](contrib/relayer-tests/README.md), and [developer tools](contrib/devtools/README.md).
