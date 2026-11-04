defmodule Explorer.Repo.Migrations.HotTablesBlockNumberStatistics do
  use Ecto.Migration

  @statistics_target 1000

  @columns [
    {"blocks", "number"},
    {"blocks", "timestamp"},
    {"transactions", "block_number"},
    {"logs", "block_number"},
    {"token_transfers", "block_number"},
    {"internal_transactions", "block_number"},
    {"address_coin_balances", "block_number"}
  ]

  # Lock contention rolls back every statistics change. Retry the migration
  # after the busy transaction finishes, including when rolling back.
  def up do
    set_statistics(@statistics_target)
  end

  def down do
    set_statistics(-1)
  end

  defp set_statistics(target) do
    execute(fn ->
      repo().checkout(fn ->
        repo().query!("SET LOCAL lock_timeout = '5s'")

        Enum.each(@columns, fn {table, column} ->
          repo().query!(~s(ALTER TABLE #{table} ALTER COLUMN "#{column}" SET STATISTICS #{target}))
        end)
      end)
    end)
  end
end
