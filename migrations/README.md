# Kernel SQLite migrations

This directory retains SQLite schemas for C projection tooling and historical import bookkeeping. Paxeer is one network: the kernel runs inside `paxd` and participates in block proposal. Committed chain state is authoritative, and a receipt is final when its block is committed. The kernel host loads and commits module state through the chain store in [`modules/lxphost/host.go`](../modules/lxphost/host.go); these SQLite files do not migrate that store.

The projection code can rebuild a local SQLite projection from compatible log records. Committed chain state remains authoritative for network recovery and upgrades.

| File | Creates | Current use |
| --- | --- | --- |
| [`0001_genesis_sections.sql`](0001_genesis_sections.sql) | `genesis_import_sections`, `genesis_import_asset_totals` and `genesis_historical_commitments` | Retained schema; no source file or build target loads it today |
| [`0001_projection.sql`](0001_projection.sql) | `projection_meta` (watermark and stale flag), `balances`, `balance_view`, `receipts`, `module_index` and `agent_queries` | `lxp_projection_open` and `lxp_projection_rebuild` in [`src/storage/lxp_projection.c`](../src/storage/lxp_projection.c) take the schema path explicitly; the storage tests use this file |
| [`0007_history_index.sql`](0007_history_index.sql) | `history_index_meta` and `history_records`, with sequence, batch, checkpoint, activity, transaction and idempotency indexes | Retained historical schema; the history-index implementation and its standalone test target are retired |

## Projection development checks

The retained [`projection`](../tests/storage/lxp_test_projection.c) and [`rebuild`](../tests/storage/lxp_test_rebuild.c) tests open `0001_projection.sql` by its repository-relative path. Their targets in the root [Makefile](../Makefile) are:

```bash
make test-projection
make test-rebuild
```

Run these from the repository root when working on the C projection tooling. They check local projection behavior, not committed network state.

## Related code

| Kind | Location |
| --- | --- |
| Genesis builder and import sources | [`cmd/layerx-genesis/`](../cmd/layerx-genesis/) |
| Ethereum and Solana source-chain migration | [`services/interop/crates/layerx-migrate`](../services/interop/crates/layerx-migrate/OPERATIONS.md) |
| Chain EVM module store migrations | [`modules/evm/migrations/`](../modules/evm/migrations/) |
