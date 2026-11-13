# Explorer database storage and replication

The Paxeer X Network explorer uses PostgreSQL 16 in its local deployment. This
guide describes storage migrations, the table keys used by the importers,
read-only API connections and database copying.

## 1. Hot-table keys and indexes

| table | primary key | foreign keys | access path on block number |
|---|---|---|---|
| `blocks` | `(hash)` | — | `blocks_number_index`; `one_consensus_block_at_height` unique on `number` where `consensus` |
| `transactions` | `(hash)` | `block_hash → blocks(hash)` cascade | `transactions_block_number_index`, `transactions_recent_collated_index (block_number DESC, index DESC)`, four `(address, block_number, index, inserted_at, hash)` composites |
| `logs` | `(transaction_hash, block_hash, index)` | `block_hash → blocks(hash)`, `transaction_hash → transactions(hash)` cascade | `logs_block_number_DESC__index_DESC_index` |
| `token_transfers` | `(transaction_hash, block_hash, log_index)` | `block_hash → blocks(hash)`, `transaction_hash → transactions(hash)` cascade | `token_transfers_block_number_DESC_log_index_DESC_index` |
| `internal_transactions` | `(block_number, transaction_index, index)` | `block_hash → blocks(hash)`, `transaction_hash → transactions(hash)` cascade | `internal_transactions_block_number_brin_index` and `internal_transactions_block_number_DESC_transaction_index_DESC_index_DESC_index`; address-specific indexes cover additional queries |
| `address_coin_balances` | `(address_hash, block_number)` | — | `address_coin_balances_block_number_index` |

The table describes the schema after the background index and key migrations
complete. Those migrations remove the ascending block-number indexes on `logs`
and `token_transfers`, remove the single-column token-transfer block-number index,
and replace the internal-transaction primary key `(block_hash, block_index)`.
A database still running these migrations can retain the earlier keys and indexes.

Nothing references `logs`, `token_transfers` or `internal_transactions` by foreign key.
`block_number` is `NOT NULL` on `address_coin_balances` and, after the background
key migration, `internal_transactions`. It remains nullable `integer` on
`transactions`, `logs` and `token_transfers`.

## 2. Storage migrations

Three migrations under `services/explorer/backend/apps/explorer/priv/repo/migrations/`
provide storage parameters, a BRIN index and statistics targets, each with a
`down` implementation. They use `SHARE UPDATE EXCLUSIVE` table locks and
concurrent index operations; run only one migrator against the database.

### `..._hot_tables_storage_parameters`

`ALTER TABLE ... SET (...)` on the six tables above, plus the TOAST relations of
`transactions`, `logs` and `internal_transactions`.

* `fillfactor` is 85 for `transactions`, 80 for `address_coin_balances`, and
  90 for `blocks`, `logs`, `token_transfers` and `internal_transactions`.
* Vacuum, insert-vacuum and analyze scale factors are zero. Absolute thresholds
  vary by table; the migration lists each value.
* Heap vacuum cost limits are 4000 or 6000, depending on the table, and
  `vacuum_truncate` is false on all six heaps.
* The three TOAST relations receive vacuum and insert-vacuum thresholds,
  zero scale factors and a vacuum cost limit of 4000 or 6000.

### `..._internal_transactions_block_number_brin_index`

The migration creates a BRIN access path on `internal_transactions.block_number`
for block-range queries, including reorganization cleanup, the internal-transaction
delete queue and trace refetching.

The index uses `pages_per_range = 128` and `autosummarize = on`.

The index is built `CONCURRENTLY` outside a migration transaction, avoiding a
writer-blocking lock for the full duration of the index build. If the build is interrupted,
rerunning the migration drops an invalid index with this name before rebuilding
it and checks that the resulting index is valid.

This migration adds no BRIN index to `logs`, `token_transfers`, `transactions`
or `blocks`.

### `..._hot_tables_block_number_statistics`

The migration applies `ALTER COLUMN ... SET STATISTICS 1000` to the six
block-number columns and `blocks.timestamp`. It changes the statistics targets;
PostgreSQL collects statistics during `ANALYZE`.

### Rolling back

**Use a tuning-only migration directory, not a rollback count or a historical target.**
On a fully migrated database the newest migrations include application and pricefeed schema;
rolling back three steps in the normal directory can select application schema
instead of these storage migrations.
`--to` and `--to-exclusive` against that directory also traverse newer migrations and can
delete indexed application data. Neither is a tuning rollback.

