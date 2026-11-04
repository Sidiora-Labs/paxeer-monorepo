defmodule Explorer.Repo.Migrations.BackfillPaxeerXPriceAcceptance do
  use Ecto.Migration

  # This migration only installs the durable replay queue and enqueues the blocks
  # that carry price updates. It never rewrites paxeer_x_price_observations and
  # never takes an exclusive lock on it: the acceptance replay in
  # scripts/price_acceptance_backfill.exs resets one block's observations to
  # unknown inside the replay transaction that starts that block, so the reset is
  # bounded to a single block and recovers with the replay cursor after a restart.
  # The accepted column is nullable since 20261028000000; no column change is needed.
  def up do
    create table(:paxeer_x_price_acceptance_backfill, primary_key: false) do
      add(:block_hash, references(:blocks, column: :hash, type: :bytea, on_delete: :delete_all), primary_key: true)
      add(:block_number, :bigint, null: false)
      add(:transactions, :map)
      add(:results, :map)
      add(:through_index, :integer, null: false, default: -1)
      add(:through_market_id, :integer, null: false, default: -1)
      add(:completed, :boolean, null: false, default: false)
      add(:error, :text)
    end

    create(index(:paxeer_x_price_acceptance_backfill, [:block_number], where: "completed = false"))

    execute("""
    INSERT INTO paxeer_x_price_acceptance_backfill (block_hash, block_number)
    SELECT DISTINCT u.block_hash, u.block_number
    FROM paxeer_x_price_updates u
    JOIN blocks b ON b.hash = u.block_hash
    """)
  end

  def down do
    drop(table(:paxeer_x_price_acceptance_backfill))
  end
end
