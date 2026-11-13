# layerxauth

`layerxauth` is the Go module for identity lifecycle operations, session and
capability grants, revocation, and SDK authorization in the Paxeer network. Its
keeper resolves EVM callers through their bound DIDs, executes authority
activities through the shared kernel host, and exposes grant and scope views to
the [authority precompile](../../precompiles/layerxauth/). The module runs inside
`paxd`; its state changes become final when their block is committed.

## Layout

- [ante/](ante/) — transaction decorator that supplies grantor revocation state to SDK authorization checks, with integration tests.
- [keeper/](keeper/) — kernel execution, lifecycle operations, grants, revocation, receipt events, views, parameter storage, genesis handling, and tests.
- [types/](types/) — scoped authorization, payload encoding and validation, keeper interfaces, codecs, errors, parameters, generated protobuf types, and tests.
- [module.go](module.go) — module registration, message service wiring, and genesis import and export.
- [module_test.go](module_test.go) — module service, codec, genesis, and parameter authority tests.

## Build and test

This directory belongs to the root [Go module](../../go.mod) and uses the root
[Makefile](../../Makefile) and [chain.mk](../../chain.mk). Run these commands from
the repository root:

```sh
make paxd-build
make paxeer-test
```

`paxd-build` builds the native libraries and the `paxd` binary that includes this
module. `paxeer-test` runs the chain's Go package tests, including `layerxauth`,
through the package groups in `chain.mk`; it covers more than this directory.
The native host integration requires cgo and the native library toolchains.

## Configuration

The module reads its parameters from chain state and receives its host, SDK
authorization keeper, EVM keeper, and module authority through `NewKeeper`.
It reads no environment variables or standalone configuration files.

The genesis `params` fields are `paused` and `idempotency_retention`.
`MsgUpdateParams` updates them through the configured module authority.
Genesis import and export cover these parameters; authority grants reside in
the shared kernel host store. See [keeper/genesis.go](keeper/genesis.go) and
[types/params.go](types/params.go).

## Documentation

- [Protocol and network integration](../../docs/wiki/Protocol.md)
- [Owner identity registration and recovery](../../docs/wiki/HumanOwnerRegistration.md)
- [Module parameters and messages](../../docs/system/chain-modules.md#layerxauth)
- [Authority precompile reference](../../docs/system/precompiles.md#layerxauth-abijson)
- [Documentation site: modules](../../docs/site/paxeer-docs/public/markdown/modules.md)
- [Documentation site: identity](../../docs/site/paxeer-docs/public/markdown/identity.md)
