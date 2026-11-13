# Paxeer daemon

This directory holds the `paxd` chain-node entry point and the committed receipt fixture. `paxd` runs the Cosmos SDK application, native EVM, and the LayerX kernel in one BFT network; the fixture records and checks receipt vectors against a local node. A transaction is final when its block is committed.

## Layout

- [`paxd/`](paxd/) — the `paxd` executable and its Cobra commands, including node initialization, sequencer key generation, snapshots, and logging.
- [`committed-fixture/`](committed-fixture/) — the receipt-vector recorder, vector implementations and tests, contract fixtures, and replication helpers.
- [`paxd/main.go`](paxd/main.go) — the `paxd` process entry point.
- [`committed-fixture/main.go`](committed-fixture/main.go) — the `committed-fixture record --paxd PATH --out DIRECTORY` command.
- [`committed-fixture/vectors/`](committed-fixture/vectors/) — recorded JSON receipt vectors.
- [`committed-fixture/programs_abi.json`](committed-fixture/programs_abi.json) — ABI data used by the fixture's Programs vectors.

## Build and test

From the repository root, build the node with `make paxd-build`. This target builds the native libraries and runs the `chain.mk` Go build for `./daemon/paxd`.

Run the node-domain tests with `make test-paxd-domain`; the target covers `modules/lxphost`, node packages, EVM keeper code, and RPC code. Run the complete Go test target with `make paxeer-test`. The committed fixture is exercised by the Go tests in `daemon/committed-fixture` and records vectors with its `record` command.

## Configuration

`paxd init` creates and reads `config.toml`, `app.toml`, `client.toml`, and `genesis.json` under the node home. The node configuration also reads `PAX_LOG_LEVEL`, `PAX_LOG_FORMAT`, and `PAX_LOG_OUTPUT`. The committed-fixture test runner reads `COMMITTED_FIXTURE_EXEC_PAXD` when it executes the node command.

## Documentation

- [Sequencing and block finality](../docs/wiki/Sequencing.md)
- [Settlement evidence](../docs/wiki/SettlementEvidence.md)
- [Public node and hosted gateway JSON-RPC](../docs/wiki/PublicRpc.md)
- [Run a node](../docs/site/paxeer-docs/src/app/run-node/page.tsx)
- [Node configuration](../docs/site/paxeer-docs/src/app/configuration/page.tsx)
