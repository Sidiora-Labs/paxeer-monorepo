# Paxeer daemon

This directory holds the `paxd` chain-node entry point and the committed receipt fixture. `paxd` runs the Cosmos SDK application and native EVM; the LayerX kernel inside every `paxd` node executes and orders each transaction and builds each block, and validators check and commit it; the fixture records and checks receipt vectors against a local node. A transaction is final when its block is committed.

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

## Compare transaction execution

Stop the node and wait for its process to exit. Copy the entire home, keep its
configured stores and symlinks inside the copy, and create its marker file:

```sh
cp -a /path/to/stopped-home /path/to/check-home
touch /path/to/check-home/applycheck-copy
paxd applycheck --home /path/to/check-home --tx-file transactions.txt --output report.json
```

`applycheck` requires a regular `applycheck-copy` file at the root and refuses
stores held by another process or an empty transaction list. It applies one
block at the next height in three forms: sequencer admission and execution on
arrival, a follower using the fast executor, and a follower using the ordinary
executor. Each form runs on an independent temporary copy of the same committed
state. It never commits or writes to the supplied home's committed state.

`--home` selects the copy. `--tx-file` reads one signed Ethereum transaction in
hex or one signed chain transaction in base64 per line. Repeatable `--tx` adds
transactions after the file's entries. `--block-time` accepts RFC3339 and
defaults to the last committed block's time plus one second. `--output` writes
the JSON report outside the copied home; without it the report goes to stdout.
On a home with active sequencing, copy the configured receipt and consensus
keys and set `sequencer.receipt-key-file` in the copy's `config/app.toml` to the
copied receipt key so the command can sign the block's sequence batch. For a
copy that has neither key file, pass `--stand-in-receipt-key`; the command uses
an in-memory key in the temporary forms and does not write it to the copy.

The report contains the chain ID, height, block time, receipt key mode
(`registered` or `stand-in`), committed height and hash,
and each form's transaction codes, codespaces, gas, events, EVM status, logs,
contract addresses, receipts and resulting application hash. Its comparison
names the first differing transaction and field; transaction positions start
at one and exclude the generated batch. A panic is reported with its message
and counts as a difference. Exit codes: `0` when all forms agree, `1` when they
differ, and `2` for refused input or an operational error.

## Documentation

- [Sequencing and block finality](../docs/wiki/Sequencing.md)
- [Settlement evidence](../docs/wiki/SettlementEvidence.md)
- [Public node and hosted gateway JSON-RPC](../docs/wiki/PublicRpc.md)
- [Run a node](../docs/site/paxeer-docs/src/app/run-node/page.tsx)
- [Node configuration](../docs/site/paxeer-docs/src/app/configuration/page.tsx)
