# Consensus engine

This directory contains the Byzantine Fault Tolerant consensus engine embedded in `paxd` for Paxeer mainnet-beta, one network with EVM chain ID 125 (`0x7d`). The engine is a fork of Tendermint Core: [`version/version.go`](version/version.go) defines the fallback Tendermint version as `0.35.0-unreleased` and the ABCI library version as `0.17.0`.

The engine is not a separate Go module. Its packages are imported as `github.com/Sidiora-Labs/Paxeer-X-Network/consensus/...` from the repository's root [`go.mod`](../go.mod), and it is embedded in `paxd` rather than shipped as its own binary.

The [specification](./spec/README.md) includes inherited Tendermint protocol descriptions and research models. The paper "[The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)" provides the classic protocol's safety and liveness analysis. For this fork's application interface, proposal handling and optional Autobahn mode, consult the linked code below; upstream descriptions do not cover every mode implemented here.

## How it fits the network

- `paxd start` constructs the node with [`node`](node/) from [`sdk/server/start.go`](../sdk/server/start.go).
- `paxd tendermint` exposes the engine's operator subcommands, wired in [`sdk/server/util.go`](../sdk/server/util.go): `show-node-id`, `show-validator`, `show-address`, `version`, `gen-autobahn-config`, `gen-validator`, `reindex-event`, `light`, `reset`, `unsafe-reset-all`, `gen-node-key`, `inspect`, `key-migrate`, `debug` and `completion`. Their implementations live in [`cmd/tendermint/commands`](cmd/tendermint/commands/) and [`sdk/server`](../sdk/server/).
- The application is called in-process through the Go `Application` interface in [`abci/types/application.go`](abci/types/application.go), wrapped by [`internal/proxy`](internal/proxy/). Its block lifecycle includes `ProcessProposal`, `FinalizeBlock` and `Commit`. This tree has no socket or gRPC ABCI transport.
- Autobahn is optional. `autobahn-config-file` in the node configuration ([`config/config.go`](config/config.go)) points at its JSON configuration; leaving it empty disables Autobahn. Configuration validation rejects combining Autobahn with sequencer mode, and [`node/node.go`](node/node.go) rejects Autobahn with a remote validator signer. `paxd tendermint gen-autobahn-config [node-dirs...] --output <file>` generates the JSON file.

The LayerX kernel runs inside `paxd` through the cgo host in [`modules/lxphost`](../modules/lxphost/), wired into the application in [`node/app.go`](../node/app.go). It participates in this network's execution and block proposal path. In sequencer mode, the in-process [`ConsensusSource`](../node/sequencer/consensus_source.go) supplies the executor's sealed pending block through the [`SequencerSource`](types/sequencer.go) interface. A transaction receipt is final once its block is committed.

## Layout

| Path | Contents |
| --- | --- |
| [`abci/`](abci/) | Application interface types and the example key-value application |
| [`autobahn/`](autobahn/), [`internal/autobahn/`](internal/autobahn/) | Autobahn types and implementation |
| [`cmd/tendermint/commands/`](cmd/tendermint/commands/) | Cobra commands mounted under `paxd tendermint` |
| [`cmd/priv_val_server/`](cmd/priv_val_server/) | gRPC remote signer server built on [`privval/`](privval/) |
| [`cmd/contract_tests/`](cmd/contract_tests/) | Dredd hooks for the RPC contract tests against [`rpc/openapi/openapi.yaml`](rpc/openapi/openapi.yaml) |
| [`config/`](config/) | Node configuration |
| [`crypto/`](crypto/) | Ed25519 keys, hashing and Merkle trees |
| [`internal/`](internal/) | Consensus state machine, mempool, evidence, p2p, block sync, state sync, block and state stores, RPC handlers |
| [`light/`](light/) | Light client |
| [`node/`](node/) | Node assembly |
| [`proto/`](proto/) | Protocol buffer definitions and generated Go code |
| [`rpc/`](rpc/) | RPC client, JSON-RPC server library, core types and the OpenAPI description |
| [`types/`](types/) | Blocks, votes, validators, evidence, genesis and consensus parameters |
| [`spec/`](spec/) | Upstream protocol specifications, TLA+ and Ivy models |
| [`test/`](test/) | Fuzz targets and end-to-end harness |
| [`networks/`](networks/), [`scripts/`](scripts/) | Upstream deployment and maintenance tooling |

## Build and test

From the repository root:

```sh
make paxeer-build        # builds ./build/paxd, which embeds this engine
go test ./consensus/...  # runs the engine's Go tests
```

[`Makefile`](Makefile) in this directory is inherited from upstream. Its `build` and `install` targets point at `./cmd/tendermint`, which contains the `commands` subdirectory but no Go `main` package, so they do not produce a binary. Build the engine through `paxd`. The upstream [remote deployment tooling](networks/remote/README.md) and [end-to-end Docker image](test/e2e/docker/Dockerfile) also assume a standalone `tendermint` binary and are not current `paxd` deployment instructions.

The fuzz targets are described in [`test/fuzz/README.md`](test/fuzz/README.md).

## Security

To report a vulnerability, follow the repository [security policy](../SECURITY.md).

## Documentation

Hosted documentation for Paxeer X Network is at [docs.paxeer.app](https://docs.paxeer.app/). The source repository is [Sidiora-Labs/paxeer-monorepo](https://github.com/Sidiora-Labs/paxeer-monorepo).
