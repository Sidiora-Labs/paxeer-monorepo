# xweb precompile

This directory holds the Go implementation, embedded ABI, Solidity interfaces and tests for the xweb native EVM precompile on Paxeer X Network. It connects contracts to the [xweb module](../../modules/xweb/) for paid requests for attested web data, fulfilment, bounded callbacks, refunds and result queries. It also accepts program-origin observations through `fulfilProgram` and exposes committed answers through `getProgramResult`, with `getKernelNetworkId` identifying the kernel network.

## Layout

- [xweb.go](xweb.go): precompile construction, method dispatch, contract requests, callbacks, refunds, queries and request events.
- [program.go](program.go): program fulfilment, result queries and kernel network queries.
- [gas.go](gas.go): gas calculation for program observations and their signatures.
- [abi.json](abi.json): embedded method and event definitions.
- [XWeb.sol](XWeb.sol): `IXWeb` and `IXWebConsumer` Solidity interfaces and precompile constants.
- [setup.go](setup.go): registration for the application's selected upgrade version.
- [versions](versions): version list used by precompile version generation.
- [legacy/](legacy/): versioned Go implementation and ABI under `v68/`.
- [testdata/](testdata/): Rust consumer used by the program event interoperability tests.
- [xweb_test.go](xweb_test.go): contract requests, callbacks, refunds, attestation and Solidity interface tests.
- [gas_test.go](gas_test.go) and [methods_test.go](methods_test.go): program gas accounting and ABI method checks.
- [origin_two_flow_test.go](origin_two_flow_test.go), [program_dispatch_test.go](program_dispatch_test.go) and [program_gate_test.go](program_gate_test.go): program result flow, EVM dispatch and activation checks.
- [program_requested_event_test.go](program_requested_event_test.go), [repair_130_test.go](repair_130_test.go) and [repair_m1282_test.go](repair_m1282_test.go): program event schemas, journal behavior and consumer interoperability tests.
- [proof_ixweb_test.go](proof_ixweb_test.go) and [proof_xweb_0x1019_test.go](proof_xweb_0x1019_test.go): Solidity selectors and application-level program observation tests across Go, Solidity and Rust.

## Build and test

The package belongs to the root [Go module](../../go.mod) and uses the root build targets. Run these commands from the repository root:

```sh
make paxd-native-libs
make paxeer-build
go test -count=1 ./modules/xweb/... ./precompiles/xweb/...
```

The [Makefile](../../Makefile) defines `paxd-native-libs` for the native dependencies and `paxeer-build` for the node build through [chain.mk](../../chain.mk). The focused Go test command is also declared in [the xweb command inventory script](../../scripts/ci/xweb-workflow-check.sh) and [the xweb workflow](../../.github/workflows/xweb-test.yml).

Use the Go version declared in `go.mod`. The application-level tests also invoke Cargo and `npx` with `solc@0.8.27`, so they require Rust, Node.js and npm as well as the native build dependencies.

## Configuration

The precompile reads parameters and the attestor set from the xweb keeper supplied by the application. `getParams` exposes `fee`, `maxPayloadBytes`, `maxCallbackGas`, `timeoutBlocks` and `paused`; `getKernelNetworkId` reads `KernelNetworkId`. See the [module parameter reference](../../docs/system/chain-modules.md) for the underlying configuration.

The runtime package reads no environment variables or external configuration files. `abi.json` is embedded at build time. The Rust consumer test helper reads `CARGO_TARGET_DIR` to select its Cargo build output directory.

## Documentation

- [Precompile methods, events and Solidity interfaces](../../docs/system/precompiles.md).
- [xweb module parameters and messages](../../docs/system/chain-modules.md).
- [Web Search and xweb site page](../../docs/site/paxeer-docs/src/app/xweb/page.tsx).
- [Web Search and xweb Markdown page](../../docs/site/paxeer-docs/public/markdown/xweb.md).
