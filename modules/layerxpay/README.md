# LayerX Pay

This directory contains the Go chain module for escrow, budgets, streams and
service agreements in Paxeer X. Its keeper connects the payment precompile to
the shared kernel host in `modules/lxphost`, resolves bound EVM callers, applies
payment family and pause gates, and exposes payment state. The module runs
inside `paxd` as part of the same network; a receipt is final when its block is
committed.

## Layout

- `client/` — CLI queries for the module's parameters.
- `integration/` — application tests for shared-host genesis and governance updates.
- `keeper/` — payment execution, state views, authority checks, parameter services and genesis import/export.
- `testutil/` — EVM binding reader construction for tests.
- `types/` — activity mappings, payload encoders, record decoders, parameters, genesis types and generated message/query bindings.
- `module.go` — SDK module registration, services, query commands and genesis lifecycle.
- `module_test.go` — module genesis and service registration tests.

## Build and test

This directory belongs to the root [Go module](../../go.mod). Build it as part
of the node and run its tests through the root [Makefile](../../Makefile):

```sh
make paxd-build
make paxeer-test
```

Run these commands from the repository root. `paxd-build` builds the native
libraries and the node at `build/paxd`. `paxeer-test` invokes the Go test targets
in [chain.mk](../../chain.mk), including this directory's packages, with race
detection and coverage. Both targets use the local Go dependency cache with
`GOPROXY=off`; dependencies must be available before running them. The root
Makefile and `chain.mk` supply the native library prerequisites.

## Configuration

Runtime settings are the on-chain `Params` fields: `paused`, `escrow_enabled`,
`budget_enabled`, `stream_enabled` and `service_enabled`. Genesis supplies
`params`, `kernel_records`, `escrow_receipts` and `agent_stream_metadata` through
`types.GenesisState`. Authorized `MsgUpdateParams` messages update the module
parameters; enabling a family also requires its shared kernel enable flag.

The module reads its parameters from the SDK store. It reads no environment
variables or standalone configuration files directly. The shared host, store
keys, EVM binding reader, custody reader and authority are supplied to
`keeper.NewKeeper` by the application.

## Documentation

- [Chain module parameters and messages](../../docs/system/chain-modules.md#layerxpay)
- [Payment precompile declarations](../../docs/system/precompiles.md#layerxpay-abijson)
- [Documentation site: module overview](../../docs/site/paxeer-docs/src/app/modules/page.tsx)
- [Documentation site: streaming payments](../../docs/site/paxeer-docs/src/app/streams/page.tsx)
