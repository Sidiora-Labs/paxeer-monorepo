use migration::{DbErr, Migrator, MigratorTrait, SchemaManager};
use sea_orm_migration::sea_orm::DatabaseConnection;

const RUNBOOK: &str = include_str!("../README.md");

#[async_std::test]
async fn migration_rollback_runbook_matches_registered_migration() {
    let migrations = Migrator::migrations();
    assert_eq!(migrations.len(), 1, "reassess recovery for new migrations");
    let initial = &migrations[0];
    assert_eq!(initial.name(), "m20220101_000001_create_table");

    // The production default down operation refuses before accessing a database.
    // A disconnected SeaORM connection proves that refusal needs no server.
    let connection = DatabaseConnection::Disconnected;
    let manager = SchemaManager::new(&connection);
    let error = initial
        .down(&manager)
        .await
        .expect_err("the initial snapshot must not advertise an implemented inverse");
    match error {
        DbErr::Migration(message) => {
            assert_eq!(message, "We Don't Do That Here");
            assert!(RUNBOOK.contains(&format!("`{message}`")));
        }
        other => panic!("expected an unsupported migration, got {other:?}"),
    }

    let runbook = RUNBOOK.split_whitespace().collect::<Vec<_>>().join(" ");
    assert!(runbook.contains(&format!(
        "The initial migration, `{}`, has no supported down operation.",
        initial.name()
    )));
    assert!(runbook.contains(
        "`down`, `refresh` and `reset` are not supported recovery procedures for this migration."
    ));
    assert!(runbook.contains(
        "`fresh` drops tables and recreates the schema; it is destructive initialization, not a way to restore data."
    ));
}

#[test]
fn migration_rollback_runbook_requires_backup_restore_or_disposable_recreation() {
    let runbook = RUNBOOK.split_whitespace().collect::<Vec<_>>().join(" ");
    for required in [
        "Take a PostgreSQL backup including data and migration history and ensure it can be restored.",
        "Restore the backup into a separate empty database using compatible PostgreSQL tooling.",
        "Check schema, migration history and application reads there before directing services to it.",
        "For a disposable development/test database, provision a new empty database, set `DATABASE_URL` to it, and run `up`.",
        "Delete an old database only after explicitly establishing that its data can be discarded.",
        "Reverting application source alone does not reverse an applied database migration.",
    ] {
        assert!(
            runbook.contains(required),
            "missing recovery guidance: {required}"
        );
    }
}
