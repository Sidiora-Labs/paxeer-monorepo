# PostgreSQL storage

Source paths and pnpm commands below are relative to `tools/hyperfide`.

Set `HYPERFIDE_DATABASE_URL` before starting the server. A `postgres://` or
`postgresql://` URL selects PostgreSQL through Drizzle and node-postgres. An unset
value retains the configured SQLite data-directory path; a file path or `sqlite:`
URL selects SQLite through better-sqlite3. PostgreSQL connections use the
`platform` schema. Socket parameters in the URL are supported.

The main data API and transactions return promises on both engines. Await all
reads, writes, migrations and connection closure. `afterCommit` runs callbacks
only after a successful commit; rolled-back savepoint callbacks are discarded.
PostgreSQL transactions use an advisory writer lock to preserve atomic sequence
allocation and the read-modify-write ordering provided by SQLite transactions.
PostgreSQL tracks event changes by connection in the internal
`platform.__hyperfide_changes` table, so timeline caches detect writes from other
connections without discarding incremental work after their own appends.

The SQLite migration history remains in `core/runtime-db/drizzle`. PostgreSQL has a
baseline and append-only journal in `core/runtime-db/drizzle-postgres`. Generate both
with `pnpm -C core/runtime-db db:generate`; review both outputs together.
`dialect-parity.test.ts` compares every declared table and column and checks the
migrated database against that declaration on both engines. Existing SQLite data
is not copied automatically to PostgreSQL.

`pnpm -C core/runtime-db db:studio` selects the configured dialect. The reset
command resets the configured database after its existing confirmation, including
the PostgreSQL `platform` schema. Local directory cleanup still follows the reset
command's selected targets.

The SDK exports the `PluginAsyncDatabase` interface for this storage handle.

Plugins can use `await storage.asyncDatabase()` on either engine. Its `query`,
`execute`, `transaction`, `migrate` and `close` methods return promises. Supply
native placeholders (`?` on SQLite, `$1` on PostgreSQL), and select dialect-specific
SQL through the handle's `dialect` property where necessary. PostgreSQL plugin
schemas are isolated by a hash of the plugin id. Ordered migrations are
transactional, append-only and checked by statement hash.

The synchronous `storage.database()` and `storage.migrate()` API uses SQLite.
A plugin using that API retains its SQLite database when the server database
configuration changes.

SQLite PRAGMA/WAL diagnostics and query plans apply only to SQLite.

Server-move archives currently contain physical SQLite backups. That archive
transfer workflow and its tests remain SQLite-specific; PostgreSQL deployments
use database-native backup and relocation tools.
