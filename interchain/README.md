# interchain (ibc-go)

This directory contains the vendored [ibc-go](https://github.com/cosmos/ibc-go) implementation of the Inter-Blockchain Communication protocol (IBC), its protocol definitions and its test applications. It provides the Cosmos SDK modules for connection-oriented, stateful, reliable, ordered and authenticated communication between sovereign blockchains. In Paxeer mainnet-beta, [`node/app.go`](../node/app.go) wires the IBC core module and the ICS 20 transfer application into `paxd`; [`wasm/x/wasm`](../wasm/x/wasm) uses the core channel and port packages for contract IBC. Packages are imported as `github.com/Sidiora-Labs/Paxeer-X-Network/interchain/...`.

## Layout

- [`modules/`](modules) — IBC core, transfer and interchain-account applications, and light clients.
- [`proto/`](proto) — Protocol Buffer definitions for IBC state, messages and queries.
- [`testing/`](testing) — chain, endpoint and coordinator helpers, with a simulation application.
- [`third_party/`](third_party) — Protocol Buffer dependencies.
- [`.gitignore`](.gitignore) — ignore patterns for build outputs and local files.
- [`releases-decision-tree.png`](releases-decision-tree.png) — release decision diagram.

The [core implementation](modules/core) contains [ICS 02 Client](modules/core/02-client), [ICS 03 Connection](modules/core/03-connection), [ICS 04 Channel](modules/core/04-channel), [ICS 05 Port](modules/core/05-port), [ICS 23 Commitment](modules/core/23-commitment/types) and [ICS 24 Host](modules/core/24-host).

The applications implement [ICS 20 Fungible Token Transfers](modules/apps/transfer) and [ICS 27 Interchain Accounts](modules/apps/27-interchain-accounts). Interchain accounts are provided for applications that explicitly integrate them, including the [test application](testing/simapp). They are not wired into `paxd`, whose IBC routes are transfer and CosmWasm.

Light clients include [ICS 07 Tendermint](modules/light-clients/07-tendermint), [ICS 06 Solo Machine](modules/light-clients/06-solomachine) and [ICS 09 Localhost](modules/light-clients/09-localhost), which verifies against the local store.

## Build and test

These packages belong to the repository's root [Go module](../go.mod). The root [Makefile](../Makefile) delegates the chain build and tests to [chain.mk](../chain.mk). Run from the repository root:

```sh
make paxeer-build
make paxeer-test
```

The build compiles `paxd` and its native library prerequisites. The test target splits the root Go module's test packages into groups, including the packages under this directory, and runs them with race detection and coverage. Both targets use the locally available Go dependencies with `GOPROXY` set by the Makefile.

## Configuration

IBC module configuration is carried in genesis state and on-chain parameters. Core parameters include `InboundEnabled` and `OutboundEnabled`; transfer parameters include `SendEnabled` and `ReceiveEnabled`. The application supplies keepers, scoped capabilities and IBC routes through [`node/app.go`](../node/app.go).

The [`simd` test application](testing/simapp/simd/cmd/root.go) uses the SDK client and server configuration readers for `client.toml`, `config.toml` and `app.toml`, and loads `genesis.json`. These files reside in the configuration directory under its selected home directory. The IBC modules do not read component-specific environment variables.

## Documentation

- [IBC core concepts](modules/core/spec/01_concepts.md)
- [IBC testing package](testing/README.md)
- [Interchain-account integration](modules/apps/27-interchain-accounts/README.md)
- [IBC precompile reference](../docs/system/precompiles.md#ibc-abijson)
- [Interchain documentation site page](../docs/site/paxeer-docs/src/app/interchain/page.tsx)
- [Interchain documentation Markdown](../docs/site/paxeer-docs/public/markdown/interchain.md)
- [IBC Website](https://ibcprotocol.org/)
- [IBC Specification](https://github.com/cosmos/ibc)
- [Documentation](https://ibc.cosmos.network/main/ibc/overview.html)
