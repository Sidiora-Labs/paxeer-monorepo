Code.require_file(
  Path.expand("../../../priv/repo/migrations/20261022000002_tune_paxeer_x_price_feed_tables.exs", __DIR__)
)

defmodule Explorer.Repo.TunePaxeerXPriceFeedTablesTest do
  use ExUnit.Case, async: false

  alias Explorer.Repo
  alias Explorer.Repo.Migrations.TunePaxeerXPriceFeedTables

  @version 20_261_022_000_002

  test "tune_price_feed_migration_reruns_after_partial_failure" do
    tune_price_feed_migration_reruns_after_partial_failure()
  end

  def tune_price_feed_migration_reruns_after_partial_failure do
    assert :ok = Ecto.Migrator.down(Repo, @version, TunePaxeerXPriceFeedTables, log: false)

    assert %{rows: [[true]]} =
             Repo.query!("""
             SELECT indisvalid FROM pg_index
             WHERE indexrelid = 'paxeer_x_price_updates_feeder_hash_index'::regclass
             """)

    feeder_index =
      Ecto.Migration.index(:paxeer_x_price_updates, [:feeder_hash, "block_number DESC", "index DESC"])

    index_name = to_string(feeder_index.name)
    index_regclass = ~s("#{index_name}")

    Repo.query!("""
    CREATE INDEX "#{index_name}" ON paxeer_x_price_updates (feeder_hash, block_number DESC, "index" DESC)
    """)

    assert %{num_rows: 1} =
             Repo.query!("UPDATE pg_index SET indisvalid = false WHERE indexrelid = to_regclass($1)", [index_regclass])

    Repo.query!("""
    ALTER TABLE paxeer_x_price_feed_scanned_blocks
      ADD CONSTRAINT paxeer_x_price_feed_scanned_blocks_status_check
      CHECK (status IN ('scanned', 'unavailable', 'failed')) NOT VALID
    """)

    assert %{rows: [[false]]} =
             Repo.query!("SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass($1)", [index_regclass])

    assert %{rows: [[false]]} = constraint_validation()

    on_exit(fn ->
      Repo.query!("DROP EVENT TRIGGER IF EXISTS tune_price_feed_lock_timeout")
      Repo.query!("DROP FUNCTION IF EXISTS tune_price_feed_capture_lock_timeout()")
      Repo.query!("DROP TABLE IF EXISTS tune_price_feed_lock_timeouts")
    end)

    Repo.query!("CREATE TABLE tune_price_feed_lock_timeouts (value text NOT NULL)")

    Repo.query!("""
    CREATE FUNCTION tune_price_feed_capture_lock_timeout() RETURNS event_trigger
    LANGUAGE plpgsql AS $$
    DECLARE command record;
    BEGIN
      FOR command IN SELECT * FROM pg_event_trigger_ddl_commands() LOOP
        IF command.objid IN (
          'paxeer_x_price_updates'::regclass,
          'paxeer_x_price_observations'::regclass,
          'paxeer_x_price_feed_scanned_blocks'::regclass
        ) THEN
          INSERT INTO tune_price_feed_lock_timeouts VALUES (current_setting('lock_timeout'));
        END IF;
      END LOOP;
    END;
    $$
    """)

    Repo.query!("""
    CREATE EVENT TRIGGER tune_price_feed_lock_timeout ON ddl_command_end
      WHEN TAG IN ('ALTER TABLE') EXECUTE FUNCTION tune_price_feed_capture_lock_timeout()
    """)

    assert :ok = Ecto.Migrator.up(Repo, @version, TunePaxeerXPriceFeedTables, log: false)

    assert %{rows: [[true]]} =
             Repo.query!("SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass($1)", [index_regclass])

    assert %{rows: [[nil]]} =
             Repo.query!("SELECT to_regclass('paxeer_x_price_updates_feeder_hash_index')")

    assert %{rows: [[true]]} = constraint_validation()
    assert %{rows: [["5s"], ["5s"], ["5s"], ["5s"]]} = Repo.query!("SELECT value FROM tune_price_feed_lock_timeouts")
  end

  defp constraint_validation do
    Repo.query!("""
    SELECT convalidated FROM pg_constraint
    WHERE conname = 'paxeer_x_price_feed_scanned_blocks_status_check'
      AND conrelid = 'paxeer_x_price_feed_scanned_blocks'::regclass
    """)
  end
end
