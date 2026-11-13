# Consensus engine

This directory contains the Byzantine Fault Tolerant consensus engine embedded in `paxd` for Paxeer mainnet-beta, one network with EVM chain ID 125 (`0x7d`). The Tendermint-derived engine orders and commits blocks, connects to the application in-process, and includes classic consensus and optional Autobahn components. The LayerX kernel runs inside `paxd`, participates in execution and block proposal, and a transaction receipt is final once its block is committed.

## Layout

| Path | Contents |
| --- | --- |
| [`abci/`](abci/) | Go application interface, request and response types, and the example key-value application |
| [`autobahn/`](autobahn/) | Autobahn blocks, proposals, votes, committees and quorum certificates |
| [`cmd/`](cmd/) | Cobra [operator commands](cmd/tendermint/commands/), the gRPC [remote signer server](cmd/priv_val_server/), and [Dredd hooks](cmd/contract_tests/) for the [RPC contract](rpc/openapi/openapi.yaml) |
| [`config/`](config/) | Node configuration types, TOML templates and Autobahn JSON configuration loading |
| [`crypto/`](crypto/) | Ed25519 keys, hashing and Merkle trees |
| [`export/`](export/) | Exported aliases for block stores, state stores and event queries |
| [`internal/`](internal/) | Classic consensus and Autobahn implementations, application proxy, mempool, evidence, peer networking, synchronization, stores and RPC handlers |
| [`libs/`](libs/) | Shared utilities for encoding, command-line handling, services, synchronization and structured concurrency |
| [`light/`](light/) | Light client and trusted-header verification |
| [`networks/`](networks/) | Network examples and remote deployment tooling |
| [`node/`](node/) | Consensus node assembly and service lifecycle |
| [`privval/`](privval/) | File-based validator signing state and remote signing support |
| [`proto/`](proto/) | Protocol buffer definitions and generated Go code |
| [`rpc/`](rpc/) | RPC clients, JSON-RPC server library, core types and OpenAPI description |
| [`scripts/`](scripts/) | Code generation, metrics and maintenance scripts |
| [`spec/`](spec/) | Tendermint protocol specifications, fork-specific implementation references, TLA+ and Ivy models |
| [`state/`](state/) | Application response result hashing |
| [`test/`](test/) | Fuzz targets, test fixtures and end-to-end harness |
| [`types/`](types/) | Blocks, votes, validators, evidence, genesis, consensus parameters and proposer interfaces |
| [`utils/`](utils/) | Slice mapping helper |
| [`version/`](version/) | Engine, ABCI and protocol version declarations |
| [`Makefile`](Makefile) | Engine test, code generation and remote signer build targets |

## Build and test

The packages belong to the repository's root [`go.mod`](../go.mod), under `github.com/Sidiora-Labs/Paxeer-X-Network/consensus/...`. Build the embedded engine through the root [`Makefile`](../Makefile), which delegates to [`chain.mk`](../chain.mk). From the repository root:

```sh
make paxeer-build
make -C consensus test
```

The build produces `build/paxd`. The test target comes from [`test/Makefile`](test/Makefile) and runs the directory's Go packages with `go test -p 1` and the `deadlock` build tag. For a direct Go test invocation from the repository root, use `go test ./consensus/...`.

The directory's [`Makefile`](Makefile) also provides `build_privval_server` for the remote signer server. Fuzz targets are described in [`test/fuzz/README.md`](test/fuzz/README.md).

## Configuration

[`paxd start`](../sdk/server/start.go) constructs the consensus node. The server reads `config/config.toml` under the node's `--home` directory through [`sdk/server/util.go`](../sdk/server/util.go). The engine also loads `config/genesis.json`, `config/node_key.json`, `config/priv_validator_key.json` and `data/priv_validator_state.json` through the paths defined in [`config/config.go`](config/config.go). Remote signer TLS files are selected by `priv-validator.client-certificate-file`, `priv-validator.client-key-file` and `priv-validator.root-ca-file`.

The server binds environment variables using the executable name as a prefix and replaces dots and hyphens in configuration keys with underscores. With `paxd`, this includes `PAXD_HOME`. The end-to-end node reads `TMHOME`; the end-to-end runner reads `DOCKER_HOST`, `DOCKER_TLS_VERIFY` and `DOCKER_CERT_PATH` in [`test/e2e/runner/cleanup.go`](test/e2e/runner/cleanup.go).

The `autobahn-config-file` key selects a JSON file described by [`AutobahnFileConfig`](config/autobahn.go). An empty setting selects classic consensus. Autobahn persists state through `persistent_state_dir`, with relative paths resolved against the node home. The mutually exclusive `unsafe_test_only_disable_persistence` option permits in-memory operation for test networks. The `consensus.sequencer` settings configure the integrated proposer mode; validation rejects combining it with Autobahn. Autobahn requires a local validator signer.

[`paxd tendermint gen-autobahn-config`](cmd/tendermint/commands/gen_autobahn_config.go) accepts node directories and the `--output` and `--persistent-state-dir` flags. It reads `validator_pubkey.txt`, `node_pubkey.txt`, `autobahn_address.txt` and `evmrpc_url.txt` from each supplied node directory.

## Documentation

- [Consensus documentation site page](../docs/site/paxeer-docs/src/app/consensus/page.tsx) and its [Markdown page](../docs/site/paxeer-docs/public/markdown/consensus.md).
- [Validator runbook](../docs/release/7.0.0-validator-runbook.md) and [consensus-changing release commits](../docs/release/consensus-commits.md).
- [Protocol specifications and implementation map](spec/README.md), including inherited Tendermint descriptions and research models. The paper "[The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)" analyzes the classic protocol's safety and liveness; Autobahn behavior is defined by the linked implementation.
- [Application interface](abci/types/application.go) and [in-process proxy](internal/proxy/). The lifecycle includes `ProcessProposal`, `FinalizeBlock` and `Commit`; the proxy invokes Go methods directly.
- [Operator command wiring](../sdk/server/util.go) under `paxd tendermint`: `show-node-id`, `show-validator`, `show-address`, `version`, `gen-autobahn-config`, `gen-validator`, `reindex-event`, `light`, `reset`, `unsafe-reset-all`, `gen-node-key`, `inspect`, `key-migrate`, `debug` and `completion`.
- [cgo kernel host](../modules/lxphost/) and [application integration](../node/app.go). In sequencer mode, the in-process [ConsensusSource](../node/sequencer/consensus_source.go) supplies the executor's sealed pending block through [SequencerSource](types/sequencer.go).
- [Version declarations](version/version.go): the fallback Tendermint version is `0.35.0-unreleased` and the ABCI library version is `0.17.0`.
- [Remote deployment tooling](networks/remote/README.md) and [end-to-end Docker image definition](test/e2e/docker/Dockerfile).
- [Security policy](../SECURITY.md) for vulnerability reports.
