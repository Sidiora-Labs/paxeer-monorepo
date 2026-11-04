defmodule Explorer.Repo.Migrations.InternalTransactionsBlockNumberBrinIndex do
  use Ecto.Migration

  @disable_ddl_transaction true
  @disable_migration_lock true

  @index_name "internal_transactions_block_number_brin_index"

  def up do
    execute(fn ->
      case index_validity() do
        [[false]] ->
          repo().query!("DROP INDEX CONCURRENTLY IF EXISTS #{@index_name}", [], timeout: :infinity)

        _ ->
          :ok
      end

      repo().query!(
        """
        CREATE INDEX CONCURRENTLY IF NOT EXISTS #{@index_name}
        ON internal_transactions
        USING brin (block_number) WITH (pages_per_range = 128, autosummarize = on)
        """,
        [],
        timeout: :infinity
      )

      unless index_validity() == [[true]] do
        raise Ecto.MigrationError,
          message: "#{@index_name} is not a valid index on internal_transactions; retry the migration"
      end
    end)
  end

  defp index_validity do
    %{rows: rows} =
      repo().query!(
        """
        SELECT i.indisvalid
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        WHERE c.relname = $1::text::name
          AND i.indrelid = 'internal_transactions'::regclass
        """,
        [@index_name]
      )

    rows
  end

  def down do
    execute("DROP INDEX CONCURRENTLY IF EXISTS #{@index_name}")
  end
end
