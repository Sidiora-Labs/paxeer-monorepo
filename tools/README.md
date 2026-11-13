# Tools

Developer and operator tools for the repository. Selected subdirectories:

| Directory | What it is |
| --- | --- |
| [`chain/`](chain/cmd.go) | The `paxd tools` command group, wired into `paxd` by `daemon/paxd/cmd/root.go`; `chain/upgrade-replay` replays an upgrade plan against a copy of a chain's data directory |
| [`tx-scanner/`](tx-scanner/cmd/scan.go) | The `paxd tools scan-tx` command described below |
| [`codebase-map/`](codebase-map/README.md) | An offline, browsable map of the checkout |
| [`hyperfide/`](hyperfide/README.md) | Vendored Hyperfide code and tooling; [documentation](../docs/hyperfide/DOCUMENTATION.md) |
| `specgen/` | Compiles the kvx specs under `spec/` into generated rule files (`go run ./tools/specgen -root .`) |
| `bringup/` | Operator scripts for serving public names (`edge.sh`) and checking live systems (`check-live.sh`) |
| `flyci/` | The controller and the runner image of the CI runners hosted on Fly.io (each has its own `fly.toml`) |
| `build/` | Shared build pieces such as `sanitizers.mk`, included by the root `Makefile` |

## Transaction scanner

The scanner compares block transactions with indexed transaction hashes and
checks that each indexed transaction can be queried by hash. Use an archive
node with the required block history and gRPC transaction queries enabled.
The scanner records blocks with missing or mismatched indexed transactions;
reindexing is a separate offline command.

### Usage

The scanner reads blocks through the node's gRPC server. `--endpoint` defaults
to `127.0.0.1` and `--port` to `9090`. `--start-height` sets the first height;
zero or negative values start at height 1. `--state-dir` enables checkpoint
storage in the named directory. `--batch-size` (default `100`) and `--bps-limit`
(default `400` blocks per second) must be positive; batches are capped at the
rate limit.

```bash
paxd tools scan-tx --start-height 1 --state-dir ./scanner-state > scan.log &
```

A valid checkpoint takes precedence over `--start-height`. The scanner resumes
from its saved height, processes blocks below the latest height, and polls every
ten seconds when caught up. It continues until stopped.

### State format

The checkpoint, `tx-scanner-state.json`, contains:

```json
{
  "last_processed_height": 44394319,
  "blocks_missing_txs": [123400, 2124542]
}
```

`last_processed_height` is the next height to scan after a successful batch.
`blocks_missing_txs` contains the heights with missing or mismatched indexed
transactions. Checkpoint writes preserve the previous valid checkpoint as
`tx-scanner-state.json.previous` before replacing the current file.

### Reindex transactions

Stop the node before running the offline reindex command against its home:

```bash
paxd tendermint reindex-event --home "$NODE_HOME" --start-height 2124542 --end-height 2124543
```

Both height bounds are inclusive. The command rebuilds block and transaction
events for the selected range using the configured event sinks.
