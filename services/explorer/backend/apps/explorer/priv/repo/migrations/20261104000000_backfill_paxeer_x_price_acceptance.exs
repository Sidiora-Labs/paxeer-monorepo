defmodule Explorer.Repo.Migrations.BackfillPaxeerXPriceAcceptance do
  use Ecto.Migration

  def up do
    alter table(:paxeer_x_price_observations) do
      modify(:accepted, :boolean, null: true, default: nil)
    end

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

    execute("UPDATE paxeer_x_price_observations SET accepted = NULL")

    execute("""
    INSERT INTO paxeer_x_price_acceptance_backfill (block_hash, block_number)
    SELECT DISTINCT u.block_hash, u.block_number
    FROM paxeer_x_price_updates u
    JOIN paxeer_x_price_observations o ON o.update_hash = u.hash
    """)
  end

  def down do
    drop(table(:paxeer_x_price_acceptance_backfill))
  end
end
