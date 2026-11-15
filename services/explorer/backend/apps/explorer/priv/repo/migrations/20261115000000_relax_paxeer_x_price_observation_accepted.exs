defmodule Explorer.Repo.Migrations.RelaxPaxeerXPriceObservationAccepted do
  use Ecto.Migration

  @index "paxeer_x_price_acceptance_backfill_next_job_index"

  def up do
    execute("""
    ALTER TABLE paxeer_x_price_observations
      ALTER COLUMN accepted DROP NOT NULL,
      ALTER COLUMN accepted DROP DEFAULT
    """)

    execute("""
    CREATE INDEX IF NOT EXISTS #{@index}
    ON paxeer_x_price_acceptance_backfill ((error IS NOT NULL), block_number, block_hash)
    WHERE completed = false
    """)
  end

  def down do
    execute("DROP INDEX IF EXISTS #{@index}")
  end
end
