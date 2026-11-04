defmodule Explorer.Repair.Repair2233278Test do
  @moduledoc """
  Task 22.33.278, audit 22.30.132: migration-cache-before-commit is proved by
  "commit rejection preserves the old importer layout across cache restart" and
  "committed migration reconstructs the new importer layout after restart".
  These tests use actual PostgreSQL commits, a deferred constraint failure,
  the committed Geth callTracer/struct logger recording, HTTP, production trace
  decoding, schema casting, and the internal-transactions import Multi.

  Baseline: no prior gate ledger or repair implementation was present. No tests
  were run on the unchanged tree: the ordered gate instructions require full
  implementation before formatting, hf-build and hf-done. Qualification evidence
  (including the verified revision) is recorded by those gates.
  No out-of-path wiring changes are required.
  Gotcha: a normal Sandbox checkout wraps migration transactions, so it cannot
  exercise a real COMMIT failure; these serial tests check out sandbox: false
  and restore durable schema/status and the cache in on_exit.
  Qualification attempt 1 (9229ddfefa4318863a78887b20f8a66cf4c83cbc) failed
  both cases in setup: get_all invoked the unrelated, missing fallback for
  backfill_call_type_enum_finished. Snapshot raw cache entries instead.
  """

  use ExUnit.Case, async: false

  import Ecto.Query
  import Explorer.Factory

  alias Ecto.Adapters.SQL.Sandbox
  alias Ecto.{Changeset, Multi}
  alias EthereumJSONRPC.Geth
  alias Explorer.Chain.{Hash, Import, InternalTransaction}
  alias Explorer.Chain.Cache.BackgroundMigrations
  alias Explorer.Chain.Import.Runner.InternalTransactions
  alias Explorer.Migrator.HeavyDbIndexOperation.UpdateInternalTransactionsPrimaryKey, as: Migration

  alias Explorer.Migrator.HeavyDbIndexOperation.CreateInternalTransactionsBlockNumberTransactionIndexIndexUniqueIndex,
    as: NewIndex

  alias Explorer.Migrator.MigrationStatus
  alias Explorer.Repo
  alias Explorer.Utility.InternalTransactionHelper

  @cache_key :heavy_indexes_update_internal_transactions_primary_key_finished
  @old_key ["block_hash", "block_index"]
  @new_key ["block_number", "transaction_index", "index"]
  @recordings Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace", __DIR__)

  setup do
    :ok = Sandbox.checkout(Repo, sandbox: false)
    :ok = Sandbox.mode(Repo, {:shared, self()})
    cache = Map.new(BackgroundMigrations.cache_keys(), &{&1, ConCache.get(BackgroundMigrations.cache_name(), &1)})
    geth_config = Application.fetch_env!(:ethereum_jsonrpc, Geth)

    on_exit(fn ->
      Sandbox.unboxed_run(Repo, fn ->
        drop_commit_rejection()

        if primary_key_columns() == @old_key do
          assert :ok = Migration.db_index_operation()
        end

        assert primary_key_columns() == @new_key
        assert {:ok, _} = MigrationStatus.set_status(Migration.migration_name(), "completed")
      end)

      Enum.each(cache, fn {key, value} -> BackgroundMigrations.set(key, value) end)
      Application.put_env(:ethereum_jsonrpc, Geth, geth_config)
    end)

    Application.put_env(
      :ethereum_jsonrpc,
      Geth,
      Keyword.merge(geth_config, tracer: "call_tracer", debug_trace_timeout: "5s", block_traceable?: true)
    )

    assert :ok = Migration.restart_db_index_operation()

    Repo.query!("""
    CREATE UNIQUE INDEX #{NewIndex.index_name()}
    ON internal_transactions (block_number, transaction_index, "index")
    """)

    assert {:ok, _} = MigrationStatus.set_status(Migration.migration_name(), "started")
    assert :ok = BackgroundMigrations.set(@cache_key, false)
    assert primary_key_columns() == @old_key
    refute InternalTransactionHelper.primary_key_updated?()
    :ok
  end

  test "commit rejection preserves the old importer layout across cache restart" do
    install_commit_rejection()

    error = assert_raise Postgrex.Error, fn -> Migration.db_index_operation() end
    assert error.postgres.code == :check_violation
    assert error.postgres.message == "repair_22_33_278: completed DDL rejected at commit"
    assert MigrationStatus.get_status(Migration.migration_name()) == "started"
    assert primary_key_columns() == @old_key
    assert Migration.db_index_operation_status() == :not_initialized
    refute InternalTransactionHelper.primary_key_updated?()
    assert_import_layout(false)

    restart_cache()
    assert ConCache.get(BackgroundMigrations.cache_name(), @cache_key) == nil
    refute InternalTransactionHelper.primary_key_updated?()
    assert ConCache.get(BackgroundMigrations.cache_name(), @cache_key) == false
    assert_import_layout(false)
  end

  test "committed migration reconstructs the new importer layout after restart" do
    assert :ok = Migration.db_index_operation()
    assert primary_key_columns() == @new_key
    assert Migration.db_index_operation_status() == :completed
    assert InternalTransactionHelper.primary_key_updated?()

    assert %MigrationStatus{status: "completed", updated_at: updated_at} =
             MigrationStatus.fetch(Migration.migration_name())

    assert_import_layout(true)

    restart_cache()
    assert ConCache.get(BackgroundMigrations.cache_name(), @cache_key) == nil
    assert InternalTransactionHelper.primary_key_updated?()
    assert ConCache.get(BackgroundMigrations.cache_name(), @cache_key) == true
    assert_import_layout(true)

    restart_cache()
    Process.flag(:trap_exit, true)
    assert {:ok, pid} = Migration.start_link([])
    assert_receive {:EXIT, ^pid, :normal}, 5_000
    assert ConCache.get(BackgroundMigrations.cache_name(), @cache_key) == true

    assert %MigrationStatus{status: "completed", updated_at: ^updated_at} =
             MigrationStatus.fetch(Migration.migration_name())

    assert primary_key_columns() == @new_key
    assert_import_layout(true)
  end

  defp install_commit_rejection do
    Repo.query!("""
    CREATE FUNCTION repair_22_33_278_reject_commit() RETURNS trigger AS $$
    BEGIN
      IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'internal_transactions'
          AND column_name = 'block_index' AND is_nullable = 'YES'
      ) THEN
        RAISE EXCEPTION 'repair_22_33_278: DDL did not finish';
      END IF;
      RAISE EXCEPTION 'repair_22_33_278: completed DDL rejected at commit'
        USING ERRCODE = '23514';
    END;
    $$ LANGUAGE plpgsql
    """)

    Repo.query!("""
    CREATE CONSTRAINT TRIGGER repair_22_33_278_commit
    AFTER INSERT OR UPDATE ON migrations_status
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
    WHEN (NEW.migration_name = '#{Migration.migration_name()}' AND NEW.status = 'completed')
    EXECUTE FUNCTION repair_22_33_278_reject_commit()
    """)
  end

  defp drop_commit_rejection do
    Repo.query!("DROP TRIGGER IF EXISTS repair_22_33_278_commit ON migrations_status")
    Repo.query!("DROP FUNCTION IF EXISTS repair_22_33_278_reject_commit()")
  end

  defp restart_cache do
    cache_name = BackgroundMigrations.cache_name()
    old_pid = Process.whereis(cache_name)
    assert is_pid(old_pid)
    assert :ok = Supervisor.terminate_child(Explorer.Supervisor, BackgroundMigrations.child_id())
    refute Process.alive?(old_pid)
    assert {:ok, new_pid} = Supervisor.restart_child(Explorer.Supervisor, BackgroundMigrations.child_id())
    assert new_pid != old_pid
    assert Process.whereis(cache_name) == new_pid
  end

  defp primary_key_columns do
    %{rows: rows} =
      Repo.query!("""
      SELECT k.column_name
      FROM information_schema.table_constraints AS c
      JOIN information_schema.key_column_usage AS k
        ON k.constraint_schema = c.constraint_schema AND k.constraint_name = c.constraint_name
      WHERE c.table_schema = 'public' AND c.table_name = 'internal_transactions'
        AND c.constraint_type = 'PRIMARY KEY'
      ORDER BY k.ordinal_position
      """)

    List.flatten(rows)
  end

  defp assert_import_layout(updated?) do
    %{"tx" => tx} = @recordings |> Path.join("struct_logger.json") |> File.read!() |> Jason.decode!()
    recording = @recordings |> Path.join("calltracer.json") |> File.read!() |> Jason.decode!()
    server = Bypass.open()

    Bypass.expect_once(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)

      assert [%{"id" => id, "jsonrpc" => "2.0", "method" => "debug_traceTransaction", "params" => params}] =
               Jason.decode!(body)

      assert params == [tx["hash"], %{"tracer" => "callTracer", "timeout" => "5s"}]

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!([%{jsonrpc: "2.0", id: id, result: recording["result"]}]))
    end)

    params = %{
      hash_data: tx["hash"],
      block_number: EthereumJSONRPC.quantity_to_integer(tx["blockNumber"]),
      transaction_index: EthereumJSONRPC.quantity_to_integer(tx["transactionIndex"])
    }

    assert {:ok, traces} =
             Geth.fetch_internal_transactions([params],
               transport: EthereumJSONRPC.HTTP,
               transport_options: [
                 urls: ["http://127.0.0.1:#{server.port}"],
                 fallback_urls: [],
                 http: EthereumJSONRPC.HTTP.HTTPoison
               ],
               variant: Geth
             )

    assert length(traces) > 1

    assert {:error, :fixture_cleanup} =
             Repo.transaction(fn ->
               addresses =
                 traces
                 |> Enum.flat_map(&[Map.fetch!(&1, :from_address_hash), Map.fetch!(&1, :to_address_hash)])
                 |> Enum.uniq()
                 |> Map.new(fn hash -> {hash, insert(:address, hash: hash)} end)

               block = insert(:block, hash: tx["blockHash"], number: params.block_number)

               transaction =
                 insert(:transaction,
                   hash: tx["hash"],
                   from_address: Map.fetch!(addresses, tx["from"]),
                   to_address: Map.fetch!(addresses, tx["to"])
                 )
                 |> with_block(block, index: params.transaction_index, status: :ok)

               insert(:pending_block_operation, block_hash: block.hash, block_number: block.number)

               changes =
                 Enum.map(traces, fn trace ->
                   changeset = InternalTransaction.blockless_changeset(%InternalTransaction{}, trace)
                   assert changeset.valid?, inspect(changeset.errors)
                   Changeset.apply_changes(changeset) |> Map.from_struct() |> Map.take(Map.keys(changeset.changes))
                 end)

               assert {:ok, result} =
                        Multi.new()
                        |> InternalTransactions.run(changes, %{timestamps: Import.timestamps()})
                        |> Repo.transaction()

               assert length(result.internal_transactions) == length(traces)

               rows =
                 Repo.all(
                   from(row in InternalTransaction,
                     where: row.transaction_hash == ^transaction.hash,
                     order_by: row.index
                   )
                 )

               assert length(rows) == length(traces)

               Enum.each(Enum.with_index(rows), fn {row, index} ->
                 assert row.block_number == params.block_number
                 assert row.transaction_index == params.transaction_index
                 assert row.index == index

                 if updated? do
                   assert row.block_hash == nil
                   assert row.block_index == nil
                 else
                   assert row.block_hash == block.hash
                   assert row.block_index == index
                 end
               end)

               assert {:ok, transaction_hash} = Hash.Full.cast(tx["hash"])
               assert Enum.all?(rows, &(&1.transaction_hash == transaction_hash))
               Repo.rollback(:fixture_cleanup)
             end)
  end
end