This procedure uses the three existing `down` implementations, in this order:

| version | effect |
|---|---|
| `20260301000200` | Set the seven column statistics targets listed in the migration to `-1`. |
| `20260301000100` | Drop only `internal_transactions_block_number_brin_index`, concurrently. |
| `20260301000000` | Reset only the named heap and TOAST storage parameters in the migration. |

#### Prerequisites and before-state

1. Obtain approval for this maintenance operation, a recoverable backup, and a successful
   rehearsal on an isolated restored database with the same migration state.
   Verify the selected migration versions and object changes there.
2. Use the matching, reviewed source release and installed dependencies from
   `services/explorer/backend`, with `MIX_ENV=prod` and the intended primary database
   configuration already set. Confirm the effective `Explorer.Repo` connection, database,
   role and search path without printing credentials. The commands below assume the six
   tables, the index and the default `schema_migrations` table are in `public`, with raw SQL
   resolving those table names to `public`. Stop for a separately reviewed procedure if
   using a custom schema, migration source/repo, or an incompatible schema version; a
   `--prefix` alone does not schema-qualify the migrations' raw SQL.
3. Stop deployment/startup auto-migrators and exclude all other migrators and schema changes
   for the entire operation, including verification. The BRIN `down` disables both the DDL
   transaction and migration lock. Have the DBA configure and verify a short `lock_timeout`
   (for example, 5 seconds) on the actual migrator connections before proceeding; do not
   assume a client-side setting in a separate SQL session applies to them.
4. Capture the complete migration status before the operation, using the same configuration:

   ```sh
   mix ecto.migrations -r Explorer.Repo --prefix public
   ```

   **All three storage migration versions above must be `up` on the initial attempt.** Later versions may
   also be `up`; they must remain unchanged. Stop on missing versions, unexplained migration
   history, or a partially applied tuning migration. A known interruption of this procedure
   is handled below, not by guessing a new step count.
5. Capture the six tables' `pg_class.reloptions`, the TOAST relations' `reloptions` for
   `transactions`, `logs` and `internal_transactions`, the seven columns' `pg_attribute.attstattarget`,
   and the BRIN index definition/validity. Compare them with the three reviewed migration
   files. Confirm that no subsequent migration or manual tuning repurposed that index,
   changed the affected objects, or introduced a dependency requiring it. These `down`
   functions reset to defaults, **not to captured previous custom values**. If any affected
   setting must retain a later/custom value, stop and obtain a setting-specific plan instead.

#### Execute only the three tuning downs

From `services/explorer/backend`, run this in Bash only after the prerequisites pass. Copy
the three literal filenames into a new private directory; do not copy the whole migrations
tree, use a filename glob, or append another `--migrations-path`.

```bash
(
  set -eu
  test "${MIX_ENV:-}" = prod
  source_migrations="$PWD/apps/explorer/priv/repo/migrations"
  tuning_migrations=$(mktemp -d "${TMPDIR:-/tmp}/explorer-tuning-rollback.XXXXXXXX")
  trap 'rm -rf -- "$tuning_migrations"' EXIT

  for migration in \
    20260301000000_hot_tables_storage_parameters.exs \
    20260301000100_internal_transactions_block_number_brin_index.exs \
    20260301000200_hot_tables_block_number_statistics.exs
  do
    cp -- "$source_migrations/$migration" "$tuning_migrations/$migration"
  done

  mix ecto.rollback -r Explorer.Repo --prefix public \
    --migrations-path "$tuning_migrations" --all
)
```

