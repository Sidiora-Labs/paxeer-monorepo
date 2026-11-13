# IBC transfer precompile

This directory holds the Go implementation, Solidity interface, embedded ABI and tests for the IBC transfer precompile in Paxeer's BFT network. It connects EVM calls to the native ICS-20 transfer keeper through `transfer` and `transferWithDefaultTimeout`. The immediate caller's associated Paxeer account supplies the tokens; both methods are nonpayable and reject staticcall and delegatecall. A successful call submits the source-chain transfer; destination delivery, acknowledgement and timeout handling follow the IBC packet lifecycle.

## Layout

- [ibc.go](ibc.go) — transfer execution, caller and argument validation, timeout derivation and keeper calls.
- [IBC.sol](IBC.sol) — Solidity interface and precompile constants.
- [abi.json](abi.json) — embedded ABI for both transfer methods.
- [setup.go](setup.go) — generated registration of versioned implementations.
- [versions](versions) — implementation version list.
- [legacy/](legacy/) — versioned implementations and their embedded ABIs.
- [ibc_test.go](ibc_test.go) — transfer validation, timeout tests and persistent IBC application fixtures.
- [abi_freeze_test.go](abi_freeze_test.go) — ABI, selector and precompile address checks.
- [gas_golden_test.go](gas_golden_test.go) — gas checks for zero and positive transfers through constructed and registered implementations.
- [repair_m1265_test.go](repair_m1265_test.go) — packet delivery, acknowledgements, timeout refunds, recovery and replay tests.
- [repair_m1265.kvx](repair_m1265.kvx) — transfer and gas test coverage metadata.

## Build and test

The package belongs to the root [go.mod](../../go.mod) and has no separate Makefile. From the repository root, use the targets in [Makefile](../../Makefile), which delegate to [chain.mk](../../chain.mk):

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds `build/paxd`, including the registered IBC precompile and its native library dependencies. `paxeer-test` runs the chain's Go package test groups, including this package, with the race detector and coverage. Both targets use the local Go dependency cache with `GOPROXY` disabled for downloads.

## Configuration

The precompile reads no environment variables or runtime configuration files. It embeds `abi.json` at build time and receives the transfer, EVM, client, connection and channel keepers from the node. Transfer parameters come from the ABI arguments. `transferWithDefaultTimeout` derives its timeout from channel and client state, block time, and `DefaultRelativePacketTimeoutHeight` and `DefaultRelativePacketTimeoutTimestamp` in [transfer types](../../interchain/modules/apps/transfer/types/).

## Documentation

- [Precompile ABI reference](../../docs/system/precompiles.md) — includes the IBC method signatures.
- [IBC guide on the documentation site](../../docs/site/paxeer-docs/src/app/interchain/page.tsx) — transfer prerequisites and packet lifecycle.
- [Precompile guide on the documentation site](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx) — registered EVM precompiles.
- [IBC guide in Markdown](../../docs/site/paxeer-docs/public/markdown/interchain.md).
