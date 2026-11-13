# Heavy database index operations

[Explorer.Migrator.HeavyDbIndexOperation](../heavy_db_index_operation.ex)
provides a GenServer implementation for background index creation and removal.
It waits for declared migration dependencies and for other heavy migrations
on the same table, records migration progress in `MigrationStatus`, checks
the database operation, and updates the application cache on completion.
A completed migration exits normally.

## Implementations

Modules using this behaviour define the following callbacks:

| Callback | Purpose |
| --- | --- |
| `table_name/0` | Target database table |
| `operation_type/0` | Index operation kind |
| `index_name/0` | Index name |
| `dependent_from_migrations/0` | Required migration names |
| `db_index_operation/0` | Start the operation |
| `check_db_index_operation_progress/0` | Check whether the database is still executing it |
| `db_index_operation_status/0` | Inspect the resulting index state |
| `restart_db_index_operation/0` | Prepare an operation for restart |
| `running_other_heavy_migration_exists?/1` | Check for a conflicting migration |
| `update_cache/0` | Publish completion in the application cache |

The macro supplies `migration_name/0`, `migration_finished?/0` and the
GenServer lifecycle functions.

[CreateLogsBlockHashIndex](create_logs_block_hash_index.ex) is a concrete
implementation. It creates `logs_block_hash_index` on `logs(block_hash)`
after `DropLogsBlockNumberAscIndexAscIndex` completes. On completion it calls
`BackgroundMigrations.set_heavy_indexes_create_logs_block_hash_index_finished(true)`.

## Database helpers and scheduling

[Helper](helper.ex) builds and executes index SQL. `create_db_index/3` accepts
an index name, table atom and column list; its optional fourth argument
selects a unique index. `create_db_index/1` accepts a query directly.
`create_index_query_string/3` has the same optional uniqueness argument.
`safely_drop_db_index/1` removes an index, and `add_concurrently_flag?/0`
selects whether the SQL uses `CONCURRENTLY`.

`MIGRATION_HEAVY_INDEX_OPERATIONS_CHECK_INTERVAL` configures readiness and
operation-status checks in the backend runtime configuration; its default
is `10m`. The helper also uses ten minutes when no interval is configured.
A failed restart preparation is retried after ten seconds.