The locked Ecto SQL version is `3.13.4`. Its
[`ensure_migrations_paths/2`](https://github.com/elixir-ecto/ecto_sql/blob/ed07cf27a49109be3b2558574ff03d9499f79557/lib/mix/ecto_sql.ex#L8-L11)
replaces the default path when an explicit path is supplied. Its
[`pending_in_direction/3`](https://github.com/elixir-ecto/ecto_sql/blob/ed07cf27a49109be3b2558574ff03d9499f79557/lib/ecto/migrator.ex#L644-L674)
selects only applied versions found in that path, in descending order for `down`.
Thus `--all` here means only the three copied tuning migrations, even when newer application
migrations are applied. Recheck this behavior before using a different dependency version.

#### Verify, handle interruption, and keep the rollback in effect

Run the same full `mix ecto.migrations -r Explorer.Repo --prefix public` status command again
and compare it with the before-state. Exactly the three storage migration versions must change from
`up` to `down`; every other version must retain its status. Verify the database objects too:

* The seven statistics targets are `-1` (`default_statistics_target`).
* `public.internal_transactions_block_number_brin_index` is absent.
* The specific heap and TOAST options named by `20260301000000` are absent from their
  respective `reloptions`. Unrelated options must remain unchanged; an entire `reloptions`
  value need **not** be null. Resetting options does not restore historical custom values or
  rewrite existing heap pages.

Keep the migration output and before/after evidence in the approved operations record. The
three migrations are not one atomic rollback: earlier downs can commit before a later one
fails, and the concurrent index drop can complete before its migration-history update.
On any failure, keep auto-migrators disabled, inspect both the ledger and affected objects,
and resolve the failure before continuing. Only when the partial state is explained by this
attempt and the same prerequisites still hold may the operator repeat the **same isolated
directory procedure**; Ecto skips versions already down, and the BRIN drop uses `IF EXISTS`.
Do not manually edit the migration ledger or switch to an unscoped rollback to recover.

Successful downs remove these three versions from migration history. A subsequent ordinary
`mix ecto.migrate` can therefore **reapply the tuning**, even though newer versions are
already up. Keep automatic migration/deployment disabled until an approved release plan
either intentionally reapplies the tuning or preserves the rollback. This procedure does
not change deployment defaults or install a new automatically applied migration.

### Applying to a live database

From `services/explorer/backend`, with the intended production database configured:

```sh
MIX_ENV=prod mix ecto.migrate
```

The storage-parameter and statistics migrations set `SET LOCAL lock_timeout = '5s'`
on their own Postgrex transaction connections. A lock timeout aborts the transaction.
The BRIN migration runs outside a DDL transaction and disables the migration lock;
configure any required session timeout on its actual migrator connection and
exclude other migrators for the entire operation.

## 3. Table keys and range partitioning

Moving `logs` or `token_transfers` to `PARTITION BY RANGE (block_number)` is not possible without changing
application code: the ORM and the existing unique constraints do not allow it. The blockers are
structural.

**A. The primary keys do not contain the partition key.** PostgreSQL requires every unique
constraint on a partitioned table to include all partition key columns, so PostgreSQL refuses
it against this schema:

```text
CREATE TABLE logs_partitioned (LIKE logs INCLUDING ALL) PARTITION BY RANGE (block_number);
ERROR:  unique constraint on partitioned table must include all partitioning columns
DETAIL:  PRIMARY KEY constraint on table "logs_partitioned" lacks column "block_number"
         which is part of the partition key.
```

`token_transfers` fails identically. So `block_number` has to join both primary keys.

**B. The bulk importers pin the conflict target in code.**
`Explorer.Chain.Import.Runner.Logs` passes
`conflict_target: [:transaction_hash, :index, :block_hash]` and
`Explorer.Chain.Import.Runner.TokenTransfers` passes
`[:transaction_hash, :log_index, :block_hash]`. PostgreSQL matches `ON CONFLICT (cols)` to a
unique index on exactly those columns, and after (A) no such index can exist. Against a
partitioned table whose key is widened as (A) demands, PostgreSQL answers:

```text
INSERT INTO logs_part VALUES (...) ON CONFLICT (transaction_hash, index, block_hash) DO NOTHING;
ERROR:  there is no unique or exclusion constraint matching the ON CONFLICT specification
```

Every import batch would fail. Both runners also carry a second, different conflict target for
the `optimism`/`celo` chain identity, so there are four call sites to change, not two — and
changing them is an application change.

**C. The Ecto schemas declare the composite key.** `Explorer.Chain.Log` and
`Explorer.Chain.TokenTransfer` mark `index`/`log_index`, `block_hash` and `transaction_hash`
as `primary_key: true` under `@primary_key false`. That key drives `Repo.get`, changeset
uniqueness, association loading and the `returning: true` round-trip the importers depend on.
Widening the database key without widening the schema leaves the two disagreeing; widening the
schema changes the public shape of those structs and is, again, application code.

**D. The partition key is nullable today.** `logs.block_number` and
`token_transfers.block_number` both allow NULL. Putting the column in the primary key makes it
`NOT NULL` implicitly, which needs a validated backfill over the existing rows and a guarantee
that no importer path ever writes NULL. Without that, rows are simply rejected:

```text
ERROR:  no partition of relation "logs_part" found for row
DETAIL:  Partition key of the failing row contains (block_number) = (null).
```

The alternative — a permanent `DEFAULT` partition — silently collects those rows and defeats
partition pruning for every query that does not constrain `block_number`.

**E. The deployment does not schedule partition creation.** The current migrations
create ordinary tables. Range partitioning would also need partition lifecycle
management as block heights increase.

The storage migrations in section 2 do not create range partitions. The current
schemas and importers use the keys described above.

## 4. Read-only API endpoint

`DATABASE_READ_ONLY_API_URL` selects the API read connection through these components:

* `Explorer.Repo.ConfigHelper.get_api_db_url/0` returns it, falling back to `DATABASE_URL`.
* `config/runtime/prod.exs` feeds that into `Explorer.Repo.Replica1` together with
  `POOL_SIZE_API`; `config/runtime/dev.exs` does the same and drops the primary pool default
  from 40 to 30 when the variable is present.
* `Explorer.Repo.Replica1` is declared `read_only: true`, so Ecto generates no write callbacks
  on it, and it is absent from `ConfigHelper.repos/0`, so migrations never target it.
* `Explorer.Chain.select_repo/1` returns `Explorer.Repo.replica/0` for `api?: true`, and the
  `Repo.replica()` call sites in the Etherscan-compatible and API v2 read paths resolve to the
  same module.
* `Explorer.Utility.ReplicaAccessibilityManager` starts only when the variable is set. Every
  ten seconds it reads `pg_is_in_recovery()` and the replay lag; if a recovering replica's lag exceeds
  `REPLICA_MAX_LAG` (`config/runtime.exs`, default five minutes) it sets
  `:replica_inaccessible?`, which makes `Explorer.Repo.replica/0` fall back to the primary
  until the replica catches up. A query failure also selects the primary.

Size `POOL_SIZE_API` against the replica's own `max_connections` rather than the primary's —
the two pools are independent.

## 5. Seeding a fresh database from an older one

`../tools/copy-blockscout-11-to-10.sh` copies the chain tables of a Blockscout 11.x database
into a freshly migrated 10.2.6 one. It reads both connection strings from `SRC_DATABASE_URL`
and `DST_DATABASE_URL` and from nowhere else, and it prints neither.

### The two cuts

Before the first table is read the run pins two cuts, and every table is copied and counted
under one of them.

The tool retains one repeatable-read source snapshot throughout copying and
verification. Its destination session holds an advisory lock across staging,
copying and progress commits. Losing either database session stops the run.

The first cut is a **block ceiling**: the highest block number the source has marked `consensus`
at that moment. Every table that carries a block number is bounded by it on both sides of the
comparison.

The second is an **insert cut**: the source's own clock, read in UTC at the same moment. It
bounds the tables that carry no block number at all — `addresses`, `tokens`,
`contract_methods` — through the `inserted_at` column Blockscout stamps on every row and the
copy carries across unchanged. Both sides are compared as epoch seconds, so neither session's
`TimeZone` can shift the comparison.

The retained snapshot supplies the same source rows for copying and verification,
including when the source continues indexing or backfilling below a cut. Both
cuts are printed at the start of the run and repeated in the summary. A table
with neither a block number nor a timestamp-compatible `inserted_at` on both
sides is copied and compared whole within the retained snapshot; the summary
names it as unbounded.

A row whose block number is null — a pending transaction, a log the indexer has not yet
assigned — is below no ceiling and is not copied. The same predicate excludes it from both
counts, so it is not a mismatch either. Re-indexing writes those rows in the destination.

### Resuming

A table that carries a block number on both sides is copied in block-aligned batches;
`BLOCK_BATCH` sets how many block numbers one batch spans and defaults to 100000. A batch is
one statement, so it is either wholly inserted or not inserted at all and no block is ever
half copied. That statement also advances the table's row in
`public.paxeer_x_copy_progress` in the destination, which the tool creates:

| column | meaning |
|---|---|
| `table_name` | the table the row is about |
| `ceiling` | the ceiling the last run used |
| `highest_key` | the highest block number the tool has copied, including ranges that held no row |
| `rows_copied` | rows inserted across every run |
| `updated_at` | when that last changed |
| `snapshot_id` | retained boundary snapshot identifier |
| `source_fingerprint` | selected column list, row count and SHA-256 of projected source content |

A later run continues each table above the greater of that record and the table's own highest
copied block, and inserts with `ON CONFLICT DO NOTHING`, so a row that is already there keeps
the values it has and a rerun with nothing to do writes nothing. An interrupted copy is
resumed by running the same command again. The tool retains the original cuts,
source identity and table list in `public.paxeer_x_copy_run` for an interrupted
boundary. A completed boundary permits a new cut. On resume, the new
source snapshot must match each retained table fingerprint before that table
can copy more rows. Changes to the source connection identity, table list or
previously selected source content are refused.

Tables with no block number on both sides — `addresses`, `tokens`, `contract_methods` and the
rest — have no key to resume from. They are copied whole up to the insert cut under the same
conflict handling, so a rerun still writes nothing.

Rows land in an unlogged staging table (`public.paxeer_x_copy_stage`) first, because `COPY`
itself has no conflict handling; the tool drops it when the table is done. The
staging table, progress table and `public.paxeer_x_copy_run` can be removed after
seeding is complete and no further resume is needed.

### Comparing copied data

Verification compares row counts and SHA-256 fingerprints of sorted projected
rows from the retained source snapshot and destination. The projection includes
the copied keys and content, using the destination column names and any trace
address derivation. Equal row counts alone do not establish a match.

A destination with fewer rows is rescanned across its whole selected range,
using `ON CONFLICT DO NOTHING`, then compared again. Existing values remain
unchanged. Missing, extra or different copied content returns exit code 3;
the tool neither overwrites existing rows nor deletes them to make a match.

### The destination guard

A destination table that holds rows the tool has no progress record for is refused, and the
run stops naming that table; `ALLOW_NONEMPTY_DESTINATION=1` appends to it anyway. Resuming the
tool's own interrupted run needs no flag — those rows are recorded — so the guard still only
stands between the copy and data that came from somewhere else.

### Columns

The two schemas are intersected by column name at run time rather than mapped in advance. A
source column the destination has no name for is reported as it is skipped, and the summary
repeats them by name for `transactions` and `internal_transactions`. A destination column the
source cannot answer for is left at its default.

`internal_transactions.trace_address` is the exception. The 10.2.6 schema creates it `NOT
NULL` while an 11.x row may carry no trace address at all, so copying the value across as it
stands puts a null into a column that forbids it and the table fails as a whole. The copy
derives it instead from the source's own representation of where the row sits: the root call
of a transaction, at index 0, gets the empty array Blockscout writes for it, and every other
row gets its index as a one-element array. The source carries no parent link, so the derived
value is the row's flat position within its transaction rather than a reconstruction of the
call tree; it is unique per transaction and orders the rows the way the source orders them. A
source row that does carry a trace address keeps the one it has.

### Verifying

After copying, the tool compares both sides under each table's cut, rescans
short tables and prints a summary. It exits `0` when all processed tables match,
`1` on a usage or initial precondition refusal, `2` when copying or comparison
cannot finish, and `3` when copied keys or content differ. The summary names
each mismatching table and both row counts, including equal counts when content
differs.

`DRY_RUN=1` pins and prints both cuts, plans each table and reports the column differences
without writing anything at all, including the progress table.

### Proving it

`../tools/tests/copy-blockscout-test.sh` starts two disposable PostgreSQL 16 containers on
ephemeral loopback ports, creates a source schema shaped like 11.x and a destination schema
shaped like 10.2.6, seeds synthetic rows, and asserts the tool's exit code and the resulting
row counts across a dry run, a full copy, a rerun, a copy interrupted part way through a
table, its resume, a source that grows in both a block table and a block-less one after the
cuts are pinned, the non-empty destination guard, a source row that appears under the ceiling
after the copy has passed it, and a destination holding a row the source does not. It removes
both containers on exit and needs Bash, Python 3, `docker` and `psql` on the path.
It also covers equal-count content differences, destination lock ownership and
source-content checks when resuming an interrupted boundary.
