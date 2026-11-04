# Running Migrator CLI

This crate is `blockscout-db-migration`, the SeaORM migration helper used by
Rust services and tests. It contains one registered migration,
`m20220101_000001_create_table`, which imports the bundled
[src/migration.sql](src/migration.sql) through `from_sql`. The SQL is a
PostgreSQL schema snapshot; applying it initializes a database and does not
upgrade an arbitrary existing Blockscout schema. It is separate from the
Elixir backend's Ecto migration lifecycle.

The snapshot was dumped from PostgreSQL 17.2 and includes version-specific
settings such as `transaction_timeout` and extension creation. Use a compatible
server with the required extensions available and a role permitted to create
them.

[src/lib.rs](src/lib.rs) defines the registration and SQL execution path.
`from_sql` strips full-line SQL comments, splits statements, executes them
inside a transaction and commits after all statements succeed. A SQL failure
returns an error containing the failed query. Keep raw errors in private
operational logs when statements contain sensitive data.

## Supported commands

Run from `services/explorer/services/libs/blockscout-db/migration`. Set
`DATABASE_URL` to a PostgreSQL database intended for this schema; the crate
build enables the PostgreSQL driver. Supply the URL through the environment
without embedding credentials in commands committed to the repository.

Check migration status:

```sh
cargo run -- status
```

Apply pending migrations:

```sh
cargo run -- up
```

Running `cargo run` with no subcommand also applies pending migrations. A
bounded application is available with `cargo run -- up -n 1`; currently there
is only one migration. The CLI uses its migration history to track application.
Do not point it at an already populated explorer database just to mark the
snapshot as applied, or manually change history to bypass a schema conflict.

A new migration needs its implementation registered in `Migrator::migrations()`
in src/lib.rs. Adding a file alone does not make it execute. Review the actual
forward and recovery behavior before using new migrations with service data.

## Recovery and rollback

The registered migration implements `up` only. It has no `down` implementation,
so `down`, `refresh` and `reset` are not supported recovery procedures for
this migration. SeaORM offering those CLI commands does not supply an inverse
for the bundled SQL snapshot. The parent justfile's `migrate-down` recipe does
not change that limitation. `fresh` drops tables and recreates the schema; it
is destructive initialization, not a way to restore data.

For a database holding useful data, plan recovery before applying migrations:

1. Stop writers or arrange a consistent backup window. Record the application
   revision, migration status and target database identity privately.
2. Take a PostgreSQL backup including data and migration history and ensure it
   can be restored. Keep credentials and backup files outside source control.
3. If an apply fails, retain the command, exit status and private log. The SQL
   helper's transaction is rolled back on statement failure; inspect migration
   status and database state before deciding on a repair or restore.
4. Restore the backup into a separate empty database using compatible PostgreSQL
   tooling. Check schema, migration history and application reads there before
   directing services to it. Restore application and database revisions together
   when recovering from a successful schema change.

For a disposable development/test database, provision a new empty database,
set `DATABASE_URL` to it, and run `up`. Delete an old database only after
explicitly establishing that its data can be discarded. Reverting application
source alone does not reverse an applied database migration.

## Related documentation

The [Blockscout client](../../blockscout-client/README.md) documents local REST
producer/client checks. The
[account API](../../../../backend/apps/block_scout_web/API.md) describes
explorer account data. Paxeer is one network; sub-chains are tenant domains in
the database. Creating a tenant domain is not a reason to provision another
chain or rerun this whole-database schema snapshot against a populated database.
